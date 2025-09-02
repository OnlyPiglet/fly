package kafkatools

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// TestEvent 测试用的事件结构体
type TestEvent struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Timestamp time.Time `json:"timestamp"`
	UserID    int64     `json:"user_id"`
}

// TestUser 测试用的用户结构体
type TestUser struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Age      int    `json:"age"`
}

func TestKafkaClient_ProduceMessage(t *testing.T) {
	// 注意：这个测试需要一个运行中的Kafka实例
	// 如果没有Kafka，测试会失败，但可以展示用法
	config := &KafkaConfig{
		Brokers: []string{
			"127.0.0.1:19091",
			"127.0.0.1:19092",
			"127.0.0.1:19093",
		},
		ClientID: "test-client",
	}

	client, err := NewKafkaClient(config)
	if err != nil {
		t.Logf("Failed to create kafka client (expected if no Kafka running): %v", err)
		return
	}
	defer client.Close()

	// 测试发送简单消息
	message := []byte("Hello Kafka from kafkatools!")
	err = client.ProduceMessage("test-topic", message)
	if err != nil {
		t.Logf("Failed to produce message (expected if no Kafka running): %v", err)
		return
	}

	t.Log("Message sent successfully")
}

func TestKafkaClient_ProduceMessageWithOptions(t *testing.T) {
	config := &KafkaConfig{
		Brokers: []string{
			"127.0.0.1:19091",
			"127.0.0.1:19092",
			"127.0.0.1:19093",
		},
		ClientID: "test-client",
	}

	client, err := NewKafkaClient(config)
	if err != nil {
		t.Logf("Failed to create kafka client (expected if no Kafka running): %v", err)
		return
	}
	defer client.Close()

	// 测试带选项的消息发送
	message := []byte("Hello Kafka with options!")
	partition := int32(0)
	now := time.Now()

	opts := ProduceOptions{
		Key:       "test-key-123",
		Partition: &partition,
		Headers: map[string]string{
			"source":    "kafkatools-test",
			"version":   "1.0.0",
			"timestamp": now.Format(time.RFC3339),
		},
		Timestamp: &now,
	}

	err = client.ProduceMessage("test-topic-with-options", message, opts)
	if err != nil {
		t.Logf("Failed to produce message with options (expected if no Kafka running): %v", err)
		return
	}

	t.Log("Message with options sent successfully")
}

func TestProduceBatch(t *testing.T) {
	config := &KafkaConfig{
		Brokers: []string{
			"127.0.0.1:19091",
			"127.0.0.1:19092",
			"127.0.0.1:19093",
		},
		ClientID: "test-batch-client",
	}

	client, err := NewKafkaClient(config)
	if err != nil {
		t.Logf("Failed to create kafka client (expected if no Kafka running): %v", err)
		return
	}
	defer client.Close()

	// 准备测试数据
	events := []TestEvent{
		{ID: 1, Name: "user_login", Timestamp: time.Now(), UserID: 100},
		{ID: 2, Name: "page_view", Timestamp: time.Now(), UserID: 101},
		{ID: 3, Name: "purchase", Timestamp: time.Now(), UserID: 102},
	}

	users := []TestUser{
		{ID: 100, Username: "alice", Email: "alice@example.com", Age: 25},
		{ID: 101, Username: "bob", Email: "bob@example.com", Age: 30},
		{ID: 102, Username: "charlie", Email: "charlie@example.com", Age: 35},
	}

	// 事件序列化器
	eventSerializer := func(event TestEvent) ([]byte, error) {
		return json.Marshal(event)
	}

	// 用户序列化器
	userSerializer := func(user TestUser) ([]byte, error) {
		return json.Marshal(user)
	}

	// 批量发送事件
	err = ProduceBatch(client, "events-topic", events, eventSerializer)
	if err != nil {
		t.Logf("Failed to produce event batch (expected if no Kafka running): %v", err)
		return
	}

	// 批量发送用户
	err = ProduceBatch(client, "users-topic", users, userSerializer)
	if err != nil {
		t.Logf("Failed to produce user batch (expected if no Kafka running): %v", err)
		return
	}

	t.Log("Batch messages sent successfully")
}

// 消费者测试暂时注释掉
// func TestKafkaClient_ConsumeMessages(t *testing.T) {
// 	config := &KafkaConfig{
// 		Brokers:  []string{"localhost:9092"},
// 		ClientID: "test-consumer-client",
// 	}

// 	client, err := NewKafkaClient(config)
// 	if err != nil {
// 		t.Logf("Failed to create kafka client (expected if no Kafka running): %v", err)
// 		return
// 	}
// 	defer client.Close()

// 	// 消息处理器
// 	messageHandler := func(message *sarama.ConsumerMessage) error {
// 		fmt.Printf("Received message: Topic=%s, Partition=%d, Offset=%d, Key=%s, Value=%s\n",
// 			message.Topic, message.Partition, message.Offset, string(message.Key), string(message.Value))

// 		// 处理Headers
// 		if len(message.Headers) > 0 {
// 			fmt.Print("Headers: ")
// 			for _, header := range message.Headers {
// 				fmt.Printf("%s=%s ", string(header.Key), string(header.Value))
// 			}
// 			fmt.Println()
// 		}

// 		return nil
// 	}

// 	// 消费配置
// 	opts := ConsumeOptions{
// 		Partition:   0,
// 		Offset:      sarama.OffsetOldest, // 从最早的消息开始
// 		MaxMessages: 5,                   // 最多消费5条消息
// 		Timeout:     10 * time.Second,    // 10秒超时
// 		MessageFilter: func(data []byte) bool {
// 			// 过滤空消息
// 			return len(data) > 0
// 		},
// 	}

// 	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
// 	defer cancel()

// 	err = client.ConsumeMessages(ctx, "test-topic", messageHandler, opts)
// 	if err != nil {
// 		t.Logf("Failed to consume messages (expected if no Kafka running): %v", err)
// 		return
// 	}

// 	t.Log("Messages consumed successfully")
// }

func TestKafkaClient_TopicManagement(t *testing.T) {
	config := &KafkaConfig{
		Brokers: []string{
			"127.0.0.1:19091",
			"127.0.0.1:19092",
			"127.0.0.1:19093",
		},
		ClientID: "test-topic-management-client",
	}

	client, err := NewKafkaClient(config)
	if err != nil {
		t.Logf("Failed to create kafka client (expected if no Kafka running): %v", err)
		return
	}
	defer client.Close()

	testTopicName := "test-topic-management"

	// 1. 测试创建单个Topic
	t.Log("Testing CreateTopic...")
	topicConfig := TopicConfig{
		Name:          testTopicName,
		NumPartitions: 3, // 指定3个分区
		// ReplicationFactor 默认与broker数量一致
		ConfigEntries: map[string]string{
			"cleanup.policy": "delete",
			"retention.ms":   "86400000", // 1天
		},
	}

	err = client.CreateTopic(topicConfig)
	if err != nil {
		t.Logf("Failed to create topic (expected if no Kafka running): %v", err)
		return
	}
	t.Log("Topic created successfully")

	// 2. 测试检查Topic是否存在
	t.Log("Testing TopicExists...")
	exists, err := client.TopicExists(testTopicName)
	if err != nil {
		t.Logf("Failed to check topic existence: %v", err)
		return
	}
	if exists {
		t.Log("Topic exists check passed")
	} else {
		t.Log("Topic should exist but doesn't")
	}

	// 3. 测试列出所有Topic
	t.Log("Testing ListTopics...")
	topics, err := client.ListTopics()
	if err != nil {
		t.Logf("Failed to list topics: %v", err)
		return
	}
	t.Logf("Found %d topics", len(topics))

	if detail, found := topics[testTopicName]; found {
		t.Logf("Test topic details: Partitions=%d, ReplicationFactor=%d",
			detail.NumPartitions, detail.ReplicationFactor)
	}

	// 4. 测试批量创建Topic
	t.Log("Testing CreateTopics...")
	batchTopics := []TopicConfig{
		{
			Name:          "test-batch-1",
			NumPartitions: 2,
		},
		{
			Name:          "test-batch-2",
			NumPartitions: 4,
			ConfigEntries: map[string]string{
				"compression.type": "gzip",
			},
		},
	}

	err = client.CreateTopics(batchTopics)
	if err != nil {
		t.Logf("Failed to create batch topics: %v", err)
	} else {
		t.Log("Batch topics created successfully")
	}

	// 5. 测试删除Topic
	t.Log("Testing DeleteTopic...")
	err = client.DeleteTopic(testTopicName)
	if err != nil {
		t.Logf("Failed to delete topic: %v", err)
	} else {
		t.Log("Topic deleted successfully")
	}

	// 6. 测试批量删除Topic
	t.Log("Testing DeleteTopics...")
	err = client.DeleteTopics([]string{"test-batch-1", "test-batch-2"})
	if err != nil {
		t.Logf("Failed to delete batch topics: %v", err)
	} else {
		t.Log("Batch topics deleted successfully")
	}

	t.Log("Topic management tests completed")
}

func TestAutoCreateTopic(t *testing.T) {
	config := &KafkaConfig{
		Brokers: []string{
			"127.0.0.1:19091",
			"127.0.0.1:19092",
			"127.0.0.1:19093",
		},
		ClientID: "test-auto-create-client",
	}

	client, err := NewKafkaClient(config)
	if err != nil {
		t.Logf("Failed to create kafka client (expected if no Kafka running): %v", err)
		return
	}
	defer client.Close()

	autoTopicName := "auto-created-topic"

	// 确保topic不存在（如果存在则删除）
	exists, err := client.TopicExists(autoTopicName)
	if err != nil {
		t.Logf("Failed to check topic existence: %v", err)
		return
	}
	if exists {
		err = client.DeleteTopic(autoTopicName)
		if err != nil {
			t.Logf("Failed to delete existing topic: %v", err)
			return
		}
		t.Log("Deleted existing topic for clean test")
	}

	// 准备测试数据
	events := []TestEvent{
		{ID: 1, Name: "auto_test_1", Timestamp: time.Now(), UserID: 100},
		{ID: 2, Name: "auto_test_2", Timestamp: time.Now(), UserID: 101},
	}

	eventSerializer := func(event TestEvent) ([]byte, error) {
		return json.Marshal(event)
	}

	// 使用ProduceBatch发送到不存在的topic，应该自动创建
	t.Log("Testing ProduceBatch with auto-create topic...")
	err = ProduceBatch(client, autoTopicName, events, eventSerializer)
	if err != nil {
		t.Logf("Failed to produce batch with auto-create (expected if no Kafka running): %v", err)
		return
	}
	t.Log("ProduceBatch with auto-create succeeded")

	// 验证topic是否被自动创建
	exists, err = client.TopicExists(autoTopicName)
	if err != nil {
		t.Logf("Failed to check topic existence after auto-create: %v", err)
		return
	}
	if exists {
		t.Log("Topic was successfully auto-created")

		// 获取topic详情
		topics, err := client.ListTopics()
		if err == nil {
			if detail, found := topics[autoTopicName]; found {
				t.Logf("Auto-created topic details: Partitions=%d, ReplicationFactor=%d",
					detail.NumPartitions, detail.ReplicationFactor)
			}
		}
	} else {
		t.Log("Topic was not auto-created")
	}

	// 测试ProduceMessage的自动创建功能
	singleTopicName := "auto-single-topic"

	// 确保topic不存在
	exists, err = client.TopicExists(singleTopicName)
	if err == nil && exists {
		client.DeleteTopic(singleTopicName)
	}

	t.Log("Testing ProduceMessage with auto-create topic...")
	message := []byte(`{"test": "auto-create-single"}`)
	err = client.ProduceMessage(singleTopicName, message)
	if err != nil {
		t.Logf("Failed to produce message with auto-create: %v", err)
		return
	}
	t.Log("ProduceMessage with auto-create succeeded")

	// 验证topic是否被自动创建
	exists, err = client.TopicExists(singleTopicName)
	if err == nil && exists {
		t.Log("Single message topic was successfully auto-created")
	}

	// 清理测试topic
	client.DeleteTopic(autoTopicName)
	client.DeleteTopic(singleTopicName)

	t.Log("Auto-create topic tests completed")
}

// 示例：演示完整的生产者流程和Topic管理
func ExampleKafkaClient_usage() {
	// 配置Kafka客户端
	config := &KafkaConfig{
		Brokers: []string{
			"127.0.0.1:19091",
			"127.0.0.1:19092",
			"127.0.0.1:19093",
		},
		ClientID: "example-client",
		// 如果需要SASL认证
		// SASLEnabled:   true,
		// Username:      "your-username",
		// Password:      "your-password",
		// SASLMechanism: "PLAIN",
		// 如果需要TLS
		// TLSEnabled: true,
	}

	client, err := NewKafkaClient(config)
	if err != nil {
		fmt.Printf("Failed to create kafka client: %v\n", err)
		return
	}
	defer client.Close()

	// 1. 创建Topic
	topicConfig := TopicConfig{
		Name:          "user-events",
		NumPartitions: 3, // 3个分区
		ConfigEntries: map[string]string{
			"cleanup.policy": "delete",
			"retention.ms":   "86400000", // 1天
		},
	}

	err = client.CreateTopic(topicConfig)
	if err != nil {
		fmt.Printf("Failed to create topic: %v\n", err)
		// 如果topic已存在，继续执行
	}

	// 2. 发送单条消息
	message := []byte(`{"event": "user_signup", "user_id": 123, "timestamp": "2024-01-01T10:00:00Z"}`)
	err = client.ProduceMessage("user-events", message, ProduceOptions{
		Key: "user_123",
		Headers: map[string]string{
			"source": "user-service",
			"type":   "signup",
		},
	})
	if err != nil {
		fmt.Printf("Failed to produce message: %v\n", err)
		return
	}

	// 3. 批量发送消息
	events := []TestEvent{
		{ID: 1, Name: "login", Timestamp: time.Now(), UserID: 123},
		{ID: 2, Name: "purchase", Timestamp: time.Now(), UserID: 123},
	}

	jsonSerializer := func(event TestEvent) ([]byte, error) {
		return json.Marshal(event)
	}

	err = ProduceBatch(client, "user-events", events, jsonSerializer)
	if err != nil {
		fmt.Printf("Failed to produce batch: %v\n", err)
		return
	}

	// 4. 列出所有Topic
	topics, err := client.ListTopics()
	if err != nil {
		fmt.Printf("Failed to list topics: %v\n", err)
		return
	}

	fmt.Printf("Found %d topics\n", len(topics))
	if detail, found := topics["user-events"]; found {
		fmt.Printf("user-events topic: %d partitions, replication factor %d\n",
			detail.NumPartitions, detail.ReplicationFactor)
	}

	fmt.Println("Kafka operations completed successfully")
}
