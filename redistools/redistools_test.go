package redistools

import (
	"crypto/tls"
	"testing"
	"time"
)

func TestGetRedisOpt(t *testing.T) {
	o := defaultOptions()
	o.dialTimeout = time.Second
	o.tlsConfig = &tls.Config{InsecureSkipVerify: true}

	opt, err := getRedisOpt("localhost:6379", o)
	if err != nil {
		t.Fatalf("getRedisOpt error: %v", err)
	}
	if opt.Addr != "localhost:6379" {
		t.Errorf("unexpected addr: %s", opt.Addr)
	}
	if opt.DB != 0 {
		t.Errorf("expected DB 0, got %d", opt.DB)
	}
	if opt.DialTimeout != o.dialTimeout {
		t.Errorf("expected dial timeout %v, got %v", o.dialTimeout, opt.DialTimeout)
	}
	if opt.TLSConfig == nil || !opt.TLSConfig.InsecureSkipVerify {
		t.Errorf("TLS config not applied")
	}
}

func TestCloseNil(t *testing.T) {
	if err := Close(nil); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}
