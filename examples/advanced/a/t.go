package a

import "github.com/lcylpzls/logx"

func TestLogCaller(l logx.Logger) {
	l.Info("这是一条测试消息，用于跟踪发送该条消息的源码源头。")
}
