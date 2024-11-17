package ziptools

import (
	"compress/gzip"
	"io"
)

func CompressGzip(reader io.Reader, dest io.Writer) error {
	all, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	writer, err := gzip.NewWriterLevel(dest, 9)
	if err != nil {
		return err
	}
	defer writer.Close()
	_, err = writer.Write(all)
	if err != nil {
		return err
	}
	return writer.Flush()
}

func UncompressGzip(reader io.Reader, dest io.Writer) error {
	readData, err := gzip.NewReader(reader)
	if err != nil {
		return err
	}
	all, err := io.ReadAll(readData)
	if err != nil {
		return err
	}
	_, err = dest.Write(all)
	return err
}
