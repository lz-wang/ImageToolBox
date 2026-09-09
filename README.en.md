# Go Image Toolbox

[![codecov](https://codecov.io/gh/lz-wang/image-tool-box/graph/badge.svg?token=UW9vZvWwxY)](https://codecov.io/gh/lz-wang/image-tool-box)

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

> **File safety:** when an explicit `<dst>` is provided, the output must not resolve to the same file as any input resource, including equivalent paths, hard links, and symbolic links. `resize`, `crop`, `rotate`, and `watermark` may derive a default destination when `[dst]` is omitted; `convert` requires `<dst>`. Use `compress --in-place` for in-place compression.

## Compress images

Auto-detects the image format (PNG/JPEG) and compresses it, keeping the original file by default:

```bash
# Compress a PNG (writes photo_compressed.png)
./itb compress photo.png

# Compress a JPEG (writes photo_compressed.jpg)
./itb compress photo.jpg

# Specify the output file
./itb compress photo.png compressed.png

# Overwrite the original (cannot be used with a destination path)
./itb compress --in-place photo.jpg

# Specify compression quality (1-100, default 80)
./itb compress -q 90 photo.jpg
```

<details>
<summary>Options & compression pipeline</summary>

| Option | Default | Description |
|------|--------|------|
| `<src>` | (required) | Input image file path |
| `[dst]` | `*_compressed.*` | Output path; appends `_compressed` to the original name when omitted |
| `--in-place` | `false` | Overwrite the input file (cannot be used with `[dst]`) |
| `-q, --quality` | `80` | Compression quality 1-100 |

**Compression pipeline**

- **PNG**: `pngquant` → `oxipng` (lossy + lossless, two-stage compression)
- **JPEG**: `djpeg` → `cjpeg` (libjpeg-turbo decode + encode)

</details>

## Crop images

Keep a region of the image by anchor and percentage.

```bash
# Keep the left 40% of the width
./itb crop --anchor left --width 40% a.jpg

# Keep the right 40% of the width
./itb crop --anchor right --width 40% a.jpg

# Keep the top-left 40% x 40% region
./itb crop --anchor top-left --width 40% --height 40% a.jpg

# Keep the center 40% x 40% region
./itb crop --anchor center --width 40% --height 40% a.jpg
```

<details>
<summary>Options & rules</summary>

| Option | Default | Description |
|------|--------|------|
| `<src>` | (required) | Input image path |
| `[dst]` | `*_cropped.*` | Output path; appends `_cropped` to the original name when omitted |
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
./itb resize --width 1200 photo.jpg

# Specify a box, preserve aspect ratio (fit)
./itb resize --width 1200 --height 630 --mode fit photo.jpg

# Specify a box and crop to fill
./itb resize --width 1200 --height 630 --mode fill --anchor top photo.jpg

# Scale by percentage
./itb resize --percent 50% photo.png
```

<details>
<summary>Options</summary>

| Option | Default | Description |
|------|--------|------|
| `<src>` | (required) | Input image path |
| `[dst]` | `*_resized.*` | Output path |
| `--width` | | Target width (pixels) |
| `--height` | | Target height (pixels) |
| `--percent` | | Scale exactly by percentage, e.g. `50%`; upscaling like `200%` works |
| `--mode` | `fit` | Resize mode: `fit` / `fill` / `stretch` |
| `--anchor` | `center` | Anchor for `fill` mode |
| `--filter` | `lanczos` | Resampler: `nearest` / `linear` / `mitchell` / `catmullrom` / `lanczos` |

**Rules**

- Either `--percent` or at least one of `--width` / `--height` must be provided
- `--percent` cannot be combined with `--width` / `--height`
- `fit` accepts a single dimension and preserves the aspect ratio
- `fill` requires both width and height
- `stretch` does not preserve the aspect ratio when both dimensions are given

**Choosing a resampler**

- `nearest`: fastest, no antialiasing; useful for pixel art and masks
- `linear`: fast bilinear interpolation with smooth output
- `mitchell`: Mitchell-Netravali cubic filter; smoother output with less ringing than Catmull-Rom
- `catmullrom`: sharp cubic filter with a good quality/performance balance
- `lanczos`: default high-quality filter for photographic images where maximum detail retention matters

</details>

## Rotate images

Rotate by any angle: positive angles rotate counter-clockwise, negative angles clockwise; exact `90/180/270` are interpolation-free, while arbitrary angles use bilinear interpolation and adjust the output canvas following imaging's rotated bounding-box rules, avoiding cropping the subject at common angles.

```bash
# 90 degrees counter-clockwise (writes photo_rotated.jpg)
./itb rotate --angle 90 photo.jpg

# 90 degrees clockwise
./itb rotate --angle -90 photo.jpg clockwise.jpg

# Arbitrary angle (canvas adjusts as needed; PNG keeps transparency)
./itb rotate --angle 45 transparent.png result.png

# Fractional angle
./itb rotate --angle 22.5 photo.webp rotated.webp
```

<details>
<summary>Options</summary>

| Option | Default | Description |
|------|--------|------|
| `<src>` | (required) | Input image file path (`jpg` / `jpeg` / `png` / `webp` only) |
| `[dst]` | `*_rotated.*` | Output path; appends `_rotated` to the original name when omitted |
| `--angle` | (required) | Rotation angle (degrees): positive = counter-clockwise, negative = clockwise; fractional values allowed, range `(-360, 360)` and must not be `0` |

</details>

Rotation semantics:

- Inputs follow the unified transform contract: JPEG/PNG/WebP only, and the JPEG EXIF orientation is normalized before the user rotation is applied
- Arbitrary angles adjust the output canvas following imaging's rotated bounding-box rules, avoiding cropping the subject at common angles; uncovered areas stay transparent for PNG/WebP and are flattened onto white for JPEG
- `<dst>` must not resolve to the same file as `<src>` (equivalent paths, hard links, and symlinks are rejected); the HTTP API exposes `rotate` as well

## Format conversion

Convert between `jpg/jpeg/png/webp`; the output format is determined only by the required `<dst>` extension. Inputs are limited to `jpg/jpeg/png/webp`; the EXIF orientation of **JPEG** inputs is applied to the actual pixels during conversion (orientation metadata embedded in WebP files is not processed), and EXIF/GPS/XMP metadata is not carried over to the output.

**Unified input format and orientation contract** (applies to `convert` / `resize` / `crop` / `rotate` / `watermark`): every transform command strictly accepts `JPEG/PNG/WebP` only (GIF/BMP/TIFF are rejected, so animated GIFs are never silently reduced to their first frame), and the **JPEG EXIF orientation is always baked into the pixels** — planning and resource admission for `resize`/`crop`/`rotate`/`watermark` use the post-rotation logical dimensions, matching the actual output.

```bash
# Convert to WebP
./itb convert photo.png photo.webp

# Transparent PNG → JPG with a custom background color
./itb convert photo.png photo.jpg --background "#FFFFFF"

# Specify the output path
./itb convert photo.jpg output.png
```

Conversion semantics are fixed per target format:

| Target | quality | lossless | Alpha | background |
|--------|---------|----------|-------|------------|
| JPEG | applies | unsupported (error) | flattened onto the background | applies |
| PNG | ignored | always lossless (no extra effect) | preserved | ignored |
| WebP | applies (compression effort in lossless mode) | switches to lossless encoding | preserved (kept in both lossy and lossless modes) | ignored |

<details>
<summary>Options</summary>

| Option | Default | Description |
|------|--------|------|
| `<src>` | (required) | Input image path (`jpg` / `jpeg` / `png` / `webp` only) |
| `<dst>` | (required) | Output path; its `.jpg` / `.jpeg` / `.png` / `.webp` extension determines the target format |
| `-q, --quality` | `80` | JPEG/WebP output quality; compression effort in lossless WebP mode; ignored for PNG |
| `--lossless` | `false` | Use lossless WebP encoding; PNG is always lossless, so this has no extra effect on PNG |
| `--background` | `#FFFFFF` | Background color for transparent areas when converting to JPEG (must be opaque) |

</details>

## Watermark

Add text or image watermarks to images; text watermarks support two modes — position (single point) and repeated tile — while image watermarks support the position mode only. For image watermarks, `<dst>` must not alias the file passed to `--image`.

### Position watermark (position)

Adds a single watermark at the specified position. The text color (black/white) is chosen automatically based on background brightness, with an outline for readability.

```bash
# Default: bottom-right
./itb watermark -t "© Author" photo.jpg

# Specify the position
./itb watermark -t "Copyright" --position center photo.png

# Adjust opacity
./itb watermark -t "Author" --opacity 0.8 photo.png

# Specify the output path
./itb watermark -t "Author" photo.jpg output.jpg

# Image watermark
./itb watermark --image logo.png --scale 0.2 --position bottom-right photo.jpg
```

### Repeated tile watermark (repeat)

Text is tiled across the whole image, with adjustable rotation angle and spacing.

```bash
# Basic usage
./itb watermark -t "WATERMARK" --mode repeat photo.png

# Custom angle and opacity
./itb watermark -t "DRAFT" --mode repeat --angle 45 --opacity 0.3 photo.png

# Custom color
./itb watermark -t "CONFIDENTIAL" --mode repeat --color "#FF0000" photo.png
```

<details>
<summary>Options</summary>

**Common options**

| Option | Default | Description |
|------|--------|------|
| `<src>` | (required) | Input image path |
| `-t, --text` | Required unless using `--image` | Watermark text |
| `[dst]` | `*_watermarked.*` | Output path; appends `_watermarked` to the original name when omitted |
| `-m, --mode` | `position` | Watermark mode: `position` / `repeat` |
| `--color` | (auto) | Watermark color (`#RGB` / `#RRGGBB` / `#RRGGBBAA`); empty = auto black/white |
| `--opacity` | `0.5` | Opacity, range 0–1 |
| `--font-size` | `0` | Font size; `0` = computed from the image; max `4096` |
| `--font` | (auto) | Font file path; empty = auto-selects an available default font |
| `--image` | Required unless using `--text` | Image watermark path |
| `--scale` | `0.2` | Image watermark scale, relative to the shorter side of the base image |

**position mode options**

| Option | Default | Description |
|------|--------|------|
| `--position` | `bottom-right` | Position: `bottom-right` / `bottom-left` / `top-right` / `top-left` / `center` |
| `--margin` | `0.04` | Margin ratio, relative to the shorter side of the image; must be `>= 0` |

**repeat mode options**

| Option | Default | Description |
|------|--------|------|
| `--angle` | `30` | Rotation angle (degrees), range `-360` to `360` |
| `--space` | `0` | Tile spacing; `0` = auto-computed from font size |

</details>

## Image quality comparison

Read-only comparison of two images using objective quality metrics (PSNR / SSIM / MS-SSIM), implemented in pure Go with no external tools.

```bash
# Default: PSNR + MS-SSIM
./itb compare original.jpg compressed.jpg

# SSIM only
./itb compare original.jpg compressed.jpg --ssim

# All metrics
./itb compare original.jpg compressed.jpg \
  --psnr --ssim --ms-ssim
```

<details>
<summary>Options</summary>

| Option | Default | Description |
|------|--------|------|
| `<src>` | (required) | Reference image |
| `<dst>` | (required) | Image to compare |
| `--psnr` | - | Compute PSNR (peak signal-to-noise ratio in dB; `+Inf` when identical) |
| `--ssim` | - | Compute SSIM (structural similarity, standard 11×11 Gaussian window) |
| `--ms-ssim` | - | Compute MS-SSIM (fixed five scales; short edge must be ≥ 161 pixels) |

</details>

> When no metric flag is given, PSNR and MS-SSIM are computed by default; once any metric flag is provided, only the explicitly selected metrics are computed.

Comparison semantics:

- Supports JPEG / PNG / WebP; JPEG EXIF orientation is normalized, so the comparison targets the actual visual pixels
- Both images must share identical logical dimensions (`1920×1080` vs `1280×720` fails immediately); there is no implicit resize / crop / pad
- `<src>` and `<dst>` are both read-only inputs; comparing a file with itself is a valid sanity check (prints `PSNR: +Inf dB`)
- SSIM requires both sides to be ≥ 11×11; MS-SSIM uses a fixed five-scale definition and requires a short edge ≥ 161 pixels — use `--psnr` or `--ssim` for smaller images
- Images with alpha use an itb-defined alpha-aware variant (premultiplied RGB plus A compared together); values are not expected to match third-party tools that compare RGB only
- Output order is fixed as PSNR, SSIM, MS-SSIM, for example:

```text
PSNR: 42.318274 dB
SSIM: 0.976391
MS-SSIM: 0.987423
```

This capability is exposed through the CLI only and is not part of the HTTP API.

## Image inspection

Read file info, basic image info, detailed metadata, and file hash.

```bash
# Default table output; computes all hashes; prints detailed data
./itb inspect photo.jpg

# JSON output
./itb inspect --format json photo.jpg

# Print only sha256
./itb inspect --format plain photo.jpg

# Disable detailed data
./itb inspect --no-detail photo.jpg

# Skip hash computation
./itb inspect --no-hash photo.jpg

# Full-decode validation (frame-by-frame for GIF); usable as an upload preflight
./itb inspect --strict --full-decode --format json image.png
```

<details>
<summary>Options</summary>

| Option | Default | Description |
|------|--------|------|
| `<src>` | (required) | Input image path |
| `--format` | `table` | Output format: `table` / `json` / `plain` (`plain` prints only the SHA-256) |
| `--no-detail` | `false` | Skip detailed metadata |
| `--detail` | `true` | Kept for compatibility (hidden from help); equivalent to not passing `--no-detail` |
| `--no-hash` | `false` | Skip file hash computation |
| `--strict` | `false` | Return an error immediately if image parsing fails |
| `--full-decode` | `false` | Fully decode the image (frame-by-frame for GIF), validating the file tail and reporting frame/animation info |

</details>

The JSON contract version is `itb.inspect.v2`. By default (header decoding)
only the image header is read, which cannot detect files whose header is fine
but whose tail is corrupted; `--full-decode` fully decodes the file and adds:

| Field | Description |
|------|------|
| `full_decode_ok` | Tri-state: omitted = not attempted; `true` = full decode passed; `false` = corrupted tail |
| `frame_count` | Frame count from full GIF decode (omitted for other formats) |
| `animation_known` | Whether `animated` is trustworthy: always `true` for JPEG/PNG; GIF requires `--full-decode`; WebP comes from the VP8X header sniff |
| `animated` | Animation state, meaningful only when `animation_known=true` |

## HTTP API (itb serve)

Alongside the local CLI, `itb serve` provides a `/api/v1` HTTP API that calls domain packages directly rather than spawning CLI subprocesses. It is intended for trusted personal VPS deployments, not a WebUI, S3-management, workflow, or user-management service.

```bash
# Start the HTTP API (default http://127.0.0.1:8080)
./itb serve

# Custom listen address
./itb serve --addr 127.0.0.1:9000

```

### Feature scope

- **Image operations**: `compress`, `resize`, `crop`, `rotate`, `convert`, `watermark`, and `inspect`
- **Not exposed**: S3 management, WebUI, workflows, user management, databases, or job queues

### Security boundary

The server binds to `127.0.0.1` by default. `ITB_API_TOKEN` is the required production Bearer token; use Nginx/Caddy to reverse proxy HTTPS traffic to localhost. `--no-auth` is only for local development and only permits loopback addresses.

<details>
<summary>Command options and HTTP API</summary>

| Option | Default | Description |
|------|--------|------|
| `--addr` | `127.0.0.1:8080` | Listen address |
| `--max-upload` | `64MiB` | Maximum multipart request size |
| `--max-pixels` | `50000000` | Maximum image pixel count (applies to uploads, watermark images, and planned output sizes) |
| `--max-dimension` | `16384` | Maximum image dimension (applies to uploads, watermark images, and planned output sizes) |
| `--max-concurrent` | `2` | Maximum concurrent image operations |
| `--max-working-bytes` | `512MiB` | Per-operation intermediate canvas memory limit (watermark, arbitrary-angle rotate, etc.) |
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

S3 object/file operands use positional arguments: `upload <src> [key]`, `download <key> [dst]`, `stat/delete <key>`, and `list [prefix]`. Connection settings and execution behavior remain flags or `ITB_S3_*` environment variables.

Supports any S3-protocol-compatible storage: AWS S3, MinIO, Alibaba Cloud OSS, Tencent Cloud COS, etc.

Output convention: **stdout carries formal results only** (switch with
`--format table|json`; upload/download/stat/list JSON all carry a `schema_version`
contract — list uses `itb.s3.list.v2`), while
progress hints and diagnostics go to **stderr** — scripts can safely pipe stdout JSON.

MinIO compatibility is continuously verified in CI: a real MinIO container
started via `docker run` in a step (GitHub Actions service containers do
not support container commands, so `server /data` cannot be passed) runs
the integration test (`internal/s3/minio_test.go`) covering
upload / stat / download / skip-existing / skip-unchanged / metadata /
cache-control / overwrite / verify / delete, and path-style addressing.
CI sets `ITB_REQUIRE_MINIO=1` (strict mode: the test fails instead of
skipping when MinIO is unavailable). Local `go test` skips it when MinIO
is unreachable; point `ITB_TEST_MINIO_ENDPOINT` (and friends) at your own
instance to run it.

### Environment variables

```bash
ITB_S3_ENDPOINT           # S3 endpoint URL
ITB_S3_ACCESS_KEY_ID      # Access Key ID
ITB_S3_SECRET_ACCESS_KEY  # Secret Access Key
ITB_S3_SESSION_TOKEN      # Session token for temporary credentials (optional)
ITB_S3_REGION             # Region (default us-east-1)
ITB_S3_BUCKET             # Bucket name (can omit -b)
ITB_S3_FORCE_PATH_STYLE   # Force path-style URL (true/false)
```

Priority: CLI flag > `ITB_S3_*` environment variables > defaults; environment variables satisfy the required checks for `--endpoint` / `--access-key` / `--secret-key` / `--bucket`.

For temporary credentials (access key + secret key + session token), prefer
injecting them all via environment variables so the session token never
lands in your shell history.

<details>
<summary>Common options</summary>

All S3 subcommands share the following options:

| Option | Default | Description |
|------|--------|------|
| `-e, --endpoint` | (required) | S3 endpoint URL (or `ITB_S3_ENDPOINT`) |
| `-a, --access-key` | (required) | Access Key ID (or `ITB_S3_ACCESS_KEY_ID`) |
| `-s, --secret-key` | (required) | Secret Access Key (or `ITB_S3_SECRET_ACCESS_KEY`) |
| `--session-token` | (empty) | Session token for temporary credentials (or `ITB_S3_SESSION_TOKEN`; prefer the env var) |
| `-r, --region` | `us-east-1` | Region |
| `-b, --bucket` | (required) | Bucket name (or `ITB_S3_BUCKET`) |
| `--force-path-style` | `false` | Force path-style URL (required by MinIO; or `ITB_S3_FORCE_PATH_STYLE`; auto-enabled for loopback / `:9000` endpoints) |

</details>

### Upload file

```bash
# Upload a file to the bucket
./itb s3 upload -b my-bucket -e http://localhost:9000 photo.jpg

# Specify the object key (defaults to the file name)
./itb s3 upload -b my-bucket photo.jpg images/photo.jpg

# Specify the Content-Type
./itb s3 upload -b my-bucket --content-type application/json data.json

# Attach user metadata (key=value, repeatable; keys are lowercased, itb-sha256 is reserved)
./itb s3 upload -b my-bucket image.webp image/xx.webp \
  --metadata source-sha256=abc123 --metadata width=1920 --metadata height=1080

# Set standard HTTP response headers (stable-URL publishing)
./itb s3 upload -b my-bucket --cache-control no-cache image.webp

# Skip when an object with the same key already exists (one HEAD instead of a full upload)
./itb s3 upload -b my-bucket --skip-existing photo.jpg

# Skip only when content is unchanged (compares itb-sha256 metadata, not ETag)
./itb s3 upload -b my-bucket --skip-unchanged photo.jpg

# Follow the PUT with one HEAD to verify the stored object matches this upload
./itb s3 upload -b my-bucket --verify photo.jpg
```

Every upload stores the local file's SHA-256 in object user metadata
(`x-amz-meta-itb-sha256`), which `--skip-unchanged` compares against;
the default behavior remains unconditional overwrite. `--skip-existing`
and `--skip-unchanged` are mutually exclusive upload strategies;
combining them is rejected as a flag error.

`--metadata` entries must be `key=value` (repeatable); keys are lowercased,
must be non-empty, and may not contain control characters. Duplicate keys
and the reserved key `itb-sha256` are rejected before any network request.

When `--content-type` is omitted, the MIME type is detected from the **file
content** (magic sniffing covers JPEG/PNG/GIF/WebP/PDF/ZIP/HTML/JSON/SVG);
the extension is only a fallback — an HTML error page renamed to `error.jpg`
uploads as `text/html`, not `image/jpeg`.

<details>
<summary>upload options</summary>

| Option | Default | Description |
|------|--------|------|
| `<src>` | (required) | Local file path |
| `[key]` | file name | Object key |
| `--content-type` | content-detected | Content type (explicit value is used verbatim) |
| `--metadata` | (empty) | Object user metadata `KEY=VALUE` (repeatable) |
| `--cache-control` | (empty) | Cache-Control response header (e.g. `no-cache`, `max-age=31536000`) |
| `--content-disposition` | (empty) | Content-Disposition response header |
| `--content-encoding` | (empty) | Content-Encoding response header |
| `--skip-existing` | `false` | Skip upload when the object key already exists |
| `--skip-unchanged` | `false` | Skip upload only when content is unchanged (compares itb-sha256 metadata) |
| `--verify` | `false` | After PUT, issue one HEAD to verify remote size/Content-Type/HTTP headers/metadata match this upload |
| `--format` | `table` | Output format: `table` / `json` (JSON contract `itb.s3.upload.v1`) |

</details>

Request contract of `--verify`: a plain upload is `PUT`; `--verify` makes it
`PUT → HEAD`; a `--skip-existing` hit is a single `HEAD` (a miss with
`--verify` is `HEAD → PUT → HEAD`); a `--skip-unchanged` hit is a single
`HEAD`. The HEAD check only proves the stored headers/metadata match — it is
**not** a body SHA-256 check; body integrity is covered by
`download --verify` / `--verify-sha256`.

Field semantics of a skipped JSON result: on a `--skip-existing` hit, `size`
carries the local input file's byte count (`sha256` was not computed and is
empty); on a `--skip-unchanged` hit, both `size` and `sha256` carry the exact
local file values.

### Download file

```bash
# Download a file
./itb s3 download -b my-bucket photo.jpg ./photo.jpg

# Without [dst], saves to the current directory using the last segment of the key (photo.jpg)
./itb s3 download -b my-bucket images/photo.jpg

# Verify while downloading (reads the object's itb-sha256 metadata, single pass)
./itb s3 download -b my-bucket --verify photo.jpg

# Verify against a known hash (provider-neutral integrity check; combinable with --verify)
./itb s3 download -b my-bucket --verify-sha256 "$SOURCE_SHA256" sha256/xxx /tmp/original.png
```

Downloads stream into a temp file in the output directory and rename into place
on success; any failure (network interruption, write error, checksum mismatch)
removes the temp file, so the target path never holds a partial file. A hash
mismatch fails with `ErrChecksumMismatch` — this is the real body-integrity
check (upload `--verify` only checks headers/metadata). `--verify-sha256` must
be a valid SHA-256 digest (64 hex characters / 32 bytes); anything else fails
with `ErrInvalidSHA256` before any network request is made.

<details>
<summary>download options</summary>

| Option | Description |
|------|------|
| `<key>` | Object key (required) |
| `[dst]` | Local output path (defaults to the current directory, file name taken from the last segment of the object key) |
| `--verify` | Read the object's itb-sha256 metadata and compare the SHA-256 computed while streaming |
| `--verify-sha256` | Expected SHA-256 (64 hex characters), independent of object metadata |
| `--format` | Output format: `table` / `json` (JSON contract `itb.s3.download.v1`) |

</details>

### Delete object

```bash
# Delete an object (confirmation required)
./itb s3 delete -b my-bucket photo.jpg

# Force delete (no confirmation)
./itb s3 delete -b my-bucket -f photo.jpg
```

<details>
<summary>delete options</summary>

| Option | Description |
|------|------|
| `<key>` | Object key (required) |
| `-f, --force` | Force delete without confirmation |

</details>

### List objects

```bash
# List all objects
./itb s3 list -b my-bucket

# Filter by prefix
./itb s3 list -b my-bucket images/

# JSON output
./itb s3 list -b my-bucket --format json

# Full pagination: keep paginating until the listing is complete
./itb s3 list -b my-bucket image/ --all --format json

# Control the per-request page size and the total output limit
./itb s3 list -b my-bucket image/ --page-size 500 --limit 5000 --format json

# Resume a previous listing from its continuation token
./itb s3 list -b my-bucket image/ --continuation-token TOKEN --format json
```

By default list requests a single page (`MaxKeys` up to 1000, matching the
v0.9.x single-page behavior); only `--all` keeps paginating until the
traversal finishes.

The JSON contract version is `itb.s3.list.v2` (as of v2 the JSON output is a
structured object instead of a bare object array):

```json
{
  "schema_version": "itb.s3.list.v2",
  "bucket": "my-bucket",
  "prefix": "image/",
  "complete": true,
  "count": 2,
  "pages": 1,
  "next_continuation_token": "...",
  "objects": [
    {"key": "image/a.png", "size": 123, "last_modified": "...", "etag": "\"...\"", "storage_class": "STANDARD"}
  ]
}
```

`complete=true` only means **the traversal from this run's starting token
finished normally**:

- When a single-page listing has more server-side pages, or `--limit`
  truncates the output, `complete=false` and `next_continuation_token` is
  returned for `--continuation-token` resumption;
- `--limit` truncation happens on S3 request boundaries (the final request's
  `MaxKeys` shrinks to the remaining quota), so resuming never skips objects;
- If the server reports more objects but the continuation token is missing,
  repeated, or not advancing, the whole command fails with
  `E_INCOMPLETE_LIST` instead of returning a partial result;
- If any middle page request fails, the whole command fails and no partial
  JSON is emitted either.

<details>
<summary>list options</summary>

| Option | Default | Description |
|------|--------|------|
| `[prefix]` | | Object key prefix |
| `--page-size` | `1000` | `MaxKeys` per ListObjectsV2 request (1-1000); `--max-keys` is kept as a v0.9.x alias |
| `--all` | `false` | Keep paginating until the listing is complete (default: one page only) |
| `--limit` | `0` | Total object output limit (`0` = unlimited); truncated results set `complete=false` and carry a resumption token |
| `--continuation-token` | (empty) | Resume a previous listing from its token |
| `--format` | `table` | Output format: `table` / `json` / `plain` (JSON contract `itb.s3.list.v2`) |

</details>

### Stat object metadata

```bash
# Show full metadata of a single object (one HEAD request, no content transfer)
./itb s3 stat -b my-bucket images/photo.jpg

# JSON output
./itb s3 stat -b my-bucket --format json images/photo.jpg
```

stat always queries by the exact object key and never falls back to list
inference when the object does not exist. The returned metadata includes
Size, ETag, Content-Type, Storage Class, Cache-Control, Version ID and
user metadata.

`--format json` carries the machine-readable contract version
`schema_version: itb.s3.stat.v1`:

```json
{
  "schema_version": "itb.s3.stat.v1",
  "key": "...",
  "size": 123,
  "content_type": "...",
  "metadata": {}
}
```

Scripts should branch on `schema_version` instead of parsing terminal text.

<details>
<summary>stat options</summary>

| Option | Default | Description |
|------|--------|------|
| `<key>` | (required) | Object key |
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
