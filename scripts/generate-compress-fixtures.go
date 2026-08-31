package main

import (
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) != 2 {
		panic("usage: generate-compress-fixtures <output-dir>")
	}

	image := image.NewRGBA(image.Rect(0, 0, 2, 2))
	image.Set(0, 0, color.RGBA{R: 255, A: 255})
	image.Set(1, 0, color.RGBA{G: 255, A: 255})
	image.Set(0, 1, color.RGBA{B: 255, A: 255})
	image.Set(1, 1, color.RGBA{R: 255, G: 255, B: 255, A: 255})

	writePNG(filepath.Join(os.Args[1], "sample.png"), image)
	writeJPEG(filepath.Join(os.Args[1], "sample.jpg"), image)
}

func writePNG(path string, image image.Image) {
	file, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	if err := png.Encode(file, image); err != nil {
		file.Close()
		panic(err)
	}
	if err := file.Close(); err != nil {
		panic(err)
	}
}

func writeJPEG(path string, image image.Image) {
	file, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	if err := jpeg.Encode(file, image, &jpeg.Options{Quality: 90}); err != nil {
		file.Close()
		panic(err)
	}
	if err := file.Close(); err != nil {
		panic(err)
	}
}
