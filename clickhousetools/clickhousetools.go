package clickhousetools

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// ClickHouseConfig Addresses ck集群链接地址 []string{"ch1:9000", "ch2:9000", "ch3:9000"}
type ClickHouseConfig struct {
	Addresses  []string `json:"address"`
	Username   string   `json:"username"`
	Password   string   `json:"password"`
	Database   string   `json:"database"`
	InitTables []string `json:"init_tables"`
}

type ClickHouseClient struct {
	config ClickHouseConfig
	conn   driver.Conn
}

// initialize 初始化数据库与表
func (c *ClickHouseClient) initialize() error {
	err := c.execWithTimeout(2*time.Second, fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", c.config.Database))
	if err != nil {
		return err
	}
	
	// 切换到目标数据库
	err = c.execWithTimeout(2*time.Second, fmt.Sprintf("USE %s", c.config.Database))
	if err != nil {
		return err
	}
	
	if c.config.InitTables != nil {
		for _, it := range c.config.InitTables {
			err := c.execWithTimeout(2*time.Second, it)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// execWithTimeout 独立超时执行器
func (c *ClickHouseClient) execWithTimeout(timeout time.Duration, exec string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return c.conn.Exec(ctx, exec)
}

func NewClickHouseClient(chc *ClickHouseConfig) (*ClickHouseClient, error) {
	chci := &ClickHouseClient{config: *chc}
	var err error
	
	// 首先连接到默认数据库进行初始化
	chci.conn, err = clickhouse.Open(&clickhouse.Options{
		Protocol: clickhouse.Native,
		Addr:     chc.Addresses,
		Auth: clickhouse.Auth{
			Username: chc.Username,
			Password: chc.Password,
			Database: "default", // 先连接到默认数据库
		},
		DialTimeout:      3 * time.Second,
		ReadTimeout:      3 * time.Minute,
		MaxIdleConns:     32,
		MaxOpenConns:     128,
		ConnMaxLifetime:  25 * time.Minute,
		ConnOpenStrategy: clickhouse.ConnOpenRoundRobin,
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
		MaxCompressionBuffer: 32 << 20, //32MiB
		FreeBufOnConnRelease: false,
	})
	if err != nil {
		return nil, err
	}
	
	// 可选：健康检查
	if err := chci.conn.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("clickhouse ping: %w", err)
	}

	// 可选：初始化库表
	if err := chci.initialize(); err != nil {
		return nil, fmt.Errorf("initialize schema: %w", err)
	}
	
	// 重新连接到目标数据库
	chci.conn.Close()
	chci.conn, err = clickhouse.Open(&clickhouse.Options{
		Protocol: clickhouse.Native,
		Addr:     chc.Addresses,
		Auth: clickhouse.Auth{
			Username: chc.Username,
			Password: chc.Password,
			Database: chc.Database,
		},
		DialTimeout:      3 * time.Second,
		ReadTimeout:      3 * time.Minute,
		MaxIdleConns:     32,
		MaxOpenConns:     128,
		ConnMaxLifetime:  25 * time.Minute,
		ConnOpenStrategy: clickhouse.ConnOpenRoundRobin,
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
		MaxCompressionBuffer: 32 << 20, //32MiB
		FreeBufOnConnRelease: false,
	})
	if err != nil {
		return nil, err
	}
	
	return chci, nil
}

// Close 关闭 ClickHouse 连接
func (c *ClickHouseClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Query 执行查询并返回结果
func (c *ClickHouseClient) Query(ctx context.Context, query string, args ...interface{}) (driver.Rows, error) {
	return c.conn.Query(ctx, query, args...)
}

// -------- 写入：通用 AddData --------

// WithBatchOptions 为单次 AddData 提供可选配置
type WithBatchOptions struct {
	Timeout         time.Duration  // 单次操作超时
	BlockBufferSize uint8          // 覆盖客户端块大小（默认 2；批量可设 8~32）
	AsyncInsert     bool           // 是否启用 async_insert
	WaitAsyncInsert bool           // async_insert 后是否等待确认
	Settings        map[string]any // 额外 Settings
}

// AddData 使用 AppendStruct 批量写入任意结构体切片（要求结构体字段带 ch tag 对齐列名）
// 示例：AddData(client, "db.events", []Event{{...}, {...}}, WithBatchOptions{BlockBufferSize:16})
func AddData[T any](c *ClickHouseClient, table string, rows []T, opt ...WithBatchOptions) error {
	if len(rows) == 0 {
		return nil
	}

	// 1) 组织上下文：超时 + 释放连接 + 本次 Settings
	var o WithBatchOptions
	if len(opt) > 0 {
		o = opt[0]
	}
	if o.Timeout <= 0 {
		o.Timeout = 2 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), o.Timeout)
	defer cancel()

	// Settings：按需打开 async_insert
	settings := clickhouse.Settings{}
	for k, v := range o.Settings {
		settings[k] = v
	}
	if o.AsyncInsert {
		settings["async_insert"] = 1
		if o.WaitAsyncInsert {
			settings["wait_for_async_insert"] = 1
		} else {
			settings["wait_for_async_insert"] = 0
		}
	}

	// 2) PrepareBatch：仅对本次操作覆盖 BlockBufferSize；并在完成后释放连接
	bctx := clickhouse.Context(ctx,
		clickhouse.WithSettings(settings),
	)
	if o.BlockBufferSize > 0 {
		bctx = clickhouse.Context(bctx, clickhouse.WithBlockBufferSize(o.BlockBufferSize))
	}

	batch, err := c.conn.PrepareBatch(bctx, fmt.Sprintf("INSERT INTO %s", table))
	if err != nil {
		return err
	}

	// 3) 逐条 AppendStruct（行式）；如需极致性能可改为列式 Append(Column)
	for i := range rows {
		if err := batch.AppendStruct(&rows[i]); err != nil {
			return fmt.Errorf("append struct #%d: %w", i, err)
		}
	}

	// 4) 发送
	return batch.Send()
}
