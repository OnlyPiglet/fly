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
	if str == "" {
		return []byte{}
	}
	sh := &reflect.SliceHeader{
		Data: (*reflect.StringHeader)(unsafe.Pointer(&str)).Data,
		Len:  len(str),
		Cap:  len(str),
	}
	return *(*[]byte)(unsafe.Pointer(sh))
}

func QuickBytesToString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}
