package compress

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"testing/fstest"
)

func TestExtractedBinaryName(t *testing.T) {
	got := extractedBinaryName(PngQuant)
	if runtime.GOOS == "windows" {
		if got != "pngquant.exe" {
			t.Fatalf("expected Windows binary name to end with .exe, got %q", got)
		}
		return
	}
	if got != "pngquant" {
		t.Fatalf("expected non-Windows binary name without extension, got %q", got)
	}
}

func TestEnsureBinaryLazyContentAddressedAndCached(t *testing.T) {
	cacheDir := t.TempDir()
	withTestBinaries(t, cacheDir, []byte("first fake pngquant"))

	path, err := EnsureBinary(PngQuant)
	if err != nil {
		t.Fatalf("EnsureBinary(PngQuant): %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("extracted binary missing: %v", err)
	}
	for _, binType := range []BinaryType{OxiPng, DJpeg, CJpeg} {
		matches, _ := filepath.Glob(filepath.Join(cacheDir, "itb", "bins", getPlatformKey(), "*", extractedBinaryName(binType)))
		if len(matches) != 0 {
			t.Fatalf("%s should not be extracted by pngquant request: %v", binType, matches)
		}
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	pathAgain, err := EnsureBinary(PngQuant)
	if err != nil {
		t.Fatalf("second EnsureBinary(PngQuant): %v", err)
	}
	if pathAgain != path {
		t.Fatalf("cache path changed: got %q, want %q", pathAgain, path)
	}
	infoAgain, err := os.Stat(pathAgain)
	if err != nil {
		t.Fatal(err)
	}
	if !infoAgain.ModTime().Equal(info.ModTime()) {
		t.Fatal("cache hit rewrote the binary")
	}
}

func TestEnsureBinarySameSizeDifferentContentUsesDifferentPaths(t *testing.T) {
	cacheDir := t.TempDir()
	first := []byte("same-length-content-a")
	second := []byte("same-length-content-b")
	if len(first) != len(second) {
		t.Fatal("test inputs must have the same size")
	}
	withTestBinaries(t, cacheDir, first)
	firstPath, err := EnsureBinary(PngQuant)
	if err != nil {
		t.Fatal(err)
	}
	withTestBinaries(t, cacheDir, second)
	secondPath, err := EnsureBinary(PngQuant)
	if err != nil {
		t.Fatal(err)
	}
	if firstPath == secondPath {
		t.Fatalf("same-size binaries used one cache path: %q", firstPath)
	}
}

func TestEnsureBinaryConcurrentExtraction(t *testing.T) {
	cacheDir := t.TempDir()
	withTestBinaries(t, cacheDir, []byte("concurrent fake pngquant"))

	const goroutines = 32
	paths := make(chan string, goroutines)
	errs := make(chan error, goroutines)
	var wg sync.WaitGroup
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			path, err := EnsureBinary(PngQuant)
			if err != nil {
				errs <- err
				return
			}
			paths <- path
		}()
	}
	wg.Wait()
	close(paths)
	close(errs)
	for err := range errs {
		t.Errorf("EnsureBinary: %v", err)
	}

	var first string
	for path := range paths {
		if first == "" {
			first = path
		} else if path != first {
			t.Errorf("got path %q, want %q", path, first)
		}
	}
	data, err := os.ReadFile(first)
	if err != nil {
		t.Fatalf("read extracted binary: %v", err)
	}
	if string(data) != "concurrent fake pngquant" {
		t.Fatalf("unexpected extracted content: %q", data)
	}
}

func withTestBinaries(t *testing.T, cacheDir string, pngquant []byte) {
	t.Helper()
	platformPaths, ok := binaryPaths[getPlatformKey()]
	if !ok {
		t.Skipf("unsupported test platform: %s", getPlatformKey())
	}
	source := make(fstest.MapFS, len(platformPaths))
	for binType, path := range platformPaths {
		data := []byte("fake " + string(binType))
		if binType == PngQuant {
			data = pngquant
		}
		source[path] = &fstest.MapFile{Data: data, Mode: fs.FileMode(0755)}
	}

	binariesMu.Lock()
	previousCacheBaseDir := cacheBaseDir
	cacheBaseDir = func() (string, error) { return cacheDir, nil }
	binariesMu.Unlock()
	InitBinaries(source)
	t.Cleanup(func() {
		binariesMu.Lock()
		cacheBaseDir = previousCacheBaseDir
		binariesMu.Unlock()
	})
}
