package main

import (
	"context"
	"embed"
	"fmt"
	"os"

	"imagetoolbox/internal/cmd"
	"imagetoolbox/internal/compress"
)

//go:embed bins/**
var binaries embed.FS

var version = "dev"

func main() {
	compress.InitBinaries(binaries)

	if err := cmd.Execute(context.Background(), version); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
