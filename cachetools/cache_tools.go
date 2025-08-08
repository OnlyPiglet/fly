package cachetools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/maypok86/otter"
	redis "github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

/**
    多级缓存库，l1为内存缓存;redis为2级缓存;l3是当1，2级缓存不存在提供的获取对象的函数，可以是从mysql中获取，http调用获取等等
    Multi-level cache library: L1 is memory cache; Redis is L2 cache; L3 is a function to get objects when L1 and L2 cache don't exist, can be from MySQL, HTTP calls, etc.
	memory l1cache <- redis l2cache <- l3 directFunc (使用单飞，防止击穿),使用 L3FlightErrContinue 控制当 l3 获取结果失败 是否继续单飞
	memory l1cache <- redis l2cache <- l3 directFunc (using singleflight to prevent cache stampede), use L3FlightErrContinue to control whether to continue singleflight when L3 result fails
*/

type Key interface {
	ToString() string
}

type StringKey string

func (k StringKey) ToString() string {
	return string(k)
}

type DirectFunc[V any] func(ctx context.Context, k Key) (V, error)

type LocalL2Client interface{}

type L1RedisClient interface{}

type XCache[V any] struct {
	CachePrefixKey string
	// L1Enable 内存缓存是否开启
	// L1Enable whether memory cache is enabled
	L1Enable      bool
	L1CacheTTL    time.Duration
	L1CacheClient otter.Cache[string, V]
	// L2Enable Redis是否进行二级缓存
	// L2Enable whether Redis L2 cache is enabled
	L2Enable      bool
	L2CacheTTL    time.Duration
	L2RedisClient *redis.Client
	// 用于防止缓存击穿的单飞模式
	// Singleflight pattern to prevent cache stampede
	L3DirectFunc        DirectFunc[V]
	flightGroup         *singleflight.Group
	L3FlightErrContinue bool
}

func (xc *XCache[V]) realKey(k Key) string {
	return fmt.Sprintf("%s:%s", xc.CachePrefixKey, k.ToString())
}

type CacheOption[V any] struct {
	PrefixKey           string
	Capacity            int
	L1Enable            bool
	L1CacheTTL          time.Duration
	L2Enable            bool
	L2Config            *redis.Options
	L2CacheTTL          time.Duration
	DirectFunc          DirectFunc[V]
	L3FlightErrContinue bool
}

// CacheOptionFunc defines a function type for configuring CacheOption
type CacheOptionFunc[V any] func(*CacheOption[V])

// WithPrefixKey sets the cache prefix key
func WithPrefixKey[V any](prefixKey string) CacheOptionFunc[V] {
	return func(opt *CacheOption[V]) {
		opt.PrefixKey = prefixKey
	}
}

// WithL1Cache enables l1 cache with TTL with capacity,TTL为0时 永不过期
// WithL1Cache enables L1 cache with TTL and capacity, TTL=0 means never expire
func WithL1Cache[V any](enable bool, capacity int, ttl time.Duration) CacheOptionFunc[V] {
	return func(opt *CacheOption[V]) {
		opt.L1Enable = enable
		opt.L1CacheTTL = ttl
		opt.Capacity = capacity
	}
}

// WithL2Cache enables l2 cache with Redis config and TTL, 0 永不过期
// WithL2Cache enables L2 cache with Redis config and TTL, TTL=0 means never expire
func WithL2Cache[V any](enable bool, config *redis.Options, ttl time.Duration) CacheOptionFunc[V] {
	return func(opt *CacheOption[V]) {
		opt.L2Enable = enable
		opt.L2Config = config
		opt.L2CacheTTL = ttl
	}
}

// WithDirectFunc sets the direct function for L3 cache
func WithDirectFunc[V any](directFunc DirectFunc[V]) CacheOptionFunc[V] {
	return func(opt *CacheOption[V]) {
		opt.DirectFunc = directFunc
	}
}

func WithL3FlightErrContinue[V any](con bool) CacheOptionFunc[V] {
	return func(opt *CacheOption[V]) {
		opt.L3FlightErrContinue = con
	}
}

func NewCacheBuilder[V any](optFuncs ...CacheOptionFunc[V]) (*XCache[V], error) {
	// Initialize default options
	opt := &CacheOption[V]{
		Capacity:            1000, // default capacity
		L1Enable:            true, // 默认启用L1缓存 (L1 cache enabled by default)
		L1CacheTTL:          5 * time.Minute,
		L2Enable:            false,
		L2CacheTTL:          10 * time.Minute,
		L3FlightErrContinue: false,
	}

	// Apply all option functions
	for _, optFunc := range optFuncs {
		optFunc(opt)
	}

	if opt.L2Enable && opt.L1Enable && opt.L2CacheTTL != 0 && opt.L2CacheTTL < opt.L1CacheTTL {
		return nil, fmt.Errorf("l2 cache ttl shoud bigger than l1 cache ttl")
	}

	// Validate required options
	if opt.PrefixKey == "" {
		return nil, fmt.Errorf("prefix key is required")
	}
	if opt.DirectFunc == nil {
		return nil, fmt.Errorf("direct function is required")
	}

	cb := new(XCache[V])
	cb.CachePrefixKey = opt.PrefixKey
	cb.L1Enable = opt.L1Enable
	cb.L1CacheTTL = opt.L1CacheTTL
	cb.L2Enable = opt.L2Enable
	cb.L2CacheTTL = opt.L2CacheTTL
	cb.L3DirectFunc = opt.DirectFunc
	cb.flightGroup = &singleflight.Group{}
	cb.L3FlightErrContinue = opt.L3FlightErrContinue

	if opt.L1Enable {
		cache, err := otter.MustBuilder[string, V](opt.Capacity).
			CollectStats().
			WithTTL(opt.L1CacheTTL).
			Build()
		if err != nil {
			return nil, err
		}
		cb.L1CacheClient = cache
	}
	if opt.L2Enable {
		if opt.L2Config == nil {
			return nil, fmt.Errorf("l2 cache is enabled but Redis config is not provided")
		}
		cb.L2RedisClient = redis.NewClient(opt.L2Config)
	}

	return cb, nil
}

func (xc *XCache[V]) Get(ctx context.Context, key Key) (V, error) {
	realKey := xc.realKey(key)

	if xc.L1Enable {
		if v, ok := xc.L1CacheClient.Get(realKey); ok {
			slog.Debug(fmt.Sprintf("get key %s from l1 cache", key))
			return v, nil
		}
	}

	if xc.L2Enable {
		if vs, e := xc.L2RedisClient.Get(ctx, realKey).Result(); e == nil {
			v := new(V)
			em := json.Unmarshal([]byte(vs), v)
			if em != nil {
				slog.Error("l2 cache get Unmarshal err", "error", em.Error())
			} else {
				slog.Debug(fmt.Sprintf("get key %s from l2 cache", key))
				if xc.L1Enable {
					go func() {
						xc.L1CacheClient.Set(realKey, *v)
					}()
				}
				return *v, nil
			}
		}
	}

	return xc.getFromL3WithSingleFlight(ctx, key)
}

func (xc *XCache[V]) Put(ctx context.Context, key Key, v V) error {
	return xc.put(ctx, xc.realKey(key), v)
}

func (xc *XCache[V]) getFromL3WithSingleFlight(ctx context.Context, key Key) (V, error) {
	realKey := xc.realKey(key)

	slog.Debug(fmt.Sprintf("get key %s from L3 directFunc", key))

	v, err, shared := xc.flightGroup.Do(realKey, func() (interface{}, error) {
		value, err := xc.L3DirectFunc(ctx, key)
		if err != nil {
			if xc.L3FlightErrContinue {
				xc.flightGroup.Forget(key.ToString())
			}
			return value, err
		}
		go func() {
			if err := xc.put(ctx, realKey, value); err != nil {
				slog.Error("failed to cache L3 result", "key", key, "error", err)
			}
		}()
		return value, nil
	})
	if shared {
		slog.Debug(fmt.Sprintf("key %s result shared from singleflight", key))
	}
	return v.(V), err
}

func (xc *XCache[V]) put(ctx context.Context, key string, v V) error {

	if !xc.L1Enable && !xc.L2Enable {
		return nil
	}

	var l1Err, l2Err error

	// 写入L1缓存
	// Write to L1 cache
	if xc.L1Enable {
		if ok := xc.L1CacheClient.Set(key, v); !ok {
			l1Err = fmt.Errorf("l1 memory cache set failed, cost too much")
			slog.Error("l1 cache set failed", "key", key, "error", l1Err)
		}
	}

	// 写入L2缓存
	// Write to L2 cache
	if xc.L2Enable {
		if vb, e := json.Marshal(v); e != nil {
			l2Err = fmt.Errorf("l2 cache marshal failed: %w", e)
			slog.Error("l2 cache marshal failed", "key", key, "error", l2Err)
		} else {
			if err := xc.L2RedisClient.Set(ctx, key, string(vb), xc.L2CacheTTL).Err(); err != nil {
				l2Err = fmt.Errorf("l2 cache set failed: %w", err)
				slog.Error("l2 cache set failed", "key", key, "error", l2Err)
			}
		}
	}

	if l1Err != nil {
		return fmt.Errorf("l1 cache failed: %s", l1Err.Error())
	}

	if l2Err != nil {
		return fmt.Errorf("l2 cache failed: %s", l2Err.Error())
	}

	return nil
}
