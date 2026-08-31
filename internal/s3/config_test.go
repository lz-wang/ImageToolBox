package s3

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want Config
	}{
		{
			name: "region 为空时默认 us-east-1",
			cfg:  Config{Endpoint: "e", AccessKeyID: "a", SecretAccessKey: "s", Bucket: "b"},
			want: Config{Endpoint: "e", AccessKeyID: "a", SecretAccessKey: "s", Region: "us-east-1", Bucket: "b"},
		},
		{
			name: "region 已设值时保持不变",
			cfg:  Config{Endpoint: "e", Region: "cn-north-1"},
			want: Config{Endpoint: "e", Region: "cn-north-1"},
		},
		{
			name: "localhost 自动启用路径样式",
			cfg:  Config{Endpoint: "http://localhost:9000"},
			want: Config{Endpoint: "http://localhost:9000", Region: "us-east-1", ForcePathStyle: true},
		},
		{
			name: "127.0.0.1 自动启用路径样式",
			cfg:  Config{Endpoint: "http://127.0.0.1:9000"},
			want: Config{Endpoint: "http://127.0.0.1:9000", Region: "us-east-1", ForcePathStyle: true},
		},
		{
			name: ":9000 自动启用路径样式",
			cfg:  Config{Endpoint: "http://minio:9000"},
			want: Config{Endpoint: "http://minio:9000", Region: "us-east-1", ForcePathStyle: true},
		},
		{
			name: "普通端点不启用路径样式",
			cfg:  Config{Endpoint: "https://s3.amazonaws.com"},
			want: Config{Endpoint: "https://s3.amazonaws.com", Region: "us-east-1"},
		},
		{
			name: "已显式启用保持不变",
			cfg:  Config{Endpoint: "https://s3.amazonaws.com", ForcePathStyle: true},
			want: Config{Endpoint: "https://s3.amazonaws.com", Region: "us-east-1", ForcePathStyle: true},
		},
		{
			name: "空端点不触发检测",
			cfg:  Config{},
			want: Config{Region: "us-east-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.cfg
			cfg.Normalize()
			if cfg != tt.want {
				t.Fatalf("got %+v, want %+v", cfg, tt.want)
			}
		})
	}
}

// TestNormalizeEndpointDetection 锁定端点 → 路径样式的自动检测：
// 基于 url.Parse 的 Hostname/Port 精确判断，URL 中的子串（查询参数、
// 路径）不得误命中；9000 作为 MinIO 默认端口触发路径样式。
func TestNormalizeEndpointDetection(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		want     bool
	}{
		{"localhost 启用路径样式", "http://localhost:9000", true},
		{"127.0.0.1 启用路径样式", "http://127.0.0.1:9000", true},
		{"IPv6 loopback 启用路径样式", "http://[::1]:9000", true},
		{"MinIO 默认端口 9000 启用路径样式", "http://minio.internal:9000", true},
		{"非 loopback 且非 9000 端口不启用", "http://minio.internal:9001", false},
		{"普通公网端点不启用", "https://s3.example.com", false},
		{"查询参数包含 localhost 不误命中", "https://example.com/?redirect=localhost", false},
		{"路径包含 127.0.0.1 不误命中", "https://example.com/127.0.0.1/asset", false},
		{"端口 9000 但主机名为 localhost 也启用", "http://localhost:8080", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{Endpoint: tt.endpoint}
			cfg.Normalize()
			if cfg.ForcePathStyle != tt.want {
				t.Fatalf("ForcePathStyle = %v, want %v", cfg.ForcePathStyle, tt.want)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr error
	}{
		{"缺 endpoint", Config{AccessKeyID: "a", SecretAccessKey: "s", Bucket: "b"}, ErrMissingEndpoint},
		{"缺凭证", Config{Endpoint: "e", Bucket: "b"}, ErrMissingCredentials},
		{"缺 bucket", Config{Endpoint: "e", AccessKeyID: "a", SecretAccessKey: "s"}, ErrMissingBucket},
		{"全部合法", Config{Endpoint: "e", AccessKeyID: "a", SecretAccessKey: "s", Bucket: "b"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == nil && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("got %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateWithoutBucket(t *testing.T) {
	cfgOK := Config{Endpoint: "e", AccessKeyID: "a", SecretAccessKey: "s"}
	if err := cfgOK.ValidateWithoutBucket(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cfgNoEndpoint := Config{AccessKeyID: "a", SecretAccessKey: "s"}
	if err := cfgNoEndpoint.ValidateWithoutBucket(); err != ErrMissingEndpoint {
		t.Fatalf("got %v, want %v", err, ErrMissingEndpoint)
	}
}

func TestErrMissingCredentialsMessage(t *testing.T) {
	msg := ErrMissingCredentials.Error()
	if !strings.Contains(msg, "ITB_S3_ACCESS_KEY_ID") {
		t.Fatalf("error message should mention ITB_S3_ACCESS_KEY_ID, got: %s", msg)
	}
	if !strings.Contains(msg, "ITB_S3_SECRET_ACCESS_KEY") {
		t.Fatalf("error message should mention ITB_S3_SECRET_ACCESS_KEY, got: %s", msg)
	}
}
