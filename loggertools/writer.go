package logger

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

type FileWriter struct {
	f       *os.File
	rotator Rotator
	path    string
	wSize   *atomic.Int64
	bw      *bufio.Writer
	flushAt time.Time
}

func (f *FileWriter) open() {
	f.f, _ = os.OpenFile(f.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	f.bw = bufio.NewWriter(f.f)
}

func (f *FileWriter) loadSize() {
	fs, _ := f.f.Stat()
	f.wSize.Store(fs.Size())
}

func (f *FileWriter) Close() error {
	_ = f.bw.Flush()
	return f.f.Close()
}

func (f *FileWriter) flush() {
	if time.Now().Add(-time.Second).After(f.flushAt) && f.bw.Buffered() > 0 {
		_ = f.bw.Flush()
		f.flushAt = time.Now()
	}
}

func (f *FileWriter) Write(p []byte) (int, error) {
	n, err := f.bw.Write(p)
	f.wSize.Add(int64(n))
	if f.wSize.Load() >= f.rotator.MaxSize() {
		_ = f.bw.Flush()
		_ = f.f.Close()
		f.rotator.Rotate(f.path)
		f.open()
		f.wSize.Store(0)
	}
	f.flush()
	return n, err
}

func NewFileWriter(path string, rotator Rotator) io.WriteCloser {
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	fw := &FileWriter{path: path, rotator: rotator, wSize: new(atomic.Int64), flushAt: time.Now()}
	fw.open()
	fw.loadSize()
	return fw
}

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
