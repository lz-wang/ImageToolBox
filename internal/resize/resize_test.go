package resize

import (
	"image"
	"image/color"
	"testing"
)

func TestApplyPercent(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 200, 100))
	got, err := Apply(img, Options{Percent: "50%"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Bounds().Dx() != 100 || got.Bounds().Dy() != 50 {
		t.Fatalf("got %dx%d, want 100x50", got.Bounds().Dx(), got.Bounds().Dy())
	}
}

func TestApplyWidthOnly(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 200, 100))
	got, err := Apply(img, Options{Width: 50})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Bounds().Dx() != 50 || got.Bounds().Dy() != 25 {
		t.Fatalf("got %dx%d, want 50x25", got.Bounds().Dx(), got.Bounds().Dy())
	}
}

func TestApplyStretch(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 200, 100))
	got, err := Apply(img, Options{Width: 50, Height: 50, Mode: ModeStretch})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Bounds().Dx() != 50 || got.Bounds().Dy() != 50 {
		t.Fatalf("got %dx%d, want 50x50", got.Bounds().Dx(), got.Bounds().Dy())
	}
}

func TestApplyFillUsesAnchor(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 200, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.NRGBA{255, 0, 0, 255})
		}
		for x := 100; x < 200; x++ {
			img.Set(x, y, color.NRGBA{0, 0, 255, 255})
		}
	}

	left, err := Apply(img, Options{Width: 50, Height: 50, Mode: ModeFill, Anchor: "left"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	right, err := Apply(img, Options{Width: 50, Height: 50, Mode: ModeFill, Anchor: "right"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lr, _, _, _ := left.At(5, 25).RGBA()
	rr, _, bb, _ := right.At(45, 25).RGBA()
	if lr == 0 {
		t.Fatal("expected left-anchored image to keep red area")
	}
	if rr != 0 || bb == 0 {
		t.Fatal("expected right-anchored image to keep blue area")
	}
}

func TestValidateOptions(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 100, 100))
	_, err := Apply(img, Options{Percent: "50%", Width: 50})
	if err == nil {
		t.Fatal("expected conflict error")
	}

	_, err = Apply(img, Options{Mode: ModeFill, Width: 50})
	if err == nil {
		t.Fatal("expected fill validation error")
	}
}

func TestDefaultResizeContract(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 200, 100))
	got, err := Apply(img, Options{Width: 50})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if got.Bounds() != image.Rect(0, 0, 50, 25) {
		t.Fatalf("default fit dimensions = %v", got.Bounds())
	}
}

// TestApplyPercentUpscale 验证 percent 语义是按百分比精确缩放：
// 200% 必须真正放大，而不是被 fit 的"只缩小不放大"逻辑吞掉。
func TestApplyPercentUpscale(t *testing.T) {
	for _, tt := range []struct {
		percent string
		srcW    int
		srcH    int
		wantW   int
		wantH   int
	}{
		{percent: "200%", srcW: 100, srcH: 100, wantW: 200, wantH: 200},
		{percent: "150%", srcW: 200, srcH: 100, wantW: 300, wantH: 150},
		{percent: "50%", srcW: 100, srcH: 100, wantW: 50, wantH: 50},
	} {
		t.Run(tt.percent, func(t *testing.T) {
			img := image.NewNRGBA(image.Rect(0, 0, tt.srcW, tt.srcH))
			got, err := Apply(img, Options{Percent: tt.percent})
			if err != nil {
				t.Fatalf("Apply() error = %v", err)
			}
			if got.Bounds().Dx() != tt.wantW || got.Bounds().Dy() != tt.wantH {
				t.Fatalf("Apply(percent=%s) = %dx%d, want %dx%d", tt.percent, got.Bounds().Dx(), got.Bounds().Dy(), tt.wantW, tt.wantH)
			}
		})
	}
}

func TestResolve(t *testing.T) {
	tests := []struct {
		name   string
		bounds image.Rectangle
		opts   Options
		width  int
		height int
	}{
		// fit 单边指定时推导另一边，Resolve 结果等于真实输出尺寸。
		{name: "width only keeps aspect ratio", bounds: image.Rect(0, 0, 4000, 2000), opts: Options{Width: 1000}, width: 1000, height: 500},
		{name: "height only keeps aspect ratio", bounds: image.Rect(0, 0, 4000, 2000), opts: Options{Height: 500}, width: 1000, height: 500},
		{name: "stretch single side", bounds: image.Rect(0, 0, 4000, 2000), opts: Options{Width: 1000, Mode: ModeStretch}, width: 1000, height: 500},
		{name: "percent", bounds: image.Rect(0, 0, 100, 100), opts: Options{Percent: "50%"}, width: 50, height: 50},
		{name: "percent upscale", bounds: image.Rect(0, 0, 100, 100), opts: Options{Percent: "1000000%"}, width: 1_000_000, height: 1_000_000},
		{name: "explicit box", bounds: image.Rect(0, 0, 4000, 2000), opts: Options{Width: 100, Height: 50}, width: 100, height: 50},
		// fit 的 box 只是包围盒：源图在 box 内时输出等于源图尺寸，
		// 源图更大时按宽高比贴边缩小（与 imaging.Fit 完全一致）。
		{name: "fit bounding box larger than source", bounds: image.Rect(0, 0, 1000, 100), opts: Options{Width: 20000, Height: 100}, width: 1000, height: 100},
		{name: "fit box within both sides clones source", bounds: image.Rect(0, 0, 100, 100), opts: Options{Width: 500, Height: 500}, width: 100, height: 100},
		{name: "fit downscale keeps aspect", bounds: image.Rect(0, 0, 1000, 100), opts: Options{Width: 200, Height: 200}, width: 200, height: 20},
		{name: "stretch box is exact", bounds: image.Rect(0, 0, 1000, 100), opts: Options{Width: 20000, Height: 100, Mode: ModeStretch}, width: 20000, height: 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := Resolve(tt.bounds, tt.opts)
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			if plan.Width != tt.width || plan.Height != tt.height {
				t.Fatalf("Resolve() = %dx%d, want %dx%d", plan.Width, plan.Height, tt.width, tt.height)
			}
		})
	}
	t.Run("resolve matches apply output", func(t *testing.T) {
		img := image.NewNRGBA(image.Rect(0, 0, 200, 100))
		for _, opts := range []Options{
			{Width: 50},
			{Height: 25},
			{Percent: "33%"},
			{Percent: "200%"},
			{Width: 50, Height: 50, Mode: ModeStretch},
			// fit 放大 box：真实输出等于源图（Fit 直接 Clone）。
			{Width: 500, Height: 500},
			{Width: 500, Height: 500, Mode: ModeFit},
			// fit 缩小 box：按宽高比贴边。
			{Width: 80, Height: 80},
		} {
			plan, err := Resolve(img.Bounds(), opts)
			if err != nil {
				t.Fatalf("Resolve(%+v) error = %v", opts, err)
			}
			got, err := Apply(img, opts)
			if err != nil {
				t.Fatalf("Apply(%+v) error = %v", opts, err)
			}
			if got.Bounds().Dx() != plan.Width || got.Bounds().Dy() != plan.Height {
				t.Fatalf("Apply(%+v) = %dx%d, Resolve planned %dx%d", opts, got.Bounds().Dx(), got.Bounds().Dy(), plan.Width, plan.Height)
			}
		}
	})
	t.Run("invalid options return errors", func(t *testing.T) {
		bounds := image.Rect(0, 0, 10, 10)
		for _, opts := range []Options{
			{},
			{Percent: "50%", Width: 10},
			{Percent: "NaN%"},
			{Percent: "Inf%"},
			{Mode: ModeFill, Width: 10},
			{Mode: "bogus"},
			{Width: 10, Anchor: "bogus"},
			{Width: 10, Filter: "bogus"},
		} {
			if _, err := Resolve(bounds, opts); err == nil {
				t.Fatalf("Resolve(%+v) = nil error, want error", opts)
			}
		}
	})
}
