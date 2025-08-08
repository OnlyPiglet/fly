package cachetools

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// Mock direct function for testing - directly access key content
func mockDirectFunc(ctx context.Context, key Key) (TestUser, error) {
	keyStr := key.ToString()
	if keyStr == "error" {
		return TestUser{}, errors.New("direct function error")
	}

	// 直接基于key内容生成用户数据
	// 使用key的长度和内容来生成ID和年龄
	id := len(keyStr)
	age := 20 + (id % 30) // 年龄在20-49之间

	return TestUser{
		ID:   id,
		Name: "User_" + keyStr, // 直接使用key作为名字的一部分
		Age:  age,
	}, nil
}

func mockDirectFuncString(ctx context.Context, key Key) (string, error) {
	keyStr := key.ToString()
	if keyStr == "error" {
		return "", errors.New("direct function error")
	}

	// 直接基于key内容生成值
	return "value_for_" + keyStr, nil
}

func TestXCache_BasicFunctionality(t *testing.T) {
	// Create a mock direct function that returns a TestUser with the specified properties
	mockDirectFunc := func(ctx context.Context, key Key) (TestUser, error) {
		return TestUser{
			ID:   42, // Using a fixed ID for consistency in tests
			Name: key.ToString(),
			Age:  123,
		}, nil
	}

	// Create cache with both L1 and L2 enabled
	cache, err := NewCacheBuilder(
		WithPrefixKey[TestUser]("test"),
		WithDirectFunc[TestUser](mockDirectFunc),
		WithL1Cache[TestUser](true, 2000, 10*time.Second),                                   // L1 cache TTL: 10 seconds
		WithL2Cache[TestUser](true, &redis.Options{Addr: "127.0.0.1:6379"}, 23*time.Second), // L2 cache TTL: 23 seconds
	)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	go func() {
		for {
			select {
			case <-ticker.C:
				fmt.Printf("命中数: %d\n", cache.L1CacheClient.Stats().Hits())
				fmt.Printf("未命中数: %d\n", cache.L1CacheClient.Stats().Misses())
				fmt.Printf("命中率: %.2f%%\n", cache.L1CacheClient.Stats().Ratio()*100)
				fmt.Printf("被拒绝的设置操作: %d\n", cache.L1CacheClient.Stats().RejectedSets())
				fmt.Printf("被驱逐的条目数: %d\n", cache.L1CacheClient.Stats().EvictedCount())
			}
		}
	}()

	ctx := context.Background()
	key := StringKey("test-user")

	// Set an object in the cache
	userToSet := TestUser{
		ID:   99,
		Name: "set-user",
		Age:  999,
	}

	err = cache.Put(ctx, key, userToSet)
	if err != nil {
		t.Fatalf("Failed to set user in cache: %v", err)
	}

	t.Logf("Set user: %+v", userToSet)

	// Immediately get the object (should be from L1 cache)
	user, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Failed to get user immediately after setting: %v", err)
	}

	t.Logf("Got user after setting (should be from L1): %+v", user)

	// Check that we got the correct user
	if user != userToSet {
		t.Errorf("Expected user %+v, but got %+v", userToSet, user)
	}

	// Wait 5 seconds and get the object (should still be from L1 cache)
	time.Sleep(5 * time.Second)

	user, err = cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Failed to get user after 5 seconds: %v", err)
	}

	t.Logf("Got user after 5 seconds (should be from L1): %+v", user)

	if user != userToSet {
		t.Errorf("Expected user %+v, but got %+v after 5 seconds", userToSet, user)
	}

	// Wait another 6 seconds (total 11 seconds) - L1 should be expired, should come from L2
	time.Sleep(6 * time.Second)

	user, err = cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Failed to get user after 11 seconds: %v", err)
	}

	t.Logf("Got user after 11 seconds (should be from L2): %+v", user)

	// Should be the originally set user, not from directFunc yet
	if user != userToSet {
		t.Errorf("Expected user %+v, but got %+v after 11 seconds", userToSet, user)
	}

	// Wait another 19 seconds (total 30 seconds) - both L1 and L2 should be expired
	// Should come from directFunc with different values
	time.Sleep(19 * time.Second)

	user, err = cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Failed to get user after 30 seconds: %v", err)
	}

	t.Logf("Got user after 30 seconds (should be from directFunc): %+v", user)

	// Should be from directFunc now (different values)
	expectedUser := TestUser{
		ID:   42,
		Name: key.ToString(),
		Age:  123,
	}

	if user.Name != expectedUser.Name || user.Age != expectedUser.Age {
		t.Errorf("Expected user %+v, but got %+v after 30 seconds", expectedUser, user)
	}
}

// Test data structures
type TestUser struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func TestWithPrefixKey(t *testing.T) {
	opt := &CacheOption[string]{}
	WithPrefixKey[string]("test-prefix")(opt)
	if opt.PrefixKey != "test-prefix" {
		t.Errorf("Expected PrefixKey to be 'test-prefix', got '%s'", opt.PrefixKey)
	}
}

func TestWithL1Cache(t *testing.T) {
	opt := &CacheOption[string]{}
	ttl := 10 * time.Minute
	WithL1Cache[string](true, 2000, ttl)(opt)
	if !opt.L1Enable {
		t.Error("Expected L1Enable to be true")
	}
	if opt.L1CacheTTL != ttl {
		t.Errorf("Expected L1CacheTTL to be %v, got %v", ttl, opt.L1CacheTTL)
	}
	if opt.Capacity != 2000 {
		t.Errorf("Expected Capacity to be 2000, got %v", opt.Capacity)
	}
}

func TestWithL2Cache(t *testing.T) {
	opt := &CacheOption[string]{}
	redisOpts := &redis.Options{Addr: "127.0.0.1:6379"}
	ttl := 15 * time.Minute
	WithL2Cache[string](true, redisOpts, ttl)(opt)
	if !opt.L2Enable {
		t.Error("Expected L2Enable to be true")
	}
	if opt.L2Config != redisOpts {
		t.Error("Expected L2Config to match provided redis options")
	}
	if opt.L2CacheTTL != ttl {
		t.Errorf("Expected L2CacheTTL to be %v, got %v", ttl, opt.L2CacheTTL)
	}
}

func TestWithDirectFunc(t *testing.T) {
	opt := &CacheOption[string]{}
	WithDirectFunc[string](mockDirectFuncString)(opt)
	if opt.DirectFunc == nil {
		t.Error("Expected DirectFunc to not be nil")
	}

	// Test the function works
	result, err := opt.DirectFunc(context.Background(), StringKey("key1"))
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result != "value_for_key1" {
		t.Errorf("Expected result to be 'value_for_key1', got '%s'", result)
	}
}

func TestNewCacheBuilder_L1Only(t *testing.T) {
	cache, err := NewCacheBuilder(
		WithPrefixKey[TestUser]("test"),
		WithDirectFunc[TestUser](mockDirectFunc),
		WithL1Cache[TestUser](true, 2000, 5*time.Minute),
	)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if cache == nil {
		t.Fatal("Expected cache to not be nil")
	}
	if cache.CachePrefixKey != "test" {
		t.Errorf("Expected CachePrefixKey to be 'test', got '%s'", cache.CachePrefixKey)
	}
	if !cache.L1Enable {
		t.Error("Expected L1Enable to be true")
	}
	if cache.L2Enable {
		t.Error("Expected L2Enable to be false")
	}
	if cache.L1CacheTTL != 5*time.Minute {
		t.Errorf("Expected L1CacheTTL to be 5m, got %v", cache.L1CacheTTL)
	}
	if cache.L2RedisClient != nil {
		t.Error("Expected L2RedisClient to be nil")
	}
}

func TestXCache_Get_L1Only(t *testing.T) {
	cache, err := NewCacheBuilder(
		WithPrefixKey[TestUser]("test"),
		WithDirectFunc[TestUser](mockDirectFunc),
		WithL1Cache[TestUser](true, 2000, 5*time.Minute),
		WithL2Cache[TestUser](false, nil, 0),
	)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	ctx := context.Background()

	// Test cache miss - should call direct function
	user, err := cache.Get(ctx, StringKey("user1"))
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	// user1 的长度是5，年龄是 20 + (5 % 30) = 25
	expected := TestUser{ID: 5, Name: "User_user1", Age: 25}
	if user != expected {
		t.Errorf("Expected user to be %+v, got %+v", expected, user)
	}

	// Test direct function error
	_, err = cache.Get(ctx, StringKey("error"))
	if err == nil {
		t.Error("Expected error but got none")
	}
	if err.Error() != "direct function error" {
		t.Errorf("Expected error to be 'direct function error', got '%s'", err.Error())
	}
}

func TestXCache_Put_L1Only(t *testing.T) {
	cache, err := NewCacheBuilder(
		WithPrefixKey[TestUser]("test"),
		WithDirectFunc[TestUser](mockDirectFunc),
		WithL1Cache[TestUser](true, 2000, 5*time.Minute),
		WithL2Cache[TestUser](false, nil, 0),
	)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	ctx := context.Background()
	user := TestUser{ID: 3, Name: "Charlie", Age: 35}
	key := StringKey("user3")
	realKey := cache.realKey(key)

	err = cache.Put(ctx, key, user)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify it was stored in L1 cache
	cachedUser, found := cache.L1CacheClient.Get(realKey)
	if !found {
		t.Error("Expected to find user in L1 cache")
	}
	if cachedUser != user {
		t.Errorf("Expected cached user to be %+v, got %+v", user, cachedUser)
	}
}

func BenchmarkXCache_Get_DirectFunction(b *testing.B) {
	cache, err := NewCacheBuilder(
		WithPrefixKey[string]("bench"),
		WithDirectFunc[string](mockDirectFuncString),
		WithL1Cache[string](false, 2000, 0), // Disable cache to force direct function calls
	)
	if err != nil {
		b.Fatalf("Expected no error, got %v", err)
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cache.Get(ctx, StringKey("key1"))
	}
}
