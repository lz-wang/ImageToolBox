package compress

import "testing"

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
