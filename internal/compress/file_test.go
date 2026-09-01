package compress

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"imagetoolbox/internal/imageio"
)

func TestFileOptionsNormalizeAndValidate(t *testing.T) {
	tests := []struct {
		name    string
		opts    FileOptions
		want    int
		wantErr bool
	}{
		{name: "default quality", want: DefaultQuality},
		{name: "minimum quality", opts: FileOptions{Quality: 1}, want: 1},
		{name: "maximum quality", opts: FileOptions{Quality: 100}, want: 100},
		{name: "negative quality", opts: FileOptions{Quality: -1}, want: -1, wantErr: true},
		{name: "quality above maximum", opts: FileOptions{Quality: 101}, want: 101, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.opts.Normalize()
			if tt.opts.Quality != tt.want {
				t.Fatalf("quality = %d, want %d", tt.opts.Quality, tt.want)
			}
			if err := tt.opts.Validate(); (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCompressFileRejectsSameFileBeforeStartingCommands(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.jpg")
	if err := os.WriteFile(input, []byte("not an image"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := CompressFile(context.Background(), input, input, FileOptions{})
	if !errors.Is(err, imageio.ErrSameFile) {
		t.Fatalf("CompressFile() error = %v, want imageio.ErrSameFile", err)
	}
}

func TestCompressFileStopsBeforeStartingCommandsWhenContextIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := CompressFile(ctx, "missing.png", "output.png", FileOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("CompressFile() error = %v, want context.Canceled", err)
	}
}
