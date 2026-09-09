package inspect

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"strings"
)

// svgScanLimit 限制 SVG 识别的扫描量：SVG 的根元素必须出现在文件
// 头部（此前只允许 BOM、XML 声明、DOCTYPE 与注释），超过限制即判定
// 非 SVG，避免对大二进制文件做无谓的流式解析。
const svgScanLimit = 64 << 10

// sniffSVG 流式识别 SVG：找到文档第一个 element 并要求它是 <svg>
//（任意 namespace，含无 namespace）。允许根元素前出现：
//
//   - UTF-8 BOM
//   - XML 声明（<?xml ...?>）
//   - DOCTYPE（<!DOCTYPE ...>）
//   - 注释（<!-- ... -->）
//   - 空白
//
// HTML 改名为 .svg（根元素是 <html>）、非 XML 内容、被截断的 XML
// 一律返回 false。识别失败与"不是 SVG"不做区分——调用方只关心
// 内容能否被认定为 SVG。
func sniffSVG(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	buffered := bufio.NewReader(io.LimitReader(f, svgScanLimit))

	// encoding/xml 不处理 UTF-8 BOM，需要显式剥除
	if head, err := buffered.Peek(3); err == nil && bytes.Equal(head, []byte("\xef\xbb\xbf")) {
		if _, err := buffered.Discard(3); err != nil {
			return false
		}
	}

	decoder := xml.NewDecoder(buffered)
	for {
		token, err := decoder.Token()
		if err != nil {
			// 语法错误 / EOF：都不是合法 SVG
			return false
		}
		switch t := token.(type) {
		case xml.ProcInst, xml.Comment, xml.Directive:
			// 声明 / DOCTYPE / 注释：继续
		case xml.CharData:
			// 根元素前只允许空白文本
			if strings.TrimSpace(string(t)) != "" {
				return false
			}
		case xml.StartElement:
			return t.Name.Local == "svg"
		case xml.EndElement:
			return false
		}
	}
}
