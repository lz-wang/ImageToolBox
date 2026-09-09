package inspect

import (
	"bytes"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"strings"

	_ "github.com/deepteams/webp"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
)

// FormatSpec 是 inspect 内容识别层的格式注册表条目：一种可识别内容
// 格式的全部静态属性。格式名称、扩展名、MIME、能否 raster 解码在此
// 单点维护，extension_matches / MIME / decoder 注册 / 动画断言都从
// 这里推导，禁止在别处另立格式表。
type FormatSpec struct {
	// Name 是识别出的规范格式名（image.DecodeConfig 的格式名约定：
	// png/jpeg/gif/webp/bmp/tiff）
	Name string

	// CanonicalExtension 是规范扩展名（小写、带点）
	CanonicalExtension string

	// MIMEType 是该格式的权威 MIME 类型
	MIMEType string

	// DecodeSupported 表示是否存在注册的 raster decoder
	//（image.DecodeConfig / image.Decode 可用）
	DecodeSupported bool

	// FullDecodeSupported 表示是否支持完整解码（含逐帧能力）
	FullDecodeSupported bool

	// Aliases 是额外接受的扩展名（如 jpeg 的 .jpeg、tiff 的 .tif）
	Aliases []string
}

// formatRegistry 唯一事实来源。新增可识别格式时在此追加一行，
// 并确保对应 decoder 已在上文 blank import 注册。
var formatRegistry = []FormatSpec{
	{Name: "png", CanonicalExtension: ".png", MIMEType: "image/png", DecodeSupported: true, FullDecodeSupported: true},
	{Name: "jpeg", CanonicalExtension: ".jpg", MIMEType: "image/jpeg", DecodeSupported: true, FullDecodeSupported: true, Aliases: []string{".jpeg", ".jpe", ".jfif"}},
	{Name: "gif", CanonicalExtension: ".gif", MIMEType: "image/gif", DecodeSupported: true, FullDecodeSupported: true},
	{Name: "webp", CanonicalExtension: ".webp", MIMEType: "image/webp", DecodeSupported: true, FullDecodeSupported: true},
	{Name: "bmp", CanonicalExtension: ".bmp", MIMEType: "image/bmp", DecodeSupported: true, FullDecodeSupported: true},
	{Name: "tiff", CanonicalExtension: ".tiff", MIMEType: "image/tiff", DecodeSupported: true, FullDecodeSupported: true, Aliases: []string{".tif"}},
	{Name: "svg", CanonicalExtension: ".svg", MIMEType: "image/svg+xml", DecodeSupported: false, FullDecodeSupported: false},
}

// LookupFormat 按规范格式名查找注册表。
func LookupFormat(name string) (FormatSpec, bool) {
	for _, spec := range formatRegistry {
		if spec.Name == name {
			return spec, true
		}
	}
	return FormatSpec{}, false
}

// FormatByExtension 按文件扩展名（规范名或 alias，大小写不敏感）查找。
func FormatByExtension(ext string) (FormatSpec, bool) {
	ext = strings.ToLower(ext)
	for _, spec := range formatRegistry {
		if ext == spec.CanonicalExtension {
			return spec, true
		}
		for _, alias := range spec.Aliases {
			if ext == alias {
				return spec, true
			}
		}
	}
	return FormatSpec{}, false
}

// ExtensionMatches 报告文件扩展名是否与该格式的扩展名族一致。
func (s FormatSpec) ExtensionMatches(ext string) bool {
	ext = strings.ToLower(ext)
	if ext == "" || s.CanonicalExtension == "" {
		return false
	}
	if ext == s.CanonicalExtension {
		return true
	}
	for _, alias := range s.Aliases {
		if ext == alias {
			return true
		}
	}
	return false
}

// staticAnimationFormats 是 header 阶段即可断言非动画的格式
//（单帧光栅格式）。GIF 需逐帧解码、WebP 需 VP8X 嗅探，不在其列。
var staticAnimationFormats = map[string]bool{
	"png":  true,
	"jpeg": true,
	"bmp":  true,
	"tiff": true,
}

// magicSniff 根据文件头 magic 识别光栅格式；返回 nil 表示不匹配。
func magicSniff(header []byte) *FormatSpec {
	magic := func(magic []byte, name string) bool {
		return bytes.HasPrefix(header, magic) && lookupName(name)
	}
	switch {
	case magic([]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, "png"):
		return specPointer("png")
	case magic([]byte{0xFF, 0xD8, 0xFF}, "jpeg"):
		return specPointer("jpeg")
	case magic([]byte("GIF87a"), "gif"), magic([]byte("GIF89a"), "gif"):
		return specPointer("gif")
	case bytes.HasPrefix(header, []byte("RIFF")) && len(header) >= 12 && bytes.Equal(header[8:12], []byte("WEBP")):
		return specPointer("webp")
	case magic([]byte("BM"), "bmp"):
		return specPointer("bmp")
	case magic([]byte("II*\x00"), "tiff"), magic([]byte("MM\x00*"), "tiff"):
		return specPointer("tiff")
	default:
		return nil
	}
}

func lookupName(name string) bool {
	_, ok := LookupFormat(name)
	return ok
}

func specPointer(name string) *FormatSpec {
	spec, ok := LookupFormat(name)
	if !ok {
		return nil
	}
	return &spec
}
