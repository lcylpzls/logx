// logx 压力测试：测量每秒可写入的日志条数。
//
// 用法：go run .
package main

import (
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/lcylpzls/logx"
)

func main() {
	fmt.Println()
	fmt.Println("  ╔══════════════════════════════════════════╗")
	fmt.Println("  ║        logx 吞吐量压力测试               ║")
	fmt.Println("  ╚══════════════════════════════════════════╝")
	fmt.Println()

	// 测试 1：文件异步写入（默认模式）
	bench("文件异步写入", func() logx.Logger {
		dir := "./logx-bench-async"
		os.RemoveAll(dir)
		l, _ := logx.NewBuilder().
			EnableFileLog(
				logx.WithLogDir(dir),
				logx.WithFilename("bench.log"),
				logx.WithWriteMode(logx.AsyncWriteMode),
				logx.WithBufferSize(65536),
				logx.WithFlushInterval(100*time.Millisecond),
				logx.WithLevels(logx.InfoLevel),
			).
			Build()
		return l
	})

	// 测试 2：文件同步写入
	bench("文件同步写入", func() logx.Logger {
		dir := "./logx-bench-sync"
		os.RemoveAll(dir)
		l, _ := logx.NewBuilder().
			EnableFileLog(
				logx.WithLogDir(dir),
				logx.WithFilename("bench.log"),
				logx.WithWriteMode(logx.SyncWriteMode),
				logx.WithLevels(logx.InfoLevel),
			).
			Build()
		return l
	})

	// 测试 3：文件异步 + 3 个结构化字段
	benchFields("文件异步 + 3 个字段", func() logx.Logger {
		dir := "./logx-bench-fields"
		os.RemoveAll(dir)
		l, _ := logx.NewBuilder().
			EnableFileLog(
				logx.WithLogDir(dir),
				logx.WithFilename("bench.log"),
				logx.WithWriteMode(logx.AsyncWriteMode),
				logx.WithBufferSize(65536),
				logx.WithFlushInterval(100*time.Millisecond),
				logx.WithLevels(logx.InfoLevel),
			).
			Build()
		return l
	})

	// 测试 4：控制台 Info（抑制输出）
	benchMute("控制台 Info（无颜色）", func() logx.Logger {
		l, _ := logx.NewBuilder().
			EnableConsole(logx.InfoLevel).
			Build()
		return l
	})

	// 测试 5：未启用的 Debug 级别（过滤开销）
	benchMute("未启用级别过滤", func() logx.Logger {
		l, _ := logx.NewBuilder().
			EnableConsole(logx.InfoLevel).
			Build()
		return l
	})

	// 清理
	os.RemoveAll("./logx-bench-async")
	os.RemoveAll("./logx-bench-sync")
	os.RemoveAll("./logx-bench-fields")

	fmt.Println()
	fmt.Println("  注：控制台测试写入 /dev/null，文件测试写入磁盘")
	fmt.Println("     异步模式使用无锁环形通道 + 批量刷盘")
}

// bench 运行一轮基准测试（普通消息，1 秒）。
func bench(name string, factory func() logx.Logger) {
	logger := factory()
	defer logger.Close()

	var count atomic.Int64
	stop := make(chan struct{})

	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				fmt.Printf("\r     %-32s  %d 条", name, count.Load())
			}
		}
	}()

	start := time.Now()
	deadline := start.Add(1 * time.Second)
	for time.Now().Before(deadline) {
		logger.Info("benchmark message")
		count.Add(1)
	}
	elapsed := time.Since(start)

	close(stop)
	logger.Sync()

	total := count.Load()
	rate := float64(total) / elapsed.Seconds()

	fmt.Printf("\r  ✅ %-32s %10s 条/s  (%d 条 / %.2fs)\n",
		name, formatNum(int64(rate)), total, elapsed.Seconds())
}

// benchFields 运行一轮基准测试（带 3 个字段，1 秒）。
func benchFields(name string, factory func() logx.Logger) {
	logger := factory()
	defer logger.Close()

	var count atomic.Int64
	stop := make(chan struct{})

	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				fmt.Printf("\r     %-32s  %d 条", name, count.Load())
			}
		}
	}()

	start := time.Now()
	deadline := start.Add(1 * time.Second)
	for time.Now().Before(deadline) {
		logger.Info("benchmark message",
			logx.Int("id", int(count.Load())),
			logx.String("status", "ok"),
			logx.Bool("cached", true),
		)
		count.Add(1)
	}
	elapsed := time.Since(start)

	close(stop)
	logger.Sync()

	total := count.Load()
	rate := float64(total) / elapsed.Seconds()

	fmt.Printf("\r  ✅ %-32s %10s 条/s  (%d 条 / %.2fs)\n",
		name, formatNum(int64(rate)), total, elapsed.Seconds())
}

// benchMute 运行一轮测试，抑制 stdout/stderr（用于控制台测试）。
func benchMute(name string, factory func() logx.Logger) {
	// 抑制输出
	null, _ := os.Open(os.DevNull)
	stdout := os.Stdout
	stderr := os.Stderr
	os.Stdout = null
	os.Stderr = null

	logger := factory()

	var count atomic.Int64
	stop := make(chan struct{})

	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				fmt.Fprintf(stdout, "\r     %-32s  %d 条", name, count.Load())
			}
		}
	}()

	start := time.Now()
	deadline := start.Add(1 * time.Second)

	if name == "未启用级别过滤" {
		for time.Now().Before(deadline) {
			logger.Debug("this will be filtered")
			count.Add(1)
		}
	} else {
		for time.Now().Before(deadline) {
			logger.Info("benchmark message")
			count.Add(1)
		}
	}
	elapsed := time.Since(start)

	close(stop)
	logger.Sync()
	logger.Close()

	// 恢复输出
	os.Stdout = stdout
	os.Stderr = stderr
	null.Close()

	total := count.Load()
	rate := float64(total) / elapsed.Seconds()

	fmt.Printf("\r  ✅ %-32s %10s 条/s  (%d 条 / %.2fs)\n",
		name, formatNum(int64(rate)), total, elapsed.Seconds())
}

// formatNum 格式化数字为带千分位的字符串。
func formatNum(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	s := fmt.Sprintf("%d", n)
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}
