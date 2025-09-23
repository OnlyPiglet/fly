package pooltools

import (
	"bytes"
	"strings"
	"sync"
	"testing"
)

// 测试基本的Get和Put功能
func TestPool_BasicUsage(t *testing.T) {
	// 创建一个字符串池
	stringPool := New("string-pool", func() string {
		return "default"
	})

	// 获取对象
	str := stringPool.Get()
	if str != "default" {
		t.Errorf("Expected 'default', got '%s'", str)
	}

	// 归还对象
	stringPool.Put("reused")

	// 再次获取，应该得到归还的对象
	str2 := stringPool.Get()
	if str2 != "reused" {
		t.Errorf("Expected 'reused', got '%s'", str2)
	}
}

// 测试bytes.Buffer池
func TestPool_BytesBuffer(t *testing.T) {
	bufferPool := New("buffer-pool", func() *bytes.Buffer {
		return &bytes.Buffer{}
	})

	// 获取buffer
	buf := bufferPool.Get()
	buf.WriteString("hello world")

	// 使用Reset方法重置并归还
	bufferPool.Reset(buf, func(b *bytes.Buffer) {
		b.Reset()
	})

	// 再次获取，应该是空的buffer
	buf2 := bufferPool.Get()
	if buf2.Len() != 0 {
		t.Errorf("Expected empty buffer, got length %d", buf2.Len())
	}
}

// 测试结构体池
type TestStruct struct {
	Name  string
	Count int
}

func TestPool_Struct(t *testing.T) {
	structPool := New("struct-pool", func() *TestStruct {
		return &TestStruct{}
	})

	// 获取结构体
	obj := structPool.Get()
	obj.Name = "test"
	obj.Count = 42

	// 归还前重置
	structPool.Reset(obj, func(s *TestStruct) {
		s.Name = ""
		s.Count = 0
	})

	// 再次获取
	obj2 := structPool.Get()
	if obj2.Name != "" || obj2.Count != 0 {
		t.Errorf("Expected reset struct, got Name='%s', Count=%d", obj2.Name, obj2.Count)
	}
}

// 测试并发安全性
func TestPool_Concurrent(t *testing.T) {
	stringPool := New("concurrent-pool", func() string {
		return "new"
	})

	var wg sync.WaitGroup
	numGoroutines := 100
	numOperations := 1000

	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				// 获取对象
				str := stringPool.Get()
				
				// 模拟使用
				_ = strings.ToUpper(str)
				
				// 归还对象
				stringPool.Put(str)
			}
		}(i)
	}

	wg.Wait()
	// 如果没有panic，说明并发安全
}

// 基准测试：对比直接创建vs池化
func BenchmarkDirectAllocation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := &bytes.Buffer{}
		buf.WriteString("benchmark test")
		buf.Reset()
	}
}

func BenchmarkPoolAllocation(b *testing.B) {
	bufferPool := New("buffer-pool", func() *bytes.Buffer {
		return &bytes.Buffer{}
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := bufferPool.Get()
		buf.WriteString("benchmark test")
		bufferPool.Reset(buf, func(b *bytes.Buffer) {
			b.Reset()
		})
	}
}

// 测试不同类型的池
func TestPool_DifferentTypes(t *testing.T) {
	// int池
	intPool := New("int-pool", func() int {
		return 0
	})
	
	num := intPool.Get()
	if num != 0 {
		t.Errorf("Expected 0, got %d", num)
	}
	intPool.Put(42)
	
	num2 := intPool.Get()
	if num2 != 42 {
		t.Errorf("Expected 42, got %d", num2)
	}

	// slice池
	slicePool := New("slice-pool", func() []string {
		return make([]string, 0, 10)
	})
	
	slice := slicePool.Get()
	slice = append(slice, "test")
	
	slicePool.Reset(slice, func(s []string) {
		// 重置slice长度但保留容量
		s = s[:0]
	})
}

// 测试监控功能
func TestPool_Monitoring(t *testing.T) {
	stringPool := New("monitoring-pool", func() string {
		return "new"
	})

	// 初始状态检查
	if stringPool.GetCount() != 0 {
		t.Errorf("Expected GetCount 0, got %d", stringPool.GetCount())
	}
	if stringPool.PutCount() != 0 {
		t.Errorf("Expected PutCount 0, got %d", stringPool.PutCount())
	}
	if stringPool.NewCount() != 0 {
		t.Errorf("Expected NewCount 0, got %d", stringPool.NewCount())
	}

	// 第一次获取（会触发新建）
	str1 := stringPool.Get()
	if stringPool.GetCount() != 1 {
		t.Errorf("Expected GetCount 1, got %d", stringPool.GetCount())
	}
	if stringPool.NewCount() != 1 {
		t.Errorf("Expected NewCount 1, got %d", stringPool.NewCount())
	}

	// 归还对象
	stringPool.Put(str1)
	if stringPool.PutCount() != 1 {
		t.Errorf("Expected PutCount 1, got %d", stringPool.PutCount())
	}
	// PoolSize是估算值，可能因为sync.Pool的内部机制而不准确
	poolSize := stringPool.PoolSize()
	t.Logf("PoolSize after put: %d (estimated)", poolSize)

	// 再次获取（应该从池中获取，不会新建）
	str2 := stringPool.Get()
	if stringPool.GetCount() != 2 {
		t.Errorf("Expected GetCount 2, got %d", stringPool.GetCount())
	}
	if stringPool.NewCount() != 1 {
		t.Errorf("Expected NewCount still 1, got %d", stringPool.NewCount())
	}

	// 检查命中率
	hitRate := stringPool.HitRate()
	expectedHitRate := 0.5 // 2次获取，1次命中
	if hitRate != expectedHitRate {
		t.Errorf("Expected HitRate %.2f, got %.2f", expectedHitRate, hitRate)
	}

	// 测试Stats方法
	stats := stringPool.Stats()
	if stats.GetCount != 2 {
		t.Errorf("Expected Stats.GetCount 2, got %d", stats.GetCount)
	}
	if stats.PutCount != 1 {
		t.Errorf("Expected Stats.PutCount 1, got %d", stats.PutCount)
	}
	if stats.NewCount != 1 {
		t.Errorf("Expected Stats.NewCount 1, got %d", stats.NewCount)
	}
	if stats.HitRate != expectedHitRate {
		t.Errorf("Expected Stats.HitRate %.2f, got %.2f", expectedHitRate, stats.HitRate)
	}

	// 测试重置统计
	stringPool.ResetStats()
	if stringPool.GetCount() != 0 {
		t.Errorf("Expected GetCount 0 after reset, got %d", stringPool.GetCount())
	}
	if stringPool.PutCount() != 0 {
		t.Errorf("Expected PutCount 0 after reset, got %d", stringPool.PutCount())
	}
	if stringPool.NewCount() != 0 {
		t.Errorf("Expected NewCount 0 after reset, got %d", stringPool.NewCount())
	}

	// 归还之前获取的对象
	stringPool.Put(str2)
}

// 测试高命中率场景
func TestPool_HighHitRate(t *testing.T) {
	bufferPool := New("buffer-pool", func() *bytes.Buffer {
		return &bytes.Buffer{}
	})

	// 先放入一些对象
	for i := 0; i < 5; i++ {
		buf := &bytes.Buffer{}
		bufferPool.Put(buf)
	}

	// 重置统计，只统计后续操作
	bufferPool.ResetStats()

	// 多次获取和归还
	for i := 0; i < 10; i++ {
		buf := bufferPool.Get()
		buf.WriteString("test")
		bufferPool.Reset(buf, func(b *bytes.Buffer) {
			b.Reset()
		})
	}

	// 检查命中率应该很高（接近100%）
	hitRate := bufferPool.HitRate()
	if hitRate < 0.8 { // 至少80%命中率
		t.Errorf("Expected high hit rate (>0.8), got %.2f", hitRate)
	}

	stats := bufferPool.Stats()
	t.Logf("High hit rate test - Gets: %d, News: %d, Hit Rate: %.2f%%", 
		stats.GetCount, stats.NewCount, stats.HitRate*100)
}

// 测试并发监控
func TestPool_ConcurrentMonitoring(t *testing.T) {
	stringPool := New("concurrent-monitoring-pool", func() string {
		return "concurrent"
	})

	var wg sync.WaitGroup
	numGoroutines := 10
	numOperations := 100

	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				str := stringPool.Get()
				stringPool.Put(str)
			}
		}()
	}

	wg.Wait()

	// 检查统计数据的一致性
	expectedOps := int64(numGoroutines * numOperations)
	if stringPool.GetCount() != expectedOps {
		t.Errorf("Expected GetCount %d, got %d", expectedOps, stringPool.GetCount())
	}
	if stringPool.PutCount() != expectedOps {
		t.Errorf("Expected PutCount %d, got %d", expectedOps, stringPool.PutCount())
	}

	stats := stringPool.Stats()
	t.Logf("Concurrent test - Gets: %d, Puts: %d, News: %d, Hit Rate: %.2f%%", 
		stats.GetCount, stats.PutCount, stats.NewCount, stats.HitRate*100)
}

// 测试池名称功能
func TestPool_PoolName(t *testing.T) {
	// 创建多个不同名称的池
	stringPool := New("user-cache", func() string {
		return "default"
	})
	
	bufferPool := New("buffer-cache", func() *bytes.Buffer {
		return &bytes.Buffer{}
	})
	
	intPool := New("number-cache", func() int {
		return 0
	})
	
	// 测试Name()方法
	if stringPool.Name() != "user-cache" {
		t.Errorf("Expected pool name 'user-cache', got '%s'", stringPool.Name())
	}
	
	if bufferPool.Name() != "buffer-cache" {
		t.Errorf("Expected pool name 'buffer-cache', got '%s'", bufferPool.Name())
	}
	
	if intPool.Name() != "number-cache" {
		t.Errorf("Expected pool name 'number-cache', got '%s'", intPool.Name())
	}
	
	// 测试Stats中的池名称
	stringPool.Get()
	stringStats := stringPool.Stats()
	if stringStats.PoolName != "user-cache" {
		t.Errorf("Expected stats pool name 'user-cache', got '%s'", stringStats.PoolName)
	}
	
	bufferPool.Get()
	bufferStats := bufferPool.Stats()
	if bufferStats.PoolName != "buffer-cache" {
		t.Errorf("Expected stats pool name 'buffer-cache', got '%s'", bufferStats.PoolName)
	}
	
	// 验证不同池的独立性
	if stringStats.PoolName == bufferStats.PoolName {
		t.Error("Different pools should have different names")
	}
	
	t.Logf("Pool names: %s, %s, %s", 
		stringPool.Name(), bufferPool.Name(), intPool.Name())
}
