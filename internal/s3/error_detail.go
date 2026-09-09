package s3

import (
	"errors"
	"net"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// ErrorDetail 是可安全透出的 provider 错误摘要：只有 HTTP 状态码、
// provider 错误码与限流标记。绝不超过这一边界——Authorization、
// SecretAccessKey、SessionToken、signed URL 等（即便出现在 provider
// 响应里）也绝不进入机器可读输出。
type ErrorDetail struct {
	HTTPStatus   *int
	ProviderCode *string
	Retryable    bool
}

// throttledProviderCodes 是常见限流/过载 provider 错误码的并集
//（AWS S3 与主流 S3 兼容实现）。命中即标记 retryable。
var throttledProviderCodes = map[string]bool{
	"ThrottlingException":                 true,
	"Throttling":                          true,
	"ThrottledException":                  true,
	"SlowDown":                            true,
	"RequestTimeout":                      true,
	"RequestTimeoutException":             true,
	"ProvisionedThroughputExceededException": true,
	"BandwidthLimitExceeded":              true,
	"RequestLimitExceeded":                true,
	"RequestThrottled":                    true,
	"RequestThrottledException":           true,
	"TransactionInProgressException":      true,
	"InternalError":                       true,
	"InternalServerException":             true,
	"ServiceUnavailable":                  true,
}

// DetailFromError 提取错误的 provider 摘要；found 为 false 表示错误
// 不含任何 S3 provider 信息（本地 IO、参数等）。
//
// 提取的信息仅限：
//   - HTTP 状态码（smithyhttp.ResponseError）；
//   - provider 错误码（smithy.APIError / typed S3 错误）；
//   - 限流标记（限流错误码集合，或底层连接超时）。
func DetailFromError(err error) (detail ErrorDetail, found bool) {
	if err == nil {
		return detail, false
	}

	var respErr *smithyhttp.ResponseError
	if errors.As(err, &respErr) {
		status := respErr.HTTPStatusCode()
		detail.HTTPStatus = &status
		found = true
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		code := apiErr.ErrorCode()
		detail.ProviderCode = &code
		found = true
	}

	// typed S3 错误可能不带 ResponseError 包装（如解析出的 NoSuchKey）
	var noSuchKey *types.NoSuchKey
	if errors.As(err, &noSuchKey) {
		code := "NoSuchKey"
		detail.ProviderCode = &code
		found = true
	}
	var noSuchBucket *types.NoSuchBucket
	if errors.As(err, &noSuchBucket) {
		code := "NoSuchBucket"
		detail.ProviderCode = &code
		found = true
	}

	if found {
		detail.Retryable = providerErrorRetryable(err, detail)
	}
	return detail, found
}

// providerErrorRetryable 判断 provider 错误是否值得重试：
// 限流错误码、HTTP 5xx、底层网络超时。
func providerErrorRetryable(err error, detail ErrorDetail) bool {
	if detail.ProviderCode != nil && throttledProviderCodes[*detail.ProviderCode] {
		return true
	}
	if detail.HTTPStatus != nil && *detail.HTTPStatus >= http.StatusInternalServerError {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}
	return false
}
