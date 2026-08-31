package s3

import (
	"context"
	"errors"
	"testing"
)

func TestParseMetadata(t *testing.T) {
	tests := []struct {
		name    string
		entries []string
		want    map[string]string
		wantErr error
	}{
		{
			name:    "空入参返回 nil",
			entries: nil,
			want:    nil,
		},
		{
			name:    "单个 key=value",
			entries: []string{"source-sha256=abc123"},
			want:    map[string]string{"source-sha256": "abc123"},
		},
		{
			name:    "多个 key=value 且键小写化",
			entries: []string{"source-sha256=abc", "Width=1920", "Height=1080"},
			want:    map[string]string{"source-sha256": "abc", "width": "1920", "height": "1080"},
		},
		{
			name:    "value 可以包含等号",
			entries: []string{"formula=a=b"},
			want:    map[string]string{"formula": "a=b"},
		},
		{
			name:    "key 首尾空白被去除",
			entries: []string{"  origin  =itb"},
			want:    map[string]string{"origin": "itb"},
		},
		{
			name:    "缺少 = 报错",
			entries: []string{"novalue"},
			wantErr: ErrInvalidMetadata,
		},
		{
			name:    "空 key 报错",
			entries: []string{"=value"},
			wantErr: ErrInvalidMetadata,
		},
		{
			name:    "仅空白的 key 报错",
			entries: []string{"   =value"},
			wantErr: ErrInvalidMetadata,
		},
		{
			name:    "重复 key 报错",
			entries: []string{"origin=itb", "origin=other"},
			wantErr: ErrInvalidMetadata,
		},
		{
			name:    "大小写不同的重复 key 报错",
			entries: []string{"Origin=itb", "origin=other"},
			wantErr: ErrInvalidMetadata,
		},
		{
			name:    "保留键 itb-sha256 报错",
			entries: []string{"itb-sha256=fake"},
			wantErr: ErrReservedMetadataKey,
		},
		{
			name:    "key 含控制字符报错",
			entries: []string{"bad\tkey=value"},
			wantErr: ErrInvalidMetadata,
		},
		{
			name:    "value 含控制字符报错",
			entries: []string{"key=bad\nvalue"},
			wantErr: ErrInvalidMetadata,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMetadata(tt.entries)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("got %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("metadata[%q] = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

func TestNormalizeMetadata(t *testing.T) {
	t.Run("空 map 返回 nil", func(t *testing.T) {
		got, err := NormalizeMetadata(nil)
		if err != nil || got != nil {
			t.Fatalf("got (%v, %v), want (nil, nil)", got, err)
		}
	})

	t.Run("不修改入参 map", func(t *testing.T) {
		input := map[string]string{"Origin": "itb"}
		if _, err := NormalizeMetadata(input); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, exists := input["origin"]; exists {
			t.Error("input map must not be mutated")
		}
	})

	t.Run("小写化后重复报错", func(t *testing.T) {
		_, err := NormalizeMetadata(map[string]string{"A": "1", "a": "2"})
		if !errors.Is(err, ErrInvalidMetadata) {
			t.Fatalf("got %v, want ErrInvalidMetadata", err)
		}
	})

	t.Run("保留键报错", func(t *testing.T) {
		_, err := NormalizeMetadata(map[string]string{"ITB-SHA256": "fake"})
		if !errors.Is(err, ErrReservedMetadataKey) {
			t.Fatalf("got %v, want ErrReservedMetadataKey", err)
		}
	})
}

// TestUploadMetadataAndHeaders 协议级断言：用户 metadata 以
// x-amz-meta-* 小写键写入，标准 HTTP 头原样携带，itb-sha256 始终在场。
func TestUploadMetadataAndHeaders(t *testing.T) {
	rec, client := newUploadTestServer(t, nil)
	path := writeUploadFixture(t)

	_, err := Upload(context.Background(), client, path, "hello.txt", &UploadOptions{
		CacheControl:       "no-cache",
		ContentDisposition: "attachment",
		ContentEncoding:    "gzip",
		Metadata: map[string]string{
			"source-sha256": "abc123",
			"Width":         "1920",
		},
	})
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}

	headers := rec.recordedPutHeaders()
	if got := headers.Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", got)
	}
	if got := headers.Get("Content-Disposition"); got != "attachment" {
		t.Errorf("Content-Disposition = %q, want attachment", got)
	}
	if got := headers.Get("Content-Encoding"); got != "gzip" {
		t.Errorf("Content-Encoding = %q, want gzip", got)
	}
	if got := headers.Get("x-amz-meta-source-sha256"); got != "abc123" {
		t.Errorf("x-amz-meta-source-sha256 = %q, want abc123", got)
	}
	if got := headers.Get("x-amz-meta-width"); got != "1920" {
		t.Errorf("x-amz-meta-width = %q, want 1920（key 小写化）", got)
	}
	if got := headers.Get("x-amz-meta-itb-sha256"); got != helloSHA256 {
		t.Errorf("x-amz-meta-itb-sha256 = %q, want %q", got, helloSHA256)
	}
}

// TestUploadRejectsReservedMetadataBeforeRequests 保留键在发起任何
// 请求之前被拒绝。
func TestUploadRejectsReservedMetadataBeforeRequests(t *testing.T) {
	rec, client := newUploadTestServer(t, nil)
	path := writeUploadFixture(t)

	_, err := Upload(context.Background(), client, path, "hello.txt", &UploadOptions{
		Metadata: map[string]string{"itb-sha256": "fake"},
	})
	if !errors.Is(err, ErrReservedMetadataKey) {
		t.Fatalf("got %v, want ErrReservedMetadataKey", err)
	}
	assertMethods(t, rec.snapshotMethods(), nil)
}
