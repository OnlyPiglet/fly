package fly

import (
	"fmt"
	"testing"
)

func TestIsChanClosed(t *testing.T) {

	ch := make(chan struct{})
	close(ch)

	t.Run("close chan", func(t *testing.T) {
		if !IfChanClosed(ch) {
			t.Errorf("should be true a closed channel")
		}
	})

	t.Run("open and close chan", func(t *testing.T) {
		ch := make(chan int, 1)
		fmt.Println(IfChanClosed(ch)) // 输出: false
		if IfChanClosed(ch) {
			t.Errorf("should be false a open channel")
		}
		close(ch)
		if !IfChanClosed(ch) {
			t.Errorf("should be true a closed channel")
		}
	})

}

func TestStringToBytes(t *testing.T) {
	t.Run("chstr2bytes", func(t *testing.T) {
		//https://www.toolhelper.cn/EncodeDecode/EncodeDecode
		str := "你好"
		bytes := QuickStringToBytes(str)
		for _, b := range bytes {
			println(b)
		}
	})
	t.Run("enstr2bytes", func(t *testing.T) {
		str := "hello,world"
		bytes := QuickStringToBytes(str)
		for _, b := range bytes {
			println(b)
		}
	})

	t.Run("bytes2chstr", func(t *testing.T) {
		b := []byte{0xe4, 0xbd, 0xa0}
		toString := QuickBytesToString(b)
		println(toString)
	})

	t.Run("bytes2enstr", func(t *testing.T) {
		b := []byte{104, 101, 108, 108, 111, 44, 119, 111, 114, 108, 100}
		toString := QuickBytesToString(b)
		println(toString)
	})

}

func TestName(t *testing.T) {
	t.Run("dst which not in src", func(t *testing.T) {
		src := []int{1, 2, 3}
		dst := []int{2, 3, 4, 5}
		inDst := InDstNotInSrc(src, dst)
		for _, i := range inDst {
			println(i)
		}
	})
}

func TestUniqueSlice(t *testing.T) {

	a := []int{1, 1, 2, 2, 4}

	slice := UniqueSlice(a)

	for _, v := range slice {
		println(v)
	}

}
