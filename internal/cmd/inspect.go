package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"

	"imagetoolbox/internal/inspect"
)

func newInspectCommand() *cli.Command {
	return &cli.Command{
		Name:  "inspect",
		Usage: "查看图片信息、元数据和文件哈希",
		Description: `检查本地图片文件的文件信息、图像基本信息、详细元数据和文件 hash。

该命令为只读操作，不会修改原始图片。

示例:
  itb inspect -i photo.jpg
  itb inspect -i photo.jpg --format json
  itb inspect -i photo.jpg --format plain
  itb inspect -i photo.jpg --no-detail
  itb inspect -i photo.jpg --no-hash`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "input",
				Aliases:  []string{"i"},
				Usage:    "输入图片 `FILE`",
				Required: true,
			},
			&cli.StringFlag{
				Name:      "format",
				Value:     "table",
				Usage:     "输出格式 `FORMAT`: table/json/plain（plain 仅输出 SHA-256）",
				Validator: enumValidator("format", "table", "json", "plain"),
			},
			&cli.BoolFlag{
				Name:  "detail",
				Value: true,
				Usage: "输出详细元数据（兼容保留，关闭请使用 --no-detail）",
			},
			&cli.BoolFlag{
				Name:  "no-detail",
				Usage: "不输出详细元数据（优先于 --detail）",
			},
			&cli.BoolFlag{
				Name:  "no-hash",
				Usage: "不计算文件 hash",
			},
			&cli.BoolFlag{
				Name:  "strict",
				Usage: "图像解析失败时直接返回错误",
			},
		},
		Action: runInspect,
	}
}

func runInspect(ctx context.Context, cmd *cli.Command) error {
	result, err := inspect.File(cmd.String("input"), inspect.Options{
		Detail: cmd.Bool("detail") && !cmd.Bool("no-detail"),
		NoHash: cmd.Bool("no-hash"),
		Strict: cmd.Bool("strict"),
	})
	if err != nil {
		return err
	}

	switch cmd.String("format") {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)

	case "plain":
		if result.Hashes == nil || result.Hashes.SHA256 == "" {
			return fmt.Errorf("plain 输出需要 sha256；请移除 --no-hash")
		}
		fmt.Fprintln(os.Stdout, result.Hashes.SHA256)
		return nil

	case "table":
		return inspect.PrintTable(os.Stdout, result)

	default:
		return fmt.Errorf("不支持的输出格式: %s（支持: table, json, plain）", cmd.String("format"))
	}
}
