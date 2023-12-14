package ctxtools

import (
	"context"
	"reflect"
	"strings"
	"unsafe"
)

//thanks for this document https://golang.design/go-questions/interface/iface-eface/
// https://www.cnblogs.com/liujh2010/p/how-to-get-all-keys-and-values-of-context.html
// https://www.cnblogs.com/jiujuan/p/17142703.html

type iface struct {
	itab, data uintptr
}

type valueCtx struct {
	context.Context
	key, val any
}

func GetKeys(ctx context.Context) []any {
	keys := make([]any, 0)
	maps := GetKeyValues(ctx)
	for k, _ := range maps {
		keys = append(keys, k)
	}
	return keys
}

func GetKeyValues(ctx context.Context) map[any]any {
	m := make(map[any]any)
	if reflect.ValueOf(ctx).Kind().String() != "ptr" {
		return m
	}
	getKeyValue(ctx, m)
	return m
}

func getKeyValue(ctx context.Context, m map[any]any) {
	ictx := *(*iface)(unsafe.Pointer(&ctx))
	if ictx.data == 0 {
		return
	}
	valCtx := (*valueCtx)(unsafe.Pointer(ictx.data))
	if valCtx != nil && valCtx.key != nil && valCtx.val != nil {
		m[valCtx.key] = valCtx.val
	}
	if strings.ToLower(reflect.ValueOf(valCtx.Context).Kind().String()) != "ptr" {
		return
	}
	getKeyValue(valCtx.Context, m)
}
