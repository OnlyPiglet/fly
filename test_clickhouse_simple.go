package main

import (
	"fmt"
	"time"

	"github.com/OnlyPiglet/fly/clickhousetools"
)

// TestEvent 测试事件结构体
type TestEvent struct {
	ID        uint64    `ch:"id"`
	Name      string    `ch:"name"`
	Value     float64   `ch:"value"`
	Status    int32     `ch:"status"`
	Timestamp time.Time `ch:"timestamp"`
	Tags      []string  `ch:"tags"`
}

func main() {
	fmt.Println("=== ClickHouse Simple Test ===")

	// 配置 ClickHouse 客户端，包含表初始化
	config := &clickhousetools.ClickHouseConfig{
		Addresses: []string{"127.0.0.1:9000"},
		Username:  "default",
		Password:  "",
		Database:  "test_simple_db",
		InitTables: []string{
			`CREATE TABLE IF NOT EXISTS test_events (
				id UInt64,
				name String,
				value Float64,
				status Int32,
				timestamp DateTime,
				tags Array(String)
			) ENGINE = MergeTree()
			ORDER BY (id, timestamp)`,
		},
	}

	fmt.Printf("Connecting to ClickHouse: %v\n", config.Addresses)

	client, err := clickhousetools.NewClickHouseClient(config)
	if err != nil {
		fmt.Printf("❌ Failed to create ClickHouse client: %v\n", err)
		return
	}
	defer client.Close()

	fmt.Println("✅ ClickHouse client created successfully!")

	// 准备测试数据
	events := []TestEvent{
		{
			ID:        1,
			Name:      "simple_test_event_1",
			Value:     123.45,
			Status:    1,
			Timestamp: time.Now(),
			Tags:      []string{"test", "simple"},
		},
		{
			ID:        2,
			Name:      "simple_test_event_2",
			Value:     678.90,
			Status:    2,
			Timestamp: time.Now().Add(time.Minute),
			Tags:      []string{"test", "batch"},
		},
		{
			ID:        3,
			Name:      "simple_test_event_3",
			Value:     999.99,
			Status:    0,
			Timestamp: time.Now().Add(2 * time.Minute),
			Tags:      []string{"test", "final"},
		},
	}

	fmt.Printf("Inserting %d events...\n", len(events))

	// 测试基本插入
	err = clickhousetools.AddData(client, "test_events", events)
	if err != nil {
		fmt.Printf("❌ Failed to insert events: %v\n", err)
		return
	}

	fmt.Printf("✅ Successfully inserted %d events\n", len(events))

	// 测试带选项的插入
	fmt.Println("\n=== Testing with options ===")
	
	moreEvents := []TestEvent{
		{
			ID:        4,
			Name:      "options_test_event_1",
			Value:     111.11,
			Status:    1,
			Timestamp: time.Now().Add(3 * time.Minute),
			Tags:      []string{"options", "test"},
		},
		{
			ID:        5,
			Name:      "options_test_event_2",
			Value:     222.22,
			Status:    2,
			Timestamp: time.Now().Add(4 * time.Minute),
			Tags:      []string{"options", "batch"},
		},
	}

	options := clickhousetools.WithBatchOptions{
		BlockBufferSize: 16,
		Timeout:         30 * time.Second,
		AsyncInsert:     false,
		Settings: map[string]interface{}{
			"max_insert_block_size": 1000,
		},
	}

	err = clickhousetools.AddData(client, "test_events", moreEvents, options)
	if err != nil {
		fmt.Printf("❌ Failed to insert events with options: %v\n", err)
		return
	}

	fmt.Printf("✅ Successfully inserted %d events with options\n", len(moreEvents))

	// 测试空切片
	fmt.Println("\n=== Testing empty slice ===")
	var emptyEvents []TestEvent
	err = clickhousetools.AddData(client, "test_events", emptyEvents)
	if err != nil {
		fmt.Printf("❌ Failed to handle empty slice: %v\n", err)
		return
	}

	fmt.Println("✅ Empty slice handled successfully")

	fmt.Println("\n🎉 All ClickHouse tests completed successfully!")
}
