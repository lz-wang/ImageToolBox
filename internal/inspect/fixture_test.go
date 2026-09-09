package inspect

import (
	"os"
	"path/filepath"
	"testing"
)

// writeSVG 写入以给定根元素内容为主体的 .svg 文件。
func writeSVG(t *testing.T, root string) string {
	t.Helper()
	return writeTextFile(t, "image.svg", root+"\n")
}

// writeTextFile 写入文本 fixture。
func writeTextFile(t *testing.T, name, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}
