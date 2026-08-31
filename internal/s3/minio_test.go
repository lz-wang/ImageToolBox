package s3

import (
	"context"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

// 本测试针对真实 MinIO 验证 S3 兼容性（upload/stat/download/
// skip-existing/skip-unchanged/metadata/cache-control/overwrite/
// verify/delete + path-style）。MinIO 不可达时自动跳过：
//
//	ITB_TEST_MINIO_ENDPOINT     默认 http://127.0.0.1:9000
//	ITB_TEST_MINIO_ACCESS_KEY   默认 minioadmin
//	ITB_TEST_MINIO_SECRET_KEY   默认 minioadmin
//
// CI（.github/workflows/build-binaries.yml、release.yml）在 step 内以
// docker run 启动 MinIO（service container 不支持容器命令，无法传入
// server /data），并设置 ITB_REQUIRE_MINIO=1（strict 模式），使该测试
// 在每次 push 时真实执行，且 MinIO 起不来时必须失败而不是悄悄 skip，
// 防止 AWS SDK 升级或 S3 层重构在单测全部通过的情况下悄悄破坏
// MinIO 兼容性。

const minioTestBucket = "itb-test"

func minioTestConfig(t *testing.T) *Config {
	t.Helper()

	getenv := func(key, fallback string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return fallback
	}
	return &Config{
		Endpoint:        getenv("ITB_TEST_MINIO_ENDPOINT", "http://127.0.0.1:9000"),
		AccessKeyID:     getenv("ITB_TEST_MINIO_ACCESS_KEY", "minioadmin"),
		SecretAccessKey: getenv("ITB_TEST_MINIO_SECRET_KEY", "minioadmin"),
		Region:          "us-east-1",
		Bucket:          minioTestBucket,
		// MinIO 必须使用路径样式：整个测试套件都在 path-style 下运行
		ForcePathStyle: true,
	}
}

// skipIfMinIOUnreachable 以 TCP 探测 MinIO；不可达时默认跳过测试，
// 仅当 CI 设置 ITB_REQUIRE_MINIO=1（strict 模式）时改为失败——
// CI 的目标是持续验证 MinIO 兼容性，MinIO 起不来必须红而不是悄悄 skip。
// 只做连通性判断，鉴权问题留给后续真实断言暴露。
func skipIfMinIOUnreachable(t *testing.T, cfg *Config) {
	t.Helper()

	u, err := url.Parse(cfg.Endpoint)
	if err != nil {
		t.Skipf("invalid MinIO endpoint %q: %v", cfg.Endpoint, err)
	}
	host := u.Host
	if u.Port() == "" {
		if u.Scheme == "https" {
			host = net.JoinHostPort(host, "443")
		} else {
			host = net.JoinHostPort(host, "80")
		}
	}
	conn, err := net.DialTimeout("tcp", host, 2*time.Second)
	if err != nil {
		if minioRequired() {
			t.Fatalf("MinIO required (ITB_REQUIRE_MINIO=1) but unreachable at %s: %v", cfg.Endpoint, err)
		}
		t.Skipf("MinIO not reachable at %s (%v); start MinIO or set ITB_TEST_MINIO_ENDPOINT to run this integration test", cfg.Endpoint, err)
	}
	conn.Close()
}

// minioRequired 报告 MinIO 集成测试是否处于 strict 模式：CI workflow
// 设置 ITB_REQUIRE_MINIO=1，MinIO 不可达时测试必须失败；本地开发不设置，
// 不可达时优雅跳过。
func minioRequired() bool {
	return os.Getenv("ITB_REQUIRE_MINIO") == "1"
}

// TestMinIORequiredStrictMode 锁定 strict 模式的决策契约：
// 默认关闭（本地可跳过），ITB_REQUIRE_MINIO=1 时开启（CI 必须真实执行）。
func TestMinIORequiredStrictMode(t *testing.T) {
	t.Setenv("ITB_REQUIRE_MINIO", "")
	if minioRequired() {
		t.Fatal("minioRequired() = true by default, want false (local skip)")
	}

	t.Setenv("ITB_REQUIRE_MINIO", "1")
	if !minioRequired() {
		t.Fatal("minioRequired() = false with ITB_REQUIRE_MINIO=1, want true")
	}
}

// newMinIOTestClient 返回连接真实 MinIO 的客户端，并确保测试桶存在。
func newMinIOTestClient(t *testing.T) (*Client, string) {
	t.Helper()

	cfg := minioTestConfig(t)
	skipIfMinIOUnreachable(t, cfg)
	cfg.Normalize()

	ctx := context.Background()
	client, err := NewClient(ctx, cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	if _, err := client.client.HeadBucket(ctx, &awss3.HeadBucketInput{Bucket: aws.String(client.bucket)}); err != nil {
		if _, cerr := client.client.CreateBucket(ctx, &awss3.CreateBucketInput{Bucket: aws.String(client.bucket)}); cerr != nil {
			t.Fatalf("create bucket %s: %v (HeadBucket: %v)", client.bucket, cerr, err)
		}
	}

	// 每次运行使用唯一 key 前缀，避免并行/重复运行的键冲突
	prefix := "itb-integration/" + strconv.FormatInt(time.Now().UnixNano(), 36) + "/"
	return client, prefix
}

func TestMinIOIntegration(t *testing.T) {
	client, prefix := newMinIOTestClient(t)
	ctx := context.Background()

	uploadFixture := func(name, content string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
		return path
	}

	var (
		basicKey  = prefix + "hello.txt"
		basicPath string
	)

	t.Run("upload with metadata and cache-control", func(t *testing.T) {
		basicPath = uploadFixture("hello.txt", helloContent)
		result, err := Upload(ctx, client, basicPath, basicKey, &UploadOptions{
			CacheControl: "no-cache",
			Metadata:     map[string]string{"source-sha256": helloSHA256, "width": "1920"},
			Verify:       true, // PUT → HEAD：远端属性与本次上传一致
		})
		if err != nil {
			t.Fatalf("Upload: %v", err)
		}
		if result.Skipped || result.SHA256 != helloSHA256 {
			t.Fatalf("result = %+v", result)
		}
	})

	t.Run("stat reflects upload", func(t *testing.T) {
		info, err := Stat(ctx, client, basicKey)
		if err != nil {
			t.Fatalf("Stat: %v", err)
		}
		if info.Size != int64(len(helloContent)) {
			t.Errorf("size = %d, want %d", info.Size, len(helloContent))
		}
		if info.ContentType != "text/plain; charset=utf-8" {
			t.Errorf("content-type = %q, want text/plain（内容检测，MinIO 回读）", info.ContentType)
		}
		if info.CacheControl != "no-cache" {
			t.Errorf("cache-control = %q, want no-cache", info.CacheControl)
		}
		if info.Metadata[MetadataSHA256Key] != helloSHA256 {
			t.Errorf("metadata itb-sha256 = %q, want %q", info.Metadata[MetadataSHA256Key], helloSHA256)
		}
		if info.Metadata["width"] != "1920" {
			t.Errorf("metadata width = %q, want 1920", info.Metadata["width"])
		}
	})

	t.Run("skip-existing skips second upload", func(t *testing.T) {
		result, err := Upload(ctx, client, basicPath, basicKey, &UploadOptions{SkipExisting: true})
		if err != nil {
			t.Fatalf("Upload: %v", err)
		}
		if !result.Skipped {
			t.Fatal("expected skip on existing key")
		}
	})

	t.Run("skip-unchanged skips identical content", func(t *testing.T) {
		result, err := Upload(ctx, client, basicPath, basicKey, &UploadOptions{SkipUnchanged: true})
		if err != nil {
			t.Fatalf("Upload: %v", err)
		}
		if !result.Skipped {
			t.Fatal("expected skip on unchanged content")
		}
	})

	t.Run("download with metadata verify", func(t *testing.T) {
		output := filepath.Join(t.TempDir(), "downloaded.txt")
		result, err := Download(ctx, client, basicKey, output, &DownloadOptions{Verify: true})
		if err != nil {
			t.Fatalf("Download --verify: %v", err)
		}
		if result.SHA256 != helloSHA256 {
			t.Errorf("downloaded sha256 = %q, want %q", result.SHA256, helloSHA256)
		}
		got, err := os.ReadFile(output)
		if err != nil {
			t.Fatalf("read downloaded: %v", err)
		}
		if string(got) != helloContent {
			t.Errorf("downloaded content = %q, want %q", got, helloContent)
		}
	})

	t.Run("overwrite replaces content and verify-sha256 validates", func(t *testing.T) {
		replacement := "replaced\n"
		path := uploadFixture("replaced.txt", replacement)
		result, err := Upload(ctx, client, path, basicKey, nil)
		if err != nil {
			t.Fatalf("Upload (overwrite): %v", err)
		}
		if result.Skipped {
			t.Fatal("default upload must overwrite, not skip")
		}

		output := filepath.Join(t.TempDir(), "overwritten.txt")
		if _, err := Download(ctx, client, basicKey, output, &DownloadOptions{VerifySHA256: result.SHA256}); err != nil {
			t.Fatalf("Download --verify-sha256: %v", err)
		}
		got, err := os.ReadFile(output)
		if err != nil {
			t.Fatalf("read downloaded: %v", err)
		}
		if string(got) != replacement {
			t.Errorf("content after overwrite = %q, want %q", got, replacement)
		}

		// 旧内容哈希必须失配（provider-neutral 校验真实生效）
		stale := filepath.Join(t.TempDir(), "stale.txt")
		if _, err := Download(ctx, client, basicKey, stale, &DownloadOptions{VerifySHA256: helloSHA256}); err == nil || !strings.Contains(err.Error(), ErrChecksumMismatch.Error()) {
			t.Fatalf("expected checksum mismatch for stale hash, got %v", err)
		}
		if _, err := os.Stat(stale); !os.IsNotExist(err) {
			t.Errorf("stale download must not leave a partial file, stat err = %v", err)
		}
	})

	t.Run("delete removes object", func(t *testing.T) {
		if err := Delete(ctx, client, basicKey, nil); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if _, err := Stat(ctx, client, basicKey); err == nil || !strings.Contains(err.Error(), ErrObjectNotFound.Error()) {
			t.Fatalf("expected not found after delete, got %v", err)
		}
	})
}
