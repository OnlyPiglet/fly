package bytestools

import "encoding/hex"

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

func BcdBytesToNumberHexString(bcd []byte) string {
	// 打印BCD字节数组为十六进制字符串
	return BcdBytesToNumberString(bcd)
	//result := ""
	//for _, b := range bcd {
	//	r, ok := bcdToStringMap[b]
	//	if ok {
	//		result += r
	//	}
	//}
	//return result
}

// BcdBytesToNumberString 字节数组 [0x12,0x34,0x56,0x78] -> "12345678"
func BcdBytesToNumberString(bcd []byte) string {
	// 打印BCD字节数组为十六进制字符串
	return hex.EncodeToString(bcd)
	//result := ""
	//for _, b := range bcd {
	//	r, ok := bcdToStringMap[b]
	//	if ok {
	//		result += r
	//	}
	//}
	//return result
}

// NumberStringToBcdByte "12345678" 字符串 转为字节数组 [0x12,0x34,0x56,0x78]
func NumberStringToBcdByte(s string) []byte {
	// 若长度为奇数，则在前面补0
	if len(s)%2 != 0 {
		s = "0" + s
	}

	// 转成字节数组，每两个数字一组
	bcd := make([]byte, len(s)/2)
	for i := 0; i < len(s); i += 2 {
		hi := s[i]
		lo := s[i+1]

		if hi < '0' || hi > '9' || lo < '0' || lo > '9' {
			return nil
		}

		// 合成一个 BCD 字节
		bcd[i/2] = ((hi - '0') << 4) | (lo - '0')
	}
	return bcd

}

// BytesToASCIIString 将字节切片转换为 ASCII 字符串
func BytesToASCIIString(data []byte) string {
	if len(data) > 0 && data[len(data)-1] == 0x00 {
		return string(data[:len(data)-1]) // 移除最后一个字节
	}
	return string(data) // 无需移除
}

// ASCIIStringToBytes 将 ASCII 字符串转换为字节切片，并可选是否添加末尾的 \0
func ASCIIStringToBytes(s string) []byte {
	return aSCIIStringToBytes(s, false)
}

// aSCIIStringToBytes 将 ASCII 字符串转换为字节切片，并可选是否添加末尾的 \0
func aSCIIStringToBytes(s string, addNullTerminator bool) []byte {
	bytes := []byte(s)
	if addNullTerminator {
		return append(bytes, 0x00) // 在末尾添加 \0
	}
	return bytes // 直接返回字节切片
}
