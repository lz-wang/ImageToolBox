package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/urfave/cli/v3"
	"imagetoolbox/internal/httpapi"
)

func newServeCommand() *cli.Command {
	return &cli.Command{
		Name:  "serve",
		Usage: "启动 HTTP API 服务",
		Description: `启动 Image Tool Box HTTP API 服务。

默认只监听 127.0.0.1，请勿绑定到不可信网络。

示例:
  itb serve
	  itb serve --addr 127.0.0.1:9000`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "addr",
				Value: "127.0.0.1:8080",
				Usage: "监听地址",
			},
			&cli.StringFlag{Name: "max-upload", Value: "64MiB", Usage: "最大 multipart 请求大小"},
			&cli.Int64Flag{Name: "max-pixels", Value: httpapi.DefaultMaxPixels, Usage: "最大图片像素数"},
			&cli.IntFlag{Name: "max-dimension", Value: httpapi.DefaultMaxDimension, Usage: "最大图片单边尺寸"},
			&cli.IntFlag{Name: "max-concurrent", Value: httpapi.DefaultMaxConcurrent, Usage: "最大并发图片操作数"},
			&cli.DurationFlag{Name: "timeout", Value: httpapi.DefaultTimeout, Usage: "单个图片操作超时"},
			&cli.BoolFlag{Name: "no-auth", Usage: "仅 loopback 本地开发时禁用认证"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return runServe(ctx, cmd)
		},
	}
}

func runServe(_ context.Context, cmd *cli.Command) error {
	addr := cmd.String("addr")
	noAuth := cmd.Bool("no-auth")
	if noAuth && !isLoopbackAddress(addr) {
		return fmt.Errorf("--no-auth 只能用于 loopback 地址")
	}
	token := os.Getenv("ITB_API_TOKEN")
	if !noAuth && token == "" {
		return fmt.Errorf("ITB_API_TOKEN is required unless --no-auth is set")
	}
	maxUpload, err := parseByteSize(cmd.String("max-upload"))
	if err != nil {
		return err
	}
	srv := &http.Server{
		Addr:    addr,
		Handler: httpapi.New(httpapi.Config{Token: token, NoAuth: noAuth, MaxUpload: maxUpload, MaxPixels: cmd.Int64("max-pixels"), MaxDimension: cmd.Int("max-dimension"), MaxConcurrent: cmd.Int("max-concurrent"), Timeout: cmd.Duration("timeout")}),
	}

	url := "http://" + addr
	fmt.Printf("Image Tool Box HTTP API 已启动: %s/api/v1/health\n", url)
	fmt.Println("按 Ctrl+C 停止")

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("启动 HTTP API 失败: %w", err)
	}
	return nil
}

func isLoopbackAddress(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
func parseByteSize(value string) (int64, error) {
	v := strings.ToUpper(strings.TrimSpace(value))
	units := map[string]int64{"KIB": 1 << 10, "MIB": 1 << 20, "GIB": 1 << 30}
	for suffix, multiplier := range units {
		if strings.HasSuffix(v, suffix) {
			n, err := strconv.ParseInt(strings.TrimSpace(strings.TrimSuffix(v, suffix)), 10, 64)
			if err != nil || n <= 0 {
				return 0, fmt.Errorf("invalid byte size: %s", value)
			}
			return n * multiplier, nil
		}
	}
	return 0, fmt.Errorf("byte size must use KiB, MiB, or GiB: %s", value)
}
