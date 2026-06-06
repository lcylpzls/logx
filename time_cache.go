package logx

import (
	"sync"
	"sync/atomic"
	"time"
)

// ---------------------------------------------------------------------------
// 时间缓存 — 100ms 更新一次预格式化时间，零分配时间输出
// ---------------------------------------------------------------------------

// cachedTime 存储预格式化的当前时间字节数组。
var cachedTime atomic.Value // stores []byte

var initTimeCacheOnce sync.Once

// initTimeCache 启动后台时间缓存协程（全局唯一）。
func initTimeCache() {
	initTimeCacheOnce.Do(func() {
		// 初始值
		cachedTime.Store(formatTimeBytes(time.Now()))

		go func() {
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()
			for range ticker.C {
				cachedTime.Store(formatTimeBytes(time.Now()))
			}
		}()
	})
}

// getCachedTime 返回当前缓存的格式化时间字节数组。
// 首次调用时自动启动时间缓存协程。
func getCachedTime() []byte {
	if v := cachedTime.Load(); v != nil {
		return v.([]byte)
	}
	initTimeCache()
	return []byte(time.Now().Format(timeLayout))
}

// timeLayout 日志时间格式："2006-01-02 15:04:05.000"
const timeLayout = "2006-01-02 15:04:05.000"

// formatTimeBytes 手动格式化时间为 []byte，避免 time.Format 的内存分配。
// 输出格式："2006-01-02 15:04:05.000"（23 字节）。
func formatTimeBytes(t time.Time) []byte {
	year, month, day := t.Date()
	hour, minute, second := t.Clock()
	ms := t.Nanosecond() / 1e6

	buf := make([]byte, 0, 23)
	buf = appendInt4(buf, year)
	buf = append(buf, '-')
	buf = appendInt2(buf, int(month))
	buf = append(buf, '-')
	buf = appendInt2(buf, day)
	buf = append(buf, ' ')
	buf = appendInt2(buf, hour)
	buf = append(buf, ':')
	buf = appendInt2(buf, minute)
	buf = append(buf, ':')
	buf = appendInt2(buf, second)
	buf = append(buf, '.')
	buf = appendInt3(buf, ms)
	return buf
}

// --- 整数格式化辅助函数 ---

func appendInt4(buf []byte, v int) []byte {
	return append(buf,
		byte('0'+v/1000%10),
		byte('0'+v/100%10),
		byte('0'+v/10%10),
		byte('0'+v%10),
	)
}

func appendInt2(buf []byte, v int) []byte {
	return append(buf,
		byte('0'+v/10%10),
		byte('0'+v%10),
	)
}

func appendInt3(buf []byte, v int) []byte {
	return append(buf,
		byte('0'+v/100%10),
		byte('0'+v/10%10),
		byte('0'+v%10),
	)
}
