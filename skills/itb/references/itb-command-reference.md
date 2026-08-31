# ITB Command Reference

Use `itb` from the current shell path, or `./itb` when the executable is in the current directory.

```bash
itb --help
itb --version
itb version
```

## Compress

Compress PNG/JPEG. By default the input is kept and the result is written next to it as `<name>_compressed.<ext>`; pass `--in-place` to overwrite the input.

```bash
itb compress -i photo.png
itb compress -i photo.png -o compressed.png
itb compress -i photo.jpg -o compressed.jpg -q 90
itb compress -i photo.jpg --in-place
```

Flags:

- `-i, --input`: input image path.
- `-o, --output`: output image path; default adds `_compressed`.
- `--in-place`: overwrite the input file; mutually exclusive with `--output`.
- `-q, --quality`: quality `1-100`, default `80`.

Pipeline:

- PNG: `pngquant` then `oxipng`.
- JPEG: `djpeg` then `cjpeg` from libjpeg-turbo.

## Crop

Crop by anchor and percentage.

```bash
itb crop -i a.jpg -o left.jpg --anchor left --width 40%
itb crop -i a.jpg -o center.jpg --anchor center --width 40% --height 40%
```

Flags:

- `-i, --input`: input image path.
- `-o, --output`: output path; default adds `_cropped`.
- `--anchor`: `left`, `right`, `top`, `bottom`, `top-left`, `top-right`, `bottom-left`, `bottom-right`, `center`.
- `--width`: crop width percentage, e.g. `40%`.
- `--height`: crop height percentage, e.g. `40%`.

Rules:

- Percentages must be `(0, 100]`.
- `left` / `right` require `--width` only.
- `top` / `bottom` require `--height` only.
- Corner and `center` anchors require both `--width` and `--height`.

## Resize

Resize by width, height, bounding box, fill crop, stretch, or percent.

```bash
itb resize -i photo.jpg -o wide.jpg --width 1200
itb resize -i photo.jpg -o social.jpg --width 1200 --height 630 --mode fit
itb resize -i photo.jpg -o filled.jpg --width 1200 --height 630 --mode fill --anchor top
itb resize -i photo.png -o half.png --percent 50%
```

Flags:

- `-i, --input`: input image path.
- `-o, --output`: output path; default adds `_resized`.
- `--width`, `--height`: target dimensions.
- `--percent`: scale percentage, e.g. `50%`.
- `--mode`: `fit`, `fill`, `stretch`; default `fit`.
- `--anchor`: fill-mode anchor; default `center`.
- `--filter`: `nearest`, `linear`, `catmullrom`, `lanczos`; default `lanczos`.

## Convert

Convert between `jpg`, `jpeg`, `png`, and `webp`.

```bash
itb convert -i photo.png -o photo.webp --to webp -q 85
itb convert -i transparent.png -o flat.jpg --to jpg --background "#FFFFFF"
itb convert -i photo.jpg -o photo.png --to png
```

Flags:

- `-i, --input`: input image path.
- `-o, --output`: output path; default adds `_converted.<ext>`.
- `--to`: required target format: `jpg`, `jpeg`, `png`, `webp`.
- `-q, --quality`: lossy quality; default `80`.
- `--lossless`: lossless encoding for webp/png.
- `--background`: background for transparent-to-opaque conversion; default `#FFFFFF`.

## Watermark

Add text or image watermarks.

Position text watermark:

```bash
itb watermark -i photo.jpg -o marked.jpg -t "Author"
itb watermark -i photo.png -o center.png -t "Copyright" --position center --opacity 0.8
```

Repeated text watermark:

```bash
itb watermark -i photo.png -o draft.png -t "DRAFT" --mode repeat --angle 45 --opacity 0.3
itb watermark -i photo.png -o red.png -t "CONFIDENTIAL" --mode repeat --color "#FF0000"
```

Image watermark:

```bash
itb watermark -i photo.jpg -o logo.jpg --image logo.png --scale 0.2 --position bottom-right
```

Flags:

- `-i, --input`: input image path.
- `-t, --text`: watermark text. Required unless using `--image`.
- `-o, --output`: output path; default adds `_watermarked`.
- `-m, --mode`: `position` or `repeat`; default `position`.
- `--color`: watermark hex color (`#RGB`/`#RRGGBB`/`#RRGGBBAA`); empty auto-selects black/white.
- `--opacity`: `0` to `1`; default `0.5`.
- `--font-size`: `0` means auto-size; max `4096`.
- `--font`: font file path; empty auto-selects an available default font.
- `--image`: image watermark path.
- `--scale`: image watermark scale based on base image short edge; default `0.2`.
- `--position`: `bottom-right`, `bottom-left`, `top-right`, `top-left`, `center`; default `bottom-right`.
- `--margin`: margin ratio based on short edge; default `0.04`; must be `>= 0`.
- `--angle`: repeat text angle; default `30`; range `-360` to `360`.
- `--space`: repeat spacing; `0` auto-calculates.

## S3-Compatible Storage

Supports AWS S3, MinIO, Alibaba OSS, Tencent COS, and other S3-compatible services.

Environment variables:

```bash
ITB_S3_ENDPOINT
ITB_S3_ACCESS_KEY_ID
ITB_S3_SECRET_ACCESS_KEY
ITB_S3_REGION
ITB_S3_BUCKET
```

Priority: CLI flag > `ITB_S3_*` environment variables > defaults. Environment variables satisfy required flags such as `--bucket`.

Common flags:

- `-e, --endpoint`: required S3 endpoint URL (or `ITB_S3_ENDPOINT`).
- `-a, --access-key`: required access key; prefer the `ITB_S3_ACCESS_KEY_ID` environment variable.
- `-s, --secret-key`: required secret key; prefer the `ITB_S3_SECRET_ACCESS_KEY` environment variable.
- `-r, --region`: default `us-east-1`.
- `-b, --bucket`: required bucket (or `ITB_S3_BUCKET`).
- `--force-path-style`: often required for MinIO.

Examples:

```bash
itb s3 upload -i photo.jpg -b my-bucket -e http://localhost:9000 --force-path-style
itb s3 upload -i photo.jpg -b my-bucket -k images/photo.jpg
itb s3 upload -i photo.jpg -b my-bucket --skip-existing
itb s3 upload -i photo.jpg -b my-bucket --skip-unchanged
itb s3 download -b my-bucket -k images/photo.jpg -o ./photo.jpg
itb s3 download -b my-bucket -k images/photo.jpg   # saves ./photo.jpg (last key segment)
itb s3 list -b my-bucket -p images/ --format json
itb s3 stat -b my-bucket -k images/photo.jpg
itb s3 stat -b my-bucket -k images/photo.jpg --format json
itb s3 delete -b my-bucket -k images/photo.jpg
itb s3 delete -b my-bucket -k images/photo.jpg -f
```

Use `s3 delete -f` only for explicitly requested non-interactive deletion.

`s3 stat` shows full metadata of one object with a single HEAD request (no
content transfer) and never falls back to list inference; prefer it over
`s3 list`/`s3 download` when only checking whether an object exists or
inspecting its metadata.

`s3 upload` stores the file's SHA-256 in `x-amz-meta-itb-sha256` metadata by
default. `--skip-existing` skips when the key exists; `--skip-unchanged`
skips only when that metadata hash matches the local file (never rely on
ETag for this). Default upload always overwrites. The two skip flags are
mutually exclusive — combining them is a flag error.

## Serve (WebUI)

Starts the local-first WebUI. It calls the same Go domain packages as the CLI (no subprocesses) and embeds the built frontend, so a single binary is all you need.

Examples:

```bash
itb serve
itb serve --addr 127.0.0.1:9000
itb serve --open
curl http://127.0.0.1:8080/api/v1/health
```

Flags:

- `--addr`: listen address; default `127.0.0.1:8080`. Keep it on loopback unless the network is trusted.
- `--open`: open the browser after startup; default `false`.

Feature scope: single-image tools (compress/resize/crop/convert/watermark/inspect).

Security notes:

- The WebUI is stateless: uploads land in per-request temp directories that are removed after the response.
- The WebUI only performs local image processing and never handles external-service credentials.
