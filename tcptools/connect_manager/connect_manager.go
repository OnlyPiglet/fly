package connect_manager

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

/**

		┌─────────────────────────────┐
		│        Server (Control)     │
		│ ┌────────────┐  ┌────────┐ │
		│ │ ConnTable  │  │ Reaper │ │
		│ └────────────┘  └────────┘ │
		│ ┌────────────┐  ┌────────┐ │
		│ │  Metrics   │  │ Limiter│ │
		│ └────────────┘  └────────┘ │
		└─────────▲──────────────────┘
				  │
				  │ ConnContext
				  ▼
		┌─────────────────────────────┐
		│     Business Handler        │
		│   (Read / Write / Protocol)│
		└─────────────────────────────┘
**/

var (
	ErrMaxConnReached = errors.New("max connection reached")
	ErrIdleTimeout    = errors.New("idle timeout")
	ErrServerClosed   = errors.New("server closed")
)

// ======================
// Conn State
// ======================

type ConnState int32

const (
	StateActive ConnState = iota
	StateIdle             // 预留：如果你未来要严格区分 idle/active，可以用它
	StateKicked
	StateClosed
)

// ======================
// Conn Context
// ======================

type ConnContext struct {
	ID       string
	Conn     net.Conn
	CreateAt time.Time

	lastActive atomic.Int64 // unix nano
	state      atomic.Int32

	closeOnce sync.Once
}

// Touch：业务方在读写/处理到数据时调用，用于刷新活跃时间（Server 不接管 IO，所以需要业务协作）
func (c *ConnContext) Touch() {
	now := time.Now().UnixNano()
	c.lastActive.Store(now)
	c.state.Store(int32(StateActive))
}

// LastActiveTime：用于 idleReaper 判断是否超时
func (c *ConnContext) LastActiveTime() time.Time {
	n := c.lastActive.Load()
	if n == 0 {
		return c.CreateAt
	}
	return time.Unix(0, n)
}

func (c *ConnContext) State() ConnState {
	return ConnState(c.state.Load())
}

// ======================
// Metrics
// ======================

type Metrics struct {
	active atomic.Int64
	idle   atomic.Int64 // 预留：如果未来你想严格维护 idle/active，这里可用
	kicked atomic.Int64
}

func (m *Metrics) Active() int64 { return m.active.Load() }
func (m *Metrics) Idle() int64   { return m.idle.Load() }
func (m *Metrics) Kicked() int64 { return m.kicked.Load() }

// ======================
// Handler（可选）
// ======================

type Handler interface {
	// OnAccept: 返回 error 则拒绝该连接（例如鉴权失败/黑名单/限流等）
	OnAccept(ctx *ConnContext) error
	// OnDisconnect: 连接被关闭/被踢/超时等回调
	OnDisconnect(ctx *ConnContext, reason error)
}

// ======================
// Server
// ======================

type Server struct {
	addr        string
	maxConn     int
	idleTimeout time.Duration
	reapTick    time.Duration

	handler Handler

	mu    sync.Mutex
	conns map[string]*ConnContext

	metrics Metrics

	ln     net.Listener
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	idSeq atomic.Uint64
}

// ======================
// Options
// ======================

type Option func(*Server)

func WithMaxConn(n int) Option {
	return func(s *Server) { s.maxConn = n }
}

func WithIdleTimeout(d time.Duration) Option {
	return func(s *Server) { s.idleTimeout = d }
}

// idle 扫描间隔（默认 1min）；如果你希望更快踢 idle，可调小，比如 10s
func WithReapTick(d time.Duration) Option {
	return func(s *Server) { s.reapTick = d }
}

func WithHandler(h Handler) Option {
	return func(s *Server) { s.handler = h }
}

// ======================
// New
// ======================

func New(addr string, opts ...Option) *Server {
	ctx, cancel := context.WithCancel(context.Background())

	s := &Server{
		addr:        addr,
		maxConn:     10000,
		idleTimeout: 3 * time.Minute,
		reapTick:    1 * time.Minute,
		conns:       make(map[string]*ConnContext),
		ctx:         ctx,
		cancel:      cancel,
	}

	for _, opt := range opts {
		opt(s)
	}
	return s
}

// ======================
// Start / Stop
// ======================

func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	s.ln = ln

	// acceptLoop / idleReaper 都是 server 自己的 goroutine
	s.wg.Add(2)
	go s.acceptLoop()
	go s.idleReaper()

	return nil
}

func (s *Server) Stop() {
	// 1) 广播退出
	s.cancel()

	// 2) 关闭 listener，打断 Accept
	if s.ln != nil {
		_ = s.ln.Close()
	}

	// 3) 关闭所有连接（并发安全：注意 closeConn 内部会删 map）
	s.mu.Lock()
	for _, c := range s.conns {
		s.closeConnLocked(c, ErrServerClosed) // 注意：这里用 Locked 版本避免重复锁
	}
	s.mu.Unlock()

	// 4) 等待 goroutine 退出
	s.wg.Wait()
}

// ======================
// acceptLoop
// ======================

func (s *Server) acceptLoop() {
	defer s.wg.Done()

	for {
		conn, err := s.ln.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return
			default:
				// listener 抖动/临时错误：继续
				continue
			}
		}

		// 超限：直接拒绝（不进入 conns map）
		if !s.allowAccept() {
			_ = conn.Close()
			continue
		}

		c := &ConnContext{
			ID:       s.generateID(conn),
			Conn:     conn,
			CreateAt: time.Now(),
		}
		c.Touch() // 初始化 lastActive

		// 可选：业务层拒绝连接（鉴权/黑名单等）
		if s.handler != nil {
			if err := s.handler.OnAccept(c); err != nil {
				_ = conn.Close()
				continue
			}
		}

		s.register(c)
	}
}

// ======================
// allowAccept
// ======================

func (s *Server) allowAccept() bool {
	if s.maxConn <= 0 {
		return true
	}
	// 这里是“近似限制”，高并发下可能短暂超出 1~N 个，换来更高吞吐；工具库通常接受这种权衡
	return int(s.metrics.active.Load()) < s.maxConn
}

// ======================
// register
// ======================

func (s *Server) register(c *ConnContext) {
	s.mu.Lock()
	s.conns[c.ID] = c
	s.mu.Unlock()

	s.metrics.active.Add(1)
}

// ======================
// idleReaper
// ======================

func (s *Server) idleReaper() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.reapTick)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.reapIdle()
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *Server) reapIdle() {
	if s.idleTimeout <= 0 {
		return
	}

	now := time.Now()

	// 注意：range map 的时候必须持锁
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, c := range s.conns {
		if now.Sub(c.LastActiveTime()) > s.idleTimeout {
			s.kickConnLocked(c, ErrIdleTimeout) // locked 版本避免死锁
		}
	}
}

/**

        idle
		 ↓
	  kickConn
		 ↘
stop → closeOnce → cleanup → CLOSED
		 ↗
	  closeConn
		 ↑
	 read error


*/
/*

	1️⃣ 现实中「关闭触发是并发的」,所以通过 closeOnce 进行收敛

	举几个真实的并发触发场景（这些在生产里一定会发生）：

	场景 A：idle + 业务同时触发
	idleReaper goroutine      业务 goroutine
		   |                        |
		   | idle timeout           | conn.Read() 出错
		   | kickConn()             | closeConn()
		   |                        |
		   +---------- race --------+

	场景 B：Stop + KickByID
	Server.Stop()        管控接口
	   |                    |
	   | kickAll()          | KickByID()
	   |                    |
	   +-------- race ------+

	场景 C：Stop + idleReaper tick
	idleReaper           Stop()
		 |                 |
		 | tick            | cancel()
		 | kickConn()      | closeConn()
		 +------- race ----+


	👉 这些触发路径是“独立 goroutine 并发发生”的
*/

// ======================
// closeConn / kickConn
// 说明：提供 public 入口 + 内部 Locked 版本，避免“持锁再调用导致重复锁/死锁”
// ======================

// CloseByID：业务主动关闭
func (s *Server) CloseByID(id string, reason error) {
	s.mu.Lock()
	c := s.conns[id]
	s.mu.Unlock()

	if c != nil {
		s.closeConn(c, reason)
	}
}

// KickByID：业务主动踢人
func (s *Server) KickByID(id string, reason error) {
	s.mu.Lock()
	c := s.conns[id]
	s.mu.Unlock()

	if c != nil {
		s.kickConn(c, reason)
	}
}

// KickAll：踢全部（比如灰度回滚/发布重启）
func (s *Server) KickAll(reason error) {
	s.mu.Lock()
	for _, c := range s.conns {
		s.kickConnLocked(c, reason)
	}
	s.mu.Unlock()
}

// closeConn：不要求调用方持锁
func (s *Server) closeConn(c *ConnContext, reason error) {
	c.closeOnce.Do(func() {
		c.state.Store(int32(StateClosed))

		// 先关 conn，尽快释放 fd
		_ = c.Conn.Close()

		// 删除 map（必须持锁）
		s.mu.Lock()
		// 如果已经被其他路径删了，delete 是安全的
		delete(s.conns, c.ID)
		s.mu.Unlock()

		s.metrics.active.Add(-1)

		if s.handler != nil {
			s.handler.OnDisconnect(c, reason)
		}
	})
}

// kickConn：不要求调用方持锁
func (s *Server) kickConn(c *ConnContext, reason error) {
	c.closeOnce.Do(func() {
		c.state.Store(int32(StateKicked))

		_ = c.Conn.Close()

		s.mu.Lock()
		delete(s.conns, c.ID)
		s.mu.Unlock()

		s.metrics.active.Add(-1)
		s.metrics.kicked.Add(1)

		if s.handler != nil {
			s.handler.OnDisconnect(c, reason)
		}
	})
}

// closeConnLocked：调用方必须已持有 s.mu
func (s *Server) closeConnLocked(c *ConnContext, reason error) {
	c.closeOnce.Do(func() {
		c.state.Store(int32(StateClosed))

		_ = c.Conn.Close()

		delete(s.conns, c.ID)

		s.metrics.active.Add(-1)

		if s.handler != nil {
			s.handler.OnDisconnect(c, reason)
		}
	})
}

// kickConnLocked：调用方必须已持有 s.mu
func (s *Server) kickConnLocked(c *ConnContext, reason error) {
	c.closeOnce.Do(func() {
		c.state.Store(int32(StateKicked))

		_ = c.Conn.Close()

		delete(s.conns, c.ID)

		s.metrics.active.Add(-1)
		s.metrics.kicked.Add(1)

		if s.handler != nil {
			s.handler.OnDisconnect(c, reason)
		}
	})
}

// ======================
// Metrics API
// ======================

func (s *Server) Metrics() Metrics {
	// atomic 字段可直接拷贝使用（读 Load 即可）
	return s.metrics
}

func (s *Server) ActiveConn() int64 { return s.metrics.Active() }
func (s *Server) IdleConn() int64   { return s.metrics.Idle() } // 预留
func (s *Server) KickedConn() int64 { return s.metrics.Kicked() }

// ======================
// Helpers
// ======================

func (s *Server) generateID(conn net.Conn) string {
	seq := s.idSeq.Add(1)
	// remote + seq + nano：足够唯一且可读
	return fmt.Sprintf("%s-%d-%d", conn.RemoteAddr().String(), seq, time.Now().UnixNano())
}
