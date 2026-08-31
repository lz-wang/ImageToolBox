package imageio

import (
	"errors"
	"testing"
)

func TestNormalizeFormat(t *testing.T) {
	tests := []struct {
		value string
		want  Format
	}{
		{value: "png", want: FormatPNG},
		{value: ".PNG", want: FormatPNG},
		{value: "jpg", want: FormatJPEG},
		{value: " jpeg ", want: FormatJPEG},
		{value: "webp", want: FormatWEBP},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got, err := NormalizeFormat(tt.value)
			if err != nil {
				t.Fatalf("NormalizeFormat(%q) error = %v", tt.value, err)
			}
			if got != tt.want {
				t.Fatalf("NormalizeFormat(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
	t.Run("unsupported format is a typed error", func(t *testing.T) {
		_, err := NormalizeFormat("gif")
		if !errors.Is(err, ErrUnsupportedFormat) {
			t.Fatalf("NormalizeFormat(gif) error = %v, want ErrUnsupportedFormat", err)
		}
	})
}
