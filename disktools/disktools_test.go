package disktools

import "testing"

func TestFormatBytes(t *testing.T) {
	t.Run("test kb", func(t *testing.T) {
		println(FormatDiskSizeBytes(1024))
	})
	t.Run("test mb", func(t *testing.T) {
		println(FormatDiskSizeBytes(1024 * 1024))
	})
	t.Run("FormatDiskSizeBytes gb", func(t *testing.T) {
		println(FormatDiskSizeBytes(12*1024*1024*1024 + 11*1024 + 1024))
	})
	t.Run("test tb", func(t *testing.T) {
		println(FormatDiskSizeBytes(1024 * 1024 * 1024 * 1024))
	})
}
