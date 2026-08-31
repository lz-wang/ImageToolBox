# Go Image Toolbox

> **中文文档**：[README.md](./README.md)

> External dependencies are documented in [docs/build-bins.md](docs/build-bins.md).
> 
> CI builds the following platforms in parallel:
> 
> - macOS amd64 / arm64
> - Linux amd64 / arm64
> - Windows amd64 / arm64
> 
> Release artifacts: macOS / Linux are packaged as `.tar.gz`, Windows as `.zip`; Windows executables and the bundled compression tools all carry the `.exe` extension.

## Linux compatibility

The `compress` feature in official Linux amd64 / arm64 builds requires **glibc >= 2.28**. Other Go-implemented features are not rejected at startup when this requirement is absent. Alpine Linux / musl is not currently supported.

| System | `compress` support |
|--------|--------------------|
| CentOS 7 (glibc 2.17) | ❌ |
| Ubuntu 18.04 (glibc 2.27) | ❌ |
| Rocky / AlmaLinux 8 (glibc 2.28) | ✅ |
| Debian 10+ | ✅ |
| Ubuntu 20.04+ | ✅ |

CI builds bundled Linux compressors in `manylinux_2_28` and checks both their highest `GLIBC_*` symbol version and dynamic-library dependencies. Release archives contain only `itb`; the compressors remain embedded and are extracted on demand into the user cache.

## Install

After a new version is released, install it with Homebrew (macOS / Linux):

```bash
brew tap lz-wang/tap
brew install lz-wang/tap/itb
```

After installing, verify with `itb --version` (or `itb version`).

> [!WARNING]
> **macOS runtime note**
>
> If running the binary on macOS reports "cannot verify developer", and you have to manually allow it under "Security & Privacy" every time, for internal use you can strip the `quarantine` attribute right after downloading or extracting:
>
> ```bash
> xattr -d com.apple.quarantine your_binary
> ```

## Compress images

Auto-detects the image format (PNG/JPEG) and compresses it, keeping the original file by default:

```bash
# Compress a PNG (writes photo_compressed.png)
./itb compress -i photo.png

# Compress a JPEG (writes photo_compressed.jpg)
./itb compress -i photo.jpg

# Specify the output file
./itb compress -i photo.png -o compressed.png

# Overwrite the original (mutually exclusive with --output)
./itb compress -i photo.jpg --in-place

# Specify compression quality (1-100, default 80)
./itb compress -i photo.jpg -q 90
```

<details>
<summary>Options & compression pipeline</summary>

| Option | Default | Description |
|------|--------|------|
| `-i, --input` | (required) | Input image file path |
| `-o, --output` | `*_compressed.*` | Output path; appends `_compressed` to the original name by default |
| `--in-place` | `false` | Overwrite the input file (mutually exclusive with `--output`) |
| `-q, --quality` | `80` | Compression quality 1-100 |

**Compression pipeline**

- **PNG**: `pngquant` → `oxipng` (lossy + lossless, two-stage compression)
- **JPEG**: `djpeg` → `cjpeg` (libjpeg-turbo decode + encode)

</details>

## Crop images

Keep a region of the image by anchor and percentage.

```bash
# Keep the left 40% of the width
./itb crop -i a.jpg --anchor left --width 40%

# Keep the right 40% of the width
./itb crop -i a.jpg --anchor right --width 40%

# Keep the top-left 40% x 40% region
./itb crop -i a.jpg --anchor top-left --width 40% --height 40%

# Keep the center 40% x 40% region
./itb crop -i a.jpg --anchor center --width 40% --height 40%
```

<details>
<summary>Options & rules</summary>

| Option | Default | Description |
|------|--------|------|
| `-i, --input` | (required) | Input image path |
| `-o, --output` | `*_cropped.*` | Output path; appends `_cropped` to the original name by default |
| `--anchor` | (required) | Crop anchor: `left` / `right` / `top` / `bottom` / `top-left` / `top-right` / `bottom-left` / `bottom-right` / `center` |
| `--width` | | Crop width as a percentage, e.g. `40%` |
| `--height` | | Crop height as a percentage, e.g. `40%` |

**Rules**

- Only the percentage format is supported, in the range `(0, 100]`
- `left` / `right` require `--width` and must not provide `--height`
- `top` / `bottom` require `--height` and must not provide `--width`
- `top-left` / `top-right` / `bottom-left` / `bottom-right` / `center` require both `--width` and `--height`

</details>

## Resize images

Resize by width/height, by percentage, or using different modes.

```bash
# Specify the width, scale proportionally
./itb resize -i photo.jpg --width 1200

# Specify a box, preserve aspect ratio (fit)
./itb resize -i photo.jpg --width 1200 --height 630 --mode fit

# Specify a box and crop to fill
./itb resize -i photo.jpg --width 1200 --height 630 --mode fill --anchor top

# Scale by percentage
./itb resize -i photo.png --percent 50%
```

<details>
<summary>Options</summary>

| Option | Default | Description |
|------|--------|------|
| `-i, --input` | (required) | Input image path |
| `-o, --output` | `*_resized.*` | Output path |
| `--width` | | Target width (pixels) |
| `--height` | | Target height (pixels) |
| `--percent` | | Scale by percentage, e.g. `50%` |
| `--mode` | `fit` | Resize mode: `fit` / `fill` / `stretch` |
| `--anchor` | `center` | Anchor for `fill` mode |
| `--filter` | `lanczos` | Resampler: `nearest` / `linear` / `catmullrom` / `lanczos` |

**Rules**

- Either `--percent` or at least one of `--width` / `--height` must be provided
- `--percent` cannot be combined with `--width` / `--height`
- `fit` accepts a single dimension and preserves the aspect ratio
- `fill` requires both width and height
- `stretch` does not preserve the aspect ratio when both dimensions are given

</details>

## Format conversion

Convert between `jpg/jpeg/png/webp`; the output format is set by `--to`.

```bash
# Convert to WebP
./itb convert -i photo.png --to webp

# Transparent PNG → JPG with a custom background color
./itb convert -i photo.png --to jpg --background "#FFFFFF"

# Specify the output path
./itb convert -i photo.jpg --to png -o output.png
```

<details>
<summary>Options</summary>

| Option | Default | Description |
|------|--------|------|
| `-i, --input` | (required) | Input image path |
| `-o, --output` | `*_converted.<ext>` | Output path |
| `--to` | (required) | Target format: `jpg` / `jpeg` / `png` / `webp` |
| `-q, --quality` | `80` | Quality for lossy formats |
| `--lossless` | `false` | Lossless encoding (webp/png) |
| `--background` | `#FFFFFF` | Background color when converting to an opaque format |

</details>

## Watermark

Add text or image watermarks to images; text watermarks support two modes — position (single point) and repeated tile — while image watermarks support the position mode only.

### Position watermark (position)

Adds a single watermark at the specified position. The text color (black/white) is chosen automatically based on background brightness, with an outline for readability.

```bash
# Default: bottom-right
./itb watermark -i photo.jpg -t "© Author"

# Specify the position
./itb watermark -i photo.png -t "Copyright" --position center

# Adjust opacity
./itb watermark -i photo.png -t "Author" --opacity 0.8

# Specify the output path
./itb watermark -i photo.jpg -t "Author" -o output.jpg

# Image watermark
./itb watermark -i photo.jpg --image logo.png --scale 0.2 --position bottom-right
```

### Repeated tile watermark (repeat)

Text is tiled across the whole image, with adjustable rotation angle and spacing.

```bash
# Basic usage
./itb watermark -i photo.png -t "WATERMARK" --mode repeat

# Custom angle and opacity
./itb watermark -i photo.png -t "DRAFT" --mode repeat --angle 45 --opacity 0.3

# Custom color
./itb watermark -i photo.png -t "CONFIDENTIAL" --mode repeat --color "#FF0000"
```

<details>
<summary>Options</summary>

**Common options**

| Option | Default | Description |
|------|--------|------|
| `-i, --input` | (required) | Input image path |
| `-t, --text` | Required unless using `--image` | Watermark text |
| `-o, --output` | `*_watermarked.*` | Output path; appends `_watermarked` to the original name by default |
| `-m, --mode` | `position` | Watermark mode: `position` / `repeat` |
| `--color` | (auto) | Watermark color, e.g. `#FF0000`; empty = auto black/white |
| `--opacity` | `0.5` | Opacity, range 0–1 |
| `--font-size` | `0` | Font size; `0` = computed from the image |
| `--font` | (auto) | Font file path; empty = auto-selects an available default font |
| `--image` | Required unless using `--text` | Image watermark path |
| `--scale` | `0.2` | Image watermark scale, relative to the shorter side of the base image |

**position mode options**

| Option | Default | Description |
|------|--------|------|
| `--position` | `bottom-right` | Position: `bottom-right` / `bottom-left` / `top-right` / `top-left` / `center` |
| `--margin` | `0.04` | Margin ratio, relative to the shorter side of the image |

**repeat mode options**

| Option | Default | Description |
|------|--------|------|
| `--angle` | `30` | Rotation angle (degrees) |
| `--space` | `0` | Tile spacing; `0` = auto-computed from font size |

</details>

## Image inspection

Read file info, basic image info, detailed metadata, and file hash.

```bash
# Default table output; computes all hashes; prints detailed data
./itb inspect -i photo.jpg

# JSON output
./itb inspect -i photo.jpg --format json

# Print only sha256
./itb inspect -i photo.jpg --format plain

# Disable detailed data
./itb inspect -i photo.jpg --no-detail

# Skip hash computation
./itb inspect -i photo.jpg --no-hash
```

<details>
<summary>Options</summary>

| Option | Default | Description |
|------|--------|------|
| `-i, --input` | (required) | Input image path |
| `--format` | `table` | Output format: `table` / `json` / `plain` (`plain` prints only the SHA-256) |
| `--no-detail` | `false` | Skip detailed metadata (takes precedence over `--detail`) |
| `--detail` | `true` | Kept for compatibility; equivalent to not passing `--no-detail` |
| `--no-hash` | `false` | Skip file hash computation |
| `--strict` | `false` | Return an error immediately if image parsing fails |

</details>

## HTTP API (itb serve)

Alongside the local CLI, `itb serve` provides a `/api/v1` HTTP API that calls domain packages directly rather than spawning CLI subprocesses. It is intended for trusted personal VPS deployments, not a WebUI, S3-management, workflow, or user-management service.

```bash
# Start the HTTP API (default http://127.0.0.1:8080)
./itb serve

# Custom listen address
./itb serve --addr 127.0.0.1:9000

```

### Feature scope

- **Image operations**: `compress`, `resize`, `crop`, `convert`, `watermark`, and `inspect`
- **Not exposed**: S3 management, WebUI, workflows, user management, databases, or job queues

### Security boundary

The server binds to `127.0.0.1` by default. `ITB_API_TOKEN` is the required production Bearer token; use Nginx/Caddy to reverse proxy HTTPS traffic to localhost. `--no-auth` is only for local development and only permits loopback addresses.

<details>
<summary>Command options and HTTP API</summary>

| Option | Default | Description |
|------|--------|------|
| `--addr` | `127.0.0.1:8080` | Listen address |
| `--max-upload` | `64MiB` | Maximum multipart request size |
| `--max-pixels` | `50000000` | Maximum image pixel count |
| `--max-dimension` | `16384` | Maximum image dimension |
| `--max-concurrent` | `2` | Maximum concurrent image operations |
| `--timeout` | `2m` | Per-image operation timeout |
| `--no-auth` | `false` | Disable authentication only for loopback development |

The API uses the `/api/v1` prefix, for example the health check:

```bash
curl http://127.0.0.1:8080/api/v1/health
```

Image endpoints accept `multipart/form-data` fields named after CLI long flags and stream binary responses. See the complete [API reference](docs/api.md) and [VPS deployment guide](docs/deployment.md).

```bash
curl -H "Authorization: Bearer $ITB_API_TOKEN" \
  -F 'input=@photo.png' \
  -F 'to=webp' \
  -F 'quality=80' \
  https://itb.example.com/api/v1/convert -o photo.webp
```

</details>

<details>
<summary>Building from source</summary>

```bash
make build    # Compile itb
make serve    # Build and start the HTTP API
make check    # go vet
make test     # go test
```

</details>

## S3-compatible storage

Supports any S3-protocol-compatible storage: AWS S3, MinIO, Alibaba Cloud OSS, Tencent Cloud COS, etc.

### Environment variables

```bash
ITB_S3_ENDPOINT           # S3 endpoint URL
ITB_S3_ACCESS_KEY_ID      # Access Key ID
ITB_S3_SECRET_ACCESS_KEY  # Secret Access Key
ITB_S3_REGION             # Region (default us-east-1)
ITB_S3_BUCKET             # Bucket name (can omit -b)
```

Priority: CLI flag > `ITB_S3_*` environment variables > defaults; environment variables satisfy the required checks for `--endpoint` / `--access-key` / `--secret-key` / `--bucket`.

<details>
<summary>Common options</summary>

All S3 subcommands share the following options:

| Option | Default | Description |
|------|--------|------|
| `-e, --endpoint` | (required) | S3 endpoint URL (or `ITB_S3_ENDPOINT`) |
| `-a, --access-key` | (required) | Access Key ID (or `ITB_S3_ACCESS_KEY_ID`) |
| `-s, --secret-key` | (required) | Secret Access Key (or `ITB_S3_SECRET_ACCESS_KEY`) |
| `-r, --region` | `us-east-1` | Region |
| `-b, --bucket` | (required) | Bucket name (or `ITB_S3_BUCKET`) |
| `--force-path-style` | `false` | Force path-style URL (required by MinIO) |

</details>

### Upload file

```bash
# Upload a file to the bucket
./itb s3 upload -i photo.jpg -b my-bucket -e http://localhost:9000

# Specify the object key (defaults to the file name)
./itb s3 upload -i photo.jpg -b my-bucket -k images/photo.jpg

# Specify the Content-Type
./itb s3 upload -i data.json -b my-bucket --content-type application/json

# Skip when an object with the same key already exists (one HEAD instead of a full upload)
./itb s3 upload -i photo.jpg -b my-bucket --skip-existing

# Skip only when content is unchanged (compares itb-sha256 metadata, not ETag)
./itb s3 upload -i photo.jpg -b my-bucket --skip-unchanged
```

Every upload stores the local file's SHA-256 in object user metadata
(`x-amz-meta-itb-sha256`), which `--skip-unchanged` compares against;
the default behavior remains unconditional overwrite. `--skip-existing`
and `--skip-unchanged` are mutually exclusive upload strategies;
combining them is rejected as a flag error.

<details>
<summary>upload options</summary>

| Option | Default | Description |
|------|--------|------|
| `-i, --input` | (required) | Local file path |
| `-k, --key` | file name | Object key |
| `--content-type` | auto-detected | Content type |
| `--skip-existing` | `false` | Skip upload when the object key already exists |
| `--skip-unchanged` | `false` | Skip upload only when content is unchanged (compares itb-sha256 metadata) |

</details>

### Download file

```bash
# Download a file
./itb s3 download -b my-bucket -k photo.jpg -o ./photo.jpg

# Without -o, saves to the current directory using the last segment of the key (photo.jpg)
./itb s3 download -b my-bucket -k images/photo.jpg
```

<details>
<summary>download options</summary>

| Option | Description |
|------|------|
| `-k, --key` | Object key (required) |
| `-o, --output` | Local output path (defaults to the current directory, file name taken from the last segment of the object key) |

</details>

### Delete object

```bash
# Delete an object (confirmation required)
./itb s3 delete -b my-bucket -k photo.jpg

# Force delete (no confirmation)
./itb s3 delete -b my-bucket -k photo.jpg -f
```

<details>
<summary>delete options</summary>

| Option | Description |
|------|------|
| `-k, --key` | Object key (required) |
| `-f, --force` | Force delete without confirmation |

</details>

### List objects

```bash
# List all objects
./itb s3 list -b my-bucket

# Filter by prefix
./itb s3 list -b my-bucket -p images/

# JSON output
./itb s3 list -b my-bucket --format json
```

<details>
<summary>list options</summary>

| Option | Default | Description |
|------|--------|------|
| `-p, --prefix` | | Object key prefix |
| `--max-keys` | `1000` | Maximum number of results |
| `--format` | `table` | Output format: `table` / `json` / `plain` |

</details>

### Stat object metadata

```bash
# Show full metadata of a single object (one HEAD request, no content transfer)
./itb s3 stat -b my-bucket -k images/photo.jpg

# JSON output
./itb s3 stat -b my-bucket -k images/photo.jpg --format json
```

stat always queries by the exact object key and never falls back to list
inference when the object does not exist. The returned metadata includes
Size, ETag, Content-Type, Storage Class, Cache-Control, Version ID and
user metadata.

<details>
<summary>stat options</summary>

| Option | Default | Description |
|------|--------|------|
| `-k, --key` | (required) | Object key |
| `--format` | `table` | Output format: `table` / `json` |

</details>

<details>
<summary>Cloud provider configuration examples</summary>

| Provider | Endpoint example | ForcePathStyle |
|---------|---------------|----------------|
| AWS S3 | `https://s3.amazonaws.com` | `false` |
| MinIO | `http://localhost:9000` | `true` |
| Alibaba Cloud OSS | `https://oss-cn-hangzhou.aliyuncs.com` | `false` |
| Tencent Cloud COS | `https://cos.ap-guangzhou.myqcloud.com` | `false` |

</details>

## License

This project is released under the MIT license. For the bundled third-party tools, see [LICENSE-THIRD-PARTY.md](./LICENSE-THIRD-PARTY.md).
