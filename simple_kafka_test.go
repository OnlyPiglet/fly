package main

import (
	"fmt"
	"log"
	"time"

	"github.com/IBM/sarama"
)

func main() {
	fmt.Println("=== Simple Kafka Connection Test ===")

	// 直接使用 Sarama 测试连接
	config := sarama.NewConfig()
	config.Version = sarama.V2_8_0_0
	config.ClientID = "simple-test-client"
	
	// 网络配置
	config.Net.DialTimeout = 10 * time.Second
	config.Net.ReadTimeout = 10 * time.Second
	config.Net.WriteTimeout = 10 * time.Second
	config.Metadata.Timeout = 10 * time.Second
	config.Metadata.Retry.Max = 3
	config.Metadata.Retry.Backoff = 250 * time.Millisecond
	
	// Producer配置
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true

	brokers := []string{
		"127.0.0.1:19091",
		"127.0.0.1:19092",
		"127.0.0.1:19093",
	}

	fmt.Printf("Testing brokers: %v\n", brokers)

	// 1. 测试创建 Producer
	fmt.Println("\n=== Testing Producer Creation ===")
	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		log.Printf("❌ Failed to create producer: %v", err)
		return
	}
	defer producer.Close()
	fmt.Println("✅ Producer created successfully")

	// 2. 测试创建 ClusterAdmin
	fmt.Println("\n=== Testing ClusterAdmin Creation ===")
	admin, err := sarama.NewClusterAdmin(brokers, config)
	if err != nil {
		log.Printf("❌ Failed to create cluster admin: %v", err)
		return
	}
	defer admin.Close()
	fmt.Println("✅ ClusterAdmin created successfully")

	// 3. 测试获取集群信息
	fmt.Println("\n=== Testing Cluster Metadata ===")
	brokerList, controllerID, err := admin.DescribeCluster()
	if err != nil {
		log.Printf("❌ Failed to describe cluster: %v", err)
		return
	}
	
	fmt.Printf("✅ Cluster info retrieved successfully\n")
	fmt.Printf("  Controller ID: %d\n", controllerID)
	fmt.Printf("  Brokers found: %d\n", len(brokerList))
	for i, broker := range brokerList {
		fmt.Printf("    Broker %d: ID=%d, Addr=%s\n", i+1, broker.ID(), broker.Addr())
	}

	// 4. 测试列出 topics
	fmt.Println("\n=== Testing List Topics ===")
	topics, err := admin.ListTopics()
	if err != nil {
		log.Printf("❌ Failed to list topics: %v", err)
		return
	}
	
	fmt.Printf("✅ Topics listed successfully: %d topics found\n", len(topics))
	for name, detail := range topics {
		fmt.Printf("  - %s: %d partitions, replication factor %d\n", 
			name, detail.NumPartitions, detail.ReplicationFactor)
	}

	// 5. 测试发送消息
	fmt.Println("\n=== Testing Message Production ===")
	testTopic := "simple-test-topic"
	
	// 先创建topic（如果不存在）
	exists := false
	if _, found := topics[testTopic]; found {
		exists = true
		fmt.Printf("Topic '%s' already exists\n", testTopic)
	} else {
		fmt.Printf("Creating topic '%s'...\n", testTopic)
		topicDetail := &sarama.TopicDetail{
			NumPartitions:     1,
			ReplicationFactor: int16(len(brokerList)),
		}
		err = admin.CreateTopic(testTopic, topicDetail, false)
		if err != nil {
			log.Printf("❌ Failed to create topic: %v", err)
			return
		}
		fmt.Printf("✅ Topic '%s' created successfully\n", testTopic)
		exists = true
	}

	if exists {
		// 发送测试消息
		message := &sarama.ProducerMessage{
			Topic: testTopic,
			Value: sarama.StringEncoder(fmt.Sprintf("Test message at %s", time.Now().Format(time.RFC3339))),
		}

		partition, offset, err := producer.SendMessage(message)
		if err != nil {
			log.Printf("❌ Failed to send message: %v", err)
			return
		}
		
		fmt.Printf("✅ Message sent successfully to partition %d, offset %d\n", partition, offset)
		
		// 清理测试topic
		fmt.Printf("Cleaning up test topic...\n")
		err = admin.DeleteTopic(testTopic)
		if err != nil {
			log.Printf("⚠️  Failed to delete test topic: %v", err)
		} else {
			fmt.Printf("✅ Test topic deleted successfully\n")
		}
	}

	fmt.Println("\n🎉 All tests completed successfully!")
}
