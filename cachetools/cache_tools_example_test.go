package cachetools

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	redis "github.com/redis/go-redis/v9"
)

// TestUser 测试用户结构
type TestUser struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Age  int    `json:"age"`
}

// Example 1: 仅使用 L1 内存缓存（最简单）
func TestXCache_L1Only(t *testing.T) {
	// 定义 L3 直接获取函数（模拟从数据库获取）
	directFunc := func(ctx context.Context, key StringKey) (TestUser, error) {
		keyStr := key.ToString()
		if keyStr == "error" {
			return TestUser{}, errors.New("user not found")
		}
		
		// 模拟从数据库查询
		return TestUser{
			ID:   1,
			Name: "User_" + keyStr,
			Age:  25,
		}, nil
	}

	// 创建缓存实例（仅 L1）
	cache, err := NewCacheBuilder(
		directFunc,
		WithPrefixKey("user"),
		WithL1Cache(true, 1000, 5*time.Minute), // 容量1000，TTL 5分钟
	)
	if err != nil {
		t.Fatalf("创建缓存失败: %v", err)
	}

	ctx := context.Background()
	key := StringKey("john")

	// 第一次获取（缓存未命中，调用 directFunc）
	user, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("获取用户失败: %v", err)
	}
	t.Logf("第一次获取（来自 directFunc）: %+v", user)

	// 第二次获取（缓存命中，从 L1 获取）
	user, err = cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("获取用户失败: %v", err)
	}
	t.Logf("第二次获取（来自 L1 缓存）: %+v", user)

	// 检查缓存统计
	stats := cache.L1CacheClient.Stats()
	t.Logf("L1 统计信息: 命中=%d, 未命中=%d, 命中率=%.2f%%",
		stats.Hits(), stats.Misses(), stats.Ratio()*100)
}

// Example 2: L1 + L2（Redis）两级缓存
func TestXCache_L1AndL2(t *testing.T) {
	callCount := 0
	directFunc := func(ctx context.Context, key StringKey) (TestUser, error) {
		callCount++
		t.Logf("directFunc 被调用（第 %d 次）", callCount)
		
		return TestUser{
			ID:   callCount,
			Name: "User_" + key.ToString(),
			Age:  20 + callCount,
		}, nil
	}

	// 创建缓存实例（L1 + L2）
	cache, err := NewCacheBuilder(
		directFunc,
		WithPrefixKey("user"),
		WithL1Cache(true, 1000, 3*time.Second),  // L1: 3秒过期
		WithL2Cache(true, &redis.Options{
			Addr: "127.0.0.1:6379",
			DB:   0,
		}, 10*time.Second), // L2: 10秒过期
	)
	if err != nil {
		t.Fatalf("创建缓存失败: %v", err)
	}

	ctx := context.Background()
	key := StringKey("alice")

	// 第一次获取（未命中，从 directFunc 获取）
	user1, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("第一次获取失败: %v", err)
	}
	t.Logf("1. 获取结果（来自 directFunc）: %+v", user1)

	// 第二次获取（从 L1 缓存获取）
	user2, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("第二次获取失败: %v", err)
	}
	t.Logf("2. 获取结果（来自 L1）: %+v", user2)

	// 等待 L1 过期（4秒）
	t.Log("等待 4 秒让 L1 过期...")
	time.Sleep(4 * time.Second)

	// 第三次获取（L1 过期，从 L2 获取）
	user3, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("第三次获取失败: %v", err)
	}
	t.Logf("3. 获取结果（来自 L2）: %+v", user3)

	// 验证 directFunc 只被调用了一次
	if callCount != 1 {
		t.Errorf("directFunc 应该只被调用 1 次，实际调用了 %d 次", callCount)
	}

	// 等待 L2 也过期
	t.Log("等待 7 秒让 L2 也过期...")
	time.Sleep(7 * time.Second)

	// 第四次获取（L1、L2 都过期，再次调用 directFunc）
	user4, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("第四次获取失败: %v", err)
	}
	t.Logf("4. 获取结果（再次来自 directFunc）: %+v", user4)

	// 验证 directFunc 被调用了两次
	if callCount != 2 {
		t.Errorf("directFunc 应该被调用 2 次，实际调用了 %d 次", callCount)
	}
}

// Example 3: 使用 L1 过期自动重载功能
func TestXCache_L1ExpireReload(t *testing.T) {
	callCount := 0
	directFunc := func(ctx context.Context, key StringKey) (string, error) {
		callCount++
		timestamp := time.Now().Format("15:04:05")
		result := fmt.Sprintf("Value_%s_%d", timestamp, callCount)
		t.Logf("directFunc 被调用: %s", result)
		return result, nil
	}

	// 创建缓存实例，启用 L1 过期自动重载
	cache, err := NewCacheBuilder(
		directFunc,
		WithPrefixKey("data"),
		WithL1Cache(true, 100, 2*time.Second), // 2秒过期
		WithL1ExpireReload(true),              // 启用过期自动重载
	)
	if err != nil {
		t.Fatalf("创建缓存失败: %v", err)
	}

	ctx := context.Background()
	key := StringKey("key1")

	// 第一次获取
	value1, _ := cache.Get(ctx, key)
	t.Logf("第一次获取: %s", value1)

	// 等待过期并自动重载
	t.Log("等待 3 秒，触发自动重载...")
	time.Sleep(3 * time.Second)

	// 此时 directFunc 应该已经被自动调用重载了
	if callCount < 2 {
		t.Logf("警告: 期望 directFunc 被调用至少 2 次，实际 %d 次", callCount)
	}
}

// Example 4: 防止缓存击穿（单飞模式）
func TestXCache_SingleFlight(t *testing.T) {
	callCount := 0
	directFunc := func(ctx context.Context, key StringKey) (string, error) {
		callCount++
		t.Logf("directFunc 开始执行（第 %d 次）", callCount)
		// 模拟慢查询
		time.Sleep(500 * time.Millisecond)
		t.Logf("directFunc 执行完成（第 %d 次）", callCount)
		return "value_" + key.ToString(), nil
	}

	cache, err := NewCacheBuilder(
		directFunc,
		WithPrefixKey("singleflight"),
		WithL1Cache(false, 100, 0), // 禁用 L1 以测试单飞
	)
	if err != nil {
		t.Fatalf("创建缓存失败: %v", err)
	}

	ctx := context.Background()
	key := StringKey("expensive_key")

	// 并发发起 10 个请求
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			value, err := cache.Get(ctx, key)
			if err != nil {
				t.Errorf("goroutine %d 获取失败: %v", id, err)
			} else {
				t.Logf("goroutine %d 获取成功: %s", id, value)
			}
			done <- true
		}(i)
	}

	// 等待所有请求完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证 directFunc 只被调用了一次（单飞生效）
	if callCount != 1 {
		t.Errorf("单飞失败: directFunc 应该只被调用 1 次，实际调用了 %d 次", callCount)
	} else {
		t.Log("✓ 单飞成功: 10 个并发请求只触发了 1 次 directFunc 调用")
	}
}

// Example 5: 错误处理和继续单飞
func TestXCache_ErrorHandling(t *testing.T) {
	callCount := 0
	directFunc := func(ctx context.Context, key StringKey) (string, error) {
		callCount++
		if callCount <= 2 {
			// 前两次调用返回错误
			return "", fmt.Errorf("temporary error (attempt %d)", callCount)
		}
		// 第三次调用成功
		return "success", nil
	}

	// 启用 L3FlightErrContinue，错误时继续单飞
	cache, err := NewCacheBuilder(
		directFunc,
		WithPrefixKey("error_test"),
		WithL1Cache(false, 100, 0),
		WithL3FlightErrContinue(true), // 错误时继续单飞
	)
	if err != nil {
		t.Fatalf("创建缓存失败: %v", err)
	}

	ctx := context.Background()
	key := StringKey("retry_key")

	// 第一次调用（失败）
	_, err1 := cache.Get(ctx, key)
	t.Logf("第一次调用: err=%v", err1)

	// 第二次调用（失败）
	_, err2 := cache.Get(ctx, key)
	t.Logf("第二次调用: err=%v", err2)

	// 第三次调用（成功）
	value, err3 := cache.Get(ctx, key)
	t.Logf("第三次调用: value=%s, err=%v", value, err3)

	if err3 != nil {
		t.Errorf("第三次调用应该成功，但失败了: %v", err3)
	}

	if callCount != 3 {
		t.Errorf("directFunc 应该被调用 3 次，实际 %d 次", callCount)
	}
}

// Example 6: 完整示例 - 用户查询场景
func ExampleXCache_UserQuery() {
	// 模拟数据库
	database := map[string]TestUser{
		"user:1": {ID: 1, Name: "Alice", Age: 28},
		"user:2": {ID: 2, Name: "Bob", Age: 32},
		"user:3": {ID: 3, Name: "Charlie", Age: 25},
	}

	// L3 直接获取函数（从数据库查询）
	directFunc := func(ctx context.Context, key StringKey) (TestUser, error) {
		keyStr := key.ToString()
		fmt.Printf("[DB查询] 查询用户: %s\n", keyStr)
		
		user, ok := database[keyStr]
		if !ok {
			return TestUser{}, fmt.Errorf("user not found: %s", keyStr)
		}
		return user, nil
	}

	// 创建多级缓存
	cache, err := NewCacheBuilder(
		directFunc,
		WithPrefixKey("user"),
		WithL1Cache(true, 1000, 5*time.Minute),
		// 如果有 Redis，可以取消下面的注释
		// WithL2Cache(true, &redis.Options{Addr: "127.0.0.1:6379"}, 10*time.Minute),
	)
	if err != nil {
		fmt.Printf("创建缓存失败: %v\n", err)
		return
	}

	ctx := context.Background()

	// 查询用户（第一次，缓存未命中）
	user1, _ := cache.Get(ctx, StringKey("user:1"))
	fmt.Printf("获取用户 1: %+v\n", user1)

	// 再次查询（命中 L1 缓存）
	user1Again, _ := cache.Get(ctx, StringKey("user:1"))
	fmt.Printf("再次获取用户 1: %+v\n", user1Again)

	// 查询其他用户
	user2, _ := cache.Get(ctx, StringKey("user:2"))
	fmt.Printf("获取用户 2: %+v\n", user2)

	// 查看缓存统计
	stats := cache.L1CacheClient.Stats()
	fmt.Printf("\n缓存统计:\n")
	fmt.Printf("  命中数: %d\n", stats.Hits())
	fmt.Printf("  未命中数: %d\n", stats.Misses())
	fmt.Printf("  命中率: %.2f%%\n", stats.Ratio()*100)
}

