// 优雅退出示例：监听 SIGINT/SIGTERM，先同步刷盘再退出，避免异步日志丢失。
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lcylpzls/logx"
)

func main() {
	logger, err := logx.NewBuilder().
		EnableConsole(logx.InfoLevel).
		EnableFileLog(
			logx.WithLogDir("./logs"),
			logx.WithFilename("app.log"),
			logx.WithWriteMode(logx.AsyncWriteMode),
			logx.WithLevels(logx.InfoLevel),
		).
		Build()
	if err != nil {
		fmt.Fprintf(os.Stderr, "初始化 logx 失败：%v\n", err)
		os.Exit(1)
	}

	// 监听 SIGINT（Ctrl+C）与 SIGTERM（kill）
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Info("服务启动，按 Ctrl+C 优雅退出")
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("收到退出信号，正在刷盘...")
			if err := logger.Sync(); err != nil {
				fmt.Fprintf(os.Stderr, "刷盘失败：%v\n", err)
			}
			if err := logger.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "关闭失败：%v\n", err)
			}
			fmt.Println("已安全退出")
			return
		case <-ticker.C:
			logger.Info("服务运行中")
		}
	}
}
