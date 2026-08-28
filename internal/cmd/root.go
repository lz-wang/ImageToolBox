package cmd

import (
	"context"
	"io/fs"
	"os"

	"github.com/urfave/cli/v3"
)

// New 构造 itb 根命令。version 由 main 注入，staticFS 为 WebUI
// 静态资源（web/dist），仅 serve 命令使用。
func New(version string, staticFS fs.FS) *cli.Command {
	return &cli.Command{
		Name:  "itb",
		Usage: "图片处理与 S3 存储工具箱",
		// 命令清单由 urfave 根据 Commands 自动生成，
		// 禁止在 Description 中手写命令目录，避免与注册表漂移。
		Description: `Image Tool Box 提供本地图像处理、图片检查、S3 兼容存储操作和本地 WebUI。

使用 "itb <command> --help" 查看具体命令帮助。`,
		Commands: []*cli.Command{
			newCompressCommand(),
			newResizeCommand(),
			newCropCommand(),
			newConvertCommand(),
			newWatermarkCommand(),
			newInspectCommand(),
			newS3Command(),
			newServeCommand(staticFS),
			newVersionCommand(version),
		},
	}
}

// Execute 运行根命令。
func Execute(ctx context.Context, version string, staticFS fs.FS) error {
	return New(version, staticFS).Run(ctx, os.Args)
}
