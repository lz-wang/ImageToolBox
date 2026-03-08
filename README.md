# GO 图像工具箱

## 图像压缩

### 外部依赖

#### libjpeg-turbo 有损压缩JPG

- 仓库地址: <https://github.com/libjpeg-turbo/libjpeg-turbo.git>
- 当前版本: [Release 3.1.3 · libjpeg-turbo/libjpeg-turbo](https://github.com/libjpeg-turbo/libjpeg-turbo/releases/tag/3.1.3)

构建方式

```bash
git clone https://github.com/libjpeg-turbo/libjpeg-turbo.git
cd libjpeg-turbo

mkdir build-macos-arm64
cd build-macos-arm64

cmake .. \
  -DENABLE_SHARED=FALSE \
  -DENABLE_STATIC=TRUE \
  -DCMAKE_OSX_ARCHITECTURES=x86_64 \
  -DCMAKE_BUILD_TYPE=Release

make -j
```

#### pngquant 有损压缩PNG

- 仓库地址: <https://github.com/kornelski/pngquant>
- 项目网站: [pngquant — lossy PNG compressor](https://pngquant.org/)
- 当前版本: 3.0.3

#### oxipng 无损压缩PNG


- 仓库地址: <https://github.com/oxipng/oxipng.git>
- 当前版本: [Release v10.1.0 · oxipng/oxipng](https:-//github.com/oxipng/oxipng/releases/tag/v10.1.0)

### 使用方法

#### 构建

```bash
make build
```

#### 压缩图片

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

#### 命令参数

| 参数 | 说明 |
|------|------|
| `-i, --input` | 输入图片文件路径 |
| `-o, --output` | 输出图片文件路径（不指定则覆盖原文件） |
| `-q, --quality` | 压缩质量 1-100（默认 80） |

#### 压缩管道

- **PNG**: `pngquant` → `oxipng`（有损 + 无损双重压缩）
- **JPEG**: `djpeg` → `cjpeg`（libjpeg-turbo 解码 + 编码）

## 图像水印

### 位置水印

TODO

### 重复水印

TODO

## 图像上传

### S3上传

TODO

### LskyPro 上传

TODO
