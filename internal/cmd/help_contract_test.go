package cmd

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"unicode"
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

// s3：环境变量契约与配置优先级必须在 help 中说明。
func TestS3HelpContract(t *testing.T) {
	out := helpOutput(t, "s3")

	for _, want := range []string{
		"ITB_S3_ENDPOINT",
		"ITB_S3_ACCESS_KEY_ID",
		"ITB_S3_SECRET_ACCESS_KEY",
		"ITB_S3_SESSION_TOKEN",
		"ITB_S3_BUCKET",
		"ITB_S3_FORCE_PATH_STYLE",
		"CLI flag > ITB_S3_*",
	} {
		assertContains(t, out, want)
	}
}

// s3 upload：metadata 与标准 HTTP 响应头能力必须在 help 中可读。
func TestS3UploadHelpContract(t *testing.T) {
	out := helpOutput(t, "s3", "upload")

	for _, want := range []string{
		"<src>", "[key]",
		"--metadata",
		"--cache-control",
		"--content-disposition",
		"--content-encoding",
		"--verify",
		"--format",
		"itb-sha256",
		"按文件内容检测",
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

	assertContains(t, out, "<key>")
	assertContains(t, out, "[dst]")
	assertContains(t, out, "当前目录")
	assertContains(t, out, "最后一段")
	assertContains(t, out, "--verify")
	assertContains(t, out, "--verify-sha256")
	assertContains(t, out, "--format")
	assertContains(t, out, "partial")
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
