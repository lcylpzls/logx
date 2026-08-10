package logx

import "github.com/lcylpzls/errx"

// logx 错误码：统一使用 errx 结构化错误。
const (
	// CodeInvalidConfig 日志配置非法。
	CodeInvalidConfig errx.Code = "LOGX_INVALID_CONFIG"
	// CodeIOFailed 日志 IO 操作失败（创建/写入/刷盘/轮转/压缩/清理等）。
	CodeIOFailed errx.Code = "LOGX_IO_FAILED"
	// CodeClosed 日志输出器已关闭。
	CodeClosed errx.Code = "LOGX_CLOSED"
)

func init() {
	errx.RegisterCode(CodeInvalidConfig, "日志配置非法")
	errx.RegisterCodeKind(CodeInvalidConfig, errx.KindInvalid)
	errx.RegisterCode(CodeIOFailed, "日志 IO 操作失败")
	errx.RegisterCodeKind(CodeIOFailed, errx.KindUnavailable)
	errx.RegisterCode(CodeClosed, "日志输出器已关闭")
	errx.RegisterCodeKind(CodeClosed, errx.KindUnavailable)
}
