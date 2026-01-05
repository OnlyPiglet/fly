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

	// 验证第一个连接已注册
	if srv.ActiveConn() != 1 {
		t.Fatalf("expected 1 active conn, got %d", srv.ActiveConn())
	}

	// 第二个连接：TCP 层面会成功（操作系统 backlog 队列），
	// 但服务端检测到超过 maxConn 后会关闭连接
	c2, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer c2.Close()

	// 等待服务端处理并关闭超限连接
	time.Sleep(50 * time.Millisecond)

	// 验证活跃连接数仍然是 1（第二个被拒绝）
	if srv.ActiveConn() != 1 {
		t.Fatalf("expected 1 active conn after reject, got %d", srv.ActiveConn())
	}

	// 验证第二个连接已被服务端关闭：尝试读取应该返回 EOF 或错误
	c2.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	buf := make([]byte, 1)
	_, err = c2.Read(buf)
	if err == nil {
		t.Fatal("expected read to fail on rejected connection")
	} else {
		println(err.Error())
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

func TestGetConn(t *testing.T) {
	srv, addr := startTestServer(t)
	defer srv.Stop()

	c := dial(t, addr)
	defer c.Close()

	time.Sleep(50 * time.Millisecond)

	var id string
	srv.mu.Lock()
	for k := range srv.conns {
		id = k
		break
	}
	srv.mu.Unlock()

	conn, ok := srv.GetConn(id)
	if !ok {
		t.Fatal("expected to find connection")
	}
	if conn.ID != id {
		t.Fatalf("expected conn ID %s, got %s", id, conn.ID)
	}

	_, ok = srv.GetConn("non-existent")
	if ok {
		t.Fatal("expected not to find non-existent connection")
	}
}

func TestFindByTag(t *testing.T) {
	srv, addr := startTestServer(t)
	defer srv.Stop()

	c1 := dial(t, addr)
	c2 := dial(t, addr)
	defer c1.Close()
	defer c2.Close()

	time.Sleep(50 * time.Millisecond)

	// Get connections and set tags
	var conns []*ConnContext
	srv.mu.Lock()
	for _, c := range srv.conns {
		conns = append(conns, c)
	}
	srv.mu.Unlock()

	if len(conns) < 2 {
		t.Fatal("expected at least 2 connections")
	}

	conns[0].SetTag("tenant", "tenant1")
	conns[1].SetTag("tenant", "tenant2")

	// Test FindByTag
	found := srv.FindByTag("tenant", "tenant1")
	if len(found) != 1 {
		t.Fatalf("expected 1 connection with tenant1, got %d", len(found))
	}
	if found[0].ID != conns[0].ID {
		t.Fatal("found wrong connection")
	}

	found = srv.FindByTag("tenant", "tenant2")
	if len(found) != 1 {
		t.Fatalf("expected 1 connection with tenant2, got %d", len(found))
	}

	found = srv.FindByTag("tenant", "non-existent")
	if len(found) != 0 {
		t.Fatalf("expected 0 connections, got %d", len(found))
	}
}

func TestFilterByTag(t *testing.T) {
	srv, addr := startTestServer(t)
	defer srv.Stop()

	c1 := dial(t, addr)
	c2 := dial(t, addr)
	c3 := dial(t, addr)
	defer c1.Close()
	defer c2.Close()
	defer c3.Close()

	time.Sleep(50 * time.Millisecond)

	// Get connections and set tags
	var conns []*ConnContext
	srv.mu.Lock()
	for _, c := range srv.conns {
		conns = append(conns, c)
	}
	srv.mu.Unlock()

	if len(conns) < 3 {
		t.Fatal("expected at least 3 connections")
	}

	conns[0].SetTag("region", "us-east")
	conns[1].SetTag("region", "us-east")
	conns[2].SetTag("region", "us-west")

	// Test FilterByTag
	filtered := srv.FilterByTag("region")
	if len(filtered) != 2 {
		t.Fatalf("expected 2 regions, got %d", len(filtered))
	}

	if len(filtered["us-east"]) != 2 {
		t.Fatalf("expected 2 connections in us-east, got %d", len(filtered["us-east"]))
	}

	if len(filtered["us-west"]) != 1 {
		t.Fatalf("expected 1 connection in us-west, got %d", len(filtered["us-west"]))
	}

	// Test non-existent tag
	filtered = srv.FilterByTag("non-existent")
	if len(filtered) != 0 {
		t.Fatalf("expected 0 groups, got %d", len(filtered))
	}
}

func TestCountByTag(t *testing.T) {
	srv, addr := startTestServer(t)
	defer srv.Stop()

	c1 := dial(t, addr)
	c2 := dial(t, addr)
	c3 := dial(t, addr)
	defer c1.Close()
	defer c2.Close()
	defer c3.Close()

	time.Sleep(50 * time.Millisecond)

	// Get connections and set tags
	var conns []*ConnContext
	srv.mu.Lock()
	for _, c := range srv.conns {
		conns = append(conns, c)
	}
	srv.mu.Unlock()

	if len(conns) < 3 {
		t.Fatal("expected at least 3 connections")
	}

	conns[0].SetTag("env", "prod")
	conns[1].SetTag("env", "prod")
	conns[2].SetTag("env", "dev")

	// Test CountByTag
	if srv.CountByTag("env", "prod") != 2 {
		t.Fatal("expected 2 prod connections")
	}

	if srv.CountByTag("env", "dev") != 1 {
		t.Fatal("expected 1 dev connection")
	}

	if srv.CountByTag("env", "staging") != 0 {
		t.Fatal("expected 0 staging connections")
	}
}

func TestQueryAPIWithKick(t *testing.T) {
	h := &testHandler{}
	srv, addr := startTestServer(t, WithHandler(h))
	defer srv.Stop()

	c1 := dial(t, addr)
	c2 := dial(t, addr)
	defer c1.Close()
	defer c2.Close()

	time.Sleep(50 * time.Millisecond)

	// Set tags
	var conns []*ConnContext
	srv.mu.Lock()
	for _, c := range srv.conns {
		conns = append(conns, c)
	}
	srv.mu.Unlock()

	if len(conns) < 2 {
		t.Fatal("expected at least 2 connections")
	}

	conns[0].SetTag("tenant", "tenant1")
	conns[1].SetTag("tenant", "tenant1")

	// Use query API to find connections, then kick them manually
	found := srv.FindByTag("tenant", "tenant1")
	if len(found) != 2 {
		t.Fatalf("expected 2 connections, got %d", len(found))
	}

	// Kick connections manually using the query result
	for _, conn := range found {
		srv.KickByID(conn.ID, context.Canceled)
	}

	time.Sleep(50 * time.Millisecond)

	if srv.ActiveConn() != 0 {
		t.Fatal("expected all connections to be kicked")
	}

	if h.disconnectCnt.Load() != 2 {
		t.Fatalf("expected 2 disconnects, got %d", h.disconnectCnt.Load())
	}
}
