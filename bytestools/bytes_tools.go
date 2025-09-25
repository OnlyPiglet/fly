package bytestools

// CalculateChecksum 计算字节数组的累加和校验值（十进制）
// 参数: data - 要计算的字节数组
// 返回: 十进制校验和值 (0-255)
func CalculateChecksum(data []byte) int {
	sum := 0
	for _, b := range data {
		sum += int(b)
	}
	return sum & 0xFF
}
