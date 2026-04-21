package crop

import (
	"image"
	"testing"
)

func TestParsePercent(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    float64
		wantErr bool
	}{
		{name: "valid 40", input: "40%", want: 40},
		{name: "valid 100", input: "100%", want: 100},
		{name: "zero", input: "0%", wantErr: true},
		{name: "over 100", input: "101%", wantErr: true},
		{name: "missing suffix", input: "40", wantErr: true},
		{name: "invalid number", input: "abc%", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePercent(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateOptions(t *testing.T) {
	tests := []struct {
		name    string
		opts    Options
		wantErr bool
	}{
		{
			name:    "left with width",
			opts:    Options{Anchor: AnchorLeft, Width: "40%"},
			wantErr: false,
		},
		{
			name:    "top-left with both",
			opts:    Options{Anchor: AnchorTopLeft, Width: "40%", Height: "40%"},
			wantErr: false,
		},
		{
			name:    "left with height only",
			opts:    Options{Anchor: AnchorLeft, Height: "40%"},
			wantErr: true,
		},
		{
			name:    "center with width only",
			opts:    Options{Anchor: AnchorCenter, Width: "40%"},
			wantErr: true,
		},
		{
			name:    "top-left with width only",
			opts:    Options{Anchor: AnchorTopLeft, Width: "40%"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := validateOptions(tt.opts)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestComputeRect(t *testing.T) {
	tests := []struct {
		name string
		opts Options
		want image.Rectangle
	}{
		{
			name: "left 40",
			opts: Options{Anchor: AnchorLeft, Width: "40%"},
			want: image.Rect(0, 0, 40, 80),
		},
		{
			name: "right 40",
			opts: Options{Anchor: AnchorRight, Width: "40%"},
			want: image.Rect(60, 0, 100, 80),
		},
		{
			name: "top 40",
			opts: Options{Anchor: AnchorTop, Height: "40%"},
			want: image.Rect(0, 0, 100, 32),
		},
		{
			name: "bottom 40",
			opts: Options{Anchor: AnchorBottom, Height: "40%"},
			want: image.Rect(0, 48, 100, 80),
		},
		{
			name: "top-left 40x40",
			opts: Options{Anchor: AnchorTopLeft, Width: "40%", Height: "40%"},
			want: image.Rect(0, 0, 40, 32),
		},
		{
			name: "center 40x40",
			opts: Options{Anchor: AnchorCenter, Width: "40%", Height: "40%"},
			want: image.Rect(30, 24, 70, 56),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ComputeRect(100, 80, tt.opts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestComputeRectOddDimensions(t *testing.T) {
	rect, err := ComputeRect(101, 99, Options{
		Anchor: AnchorCenter,
		Width:  "40%",
		Height: "40%",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rect.Min.X < 0 || rect.Min.Y < 0 || rect.Max.X > 101 || rect.Max.Y > 99 {
		t.Fatalf("rect out of bounds: %v", rect)
	}
	if rect.Dx() < 1 || rect.Dy() < 1 {
		t.Fatalf("rect too small: %v", rect)
	}
	if rect != image.Rect(30, 29, 70, 69) {
		t.Fatalf("got %v, want %v", rect, image.Rect(30, 29, 70, 69))
	}
}
