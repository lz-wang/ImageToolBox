package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v3"
	"imagetoolbox/internal/s3"
)

func newS3Command() *cli.Command {
	return &cli.Command{
		Name:     "s3",
		Usage:    "Operate S3-compatible object storage",
		Category: categoryStorage,
		Suggest:  true,
		Description: `Operate S3-compatible object storage: AWS S3, MinIO,
Alibaba Cloud OSS, Tencent Cloud COS, and similar services.

Configuration precedence:
  CLI flag > ITB_S3_* environment variable > built-in default

Environment variables can satisfy the required endpoint /
access-key / secret-key / bucket flags:
  ITB_S3_ENDPOINT           S3 endpoint URL
  ITB_S3_ACCESS_KEY_ID      Access Key ID
  ITB_S3_SECRET_ACCESS_KEY  Secret Access Key
  ITB_S3_SESSION_TOKEN      Session token for temporary credentials
  ITB_S3_REGION             Region
  ITB_S3_BUCKET             Bucket name (replaces --bucket)
  ITB_S3_FORCE_PATH_STYLE   Force path-style URLs (true/false)

Prefer environment variables for temporary credentials
(AccessKey + SecretKey + SessionToken) so session tokens
never land in shell history.`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "endpoint",
				Aliases:  []string{"e"},
				Usage:    "S3 endpoint `URL`",
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
				Usage:    "Secret Access Key (prefer the ITB_S3_SECRET_ACCESS_KEY environment variable)",
				Sources:  cli.EnvVars("ITB_S3_SECRET_ACCESS_KEY"),
				Required: true,
			},
			&cli.StringFlag{
				Name:    "session-token",
				Usage:   "Session token for temporary credentials (prefer the ITB_S3_SESSION_TOKEN environment variable)",
				Sources: cli.EnvVars("ITB_S3_SESSION_TOKEN"),
			},
			&cli.StringFlag{
				Name:    "region",
				Aliases: []string{"r"},
				Value:   "us-east-1",
				Usage:   "S3 `REGION`",
				Sources: cli.EnvVars("ITB_S3_REGION"),
			},
			&cli.StringFlag{
				Name:     "bucket",
				Aliases:  []string{"b"},
				Usage:    "Bucket name",
				Sources:  cli.EnvVars("ITB_S3_BUCKET"),
				Required: true,
			},
			&cli.BoolFlag{
				Name:    "force-path-style",
				Usage:   "Force path-style URLs (needed by MinIO). Effective default: enabled automatically for loopback endpoints and endpoints on port 9000",
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
		Name:      "upload",
		Usage:     "Upload a file to a bucket",
		ArgsUsage: "<src> [key]",
		Description: `Upload a local file to an S3-compatible bucket.

DEFAULTS:
  If [key] is omitted, the object key is basename(<src>).
  An existing object with the same key is overwritten
  unconditionally.
  The uploaded file's SHA-256 is stored in the object
  metadata (x-amz-meta-itb-sha256) for --skip-unchanged.

CONSTRAINTS:
  --skip-existing and --skip-unchanged are mutually
  exclusive.
  --skip-existing skips the upload when the object key
  already exists (one HEAD instead of a full upload).
  --skip-unchanged skips only when the stored itb-sha256
  metadata matches (no ETag dependency).
  --verify issues one HEAD after the PUT and checks that
  the remote size / Content-Type / HTTP headers / metadata
  match this upload (body bytes are not re-checked).

EXAMPLES:
  itb s3 upload -b my-bucket -e http://localhost:9000 photo.jpg
  itb s3 upload -b my-bucket photo.jpg images/photo.jpg
  itb s3 upload -b my-bucket --content-type application/json data.json
  itb s3 upload -b my-bucket image.webp image/xx.webp \
    --metadata source-sha256=abc123 --metadata width=1920
  itb s3 upload -b my-bucket --cache-control no-cache image.webp
  itb s3 upload -b my-bucket --skip-existing photo.jpg
  itb s3 upload -b my-bucket --skip-unchanged photo.jpg
  itb s3 upload -b my-bucket --verify photo.jpg`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "content-type",
				Usage:       "Content-Type `MIME` (detected from file content; the extension is only a fallback)",
				DefaultText: "auto-detect",
			},
			&cli.StringSliceFlag{
				Name:  "metadata",
				Usage: "Object user metadata `KEY=VALUE` (repeatable; keys are lowercased; itb-sha256 is reserved)",
			},
			&cli.StringFlag{
				Name:  "cache-control",
				Usage: "Cache-Control response header `VALUE` (e.g. no-cache, max-age=31536000)",
			},
			&cli.StringFlag{
				Name:  "content-disposition",
				Usage: "Content-Disposition response header `VALUE` (e.g. attachment)",
			},
			&cli.StringFlag{
				Name:  "content-encoding",
				Usage: "Content-Encoding response header `VALUE` (e.g. gzip)",
			},
			&cli.BoolFlag{
				Name:  "verify",
				Usage: "Issue one HEAD after the PUT to check that the remote size / Content-Type / HTTP headers / metadata match this upload (body bytes are not re-checked)",
			},
			&cli.StringFlag{
				Name:      "format",
				Value:     "table",
				Usage:     "Output `FORMAT`: table/json (stdout carries results only; progress goes to stderr)",
				Validator: enumValidator("format", "table", "json"),
			},
		},
		// 两个跳过选项是互斥的上传策略：同名跳过 or 内容一致跳过
		MutuallyExclusiveFlags: []cli.MutuallyExclusiveFlags{
			{
				Flags: [][]cli.Flag{
					{
						&cli.BoolFlag{
							Name:  "skip-existing",
							Usage: "Skip the upload when the object key already exists",
						},
					},
					{
						&cli.BoolFlag{
							Name:  "skip-unchanged",
							Usage: "Skip the upload only when content is unchanged (compares itb-sha256 metadata)",
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
		Name:      "download",
		Usage:     "Download an object from a bucket",
		ArgsUsage: "<key> [dst]",
		Description: `Download an object from an S3-compatible bucket to a
local file.

DEFAULTS:
  If [dst] is omitted, the file is saved to the current
  directory under the last segment of the object key.

CONSTRAINTS:
  The download writes to a temporary file in the same
  directory and renames it to the target path on success;
  any failure (network interruption, disk error, failed
  verification) leaves no partial file at the target path.
  --verify reads the object's itb-sha256 metadata and
  computes the SHA-256 in a single pass while downloading.
  --verify-sha256 checks against a known hexadecimal hash
  (provider-neutral integrity check) and can be combined
  with --verify.

EXAMPLES:
  itb s3 download -b my-bucket photo.jpg ./photo.jpg
  itb s3 download -b my-bucket images/photo.jpg
  itb s3 download -b my-bucket --verify photo.jpg
  itb s3 download -b my-bucket --verify-sha256 "$SOURCE_SHA256" \
    sha256/xxx /tmp/original.png`,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "verify",
				Usage: "Verify against the object's itb-sha256 metadata while downloading (single pass, no second local read)",
			},
			&cli.StringFlag{
				Name:  "verify-sha256",
				Usage: "Expected hexadecimal SHA-256 `HASH` (independent of object metadata; can be combined with --verify)",
			},
			&cli.StringFlag{
				Name:      "format",
				Value:     "table",
				Usage:     "Output `FORMAT`: table/json (stdout carries results only; progress goes to stderr)",
				Validator: enumValidator("format", "table", "json"),
			},
		},
		Action: runS3Download,
	}
}

func newS3DeleteCommand() *cli.Command {
	return &cli.Command{
		Name:      "delete",
		Usage:     "Delete an object from a bucket",
		ArgsUsage: "<key>",
		Description: `Delete an object from an S3-compatible bucket.

This command is destructive. It asks for confirmation by
default; pass -f to skip the confirmation.

EXAMPLES:
  itb s3 delete -b my-bucket photo.jpg
  itb s3 delete -b my-bucket -f photo.jpg`,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "force",
				Aliases: []string{"f"},
				Usage:   "Delete without confirmation",
			},
		},
		Action: runS3Delete,
	}
}

func newS3ListCommand() *cli.Command {
	return &cli.Command{
		Name:      "list",
		Usage:     "List objects in a bucket",
		ArgsUsage: "[prefix]",
		Description: `List objects in an S3-compatible bucket.

DEFAULTS:
  If [prefix] is omitted, all objects are listed.
  Only one page (up to --page-size keys) is requested.
  complete=true in JSON output means the traversal from the
  starting token finished normally; false means the listing
  is partial and --continuation-token can resume it.

CONSTRAINTS:
  --all keeps paginating until the listing is complete.
  --limit stops at N objects with complete=false and a
  next_continuation_token; the S3 request is shrunk so the
  token never skips objects.
  A page that reports more objects without a usable
  continuation token fails with E_INCOMPLETE_LIST instead of
  returning a partial result.

EXAMPLES:
  itb s3 list -b my-bucket
  itb s3 list -b my-bucket images/
  itb s3 list -b my-bucket images/ --all --format json
  itb s3 list -b my-bucket --page-size 500 --limit 5000 --format json
  itb s3 list -b my-bucket --continuation-token TOKEN --format json`,
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:      "page-size",
				Aliases:   []string{"max-keys"},
				Value:     1000,
				Usage:     "MaxKeys per ListObjectsV2 request (1-1000); --max-keys is kept as a v0.9.x alias",
				Validator: intRangeValidator("page-size", 1, 1000),
			},
			&cli.BoolFlag{
				Name:  "all",
				Usage: "Paginate until the listing is complete (default: one page only)",
			},
			&cli.IntFlag{
				Name:      "limit",
				Value:     0,
				Usage:     "Stop after `N` objects (0 = unlimited); incomplete results carry next_continuation_token",
				Validator: nonNegativeIntValidator("limit"),
			},
			&cli.StringFlag{
				Name:  "continuation-token",
				Usage: "Resume a previous listing from its continuation `TOKEN`",
			},
			&cli.StringFlag{
				Name:      "format",
				Value:     "table",
				Usage:     "Output `FORMAT`: table/json/plain (JSON contract itb.s3.list.v2)",
				Validator: enumValidator("format", "table", "json", "plain"),
			},
		},
		Action: runS3List,
	}
}

func newS3StatCommand() *cli.Command {
	return &cli.Command{
		Name:      "stat",
		Usage:     "Show object metadata without downloading the body",
		ArgsUsage: "<key>",
		Description: `Show the complete metadata of a single object.

This command issues exactly one HEAD request and never
transfers the object body. A missing object never falls
back to a list-based inference; the exact object key is
always queried.

EXAMPLES:
  itb s3 stat -b my-bucket images/photo.jpg
  itb s3 stat -b my-bucket --format json images/photo.jpg`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:      "format",
				Value:     "table",
				Usage:     "Output `FORMAT`: table/json",
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
	input, key, err := s3UploadArgs(cmd)
	if err != nil {
		return err
	}

	metadata, err := s3.ParseMetadata(cmd.StringSlice("metadata"))
	if err != nil {
		return err
	}

	client, err := newS3Client(ctx, cmd)
	if err != nil {
		return err
	}

	if key == "" {
		key = filepath.Base(input)
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
		Verify:             cmd.Bool("verify"),
	}

	result, err := s3.Upload(ctx, client, input, key, opts)
	if err != nil {
		return err
	}

	// 输出统一由 CLI 层负责：stdout 只承载正式结果（table 或 json），
	// 进度提示已由 domain 写入 opts.Progress（stderr）。
	if cmd.String("format") == "json" {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	}
	if result.Skipped {
		fmt.Printf("Upload skipped: %s -> s3://%s/%s (%s)\n", input, cmd.String("bucket"), key, result.Reason)
		return nil
	}
	fmt.Printf("Upload completed: %s -> s3://%s/%s (%d bytes)\n", input, cmd.String("bucket"), key, result.Size)
	return nil
}

func runS3Download(ctx context.Context, cmd *cli.Command) error {
	key, output, err := s3DownloadArgs(cmd)
	if err != nil {
		return err
	}

	client, err := newS3Client(ctx, cmd)
	if err != nil {
		return err
	}

	if output == "" {
		output = path.Base(key)
	}

	result, err := s3.Download(ctx, client, key, output, &s3.DownloadOptions{
		Verify:       cmd.Bool("verify"),
		VerifySHA256: cmd.String("verify-sha256"),
		Progress:     os.Stderr,
	})
	if err != nil {
		return err
	}

	if cmd.String("format") == "json" {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	}
	fmt.Printf("Download completed: %s -> %s (%d bytes)\n", result.Key, result.OutputPath, result.Size)
	return nil
}

func runS3Delete(ctx context.Context, cmd *cli.Command) error {
	key, err := s3KeyArg(cmd)
	if err != nil {
		return err
	}

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
	prefix, err := s3PrefixArg(cmd)
	if err != nil {
		return err
	}

	client, err := newS3Client(ctx, cmd)
	if err != nil {
		return err
	}

	opts := &s3.ListOptions{
		Prefix:            prefix,
		PageSize:          int32(cmd.Int("page-size")),
		Limit:             cmd.Int("limit"),
		ContinuationToken: cmd.String("continuation-token"),
		All:               cmd.Bool("all"),
	}

	result, err := s3.List(ctx, client, opts)
	if err != nil {
		return err
	}

	fmt.Print(s3.FormatOutput(result, cmd.String("format")))
	return nil
}

func runS3Stat(ctx context.Context, cmd *cli.Command) error {
	key, err := s3KeyArg(cmd)
	if err != nil {
		return err
	}

	client, err := newS3Client(ctx, cmd)
	if err != nil {
		return err
	}

	info, err := s3.Stat(ctx, client, key)
	if err != nil {
		return err
	}

	fmt.Print(s3.FormatStatOutput(info, cmd.String("format")))
	return nil
}
