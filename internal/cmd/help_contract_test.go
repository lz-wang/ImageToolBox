package cmd

import (
	"bytes"
	"context"
	"strings"
	"testing"
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

// 根 help：命令目录只能来自 urfave 生成的 COMMANDS，
// 禁止手写命令清单回归（"功能:" 是旧手写清单的标志）。
func TestRootHelpContract(t *testing.T) {
	out := helpOutput(t)

	for _, name := range []string{"compress", "resize", "crop", "convert", "watermark", "inspect", "s3", "serve", "version"} {
		assertContains(t, out, name)
	}
	assertNotContains(t, out, "功能:")
	assertContains(t, out, "--version")
}

// compress：默认非破坏式输出与 --in-place 契约。
func TestCompressHelpContract(t *testing.T) {
	out := helpOutput(t, "compress")

	assertContains(t, out, "--in-place")
	assertContains(t, out, "_compressed")
	assertContains(t, out, "1-100")
	assertContains(t, out, "PNG")
	assertContains(t, out, "JPEG")
}

// resize：三种模式与参数组合规则必须在 help 中可读。
func TestResizeHelpContract(t *testing.T) {
	out := helpOutput(t, "resize")

	for _, want := range []string{
		"--percent", "--width", "--height",
		"fit", "fill", "stretch",
		"--percent 不能与 --width / --height 同时使用",
		"fill 必须同时指定宽度和高度",
		"（像素）",
	} {
		assertContains(t, out, want)
	}
}

// convert：格式特定参数语义（quality/lossless/background）必须在 help
// 中可读，且不得回退到旧的模糊文案。
func TestConvertHelpContract(t *testing.T) {
	out := helpOutput(t, "convert")

	for _, want := range []string{
		"JPEG/WebP 输出质量",
		"PNG 忽略该参数",
		"使用 WebP 无损编码",
		"PNG 始终为无损格式",
		"输出 JPEG 时透明区域使用的背景色",
		"必须为不透明颜色",
	} {
		assertContains(t, out, want)
	}
	assertNotContains(t, out, "透明图转不透明格式时的背景色")
	assertNotContains(t, out, "无损编码（webp/png）")
}

// crop：锚点参数组合规则与百分比范围。
func TestCropHelpContract(t *testing.T) {
	out := helpOutput(t, "crop")

	for _, want := range []string{
		"(0,100]",
		"left / right",
		"top / bottom",
		"必须同时提供 --width 和 --height",
	} {
		assertContains(t, out, want)
	}
}

// watermark：文字/图片两种来源、两种模式；
// 未实现的 capability（--tile）与错误的字体要求不得再出现。
func TestWatermarkHelpContract(t *testing.T) {
	out := helpOutput(t, "watermark")

	for _, want := range []string{"文字", "图片", "position", "repeat"} {
		assertContains(t, out, want)
	}
	for _, banned := range []string{"当前版本暂不支持", "需要指定字体", "--tile"} {
		assertNotContains(t, out, banned)
	}
}

// inspect：--no-detail 与 plain 格式语义。
func TestInspectHelpContract(t *testing.T) {
	out := helpOutput(t, "inspect")

	assertContains(t, out, "--no-detail")
	assertContains(t, out, "plain 仅输出 SHA-256")
}

// s3：环境变量契约与配置优先级必须在 help 中说明。
func TestS3HelpContract(t *testing.T) {
	out := helpOutput(t, "s3")

	for _, want := range []string{
		"ITB_S3_ENDPOINT",
		"ITB_S3_ACCESS_KEY_ID",
		"ITB_S3_SECRET_ACCESS_KEY",
		"ITB_S3_BUCKET",
		"CLI flag > ITB_S3_*",
	} {
		assertContains(t, out, want)
	}
}

// s3 download：默认输出是对象键最后一段，不是完整键名。
func TestS3DownloadHelpContract(t *testing.T) {
	out := helpOutput(t, "s3", "download")

	assertContains(t, out, "当前目录")
	assertContains(t, out, "最后一段")
	assertNotContains(t, out, "默认使用对象键名")
}
