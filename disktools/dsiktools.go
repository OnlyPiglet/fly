package disktools

import (
	"strconv"
	"strings"
)

const (
	_   = iota
	kiB = 1 << (10 * iota)
	miB
	giB
	tiB
)

// FormatDiskSizeBytes 将字节转换为mib/kib/gib/tib , 入参大小格式为 int64
func FormatDiskSizeBytes(bytes int64) string {
	output := float64(bytes)
	unit := ""

	switch {
	case bytes >= tiB:
		output = output / tiB
		unit = "Ti"
	case bytes >= giB:
		output = output / giB
		unit = "Gi"
	case bytes >= miB:
		output = output / miB
		unit = "Mi"
	case bytes >= kiB:
		output = output / kiB
		unit = "Ki"
	case bytes == 0:
		return "0"
	}

	result := strconv.FormatFloat(output, 'f', 0, 64)
	result = strings.TrimSuffix(result, ".0")
	return result + unit
}
