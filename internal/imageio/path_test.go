package imageio

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRejectSameFile(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.png")
	if err := os.WriteFile(input, []byte("input"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		output  string
		prepare func(string) error
		wantErr error
	}{
		{name: "literal path", output: input, wantErr: ErrSameFile},
		{name: "equivalent path", output: filepath.Join(dir, ".", "input.png"), wantErr: ErrSameFile},
		{name: "hardlink", output: filepath.Join(dir, "hardlink.png"), prepare: func(output string) error { return os.Link(input, output) }, wantErr: ErrSameFile},
		{name: "symlink", output: filepath.Join(dir, "symlink.png"), prepare: func(output string) error { return os.Symlink(input, output) }, wantErr: ErrSameFile},
		{name: "different nonexistent output", output: filepath.Join(dir, "new.png")},
		{name: "different existing output", output: filepath.Join(dir, "other.png"), prepare: func(output string) error { return os.WriteFile(output, []byte("other"), 0o600) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.prepare != nil {
				if err := tt.prepare(tt.output); err != nil {
					t.Fatal(err)
				}
			}
			err := RejectSameFile(input, tt.output)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("RejectSameFile() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
