package s3

import (
	"fmt"
	"strings"
)

// Config S3 客户端配置
type Config struct {
	Endpoint        string // S3 端点 URL
	AccessKeyID     string // Access Key ID
	SecretAccessKey string // Secret Access Key
	Region          string // 区域
	Bucket          string // 存储桶名称
	ForcePathStyle  bool   // 是否强制路径样式（MinIO 需要）
}

// Normalize 归一化配置：补全 region 默认值，并按端点自动启用路径样式。
//
// 环境变量不在领域层读取：ITB_S3_* 由 CLI 层（urfave/cli 的
// Sources）解析注入，优先级为 CLI flag > 环境变量 > 默认值。
func (c *Config) Normalize() {
	if c.Region == "" {
		c.Region = "us-east-1"
	}

	// 自动检测 MinIO，默认启用路径样式
	if c.Endpoint != "" && !c.ForcePathStyle {
		// MinIO 通常使用 localhost 或内网地址
		if strings.Contains(c.Endpoint, "localhost") ||
			strings.Contains(c.Endpoint, "127.0.0.1") ||
			strings.Contains(c.Endpoint, ":9000") {
			c.ForcePathStyle = true
		}
	}
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.Endpoint == "" {
		return ErrMissingEndpoint
	}
	if c.AccessKeyID == "" || c.SecretAccessKey == "" {
		return ErrMissingCredentials
	}
	if c.Bucket == "" {
		return ErrMissingBucket
	}
	return nil
}

// ValidateWithoutBucket 验证配置（不验证 bucket，用于 list buckets 等操作）
func (c *Config) ValidateWithoutBucket() error {
	if c.Endpoint == "" {
		return ErrMissingEndpoint
	}
	if c.AccessKeyID == "" || c.SecretAccessKey == "" {
		return ErrMissingCredentials
	}
	return nil
}

// String 返回配置的安全字符串表示（隐藏敏感信息）
func (c *Config) String() string {
	secret := ""
	if c.SecretAccessKey != "" {
		if len(c.SecretAccessKey) > 4 {
			secret = c.SecretAccessKey[:4] + "****"
		} else {
			secret = "****"
		}
	}
	return fmt.Sprintf("Config{Endpoint: %s, AccessKeyID: %s, SecretAccessKey: %s, Region: %s, Bucket: %s, ForcePathStyle: %v}",
		c.Endpoint, c.AccessKeyID, secret, c.Region, c.Bucket, c.ForcePathStyle)
}
