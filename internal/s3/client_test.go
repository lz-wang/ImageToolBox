package s3

import (
	"net/http"
	"testing"
)

// TestNewHTTPClientNoTotalTimeout 锁定设计约束：http.Client.Timeout
// 是覆盖整个请求（含大文件 body 传输）的总超时，必须保持为 0，
// 避免大文件上传/下载在固定时限处被截断。
func TestNewHTTPClientNoTotalTimeout(t *testing.T) {
	client := newHTTPClient()

	if client.Timeout != 0 {
		t.Fatalf("http client total timeout = %v, want 0", client.Timeout)
	}
}

// TestNewHTTPClientTransportTimeouts 确保 Transport 层超时已配置：
// 只限制等待响应头与空闲连接回收，不涉及 body 传输时长。
func TestNewHTTPClientTransportTimeouts(t *testing.T) {
	client := newHTTPClient()

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("expected *http.Transport")
	}

	if transport.ResponseHeaderTimeout <= 0 {
		t.Error("ResponseHeaderTimeout must be configured")
	}
	if transport.IdleConnTimeout <= 0 {
		t.Error("IdleConnTimeout must be configured")
	}
}
