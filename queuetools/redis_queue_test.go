package queue_tools

import (
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OnlyPiglet/fly/redistools"
)

type dv struct {
	Abcdefgh string `json:"abcdefgh"`
	Abcdef   bool   `json:"abcdef"`
}

type SV struct {
	A string
}

func (S SV) MarshalBinary() (data []byte, err error) {
	return json.Marshal(S)
}

func (S SV) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, &S)
}

type SVS []SV

func (S SVS) MarshalBinary() (data []byte, err error) {
	return json.Marshal(S)
}

func (S SVS) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, &S)
}

func TestEnQueueWithSize(t *testing.T) {
	single, err := redistools.InitSingle("127.0.0.1:6379", "", "", 1)
	if err != nil {
		t.Error(err)
	}
	queue := NewRedisQueue[SV]("abc", single, 5, 300*time.Millisecond)
	err = queue.Enqueue([]SV{
		{"1"}, {"2"}, {"2"}, {"2"}, {"2"}, {"2"}, {"2"}, {"2"}, {"2"}, {"2"},
	})
	if err != nil {
		t.Error(err)
	}
}

func TestEnQueueWithoutSize(t *testing.T) {
	single, err := redistools.InitSingle("127.0.0.1:6379", "", "", 1)
	if err != nil {
		t.Error(err)
	}
	queue := NewRedisQueue[SV]("abc", single, 0, 300*time.Millisecond)
	err = queue.Enqueue([]SV{
		{"1"}, {"2"}, {"2"}, {"2"}, {"2"}, {"2"}, {"2"}, {"2"}, {"2"}, {"2"},
	})
	if err != nil {
		t.Error(err)
	}
}

func TestProductAndCustomer(t *testing.T) {
	dvs := make([]dv, 0, 10000)
	for i := 0; i < 10000; i++ {
		dvs = append(dvs, dv{
			Abcdefgh: "2ba2d4bd-af24-48ae-9e66-22d1c30c9f01",
			Abcdef:   true,
		})
	}
	//single, err := redistools.InitSingle("r-bp1dqgkbwcsqyto3lcpd.redis.rds.aliyuncs.com:6379", "r-bp1dqgkbwcsqyto3lc", "4553283@wch", 0)
	single, err := redistools.InitSingle("127.0.0.1:6379", "", "", 0)

	if err != nil {
		t.Error(err)
	}

	ts := time.Now()

	// 修复方案：
	// 1. 移除容量限制（改为 0 或 -1）或增大容量到 1000000+
	// 2. 分批写入，每批 5000 条，避免单次传输数据量过大
	queue := NewRedisQueue[dv]("abc", single, 0, 30000*time.Millisecond) // 容量改为 0（无限制）

	batchSize := 10000 // 每批 5000 条
	totalBatches := (len(dvs) + batchSize - 1) / batchSize

	t.Logf("开始分批写入: 总数=%d, 批次大小=%d, 批次数=%d", len(dvs), batchSize, totalBatches)

	for i := 0; i < len(dvs); i += batchSize {
		end := i + batchSize
		if end > len(dvs) {
			end = len(dvs)
		}

		batch := dvs[i:end]
		err = queue.Enqueue(batch)
		if err != nil {
			t.Errorf("批次 %d-%d 写入失败: %v", i, end, err)
			return
		}

		if (i/batchSize+1)%20 == 0 { // 每 20 批输出一次进度
			t.Logf("已写入: %d / %d (%.1f%%)", end, len(dvs), float64(end)*100/float64(len(dvs)))
		}
	}

	sub := time.Now().Sub(ts)
	t.Logf("✓ 写入完成! 总耗时: %d ms (%.2f 秒)", sub.Milliseconds(), sub.Seconds())
	t.Logf("  平均速度: %.0f 条/秒", float64(len(dvs))/sub.Seconds())

	// 验证队列长度
	queueLen, err := queue.Len()
	if err != nil {
		t.Errorf("获取队列长度失败: %v", err)
	} else {
		t.Logf("  队列最终长度: %d", queueLen)
	}
}

// TestProductAndCustomer_SmallBatch 小批量测试（用于快速验证）
func TestProductAndCustomer_SmallBatch(t *testing.T) {
	dvs := make([]dv, 0, 10000)
	for i := 0; i < 10000; i++ {
		dvs = append(dvs, dv{
			Abcdefgh: "2ba2d4bd-af24-48ae-9e66-22d1c30c9f01",
			Abcdef:   true,
		})
	}

	single, err := redistools.InitSingle("r-bp1dqgkbwcsqyto3lcpd.redis.rds.aliyuncs.com:6379", "r-bp1dqgkbwcsqyto3lc", "4553283@wch", 0)
	if err != nil {
		t.Error(err)
		return
	}

	ts := time.Now()

	// 小批量测试：1 万条数据
	queue := NewRedisQueue[dv]("abc_small", single, 0, 10000*time.Millisecond)

	batchSize := 1000
	totalBatches := (len(dvs) + batchSize - 1) / batchSize

	t.Logf("小批量测试: 总数=%d, 批次大小=%d, 批次数=%d", len(dvs), batchSize, totalBatches)

	for i := 0; i < len(dvs); i += batchSize {
		end := i + batchSize
		if end > len(dvs) {
			end = len(dvs)
		}

		batch := dvs[i:end]
		err = queue.Enqueue(batch)
		if err != nil {
			t.Errorf("批次 %d-%d 写入失败: %v", i, end, err)
			return
		}
	}

	sub := time.Now().Sub(ts)
	t.Logf("✓ 小批量测试完成! 耗时: %d ms", sub.Milliseconds())

	queueLen, _ := queue.Len()
	t.Logf("  队列长度: %d", queueLen)
}

// TestProductAndCustomer_CapacityLimit 测试容量限制
func TestProductAndCustomer_CapacityLimit(t *testing.T) {
	single, err := redistools.InitSingle("r-bp1dqgkbwcsqyto3lcpd.redis.rds.aliyuncs.com:6379", "r-bp1dqgkbwcsqyto3lc", "4553283@wch", 0)
	if err != nil {
		t.Error(err)
		return
	}

	// 测试容量限制：队列容量 100，尝试写入 150 条
	queue := NewRedisQueue[dv]("abc_capacity_test", single, 100, 5000*time.Millisecond)

	t.Log("=== 测试1: 写入 50 条（应该成功）===")
	batch1 := make([]dv, 50)
	for i := 0; i < 50; i++ {
		batch1[i] = dv{Abcdefgh: "test1", Abcdef: true}
	}
	err = queue.Enqueue(batch1)
	if err != nil {
		t.Errorf("❌ 第一批写入失败: %v", err)
	} else {
		queueLen, _ := queue.Len()
		t.Logf("✓ 第一批写入成功，队列长度: %d", queueLen)
	}

	t.Log("\n=== 测试2: 再写入 40 条（应该成功）===")
	batch2 := make([]dv, 40)
	for i := 0; i < 40; i++ {
		batch2[i] = dv{Abcdefgh: "test2", Abcdef: false}
	}
	err = queue.Enqueue(batch2)
	if err != nil {
		t.Errorf("❌ 第二批写入失败: %v", err)
	} else {
		queueLen, _ := queue.Len()
		t.Logf("✓ 第二批写入成功，队列长度: %d", queueLen)
	}

	t.Log("\n=== 测试3: 再写入 20 条（应该失败：超出容量）===")
	batch3 := make([]dv, 20)
	for i := 0; i < 20; i++ {
		batch3[i] = dv{Abcdefgh: "test3", Abcdef: true}
	}
	err = queue.Enqueue(batch3)
	if err != nil {
		t.Logf("✓ 第三批写入失败（符合预期）: %v", err)
	} else {
		t.Error("❌ 第三批写入成功了，但应该失败（队列容量超限）")
	}
}

// TestConcurrentWrite_PressureTest 多实例并发写入压力测试
// 场景：多个生产者并发写入，持续10分钟，每5秒写入一批，观察Redis性能
func TestConcurrentWrite_PressureTest(t *testing.T) {
	// 使用本地 Redis（改为远程地址测试远程 Redis）
	single, err := redistools.InitSingle("127.0.0.1:6379", "", "", 0)
	if err != nil {
		t.Error(err)
		return
	}

	// 配置参数
	const (
		numWorkers       = 10               // 并发写入的 worker 数量
		numConsumers     = 10               // 并发消费者数量（建议与生产者数量相当）
		batchSize        = 1000             // 每批写入的数据量
		consumeBatchSize = 100              // 每次批量消费的数据量 ⚡
		writeInterval    = 5 * time.Second  // 每 5 秒写入一次
		testDuration     = 10 * time.Minute // 持续 10 分钟
		queueCapacity    = 0                // 无容量限制
		redisTimeout     = 30 * time.Second // Redis 操作超时
		consumeLogEvery  = 10000            // 每消费多少条打印一次日志
	)

	t.Logf("=== 🚀 批量消费压力测试配置 ===")
	t.Logf("  ⚙️  生产配置:")
	t.Logf("     - 生产者数量: %d", numWorkers)
	t.Logf("     - 生产批次: %d 条/批", batchSize)
	t.Logf("     - 写入间隔: %s", writeInterval)
	t.Logf("  ⚡ 消费配置 (批量模式):")
	t.Logf("     - 消费者数量: %d", numConsumers)
	t.Logf("     - 消费批次: %d 条/批 ← 关键优化！", consumeBatchSize)
	t.Logf("  📊 预测:")
	expectedTotal := numWorkers * batchSize * int(testDuration/writeInterval)
	t.Logf("     - 理论生产: %d 条/10分钟 (~%.0f 条/秒)", expectedTotal, float64(numWorkers*batchSize)/writeInterval.Seconds())
	t.Logf("     - 理论消费: ~%d 条/秒 (单次批量×消费者数)", numConsumers*consumeBatchSize*10) // 假设每次10ms
	t.Logf("  ⏱️  测试时长: %s", testDuration)
	t.Logf("")

	// 创建队列
	queue := NewRedisQueue[dv]("pressure_test", single, queueCapacity, redisTimeout)

	// 清空队列（避免之前测试的数据影响）
	t.Log("清空旧数据...")
	for {
		_, err := queue.Dequeue()
		if err != nil {
			break
		}
	}

	// 统计信息
	type Stats struct {
		TotalWrites   int64
		TotalRecords  int64
		SuccessWrites int64
		FailedWrites  int64
		TotalDuration time.Duration
		MaxDuration   time.Duration
		MinDuration   time.Duration
		Errors        []string
	}

	stats := make([]Stats, numWorkers)
	for i := range stats {
		stats[i].MinDuration = time.Hour // 初始化为一个大值
	}

	// 消费者统计
	var consumerCount int64  // 总消费数量
	var consumerErrors int64 // 消费失败数量
	var _ time.Time          // 最后一次消费时间

	startTime := time.Now()
	stopChan := make(chan struct{})
	consumerStopChan := make(chan struct{})
	consumerDoneChan := make(chan bool, numConsumers)
	doneChan := make(chan int, numWorkers)

	// 启动多个批量消费者协程
	t.Logf("🔄 启动 %d 个批量消费者协程（每次消费 %d 条）...\n", numConsumers, consumeBatchSize)
	for consumerID := 0; consumerID < numConsumers; consumerID++ {
		go func(id int) {
			defer func() {
				consumerDoneChan <- true
			}()

			consecutiveErrors := 0
			maxConsecutiveErrors := 5 // 连续失败5次后，暂停一下
			localConsumeCount := int64(0)
			localBatchCount := int64(0)

			for {
				select {
				case <-consumerStopChan:
					t.Logf("🛑 消费者-%d 停止 | 本地消费: %d 条 (分 %d 批)", id, localConsumeCount, localBatchCount)
					return
				default:
					// 批量消费 - 每次尝试消费 consumeBatchSize 条
					items, err := queue.DequeueBatch(consumeBatchSize)
					if err != nil || len(items) == 0 {
						// 队列为空或其他错误
						consecutiveErrors++
						if err != nil {
							atomic.AddInt64(&consumerErrors, 1)
						}

						// 如果连续失败多次，说明队列可能长时间为空，稍微休息一下
						if consecutiveErrors >= maxConsecutiveErrors {
							time.Sleep(50 * time.Millisecond) // 批量消费时可以更频繁重试
							consecutiveErrors = 0
						}
						continue
					}

					// 批量消费成功
					consecutiveErrors = 0
					batchCount := int64(len(items))
					localConsumeCount += batchCount
					localBatchCount++
					totalCount := atomic.AddInt64(&consumerCount, batchCount)

					// 只打印内容，不做任何处理（按配置的频率打印日志）
					if totalCount%int64(consumeLogEvery) < batchCount || totalCount == batchCount {
						t.Logf("⚡ [消费者-%d] 批量: %d条 | 累计: %d条 | 数据: %s",
							id, batchCount, totalCount, items[0].Abcdefgh)
					}
				}
			}
		}(consumerID)
	}

	// 启动统计输出协程
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				elapsed := time.Since(startTime)
				var totalWrites, totalRecords, successWrites, failedWrites int64
				var totalDuration, maxDuration time.Duration
				minDuration := time.Hour

				for i := 0; i < numWorkers; i++ {
					totalWrites += stats[i].TotalWrites
					totalRecords += stats[i].TotalRecords
					successWrites += stats[i].SuccessWrites
					failedWrites += stats[i].FailedWrites
					totalDuration += stats[i].TotalDuration
					if stats[i].MaxDuration > maxDuration {
						maxDuration = stats[i].MaxDuration
					}
					if stats[i].MinDuration < minDuration && stats[i].MinDuration > 0 {
						minDuration = stats[i].MinDuration
					}
				}

				queueLen, _ := queue.Len()
				avgDuration := time.Duration(0)
				if totalWrites > 0 {
					avgDuration = totalDuration / time.Duration(totalWrites)
				}

				progress := elapsed.Minutes() / testDuration.Minutes() * 100
				currentConsumerCount := atomic.LoadInt64(&consumerCount)
				currentConsumerErrors := atomic.LoadInt64(&consumerErrors)

				t.Logf("\n⏱️  [进度报告] 已运行: %.1f 分钟 / %.0f 分钟 (%.1f%%)",
					elapsed.Minutes(), testDuration.Minutes(), progress)
				t.Logf("  📊 生产者统计:")
				t.Logf("     - 总写入次数: %d", totalWrites)
				t.Logf("     - 总记录数: %d", totalRecords)
				t.Logf("     - 成功/失败: %d / %d", successWrites, failedWrites)
				if totalWrites > 0 {
					t.Logf("     - 成功率: %.2f%%", float64(successWrites)*100/float64(totalWrites))
				}
				t.Logf("  🍽️  消费者统计 (批量模式 %d条/次):", consumeBatchSize)
				t.Logf("     - 已消费: %d 条", currentConsumerCount)
				t.Logf("     - 消费速度: %.0f 条/秒 ⚡", float64(currentConsumerCount)/elapsed.Seconds())
				t.Logf("     - 消费错误: %d 次 (空队列尝试)", currentConsumerErrors)
				if totalRecords > 0 {
					consumeRatio := float64(currentConsumerCount) / float64(totalRecords) * 100
					t.Logf("     - 消费进度: %.2f%%", consumeRatio)
					produceSpeed := float64(totalRecords) / elapsed.Seconds()
					consumeSpeed := float64(currentConsumerCount) / elapsed.Seconds()
					if produceSpeed > 0 {
						t.Logf("     - 消费/生产比: %.2f%% (>100%%为消费快)", consumeSpeed/produceSpeed*100)
					}
				}
				t.Logf("  ⚡ 性能指标:")
				t.Logf("     - 生产速度: %.0f 条/秒", float64(totalRecords)/elapsed.Seconds())
				t.Logf("     - 平均写入耗时: %s", avgDuration)
				t.Logf("     - 最快/最慢: %s / %s", minDuration, maxDuration)
				t.Logf("  📦 队列状态:")
				t.Logf("     - 当前队列长度: %d", queueLen)
				t.Logf("     - 堆积量: 生产 %d - 消费 %d = %d", totalRecords, currentConsumerCount, totalRecords-currentConsumerCount)
				t.Logf("")

			case <-stopChan:
				return
			}
		}
	}()

	// 启动多个并发 worker
	t.Logf("🚀 启动 %d 个并发 Worker...\n", numWorkers)

	for workerID := 0; workerID < numWorkers; workerID++ {
		go func(id int) {
			ticker := time.NewTicker(writeInterval)
			defer ticker.Stop()

			timeout := time.After(testDuration)

			for {
				select {
				case <-timeout:
					doneChan <- id
					return

				case <-ticker.C:
					batch := make([]dv, batchSize)
					for i := 0; i < batchSize; i++ {
						batch[i] = dv{
							Abcdefgh: "2ba2d4bd-af24-48ae-9e66-22d1c30c9f01",
							Abcdef:   true,
						}
					}

					writeStart := time.Now()
					err := queue.Enqueue(batch)
					writeDuration := time.Since(writeStart)

					stats[id].TotalWrites++
					stats[id].TotalRecords += int64(batchSize)
					stats[id].TotalDuration += writeDuration

					if writeDuration > stats[id].MaxDuration {
						stats[id].MaxDuration = writeDuration
					}
					if writeDuration < stats[id].MinDuration {
						stats[id].MinDuration = writeDuration
					}

					if err != nil {
						stats[id].FailedWrites++
						errMsg := err.Error()
						if len(stats[id].Errors) < 10 {
							stats[id].Errors = append(stats[id].Errors, errMsg)
						}
						t.Logf("❌ Worker-%d 写入失败: %v (耗时: %s)", id, err, writeDuration)
					} else {
						stats[id].SuccessWrites++
						if writeDuration > 2*time.Second {
							t.Logf("⚠️  Worker-%d 写入较慢: %s", id, writeDuration)
						}
					}
				}
			}
		}(workerID)
	}

	// 等待所有 worker 完成
	for i := 0; i < numWorkers; i++ {
		workerID := <-doneChan
		t.Logf("✓ Worker-%d 已完成", workerID)
	}

	t.Log("\n⏳ 生产者已全部完成，等待消费者消费剩余数据...")

	// 给消费者一些时间消费剩余数据
	remainingTimeout := time.After(30 * time.Second)
	checkTicker := time.NewTicker(2 * time.Second)
	defer checkTicker.Stop()

consumeRemaining:
	for {
		select {
		case <-remainingTimeout:
			t.Log("⚠️  等待超时（30秒），停止消费者")
			break consumeRemaining
		case <-checkTicker.C:
			queueLen, _ := queue.Len()
			if queueLen == 0 {
				t.Log("✓ 队列已清空")
				break consumeRemaining
			}
			currentConsumerCount := atomic.LoadInt64(&consumerCount)
			t.Logf("  剩余队列长度: %d, 已消费: %d", queueLen, currentConsumerCount)
		}
	}

	// 停止所有消费者
	close(consumerStopChan)
	for i := 0; i < numConsumers; i++ {
		<-consumerDoneChan
	}
	t.Logf("✓ 所有 %d 个消费者已停止", numConsumers)

	close(stopChan)
	time.Sleep(100 * time.Millisecond) // 等待统计协程退出

	// 最终统计
	totalElapsed := time.Since(startTime)
	t.Logf("\n%s", "======================================================")
	t.Logf("📊 最终测试报告")
	t.Logf("%s", "======================================================")

	var totalWrites, totalRecords, successWrites, failedWrites int64
	var totalDuration, maxDuration time.Duration
	minDuration := time.Hour
	var allErrors []string

	for i := 0; i < numWorkers; i++ {
		totalWrites += stats[i].TotalWrites
		totalRecords += stats[i].TotalRecords
		successWrites += stats[i].SuccessWrites
		failedWrites += stats[i].FailedWrites
		totalDuration += stats[i].TotalDuration
		if stats[i].MaxDuration > maxDuration {
			maxDuration = stats[i].MaxDuration
		}
		if stats[i].MinDuration < minDuration && stats[i].MinDuration > 0 {
			minDuration = stats[i].MinDuration
		}
		allErrors = append(allErrors, stats[i].Errors...)
	}

	finalQueueLen, _ := queue.Len()
	finalConsumerCount := atomic.LoadInt64(&consumerCount)
	finalConsumerErrors := atomic.LoadInt64(&consumerErrors)

	t.Logf("\n⏱️  时间统计:")
	t.Logf("  - 实际运行时长: %.2f 分钟", totalElapsed.Minutes())
	t.Logf("  - 总写入耗时: %.2f 秒", totalDuration.Seconds())
	if totalWrites > 0 {
		t.Logf("  - 平均单次写入: %s", totalDuration/time.Duration(totalWrites))
	}
	t.Logf("  - 最快/最慢写入: %s / %s", minDuration, maxDuration)

	t.Logf("\n📊 生产者统计:")
	t.Logf("  - 总写入次数: %d", totalWrites)
	t.Logf("  - 总记录数: %d", totalRecords)
	if totalWrites > 0 {
		t.Logf("  - 成功次数: %d (%.2f%%)", successWrites, float64(successWrites)*100/float64(totalWrites))
		t.Logf("  - 失败次数: %d (%.2f%%)", failedWrites, float64(failedWrites)*100/float64(totalWrites))
	}

	t.Logf("\n🍽️  消费者统计:")
	t.Logf("  - 总消费数量: %d", finalConsumerCount)
	t.Logf("  - 消费错误: %d 次", finalConsumerErrors)
	if totalRecords > 0 {
		t.Logf("  - 消费完成率: %.2f%%", float64(finalConsumerCount)*100/float64(totalRecords))
	}
	t.Logf("  - 平均消费速度: %.0f 条/秒", float64(finalConsumerCount)/totalElapsed.Seconds())

	t.Logf("\n⚡ 性能指标:")
	avgProduceThroughput := float64(totalRecords) / totalElapsed.Seconds()
	avgConsumeThroughput := float64(finalConsumerCount) / totalElapsed.Seconds()
	peakThroughput := float64(numWorkers*batchSize) / writeInterval.Seconds()
	t.Logf("  - 生产吞吐量: %.0f 条/秒", avgProduceThroughput)
	t.Logf("  - 消费吞吐量: %.0f 条/秒", avgConsumeThroughput)
	t.Logf("  - 峰值吞吐量: %.0f 条/秒 (理论)", peakThroughput)
	t.Logf("  - 生产/理论比: %.2f%%", avgProduceThroughput/peakThroughput*100)
	if avgProduceThroughput > 0 {
		t.Logf("  - 消费/生产比: %.2f%% (>100%%说明消费快于生产)", avgConsumeThroughput/avgProduceThroughput*100)
	}

	t.Logf("\n📦 队列状态:")
	t.Logf("  - 最终队列长度: %d", finalQueueLen)
	t.Logf("  - 生产总量: %d", totalRecords)
	t.Logf("  - 消费总量: %d", finalConsumerCount)
	t.Logf("  - 剩余未消费: %d", totalRecords-finalConsumerCount)
	if totalRecords > 0 {
		t.Logf("  - 剩余比例: %.2f%%", float64(finalQueueLen)*100/float64(totalRecords))
	}

	if len(allErrors) > 0 {
		t.Logf("\n❌ 错误汇总 (前10条):")
		for i, errMsg := range allErrors {
			if i >= 10 {
				t.Logf("  ... 还有 %d 条错误未显示", len(allErrors)-10)
				break
			}
			t.Logf("  %d. %s", i+1, errMsg)
		}
	}

	t.Logf("\n%s", "======================================================")

	// 性能评估
	t.Logf("\n🎯 性能评估:")

	// 生产者评估
	if failedWrites == 0 {
		t.Log("  ✅ 生产者: 所有写入均成功")
	} else {
		failRate := float64(failedWrites) * 100 / float64(totalWrites)
		if failRate > 5 {
			t.Errorf("  ❌ 生产者失败率过高: %.2f%%", failRate)
		} else {
			t.Logf("  ⚠️  生产者有少量失败: %.2f%%", failRate)
		}
	}

	// 消费者评估
	if totalRecords > 0 {
		consumeRate := float64(finalConsumerCount) * 100 / float64(totalRecords)
		if consumeRate >= 99 {
			t.Log("  ✅ 消费者: 消费完成率优秀 (≥99%)")
		} else if consumeRate >= 95 {
			t.Log("  ✅ 消费者: 消费完成率良好 (≥95%)")
		} else if consumeRate >= 90 {
			t.Logf("  ⚠️  消费者: 消费完成率一般 (%.2f%%, ≥90%%)", consumeRate)
		} else {
			t.Logf("  ❌ 消费者: 消费完成率较低 (%.2f%%, <90%%)", consumeRate)
		}
	}

	// 整体评估
	if finalQueueLen == 0 && failedWrites == 0 {
		t.Log("  🎉 整体评估: 完美！队列已清空，无失败写入")
	} else if finalQueueLen < int64(batchSize) && failedWrites == 0 {
		t.Log("  ✅ 整体评估: 优秀！剩余数据少，无失败写入")
	} else {
		t.Log("  ⚠️  整体评估: 可接受，但仍有优化空间")
	}
}

// TestConcurrentWrite_BatchConsume 批量消费测试
func TestConcurrentWrite_BatchConsume(t *testing.T) {
	single, err := redistools.InitSingle("r-bp1dqgkbwcsqyto3lcpd.redis.rds.aliyuncs.com:6379", "r-bp1dqgkbwcsqyto3lc", "4553283@wch", 0)
	if err != nil {
		t.Error(err)
		return
	}

	const (
		numWorkers       = 10
		numConsumers     = 5 // 批量消费可以用更少的消费者
		batchSize        = 1000
		consumeBatchSize = 100 // 每次批量消费100条
		writeInterval    = 5 * time.Second
		testDuration     = 2 * time.Minute // 2分钟测试
	)

	t.Logf("=== 批量消费压力测试配置 ===")
	t.Logf("  生产者数: %d, 消费者数: %d", numWorkers, numConsumers)
	t.Logf("  生产批次: %d 条, 消费批次: %d 条", batchSize, consumeBatchSize)
	t.Logf("  测试时长: %s", testDuration)
	t.Logf("")

	queue := NewRedisQueue[dv]("batch_consume_test", single, 0, 30*time.Second)

	// 清空旧数据
	t.Log("清空旧数据...")
	for {
		_, err := queue.Dequeue()
		if err != nil {
			break
		}
	}

	var producedCount, consumedCount int64
	startTime := time.Now()
	stopChan := make(chan struct{})
	consumerStopChan := make(chan struct{})
	consumerDoneChan := make(chan bool, numConsumers)
	producerDoneChan := make(chan bool, numWorkers)

	// 启动批量消费者
	t.Logf("🔄 启动 %d 个批量消费者...", numConsumers)
	for consumerID := 0; consumerID < numConsumers; consumerID++ {
		go func(id int) {
			defer func() {
				consumerDoneChan <- true
			}()

			for {
				select {
				case <-consumerStopChan:
					return
				default:
					// 批量消费
					items, err := queue.DequeueBatch(consumeBatchSize)
					if err != nil || len(items) == 0 {
						time.Sleep(50 * time.Millisecond) // 队列为空，稍等
						continue
					}

					count := atomic.AddInt64(&consumedCount, int64(len(items)))
					if count%10000 == 0 {
						t.Logf("🍽️  [消费者-%d] 批量消费: 本次 %d 条, 累计 %d 条", id, len(items), count)
					}
				}
			}
		}(consumerID)
	}

	// 启动生产者
	t.Logf("🚀 启动 %d 个生产者...", numWorkers)
	for workerID := 0; workerID < numWorkers; workerID++ {
		go func(id int) {
			ticker := time.NewTicker(writeInterval)
			defer ticker.Stop()
			timeout := time.After(testDuration)

			for {
				select {
				case <-timeout:
					producerDoneChan <- true
					return
				case <-ticker.C:
					batch := make([]dv, batchSize)
					for i := 0; i < batchSize; i++ {
						batch[i] = dv{
							Abcdefgh: "2ba2d4bd-af24-48ae-9e66-22d1c30c9f01",
							Abcdef:   true,
						}
					}

					if err := queue.Enqueue(batch); err == nil {
						atomic.AddInt64(&producedCount, int64(batchSize))
					}
				}
			}
		}(workerID)
	}

	// 定期输出统计
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-stopChan:
				return
			case <-ticker.C:
				elapsed := time.Since(startTime)
				produced := atomic.LoadInt64(&producedCount)
				consumed := atomic.LoadInt64(&consumedCount)
				queueLen, _ := queue.Len()

				produceSpeed := float64(produced) / elapsed.Seconds()
				consumeSpeed := float64(consumed) / elapsed.Seconds()

				t.Logf("\n⏱️  [批量消费进度] 运行: %.1f 秒", elapsed.Seconds())
				t.Logf("  生产: %d 条 (%.0f 条/秒)", produced, produceSpeed)
				t.Logf("  消费: %d 条 (%.0f 条/秒)", consumed, consumeSpeed)
				t.Logf("  队列: %d 条, 消费率: %.2f%%", queueLen, consumeSpeed/produceSpeed*100)
				t.Logf("")
			}
		}
	}()

	// 等待生产者完成
	for i := 0; i < numWorkers; i++ {
		<-producerDoneChan
	}
	t.Log("✓ 所有生产者已完成")

	// 等待消费剩余数据
	time.Sleep(10 * time.Second)
	close(consumerStopChan)
	for i := 0; i < numConsumers; i++ {
		<-consumerDoneChan
	}

	close(stopChan)
	time.Sleep(100 * time.Millisecond)

	// 最终统计
	totalElapsed := time.Since(startTime)
	finalProduced := atomic.LoadInt64(&producedCount)
	finalConsumed := atomic.LoadInt64(&consumedCount)
	finalQueueLen, _ := queue.Len()

	t.Logf("\n%s", "======================================================")
	t.Logf("📊 批量消费测试报告")
	t.Logf("%s", "======================================================")
	t.Logf("\n耗时: %.2f 秒", totalElapsed.Seconds())
	t.Logf("生产: %d 条 (%.0f 条/秒)", finalProduced, float64(finalProduced)/totalElapsed.Seconds())
	t.Logf("消费: %d 条 (%.0f 条/秒)", finalConsumed, float64(finalConsumed)/totalElapsed.Seconds())
	t.Logf("剩余: %d 条", finalQueueLen)
	t.Logf("消费完成率: %.2f%%", float64(finalConsumed)*100/float64(finalProduced))

	if float64(finalConsumed)/float64(finalProduced) >= 0.95 {
		t.Log("✅ 批量消费性能优秀！")
	}
}

// TestConcurrentWrite_ShortTest 短时并发测试（1分钟，用于快速验证）
func TestConcurrentWrite_ShortTest(t *testing.T) {
	single, err := redistools.InitSingle("127.0.0.1:6379", "", "", 0)
	if err != nil {
		t.Error(err)
		return
	}

	const (
		numWorkers    = 5
		batchSize     = 500
		writeInterval = 5 * time.Second
		testDuration  = 1 * time.Minute
	)

	t.Logf("🚀 快速并发测试 (1分钟)")
	t.Logf("  Workers: %d, 每批: %d 条, 间隔: %s", numWorkers, batchSize, writeInterval)

	queue := NewRedisQueue[dv]("short_test", single, 0, 10*time.Second)

	// 清空旧数据
	for {
		_, err := queue.Dequeue()
		if err != nil {
			break
		}
	}

	startTime := time.Now()
	var successCount, failCount int64
	doneChan := make(chan bool, numWorkers)

	for workerID := 0; workerID < numWorkers; workerID++ {
		go func(id int) {
			ticker := time.NewTicker(writeInterval)
			defer ticker.Stop()
			timeout := time.After(testDuration)

			for {
				select {
				case <-timeout:
					doneChan <- true
					return
				case <-ticker.C:
					batch := make([]dv, batchSize)
					for i := 0; i < batchSize; i++ {
						batch[i] = dv{Abcdefgh: "test", Abcdef: true}
					}

					if err := queue.Enqueue(batch); err != nil {
						atomic.AddInt64(&failCount, 1)
						t.Logf("❌ Worker-%d 失败: %v", id, err)
					} else {
						atomic.AddInt64(&successCount, 1)
					}
				}
			}
		}(workerID)
	}

	for i := 0; i < numWorkers; i++ {
		<-doneChan
	}

	elapsed := time.Since(startTime)
	finalSuccessCount := atomic.LoadInt64(&successCount)
	finalFailCount := atomic.LoadInt64(&failCount)
	totalWrites := finalSuccessCount + finalFailCount
	totalRecords := finalSuccessCount * int64(batchSize)
	queueLen, _ := queue.Len()

	t.Logf("\n✓ 测试完成 (%.1f 秒)", elapsed.Seconds())
	t.Logf("  成功: %d, 失败: %d (总计: %d 次)", finalSuccessCount, finalFailCount, totalWrites)
	t.Logf("  总记录: %d, 速度: %.0f 条/秒", totalRecords, float64(totalRecords)/elapsed.Seconds())
	t.Logf("  队列长度: %d", queueLen)

	if finalFailCount > 0 {
		t.Errorf("存在失败的写入: %d 次", finalFailCount)
	}
}
