package logger

import (
	"fmt"
	"os"
	"strconv"
)

const (
	SizeMiB = 1 << 20
	SizeGiB = SizeMiB << 10
)

// DefaultFileRotator keep ten history log files, each with maximum size of 200MiB
var DefaultFileRotator = NewFileRotator(200*SizeMiB, 10)

type Rotator interface {
	MaxSize() int64
	Rotate(path string)
}

type FileRotator struct {
	size int64
	keep int64
}

func (f *FileRotator) MaxSize() int64 {
	return f.size
}

func (f *FileRotator) Rotate(path string) {
	width := len(strconv.Itoa(int(f.keep)))
	for i := f.keep; i > 0; i-- {
		src := fmt.Sprintf("%s.%0*d", path, width, i-1)
		dest := fmt.Sprintf("%s.%0*d", path, width, i)
		if i == 1 {
			src = path
		}
		_ = os.Rename(src, dest)
	}
}

func NewFileRotator(size, keep int64) Rotator {
	return &FileRotator{size: size, keep: keep}
}
