package main

import (
	"fmt"
	"time"

	"github.com/OnlyPiglet/fly/kafkatools"
)

func main() {
	fmt.Println("=== Kafka Connection Debug ===")
	
	// 测试连接配置
	config := &kafkatools.KafkaConfig{
		Brokers: []string{
			"127.0.0.1:19091",
			"127.0.0.1:19092",
			"127.0.0.1:19093",
		},
		ClientID:              "debug-client",
		ForceDirectConnection: true, // 强制使用指定的broker地址
	}

	fmt.Printf("Attempting to connect to brokers: %v\n", config.Brokers)

	// 创建客户端
	client, err := kafkatools.NewKafkaClient(config)
	if err != nil {
		fmt.Printf("❌ Failed to create kafka client: %v\n", err)
		
		// 尝试单个broker连接
		fmt.Println("\n=== Testing individual brokers ===")
		for i, broker := range config.Brokers {
			fmt.Printf("Testing broker %d: %s\n", i+1, broker)
			singleConfig := &kafkatools.KafkaConfig{
				Brokers:  []string{broker},
				ClientID: fmt.Sprintf("debug-client-%d", i+1),
			}
			
			singleClient, singleErr := kafkatools.NewKafkaClient(singleConfig)
			if singleErr != nil {
				fmt.Printf("  ❌ Failed: %v\n", singleErr)
			} else {
				fmt.Printf("  ✅ Success\n")
				singleClient.Close()
			}
		}
		return
	}
	defer client.Close()

	fmt.Println("✅ Kafka client created successfully")

	// 测试列出topics
	fmt.Println("\n=== Testing ListTopics ===")
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

	// 测试发送简单消息
	fmt.Println("\n=== Testing ProduceMessage ===")
	testTopic := "debug-test-topic"
	message := []byte(fmt.Sprintf("Debug test message at %s", time.Now().Format(time.RFC3339)))
	
	err = client.ProduceMessage(testTopic, message)
	if err != nil {
		fmt.Printf("❌ Failed to produce message: %v\n", err)
		return
	}
	
	fmt.Println("✅ Message sent successfully")

	// 验证topic是否被创建
	fmt.Println("\n=== Verifying topic creation ===")
	exists, err := client.TopicExists(testTopic)
	if err != nil {
		fmt.Printf("❌ Failed to check topic existence: %v\n", err)
		return
	}
	
	if exists {
		fmt.Printf("✅ Topic '%s' was created successfully\n", testTopic)
		
		// 获取topic详情
		topics, err := client.ListTopics()
		if err == nil {
			if detail, found := topics[testTopic]; found {
				fmt.Printf("  Topic details: %d partitions, replication factor %d\n", 
					detail.NumPartitions, detail.ReplicationFactor)
			}
		}
		
		// 清理测试topic
		fmt.Println("\n=== Cleaning up ===")
		err = client.DeleteTopic(testTopic)
		if err != nil {
			fmt.Printf("⚠️  Failed to delete test topic: %v\n", err)
		} else {
			fmt.Println("✅ Test topic deleted successfully")
		}
	} else {
		fmt.Printf("❌ Topic '%s' was not created\n", testTopic)
	}

	fmt.Println("\n🎉 Debug completed!")
}
