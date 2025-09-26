package bytestools

// ExtractBits 获取 bit 位的内容
func ExtractBits(b byte, index, length uint) int {
	// 处理边界情况
	if index >= 8 || length == 0 {
		return 0
	}

	// 如果超出范围，则调整长度
	if index+length > 8 {
		length = 8 - index
	}

	// 将目标位移到最右侧
	shifted := b >> index

	// 创建掩码以截取指定长度
	mask := byte(1<<length) - 1

	// 返回整数结果
	return int(shifted & mask)
}
