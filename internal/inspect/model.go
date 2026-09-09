package inspect

import (
	"time"

	"imagetoolbox/internal/filehash"
)

// SchemaVersion 是 inspect JSON 输出的契约版本。
// v2：新增 full_decode_ok / frame_count / animation_known。
// v3：新增 content 内容识别对象（三阶段：内容识别 → 结构校验 →
// 可选完整解码），支持 BMP/TIFF/SVG 识别。
const SchemaVersion = "itb.inspect.v3"

type Options struct {
	Detail bool
	NoHash bool
	Strict bool

	// FullDecode 为 true 时对文件做完整解码（GIF 用 gif.DecodeAll，
	// 其余格式用 image.Decode），捕获"文件头正常但后半部分损坏"的
	// 情况，并解析动画/帧数信息。
	FullDecode bool

	// Hashes 是选择性计算的哈希算法集合；nil 且 NoHash=false 表示
	// 全部计算（历史行为）。NoHash=true 时忽略本字段。
	Hashes []filehash.Algorithm
}

type Result struct {
	SchemaVersion string      `json:"schema_version"`
	File          FileInfo    `json:"file"`
	Content       ContentInfo `json:"content"`
	Image         *ImageInfo  `json:"image,omitempty"`
	Detail        *DetailInfo `json:"detail,omitempty"`
	Hashes        *HashInfo   `json:"hashes,omitempty"`
	Warnings      []string    `json:"warnings,omitempty"`
	Error         *InfoError  `json:"error,omitempty"`
}

// ContentInfo 是内容识别层（v3 新增）的结论：格式从文件内容识别，
// 而不是从扩展名推断。
type ContentInfo struct {
	// Format 是规范格式名；未识别时为空串
	Format string `json:"format,omitempty"`

	// CanonicalExtension 是该格式的规范扩展名；未识别时为空串
	CanonicalExtension string `json:"canonical_extension,omitempty"`

	// MIMEType 是该格式的权威 MIME 类型；未识别时为空串
	MIMEType string `json:"mime_type,omitempty"`

	// Recognized 表示文件内容被识别为受支持格式（含 SVG 这类
	// 不支持 raster 解码的格式）
	Recognized bool `json:"recognized"`

	// DecodeSupported 表示是否存在注册的 raster decoder。
	// SVG 为 false：不支持 raster 解码不是"图片损坏"。
	DecodeSupported bool `json:"decode_supported"`

	// FullDecodeSupported 表示是否支持 --full-decode。
	FullDecodeSupported bool `json:"full_decode_supported"`

	// ExtensionMatches 报告文件扩展名是否与识别出的格式一致
	ExtensionMatches bool `json:"extension_matches"`
}

type FileInfo struct {
	Path       string    `json:"path"`
	AbsPath    string    `json:"abs_path,omitempty"`
	Name       string    `json:"name"`
	Ext        string    `json:"ext"`
	SizeBytes  int64     `json:"size_bytes"`
	Mode       string    `json:"mode"`
	ModifiedAt time.Time `json:"modified_at"`
	MIMEType   string    `json:"mime_type,omitempty"`
	MagicHex   string    `json:"magic_hex,omitempty"`
}

type ImageInfo struct {
	Format         string  `json:"format"`
	Width          int     `json:"width"`
	Height         int     `json:"height"`
	AspectRatio    string  `json:"aspect_ratio"`
	Megapixels     float64 `json:"megapixels"`
	ColorModel     string  `json:"color_model,omitempty"`
	HasAlpha       bool    `json:"has_alpha"`

	// DecodeConfigOK 表示 header 解码（image.DecodeConfig）成功，
	// 是 v1 就存在的字段。
	DecodeConfigOK bool `json:"decode_config_ok"`

	// FullDecodeOK 三态：nil 表示未尝试（--full-decode 未开启），
	// 非 nil 表示完整解码结果（true = 通过，false = 文件后半部分
	// 损坏等完整解码失败）。指针 + omitempty 区分"未尝试"与"失败"。
	FullDecodeOK *bool `json:"full_decode_ok,omitempty"`

	// FrameCount 是完整解码得到的帧数；0 表示未知或非动画格式
	//（omitempty 省略）。仅 GIF 支持逐帧计数。
	FrameCount int `json:"frame_count,omitempty"`

	// AnimationKnown 表示 animated 字段是否可信：
	//   - GIF（DecodeAll）与 WebP（VP8X 头嗅探）→ true
	//   - JPEG/PNG 等静态格式 → true（animated 恒为 false）
	//   - 仅 DecodeConfig、未开启 --full-decode 的 GIF → false
	//     （此时 animated=false 是"未知"，不是"断言非动画"）
	AnimationKnown bool `json:"animation_known"`
	Animated       bool `json:"animated"`
}

type DetailInfo struct {
	MagicBytes             string `json:"magic_bytes,omitempty"`
	HeaderBytes            string `json:"header_bytes,omitempty"`
	DetectedBy             string `json:"detected_by,omitempty"`
	ExtensionMatchesFormat bool   `json:"extension_matches_format"`
}

type HashInfo struct {
	// 选择性计算（--hash）时未选中的算法省略；默认全量计算的输出
	// 形状与 v2 完全一致
	SHA256 string `json:"sha256,omitempty"`
	SHA1   string `json:"sha1,omitempty"`
	MD5    string `json:"md5,omitempty"`
	CRC32  string `json:"crc32,omitempty"`
}

type InfoError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
