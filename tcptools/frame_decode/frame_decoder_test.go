package frame_decode

import (
	"encoding/binary"
	"fmt"
	"net"
	"testing"
)

// 起始2字节 长度2字节【body+校验+结束】 +body N字节 + 校验 2字节【长度+bvk】+ 结束 2字节

type TestoneFrameDecoder struct {
}

func (d *TestoneFrameDecoder) GetConfig() FrameConfig {
	return FrameConfig{
		StartBytes:             []byte{0xfc, 0xfe},
		EndBytes:               []byte{0xfc, 0xee},
		ByteOrder:              binary.BigEndian,
		FrameLengthOffset:      2, //长度的index
		FrameLengthSize:        2, // 长度的字节长度
		FrameTotalLengthAdjust: 4, // 起始2字节 + 长度2字节
		ChecksumSize:           2,
	}
}

func (d *TestoneFrameDecoder) ValidateChecksum(frame Frame) bool {
	return true
}

func HandlerFrame(frame *Frame) error {
	fmt.Printf("%x\n", frame.RawData)
	return nil
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

func (d *TestTwoFrameDecoder) GetConfig() FrameConfig {
	return FrameConfig{
		StartBytes:             []byte{0x68},
		EndBytes:               []byte{},
		ByteOrder:              binary.BigEndian,
		FrameLengthOffset:      1, //长度的index
		FrameLengthSize:        1, // 长度的字节长度
		FrameTotalLengthAdjust: 4, // 起始2字节 + 长度2字节
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
