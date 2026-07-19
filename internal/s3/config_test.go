package s3

import (
	"errors"
	"strings"
	"testing"
)

func TestLoadFromEnv(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		env    map[string]string
		want   Config
	}{
		{
			name: "flag 已设值时 env 不覆盖",
			config: Config{
				Endpoint:        "flag-endpoint",
				AccessKeyID:     "flag-ak",
				SecretAccessKey: "flag-sk",
				Region:          "flag-region",
			},
			env: map[string]string{
				"ITB_S3_ENDPOINT":          "env-endpoint",
				"ITB_S3_ACCESS_KEY_ID":     "env-ak",
				"ITB_S3_SECRET_ACCESS_KEY": "env-sk",
				"ITB_S3_REGION":            "env-region",
			},
			want: Config{
				Endpoint:        "flag-endpoint",
				AccessKeyID:     "flag-ak",
				SecretAccessKey: "flag-sk",
				Region:          "flag-region",
			},
		},
		{
			name:   "flag 为空时从 ITB_S3_* 读取",
			config: Config{},
			env: map[string]string{
				"ITB_S3_ENDPOINT":          "env-endpoint",
				"ITB_S3_ACCESS_KEY_ID":     "env-ak",
				"ITB_S3_SECRET_ACCESS_KEY": "env-sk",
				"ITB_S3_REGION":            "env-region",
			},
			want: Config{
				Endpoint:        "env-endpoint",
				AccessKeyID:     "env-ak",
				SecretAccessKey: "env-sk",
				Region:          "env-region",
			},
		},
		{
			name:   "region 为空且无 env 时 fallback us-east-1",
			config: Config{},
			env: map[string]string{
				"ITB_S3_ENDPOINT":          "e",
				"ITB_S3_ACCESS_KEY_ID":     "a",
				"ITB_S3_SECRET_ACCESS_KEY": "s",
			},
			want: Config{
				Endpoint:        "e",
				AccessKeyID:     "a",
				SecretAccessKey: "s",
				Region:          "us-east-1",
			},
		},
		{
			name:   "旧变量名 S3_* 不再被读取",
			config: Config{},
			env: map[string]string{
				"S3_ENDPOINT":              "old-endpoint",
				"S3_ACCESS_KEY_ID":         "old-ak",
				"S3_SECRET_ACCESS_KEY":     "old-sk",
				"S3_REGION":                "old-region",
				"ITB_S3_ENDPOINT":          "",
				"ITB_S3_ACCESS_KEY_ID":     "",
				"ITB_S3_SECRET_ACCESS_KEY": "",
				"ITB_S3_REGION":            "",
			},
			want: Config{Region: "us-east-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			cfg := tt.config
			cfg.LoadFromEnv()
			if cfg != tt.want {
				t.Fatalf("got %+v, want %+v", cfg, tt.want)
			}
		})
	}
}

func TestLoadFromEnv_ForcePathStyle(t *testing.T) {
	tests := []struct {
		name      string
		endpoint  string
		force     bool
		wantForce bool
	}{
		{"localhost 自动启用", "http://localhost:9000", false, true},
		{"127.0.0.1 自动启用", "http://127.0.0.1:9000", false, true},
		{":9000 自动启用", "http://minio:9000", false, true},
		{"普通端点不启用", "https://s3.amazonaws.com", false, false},
		{"已显式启用保持不变", "https://s3.amazonaws.com", true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{Endpoint: tt.endpoint, ForcePathStyle: tt.force}
			cfg.LoadFromEnv()
			if cfg.ForcePathStyle != tt.wantForce {
				t.Fatalf("ForcePathStyle got %v, want %v", cfg.ForcePathStyle, tt.wantForce)
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
