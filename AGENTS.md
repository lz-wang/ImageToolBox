# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

`itb`（imagetoolbox）是一个用 Go 编写的图片处理 CLI 工具箱，基于 `spf13/cobra`。模块路径为 `imagetoolbox`，Go 版本见 `go.mod`（当前 1.26.x）。所有原生压缩工具（pngquant、oxipng、libjpeg-turbo 的 cjpeg/djpeg）以内嵌二进制形式分发，运行时无需外部依赖。

## 常用命令

```bash
make build              # 构建 itb（注入 version ldflags：日期+短 SHA）
make clean              # 删除 itb
go build ./...          # 仅编译，不产出二进制
go test ./...           # 运行全部测试
go test ./internal/resize -run TestApplyPercent -v   # 运行单个测试
go vet ./...            # 静态检查
```

构建产物为根目录的 `itb`（已被 gitignore）。CI 中固定使用 `CGO_ENABLED=0`，本地无需 CGO。

## 架构

### 分层与依赖方向

```
main.go  ──→  internal/cmd  ──→  各领域包 (compress/resize/convert/crop/watermark/...)
                     │                      │
                     └──────────→  internal/imageio（共享的编解码/格式/铺底/取色层）
```

- `internal/cmd`：所有 `cobra.Command` 定义、flag 绑定、文件 IO 与错误打印。命令逻辑只做参数解析和编排，真正处理委托给领域包。
- 领域包（`resize`、`convert`、`crop`、`watermark`、`compress`、`batch`、`s3`、`lsky`、`inspect`）：接受 `Options` 结构体、操作 `image.Image` 或文件路径，**不依赖 cobra**。这种解耦使 `batch` 能直接复用 `resize/convert/watermark` 的处理函数。
- `internal/imageio`：跨领域共享的格式归一化（`NormalizeFormat`/`FormatFromPath`）、保存（`Save`/`SaveWithFormat`）、编码（`Encode`，含 JPEG/PNG/WEBP）、透明图铺底（`Flatten`）、十六进制颜色解析（`ParseHexColor`）。新增格式编解码应集中在这里。
- `internal/s3`、`internal/lsky`：存储后端，通过 `cmd/s3.go`、`cmd/lsky.go` 暴露为子命令，凭证优先读环境变量。

### 命令注册约定

每个命令位于 `internal/cmd/<name>.go`，定义一个包级 `*cobra.Command` 变量，并在 `init()` 中调用 `rootCmd.AddCommand(...)`。根命令在 `root.go`，`Execute(version)` 由 `main.go` 调用并注入版本号（`-ldflags "-X main.version=..."`）。

**注意**：`cmd` 包内的 flag 变量是**包级共享**的——单命令与 `batch` 子命令复用同一组变量（例如 `resizeWidth`、`convertTo`、`wmText` 定义在各自的单命令文件中，却在 `batch.go` 里被引用）。新增/重命名 flag 时需同步检查单命令与 batch 两处。

### 内嵌二进制机制（核心约束）

`main.go` 通过 `//go:embed bins/**` 把 `bins/<os>-<arch>/` 下的原生工具嵌入二进制：

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

## 外部工具与 CI

`docs/build-bins.md` 记录 pngquant（3.0.3）、oxipng（v10.1.0）、libjpeg-turbo（3.1.3）的版本与各平台 cmake 构建方式。`.github/workflows/build-binaries.yml` 与 `release.yml` 在 CI 中从源码构建这些原生工具，注入 `bins/`，再用 `CGO_ENABLED=0` 交叉构建 darwin/linux/windows × amd64/arm64；macOS/Linux 打 `.tar.gz`，Windows 打 `.zip`。

## skills/itb

`skills/itb/` 是随仓库签入的 Claude Code Skill（`.gitignore` 中显式 `!skills/itb/**` 保留），指导在图像工作流里正确选择 `itb` 命令与 flag。修改 CLI 行为时同步检查其 `SKILL.md` 与 `references/` 是否需要更新。

## 开发约定（来自全局规则）

- Go 代码统一使用 **Tab** 缩进；使用内置 Edit 工具时 `old_string`/`new_string` 必须用 Tab 匹配。
- 文件读写只用 Read/Write/Edit，禁止 `sed`/`awk`/`cat` 等编辑文件内容。
