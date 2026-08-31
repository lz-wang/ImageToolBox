# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

`itb`（imagetoolbox）是一个用 Go 编写的图片处理 CLI 工具箱，基于 `urfave/cli/v3`。模块路径为 `imagetoolbox`，Go 版本见 `go.mod`（当前 1.26.x）。所有原生压缩工具（pngquant、oxipng、libjpeg-turbo 的 cjpeg/djpeg）以内嵌二进制形式分发，运行时无需外部依赖。

## 常用命令

```bash
make build              # 编译 itb（注入 version ldflags）
make serve              # make build 后启动 HTTP API
make check              # go vet
make test               # go test
make clean              # 删除 itb 构建产物
go build ./...          # 仅编译，不产出二进制
go test ./...           # 运行全部测试
go test ./internal/resize -run TestApplyPercent -v   # 运行单个测试
go vet ./...            # 静态检查
```

构建产物为根目录的 `itb`（已被 gitignore）。CI 中固定使用 `CGO_ENABLED=0`，本地无需 CGO。

## 架构

### 适配器与领域层边界

- Domain 包是图片操作参数的唯一 Normalize/Validate 与业务规则来源。CLI 和 HTTP 只能将传输参数映射为领域 `Options`，不得通过调用 `cli.Command` 或复制业务分派来复用逻辑。
- HTTP API 只暴露 `compress`、`resize`、`crop`、`convert`、`watermark` 和 `inspect`；S3 管理能力只由 CLI 暴露。
- HTTP 操作参数使用对应 CLI long flag 名称；`input` 是 multipart 上传文件，`output` 和 `in-place` 不属于 HTTP 参数。
- HTTP API 是可信远程服务而非远程 Shell：不提供 WebUI、工作流、用户系统、数据库、任务队列、TLS/ACME 或 API S3 管理能力。
- `serve` 负责 HTTP server 生命周期；`internal/httpapi` 只负责 HTTP adapter。每个请求必须有独立临时目录并在结束时清理。

### 分层与依赖方向

```
main.go ──→ internal/cmd（CLI）──→ 各领域包 (compress/resize/convert/crop/watermark/...)
   │                                │
   └──→ internal/httpapi（HTTP API）┴──→ internal/imageio（共享的编解码/格式/铺底/取色层）
```

- `internal/cmd`：所有 `cli.Command` 定义、flag 绑定、文件 IO 与错误打印。命令逻辑只做参数解析和编排，真正处理委托给领域包。
- 领域包（`resize`、`convert`、`crop`、`watermark`、`compress`、`s3`、`inspect`）：接受 `Options` 结构体、操作 `image.Image` 或文件路径，**不依赖 urfave/cli**。这种解耦使 Web API 能直接复用领域包的处理函数。
- `internal/imageio`：跨领域共享的格式归一化（`NormalizeFormat`/`FormatFromPath`）、保存（`Save`/`SaveWithFormat`）、编码（`Encode`，含 JPEG/PNG/WEBP）、透明图铺底（`Flatten`）、十六进制颜色解析（`ParseHexColor`）。新增格式编解码应集中在这里。
- `internal/s3`：存储后端，通过 `cmd/s3.go` 暴露为子命令。`ITB_S3_*` 环境变量由 CLI 层（urfave/cli 的 `Sources`）解析注入，优先级为 CLI flag > 环境变量 > 默认值；`internal/s3` 是纯领域包，自身不读取环境变量。注意：存储后端仅暴露为 CLI 子命令，HTTP API 不提供任何存储相关 API。
- `internal/httpapi`：`itb serve` 的标准库 HTTP API（`/api/v1`），直接调用领域包而非 CLI 子进程。

### 命令注册约定

每个命令位于 `internal/cmd/<name>.go`，导出一个 `newXxxCommand() *cli.Command` constructor。根命令在 `root.go` 的 `New(version)` 中一次性显式拼装命令树，`Execute(ctx, version)` 由 `main.go` 调用并注入版本号（`-ldflags "-X main.version=..."`）。

**注意**：`cmd` 包内**禁止包级可变状态**——flag 值一律通过 Action 内的 `cmd.String()`/`cmd.Int()`/`cmd.Bool()` 读取，不引入包级 flag 变量或 `init()` 注册。

### HTTP API 约束（internal/httpapi）

- Web handler 直接构造领域 `Options`，与 CLI 命令参数状态完全隔离，保证并发 HTTP 请求互不污染。
- 图片处理端点统一 `multipart/form-data`，使用 CLI long flag 名称；结果以二进制流返回并带 `Content-Disposition` 与 `X-ITB-*-Size` 头。
- 每个请求使用独立临时目录（`os.MkdirTemp`），`defer` 清理；不引入数据库/session/任务系统。
- 安全边界：默认只绑定 `127.0.0.1`；远程部署必须使用 `ITB_API_TOKEN` Bearer token 和反向代理保护。`--no-auth` 仅限 loopback 开发。
- 上传文件名经 `sanitizeFilename` 清洗，防止路径穿越。

### 内嵌二进制机制（核心约束）

`main.go` 通过 `//go:embed bins/**` 把 `bins/<os>-<arch>/` 下的原生工具嵌入二进制（平台目录被 gitignore，由 CI 构建注入；`bins/README.md` 是 embed 的兜底匹配文件，必须保留在 git 中，否则全新 checkout 无法编译）：

1. `compress.InitBinaries(embed.FS)` 在 `main` 启动时注入 FS（避免 `compress` 包直接依赖 `main`）。
2. 首次调用 `compress.EnsureBinary(binType)` 时，`sync.Once` 触发 `extractAllBinaries()`，按 `runtime.GOOS-GOARCH` 选出对应平台的 pngquant/oxipng/djpeg/cjpeg，解压到 `os.TempDir()/img-compress-bins`（已存在且大小相同则跳过写入），返回临时路径供 `exec.Command` 调用。
3. 平台映射在 `internal/compress/embed.go` 的 `binaryPaths`。**新增平台或工具必须同步更新该映射**，并按 `docs/build-bins.md` 的约定把产物放入 `bins/<os>-<arch>/`（Windows 一律带 `.exe`）。

### 两套格式检测

- `compress.DetectFormat(io.ReadSeeker)`：基于文件头，返回小写格式名（`"png"`/`"jpeg"`），用于压缩命令分流。
- `imageio.DetectFormat(path)`/`FormatFromPath`：返回 `Format` 枚举，供通用编解码使用。改动格式支持时两者都要顾及。

## 测试约定

- 表驱动 + `t.Run`，测试文件与被测代码同包（如 `package resize`）。
- 不使用 `testdata/`，测试中用 `image.NewNRGBA` 等就地合成图片，避免二进制 fixture。
- 根目录的 `test-images/` 仅作手动验证（已 gitignore），不要在测试里引用。
- 纯 Go 单测不依赖内嵌的原生二进制；涉及 `compress` 的集成测试才会触发解压流程。
- `internal/s3/minio_test.go` 是针对真实 MinIO 的集成测试（upload/stat/download/skip/metadata/cache-control/overwrite/verify/delete + path-style）：CI 以 service container 启动 MinIO 真实执行；本地默认跳过，可用 `ITB_TEST_MINIO_ENDPOINT`（默认 `http://127.0.0.1:9000`）、`ITB_TEST_MINIO_ACCESS_KEY`/`ITB_TEST_MINIO_SECRET_KEY`（默认 `minioadmin`）指向自建实例运行。

## 文档约定

- 仓库维护双语 README：中文 `README.md`、英文 `README.en.md`，两者页首通过互链相互引用（中文顶部链向英文版，英文顶部链向中文版）。
- 任何对功能、命令、flag、参数、示例、环境变量或目录结构的变更，**必须同步更新两个 README**，严禁只改其一。
- 两个文件的小节结构与 `<details>` 折叠块应一一对应；新增、删除、重命名内容时同步处理两侧。
- 翻译时命令、flag、参数名、代码块保持原样，只翻译说明性文字；表头在英文版用 Option / Default / Description。

## 外部工具与 CI

`docs/build-bins.md` 记录 pngquant（3.0.3）、oxipng（v10.1.0）、libjpeg-turbo（3.1.3）的版本与各平台 cmake 构建方式。`.github/workflows/build-binaries.yml` 与 `release.yml` 在 CI 中从源码构建这些原生工具，注入 `bins/`，最后用 `CGO_ENABLED=0` 交叉构建 darwin/linux/windows × amd64/arm64；macOS/Linux 打 `.tar.gz`，Windows 打 `.zip`。Release 会发布六个平台归档及各自 SHA-256 校验和。原生压缩工具在构建阶段放入 `bins/<platform>`，通过 `go:embed` 编入 `itb` 可执行文件；最终发行归档只包含 `itb`，无需独立携带 `bins/` 目录。

## Release and Homebrew publishing

发布标签必须是 `vX.Y.Z`。`release.yml` 会先执行 `make check` 与 `make test`，再构建六个平台归档、创建 GitHub Release、上传归档及校验和到 `/Shares/github/<owner>/<repo>/<version>/`，最后从已发布资产读取四个 macOS/Linux 校验和，生成、审计、安装并测试 `lz-wang/homebrew-tap` 的 `Formula/itb.rb` 后才推送 Formula。

发布前运行 `make check`、`make test` 与 `git diff --check`；推送带注释标签后，确认 GitHub Release 的六个归档和六个 `.sha256` 文件，以及 Homebrew Formula 提交均已完成。`HOMEBREW_TAP_TOKEN` 必须对 `lz-wang/homebrew-tap` 具有 Contents 读写权限；WebDAV 凭据与可选 Pushover 通知均通过 GitHub Actions Secrets 配置，绝不写入仓库。

## skills/itb

`skills/itb/` 是随仓库签入的 Claude Code Skill（`.gitignore` 中显式 `!skills/itb/**` 保留），指导在图像工作流里正确选择 `itb` 命令与 flag。修改 CLI 行为时同步检查其 `SKILL.md` 与 `references/` 是否需要更新。

## 开发约定（来自全局规则）

- Go 代码统一使用 **Tab** 缩进；使用内置 Edit 工具时 `old_string`/`new_string` 必须用 Tab 匹配。
- 文件读写只用 Read/Write/Edit，禁止 `sed`/`awk`/`cat` 等编辑文件内容。
