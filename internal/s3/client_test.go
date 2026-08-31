package s3

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestSessionTokenSigning 协议级验证临时凭证：配置 SessionToken 后，
// 签名请求必须携带 X-Amz-Security-Token；长期凭证（token 为空）则
// 不携带该头。
func TestSessionTokenSigning(t *testing.T) {
	const token = "test-session-token"

	tests := []struct {
		name      string
		session   string
		wantToken string
	}{
		{"临时凭证请求携带 X-Amz-Security-Token", token, token},
		{"长期凭证不携带 X-Amz-Security-Token", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotToken string
			var sawPut bool
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPut {
					gotToken = r.Header.Get("X-Amz-Security-Token")
					sawPut = true
				}
				w.WriteHeader(http.StatusOK)
			}))
			t.Cleanup(srv.Close)

			cfg := &Config{
				Endpoint:        srv.URL,
				AccessKeyID:     "test-access-key",
				SecretAccessKey: "test-secret-key",
				SessionToken:    tt.session,
				Region:          "us-east-1",
				Bucket:          "test-bucket",
				ForcePathStyle:  true,
			}
			client, err := NewClient(context.Background(), cfg)
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}

			path := writeUploadFixture(t)
			if _, err := Upload(context.Background(), client, path, "hello.txt", nil); err != nil {
				t.Fatalf("Upload: %v", err)
			}
			if !sawPut {
				t.Fatal("no PUT request recorded")
			}
			if gotToken != tt.wantToken {
				t.Errorf("X-Amz-Security-Token = %q, want %q", gotToken, tt.wantToken)
			}
		})
	}
}

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
