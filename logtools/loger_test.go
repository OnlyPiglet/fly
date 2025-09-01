package logtools

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"
)

func TestLogUtilForConcurrency(t *testing.T) {
	klog := NewXLog(WithLevel(INFO), WithLogFormat(JsonFormat), WithFileName("test.log"))
	for i := 0; i < 100; i++ {
		ctx := context.Background()
		go AddLogRecord(ctx, klog, strconv.Itoa(i))
	}
	time.Sleep(30 * time.Second)
	klog.Close()
}

func AddLogRecord(ctx context.Context, klog *Log, traceId string) {
	ctx = klog.AddAttrWithContext(ctx, "traceId", traceId)
	klog.DebugWithContext(ctx, "greeting", "name", "tony")
	klog.InfoWithContext(ctx, "greeting", "name", "tony")
	klog.ErrorWithContext(ctx, "oops", "err", net.ErrClosed, "status", 500)
}

func TestLogUtil(t *testing.T) {
	klog := NewXLog(WithLevel(INFO), WithLogFormat(JsonFormat), WithFileName("test.log"))
	klog.AddAttrs("traceId", "123456")
	klog.Debug("greeting", "name", "tony")
	klog.Info("greeting", "name", "tony")
	klog.Error("oops", "err", net.ErrClosed, "status", 500)
}

func TestLogUtilWithFields(t *testing.T) {
	// 创建预设字段
	fields := []map[string]interface{}{
		{"service": "test-service"},
		{"version": 123},
		{"environment": "test"},
		{"component": "logger"},
	}

	// 使用 WithFields 创建 logger
	klog := NewXLog(
		WithLevel(INFO),
		WithLogFormat(JsonFormat),
		WithFileName("test_with_fields.log"),
		WithFields(fields),
	)

	// 测试各种日志级别，预设字段应该自动包含在每条日志中
	klog.Debug("debug message", "debugKey", "debugValue")
	klog.Info("info message", "userId", "12345", "action", "login")
	klog.Warn("warning message", "warningType", "performance", "threshold", "80%")
	klog.Error("error message", "err", net.ErrClosed, "status", 500, "retries", 3)

	// 测试格式化日志方法
	klog.Infof("formatted info: user %s performed %s", "john_doe", "logout")
	klog.Errorf("formatted error: failed to connect to %s after %d attempts", "database", 5)

	klog.Close()
}
