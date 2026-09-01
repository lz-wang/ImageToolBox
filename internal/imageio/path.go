package imageio

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var ErrSameFile = errors.New("input and output must not refer to the same file")

// RejectSameFile rejects an output path that resolves to the input file.
func RejectSameFile(inputPath, outputPath string) error {
	inputAbs, err := filepath.Abs(inputPath)
	if err != nil {
		return fmt.Errorf("resolve input path: %w", err)
	}
	outputAbs, err := filepath.Abs(outputPath)
	if err != nil {
		return fmt.Errorf("resolve output path: %w", err)
	}
	if filepath.Clean(inputAbs) == filepath.Clean(outputAbs) {
		return ErrSameFile
	}

	inputInfo, err := os.Stat(inputPath)
	if err != nil {
		return fmt.Errorf("stat input: %w", err)
	}
	outputInfo, err := os.Stat(outputPath)
	if err == nil {
		if os.SameFile(inputInfo, outputInfo) {
			return ErrSameFile
		}
		return nil
	}
	if os.IsNotExist(err) {
		return nil
	}
	return fmt.Errorf("stat output: %w", err)
}
