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
			{Width: 50, Height: 50, Mode: ModeStretch},
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
