package compress

import (
	"errors"
	"strings"
	"testing"
)

func TestCommandError(t *testing.T) {
	tests := []struct {
		name   string
		stderr string
		want   string
	}{
		{
			name:   "glibc incompatibility",
			stderr: "pngquant: version `GLIBC_2.28' not found",
			want:   "当前 Linux 运行环境无法满足内置压缩器的 glibc 依赖；官方 Linux 构建兼容基线为 glibc >= 2.28",
		},
		{
			name:   "ordinary native error",
			stderr: "invalid input image",
			want:   "native command failed: invalid input image",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := commandError(errors.New("native command failed"), tt.stderr)
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("commandError() = %q, want substring %q", err, tt.want)
			}
		})
	}
}
