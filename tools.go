package fly

import (
	"reflect"
	"unsafe"
)

/*
not a atomic op
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

/*
*
去除切片中重复的元素
*/
func UniqueSlice[T comparable](in []T) []T {

	if in == nil || len(in) == 0 {
		return []T{}
	}

	o := make(map[T]bool, 0)
	out := make([]T, 0)

	for _, n := range in {
		if !o[n] {
			o[n] = true
			out = append(out, n)
		}
		continue
	}

	return out
}
