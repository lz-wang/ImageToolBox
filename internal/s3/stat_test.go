package s3

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	smithyhttp "github.com/aws/smithy-go/transport/http"
)

func TestFormatStatOutputTable(t *testing.T) {
	tests := []struct {
		name    string
		info    *StatInfo
		want    []string
		notWant []string
	}{
		{
			name: "完整字段",
			info: &StatInfo{
				Key:          "images/foo.jpg",
				Size:         2491820,
				LastModified: time.Date(2026, 8, 28, 12, 35, 24, 0, time.UTC),
				ETag:         "\"abc123\"",
				ContentType:  "image/jpeg",
				StorageClass: "STANDARD",
				Metadata:     map[string]string{"itb-sha256": "deadbeef"},
			},
			want: []string{
				"Key           images/foo.jpg",
				"Size          2.4 MiB",
				"Last Modified 2026-08-28 12:35:24",
				"ETag          \"abc123\"",
				"Content-Type  image/jpeg",
				"Storage Class STANDARD",
				"Metadata[itb-sha256]  deadbeef",
			},
		},
		{
			name: "可选字段为空时省略",
			info: &StatInfo{
				Key:  "foo.txt",
				Size: 12,
				ETag: "\"x\"",
			},
			want: []string{
				"Key           foo.txt",
				"Size          12 B",
				"ETag          \"x\"",
			},
			notWant: []string{"Content-Type", "Storage Class", "Metadata["},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatStatOutput(tt.info, "table")
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Errorf("table output missing %q:\n%s", want, got)
				}
			}
			for _, notWant := range tt.notWant {
				if strings.Contains(got, notWant) {
					t.Errorf("table output should not contain %q:\n%s", notWant, got)
				}
			}
		})
	}
}

func TestFormatStatOutputJSON(t *testing.T) {
	info := &StatInfo{
		SchemaVersion: StatSchemaVersion,
		Key:           "foo/bar.jpg",
		Size:          2491820,
		LastModified:  time.Date(2026, 8, 28, 4, 35, 24, 0, time.UTC),
		ETag:          "\"...\"",
		ContentType:   "image/jpeg",
		StorageClass:  "STANDARD",
		Metadata:      map[string]string{"origin": "itb"},
	}

	var decoded StatInfo
	if err := json.Unmarshal([]byte(FormatStatOutput(info, "json")), &decoded); err != nil {
		t.Fatalf("json output not valid json: %v", err)
	}
	if decoded.Key != info.Key || decoded.Size != info.Size || decoded.ContentType != info.ContentType {
		t.Errorf("round-trip mismatch: %+v", decoded)
	}
	if decoded.Metadata["origin"] != "itb" {
		t.Errorf("metadata lost in json output: %+v", decoded.Metadata)
	}
	// 机器可读契约：JSON 必须携带 schema_version 供脚本判别版本
	if decoded.SchemaVersion != StatSchemaVersion {
		t.Errorf("schema_version = %q, want %q", decoded.SchemaVersion, StatSchemaVersion)
	}
	if table := FormatStatOutput(info, "table"); strings.Contains(table, "schema_version") {
		t.Errorf("table output should not leak schema_version:\n%s", table)
	}
}

// smithyResponseError 构造仅含状态码的 Smithy 响应错误，
// 用于模拟 HeadObject 不带 NoSuchKey typed error 的 404/403 响应。
func smithyResponseError(statusCode int) *smithyhttp.ResponseError {
	return &smithyhttp.ResponseError{
		Response: &smithyhttp.Response{
			Response: &http.Response{StatusCode: statusCode},
		},
		Err: fmt.Errorf("api error"),
	}
}

func TestWrapErrorHTTPStatus(t *testing.T) {
	tests := []struct {
		name            string
		err             error
		wantNotFound    bool
		wantAccessDeny  bool
		wantMessagePart string
	}{
		{
			name:         "404 映射为对象不存在",
			err:          smithyResponseError(http.StatusNotFound),
			wantNotFound: true,
		},
		{
			name:           "403 保留为权限错误而非对象不存在",
			err:            smithyResponseError(http.StatusForbidden),
			wantAccessDeny: true,
		},
		{
			name: "500 原样返回",
			err:  smithyResponseError(http.StatusInternalServerError),
		},
		{
			name: "普通错误原样返回",
			err:  errors.New("boom"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapped := WrapError(tt.err)
			if got := errors.Is(wrapped, ErrObjectNotFound); got != tt.wantNotFound {
				t.Errorf("errors.Is(ErrObjectNotFound) = %v, want %v (%v)", got, tt.wantNotFound, wrapped)
			}
			if got := errors.Is(wrapped, ErrAccessDenied); got != tt.wantAccessDeny {
				t.Errorf("errors.Is(ErrAccessDenied) = %v, want %v (%v)", got, tt.wantAccessDeny, wrapped)
			}
			if tt.wantNotFound && tt.wantAccessDeny {
				t.Error("404 与 403 语义不可同时成立")
			}
		})
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KiB"},
		{2491820, "2.4 MiB"},
		{5 * 1024 * 1024 * 1024, "5.0 GiB"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := formatBytes(tt.bytes); got != tt.want {
				t.Errorf("formatBytes(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}
