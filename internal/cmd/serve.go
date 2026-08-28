package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"
	"imagetoolbox/internal/server"
)

var (
	serveAddr string
	serveOpen bool

	// webFS 前端静态资源（web/dist），由 main 通过 Execute 注入。
	// 只在进程启动时写入、serve 命令读取，不承载请求级状态。
	webFS fs.FS
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "启动 Image Tool Box WebUI",
	Long: `启动本地 WebUI，在浏览器中交互式使用图片处理能力。

默认只监听 127.0.0.1，请勿绑定到不可信网络。`,
	Example: `  itb serve
  itb serve --addr 127.0.0.1:9000
  itb serve --open`,
	RunE: runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().StringVar(&serveAddr, "addr", "127.0.0.1:8080", "监听地址")
	serveCmd.Flags().BoolVar(&serveOpen, "open", false, "启动后自动打开浏览器")
}

func runServe(cmd *cobra.Command, args []string) error {
	srv := &http.Server{
		Addr:    serveAddr,
		Handler: server.New(webFS).Handler(),
	}

	url := "http://" + serveAddr
	fmt.Printf("Image Tool Box WebUI 已启动: %s\n", url)
	fmt.Println("按 Ctrl+C 停止")

	if serveOpen {
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
