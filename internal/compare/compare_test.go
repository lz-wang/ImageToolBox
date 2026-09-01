package compare

import (
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"imagetoolbox/internal/imageio"
)

func TestDefaultMetrics(t *testing.T) {
	if DefaultMetrics != MetricPSNR|MetricMSSSIM {
		t.Fatalf("DefaultMetrics = %d, want PSNR|MSSSIM", DefaultMetrics)
	}
	if allMetrics != MetricPSNR|MetricSSIM|MetricMSSSIM {
		t.Fatalf("allMetrics = %d, want all three metric bits", allMetrics)
	}
}

func TestOptionsNormalizeValidate(t *testing.T) {
	var opts Options
	opts.Normalize()
	if opts.Metrics != DefaultMetrics {
		t.Fatalf("Normalize() Metrics = %d, want %d", opts.Metrics, DefaultMetrics)
	}
	if err := opts.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := (Options{}).Validate(); err == nil {
		t.Fatal("Validate should reject empty metrics")
	}
	if err := (Options{Metrics: Metrics(1 << 7)}).Validate(); err == nil {
		t.Fatal("Validate should reject unknown metric bits")
	}
}

// 显式指标选择：Result.Metrics 必须精确反映请求，未选指标的字段不参与。
func TestCompareImagesExplicitMetrics(t *testing.T) {
	src := gradientImage(200, 200)
	dst := distortImage(src)

	tests := []struct {
		name    string
		metrics Metrics
	}{
		{"仅 PSNR", MetricPSNR},
		{"仅 SSIM", MetricSSIM},
		{"仅 MS-SSIM", MetricMSSSIM},
		{"全部指标", MetricPSNR | MetricSSIM | MetricMSSSIM},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := CompareImages(context.Background(), src, dst, Options{Metrics: tt.metrics})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.Metrics != tt.metrics {
				t.Fatalf("Metrics = %d, want %d", res.Metrics, tt.metrics)
			}
			if res.Width != 200 || res.Height != 200 {
				t.Fatalf("dimensions = %dx%d, want 200x200", res.Width, res.Height)
			}
			if tt.metrics&MetricPSNR == 0 && res.PSNR != 0 {
				t.Fatal("PSNR should stay zero when not requested")
			}
			if tt.metrics&MetricSSIM == 0 && res.SSIM != 0 {
				t.Fatal("SSIM should stay zero when not requested")
			}
			if tt.metrics&MetricMSSSIM == 0 && res.MSSSIM != 0 {
				t.Fatal("MSSSIM should stay zero when not requested")
			}
		})
	}
}

// src == dst 是合法的自我比较 sanity check。
func TestCompareImagesSameImageAllowed(t *testing.T) {
	src := gradientImage(200, 200)
	res, err := CompareImages(context.Background(), src, src, Options{Metrics: allMetrics})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !math.IsInf(res.PSNR, 1) {
		t.Fatalf("PSNR = %v, want +Inf", res.PSNR)
	}
	if math.Abs(res.SSIM-1) > 1e-12 {
		t.Fatalf("SSIM = %v, want 1", res.SSIM)
	}
	if math.Abs(res.MSSSIM-1) > 1e-12 {
		t.Fatalf("MS-SSIM = %v, want 1", res.MSSSIM)
	}
}

func TestCompareImagesDimensionMismatch(t *testing.T) {
	_, err := CompareImages(context.Background(),
		gradientImage(1920/10, 1080/10), gradientImage(1280/10, 720/10),
		Options{Metrics: MetricPSNR})
	if err == nil || !strings.Contains(err.Error(), "图片尺寸不一致") {
		t.Fatalf("error = %v, want dimension mismatch", err)
	}
}

// SSIM 窗口约束：10×10 直接拒绝，ErrImageTooSmall。
func TestCompareImagesSSIMTooSmall(t *testing.T) {
	_, err := CompareImages(context.Background(),
		gradientImage(10, 10), gradientImage(10, 10), Options{Metrics: MetricSSIM})
	if !errors.Is(err, ErrImageTooSmall) {
		t.Fatalf("error = %v, want ErrImageTooSmall", err)
	}
}

// MS-SSIM 固定五尺度：160px 拒绝，161px 恰好满足第 5 尺度的完整窗口。
func TestCompareImagesMSSSIMMinDimension(t *testing.T) {
	for _, size := range []int{160, 161} {
		src := gradientImage(size, size)
		dst := distortImage(src)
		_, err := CompareImages(context.Background(), src, dst, Options{Metrics: MetricMSSSIM})
		if size == 160 {
			if !errors.Is(err, ErrImageTooSmall) {
				t.Fatalf("size %d: error = %v, want ErrImageTooSmall", size, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("size %d: unexpected error: %v", size, err)
		}
	}
}

// 极不相似（反相）的图片不能产生 NaN：cs 负值在幂运算前被钳制。
func TestCompareImagesVeryDifferentNoNaN(t *testing.T) {
	src := gradientImage(200, 200)
	dst := fillNRGBA(200, 200, func(x, y int) color.NRGBA {
		s := src.NRGBAAt(x, y)
		return color.NRGBA{R: 255 - s.R, G: 255 - s.G, B: 255 - s.B, A: 255}
	})
	res, err := CompareImages(context.Background(), src, dst, Options{Metrics: allMetrics})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for name, v := range map[string]float64{"PSNR": res.PSNR, "SSIM": res.SSIM, "MS-SSIM": res.MSSSIM} {
		if math.IsNaN(v) {
			t.Fatalf("%s is NaN for very different images", name)
		}
	}
	if res.MSSSIM < 0 || res.MSSSIM > 1 {
		t.Fatalf("MS-SSIM = %v, want within [0,1]", res.MSSSIM)
	}
}

func TestCompareImagesCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := CompareImages(ctx, gradientImage(64, 64), gradientImage(64, 64), Options{Metrics: MetricPSNR})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

// writeImage 把图片按扩展名写出（PNG/JPEG 走标准库，WebP 走 imageio）。
func writeImage(t *testing.T, path string, img image.Image, quality int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		err = png.Encode(f, img)
	case ".jpg", ".jpeg":
		err = jpeg.Encode(f, img, &jpeg.Options{Quality: quality})
	case ".webp":
		err = imageio.Encode(f, img, imageio.FormatWEBP, imageio.SaveOptions{Quality: quality})
	default:
		t.Fatalf("unsupported test image extension: %s", path)
	}
	if err != nil {
		t.Fatal(err)
	}
}

// 跨格式比较：JPEG ↔ WebP、PNG ↔ WebP，只要逻辑尺寸一致即合法。
func TestCompareFilesCrossFormat(t *testing.T) {
	src := gradientImage(192, 192)
	dst := distortImage(src)

	dir := t.TempDir()
	writeImage(t, filepath.Join(dir, "src.png"), src, 0)
	writeImage(t, filepath.Join(dir, "src.jpg"), src, 90)
	writeImage(t, filepath.Join(dir, "dst.webp"), dst, 90)

	tests := []struct {
		name string
		a, b string
	}{
		{"JPEG ↔ WebP", "src.jpg", "dst.webp"},
		{"PNG ↔ WebP", "src.png", "dst.webp"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := CompareFiles(context.Background(),
				filepath.Join(dir, tt.a), filepath.Join(dir, tt.b), Options{Metrics: MetricPSNR | MetricSSIM})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.Width != 192 || res.Height != 192 {
				t.Fatalf("dimensions = %dx%d, want 192x192", res.Width, res.Height)
			}
			if math.IsNaN(res.PSNR) || math.IsNaN(res.SSIM) {
				t.Fatalf("NaN in cross-format result: %+v", res)
			}
		})
	}
}

// 同一文件自我比较：CompareFiles 绝不调用 RejectSameFile。
func TestCompareFilesSameFileAllowed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "photo.png")
	writeImage(t, path, gradientImage(192, 192), 0)

	res, err := CompareFiles(context.Background(), path, path, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Metrics != DefaultMetrics {
		t.Fatalf("Metrics = %d, want default %d", res.Metrics, DefaultMetrics)
	}
	if !math.IsInf(res.PSNR, 1) {
		t.Fatalf("PSNR = %v, want +Inf", res.PSNR)
	}
	if math.Abs(res.MSSSIM-1) > 1e-9 {
		t.Fatalf("MS-SSIM = %v, want 1", res.MSSSIM)
	}
}

func TestCompareFilesUnsupportedFormat(t *testing.T) {
	dir := t.TempDir()
	bmp := filepath.Join(dir, "x.bmp")
	// 8 字节的 BMP 文件头（"BM" + 大小）即可被格式检测拒绝
	if err := os.WriteFile(bmp, []byte{'B', 'M', 0, 0, 0, 0, 0, 0}, 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := CompareFiles(context.Background(), bmp, bmp, Options{Metrics: MetricPSNR}); err == nil {
		t.Fatal("expected unsupported-format error, got nil")
	}
}
