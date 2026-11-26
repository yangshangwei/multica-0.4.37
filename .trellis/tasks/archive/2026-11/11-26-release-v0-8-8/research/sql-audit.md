# Research: V0.8.8 SQL 发布审计与升级演练

- Query: 从最近正式交付及支持升级起点，对比全部 SQL、迁移工具、条件对象、构件内 SQL；给出可以执行的隔离升级验收。
- Scope: internal repository and actual delivered artifact; GitHub read-only release API for provenance
- Date: 2026-11-26
- Status: 研究及静态审计完成；尚未对最终 V0.8.8 SHA / 构件 / 数据库执行发布验收。

## Findings

### 1. 最近实际发布基线确认为 v0.6.7

`gh release list --repo yangshangwei/multica-0.4.37 --limit 30` 返回最近正式版为 v0.6.7（2026-10-17T06:57:46Z），不是 v0.6.6。`gh api repos/yangshangwei/multica-0.4.37/commits/v0.6.7 --jq .sha` 返回：

`9caea35808afc8d05fb6135bfc74d4bfb0b45cf1`

最近实际本地交付目录使用例外命名 `dist/release/0.6.7/`，没有前导 `v`。本次仍使用用户指定 `dist/release/v0.8.8/`。

已读取实际离线包 `dist/release/0.6.7/multica-server-upgrade-v0.6.7-linux-amd64.tar.gz`，重新计算外层 SHA-256：

`cb963621b30c782cc7a7f073dde94065f4dbd43474c38d4879493004e4079d64`

该值与历史 `verification-v0.6.7.json` 和包的 `.sha256` 一致。已直接遍历其内层 `multica-images.tar.gz`，读取 image layers 内 `/app/migrations/` 的 **1188 个 SQL 文件（594 up、594 down）**，不是照抄历史“最大编号570”。逐文件 SHA-256 和 layer 来源保存在 `sql-baseline-v0.6.7-inventory.json`；复制 SQL 暂存在 `baseline-v0.6.7-sql/`，仅用于受控演练，无须把全部复制文件提交到 Git。

历史 `server-verification.json` 记载 v0.6.6→v0.6.7 的 password/device 两种认证保留、恢复备份、594 条账本及零无效索引。它是基线可信度证据，**不属于本次升级通过证据**。

### 2. 当前源码完整清单及差异

研究时主工作区尚未冻结最终发布 SHA。以下结果必须在修复合并且发布编辑提交后重做：

| 范围 | up | down | 合计 |
|---|---:|---:|---:|
| v0.6.7 实际 backend 构件 | 594 | 594 | 1188 |
| 当前 `server/migrations/` | 877 | 877 | 1754 |
| 相比实际 v0.6.7 新增 | 283 | 283 | 566 |
| 旧文件删除 | 0 | 0 | 0 |
| 旧文件内容变更 | 0 | 0 | 0 |

- 当前所有 up/down stem 成对，没有缺方向文件。
- 最大数字前缀889，但不是889条迁移。571–889区间缺号为 `643–649,653–659,661–679,710–712`；这不代表发布漏文件，因为真实 v0.6.7 旧文件全部仍在、字节一致。最终审计应记录确切文件集合及 Git 祖先/增量证明。
- 前缀≤128存在既有同号不同名文件；账本 key 为完整文件 stem，绝非数字前缀。`server/internal/migrations/migrations_lint_test.go:14` 固定既有重复清单；`:79` 禁止新的重复。
- 283 条新增 up 分为209条索引、13条 ready/guard、61条其他 DDL/data（静态分类）。209条索引都使用 `CREATE [UNIQUE] INDEX CONCURRENTLY`，每个文件只有一条 SQL 语句，且名称与 migrator 的 invalid-index cleanup map 一致。详见 `sql-concurrent-index-audit.json`。新增 SQL 的去注释词法检查没有发现 `FOREIGN KEY` 或 `REFERENCES`；这不是 SQL 引擎执行结果。
- 当前 `server/**/*.sql` 总数1900：1754迁移 + `server/pkg` 下146个查询源文件。`sql-all-server-working-tree.json` 包含全部路径/哈希，查询源也应与 v0.6.7 的 Git 源码对比，不能仅审计迁移而遗漏近期业务查询。查询通常由 sqlc 编译进程序，构件不含对应 `.sql`，应以源码清单、生成代码检查和二进制源码SHA建立关系。

输出清单：

- `sql-working-tree-inventory.json`：1754迁移完整文件名、字节数、SHA-256、方向与完整version。
- `sql-baseline-v0.6.7-inventory.json`：1188基线构件迁移及镜像层来源。
- `sql-baseline-v0.6.7-delta.json`：全部566新增文件；删除/修改为空。
- `sql-added-migrations-review.json`：283新增up的分类及需细查风险关键词。
- `sql-concurrent-index-audit.json`：209索引单句/并发/cleanup一致性。
- `sql-all-server-working-tree.json`：全量1900 SQL（含查询）。

### 3. 执行顺序、账本及健康门槛

- `server/internal/migrations/migrations.go:54`：按完整路径字典序排列 up；down 反序。`:94`：version为完整 basename 去 `.up.sql` / `.down.sql`。
- `server/cmd/migrate/main.go:1077`：单个连接固定 advisory lock，串行运行，未把整体放进事务；并发索引需要这一边界。部分数据迁移本身有 `BEGIN/COMMIT`，不应把“整个runner无事务”误解为每条文件都不带事务。
- `:1110`：账本只有 `version` 和 `applied_at`，**没有内容哈希**；已记录版本跳过SQL（`:1131`），因此必须外部比对文件内容。
- `:1154`：先运行修复/回填hook；`:1171`执行SQL失败直接返回，`:1177`只有成功或条件跳过后才记账。
- `Dockerfile:42` 复制整个 `server/migrations/` 到 `/app/migrations/`。最终必须从分发的 backend image 再读清单，与冻结源码逐文件一致；只看Dockerfile不算验证。
- `docker/entrypoint.sh:27` 每次启动先`./migrate up`，非零则退出，不启动API，成功才`exec ./server`。
- `server/cmd/server/health.go:24,165` 的 `/healthz` 比较全部必需version在账本中的匹配数；缺任意旧编号也返回503。这比“最大编号”严格，但不能检查索引有效性、条件对象、历史SQL哈希或额外未知账本行。`/health`只证明进程存活和commit，不返回应用版本。
- `server/cmd/migrate/main.go:976` 尚有无DATABASE_URL时的本地multica默认值，演练必须显式选定私有数据库且验证 `current_database()`，不能在空环境下直接运行。

### 4. 条件执行及真实schema必查

Migrator只有两条外部条件：

| version | 条件 | 真实对象检查 |
|---|---|---|
| 140_comment_content_trgm_index | 已有可用bigram时跳过trgm | `idx_comment_content_bigm` 确实可用，否则必须保留有效trgm |
| 371_comment_content_search_index_strategy | bigram不可用时跳过删除trgm | 不能因为账本有371就认定trgm已删；必须至少有一种有效搜索索引 |

来源：`server/cmd/migrate/main.go:688`。可用bigram要求在`:775`校验 live/ready/valid、GIN、非唯一、无predicate、表达式`lower(content)`以及pg_bigm-owned `gin_bigm_ops`。不能只检查同名relation存在。

历史SQL也会内部捕获错误/条件跳过：032_issue_search_index、033_comment_search_index、036_search_index_lower、039_project_search_index的pg_bigm；076_task_usage_pgcron_extension的pg_cron。部署常用pgvector:pg17不保证带pg_bigm/pg_cron。137建立pg_trgm、138–142提供fallback；应记录真实extensions和索引而非要求所有历史索引仍存在。103清理旧rollup有环境分支；prehook先做小时回填。198有旧attribution回填hook；431有chat-origin回填hook。

修复条件对象丢失时不能删除账本或把重启当修复。已应用SQL不会再执行。`server/cmd/migrate/README.md` 给出comment索引恢复流程（每句单独、事务外执行；先恢复有效trgm，再考虑删除fallback）。

### 5. 数据损失与旧候选升级范围

- 745删除用户`cloud_waitlist_email/cloud_waitlist_reason`，真实删除数据；down只建空字段。手册必须明确从升级前备份找回，不能用旧镜像恢复数据。
- 680删除未发布 single-session 路径的 `legacy_unresolved` runs/events/termination/commands/grants，以及旧列。来自正式v0.6.7时这些表尚不存在，迁移中创建后为空；**任意未发布协作候选并非等价的升级起点**。
- 699重排已有channel输入并回填group_input，对“已取消非root输入消费情况不明”或缺少唯一root时主动失败。717依赖恢复、725 leader-order恢复也会对不完整历史主动拒绝。不能跳过这些保护以通过发布。
- 744开放协作创建默认值，保留后台成功设置过的creation选择，不改AI执行开关。旧正式版没有此表，升后创建默认开放；需在发行说明中显式记录。
- 许多down保护retained数据；这是防损失护栏，不表示可以完整退到v0.6.7。新旧API不能同时连接迁移后的库，745使旧查询读用户失败。先停所有旧writer，后升级，再开桌面/daemon。

### 6. 可执行验收矩阵（主会话/实施代理执行，不是本研究的通过声明）

1. **P0 最近正式交付**：真实v0.6.7完整包部署到任务独占Docker Compose project，loopback端口、新卷；分别password/device认证。生成私有随机JWT、数据库密码，关闭外部agent/provider调用，不写日志。创建用户/工作空间/issue/附件和保存设置，记录非敏感ID、计数及附件hash。确认旧账本594条、与基线完整set相等，`/healthz`及原账号登录成功。
2. **P0 实包升级**：使用最终V0.8.8 tar解包内的`offline-upgrade.sh`，保留ordered overlays。必须先核验包里的源SHA、SQL hash与镜像架构。脚本备份当前DB/配置后升级；确认877条完整version集合、283条准确新增、旧594条不丢、未知额外账本项为空（如现有环境确有额外项须解释）。
3. **P0 schema**：导出所有非系统表/列/默认值、约束、索引定义和valid/ready/live、触发器及functions。另建全新V0.8.8 DB跑同源迁移，规范化schema dump后比较旧升新与fresh；排除owner/ACL、dump随机restrict token等非schema差异并记录规则，不忽略业务DDL。检查全部非系统索引有效，重点comment fallback，ready迁移覆盖的对象及最新inbox/budget/knowledge表。
4. **P0 重启幂等**：升级后保留完整ledger(version,applied_at)、schema签名、关键用户数据；重启最终backend入口，再次确认“已应用跳过”、账本与applied_at不变、schema不变、登录与`/healthz`成功。不是只跑第二次迁移返回0。
5. **P0 失败门槛**：第三个任务私有DB/容器，用一个只挂载给测试容器的迁移目录副本追加故意失败migration；最终原镜像不变。运行实际entrypoint，断言非零、失败version不进账本、不启动API/无healthz成功。修正仅测试副本后重试，确认恢复。另在一次性克隆DB上删除一条旧账本记录且直接启API（绕过migrate），断言healthz503；此故障注入不得作用于主验收DB或任何非任务库。
6. **P0 备份恢复**：把本次脚本生成的普通SQL dump导入另一个空的隔离DB，使用旧版对应镜像验证旧账本/data/login；不是导入已迁移库，也不是直接启动旧镜像“回滚”。
7. **P1 历史起点**：手册当前列v0.5.5/v0.5.6/v0.6.0/v0.6.1/v0.6.5/v0.6.6/v0.6.7。应为宣称支持的每个可靠源码/交付起点做清单差异，优先合并字节一致的迁移基线（v0.6.5/6/7历史均594）。v0.6.0存在“本地交付与v0.6.1正式版”身份差异，不能仅用同名标签代替实际包。对较旧基线可跑空schema→最新的直接升级加dump比较，但只能声明schema链路验收；seeded真实用户数据只覆盖明确演练的起点。
8. **P1 pg_bigm分支**：现有标准pgvector环境若没有pg_bigm，只能声明fallback分支已测；`TestCommentContentBigramRequirementMatchesRealPGBigm`会skip，保留skip而不能冒充真实扩展兼容。普通矩阵测试可验证形状选择，但不是生产扩展二进制验收。

推荐真实包升级命令（变量必须先由任务分配，不要复制生产路径）：

```bash
COMPOSE_PROJECT_NAME="$RELEASE_REHEARSAL_PROJECT" \
DOCKER_DEFAULT_PLATFORM=linux/amd64 \
bash "$TARGET_PACKAGE/offline-upgrade.sh" \
  --deployment-dir "$RELEASE_REHEARSAL_DEPLOYMENT" \
  --backup-dir "$RELEASE_REHEARSAL_BACKUP" \
  --compose-file docker-compose.rehearsal.yml --yes
```

macOS ARM宿主的脚本uname检查可能拒绝amd64包；历史用Rosetta执行完整脚本并由Docker运行amd64容器。不要删除架构检查，优先真正Linux amd64执行环境；如用仿真如实记录。独占 `COMPOSE_PROJECT_NAME` 是硬要求，Compose默认name为multica，仅换目录不隔离。

只读账本/schema查询示例：

```sql
SELECT current_database(), current_user;
SELECT version, applied_at FROM schema_migrations ORDER BY version;
SELECT e.extname,e.extversion FROM pg_extension e ORDER BY 1;
SELECT n.nspname,c.relname,i.indisvalid,i.indisready,i.indislive,pg_get_indexdef(i.indexrelid)
FROM pg_index i JOIN pg_class c ON c.oid=i.indexrelid JOIN pg_namespace n ON n.oid=c.relnamespace
WHERE n.nspname NOT IN ('pg_catalog','information_schema') ORDER BY 1,2;
SELECT table_name,column_name,data_type,is_nullable,column_default
FROM information_schema.columns WHERE table_schema='public' ORDER BY table_name,ordinal_position;
SELECT conrelid::regclass,conname,contype,convalidated,pg_get_constraintdef(oid)
FROM pg_constraint WHERE connamespace='public'::regnamespace ORDER BY conrelid::regclass::text,conname;
```

目标完整version集合生成（在冻结源码/构件各运行一次，对比输出）：

```python
from pathlib import Path
import hashlib, json
files=sorted(Path('/verified/path/migrations').glob('*.sql'))
rows=[dict(file=p.name,sha256=hashlib.sha256(p.read_bytes()).hexdigest(),bytes=p.stat().st_size) for p in files]
print(json.dumps(rows,indent=2))
```

数据库ledger应读到Python集合，与所有`.up.sql`去后缀后的集合做双向差集；计数相等不是集合相等。

### 7. 现有测试入口和常见误判

```bash
# Static migration contracts; inspect test skip counts as well as exit.
bash scripts/go-test-with-agent-cli-guard.sh go -C server test -count=1 -json ./internal/migrations
bash scripts/entrypoint.test.sh
# Requires explicitly allocated task-owned Postgres and verified identity first.
bash scripts/go-test-with-agent-cli-guard.sh go -C server test -count=1 -json ./cmd/migrate
```

`cmd/migrate` tests cover concurrent serialization, INVALID-index retry, source-context/knowledge/history/down guards, stage/issue-budget schema readiness and conditional search strategy. `server/cmd/migrate/migrate_concurrent_test.go:75` helpers can default or skip on DB failure; record actual named `run/pass/skip` events, not a package-level exit0. No test is executed by this research agent.

`health_test.go` contains partial-application503 cases. `scripts/entrypoint.test.sh` covers real shell signal forwarding, failed migration exit42, no API launch, restart ordering; it uses mocked binaries and supplements, not replaces, the real image failure rehearsal.

### 8. 计划审查结论

计划方向可执行，但在正式构建/发布前需要补齐以下明确门槛：

- 固定最后源码SHA后重新生成所有清单，并从最终保存/加载的镜像抽取SQL逐文件比对。
- 不应承诺“所有历史版本升级都验证过”：为每个宣称起点建立确切源码/交付映射；无法取得真实包的条目只写源码迁移验收范围。
- 最近正式版的旧SQL未变证明有价值，但尚未覆盖所有业务查询SQL、全部历史起点和最终源码；这些须等父会话的Git不可变比较。
- 在标签前完成最终main CI/live漏洞门槛（offline-delivery规范），不能用旧扫描或另一会话合并替代。
- schema确认、重启幂等、失败退出、旧备份恢复和原账号登录必须同时保留具体证据；`/healthz`不能替代这几项。

## Files found / Related specs

- `CLAUDE.md`：迁移不得新增外键、索引必须并发且独立文件、交付认证手册强制保留。
- `.trellis/spec/server/offline-delivery.md`：完整离线包、架构/实际镜像、数据库演练、CI gate。
- `.trellis/spec/server/database-test-isolation.md`：任务独占数据库、默认fallback风险、连接预算、skip不能当pass。
- `docs/offline-upgrade.zh-CN.md:27–37`：当前候选/763/旧版示例过时，需要按最终清单更新。
- `server/cmd/migrate/main.go` / `server/internal/migrations/migrations.go`：runner/版本排序/条件/hook。
- `server/cmd/migrate/README.md`：条件搜索索引恢复操作。
- `server/cmd/server/health.go`：所有必需账本version就绪检查。
- `docker/entrypoint.sh`、`Dockerfile`、`scripts/entrypoint.test.sh`：镜像迁移先行和失败边界。
- `dist/release/0.6.7/verification-v0.6.7.json`、`server-verification.json`：历史构件与594账本基线证据。

## External references

- GitHub Release: https://github.com/yangshangwei/multica-0.4.37/releases/tag/v0.6.7 （2026-11-26通过gh只读查询）。
- Baseline source: https://github.com/yangshangwei/multica-0.4.37/commit/9caea35808afc8d05fb6135bfc74d4bfb0b45cf1 。

## Caveats / Not Found

- 研究角色禁止Git操作；只读tag查询使用gh API。最终修复合并/ancestor证明、Git SQL差异由主会话执行并提供。
- 没有启动容器、数据库、构建或运行测试；本文件仅对当前源码和v0.6.7实际包做静态取证。
- 数量以本任务JSON清单为准。本次为1754迁移/1900全部SQL，最终SHA可能继续变化，发布前必须重算。
- v0.6.7历史数据库验收使用Docker Desktop amd64仿真；无法代表本次Linux目标主机或真实Windows客户端。
