package s3

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/aws/smithy-go"
)

// fakeSmithyHTTPError 构造带 provider code 的 smithy.APIError。
type fakeSmithyHTTPError struct {
	code string
}

func (e *fakeSmithyHTTPError) Error() string           { return "api error " + e.code }
func (e *fakeSmithyHTTPError) ErrorCode() string       { return e.code }
func (e *fakeSmithyHTTPError) ErrorMessage() string    { return e.code }
func (e *fakeSmithyHTTPError) ErrorFault() smithy.ErrorFault {
	return smithy.FaultClient
}

// TestConditionalWriteErrorClassification 锁定条件写响应分类。
func TestConditionalWriteErrorClassification(t *testing.T) {
	tests := []struct {
		name string
		code string
		want string
	}{
		{"412 PreconditionFailed", "PreconditionFailed", "precondition_failed"},
		{"409 ConditionalRequestConflict", "ConditionalRequestConflict", "conflict"},
		{"501 NotImplemented", "NotImplemented", "unsupported"},
		{"500 InternalError", "InternalError", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &fakeSmithyHTTPError{code: tt.code}
			if got := conditionalWriteError(err); got != tt.want {
				t.Errorf("conditionalWriteError = %q, want %q", got, tt.want)
			}
		})
	}
}

// conditionalUploadServer 模拟支持条件写的存储：
//   - If-None-Match="*" 且键已存在 → 412 PreconditionFailed
//   - 其余 PUT → 正常写入并回放 header
type conditionalUploadServer struct {
	exists bool
	header http.Header
	body   []byte
}

func (s *conditionalUploadServer) newClient(t *testing.T) *Client {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			if s.exists && r.Header.Get("If-None-Match") == "*" {
				w.WriteHeader(http.StatusPreconditionFailed)
				_, _ = io.WriteString(w, `<Error><Code>PreconditionFailed</Code></Error>`)
				return
			}
			s.body = body
			s.header = headReplayHeaders(r.Header)
			s.exists = true
			w.WriteHeader(http.StatusOK)
		case http.MethodHead:
			if !s.exists {
				w.WriteHeader(http.StatusNotFound)
				_, _ = io.WriteString(w, `<Error><Code>NoSuchKey</Code></Error>`)
				return
			}
			for name, vals := range s.header {
				for _, v := range vals {
					w.Header().Add(name, v)
				}
			}
			w.Header().Set("Content-Length", strconv.Itoa(len(s.body)))
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	t.Cleanup(server.Close)

	cfg := &Config{
		Endpoint: server.URL, AccessKeyID: "ak", SecretAccessKey: "sk",
		Region: "us-east-1", Bucket: "test-bucket", ForcePathStyle: true,
	}
	client, err := NewClient(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client
}

// TestConditionalUploadCreatesWhenMissing 键不存在：条件 PUT 成功。
func TestConditionalUploadCreatesWhenMissing(t *testing.T) {
	server := &conditionalUploadServer{}
	client := server.newClient(t)
	path := writeUploadFixture(t)

	result, err := Upload(context.Background(), client, path, "hello.txt", &UploadOptions{
		IfExists: IfExistsVerify,
	})
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if result.Status != StatusUploaded {
		t.Fatalf("status = %q, want uploaded", result.Status)
	}
	if string(server.body) != helloContent {
		t.Errorf("PUT body = %q", server.body)
	}
}

// TestConditionalUploadReusesIdenticalExisting 对象已存在且状态完全
// 一致：412 → HEAD 匹配 → status=reused。
func TestConditionalUploadReusesIdenticalExisting(t *testing.T) {
	server := &conditionalUploadServer{}
	client := server.newClient(t)
	path := writeUploadFixture(t)

	// 预置完全相同内容的对象
	if _, err := Upload(context.Background(), client, path, "hello.txt", nil); err != nil {
		t.Fatalf("seed upload: %v", err)
	}

	result, err := Upload(context.Background(), client, path, "hello.txt", &UploadOptions{
		IfExists: IfExistsVerify,
	})
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if result.Status != StatusReused || !result.Skipped {
		t.Fatalf("status = %q skipped = %v, want reused/true", result.Status, result.Skipped)
	}
	if result.SHA256 != helloSHA256 {
		t.Errorf("sha256 = %q", result.SHA256)
	}
}

// TestConditionalUploadConflictsOnDifferentExisting 对象存在但内容
// 不一致：E_TARGET_CONFLICT（ErrExpectationMismatch），绝不覆盖。
func TestConditionalUploadConflictsOnDifferentExisting(t *testing.T) {
	server := &conditionalUploadServer{}
	client := server.newClient(t)
	path := writeUploadFixture(t)

	// 预置"不同内容"的对象
	different := writeContentFixture(t, "other.txt", []byte("different content"))
	if _, err := Upload(context.Background(), client, different, "hello.txt", nil); err != nil {
		t.Fatalf("seed upload: %v", err)
	}

	result, err := Upload(context.Background(), client, path, "hello.txt", &UploadOptions{
		IfExists: IfExistsVerify,
	})
	if err == nil || !errors.Is(err, ErrExpectationMismatch) {
		t.Fatalf("err = %v, want ErrExpectationMismatch", err)
	}
	if result != nil {
		t.Fatal("conflict must not return a result")
	}
	if string(server.body) != "different content" {
		t.Error("existing object must remain untouched on conflict")
	}
}

// TestConditionalUploadUnsupportedCapability provider 返回 501
// NotImplemented 时必须以 ErrUnsupportedCapability 失败，
// 绝不降级为非条件 PUT。
func TestConditionalUploadUnsupportedCapability(t *testing.T) {
	var conditionalPuts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			if r.Header.Get("If-None-Match") == "*" {
				conditionalPuts++
				w.WriteHeader(http.StatusNotImplemented)
				_, _ = io.WriteString(w, `<Error><Code>NotImplemented</Code><Message>A header you provided implies functionality that is not implemented</Message></Error>`)
				return
			}
			t.Errorf("degraded to a non-conditional PUT — forbidden")
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	cfg := &Config{
		Endpoint: srv.URL, AccessKeyID: "ak", SecretAccessKey: "sk",
		Region: "us-east-1", Bucket: "test-bucket", ForcePathStyle: true,
		MaxAttempts: 1,
	}
	client, err := NewClient(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	path := writeUploadFixture(t)

	_, err = Upload(context.Background(), client, path, "hello.txt", &UploadOptions{
		IfExists: IfExistsVerify,
	})
	if err == nil || !errors.Is(err, ErrUnsupportedCapability) {
		t.Fatalf("err = %v, want ErrUnsupportedCapability", err)
	}
	if conditionalPuts != 1 {
		t.Errorf("conditional PUT attempts = %d, want exactly 1 (no non-conditional retry)", conditionalPuts)
	}
}

// TestConditionalUploadConflictExhaustsRetries 409 冲突重试耗尽后
// 返回冲突错误。
func TestConditionalUploadConflictExhaustsRetries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && r.Header.Get("If-None-Match") == "*" {
			w.WriteHeader(http.StatusConflict)
			_, _ = io.WriteString(w, `<Error><Code>ConditionalRequestConflict</Code><Message>The conditional request cannot succeed due to a competing conditional request</Message></Error>`)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	cfg := &Config{
		Endpoint: srv.URL, AccessKeyID: "ak", SecretAccessKey: "sk",
		Region: "us-east-1", Bucket: "test-bucket", ForcePathStyle: true,
		MaxAttempts: 2,
	}
	client, err := NewClient(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	path := writeUploadFixture(t)

	_, err = Upload(context.Background(), client, path, "hello.txt", &UploadOptions{IfExists: IfExistsVerify})
	if err == nil || !errors.Is(err, ErrExpectationMismatch) {
		t.Fatalf("err = %v, want ErrExpectationMismatch after retry exhaustion", err)
	}
}

// TestUploadInvalidIfExists 非法 --if-exists 在参数阶段报错。
func TestUploadInvalidIfExists(t *testing.T) {
	_, client := newUploadTestServer(t, nil)
	path := writeUploadFixture(t)

	_, err := Upload(context.Background(), client, path, "hello.txt", &UploadOptions{IfExists: "fail"})
	if err == nil || !errors.Is(err, ErrInvalidIfExists) {
		t.Fatalf("err = %v, want ErrInvalidIfExists", err)
	}
}
