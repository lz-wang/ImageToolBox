package s3

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
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

	// SkipExisting 为 true 时，对象键已存在即跳过上传（同名跳过）。
	SkipExisting bool

	// SkipUnchanged 为 true 时，仅当远端 metadata 中的 itb-sha256
	// 与本地文件 SHA-256 一致才跳过上传（内容一致跳过）。
	SkipUnchanged bool
}

// UploadResult 上传结果
type UploadResult struct {
	// Skipped 表示命中跳过规则，未执行上传
	Skipped bool `json:"skipped"`

	// Reason 跳过原因，仅 Skipped 为 true 时有值
	Reason string `json:"reason,omitempty"`
}

// Upload 上传文件到存储桶。
//
// 上传前计算本地文件 SHA-256 并随对象写入 itb-sha256 用户 metadata，
// 供后续 --skip-unchanged 比对。默认无条件覆盖已存在对象；
// SkipExisting/SkipUnchanged 只增加跳过语义，不改变默认行为。
func Upload(ctx context.Context, client *Client, inputPath string, key string, opts *UploadOptions) (*UploadResult, error) {
	if inputPath == "" {
		return nil, ErrMissingInput
	}
	if key == "" {
		return nil, ErrMissingKey
	}

	sha256Value, err := fileSHA256(inputPath)
	if err != nil {
		return nil, err
	}

	// 仅在启用跳过语义时做 1 次 HEAD preflight
	if opts != nil && (opts.SkipExisting || opts.SkipUnchanged) {
		skip, reason, err := shouldSkipUpload(ctx, client, key, sha256Value, opts)
		if err != nil {
			return nil, err
		}
		if skip {
			fmt.Printf("Upload skipped: %s -> s3://%s/%s (%s)\n", inputPath, client.bucket, key, reason)
			return &UploadResult{Skipped: true, Reason: reason}, nil
		}
	}

	// 打开本地文件
	file, err := os.Open(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open input file: %w", err)
	}
	defer file.Close()

	// 获取文件信息
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
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

	// 如果文件大于 5MB，显示进度提示
	if fileSize > 5*1024*1024 {
		fmt.Printf("Uploading %s (%.2f MB)...\n", inputPath, float64(fileSize)/(1024*1024))
	}

	// 执行上传
	_, err = client.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(client.bucket),
		Key:         aws.String(key),
		Body:        file,
		ContentType: aws.String(contentType),
		Metadata: map[string]string{
			MetadataSHA256Key: sha256Value,
		},
	})
	if err != nil {
		return nil, WrapError(err)
	}
	fmt.Printf("Upload completed: %s -> s3://%s/%s (%d bytes)\n", inputPath, client.bucket, key, fileSize)
	return &UploadResult{}, nil
}

// shouldSkipUpload 用一次 HeadObject 判断是否跳过上传。
// 对象不存在（404）时正常上传；403 等权限错误原样返回，绝不当作"不存在"。
func shouldSkipUpload(ctx context.Context, client *Client, key, localSHA256 string, opts *UploadOptions) (bool, string, error) {
	remote, err := Stat(ctx, client, key)
	if err != nil {
		if errors.Is(err, ErrObjectNotFound) {
			return false, "", nil
		}
		return false, "", err
	}
	skip, reason := decideSkip(remote, localSHA256, opts)
	return skip, reason, nil
}

// decideSkip 根据远端对象状态与跳过选项决定是否跳过上传（纯函数），
// 返回是否跳过及原因。
func decideSkip(remote *StatInfo, localSHA256 string, opts *UploadOptions) (bool, string) {
	if opts == nil || remote == nil {
		return false, ""
	}
	if opts.SkipExisting {
		return true, "object already exists"
	}
	if opts.SkipUnchanged && remote.Metadata != nil &&
		remote.Metadata[MetadataSHA256Key] == localSHA256 {
		return true, "content unchanged (itb-sha256 match)"
	}
	return false, ""
}

// fileSHA256 计算文件内容的 SHA-256，返回十六进制编码。
func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open input file: %w", err)
	}
	defer file.Close()

	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", fmt.Errorf("failed to hash input file: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
