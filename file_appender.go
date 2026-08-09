package logx

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// defaultSlotSize 是异步写入槽位的默认容量。
// 槽位由空闲通道复用，稳态下零分配；超长日志按需扩容。
const defaultSlotSize = 4 * 1024

// ---------------------------------------------------------------------------
// FileAppender — 文件输出器
// ---------------------------------------------------------------------------

// fileAppender 实现了 Appender 接口，支持同步/异步双引擎写入、
// 基于文件大小和自然天的日志轮转、以及软链接自动维护。
//
// 物理文件命名：<basename>-2006-01-02T15-04-05.000.log
// 软链接：       <basename>.log → 最新物理文件
type fileAppender struct {
	cfg FileConfig

	mu          sync.Mutex
	file        *os.File
	currentSize int64
	rotateAt    time.Time // 下一次自然天轮转时间

	dir           string // 绝对路径的日志目录
	basenameNoExt string // 不带后缀的基础名
	ext           string // 文件后缀（含点）
	symlinkPath   string // 软链接完整路径

	// 异步模式专属字段
	writeCh chan []byte // 待写日志槽位
	freeCh  chan []byte // 空闲槽位（复用，稳态零分配）
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup

	closeOnce sync.Once
	closed    int32 // atomic: 0=open, 1=closed

	errorHandler func(error) // 内部错误统一出口，nil 时降级为 stderr

	// 运行指标（原子计数）
	written      atomic.Uint64 // 成功写入的日志条数
	writeBytes   atomic.Uint64 // 成功写入的字节数
	rotations    atomic.Uint64 // 文件轮转次数
	compressions atomic.Uint64 // gzip 压缩成功次数
	cleanups     atomic.Uint64 // 生命周期清理执行次数
}

// lifecycleCheckInterval 生命周期后台扫描间隔（测试注入用）。
var lifecycleCheckInterval = 10 * time.Minute

// platformIsWindows 平台判断（测试注入用，生产行为不变）。
var platformIsWindows = runtime.GOOS == "windows"

// newFileAppender 创建一个新的文件输出器。
// cfg 必须至少设置 LogDir 和 Filename。
func newFileAppender(cfg *FileConfig) (Appender, error) {
	if cfg == nil {
		return nil, fmt.Errorf("logx：FileConfig 不能为 nil")
	}
	if cfg.LogDir == "" {
		return nil, fmt.Errorf("logx：日志目录不能为空")
	}
	if cfg.Filename == "" {
		return nil, fmt.Errorf("logx：日志文件名不能为空")
	}
	if cfg.MaxSize <= 0 {
		cfg.MaxSize = 100
	}
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 4096
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = time.Second
	}
	if cfg.MaxAge <= 0 {
		cfg.MaxAge = 180
	}
	if cfg.MaxBackups <= 0 {
		cfg.MaxBackups = 100
	}

	// 确保目录存在
	absDir, err := absPathFn(cfg.LogDir)
	if err != nil {
		return nil, fmt.Errorf("logx：解析日志目录路径失败：%w", err)
	}
	if err := os.MkdirAll(absDir, 0755); err != nil {
		return nil, fmt.Errorf("logx：日志目录创建失败：%w", err)
	}

	// 分离文件名和后缀
	ext := filepath.Ext(cfg.Filename)
	basenameNoExt := strings.TrimSuffix(cfg.Filename, ext)
	if ext == "" {
		ext = ".log"
	}

	fa := &fileAppender{
		cfg:           *cfg,
		dir:           absDir,
		basenameNoExt: basenameNoExt,
		ext:           ext,
		symlinkPath:   filepath.Join(absDir, cfg.Filename),
		errorHandler:  cfg.ErrorHandler,
	}

	// 打开初始物理文件
	if err := fa.openNewFile(); err != nil {
		return nil, fmt.Errorf("logx：创建日志文件失败：%w", err)
	}

	// 创建统一的 context 用于控制所有后台协程
	fa.ctx, fa.cancel = context.WithCancel(context.Background())

	// 异步模式：槽位通道 + 后台刷盘协程
	if fa.cfg.WriteMode == AsyncWriteMode {
		fa.writeCh = make(chan []byte, fa.cfg.BufferSize)
		fa.freeCh = make(chan []byte, fa.cfg.BufferSize)
		// 预分配槽位：以固定内存换取运行时零分配（内存 ≈ BufferSize × 4KB）
		for i := 0; i < fa.cfg.BufferSize; i++ {
			fa.freeCh <- make([]byte, 0, defaultSlotSize)
		}
		fa.wg.Add(1)
		go fa.runFlushLoop()
	}

	// 启动后台生命周期管理
	fa.wg.Add(1)
	go fa.runLifecycle()

	return fa, nil
}

// ---------------------------------------------------------------------------
// Appender 接口实现
// ---------------------------------------------------------------------------

// Append 写入日志数据。
func (fa *fileAppender) Append(level Level, p []byte) (n int, err error) {
	if atomic.LoadInt32(&fa.closed) == 1 {
		return 0, fmt.Errorf("logx：文件输出器已关闭")
	}

	if fa.cfg.WriteMode == AsyncWriteMode {
		return fa.appendAsync(p)
	}
	return fa.appendSync(p)
}

// Sync 强制同步刷盘。
func (fa *fileAppender) Sync() error {
	if fa.cfg.WriteMode == AsyncWriteMode {
		// 异步模式：Sync 将通道中所有积压数据刷入磁盘
		return fa.syncAsync()
	}

	fa.mu.Lock()
	defer fa.mu.Unlock()
	if fa.file != nil {
		return fa.file.Sync()
	}
	return nil
}

// Close 关闭输出器，释放资源。
func (fa *fileAppender) Close() error {
	var err error
	fa.closeOnce.Do(func() {
		atomic.StoreInt32(&fa.closed, 1)

		// 取消所有后台协程
		fa.cancel()

		// 排空异步通道中残留的日志
		if fa.cfg.WriteMode == AsyncWriteMode {
			fa.drainAsync()
		}

		// 等待所有后台协程退出
		fa.wg.Wait()

		fa.mu.Lock()
		defer fa.mu.Unlock()
		if fa.file != nil {
			if syncErr := fa.file.Sync(); syncErr != nil && err == nil {
				err = syncErr
			}
			if closeErr := closeFileFn(fa.file); closeErr != nil && err == nil {
				err = closeErr
			}
			fa.file = nil
		}
	})
	return err
}

// ---------------------------------------------------------------------------
// 同步写入
// ---------------------------------------------------------------------------

func (fa *fileAppender) appendSync(p []byte) (int, error) {
	fa.mu.Lock()
	defer fa.mu.Unlock()

	if err := fa.checkRotation(len(p)); err != nil {
		return 0, err
	}

	n, err := fa.file.Write(p)
	if err == nil {
		fa.currentSize += int64(n)
		fa.written.Add(1)
		fa.writeBytes.Add(uint64(n))
	}
	return n, err
}

// ---------------------------------------------------------------------------
// 异步写入
// ---------------------------------------------------------------------------

func (fa *fileAppender) appendAsync(p []byte) (int, error) {
	// 有界背压：无空闲槽时阻塞等待消费者归还，保证稳态零分配、零丢弃。
	// 槽位守恒（预填 BufferSize 个，写盘后归还）保证发送方永不阻塞。
	data := <-fa.freeCh
	data = data[:0]
	if cap(data) < len(p) {
		// 超长日志（罕见）：按需分配新槽位，旧小槽由 GC 回收
		data = make([]byte, len(p))
	}
	data = append(data[:0], p...)
	fa.writeCh <- data
	return len(p), nil
}

// runFlushLoop 后台批量刷盘协程。
func (fa *fileAppender) runFlushLoop() {
	defer fa.wg.Done()

	var buf bytes.Buffer
	ticker := time.NewTicker(fa.cfg.FlushInterval)
	defer ticker.Stop()

	flush := func() {
		if buf.Len() == 0 {
			return
		}

		fa.mu.Lock()
		if err := fa.checkRotation(buf.Len()); err != nil {
			fa.reportError(fmt.Errorf("异步刷盘轮转失败：%w", err))
			buf.Reset()
			fa.mu.Unlock()
			return
		}
		n, err := fa.file.Write(buf.Bytes())
		if err == nil {
			fa.currentSize += int64(n)
			fa.written.Add(1)
			fa.writeBytes.Add(uint64(n))
		} else {
			fa.reportError(fmt.Errorf("异步刷盘写入失败：%w", err))
		}
		buf.Reset()
		fa.mu.Unlock()
	}

	for {
		select {
		case <-fa.ctx.Done():
			// 退出前排空通道
			fa.drainPending(&buf)
			flush()
			return

		case data := <-fa.writeCh:
			buf.Write(data)
			fa.recycleSlot(data)
			// 达到批量大小阈值时立即刷盘
			if buf.Len() >= 64*1024 { // 64KB 批量阈值
				flush()
			}

		case <-ticker.C:
			flush()
		}
	}
}

// drainPending 将写通道中残留的数据全部写入本地批量缓冲（不落盘），并归还槽位。
func (fa *fileAppender) drainPending(buf *bytes.Buffer) {
	for {
		select {
		case data := <-fa.writeCh:
			buf.Write(data)
			fa.recycleSlot(data)
		default:
			return
		}
	}
}

// drainAsync 排空通道中所有残留数据。
func (fa *fileAppender) drainAsync() {
	for {
		select {
		case data := <-fa.writeCh:
			fa.mu.Lock()
			if err := fa.checkRotation(len(data)); err != nil {
				fa.reportError(fmt.Errorf("关闭排空轮转失败：%w", err))
				fa.recycleSlot(data)
				fa.mu.Unlock()
				continue
			}
			n, err := fa.file.Write(data)
			if err == nil {
				fa.currentSize += int64(n)
				fa.written.Add(1)
				fa.writeBytes.Add(uint64(n))
			} else {
				fa.reportError(fmt.Errorf("关闭排空写入失败：%w", err))
			}
			fa.recycleSlot(data)
			fa.mu.Unlock()
		default:
			return
		}
	}
}

// syncAsync 将异步通道中的积压数据强制刷盘。
func (fa *fileAppender) syncAsync() error {
	fa.mu.Lock()
	defer fa.mu.Unlock()

	// 排空通道
	for {
		select {
		case data := <-fa.writeCh:
			if err := fa.checkRotation(len(data)); err != nil {
				fa.recycleSlot(data)
				return err
			}
			n, err := fa.file.Write(data)
			fa.recycleSlot(data)
			if err != nil {
				return err
			}
			fa.written.Add(1)
			fa.writeBytes.Add(uint64(n))
		default:
			if fa.file != nil {
				return fa.file.Sync()
			}
			return nil
		}
	}
}

// recycleSlot 将槽位归还空闲池。池满时丢弃该槽位（由 GC 回收），
// 绝不在热路径上阻塞——并发新建槽位可能导致池容量瞬时不足，阻塞会引发死锁。
func (fa *fileAppender) recycleSlot(data []byte) {
	select {
	case fa.freeCh <- data:
	default:
	}
}

// ---------------------------------------------------------------------------
// 文件管理与轮转
// ---------------------------------------------------------------------------

// physicalName 根据时间戳生成物理文件名。
func (fa *fileAppender) physicalName(t time.Time) string {
	ts := t.Format("2006-01-02T15-04-05.000")
	return fmt.Sprintf("%s-%s%s", fa.basenameNoExt, ts, fa.ext)
}

// openNewFile 创建新的物理日志文件并更新软链接。
func (fa *fileAppender) openNewFile() error {
	now := time.Now()
	physicalPath := filepath.Join(fa.dir, fa.physicalName(now))

	f, err := openNewFileFn(physicalPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("logx：无法创建物理日志文件 %s：%w", physicalPath, err)
	}

	// 关闭旧文件
	if fa.file != nil {
		closeFileFn(fa.file)
	}

	fa.file = f

	// 获取当前文件大小
	info, err := fileStatFn(f)
	if err != nil {
		return fmt.Errorf("logx：获取文件状态失败：%w", err)
	}
	fa.currentSize = info.Size()

	// 计算下一次自然天轮转时间
	fa.rotateAt = nextMidnight(now)

	// 更新软链接
	fa.updateSymlink(physicalPath)

	return nil
}

// checkRotation 检查是否需要轮转（容量 or 时间），必要时执行。
// 必须在持有 mu 的情况下调用。
func (fa *fileAppender) checkRotation(dataLen int) error {
	needRotate := false

	// 容量预判：原子边界保护 —— 若当前大小 + 新日志 > 阈值，先轮转
	if fa.cfg.MaxSize > 0 {
		maxBytes := int64(fa.cfg.MaxSize) * 1024 * 1024
		if fa.currentSize+int64(dataLen) > maxBytes {
			needRotate = true
		}
	}

	// 自然天轮转
	if !needRotate && time.Now().After(fa.rotateAt) {
		needRotate = true
	}

	if needRotate {
		fa.rotations.Add(1)
		return fa.openNewFile()
	}

	return nil
}

// 可替换的系统函数（测试注入用，生产行为不变）。
var (
	removePathFn    = os.Remove
	createSymlinkFn = os.Symlink
	absPathFn       = filepath.Abs
	openSrcFileFn   = os.Open
	createDstFileFn = os.Create
	pathStatFn      = os.Stat
	fileStatFn      = func(f *os.File) (os.FileInfo, error) { return f.Stat() }
	openNewFileFn   = os.OpenFile
	sortStatFn      = os.Stat
	ioCopyFn        = io.Copy
	closeFileFn     = func(f *os.File) error { return f.Close() }
	closeGzipFn     = func(w gzipWriteCloser) error { return w.Close() }
)

// gzipWriteCloser 是 gzip.Writer 的窄接口，便于测试注入关闭失败。
type gzipWriteCloser interface {
	io.Writer
	Close() error
}

// newGzipWriterFn 创建 gzip 写入器（可注入）。
var newGzipWriterFn = func(w io.Writer) gzipWriteCloser {
	return gzip.NewWriter(w)
}

// updateSymlink 创建或更新软链接，使其指向最新的物理文件。
func (fa *fileAppender) updateSymlink(physicalPath string) {
	// Windows 不支持 Symlink（需要特殊权限），非 Windows 才创建
	if platformIsWindows {
		return
	}
	fa.createSymlink(physicalPath)
}

// createSymlink 实际创建软链接（不包含平台判断，便于测试）。
func (fa *fileAppender) createSymlink(physicalPath string) {
	// 删除旧软链接
	removePathFn(fa.symlinkPath)

	// 创建新软链接
	if err := createSymlinkFn(filepath.Base(physicalPath), fa.symlinkPath); err != nil {
		fa.reportError(fmt.Errorf("创建软链接失败：%w", err))
	}
}

// nextMidnight 返回下一个午夜的 time.Time。
func nextMidnight(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d+1, 0, 0, 0, 0, t.Location())
}

// ---------------------------------------------------------------------------
// 生命周期管理
// ---------------------------------------------------------------------------

// runLifecycle 后台生命周期协程：定期扫描并执行清理和压缩。
func (fa *fileAppender) runLifecycle() {
	defer fa.wg.Done()

	// 启动时立即执行一次清理
	fa.cleanup()

	ticker := time.NewTicker(lifecycleCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-fa.ctx.Done():
			return
		case <-ticker.C:
			fa.cleanup()
		}
	}
}

// cleanup 执行一次完整的清理周期：删除过期文件 + 压缩旧文件。
func (fa *fileAppender) cleanup() {
	fa.cleanups.Add(1)

	pattern := fmt.Sprintf("%s-*%s", fa.basenameNoExt, fa.ext)
	globPattern := filepath.Join(fa.dir, pattern)

	matches := fa.scanMatches(globPattern)
	if matches == nil {
		return
	}

	// 按修改时间排序（旧→新）
	sortByModTime(matches)

	fa.mu.Lock()
	currentPhysical := ""
	if fa.file != nil {
		currentPhysical = fa.file.Name()
	}
	fa.mu.Unlock()

	now := time.Now()

	// 第一遍：按 MaxAge 删除
	if fa.cfg.MaxAge > 0 {
		cutoff := now.AddDate(0, 0, -fa.cfg.MaxAge)
		for _, path := range matches {
			if path == currentPhysical {
				continue // 不删除当前文件
			}
			info, err := pathStatFn(path)
			if err != nil {
				continue
			}
			if info.ModTime().Before(cutoff) {
				if rmErr := os.Remove(path); rmErr != nil {
					fa.reportError(fmt.Errorf("按 MaxAge 清理日志文件失败 %s：%w", path, rmErr))
				}
			}
		}
	}

	// 重新扫描（部分文件已被删除）
	matches = fa.scanMatches(globPattern)
	sortByModTime(matches)

	// 第二遍：按 MaxBackups 删除（保留最新的 N 个）
	if fa.cfg.MaxBackups > 0 && len(matches) > fa.cfg.MaxBackups {
		toDelete := len(matches) - fa.cfg.MaxBackups
		deleted := 0
		for _, path := range matches {
			if path == currentPhysical {
				continue
			}
			if deleted >= toDelete {
				break
			}
			if rmErr := os.Remove(path); rmErr != nil {
				fa.reportError(fmt.Errorf("按 MaxBackups 清理日志文件失败 %s：%w", path, rmErr))
			}
			deleted++
		}
	}

	// 第三遍：压缩超过 CompressAfter 天的文件
	if fa.cfg.CompressAfter > 0 {
		compressCutoff := now.AddDate(0, 0, -fa.cfg.CompressAfter)
		matches = fa.scanMatches(globPattern)
		for _, path := range matches {
			if path == currentPhysical {
				continue
			}
			if strings.HasSuffix(path, ".gz") {
				continue
			}
			info, err := pathStatFn(path)
			if err != nil {
				continue
			}
			if !info.ModTime().After(compressCutoff) {
				fa.compressFile(path)
			}
		}
	}
}

// scanMatches 扫描匹配的日志文件。扫描失败时上报错误并返回 nil。
func (fa *fileAppender) scanMatches(globPattern string) []string {
	matches, err := filepath.Glob(globPattern)
	if err != nil {
		fa.reportError(fmt.Errorf("扫描日志文件失败：%w", err))
		return nil
	}
	return matches
}

// compressFile 将指定文件压缩为 .gz 格式，压缩成功后将原文件删除。
func (fa *fileAppender) compressFile(path string) {
	gzPath := path + ".gz"

	// 打开源文件
	src, err := openSrcFileFn(path)
	if err != nil {
		fa.reportError(fmt.Errorf("压缩-打开源文件失败 %s：%w", path, err))
		return
	}

	// 创建目标 .gz 文件
	dst, err := createDstFileFn(gzPath)
	if err != nil {
		fa.reportError(fmt.Errorf("压缩-创建目标文件失败 %s：%w", gzPath, err))
		return
	}

	gw := newGzipWriterFn(dst)
	if _, err := ioCopyFn(gw, src); err != nil {
		closeGzipFn(gw)
		closeFileFn(dst)
		removePathFn(gzPath)
		fa.reportError(fmt.Errorf("压缩-写入失败 %s：%w", path, err))
		return
	}

	if err := closeGzipFn(gw); err != nil {
		closeFileFn(dst)
		removePathFn(gzPath)
		fa.reportError(fmt.Errorf("压缩-关闭gzip失败 %s：%w", path, err))
		return
	}
	if err := closeFileFn(dst); err != nil {
		removePathFn(gzPath)
		fa.reportError(fmt.Errorf("压缩-关闭文件失败 %s：%w", path, err))
		return
	}

	// 压缩成功后先关闭源文件再删除（Windows 下打开的文件无法删除）
	if err := closeFileFn(src); err != nil {
		removePathFn(gzPath)
		fa.reportError(fmt.Errorf("压缩-关闭源文件失败 %s：%w", path, err))
		return
	}
	if rmErr := removePathFn(path); rmErr != nil {
		fa.reportError(fmt.Errorf("压缩后删除源文件失败 %s：%w", path, rmErr))
		return
	}
	fa.compressions.Add(1)
}

// reportError 将内部错误统一交给错误处理器；未配置时降级输出到 stderr。
func (fa *fileAppender) reportError(err error) {
	if err == nil {
		return
	}
	if fa.errorHandler != nil {
		fa.errorHandler(err)
		return
	}
	fmt.Fprintf(os.Stderr, "logx：%v\n", err)
}

// Metrics 返回文件输出器的运行指标快照。
func (fa *fileAppender) Metrics() Metrics {
	return Metrics{
		Writes:       fa.written.Load(),
		WriteBytes:   fa.writeBytes.Load(),
		Rotations:    fa.rotations.Load(),
		Compressions: fa.compressions.Load(),
		Cleanups:     fa.cleanups.Load(),
	}
}

// ---------------------------------------------------------------------------
// 工具函数
// ---------------------------------------------------------------------------

// sortByModTime 按文件修改时间升序排列（最旧的在前）。
func sortByModTime(paths []string) {
	for i := 0; i < len(paths); i++ {
		for j := i + 1; j < len(paths); j++ {
			infoI, errI := sortStatFn(paths[i])
			infoJ, errJ := sortStatFn(paths[j])
			if errI != nil || errJ != nil {
				continue
			}
			if infoJ.ModTime().Before(infoI.ModTime()) {
				paths[i], paths[j] = paths[j], paths[i]
			}
		}
	}
}
