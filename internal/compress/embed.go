package compress

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// BinaryType 定义二进制文件类型。
type BinaryType string

const (
	PngQuant BinaryType = "pngquant"
	OxiPng   BinaryType = "oxipng"
	DJpeg    BinaryType = "djpeg"
	CJpeg    BinaryType = "cjpeg"
)

// binaryPaths 定义不同平台的二进制文件路径。
var binaryPaths = map[string]map[BinaryType]string{
	"darwin-amd64": {
		PngQuant: "bins/macos-amd64/pngquant", OxiPng: "bins/macos-amd64/oxipng", DJpeg: "bins/macos-amd64/djpeg-static", CJpeg: "bins/macos-amd64/cjpeg-static",
	},
	"darwin-arm64": {
		PngQuant: "bins/macos-arm64/pngquant", OxiPng: "bins/macos-arm64/oxipng", DJpeg: "bins/macos-arm64/djpeg-static", CJpeg: "bins/macos-arm64/cjpeg-static",
	},
	"linux-amd64": {
		PngQuant: "bins/linux-amd64/pngquant", OxiPng: "bins/linux-amd64/oxipng", DJpeg: "bins/linux-amd64/djpeg-static", CJpeg: "bins/linux-amd64/cjpeg-static",
	},
	"linux-arm64": {
		PngQuant: "bins/linux-arm64/pngquant", OxiPng: "bins/linux-arm64/oxipng", DJpeg: "bins/linux-arm64/djpeg-static", CJpeg: "bins/linux-arm64/cjpeg-static",
	},
	"windows-amd64": {
		PngQuant: "bins/windows-amd64/pngquant.exe", OxiPng: "bins/windows-amd64/oxipng.exe", DJpeg: "bins/windows-amd64/djpeg-static.exe", CJpeg: "bins/windows-amd64/cjpeg-static.exe",
	},
	"windows-arm64": {
		PngQuant: "bins/windows-arm64/pngquant.exe", OxiPng: "bins/windows-arm64/oxipng.exe", DJpeg: "bins/windows-arm64/djpeg-static.exe", CJpeg: "bins/windows-arm64/cjpeg-static.exe",
	},
}

type binaryState struct {
	once sync.Once
	path string
	err  error
}

var (
	binariesMu sync.Mutex
	binariesFS fs.FS
	states     = newBinaryStates()

	cacheBaseDir = defaultCacheBaseDir
)

func newBinaryStates() map[BinaryType]*binaryState {
	return map[BinaryType]*binaryState{PngQuant: {}, OxiPng: {}, DJpeg: {}, CJpeg: {}}
}

// InitBinaries 初始化二进制文件（从 main.go 调用，传入 //go:embed bins/** 的 embed.FS）。
// 参数为 fs.FS 以便测试注入其他文件系统。调用它会丢弃当前进程中已缓存的二进制路径。
func InitBinaries(source fs.FS) {
	binariesMu.Lock()
	defer binariesMu.Unlock()
	binariesFS = source
	states = newBinaryStates()
}

// getPlatformKey 获取当前平台的 key。
func getPlatformKey() string { return fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH) }

func extractedBinaryName(binType BinaryType) string {
	if runtime.GOOS == "windows" {
		return string(binType) + ".exe"
	}
	return string(binType)
}

// EnsureBinary 确保指定二进制文件可用，返回内容寻址缓存中的可执行文件路径。
func EnsureBinary(binType BinaryType) (string, error) {
	binariesMu.Lock()
	state, ok := states[binType]
	binariesMu.Unlock()
	if !ok {
		return "", fmt.Errorf("unknown binary: %s", binType)
	}
	state.once.Do(func() { state.path, state.err = extractBinary(binType) })
	if state.err != nil {
		return "", state.err
	}
	return state.path, nil
}

func extractBinary(binType BinaryType) (string, error) {
	platformKey := getPlatformKey()
	paths, ok := binaryPaths[platformKey]
	if !ok {
		return "", fmt.Errorf("unsupported platform: %s", platformKey)
	}
	relPath, ok := paths[binType]
	if !ok {
		return "", fmt.Errorf("binary %s is not available for %s", binType, platformKey)
	}
	binariesMu.Lock()
	source := binariesFS
	binariesMu.Unlock()
	if source == nil {
		return "", fmt.Errorf("embedded binaries are not initialized")
	}
	data, err := fs.ReadFile(source, relPath)
	if err != nil {
		return "", fmt.Errorf("read embedded binary %s: %w", binType, err)
	}

	sum := sha256.Sum256(data)
	cacheDir, err := cacheBaseDir()
	if err != nil {
		return "", fmt.Errorf("resolve binary cache directory: %w", err)
	}
	targetDir := filepath.Join(cacheDir, "itb", "bins", platformKey, hex.EncodeToString(sum[:]))
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", fmt.Errorf("create binary cache directory: %w", err)
	}
	targetPath := filepath.Join(targetDir, extractedBinaryName(binType))
	match, err := fileMatchesHash(targetPath, sum)
	if err != nil {
		return "", fmt.Errorf("verify cached binary %s: %w", binType, err)
	}
	if match {
		return targetPath, nil
	}
	if err := writeAtomically(targetDir, targetPath, data); err != nil {
		return "", fmt.Errorf("extract binary %s: %w", binType, err)
	}
	return targetPath, nil
}

func defaultCacheBaseDir() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err == nil {
		return cacheDir, nil
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("itb-%d", os.Getuid())), nil
}

func fileMatchesHash(path string, expected [sha256.Size]byte) (bool, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return false, err
	}
	if !isUsableBinaryFile(info) {
		return false, nil
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return false, err
	}
	return bytes.Equal(hash.Sum(nil), expected[:]), nil
}

func isUsableBinaryFile(info fs.FileInfo) bool {
	if !info.Mode().IsRegular() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return info.Mode().Perm()&0111 != 0
}

func writeAtomically(dir, targetPath string, data []byte) error {
	tmp, err := os.CreateTemp(dir, ".extract-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0755); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, targetPath); err != nil {
		sum := sha256.Sum256(data)
		if match, matchErr := fileMatchesHash(targetPath, sum); matchErr == nil && match {
			return nil
		}
		return err
	}
	return nil
}
