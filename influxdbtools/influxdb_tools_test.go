package influxdbtools

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"
)

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			return n
		}
	}
	return def
}

func getenvDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		d, err := time.ParseDuration(v)
		if err == nil {
			return d
		}
	}
	return def
}

// TestContinuousWriteWeather 持续写入天气数据到指定bucket（默认 weather）
// 环境变量：
//
//	INFLUX_URL     (默认 http://localhost:8086)
//	INFLUX_TOKEN   (默认使用用户提供的token)
//	INFLUX_ORG     (默认 your-org)
//	INFLUX_BUCKET  (默认 weather)
//	INFLUX_COUNT   (默认 10；<=0 表示使用 INFLUX_DURATION)
//	INFLUX_DURATION(默认 5s；在 INFLUX_COUNT<=0 时生效)
//	INFLUX_INTERVAL(默认 500ms 写入间隔)
func TestContinuousWriteWeather(t *testing.T) {
	url := getenv("INFLUX_URL", "http://localhost:8086")
	token := getenv("INFLUX_TOKEN", "tcgd5hR4Sbf98gg25PVqDpHcIxyqNW4glvJyB8McthZXyglqzb3UIsQoENCmiQq4Z3S7fbS7fnroPAjHg3p3hg==")
	org := getenv("INFLUX_ORG", "wswch")
	bucket := getenv("INFLUX_BUCKET", "weather")
	count := getenvInt("INFLUX_COUNT", 1000000)
	interval := getenvDuration("INFLUX_INTERVAL", 500*time.Millisecond)
	duration := getenvDuration("INFLUX_DURATION", 5*time.Second)

	client, err := NewInfluxClient(&InfluxConfig{
		URL:         url,
		Token:       token,
		Org:         org,
		Bucket:      bucket,
		UseGzip:     true,
		PingTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("init influx client: %v", err)
	}
	defer client.Close()

	t.Logf("writing weather data -> url=%s org=%s bucket=%s", url, org, bucket)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	writeOnce := func(i int) {
		location := fmt.Sprintf("sensor-%d", rand.Intn(3)+1)
		temperature := 20 + rand.Float64()*10
		humidity := 40 + rand.Float64()*30
		pressure := 1000 + rand.Float64()*20
		wind := rand.Float64() * 10
		windDir := []string{"N", "E", "S", "W"}[rand.Intn(4)]

		now := time.Now()
		p := Point{
			Measurement: "weather",
			Tags: map[string]string{
				"location": location,
				"wind_dir": windDir,
			},
			Fields: map[string]interface{}{
				"temperature": temperature,
				"humidity":    humidity,
				"pressure":    pressure,
				"wind":        wind,
			},
			Timestamp: &now,
		}
		if err := client.WritePoint(p); err != nil {
			t.Errorf("write %d error: %v", i, err)
			return
		}
		t.Logf("[%d] 写入: 位置=%s 温度=%.1f℃ 湿度=%.1f%% 气压=%.1fhPa 风速=%.1fm/s 风向=%s",
			i, location, temperature, humidity, pressure, wind, windDir)
	}

	if count > 0 {
		for i := 1; i <= count; i++ {
			select {
			case <-ctx.Done():
				return
			default:
			}
			writeOnce(i)
			time.Sleep(interval)
		}
	} else {
		end := time.Now().Add(duration)
		i := 0
		for time.Now().Before(end) {
			i++
			select {
			case <-ctx.Done():
				return
			default:
			}
			writeOnce(i)
			time.Sleep(interval)
		}
	}

	// 简单查询验证（可选）
	flux := fmt.Sprintf(`from(bucket:%q) |> range(start: -5m) |> filter(fn: (r) => r._measurement == "weather") |> limit(n:1)`, bucket)
	rows, err := client.Query(context.Background(), flux)
	if err != nil {
		// 仅记录错误，不使测试失败（某些环境未启用查询权限）
		t.Logf("query warning: %v", err)
		return
	}
	if len(rows) == 0 {
		t.Logf("no rows returned from verification query")
	}
}
