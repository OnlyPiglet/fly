package connect_manager

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type testHandler struct {
	acceptCnt     atomic.Int64
	disconnectCnt atomic.Int64
	lastReason    atomic.Value
}

func (h *testHandler) OnAccept(c *ConnContext) error {
	h.acceptCnt.Add(1)
	return nil
}

func (h *testHandler) OnDisconnect(c *ConnContext, reason error) {
	h.disconnectCnt.Add(1)
	h.lastReason.Store(reason)
}

func startTestServer(t *testing.T, opts ...Option) (*Server, string) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	srv := New(addr, opts...)
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	return srv, addr
}

func dial(t *testing.T, addr string) net.Conn {
	t.Helper()
	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestServerStartStop(t *testing.T) {
	srv, _ := startTestServer(t)
	srv.Stop()
}

func TestAcceptAndRegister(t *testing.T) {
	h := &testHandler{}
	srv, addr := startTestServer(t, WithHandler(h))
	defer srv.Stop()

	c := dial(t, addr)
	defer c.Close()

	time.Sleep(50 * time.Millisecond)

	if srv.ActiveConn() != 1 {
		t.Fatalf("expected 1 active conn, got %d", srv.ActiveConn())
	}

	if h.acceptCnt.Load() != 1 {
		t.Fatalf("expected OnAccept=1, got %d", h.acceptCnt.Load())
	}
}

func TestMaxConn(t *testing.T) {
	srv, addr := startTestServer(t, WithMaxConn(1))
	defer srv.Stop()

	c1 := dial(t, addr)
	defer c1.Close()

	time.Sleep(50 * time.Millisecond)

	c2, err := net.Dial("tcp", addr)
	if err == nil {
		_ = c2.Close()
		t.Fatal("expected second connection to be rejected")
	}
}

func TestTouchAndLastActive(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	ctx := &ConnContext{
		Conn:     c1,
		CreateAt: time.Now(),
	}
	ctx.Touch()

	t1 := ctx.LastActiveTime()
	time.Sleep(10 * time.Millisecond)
	ctx.Touch()
	t2 := ctx.LastActiveTime()

	if !t2.After(t1) {
		t.Fatal("LastActiveTime not updated")
	}
}

func TestIdleReaperKick(t *testing.T) {
	h := &testHandler{}

	srv, addr := startTestServer(
		t,
		WithHandler(h),
		WithIdleTimeout(50*time.Millisecond),
		WithReapTick(20*time.Millisecond),
	)
	defer srv.Stop()

	c := dial(t, addr)
	defer c.Close()

	time.Sleep(200 * time.Millisecond)

	if srv.ActiveConn() != 0 {
		t.Fatalf("expected active=0 after idle kick, got %d", srv.ActiveConn())
	}

	if h.disconnectCnt.Load() != 1 {
		t.Fatalf("expected disconnect=1, got %d", h.disconnectCnt.Load())
	}
}

func TestKickByID(t *testing.T) {
	h := &testHandler{}
	srv, addr := startTestServer(t, WithHandler(h))
	defer srv.Stop()

	c := dial(t, addr)
	defer c.Close()

	time.Sleep(50 * time.Millisecond)

	var id string
	srv.mu.Lock()
	for k := range srv.conns {
		id = k
	}
	srv.mu.Unlock()

	srv.KickByID(id, context.Canceled)
	time.Sleep(50 * time.Millisecond)

	if srv.ActiveConn() != 0 {
		t.Fatal("expected active=0 after kick")
	}
}
func TestKickAll(t *testing.T) {
	srv, addr := startTestServer(t)
	defer srv.Stop()

	c1 := dial(t, addr)
	c2 := dial(t, addr)
	defer c1.Close()
	defer c2.Close()

	time.Sleep(50 * time.Millisecond)

	srv.KickAll(context.Canceled)
	time.Sleep(50 * time.Millisecond)

	if srv.ActiveConn() != 0 {
		t.Fatal("expected all conns kicked")
	}
}

func TestCloseOnceConcurrent(t *testing.T) {
	h := &testHandler{}
	srv, addr := startTestServer(t, WithHandler(h))
	defer srv.Stop()

	c := dial(t, addr)
	defer c.Close()

	time.Sleep(50 * time.Millisecond)

	var ctx *ConnContext
	srv.mu.Lock()
	for _, v := range srv.conns {
		ctx = v
	}
	srv.mu.Unlock()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			srv.KickByID(ctx.ID, context.Canceled)
		}()
	}

	wg.Wait()
	time.Sleep(50 * time.Millisecond)

	if h.disconnectCnt.Load() != 1 {
		t.Fatalf("expected disconnect once, got %d", h.disconnectCnt.Load())
	}
}

func TestStopClosesAll(t *testing.T) {
	srv, addr := startTestServer(t)

	c1 := dial(t, addr)
	c2 := dial(t, addr)
	defer c1.Close()
	defer c2.Close()

	time.Sleep(50 * time.Millisecond)

	srv.Stop()

	if srv.ActiveConn() != 0 {
		t.Fatal("expected no active conn after Stop")
	}
}
