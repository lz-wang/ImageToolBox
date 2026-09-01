# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Fixed

- Image transform commands reject destinations that resolve to the source file, including equivalent paths, hard links, and symbolic links, preventing destructive partial overwrites. `compress --in-place` continues to use its temporary-file + rename path.

### Changed

- **BREAKING:** local image commands now use positional paths: `itb <command> [options] <src> [dst]`; `convert` requires `<dst>` and determines its format from the destination extension. Removed local image-command `-i`/`--input`, `-o`/`--output`, and `convert --to`; S3 command flags are unchanged.

## [v0.7.0] - 2026-09-01

### Added

- `itb s3 upload` 新增对象元数据与 HTTP 头：`--metadata key=value`（可重复）、`--cache-control`、`--content-disposition`、`--content-encoding`，随 PUT 写入对象；非法键值与系统保留键 `itb-sha256` 在任何网络请求之前拒绝。
- `itb s3 upload --verify`：PUT 成功后追加一次 HEAD，比对 Content-Length、Content-Type、Cache-Control、`itb-sha256` 与全部用户 metadata，不一致时明确指出失配字段。
- `itb s3 download --verify` / `--verify-sha256 <hex>`：边下载边计算 SHA-256，两种校验可同用；内容先写同目录临时文件，成功后 rename，任何失败路径都不留 partial 文件。
- `itb s3` 支持会话凭证：`--session-token` / `ITB_S3_SESSION_TOKEN`（以 `X-Amz-Security-Token` 参与签名）；`--force-path-style` 新增 `ITB_S3_FORCE_PATH_STYLE` 环境变量。
- `itb s3 stat`/`upload`/`download` 的 JSON 输出携带 `schema_version`；`upload`/`download` 新增 `--format table|json`（默认 table 不变）。stdout 只承载正式结果，进度与诊断走 stderr。
- `itb inspect --full-decode`（HTTP `full-decode`）：GIF 逐帧、其余格式整图解码，捕获"文件头正常但后半部分损坏"；schema 升级 `itb.inspect.v2`，新增 `full_decode_ok`、`frame_count`、`animation_known`、`animated`。
- `itb serve --max-working-bytes`（默认 512MiB）：对水印等操作的中间工作集执行内存准入，超限在分配前返回 413。
- MinIO 集成测试：CI 以 service 方式启动真实 MinIO 并强制执行（`ITB_REQUIRE_MINIO=1`），本地默认优雅跳过，可用 `ITB_TEST_MINIO_*` 指向自建实例。

### Changed

- S3 Content-Type 按文件内容检测：显式 `--content-type` > magic sniff（覆盖 JPEG/PNG/GIF/WebP/PDF/ZIP/HTML/JSON/SVG）> 扩展名兜底 > `application/octet-stream`；HTML 错误页改名 `.jpg` 后不再伪装 `image/jpeg` 上传。
- convert/resize/crop/watermark 统一解码入口 `imageio.OpenStatic`：输入严格限定 JPEG/PNG/WebP，GIF/BMP/TIFF 一律拒绝，animated GIF 不再被静默处理首帧（水印 logo 输入同样受限）。
- EXIF Orientation 统一语义：`imageio.Info` 区分物理尺寸与逻辑尺寸，Probe 返回应用旋转后的逻辑尺寸并与解码后 bounds 恒等；所有静态 transform 在解码阶段烘焙方向，竖拍 JPEG 不再横躺。
- resize `--percent` 修正为按百分比精确缩放（此前 `--percent 200` 在默认 fit 模式下输出仍为原尺寸）；输出规划 `Resolve` 与实际执行 `Apply` 恒等，fit 包围盒超限但真实输出在限内的请求不再被保守拒绝。
- WebP 编码固定保留 Alpha；lossless WebP 开启 `Exact`，透明像素下 RGB 完整保留；是否铺底由目标格式唯一决定，调用方无法再让 lossy WebP 静默丢失透明度。
- S3 领域结果与 CLI 输出分离：upload/download 返回结构化结果，进度提示经 stderr 输出，stdout 由 CLI 统一渲染。
- HTTP API 错误状态映射改为 typed errors 分派：超时 504、尺寸超限 413、不支持的格式 415；handler panic 收敛为 JSON 500；`internal/httpapi` 按职责拆分为 handler/middleware/multipart/operations/response。

### Fixed

- multipart 上传文件存储路径隔离：客户端 filename 只作为元数据，服务端存储路径一律由 `os.CreateTemp` 生成，`input=image=logo.png` 同名碰撞与输入输出互相覆盖不再发生。
- 数值校验拒绝 NaN/Inf：`opacity=NaN`、`scale=Inf`、`percent=NaN%` 此前可绕过范围检查；multipart 同名字段不论标量还是文件一律 400；API token 强制 `Bearer ` 前缀，裸 token 即使值正确也拒绝。
- 水印资源边界升级为操作工作集准入：文字 repeat 画布、平铺画布与旋转包围盒纳入保守内存估算，`scale=1000000`、`font-size=4096` 之类参数在分配前返回 413。
- JPEG background 强制不透明：`#00000000` 解析出的透明值不再静默变成默认白色，半透明色统一显式拒绝。
- `--verify-sha256` 严格校验 64 个十六进制字符，短串在任何网络请求之前失败。
- `--skip-existing` 命中时填充本地文件 size；`--skip-unchanged` 命中时填充 size 与 sha256，脚本消费方无需二次 stat。
- JPEG orientation 解析统一为单一 parser：XMP APP1 前置于 EXIF APP1 的文件不再出现 Probe 与解码方向漂移；非 EXIF APP1 的伪造 TIFF 头被拒绝。

## [v0.6.0] - 2026-08-31

### Changed

- HTTP API 由 Gin 迁移至 Go 标准库 `net/http`，并改用与 CLI long flag 一致的 multipart 字段。
- 图片操作的默认值和最终校验收敛至领域包；水印改为统一文件级领域入口。
- Linux 内嵌压缩工具的构建与 CI 门禁固定为 glibc 2.28 兼容契约；修正 Windows 内嵌二进制缓存命中。

### Added

- `ITB_API_TOKEN` Bearer 认证、上传/图片/并发/超时限制与优雅关闭。
- 结构化 API 错误、`slog` 访问日志与流式图片响应。

### Removed

- **BREAKING:** 移除内置 WebUI、React/MUI/Vite 前端及 `itb serve --open`；`itb serve` 现为纯 HTTP API 服务。
- **BREAKING:** 移除 Gin 和旧版 `file + options JSON` HTTP 参数契约。

## [v0.5.0] - 2026-08-30

### Added

- 新增 `itb serve` WebUI：React 19 + TypeScript + MUI 前端经 `go:embed` 内嵌进二进制，默认绑定 `127.0.0.1:8080`，支持 `--addr` 与 `--open`；`/api/v1` 提供压缩、缩放、裁剪、转换、文字/图片水印等单图处理接口，前端支持 Before/After 对比、以鼠标为中心的滚轮缩放、拖拽平移、系统明暗主题持久化与处理结果并排展示。
- 新增 `itb s3 stat`：单次 HEAD 查询对象元数据（大小、ETag、Content-Type、用户 metadata 等），不传输对象内容，支持 `table`/`json` 输出。
- `itb s3 upload` 新增 `--skip-existing` 与 `--skip-unchanged`（互斥）：分别按"对象键已存在"与"远端 `x-amz-meta-itb-sha256` 与本地 SHA-256 一致"跳过上传，以 1 次 HEAD 代替整文件传输；默认行为仍为无条件覆盖。
- 根命令支持 `itb --version` / `-v` 与未知命令名的纠错建议（Suggest），`s3` 子命令同步开启。
- 发布工作流在发布前于 CI 执行 `make check` 与 `make test`，并将发布归档同步上传到 WebDAV。

### Changed

- **BREAKING:** compress 未指定 `--output` 时不再覆盖原文件，改为输出 `*_compressed.*`；需要覆盖时显式传入新增的 `--in-place`（与 `--output` 互斥）。
- **BREAKING:** CLI 框架由 `spf13/cobra` 迁移至 `urfave/cli/v3`，命令树在 `cmd.New()` 中显式构造，移除包级 flag 全局变量与 `init()` 注册；用户可见命令与 `itb version` 输出保持不变。
- 参数校验前移至 CLI 层：枚举（resize/crop/convert/watermark/inspect/s3）、范围（quality、opacity、尺寸等）与百分比参数在文件 IO 之前报错；flag Usage 统一 `FILE`/`FORMAT`/`MODE` 等占位符。
- compress 编排逻辑下沉到 `internal/compress` 领域包，供 CLI 与 Web API 复用。
- S3 上传在 skip 模式下 HEAD 前置并只打开一次文件，减少本地 IO。

### Removed

- **BREAKING:** 移除 `itb metadata` 别名，请使用 `itb inspect`；`inspect` 新增 `--no-detail` 关闭详细元数据（优先于 `--detail`）。
- 完全移除批处理能力：`itb batch` CLI、`internal/batch` 包、`/api/v1/batch/*` HTTP API 及前端残留的 Batch 组件与 API client。
- 完全移除 LskyPro 集成：`itb lsky` CLI、`internal/lsky` 包、`ITB_LSKY_*` 环境变量，以及 WebUI 的 `/api/v1/lsky/images` 上传接口、前端 `LskyPanel` 与整个 `web/src/storage/` 目录。
- WebUI 移除 S3 面板与 `/api/v1/s3/*` 接口；WebUI 收敛为图像处理功能（压缩/缩放/裁剪/转换/水印）。`itb s3` CLI 与 `ITB_S3_*` 环境变量不受影响。

### Fixed

- S3 配置来源统一到 CLI 层：`ITB_S3_*` 环境变量改由 urfave/cli `Sources` 绑定，`ITB_S3_BUCKET` 等环境变量现在能真正满足 `--bucket` 等 required flag 校验（此前因 Action 前置校验而失效），优先级为 CLI flag > 环境变量 > 默认值；`internal/s3` 不再读取环境变量，收敛为纯领域包。
- S3 HTTP 客户端移除 30s 总超时（会截断大文件上传/下载），改为 Transport 层 `ResponseHeaderTimeout=30s`，连接/TLS/连接池继承标准库默认值。
- 修正 CLI help 漂移：watermark 移除未实现的 `--tile` flag，resize/crop 补充尺寸与锚点参数组合规则，s3 必填项与默认值描述与实际行为一致。
- WebUI 修复中文下载文件名乱码。

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
