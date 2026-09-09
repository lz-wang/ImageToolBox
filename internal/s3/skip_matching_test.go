package s3

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// TestMatchesExpectedState 锁定完整状态比较语义：requested subset
// matching（远端多出的 metadata 无关；未指定的 header = don't care）。
func TestMatchesExpectedState(t *testing.T) {
	newRemote := func() *StatInfo {
		return &StatInfo{
			Key:         "k",
			Size:        6,
			ContentType: "text/plain; charset=utf-8",
			Metadata: map[string]string{
				MetadataSHA256Key: helloSHA256,
				"width":           "1920",
				"legacy":          "extra",
			},
		}
	}
	expect := uploadExpectations{
		Size:         6,
		ContentType:  "text/plain; charset=utf-8",
		CacheControl: "no-cache",
		Metadata: map[string]string{
			MetadataSHA256Key: helloSHA256,
			"width":           "1920",
		},
	}

	t.Run("远端缺 header 视为不一致", func(t *testing.T) {
		remote := newRemote()
		remote.CacheControl = ""
		if matchesExpectedState(remote, expect) {
			t.Fatal("explicit cache-control mismatch must not match")
		}
	})

	t.Run("远端多出的 metadata 不影响匹配", func(t *testing.T) {
		relaxed := expect
		relaxed.CacheControl = "" // 本子测试只关注 metadata 子集语义
		if !matchesExpectedState(newRemote(), relaxed) {
			t.Fatal("extra remote metadata must not break matching")
		}
	})

	t.Run("未指定的 header 表示 don't care", func(t *testing.T) {
		relaxed := expect
		relaxed.CacheControl = ""
		if !matchesExpectedState(newRemote(), relaxed) {
			t.Fatal("unspecified header must mean don't care")
		}
	})

	t.Run("sha/size/content-type 始终比对", func(t *testing.T) {
		remote := newRemote()
		remote.Metadata[MetadataSHA256Key] = "different"
		if matchesExpectedState(remote, expect) {
			t.Fatal("sha mismatch must not match")
		}
		remote = newRemote()
		remote.Size++
		if matchesExpectedState(remote, expect) {
			t.Fatal("size mismatch must not match")
		}
		remote = newRemote()
		remote.ContentType = "application/json"
		if matchesExpectedState(remote, expect) {
			t.Fatal("content-type mismatch must not match")
		}
	})

	t.Run("请求的 metadata 缺失即不一致", func(t *testing.T) {
		remote := newRemote()
		delete(remote.Metadata, "width")
		if matchesExpectedState(remote, expect) {
			t.Fatal("missing requested metadata must not match")
		}
	})

	t.Run("远端不存在永不匹配", func(t *testing.T) {
		if matchesExpectedState(nil, expect) {
			t.Fatal("nil remote must never match")
		}
	})

	t.Run("mismatch 描述定位字段", func(t *testing.T) {
		remote := newRemote()
		remote.CacheControl = "max-age=60"
		if detail := expectedStateMismatch(remote, expect); !strings.Contains(detail, "cache-control") {
			t.Fatalf("mismatch detail = %q, want cache-control", detail)
		}
	})
}

// TestSkipMatchingReusesIdenticalObject 完整状态一致时复用远端对象：
// status=reused、skipped=true，仅 1 × HEAD。
func TestSkipMatchingReusesIdenticalObject(t *testing.T) {
	state, client := newUploadVerifyTestServer(t)
	path := writeUploadFixture(t)

	// 预置完全相同状态的对象（同内容 + 同 header/metadata）
	if _, err := Upload(context.Background(), client, path, "hello.txt", &UploadOptions{
		CacheControl: "no-cache",
		Metadata:     map[string]string{"width": "1920"},
	}); err != nil {
		t.Fatalf("seed upload: %v", err)
	}
	state.mu.Lock()
	state.methods = nil
	state.mu.Unlock()

	result, err := Upload(context.Background(), client, path, "hello.txt", &UploadOptions{
		SkipMatching: true,
		CacheControl: "no-cache",
		Metadata:     map[string]string{"width": "1920"},
	})
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if result.Status != StatusReused || !result.Skipped {
		t.Fatalf("status = %q skipped = %v, want reused/true", result.Status, result.Skipped)
	}
	if result.SHA256 != helloSHA256 {
		t.Errorf("sha256 = %q, want %q", result.SHA256, helloSHA256)
	}
	assertMethods(t, state.snapshotMethods(), []string{http.MethodHead})
}

// TestSkipMatchingUploadsOnStateMismatch 任一显式请求字段不一致时
// 执行上传：HEAD → PUT，status=uploaded。
func TestSkipMatchingUploadsOnStateMismatch(t *testing.T) {
	tests := []struct {
		name   string
		tamper func(h http.Header, size *int64)
	}{
		{
			name:   "cache-control 不一致",
			tamper: func(h http.Header, size *int64) { h.Set("Cache-Control", "max-age=60") },
		},
		{
			name:   "用户 metadata 不一致",
			tamper: func(h http.Header, size *int64) { h.Set("x-amz-meta-width", "1280") },
		},
		{
			name:   "内容不一致",
			tamper: func(h http.Header, size *int64) { h.Set("x-amz-meta-itb-sha256", "deadbeef") },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, client := newUploadVerifyTestServer(t)
			state.mu.Lock()
			state.tamper = tt.tamper
			state.mu.Unlock()
			path := writeUploadFixture(t)

			// 预置对象（tamper 使其状态偏离本次上传预期）
			if _, err := Upload(context.Background(), client, path, "hello.txt", &UploadOptions{
				CacheControl: "no-cache",
				Metadata:     map[string]string{"width": "1920"},
			}); err != nil {
				t.Fatalf("seed upload: %v", err)
			}
			state.mu.Lock()
			state.methods = nil
			state.tamper = nil
			state.mu.Unlock()

			result, err := Upload(context.Background(), client, path, "hello.txt", &UploadOptions{
				SkipMatching: true,
				CacheControl: "no-cache",
				Metadata:     map[string]string{"width": "1920"},
			})
			if err != nil {
				t.Fatalf("Upload: %v", err)
			}
			if result.Status != StatusUploaded || result.Skipped {
				t.Fatalf("status = %q skipped = %v, want uploaded/false", result.Status, result.Skipped)
			}
			assertMethods(t, state.snapshotMethods(), []string{http.MethodHead, http.MethodPut})
		})
	}
}

// TestSkipMatchingWithVerifyMissRequestContract skip-matching 未命中 +
// verify：HEAD → PUT → HEAD（最多两个 HEAD）。
func TestSkipMatchingWithVerifyMissRequestContract(t *testing.T) {
	state, client := newUploadVerifyTestServer(t)
	path := writeUploadFixture(t)

	result, err := Upload(context.Background(), client, path, "hello.txt", &UploadOptions{
		SkipMatching: true,
		CacheControl: "no-cache",
		Verify:       true,
	})
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if result.Status != StatusUploaded {
		t.Fatalf("status = %q, want uploaded", result.Status)
	}
	assertMethods(t, state.snapshotMethods(), []string{http.MethodHead, http.MethodPut, http.MethodHead})
}

// TestUploadStatusUploaded 默认上传 status=uploaded。
func TestUploadStatusUploaded(t *testing.T) {
	_, client := newUploadTestServer(t, nil)
	path := writeUploadFixture(t)

	result, err := Upload(context.Background(), client, path, "hello.txt", nil)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if result.Status != StatusUploaded {
		t.Fatalf("status = %q, want uploaded", result.Status)
	}
	if result.SchemaVersion != "itb.s3.upload.v2" {
		t.Fatalf("schema_version = %q, want itb.s3.upload.v2", result.SchemaVersion)
	}
}

// TestUploadSkipStrategyConflict 领域层拒绝多个互斥策略。
func TestUploadSkipStrategyConflict(t *testing.T) {
	_, client := newUploadTestServer(t, nil)
	path := writeUploadFixture(t)

	for _, combo := range []UploadOptions{
		{SkipExisting: true, SkipUnchanged: true},
		{SkipExisting: true, SkipMatching: true},
		{SkipUnchanged: true, SkipMatching: true},
		{SkipExisting: true, SkipUnchanged: true, SkipMatching: true},
	} {
		if _, err := Upload(context.Background(), client, path, "hello.txt", &combo); err != ErrSkipStrategyConflict {
			t.Errorf("opts %+v: err = %v, want ErrSkipStrategyConflict", combo, err)
		}
	}
}
