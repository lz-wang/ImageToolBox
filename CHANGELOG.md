# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Removed

- WebUI 移除 S3 面板与 `/api/v1/s3/*` 接口；WebUI 收敛为图像处理功能（压缩/缩放/裁剪/转换/水印）。`itb s3` CLI 与 `ITB_S3_*` 环境变量不受影响。
- WebUI 移除 Lsky 上传接口 `/api/v1/lsky/images`、前端 `LskyPanel` 与整个 `web/src/storage/` 目录；Lsky 上传仍可通过 `itb lsky` CLI 使用。

## [v0.4.1] - 2026-08-13

### Added

- 发布工作流会在 GitHub Release 完成后，将四个平台的校验和同步为 `lz-wang/homebrew-tap` 中的 `itb` Formula；Formula 使用发布版本，且审计、安装和测试通过后才会推送。

## [v0.4.0] - 2026-07-20

### Added

- 新增 `inspect` 子命令：查看文件信息、图像元数据（宽高/格式/色彩模型）与文件 hash，支持 `table`/`json`/`plain` 三种输出格式，默认只读不解码像素。

### Changed

- **BREAKING:** 环境变量统一为 `ITB_` 前缀：`S3_*` → `ITB_S3_*`、`LSKY_*` → `ITB_LSKY_*`，旧名不再被读取。
- 文档与帮助文本不再宣传 `AWS_*` 环境变量（代码此前从未读取，属 dead config）。
- 修复 `--region` 默认值导致 `ITB_S3_REGION` 失效的问题，环境变量现在可正常覆盖区域。
- 统一根命令与所有子命令帮助文本中的命令名为 `itb`，禁用 cobra 默认 `completion` 子命令，并精简与短格式重复的描述。

## [v0.3.0] - 2026-04-22

### Added

- Added `resize` command for image scaling with percentage and explicit width/height controls.
- Added `convert` command for converting images between `jpg/jpeg/png/webp`.
- Added text and image watermark support, including tiled and positioned watermark modes.
- Added `batch` command for batch image processing across directories.
- Added S3-compatible object storage and LskyPro upload commands.

### Changed

- Switched WebP encoding from `github.com/chai2010/webp` to the pure Go `github.com/deepteams/webp` implementation.
- Restored `CGO_ENABLED=0` compatibility for build and release workflows across all target platforms.
- Expanded and refined README coverage for image processing and upload commands.

## [v0.2.0] - 2026-04-22

### Added

- Added `crop` command with anchor-based percentage cropping for `left`, `right`, `top`, `bottom`, corners, and `center`.
- Added Windows release artifacts for both `amd64` and `arm64`.
- Added Pushover notifications for CI and release workflow completion.

### Changed

- Extended build and release workflows to produce bundled binaries for macOS, Linux, and Windows in parallel.
- Updated GitHub Actions dependencies to current major versions and enabled Node.js 24 preflight mode.
- Improved Windows runner dependency setup to prefer preinstalled `cmake` and `nasm`, avoiding ARM64 Chocolatey install failures.

## [v0.1.0] - 2026-04-07

- Initial tagged release.
