package logx

import (
	"testing"

	"github.com/lcylpzls/errx"
)

// fieldsMap 将字段组转为键值映射，便于断言。
func fieldsMap(g FieldGroup) map[string]Field {
	m := make(map[string]Field, g.Len())
	for i := 0; i < g.Len(); i++ {
		f := g.At(i)
		m[f.Key] = f
	}
	return m
}

func TestFieldsFromError(t *testing.T) {
	err := errx.New(errx.KindBusiness, errx.Code("demo_failed"), "下单失败").WithField("order_id", "123")
	g := FieldsFromError(err)
	m := fieldsMap(g)

	if got := m["err.code"].str; got != "demo_failed" {
		t.Fatalf("err.code = %q，期望 demo_failed", got)
	}
	if got := m["err.kind"].str; got != errx.KindBusiness.String() {
		t.Fatalf("err.kind = %q，期望 %q", got, errx.KindBusiness.String())
	}
	if m["err.retryable"].b {
		t.Fatal("err.retryable 应为 false")
	}
	if got := m["err.message"].str; got != "下单失败" {
		t.Fatalf("err.message = %q，期望 下单失败", got)
	}
	if got := m["order_id"].Value; got != "123" {
		t.Fatalf("order_id = %v，期望 123", got)
	}
}

func TestFieldsFromErrorNil(t *testing.T) {
	g := FieldsFromError(nil)
	m := fieldsMap(g)
	if got := m["err.kind"].str; got != errx.KindOf(nil).String() {
		t.Fatalf("nil 错误 err.kind = %q，期望 %q", got, errx.KindOf(nil).String())
	}
}
