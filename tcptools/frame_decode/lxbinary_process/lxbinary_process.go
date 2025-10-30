package lxbinary_process

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"math"

	"github.com/OnlyPiglet/fly/tcptools/frame_decode"
)

// -------------------- 对外：给 DecodeFrames 的回调 --------------------

func HandleLXFrame(f *frame_decode.Frame) error {
	raw := f.RawData
	if len(raw) < 1+2+2+1+1+6+1+1 { // 70 + L + L + 70 + C + A6 + AFN + 末尾20
		return errors.New("lx: frame too short")
	}
	// 帧头检查
	if raw[0] != 0x70 || raw[5] != 0x70 || raw[len(raw)-1] != 0x20 {
		return errors.New("lx: header/tail mismatch")
	}
	// 两个 L（小端）一致性 & 总长 = L + 8
	L1 := int(binary.LittleEndian.Uint16(raw[1:3]))
	L2 := int(binary.LittleEndian.Uint16(raw[3:5]))
	if L1 != L2 {
		return errors.New("lx: L1 != L2")
	}
	if len(raw) != L1+8 {
		return fmt.Errorf("lx: length mismatch want=%d got=%d", L1+8, len(raw))
	}
	// CS 校验：对 [C..CS 前] 求和（不进位）
	cIndex := len(raw) - 2 // 倒数第2字节
	bodyStart := 6         // 第二个 0x70 的下一个字节，就是 C 的位置
	var sum byte
	for i := bodyStart; i < cIndex; i++ {
		sum += raw[i]
	}
	if sum != raw[cIndex] {
		return errors.New("lx: checksum mismatch")
	}

	// -------- 解析头部 --------
	C := raw[6]
	ctrl := CtrlField{
		Raw: C, DIR: int((C >> 7) & 0x01), PRM: int((C >> 6) & 0x01), VN: int(C & 0x3F),
	}
	mac := parseMACBCD(raw[7 : 7+6])

	// -------- 应用数据域 --------
	app := raw[13:cIndex] // 从 AFN 开始到 CS 前
	if len(app) < 1 {
		return errors.New("lx: empty app data")
	}
	afn := app[0]

	switch afn {
	case 0x02: // 登录
		log.Printf("[LX] LOGIN mac=%s vn=%d", mac, ctrl.VN)
		return onLogin(ctrl, mac)
	case 0x03: // 心跳
		log.Printf("[LX] HEARTBEAT mac=%s", mac)
		return onHeartbeat(ctrl, mac)
	case 0x0B: // 实时数据：AFN + Pn + Fn + DATA + TIME(6)
		if len(app) < 1+2+6 {
			return errors.New("lx: realtime too short")
		}
		pn, fn := int(app[1]), int(app[2])
		if len(app) < 1+2+6 { // 再次保护
			return errors.New("lx: realtime missing time")
		}
		timeStart := len(app) - 6
		data := app[3:timeStart]
		tm := parseTimeBCD(app[timeStart:])
		return dispatchRealtime(ctrl, mac, pn, fn, data, tm)
	case 0x0C: // 历史（结构与实时类似，可按需实现）
		return onHistory(ctrl, mac, app[1:])
	case 0x0E: // 事件
		return onEvent(ctrl, mac, app[1:])
	case 0x0F: // 采集器状态
		return onCollectorStatus(ctrl, mac, app[1:])
	default:
		log.Printf("[LX] AFN=0x%02X (len=%d) not handled", afn, len(app))
		return nil
	}
}

// -------------------- 头部/常用解析工具 --------------------

type CtrlField struct {
	Raw          byte
	DIR, PRM, VN int
}

func parseMACBCD(b []byte) string {
	out := make([]byte, 0, 12)
	for _, x := range b {
		hi, lo := (x>>4)&0x0F, x&0x0F
		out = append(out, hexNib(hi), hexNib(lo))
	}
	return string(out)
}
func hexNib(n byte) byte {
	if n < 10 {
		return '0' + n
	}
	return 'A' + (n - 10)
}

type LXTime struct{ Y, Mo, D, H, Mi, S int }

func parseTimeBCD(b []byte) LXTime {
	if len(b) < 6 {
		return LXTime{}
	}
	d := func(x byte) int { return int(x>>4)*10 + int(x&0x0F) }
	return LXTime{Y: 2000 + d(b[0]), Mo: d(b[1]), D: d(b[2]), H: d(b[3]), Mi: d(b[4]), S: d(b[5])}
}

// -------------------- 业务回调（按需落库/回写） --------------------

func onLogin(c CtrlField, mac string) error                     { return nil }
func onHeartbeat(c CtrlField, mac string) error                 { return nil }
func onHistory(c CtrlField, mac string, p []byte) error         { return nil }
func onEvent(c CtrlField, mac string, p []byte) error           { return nil }
func onCollectorStatus(c CtrlField, mac string, p []byte) error { return nil }

// -------------------- 实时数据分发（AFN=0x0B） --------------------

func dispatchRealtime(c CtrlField, mac string, pn, fn int, data []byte, t LXTime) error {
	switch fn {
	case 0x01:
		v, err := parseF1Inverter(data, c.VN)
		if err != nil {
			return err
		}
		log.Printf("[RT][F1] mac=%s pn=%d t=%04d-%02d-%02d %02d:%02d:%02d MPPT=%d Pdc=%dW PF=%.2f Eff=%.2f%%",
			mac, pn, t.Y, t.Mo, t.D, t.H, t.Mi, t.S, v.MPPT, v.Pdc, v.PF, v.Eff*100)
	case 0x03:
		m, err := parseF3Meter(data)
		if err != nil {
			return err
		}
		log.Printf("[RT][F3] mac=%s pn=%d Uabc=(%.1f/%.1f/%.1f)V Iabc=(%.2f/%.2f/%.2f)A PF=%.3f",
			mac, pn, m.UA, m.UB, m.UC, m.IA, m.IB, m.IC, m.PF)
	case 0x04:
		e, err := parseF4Env(data, c.VN)
		if err != nil {
			return err
		}
		log.Printf("[RT][F4] mac=%s pn=%d Wind=%.1fm/s Temp=%.1f℃", mac, pn, e.WindSpeed, e.AmbTemp)
	default:
		log.Printf("[RT] mac=%s Pn=%d Fn=0x%02X len=%d (skip)", mac, pn, fn, len(data))
	}
	return nil
}

// -------------------- 示例：F1/F3/F4 解析 --------------------

// F1 逆变器（示范：核心字段；更多字段可按表追加）
type F1Inverter struct {
	MPPT   uint8
	DCVolt []float64 // /10
	DCCurr []float64 // /10
	Pdc    uint32    // W
	PF     float64   // /100
	Eff    float64   // /100
}

func parseF1Inverter(b []byte, vn int) (F1Inverter, error) {
	var o F1Inverter
	if len(b) < 1 {
		return o, errors.New("F1 too short")
	}
	o.MPPT = b[0]
	off := 1
	rdI16 := func() (int16, error) {
		if off+2 > len(b) {
			return 0, errors.New("F1 oob")
		}
		v := int16(binary.LittleEndian.Uint16(b[off:]))
		off += 2
		return v, nil
	}
	for i := 0; i < int(o.MPPT); i++ {
		v, e := rdI16()
		if e != nil {
			return o, e
		}
		o.DCVolt = append(o.DCVolt, float64(v)/10)
	}
	for i := 0; i < int(o.MPPT); i++ {
		v, e := rdI16()
		if e != nil {
			return o, e
		}
		o.DCCurr = append(o.DCCurr, float64(v)/10)
	}
	if off+4 > len(b) {
		return o, errors.New("F1 no Pdc")
	}
	o.Pdc = binary.LittleEndian.Uint32(b[off:])
	off += 4
	// ……中间很多字段，这里略去，仅演示 PF/Eff 的解法：
	if off+2 > len(b) {
		return o, nil
	} // 容错
	off += 2 // 跳过机内温度
	if off+2 > len(b) {
		return o, nil
	}
	off += 2 // 跳过电网频率
	if off+2 > len(b) {
		return o, nil
	}
	off += 2 // 跳过绝缘
	if off+2 > len(b) {
		return o, nil
	}
	pf := int16(binary.LittleEndian.Uint16(b[off:]))
	off += 2
	o.PF = float64(pf) / 100.0
	if off+2 > len(b) {
		return o, nil
	}
	ef := int16(binary.LittleEndian.Uint16(b[off:]))
	o.Eff = float64(ef) / 100.0
	return o, nil
}

// F3 三相电表（大量 float32，小端）
type F3Meter struct {
	UA, UB, UC float32
	IA, IB, IC float32
	PF         float32
}

func parseF3Meter(b []byte) (F3Meter, error) {
	var m F3Meter
	rdF := func(off int) (float32, error) {
		if off+4 > len(b) {
			return 0, errors.New("F3 oob")
		}
		return math.Float32frombits(binary.LittleEndian.Uint32(b[off:])), nil
	}
	var e error
	if m.UA, e = rdF(0); e != nil {
		return m, e
	}
	if m.UB, e = rdF(4); e != nil {
		return m, e
	}
	if m.UC, e = rdF(8); e != nil {
		return m, e
	}
	if m.IA, e = rdF(12); e != nil {
		return m, e
	}
	if m.IB, e = rdF(16); e != nil {
		return m, e
	}
	if m.IC, e = rdF(20); e != nil {
		return m, e
	}
	if m.PF, e = rdF(4 * 16); e != nil { /* 若表结构差异，可调整偏移 */
	}
	return m, nil
}

// F4 环境仪（示范）
type F4Env struct {
	WindSpeed float64 // m/s
	AmbTemp   float64 // ℃
}

func parseF4Env(b []byte, vn int) (F4Env, error) {
	if len(b) < 6 {
		return F4Env{}, errors.New("F4 too short")
	}
	ws := int16(binary.LittleEndian.Uint16(b[0:]))
	ta := int16(binary.LittleEndian.Uint16(b[4:]))
	return F4Env{WindSpeed: float64(ws) / 10.0, AmbTemp: float64(ta) / 10.0}, nil
}
