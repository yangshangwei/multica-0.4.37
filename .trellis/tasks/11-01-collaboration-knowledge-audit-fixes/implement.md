# 知识交付审计缺陷修复：实施计划

## 环境

- 修复基线：`4e3b61500`，位于 `codex/collaboration-knowledge-fixes-20261101`，worktree 为 `/Volumes/artisan/code/2026/mc-knowledge-fixes-20261101`。
- 每条分道有自己的 worktree，都从 `4e3b61500` 分出（见 `design.md`「实施分道」）。所有命令都先 `cd` 到该分道的绝对路径，不在共享 checkout 里运行任何命令。
- 数据库：
  - `multica-postgres-1` 中的模板库 `kfix_template_20261101` 已迁移到 760。
  - 每条分道的每个 DB-backed 包使用一份独立克隆，不同包不共用。

| 分道 | service | handler | cmd/server | cmd/migrate |
| --- | --- | --- | --- | --- |
| claim | `kfix_claim_svc` | `kfix_claim_handler` | `kfix_claim_cmd` | `kfix_claim_migrate` |
| launch | `kfix_launch_svc` | `kfix_launch_handler` | — | — |
| write | `kfix_write_svc` | `kfix_write_handler` | `kfix_write_cmd` | — |

- 连接串：`postgres://multica:multica@127.0.0.1:5432/<db>?sslmode=disable`。
- 新 migration 只对本分道自己的克隆执行 `go run ./cmd/migrate up`。
- Go 测试命令统一加 `env -u GIT_CONFIG_COUNT -u GIT_CONFIG_KEY_0 -u GIT_CONFIG_VALUE_0` 前缀，否则 `TestGitEnv` 会在本机误报失败。
- DB-backed 包在数据库不可达时会静默 `ok`。必须用 `-v` 确认输出里有真实的测试名。
- sqlc：`cd server && go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate`。基线上执行没有差异。
- 前端：claim 与 web 两个 worktree 已执行 `pnpm install --filter "@multica/views..." --filter "@multica/core..." --frozen-lockfile`。

## 每个缺陷的流程（TDD）

1. 按 `design.md` 写测试，放在 CLAUDE.md「Testing」规定的正确层级。
2. 在未修复的代码上运行这个测试，确认失败，并且失败原因就是这个缺陷。把输出保存到 `research/evidence/<ID>-red.txt`。
3. 实现修复。
4. 运行同一个测试，确认通过，把输出保存到 `research/evidence/<ID>-green.txt`。
5. 跑本包的相关测试，加上受影响的相邻测试。
6. 同步更新 spec 和内置 skill 文档中描述了这个行为的地方。
7. 原子提交，提交信息以 conventional 前缀开头，以 `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>` 结尾。只 `git add` 本缺陷改动的文件。

## 分道内顺序

<!-- LANE_ORDER -->

## 分道完成门

- 本分道改动的 Go 包：`go build ./...`、`go vet` 通过。所有 DB-backed 包按上表各用一份克隆，完整跑一遍并通过。涉及并发的包另加 `-race` 跑一遍。
- web 分道：`pnpm --filter @multica/core --filter @multica/views typecheck`、`lint`、`test` 全部通过，其中包括 `packages/views/locales/parity.test.ts`。
- 另一个 agent 独立审查本分道相对 `4e3b61500` 的 diff，结论为 pass 后，本分道才算完成。

## 集成与最终验证

1. 按 claim → launch → write → web 的顺序，把各分道合入 `codex/collaboration-knowledge-fixes-20261101`。`service/task.go` 和 spec 的冲突手工解决。合并后执行 sqlc，确认生成代码没有差异。
2. 在修复分支上：
   - Go：`go build ./...`；`go vet ./...`；所有包各用一份新克隆完整跑测试；改动过的并发包加 `-race`；daemon 的 Codex 握手测试只在失败时单独重跑一次确认是否为 flake。
   - TS：core 和 views 的 typecheck、lint、test。
3. 另一个 agent 独立审查合并后的整体 diff，重点看各分道之间的交互。
4. 与会话 multica-0-4-37-54 协调时间后，再跑门控的 `e2e/collaboration-knowledge.spec.ts`；必要时再跑 `make check`。
5. 合入 `codex/cross-workspace-collaboration` 需要用户同意。

## 回滚点

- 每个缺陷一个提交，可以单独 revert。
- migration 761+ 的 down 文件沿用 748–760 的约定：先调用 `collaboration_assert_knowledge_empty()`，再删除索引。
