package inspect

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSniffSVGVariants 覆盖合法前置元素与必须拒绝的内容。
func TestSniffSVGVariants(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"裸 svg", `<svg xmlns="http://www.w3.org/2000/svg"></svg>`, true},
		{"带 XML 声明", `<?xml version="1.0" encoding="UTF-8"?><svg/>`, true},
		{"声明+注释", `<?xml version="1.0"?><!-- generated --><svg/>`, true},
		{"DOCTYPE", `<?xml version="1.0"?><!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN" "http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd"><svg/>`, true},
		{"多个注释", `<!-- a --><!-- b --><svg width="1"/>`, true},
		{"无 namespace", `<svg></svg>`, true},
		{"属性跨行", "<svg\n  width=\"10\"\n  height=\"10\"\n/>", true},
		{"HTML 改名 .svg", `<!DOCTYPE html><html><body></body></html>`, false},
		{"HTML 无 doctype", `<html><body>hi</body></html>`, false},
		{"根元素前有文本", `oops <svg/>`, false},
		{"截断 XML", `<?xml version="1.0"?><svg`, false},
		{"空文件", ``, false},
		{"二进制垃圾", "\x00\x01\x02\xff\xfe", false},
		{"svg 拼写在注释里", `<!-- <svg/> --><html/>`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTextFile(t, "probe.svg", tt.content)
			if got := sniffSVG(path); got != tt.want {
				t.Errorf("sniffSVG = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestSniffSVGBOM UTF-8 BOM 前缀合法。
func TestSniffSVGBOM(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bom.svg")
	if err := os.WriteFile(path, append([]byte("\xef\xbb\xbf"), []byte(`<?xml version="1.0"?><svg/>`)...), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if !sniffSVG(path) {
		t.Fatal("BOM-prefixed SVG must be recognized")
	}
}

// TestSniffSVGLargeBinary 前置大二进制内容不得触发无界解析。
func TestSniffSVGLargeBinary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "big.bin")
	data := make([]byte, 128<<10) // 128KB 零字节
	for i := range data {
		data[i] = byte(i % 251)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if sniffSVG(path) {
		t.Fatal("large binary must not be recognized as svg")
	}
}

// TestInspectSVGFile 端到端：SVG 识别为合法图片内容但不做 raster 解码。
func TestInspectSVGFile(t *testing.T) {
	svg := `<?xml version="1.0"?><!-- vector --><svg xmlns="http://www.w3.org/2000/svg" width="120" height="60"><rect width="120" height="60"/></svg>`
	path := writeTextFile(t, "vector.svg", svg)

	t.Run("默认检查", func(t *testing.T) {
		result, err := File(path, Options{NoHash: true})
		if err != nil {
			t.Fatalf("File: %v", err)
		}
		if !result.Content.Recognized || result.Content.Format != "svg" {
			t.Fatalf("content = %+v, want recognized svg", result.Content)
		}
		if result.Content.DecodeSupported {
			t.Error("svg decode_supported must be false")
		}
		// 不支持 raster 解码不是"图片损坏"：无 error 对象
		if result.Error != nil {
			t.Errorf("svg must not produce error object: %+v", result.Error)
		}
		// image 对象缺省：SVG 尺寸属于可选信息
		if result.Image != nil {
			t.Errorf("svg must not produce image info: %+v", result.Image)
		}
	})

	t.Run("strict 不因 SVG 失败", func(t *testing.T) {
		if _, err := File(path, Options{NoHash: true, Strict: true}); err != nil {
			t.Fatalf("strict File on valid svg: %v", err)
		}
	})

	t.Run("full-decode 记录 warning 而非报错", func(t *testing.T) {
		result, err := File(path, Options{NoHash: true, FullDecode: true})
		if err != nil {
			t.Fatalf("File: %v", err)
		}
		if len(result.Warnings) == 0 {
			t.Error("expected a warning noting full decode is unsupported")
		}
	})

	t.Run("HTML 改名 .svg 不得被识别", func(t *testing.T) {
		htmlPath := writeTextFile(t, "page.svg", `<!DOCTYPE html><html><body>x</body></html>`)
		result, err := File(htmlPath, Options{NoHash: true})
		if err != nil {
			t.Fatalf("File: %v", err)
		}
		if result.Content.Recognized {
			t.Fatalf("html content must not be recognized as svg: %+v", result.Content)
		}
	})
}
