package compress

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

func runCommand(cmd *exec.Cmd) error {
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return commandError(err, stderr.String())
	}
	return nil
}

func commandError(err error, stderr string) error {
	message := strings.TrimSpace(stderr)
	if strings.Contains(message, "GLIBC_") && strings.Contains(strings.ToLower(message), "not found") {
		return fmt.Errorf("当前 Linux 运行环境不兼容；itb compress 要求 glibc >= 2.28: %s", message)
	}
	if message != "" {
		return fmt.Errorf("%w: %s", err, message)
	}
	return err
}
