package logger

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"
)

func TestLogUtilForConcurrency(t *testing.T) {
	klog := NewKeLog(WithLevel(INFO), WithLogFormat(JsonFormat), WithLogFileName("test.log"))
	for i := 0; i < 100; i++ {
		ctx := context.Background()
		go AddLogRecord(ctx, klog, strconv.Itoa(i))
	}
	time.Sleep(30 * time.Second)
	klog.Close()
}

func AddLogRecord(ctx context.Context, klog *KeLog, traceId string) {
	ctx = klog.AddAttrWithContext(ctx, "traceId", traceId)
	klog.DebugWithContext(ctx, "greeting", "name", "tony")
	klog.InfoWithContext(ctx, "greeting", "name", "tony")
	klog.ErrorWithContext(ctx, "oops", "err", net.ErrClosed, "status", 500)
}

func TestLogUtil(t *testing.T) {
	klog := NewKeLog(WithLevel(INFO), WithLogFormat(JsonFormat), WithLogFileName("test.log"))
	klog.AddAttrs("traceId", "123456")
	klog.Debug("greeting", "name", "tony")
	klog.Info("greeting", "name", "tony")
	klog.Error("oops", "err", net.ErrClosed, "status", 500)
}
