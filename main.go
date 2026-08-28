package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"os"

	"imagetoolbox/internal/cmd"
	"imagetoolbox/internal/compress"
)

//go:embed bins/**
var binaries embed.FS

//go:embed all:web/dist
var webDist embed.FS

var version = "dev"

func main() {
	compress.InitBinaries(binaries)

	staticFS, err := fs.Sub(webDist, "web/dist")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := cmd.Execute(context.Background(), version, staticFS); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
