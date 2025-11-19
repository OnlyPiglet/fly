package frame_decode

import (
	"encoding/binary"
	"fmt"
	"net"
	"testing"
)

type NoHeadOnlyTailDecoder struct {
}

func (d *NoHeadOnlyTailDecoder) FrameType() int {
	return HeadTailType
}

func (d *NoHeadOnlyTailDecoder) GetConfig() FrameConfig {
	return FrameConfig{
		StartBytes:             []byte{},
		EndBytes:               []byte{0x0d, 0x0a},
		ByteOrder:              binary.BigEndian,
		FrameLengthOffset:      0, //长度的index
		FrameLengthSize:        0, // 长度的字节长度
		FrameTotalLengthAdjust: 0, // 起始2字节 + 长度2字节
		ChecksumIndex:          0, // checksum在frame中的起始位置，需要根据实际协议设置
		ChecksumSize:           0,
	}
}

func (d *NoHeadOnlyTailDecoder) ValidateChecksum(frame Frame) bool {
	return true
}

func HandlerNoHeadOnlyTailDecoder(frame *Frame) error {
	fmt.Printf("%x\n", frame.RawData)
	return nil
}

func Test_NoHeadOnlyTailDecoder(t *testing.T) {
	tcpc, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP("0.0.0.0"), Port: 2222})
	if err != nil {
		t.Fatal(err)
	}
	for {
		cc, _ := tcpc.Accept()
		err := DecodeFrames(cc.(*net.TCPConn), new(NoHeadOnlyTailDecoder), 1024, HandlerNoHeadOnlyTailDecoder)
		if err != nil {
			t.Fatal(err)
		}
	}
}

// 起始2字节 长度2字节【body+校验+结束】 +body N字节 + 校验 1字节【长度+bvk】+ 结束 2字节

type TestoneFrameDecoder struct {
}

func (d *TestoneFrameDecoder) GetConfig() FrameConfig {
	return FrameConfig{
		StartBytes:             []byte{0xfc, 0xfe},
		EndBytes:               []byte{0xfc, 0xee},
		ByteOrder:              binary.BigEndian,
		FrameLengthOffset:      2,  //长度的index
		FrameLengthSize:        2,  // 长度的字节长度
		FrameTotalLengthAdjust: 4,  // 起始2字节 + 长度2字节
		ChecksumIndex:          -3, // checksum在frame中的起始位置，需要根据实际协议设置
		ChecksumSize:           1,
	}
}

func (d *TestoneFrameDecoder) ValidateChecksum(frame Frame) bool {
	return true
}

func HandlerFrame(frame *Frame) error {
	fmt.Printf("%x\n", frame.RawData)
	return nil
}

func (d *TestoneFrameDecoder) FrameType() int {
	return TLVType
}

func TestOneFrameDecode(t *testing.T) {
	tcpc, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP("0.0.0.0"), Port: 2222})
	if err != nil {
		t.Fatal(err)
	}
	for {
		cc, _ := tcpc.Accept()
		err := DecodeFrames(cc.(*net.TCPConn), new(TestoneFrameDecoder), 1024, HandlerFrame)
		if err != nil {
			t.Fatal(err)
		}
	}
}

// **   起始标志 数据长度(序列号域 + 加密标志 +  帧类型标志 + 消息体) 序列号域 加密标志 帧类型标志 消息体 帧校验域
// **    1字节    1字节                                         2字节   1字节    1字节    N字节  2字节
type TestTwoFrameDecoder struct {
}

func (d *TestTwoFrameDecoder) FrameType() int {
	return TLVType
}

func (d *TestTwoFrameDecoder) GetConfig() FrameConfig {
	return FrameConfig{
		StartBytes:             []byte{0x68},
		EndBytes:               []byte{},
		ByteOrder:              binary.BigEndian,
		FrameLengthOffset:      1,  //长度的index
		FrameLengthSize:        1,  // 长度的字节长度
		FrameTotalLengthAdjust: 4,  // 起始2字节 + 长度2字节
		ChecksumIndex:          -2, // checksum在frame中的起始位置，需要根据实际协议设置
		ChecksumSize:           2,
	}
}

func (d *TestTwoFrameDecoder) ValidateChecksum(frame Frame) bool {
	return true
}

func TestTwoFrameDecode(t *testing.T) {
	tcpc, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP("0.0.0.0"), Port: 2223})
	if err != nil {
		t.Fatal(err)
	}
	for {
		cc, _ := tcpc.Accept()
		err := DecodeFrames(cc.(*net.TCPConn), new(TestTwoFrameDecoder), 1024, HandlerFrame)
		if err != nil {
			t.Fatal(err)
		}
	}
}

type LXFrameDecoder struct {
}

func (d *LXFrameDecoder) GetConfig() FrameConfig {
	return FrameConfig{
		StartBytes:             []byte{0x70},
		EndBytes:               []byte{0x20},
		ByteOrder:              binary.LittleEndian,
		FrameLengthOffset:      1,  //长度的index
		FrameLengthSize:        2,  // 长度的字节长度
		FrameTotalLengthAdjust: 8,  // 起始2字节 + 长度2字节
		ChecksumIndex:          -2, // checksum在frame中的起始位置，需要根据实际协议设置
		ChecksumSize:           1,
	}
}

func (d *LXFrameDecoder) FrameType() int {
	return TLVType
}

func (d *LXFrameDecoder) ValidateChecksum(frame Frame) bool {
	return true
}

func TestLXFrameDecoder(t *testing.T) {
	tcpc, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP("0.0.0.0"), Port: 2223})
	if err != nil {
		t.Fatal(err)
	}
	for {
		cc, _ := tcpc.Accept()
		err := DecodeFrames(cc.(*net.TCPConn), new(LXFrameDecoder), 1024, HandlerFrame)
		if err != nil {
			t.Fatal(err)
		}
	}
}
