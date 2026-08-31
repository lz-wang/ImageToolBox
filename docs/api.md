# HTTP API

`itb serve` exposes a trusted, versioned image-processing API at `/api/v1`.
It is not a remote shell: S3 management, workflows, user management, queues, and WebUI endpoints are intentionally absent.

All image operations require `Authorization: Bearer $ITB_API_TOKEN`. `GET /api/v1/health` does not require authentication and returns `{"status":"ok"}`.

Every operation uses `multipart/form-data`. `input` is the required source image file; all other field names match the corresponding CLI long flag. Image-transforming endpoints stream binary downloads with `Content-Disposition`, `X-ITB-Input-Size`, `X-ITB-Output-Size`, and `X-ITB-Operation` headers. `inspect` always returns JSON.

Scalar fields are limited to 4 KiB (16 KiB for `text`); an oversized field is rejected with `413 payload_too_large`. Uploaded files are stored under server-generated temporary names — client filenames are only used for download names — so identical filenames for `input` and `image` never collide.

## Operations

| Endpoint | Multipart fields |
| --- | --- |
| `POST /api/v1/compress` | `input`, `quality` (default `80`) |
| `POST /api/v1/resize` | `input`, `width`, `height`, `percent`, `mode`, `anchor`, `filter` |
| `POST /api/v1/crop` | `input`, `anchor`, `width`, `height` |
| `POST /api/v1/convert` | `input`, `to`, `quality`, `lossless`, `background` |
| `POST /api/v1/watermark` | `input`, `text`, `image`, `mode`, `color`, `space`, `angle`, `opacity`, `font`, `font-size`, `position`, `margin`, `scale` |
| `POST /api/v1/inspect` | `input`, `detail`, `no-detail`, `no-hash`, `strict` |

`inspect` never requires a decodable image: with `strict` unset (or `false`) it returns file metadata plus an `error` object for undecodable inputs; with `strict=true` decoding failures return `400`.

`width`, `height`, `quality`, `space`, `angle`, and `font-size` are integers. `opacity`, `margin`, and `scale` are floating-point numbers. Boolean fields accept the standard Go boolean forms, including `true`, `false`, `1`, and `0`.

Unknown fields, duplicate fields, and legacy `file`, `watermark`, or `options` fields return `400`; they are not silently ignored.

### Examples

```bash
export ITB_API_TOKEN='replace-with-a-long-random-token'
API=https://itb.example.com/api/v1

curl -H "Authorization: Bearer $ITB_API_TOKEN" \
  -F 'input=@photo.jpg' \
  -F 'width=1920' \
  -F 'height=1080' \
  -F 'mode=fill' \
  -F 'anchor=center' \
  -F 'filter=lanczos' \
  "$API/resize" -o result.jpg

curl -H "Authorization: Bearer $ITB_API_TOKEN" \
  -F 'input=@photo.png' \
  -F 'to=webp' \
  -F 'quality=80' \
  "$API/convert" -o photo.webp

curl -H "Authorization: Bearer $ITB_API_TOKEN" \
  -F 'input=@photo.jpg' \
  -F 'text=Confidential' \
  -F 'mode=repeat' \
  -F 'opacity=0.35' \
  "$API/watermark" -o watermarked.jpg

curl -H "Authorization: Bearer $ITB_API_TOKEN" \
  -F 'input=@photo.jpg' \
  -F 'detail=true' \
  "$API/inspect"
```

## Errors and limits

Errors use one stable JSON shape:

```json
{
  "error": {
    "code": "invalid_argument",
    "message": "quality must be between 1 and 100"
  }
}
```

Codes are `invalid_argument`, `missing_input`, `unsupported_format`, `payload_too_large`, `image_too_large`, `unauthorized`, `busy`, `timeout`, `not_found`, `method_not_allowed`, and `internal_error`. Router-level errors (unknown routes, wrong methods) use the same JSON shape.

The default limits are a 64 MiB multipart request, 50,000,000 pixels, a 16,384 px maximum dimension, two concurrent image operations, and a two-minute operation timeout. `413` indicates request or image limits, `429` indicates all operation slots are busy, and `504` indicates a timeout.

Limits apply to uploaded images and planned output dimensions where applicable: `--max-pixels` / `--max-dimension` gate the input image, the resolved resize target (including `percent` upscales and single-side `fit` outputs), and the final output. Uploaded watermark images (`image` on `watermark`) are also subject to the image limits.

Configure these when starting the service:

```bash
itb serve \
  --addr 127.0.0.1:8080 \
  --max-upload 64MiB \
  --max-pixels 50000000 \
  --max-dimension 16384 \
  --max-concurrent 2 \
  --timeout 2m
```
