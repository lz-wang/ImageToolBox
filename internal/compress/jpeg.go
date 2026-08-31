package compress

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// JPEGOptions JPEG 压缩选项
type JPEGOptions struct {
	Context     context.Context
	Quality     int    // 压缩质量 1-100
	Progressive bool   // 是否使用渐进式编码
	Optimize    bool   // 是否优化霍夫曼表
	InputPath   string // 输入文件路径（djpeg 需要直接读取文件）
	Output      io.Writer
}

// CompressJPEG 执行 JPEG 压缩管道
// 管道: djpeg input.jpg | cjpeg -quality 80 -optimize -progressive
func CompressJPEG(opts JPEGOptions) error {
	djpegPath, err := EnsureBinary(DJpeg)
	if err != nil {
		return err
	}

	cjpegPath, err := EnsureBinary(CJpeg)
	if err != nil {
		return err
	}

	// 构建 cjpeg 参数
	cjpegArgs := []string{
		"-quality", fmt.Sprintf("%d", opts.Quality),
	}
	if opts.Optimize {
		cjpegArgs = append(cjpegArgs, "-optimize")
	}
	if opts.Progressive {
		cjpegArgs = append(cjpegArgs, "-progressive")
	}

	// djpeg 读取文件，输出到 stdout
	djpegCmd := exec.CommandContext(commandContext(opts.Context), djpegPath, opts.InputPath)

	// cjpeg 从 stdin 读取，输出到 stdout
	cjpegCmd := exec.CommandContext(commandContext(opts.Context), cjpegPath, cjpegArgs...)

	// 连接管道：djpeg stdout -> cjpeg stdin
	pipe, err := djpegCmd.StdoutPipe()
	if err != nil {
		return err
	}
	cjpegCmd.Stdin = pipe
	cjpegCmd.Stdout = opts.Output
	var djpegStderr, cjpegStderr bytes.Buffer
	djpegCmd.Stderr = &djpegStderr
	cjpegCmd.Stderr = &cjpegStderr

	// Start both commands before waiting so either process can be terminated if
	// its pipeline sibling fails.
	if err := djpegCmd.Start(); err != nil {
		return commandError(err, djpegStderr.String())
	}
	if err := cjpegCmd.Start(); err != nil {
		if stopErr := stopCommand(djpegCmd); stopErr != nil {
			return fmt.Errorf("%w; terminate djpeg: %v", commandError(err, cjpegStderr.String()), stopErr)
		}
		return commandError(err, cjpegStderr.String())
	}
	if err := pipe.Close(); err != nil {
		if stopErr := stopCommand(cjpegCmd); stopErr != nil {
			return fmt.Errorf("close JPEG pipe: %w; terminate cjpeg: %v", err, stopErr)
		}
		if stopErr := stopCommand(djpegCmd); stopErr != nil {
			return fmt.Errorf("close JPEG pipe: %w; terminate djpeg: %v", err, stopErr)
		}
		return fmt.Errorf("close JPEG pipe: %w", err)
	}

	if err := cjpegCmd.Wait(); err != nil {
		if stopErr := stopCommand(djpegCmd); stopErr != nil {
			return fmt.Errorf("%w; terminate djpeg: %v", commandError(err, cjpegStderr.String()), stopErr)
		}
		return commandError(err, cjpegStderr.String())
	}
	if err := djpegCmd.Wait(); err != nil {
		return commandError(err, djpegStderr.String())
	}
	return nil
}

func stopCommand(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	if err := cmd.Wait(); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			return err
		}
	}
	return nil
}
