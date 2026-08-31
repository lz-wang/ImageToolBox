package s3

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// MetadataSHA256Key 上传时写入对象用户 metadata 的内容 SHA-256 键名。
// --skip-unchanged 依赖该值判断远端对象与本地是否一致；
// 不使用 ETag 做该判断（multipart 上传、SSE 与部分 S3 兼容实现下
// ETag 不是可靠的内容哈希）。
const MetadataSHA256Key = "itb-sha256"

// contentTypes 内容类型映射
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

// UploadOptions 上传选项
type UploadOptions struct {
	ContentType string

	// CacheControl / ContentDisposition / ContentEncoding 写入对象的
	// 标准 HTTP 响应头（如 Cache-Control: no-cache），留空则不设置。
	CacheControl       string
	ContentDisposition string
	ContentEncoding    string

	// Metadata 写入对象的自定义用户 metadata（x-amz-meta-*）。
	// 键经 NormalizeMetadata 归一化；itb-sha256 为保留键。
	Metadata map[string]string

	// Progress 接收大文件（>5MB）传输提示等进度信息，nil 表示不输出。
	// 进度信息不是执行结果，domain 不直接写 stdout，由 adapter 决定
	// 输出去向（CLI 传 os.Stderr，保持 stdout 只承载正式结果）。
	Progress io.Writer

	// SkipExisting 为 true 时，对象键已存在即跳过上传（同名跳过）。
	SkipExisting bool

	// SkipUnchanged 为 true 时，仅当远端 metadata 中的 itb-sha256
	// 与本地文件 SHA-256 一致才跳过上传（内容一致跳过）。
	SkipUnchanged bool
}

// UploadResult 上传结果
type UploadResult struct {
	// Key 实际写入的对象键
	Key string `json:"key"`

	// Size 上传对象的字节数
	Size int64 `json:"size"`

	// SHA256 本地文件内容 SHA-256，同时写入 itb-sha256 metadata
	SHA256 string `json:"sha256"`

	// Skipped 表示命中跳过规则，未执行上传
	Skipped bool `json:"skipped"`

	// Reason 跳过原因，仅 Skipped 为 true 时有值
	Reason string `json:"reason,omitempty"`
}

// Upload 上传文件到存储桶。
//
// 执行顺序：open → HEAD preflight（仅启用跳过语义时）→ SHA-256 →
// Seek(0) → PUT，整个函数只打开一次文件。HEAD 必须先于 hash：
// --skip-existing 命中时在 hash 之前直接返回，0 字节本地内容读取；
// --skip-unchanged 复用同一次 HEAD 结果，单次上传最多
// 1 × HEAD + 1 × PUT。
//
// 上传时把本地文件 SHA-256 写入 itb-sha256 用户 metadata，供后续
// --skip-unchanged 比对。默认无条件覆盖已存在对象；
// SkipExisting/SkipUnchanged 只增加跳过语义，不改变默认行为。
//
// 本函数不输出任何内容：结果通过 UploadResult 返回，进度提示写入
// opts.Progress，由 adapter（CLI/脚本）决定如何呈现。
func Upload(ctx context.Context, client *Client, inputPath string, key string, opts *UploadOptions) (*UploadResult, error) {
	if inputPath == "" {
		return nil, ErrMissingInput
	}
	if key == "" {
		return nil, ErrMissingKey
	}

	// 用户 metadata 在任何网络请求之前完成归一化校验，
	// 非法参数不产生副作用。
	var metadata map[string]string
	if opts != nil {
		normalized, err := NormalizeMetadata(opts.Metadata)
		if err != nil {
			return nil, err
		}
		metadata = normalized
	}

	// 打开文件但不读取内容，输入文件不存在时立即报错
	file, err := os.Open(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open input file: %w", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	// HEAD preflight 必须发生在 hash 之前
	var remote *StatInfo

	if opts != nil && (opts.SkipExisting || opts.SkipUnchanged) {
		remote, err = statUploadTarget(ctx, client, key)
		if err != nil {
			return nil, err
		}

		if remote != nil && opts.SkipExisting {
			return &UploadResult{Key: key, Skipped: true, Reason: "object already exists"}, nil
		}
	}

	sha256Value, err := readerSHA256(file)
	if err != nil {
		return nil, err
	}

	if opts != nil && opts.SkipUnchanged && isUnchanged(remote, sha256Value) {
		return &UploadResult{Key: key, Skipped: true, Reason: "content unchanged (itb-sha256 match)"}, nil
	}

	// hash 已消费文件内容，回卷到起点后再交给 PutObject
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to rewind input file: %w", err)
	}

	// 自动检测 Content type
	contentType := "application/octet-stream"
	if opts != nil && opts.ContentType != "" {
		contentType = opts.ContentType
	} else {
		ext := filepath.Ext(inputPath)
		if ct, ok := contentTypes[ext]; ok {
			contentType = ct
		}
	}

	// 获取文件大小
	fileSize := fileInfo.Size()

	// 如果文件大于 5MB，向 Progress 输出传输提示
	var progress io.Writer
	if opts != nil {
		progress = opts.Progress
	}
	if fileSize > 5*1024*1024 && progress != nil {
		fmt.Fprintf(progress, "Uploading %s (%.2f MB)...\n", inputPath, float64(fileSize)/(1024*1024))
	}

	// 执行上传：itb-sha256 与用户 metadata 合并写入，用户不可覆盖
	// 保留键（NormalizeMetadata 已拒绝）。
	objectMetadata := make(map[string]string, len(metadata)+1)
	maps.Copy(objectMetadata, metadata)
	objectMetadata[MetadataSHA256Key] = sha256Value

	putInput := &s3.PutObjectInput{
		Bucket:      aws.String(client.bucket),
		Key:         aws.String(key),
		Body:        file,
		ContentType: aws.String(contentType),
		Metadata:    objectMetadata,
	}
	if opts != nil {
		if opts.CacheControl != "" {
			putInput.CacheControl = aws.String(opts.CacheControl)
		}
		if opts.ContentDisposition != "" {
			putInput.ContentDisposition = aws.String(opts.ContentDisposition)
		}
		if opts.ContentEncoding != "" {
			putInput.ContentEncoding = aws.String(opts.ContentEncoding)
		}
	}

	_, err = client.client.PutObject(ctx, putInput)
	if err != nil {
		return nil, WrapError(err)
	}
	return &UploadResult{Key: key, Size: fileSize, SHA256: sha256Value}, nil
}

// statUploadTarget 对上传目标执行 1 次 HeadObject preflight。
// 对象不存在（404）返回 nil，由调用方继续上传；
// 403 等权限错误原样返回，绝不当作"不存在"。
func statUploadTarget(ctx context.Context, client *Client, key string) (*StatInfo, error) {
	info, err := Stat(ctx, client, key)
	if err != nil {
		if errors.Is(err, ErrObjectNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return info, nil
}

// isUnchanged 判断远端对象的 itb-sha256 metadata 与本地哈希是否一致。
func isUnchanged(remote *StatInfo, localSHA256 string) bool {
	return remote != nil &&
		remote.Metadata != nil &&
		remote.Metadata[MetadataSHA256Key] == localSHA256
}

// readerSHA256 计算读取内容的 SHA-256，返回十六进制编码。
func readerSHA256(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", fmt.Errorf("failed to hash input file: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
