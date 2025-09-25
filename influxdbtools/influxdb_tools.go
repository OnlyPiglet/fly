package influxdbtools

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	influxapi "github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
	"github.com/influxdata/influxdb-client-go/v2/domain"
)

// InfluxConfig InfluxDB集群配置
type InfluxConfig struct {
	// 连接配置
	URL    string `json:"url"`
	Token  string `json:"token"`
	Org    string `json:"org"`
	Bucket string `json:"bucket"`

	// 可选项
	UseGzip       bool          `json:"use_gzip,omitempty"`
	TLSSkipVerify bool          `json:"tls_skip_verify,omitempty"`
	PingTimeout   time.Duration `json:"ping_timeout,omitempty"` // 连接探活超时
}

type InfluxClient struct {
	config   InfluxConfig
	client   influxdb2.Client
	writeAPI influxapi.WriteAPIBlocking
	queryAPI influxapi.QueryAPI
}

// initialize 初始化InfluxDB客户端
func (i *InfluxClient) initialize() error {
	if i.config.URL == "" {
		return fmt.Errorf("influxdb url cannot be empty")
	}
	if i.config.Token == "" {
		return fmt.Errorf("influxdb token cannot be empty")
	}
	if i.config.Org == "" {
		return fmt.Errorf("influxdb org cannot be empty")
	}
	if i.config.Bucket == "" {
		return fmt.Errorf("influxdb bucket cannot be empty")
	}

	// 构建客户端选项
	options := influxdb2.DefaultOptions().
		SetUseGZip(i.config.UseGzip)

	// TLS配置
	if i.config.TLSSkipVerify {
		options = options.SetTLSConfig(&tls.Config{InsecureSkipVerify: true})
	}

	client := influxdb2.NewClientWithOptions(i.config.URL, i.config.Token, options)
	// 探活
	ctx, cancel := context.WithTimeout(context.Background(), getOrDefaultTimeout(i.config.PingTimeout))
	defer cancel()
	ok, err := client.Ping(ctx)
	if err != nil {
		client.Close()
		return fmt.Errorf("failed to ping influxdb: %w", err)
	}
	if !ok {
		client.Close()
		return fmt.Errorf("failed to ping influxdb: unreachable")
	}

	// 构建写入与查询API
	i.client = client
	i.writeAPI = client.WriteAPIBlocking(i.config.Org, i.config.Bucket)
	i.queryAPI = client.QueryAPI(i.config.Org)

	// 检查并确保Bucket存在；若不存在则尝试自动创建
	if err := i.ensureBucketExists(ctx); err != nil {
		fmt.Printf("Warning: could not ensure bucket exists: %v\n", err)
	}

	return nil
}

func getOrDefaultTimeout(t time.Duration) time.Duration {
	if t <= 0 {
		return 5 * time.Second
	}
	return t
}

// NewInfluxClient 创建InfluxDB客户端
func NewInfluxClient(cfg *InfluxConfig) (*InfluxClient, error) {
	client := &InfluxClient{config: *cfg}
	if err := client.initialize(); err != nil {
		return nil, fmt.Errorf("initialize influx client: %w", err)
	}
	return client, nil
}

// Close 关闭InfluxDB客户端
func (i *InfluxClient) Close() error {
	if i.client != nil {
		i.client.Close()
	}
	return nil
}

// -------- 写入功能 --------

// WritePoint 写入单个数据点（使用官方类型）
func (i *InfluxClient) WritePoint(p *write.Point) error {
	if i.writeAPI == nil {
		return fmt.Errorf("influx write api not initialized")
	}
	return i.writeAPI.WritePoint(context.Background(), p)
}

// WriteBatch 批量写入数据点
func (i *InfluxClient) WriteBatch(points []*write.Point) error {
	if len(points) == 0 {
		return nil
	}
	if i.writeAPI == nil {
		return fmt.Errorf("influx write api not initialized")
	}
	return i.writeAPI.WritePoint(context.Background(), points...)
}

// WriteLineProtocol 以Line Protocol写入
func (i *InfluxClient) WriteLineProtocol(lines []string) error {
	if len(lines) == 0 {
		return nil
	}
	if i.writeAPI == nil {
		return fmt.Errorf("influx write api not initialized")
	}
	return i.writeAPI.WriteRecord(context.Background(), lines...)
}

// -------- 查询功能 --------

// Query 执行Flux查询，返回每行记录的值映射
func (i *InfluxClient) Query(ctx context.Context, flux string) ([]map[string]interface{}, error) {
	if i.queryAPI == nil {
		return nil, fmt.Errorf("influx query api not initialized")
	}
	res, err := i.queryAPI.Query(ctx, flux)
	if err != nil {
		return nil, fmt.Errorf("query influxdb: %w", err)
	}
	defer res.Close()

	var rows []map[string]interface{}
	for res.Next() {
		record := res.Record()
		row := make(map[string]interface{})
		// 基本元信息
		row["_measurement"] = record.Measurement()
		row["_field"] = record.Field()
		row["_value"] = record.Value()
		row["_time"] = record.Time()
		// 标签
		for k, v := range record.Values() {
			row[k] = v
		}
		rows = append(rows, row)
	}
	if res.Err() != nil {
		return nil, fmt.Errorf("query result error: %w", res.Err())
	}
	return rows, nil
}

// ensureBucketExists 确保Bucket存在（失败仅警告，不中断功能）
func (i *InfluxClient) ensureBucketExists(ctx context.Context) error {
	if i.client == nil {
		return fmt.Errorf("influx client not initialized")
	}
	buckets := i.client.BucketsAPI()
	if buckets == nil {
		return fmt.Errorf("buckets api not available")
	}
	b, err := buckets.FindBucketByName(ctx, i.config.Bucket)
	if err != nil {
		return fmt.Errorf("find bucket: %w", err)
	}
	if b != nil {
		return nil
	}

	// 不存在则打印警告并创建
	fmt.Printf("Warning: bucket %q not found in org %q, creating...\n", i.config.Bucket, i.config.Org)
	orgAPI := i.client.OrganizationsAPI()
	if orgAPI == nil {
		return fmt.Errorf("organizations api not available")
	}
	var org *domain.Organization
	org, err = orgAPI.FindOrganizationByName(ctx, i.config.Org)
	if err != nil {
		return fmt.Errorf("find organization %s: %w", i.config.Org, err)
	}
	if org == nil {
		return fmt.Errorf("organization %s not found", i.config.Org)
	}
	if _, err := buckets.CreateBucketWithName(ctx, org, i.config.Bucket); err != nil {
		return fmt.Errorf("create bucket %s: %w", i.config.Bucket, err)
	}
	fmt.Printf("Info: bucket %q created successfully.\n", i.config.Bucket)
	return nil
}
