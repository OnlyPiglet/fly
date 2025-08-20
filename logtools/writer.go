package logtools

import (
	"bytes"
	"io"
	"net"
	"sync"
	"time"
)

type NetworkWriter struct {
	conn       net.Conn
	network    string
	addr       string
	err        error
	maxBufSize int
	buf        *bytes.Buffer
	l          sync.RWMutex
}

func (n *NetworkWriter) dial() {
	n.l.Lock()
	defer n.l.Unlock()
	if n.err == nil && n.conn != nil {
		return
	}
	n.conn, n.err = net.DialTimeout(n.network, n.addr, 5*time.Second)
}

func (n *NetworkWriter) bufWrite(p []byte) (int, error) {
	if n.buf.Len() >= n.maxBufSize {
		_ = n.buf.Next(len(p))
	}
	return n.buf.Write(p)
}

func (n *NetworkWriter) Write(p []byte) (int, error) {
	var rn int
	if n.err != nil || n.conn == nil {
		go n.dial()
		return n.bufWrite(p)
	}
	if n.buf.Len() > 0 {
		_, _ = n.conn.Write(n.buf.Bytes())
		n.buf.Reset()
	}
	rn, n.err = n.conn.Write(p)
	if n.err != nil {
		_, _ = n.bufWrite(p)
	}
	return rn, n.err
}

func (n *NetworkWriter) Close() error {
	if n.err == nil {
		n.err = n.conn.Close()
	}
	return n.err
}

func NewNetworkWriter(network, addr string, maxBufSize int) io.WriteCloser {
	w := &NetworkWriter{
		network:    network,
		addr:       addr,
		maxBufSize: maxBufSize,
		buf:        new(bytes.Buffer),
	}
	w.dial()
	return w
}
