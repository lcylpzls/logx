// 劫持标准库 log 示例：一行代码将 log.Println 路由到 logx。
package main

import (
	"log"
	"os"

	"github.com/lcylpzls/logx"
)

func main() {
	logDir := "./logx-example-stdlog"
	os.RemoveAll(logDir)

	logger, err := logx.NewBuilder().
		EnableConsole(logx.InfoLevel).
		EnableFileLog(
			logx.WithLogDir(logDir),
			logx.WithFilename("std.log"),
			logx.WithLevels(logx.InfoLevel),
		).
		Build()
	if err != nil {
		panic(err)
	}
	defer logger.Close()

	// 替换标准库 log
	logx.ReplaceStdLogger(logger)
	defer logx.RestoreStdLogger()

	// 老代码的 log.Println 自动流经 logx
	log.Println("这条来自标准库 log，但由 logx 接管")
	log.Printf("PID=%d", os.Getpid())
}
