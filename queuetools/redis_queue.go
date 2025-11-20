package queue

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisQueue[V any] struct {
	redis   *redis.Client
	size    int64
	timeout time.Duration
	name    string
}

func NewRedisQueue[V any](name string, redis *redis.Client, size int64, timeout time.Duration) *RedisQueue[V] {
	return &RedisQueue[V]{
		redis:   redis,
		size:    size,
		timeout: timeout,
		name:    name,
	}
}

func (r RedisQueue[V]) Enqueue(vs []V) error {
	if l, e := r.Len(); e != nil {
		return e
	} else {
		if r.size > 0 && r.size < l {
			return &QueueFullError{
				name: r.name,
				size: r.size,
			}
		}
	}
	tc, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	_, err := r.redis.LPush(tc, r.name, vs).Result()
	return err
}

func (r RedisQueue[V]) Dequeue() (V, error) {
	vp := new(V)
	tc, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	result, err := r.redis.LPop(tc, r.name).Result()
	if err != nil {
		return *vp, err
	}
	err = json.Unmarshal([]byte(result), vp)
	if err != nil {
		return *vp, err
	}
	return *vp, nil
}

func (r RedisQueue[V]) Len() (int64, error) {
	tc, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	return r.redis.LLen(tc, r.name).Result()
}
