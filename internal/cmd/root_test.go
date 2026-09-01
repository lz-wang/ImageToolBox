package cmd

import (
	"bytes"
	"context"
	"errors"
	"maps"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/urfave/cli/v3"

	"imagetoolbox/internal/inspect"
	"imagetoolbox/internal/s3"
)

func testApp() *cli.Command {
	return New("1.2.3")
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
	// S3 operand 校验应先于 Action 内的新客户端创建；为避免 urfave/cli 的
	// 父级必填 flag 解析抢先失败，这里通过环境变量满足连接配置。
	setS3Env(t, map[string]string{
		"ITB_S3_ENDPOINT":          "http://localhost:9000",
		"ITB_S3_ACCESS_KEY_ID":     "ak",
		"ITB_S3_SECRET_ACCESS_KEY": "sk",
		"ITB_S3_BUCKET":            "test",
	})

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"resize 缺 <src>", []string{"resize"}, "需要提供 <src> [dst]"},
		{"convert 缺 <dst>", []string{"convert", "a.png"}, "需要提供 <src> <dst>"},
		{"crop 缺 --anchor", []string{"crop", "a.jpg"}, "Required flag"},
		{"s3 download 缺 <key>", []string{"s3", "-b", "x", "download"}, "需要提供 <key> [dst]"},
		{"s3 stat 缺 <key>", []string{"s3", "-b", "x", "stat"}, "需要提供 <key>"},
		{"s3 upload 缺 <src>", []string{"s3", "-b", "x", "upload"}, "需要提供 <src> [key]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runContract(tt.args...)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected %q in error, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestS3ParentFlagParsing(t *testing.T) {
	// endpoint / access-key / secret-key 与 bucket 一样都是 s3 父命令的
	// required flag；这里通过环境变量满足前三者，验证 bucket 无论前置
	// 还是后置于子命令都能被解析继承。
	env := map[string]string{
		"ITB_S3_ENDPOINT":          "http://localhost:9000",
		"ITB_S3_ACCESS_KEY_ID":     "ak",
		"ITB_S3_SECRET_ACCESS_KEY": "sk",
	}

	tests := []struct {
		name string
		args []string
	}{
		{"父级 flag 前置", []string{"s3", "-b", "test", "list"}},
		{"父级 flag 后置", []string{"s3", "list", "-b", "test"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := runS3ConfigCapture(t, env, tt.args...)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Bucket != "test" {
				t.Fatalf("parent flag -b not inherited, got bucket %q", got.Bucket)
			}
		})
	}
}

// setS3Env 显式设置测试所需的 ITB_S3_* 环境变量：map 中存在的键被
// 设置，其余键一律取消设置（含恢复），避免宿主环境泄漏进测试。
func setS3Env(t *testing.T, env map[string]string) {
	t.Helper()

	for _, k := range []string{
		"ITB_S3_ENDPOINT", "ITB_S3_ACCESS_KEY_ID", "ITB_S3_SECRET_ACCESS_KEY",
		"ITB_S3_SESSION_TOKEN", "ITB_S3_REGION", "ITB_S3_BUCKET",
		"ITB_S3_FORCE_PATH_STYLE",
	} {
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
					SessionToken:    cmd.String("session-token"),
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

// withBaseEnv 在 base 配置上叠加额外环境变量，构造测试用 env。
func withBaseEnv(base, extra map[string]string) map[string]string {
	env := maps.Clone(base)
	maps.Copy(env, extra)
	return env
}

// ITB_S3_SESSION_TOKEN 注入临时凭证 Session Token，CLI flag 优先。
func TestS3SessionTokenEnv(t *testing.T) {
	base := map[string]string{
		"ITB_S3_ENDPOINT":          "https://s3.example.com",
		"ITB_S3_ACCESS_KEY_ID":     "ak",
		"ITB_S3_SECRET_ACCESS_KEY": "sk",
		"ITB_S3_BUCKET":            "test",
	}

	t.Run("环境变量注入 session token", func(t *testing.T) {
		env := withBaseEnv(base, map[string]string{"ITB_S3_SESSION_TOKEN": "env-token"})
		got, err := runS3ConfigCapture(t, env, "s3", "list")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.SessionToken != "env-token" {
			t.Fatalf("SessionToken = %q, want env-token", got.SessionToken)
		}
	})

	t.Run("CLI flag 优先于环境变量", func(t *testing.T) {
		env := withBaseEnv(base, map[string]string{"ITB_S3_SESSION_TOKEN": "env-token"})
		got, err := runS3ConfigCapture(t, env, "s3", "list", "--session-token", "flag-token")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.SessionToken != "flag-token" {
			t.Fatalf("SessionToken = %q, want flag-token", got.SessionToken)
		}
	})
}

// ITB_S3_FORCE_PATH_STYLE 显式控制路径样式，CLI flag 优先。
func TestForcePathStyleEnv(t *testing.T) {
	base := map[string]string{
		"ITB_S3_ENDPOINT":          "https://s3.example.com",
		"ITB_S3_ACCESS_KEY_ID":     "ak",
		"ITB_S3_SECRET_ACCESS_KEY": "sk",
		"ITB_S3_BUCKET":            "test",
	}

	t.Run("环境变量启用 force-path-style", func(t *testing.T) {
		env := withBaseEnv(base, map[string]string{"ITB_S3_FORCE_PATH_STYLE": "true"})
		got, err := runS3ConfigCapture(t, env, "s3", "list")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !got.ForcePathStyle {
			t.Fatal("ForcePathStyle should be enabled via env")
		}
	})

	t.Run("CLI 显式关闭优先于环境变量启用", func(t *testing.T) {
		env := withBaseEnv(base, map[string]string{"ITB_S3_FORCE_PATH_STYLE": "true"})
		got, err := runS3ConfigCapture(t, env, "s3", "list", "--force-path-style=false")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ForcePathStyle {
			t.Fatal("ForcePathStyle should stay off when flag is explicitly false")
		}
	})
}

func TestS3SkipFlagsMutuallyExclusive(t *testing.T) {
	err := runContract("s3", "upload", "a.jpg", "-b", "test", "--skip-existing", "--skip-unchanged")
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
		{"text 与 image 互斥", []string{"watermark", "a.jpg", "-t", "x", "--image", "y.png"}, "cannot be set along with"},
		{"text 与 image 必须提供其一", []string{"watermark", "a.jpg"}, "needs to be provided"},
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

func TestPositionalPathContracts(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"compress missing source", []string{"compress"}, "需要提供 <src> [dst]"},
		{"compress too many paths", []string{"compress", "a", "b", "c"}, "需要提供 <src> [dst]"},
		{"compress in-place with destination", []string{"compress", "--in-place", "a", "b"}, "--in-place 不能与 <dst> 同时使用"},
		{"resize too many paths", []string{"resize", "--width", "1", "a", "b", "c"}, "需要提供 <src> [dst]"},
		{"crop missing source", []string{"crop", "--anchor", "left", "--width", "40%"}, "需要提供 <src> [dst]"},
		{"watermark too many paths", []string{"watermark", "-t", "x", "a", "b", "c"}, "需要提供 <src> [dst]"},
		{"inspect missing source", []string{"inspect"}, "需要提供 <src>"},
		{"inspect extra source", []string{"inspect", "a", "b"}, "需要提供 <src>"},
		{"convert missing destination", []string{"convert", "a"}, "需要提供 <src> <dst>"},
		{"convert extra path", []string{"convert", "a", "b", "c"}, "需要提供 <src> <dst>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runContract(tt.args...)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestPositionalPathsAllowFlagsAfterOperands(t *testing.T) {
	err := runContract("resize", "missing.jpg", "out.jpg", "--width", "100")
	if err == nil {
		t.Fatal("expected missing input error, got nil")
	}
	if strings.Contains(err.Error(), "flag provided but not defined") {
		t.Fatalf("flags after operands were parsed as unknown: %v", err)
	}
}

func TestRemovedImageInputOutputFlagsAreRejected(t *testing.T) {
	tests := [][]string{
		{"resize", "--input", "a.jpg", "--width", "100"},
		{"convert", "--to", "webp", "a.png", "b.webp"},
		{"crop", "-i", "a.jpg", "--anchor", "left", "--width", "40%"},
	}
	for _, args := range tests {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			err := runContract(args...)
			if err == nil || !strings.Contains(err.Error(), "flag provided but not defined") {
				t.Fatalf("error = %v, want undefined flag error", err)
			}
		})
	}

	err := runContract("s3", "upload", "-i", "a.jpg", "-b", "test")
	if err == nil || !strings.Contains(err.Error(), "flag provided but not defined") {
		t.Fatalf("s3 upload -i should be rejected, error = %v", err)
	}
}

func TestRemovedS3LocatorFlagsAreRejected(t *testing.T) {
	tests := [][]string{
		{"s3", "upload", "-i", "a.jpg"},
		{"s3", "upload", "a.jpg", "-k", "x.jpg"},
		{"s3", "download", "-k", "x.jpg"},
		{"s3", "download", "x.jpg", "-o", "y.jpg"},
		{"s3", "stat", "-k", "x.jpg"},
		{"s3", "delete", "-k", "x.jpg"},
		{"s3", "list", "-p", "images/"},
	}
	for _, args := range tests {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			err := runContract(args...)
			if err == nil || !strings.Contains(err.Error(), "flag provided but not defined") {
				t.Fatalf("error = %v, want undefined flag error", err)
			}
		})
	}
}

func TestS3StatAcceptsDashPrefixedKeyAfterSeparator(t *testing.T) {
	setS3Env(t, map[string]string{
		"ITB_S3_ENDPOINT":          "http://localhost:9000",
		"ITB_S3_ACCESS_KEY_ID":     "ak",
		"ITB_S3_SECRET_ACCESS_KEY": "sk",
		"ITB_S3_BUCKET":            "test",
	})

	app := testApp()
	for _, sub := range app.Commands {
		if sub.Name != "s3" {
			continue
		}
		for _, command := range sub.Commands {
			if command.Name == "stat" {
				command.Action = func(_ context.Context, cmd *cli.Command) error {
					key, err := s3KeyArg(cmd)
					if err != nil {
						return err
					}
					if key != "-object-name" {
						t.Fatalf("key = %q, want %q", key, "-object-name")
					}
					return nil
				}
			}
		}
	}
	if err := app.Run(context.Background(), []string{"itb", "s3", "stat", "--", "-object-name"}); err != nil {
		t.Fatalf("stat with -- separator: %v", err)
	}
}

func TestVersionCommand(t *testing.T) {
	if err := runContract("version"); err != nil {
		t.Fatalf("version command failed: %v", err)
	}
}

// 根命令设置 Version 后，--version / -v 与 version 子命令等价。
func TestVersionFlag(t *testing.T) {
	for _, arg := range []string{"--version", "-v"} {
		t.Run(arg, func(t *testing.T) {
			app := testApp()
			var buf bytes.Buffer
			app.Writer = &buf
			if err := app.Run(context.Background(), []string{"itb", arg}); err != nil {
				t.Fatalf("%s failed: %v", arg, err)
			}
			if out := buf.String(); !strings.Contains(out, "1.2.3") {
				t.Fatalf("expected version in output, got %q", out)
			}
		})
	}
}

// TestFlagValidators 验证枚举与范围校验在 CLI 层生效，
// 且报错先于文件 IO（输入文件不存在也不影响参数错误优先暴露）。
func TestFlagValidators(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"resize 非法 mode", []string{"resize", "nope.jpg", "--mode", "abc"}, "--mode 仅支持"},
		{"resize 非法 filter", []string{"resize", "nope.jpg", "--filter", "bicubic"}, "--filter 仅支持"},
		{"resize 非法 anchor", []string{"resize", "nope.jpg", "--anchor", "middle"}, "--anchor 仅支持"},
		{"resize width 非正数", []string{"resize", "nope.jpg", "--width", "0"}, "--width 必须大于 0"},
		{"resize height 非正数", []string{"resize", "nope.jpg", "--height", "-3"}, "--height 必须大于 0"},
		{"resize percent 缺百分号", []string{"resize", "nope.jpg", "--percent", "50"}, "百分比格式"},
		{"resize percent NaN", []string{"resize", "nope.jpg", "--percent", "NaN%"}, "有限数值"},
		{"crop 非法 anchor", []string{"crop", "nope.jpg", "--anchor", "middle", "--width", "40%"}, "--anchor 仅支持"},
		{"crop width 超上限", []string{"crop", "nope.jpg", "--anchor", "left", "--width", "140%"}, "(0,100]"},
		{"crop height 缺百分号", []string{"crop", "nope.jpg", "--anchor", "top", "--height", "40"}, "百分比格式"},
		{"convert 非法格式", []string{"convert", "nope.png", "output.gif"}, "unsupported image format"},
		{"convert quality 超范围", []string{"convert", "nope.png", "output.png", "-q", "0"}, "--quality 必须在"},
		{"compress quality 超范围", []string{"compress", "nope.png", "-q", "101"}, "--quality 必须在"},
		{"watermark 非法 mode", []string{"watermark", "nope.jpg", "-t", "x", "--mode", "tile"}, "--mode 仅支持"},
		{"watermark opacity 超范围", []string{"watermark", "nope.jpg", "-t", "x", "--opacity", "1.5"}, "--opacity 必须在"},
		{"watermark opacity NaN", []string{"watermark", "nope.jpg", "-t", "x", "--opacity", "NaN"}, "有限数值"},
		{"watermark scale Inf", []string{"watermark", "nope.jpg", "--image", "logo.png", "--scale", "Inf"}, "有限数值"},
		{"watermark 非法 position", []string{"watermark", "nope.jpg", "-t", "x", "--position", "middle"}, "--position 仅支持"},
		{"watermark scale 非正数", []string{"watermark", "nope.jpg", "--image", "logo.png", "--scale", "0"}, "--scale 必须大于 0"},
		{"watermark font-size 负数", []string{"watermark", "nope.jpg", "-t", "x", "--font-size", "-1"}, "必须在 0-4096"},
		{"watermark font-size 超上限", []string{"watermark", "nope.jpg", "-t", "x", "--font-size", "5000"}, "必须在 0-4096"},
		{"watermark angle 超范围", []string{"watermark", "nope.jpg", "-t", "x", "--mode", "repeat", "--angle", "720"}, "必须在 -360-360"},
		{"watermark margin 负数", []string{"watermark", "nope.jpg", "-t", "x", "--margin", "-0.1"}, "不能为负数"},
		{"watermark 非法 color", []string{"watermark", "nope.jpg", "-t", "x", "--color", "red"}, "十六进制颜色"},
		{"watermark space 负数", []string{"watermark", "nope.jpg", "-t", "x", "--mode", "repeat", "--space", "-5"}, "不能为负数"},
		{"serve 非法 timeout", []string{"serve", "--timeout", "-1s"}, "--timeout 必须大于 0"},
		{"serve 非法 max-concurrent", []string{"serve", "--max-concurrent", "0"}, "--max-concurrent 必须大于 0"},
		{"inspect 非法 format", []string{"inspect", "nope.jpg", "--format", "xml"}, "--format 仅支持"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runContract(tt.args...)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected %q in error, got: %v", tt.wantErr, err)
			}
			// 参数错误必须先于文件 IO 报出
			if strings.Contains(err.Error(), "nope") || strings.Contains(err.Error(), "打开输入图片") {
				t.Fatalf("validator should fire before file IO, got: %v", err)
			}
		})
	}
}

// TestS3Validators 验证 S3 子命令的 format / max-keys 校验。
func TestS3Validators(t *testing.T) {
	setS3Env(t, map[string]string{
		"ITB_S3_ENDPOINT":          "http://localhost:9000",
		"ITB_S3_ACCESS_KEY_ID":     "ak",
		"ITB_S3_SECRET_ACCESS_KEY": "sk",
		"ITB_S3_BUCKET":            "test",
	})

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"list 非法 format", []string{"s3", "list", "--format", "xml"}, "--format 仅支持"},
		{"list max-keys 非正数", []string{"s3", "list", "--max-keys", "0"}, "--max-keys 必须大于 0"},
		{"stat 非法 format", []string{"s3", "stat", "a.jpg", "--format", "plain"}, "--format 仅支持"},
		{"upload 非法 format", []string{"s3", "upload", "a.jpg", "--format", "plain"}, "--format 仅支持"},
		{"download 非法 format", []string{"s3", "download", "a.jpg", "--format", "plain"}, "--format 仅支持"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runContract(tt.args...)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected %q in error, got: %v", tt.wantErr, err)
			}
		})
	}
}

// metadata alias 已移除：inspect 的能力远大于"元数据"，
// 无意义的历史 alias 不再进入 public command contract。
func TestInspectAliasRemoved(t *testing.T) {
	app := testApp()
	for _, sub := range app.Commands {
		if sub.Name != "inspect" {
			continue
		}
		for _, alias := range sub.Aliases {
			if alias == "metadata" {
				t.Fatal("inspect command should no longer have alias metadata")
			}
		}
		return
	}
	t.Fatal("inspect command not found")
}

// --no-detail 优先于 --detail，--detail=false 兼容保留。
func TestInspectNoDetailFlag(t *testing.T) {
	run := func(args ...string) inspect.Options {
		t.Helper()
		var got inspect.Options
		app := testApp()
		for _, sub := range app.Commands {
			if sub.Name == "inspect" {
				sub.Action = func(ctx context.Context, cmd *cli.Command) error {
					got = inspect.Options{
						Detail: cmd.Bool("detail") && !cmd.Bool("no-detail"),
						NoHash: cmd.Bool("no-hash"),
					}
					return nil
				}
			}
		}
		if err := app.Run(context.Background(), append([]string{"itb"}, args...)); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return got
	}

	if got := run("inspect", "nope.jpg"); !got.Detail {
		t.Fatal("detail should default to true")
	}
	if got := run("inspect", "nope.jpg", "--no-detail"); got.Detail {
		t.Fatal("--no-detail should disable detail")
	}
	if got := run("inspect", "nope.jpg", "--detail", "--no-detail"); got.Detail {
		t.Fatal("--no-detail should take precedence over --detail")
	}
	if got := run("inspect", "nope.jpg", "--detail=false"); got.Detail {
		t.Fatal("--detail=false should disable detail")
	}
}

func TestServeFlagParsing(t *testing.T) {
	cmd := New("1.2.3")
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
