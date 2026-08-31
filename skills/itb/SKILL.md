---
name: itb
description: Use the `itb` CLI in image-processing workflows. Trigger when a user asks to compress, crop, resize, convert, watermark, or inspect images, upload images to S3-compatible storage, launch the `itb serve` WebUI, or choose the right `itb` command/flags for an image workflow.
---

# ITB

Use this skill to turn image-processing requests into safe, concrete `itb` CLI commands.

## Core Workflow

1. Prefer an existing `itb` executable available in the current workspace or shell path.
2. Inspect the source image(s) before destructive operations. Preserve originals unless the user explicitly wants in-place mutation.
3. Choose the narrowest command:
   - `compress` for PNG/JPEG size reduction.
   - `crop` for percentage-based anchored cuts.
   - `resize` for dimensions, aspect-ratio fitting, filling, stretching, or percentage scaling.
   - `convert` for `jpg` / `jpeg` / `png` / `webp` conversion (inputs limited to these formats).
   - `watermark` for text or image watermarking.
   - `inspect` for metadata and file hash checks.
   - `s3` for S3-compatible upload/download/list/stat/delete.
   - `serve` for the interactive local WebUI (browser-based, same processing core as the CLI).
4. Prefer explicit `-o` / `--output` for single-file transformations so follow-up steps can use predictable paths.
5. For multi-step local image pipelines, write intermediate outputs to a temporary or task-specific output directory and run commands in sequence.
6. Verify outputs with file existence, dimensions/format checks, or a visual preview when the result is user-facing.

CLI vs WebUI: use CLI commands for scripted/automated work; suggest `itb serve` when the user wants to interactively browse, tweak parameters, and preview results in a browser.

## Command Use

Load `references/itb-command-reference.md` when exact flags, examples, defaults, environment variables, or cloud upload settings are needed.

Common patterns:

```bash
itb resize -i input.jpg -o output.jpg --width 1200 --mode fit
itb convert -i output.jpg -o output.webp --to webp -q 85
itb watermark -i output.webp -o marked.webp -t "Draft" --mode repeat --opacity 0.25
```

## Safety Rules

- `convert` accepts only `jpg`/`jpeg`/`png`/`webp` inputs; it preserves alpha for PNG/WebP output (lossy and lossless alike) and only flattens transparent areas onto `--background` when converting to JPEG.
- `compress` keeps the input and writes `<name>_compressed.<ext>` by default; pass `--in-place` only when the user explicitly wants to overwrite the original (`--in-place` is mutually exclusive with `-o`/`--output`).
- Treat `s3 delete` as destructive; use `-f` only when the user clearly requested non-interactive deletion.
- Do not print secrets. Prefer environment variables for `ITB_S3_*` credentials.
- Use `--force-path-style` for MinIO-style endpoints when needed.
- For `watermark`, use either text (`-t`) or image (`--image`) watermarks. Image watermarks only support `position` mode; tiled image watermarks are not supported.
- `serve` binds to `127.0.0.1` by default; never suggest `0.0.0.0` on untrusted networks.
- When a result is user-facing, confirm the expected output path exists and preview the image when practical.
