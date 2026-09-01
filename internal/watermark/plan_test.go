package watermark

import (
	"image"
	"math"
	"strings"
	"testing"
)

func TestResolveImagePlan(t *testing.T) {
	scale := func(v float64) *float64 { return &v }
	t.Run("default scale matches image watermark formula", func(t *testing.T) {
		plan, err := ResolveImagePlan(image.Rect(0, 0, 1000, 1000), image.Rect(0, 0, 100, 100), Options{ImagePath: "logo.png"})
		if err != nil {
			t.Fatalf("ResolveImagePlan() error = %v", err)
		}
		if plan.TargetWidth != 200 || plan.TargetHeight != 200 {
			t.Fatalf("plan = %dx%d, want 200x200", plan.TargetWidth, plan.TargetHeight)
		}
	})
	t.Run("explicit scale", func(t *testing.T) {
		plan, err := ResolveImagePlan(image.Rect(0, 0, 100, 100), image.Rect(0, 0, 10, 10), Options{ImagePath: "logo.png", Scale: scale(0.5)})
		if err != nil {
			t.Fatalf("ResolveImagePlan() error = %v", err)
		}
		if plan.TargetWidth != 50 || plan.TargetHeight != 50 {
			t.Fatalf("plan = %dx%d, want 50x50", plan.TargetWidth, plan.TargetHeight)
		}
	})
	t.Run("huge scale produces huge target", func(t *testing.T) {
		plan, err := ResolveImagePlan(image.Rect(0, 0, 16, 16), image.Rect(0, 0, 8, 8), Options{ImagePath: "logo.png", Scale: scale(1000000)})
		if err != nil {
			t.Fatalf("ResolveImagePlan() error = %v", err)
		}
		if plan.TargetWidth <= 16 || plan.TargetHeight <= 16 {
			t.Fatalf("plan = %dx%d, want upscaled target", plan.TargetWidth, plan.TargetHeight)
		}
	})
	t.Run("invalid bounds", func(t *testing.T) {
		if _, err := ResolveImagePlan(image.Rect(0, 0, 0, 10), image.Rect(0, 0, 8, 8), Options{ImagePath: "logo.png"}); err == nil {
			t.Fatal("expected error for invalid base bounds")
		}
		if _, err := ResolveImagePlan(image.Rect(0, 0, 10, 10), image.Rect(0, 0, 8, 0), Options{ImagePath: "logo.png"}); err == nil {
			t.Fatal("expected error for invalid logo bounds")
		}
	})
}

func TestResolveWorkingSet(t *testing.T) {
	t.Run("position text mode has no mark or tile canvas", func(t *testing.T) {
		set, err := ResolveWorkingSet(image.Rect(0, 0, 100, 100), image.Rectangle{}, Options{Text: "mark"})
		if err != nil {
			t.Fatalf("ResolveWorkingSet() error = %v", err)
		}
		if set.MarkWidth != 0 || set.MarkHeight != 0 || set.TileWidth != 0 || set.TileHeight != 0 {
			t.Fatalf("set = %+v, want zero mark/tile", set)
		}
		want := int64(100 * 100 * 4 * 4)
		if set.Bytes() != want {
			t.Fatalf("Bytes() = %d, want %d", set.Bytes(), want)
		}
	})
	t.Run("image mode marks scaled logo", func(t *testing.T) {
		set, err := ResolveWorkingSet(image.Rect(0, 0, 100, 100), image.Rect(0, 0, 10, 10), Options{ImagePath: "logo.png"})
		if err != nil {
			t.Fatalf("ResolveWorkingSet() error = %v", err)
		}
		if set.MarkWidth != 20 || set.MarkHeight != 20 {
			t.Fatalf("mark = %dx%d, want 20x20", set.MarkWidth, set.MarkHeight)
		}
		want := int64(100*100*4+20*20*2) * 4
		if set.Bytes() != want {
			t.Fatalf("Bytes() = %d, want %d", set.Bytes(), want)
		}
	})
	t.Run("repeat mode estimates mark and rotated tile canvas", func(t *testing.T) {
		base := image.Rect(0, 0, 32, 16)
		set, err := ResolveWorkingSet(base, image.Rectangle{}, Options{Text: "mark", Mode: ModeRepeat})
		if err != nil {
			t.Fatalf("ResolveWorkingSet() error = %v", err)
		}
		// 自动字号 = max(min(32,16)/25, 16) = 16；画布 = (200, 64)。
		if set.MarkWidth != 200 || set.MarkHeight != 64 {
			t.Fatalf("mark = %dx%d, want 200x64", set.MarkWidth, set.MarkHeight)
		}
		c := int(math.Hypot(32, 16)) + 200*2
		if set.TileWidth < c || set.TileHeight < c {
			t.Fatalf("tile = %dx%d, want at least %d (rotate 放大上界)", set.TileWidth, set.TileHeight, c)
		}
		if set.Bytes() <= 0 {
			t.Fatal("Bytes() must be positive")
		}
	})
	t.Run("explicit font size drives mark canvas", func(t *testing.T) {
		size := 4096
		set, err := ResolveWorkingSet(image.Rect(0, 0, 16, 16), image.Rectangle{}, Options{Text: "水", Mode: ModeRepeat, FontSize: &size})
		if err != nil {
			t.Fatalf("ResolveWorkingSet() error = %v", err)
		}
		if set.MarkWidth != 4096*4 || set.MarkHeight != int(float64(4096)*2.5) {
			t.Fatalf("mark = %dx%d, want 16384x10240", set.MarkWidth, set.MarkHeight)
		}
		// 16384×10240 RGBA 仅 mark 一项（两份）已超过 512 MiB。
		if set.Bytes() <= 512<<20 {
			t.Fatalf("Bytes() = %d, want > 512MiB", set.Bytes())
		}
	})
	t.Run("requires exactly one source", func(t *testing.T) {
		if _, err := ResolveWorkingSet(image.Rect(0, 0, 10, 10), image.Rectangle{}, Options{}); err == nil {
			t.Fatal("expected error for empty options")
		}
		if _, err := ResolveWorkingSet(image.Rect(0, 0, 10, 10), image.Rectangle{}, Options{Text: "a", ImagePath: "b.png"}); err == nil {
			t.Fatal("expected error for both sources")
		}
	})
}

// TestWorkingSetMatchesRealCanvas 验证规划上界覆盖真实执行的画布公式。
func TestWorkingSetMatchesRealCanvas(t *testing.T) {
	fontSize := 100
	text := strings.Repeat("水", 10)
	set, err := ResolveWorkingSet(image.Rect(0, 0, 500, 400), image.Rectangle{}, Options{Text: text, Mode: ModeRepeat, FontSize: &fontSize})
	if err != nil {
		t.Fatalf("ResolveWorkingSet() error = %v", err)
	}
	// generateMark 的真实临时画布。
	canvas := markCanvasSize(fontSize, text)
	if canvas.X > set.MarkWidth || canvas.Y > set.MarkHeight {
		t.Fatalf("real mark canvas %v exceeds planned %dx%d", canvas, set.MarkWidth, set.MarkHeight)
	}
	// Watermarker.Apply 的真实平铺边长。
	c := int(math.Hypot(500, 400)) + max(set.MarkWidth, set.MarkHeight)*2
	if c > set.TileWidth {
		t.Fatalf("real tile side %d exceeds planned %d", c, set.TileWidth)
	}
}
