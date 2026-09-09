package s3

import (
	"bytes"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
)

// sniffLen 内容检测读取的文件头长度，与 http.DetectContentType 一致。
const sniffLen = 512

// contentTypes 扩展名 → Content-Type 兜底表。仅当内容检测无法识别
// （返回 application/octet-stream）时使用；扩展名永远不覆盖内容检测。
var contentTypes = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
	".svg":  "image/svg+xml",
	".json": "application/json",
	".txt":  "text/plain",
	".html": "text/html",
	".css":  "text/css",
	".js":   "application/javascript",
	".pdf":  "application/pdf",
	".zip":  "application/zip",
}

// ResolveContentType 决定上传对象的 Content-Type，优先级：
//
//	显式指定（--content-type）
//	  ↓
//	内容 magic sniff（基于待上传内容的文件头 + SVG/JSON 特判）
//	  ↓
//	原始文件名的扩展名兜底表
//	  ↓
//	application/octet-stream
//
// 内容优先是为了防止"HTML/XML 错误页改名为 image.jpg"以 image/jpeg
// 上传：对象存储按 Content-Type 提供响应，错误的内容类型会被浏览器
// 当图片渲染请求执行。header 来自上传快照（实际 PUT body），
// originalName 始终是源文件名——快照的随机临时名不参与兜底。
func ResolveContentType(header []byte, originalName string, explicit string) string {
	if explicit != "" {
		return explicit
	}
	if ct := sniffContentTypeBytes(header); ct != "" {
		return ct
	}
	// 扩展名归一化为小写：foo.PNG / foo.JSON 这类大小写混合的
	// 文件名同样能命中兜底表
	if ct, ok := contentTypes[strings.ToLower(filepath.Ext(originalName))]; ok {
		return ct
	}
	return "application/octet-stream"
}

// sniffContentTypeBytes 对文件头字节做内容检测，返回空串表示未识别
// （调用方走扩展名兜底）。
func sniffContentTypeBytes(buf []byte) string {
	ct := http.DetectContentType(buf)
	if ct == "application/octet-stream" {
		return ""
	}
	// http.DetectContentType 对 SVG 只能给出 text/xml 或 text/plain
	// （其嗅探表不包含 image/svg+xml），需要识别 <svg> 根元素特判。
	if isSVG(buf, ct) {
		return "image/svg+xml"
	}
	// 其嗅探表同样不含 JSON：{"…"} / […] 会被归为 text/plain。
	if isJSON(buf, ct) {
		return "application/json"
	}
	return ct
}

// isJSON 判断 text/plain 内容是否为 JSON 文档。文件头完整读到时用
// json.Valid 严格校验；大文件只能看到截断前缀时退回结构前缀启发式
//（对象/数组的首个成员必须是字符串或立即闭合）。
func isJSON(data []byte, detectedContentType string) bool {
	if !strings.HasPrefix(detectedContentType, "text/plain") {
		return false
	}

	trimmed := bytes.TrimLeft(data, " \t\r\n")
	if len(trimmed) == 0 || (trimmed[0] != '{' && trimmed[0] != '[') {
		return false
	}
	if len(data) < sniffLen {
		return json.Valid(trimmed)
	}
	for _, prefix := range []string{`{"`, `["`, `{}`, `[]`} {
		if bytes.HasPrefix(trimmed, []byte(prefix)) {
			return true
		}
	}
	return false
}

// isSVG 判断 text/xml / text/plain 内容是否以 <svg> 为根元素。
// 允许前置 BOM、空白、XML 声明、doctype 与注释。
func isSVG(data []byte, detectedContentType string) bool {
	if !strings.HasPrefix(detectedContentType, "text/xml") &&
		!strings.HasPrefix(detectedContentType, "text/plain") {
		return false
	}

	rest := bytes.TrimLeft(data, "\xef\xbb\xbf \t\r\n")
	for {
		if bytes.HasPrefix(rest, []byte("<?xml")) || bytes.HasPrefix(rest, []byte("<!--")) {
			end := bytes.IndexByte(rest, '>')
			if end < 0 {
				return false
			}
			rest = bytes.TrimLeft(rest[end+1:], " \t\r\n")
			continue
		}
		if bytes.HasPrefix(rest, []byte("<!DOCTYPE")) || bytes.HasPrefix(rest, []byte("<!doctype")) {
			end := bytes.IndexByte(rest, '>')
			if end < 0 {
				return false
			}
			rest = bytes.TrimLeft(rest[end+1:], " \t\r\n")
			continue
		}
		break
	}
	return bytes.HasPrefix(rest, []byte("<svg"))
}
