package s3

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Client S3 客户端封装
type Client struct {
	client         *s3.Client
	bucket         string
	forcePathStyle bool
}

// NewClient 创建 S3 客户端
func NewClient(ctx context.Context, cfg *Config) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// 创建自定义 HTTP 客户端（不设请求总超时，见 newHTTPClient）
	httpClient := newHTTPClient()

	// 创建凭证提供者
	creds := credentials.NewStaticCredentialsProvider(
		cfg.AccessKeyID,
		cfg.SecretAccessKey,
		"", // session token，通常为空
	)

	// 加载 AWS 配置
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithCredentialsProvider(creds),
		config.WithRegion(cfg.Region),
		config.WithHTTPClient(httpClient),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// 创建 S3 客户端
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.Endpoint)
		if cfg.ForcePathStyle {
			o.UsePathStyle = true
		}
	})
	return &Client{
		client:         client,
		bucket:         cfg.Bucket,
		forcePathStyle: cfg.ForcePathStyle,
	}, nil
}

// newHTTPClient 构造 S3 专用 HTTP 客户端。
//
// 不设置 http.Client.Timeout：它覆盖整个请求生命周期（含上传/下载
// body 传输）的总时长，会把超过固定时限的大文件 GET/PUT 截断，
// 而传输耗时取决于文件大小与网络状况，没有合理的固定上限。
// 超时控制收敛在 Transport 层：
//   - ResponseHeaderTimeout 只限制"请求发出后等待响应头"的时间，
//     防止服务器无响应时永久挂起，不限制 body 传输时长；
//   - 连接、TLS 握手、keepalive、代理与连接池等配置继承
//     http.DefaultTransport 的成熟默认值。
//
// 用户主动取消由 context / 进程终止控制。
func newHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()

	transport.MaxIdleConns = 20
	transport.MaxIdleConnsPerHost = 10
	transport.IdleConnTimeout = 90 * time.Second
	transport.ResponseHeaderTimeout = 30 * time.Second

	return &http.Client{
		Transport: transport,
	}
}
