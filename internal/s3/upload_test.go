package s3

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileSHA256(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(path, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	// sha256("hello\n") 的十六进制编码
	const want = "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03"
	got, err := fileSHA256(path)
	if err != nil {
		t.Fatalf("fileSHA256: %v", err)
	}
	if got != want {
		t.Errorf("fileSHA256 = %q, want %q", got, want)
	}
}

func TestFileSHA256MissingFile(t *testing.T) {
	if _, err := fileSHA256(filepath.Join(t.TempDir(), "nope.bin")); err == nil {
		t.Error("expected error for missing file")
	}
}

func TestDecideSkip(t *testing.T) {
	const (
		localHash  = "aaa"
		remoteHash = "aaa"
		otherHash  = "bbb"
	)

	newRemote := func(hash string, hasMetadata bool) *StatInfo {
		info := &StatInfo{Key: "k", Size: 1, LastModified: time.Now(), ETag: "\"e\""}
		if hasMetadata {
			info.Metadata = map[string]string{MetadataSHA256Key: hash}
		}
		return info
	}

	tests := []struct {
		name        string
		remote      *StatInfo
		localSHA256 string
		opts        *UploadOptions
		wantSkip    bool
	}{
		{
			name:   "未启用任何跳过选项时不跳过",
			remote: newRemote(remoteHash, true),
			opts:   &UploadOptions{},
		},
		{
			name:   "对象不存在时由调用方处理，decideSkip 不跳过 nil 远端",
			remote: nil,
			opts:   &UploadOptions{SkipExisting: true},
		},
		{
			name:     "skip-existing 同名即跳过",
			remote:   newRemote(otherHash, true),
			opts:     &UploadOptions{SkipExisting: true},
			wantSkip: true,
		},
		{
			name:   "skip-unchanged 且哈希一致才跳过",
			remote: newRemote(remoteHash, true),
			opts:   &UploadOptions{SkipUnchanged: true},
			// localSHA256 默认 localHash，与 remoteHash 相同
			wantSkip: true,
		},
		{
			name:        "skip-unchanged 哈希不一致时上传",
			remote:      newRemote(otherHash, true),
			localSHA256: localHash,
			opts:        &UploadOptions{SkipUnchanged: true},
		},
		{
			name:        "skip-unchanged 远端缺少 metadata 时上传",
			remote:      newRemote("", false),
			localSHA256: localHash,
			opts:        &UploadOptions{SkipUnchanged: true},
		},
		{
			name:        "skip-existing 优先于 skip-unchanged",
			remote:      newRemote(otherHash, true),
			localSHA256: localHash,
			opts:        &UploadOptions{SkipExisting: true, SkipUnchanged: true},
			wantSkip:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			local := tt.localSHA256
			if local == "" {
				local = localHash
			}
			skip, reason := decideSkip(tt.remote, local, tt.opts)
			if skip != tt.wantSkip {
				t.Errorf("decideSkip = %v (reason %q), want %v", skip, reason, tt.wantSkip)
			}
			if skip && reason == "" {
				t.Error("skipped result must carry a reason")
			}
		})
	}
}
