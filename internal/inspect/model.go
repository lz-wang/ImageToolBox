package inspect

import "time"

// SchemaVersion 是 inspect JSON 输出的契约版本。
// v2：新增 full_decode_ok / frame_count / animation_known，
// animated 仅在 animation_known 为 true 时有意义。
const SchemaVersion = "itb.inspect.v2"

type Options struct {
	Detail bool
	NoHash bool
	Strict bool

	// FullDecode 为 true 时对文件做完整解码（GIF 用 gif.DecodeAll，
	// 其余格式用 image.Decode），捕获"文件头正常但后半部分损坏"的
	// 情况，并解析动画/帧数信息。
	FullDecode bool
}

type Result struct {
	SchemaVersion string      `json:"schema_version"`
	File          FileInfo    `json:"file"`
	Image         *ImageInfo  `json:"image,omitempty"`
	Detail        *DetailInfo `json:"detail,omitempty"`
	Hashes        *HashInfo   `json:"hashes,omitempty"`
	Warnings      []string    `json:"warnings,omitempty"`
	Error         *InfoError  `json:"error,omitempty"`
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
	SHA256 string `json:"sha256"`
	SHA1   string `json:"sha1"`
	MD5    string `json:"md5"`
	CRC32  string `json:"crc32"`
}

type InfoError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
