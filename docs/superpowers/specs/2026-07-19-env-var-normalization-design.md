# 环境变量归一化设计

- 日期：2026-07-19
- 范围：`itb` CLI 全部环境变量
- 状态：待实现

## 背景

`itb` 当前通过裸 `os.Getenv` 读取 6 个环境变量（无任何 env 库），命名不统一、且文档与代码不一致：

| 当前变量名 | 读取点 | 用途 |
|---|---|---|
| `S3_ENDPOINT` | `internal/s3/config.go:22` | S3 端点 |
| `S3_ACCESS_KEY_ID` | `internal/s3/config.go:25` | S3 Access Key ID |
| `S3_SECRET_ACCESS_KEY` | `internal/s3/config.go:28` | S3 Secret Access Key |
| `S3_REGION` | `internal/s3/config.go:31` | S3 区域（**当前失效，见下**） |
| `LSKY_URL` | `internal/lsky/client.go:20` | LskyPro 服务地址 |
| `LSKY_TOKEN` | `internal/lsky/client.go:23` | LskyPro API Token |

两个现存问题：

1. **`AWS_*` 仅文档宣传，代码不读。** `internal/cmd/s3.go:56-58` 的 help 文本与 skill 文档列出 `AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY`/`AWS_REGION`，但代码从未读取（AWS SDK 的 env 链路被显式 `WithCredentialsProvider`/`WithRegion` 覆盖，且 `Validate()` 在 access/secret 为空时提前返回 `ErrMissingCredentials`）。属于 dead config。
2. **`S3_REGION` 失效。** `--region` flag 默认值被设为 `"us-east-1"`（`internal/cmd/s3.go:126`，非空），导致 `LoadFromEnv` 中 `if c.Region == ""` 永远不成立，`S3_REGION` 永远读不到。

## 目标

1. 所有环境变量统一以 `ITB_` 前缀开头。
2. S3 相关变量使用 `S3` 命名而非 `AWS`（删除文档/help 中对 `AWS_*` 的宣传）。
3. 顺手修复 region bug，使 `ITB_S3_REGION` 真正生效。

## 命名映射

| 当前 | 目标 |
|---|---|
| `S3_ENDPOINT` | `ITB_S3_ENDPOINT` |
| `S3_ACCESS_KEY_ID` | `ITB_S3_ACCESS_KEY_ID` |
| `S3_SECRET_ACCESS_KEY` | `ITB_S3_SECRET_ACCESS_KEY` |
| `S3_REGION` | `ITB_S3_REGION` |
| `LSKY_URL` | `ITB_LSKY_URL` |
| `LSKY_TOKEN` | `ITB_LSKY_TOKEN` |

`AWS_*` 不映射、不补读，一律从文档/help 删除，统一指向对应的 `ITB_S3_*`。

## 决策

- **硬切换，不保留旧名兼容。** 旧名 `S3_*`/`LSKY_*` 不再读取，无 fallback 链。理由：个人 CLI 无下游依赖，硬切换最干净。破坏性变更记入 CHANGELOG。
- **顺手修复 region bug。** `--region` flag 默认值改为 `""`，把 `us-east-1` 的 fallback 收敛到 `LoadFromEnv` 内部（该 fallback 已存在于 `config.go:32-34`，无需新增）。最终默认值不变（仍 `us-east-1`），但 `ITB_S3_REGION` 现在能覆盖它。

## 代码改动

### `internal/s3/config.go`
- `LoadFromEnv`（20-35 行）4 处 `os.Getenv` 改名：`S3_ENDPOINT`→`ITB_S3_ENDPOINT`、`S3_ACCESS_KEY_ID`→`ITB_S3_ACCESS_KEY_ID`、`S3_SECRET_ACCESS_KEY`→`ITB_S3_SECRET_ACCESS_KEY`、`S3_REGION`→`ITB_S3_REGION`。region 的 `us-east-1` fallback 逻辑保持不变。

### `internal/cmd/s3.go`
- 第 126 行：`--region` 默认值 `"us-east-1"` → `""`（region bug 修复）。
- 第 53-59 行 `s3Cmd.Long`：删除 `AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY`/`AWS_REGION`，改为完整列出 4 个 `ITB_S3_*`（endpoint/access-key/secret-key/region）。

### `internal/s3/errors.go`
- 第 15 行 `ErrMissingCredentials` 消息：`S3_ACCESS_KEY_ID/S3_SECRET_ACCESS_KEY` → `ITB_S3_ACCESS_KEY_ID/ITB_S3_SECRET_ACCESS_KEY`。

### `internal/lsky/client.go`
- `LoadFromEnv`（20/23 行）：`LSKY_URL`→`ITB_LSKY_URL`、`LSKY_TOKEN`→`ITB_LSKY_TOKEN`。
- `Validate`（30/33 行）错误消息：`LSKY_URL`→`ITB_LSKY_URL`、`LSKY_TOKEN`→`ITB_LSKY_TOKEN`。

### `internal/cmd/lsky.go`
- 第 29-30 行 `lskyCmd.Long`：`LSKY_URL`/`LSKY_TOKEN` → `ITB_LSKY_URL`/`ITB_LSKY_TOKEN`。

## 文档改动

- `README.md`：S3 环境变量块（约 279-286 行）、lsky 环境变量块（约 390-395 行）改名；删除对 `AWS_*` 的提及。
- `skills/itb/references/itb-command-reference.md`：S3（约 165-171 行）删除 `AWS_*`、改名 `ITB_S3_*`；lsky（约 201-202、209、218-219 行）改名，示例命令中 `$LSKY_TOKEN`→`$ITB_LSKY_TOKEN`。
- `skills/itb/SKILL.md`（第 50 行）：泛指的 `S3_*`/`AWS_*`/`LSKY_*` → `ITB_S3_*`/`ITB_LSKY_*`（删除 `AWS_*`）。

## 测试（新增，从零）

s3 与 lsky 包当前无 `_test.go`。新增表驱动测试，用 `t.Setenv` 注入环境变量：

### `internal/s3/config_test.go`
- `LoadFromEnv`：flag 已设值时 env 不覆盖；flag 为空时从 `ITB_S3_*` 读取（endpoint/access/secret 各一例）。
- region：flag 空 + env 空 → fallback `us-east-1`；flag 空 + env 设值 → 取 env（验证 region bug 修复）；flag 设值 → 取 flag。
- MinIO endpoint 启发式：endpoint 含 `localhost`/`127.0.0.1`/`:9000` 时 `ForcePathStyle` 自动 true；已显式设 true 时不被改动。
- `Validate`：缺 endpoint / 缺凭证 / 缺 bucket / 全部合法 各分支；`ValidateWithoutBucket` 不校验 bucket。

### `internal/lsky/client_test.go`
- `LoadFromEnv`：flag vs env 优先（url、token）。
- `Validate`：缺 url 报错（消息含 `ITB_LSKY_URL`）；缺 token 报错（消息含 `ITB_LSKY_TOKEN`）；合法时 nil。
- `normalizeBaseURL`：根地址、已含 `/api/v1`、已含 `/api`、带尾斜杠 各分支。

## 验证方式

```bash
go build ./...
go test ./internal/s3 ./internal/lsky -v
go vet ./...
```

构建后手动验证（可选）：

```bash
ITB_S3_ENDPOINT=... ITB_S3_ACCESS_KEY_ID=... ITB_S3_SECRET_ACCESS_KEY=... ./itb s3 list -b bucket
ITB_LSKY_URL=... ITB_LSKY_TOKEN=... ./itb lsky upload -i photo.jpg
```

## 不在范围

- 不新增 `--bucket` / `--force-path-style` 的环境变量支持。
- 不保留旧名 `S3_*` / `LSKY_*` 的兼容 fallback。
- 不补读 `AWS_*` 代码（仅从文档/help 删除宣传）。

## CHANGELOG

记录破坏性变更：所有环境变量重命名为 `ITB_` 前缀；`AWS_*` 不再被文档宣传；旧名 `S3_*`/`LSKY_*` 不再读取。
