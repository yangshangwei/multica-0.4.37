# P1 迁移与回退验收

2026-10-05 16:50（Asia/Shanghai）完成。验证源码基线：`bb1c20dcb16751a64931f2321a05b8773cec3aaa`，另加本次新增的 `server/internal/migrations/project_p1_rollback_test.go`。本报告仅覆盖迁移和工作空间清理 SQL；不代替 HTTP 删除并发、Web/Desktop 或生产发布验收。

## 环境与隔离

- PostgreSQL 17.11；连接入口为本机的 `multica_multica_projects_p1_991_verification`，由 `.env.worktree` 后覆盖 `.omx/projects-p1-test-env/verification.env` 提供。未记录连接凭据。
- 每个回归测试自行创建 `projects_p1_verify_<timestamp>` 私有 schema，`search_path` 仅包含该 schema。禁止远端连接；成功清理自己的 schema，失败保留证据。不清空 public，不运行服务数据库的 down。
- 全历史空库演练另建本机专用数据库 `projects_p1_full_verify_20261005164852`，成功后普通 `DROP DATABASE` 清理；没有强制关闭其他连接。
- 所有 Go 测试和迁移二进制演练均经过 `scripts/go-test-with-agent-cli-guard.sh`，未调用真实 agent CLI。

## 实际结果

| 验收项 | 证据 | 结果 |
|---|---|---|
| 从真正空库执行完整历史 up | 生产 `cmd/migrate`：573 条 ledger，P1 536–549 共 14 条，5 个新表 | 通过 |
| P1 全部 up / 空维护 down | 私有 schema 用真实 14 个 up、逆序 14 个 down；另用生产 runner 验证 ledger 同步删除 | 通过 |
| 完整历史库仅回退 P1 | 专用数据库仅加载 536–549 down；P1 ledger=0，`project_update` 消失 | 通过；没有回退历史 T1 |
| 旧项目逐字段无损 | 五种项目状态；NULL、空字符串、CRLF、Markdown、tab、emoji 描述；标题、图标、lead、优先级、DATE、execution_squad、创建/修改时间、资源、issue、workspace 设置前后 JSON 逐字段比较 | 通过 |
| 旧正式统计口径 | 每项目 `done/cancelled/in_progress` × `not_required/accepted/pending/rejected`；调用真实 generated `GetProjectIssueStats` | 每项目 total=6、done_count=4，取消仍计入旧 done_count，pending/rejected 均排除 |
| revision / 时间基线 | 历史 revision/description_revision=1；只有 in_progress 回填当前迁移时刻和 `migration` 来源，其余 since/source=NULL；planning_timezone=NULL | 通过；effective UTC 的 API 语义由 foundation 时区测试负责 |
| 每一步 down 保留证据 | 9 类：进展、修订、请求 tombstone、outbox、状态审计、项目版本、描述版本、真实 transition 时钟、非空规划时区 × 14 个 down | 126 个场景均 SQLSTATE P0001；保留行无损，13 个索引仍在 |
| 每一步 down 遇活跃 writer | 上述 9 类首笔未提交写入 × 14 down，覆盖 workspace、project 与全部 5 新表 | 126 个场景均立即 SQLSTATE 55P03；写入随后提交仍保留，提交后 down 仍拒绝 |
| down 先锁 / writer 后到 | 显式事务先执行 536 down，通过 pg_locks 观察 writer 实际等待，再 commit down | writer 返回 42P01，未确认成功且未丢已提交数据 |
| 无新 FK / cascade / 隐式索引 | 新表 catalog 无 FK；P1 up 源码检查；13 个 index up 均独立单语句 `CREATE [UNIQUE] INDEX CONCURRENTLY` | 通过；历史已有 FK 不在此“无新增”断言内 |
| CONCURRENTLY 真实失败恢复 | 重复 ID 使 537 报 23505，留下 INVALID index，ledger 未记；修复重复数据后通过生产 runner 重试 | 原实现 red，foundation 修复后 green，catalog valid/ready/live 全真且 ledger 14 条 |
| ledger 已记但索引缺失 | 删除私有 schema 内 537 索引，确认普通 up 只 skip；仅删除该版本 ledger，再生产 up | 对象与 ledger 恢复，状态审计数据不变；不能声称普通 up 会自动修复这种状态 |
| 部分 down 失败恢复 | 模拟已成功回退 549 后进程中断；workspace writer 使下一步 548 真正报 55P03；释放 writer 后生产 up | 549 重新建立有效索引，ledger 恢复 14 条；最终完整空 down 成功 |
| workspace 删除清理 | 调用真实 generated `DeleteWorkspace`，5 张新表各放本空间和另一空间数据 | 本空间新表数据全部删除，另一空间逐字段不变 |
| workspace 删除失败原子性 | workspace BEFORE DELETE trigger 注入异常，真实 generated 删除 SQL 执行失败 | 五张新表所有数据完整保留；移除故障后删除成功 |

`go test -race`：**9 个顶层测试通过**，含子测试共 279 个 pass 事件（包括 252 个逐迁移 guard/writer 场景、18 个场景分组）；0 fail，耗时 7.041 秒。不能把 279 个事件称为 279 项独立需求。

## 复现命令与原始记录

从当前 worktree 根目录执行：

```bash
set -a
source .env.worktree
source .omx/projects-p1-test-env/verification.env
set +a
PATH=/opt/homebrew/opt/libpq/bin:$PATH bash scripts/go-test-with-agent-cli-guard.sh -- \
  go -C server test -race ./internal/migrations -run '^TestProjectP1' -count=1 -json
go -C server vet ./internal/migrations
git diff --no-index --check /dev/null server/internal/migrations/project_p1_rollback_test.go
git diff --no-index --check /dev/null .trellis/tasks/10-05-projects-p1-verification/migration-verification.md
```

上述 race、vet、差异检查均实际成功。迭代时先运行不带 race 的完整 P1 测试，green 耗时 4.455 秒。全历史空库演练由 Python `subprocess` 调用本机 `psql` 建立专用库、`go build ./cmd/migrate` 编译当前生产 runner、完整 up、复制仅 P1 down 文件至临时迁移目录后 down，再检查 catalog/ledger 并清理专用库；所有子进程检查返回码。

原始本机记录（`.omx` 被 gitignore，报告记录结果，日志不作为随仓库交付文件）：

- `.omx/projects-p1-test-env/migration-red.log`：真实 INVALID 索引恢复失败，以及首次旧数据 fixture 的 status 别名歧义（fixture 已修复）。
- `.omx/projects-p1-test-env/migration-narrow.log`：不含待修复 runner 的其余迁移用例通过。
- `.omx/projects-p1-test-env/migration-green.log`：foundation 修复后的 9 项 P1 测试通过。
- `.omx/projects-p1-test-env/migration-race.jsonl`：最后一次包含 NULL/空描述与工作空间故障原子性用例的完整 race 结果。
- `.omx/projects-p1-test-env/migration-full-history.log`：完整 573 up、P1-only 14 down 及 catalog 数值。

## Red → Green 与残留证据

真实缺陷：537–549 未注册到 `concurrentIndexCleanups`。537 因重复 ID 构建失败后，普通 `IF NOT EXISTS` 重试把同名 INVALID index 视为存在，runner 输出 `Done.` 并错误写 ledger。测试在修复前观察此行为并失败，foundation 随后补齐全部 13 项 cleanup 注册，重建生产 runner 后同一测试通过。没有用手动 DROP 掩盖这个重试路径。

只保留以下失败 schema，均位于验收连接库且无活跃测试连接：

- `projects_p1_verify_1791189848933963000`：原缺陷证据，`project_state_change_id` 的 indisvalid=false、indisready=false，而 `537_project_state_change_id` 已在 ledger。2026-10-05 16:51 再次查询确认。
- `projects_p1_verify_1791189847739877000`：最初 fixture SQL 别名歧义的空 schema，仅调试遗留。

最终生产 runner SHA-256：`4c9fbbd15ad703d4bb1a9eb6379ec5a8c5ab47ee670199c3b7969032afcfa65f`。536 up SHA-256：`d3205d18d957a08843e997b4a6010eccd64bf6f74e558e4cf07b118669f63d38`；536 down SHA-256：`c3cf606abe9982dd05ad20a839c02d0b4cdcda49dcb8ebc0dbebe059292d1de5`。

## 运维边界

- 业务有 P1 数据、非默认版本、transition 时钟或非空规划时区时，down 必须保留拒绝行为。UI 回退不能顺手清空业务记录。
- 空维护 down 需先停 API/worker/writer。NOWAIT 是最后一道并发保护，不代替停写安排。
- ledger 已记但缺失/损坏索引的恢复必须同时看 catalog 与 ledger。需停写并处理重复业务键后，只重置确切索引版本的 ledger 行，再运行 up；确认 indisvalid/indisready/indislive 与 ledger 全部正确，不能仅看 `Done.`。本测试只在私有 schema 演练，未对任何服务数据库做此操作。
- workspace 测试验证真实删除 SQL 的新表清理与原子性，关联的其他历史表使用空 fixture；完整 handler 授权/执行历史/资源/通知双向争锁由 foundation/progress 验收覆盖。
- 未恢复历史 FK，未实施 I1，未执行生产部署。
