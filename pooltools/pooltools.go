package pooltools

import (
	"sync"
	"sync/atomic"
	"time"
)

// PoolStats 池的统计信息
type PoolStats struct {
	PoolName    string    // 池名称
	GetCount    int64     // 总获取次数
	PutCount    int64     // 总归还次数
	NewCount    int64     // 新创建对象次数
	PoolSize    int64     // 当前池内对象数量（估算值，可能因GC而不准确）
	HitRate     float64   // 命中率（从池中获取的比例）
	LastGetTime time.Time // 最后一次获取时间
	LastPutTime time.Time // 最后一次归还时间
}

// Pool 通用池化工具类
type Pool[T any] struct {
	pool     sync.Pool
	new      func() T
	name     string    // 池名称
	
	// 统计信息
	getCount    int64
	putCount    int64
	newCount    int64
	poolSize    int64
	lastGetTime int64 // unix nano
	lastPutTime int64 // unix nano
}

// New 创建一个新的池化工具实例
// name: 池名称，用于区分不同的池
// newFunc: 创建新对象的函数
func New[T any](name string, newFunc func() T) *Pool[T] {
	p := &Pool[T]{
		new:  newFunc,
		name: name,
	}
	
	p.pool = sync.Pool{
		New: func() interface{} {
			atomic.AddInt64(&p.newCount, 1)
			return newFunc()
		},
	}
	
	return p
}

// Get 从池中获取一个对象
func (p *Pool[T]) Get() T {
	atomic.AddInt64(&p.getCount, 1)
	atomic.AddInt64(&p.poolSize, -1) // 从池中取出一个对象
	atomic.StoreInt64(&p.lastGetTime, time.Now().UnixNano())
	return p.pool.Get().(T)
}

// Put 将对象归还到池中
func (p *Pool[T]) Put(obj T) {
	atomic.AddInt64(&p.putCount, 1)
	atomic.AddInt64(&p.poolSize, 1) // 向池中放入一个对象
	atomic.StoreInt64(&p.lastPutTime, time.Now().UnixNano())
	p.pool.Put(obj)
}

// Reset 重置对象并归还到池中（可选的重置函数）
func (p *Pool[T]) Reset(obj T, resetFunc func(T)) {
	if resetFunc != nil {
		resetFunc(obj)
	}
	p.Put(obj)
}

// Stats 获取池的统计信息
func (p *Pool[T]) Stats() PoolStats {
	getCount := atomic.LoadInt64(&p.getCount)
	putCount := atomic.LoadInt64(&p.putCount)
	newCount := atomic.LoadInt64(&p.newCount)
	poolSize := atomic.LoadInt64(&p.poolSize)
	lastGetTime := atomic.LoadInt64(&p.lastGetTime)
	lastPutTime := atomic.LoadInt64(&p.lastPutTime)
	
	var hitRate float64
	if getCount > 0 {
		hitRate = float64(getCount-newCount) / float64(getCount)
		if hitRate < 0 {
			hitRate = 0
		}
	}
	
	return PoolStats{
		PoolName:    p.name,
		GetCount:    getCount,
		PutCount:    putCount,
		NewCount:    newCount,
		PoolSize:    poolSize,
		HitRate:     hitRate,
		LastGetTime: time.Unix(0, lastGetTime),
		LastPutTime: time.Unix(0, lastPutTime),
	}
}

// Name 获取池名称
func (p *Pool[T]) Name() string {
	return p.name
}

// GetCount 获取总的Get调用次数
func (p *Pool[T]) GetCount() int64 {
	return atomic.LoadInt64(&p.getCount)
}

// PutCount 获取总的Put调用次数
func (p *Pool[T]) PutCount() int64 {
	return atomic.LoadInt64(&p.putCount)
}

// NewCount 获取新创建对象的次数
func (p *Pool[T]) NewCount() int64 {
	return atomic.LoadInt64(&p.newCount)
}

// PoolSize 获取当前池内对象数量（估算值）
func (p *Pool[T]) PoolSize() int64 {
	size := atomic.LoadInt64(&p.poolSize)
	if size < 0 {
		return 0
	}
	return size
}

// HitRate 获取命中率（从池中获取对象的比例）
func (p *Pool[T]) HitRate() float64 {
	getCount := atomic.LoadInt64(&p.getCount)
	newCount := atomic.LoadInt64(&p.newCount)
	
	if getCount == 0 {
		return 0
	}
	
	hitRate := float64(getCount-newCount) / float64(getCount)
	if hitRate < 0 {
		return 0
	}
	return hitRate
}

// ResetStats 重置所有统计信息
func (p *Pool[T]) ResetStats() {
	atomic.StoreInt64(&p.getCount, 0)
	atomic.StoreInt64(&p.putCount, 0)
	atomic.StoreInt64(&p.newCount, 0)
	atomic.StoreInt64(&p.poolSize, 0)
	atomic.StoreInt64(&p.lastGetTime, 0)
	atomic.StoreInt64(&p.lastPutTime, 0)
}
