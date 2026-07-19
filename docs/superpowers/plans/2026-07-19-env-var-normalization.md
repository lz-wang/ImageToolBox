# 环境变量归一化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 `itb` 全部环境变量统一为 `ITB_` 前缀（`S3_*`→`ITB_S3_*`、`LSKY_*`→`ITB_LSKY_*`），删除文档/help 中对 `AWS_*` 的宣传，并顺手修复 `S3_REGION` env 失效 bug。

**Architecture:** 纯字符串重命名 + 一处 flag 默认值修复。读取点集中在 `internal/s3/config.go` 和 `internal/lsky/client.go` 的 `LoadFromEnv`；文档/help/错误消息里散落的变量名同步更新。s3 与 lsky 包当前无测试，按 TDD 从零补齐 `config_test.go` / `client_test.go` 锁定新命名。硬切换，不保留旧名兼容。

**Tech Stack:** Go 1.26.x、`spf13/cobra`、标准库 `os.Getenv`/`testing`（`t.Setenv`）。无第三方 env 库。

**分支：** `refactor/env-var-normalization`（已在此分支，spec 已提交）。所有 Task 的 commit 均在此分支。

**全局约定：**
- Go 代码一律用 **Tab** 缩进。Edit/Write 的 Go 代码块必须用 Tab。
- 每个 commit 用 conventional commit message，并以 `Co-Authored-By: Claude <noreply@anthropic.com>` 结尾。
- 文件读写只用 Read/Write/Edit，禁止 sed/awk/cat 编辑文件。

---

## 文件结构

| 文件 | 职责 | 本次动作 |
|---|---|---|
| `internal/s3/config.go` | S3 `Config.LoadFromEnv`/`Validate` | 改 4 处 env 名 |
| `internal/s3/errors.go` | S3 错误变量 | 改 `ErrMissingCredentials` 消息 |
| `internal/s3/config_test.go` | S3 config 测试 | **新建** |
| `internal/cmd/s3.go` | S3 cobra 命令、flag、help | 改 `--region` 默认值 + help 文本 |
| `internal/lsky/client.go` | lsky `Config.LoadFromEnv`/`Validate`/`normalizeBaseURL` | 改 2 处 env 名 + 2 处错误消息 |
| `internal/lsky/client_test.go` | lsky 测试 | **新建** |
| `internal/cmd/lsky.go` | lsky cobra 命令、help | 改 help 文本 |
| `README.md` | 用户文档 | 改 2 个环境变量块 |
| `skills/itb/references/itb-command-reference.md` | skill 参考 | 改 S3/lsky env + 示例 |
| `skills/itb/SKILL.md` | skill 主文件 | 改 safety rule 一行 |
| `CHANGELOG.md` | 变更日志 | 新增 Unreleased 破坏性条目 |

---

## Task 1: S3 包 — LoadFromEnv 重命名 + 测试（TDD）

**Files:**
- Create: `internal/s3/config_test.go`
- Modify: `internal/s3/config.go:20-35`
- Modify: `internal/s3/errors.go:15`

- [ ] **Step 1: 写失败测试 `internal/s3/config_test.go`**

```go
package s3

import (
	"errors"
	"strings"
	"testing"
)

func TestLoadFromEnv(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		env    map[string]string
		want   Config
	}{
		{
			name: "flag 已设值时 env 不覆盖",
			config: Config{
				Endpoint:        "flag-endpoint",
				AccessKeyID:     "flag-ak",
				SecretAccessKey: "flag-sk",
				Region:          "flag-region",
			},
			env: map[string]string{
				"ITB_S3_ENDPOINT":          "env-endpoint",
				"ITB_S3_ACCESS_KEY_ID":     "env-ak",
				"ITB_S3_SECRET_ACCESS_KEY": "env-sk",
				"ITB_S3_REGION":            "env-region",
			},
			want: Config{
				Endpoint:        "flag-endpoint",
				AccessKeyID:     "flag-ak",
				SecretAccessKey: "flag-sk",
				Region:          "flag-region",
			},
		},
		{
			name:   "flag 为空时从 ITB_S3_* 读取",
			config: Config{},
			env: map[string]string{
				"ITB_S3_ENDPOINT":          "env-endpoint",
				"ITB_S3_ACCESS_KEY_ID":     "env-ak",
				"ITB_S3_SECRET_ACCESS_KEY": "env-sk",
				"ITB_S3_REGION":            "env-region",
			},
			want: Config{
				Endpoint:        "env-endpoint",
				AccessKeyID:     "env-ak",
				SecretAccessKey: "env-sk",
				Region:          "env-region",
			},
		},
		{
			name:   "region 为空且无 env 时 fallback us-east-1",
			config: Config{},
			env: map[string]string{
				"ITB_S3_ENDPOINT":          "e",
				"ITB_S3_ACCESS_KEY_ID":     "a",
				"ITB_S3_SECRET_ACCESS_KEY": "s",
			},
			want: Config{
				Endpoint:        "e",
				AccessKeyID:     "a",
				SecretAccessKey: "s",
				Region:          "us-east-1",
			},
		},
		{
			name:   "旧变量名 S3_* 不再被读取",
			config: Config{},
			env: map[string]string{
				"S3_ENDPOINT":              "old-endpoint",
				"S3_ACCESS_KEY_ID":         "old-ak",
				"S3_SECRET_ACCESS_KEY":     "old-sk",
				"S3_REGION":                "old-region",
				"ITB_S3_ENDPOINT":          "",
				"ITB_S3_ACCESS_KEY_ID":     "",
				"ITB_S3_SECRET_ACCESS_KEY": "",
				"ITB_S3_REGION":            "",
			},
			want: Config{Region: "us-east-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			cfg := tt.config
			cfg.LoadFromEnv()
			if cfg != tt.want {
				t.Fatalf("got %+v, want %+v", cfg, tt.want)
			}
		})
	}
}

func TestLoadFromEnv_ForcePathStyle(t *testing.T) {
	tests := []struct {
		name      string
		endpoint  string
		force     bool
		wantForce bool
	}{
		{"localhost 自动启用", "http://localhost:9000", false, true},
		{"127.0.0.1 自动启用", "http://127.0.0.1:9000", false, true},
		{":9000 自动启用", "http://minio:9000", false, true},
		{"普通端点不启用", "https://s3.amazonaws.com", false, false},
		{"已显式启用保持不变", "https://s3.amazonaws.com", true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{Endpoint: tt.endpoint, ForcePathStyle: tt.force}
			cfg.LoadFromEnv()
			if cfg.ForcePathStyle != tt.wantForce {
				t.Fatalf("ForcePathStyle got %v, want %v", cfg.ForcePathStyle, tt.wantForce)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr error
	}{
		{"缺 endpoint", Config{AccessKeyID: "a", SecretAccessKey: "s", Bucket: "b"}, ErrMissingEndpoint},
		{"缺凭证", Config{Endpoint: "e", Bucket: "b"}, ErrMissingCredentials},
		{"缺 bucket", Config{Endpoint: "e", AccessKeyID: "a", SecretAccessKey: "s"}, ErrMissingBucket},
		{"全部合法", Config{Endpoint: "e", AccessKeyID: "a", SecretAccessKey: "s", Bucket: "b"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == nil && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("got %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateWithoutBucket(t *testing.T) {
	if err := (Config{Endpoint: "e", AccessKeyID: "a", SecretAccessKey: "s"}).ValidateWithoutBucket(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := (Config{AccessKeyID: "a", SecretAccessKey: "s"}).ValidateWithoutBucket(); err != ErrMissingEndpoint {
		t.Fatalf("got %v, want %v", err, ErrMissingEndpoint)
	}
}

func TestErrMissingCredentialsMessage(t *testing.T) {
	msg := ErrMissingCredentials.Error()
	if !strings.Contains(msg, "ITB_S3_ACCESS_KEY_ID") {
		t.Fatalf("error message should mention ITB_S3_ACCESS_KEY_ID, got: %s", msg)
	}
	if !strings.Contains(msg, "ITB_S3_SECRET_ACCESS_KEY") {
		t.Fatalf("error message should mention ITB_S3_SECRET_ACCESS_KEY, got: %s", msg)
	}
}
```

- [ ] **Step 2: 跑测试验证失败**

Run: `go test ./internal/s3 -run 'TestLoadFromEnv|TestValidate|TestErrMissingCredentials' -v`
Expected: FAIL（`LoadFromEnv` 仍读 `S3_*`、消息仍含 `S3_ACCESS_KEY_ID`）。

- [ ] **Step 3: 改 `internal/s3/config.go` 的 4 处 `os.Getenv`**

把 `LoadFromEnv`（20-35 行）改成：

```go
// LoadFromEnv 从环境变量加载配置
func (c *Config) LoadFromEnv() {
	if c.Endpoint == "" {
		c.Endpoint = os.Getenv("ITB_S3_ENDPOINT")
	}
	if c.AccessKeyID == "" {
		c.AccessKeyID = os.Getenv("ITB_S3_ACCESS_KEY_ID")
	}
	if c.SecretAccessKey == "" {
		c.SecretAccessKey = os.Getenv("ITB_S3_SECRET_ACCESS_KEY")
	}
	if c.Region == "" {
		c.Region = os.Getenv("ITB_S3_REGION")
		if c.Region == "" {
			c.Region = "us-east-1"
		}
	}

	// 自动检测 MinIO，默认启用路径样式
	if c.Endpoint != "" && !c.ForcePathStyle {
		// MinIO 通常使用 localhost 或内网地址
		if strings.Contains(c.Endpoint, "localhost") ||
			strings.Contains(c.Endpoint, "127.0.0.1") ||
			strings.Contains(c.Endpoint, ":9000") {
			c.ForcePathStyle = true
		}
	}
}
```

- [ ] **Step 4: 改 `internal/s3/errors.go:15` 的错误消息**

把 `ErrMissingCredentials` 改成：

```go
	// ErrMissingCredentials 凭证未配置
	ErrMissingCredentials = errors.New("access key and secret key are required (set via flags or ITB_S3_ACCESS_KEY_ID/ITB_S3_SECRET_ACCESS_KEY env vars)")
```

- [ ] **Step 5: 跑测试验证通过**

Run: `go test ./internal/s3 -v`
Expected: PASS（全部用例通过）。

- [ ] **Step 6: Commit**

```bash
git add internal/s3/config.go internal/s3/errors.go internal/s3/config_test.go
git commit -m "$(cat <<'EOF'
refactor(s3): 环境变量重命名为 ITB_S3_* 前缀并新增 config 测试

- config.go LoadFromEnv 四处 os.Getenv 改为 ITB_S3_* 前缀
- errors.go ErrMissingCredentials 消息同步更新
- 新增 config_test.go 覆盖 LoadFromEnv/Validate/ForcePathStyle

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: S3 命令 — region flag 默认值修复 + help 文本

**Files:**
- Modify: `internal/cmd/s3.go:53-59`
- Modify: `internal/cmd/s3.go:126`

> 说明：region env 失效的根因是 `--region` flag 默认值非空，导致 `LoadFromEnv` 里 `if c.Region == ""` 永不成立。把默认值改空后，`LoadFromEnv` 内部已有的 `us-east-1` fallback（Task 1 未改动）继续兜底，最终默认值不变，但 `ITB_S3_REGION` 现在能覆盖它。Task 1 的 `TestLoadFromEnv` 已锁定该行为。

- [ ] **Step 1: 改 `internal/cmd/s3.go:53-59` 的 help 文本**

把 `s3Cmd.Long` 改成（删除 `AWS_*`，补齐 4 个 `ITB_S3_*`）：

```go
	Long: `S3 兼容存储操作，支持 AWS S3、MinIO、阿里云 OSS、腾讯云 COS 等。

环境变量支持:
  ITB_S3_ENDPOINT           自定义端点
  ITB_S3_ACCESS_KEY_ID      Access Key ID
  ITB_S3_SECRET_ACCESS_KEY  Secret Access Key
  ITB_S3_REGION             区域（默认 us-east-1）`,
```

- [ ] **Step 2: 改 `internal/cmd/s3.go:126` 的 region flag 默认值**

把：

```go
	s3Cmd.PersistentFlags().StringVarP(&s3Region, "region", "r", "us-east-1", "区域")
```

改成：

```go
	s3Cmd.PersistentFlags().StringVarP(&s3Region, "region", "r", "", "区域（默认从 ITB_S3_REGION 读取，未设置时为 us-east-1）")
```

- [ ] **Step 3: 验证编译与静态检查**

Run: `go build ./... && go vet ./...`
Expected: 无输出（成功）。

- [ ] **Step 4: 验证 help 输出（可选）**

Run: `go run . s3 --help`
Expected: Long 描述里列出 `ITB_S3_*` 四个变量，不再出现 `AWS_*`。

- [ ] **Step 5: Commit**

```bash
git add internal/cmd/s3.go
git commit -m "$(cat <<'EOF'
fix(s3): 修复 region 默认值导致 env 失效并归一 help 文本

- --region 默认值改为空，使 ITB_S3_REGION 可正常覆盖区域
- s3Cmd.Long 删除 AWS_* 宣传，补齐列出 ITB_S3_* 四个变量

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: lsky 包 — LoadFromEnv + Validate 重命名 + 测试（TDD）

**Files:**
- Create: `internal/lsky/client_test.go`
- Modify: `internal/lsky/client.go:18-35`

- [ ] **Step 1: 写失败测试 `internal/lsky/client_test.go`**

```go
package lsky

import (
	"strings"
	"testing"
)

func TestLoadFromEnv_FlagOverridesEnv(t *testing.T) {
	t.Setenv("ITB_LSKY_URL", "env-url")
	t.Setenv("ITB_LSKY_TOKEN", "env-token")
	cfg := Config{BaseURL: "flag-url", Token: "flag-token"}
	cfg.LoadFromEnv()
	if cfg.BaseURL != "flag-url" || cfg.Token != "flag-token" {
		t.Fatalf("flag should override env, got %+v", cfg)
	}
}

func TestLoadFromEnv_ReadsNewEnv(t *testing.T) {
	t.Setenv("ITB_LSKY_URL", "env-url")
	t.Setenv("ITB_LSKY_TOKEN", "env-token")
	cfg := Config{}
	cfg.LoadFromEnv()
	if cfg.BaseURL != "env-url" || cfg.Token != "env-token" {
		t.Fatalf("got %+v", cfg)
	}
}

func TestLoadFromEnv_IgnoresOldEnv(t *testing.T) {
	t.Setenv("LSKY_URL", "old-url")
	t.Setenv("LSKY_TOKEN", "old-token")
	t.Setenv("ITB_LSKY_URL", "")
	t.Setenv("ITB_LSKY_TOKEN", "")
	cfg := Config{}
	cfg.LoadFromEnv()
	if cfg.BaseURL != "" || cfg.Token != "" {
		t.Fatalf("old env names must not be read, got %+v", cfg)
	}
}

func TestValidate_MissingURL(t *testing.T) {
	err := Config{Token: "t"}.Validate()
	if err == nil || !strings.Contains(err.Error(), "ITB_LSKY_URL") {
		t.Fatalf("expected error mentioning ITB_LSKY_URL, got %v", err)
	}
}

func TestValidate_MissingToken(t *testing.T) {
	err := Config{BaseURL: "u"}.Validate()
	if err == nil || !strings.Contains(err.Error(), "ITB_LSKY_TOKEN") {
		t.Fatalf("expected error mentioning ITB_LSKY_TOKEN, got %v", err)
	}
}

func TestValidate_OK(t *testing.T) {
	if err := (Config{BaseURL: "u", Token: "t"}).Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNormalizeBaseURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"根地址", "https://img.example.com", "https://img.example.com/api/v1"},
		{"已含 /api/v1", "https://img.example.com/api/v1", "https://img.example.com/api/v1"},
		{"含 /api", "https://img.example.com/api", "https://img.example.com/api/v1"},
		{"带尾斜杠", "https://img.example.com/", "https://img.example.com/api/v1"},
		{"带空格和尾斜杠", "  https://img.example.com/  ", "https://img.example.com/api/v1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeBaseURL(tt.raw); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: 跑测试验证失败**

Run: `go test ./internal/lsky -run 'TestLoadFromEnv|TestValidate|TestNormalizeBaseURL' -v`
Expected: FAIL（`LoadFromEnv` 仍读 `LSKY_*`、消息仍含 `LSKY_URL`/`LSKY_TOKEN`）。（`TestNormalizeBaseURL` 应当直接通过，因其不涉及改名。）

- [ ] **Step 3: 改 `internal/lsky/client.go` 的 `LoadFromEnv` 与 `Validate`**

把 18-35 行改成：

```go
// LoadFromEnv 从环境变量加载配置
func (c *Config) LoadFromEnv() {
	if c.BaseURL == "" {
		c.BaseURL = os.Getenv("ITB_LSKY_URL")
	}
	if c.Token == "" {
		c.Token = os.Getenv("ITB_LSKY_TOKEN")
	}
}

// Validate 校验配置
func (c *Config) Validate() error {
	if strings.TrimSpace(c.BaseURL) == "" {
		return fmt.Errorf("缺少 LskyPro 地址，请通过 --url 或 ITB_LSKY_URL 提供")
	}
	if strings.TrimSpace(c.Token) == "" {
		return fmt.Errorf("缺少 LskyPro Token，请通过 --token 或 ITB_LSKY_TOKEN 提供")
	}
	return nil
}
```

- [ ] **Step 4: 跑测试验证通过**

Run: `go test ./internal/lsky -v`
Expected: PASS（全部用例通过）。

- [ ] **Step 5: Commit**

```bash
git add internal/lsky/client.go internal/lsky/client_test.go
git commit -m "$(cat <<'EOF'
refactor(lsky): 环境变量重命名为 ITB_LSKY_* 前缀并新增 client 测试

- client.go LoadFromEnv 两处 os.Getenv 改为 ITB_LSKY_* 前缀
- Validate 错误消息同步更新
- 新增 client_test.go 覆盖 LoadFromEnv/Validate/normalizeBaseURL

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: lsky 命令 — help 文本

**Files:**
- Modify: `internal/cmd/lsky.go:28-31`

- [ ] **Step 1: 改 `internal/cmd/lsky.go` 的 `lskyCmd.Long`**

把 28-31 行（环境变量支持块）改成：

```go
环境变量支持:
  ITB_LSKY_URL    LskyPro 服务地址（支持根地址或 /api/v1）
  ITB_LSKY_TOKEN  API Token`,
```

- [ ] **Step 2: 验证编译与静态检查**

Run: `go build ./... && go vet ./...`
Expected: 无输出（成功）。

- [ ] **Step 3: 验证 help 输出（可选）**

Run: `go run . lsky --help`
Expected: 列出 `ITB_LSKY_URL` / `ITB_LSKY_TOKEN`。

- [ ] **Step 4: Commit**

```bash
git add internal/cmd/lsky.go
git commit -m "$(cat <<'EOF'
docs(lsky): 归一 help 文本为 ITB_LSKY_*

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: 文档同步（README + skill reference + SKILL.md）

**Files:**
- Modify: `README.md`（279-286 行 S3 env 块、390-395 行 lsky env 块）
- Modify: `skills/itb/references/itb-command-reference.md`（162-172、198-203、209、217-220 行）
- Modify: `skills/itb/SKILL.md`（第 50 行）

- [ ] **Step 1: 改 `README.md` 的 S3 环境变量块**

把：

```bash
S3_ENDPOINT             # S3 端点 URL（可选）
S3_ACCESS_KEY_ID        # Access Key
S3_SECRET_ACCESS_KEY    # Secret Key
S3_REGION               # 区域（默认 us-east-1）
```

改成：

```bash
ITB_S3_ENDPOINT           # S3 端点 URL（可选）
ITB_S3_ACCESS_KEY_ID      # Access Key ID
ITB_S3_SECRET_ACCESS_KEY  # Secret Access Key
ITB_S3_REGION             # 区域（默认 us-east-1）
```

- [ ] **Step 2: 改 `README.md` 的 lsky 环境变量块**

把：

```bash
LSKY_URL    # LskyPro 地址，例如 https://img.example.com 或 https://img.example.com/api/v1
LSKY_TOKEN  # API Token
```

改成：

```bash
ITB_LSKY_URL    # LskyPro 地址，例如 https://img.example.com 或 https://img.example.com/api/v1
ITB_LSKY_TOKEN  # API Token
```

- [ ] **Step 3: 改 `skills/itb/references/itb-command-reference.md` 的 S3 env 块**

把：

```bash
S3_ENDPOINT
S3_ACCESS_KEY_ID
S3_SECRET_ACCESS_KEY
S3_REGION
AWS_ACCESS_KEY_ID
AWS_SECRET_ACCESS_KEY
AWS_REGION
```

改成（删除 `AWS_*` 三行）：

```bash
ITB_S3_ENDPOINT
ITB_S3_ACCESS_KEY_ID
ITB_S3_SECRET_ACCESS_KEY
ITB_S3_REGION
```

- [ ] **Step 4: 改 `skills/itb/references/itb-command-reference.md` 的 lsky env 块**

把：

```bash
LSKY_URL
LSKY_TOKEN
```

改成：

```bash
ITB_LSKY_URL
ITB_LSKY_TOKEN
```

- [ ] **Step 5: 改同文件示例命令中的 `$LSKY_TOKEN`**

把：

```bash
itb lsky upload -i photo.jpg --url https://img.example.com --token "$LSKY_TOKEN"
```

改成：

```bash
itb lsky upload -i photo.jpg --url https://img.example.com --token "$ITB_LSKY_TOKEN"
```

- [ ] **Step 6: 改同文件 lsky flag 说明里的变量名**

把：

```
- `--url`: LskyPro root or `/api/v1` URL; prefer `LSKY_URL`.
- `--token`: API token; prefer `LSKY_TOKEN`.
```

改成：

```
- `--url`: LskyPro root or `/api/v1` URL; prefer `ITB_LSKY_URL`.
- `--token`: API token; prefer `ITB_LSKY_TOKEN`.
```

- [ ] **Step 7: 改 `skills/itb/SKILL.md` 第 50 行的 safety rule**

把：

```
- Do not print secrets. Prefer environment variables for `S3_*`, `AWS_*`, and `LSKY_*` credentials.
```

改成：

```
- Do not print secrets. Prefer environment variables for `ITB_S3_*` and `ITB_LSKY_*` credentials.
```

- [ ] **Step 8: 全局检索确认无残留**

Run: `git grep -wnE 'S3_ENDPOINT|S3_ACCESS_KEY_ID|S3_SECRET_ACCESS_KEY|S3_REGION|LSKY_URL|LSKY_TOKEN|AWS_ACCESS_KEY_ID|AWS_SECRET_ACCESS_KEY|AWS_REGION' -- internal/ README.md skills/`
Expected: 无输出（退出码 1 即表示无匹配、通过）。`-w` 强制整词匹配：新名 `ITB_S3_ENDPOINT` / `ITB_LSKY_URL` 是单个 word，不会被 `S3_ENDPOINT` / `LSKY_URL` 分支误判为残留。检索范围限定 `internal/ README.md skills/`，不覆盖 `docs/`（spec 与本 plan 记录了改名映射，保留旧名是预期的）。

- [ ] **Step 9: Commit**

```bash
git add README.md skills/itb/references/itb-command-reference.md skills/itb/SKILL.md
git commit -m "$(cat <<'EOF'
docs: 同步环境变量文档为 ITB_ 前缀并移除 AWS_ 宣传

- README S3/lsky 环境变量块改名为 ITB_S3_*/ITB_LSKY_*
- skill reference 删除 AWS_*、改名 S3/lsky，更新示例与 flag 说明
- SKILL.md safety rule 统一为 ITB_S3_*/ITB_LSKY_*

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: CHANGELOG + 全量验证

**Files:**
- Modify: `CHANGELOG.md`

- [ ] **Step 1: 在 `CHANGELOG.md` 顶部新增 Unreleased 段**

在第 3 行（`All notable changes...`）之后、`## [v0.3.0]` 之前插入：

```markdown
## [Unreleased]

### Changed

- **BREAKING:** 环境变量统一为 `ITB_` 前缀：`S3_*` → `ITB_S3_*`、`LSKY_*` → `ITB_LSKY_*`，旧名不再被读取。
- 文档与帮助文本不再宣传 `AWS_*` 环境变量（代码此前从未读取，属 dead config）。
- 修复 `--region` 默认值导致 `ITB_S3_REGION` 失效的问题，环境变量现在可正常覆盖区域。
```

- [ ] **Step 2: 全量构建 + 测试 + 静态检查**

Run: `go build ./... && go test ./... && go vet ./...`
Expected: 全部 PASS、无报错。

- [ ] **Step 3: 手动验证（可选，需真实凭证）**

```bash
make build
ITB_S3_ENDPOINT=... ITB_S3_ACCESS_KEY_ID=... ITB_S3_SECRET_ACCESS_KEY=... ITB_S3_BUCKET=... ./itb s3 list -b <bucket>
ITB_LSKY_URL=... ITB_LSKY_TOKEN=... ./itb lsky upload -i photo.jpg
```

Expected: 不报“missing credentials/url”，能正常发起请求（凭证正确时成功）。

- [ ] **Step 4: Commit**

```bash
git add CHANGELOG.md
git commit -m "$(cat <<'EOF'
docs: CHANGELOG 记录环境变量归一化破坏性变更

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## 完成判据

- `go build ./... && go test ./... && go vet ./...` 全绿。
- `internal/s3/config_test.go` 与 `internal/lsky/client_test.go` 锁定新命名与 region 修复行为。
- `git grep` 在 `internal/`、`README.md`、`skills/` 下不再出现 `AWS_*`、旧 `S3_*`、旧 `LSKY_*` 变量名。
- 分支 `refactor/env-var-normalization` 上有 spec + 6 个 Task 的提交。
