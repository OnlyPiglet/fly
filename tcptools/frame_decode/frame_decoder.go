package frame_decode

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
)

var (
	ErrInvalidFrameDecode = errors.New("invalud_frame_decode")
	ErrInvalidFrame       = errors.New("invalid frame format")
	ErrChecksumMismatch   = errors.New("checksum mismatch")
	ErrBufferOverflow     = errors.New("buffer overflow")
)

const (
	defaultMaxFrameSize = 64 * 1024
	defaultReadChunk    = 4096
)

const (
	TLVType = iota
	HeadTailType
)

type FrameConfig struct {
	StartBytes             []byte
	EndBytes               []byte
	ByteOrder              binary.ByteOrder
	FrameLengthOffset      int
	FrameLengthSize        int
	FrameTotalLengthAdjust int
	ChecksumIndex          int
	ChecksumSize           int
	MaxFrameSize           int
}

type FrameDecoder interface {
	GetConfig() FrameConfig
	ValidateChecksum(frame Frame) bool
	FrameType() int
}

type Frame struct {
	RawData  []byte
	Length   []byte
	Body     []byte
	Checksum []byte
}

type FrameHandler func(*Frame) error

func DecodeFrames(conn *net.TCPConn, dec FrameDecoder, bufSize int, handler FrameHandler) error {
	switch dec.FrameType() {
	case TLVType:
		return tlvDecodeFrame(conn, dec, bufSize, handler)
	case HeadTailType:
		return headTailDecodeFrame(conn, dec, bufSize, handler)
	default:
		return ErrInvalidFrameDecode
	}
}

func tlvDecodeFrame(conn *net.TCPConn, dec FrameDecoder, bufSize int, handler FrameHandler) error {
	if conn == nil || dec == nil {
		return errors.New("conn and decoder must be non-nil")
	}
	if bufSize <= 0 {
		bufSize = defaultReadChunk
	}

	cfg := dec.GetConfig()
	maxSize := cfg.MaxFrameSize
	if maxSize <= 0 {
		maxSize = defaultMaxFrameSize
	}

	buf := make([]byte, 0, bufSize)
	tmp := make([]byte, bufSize)

	for {
		n, err := conn.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}

		for {
			frame, consumed, perr := parseBuffer(buf, dec)
			if frame != nil {
				buf = buf[consumed:]
				if !dec.ValidateChecksum(*frame) {
					continue
				}
				if handler != nil {
					f := frame
					if herr := handler(f); herr != nil {
						fmt.Printf("[frame_decode] handler error: %v\n", herr)
					}
				}
				continue
			}
			if perr != nil {
				if errors.Is(perr, ErrInvalidFrame) || errors.Is(perr, ErrChecksumMismatch) {
					if consumed > 0 {
						buf = buf[consumed:]
						continue
					}
				} else {
					return perr
				}
			}
			break
		}

		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("read error: %w", err)
		}

		if len(buf) > maxSize*4 {
			return ErrBufferOverflow
		}
	}
}

func headTailDecodeFrame(conn *net.TCPConn, dec FrameDecoder, bufSize int, handler FrameHandler) error {
	if conn == nil || dec == nil {
		return errors.New("conn and decoder must be non-nil")
	}
	if bufSize <= 0 {
		bufSize = defaultReadChunk
	}

	cfg := dec.GetConfig()
	maxSize := cfg.MaxFrameSize
	if maxSize <= 0 {
		maxSize = defaultMaxFrameSize
	}

	buf := make([]byte, 0, bufSize)
	tmp := make([]byte, bufSize)

	for {
		n, err := conn.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}

		for {
			frame, consumed, perr := parseHeadTailBuffer(buf, dec)
			if frame != nil {
				buf = buf[consumed:]
				if !dec.ValidateChecksum(*frame) {
					continue
				}
				if handler != nil {
					f := frame
					if herr := handler(f); herr != nil {
						fmt.Printf("[frame_decode] handler error: %v\n", herr)
					}
				}
				continue
			}
			if perr != nil {
				if errors.Is(perr, ErrInvalidFrame) || errors.Is(perr, ErrChecksumMismatch) {
					if consumed > 0 {
						buf = buf[consumed:]
						continue
					}
				} else {
					return perr
				}
			}
			break
		}

		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("read error: %w", err)
		}

		if len(buf) > maxSize*4 {
			return ErrBufferOverflow
		}
	}
}

// ----------------- 内部函数 --------------------

func parseHeadTailBuffer(buf []byte, dec FrameDecoder) (*Frame, int, error) {
	cfg := dec.GetConfig()

	startLen := len(cfg.StartBytes)
	endLen := len(cfg.EndBytes)
	if startLen == 0 && endLen == 0 {
		return nil, 0, ErrInvalidFrame
	}

	// 情况 1：只有 EndBytes，根据 EndBytes 切帧
	if startLen == 0 && endLen > 0 {
		endIdx := bytes.Index(buf, cfg.EndBytes)
		if endIdx == -1 {
			// 半包，继续读
			return nil, 0, nil
		}
		frameEnd := endIdx + endLen
		frameBytes := buf[:frameEnd]
		bodyEnd := endIdx
		if bodyEnd < 0 {
			return nil, 1, ErrInvalidFrame
		}

		return &Frame{
			RawData:  append([]byte(nil), frameBytes...),
			Length:   nil,
			Body:     append([]byte(nil), frameBytes[:bodyEnd]...),
			Checksum: nil,
		}, frameEnd, nil
	}

	// 情况 2：只有 StartBytes，根据连续 StartBytes 切帧
	if startLen > 0 && endLen == 0 {
		startIdx := bytes.Index(buf, cfg.StartBytes)
		if startIdx == -1 {
			// 丢弃到倒数 startBytes 长度之前的数据
			return nil, len(buf) - startLen + 1, ErrInvalidFrame
		}
		if startIdx > 0 {
			// 丢弃起始标记之前的无效数据
			return nil, startIdx, ErrInvalidFrame
		}

		searchStart := startIdx + startLen
		nextRel := bytes.Index(buf[searchStart:], cfg.StartBytes)
		if nextRel == -1 {
			// 可能是半包，继续读
			return nil, 0, nil
		}
		nextStart := searchStart + nextRel
		frameBytes := buf[startIdx:nextStart]
		bodyStart := startIdx + startLen
		bodyEnd := nextStart
		if bodyStart > bodyEnd {
			return nil, 1, ErrInvalidFrame
		}

		return &Frame{
			RawData:  append([]byte(nil), frameBytes...),
			Length:   nil,
			Body:     append([]byte(nil), frameBytes[bodyStart-startIdx:bodyEnd-startIdx]...),
			Checksum: nil,
		}, nextStart, nil
	}

	// 情况 3：StartBytes 和 EndBytes 都存在
	startIdx := bytes.Index(buf, cfg.StartBytes)
	if startIdx == -1 {
		// 丢弃到倒数 startBytes 长度之前的数据
		return nil, len(buf) - startLen + 1, ErrInvalidFrame
	}
	if startIdx > 0 {
		// 丢弃起始标记之前的无效数据
		return nil, startIdx, ErrInvalidFrame
	}

	// 在起始标记之后查找结束标记
	searchStart := startIdx + startLen
	endRel := bytes.Index(buf[searchStart:], cfg.EndBytes)
	if endRel == -1 {
		// 可能是半包，继续读
		return nil, 0, nil
	}
	endIdx := searchStart + endRel + endLen

	frameBytes := buf[startIdx:endIdx]
	bodyStart := startIdx + startLen
	bodyEnd := endIdx - endLen
	if bodyStart > bodyEnd {
		return nil, 1, ErrInvalidFrame
	}

	return &Frame{
		RawData:  append([]byte(nil), frameBytes...),
		Length:   nil,
		Body:     append([]byte(nil), frameBytes[bodyStart-startIdx:bodyEnd-startIdx]...),
		Checksum: nil,
	}, endIdx, nil
}

func parseBuffer(buf []byte, dec FrameDecoder) (*Frame, int, error) {
	cfg := dec.GetConfig()

	startIdx := bytes.Index(buf, cfg.StartBytes)
	if startIdx == -1 {
		return nil, len(buf) - len(cfg.StartBytes) + 1, ErrInvalidFrame
	}
	if startIdx > 0 {
		return nil, startIdx, ErrInvalidFrame
	}

	lengthEnd := cfg.FrameLengthOffset + cfg.FrameLengthSize
	if len(buf) < lengthEnd {
		return nil, 0, nil
	}

	frameLen := readLength(buf[cfg.FrameLengthOffset:lengthEnd], cfg)
	totalLen := frameLen + cfg.FrameTotalLengthAdjust

	if totalLen < lengthEnd+cfg.ChecksumSize {
		return nil, 1, ErrInvalidFrame
	}
	if len(buf) < totalLen {
		return nil, 0, nil
	}

	frame := buf[:totalLen]
	if cfg.EndBytes != nil && !bytes.HasSuffix(frame, cfg.EndBytes) {
		return nil, totalLen, ErrInvalidFrame
	}

	bodyStart := lengthEnd
	bodyEnd := totalLen - cfg.ChecksumSize
	if cfg.EndBytes != nil {
		bodyEnd -= len(cfg.EndBytes)
	}

	// 计算checksum的实际起始位置
	checksumStart := cfg.ChecksumIndex
	if checksumStart < 0 {
		checksumStart = len(frame) + cfg.ChecksumIndex
	}
	checksumEnd := checksumStart + cfg.ChecksumSize

	return &Frame{
		RawData:  append([]byte(nil), frame...),
		Length:   append([]byte(nil), frame[cfg.FrameLengthOffset:cfg.FrameLengthOffset+cfg.FrameLengthSize]...),
		Body:     append([]byte(nil), frame[bodyStart:bodyEnd]...),
		Checksum: append([]byte(nil), frame[checksumStart:checksumEnd]...),
	}, totalLen, nil
}

func readLength(data []byte, cfg FrameConfig) int {
	if len(data) < cfg.FrameLengthSize {
		return 0
	}
	switch cfg.FrameLengthSize {
	case 1:
		return int(data[0])
	case 2:
		return int(cfg.ByteOrder.Uint16(data))
	case 4:
		return int(cfg.ByteOrder.Uint32(data))
	default:
		return 0
	}
}
