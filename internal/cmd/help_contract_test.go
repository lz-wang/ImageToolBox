package cmd

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"testing"
	"unicode"

	"github.com/urfave/cli/v3"
)

// Help Contract 测试：不做整页 snapshot（库升级噪声大），
// 而是锁定 help 必须传达的语义——必现的关键信息与禁止出现的漂移文案。

// helpOutput 执行 `itb <args...> --help` 并返回帮助输出。
func helpOutput(t *testing.T, args ...string) string {
	t.Helper()

	app := testApp()
	var buf bytes.Buffer
	app.Writer = &buf

	argv := append([]string{"itb"}, args...)
	argv = append(argv, "--help")
	if err := app.Run(context.Background(), argv); err != nil {
		t.Fatalf("help failed: %v", err)
	}
	return buf.String()
}

// assertContains / assertNotContains 输出失败上下文，便于定位漂移。
func assertContains(t *testing.T, out, want string) {
	t.Helper()
	if !strings.Contains(out, want) {
		t.Errorf("help output missing %q\n--- got ---\n%s", want, out)
	}
}

func assertNotContains(t *testing.T, out, want string) {
	t.Helper()
	if strings.Contains(out, want) {
		t.Errorf("help output should not contain %q\n--- got ---\n%s", want, out)
	}
}

// assertNoHan 断言输出不含 Han 字符：help 是人类用户与 LLM agent 共同
// 依赖的英文契约面，任何中文回归都在此处失败。
func assertNoHan(t *testing.T, out string) {
	t.Helper()
	for _, r := range out {
		if unicode.Is(unicode.Han, r) {
			t.Errorf("help output contains Han character %q; help must be English-only\n--- got ---\n%s", r, out)
			return
		}
	}
}

// 根 help：命令目录只能来自 urfave 生成的 COMMANDS，
// 禁止手写命令清单回归（"功能:" 是旧手写清单的标志）。
// Help 契约面必须纯英文，并按 Category 分组展示。
func TestRootHelpContract(t *testing.T) {
	out := helpOutput(t)

	for _, name := range []string{"compress", "resize", "crop", "rotate", "convert", "watermark", "compare", "inspect", "s3", "serve", "version"} {
		assertContains(t, out, name)
	}
	assertNotContains(t, out, "功能:")
	assertContains(t, out, "--version")
	for _, category := range []string{"Image transforms", "Analysis", "Storage", "Service", "Utility"} {
		assertContains(t, out, category)
	}
	assertNoHan(t, out)
}

// compress：默认非破坏式输出与 --in-place 契约。
func TestCompressHelpContract(t *testing.T) {
	out := helpOutput(t, "compress")

	for _, want := range []string{
		"<src>", "[dst]",
		"--in-place",
		"writes <name>_compressed.<ext>",
		"1-100",
		"PNG",
		"JPEG",
		"--in-place cannot be combined with a [dst] operand",
	} {
		assertContains(t, out, want)
	}
	assertNotContains(t, out, "--input")
	assertNotContains(t, out, "--output")
}

// resize：三种模式与参数组合规则必须在 help 中可读。
func TestResizeHelpContract(t *testing.T) {
	out := helpOutput(t, "resize")

	for _, want := range []string{
		"<src>", "[dst]",
		"--percent", "--width", "--height",
		"fit", "fill", "stretch",
		"writes <name>_resized.<ext>",
		"--percent cannot be combined with --width or --height",
		"fill requires both --width and --height",
		"--width PIXELS",
	} {
		assertContains(t, out, want)
	}
	assertNotContains(t, out, "--input")
	assertNotContains(t, out, "--output")
}

// convert：格式特定参数语义（quality/lossless/background）必须在 help
// 中可读，且不得回退到旧的模糊文案。
func TestConvertHelpContract(t *testing.T) {
	out := helpOutput(t, "convert")

	for _, want := range []string{
		"<src>",
		"<dst>",
		"determined only by the <dst>",
		".jpg / .jpeg / .png / .webp",
		"JPEG/WebP output quality",
		"ignored for PNG",
		"lossless WebP encoding",
		"PNG is always lossless",
		"flattened onto --background",
		"must be opaque",
	} {
		assertContains(t, out, want)
	}
	assertNotContains(t, out, "透明图转不透明格式时的背景色")
	assertNotContains(t, out, "无损编码（webp/png）")
	assertNotContains(t, out, "--input")
	assertNotContains(t, out, "--output")
	assertNotContains(t, out, "--to")
}

// crop：锚点参数组合规则与百分比范围。
func TestCropHelpContract(t *testing.T) {
	out := helpOutput(t, "crop")

	for _, want := range []string{
		"<src>", "[dst]",
		"(0,100]",
		"left / right",
		"top / bottom",
		"require both --width and --height",
		"writes <name>_cropped.<ext>",
	} {
		assertContains(t, out, want)
	}
	assertNotContains(t, out, "--input")
	assertNotContains(t, out, "--output")
}

// watermark：文字/图片两种来源、两种模式；
// 未实现的 capability（--tile）与错误的字体要求不得再出现。
func TestWatermarkHelpContract(t *testing.T) {
	out := helpOutput(t, "watermark")

	for _, want := range []string{
		"text", "image", "position", "repeat",
		"<src>", "[dst]", "--image",
		"writes <name>_watermarked.<ext>",
		"Exactly one of --text or --image is required",
		"position mode only",
	} {
		assertContains(t, out, want)
	}
	assertNotContains(t, out, "--input")
	assertNotContains(t, out, "--output")
	for _, banned := range []string{"当前版本暂不支持", "需要指定字体", "--tile"} {
		assertNotContains(t, out, banned)
	}
}

// rotate：角度方向语义、画布调整与透明背景契约必须在 help 中可读。
func TestRotateHelpContract(t *testing.T) {
	out := helpOutput(t, "rotate")

	for _, want := range []string{
		"<src>", "[dst]",
		"--angle",
		"counter-clockwise",
		"clockwise",
		"(-360, 360)",
		"cannot be 0",
		"bounding box",
		"writes <name>_rotated.<ext>",
		"PNG",
		"WebP",
		"JPEG",
		"EXIF Orientation",
	} {
		assertContains(t, out, want)
	}
	assertNotContains(t, out, "--input")
	assertNotContains(t, out, "--output")
}

// compare：只读双输入语义、默认指标组合与最小尺寸约束必须在 help 中可读。
func TestCompareHelpContract(t *testing.T) {
	out := helpOutput(t, "compare")

	for _, want := range []string{
		"<src>",
		"<dst>",
		"--psnr",
		"--ssim",
		"--ms-ssim",
		"<dst> is the comparison target, not an output path",
		"This command is read-only",
		"If no metric flag is specified, PSNR and MS-SSIM are",
		"only explicitly enabled",
		"161",
		"11x11",
	} {
		assertContains(t, out, want)
	}
	assertNotContains(t, out, "--input")
	assertNotContains(t, out, "--output")
}

// inspect：--no-detail、plain 格式语义与 full-decode 能力；
// compatibility-only 的 --detail 已从 help 隐藏（行为保留）。
func TestInspectHelpContract(t *testing.T) {
	out := helpOutput(t, "inspect")

	for _, want := range []string{
		"--no-detail",
		"plain prints the SHA-256 only",
		"--full-decode",
		"<src>",
		"Detailed metadata and SHA-256 are included by",
		"read-only",
	} {
		assertContains(t, out, want)
	}
	assertNotContains(t, out, "--detail")
	assertNotContains(t, out, "--input")
}

// s3：环境变量契约、配置优先级与 path-style 有效默认值必须在 help 中说明。
func TestS3HelpContract(t *testing.T) {
	out := helpOutput(t, "s3")

	for _, want := range []string{
		"ITB_S3_ENDPOINT",
		"ITB_S3_ACCESS_KEY_ID",
		"ITB_S3_SECRET_ACCESS_KEY",
		"ITB_S3_SESSION_TOKEN",
		"ITB_S3_BUCKET",
		"ITB_S3_FORCE_PATH_STYLE",
		"CLI flag > ITB_S3_* environment variable > built-in default",
		"enabled automatically for loopback endpoints and endpoints on port 9000",
	} {
		assertContains(t, out, want)
	}
}

// s3 upload：默认 key、Content-Type 自动检测、metadata 与标准 HTTP
// 响应头能力必须在 help 中可读。
func TestS3UploadHelpContract(t *testing.T) {
	out := helpOutput(t, "s3", "upload")

	for _, want := range []string{
		"<src>", "[key]",
		"the object key is basename(<src>)",
		"--metadata",
		"--cache-control",
		"--content-disposition",
		"--content-encoding",
		"--verify",
		"--format",
		"itb-sha256",
		"detected from file content",
		"auto-detect",
	} {
		assertContains(t, out, want)
	}
	assertNotContains(t, out, "--input")
	assertNotContains(t, out, "--key")
}

// s3 download：默认输出是对象键最后一段，不是完整键名；
// 下载校验能力必须在 help 中可读。
func TestS3DownloadHelpContract(t *testing.T) {
	out := helpOutput(t, "s3", "download")

	for _, want := range []string{
		"<key>",
		"[dst]",
		"current",
		"last segment of the object key",
		"--verify",
		"--verify-sha256",
		"--format",
		"partial",
	} {
		assertContains(t, out, want)
	}
	assertNotContains(t, out, "默认使用对象键名")
	assertNotContains(t, out, "--key")
	assertNotContains(t, out, "--output")
}

func TestS3ObjectSelectorHelpContract(t *testing.T) {
	for _, tt := range []struct {
		name   string
		args   []string
		want   string
		banned string
	}{
		{"stat", []string{"s3", "stat"}, "<key>", "--key"},
		{"delete", []string{"s3", "delete"}, "<key>", "--key"},
		{"list", []string{"s3", "list"}, "[prefix]", "--prefix"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			out := helpOutput(t, tt.args...)
			assertContains(t, out, tt.want)
			assertNotContains(t, out, tt.banned)
		})
	}
}

// TestAllHelpIsEnglish 递归遍历整个命令树，对每个命令实际渲染
// `--help` 并断言无 Han 字符。未来任何新增命令的中文 Usage/
// Description/flag 文案都会在这里直接失败。
func TestAllHelpIsEnglish(t *testing.T) {
	var walk func(path []string, cmd *cli.Command)
	walk = func(path []string, cmd *cli.Command) {
		t.Run(strings.Join(append([]string{"itb"}, path...), " "), func(t *testing.T) {
			assertNoHan(t, helpOutput(t, path...))
		})
		for _, sub := range cmd.Commands {
			// 显式复制，避免 append 复用底层数组污染兄弟分支的路径
			subPath := append(append([]string{}, path...), sub.Name)
			walk(subPath, sub)
		}
	}
	walk(nil, testApp())
}

// TestFlagDefaultContracts 锁定普通 flag 默认值（单一来源：flag Value）
// 与计算型默认值的 DefaultText 展示。命令级/位置参数级语义默认
// （_resized、PSNR + MS-SSIM 等）由各命令的 help contract 检查
// Description 关键文本，不映射成 flag value。
func TestFlagDefaultContracts(t *testing.T) {
	tests := []struct {
		name        string
		path        []string
		flag        string
		want        string
		defaultText bool
	}{
		{"compress quality", []string{"compress"}, "quality", "80", false},
		{"resize mode", []string{"resize"}, "mode", `"fit"`, false},
		{"resize anchor", []string{"resize"}, "anchor", `"center"`, false},
		{"resize filter", []string{"resize"}, "filter", `"lanczos"`, false},
		{"convert quality", []string{"convert"}, "quality", "80", false},
		{"convert background", []string{"convert"}, "background", `"#FFFFFF"`, false},
		{"watermark mode", []string{"watermark"}, "mode", `"position"`, false},
		{"watermark opacity", []string{"watermark"}, "opacity", "0.5", false},
		{"watermark position", []string{"watermark"}, "position", `"bottom-right"`, false},
		{"watermark angle", []string{"watermark"}, "angle", "30", false},
		{"watermark margin", []string{"watermark"}, "margin", "0.04", false},
		{"watermark scale", []string{"watermark"}, "scale", "0.2", false},
		{"watermark color auto", []string{"watermark"}, "color", "auto", true},
		{"watermark font auto", []string{"watermark"}, "font", "auto", true},
		{"watermark font-size auto", []string{"watermark"}, "font-size", "auto", true},
		{"watermark space auto", []string{"watermark"}, "space", "auto", true},
		{"inspect format", []string{"inspect"}, "format", `"table"`, false},
		{"s3 region", []string{"s3"}, "region", `"us-east-1"`, false},
		{"s3 list max-keys", []string{"s3", "list"}, "max-keys", "1000", false},
		{"s3 list format", []string{"s3", "list"}, "format", `"table"`, false},
		{"s3 upload content-type auto-detect", []string{"s3", "upload"}, "content-type", "auto-detect", true},
		{"serve addr", []string{"serve"}, "addr", `"127.0.0.1:8080"`, false},
		{"serve max-upload", []string{"serve"}, "max-upload", `"64MiB"`, false},
		{"serve max-pixels", []string{"serve"}, "max-pixels", "50000000", false},
		{"serve max-dimension", []string{"serve"}, "max-dimension", "16384", false},
		{"serve max-concurrent", []string{"serve"}, "max-concurrent", "2", false},
		{"serve max-working-bytes", []string{"serve"}, "max-working-bytes", `"512MiB"`, false},
		{"serve timeout", []string{"serve"}, "timeout", "2m0s", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := findCommand(t, tt.path)
			flag := findFlag(t, cmd, tt.flag)
			doc, ok := flag.(cli.DocGenerationFlag)
			if !ok {
				t.Fatalf("flag --%s does not implement DocGenerationFlag", tt.flag)
			}
			got := doc.GetValue()
			if tt.defaultText {
				got = doc.GetDefaultText()
			}
			if got != tt.want {
				t.Fatalf("%s = %q, want %q", tt.flag, got, tt.want)
			}
		})
	}
}

// TestRemovedFlagsStayOutOfHelp 锁定已删除的旧 flag 名称不得重新进入
// 任何命令的 flag 面（含 MutuallyExclusiveFlags 组内）。
func TestRemovedFlagsStayOutOfHelp(t *testing.T) {
	banned := map[string]bool{
		"input":  true,
		"output": true,
		"to":     true,
		"key":    true,
		"prefix": true,
		"tile":   true,
	}

	var check func(path []string, cmd *cli.Command)
	check = func(path []string, cmd *cli.Command) {
		flags := append([]cli.Flag{}, cmd.Flags...)
		for _, group := range cmd.MutuallyExclusiveFlags {
			for _, fs := range group.Flags {
				flags = append(flags, fs...)
			}
		}
		for _, f := range flags {
			for _, name := range f.Names() {
				if banned[name] {
					t.Errorf("%s still defines removed flag --%s", strings.Join(append([]string{"itb"}, path...), " "), name)
				}
			}
		}
		for _, sub := range cmd.Commands {
			subPath := append(append([]string{}, path...), sub.Name)
			check(subPath, sub)
		}
	}
	check(nil, testApp())
}

// findCommand 沿 path 逐级查找命令，找不到时 fail。
func findCommand(t *testing.T, path []string) *cli.Command {
	t.Helper()
	cmd := testApp()
	for _, name := range path {
		var next *cli.Command
		for _, sub := range cmd.Commands {
			if sub.Name == name {
				next = sub
				break
			}
		}
		if next == nil {
			t.Fatalf("command %q not found under %q", name, cmd.Name)
		}
		cmd = next
	}
	return cmd
}

// findFlag 按名称查找命令的 flag（含 MutuallyExclusiveFlags 组内）。
func findFlag(t *testing.T, cmd *cli.Command, name string) cli.Flag {
	t.Helper()
	flags := append([]cli.Flag{}, cmd.Flags...)
	for _, group := range cmd.MutuallyExclusiveFlags {
		for _, fs := range group.Flags {
			flags = append(flags, fs...)
		}
	}
	for _, f := range flags {
		if slices.Contains(f.Names(), name) {
			return f
		}
	}
	t.Fatalf("flag --%s not found on %q", name, cmd.Name)
	return nil
}
