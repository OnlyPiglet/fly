package connect_manager

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/OnlyPiglet/fly/logtools"
	"github.com/OnlyPiglet/fly/tcptools"
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
	logger  *logtools.Log

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

// 外部注入 logger（推荐）
func WithLogger(l *logtools.Log) Option {
	return func(s *Server) { s.logger = l }
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
		logger:      logtools.NewXLog(), // fallback
	}

	for _, opt := range opts {
		opt(s)
	}

	s.logger.AddAttrs("component", "connect_manager", "addr", addr)
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

	s.logger.Info("server started",
		"maxConn", s.maxConn,
		"idleTimeout", s.idleTimeout.String(),
		"reapTick", s.reapTick.String(),
	)

	s.wg.Add(2)
	go s.acceptLoop()
	go s.idleReaper()

	return nil
}

func (s *Server) Stop() {
	s.logger.Info("server stopping")

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
	s.logger.Info("server stopped")
}

// ======================
// acceptLoop
// ======================

func (s *Server) acceptLoop() {
	defer s.wg.Done()

	for {
		// 先检查 context 是否已取消，避免在已关闭的 listener 上调用 Accept
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		conn, err := s.ln.Accept()
		if err != nil {
			// 如果 context 已取消，说明是正常关闭，直接返回
			select {
			case <-s.ctx.Done():
				return
			default:
				// 如果 listener 被关闭（非 context 取消），也应该退出
				// 因为此时 Accept 会返回 "use of closed network connection" 错误
				// 继续循环会不断重试，应该退出
				return
			}
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
				s.logger.Warn("connection rejected by handler",
					"connID", c.ID,
					"err", err,
				)
				if tc, ok := conn.(*net.TCPConn); ok {
					tcptools.ResetTcpConn(tc)
				}
				continue
			}
		}

		if !s.register(c) {
			s.logger.Warn("reject connection: maxConn reached",
				"maxConn", s.maxConn,
				"active", s.metrics.active.Load(),
			)
			if tc, ok := conn.(*net.TCPConn); ok {
				tcptools.ResetTcpConn(tc)
			}
			continue
		}
	}
}

// ======================
// register
// ======================

// register 注册连接，返回是否成功注册
// 如果超过最大连接数限制，会原子性地回滚计数并返回 false
func (s *Server) register(c *ConnContext) bool {
	// 先原子性地增加计数
	newCount := s.metrics.active.Add(1)

	// 检查是否超过限制
	if s.maxConn > 0 && int(newCount) > s.maxConn {
		// 超过限制，回滚计数
		s.metrics.active.Add(-1)
		return false
	}

	// 在限制内，加入到 map
	s.mu.Lock()
	s.conns[c.ID] = c
	s.mu.Unlock()

	s.logger.Debug("connection accepted",
		"connID", c.ID,
		"active", s.metrics.active.Load(),
	)
	return true
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
		idle := now.Sub(c.LastActiveTime())
		if idle > s.idleTimeout {
			s.logger.Info("idle kick",
				"connID", c.ID,
				"idle", idle.String(),
			)
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

	s.logger.Info("kick all connections", "reason", reason)
}

// ======================
// Query API
// ======================

// GetConn 根据ID获取连接上下文
func (s *Server) GetConn(id string) (*ConnContext, bool) {
	s.mu.Lock()
	c, ok := s.conns[id]
	s.mu.Unlock()
	return c, ok
}

// FindByTag 根据标签查找连接，返回匹配的连接列表
func (s *Server) FindByTag(key, value string) []*ConnContext {
	var result []*ConnContext
	s.mu.Lock()
	for _, c := range s.conns {
		if v, ok := c.GetTag(key); ok && v == value {
			result = append(result, c)
		}
	}
	s.mu.Unlock()
	return result
}

// FilterByTag 根据标签键查找所有匹配的连接，按标签值分组返回
func (s *Server) FilterByTag(key string) map[string][]*ConnContext {
	result := make(map[string][]*ConnContext)
	s.mu.Lock()
	for _, c := range s.conns {
		if v, ok := c.GetTag(key); ok {
			result[v] = append(result[v], c)
		}
	}
	s.mu.Unlock()
	return result
}

// CountByTag 统计指定标签的连接数量（便捷方法）
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
		s.safeOnDisconnect(c, reason)
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

		s.safeOnDisconnect(c, reason)
	})
}

func (s *Server) closeConnLocked(c *ConnContext, reason error) {
	c.closeOnce.Do(func() {
		c.state.Store(int32(StateClosed))
		_ = c.Conn.Close()
		delete(s.conns, c.ID)
		s.metrics.active.Add(-1)
		s.safeOnDisconnect(c, reason)
	})
}

func (s *Server) kickConnLocked(c *ConnContext, reason error) {
	c.closeOnce.Do(func() {
		c.state.Store(int32(StateKicked))
		_ = c.Conn.Close()
		delete(s.conns, c.ID)
		s.metrics.active.Add(-1)
		s.metrics.kicked.Add(1)
		s.safeOnDisconnect(c, reason)
	})
}

// ======================
// handler safety
// ======================

func (s *Server) safeOnDisconnect(c *ConnContext, reason error) {
	if s.handler == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("panic in handler.OnDisconnect",
				"connID", c.ID,
				"panic", r,
			)
		}
	}()
	s.handler.OnDisconnect(c, reason)
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
