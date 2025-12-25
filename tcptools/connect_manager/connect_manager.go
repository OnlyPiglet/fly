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
	StateIdle
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

	lastActive atomic.Int64
	state      atomic.Int32

	// tag / tenant / region
	tags  map[string]string
	tagMu sync.RWMutex

	closeOnce sync.Once
}

// Touch 刷新活跃时间（业务调用）
func (c *ConnContext) Touch() {
	c.lastActive.Store(time.Now().UnixNano())
	c.state.Store(int32(StateActive))
}

// LastActiveTime 返回最近活跃时间
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

// -------- tag API --------

func (c *ConnContext) SetTag(key, value string) {
	c.tagMu.Lock()
	c.tags[key] = value
	c.tagMu.Unlock()
}

func (c *ConnContext) GetTag(key string) (string, bool) {
	c.tagMu.RLock()
	v, ok := c.tags[key]
	c.tagMu.RUnlock()
	return v, ok
}

func (c *ConnContext) AllTags() map[string]string {
	c.tagMu.RLock()
	defer c.tagMu.RUnlock()

	cp := make(map[string]string, len(c.tags))
	for k, v := range c.tags {
		cp[k] = v
	}
	return cp
}

// ======================
// Metrics
// ======================

type Metrics struct {
	active atomic.Int64
	idle   atomic.Int64
	kicked atomic.Int64
}

func (m *Metrics) Active() int64 { return m.active.Load() }
func (m *Metrics) Idle() int64   { return m.idle.Load() }
func (m *Metrics) Kicked() int64 { return m.kicked.Load() }

// ======================
// Handler
// ======================

type Handler interface {
	OnAccept(ctx *ConnContext) error
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

	s.wg.Add(2)
	go s.acceptLoop()
	go s.idleReaper()

	return nil
}

func (s *Server) Stop() {
	s.cancel()
	if s.ln != nil {
		_ = s.ln.Close()
	}

	s.mu.Lock()
	for _, c := range s.conns {
		s.closeConnLocked(c, ErrServerClosed)
	}
	s.mu.Unlock()

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
				continue
			}
		}

		if !s.allowAccept() {
			_ = conn.Close()
			continue
		}

		c := &ConnContext{
			ID:       s.generateID(conn),
			Conn:     conn,
			CreateAt: time.Now(),
			tags:     make(map[string]string),
		}
		c.Touch()

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

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, c := range s.conns {
		if now.Sub(c.LastActiveTime()) > s.idleTimeout {
			s.kickConnLocked(c, ErrIdleTimeout)
		}
	}
}

// ======================
// close / kick
// ======================

func (s *Server) CloseByID(id string, reason error) {
	s.mu.Lock()
	c := s.conns[id]
	s.mu.Unlock()

	if c != nil {
		s.closeConn(c, reason)
	}
}

func (s *Server) KickByID(id string, reason error) {
	s.mu.Lock()
	c := s.conns[id]
	s.mu.Unlock()

	if c != nil {
		s.kickConn(c, reason)
	}
}

func (s *Server) KickAll(reason error) {
	s.mu.Lock()
	for _, c := range s.conns {
		s.kickConnLocked(c, reason)
	}
	s.mu.Unlock()
}

// -------- tag 管理 --------

func (s *Server) KickByTag(key, value string, reason error) int {
	n := 0
	s.mu.Lock()
	for _, c := range s.conns {
		if v, ok := c.GetTag(key); ok && v == value {
			s.kickConnLocked(c, reason)
			n++
		}
	}
	s.mu.Unlock()
	return n
}

func (s *Server) CountByTag(key, value string) int {
	cnt := 0
	s.mu.Lock()
	for _, c := range s.conns {
		if v, ok := c.GetTag(key); ok && v == value {
			cnt++
		}
	}
	s.mu.Unlock()
	return cnt
}

// ======================
// close helpers
// ======================

func (s *Server) closeConn(c *ConnContext, reason error) {
	c.closeOnce.Do(func() {
		c.state.Store(int32(StateClosed))
		_ = c.Conn.Close()

		s.mu.Lock()
		delete(s.conns, c.ID)
		s.mu.Unlock()

		s.metrics.active.Add(-1)
		if s.handler != nil {
			s.handler.OnDisconnect(c, reason)
		}
	})
}

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

func (s *Server) Metrics() Metrics { return s.metrics }
func (s *Server) ActiveConn() int64 {
	return s.metrics.Active()
}
func (s *Server) IdleConn() int64 {
	return s.metrics.Idle()
}
func (s *Server) KickedConn() int64 {
	return s.metrics.Kicked()
}

// ======================
// Helpers
// ======================

func (s *Server) generateID(conn net.Conn) string {
	seq := s.idSeq.Add(1)
	return fmt.Sprintf("%s-%d-%d", conn.RemoteAddr(), seq, time.Now().UnixNano())
}
