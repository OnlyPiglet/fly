package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/OnlyPiglet/fly/kafkatools"
)

// Event 测试事件结构体
type Event struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Timestamp time.Time `json:"timestamp"`
	UserID    int64     `json:"user_id"`
}

func main() {
	fmt.Println("=== Kafka Single Broker Test ===")

	// 尝试使用内部 Docker 地址（从错误信息中获得）
	config := &kafkatools.KafkaConfig{
		Brokers: []string{
			"172.21.52.178:19091",
			"172.21.52.178:19092", 
			"172.21.52.178:19093",
		},
		ClientID:     "docker-internal-test-client",
		KafkaVersion: "3.7.1", // 根据你的 APP_VERSION
	}

	fmt.Printf("Connecting to Kafka broker: %v\n", config.Brokers)

	client, err := kafkatools.NewKafkaClient(config)
	if err != nil {
		fmt.Printf("❌ Failed to create kafka client: %v\n", err)
		return
	}
	defer client.Close()

	fmt.Println("✅ Kafka client created successfully")

	// 1. 测试发送单条消息（自动创建topic）
	fmt.Println("\n=== Testing ProduceMessage with auto-create ===")
	testTopic := "single-test-topic"
	message := []byte(fmt.Sprintf(`{"test": "single broker test", "timestamp": "%s"}`, time.Now().Format(time.RFC3339)))

	err = client.ProduceMessage(testTopic, message, kafkatools.ProduceOptions{
		Key: "test-key",
		Headers: map[string]string{
			"source": "single-test",
			"type":   "test-message",
		},
	})
	if err != nil {
		fmt.Printf("❌ Failed to produce message: %v\n", err)
		return
	}
	fmt.Println("✅ Single message sent successfully")

	// 2. 验证topic是否被创建
	fmt.Println("\n=== Verifying topic creation ===")
	exists, err := client.TopicExists(testTopic)
	if err != nil {
		fmt.Printf("❌ Failed to check topic existence: %v\n", err)
		return
	}
	
	if exists {
		fmt.Printf("✅ Topic '%s' was auto-created\n", testTopic)
		
		// 获取topic详情
		topics, err := client.ListTopics()
		if err == nil {
			if detail, found := topics[testTopic]; found {
				fmt.Printf("  Topic details: %d partitions, replication factor %d\n", 
					detail.NumPartitions, detail.ReplicationFactor)
			}
		}
	} else {
		fmt.Printf("❌ Topic '%s' was not created\n", testTopic)
	}

	// 3. 测试批量发送（自动创建topic）
	fmt.Println("\n=== Testing ProduceBatch with auto-create ===")
	batchTopic := "batch-test-topic"
	
	events := []Event{
		{ID: 1, Name: "user_login", Timestamp: time.Now(), UserID: 100},
		{ID: 2, Name: "page_view", Timestamp: time.Now(), UserID: 101},
		{ID: 3, Name: "purchase", Timestamp: time.Now(), UserID: 102},
	}

	serializer := func(event Event) ([]byte, error) {
		return json.Marshal(event)
	}

	err = kafkatools.ProduceBatch(client, batchTopic, events, serializer, kafkatools.ProduceOptions{
		Headers: map[string]string{
			"source": "batch-test",
			"count":  fmt.Sprintf("%d", len(events)),
		},
	})
	if err != nil {
		fmt.Printf("❌ Failed to produce batch: %v\n", err)
		return
	}
	fmt.Printf("✅ Batch of %d events sent successfully\n", len(events))

	// 4. 列出所有topics
	fmt.Println("\n=== Listing all topics ===")
	topics, err := client.ListTopics()
	if err != nil {
		fmt.Printf("❌ Failed to list topics: %v\n", err)
		return
	}

	fmt.Printf("✅ Found %d topics:\n", len(topics))
	for name, detail := range topics {
		fmt.Printf("  - %s: %d partitions, replication factor %d\n", 
			name, detail.NumPartitions, detail.ReplicationFactor)
	}

	// 5. 清理测试topics
	fmt.Println("\n=== Cleaning up test topics ===")
	err = client.DeleteTopics([]string{testTopic, batchTopic})
	if err != nil {
		fmt.Printf("⚠️  Failed to cleanup topics: %v\n", err)
	} else {
		fmt.Println("✅ Test topics cleaned up successfully")
	}

	fmt.Println("\n🎉 Single broker test completed successfully!")
}
