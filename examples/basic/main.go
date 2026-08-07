// 最简用法：控制台输出 Info 级别日志。
package main

import "github.com/lcylpzls/logx"

func main() {
	logger, err := logx.NewBuilder().
		EnableConsole(logx.InfoLevel).
		Build()
	if err != nil {
		panic(err)
	}

	logger.Info("Hello, logx!", logx.FieldGroup{})
	logger.Info("结构化字段", logx.Fields(logx.String("user", "admin"), logx.Int("port", 8080)))
	logger.Infof("格式化输出：服务运行在第 %d 端口", 8080)
}
