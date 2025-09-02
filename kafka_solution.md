# Kafka 连接问题解决方案

## 问题分析
错误信息显示：`dial tcp 172.21.52.178:19091: i/o timeout`

这表明 Kafka 集群的 `advertised.listeners` 配置指向了内部 Docker 网络地址 `172.21.52.178`，而不是外部可访问的 `127.0.0.1`。

## 解决方案

### 方案1：修改 Kafka 集群配置（推荐）
在 Kafka 的 Docker Compose 或配置文件中，需要正确配置 `advertised.listeners`：

```yaml
# docker-compose.yml
services:
  kafka1:
    environment:
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://127.0.0.1:19091
      # 或者
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://localhost:19091
```

### 方案2：使用 Docker 网络地址
如果无法修改 Kafka 配置，可以直接使用 Docker 内部地址：

```go
config := &kafkatools.KafkaConfig{
    Brokers: []string{
        "172.21.52.178:19091",
        "172.21.52.178:19092", 
        "172.21.52.178:19093",
    },
    ClientID: "test-client",
}
```

### 方案3：添加主机映射
在 `/etc/hosts` 文件中添加映射：
```
172.21.52.178 kafka-broker-1
172.21.52.178 kafka-broker-2  
172.21.52.178 kafka-broker-3
```

### 方案4：使用我们的 kafkatools 配置
我已经在 kafkatools 中添加了网络配置选项，可以尝试：

```go
config := &kafkatools.KafkaConfig{
    Brokers: []string{
        "127.0.0.1:19091",
        "127.0.0.1:19092",
        "127.0.0.1:19093",
    },
    ClientID:              "test-client",
    ForceDirectConnection: true,
    KafkaVersion:          "2.8.0",
}
```

## 当前状态
- ✅ kafkatools 代码没有问题
- ✅ 自动创建 topic 功能已实现
- ❌ Kafka 集群网络配置需要调整

## 建议
最好的解决方案是修改 Kafka 集群的 `advertised.listeners` 配置，确保它指向外部可访问的地址。
