package disktools

import (
	"strconv"
	"strings"
)

const (
	_   = iota
	KiB = 1 << (10 * iota)
	MiB
	GiB
	TiB
)

// FormatDiskSizeBytes 将字节转换为mib/kib/gib/tib , 入参大小格式为 int64
func FormatDiskSizeBytes(bytes int64) string {
	output := float64(bytes)
	unit := ""

	switch {
	case bytes >= TiB:
		output = output / TiB
		unit = "Ti"
	case bytes >= GiB:
		output = output / GiB
		unit = "Gi"
	case bytes >= MiB:
		output = output / MiB
		unit = "Mi"
	case bytes >= KiB:
		output = output / KiB
		unit = "Ki"
	case bytes == 0:
		return "0"
	}

	result := strconv.FormatFloat(output, 'f', 0, 64)
	result = strings.TrimSuffix(result, ".0")
	return result + unit
}

func BytesToKiB(bytes int64) int64 {
	output := float64(bytes)
	output = output / KiB
	return int64(output)
}

func BytesToGiB(bytes int64) int64 {
	output := float64(bytes)
	output = output / GiB
	return int64(output)
}

func BytesToMiB(bytes int64) int64 {
	output := float64(bytes)
	output = output / MiB
	return int64(output)
}

func BytesToTiB(bytes int64) int64 {
	output := float64(bytes)
	output = output / TiB
	return int64(output)
}

// 磁盘分区
