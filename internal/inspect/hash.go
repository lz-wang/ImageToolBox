package inspect

import (
	"imagetoolbox/internal/filehash"
)

// computeHashes 按指定算法集合计算文件摘要；algorithm 为空表示全部算法
//（历史行为）。实现委托给共享的 internal/filehash 包，inspect 不再
// 自行维护哈希管道。
func computeHashes(path string, algorithms []filehash.Algorithm) (*HashInfo, error) {
	result, err := filehash.SumFile(path, algorithms)
	if err != nil {
		return nil, err
	}
	info := &HashInfo{}
	for algorithm, digest := range result.Digests {
		switch algorithm {
		case filehash.SHA256:
			info.SHA256 = digest
		case filehash.SHA1:
			info.SHA1 = digest
		case filehash.MD5:
			info.MD5 = digest
		case filehash.CRC32:
			info.CRC32 = digest
		}
	}
	return info, nil
}
