package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/urfave/cli/v3"

	"imagetoolbox/internal/s3"
)

func testApp() *cli.Command {
	return New("1.2.3", fstest.MapFS{})
}

// runContract 执行 CLI 并返回错误，用于断言参数契约。
func runContract(args ...string) error {
	return testApp().Run(context.Background(), append([]string{"itb"}, args...))
}

// TestRootCommands 检查根命令注册的子命令清单。
func TestRootCommands(t *testing.T) {
	app := testApp()

	want := []string{"compress", "resize", "crop", "convert", "watermark", "inspect", "s3", "serve", "version"}
	for _, name := range want {
		found := false
		for _, sub := range app.Commands {
			if sub.Name == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("root command missing subcommand %q", name)
		}
	}
}

// TestRootHelpOutput 真正执行 `itb --help`，验证命令清单出现在
// 帮助输出中、已删除的命令不再被宣传。
func TestRootHelpOutput(t *testing.T) {
	app := testApp()
	var buf bytes.Buffer
	app.Writer = &buf

	if err := app.Run(context.Background(), []string{"itb", "--help"}); err != nil {
		t.Fatalf("help failed: %v", err)
	}

	out := buf.String()
	for _, name := range []string{"compress", "resize", "crop", "convert", "watermark", "inspect", "s3", "serve", "version"} {
		if !strings.Contains(out, name) {
			t.Errorf("help output missing command %q", name)
		}
	}
	for _, name := range []string{"batch", "lsky"} {
		if strings.Contains(out, name) {
			t.Errorf("help output should not mention removed command %q", name)
		}
	}
}

func TestRemovedCommandsAreGone(t *testing.T) {
	app := testApp()

	for _, name := range []string{"batch", "lsky"} {
		for _, sub := range app.Commands {
			if sub.Name == name {
				t.Errorf("removed command %q still registered", name)
			}
		}
	}
}

// urfave/cli 对未知命令会 os.Exit(3)，无法在测试进程内直接断言，
// 通过子进程验证退出码非 0。
func TestRemovedCommandsExitNonZero(t *testing.T) {
	if os.Getenv("ITB_TEST_UNKNOWN_CMD") != "" {
		_ = testApp().Run(context.Background(), []string{"itb", os.Getenv("ITB_TEST_UNKNOWN_CMD")})
		return
	}

	for _, name := range []string{"batch", "lsky"} {
		t.Run(name, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=TestRemovedCommandsExitNonZero", "-test.v=false")
			cmd.Env = append(os.Environ(), "ITB_TEST_UNKNOWN_CMD="+name)
			err := cmd.Run()
			if _, ok := errors.AsType[*exec.ExitError](err); !ok {
				t.Fatalf("expected non-zero exit for unknown command %q, got %v", name, err)
			}
		})
	}
}

func TestRequiredFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"resize 缺 --input", []string{"resize"}},
		{"convert 缺 --to", []string{"convert", "-i", "a.png"}},
		{"crop 缺 --anchor", []string{"crop", "-i", "a.jpg"}},
		{"s3 download 缺 --key", []string{"s3", "-b", "x", "download"}},
		{"s3 stat 缺 --key", []string{"s3", "-b", "x", "stat"}},
		{"s3 upload 缺 --input", []string{"s3", "-b", "x", "upload"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runContract(tt.args...)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(err.Error(), "Required flag") {
				t.Fatalf("expected required flag error, got: %v", err)
			}
		})
	}
}

func TestS3ParentFlagParsing(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		// bucket 为 s3 父命令 required flag，upload 子命令须能继承
		{"父级 flag 前置", []string{"s3", "-b", "test", "upload", "-i", "nonexistent.jpg"}},
		{"父级 flag 后置", []string{"s3", "upload", "-i", "nonexistent.jpg", "-b", "test"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runContract(tt.args...)
			if err == nil {
				t.Fatal("expected error from missing endpoint, got nil")
			}
			// flag 解析失败会报 "Required flag ... not set"，
			// 而 endpoint is required 说明 bucket 已正确继承并进入 Config 校验。
			if strings.Contains(err.Error(), "Required flag") {
				t.Fatalf("parent flag not inherited: %v", err)
			}
		})
	}
}

// setS3Env 显式设置测试所需的 ITB_S3_* 环境变量：map 中存在的键被
// 设置，其余键一律取消设置（含恢复），避免宿主环境泄漏进测试。
func setS3Env(t *testing.T, env map[string]string) {
	t.Helper()

	for _, k := range []string{"ITB_S3_ENDPOINT", "ITB_S3_ACCESS_KEY_ID", "ITB_S3_SECRET_ACCESS_KEY", "ITB_S3_REGION", "ITB_S3_BUCKET"} {
		if v, ok := env[k]; ok {
			t.Setenv(k, v)
			continue
		}
		if orig, had := os.LookupEnv(k); had {
			os.Unsetenv(k)
			t.Cleanup(func() { os.Setenv(k, orig) })
		}
	}
}

// runS3ConfigCapture 把 `s3 list` 的 Action 替换为配置捕获，返回 Action
// 内解析到的父级配置，用于断言 flag 与 ITB_S3_* 环境变量的解析结果。
func runS3ConfigCapture(t *testing.T, env map[string]string, args ...string) (s3.Config, error) {
	t.Helper()

	setS3Env(t, env)

	var got s3.Config
	app := testApp()
	for _, sub := range app.Commands {
		if sub.Name != "s3" {
			continue
		}
		for _, list := range sub.Commands {
			if list.Name != "list" {
				continue
			}
			list.Action = func(ctx context.Context, cmd *cli.Command) error {
				got = s3.Config{
					Endpoint:        cmd.String("endpoint"),
					AccessKeyID:     cmd.String("access-key"),
					SecretAccessKey: cmd.String("secret-key"),
					Region:          cmd.String("region"),
					Bucket:          cmd.String("bucket"),
					ForcePathStyle:  cmd.Bool("force-path-style"),
				}
				return nil
			}
		}
	}

	err := app.Run(context.Background(), append([]string{"itb"}, args...))
	return got, err
}

// ITB_S3_* 由 CLI 层（urfave/cli Sources）解析：环境变量可满足
// required flag（bucket），优先级为 CLI flag > 环境变量 > 默认值。
func TestS3EnvSources(t *testing.T) {
	envAll := map[string]string{
		"ITB_S3_ENDPOINT":          "http://env-endpoint:9000",
		"ITB_S3_ACCESS_KEY_ID":     "env-ak",
		"ITB_S3_SECRET_ACCESS_KEY": "env-sk",
		"ITB_S3_REGION":            "env-region",
		"ITB_S3_BUCKET":            "env-bucket",
	}

	t.Run("环境变量注入全部父级配置并满足 required bucket", func(t *testing.T) {
		got, err := runS3ConfigCapture(t, envAll, "s3", "list")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := s3.Config{
			Endpoint:        "http://env-endpoint:9000",
			AccessKeyID:     "env-ak",
			SecretAccessKey: "env-sk",
			Region:          "env-region",
			Bucket:          "env-bucket",
		}
		if got != want {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	})

	t.Run("CLI flag 优先于环境变量", func(t *testing.T) {
		got, err := runS3ConfigCapture(t, envAll,
			"s3", "list",
			"-e", "http://flag-endpoint:9000",
			"-a", "flag-ak",
			"-s", "flag-sk",
			"-r", "flag-region",
			"-b", "flag-bucket")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := s3.Config{
			Endpoint:        "http://flag-endpoint:9000",
			AccessKeyID:     "flag-ak",
			SecretAccessKey: "flag-sk",
			Region:          "flag-region",
			Bucket:          "flag-bucket",
		}
		if got != want {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	})

	t.Run("无环境变量且无 flag 时 bucket required 报错", func(t *testing.T) {
		_, err := runS3ConfigCapture(t, nil, "s3", "list")
		if err == nil {
			t.Fatal("expected required flag error, got nil")
		}
		if !strings.Contains(err.Error(), "Required flag") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestS3SkipFlagsMutuallyExclusive(t *testing.T) {
	err := runContract("s3", "upload", "-i", "a.jpg", "-b", "test", "--skip-existing", "--skip-unchanged")
	if err == nil {
		t.Fatal("expected mutually exclusive error, got nil")
	}
	if !strings.Contains(err.Error(), "cannot be set along with") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWatermarkTextImageMutuallyExclusive(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"text 与 image 互斥", []string{"watermark", "-i", "a.jpg", "-t", "x", "--image", "y.png"}, "cannot be set along with"},
		{"text 与 image 必须提供其一", []string{"watermark", "-i", "a.jpg"}, "needs to be provided"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runContract(tt.args...)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected %q in error, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestVersionCommand(t *testing.T) {
	if err := runContract("version"); err != nil {
		t.Fatalf("version command failed: %v", err)
	}
}

func TestInspectAlias(t *testing.T) {
	app := testApp()
	found := false
	for _, sub := range app.Commands {
		if sub.Name == "inspect" {
			for _, alias := range sub.Aliases {
				if alias == "metadata" {
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatal("inspect command missing alias metadata")
	}
}

func TestServeFlagParsing(t *testing.T) {
	cmd := New("1.2.3", fstest.MapFS{})
	for _, sub := range cmd.Commands {
		if sub.Name != "serve" {
			continue
		}
		var addrFlag bool
		for _, f := range sub.Flags {
			for _, name := range f.Names() {
				if name == "addr" {
					addrFlag = true
				}
			}
		}
		if !addrFlag {
			t.Error("serve command missing --addr flag")
		}
		return
	}
	t.Fatal("serve command not found")
}
