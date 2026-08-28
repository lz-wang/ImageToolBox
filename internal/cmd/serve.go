package cmd

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"runtime"

	"github.com/urfave/cli/v3"
	"imagetoolbox/internal/server"
)

func newServeCommand(staticFS fs.FS) *cli.Command {
	return &cli.Command{
		Name:  "serve",
		Usage: "启动本地 WebUI",
		Description: `启动本地 WebUI，在浏览器中交互式使用图片处理能力。

默认只监听 127.0.0.1，请勿绑定到不可信网络。

示例:
  itb serve
  itb serve --addr 127.0.0.1:9000
  itb serve --open`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "addr",
				Value: "127.0.0.1:8080",
				Usage: "监听地址",
			},
			&cli.BoolFlag{
				Name:  "open",
				Usage: "启动后自动打开浏览器",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return runServe(ctx, cmd, staticFS)
		},
	}
}

func runServe(_ context.Context, cmd *cli.Command, staticFS fs.FS) error {
	addr := cmd.String("addr")
	srv := &http.Server{
		Addr:    addr,
		Handler: server.New(staticFS).Handler(),
	}

	url := "http://" + addr
	fmt.Printf("Image Tool Box WebUI 已启动: %s\n", url)
	fmt.Println("按 Ctrl+C 停止")

	if cmd.Bool("open") {
		go openBrowser(url)
	}

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("启动 WebUI 失败: %w", err)
	}
	return nil
}

func openBrowser(url string) {
	var openCmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		openCmd = exec.Command("open", url)
	case "windows":
		openCmd = exec.Command("cmd", "/c", "start", url)
	default:
		openCmd = exec.Command("xdg-open", url)
	}
	if err := openCmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "无法自动打开浏览器: %v\n", err)
	}
}
