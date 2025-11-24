package queue_tools

import (
	"context"
	"encoding/json"
	"fmt"
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

// Lua 脚本：在 Redis 端原子地 LLEN + 容量校验 + 批量 LPUSH（分批，防止 unpack 过多）
var enqueueScript = redis.NewScript(`
local key     = KEYS[1]
local maxSize = tonumber(ARGV[1])
local nVals   = tonumber(ARGV[2])

local firstValIndex = 3
local lastValIndex  = 2 + nVals

-- 分批 LPUSH，避免一次 unpack 太多参数
local function batched_lpush(batchSize)
    local i = firstValIndex
    while i <= lastValIndex do
        local j = i + batchSize - 1
        if j > lastValIndex then
            j = lastValIndex
        end
        redis.call("LPUSH", key, unpack(ARGV, i, j))
        i = j + 1
    end
end

-- 无容量限制，直接分批 LPUSH
if maxSize <= 0 then
    if nVals > 0 then
        batched_lpush(1000)
    end
    return redis.call("LLEN", key)
end

-- 有容量限制：先看当前长度
local cur = redis.call("LLEN", key)

-- 容量检查：当前长度 + 本次要入队的数量 > maxSize 则拒绝
if cur + nVals > maxSize then
    return -1
end

if nVals > 0 then
    batched_lpush(1000)
end

return cur + nVals
`)

// Enqueue 批量入队：
// - size <= 0：不限制容量，直接 LPUSH
// - size > 0：通过 Lua 严格校验容量，满了返回 QueueFullError
func (r RedisQueue[V]) Enqueue(vs []V) error {
	if len(vs) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	// 先序列化成 JSON，再复用这个切片给 Lua 参数 / 普通 LPUSH
	values := make([]interface{}, len(vs))
	for i, v := range vs {
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		values[i] = b
	}

	// 无容量限制：直接 LPUSH，避免走 Lua，也不会触发 unpack 限制
	if r.size <= 0 {
		_, err := r.redis.LPush(ctx, r.name, values...).Result()
		return err
	}

	// 有容量限制：走 Lua 脚本
	args := make([]interface{}, 0, 2+len(values))
	args = append(args, r.size)      // ARGV[1] = maxSize
	args = append(args, len(values)) // ARGV[2] = nVals
	args = append(args, values...)   // ARGV[3..] = 每个 JSON value

	res, err := enqueueScript.Run(ctx, r.redis, []string{r.name}, args...).Result()
	if err != nil {
		return err
	}

	n, ok := res.(int64)
	if !ok {
		return fmt.Errorf("unexpected result type: %T", res)
	}
	if n == -1 {
		return &QueueFullError{
			name: r.name,
			size: r.size,
		}
	}

	return nil
}

// 任务队列建议使用 FIFO：LPUSH + RPOP
func (r RedisQueue[V]) Dequeue() (V, error) {
	var v V

	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	result, err := r.redis.RPop(ctx, r.name).Bytes()
	if err != nil {
		return v, err
	}
	if err := json.Unmarshal(result, &v); err != nil {
		return v, err
	}
	return v, nil
}

// DequeueBatch 批量出队：一次性从队列尾部取出最多 count 个元素
// 返回实际取出的元素数量和可能的错误
func (r RedisQueue[V]) DequeueBatch(count int) ([]V, error) {
	if count <= 0 {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	// 使用 Lua 脚本原子地批量 RPOP
	script := redis.NewScript(`
		local key = KEYS[1]
		local count = tonumber(ARGV[1])
		local result = {}
		
		for i = 1, count do
			local val = redis.call("RPOP", key)
			if not val then
				break
			end
			table.insert(result, val)
		end
		
		return result
	`)

	res, err := script.Run(ctx, r.redis, []string{r.name}, count).Result()
	if err != nil {
		return nil, err
	}

	// 解析结果
	rawResults, ok := res.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected result type: %T", res)
	}

	results := make([]V, 0, len(rawResults))
	for _, raw := range rawResults {
		str, ok := raw.(string)
		if !ok {
			continue
		}

		var v V
		if err := json.Unmarshal([]byte(str), &v); err != nil {
			// 跳过解析失败的数据，继续处理其他数据
			continue
		}
		results = append(results, v)
	}

	return results, nil
}

func (r RedisQueue[V]) Len() (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	return r.redis.LLen(ctx, r.name).Result()
}
