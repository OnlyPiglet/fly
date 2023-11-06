package fly

import (
	"reflect"
	"unsafe"
)

func IfChanClosed[T any](ch chan T) (closed bool) {
	select {
	case _, ok := <-ch:
		if !ok {
			closed = true
		}
	default:
		closed = false
	}
	return closed

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

func InDstNotInSrc[T comparable](src, dst []T) []T {
	m := make(map[T]bool)

	for _, item := range src {
		m[item] = true
	}

	diff := make([]T, 0)

	for _, item := range dst {
		if _, ok := m[item]; !ok {
			diff = append(diff, item)
		}
	}

	return diff
}
