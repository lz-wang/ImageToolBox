# 外部依赖说明

本文档说明本项目使用的外部依赖、`bins/<os>-<arch>/` 目录约定，以及相关二进制的构建方式。

## 目录约定

建议按以下目录组织：

- `bins/macos-amd64/`
- `bins/macos-arm64/`
- `bins/linux-amd64/`
- `bins/linux-arm64/`
- `bins/windows-amd64/`
- `bins/windows-arm64/`

其中 Windows 平台产物统一使用 `.exe` 扩展名，例如：

- `pngquant.exe`
- `oxipng.exe`
- `cjpeg-static.exe`
- `djpeg-static.exe`

## libjpeg-turbo

- 仓库地址: <https://github.com/libjpeg-turbo/libjpeg-turbo.git>
- 当前版本: [Release 3.1.3 · libjpeg-turbo/libjpeg-turbo](https://github.com/libjpeg-turbo/libjpeg-turbo/releases/tag/3.1.3)

建议统一使用 `-DENABLE_SHARED=FALSE -DENABLE_STATIC=TRUE` 构建静态版本的 `cjpeg` / `djpeg`。

本项目仅使用以下静态工具：

- `cjpeg-static`
- `djpeg-static`

`jpegtran-static` 没有运行时使用方，因此不会构建或嵌入发布二进制。

### macOS amd64

```bash
git clone https://github.com/libjpeg-turbo/libjpeg-turbo.git
cd libjpeg-turbo

mkdir build-macos-amd64
cd build-macos-amd64

cmake .. \
  -DENABLE_SHARED=FALSE \
  -DENABLE_STATIC=TRUE \
  -DCMAKE_OSX_ARCHITECTURES=x86_64 \
  -DCMAKE_BUILD_TYPE=Release

make -j
```

### macOS arm64

```bash
git clone https://github.com/libjpeg-turbo/libjpeg-turbo.git
cd libjpeg-turbo

mkdir build-macos-arm64
cd build-macos-arm64

cmake .. \
  -DENABLE_SHARED=FALSE \
  -DENABLE_STATIC=TRUE \
  -DCMAKE_OSX_ARCHITECTURES=arm64 \
  -DCMAKE_BUILD_TYPE=Release

make -j
```

### Linux amd64 / arm64

Linux 内置压缩器的发布 ABI 基线是 **glibc 2.28**。不要在 Ubuntu runner 上直接构建它们；CI 会通过 `scripts/build-linux-bins-container.sh` 在对应的 PyPA `manylinux_2_28` 容器中完成构建：

```bash
./scripts/build-linux-bins-container.sh amd64 bins/linux-amd64
./scripts/verify-linux-abi.sh bins/linux-amd64
```

arm64 使用同样的流程：

```bash
./scripts/build-linux-bins-container.sh arm64 bins/linux-arm64
./scripts/verify-linux-abi.sh bins/linux-arm64
```

构建脚本固定 Rust 1.89.0；`pngquant` 启用 `static,z-static` feature，并且 `cjpeg-static` / `djpeg-static` 静态链接 libjpeg-turbo。ABI 校验会拒绝 `GLIBC_2.29+`，也会拒绝 `libz`、`libpng`、`liblcms`、`libstdc++` 等非基础 Linux 共享库依赖。

### 手动调试 Linux amd64

```bash
git clone https://github.com/libjpeg-turbo/libjpeg-turbo.git
cd libjpeg-turbo

mkdir build-linux-amd64
cd build-linux-amd64

cmake .. \
  -DENABLE_SHARED=FALSE \
  -DENABLE_STATIC=TRUE \
  -DCMAKE_SYSTEM_NAME=Linux \
  -DCMAKE_SYSTEM_PROCESSOR=x86_64 \
  -DCMAKE_BUILD_TYPE=Release

make -j
```

### Linux arm64

如果在 arm64 Linux 主机原生构建：

```bash
git clone https://github.com/libjpeg-turbo/libjpeg-turbo.git
cd libjpeg-turbo

mkdir build-linux-arm64
cd build-linux-arm64

cmake .. \
  -DENABLE_SHARED=FALSE \
  -DENABLE_STATIC=TRUE \
  -DCMAKE_SYSTEM_NAME=Linux \
  -DCMAKE_SYSTEM_PROCESSOR=aarch64 \
  -DCMAKE_BUILD_TYPE=Release

make -j
```

如果在其他平台交叉编译，需要额外指定 toolchain，例如：

```bash
cmake .. \
  -DENABLE_SHARED=FALSE \
  -DENABLE_STATIC=TRUE \
  -DCMAKE_SYSTEM_NAME=Linux \
  -DCMAKE_SYSTEM_PROCESSOR=aarch64 \
  -DCMAKE_TOOLCHAIN_FILE=/path/to/toolchain.cmake \
  -DCMAKE_BUILD_TYPE=Release
```

构建完成后，将对应平台产物复制到本仓库的 `bins/<os>-<arch>/` 目录，并在 [internal/compress/embed.go](/Users/lzwang/projects/ImageToolBox/internal/compress/embed.go) 中补充或校验对应平台的二进制映射。

### Windows amd64 / arm64

建议在对应架构的 Windows Runner 或主机上原生构建。CI 中当前使用 GitHub Actions Windows Runner 原生构建 `pngquant`、`oxipng` 和 `libjpeg-turbo`，并将产物放入：

- `bins/windows-amd64/`
- `bins/windows-arm64/`

`libjpeg-turbo` 在 Windows 上建议使用 CMake + Visual Studio 生成器，常见输出包括：

- `Release/cjpeg-static.exe`
- `Release/djpeg-static.exe`

发布阶段，Windows 构建产物使用 `.zip` 打包；macOS / Linux 保持 `.tar.gz`。

## pngquant

- 仓库地址: <https://github.com/kornelski/pngquant>
- 项目网站: [pngquant — lossy PNG compressor](https://pngquant.org/)
- 当前版本: 3.0.3

CI 中当前通过源码构建 `pngquant`，也可以复用 workflow 中的做法在 macOS、Linux、Windows 上手工构建。

## oxipng

- 仓库地址: <https://github.com/oxipng/oxipng.git>
- 当前版本: [Release v10.1.0 · oxipng/oxipng](https://github.com/oxipng/oxipng/releases/tag/v10.1.0)

CI 中当前通过源码构建 `oxipng`，也可以复用 workflow 中的做法在 macOS、Linux、Windows 上手工构建。
