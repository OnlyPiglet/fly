package main

import (
	"fmt"
	"time"

	"github.com/OnlyPiglet/fly/kafkatools"
)

func main() {
	fmt.Println("=== Simple Kafka Test ===")

	// 使用 127.0.0.1 地址（网络检查显示这些可以连接）
	config := &kafkatools.KafkaConfig{
		Brokers: []string{
			"127.0.0.1:19091",
			"127.0.0.1:19092",
			"127.0.0.1:19093",
		},
		ClientID:              "simple-test",
		ForceDirectConnection: true, // 强制直连避免地址重定向
		KafkaVersion:          "3.7.1",
	}

	fmt.Printf("Connecting to: %v\n", config.Brokers)

	client, err := kafkatools.NewKafkaClient(config)
	if err != nil {
		fmt.Printf("❌ Failed to create client: %v\n", err)
		return
	}
	defer client.Close()

	fmt.Println("✅ Client created successfully")

	// 测试发送消息
	testTopic := "simple-test-topic"
	message := []byte(fmt.Sprintf("Test message at %s", time.Now().Format(time.RFC3339)))

	fmt.Printf("Sending message to topic '%s'...\n", testTopic)
	err = client.ProduceMessage(testTopic, message)
	if err != nil {
		fmt.Printf("❌ Failed to send message: %v\n", err)
		return
	}

	fmt.Println("✅ Message sent successfully!")

	// 检查topic是否被创建
	exists, err := client.TopicExists(testTopic)
	if err != nil {
		fmt.Printf("⚠️  Could not check topic: %v\n", err)
	} else if exists {
		fmt.Printf("✅ Topic '%s' was auto-created\n", testTopic)
		
		// 清理
		client.DeleteTopic(testTopic)
		fmt.Println("✅ Test topic cleaned up")
	}

	fmt.Println("\n🎉 Test completed successfully!")
}
