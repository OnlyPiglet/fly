package ziptools

import (
	"bytes"
	"testing"
)

func TestCompressGzip(t *testing.T) {
	b := bytes.NewBufferString("asdasdasdas123123")
	bb := bytes.NewBuffer([]byte{})
	err := CompressGzip(b, bb)
	if err != nil {
		panic(err)
	}
	println(bb.String())

	j := bytes.NewBuffer([]byte{})
	err = UncompressGzip(bb, j)
	if err != nil {
		panic(err)
	}
	println(j.String())

}

func TestUncompressGzip(t *testing.T) {

}
