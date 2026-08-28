<p align="center">
  <img src="web/public/logo.svg" width="112" height="112" alt="Image Tool Box Logo">
</p>

# GO 图像工具箱

> **English**: [README.en.md](./README.en.md)

> 外部依赖说明见 [docs/build-bins.md](docs/build-bins.md)。
> 
> CI 当前会并行构建以下平台：
> 
> - macOS amd64 / arm64
> - Linux amd64 / arm64
> - Windows amd64 / arm64
> 
> Release 产物中，macOS / Linux 使用 `.tar.gz`，Windows 使用 `.zip`；Windows 可执行文件和内置压缩工具均带 `.exe` 扩展名。

## 安装

新版本发布后，可通过 Homebrew（macOS / Linux）安装：

```bash
brew tap lz-wang/tap
brew install lz-wang/tap/itb
```

> [!WARNING]
> **macOS 运行提示**
>
> 如果在 macOS 上运行二进制时提示“无法验证开发者”，并且每次都需要到“安全性与隐私”里手动放行，内部使用场景下可以在下载或解压后先移除 `quarantine` 标记：
>
> ```bash
> xattr -d com.apple.quarantine your_binary
> ```

## 压缩图片

自动检测图片格式（PNG/JPEG）并压缩：

```bash
# 压缩 PNG 图片（覆盖原文件）
./itb compress -i photo.png

# 压缩 JPEG 图片（覆盖原文件）
./itb compress -i photo.jpg

# 指定输出文件
./itb compress -i photo.png -o compressed.png

# 指定压缩质量（1-100，默认 80）
./itb compress -i photo.jpg -q 90
```

<details>
<summary>命令参数与压缩管道</summary>

| 参数 | 说明 |
|------|------|
| `-i, --input` | 输入图片文件路径 |
| `-o, --output` | 输出图片文件路径（不指定则覆盖原文件） |
| `-q, --quality` | 压缩质量 1-100（默认 80） |

**压缩管道**

- **PNG**: `pngquant` → `oxipng`（有损 + 无损双重压缩）
- **JPEG**: `djpeg` → `cjpeg`（libjpeg-turbo 解码 + 编码）

</details>

## 图像裁剪

按锚点和百分比保留图片区域。

```bash
# 保留左侧 40% 宽度
./itb crop -i a.jpg --anchor left --width 40%

# 保留右侧 40% 宽度
./itb crop -i a.jpg --anchor right --width 40%

# 保留左上角 40% x 40% 区域
./itb crop -i a.jpg --anchor top-left --width 40% --height 40%

# 保留中心 40% x 40% 区域
./itb crop -i a.jpg --anchor center --width 40% --height 40%
```

<details>
<summary>命令参数与规则</summary>

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-i, --input` | (必填) | 输入图片路径 |
| `-o, --output` | `*_cropped.*` | 输出路径，默认在原文件名后加 `_cropped` |
| `--anchor` | (必填) | 裁剪锚点：`left` / `right` / `top` / `bottom` / `top-left` / `top-right` / `bottom-left` / `bottom-right` / `center` |
| `--width` | | 裁剪宽度百分比，例如 `40%` |
| `--height` | | 裁剪高度百分比，例如 `40%` |

**参数规则**

- 仅支持百分比格式，范围为 `(0, 100]`
- `left` / `right` 必须提供 `--width`，且不能提供 `--height`
- `top` / `bottom` 必须提供 `--height`，且不能提供 `--width`
- `top-left` / `top-right` / `bottom-left` / `bottom-right` / `center` 必须同时提供 `--width` 和 `--height`

</details>

## 图像缩放

支持按宽高、百分比和不同模式调整图片尺寸。

```bash
# 指定宽度，按比例缩放
./itb resize -i photo.jpg --width 1200

# 指定宽高框，保持比例适配
./itb resize -i photo.jpg --width 1200 --height 630 --mode fit

# 指定宽高框并裁切填满
./itb resize -i photo.jpg --width 1200 --height 630 --mode fill --anchor top

# 按百分比缩放
./itb resize -i photo.png --percent 50%
```

<details>
<summary>命令参数</summary>

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-i, --input` | (必填) | 输入图片路径 |
| `-o, --output` | `*_resized.*` | 输出路径 |
| `--width` | | 目标宽度 |
| `--height` | | 目标高度 |
| `--percent` | | 按比例缩放，例如 `50%` |
| `--mode` | `fit` | 缩放模式：`fit` / `fill` / `stretch` |
| `--anchor` | `center` | `fill` 模式的锚点 |
| `--filter` | `lanczos` | 采样器：`nearest` / `linear` / `catmullrom` / `lanczos` |

</details>

## 图像格式转换

支持 `jpg/jpeg/png/webp` 互转，输出格式由 `--to` 指定。

```bash
# 转为 WebP
./itb convert -i photo.png --to webp

# 透明 PNG 转 JPG，指定铺底颜色
./itb convert -i photo.png --to jpg --background "#FFFFFF"

# 指定输出路径
./itb convert -i photo.jpg --to png -o output.png
```

<details>
<summary>命令参数</summary>

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-i, --input` | (必填) | 输入图片路径 |
| `-o, --output` | `*_converted.<ext>` | 输出路径 |
| `--to` | (必填) | 目标格式：`jpg` / `jpeg` / `png` / `webp` |
| `-q, --quality` | `80` | 有损格式质量 |
| `--lossless` | `false` | 无损编码（webp/png） |
| `--background` | `#FFFFFF` | 转不透明格式时的背景色 |

</details>

## 图像水印

为图片添加文字水印，支持两种模式：位置水印（单点）和重复平铺水印。

### 位置水印（position）

在指定位置添加单个水印，自动根据背景亮度选择黑/白文字颜色，并添加描边提高可读性。

```bash
# 默认右下角
./itb watermark -i photo.jpg -t "© Author"

# 指定位置
./itb watermark -i photo.png -t "Copyright" --position center

# 调整透明度
./itb watermark -i photo.png -t "Author" --opacity 0.8

# 指定输出路径
./itb watermark -i photo.jpg -t "Author" -o output.jpg

# 添加图片水印
./itb watermark -i photo.jpg --image logo.png --scale 0.2 --position bottom-right
```

### 重复平铺水印（repeat）

文字以平铺方式覆盖整张图片，支持旋转角度和间距调整。

```bash
# 基本用法
./itb watermark -i photo.png -t "WATERMARK" --mode repeat

# 自定义旋转角度和透明度
./itb watermark -i photo.png -t "DRAFT" --mode repeat --angle 45 --opacity 0.3

# 自定义颜色
./itb watermark -i photo.png -t "CONFIDENTIAL" --mode repeat --color "#FF0000"
```

<details>
<summary>命令参数</summary>

**通用参数**

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-i, --input` | (必填) | 输入图片路径 |
| `-t, --text` | (必填) | 水印文字 |
| `-o, --output` | `*_watermarked.*` | 输出路径，默认在原文件名后加 `_watermarked` |
| `-m, --mode` | `position` | 水印模式：`position`（位置）/ `repeat`（平铺） |
| `--color` | (自动) | 水印颜色，如 `#FF0000`；空则自动选择黑/白 |
| `--opacity` | `0.5` | 透明度，范围 0~1 |
| `--font-size` | `0` | 字体大小，`0` 表示根据图片自动计算 |
| `--font` | (自动) | 字体文件路径，空则自动使用系统字体 |
| `--image` | | 图片水印路径，与 `--text` 二选一 |
| `--scale` | `0.2` | 图片水印缩放比例，基于底图短边 |
| `--tile` | `false` | 图片平铺水印，当前版本暂不支持 |

**position 模式参数**

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--position` | `bottom-right` | 水印位置：`bottom-right` / `bottom-left` / `top-right` / `top-left` / `center` |
| `--margin` | `0.04` | 边距比例，基于图片短边计算 |

**repeat 模式参数**

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--angle` | `30` | 旋转角度（度） |
| `--space` | `0` | 平铺间距，`0` 表示根据字体大小自动计算 |

</details>

## 图片检查

读取图片文件信息、图像基本信息、详细元数据和文件 hash。

```bash
# 默认表格输出，默认计算所有 hash，默认输出详细数据
./itb inspect -i photo.jpg

# JSON 输出
./itb inspect -i photo.jpg --format json

# 只输出 sha256
./itb inspect -i photo.jpg --format plain

# 关闭详细数据
./itb inspect -i photo.jpg --detail=false

# 不计算 hash
./itb inspect -i photo.jpg --no-hash
```

<details>
<summary>命令参数</summary>

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-i, --input` | (必填) | 输入图片路径 |
| `--format` | `table` | 输出格式：`table` / `json` / `plain` |
| `--detail` | `true` | 输出详细元数据 |
| `--no-hash` | `false` | 不计算文件 hash |
| `--strict` | `false` | 图像解析失败时直接返回错误 |

</details>

## 批量处理

支持批量执行 `resize`、`convert`、`watermark`，输出目录保留相对目录结构。

```bash
# 批量缩放
./itb batch resize --input-dir ./images --output-dir ./out --recursive --width 1200

# 批量转 WebP
./itb batch convert --input-dir ./images --output-dir ./out --glob "*.png" --to webp

# 批量添加文字水印
./itb batch watermark --input-dir ./images --output-dir ./out -t "© Author"
```

<details>
<summary>公共参数</summary>

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--input-dir` | (必填) | 输入目录 |
| `--output-dir` | (必填) | 输出目录 |
| `--glob` | `*` | 文件匹配模式 |
| `--recursive` | `false` | 递归处理子目录 |
| `--workers` | `4` | 并发 worker 数 |
| `--skip-existing` | `false` | 输出已存在时跳过 |
| `--fail-fast` | `false` | 遇错尽快停止 |

</details>

## WebUI（itb serve）

除了 CLI，`itb` 还内置一个本地优先的 WebUI，与 CLI 共享完全相同的 Go 图片处理核心（WebUI 不执行 CLI 子进程，而是直接调用领域包）。

```bash
# 启动 WebUI（默认 http://127.0.0.1:8080）
./itb serve

# 指定监听地址
./itb serve --addr 127.0.0.1:9000

# 启动后自动打开浏览器
./itb serve --open
```

前端产物已内嵌进二进制，无需额外部署文件；CI Release 仍然只发布单个 `itb` 可执行文件。

![WebUI](docs/screenshots/webui.png)

### 功能范围

- **图片工具**：压缩、缩放、裁剪、格式转换、水印（文字/图片，支持上传字体）；上传图片后自动展示元数据检查结果，带 Before/After 对比与体积变化展示

### 安全边界

默认只监听 `127.0.0.1`，请勿绑定到不可信网络。Lsky 凭证只在**服务端**从环境变量读取，Token 永远不会进入浏览器：

```bash
ITB_LSKY_URL              # LskyPro 地址
ITB_LSKY_TOKEN            # LskyPro API Token
```

<details>
<summary>命令参数与 HTTP API</summary>

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--addr` | `127.0.0.1:8080` | 监听地址 |
| `--open` | `false` | 启动后自动打开浏览器 |

WebUI 后端 API 统一前缀 `/api/v1`，例如健康检查：

```bash
curl http://127.0.0.1:8080/api/v1/health
```

图片处理端点（`compress` / `resize` / `crop` / `convert` / `watermark` / `inspect` 及 `batch/*`）使用 `multipart/form-data`：`file`（或 `files[]`）+ `options`（JSON 字符串），处理结果直接以图片二进制流返回。

</details>

<details>
<summary>从源码构建</summary>

```bash
make build    # 构建 WebUI（npm）后编译 itb
make serve    # 构建并启动 WebUI
make check    # go vet + 前端 type-check + lint
make test     # go test + 前端测试
```

本地开发前端时可用 `cd web && npm run dev`（Vite 开发服务器会把 `/api` 代理到 `127.0.0.1:8080`）。

</details>

## LskyPro 上传

支持上传图片到 LskyPro 图床，兼容直接传站点根地址或完整的 `/api/v1` 地址。

### 环境变量

```bash
ITB_LSKY_URL    # LskyPro 地址，例如 https://img.example.com 或 https://img.example.com/api/v1
ITB_LSKY_TOKEN  # API Token
```

### 上传图片

```bash
# 使用环境变量上传
./itb lsky upload -i photo.jpg

# 显式指定服务地址和 Token
./itb lsky upload -i photo.jpg --url https://img.example.com --token your-token

# 指定存储策略 ID
./itb lsky upload -i photo.jpg --strategy 2

# 以 JSON 输出完整响应
./itb lsky upload -i photo.jpg --output json

# 输出 URL
./itb lsky upload -i photo.jpg --output url
```

<details>
<summary>upload 参数</summary>

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-i, --input` | (必填) | 本地图片路径 |
| `--url` | (环境变量) | LskyPro 服务地址 |
| `--token` | (环境变量) | LskyPro API Token |
| `-s, --strategy` | `0` | 存储策略 ID，`0` 表示不指定 |
| `-o, --output` | `markdown` | 输出格式：`markdown` / `url` / `json` |

</details>

## 许可证

本项目使用 MIT 许可证。内置的第三方工具请参阅 [LICENSE-THIRD-PARTY.md](./LICENSE-THIRD-PARTY.md)。
