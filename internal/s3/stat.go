package s3

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

// StatSchemaVersion 是 stat --format json 的机器可读契约版本。
const StatSchemaVersion = "itb.s3.stat.v1"

// StatInfo 单个对象的完整元数据（HeadObject），不包含对象 body。
type StatInfo struct {
	// SchemaVersion 机器可读契约版本（itb.s3.stat.v1），由 Stat 填充
	SchemaVersion string `json:"schema_version"`

	Key                string            `json:"key"`
	Size               int64             `json:"size"`
	LastModified       time.Time         `json:"last_modified"`
	ETag               string            `json:"etag"`
	ContentType        string            `json:"content_type,omitempty"`
	CacheControl       string            `json:"cache_control,omitempty"`
	ContentDisposition string            `json:"content_disposition,omitempty"`
	ContentEncoding    string            `json:"content_encoding,omitempty"`
	StorageClass       string            `json:"storage_class,omitempty"`
	VersionID          string            `json:"version_id,omitempty"`
	Metadata           map[string]string `json:"metadata,omitempty"`
}

// Stat 精确查询单个对象的元数据：1 × HEAD 请求，0 × 对象 body，
// 响应规模只取决于 metadata/header，与对象大小无关。
// 语义等同 `mc stat --no-list`：仅按对象键执行 HeadObject，
// 404 时绝不回退 ListObjectsV2 推断 prefix/目录。
func Stat(ctx context.Context, client *Client, key string) (*StatInfo, error) {
	if key == "" {
		return nil, ErrMissingKey
	}

	out, err := client.client.HeadObject(ctx, &awss3.HeadObjectInput{
		Bucket: aws.String(client.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, WrapError(err)
	}

	info := &StatInfo{
		SchemaVersion:      StatSchemaVersion,
		Key:                key,
		ETag:               aws.ToString(out.ETag),
		ContentType:        aws.ToString(out.ContentType),
		CacheControl:       aws.ToString(out.CacheControl),
		ContentDisposition: aws.ToString(out.ContentDisposition),
		ContentEncoding:    aws.ToString(out.ContentEncoding),
		VersionID:          aws.ToString(out.VersionId),
		Metadata:           out.Metadata,
	}
	if out.ContentLength != nil {
		info.Size = *out.ContentLength
	}
	if out.LastModified != nil {
		info.LastModified = *out.LastModified
	}
	if out.StorageClass != "" {
		info.StorageClass = string(out.StorageClass)
	}
	return info, nil
}

// Exists 基于 Stat 判断对象是否存在。
// 仅 404 视为不存在；403 等权限错误原样返回，绝不误判为"不存在"
// （无 s3:ListBucket 权限时 S3 对不存在的对象也返回 403）。
func Exists(ctx context.Context, client *Client, key string) (bool, error) {
	_, err := Stat(ctx, client, key)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, ErrObjectNotFound) {
		return false, nil
	}
	return false, err
}

// FormatStatOutput 格式化 stat 输出，与 list 的 table/json 风格保持一致。
func FormatStatOutput(info *StatInfo, format string) string {
	switch format {
	case "json":
		return formatStatJSON(info)
	case "table":
		fallthrough
	default:
		return formatStatTable(info)
	}
}

func formatStatTable(info *StatInfo) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Key           %s\n", info.Key))
	sb.WriteString(fmt.Sprintf("Size          %s\n", formatBytes(info.Size)))
	sb.WriteString(fmt.Sprintf("Last Modified %s\n", info.LastModified.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("ETag          %s\n", info.ETag))
	if info.ContentType != "" {
		sb.WriteString(fmt.Sprintf("Content-Type  %s\n", info.ContentType))
	}
	if info.StorageClass != "" {
		sb.WriteString(fmt.Sprintf("Storage Class %s\n", info.StorageClass))
	}
	if len(info.Metadata) > 0 {
		// 排序 key 保证输出稳定
		keys := make([]string, 0, len(info.Metadata))
		for k := range info.Metadata {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sb.WriteString(fmt.Sprintf("Metadata[%s]  %s\n", k, info.Metadata[k]))
		}
	}
	return sb.String()
}

func formatStatJSON(info *StatInfo) string {
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Sprintf("failed to marshal stat info: %v", err)
	}
	return string(data)
}
