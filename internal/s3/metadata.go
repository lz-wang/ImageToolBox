package s3

import (
	"fmt"
	"strings"
	"unicode"
)

// ParseMetadata 解析 CLI --metadata 的 key=value 项为归一化 map。
// 校验规则与 NormalizeMetadata 一致：key 非空、key/value 禁含控制
// 字符、重复 key 报错、itb-sha256 为系统保留键。空入参返回 nil。
func ParseMetadata(entries []string) (map[string]string, error) {
	if len(entries) == 0 {
		return nil, nil
	}

	provisional := make(map[string]string, len(entries))
	for _, entry := range entries {
		key, value, found := strings.Cut(entry, "=")
		if !found {
			return nil, fmt.Errorf("%w: must be key=value, got %q", ErrInvalidMetadata, entry)
		}
		key = strings.TrimSpace(key)
		if _, exists := provisional[key]; exists {
			return nil, fmt.Errorf("%w: duplicate key %q", ErrInvalidMetadata, key)
		}
		provisional[key] = value
	}

	return NormalizeMetadata(provisional)
}

// NormalizeMetadata 归一化调用方（CLI/脚本）构造的 metadata map：
// key 小写化并去除首尾空白、key 非空、key/value 禁含控制字符、
// 小写化后重复 key 报错；itb-sha256 是内部完整性键，用户不可占用。
// 返回新 map，不修改入参。
func NormalizeMetadata(metadata map[string]string) (map[string]string, error) {
	if len(metadata) == 0 {
		return nil, nil
	}

	normalized := make(map[string]string, len(metadata))
	for key, value := range metadata {
		lower := strings.ToLower(strings.TrimSpace(key))
		if lower == "" {
			return nil, fmt.Errorf("%w: empty key", ErrInvalidMetadata)
		}
		if lower == MetadataSHA256Key {
			return nil, fmt.Errorf("%w: %s is reserved for the object checksum", ErrReservedMetadataKey, lower)
		}
		if hasControlChars(lower) || hasControlChars(value) {
			return nil, fmt.Errorf("%w: control characters are not allowed in key %q", ErrInvalidMetadata, lower)
		}
		if _, exists := normalized[lower]; exists {
			return nil, fmt.Errorf("%w: duplicate key %q", ErrInvalidMetadata, lower)
		}
		normalized[lower] = value
	}
	return normalized, nil
}

// hasControlChars 报告字符串是否含控制字符（ASCII 0x00-0x1F、0x7F
// 及其他 Unicode 控制类字符）。S3 用户 metadata 经由 HTTP header
// 传输，控制字符会导致请求被服务端拒绝。
func hasControlChars(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7F || unicode.IsControl(r) {
			return true
		}
	}
	return false
}
