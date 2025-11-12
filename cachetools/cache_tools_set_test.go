package cachetools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	redis "github.com/redis/go-redis/v9"
)

// TestRedis_BasicConnection 测试 Redis 基本连接和读写
func TestRedis_BasicConnection(t *testing.T) {
	ctx := context.Background()

	t.Log("=== 测试 Redis 基本功能 ===")

	// 1. 创建 Redis 客户端
	client := redis.NewClient(&redis.Options{
		Addr: "127.0.0.1:6379",
		DB:   0,
	})
	defer client.Close()

	// 2. 测试连接
	t.Log("步骤1: 测试 Ping")
	pong, err := client.Ping(ctx).Result()
	if err != nil {
		t.Fatalf("❌ Redis Ping 失败: %v\n提示: 请确保 Redis 已启动 (运行命令: redis-server)", err)
	}
	t.Logf("✓ Redis Ping 成功: %s", pong)

	// 3. 测试写入
	t.Log("\n步骤2: 测试写入数据")
	testKey := "test:basic:key"
	testValue := "test_value_123"
	err = client.Set(ctx, testKey, testValue, 10*time.Second).Err()
	if err != nil {
		t.Fatalf("❌ Redis Set 失败: %v", err)
	}
	t.Logf("✓ 写入成功: key=%s, value=%s", testKey, testValue)

	// 4. 测试读取
	t.Log("\n步骤3: 测试读取数据")
	result, err := client.Get(ctx, testKey).Result()
	if err != nil {
		t.Fatalf("❌ Redis Get 失败: %v", err)
	}
	if result != testValue {
		t.Errorf("❌ 读取的值不匹配: 期望 %s, 实际 %s", testValue, result)
	}
	t.Logf("✓ 读取成功: %s", result)

	// 5. 测试 JSON 数据
	t.Log("\n步骤4: 测试 JSON 数据")
	testUser := TestUser{ID: 999, Name: "TestUser", Age: 99}
	jsonData, _ := json.Marshal(testUser)
	testJsonKey := "test:basic:json"

	err = client.Set(ctx, testJsonKey, string(jsonData), 10*time.Second).Err()
	if err != nil {
		t.Fatalf("❌ Redis Set JSON 失败: %v", err)
	}
	t.Logf("✓ JSON 写入成功: %s", string(jsonData))

	jsonResult, err := client.Get(ctx, testJsonKey).Result()
	if err != nil {
		t.Fatalf("❌ Redis Get JSON 失败: %v", err)
	}
	t.Logf("✓ JSON 读取成功: %s", jsonResult)

	// 6. 列出所有测试键
	t.Log("\n步骤5: 列出 Redis 中的测试键")
	allKeys, err := client.Keys(ctx, "test:basic:*").Result()
	if err != nil {
		t.Fatalf("❌ Redis Keys 失败: %v", err)
	}
	t.Logf("✓ 找到 %d 个测试键: %v", len(allKeys), allKeys)

	// 7. 清理
	t.Log("\n步骤6: 清理测试数据")
	client.Del(ctx, testKey, testJsonKey)
	t.Log("✓ 清理完成")

	t.Log("\n=== ✓ Redis 基本功能测试全部通过 ===")
}

// TestXCache_Set_ManualVerify 手动验证测试 - 数据不会自动删除，方便在 Redis 客户端查看
// 这个测试会向 Redis 写入数据并保留 30 分钟，方便手动验证
func TestXCache_Set_ManualVerify(t *testing.T) {
	// 注意：这个测试会在 Redis 中保留数据，如果不想运行，请跳过
	// 如果想跳过，取消下面这行的注释：
	// t.Skip("跳过手动验证测试")

	directFunc := func(ctx context.Context, key StringKey) (TestUser, error) {
		return TestUser{}, nil
	}

	ctx := context.Background()

	t.Log("=== 第1步: 连接 Redis 并创建缓存实例 ===")
	cache, err := NewCacheBuilder(
		directFunc,
		WithPrefixKey("manual_test"),
		WithL1Cache(false, 1000, 5*time.Minute),
		WithL2Cache(true, &redis.Options{
			Addr: "127.0.0.1:6379",
			DB:   0,
		}, 0), // 30分钟过期，方便查看
	)
	if err != nil {
		t.Fatalf("❌ 创建缓存失败: %v", err)
	}
	t.Log("✓ 缓存实例创建成功")

	// 写入3个测试用户
	testUsers := []struct {
		key  StringKey
		user TestUser
	}{
		{StringKey("user:1001"), TestUser{ID: 1001, Name: "Alice", Age: 25}},
		{StringKey("user:1002"), TestUser{ID: 1002, Name: "Bob", Age: 30}},
		{StringKey("user:1003"), TestUser{ID: 1003, Name: "Charlie", Age: 35}},
	}

	t.Log("\n=== 第2步: 写入测试数据到 Redis ===")
	for _, item := range testUsers {
		err := cache.Set(ctx, item.key, item.user)
		if err != nil {
			t.Errorf("❌ Set 失败: %v", err)
			continue
		}
		redisKey := cache.redisCacheKey(item.key)
		t.Logf("✓ 已写入 Redis 键: %s", redisKey)

		// 立即验证是否真的写入了
		value, err := cache.L2RedisClient.Get(ctx, redisKey).Result()
		if err != nil {
			t.Errorf("❌ 验证失败，无法读取: %s", redisKey)
		} else {
			t.Logf("  数据内容: %s", value)
		}
	}

	t.Log("\n=== 第3步: 验证指南 ===")
	t.Log("📋 现在可以在你的 Redis 客户端运行以下命令：")
	t.Log("")
	t.Log("1️⃣  查看所有键:")
	t.Log("   KEYS manual_test:*")
	t.Log("")
	t.Log("2️⃣  查看具体数据:")
	t.Log("   GET manual_test:user:1001")
	t.Log("   GET manual_test:user:1002")
	t.Log("   GET manual_test:user:1003")
	t.Log("")
	t.Log("3️⃣  查看 TTL（剩余时间）:")
	t.Log("   TTL manual_test:user:1001")
	t.Log("")
	t.Log("4️⃣  手动清理（可选）:")
	t.Log("   DEL manual_test:user:1001 manual_test:user:1002 manual_test:user:1003")
	t.Log("")
	t.Log("⏰ 数据将在 30 分钟后自动过期")

	t.Log("\n=== ✅ 测试完成，数据已保留在 Redis 中供验证 ===")
}

// TestXCache_Set_L1Only 测试仅 L1 缓存的 Set 功能
func TestXCache_Set_L1Only(t *testing.T) {
	// 创建一个简单的 directFunc（不会被调用）
	directFunc := func(ctx context.Context, key StringKey) (TestUser, error) {
		t.Log("directFunc 被调用（Set 测试中不应该被调用）")
		return TestUser{}, nil
	}

	// 创建缓存实例（仅 L1）
	cache, err := NewCacheBuilder(
		directFunc,
		WithPrefixKey("test_set_l1"),
		WithL1Cache(true, 1000, 5*time.Minute),
	)
	if err != nil {
		t.Fatalf("创建缓存失败: %v", err)
	}

	ctx := context.Background()
	key := StringKey("user:100")
	expectedUser := TestUser{
		ID:   100,
		Name: "Alice",
		Age:  30,
	}

	// 测试 Set 方法
	err = cache.Set(ctx, key, expectedUser)
	if err != nil {
		t.Fatalf("Set 失败: %v", err)
	}
	t.Log("✓ Set 方法执行成功")

	// 验证是否可以 Get 到
	actualUser, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get 失败: %v", err)
	}

	// 验证数据一致性
	if actualUser.ID != expectedUser.ID {
		t.Errorf("ID 不匹配: 期望 %d, 实际 %d", expectedUser.ID, actualUser.ID)
	}
	if actualUser.Name != expectedUser.Name {
		t.Errorf("Name 不匹配: 期望 %s, 实际 %s", expectedUser.Name, actualUser.Name)
	}
	if actualUser.Age != expectedUser.Age {
		t.Errorf("Age 不匹配: 期望 %d, 实际 %d", expectedUser.Age, actualUser.Age)
	}

	t.Logf("✓ Get 获取到的数据正确: %+v", actualUser)

	// 检查缓存统计
	stats := cache.L1CacheClient.Stats()
	if stats.Hits() != 1 {
		t.Errorf("L1 缓存应该命中 1 次，实际命中 %d 次", stats.Hits())
	}
	t.Logf("✓ L1 缓存命中统计正确: 命中=%d, 未命中=%d", stats.Hits(), stats.Misses())
}

// TestXCache_Set_L2Redis 测试 L2 Redis 缓存的 Set 功能
func TestXCache_Set_L2Redis(t *testing.T) {
	directFunc := func(ctx context.Context, key StringKey) (TestUser, error) {
		t.Log("directFunc 被调用（Set 测试中不应该被调用）")
		return TestUser{}, nil
	}

	ctx := context.Background()

	// 先测试 Redis 连接
	t.Log("=== 第1步: 测试 Redis 连接 ===")
	redisClient := redis.NewClient(&redis.Options{
		Addr: "127.0.0.1:6379",
		DB:   0,
	})
	pingResult, err := redisClient.Ping(ctx).Result()
	if err != nil {
		t.Fatalf("❌ Redis 连接失败: %v (请确保 Redis 已启动: redis-server)", err)
	}
	t.Logf("✓ Redis 连接成功: %s", pingResult)

	// 创建缓存实例（L1 + L2 Redis）
	t.Log("\n=== 第2步: 创建缓存实例 ===")
	cache, err := NewCacheBuilder(
		directFunc,
		WithPrefixKey("test_set_l2"),
		WithL1Cache(true, 1000, 3*time.Second), // L1 3秒过期
		WithL2Cache(true, &redis.Options{
			Addr: "127.0.0.1:6379",
			DB:   0,
		}, 10*time.Second), // L2 10秒过期
	)
	if err != nil {
		t.Fatalf("❌ 创建缓存失败: %v", err)
	}
	t.Log("✓ 缓存实例创建成功")

	key := StringKey("user:200")
	expectedUser := TestUser{
		ID:   200,
		Name: "Bob",
		Age:  35,
	}

	// 清理 Redis 中可能存在的旧数据
	t.Log("\n=== 第3步: 清理旧数据 ===")
	redisKey := cache.redisCacheKey(key)
	t.Logf("Redis 完整键名: %s", redisKey)
	cache.L2RedisClient.Del(ctx, redisKey)

	// 测试 Set 方法
	t.Log("\n=== 第4步: 执行 Set 操作 ===")
	err = cache.Set(ctx, key, expectedUser)
	if err != nil {
		t.Fatalf("❌ Set 失败: %v", err)
	}
	t.Log("✓ Set 方法执行成功")

	// 立即检查 Redis 中是否有数据
	t.Log("\n=== 第5步: 立即检查 Redis 数据 ===")
	redisValueImmediate, err := cache.L2RedisClient.Get(ctx, redisKey).Result()
	if err != nil {
		t.Logf("⚠️  警告: Set 后立即读取 Redis 失败: %v", err)
		t.Logf("   可能原因: 1) 写入失败 2) 键名不匹配")

		// 列出所有键来调试
		allKeys, _ := cache.L2RedisClient.Keys(ctx, "*").Result()
		t.Logf("   Redis 中所有的键: %v", allKeys)
	} else {
		t.Logf("✓ Redis 中的数据（立即读取）: %s", redisValueImmediate)
	}

	// 立即从 Get 获取（应该从 L1 获取）
	t.Log("\n=== 第6步: 通过 Get 方法获取（应该命中 L1）===")
	user1, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("❌ 第一次 Get 失败: %v", err)
	}
	t.Logf("✓ 第一次 Get（来自 L1）: %+v", user1)

	// 验证立即获取的数据
	if user1.ID != expectedUser.ID || user1.Name != expectedUser.Name || user1.Age != expectedUser.Age {
		t.Errorf("❌ 从 L1 获取的数据不一致: 期望 %+v, 实际 %+v", expectedUser, user1)
	}

	// 等待 L1 过期
	t.Log("\n=== 第7步: 等待 L1 过期（4秒）===")
	time.Sleep(4 * time.Second)

	// 再次 Get（应该从 L2 Redis 获取）
	t.Log("\n=== 第8步: L1 过期后获取（应该从 L2 获取）===")
	user2, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("❌ 第二次 Get 失败: %v", err)
	}
	t.Logf("✓ 第二次 Get（来自 L2 Redis）: %+v", user2)

	// 验证数据一致性
	if user2.ID != expectedUser.ID || user2.Name != expectedUser.Name || user2.Age != expectedUser.Age {
		t.Errorf("❌ 从 L2 获取的数据不一致: 期望 %+v, 实际 %+v", expectedUser, user2)
	}

	// 最终验证 Redis 中确实有数据
	t.Log("\n=== 第9步: 最终验证 Redis 数据 ===")
	redisValue, err := cache.L2RedisClient.Get(ctx, redisKey).Result()
	if err != nil {
		t.Fatalf("❌ 直接从 Redis 读取失败: %v", err)
	}
	t.Logf("✓ Redis 中的最终数据: %s", redisValue)

	// 清理测试数据
	t.Log("\n=== 第10步: 清理测试数据 ===")
	cache.L2RedisClient.Del(ctx, redisKey)
	t.Log("✓ 清理完成")
}

// TestXCache_Set_MultipleValues 测试设置多个值
func TestXCache_Set_MultipleValues(t *testing.T) {
	directFunc := func(ctx context.Context, key StringKey) (TestUser, error) {
		return TestUser{}, nil
	}

	cache, err := NewCacheBuilder(
		directFunc,
		WithPrefixKey("test_set_multi"),
		WithL1Cache(true, 1000, 5*time.Minute),
		WithL2Cache(true, &redis.Options{
			Addr: "127.0.0.1:6379",
			DB:   0,
		}, 10*time.Minute),
	)
	if err != nil {
		t.Fatalf("创建缓存失败: %v", err)
	}

	ctx := context.Background()

	// 准备测试数据
	testUsers := []struct {
		key  StringKey
		user TestUser
	}{
		{StringKey("user:301"), TestUser{ID: 301, Name: "Charlie", Age: 25}},
		{StringKey("user:302"), TestUser{ID: 302, Name: "David", Age: 28}},
		{StringKey("user:303"), TestUser{ID: 303, Name: "Eve", Age: 32}},
	}

	// 批量 Set
	for _, item := range testUsers {
		err := cache.Set(ctx, item.key, item.user)
		if err != nil {
			t.Fatalf("Set 失败 (key=%s): %v", item.key, err)
		}
		t.Logf("✓ Set 成功: %s -> %+v", item.key, item.user)
	}

	// 验证所有数据都可以 Get 到
	for _, item := range testUsers {
		actualUser, err := cache.Get(ctx, item.key)
		if err != nil {
			t.Fatalf("Get 失败 (key=%s): %v", item.key, err)
		}

		if actualUser != item.user {
			t.Errorf("数据不匹配 (key=%s): 期望 %+v, 实际 %+v",
				item.key, item.user, actualUser)
		}
		t.Logf("✓ Get 验证成功: %s -> %+v", item.key, actualUser)
	}

	// 清理测试数据
	for _, item := range testUsers {
		cache.L2RedisClient.Del(ctx, cache.redisCacheKey(item.key))
	}
	t.Log("✓ 清理所有测试数据完成")
}

// TestXCache_Set_UpdateValue 测试更新已存在的值
func TestXCache_Set_UpdateValue(t *testing.T) {
	directFunc := func(ctx context.Context, key StringKey) (TestUser, error) {
		return TestUser{}, nil
	}

	cache, err := NewCacheBuilder(
		directFunc,
		WithPrefixKey("test_set_update"),
		WithL1Cache(true, 1000, 5*time.Minute),
	)
	if err != nil {
		t.Fatalf("创建缓存失败: %v", err)
	}

	ctx := context.Background()
	key := StringKey("user:400")

	// 第一次 Set
	user1 := TestUser{ID: 400, Name: "Frank", Age: 40}
	err = cache.Set(ctx, key, user1)
	if err != nil {
		t.Fatalf("第一次 Set 失败: %v", err)
	}
	t.Logf("✓ 第一次 Set: %+v", user1)

	// 验证第一次的值
	actual1, _ := cache.Get(ctx, key)
	if actual1 != user1 {
		t.Errorf("第一次获取的值不正确: 期望 %+v, 实际 %+v", user1, actual1)
	}

	// 第二次 Set（更新）
	user2 := TestUser{ID: 400, Name: "Frank_Updated", Age: 41}
	err = cache.Set(ctx, key, user2)
	if err != nil {
		t.Fatalf("第二次 Set 失败: %v", err)
	}
	t.Logf("✓ 第二次 Set（更新）: %+v", user2)

	// 验证值已更新
	actual2, _ := cache.Get(ctx, key)
	if actual2 != user2 {
		t.Errorf("更新后的值不正确: 期望 %+v, 实际 %+v", user2, actual2)
	}
	t.Log("✓ 值更新成功")
}

// TestXCache_Set_StringType 测试 Set 方法支持字符串类型
func TestXCache_Set_StringType(t *testing.T) {
	directFunc := func(ctx context.Context, key StringKey) (string, error) {
		return "", nil
	}

	cache, err := NewCacheBuilder(
		directFunc,
		WithPrefixKey("test_set_string"),
		WithL1Cache(true, 1000, 5*time.Minute),
		WithL2Cache(true, &redis.Options{
			Addr: "127.0.0.1:6379",
			DB:   0,
		}, 10*time.Minute),
	)
	if err != nil {
		t.Fatalf("创建缓存失败: %v", err)
	}

	ctx := context.Background()
	key := StringKey("config:app_name")
	expectedValue := "MyAwesomeApp"

	// Set 字符串值
	err = cache.Set(ctx, key, expectedValue)
	if err != nil {
		t.Fatalf("Set 字符串失败: %v", err)
	}
	t.Logf("✓ Set 字符串成功: %s", expectedValue)

	// Get 验证
	actualValue, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get 字符串失败: %v", err)
	}

	if actualValue != expectedValue {
		t.Errorf("字符串值不匹配: 期望 %s, 实际 %s", expectedValue, actualValue)
	}
	t.Logf("✓ Get 字符串成功: %s", actualValue)

	// 清理
	cache.L2RedisClient.Del(ctx, cache.redisCacheKey(key))
}

// TestXCache_Set_ComplexStruct 测试 Set 方法支持复杂结构体
func TestXCache_Set_ComplexStruct(t *testing.T) {
	type ComplexData struct {
		ID        int                    `json:"id"`
		Name      string                 `json:"name"`
		Tags      []string               `json:"tags"`
		Metadata  map[string]interface{} `json:"metadata"`
		CreatedAt time.Time              `json:"created_at"`
	}

	directFunc := func(ctx context.Context, key StringKey) (ComplexData, error) {
		return ComplexData{}, nil
	}

	cache, err := NewCacheBuilder(
		directFunc,
		WithPrefixKey("test_set_complex"),
		WithL1Cache(true, 1000, 5*time.Minute),
		WithL2Cache(true, &redis.Options{
			Addr: "127.0.0.1:6379",
			DB:   0,
		}, 10*time.Minute),
	)
	if err != nil {
		t.Fatalf("创建缓存失败: %v", err)
	}

	ctx := context.Background()
	key := StringKey("data:complex:1")

	expectedData := ComplexData{
		ID:   1,
		Name: "ComplexItem",
		Tags: []string{"tag1", "tag2", "tag3"},
		Metadata: map[string]interface{}{
			"version": 1.0,
			"enabled": true,
			"count":   float64(100), // JSON 会将数字转为 float64
		},
		CreatedAt: time.Now().Truncate(time.Second), // 去除纳秒精度
	}

	// Set 复杂结构
	err = cache.Set(ctx, key, expectedData)
	if err != nil {
		t.Fatalf("Set 复杂结构失败: %v", err)
	}
	t.Logf("✓ Set 复杂结构成功")

	// 等待一下确保异步写入完成
	time.Sleep(100 * time.Millisecond)

	// Get 验证
	actualData, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get 复杂结构失败: %v", err)
	}

	// 验证主要字段
	if actualData.ID != expectedData.ID {
		t.Errorf("ID 不匹配: 期望 %d, 实际 %d", expectedData.ID, actualData.ID)
	}
	if actualData.Name != expectedData.Name {
		t.Errorf("Name 不匹配: 期望 %s, 实际 %s", expectedData.Name, actualData.Name)
	}
	if len(actualData.Tags) != len(expectedData.Tags) {
		t.Errorf("Tags 长度不匹配: 期望 %d, 实际 %d", len(expectedData.Tags), len(actualData.Tags))
	}

	t.Logf("✓ Get 复杂结构成功: %+v", actualData)

	// 清理
	cache.L2RedisClient.Del(ctx, cache.redisCacheKey(key))
}
