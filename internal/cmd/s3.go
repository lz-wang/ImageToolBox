package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v3"
	"imagetoolbox/internal/s3"
)

func newS3Command() *cli.Command {
	return &cli.Command{
		Name:    "s3",
		Usage:   "操作 S3 兼容对象存储",
		Suggest: true,
		Description: `S3 兼容存储操作，支持 AWS S3、MinIO、阿里云 OSS、腾讯云 COS 等。

配置优先级: CLI flag > ITB_S3_* 环境变量 > 默认值；
环境变量可满足 endpoint / access-key / secret-key / bucket 的必填校验。

环境变量支持:
  ITB_S3_ENDPOINT           S3 端点 URL
  ITB_S3_ACCESS_KEY_ID      Access Key ID
  ITB_S3_SECRET_ACCESS_KEY  Secret Access Key
  ITB_S3_SESSION_TOKEN      临时凭证 Session Token
  ITB_S3_REGION             区域
  ITB_S3_BUCKET             存储桶名称（可省略 --bucket）
  ITB_S3_FORCE_PATH_STYLE   强制路径样式 URL（true/false）

临时凭证建议通过环境变量注入（AccessKey + SecretKey + SessionToken），
避免 Session Token 进入 shell history。`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "endpoint",
				Aliases:  []string{"e"},
				Usage:    "S3 端点 `URL`",
				Sources:  cli.EnvVars("ITB_S3_ENDPOINT"),
				Required: true,
			},
			&cli.StringFlag{
				Name:     "access-key",
				Aliases:  []string{"a"},
				Usage:    "Access Key ID",
				Sources:  cli.EnvVars("ITB_S3_ACCESS_KEY_ID"),
				Required: true,
			},
			&cli.StringFlag{
				Name:     "secret-key",
				Aliases:  []string{"s"},
				Usage:    "Secret Access Key（建议使用 ITB_S3_SECRET_ACCESS_KEY 环境变量）",
				Sources:  cli.EnvVars("ITB_S3_SECRET_ACCESS_KEY"),
				Required: true,
			},
			&cli.StringFlag{
				Name:    "session-token",
				Usage:   "临时凭证 Session Token（建议使用 ITB_S3_SESSION_TOKEN 环境变量，避免进入 shell history）",
				Sources: cli.EnvVars("ITB_S3_SESSION_TOKEN"),
			},
			&cli.StringFlag{
				Name:    "region",
				Aliases: []string{"r"},
				Value:   "us-east-1",
				Usage:   "S3 区域 `REGION`",
				Sources: cli.EnvVars("ITB_S3_REGION"),
			},
			&cli.StringFlag{
				Name:     "bucket",
				Aliases:  []string{"b"},
				Usage:    "存储桶名称",
				Sources:  cli.EnvVars("ITB_S3_BUCKET"),
				Required: true,
			},
			&cli.BoolFlag{
				Name:    "force-path-style",
				Usage:   "强制路径样式 URL（MinIO 需要）",
				Sources: cli.EnvVars("ITB_S3_FORCE_PATH_STYLE"),
			},
		},
		Commands: []*cli.Command{
			newS3UploadCommand(),
			newS3DownloadCommand(),
			newS3DeleteCommand(),
			newS3ListCommand(),
			newS3StatCommand(),
		},
	}
}

func newS3UploadCommand() *cli.Command {
	return &cli.Command{
		Name:  "upload",
		Usage: "上传文件到存储桶",
		Description: `上传本地文件到 S3 兼容存储桶。

默认无条件覆盖同名对象。上传时会把本地文件的 SHA-256 写入对象
metadata（x-amz-meta-itb-sha256），供 --skip-unchanged 比对。
--skip-existing 与 --skip-unchanged 互斥，同时使用会报参数错误。

示例:
  # 上传文件
  itb s3 upload -i photo.jpg -b my-bucket -e http://localhost:9000

  # 指定对象键名
  itb s3 upload -i photo.jpg -b my-bucket -k images/photo.jpg

  # 指定 Content-Type
  itb s3 upload -i data.json -b my-bucket --content-type application/json

  # 写入用户 metadata（key=value，可重复；itb-sha256 为保留键）
  itb s3 upload -i image.webp -b my-bucket -k image/xx.webp \
    --metadata source-sha256=abc123 --metadata width=1920

  # 设置标准 HTTP 响应头（稳定 URL 发布）
  itb s3 upload -i image.webp -b my-bucket --cache-control no-cache

  # 同名对象已存在即跳过（1 次 HEAD 代替整文件上传）
  itb s3 upload -i photo.jpg -b my-bucket --skip-existing

  # 内容一致才跳过（比对 itb-sha256 metadata，不依赖 ETag）
  itb s3 upload -i photo.jpg -b my-bucket --skip-unchanged`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "input",
				Aliases:  []string{"i"},
				Usage:    "本地文件 `FILE`",
				Required: true,
			},
			&cli.StringFlag{
				Name:    "key",
				Aliases: []string{"k"},
				Usage:   "对象键 `KEY`（默认使用文件名）",
			},
			&cli.StringFlag{
				Name:  "content-type",
				Usage: "内容类型 `MIME`（默认按文件内容检测，扩展名仅作兜底）",
			},
			&cli.StringSliceFlag{
				Name:  "metadata",
				Usage: "对象用户 metadata `KEY=VALUE`（可重复；键转小写，itb-sha256 为保留键）",
			},
			&cli.StringFlag{
				Name:  "cache-control",
				Usage: "Cache-Control 响应头 `VALUE`（如 no-cache、max-age=31536000）",
			},
			&cli.StringFlag{
				Name:  "content-disposition",
				Usage: "Content-Disposition 响应头 `VALUE`（如 attachment）",
			},
			&cli.StringFlag{
				Name:  "content-encoding",
				Usage: "Content-Encoding 响应头 `VALUE`（如 gzip）",
			},
		},
		// 两个跳过选项是互斥的上传策略：同名跳过 or 内容一致跳过
		MutuallyExclusiveFlags: []cli.MutuallyExclusiveFlags{
			{
				Flags: [][]cli.Flag{
					{
						&cli.BoolFlag{
							Name:  "skip-existing",
							Usage: "对象键已存在即跳过上传",
						},
					},
					{
						&cli.BoolFlag{
							Name:  "skip-unchanged",
							Usage: "内容一致才跳过上传（比对 itb-sha256 metadata）",
						},
					},
				},
			},
		},
		Action: runS3Upload,
	}
}

func newS3DownloadCommand() *cli.Command {
	return &cli.Command{
		Name:  "download",
		Usage: "从存储桶下载文件",
		Description: `从 S3 兼容存储桶下载文件到本地。

示例:
  # 下载文件
  itb s3 download -b my-bucket -k photo.jpg -o ./photo.jpg

  # 使用默认文件名
  itb s3 download -b my-bucket -k images/photo.jpg`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "key",
				Aliases:  []string{"k"},
				Usage:    "对象键 `KEY`",
				Required: true,
			},
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Usage:   "本地输出 `FILE`（默认保存到当前目录，文件名取对象键最后一段）",
			},
		},
		Action: runS3Download,
	}
}

func newS3DeleteCommand() *cli.Command {
	return &cli.Command{
		Name:  "delete",
		Usage: "从存储桶删除对象",
		Description: `从 S3 兼容存储桶删除指定对象。

示例:
  # 删除对象（需要确认）
  itb s3 delete -b my-bucket -k photo.jpg

  # 强制删除（不需要确认）
  itb s3 delete -b my-bucket -k photo.jpg -f`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "key",
				Aliases:  []string{"k"},
				Usage:    "对象键 `KEY`",
				Required: true,
			},
			&cli.BoolFlag{
				Name:    "force",
				Aliases: []string{"f"},
				Usage:   "强制删除，不确认",
			},
		},
		Action: runS3Delete,
	}
}

func newS3ListCommand() *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "列出存储桶中的对象",
		Description: `列出 S3 兼容存储桶中的对象。

示例:
  # 列出所有对象
  itb s3 list -b my-bucket

  # 按前缀过滤
  itb s3 list -b my-bucket -p images/

  # JSON 格式输出
  itb s3 list -b my-bucket --format json`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "prefix",
				Aliases: []string{"p"},
				Usage:   "对象键前缀 `PREFIX`",
			},
			&cli.IntFlag{
				Name:      "max-keys",
				Value:     1000,
				Usage:     "最大返回数量",
				Validator: positiveIntValidator("max-keys"),
			},
			&cli.StringFlag{
				Name:      "format",
				Value:     "table",
				Usage:     "输出格式 `FORMAT`: table/json/plain",
				Validator: enumValidator("format", "table", "json", "plain"),
			},
		},
		Action: runS3List,
	}
}

func newS3StatCommand() *cli.Command {
	return &cli.Command{
		Name:  "stat",
		Usage: "查看对象元数据（不下载内容）",
		Description: `查询单个对象的完整元数据，只执行一次 HEAD 请求，不传输对象内容。

对象不存在时不回退到 list 推断，始终按精确对象键查询。

示例:
  # 查看对象元数据
  itb s3 stat -b my-bucket -k images/photo.jpg

  # JSON 格式输出
  itb s3 stat -b my-bucket -k images/photo.jpg --format json`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "key",
				Aliases:  []string{"k"},
				Usage:    "对象键 `KEY`",
				Required: true,
			},
			&cli.StringFlag{
				Name:      "format",
				Value:     "table",
				Usage:     "输出格式 `FORMAT`: table/json",
				Validator: enumValidator("format", "table", "json"),
			},
		},
		Action: runS3Stat,
	}
}

func newS3Client(ctx context.Context, cmd *cli.Command) (*s3.Client, error) {
	// ITB_S3_* 已由 flag 的 Sources 在 CLI 层解析，这里只做领域归一化
	cfg := &s3.Config{
		Endpoint:        cmd.String("endpoint"),
		AccessKeyID:     cmd.String("access-key"),
		SecretAccessKey: cmd.String("secret-key"),
		SessionToken:    cmd.String("session-token"),
		Region:          cmd.String("region"),
		Bucket:          cmd.String("bucket"),
		ForcePathStyle:  cmd.Bool("force-path-style"),
	}
	cfg.Normalize()

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return s3.NewClient(ctx, cfg)
}

func runS3Upload(ctx context.Context, cmd *cli.Command) error {
	client, err := newS3Client(ctx, cmd)
	if err != nil {
		return err
	}

	// 默认使用文件名作为对象键
	input := cmd.String("input")
	key := cmd.String("key")
	if key == "" {
		key = filepath.Base(input)
	}

	metadata, err := s3.ParseMetadata(cmd.StringSlice("metadata"))
	if err != nil {
		return err
	}

	opts := &s3.UploadOptions{
		ContentType:        cmd.String("content-type"),
		CacheControl:       cmd.String("cache-control"),
		ContentDisposition: cmd.String("content-disposition"),
		ContentEncoding:    cmd.String("content-encoding"),
		Metadata:           metadata,
		Progress:           os.Stderr,
		SkipExisting:       cmd.Bool("skip-existing"),
		SkipUnchanged:      cmd.Bool("skip-unchanged"),
	}

	result, err := s3.Upload(ctx, client, input, key, opts)
	if err != nil {
		return err
	}

	// 输出统一由 CLI 层负责：stdout 只承载正式结果，
	// 进度提示已由 domain 写入 opts.Progress（stderr）。
	if result.Skipped {
		fmt.Printf("Upload skipped: %s -> s3://%s/%s (%s)\n", input, cmd.String("bucket"), key, result.Reason)
		return nil
	}
	fmt.Printf("Upload completed: %s -> s3://%s/%s (%d bytes)\n", input, cmd.String("bucket"), key, result.Size)
	return nil
}

func runS3Download(ctx context.Context, cmd *cli.Command) error {
	client, err := newS3Client(ctx, cmd)
	if err != nil {
		return err
	}

	// 默认使用对象键名作为本地文件名
	key := cmd.String("key")
	output := cmd.String("output")
	if output == "" {
		output = filepath.Base(key)
	}

	result, err := s3.Download(ctx, client, key, output, &s3.DownloadOptions{Progress: os.Stderr})
	if err != nil {
		return err
	}
	fmt.Printf("Download completed: %s -> %s (%d bytes)\n", result.Key, result.OutputPath, result.Size)
	return nil
}

func runS3Delete(ctx context.Context, cmd *cli.Command) error {
	key := cmd.String("key")

	// 确认删除
	if !cmd.Bool("force") {
		fmt.Printf("确定要删除 s3://%s/%s 吗？(y/N): ", cmd.String("bucket"), key)
		var confirm string
		fmt.Scanln(&confirm)
		if strings.ToLower(confirm) != "y" {
			fmt.Println("已取消")
			return nil
		}
	}

	client, err := newS3Client(ctx, cmd)
	if err != nil {
		return err
	}

	if err := s3.Delete(ctx, client, key, nil); err != nil {
		return err
	}
	fmt.Printf("Delete completed: s3://%s/%s\n", cmd.String("bucket"), key)
	return nil
}

func runS3List(ctx context.Context, cmd *cli.Command) error {
	client, err := newS3Client(ctx, cmd)
	if err != nil {
		return err
	}

	opts := &s3.ListOptions{
		Prefix:  cmd.String("prefix"),
		MaxKeys: int32(cmd.Int("max-keys")),
	}

	objects, err := s3.List(ctx, client, opts)
	if err != nil {
		return err
	}

	fmt.Print(s3.FormatOutput(objects, cmd.String("format")))
	return nil
}

func runS3Stat(ctx context.Context, cmd *cli.Command) error {
	client, err := newS3Client(ctx, cmd)
	if err != nil {
		return err
	}

	info, err := s3.Stat(ctx, client, cmd.String("key"))
	if err != nil {
		return err
	}

	fmt.Print(s3.FormatStatOutput(info, cmd.String("format")))
	return nil
}
