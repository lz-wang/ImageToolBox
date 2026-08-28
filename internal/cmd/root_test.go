package cmd

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/urfave/cli/v3"
)

func testApp() *cli.Command {
	return New("1.2.3", fstest.MapFS{})
}

// runContract 执行 CLI 并返回错误，用于断言参数契约。
func runContract(args ...string) error {
	return testApp().Run(context.Background(), append([]string{"itb"}, args...))
}

func TestRootHelpListsCommands(t *testing.T) {
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
