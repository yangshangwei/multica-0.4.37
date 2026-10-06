# I1 foundation 离线实施证据

最新实施见 [S0/S1 验证记录](s0-s1-verification.md)：当前本地隔离数据库可用，旧 sandbox 限制不再是阻断；FG 仍未通过。

S1 已提交为 `837e1abaf`；后续 W02/W15 接线及剩余 FG 工作见 [S2 验证记录](s2-verification.md)。

> 历史记录：以下为转移前的离线实施证据，保留原迁移编号和当时环境限制。当前分支、550–566 迁移及联合验证见[分支整合记录](../10-05-iterations-i1/branch-integration.md)。

2026-10-06，工作副本未提交。本记录不是 FG / FCG 通过证明，未开放迭代能力。

## 已写入的基础代码

- `server/migrations/536_iteration_tables.{up,down}.sql`：共享 `workspace.planning_timezone`、Issue 当前归属与结转计数、settings/iteration/participation/event/snapshot/operation/notification。默认 disabled、旧任务 NULL/0，不增加 FK 或内联唯一索引。
- 537–552：16 个独立单语句 `CREATE [UNIQUE] INDEX CONCURRENTLY`。每个 down 在同一个 DO block 内先 NOWAIT 锁表、检查所有历史/操作/共享时区数据再执行 DDL。所有索引登记到 `server/cmd/migrate/main.go` 的现有无效索引清理映射。
- `server/pkg/db/queries/iteration.sql`：作用域读取、设置与实体锁、participation/event/snapshot/operation 读取、CAS 当前归属写入。Issue 和 Workspace 显式全量投影加入新列；sqlc 产物统一生成。ListWorkspaces 继续返回已有 `db.Workspace`，未新增转换层。
- `server/internal/iteration/types.go`：核心 wire DTO；Statistics 直接嵌入已实现的 ScopeStatistics，图表直接复用 ScopeChartPoint，避免两套口径字段。
- `transaction.go`：明确由调用者先授权并取得 workspace KEY SHARE/T1/member/catalog 前置锁，再取得 iteration advisory fence；仅数据库 SQLSTATE 40001/40P01/55P03 最多三次整事务重试。未知 commit 结果直接返回，不自动再次业务写；锁后使用 clock_timestamp，事件时刻防止时钟回拨。
- `operation.go`：规范化 payload hash 绑定 workspace/actor/operation/input，按 workspace/actor/request 查询及同事务保存稳定 operation_id/result；错 hash 409，实体删除后的结果仍可重放，畸形结果拒绝成功。Load/Save 不是鉴权入口，调用者必须在每次尝试前重新授权。

纯统计/时区/canonical 的独立证据由 history 子任务记录；这里只记录其与共享 DTO、事务基础的整包验证。

## 红绿记录

1. `TestIterationSchemaIsAdditive` 首次失败：536 迁移尚不存在（原始输出 `/private/tmp/multica-i1-schema-red.log`）；增加 schema 后通过。
2. `TestRunTransaction*` / `TestEventTimeNeverMovesBackwards` 首次编译失败（待实现符号），实现后通过；覆盖完整回滚后重试、普通业务错误、未知 commit、超限 503、取消与时间回拨。
3. `TestOperation*` 首次编译失败（待实现符号），实现后通过；随后同 UUID 多身份字段回归先红后绿，修正 map key 合并导致身份漏解析。覆盖身份/意图 hash、已删除结果重放、hash 冲突、畸形存储结果。
4. 现有 `TestEveryConcurrentUpBuildHasCleanup` 首次报告 16 个新增索引缺少登记；补齐映射后通过，`TestConcurrentIndexCleanupsMatchTheirMigrations` 同时通过。
5. 全 Go build 首次暴露 ListWorkspaces 的 SQL 投影缺列导致生成行类型变化；在 query 端补列再生成后 `go build ./...` 退出 0。

## 已运行的离线检查

使用现有已缓存依赖，不新增或联网安装依赖：

```sh
cd server
export GOCACHE=/private/tmp/multica-i1-go-cache
export GOMODCACHE=/Volumes/artisan/dev-cache/go-pkg/mod
export GOPROXY=off GOSUMDB=off
/private/tmp/multica-i1-sqlc generate
go test ./internal/iteration ./internal/migrations ./cmd/migrate -count=1 -v
go test -race ./internal/iteration -count=1
go vet ./internal/iteration ./internal/migrations ./cmd/migrate
go build ./...
```

这些命令已退出 0。sqlc v1.31.1 从现有 module cache 编译到 `/private/tmp/multica-i1-sqlc`；第二次生成前后逐文件 SHA256 完全一致。完整测试日志在 `/private/tmp/multica-i1-foundation-tests.log`。所有真正访问 PostgreSQL 的用例跳过，父测试显示 PASS 但内部子项 SKIP 时也不计入数据库通过证据。

## 实际阻断与未满足 FG

对本机 `127.0.0.1:5432` 和 `127.0.0.1:6379` 的一次 socket 连通性验证均返回 `PermissionError [Errno 1] Operation not permitted`。当前 sandbox 禁止网络、不得提权，未换协议/路径绕过限制，未触碰任何生产或共享数据库。

- 新 DB fixture `TestIterationSingleActiveAndLegacyFieldPreservation`、`TestIterationEveryRollbackStepRetainsEvidence`、`TestIterationEmptyRollbackAndConcurrentWriter` 已可编译，尚未执行真实数据库 SQL。它们使用独立 fixture schema，包含两连接单 active 竞争、旧字段保留、空 down、使用过数据拒绝 down、并发 writer NOWAIT 拒绝。
- 仍缺真实 PostgreSQL 的所有迁移 apply/catalog/invalid index/部分迁移恢复演练；恢复必须核对 `pg_index.indisvalid`、表归属与完整 index definition，不可把 ledger 当结构证据。有效索引存在但 ledger 未记入等中断情形也尚未演练。
- Load/SaveOperation 尚无真实数据库并发重放、响应丢失、撤权后重放拒绝测试；纯测试不能替代这些证据。
- writer inventory 已由 root 建立，但 HTTP/service/daemon/执行启动/父子状态/项目与成员变更的所有 writer 收口与事件接线尚未实施，更未证明无反序锁。现有 SQL 新列不等于所有跨端入口已完成兼容。
- settings/planning-timezone HTTP 权限、生命周期 API、事件采集、快照冻结、closure、outbox、前端与完整 29 项验收均仍未实施或未通过。保持 FG/FCG 未完成，不放行依赖 FG 的生命周期开发。
- 未运行全 Go 集成测试、Web/Desktop E2E、吞吐与锁等待基线；未提交、推送、发布、迁移数据库或启用功能。
