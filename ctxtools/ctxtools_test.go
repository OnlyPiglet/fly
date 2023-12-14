package ctxtools

import (
	"context"
	"testing"
)

func TestGetKeys(t *testing.T) {
	rawctx := context.Background()
	ctx := context.WithValue(rawctx, "a", 20)
	cctx := context.WithValue(ctx, "b", "os")
	keys := GetKeys(cctx)
	for _, key := range keys {
		println(key)
	}

}

func TestGetKeysValues(t *testing.T) {
	t.Run("test string", func(t *testing.T) {
		rawctx := context.Background()
		ctx := context.WithValue(rawctx, "a", "v")
		values := GetKeyValues(ctx)
		for k, v := range values {
			println(k.(string))
			println(v.(string))
		}
	})
	t.Run("test int", func(t *testing.T) {
		rawctx := context.Background()
		ctx := context.WithValue(rawctx, "a", 20)
		cctx := context.WithValue(ctx, "b", 30)
		values := GetKeyValues(cctx)
		for k, v := range values {
			println(k.(string))
			println(v.(int))
		}
	})
	t.Run("test multi string", func(t *testing.T) {
		rawctx := context.Background()
		ctx := context.WithValue(rawctx, "a", "v")
		cctx := context.WithValue(ctx, "b", "b")
		values := GetKeyValues(cctx)
		for k, v := range values {
			println(k.(string))
			println(v.(string))
		}
	})

}
