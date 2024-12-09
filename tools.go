package fly

import (
	"reflect"
	"unsafe"
)

/*
IfChanClosed please be carefully if want to use to judge if chan closed
*/
func IfChanClosed[T any](ch chan T) (closed bool) {
	return (*(**struct {
		_, _   uint
		_      unsafe.Pointer
		_      uint16
		closed uint32
	})(unsafe.Pointer(&ch))).closed != 0
}

func QuickStringToBytes(str string) []byte {
	return s2b(str)
}

func QuickBytesToString(b []byte) string {
	return b2s(b)
}

// b2s thanks for fasthttp s2b_new.go
func b2s(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// s2b thanks for fasthttp b2s_new.go
func s2b(s string) (b []byte) {
	bh := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
	bh.Data = sh.Data
	bh.Cap = sh.Len
	bh.Len = sh.Len
	return b
}
