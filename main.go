package main

import (
	"context"
	"embed"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"imagetoolbox/internal/cmd"
	"imagetoolbox/internal/compress"
)

//go:embed bins/**
var binaries embed.FS

var version = "dev"

func main() {
	compress.InitBinaries(binaries)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := cmd.Execute(ctx, version); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
