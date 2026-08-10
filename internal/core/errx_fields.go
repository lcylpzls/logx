package core

import "github.com/lcylpzls/errx"

// FieldsFromError 将结构化错误转换为日志字段组：
// 固定输出 err.code / err.kind / err.retryable / err.code_desc / err.message，
// 以及错误携带的结构化 KV 字段。
//
// 用法：
//
//	logger.Error("下单失败", FieldsFromError(err))
func FieldsFromError(err error) FieldGroup {
	code, _ := errx.CodeOf(err)
	kind := errx.KindOf(err)

	fs := []Field{
		String("err.code", string(code)),
		String("err.kind", kind.String()),
		Bool("err.retryable", errx.Retryable(err)),
	}
	if desc := errx.Describe(code); desc != "" {
		fs = append(fs, String("err.code_desc", desc))
	}
	if e, ok := errx.As(err); ok {
		if msg := e.Message(); msg != "" {
			fs = append(fs, String("err.message", msg))
		}
		for _, kv := range e.Fields() {
			fs = append(fs, Any(kv.Key, kv.Value))
		}
	}
	return Fields(fs...)
}
