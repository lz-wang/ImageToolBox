package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"

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
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return runServe(ctx, cmd)
		},
	}
}

func runServe(_ context.Context, cmd *cli.Command) error {
	addr := cmd.String("addr")
	srv := &http.Server{
		Addr:    addr,
		Handler: httpapi.New(httpapi.Config{}),
	}

	url := "http://" + addr
	fmt.Printf("Image Tool Box HTTP API 已启动: %s/api/v1/health\n", url)
	fmt.Println("按 Ctrl+C 停止")

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("启动 HTTP API 失败: %w", err)
	}
	return nil
}
