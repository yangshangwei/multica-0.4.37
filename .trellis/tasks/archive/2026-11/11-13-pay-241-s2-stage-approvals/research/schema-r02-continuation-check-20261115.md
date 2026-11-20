# PAY-241 S2 r02 有限 DDL 独立检查（2026-11-15）

结论：**4949 源绑定与当前只读 schema 通过；七项 retention 的完整原始执行证据尚有一项阻挡（SCHEMA-R02-E01）**。没有发现本次源/日志哈希不符或 DDL 产品缺陷。本报告不接受整 S2 2d、A01–A12、runtime 构建或实际产品场景。

## 范围与身份

- 唯一可连接库：`multica_pay241_20261114_s2_2d_migrate_r02`。只执行 `BEGIN READ ONLY`、身份/catalog/账本/单条 synthetic 行 SELECT，最后 `ROLLBACK` 并关闭 socket。
- 两次顺序只读连接：首轮校验器字段匹配失败保留；修正后复查通过。连接前/后其他连接均为 0，没有并行连接；没有 DDL、UPDATE、DELETE、迁移、API 启动或 service/handler/router suite。
- 写入仅限本报告两文件与 `/Users/artisan/.multica/acceptance/pay241-20261115-continuation/schema-check/`。实施库、旧库和后续 `check_*` 库均未连接。

## Findings (fixed)

- File: `/Users/artisan/.multica/acceptance/pay241-20261115-continuation/schema-check/audit-database.mjs:89`。
- Issue: 独立校验器误以为 root 约束记录有 `conname`；实际只有 `contype/definition`，因此首轮为 13/14 而非产品失败。
- Fix: 改为 owner 与精确排序的类型/定义多重集合比较。首轮脚本、报告、raw log/exit 保留；修正后 16/16，通过且完成 ROLLBACK/连接关闭。未改产品、测试或规划。

## Findings (not fixed)

### SCHEMA-R02-E01：immutable 直接 SQL 控制缺原始执行记录

`retention-results.json` 的 `immutable-update-refused` 只有 `sqlstate:23514`、`pass:true` 与 after snapshot，没有其原始 SQL/error transcript 或单独 log/exit 哈希对。其他六项 migrator 控制均有可复算 raw log/exit。

已独立核对当前 BEFORE UPDATE 行触发器启用、实际函数体无条件抛 23514 且匹配冻结源，以及 retained 行指纹一致；这些证明当前结构，不等于本 reviewer 重现过 UPDATE。此项只阻挡“七项完整 raw 执行证据通过”的结论，未推断 DDL 缺陷。请主会话补充已有原始 SQL/error 证据及绑定哈希（若保留）；本 permit 不授权重跑写入或创建事实。

## Verification

| 项目 | 独立结果 |
| --- | --- |
| 冻结源 | checked 2c 4939 文件；两层 overlay 16 条/12 个唯一路径；最终 4949 文件精确字节及文件清单，158/158 文件系统/证据断言通过 |
| r02 差分 | 861/864 DDL、查询及 sqlc 共四文件只移除摘要 `sha256:` 前缀；旧版保留，不以旧库再次 up 充当匹配证据 |
| 完整 up | 新专用 r02 空库前置记录匹配；835 个 raw up 顺序与 frozen stems/实际 ledger 精确一致，末尾 864；历史 371 一项 SQL skipped，新 861–864 均实际执行 |
| 原始命令 | 8 对 raw log/exit、16 个文件 SHA256 已复算；包括 build/full-up 与六个 retention migrator 控制；六个 control 的声明 log hash 全匹配 |
| 七项 retention | root 报告七项 pass；六项 raw 命令确认，一项 immutable 仅结构化记录，完整 raw gate BLOCKED |
| 空库 down/up | 835→831→835；四个 submission migration 的实际 reverse/forward raw 记录及 index/row/ledger snapshots 对应 |
| empty partial down | 834 ledger、没有 864、结构仍 true；只证明 DDL 尚在，不证明 runtime admission |
| retained down | 全 selected chain 在 864、直接 table down 在 861 均以 23514 拒绝且零 down 进度；after schema/index/ledger/row snapshots 等于 immutable after，并与当前只读状态吻合 |
| 当前 schema | 16 非空列，6 validated CHECK，2 精确 btree index（valid/ready/live；id unique）；无新 FK，裸 lowercase hex64 摘要，实际 immutable 函数匹配冻结源 |
| 当前 retained | 1 条 synthetic；`md5(row_to_json(row)::text)=4afe7a9c8e28f10337d371fd24d82129` 与拒绝 snapshots 相同；不是正式 submission/批准/部署证据 |
| 只读复查 | 16/16，实际库/owner/read-only 正确，835 ledger，其他连接始末均 0，ROLLBACK/client close/socket destroyed 均 true |
| 分配 | 10 个唯一 clone 记录/config hash/0600/包边界匹配；分配早于 retention rehearsal；未连接 clones，不声明其当前行状态 |
| Lint | 项目 lint 未运行（证据 write scope）；两私有 JS auditor 的 `node --check` 通过 |
| TypeCheck / Tests | 项目类型检查及 service/handler/router suites 均未运行，不伪装 pass |

完整 up raw log SHA256：`5ecc8cea51b499405fa5187e926b8f82db889a8db1db3f1d0410c1587932d46c`；raw exit SHA256：`9a271f2a916b0b6ee6cecb2426f0b3206ef074578be55d9bc94f6f3fe3ab86aa`。所有其他精确哈希、命令、两轮只读结果与已保留失败见同名 JSON 及下列私有报告。

## 保留证据

- `/Users/artisan/.multica/acceptance/pay241-20261115-continuation/schema-check/input-audit-r01.json`：158 项独立源/原始证据核对与全部 raw SHA256。
- `/Users/artisan/.multica/acceptance/pay241-20261115-continuation/schema-check/database-audit-r01.json` 及同 stem log/exit、`/Users/artisan/.multica/acceptance/pay241-20261115-continuation/schema-check/audit-database-r01-failed.mjs`：首轮校验器失败，未覆盖。
- `/Users/artisan/.multica/acceptance/pay241-20261115-continuation/schema-check/database-audit-r02.json` 及同 stem log/exit：16 项最终只读核对，完整 SELECT/catalog/行指纹与关闭记录。
- 本报告仅限 DDL source binding/retention/read-only schema；完整 2d writer、政策启用、运行与 A01–A12 仍由主会话/唯一 server owner 另行验收。
