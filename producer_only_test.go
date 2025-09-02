package main

import (
	"fmt"
	"time"

	"github.com/IBM/sarama"
)

func main() {
	fmt.Println("=== Producer Only Test ===")

	// 直接使用 Sarama Producer，不依赖 ClusterAdmin
	config := sarama.NewConfig()
	config.Version = sarama.V3_7_0_0
	config.ClientID = "producer-only-test"
	
	// 网络配置
	config.Net.DialTimeout = 10 * time.Second
	config.Net.ReadTimeout = 10 * time.Second
	config.Net.WriteTimeout = 10 * time.Second
	config.Metadata.Timeout = 10 * time.Second
	config.Metadata.RefreshFrequency = 0 // 禁用元数据刷新
	
	// Producer配置
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true
	config.Producer.Retry.Max = 3

	brokers := []string{
		"127.0.0.1:19091",
		"127.0.0.1:19092",
		"127.0.0.1:19093",
	}

	fmt.Printf("Creating producer with brokers: %v\n", brokers)

	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		fmt.Printf("❌ Failed to create producer: %v\n", err)
		return
	}
	defer producer.Close()

	fmt.Println("✅ Producer created successfully")

	// 发送测试消息
	testTopic := "producer-only-test-topic"
	message := &sarama.ProducerMessage{
		Topic: testTopic,
		Key:   sarama.StringEncoder("test-key"),
		Value: sarama.StringEncoder(fmt.Sprintf("Producer test message at %s", time.Now().Format(time.RFC3339))),
		Headers: []sarama.RecordHeader{
			{Key: []byte("source"), Value: []byte("producer-only-test")},
			{Key: []byte("timestamp"), Value: []byte(time.Now().Format(time.RFC3339))},
		},
	}

	fmt.Printf("Sending message to topic '%s'...\n", testTopic)
	partition, offset, err := producer.SendMessage(message)
	if err != nil {
		fmt.Printf("❌ Failed to send message: %v\n", err)
		return
	}

	fmt.Printf("✅ Message sent successfully!\n")
	fmt.Printf("  Topic: %s\n", testTopic)
	fmt.Printf("  Partition: %d\n", partition)
	fmt.Printf("  Offset: %d\n", offset)

	// 发送批量消息
	fmt.Println("\n=== Testing batch messages ===")
	messages := []*sarama.ProducerMessage{
		{
			Topic: testTopic,
			Key:   sarama.StringEncoder("batch-1"),
			Value: sarama.StringEncoder("Batch message 1"),
		},
		{
			Topic: testTopic,
			Key:   sarama.StringEncoder("batch-2"),
			Value: sarama.StringEncoder("Batch message 2"),
		},
		{
			Topic: testTopic,
			Key:   sarama.StringEncoder("batch-3"),
			Value: sarama.StringEncoder("Batch message 3"),
		},
	}

	err = producer.SendMessages(messages)
	if err != nil {
		fmt.Printf("❌ Failed to send batch messages: %v\n", err)
		return
	}

	fmt.Printf("✅ Batch of %d messages sent successfully!\n", len(messages))

	fmt.Println("\n🎉 Producer-only test completed successfully!")
	fmt.Println("Note: Topic auto-creation depends on Kafka server configuration")
}
