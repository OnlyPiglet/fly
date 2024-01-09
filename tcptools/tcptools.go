package tcptools

const (
	TCP_ESTABLISHED = iota + 0x01
	TCP_SYN_SENT
	TCP_SYN_RECV
	TCP_FIN_WAIT1
	TCP_FIN_WAIT2
	TCP_TIME_WAIT
	TCP_CLOSE
	TCP_CLOSE_WAIT
	TCP_LAST_ACK
	TCP_LISTEN
	TCP_CLOSING
	TCP_NEW_SYN_REC
	TCP_MAX_STATES
)

var tcpStateMap = map[uint8]string{
	1:  "ESTABLISHED",
	2:  "SYN_SENT",
	3:  "SYN_RECV",
	4:  "FIN_WAIT1",
	5:  "FIN_WAIT2",
	6:  "IME_WAIT",
	7:  "CLOSE",
	8:  "CLOSE_WAIT",
	9:  "LAST_ACK",
	10: "LISTEN",
	11: "CLOSING",
	12: "NEW_SYN_REC",
	13: "MAX_STATES",
}

func TcpState2String(state uint8) string {
	return tcpStateMap[state]
}

func Listen(state uint8) bool {
	return state == TCP_LISTEN
}
