package s3

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

var (
	// ErrMissingEndpoint 端点未配置
	ErrMissingEndpoint = errors.New("endpoint is required")

	// ErrMissingCredentials 凭证未配置
	ErrMissingCredentials = errors.New("access key and secret key are required (set via flags or ITB_S3_ACCESS_KEY_ID/ITB_S3_SECRET_ACCESS_KEY env vars)")

	// ErrMissingBucket 存储桶未指定
	ErrMissingBucket = errors.New("bucket name is required")

	// ErrMissingKey 对象键未指定
	ErrMissingKey = errors.New("object key is required")

	// ErrMissingInput 输入文件未指定
	ErrMissingInput = errors.New("input file path is required")

	// ErrFileNotFound 文件未找到
	ErrFileNotFound = errors.New("file not found")

	// ErrObjectNotFound 对象未找到
	ErrObjectNotFound = errors.New("object not found in bucket")

	// ErrBucketNotFound 存储桶未找到
	ErrBucketNotFound = errors.New("bucket not found")

	// ErrAccessDenied 访问被拒绝（凭证或权限问题）。
	// 对 HeadObject 而言，403 也可能意味着无法确认对象是否存在，
	// 因此绝不把权限错误映射为"对象不存在"。
	ErrAccessDenied = errors.New("access denied")

	// ErrInvalidMetadata 用户 metadata 参数非法（缺 key=value、空 key、
	// 控制字符、重复 key 等）
	ErrInvalidMetadata = errors.New("invalid object metadata")

	// ErrReservedMetadataKey 试图占用系统保留的 metadata 键
	// （itb-sha256 由 itb 内部写入，用户不可覆盖）
	ErrReservedMetadataKey = errors.New("reserved metadata key")

	// ErrVerifyFailed 上传后 HEAD 校验未通过：远端 header/metadata
	// 与本次 PUT 的预期不一致。HEAD 只能证明 metadata/header 一致，
	// 不能证明 body 字节完整；body 校验由 download 校验承担。
	ErrVerifyFailed = errors.New("upload verification failed")

	// ErrChecksumMismatch 下载内容校验未通过：实际 SHA-256 与对象
	// metadata（--verify）或期望值（--verify-sha256）不一致。
	// 失败时本次下载的 partial 文件已被删除。
	ErrChecksumMismatch = errors.New("checksum mismatch")

	// ErrInvalidSHA256 --verify-sha256 参数不是合法的 SHA-256 digest
	//（64 个十六进制字符 / 32 字节）。参数错误必须在任何网络请求
	// 之前失败，而不是等下载完成后才变成 checksum mismatch。
	ErrInvalidSHA256 = errors.New("invalid SHA-256 digest")
)

// WrapError 包装 S3 API 错误，提供更友好的错误信息。
//
// 除 typed error 外还解析 Smithy 的 HTTP 响应错误：
// HeadObject 在对象不存在时不一定携带 NoSuchKey typed error，
// 只返回 404 状态码（无 s3:ListBucket 权限时甚至返回 403），
// 因此 404 统一映射为 ErrObjectNotFound，403 保留为权限错误。
func WrapError(err error) error {
	if err == nil {
		return nil
	}

	// 处理 NoSuchKey 错误
	var noSuchKey *types.NoSuchKey
	if errors.As(err, &noSuchKey) {
		return fmt.Errorf("%w: %s", ErrObjectNotFound, err)
	}

	// 处理 NoSuchBucket 错误
	var noSuchBucket *types.NoSuchBucket
	if errors.As(err, &noSuchBucket) {
		return fmt.Errorf("%w: %s", ErrBucketNotFound, err)
	}

	// 处理 AccessDenied 错误
	var accessDenied *types.AccessDenied
	if errors.As(err, &accessDenied) {
		return fmt.Errorf("%w: check your credentials and permissions", ErrAccessDenied)
	}

	// 按 HTTP 状态码兜底识别（HeadObject 的 404/403 不带上述 typed error）
	var responseErr *smithyhttp.ResponseError
	if errors.As(err, &responseErr) {
		switch responseErr.HTTPStatusCode() {
		case http.StatusNotFound:
			return fmt.Errorf("%w: %s", ErrObjectNotFound, err)
		case http.StatusForbidden:
			return fmt.Errorf("%w: %s", ErrAccessDenied, err)
		}
	}

	return err
}
