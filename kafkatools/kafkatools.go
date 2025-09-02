package kafkatools

import (
	"fmt"
	"time"

	"github.com/IBM/sarama"
)

// KafkaConfig Kafka集群配置
type KafkaConfig struct {
	Brokers  []string `json:"brokers"`
	Username string   `json:"username,omitempty"`
	Password string   `json:"password,omitempty"`
	ClientID string   `json:"client_id,omitempty"`
	// SASL配置
	SASLEnabled   bool   `json:"sasl_enabled,omitempty"`
	SASLMechanism string `json:"sasl_mechanism,omitempty"` // PLAIN, SCRAM-SHA-256, SCRAM-SHA-512
	// TLS配置
	TLSEnabled bool `json:"tls_enabled,omitempty"`
	// 网络配置
	ForceDirectConnection bool   `json:"force_direct_connection,omitempty"` // 强制使用指定的broker地址，忽略advertised.listeners
	KafkaVersion          string `json:"kafka_version,omitempty"`           // Kafka版本，如 "2.8.0"
}

type KafkaClient struct {
	config   KafkaConfig
	producer sarama.SyncProducer
	// consumer sarama.Consumer // 暂时不需要消费者功能
	admin    sarama.ClusterAdmin // 用于管理topic
}

// initialize 初始化Kafka配置
func (k *KafkaClient) initialize() error {
	// 创建Sarama配置
	config := sarama.NewConfig()
	
	// 设置Kafka版本
	if k.config.KafkaVersion != "" {
		version, err := sarama.ParseKafkaVersion(k.config.KafkaVersion)
		if err != nil {
			return fmt.Errorf("invalid kafka version %s: %w", k.config.KafkaVersion, err)
		}
		config.Version = version
	} else {
		// 默认使用较新的版本
		config.Version = sarama.V2_8_0_0
	}
	
	// 基础配置
	if k.config.ClientID != "" {
		config.ClientID = k.config.ClientID
	} else {
		config.ClientID = "fly-kafka-client"
	}
	
	// 网络配置 - 增加超时时间和重试
	config.Net.DialTimeout = 10 * time.Second
	config.Net.ReadTimeout = 10 * time.Second
	config.Net.WriteTimeout = 10 * time.Second
	config.Metadata.Timeout = 10 * time.Second
	config.Metadata.Retry.Max = 3
	config.Metadata.Retry.Backoff = 250 * time.Millisecond
	config.Metadata.RefreshFrequency = 0 // 禁用自动刷新元数据
	
	// 如果启用强制直连，则不允许broker地址重定向
	if k.config.ForceDirectConnection {
		config.Metadata.AllowAutoTopicCreation = false
		// 设置更短的超时以快速失败
		config.Net.DialTimeout = 5 * time.Second
		config.Metadata.Timeout = 5 * time.Second
	}
	
	// 默认启用强制直连模式来避免 advertised.listeners 问题
	config.Metadata.Full = true  // 需要完整的元数据来创建 ClusterAdmin
	config.Metadata.AllowAutoTopicCreation = true
	
	// Producer配置
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Partitioner = sarama.NewRandomPartitioner
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true
	config.Producer.Retry.Max = 3
	config.Producer.Retry.Backoff = 100 * time.Millisecond
	
	// Consumer配置 (暂时注释)
	// config.Consumer.Return.Errors = true
	// config.Consumer.Offsets.Initial = sarama.OffsetNewest
	// config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin
	
	// SASL配置
	if k.config.SASLEnabled {
		config.Net.SASL.Enable = true
		config.Net.SASL.User = k.config.Username
		config.Net.SASL.Password = k.config.Password
		
		switch k.config.SASLMechanism {
		case "PLAIN":
			config.Net.SASL.Mechanism = sarama.SASLTypePlaintext
		case "SCRAM-SHA-256":
			config.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA256
		case "SCRAM-SHA-512":
			config.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
		default:
			config.Net.SASL.Mechanism = sarama.SASLTypePlaintext
		}
	}
	
	// TLS配置
	if k.config.TLSEnabled {
		config.Net.TLS.Enable = true
	}
	
	// 创建Producer
	producer, err := sarama.NewSyncProducer(k.config.Brokers, config)
	if err != nil {
		return fmt.Errorf("failed to create producer: %w", err)
	}
	k.producer = producer
	
	// 创建Consumer (暂时注释)
	// consumer, err := sarama.NewConsumer(k.config.Brokers, config)
	// if err != nil {
	// 	producer.Close()
	// 	return fmt.Errorf("failed to create consumer: %w", err)
	// }
	// k.consumer = consumer
	
	// 创建ClusterAdmin用于管理topic（可选）
	admin, err := sarama.NewClusterAdmin(k.config.Brokers, config)
	if err != nil {
		// ClusterAdmin创建失败不影响Producer功能
		fmt.Printf("Warning: failed to create cluster admin: %v\n", err)
		fmt.Println("Topic management features will be disabled, but message production will work")
		k.admin = nil
	} else {
		k.admin = admin
	}
	
	return nil
}

func NewKafkaClient(kc *KafkaConfig) (*KafkaClient, error) {
	client := &KafkaClient{
		config: *kc,
	}
	
	// 默认配置
	if len(client.config.Brokers) == 0 {
		client.config.Brokers = []string{"localhost:9092"}
	}
	
	if err := client.initialize(); err != nil {
		return nil, fmt.Errorf("initialize kafka client: %w", err)
	}
	
	return client, nil
}

// Close 关闭Kafka客户端
func (k *KafkaClient) Close() error {
	var errs []error
	
	if k.producer != nil {
		if err := k.producer.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close producer: %w", err))
		}
	}
	
	// if k.consumer != nil {
	// 	if err := k.consumer.Close(); err != nil {
	// 		errs = append(errs, fmt.Errorf("close consumer: %w", err))
	// 	}
	// }
	
	if k.admin != nil {
		if err := k.admin.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close admin: %w", err))
		}
	}
	
	if len(errs) > 0 {
		return fmt.Errorf("close kafka client: %v", errs)
	}
	
	return nil
}

// -------- 生产者功能 --------

// ProduceOptions 生产消息的可选配置
type ProduceOptions struct {
	Partition *int32            // 指定分区，nil表示自动分区
	Key       string            // 消息Key
	Headers   map[string]string // 消息Headers
	Timestamp *time.Time        // 消息时间戳
}

// ProduceMessage 发送单条消息
// 如果topic不存在会自动创建，存在则不管
func (k *KafkaClient) ProduceMessage(topic string, message []byte, opts ...ProduceOptions) error {
	// 检查并自动创建topic（如果admin可用）
	if k.admin != nil {
		if err := k.ensureTopicExists(topic); err != nil {
			// 如果topic检查失败，记录警告但继续尝试发送
			// 某些情况下Kafka会自动创建topic
			fmt.Printf("Warning: could not ensure topic exists: %v\n", err)
		}
	}
	
	var opt ProduceOptions
	if len(opts) > 0 {
		opt = opts[0]
	}
	
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(message),
	}
	
	// 设置Key
	if opt.Key != "" {
		msg.Key = sarama.StringEncoder(opt.Key)
	}
	
	// 设置分区
	if opt.Partition != nil {
		msg.Partition = *opt.Partition
	}
	
	// 设置Headers
	if len(opt.Headers) > 0 {
		headers := make([]sarama.RecordHeader, 0, len(opt.Headers))
		for k, v := range opt.Headers {
			headers = append(headers, sarama.RecordHeader{
				Key:   []byte(k),
				Value: []byte(v),
			})
		}
		msg.Headers = headers
	}
	
	// 设置时间戳
	if opt.Timestamp != nil {
		msg.Timestamp = *opt.Timestamp
	}
	
	_, _, err := k.producer.SendMessage(msg)
	return err
}

// ProduceBatch 批量发送消息（使用泛型）
// 如果topic不存在会自动创建，存在则不管
func ProduceBatch[T any](k *KafkaClient, topic string, messages []T, serializer func(T) ([]byte, error), opts ...ProduceOptions) error {
	if len(messages) == 0 {
		return nil
	}
	
	// 检查并自动创建topic（如果admin可用）
	if k.admin != nil {
		if err := k.ensureTopicExists(topic); err != nil {
			// 如果topic检查失败，记录警告但继续尝试发送
			// 某些情况下Kafka会自动创建topic
			fmt.Printf("Warning: could not ensure topic exists: %v\n", err)
		}
	}
	
	var opt ProduceOptions
	if len(opts) > 0 {
		opt = opts[0]
	}
	
	producerMessages := make([]*sarama.ProducerMessage, 0, len(messages))
	
	for _, msg := range messages {
		data, err := serializer(msg)
		if err != nil {
			return fmt.Errorf("serialize message: %w", err)
		}
		
		producerMsg := &sarama.ProducerMessage{
			Topic: topic,
			Value: sarama.ByteEncoder(data),
		}
		
		// 应用选项
		if opt.Key != "" {
			producerMsg.Key = sarama.StringEncoder(opt.Key)
		}
		if opt.Partition != nil {
			producerMsg.Partition = *opt.Partition
		}
		if len(opt.Headers) > 0 {
			headers := make([]sarama.RecordHeader, 0, len(opt.Headers))
			for k, v := range opt.Headers {
				headers = append(headers, sarama.RecordHeader{
					Key:   []byte(k),
					Value: []byte(v),
				})
			}
			producerMsg.Headers = headers
		}
		if opt.Timestamp != nil {
			producerMsg.Timestamp = *opt.Timestamp
		}
		
		producerMessages = append(producerMessages, producerMsg)
	}
	
	return k.producer.SendMessages(producerMessages)
}

// -------- Topic 管理功能 --------

// ensureTopicExists 确保topic存在，如果不存在则自动创建
func (k *KafkaClient) ensureTopicExists(topicName string) error {
	// 检查topic是否存在
	exists, err := k.TopicExists(topicName)
	if err != nil {
		return fmt.Errorf("check topic existence: %w", err)
	}
	
	// 如果topic已存在，直接返回
	if exists {
		return nil
	}
	
	// 创建默认的topic配置
	topicConfig := TopicConfig{
		Name:          topicName,
		NumPartitions: 1, // 默认1个分区
		// ReplicationFactor 会自动设置为broker数量
		ConfigEntries: map[string]string{
			"cleanup.policy": "delete",
			"retention.ms":   "604800000", // 7天
		},
	}
	
	// 创建topic
	if err := k.CreateTopic(topicConfig); err != nil {
		return fmt.Errorf("create topic: %w", err)
	}
	
	return nil
}

// convertConfigEntries 将 map[string]string 转换为 map[string]*string
func convertConfigEntries(entries map[string]string) map[string]*string {
	if entries == nil {
		return nil
	}
	
	result := make(map[string]*string)
	for k, v := range entries {
		value := v // 创建副本避免循环变量问题
		result[k] = &value
	}
	return result
}

// TopicConfig Topic创建配置
type TopicConfig struct {
	Name              string            // Topic名称
	NumPartitions     int32             // 分区数量，默认1
	ReplicationFactor int16             // 副本因子，默认与broker数量一致（自动检测）
	ConfigEntries     map[string]string // Topic配置项
}

// CreateTopic 创建Topic
func (k *KafkaClient) CreateTopic(topicConfig TopicConfig) error {
	if topicConfig.Name == "" {
		return fmt.Errorf("topic name cannot be empty")
	}
	
	// 设置默认分区数
	if topicConfig.NumPartitions <= 0 {
		topicConfig.NumPartitions = 1
	}
	
	// 如果没有指定副本因子，自动检测broker数量
	if topicConfig.ReplicationFactor <= 0 {
		brokers, _, err := k.admin.DescribeCluster()
		if err != nil {
			return fmt.Errorf("failed to describe cluster: %w", err)
		}
		topicConfig.ReplicationFactor = int16(len(brokers))
		
		// 确保副本因子不超过broker数量
		if topicConfig.ReplicationFactor > int16(len(brokers)) {
			topicConfig.ReplicationFactor = int16(len(brokers))
		}
		
		// 最小副本因子为1
		if topicConfig.ReplicationFactor <= 0 {
			topicConfig.ReplicationFactor = 1
		}
	}
	
	// 构建Topic详情
	topicDetail := &sarama.TopicDetail{
		NumPartitions:     topicConfig.NumPartitions,
		ReplicationFactor: topicConfig.ReplicationFactor,
		ConfigEntries:     convertConfigEntries(topicConfig.ConfigEntries),
	}
	
	// 创建Topic
	err := k.admin.CreateTopic(topicConfig.Name, topicDetail, false)
	if err != nil {
		return fmt.Errorf("failed to create topic %s: %w", topicConfig.Name, err)
	}
	
	return nil
}

// CreateTopics 批量创建Topic
func (k *KafkaClient) CreateTopics(topicConfigs []TopicConfig) error {
	if len(topicConfigs) == 0 {
		return nil
	}
	
	// 获取broker信息用于自动设置副本因子
	brokers, _, err := k.admin.DescribeCluster()
	if err != nil {
		return fmt.Errorf("failed to describe cluster: %w", err)
	}
	defaultReplicationFactor := int16(len(brokers))
	if defaultReplicationFactor <= 0 {
		defaultReplicationFactor = 1
	}
	
	// 构建Topic详情映射
	topicDetails := make(map[string]*sarama.TopicDetail)
	
	for _, config := range topicConfigs {
		if config.Name == "" {
			return fmt.Errorf("topic name cannot be empty")
		}
		
		// 设置默认值
		numPartitions := config.NumPartitions
		if numPartitions <= 0 {
			numPartitions = 1
		}
		
		replicationFactor := config.ReplicationFactor
		if replicationFactor <= 0 {
			replicationFactor = defaultReplicationFactor
		}
		
		topicDetails[config.Name] = &sarama.TopicDetail{
			NumPartitions:     numPartitions,
			ReplicationFactor: replicationFactor,
			ConfigEntries:     convertConfigEntries(config.ConfigEntries),
		}
	}
	
	// 批量创建Topic
	for name, detail := range topicDetails {
		err = k.admin.CreateTopic(name, detail, false)
		if err != nil {
			return fmt.Errorf("failed to create topic %s: %w", name, err)
		}
	}
	
	return nil
}

// ListTopics 列出所有Topic
func (k *KafkaClient) ListTopics() (map[string]sarama.TopicDetail, error) {
	topics, err := k.admin.ListTopics()
	if err != nil {
		return nil, fmt.Errorf("failed to list topics: %w", err)
	}
	return topics, nil
}

// DeleteTopic 删除Topic
func (k *KafkaClient) DeleteTopic(topicName string) error {
	if topicName == "" {
		return fmt.Errorf("topic name cannot be empty")
	}
	
	err := k.admin.DeleteTopic(topicName)
	if err != nil {
		return fmt.Errorf("failed to delete topic %s: %w", topicName, err)
	}
	
	return nil
}

// DeleteTopics 批量删除Topic
func (k *KafkaClient) DeleteTopics(topicNames []string) error {
	if len(topicNames) == 0 {
		return nil
	}
	
	for _, name := range topicNames {
		if name == "" {
			return fmt.Errorf("topic name cannot be empty")
		}
		
		err := k.admin.DeleteTopic(name)
		if err != nil {
			return fmt.Errorf("failed to delete topic %s: %w", name, err)
		}
	}
	
	return nil
}

// TopicExists 检查Topic是否存在
func (k *KafkaClient) TopicExists(topicName string) (bool, error) {
	if topicName == "" {
		return false, fmt.Errorf("topic name cannot be empty")
	}
	
	topics, err := k.admin.ListTopics()
	if err != nil {
		return false, fmt.Errorf("failed to list topics: %w", err)
	}
	
	_, exists := topics[topicName]
	return exists, nil
}

// -------- 消费者功能 (暂时注释) --------

// 注释：消费者功能暂时不需要，如需使用请取消下面的注释

// ConsumeOptions 消费消息的可选配置
// type ConsumeOptions struct {
// 	Partition     int32               // 指定分区
// 	Offset        int64               // 起始偏移量
// 	MaxMessages   int                 // 最大消息数量，0表示无限制
// 	Timeout       time.Duration       // 消费超时时间
// 	MessageFilter func([]byte) bool   // 消息过滤器
// }

// MessageHandler 消息处理函数
// type MessageHandler func(message *sarama.ConsumerMessage) error

// ConsumeMessages 消费指定主题和分区的消息
// func (k *KafkaClient) ConsumeMessages(ctx context.Context, topic string, handler MessageHandler, opts ...ConsumeOptions) error {
// 	var opt ConsumeOptions
// 	if len(opts) > 0 {
// 		opt = opts[0]
// 	}
// 	
// 	// 默认配置
// 	if opt.Offset == 0 {
// 		opt.Offset = sarama.OffsetNewest
// 	}
// 	if opt.Timeout == 0 {
// 		opt.Timeout = 30 * time.Second
// 	}
// 	
// 	partitionConsumer, err := k.consumer.ConsumePartition(topic, opt.Partition, opt.Offset)
// 	if err != nil {
// 		return fmt.Errorf("create partition consumer: %w", err)
// 	}
// 	defer partitionConsumer.Close()
// 	
// 	var messageCount int
// 	timeoutTimer := time.NewTimer(opt.Timeout)
// 	defer timeoutTimer.Stop()
// 	
// 	for {
// 		select {
// 		case <-ctx.Done():
// 			return ctx.Err()
// 		case <-timeoutTimer.C:
// 			return nil // 超时正常返回
// 		case err := <-partitionConsumer.Errors():
// 			if err != nil {
// 				return fmt.Errorf("consumer error: %w", err)
// 			}
// 		case message := <-partitionConsumer.Messages():
// 			if message == nil {
// 				continue
// 			}
// 			
// 			// 应用消息过滤器
// 			if opt.MessageFilter != nil && !opt.MessageFilter(message.Value) {
// 				continue
// 			}
// 			
// 			// 处理消息
// 			if err := handler(message); err != nil {
// 				return fmt.Errorf("handle message: %w", err)
// 			}
// 			
// 			messageCount++
// 			
// 			// 检查是否达到最大消息数量
// 			if opt.MaxMessages > 0 && messageCount >= opt.MaxMessages {
// 				return nil
// 			}
// 			
// 			// 重置超时计时器
// 			if !timeoutTimer.Stop() {
// 				<-timeoutTimer.C
// 			}
// 			timeoutTimer.Reset(opt.Timeout)
// 		}
// 	}
// }