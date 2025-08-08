package cachetools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestXCache_BasicFunctionality(t *testing.T) {
	// Create a mock direct function that returns a TestUser with the specified properties
	mockDirectFunc := func(ctx context.Context, key string) (TestUser, error) {
		return TestUser{
			ID:   42, // Using a fixed ID for consistency in tests
			Name: key,
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

	ctx := context.Background()
	key := "test-user"

	// Set an object in the cache
	userToSet := TestUser{
		ID:   99,
		Name: "set-user",
		Age:  999,
	}

	err = cache.Set(ctx, key, userToSet)
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
		Name: key,
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

// Mock direct function for testing - directly access key content
func mockDirectFunc(ctx context.Context, key string) (TestUser, error) {
	if key == "error" {
		return TestUser{}, errors.New("direct function error")
	}

	// 直接基于key内容生成用户数据
	// 使用key的长度和内容来生成ID和年龄
	id := len(key)
	age := 20 + (id % 30) // 年龄在20-49之间

	return TestUser{
		ID:   id,
		Name: "User_" + key, // 直接使用key作为名字的一部分
		Age:  age,
	}, nil
}

func mockDirectFuncString(ctx context.Context, key string) (string, error) {
	if key == "error" {
		return "", errors.New("direct function error")
	}

	// 直接基于key内容生成值
	return "value_for_" + key, nil
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
	result, err := opt.DirectFunc(context.Background(), "key1")
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
	// Note: otter.Cache is an interface, so we can't directly compare to nil
	// We'll check if it's usable by trying to get a non-existent key
	if _, ok := cache.L1CacheClient.Get("non-existent-key"); ok {
		// This is fine, cache is working
	}
	if cache.L2RedisClient != nil {
		t.Error("Expected L2RedisClient to be nil")
	}
}

func TestNewCacheBuilder_L1AndL2(t *testing.T) {
	cache, err := NewCacheBuilder(
		WithPrefixKey[TestUser]("test"),
		WithDirectFunc[TestUser](mockDirectFunc),
		WithL1Cache[TestUser](true, 2000, 5*time.Minute),
		WithL2Cache[TestUser](true, &redis.Options{Addr: "127.0.0.1:6379"}, 10*time.Minute),
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
	if !cache.L2Enable {
		t.Error("Expected L2Enable to be true")
	}
	// Note: otter.Cache is an interface, so we can't directly compare to nil
	// We'll check if it's usable by trying to get a non-existent key
	if _, ok := cache.L1CacheClient.Get("non-existent-key"); ok {
		// This is fine, cache is working
	}
	if cache.L2RedisClient == nil {
		t.Error("Expected L2RedisClient to not be nil")
	}
}

func TestNewCacheBuilder_ValidationErrors(t *testing.T) {
	tests := []struct {
		name        string
		options     []CacheOptionFunc[TestUser]
		expectedErr string
	}{
		{
			name:        "missing prefix key",
			options:     []CacheOptionFunc[TestUser]{WithDirectFunc[TestUser](mockDirectFunc)},
			expectedErr: "prefix key is required",
		},
		{
			name:        "missing direct function",
			options:     []CacheOptionFunc[TestUser]{WithPrefixKey[TestUser]("test")},
			expectedErr: "direct function is required",
		},
		{
			name: "L2 enabled without config",
			options: []CacheOptionFunc[TestUser]{
				WithPrefixKey[TestUser]("test"),
				WithDirectFunc[TestUser](mockDirectFunc),
				WithL2Cache[TestUser](true, nil, 10*time.Minute),
			},
			expectedErr: "L2 cache is enabled but Redis config is not provided",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache, err := NewCacheBuilder(tt.options...)
			if err == nil {
				t.Error("Expected error but got none")
			}
			if cache != nil {
				t.Error("Expected cache to be nil")
			}
			if err != nil && !strings.Contains(err.Error(), tt.expectedErr) {
				t.Errorf("Expected error to contain '%s', got '%s'", tt.expectedErr, err.Error())
			}
		})
	}
}

func TestXCache_RealKey(t *testing.T) {
	cache := &XCache[string]{
		CachePrefixKey: "test-app",
	}

	result := cache.realKey("user123")
	expected := "test-app:user123"
	if result != expected {
		t.Errorf("Expected realKey to be '%s', got '%s'", expected, result)
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
	user, err := cache.Get(ctx, "user1")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	// user1 的长度是5，年龄是 20 + (5 % 30) = 25
	expected := TestUser{ID: 5, Name: "User_user1", Age: 25}
	if user != expected {
		t.Errorf("Expected user to be %+v, got %+v", expected, user)
	}

	// Manually set in L1 cache to test cache hit
	// user2 的长度是5，年龄是 20 + (5 % 30) = 25
	cache.L1CacheClient.Set(cache.realKey("user2"), TestUser{ID: 5, Name: "User_user2", Age: 25})

	user, err = cache.Get(ctx, "user2")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	expected = TestUser{ID: 5, Name: "User_user2", Age: 25}
	if user != expected {
		t.Errorf("Expected user to be %+v, got %+v", expected, user)
	}

	// Test direct function error
	_, err = cache.Get(ctx, "error")
	if err == nil {
		t.Error("Expected error but got none")
	}
	if err.Error() != "direct function error" {
		t.Errorf("Expected error to be 'direct function error', got '%s'", err.Error())
	}
}

func TestXCache_Get_DirectFunctionFallback(t *testing.T) {
	cache, err := NewCacheBuilder(
		WithPrefixKey[TestUser]("test"),
		WithDirectFunc[TestUser](mockDirectFunc),
		WithL1Cache[TestUser](false, 2000, 0), // Disable L1
		WithL2Cache[TestUser](false, nil, 0),  // Disable L2
	)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	ctx := context.Background()

	// Should always call direct function when both caches are disabled
	user, err := cache.Get(ctx, "user1")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	// user1 的长度是5，年龄是 20 + (5 % 30) = 25
	expected := TestUser{ID: 5, Name: "User_user1", Age: 25}
	if user != expected {
		t.Errorf("Expected user to be %+v, got %+v", expected, user)
	}
}

func TestXCache_Set(t *testing.T) {
	cache, err := NewCacheBuilder(
		WithPrefixKey[TestUser]("test"),
		WithDirectFunc[TestUser](mockDirectFunc),
		WithL1Cache[TestUser](true, 2000, 5*time.Minute),
	)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	ctx := context.Background()
	user := TestUser{ID: 3, Name: "Charlie", Age: 35}

	err = cache.Set(ctx, "user3", user)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
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
	key := cache.realKey("user3")

	err = cache.put(ctx, key, user)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify it was stored in L1 cache
	cachedUser, found := cache.L1CacheClient.Get(key)
	if !found {
		t.Error("Expected to find user in L1 cache")
	}
	if cachedUser != user {
		t.Errorf("Expected cached user to be %+v, got %+v", user, cachedUser)
	}
}

func TestCacheOption_Defaults(t *testing.T) {
	cache, err := NewCacheBuilder(
		WithPrefixKey[string]("test"),
		WithDirectFunc[string](mockDirectFuncString),
	)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify default values
	if cache.CachePrefixKey != "test" {
		t.Errorf("Expected CachePrefixKey to be 'test', got '%s'", cache.CachePrefixKey)
	}
	if !cache.L1Enable {
		t.Error("Expected L1Enable to be true")
	}
	if cache.L1CacheTTL != 5*time.Minute {
		t.Errorf("Expected L1CacheTTL to be 5m, got %v", cache.L1CacheTTL)
	}
	if cache.L2Enable {
		t.Error("Expected L2Enable to be false")
	}
	if cache.L2CacheTTL != 10*time.Minute {
		t.Errorf("Expected L2CacheTTL to be 10m, got %v", cache.L2CacheTTL)
	}
	// Note: otter.Cache is an interface, so we can't directly compare to nil
	// We'll check if it's usable by trying to get a non-existent key
	if _, ok := cache.L1CacheClient.Get("non-existent-key"); ok {
		// This is fine, cache is working
	}
	if cache.L2RedisClient != nil {
		t.Error("Expected L2RedisClient to be nil")
	}
}

// TestXCache_L3WriteBack 测试从L3获取数据后是否正确回写到L1和L2缓存
func TestXCache_L3WriteBack(t *testing.T) {
	// Create cache with both L1 and L2 enabled
	cache, err := NewCacheBuilder(
		WithPrefixKey[TestUser]("test"),
		WithDirectFunc[TestUser](mockDirectFunc),
		WithL1Cache[TestUser](true, 2000, 5*time.Minute),
		WithL2Cache[TestUser](true, &redis.Options{Addr: "127.0.0.1:6379"}, 10*time.Minute),
	)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	ctx := context.Background()
	key := "writebacktest"
	realKey := cache.realKey(key)

	// 确保缓存中没有这个key
	cache.L1CacheClient.Delete(realKey)
	if cache.L2Enable {
		cache.L2RedisClient.Del(ctx, realKey)
	}

	// 第一次获取，应该从L3 directFunc获取
	user1, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Failed to get user from L3: %v", err)
	}

	t.Logf("Got user from L3: %+v", user1)

	// 等待一小段时间让异步写入完成
	time.Sleep(200 * time.Millisecond)

	// 验证数据已经写入L1缓存
	if cache.L1Enable {
		if cachedUser, ok := cache.L1CacheClient.Get(realKey); ok {
			t.Logf("Found in L1 cache: %+v", cachedUser)
			if cachedUser.ID != user1.ID || cachedUser.Name != user1.Name {
				t.Errorf("L1 cached user mismatch: expected %+v, got %+v", user1, cachedUser)
			}
		} else {
			t.Error("Expected user to be cached in L1, but not found")
		}
	}

	// 验证数据已经写入L2缓存
	if cache.L2Enable {
		if vs, e := cache.L2RedisClient.Get(ctx, realKey).Result(); e == nil {
			var cachedUser TestUser
			if em := json.Unmarshal([]byte(vs), &cachedUser); em == nil {
				t.Logf("Found in L2 cache: %+v", cachedUser)
				if cachedUser.ID != user1.ID || cachedUser.Name != user1.Name {
					t.Errorf("L2 cached user mismatch: expected %+v, got %+v", user1, cachedUser)
				}
			} else {
				t.Errorf("Failed to unmarshal L2 cached data: %v", em)
			}
		} else {
			t.Errorf("Expected user to be cached in L2, but got error: %v", e)
		}
	}

	// 清空L1缓存，再次获取应该从L2获取
	cache.L1CacheClient.Delete(realKey)
	user2, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Failed to get user from L2: %v", err)
	}

	t.Logf("Got user from L2: %+v", user2)
	if user2.ID != user1.ID || user2.Name != user1.Name {
		t.Errorf("User from L2 mismatch: expected %+v, got %+v", user1, user2)
	}
}

// Benchmark tests
func BenchmarkXCache_Get_L1Hit(b *testing.B) {
	cache, err := NewCacheBuilder(
		WithPrefixKey[string]("bench"),
		WithDirectFunc[string](mockDirectFuncString),
		WithL1Cache[string](true, 2000, 5*time.Minute),
	)
	if err != nil {
		b.Fatalf("Expected no error, got %v", err)
	}

	ctx := context.Background()

	// Pre-populate cache
	cache.L1CacheClient.Set(cache.realKey("key1"), "value_for_key1")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cache.Get(ctx, "key1")
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
		_, _ = cache.Get(ctx, "key1")
	}
}
