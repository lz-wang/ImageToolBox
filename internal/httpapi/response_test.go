package httpapi

import "testing"

func TestContentDisposition(t *testing.T) {
	t.Run("ASCII 文件名保持兼容", func(t *testing.T) {
		got := contentDisposition("photo_resized.jpg")
		want := `attachment; filename="photo_resized.jpg"`
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("中文文件名使用 UTF-8 扩展参数", func(t *testing.T) {
		got := contentDisposition("滑稽_resized.jpg")
		want := `attachment; filename="download.jpg"; filename*=UTF-8''%E6%BB%91%E7%A8%BD_resized.jpg`
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})
}
