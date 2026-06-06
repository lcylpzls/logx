// 文件输出 + 轮转 + 压缩示例。
package main

import (
	"os"

	"github.com/lcylpzls/logx"
)

func main() {
	// 创建临时日志目录
	logDir := "./logx-example-logs"
	os.RemoveAll(logDir) // 清理旧数据

	logger, err := logx.NewBuilder().
		// 文件通道：启用 Info 级别，按 1MB 切割，7 天后压缩
		EnableFileLog(
			logx.WithLogDir(logDir),
			logx.WithFilename("app.log"),
			logx.WithMaxSize(1),                    // 1MB 自动切割
			logx.WithMaxAge(180),                   // 保留 180 天
			logx.WithCompressAfter(7),              // 7 天后 gzip 压缩
			logx.WithWriteMode(logx.SyncWriteMode), // 同步写入
			logx.WithLevels(logx.InfoLevel),
		).
		Build()
	if err != nil {
		panic(err)
	}
	defer logger.Close()

	logger.Info("文件日志示例", logx.Int("pid", os.Getpid()))
	logger.Infof("日志目录：%s", logDir)
}
