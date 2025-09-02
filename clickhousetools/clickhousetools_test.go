package clickhousetools

import (
	"fmt"
	"testing"
	"time"
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

// TestUser 测试用户结构体
type TestUser struct {
	UserID   uint64 `ch:"user_id"`
	Username string `ch:"username"`
	Email    string `ch:"email"`
	Age      uint8  `ch:"age"`
	IsActive bool   `ch:"is_active"`
}

// TestPartitionEvent 测试分区事件结构体
type TestPartitionEvent struct {
	ID          uint64    `ch:"id"`
	EventType   string    `ch:"event_type"`
	UserID      uint64    `ch:"user_id"`
	EventData   string    `ch:"event_data"`
	Amount      float64   `ch:"amount"`
	EventDate   time.Time `ch:"event_date"`   // 用于按日期分区
	EventMonth  string    `ch:"event_month"`  // 用于按月分区 (YYYY-MM)
	CreatedAt   time.Time `ch:"created_at"`
}

func TestClickHouseClient_Connection(t *testing.T) {
	config := &ClickHouseConfig{
		Addresses: []string{"127.0.0.1:9000"},
		Username:  "default",
		Password:  "",
		Database:  "test_db",
	}

	client, err := NewClickHouseClient(config)
	if err != nil {
		t.Logf("Failed to create ClickHouse client (expected if no ClickHouse running): %v", err)
		return
	}
	defer client.Close()

	t.Log("ClickHouse client created successfully")
}

func TestClickHouseClient_CreateDatabase(t *testing.T) {
	config := &ClickHouseConfig{
		Addresses: []string{"127.0.0.1:9000"},
		Username:  "default",
		Password:  "",
		Database:  "test_clickhouse_db",
	}

	client, err := NewClickHouseClient(config)
	if err != nil {
		t.Logf("Failed to create ClickHouse client (expected if no ClickHouse running): %v", err)
		return
	}
	defer client.Close()

	t.Log("Database created/verified successfully")
}

func TestClickHouseClient_InitTables(t *testing.T) {
	// 定义测试表的创建语句
	initTables := []string{
		`CREATE TABLE IF NOT EXISTS test_events (
			id UInt64,
			name String,
			value Float64,
			status Int32,
			timestamp DateTime,
			tags Array(String)
		) ENGINE = MergeTree()
		ORDER BY (id, timestamp)`,
		
		`CREATE TABLE IF NOT EXISTS test_users (
			user_id UInt64,
			username String,
			email String,
			age UInt8,
			is_active Bool
		) ENGINE = MergeTree()
		ORDER BY user_id`,
	}

	config := &ClickHouseConfig{
		Addresses:  []string{"127.0.0.1:9000"},
		Username:   "default",
		Password:   "",
		Database:   "test_clickhouse_db",
		InitTables: initTables,
	}

	client, err := NewClickHouseClient(config)
	if err != nil {
		t.Logf("Failed to create ClickHouse client (expected if no ClickHouse running): %v", err)
		return
	}
	defer client.Close()

	t.Log("Tables created/verified successfully")
}

func TestAddData_Events(t *testing.T) {
	config := &ClickHouseConfig{
		Addresses: []string{"127.0.0.1:9000"},
		Username:  "default",
		Password:  "",
		Database:  "test_clickhouse_db",
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

	client, err := NewClickHouseClient(config)
	if err != nil {
		t.Logf("Failed to create ClickHouse client (expected if no ClickHouse running): %v", err)
		return
	}
	defer client.Close()

	// 准备测试数据
	events := []TestEvent{
		{
			ID:        1,
			Name:      "test_event_1",
			Value:     123.45,
			Status:    1,
			Timestamp: time.Now(),
			Tags:      []string{"tag1", "tag2"},
		},
		{
			ID:        2,
			Name:      "test_event_2",
			Value:     678.90,
			Status:    2,
			Timestamp: time.Now().Add(time.Minute),
			Tags:      []string{"tag3", "tag4", "tag5"},
		},
		{
			ID:        3,
			Name:      "test_event_3",
			Value:     999.99,
			Status:    0,
			Timestamp: time.Now().Add(2 * time.Minute),
			Tags:      []string{"important", "urgent"},
		},
	}

	// 测试基本插入
	err = AddData(client, "test_events", events)
	if err != nil {
		t.Fatalf("Failed to insert events: %v", err)
	}

	t.Logf("Successfully inserted %d events", len(events))
}

func TestAddData_WithOptions(t *testing.T) {
	config := &ClickHouseConfig{
		Addresses: []string{"127.0.0.1:9000"},
		Username:  "default",
		Password:  "",
		Database:  "test_clickhouse_db",
		InitTables: []string{
			`CREATE TABLE IF NOT EXISTS test_users (
				user_id UInt64,
				username String,
				email String,
				age UInt8,
				is_active Bool
			) ENGINE = MergeTree()
			ORDER BY user_id`,
		},
	}

	client, err := NewClickHouseClient(config)
	if err != nil {
		t.Logf("Failed to create ClickHouse client (expected if no ClickHouse running): %v", err)
		return
	}
	defer client.Close()

	// 准备测试数据
	users := []TestUser{
		{UserID: 1, Username: "alice", Email: "alice@example.com", Age: 25, IsActive: true},
		{UserID: 2, Username: "bob", Email: "bob@example.com", Age: 30, IsActive: true},
		{UserID: 3, Username: "charlie", Email: "charlie@example.com", Age: 35, IsActive: false},
		{UserID: 4, Username: "diana", Email: "diana@example.com", Age: 28, IsActive: true},
		{UserID: 5, Username: "eve", Email: "eve@example.com", Age: 32, IsActive: false},
	}

	// 测试带选项的插入
	options := WithBatchOptions{
		BlockBufferSize: 16,
		Timeout:         30 * time.Second,
		AsyncInsert:     false,
		Settings: map[string]interface{}{
			"max_insert_block_size": 1000,
		},
	}

	err = AddData(client, "test_users", users, options)
	if err != nil {
		t.Fatalf("Failed to insert users with options: %v", err)
	}

	t.Logf("Successfully inserted %d users with options", len(users))
}

func TestAddData_AsyncInsert(t *testing.T) {
	config := &ClickHouseConfig{
		Addresses: []string{"127.0.0.1:9000"},
		Username:  "default",
		Password:  "",
		Database:  "test_clickhouse_db",
		InitTables: []string{
			`CREATE TABLE IF NOT EXISTS test_events_async (
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

	client, err := NewClickHouseClient(config)
	if err != nil {
		t.Logf("Failed to create ClickHouse client (expected if no ClickHouse running): %v", err)
		return
	}
	defer client.Close()

	// 准备大量测试数据
	events := make([]TestEvent, 100)
	for i := 0; i < 100; i++ {
		events[i] = TestEvent{
			ID:        uint64(i + 1),
			Name:      fmt.Sprintf("async_event_%d", i+1),
			Value:     float64(i) * 1.5,
			Status:    int32(i % 3),
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
			Tags:      []string{fmt.Sprintf("tag_%d", i), "async"},
		}
	}

	// 测试异步插入
	options := WithBatchOptions{
		BlockBufferSize:   32,
		Timeout:           1 * time.Minute,
		AsyncInsert:       true,
		WaitAsyncInsert:   true, // 等待异步插入完成
		Settings: map[string]interface{}{
			"async_insert_max_data_size": "1000000",
			"async_insert_busy_timeout_ms": 200,
		},
	}

	err = AddData(client, "test_events_async", events, options)
	if err != nil {
		t.Fatalf("Failed to insert events with async: %v", err)
	}

	t.Logf("Successfully inserted %d events with async insert", len(events))
}

func TestAddData_EmptySlice(t *testing.T) {
	config := &ClickHouseConfig{
		Addresses: []string{"127.0.0.1:9000"},
		Username:  "default",
		Password:  "",
		Database:  "test_clickhouse_db",
	}

	client, err := NewClickHouseClient(config)
	if err != nil {
		t.Logf("Failed to create ClickHouse client (expected if no ClickHouse running): %v", err)
		return
	}
	defer client.Close()

	// 测试空切片
	var emptyEvents []TestEvent
	err = AddData(client, "test_events", emptyEvents)
	if err != nil {
		t.Fatalf("Failed to handle empty slice: %v", err)
	}

	t.Log("Empty slice handled successfully")
}

func TestAddData_LargeDataset(t *testing.T) {
	config := &ClickHouseConfig{
		Addresses: []string{"127.0.0.1:9000"},
		Username:  "default",
		Password:  "",
		Database:  "test_clickhouse_db",
		InitTables: []string{
			`CREATE TABLE IF NOT EXISTS test_large_events (
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

	client, err := NewClickHouseClient(config)
	if err != nil {
		t.Logf("Failed to create ClickHouse client (expected if no ClickHouse running): %v", err)
		return
	}
	defer client.Close()

	// 准备大量数据（10000条记录）
	const recordCount = 10000
	events := make([]TestEvent, recordCount)
	
	for i := 0; i < recordCount; i++ {
		events[i] = TestEvent{
			ID:        uint64(i + 1),
			Name:      fmt.Sprintf("large_event_%d", i+1),
			Value:     float64(i) * 0.123,
			Status:    int32(i % 5),
			Timestamp: time.Now().Add(time.Duration(i) * time.Millisecond),
			Tags:      []string{fmt.Sprintf("batch_%d", i/1000), fmt.Sprintf("item_%d", i)},
		}
	}

	start := time.Now()

	// 测试大数据集插入
	options := WithBatchOptions{
		BlockBufferSize: 255, // uint8 最大值
		Timeout:         5 * time.Minute,
		AsyncInsert:     false,
		Settings: map[string]interface{}{
			"max_insert_block_size": 10000,
		},
	}

	err = AddData(client, "test_large_events", events, options)
	if err != nil {
		t.Fatalf("Failed to insert large dataset: %v", err)
	}

	duration := time.Since(start)
	t.Logf("Successfully inserted %d records in %v (%.2f records/sec)", 
		recordCount, duration, float64(recordCount)/duration.Seconds())
}

func TestAddData_PartitionTable_ByDate(t *testing.T) {
	config := &ClickHouseConfig{
		Addresses: []string{"127.0.0.1:9000"},
		Username:  "default",
		Password:  "",
		Database:  "test_clickhouse_db",
		InitTables: []string{
			`CREATE TABLE IF NOT EXISTS test_partition_events_by_date (
				id UInt64,
				event_type String,
				user_id UInt64,
				event_data String,
				amount Float64,
				event_date Date,
				event_month String,
				created_at DateTime
			) ENGINE = MergeTree()
			PARTITION BY toYYYYMM(event_date)
			ORDER BY (event_type, user_id, id)
			SETTINGS index_granularity = 8192`,
		},
	}

	client, err := NewClickHouseClient(config)
	if err != nil {
		t.Logf("Failed to create ClickHouse client (expected if no ClickHouse running): %v", err)
		return
	}
	defer client.Close()

	// 准备跨多个月的测试数据
	baseTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	events := []TestPartitionEvent{
		// 2024年1月的数据
		{
			ID:          1,
			EventType:   "login",
			UserID:      1001,
			EventData:   "web_login",
			Amount:      0.0,
			EventDate:   baseTime,
			EventMonth:  "2024-01",
			CreatedAt:   baseTime,
		},
		{
			ID:          2,
			EventType:   "purchase",
			UserID:      1001,
			EventData:   "product_123",
			Amount:      99.99,
			EventDate:   baseTime.AddDate(0, 0, 5),
			EventMonth:  "2024-01",
			CreatedAt:   baseTime.AddDate(0, 0, 5),
		},
		// 2024年2月的数据
		{
			ID:          3,
			EventType:   "login",
			UserID:      1002,
			EventData:   "mobile_login",
			Amount:      0.0,
			EventDate:   baseTime.AddDate(0, 1, 0),
			EventMonth:  "2024-02",
			CreatedAt:   baseTime.AddDate(0, 1, 0),
		},
		{
			ID:          4,
			EventType:   "purchase",
			UserID:      1002,
			EventData:   "product_456",
			Amount:      149.99,
			EventDate:   baseTime.AddDate(0, 1, 10),
			EventMonth:  "2024-02",
			CreatedAt:   baseTime.AddDate(0, 1, 10),
		},
		// 2024年3月的数据
		{
			ID:          5,
			EventType:   "refund",
			UserID:      1001,
			EventData:   "product_123",
			Amount:      -99.99,
			EventDate:   baseTime.AddDate(0, 2, 0),
			EventMonth:  "2024-03",
			CreatedAt:   baseTime.AddDate(0, 2, 0),
		},
	}

	// 测试分区表插入
	err = AddData(client, "test_partition_events_by_date", events)
	if err != nil {
		t.Fatalf("Failed to insert partition events: %v", err)
	}

	t.Logf("Successfully inserted %d events into partitioned table (by date)", len(events))
}

func TestAddData_PartitionTable_ByEventType(t *testing.T) {
	config := &ClickHouseConfig{
		Addresses: []string{"127.0.0.1:9000"},
		Username:  "default",
		Password:  "",
		Database:  "test_clickhouse_db",
		InitTables: []string{
			`CREATE TABLE IF NOT EXISTS test_partition_events_by_type (
				id UInt64,
				event_type String,
				user_id UInt64,
				event_data String,
				amount Float64,
				event_date Date,
				event_month String,
				created_at DateTime
			) ENGINE = MergeTree()
			PARTITION BY event_type
			ORDER BY (user_id, event_date, id)
			SETTINGS index_granularity = 8192`,
		},
	}

	client, err := NewClickHouseClient(config)
	if err != nil {
		t.Logf("Failed to create ClickHouse client (expected if no ClickHouse running): %v", err)
		return
	}
	defer client.Close()

	// 准备不同事件类型的测试数据
	baseTime := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	events := []TestPartitionEvent{
		// login 事件
		{
			ID:          101,
			EventType:   "login",
			UserID:      2001,
			EventData:   "successful_login",
			Amount:      0.0,
			EventDate:   baseTime,
			EventMonth:  "2024-06",
			CreatedAt:   baseTime,
		},
		{
			ID:          102,
			EventType:   "login",
			UserID:      2002,
			EventData:   "failed_login",
			Amount:      0.0,
			EventDate:   baseTime.Add(time.Hour),
			EventMonth:  "2024-06",
			CreatedAt:   baseTime.Add(time.Hour),
		},
		// purchase 事件
		{
			ID:          103,
			EventType:   "purchase",
			UserID:      2001,
			EventData:   "premium_subscription",
			Amount:      29.99,
			EventDate:   baseTime.Add(2 * time.Hour),
			EventMonth:  "2024-06",
			CreatedAt:   baseTime.Add(2 * time.Hour),
		},
		{
			ID:          104,
			EventType:   "purchase",
			UserID:      2003,
			EventData:   "one_time_purchase",
			Amount:      9.99,
			EventDate:   baseTime.Add(3 * time.Hour),
			EventMonth:  "2024-06",
			CreatedAt:   baseTime.Add(3 * time.Hour),
		},
		// logout 事件
		{
			ID:          105,
			EventType:   "logout",
			UserID:      2001,
			EventData:   "normal_logout",
			Amount:      0.0,
			EventDate:   baseTime.Add(4 * time.Hour),
			EventMonth:  "2024-06",
			CreatedAt:   baseTime.Add(4 * time.Hour),
		},
		{
			ID:          106,
			EventType:   "logout",
			UserID:      2002,
			EventData:   "timeout_logout",
			Amount:      0.0,
			EventDate:   baseTime.Add(5 * time.Hour),
			EventMonth:  "2024-06",
			CreatedAt:   baseTime.Add(5 * time.Hour),
		},
	}

	// 测试按事件类型分区的表插入
	err = AddData(client, "test_partition_events_by_type", events)
	if err != nil {
		t.Fatalf("Failed to insert events into type-partitioned table: %v", err)
	}

	t.Logf("Successfully inserted %d events into partitioned table (by event type)", len(events))
}

func TestAddData_PartitionTable_LargeDataset(t *testing.T) {
	config := &ClickHouseConfig{
		Addresses: []string{"127.0.0.1:9000"},
		Username:  "default",
		Password:  "",
		Database:  "test_clickhouse_db",
		InitTables: []string{
			`CREATE TABLE IF NOT EXISTS test_partition_large_events (
				id UInt64,
				event_type String,
				user_id UInt64,
				event_data String,
				amount Float64,
				event_date Date,
				event_month String,
				created_at DateTime
			) ENGINE = MergeTree()
			PARTITION BY (event_type, toYYYYMM(event_date))
			ORDER BY (user_id, event_date, id)
			SETTINGS index_granularity = 8192`,
		},
	}

	client, err := NewClickHouseClient(config)
	if err != nil {
		t.Logf("Failed to create ClickHouse client (expected if no ClickHouse running): %v", err)
		return
	}
	defer client.Close()

	// 生成大量跨多个分区的数据
	const recordCount = 50000
	events := make([]TestPartitionEvent, recordCount)
	
	eventTypes := []string{"login", "logout", "purchase", "refund", "view", "click"}
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	for i := 0; i < recordCount; i++ {
		// 随机分布到不同的月份和事件类型
		monthOffset := i % 12 // 分布到12个月
		eventTypeIndex := i % len(eventTypes)
		dayOffset := i % 28 // 避免月份天数问题
		
		eventTime := baseTime.AddDate(0, monthOffset, dayOffset).Add(time.Duration(i%24) * time.Hour)
		
		events[i] = TestPartitionEvent{
			ID:          uint64(i + 1),
			EventType:   eventTypes[eventTypeIndex],
			UserID:      uint64(1000 + (i % 5000)), // 5000个不同用户
			EventData:   fmt.Sprintf("data_%d", i),
			Amount:      float64(i%1000) * 0.99,
			EventDate:   eventTime,
			EventMonth:  eventTime.Format("2006-01"),
			CreatedAt:   eventTime,
		}
	}

	start := time.Now()

	// 测试大数据集分区表插入
	options := WithBatchOptions{
		BlockBufferSize: 255, // 最大缓冲区
		Timeout:         10 * time.Minute,
		AsyncInsert:     false,
		Settings: map[string]interface{}{
			"max_insert_block_size": 50000,
		},
	}

	err = AddData(client, "test_partition_large_events", events, options)
	if err != nil {
		t.Fatalf("Failed to insert large dataset into partitioned table: %v", err)
	}

	duration := time.Since(start)
	t.Logf("Successfully inserted %d records into partitioned table in %v (%.2f records/sec)", 
		recordCount, duration, float64(recordCount)/duration.Seconds())
	t.Logf("Data distributed across %d months and %d event types", 12, len(eventTypes))
}

// 示例：演示完整的 ClickHouse 使用流程
func ExampleClickHouseClient_usage() {
	// 配置 ClickHouse 客户端
	config := &ClickHouseConfig{
		Addresses: []string{"127.0.0.1:9000"},
		Username:  "default",
		Password:  "",
		Database:  "example_db",
		InitTables: []string{
			`CREATE TABLE IF NOT EXISTS user_events (
				user_id UInt64,
				event_type String,
				event_data String,
				created_at DateTime
			) ENGINE = MergeTree()
			ORDER BY (user_id, created_at)`,
		},
	}

	// 创建客户端
	client, err := NewClickHouseClient(config)
	if err != nil {
		fmt.Printf("Failed to create ClickHouse client: %v\n", err)
		return
	}
	defer client.Close()

	// 定义事件结构
	type UserEvent struct {
		UserID    uint64    `ch:"user_id"`
		EventType string    `ch:"event_type"`
		EventData string    `ch:"event_data"`
		CreatedAt time.Time `ch:"created_at"`
	}

	// 准备数据
	events := []UserEvent{
		{UserID: 1, EventType: "login", EventData: "web", CreatedAt: time.Now()},
		{UserID: 1, EventType: "click", EventData: "button_a", CreatedAt: time.Now()},
		{UserID: 2, EventType: "login", EventData: "mobile", CreatedAt: time.Now()},
	}

	// 插入数据
	err = AddData(client, "user_events", events, WithBatchOptions{
		BlockBufferSize: 100,
		Timeout:         30 * time.Second,
	})
	if err != nil {
		fmt.Printf("Failed to insert events: %v\n", err)
		return
	}

	fmt.Printf("Successfully inserted %d events\n", len(events))
}
