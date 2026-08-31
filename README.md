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

## Linux 兼容性

官方 Linux amd64 / arm64 构建的 `compress` 功能要求 **glibc >= 2.28**；Go 实现的其他功能不在启动时强制检查此条件。Alpine Linux / musl 当前不受支持。

| 系统 | `compress` 支持情况 |
|------|---------------------|
| CentOS 7（glibc 2.17） | ❌ |
| Ubuntu 18.04（glibc 2.27） | ❌ |
| Rocky / AlmaLinux 8（glibc 2.28） | ✅ |
| Debian 10+ | ✅ |
| Ubuntu 20.04+ | ✅ |

CI 使用 `manylinux_2_28` 构建 Linux 内置压缩器，并校验其最高 `GLIBC_*` 符号版本及动态库依赖。发行包只包含单个 `itb` 文件；压缩器仍内嵌在其中，并在首次使用时按需解压到用户缓存目录。

## 安装

新版本发布后，可通过 Homebrew（macOS / Linux）安装：

```bash
brew tap lz-wang/tap
brew install lz-wang/tap/itb
```

安装完成后可运行 `itb --version`（或 `itb version`）验证。

> [!WARNING]
> **macOS 运行提示**
>
> 如果在 macOS 上运行二进制时提示“无法验证开发者”，并且每次都需要到“安全性与隐私”里手动放行，内部使用场景下可以在下载或解压后先移除 `quarantine` 标记：
>
> ```bash
> xattr -d com.apple.quarantine your_binary
> ```

## 压缩图片

自动检测图片格式（PNG/JPEG）并压缩，默认保留原文件：

```bash
# 压缩 PNG 图片（输出 photo_compressed.png）
./itb compress -i photo.png

# 压缩 JPEG 图片（输出 photo_compressed.jpg）
./itb compress -i photo.jpg

# 指定输出文件
./itb compress -i photo.png -o compressed.png

# 覆盖原文件（与 --output 互斥）
./itb compress -i photo.jpg --in-place

# 指定压缩质量（1-100，默认 80）
./itb compress -i photo.jpg -q 90
```

<details>
<summary>命令参数与压缩管道</summary>

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-i, --input` | (必填) | 输入图片文件路径 |
| `-o, --output` | `*_compressed.*` | 输出路径，默认在原文件名后加 `_compressed` |
| `--in-place` | `false` | 覆盖输入文件（与 `--output` 互斥） |
| `-q, --quality` | `80` | 压缩质量 1-100 |

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
| `--width` | | 目标宽度（像素） |
| `--height` | | 目标高度（像素） |
| `--percent` | | 按百分比精确缩放，例如 `50%`；支持放大如 `200%` |
| `--mode` | `fit` | 缩放模式：`fit` / `fill` / `stretch` |
| `--anchor` | `center` | `fill` 模式的锚点 |
| `--filter` | `lanczos` | 采样器：`nearest` / `linear` / `catmullrom` / `lanczos` |

**参数规则**

- 必须指定 `--percent`，或至少指定 `--width` / `--height` 之一
- `--percent` 不能与 `--width` / `--height` 同时使用
- `fit` 支持仅指定宽度或高度，并保持宽高比
- `fill` 必须同时指定宽度和高度
- `stretch` 同时指定宽高时不保持原始宽高比

</details>

## 图像格式转换

支持 `jpg/jpeg/png/webp` 互转，输出格式由 `--to` 指定。输入仅接受 `jpg/jpeg/png/webp`；转换时 **JPEG** 的 EXIF Orientation 会应用到实际像素（WebP 携带的 orientation 元数据当前不处理），输出不保留 EXIF/GPS/XMP 等 metadata。

```bash
# 转为 WebP
./itb convert -i photo.png --to webp

# 透明 PNG 转 JPG，指定铺底颜色
./itb convert -i photo.png --to jpg --background "#FFFFFF"

# 指定输出路径
./itb convert -i photo.jpg --to png -o output.png
```

转换语义按目标格式固定：

| 目标格式 | quality | lossless | Alpha | background |
|---------|---------|----------|-------|------------|
| JPEG | 生效 | 不支持（报错） | 按背景色铺底 | 生效 |
| PNG | 忽略 | 始终无损（参数无额外影响） | 保留 | 忽略 |
| WebP | 生效（无损模式下为压缩强度） | 切换无损编码 | 保留（有损/无损均不丢失） | 忽略 |

<details>
<summary>命令参数</summary>

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-i, --input` | (必填) | 输入图片路径（仅 `jpg` / `jpeg` / `png` / `webp`） |
| `-o, --output` | `*_converted.<ext>` | 输出路径 |
| `--to` | (必填) | 目标格式：`jpg` / `jpeg` / `png` / `webp` |
| `-q, --quality` | `80` | JPEG/WebP 输出质量；WebP 无损模式下表示压缩强度，PNG 忽略该参数 |
| `--lossless` | `false` | 使用 WebP 无损编码；PNG 始终为无损格式，该参数对 PNG 无额外影响 |
| `--background` | `#FFFFFF` | 输出 JPEG 时透明区域使用的背景色（必须为不透明颜色） |

</details>

## 图像水印

为图片添加文字或图片水印；文字水印支持两种模式：位置水印（单点）和重复平铺水印，图片水印仅支持位置水印。

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
| `--color` | (自动) | 水印颜色（`#RGB` / `#RRGGBB` / `#RRGGBBAA`）；空则自动选择黑/白 |
| `--opacity` | `0.5` | 透明度，范围 0~1 |
| `--font-size` | `0` | 字体大小，`0` 表示根据图片自动计算，上限 4096 |
| `--font` | (自动) | 字体文件路径，空则自动使用可用的默认字体 |
| `--image` | 与 `--text` 二选一 | 图片水印路径 |
| `--scale` | `0.2` | 图片水印缩放比例，基于底图短边 |

**position 模式参数**

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--position` | `bottom-right` | 水印位置：`bottom-right` / `bottom-left` / `top-right` / `top-left` / `center` |
| `--margin` | `0.04` | 边距比例，基于图片短边计算，不能为负 |

**repeat 模式参数**

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--angle` | `30` | 旋转角度（度），范围 -360~360 |
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
./itb inspect -i photo.jpg --no-detail

# 不计算 hash
./itb inspect -i photo.jpg --no-hash
```

<details>
<summary>命令参数</summary>

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-i, --input` | (必填) | 输入图片路径 |
| `--format` | `table` | 输出格式：`table` / `json` / `plain`（`plain` 仅输出 SHA-256） |
| `--no-detail` | `false` | 不输出详细元数据（优先于 `--detail`） |
| `--detail` | `true` | 兼容保留，等价于不传 `--no-detail` |
| `--no-hash` | `false` | 不计算文件 hash |
| `--strict` | `false` | 图像解析失败时直接返回错误 |

</details>

## HTTP API（itb serve）

除本地 CLI 外，`itb serve` 提供 `/api/v1` HTTP API，直接调用领域包而不执行 CLI 子进程。API 面向可信的个人 VPS 部署，不提供 WebUI、S3 管理、工作流或用户系统。

```bash
# 启动 HTTP API（默认 http://127.0.0.1:8080）
./itb serve

# 指定监听地址
./itb serve --addr 127.0.0.1:9000

```

### 功能范围

- **图片操作**：`compress`、`resize`、`crop`、`convert`、`watermark`、`inspect`
- **不提供**：S3 管理、WebUI、工作流、用户系统、数据库或任务队列

### 安全边界

默认只监听 `127.0.0.1`。`ITB_API_TOKEN` 是生产环境必需的 Bearer Token；生产部署应由 Nginx/Caddy 在 HTTPS 下反向代理至 localhost。仅本地开发可以使用 `--no-auth`，且它只能绑定 loopback 地址。

<details>
<summary>命令参数与 HTTP API</summary>

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--addr` | `127.0.0.1:8080` | 监听地址 |
| `--max-upload` | `64MiB` | 最大 multipart 请求大小 |
| `--max-pixels` | `50000000` | 最大图片像素数（含上传图片、水印图与计划输出尺寸） |
| `--max-dimension` | `16384` | 最大图片单边尺寸（含上传图片、水印图与计划输出尺寸） |
| `--max-concurrent` | `2` | 最大并发图片操作数 |
| `--max-working-bytes` | `512MiB` | 单个操作中间画布内存上限（watermark 等） |
| `--timeout` | `2m` | 单个图片操作超时 |
| `--no-auth` | `false` | 仅 loopback 本地开发时禁用认证 |

API 统一前缀 `/api/v1`，例如健康检查：

```bash
curl http://127.0.0.1:8080/api/v1/health
```

图片处理端点使用与 CLI long flag 同名的 `multipart/form-data` 字段，处理结果以流式二进制响应返回。完整参数与部署说明见 [API 文档](docs/api.md) 和 [VPS 部署文档](docs/deployment.md)。

```bash
curl -H "Authorization: Bearer $ITB_API_TOKEN" \
  -F 'input=@photo.png' \
  -F 'to=webp' \
  -F 'quality=80' \
  https://itb.example.com/api/v1/convert -o photo.webp
```

</details>

<details>
<summary>从源码构建</summary>

```bash
make build    # 编译 itb
make serve    # 构建并启动 HTTP API
make check    # go vet
make test     # go test
```

</details>

## S3 兼容存储操作

支持 AWS S3、MinIO、阿里云 OSS、腾讯云 COS 等所有 S3 协议兼容的存储服务。

### 环境变量

```bash
ITB_S3_ENDPOINT           # S3 端点 URL
ITB_S3_ACCESS_KEY_ID      # Access Key ID
ITB_S3_SECRET_ACCESS_KEY  # Secret Access Key
ITB_S3_SESSION_TOKEN      # 临时凭证 Session Token（可选）
ITB_S3_REGION             # 区域（默认 us-east-1）
ITB_S3_BUCKET             # 存储桶名称（可省略 -b）
ITB_S3_FORCE_PATH_STYLE   # 强制路径样式 URL（true/false）
```

配置优先级：CLI flag > `ITB_S3_*` 环境变量 > 默认值；环境变量可满足 `--endpoint` / `--access-key` / `--secret-key` / `--bucket` 的必填校验。

临时凭证（AccessKey + SecretKey + SessionToken）建议全部通过环境变量注入，
避免 Session Token 进入 shell history。

<details>
<summary>公共参数</summary>

所有 S3 子命令共享以下参数：

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-e, --endpoint` | (必填) | S3 端点 URL（或 `ITB_S3_ENDPOINT`） |
| `-a, --access-key` | (必填) | Access Key ID（或 `ITB_S3_ACCESS_KEY_ID`） |
| `-s, --secret-key` | (必填) | Secret Access Key（或 `ITB_S3_SECRET_ACCESS_KEY`） |
| `--session-token` | (空) | 临时凭证 Session Token（或 `ITB_S3_SESSION_TOKEN`，建议用环境变量） |
| `-r, --region` | `us-east-1` | 区域 |
| `-b, --bucket` | (必填) | 存储桶名称（或 `ITB_S3_BUCKET`） |
| `--force-path-style` | `false` | 强制路径样式 URL（MinIO 需要；或 `ITB_S3_FORCE_PATH_STYLE`；loopback / `:9000` 端点自动启用） |

</details>

### 上传文件

```bash
# 上传文件到存储桶
./itb s3 upload -i photo.jpg -b my-bucket -e http://localhost:9000

# 指定对象键名（默认使用文件名）
./itb s3 upload -i photo.jpg -b my-bucket -k images/photo.jpg

# 指定 Content-Type
./itb s3 upload -i data.json -b my-bucket --content-type application/json

# 写入用户 metadata（key=value，可重复；键转小写，itb-sha256 为保留键）
./itb s3 upload -i image.webp -b my-bucket -k image/xx.webp \
  --metadata source-sha256=abc123 --metadata width=1920 --metadata height=1080

# 设置标准 HTTP 响应头（稳定 URL 发布）
./itb s3 upload -i image.webp -b my-bucket --cache-control no-cache

# 同名对象已存在即跳过（1 次 HEAD 代替整文件上传）
./itb s3 upload -i photo.jpg -b my-bucket --skip-existing

# 内容一致才跳过（比对 itb-sha256 metadata，不依赖 ETag）
./itb s3 upload -i photo.jpg -b my-bucket --skip-unchanged

# PUT 后追加 1 次 HEAD，校验远端属性与本次上传一致
./itb s3 upload -i photo.jpg -b my-bucket --verify
```

上传时会把本地文件的 SHA-256 写入对象用户 metadata（`x-amz-meta-itb-sha256`），
`--skip-unchanged` 依赖该值判断远端对象与本地是否一致；默认行为仍是无条件覆盖。
`--skip-existing` 与 `--skip-unchanged` 是互斥的上传策略，同时使用会报参数错误。

`--metadata` 必须是 `key=value` 格式（可重复指定），键统一转小写、不可为空、
禁含控制字符，重复键与保留键 `itb-sha256` 会报参数错误（在任何网络请求之前）。

`--content-type` 缺省时按**文件内容**检测 MIME（magic sniff 覆盖 JPEG/PNG/GIF/
WebP/PDF/ZIP/HTML/JSON/SVG），扩展名仅在内容无法识别时兜底——HTML 错误页改名成
`error.jpg` 会以 `text/html` 上传，而不是 `image/jpeg`。

<details>
<summary>upload 参数</summary>

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-i, --input` | (必填) | 本地文件路径 |
| `-k, --key` | 文件名 | 对象键名 |
| `--content-type` | 内容检测 | 内容类型（显式指定原样生效） |
| `--metadata` | (空) | 对象用户 metadata `KEY=VALUE`（可重复） |
| `--cache-control` | (空) | Cache-Control 响应头（如 `no-cache`、`max-age=31536000`） |
| `--content-disposition` | (空) | Content-Disposition 响应头 |
| `--content-encoding` | (空) | Content-Encoding 响应头 |
| `--skip-existing` | `false` | 对象键已存在即跳过上传 |
| `--skip-unchanged` | `false` | 内容一致才跳过上传（比对 itb-sha256 metadata） |
| `--verify` | `false` | PUT 后追加 1 次 HEAD，校验远端 size/Content-Type/HTTP 头/metadata 与本次上传一致 |

</details>

`--verify` 的请求契约：默认上传 `PUT`；`--verify` 为 `PUT → HEAD`；
`--skip-existing` 命中为单次 `HEAD`，未命中加 `--verify` 为
`HEAD → PUT → HEAD`；`--skip-unchanged` 命中为单次 `HEAD`。
HEAD 校验只能证明 header/metadata 与预期一致，**不等于** body SHA-256
校验；body 完整性校验由 `download --verify` / `--verify-sha256` 承担。

### 下载文件

```bash
# 下载文件
./itb s3 download -b my-bucket -k photo.jpg -o ./photo.jpg

# 未指定 -o 时保存到当前目录，文件名取对象键最后一段（photo.jpg）
./itb s3 download -b my-bucket -k images/photo.jpg
```

<details>
<summary>download 参数</summary>

| 参数 | 说明 |
|------|------|
| `-k, --key` | 对象键名（必填） |
| `-o, --output` | 本地输出路径（默认保存到当前目录，文件名取对象键最后一段） |

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
