# bins/

原生压缩工具（pngquant、oxipng、libjpeg-turbo 的 cjpeg/djpeg）按平台子目录放置，由 `main.go` 的 `//go:embed bins/**` 嵌入二进制：

```
bins/<os>-<arch>/pngquant[.exe]
bins/<os>-<arch>/oxipng[.exe]
bins/<os>-<arch>/cjpeg-static[.exe]
bins/<os>-<arch>/djpeg-static[.exe]
```

平台子目录不入库：CI（`.github/workflows/build-binaries.yml` 与 `release.yml`）会按 `docs/build-bins.md` 从源码构建并注入；本地开发如需完整压缩功能，参照该文档自行构建。

本文件是 `go:embed bins/**` 的兜底匹配文件（类似 `web/dist/.placeholder`）：保证全新 checkout 在未构建原生工具时 `go build ./...` 也能通过。**必须保留在 git 中，运行时不读取本文件。**
