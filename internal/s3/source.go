package s3

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"imagetoolbox/internal/filehash"
)

// snapshotSource 把源文件复制到私有临时快照，同时单遍计算 SHA-256，
// 并在复制完成后检测源文件的可观察变化。
//
// 这是 s3 upload 的内部实现细节，不是公开命令：上传语义要求
// itb-sha256 metadata 与实际 PUT body 严格对应、AWS SDK retry 每次
// rewind 读取的都是同一份稳定数据、原文件在快照之后的任何变化都
// 不影响本次上传。快照文件 0600 权限，由调用方 defer 删除。
//
// 返回的 digest 对应快照字节（即最终上传的内容），失败时快照已被清理。
func snapshotSource(inputPath string, file *os.File, initial os.FileInfo) (snapshotPath, digest string, err error) {
	tmp, err := os.CreateTemp("", "itb-upload-snapshot-*")
	if err != nil {
		return "", "", fmt.Errorf("failed to create upload snapshot: %w", err)
	}
	snapshotPath = tmp.Name()

	hasher := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, hasher), file); err != nil {
		tmp.Close()
		os.Remove(snapshotPath)
		return "", "", fmt.Errorf("failed to snapshot input file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(snapshotPath)
		return "", "", fmt.Errorf("failed to close upload snapshot: %w", err)
	}

	// 源文件在复制期间发生可观察变化时放弃上传：快照内容与调用方
	// 看到的文件不再对应，itb-sha256 也就失去了意义
	if err := filehash.VerifyUnchanged(inputPath, file, initial); err != nil {
		os.Remove(snapshotPath)
		return "", "", err
	}

	return snapshotPath, hex.EncodeToString(hasher.Sum(nil)), nil
}
