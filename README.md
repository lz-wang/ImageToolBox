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
| `-t, --text` | 与 `--image` 二选一 | 水印文字 |
| `-o, --output` | `*_watermarked.*` | 输出路径，默认在原文件名后加 `_watermarked` |
| `-m, --mode` | `position` | 水印模式：`position`（位置）/ `repeat`（平铺） |
| `--color` | (自动) | 水印颜色，如 `#FF0000`；空则自动选择黑/白 |
| `--opacity` | `0.5` | 透明度，范围 0~1 |
| `--font-size` | `0` | 字体大小，`0` 表示根据图片自动计算 |
| `--font` | (自动) | 字体文件路径，空则自动使用系统字体 |
| `--image` | 与 `--text` 二选一 | 图片水印路径 |
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

默认只监听 `127.0.0.1`，请勿绑定到不可信网络。WebUI 仅提供本地图像处理，不涉及任何外部服务凭证。

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

图片处理端点（`compress` / `resize` / `crop` / `convert` / `watermark` / `inspect`）使用 `multipart/form-data`：`file` + `options`（JSON 字符串），处理结果直接以图片二进制流返回。

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

## S3 兼容存储操作

支持 AWS S3、MinIO、阿里云 OSS、腾讯云 COS 等所有 S3 协议兼容的存储服务。

### 环境变量

```bash
ITB_S3_ENDPOINT           # S3 端点 URL（可选）
ITB_S3_ACCESS_KEY_ID      # Access Key ID
ITB_S3_SECRET_ACCESS_KEY  # Secret Access Key
ITB_S3_REGION             # 区域（默认 us-east-1）
ITB_S3_BUCKET             # 存储桶名称（也可用 -b 指定）
```

<details>
<summary>公共参数</summary>

所有 S3 子命令共享以下参数：

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-e, --endpoint` | (必填) | S3 端点 URL |
| `-a, --access-key` | (环境变量) | Access Key ID |
| `-s, --secret-key` | (环境变量) | Secret Access Key |
| `-r, --region` | `us-east-1` | 区域 |
| `-b, --bucket` | (必填) | 存储桶名称 |
| `--force-path-style` | `false` | 强制路径样式 URL（MinIO 需要） |

</details>

### 上传文件

```bash
# 上传文件到存储桶
./itb s3 upload -i photo.jpg -b my-bucket -e http://localhost:9000

# 指定对象键名（默认使用文件名）
./itb s3 upload -i photo.jpg -b my-bucket -k images/photo.jpg

# 指定 Content-Type
./itb s3 upload -i data.json -b my-bucket --content-type application/json

# 同名对象已存在即跳过（1 次 HEAD 代替整文件上传）
./itb s3 upload -i photo.jpg -b my-bucket --skip-existing

# 内容一致才跳过（比对 itb-sha256 metadata，不依赖 ETag）
./itb s3 upload -i photo.jpg -b my-bucket --skip-unchanged
```

上传时会把本地文件的 SHA-256 写入对象用户 metadata（`x-amz-meta-itb-sha256`），
`--skip-unchanged` 依赖该值判断远端对象与本地是否一致；默认行为仍是无条件覆盖。
`--skip-existing` 与 `--skip-unchanged` 是互斥的上传策略，同时使用会报参数错误。

<details>
<summary>upload 参数</summary>

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-i, --input` | (必填) | 本地文件路径 |
| `-k, --key` | 文件名 | 对象键名 |
| `--content-type` | 自动检测 | 内容类型 |
| `--skip-existing` | `false` | 对象键已存在即跳过上传 |
| `--skip-unchanged` | `false` | 内容一致才跳过上传（比对 itb-sha256 metadata） |

</details>

### 下载文件

```bash
# 下载文件
./itb s3 download -b my-bucket -k photo.jpg -o ./photo.jpg

# 使用对象键名作为本地文件名
./itb s3 download -b my-bucket -k images/photo.jpg
```

<details>
<summary>download 参数</summary>

| 参数 | 说明 |
|------|------|
| `-k, --key` | 对象键名（必填） |
| `-o, --output` | 本地输出路径（默认使用对象键名） |

</details>

### 删除对象

```bash
# 删除对象（需要确认）
./itb s3 delete -b my-bucket -k photo.jpg

# 强制删除（不需要确认）
./itb s3 delete -b my-bucket -k photo.jpg -f
```

<details>
<summary>delete 参数</summary>

| 参数 | 说明 |
|------|------|
| `-k, --key` | 对象键名（必填） |
| `-f, --force` | 强制删除，不确认 |

</details>

### 列出对象

```bash
# 列出所有对象
./itb s3 list -b my-bucket

# 按前缀过滤
./itb s3 list -b my-bucket -p images/

# JSON 格式输出
./itb s3 list -b my-bucket --format json
```

<details>
<summary>list 参数</summary>

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-p, --prefix` | | 对象键前缀 |
| `--max-keys` | `1000` | 最大返回数量 |
| `--format` | `table` | 输出格式：`table` / `json` / `plain` |

</details>

### 查看对象元数据

```bash
# 查看单个对象的完整元数据（只发一次 HEAD 请求，不下载内容）
./itb s3 stat -b my-bucket -k images/photo.jpg

# JSON 格式输出
./itb s3 stat -b my-bucket -k images/photo.jpg --format json
```

stat 始终按精确对象键查询，对象不存在时不回退到 list 推断。返回的元数据包括
Size、ETag、Content-Type、Storage Class、Cache-Control、Version ID 与用户 Metadata。

<details>
<summary>stat 参数</summary>

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-k, --key` | (必填) | 对象键名 |
| `--format` | `table` | 输出格式：`table` / `json` |

</details>

<details>
<summary>云服务商配置示例</summary>

| 云服务商 | Endpoint 示例 | ForcePathStyle |
|---------|---------------|----------------|
| AWS S3 | `https://s3.amazonaws.com` | `false` |
| MinIO | `http://localhost:9000` | `true` |
| 阿里云 OSS | `https://oss-cn-hangzhou.aliyuncs.com` | `false` |
| 腾讯云 COS | `https://cos.ap-guangzhou.myqcloud.com` | `false` |

</details>

## 许可证

本项目使用 MIT 许可证。内置的第三方工具请参阅 [LICENSE-THIRD-PARTY.md](./LICENSE-THIRD-PARTY.md)。
