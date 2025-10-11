package frame_decode

import (
	"encoding/binary"
	"fmt"
	"net"
	"testing"
)

// 起始2字节 长度2字节【body+校验+结束】 +body N字节 + 校验 2字节【长度+bvk】+ 结束 2字节

type BolaiFrameDecoder struct {
}

func (d *BolaiFrameDecoder) GetConfig() FrameConfig {
	return FrameConfig{
		StartBytes:             []byte{0xfc, 0xfe},
		EndBytes:               []byte{0xfc, 0xee},
		ByteOrder:              binary.BigEndian,
		FrameLengthOffset:      2,
		FrameLengthSize:        2,
		FrameTotalLengthAdjust: 4, // 起始2字节 + 长度2字节
		ChecksumSize:           2,
	}
}

func (d *BolaiFrameDecoder) ValidateChecksum(frame []byte) bool {
	return true
}

func HandlerFrame(frame *Frame) error {
	fmt.Printf("%x\n", frame.RawData)
	return nil
}

func TestFrameDecode(t *testing.T) {
	tcpc, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP("0.0.0.0"), Port: 2222})
	if err != nil {
		t.Fatal(err)
	}
	for {
		cc, _ := tcpc.Accept()
		err := DecodeFrames(cc.(*net.TCPConn), new(BolaiFrameDecoder), 1024, HandlerFrame)
		if err != nil {
			t.Fatal(err)
		}
	}
}
