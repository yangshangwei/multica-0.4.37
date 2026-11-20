# F3-S1 实施计划

> **2026-11-20 最终接管**：本会话 `01a1f915-42b9-7422-a8d8-aa97e9af8724` 独占 `mc-f3-completion-20261120`；前 Codex root 与三子代理已明确停止写入。Claude 主进程与已观察的后台测试于 21:12 退出，原树保留。已核验原功能快照的 62 行 normal/race、91 项 B54 前后对照、Core 5172、Views r4 7441（exit 0），功能提交 `584846f95`。正在对与共享 `f2ea7d50d` 合并后的源码重新验证；旧进度记录是历史，不作为当前待办。最终入口见 [接管与差异核对](research/codex-takeover-20261120.md)。

> **2026-11-20 20:45 Codex 续接通知**：用户已要求本任务推进至归档。Codex 已在 `mc-f3-completion-20261120` 实现并集成 5c/6a/6b/7a/7b，正常/race DB 专项共十组已全部通过。请原会话停止重复派发 6a/7a，并读取 `research/codex-coordination-20261120.md` 追加交接。原始工作树、报告、失败记录均保留。本通知不改变已有合同；最终归档由 Codex 会话完成。

阅读顺序：`prd.md` → `design.md` → 父任务 [F3 最小合同](../../../11-12-agent-squad-followup-roadmap/research/f3-minimum-budget-contract-20261113.md)（先看 §11）→ [验收矩阵](../../../11-12-agent-squad-followup-roadmap/research/f3-acceptance-matrix-20261113.md)。

## 工作位置

- 当前产品集成在 `/Volumes/artisan/code/2026/mc-f3-completion-20261120`，分支 `codex/f3-completion-20261120`。原 `mc-f3-budget-root-20261116` 与 6a/6b/7b/复核工作树均保留为来源；共享 checkout 的无关改动受保护。
- 任务文档留在共享 checkout 的本目录，子代理用绝对路径读写。
- 提交只用显式路径，不用 `git add -A`。不 push、不开 PR。

## 环境约束

- 数据库测试槽位全局最多两组；PAY 占一组。F3 同时只跑一组。
- normal 与 race 各用新建的任务自有数据库，从已迁移的空库克隆；证明零 skip（DB 不可达时 Go 测试会静默 skip）。
- 不调用真实 Provider，不执行用户安装的 agent CLI，不操作 POLA-1 或共享项目策略。
- 子代理并发不超过 3。

## 步骤

每步有自己的出口，前一步未过不进下一步。第 3 步之前没有持久变更。

- [x] **1. E2 清点与锁图**。在当前 HEAD 重核合同 §11 的 R1–R10 和所有行号；列全所有写 `parent_issue_id` 或创建 issue 的路径及其是否取 `iteration:<ws>` fence（含 autopilot create_issue、onboarding、协作结构 writer、工作空间拆除、quick-create 完成后关联）；画出含根行的全路径锁图。产出 `research/e2-writer-inventory.md` 与 `research/e2-lock-graph.md`。出口：独立 check 复核清单与锁图。2026-11-16 出口通过，修正见 `design.md` §5 与 §7 的 V11–V18。
- [x] **2. 纯计量**。tick/十进制编码、价格快照与舍入、A/R/U_hold reducer、累计/增量/更正。无 DB、无 I/O。出口：B15、B17–B30 的 pure 行。2026-11-17 出口通过（独立 check 含变异测试），留给第 5、6 步的条件见 `design.md` §7。
- [x] **3. schema**。六张表（`design.md` §8）与独立并发索引迁移，`make sqlc`；工作空间拆除清单登记为 `retain_audit`。出口：B51、B56，加上 `TestWorkspaceDeletionManifestCoversPublicSchema` 在克隆库上实际运行通过。2026-11-17 出口通过（独立 check 加第三轮复跑），表结构以 `design.md` §8 为准。
- [x] **4. 根与成员**。启用、继承、改父/删除、close/reopen。出口：B01–B13、B57、B59、B60；B14 在 service 层只断言跨工作空间查不到，完整的 auth 层用例在第 7 步。2026-11-17 实现完成；2026-11-18 独立复核通过（`research/step4-binding-check.md`），主会话在新克隆上复跑通过，裁定见 `design.md` §5、§6、§9.4、§10.2、§10.13、§12.7 的同日补充。B07、B08 的绑定半边要等 5b 有了 attempt 才钉得住。
- [x] **5. attempt 与影子预留**。规则见 `design.md` §10：finalize 事务接入、声明式预留额与未定额预留、账目装载（已提交前缀缓存）、绑定锁、生命周期对账、`budget_attempt_id` 下发、quick-create 归根。出口：B07、B08、B34–B38（含 B35a）、B45–B47、B55，加上 §10.9 的双连接用例。拆成三段（§10.13）：
  - [x] 5a 纯计量包：未定额预留、不修改接收者的应用方法、基准。可与第 4 步复核并行。第一轮完成（报告已写，主会话复跑 89 PASS / 0 FAIL / 0 SKIP）；第二轮第 1 项（常数时间的 hold 判定）已交回，主会话独立复跑 99 PASS / 0 FAIL / 0 SKIP、新旧对拍一致（2026-11-18）；第 2–4 项（`Apply` 私有、`EncodeEntry` / `DecodeEntry`、`Reservation`）交回后主会话复跑 112 PASS / 0 FAIL / 0 SKIP（两条 Go 发行线、含 `-race`），拷回 F3 worktree，在克隆库上跑过第 4 步的预算用例。2026-11-18 19:59 收尾：补充报告 P.1–P.12 读完，报告里 21 个文件的哈希与 F3 worktree 逐个相同，临时 worktree 已删除。
  - [x] 5b finalize 接入：表结构（含价格快照两列从 attempt 挪到条目）、账目装载与缓存、finalize 十步与补充（协议版本、`legacy_protocol`、条目身份格式、`limit` 的 detail）、绑定锁两侧、载荷下发。出口另含 B53 的 finalize 半边。等第 4 步复核通过。分两轮派发（2026-11-18）：第一轮只做表结构、管理操作（预留额、启用换序、`transact` 声明级别）、绑定方的排他锁、账目装载与缓存，交回后主会话复核并复跑；第二轮才做 finalize 十步、两个调用点和入口清点。原因是运行中的子代理收不到纠正，finalize 的改动面最大，要先把它依赖的地基审过。第一轮 2026-11-19 04:27 交回，主会话复核通过并带两处改动（“全量重放判定”、单条条目 2,048 字节上限），第二轮 04:47 发出。
  - [x] 5c 事后与周边：生命周期对账（hold 原因映射表交主会话确认）、`off` 期间规则、quick-create 归属差异。
  - [x] 整步独立复核。
- [x] **6. 用量事件入口与 daemon journal**。规则见 `design.md` §11：协议声明、attempt 维度的 daemon 入口、整批原子、原 key 恢复与查询、严格线上格式、按证据表定重叠与成本、服务端估算与逐条冻结的价格快照、daemon 回传记录。首包没有更正与结算的生产路径（§11.8）。出口：B31–B33、B49、B53，B16 与 B24 的 db 半行，B42 的晚到用量半行，加上 §11.11 的双连接与 daemon 用例。拆成两段（§11.12）：
  - [x] 证据表内容：调研 `research/step6-adapter-usage-semantics.md` 出来后由主会话逐行裁定，写进 §11.6。（2026-11-18 完成，见进度记录）
  - [x] 6a 服务端：入口、查询、严格解码、归一化、估算。在 5c 之后。
  - [x] 6b daemon：能力声明、回传记录、组批发送、重启恢复。
  - [x] 整步独立复核。
- [x] **7. 权限、API、CLI**。规则见 `design.md` §12。允许名单、事务内复核、revision、最小披露、`show` 与执行列表；Core 的 zod schema 与畸形响应测试；同步内置 skill 文档与 source map。出口：B14、B39–B44（B42 只剩“新预留被拒”一半）、B48、B50、B52、B58。拆成两半（§12.10，2026-11-19）：
  - [x] 7a 服务端：事务内复核、读取侧与分页查询、八条路由、`no-store`、§12.8 除 CLI 之外的用例。在第 5、6 步整步复核之后。
  - [x] 7b 调用端：CLI、Core 的 `show` schema 与 API 方法、内置 skill 文档与 source map。只依赖 §12.9 的线上形状，在自己的 worktree 里与第 5、6 步并行（2026-11-19 04:31 派出）。
  - [x] 整步独立复核。
- [x] **8. 不回归与收尾**。B54；normal/race 各用新建数据库；独立 check；写 `.trellis/spec/server/` 的 F3 规范；限定路径提交；更新父任务记录。

## 验证命令

```bash
cd /Volumes/artisan/code/2026/mc-f3-budget-root-20261116/server && go build ./... && go vet ./...
cd /Volumes/artisan/code/2026/mc-f3-budget-root-20261116/server && go test ./internal/budget/... -count=1          # 纯计量，无 DB
cd /Volumes/artisan/code/2026/mc-f3-budget-root-20261116/server && DATABASE_URL=<任务自有库> go test ./internal/service/... -run 'Budget' -count=1
cd /Volumes/artisan/code/2026/mc-f3-budget-root-20261116/server && DATABASE_URL=<另一新库> go test -race ./internal/service/... -run 'Budget' -count=1
```

包路径已定为 `server/internal/budget`。

第 8 步的 B54（不回归）按文件取具名用例，不靠名字前缀猜：两份验收记录对应的提交是 `463b201fa`（完成授权）与 `fafb27b1d`（报告快照），它们的 Go 用例在当前 HEAD 上位于下面这些文件。做法是从这些文件里取出全部 `func Test…` 名字，要求每一个都在克隆库上的 `-v` 输出里以 `--- PASS` 出现（出现 `--- SKIP` 或缺席都算不通过），F3 改动前后各跑一次作对照。

- `server/cmd/multica/`：`cmd_issue_completion_test.go`（10）、`cmd_issue_report_snapshot_test.go`（22）
- `server/cmd/server/`：`report_snapshot_integration_test.go`（4）、`report_snapshot_read_barrier_test.go`（2）
- `server/internal/handler/`：`delivery_completion_preview_test.go`（7）、`report_snapshot_test.go`（10）、`report_snapshot_collection_test.go`（3）、`report_snapshot_withdrawal_test.go`（4）
- `server/internal/middleware/report_snapshot_headers_test.go`（2）
- `server/internal/reportsnapshot/`：`public_test.go`（13）、`snapshot_test.go`（9）、`health_test.go`（1）、`legacy_identity_test.go`（1）
- `server/internal/service/report_snapshot_collection_audit_test.go`（3）

括号里是 2026-11-17 在 `86fea651c` 上数到的用例数，合计 91。`463b201fa` 另外带的前端用例（`packages/core` 的 completion 系列、`packages/views` 的完成授权弹窗）在第 7 步给 Core 加 schema 之后用 `pnpm --filter @multica/core test` 与 `pnpm --filter @multica/views test` 覆盖。

手工跑 Go 测试时把 `TMPDIR` 设成不含符号链接的真实路径（本机是 `/private/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/`）。默认的 `/var/folders/...` 是符号链接，`internal/service` 全包会因此多出 45 条与预算无关的 `TestStage*` 失败（artifact adapter unavailable）。输出重定向到文件再看退出码，不要接 `tail`。

## 回滚点

- 第 3 步前：丢弃 worktree 分支即可。
- 第 3 步后：停用观察开关，账目保留；迁移不做破坏性 down。

## 进度记录

（每步完成后在此追加日期、提交、实际运行结果和保留的原始失败。）

### 2026-11-16

- 第 1 步研究产出完成：[写入口清点](research/e2-writer-inventory.md)、[锁图](research/e2-lock-graph.md)，均为静态阅读，未运行任何测试。主会话抽查后把 V1–V10 写入 `design.md` §7，并修订了矩阵 B55 的 expected。
- 第 1 步出口（独立 check）进行中，报告将写入 `research/e2-independent-check.md`。出口未过之前不进第 3 步。
- 第 2 步（纯计量）与第 1 步的 check 并行启动：它是无 DB、无 I/O 的纯包，不依赖清点或锁图的任何结论，也没有持久变更。这是对“前一步未过不进下一步”的一处有意放宽，只适用于第 2 步。实现报告将写入 `research/step2-metering-implementation.md`。
- 第 2 步实现完成（未提交）：worktree 的 `server/internal/budget/`，9 个文件。主会话复跑：`gofmt -l` 空、`go vet` 退出 0、`go test -count=1 -v` 54 PASS / 0 FAIL / 0 SKIP、`go test -race` 退出 0、包内无浮点。实现报告见 `research/step2-metering-implementation.md`，选定的读法与一项 S2 前置项记在 `design.md` §7。第 2 步出口（独立 check，含变异测试）进行中，报告将写入 `research/step2-metering-check.md`。
- 任务自有基准库已建：容器 `multica-postgres-1` 里的 `f3_budget_base_865`，从 `template0` 创建并由 worktree 的 `go run ./cmd/migrate up` 迁移，退出 0；账本 853 行、最高 `865_knowledge_automation_origin`、0 工作空间、0 issue、无效索引 0。它只作克隆模板，不直接跑测试；每个包、normal 与 race 各克隆一份，用完即删。任务结束时删除所有 `f3_budget*` 库。
- 第 3 步的迁移约定已核对（F2 的 794–804 是先例）：建表迁移与每个索引迁移分开，索引文件只有一条 `CREATE [UNIQUE] INDEX CONCURRENTLY` 语句；每个并发索引迁移必须在 `server/cmd/migrate/main.go` 的 `concurrentIndexCleanups` 里登记“迁移名 → 索引名”，否则中断的构建会留下不可用索引（2026-11-17 更正：此前写的“全局登记检查仍报 8 个 F2 `report_snapshot*` 缺项”不属实，795–803 都已登记，三条登记检查在 F3 改动前后都通过）。当前最高迁移号是 865，共享 checkout 没有未提交的新迁移；F3 从 866 起编号，合入前要重新核对是否与其他分支撞号。
- 第 1 步出口通过：独立复核报告 `research/e2-independent-check.md`（F1–F14，静态阅读，未运行任何东西）。清点对产品代码完整，约 95 处引用无错行号，V1–V10 无一作废；被推翻的是“根锁排最后”这条规则。主会话对照源码核实了 F1（finalize 尾部语句顺序）、F5（拆除清单测试及其 `retain_audit` 规则）、F6（两个触发器）、F7（CLI 打 origin、`GetIssueByOrigin` 无排序、origin 索引不唯一），然后把修正写入 `design.md`：§5 重写为四条锁规则，§7 追加 V11–V18，§8 定六张表。矩阵 B55、B56、B60 的 expected 同步修订。没有采纳复核对 F3 的两个建议（给成员行加锁、或锁根后重读），理由是成员绑定不换根，见 V13。
- 第 3 步在第 2 步的独立 check 仍在跑时启动：两者改动的文件不相交（`server/internal/budget/` 对 迁移、sqlc 查询与生成代码、拆除清单），第 3 步不依赖计量包的任何内部实现。

### 2026-11-17

- 第 2 步出口通过：独立复核报告 `research/step2-metering-check.md`。复核方按矩阵重算了全部 pure 行（B15、B17–B23、B24 pure、B24a、B25–B30、B48），两个 SHA-256 基准值用 Python 另算一致；113 个变异杀掉 112 个，剩 1 个等价变异；先写测试再修了三处算错账的问题（更正记到别的预留、非法 UTF-8 让重放不一致、零值 `Ledger` panic），测试由 54 增到 69。
- 主会话的两个决定：保留“更正只能落在原条目所属预留下”的新规则（依据合同 §5）；把换算常量改为包内本地常量并加 `scale_test.go`（外部测试包）断言与 `agent.CostUSDTicksPerUSD` 相等，包的非测试依赖只剩标准库。
- 主会话在 worktree 的 `server/` 复跑（输出在 `/tmp/f3-main-step2-final*.txt`）：`gofmt -l internal/budget` 空；`go vet ./internal/budget/...` 退出 0；`go test ./internal/budget/... -count=1 -v` 退出 0，70 PASS / 0 FAIL / 0 SKIP；`go test -race ./internal/budget/... -count=1` 退出 0；`go build ./...` 退出 0；`go list -deps ./internal/budget` 里本仓库的包只有它自己。
- 复核留给后续步骤的条件已写入 `design.md` §7“第 2 步独立复核后的修正”：第 5 步接入前先定账目装载方式（快照或重放上限）；第 6 步要有严格解码、服务端推导授权与释放证明、条目信封列。其中信封列、标识符长度约束和“按序读取一个根的全部条目”三项已转给正在执行第 3 步的代理。
- 第 3 步（`f3-schema`）进行中，实现报告将写入 `research/step3-schema-implementation.md`。
- 第 3 步实现完成（未提交），报告 `research/step3-schema-implementation.md`：迁移 866（六张表）、867–878（十二个并发索引，各一条语句，均已登记到 `concurrentIndexCleanups`）、879（校验十二个索引的形状，先例是 770/773/775）；9 条 sqlc 查询；六张表在拆除清单登记为 `retain_audit`，拆除 handler 无需改动。
- 主会话在两份新克隆上复跑（输出在 `/tmp/f3-main-s3/`，克隆库用后已删，只剩 `f3_budget_base_865`）：
  - `f3_budget_m3_handler` 迁移到 `879_issue_budget_ready`，目录查询得 6 张表、12 个索引、0 个外键、0 个无效索引；`go test ./internal/handler/ -run 'TestIssueBudget|TestDeleteMemberIssueKeepsBudgetRows|TestWorkspaceDeletion|TestDeleteIssue|TestBatchDeleteIssues' -count=1 -v` 退出 0，16 PASS / 0 FAIL / 0 SKIP，其中 `TestIssueBudgetSchemaCatalog`（B51）、`TestDeleteMemberIssueKeepsBudgetRows`（B56）、`TestWorkspaceDeletionManifestCoversPublicSchema` 都是实际 PASS。
  - `f3_budget_m3_migrate`：`go test ./cmd/migrate/... -count=1 -v` 退出 0，67 PASS / 0 FAIL / 1 SKIP（跳过的是既有的 pg_bigm 用例）；`go test ./internal/migrations/... -run 'TestIssueBudget' -count=1 -v` 退出 0，3 PASS。
  - `go build ./...` 退出 0；`go vet` 相关包退出 0。
- 主会话确认的选择见 `design.md` §8“第 3 步落定”。两处与派发说明不同但保留：down 迁移在预算表有行时拒绝回滚（符合 §6“不做破坏性 down”）；多一条校验迁移 879。
- 已知缺口：没有“按 seq 读取一个根的全部条目”的查询；条目的信封是一个可空 `jsonb` 而不是独立列。两项都交给第 3 步的独立 check（`f3-schema-check`，进行中，报告将写入 `research/step3-schema-check.md`）。
- B14（auth 层）原列在第 4 步出口，但它需要第 7 步的 API；第 4 步只在 service 层断言跨工作空间查不到，完整的 B14 移到第 7 步。
- 第 3 步第二轮（实现方处理主会话转去的三项建表输入）完成：账目表的 `envelope jsonb` 换成独立的信封列；标识符长度统一为 1–200 字节；新增 `ListIssueBudgetEntries`。主会话在新克隆上复跑（输出在 `/tmp/f3-main-s3r2/`，克隆库已删）：迁移到 `879_issue_budget_ready`，6 张表、12 个索引、0 外键、0 无效索引；handler 相关用例 17 PASS / 0 FAIL / 0 SKIP；`./cmd/migrate/...` 67 PASS / 0 FAIL / 1 SKIP（pg_bigm）；`internal/migrations` 的 3 条预算用例 PASS；`go build ./...`、gofmt 干净；`internal/budget` 测试通过。独立 check 在第二轮期间只做了不依赖这些文件的部分，现已通知它对稳定后的文件做完整审查与变异测试。
- 第 3 步独立复核通过：报告 `research/step3-schema-check.md`。无缺陷；五种指定变异全部被抓；补了 23 条 CHECK 拒绝用例后 54 条约束全部被钉住；`(task_id, claim_generation)` 唯一键经静态阅读确认不会让领取失败。它留下五项表结构决定，主会话全部采纳（`design.md` §8“第 3 步独立复核后的修正”的 S1–S5），已交实现方做第三轮；第三轮回来后主会话在新克隆上复跑，第 3 步才算出口通过。
- 第 3 步出口通过。第三轮（S1–S5）完成后迁移为 866（六张表）、867–879（十三个并发索引）、880（校验）。主会话在两份新克隆上复跑（输出在 `/tmp/f3-main-s3r3/`，克隆库已删，只剩 `f3_budget_base_865`）：
  - `f3_budget_m3r3_handler` 迁移到 `880_issue_budget_ready`，6 张表、13 个索引、0 外键、0 无效索引；handler 相关用例 17 PASS / 0 FAIL / 0 SKIP，含 `TestIssueBudgetSchemaCatalog`（B51）、`TestIssueBudgetSchemaConstraints`、`TestDeleteMemberIssueKeepsBudgetRows`（B56）、`TestWorkspaceDeletionManifestCoversPublicSchema`、`TestWorkspaceDeletionLeavesBudgetRowsUntouched`。
  - `f3_budget_m3r3_migrate`：`./cmd/migrate/...` 67 PASS / 0 FAIL / 1 SKIP（既有的 pg_bigm 用例）；`internal/migrations` 的 3 条预算用例 PASS。
  - `go build ./...` 退出 0；`gofmt -l` 空；相关包 `go vet` 退出 0；`internal/budget` 70 PASS / 0 FAIL。
  - 实现方第三轮自定的四处选择已确认，记在 `design.md` §8“第三轮落定”。
- B56 的断言在第 4 步会变：删除成员 issue 之后成员行要带 tombstone 时间，其余预算行仍然原样保留。现有的 `TestDeleteMemberIssueKeepsBudgetRows` 由第 4 步一并更新。
- 第 4 步（`f3-binding`）已派发，实现报告将写入 `research/step4-binding-implementation.md`。等待期间主会话定下第 5 步的规则，写入 `design.md` §10。其中新增一个管理项 `attempt_reserve_ticks`（每次执行的声明式预留额）：合同没有规定影子预留的数额，B45 又要求经 finalize 的真实定额预留，人声明是首包里唯一不造假的来源。这一项要向用户说明，提交前可以推翻。
- 主会话审第 4 步代码时发现 V13 的论证有洞：绑定方的在途扫描先于自己的提交，扫描之后才领取、提交之前读完成员行的 finalize 两头都落空。修正为每个工作空间一把绑定锁（`design.md` §10.10，V19），绑定方排他、finalize 共享 try，取不到就让该次领取 requeue。两侧都放到第 5 步实现。这会让绑定事务进行期间同工作空间的领取短暂重试，要向用户说明。
- `f3-binding` 在 08:53 因网关 502 中断（实现和回归已基本跑完，报告未写）。已用 SendMessage 让它续跑收尾，没有重派新代理。
- 第 4 步实现完成（未提交），报告 `research/step4-binding-implementation.md`（13:55 修订版）。主会话在新克隆 `f3_budget_s4_main` 上复跑（输出在 `/tmp/f3-main-s4/`，克隆库已删，只剩 `f3_budget_base_865`）：迁移到 `880_issue_budget_ready`；`internal/service` 的预算用例 22 PASS / 0 FAIL / 0 SKIP；handler 相关用例 25 PASS / 0 FAIL / 0 SKIP；`go build ./...` 退出 0；`internal/service`、`internal/handler`、`internal/budget`、`pkg/db` 的 `go vet` 退出 0；`gofmt -l` 空；`internal/budget` 通过。报告列出的 10 个文件哈希与 worktree 一致。
- 以下是实现方报告的全量结果，主会话没有复跑全量：`internal/service` 全包 1635 PASS / 0 FAIL / 18 SKIP（用真实路径的 `TMPDIR`）；`internal/handler` 全包 5 次里 4 次 3027 / 0 / 53，另 1 次有一条失败 `TestDeliveryCompletionHintFencesPolicyAndStatusUntilCommit`（409 `artifact_authority_busy`）。实现方定位到 `internal/service/delivery_evidence.go:118` 的 `LOCK TABLE … IN SHARE MODE NOWAIT` 抢不到锁，最可能的持锁方是 autovacuum；去掉本次改动、人为持一把冲突锁时同样复现。主会话按环境性失败接受，不算第 4 步的缺陷。同一个 409 在 HEAD 的生产路径上也会出现，是任务外的发现，另行告知用户。
- 实现方的 9 处偏离（D1–D9）全部采纳，写回 `design.md` §9.2–§9.4。实现方没有做的事（它自己列出）：没有死锁压力测试；三条“断言不存在”的 handler 用例没有先跑红；`ListIssueBudgetDescendants` 与 `ListIssueBudgetInFlightTasks` 没有查询计划（后者的 quick-create 分支按 `lower(context->>'parent_issue_id')` 连接，没有索引）。这三项交给独立复核。
- 审第 4 步报告时改定的第 5 步规则（`design.md` §10）：绑定锁在启用里排在全部拒绝检查之后、在改父里只在规则 2 内（§10.10）；`off` 期间只写成员行，回到 `observe` 时写一条永不覆盖的时间段缺口 `observation_off`，取代原先的逐条扫描（§10.6，实现方指出了原写法的一个交错漏洞）；`would_refuse` 的四个固定原因（§10.2）；根的未知集合怎么读（§10.12）；归属按机制、不按 issue 种类（§10.11）；绑定锁取不到时单任务领取端点按“没有任务”应答（§10.10）；第 5 步拆成 5a、5b、5c（§10.13）。
- 这些是主会话定的，提交前用户可以推翻，要向用户说明：(1) 新增管理项 `attempt_reserve_ticks`；(2) 绑定事务进行期间同工作空间的领取短暂重试；(3) 关过一次观察的根此后一直显示未知；(4) 启用时范围内有在途执行的根从第一天起就是未知，设了额度后每次执行多一条 `unbounded_unknown` 观察；(5) `limit` 与 `unbounded_unknown` 只对设了额度的根产生。
- 派发：第 4 步独立复核（`f3-binding-check`，报告写入 `research/step4-binding-check.md`）与 5a（`f3-metering-5a`，报告写入 `research/step5a-metering-implementation.md`）并行。5a 在临时 worktree `/Volumes/artisan/code/2026/mc-f3-budget-5a-20261117`（detached，基于 `86fea651c`，拷入当前的 `server/internal/budget/`）里做；完成后主会话核对、把包拷回 F3 worktree、删除临时 worktree。复核方不得改动 `server/internal/budget/`。
- 主会话把矩阵 62 行逐行对到步骤，发现三处没有归属：B35a 归 5b；B16 与 B24 的 db 半行归第 6 步。上面的出口已补。
- 新增锁规则（`design.md` §5 第 5 条，V20）：到达预算接点的事务必须是 READ COMMITTED。fence 和绑定锁都靠“取到锁之后新发的语句看得到此前已提交的行”，REPEATABLE READ 的快照早于等锁，会读到旧状态。仓库里有 REPEATABLE READ 的写事务（`runProjectTransaction` 等），目前没有发现它们调用绑定函数；已交给第 4 步独立复核逐条核对，5b 在绑定函数里加运行时断言。
- `f3-binding` 15:36 给报告补了第 13 节（前 12 节未动，产品文件哈希未变）。三点裁决：(1) 启用不带开关参数，设计里假设“以 `off` 启用”的两句删掉，新根一律 `observe`；(2) 隔离级别与 V20 结论一致，它没追到的两个调用方（`triage_import.go:443`、`materializeStageChildren` 背后的事务）转给独立复核；(3) 采纳它对启用顺序的建议，5b 改成“绑定锁 → 在途扫描 → 取激活时刻 → 插入”（§10.10）。
- 任务外：`f3-binding` 在一次 hook 的要求下压缩了用户的记忆索引 `MEMORY.md`（23,979 → 16,596 字节）。主会话核对：109 条索引项一条不少（108 条原有加它新写的 1 条），94 个主题文件被追加了原索引文字、没有一个变短或被改写，改动前的目录备份在 `/tmp/f3-s4/memory-backup-20261117/`。要告知用户。

- 等待第 4 步复核与 5a 期间，主会话定下第 6 步的规则，写入 `design.md` §11，并同步改了 §10 里 5b 要用到的几处（协议版本与 `legacy_protocol`、价格快照两列从 attempt 挪到条目、条目身份格式、`limit` 的 detail、hold 原因映射；§10.3 里“入口先调对账”和“第 6 步结算”两句作废）。矩阵 B33、B35、B36、B42 各加了带日期的修订说明。
- 第 6 步里主会话定的、提交前用户可以推翻的几条，要向用户说明：(6) 首包不结算、不释放：每次结束的执行，声明的预留额减去已知支出的部分一直留在 H 里，所以 `would_refuse` / `limit` 会随执行次数累积而提前出现；一个根有过一次结束的执行之后 `remaining` 就为空，设了额度的根此后每次执行都带 `unbounded_unknown`。原因是“scope 完整”和“不可再执行”两项证明没有来源，合同把可靠的释放证明列为 S2 前置。(7) 用量的重叠与成本性质由服务端按证据表定：没有证据证明“只含本次调用”的用量被隔离，不贡献金额；没有实报证明的成本按估算。(8) 首包线上没有更正，一个 attempt 只报一批。(9) 没有成本数字时服务端按现有 metrics 价格表估算，换成整数费率，逐条冻结快照。(10) 没有预算协议的 daemon 执行的任务是永久缺口。
- 派发只读调研 `f3-adapter-usage`（报告写入 `research/step6-adapter-usage-semantics.md`）：逐个 adapter 给出用量来源、新会话与续用会话是否只含本次调用、成本是实报还是估算的证据，外加 daemon 里所有启动 provider 进程的位置。不运行任何 agent CLI，不调用任何 provider。
- 第 7 步的预备事实（规则尚未写进设计）：允许名单不必新造来源标记。认证中间件已经把服务端核实过的凭据种类放进请求上下文（`auth.AuthenticatedPrincipal`，`Kind` 为 `jwt` / `pat` / `task_token` / `cloud_pat`，另有只用于拒绝的 `AgentActor` 与 `Restricted`）；`handler/comment_delegation.go:86` 已有“只认 jwt 或 pat、无 agent 提示头、非受限”的人工判定写法，B58 照这个做，没有 principal 一律拒绝。`mdt_` 只被 `/api/daemon` 的 `DaemonAuth` 接受，进不了管理路由。第 4 步的 service 不解析权限（`issue_budget.go:27`），B41 要求的事务内复核需要在第 7 步补一个事务内的授权步骤。
- 第 7 步的规则写入 `design.md` §12：读写同一条权限规则（人工凭据允许名单 + owner/admin + 根任务还在时的任务加载检查）；service 写操作里加事务内的角色复核，排在原 key 重放之前（B41）；八条路由，没有全量列表；所有应答 no-store、错误正文只有代码；`show` 分列 A/E/R/H 与逐原因未知，有未知时余额为空；另加一个执行列表（合同目标问题里“有哪些执行、哪些预留不能释放”只靠合计答不了）；CLI 照 `issue report-snapshot` 的原 key 约定，不自动重试、不换 key。启用可以同时给额度与预留额（§10.1 补充，归 5b）。
- 复读第 6、7 步规则时补了几处实现时容易读错的地方（`design.md`）：§11.4 第 4 步的批冲突只看 kind 为 `usage_batch` 的操作行（同一前缀下还有 finalize 写的 `attempt_open`）；§11.3 协议检查排在解码之后、开事务之前，并写明完整的检查顺序；§11.6 缺口条目与用量条目同样挂在 attempt 的预留上；§10.4 装载函数不假设调用方持根锁，不持锁的读取（`show`）遇到比自己快照新的缓存时不动缓存、按快照全量重放、结果不发布；§12.1 事务内复核的拒绝与 handler 的拒绝是同一个应答（403 `budget_forbidden`）；§12.1 第 3 项让实现方核实 `UpdateIssue` 在加载之外有没有别的编辑权检查。
- `f3-metering-5a` 在 20:25 动笔写报告时请求超时一次，代码停在 18:52（`f3-binding-check` 同一时段也超时一次，自行续上了）。主会话让它只补报告，同时在临时 worktree 里对 18:52 的包复跑（输出在 `/tmp/f3-main-s5a/v1/`）：`gofmt -l` 无输出；`go vet ./internal/budget/` 退出 0；`go test -count=1 -v ./internal/budget/` 退出 0，89 PASS / 0 FAIL / 0 SKIP；`go test -race -count=1` 退出 0；`go build ./...` 退出 0；`go list -deps` 里本仓库的包只有它自己；非测试文件没有浮点。相对第 4 步的包，非测试文件只改了 `model.go`（`Entry.Unsized`、`ReasonReservationUnsized`）与 `ledger.go`（`With`、`Len`、`HasReservation`、未定额预留及其 `overrun`、结算规则）。条目的存储形式多了一个 `unsized` 键，第 3、4 步里钉住的条目摘要是否受影响等报告说明后再核。
- 主会话补测了首包的真实形状（每次执行一条预留加一条 hold，从不结算；`/tmp/f3-main-s5a/v1/bench-held.txt`，三轮，负载 4–13）：已有缓存的根上追加一次，1 千条目 0.13 ms、1 万 1.1–1.7 ms、10 万 15–16 ms，低于 §10.13 的 5 ms 线；全量重放却是平方的，1 千 0.9 ms、1 万 52–54 ms、10 万 4.9–5.4 s。CPU profile（`/tmp/f3-main-s5a/prof/`）里 59–63% 花在 `applyHold` 对未知列表的线性查重（`hasUnknown`）。首包从不结算（§11.8），每次结束的执行都在未知列表里永久留一条 hold，冷启动后第一次 finalize 要在持根锁的领取事务里付这笔钱。裁定：5a 拷回之前把查重改成常数时间（在预留上记已有的 hold 原因），行为、回执、未知项顺序、存储形式与摘要都不许变，配差分性质测试与变异；结算、累计、更正三处的扫描不动（首包的生产路径到不了），克隆与投影的线性成本不动。已发给 `f3-metering-5a`，报告里单列一节。
- 主会话读完 5a 报告（`research/step5a-metering-implementation.md`，53,029 字节）后的裁定，已写进 `design.md`（§10.1、§10.2 补充、§10.4、§10.5、§10.8、§10.13、§12.5）并发给 `f3-metering-5a` 做第二轮：
  - 问题 2：条目的存储形式多了 `unsized` 键，但版本仍是 1、摘要的域仍是 `budget-entry-v1`。迁移 866 没有提交过，测试库之外没有旧形式的行。版本做成纯计量包的常量 `EntryFormVersion`，包里加 `EncodeEntry` 与严格的 `DecodeEntry`（只认当前版本；字节必须恰好是解出的条目重新编码的结果），外加身份 200 字节、存储形式 65,536 字节两个上限。
  - 问题 3：设计没有写明、实现方自己取的几处（未定额配非零金额的错误类别、未定额算一次纳入的执行、金额为 0 的定额预留是另一个事实、未定额预留上的 hold、结算之后晚到的用量）全部照实现确认。一次结束的未定额执行在未知集合里占两项，按原因数的是未知项，不是执行数。
  - 问题 4：`Apply` 改成包内私有。包外只有 `With`，缓存里的状态不可能被原地修改，由类型保证。第 4 步的 worktree 里包外只用到 `budget.Reduce`（8 处，全在测试里），没有调用方要改。
  - 第 7 步要的按预留读取方法（§12.5）提前到这一轮加（`Reservation`），并用“各预留之和等于投影”的不变式钉住。
  - 缓存上限是 service 包的常量，初值 5 万条目（约 40–65 MB）；单个根超过上限时不缓存、每次全量重放并记 warn。
  - 5a 报告的第 6、7 条写成 5b 的规则：服务端写的 hold `detail` 为空；回执不是重放的条目每条恰好一行，写完 `Len()` 必须等于新的 `ledger_revision`。第 8 条（挂在预留上的缺口挡住结算）此前已在 §11.6 定为有意为之。
  - 没有采纳：条目身份的拼接函数放进纯计量包（它们留在 service）、不拷贝未知列表的读取方式。
- 核对发现：21:17 发给 `f3-metering-5a` 的“常数时间 hold 判定”还压在它的收件箱里（它的转录里没有这条消息，它在 22:48 又被压缩了一次，仍在收尾）。第二轮的消息把那一条列为第 1 项，并放宽了它的文件范围（`ledger.go`、`model.go`、可加一个存储形式的非测试文件、测试文件）。第二轮回来之前 5a 不算完成，包不拷回。
- 2026-11-18：`f3-metering-5a` 交回第二轮第 1 项（常数时间的 hold 判定），补充报告在 `research/step5a-metering-implementation.md` 第 513 行起（H.1–H.11）。第二轮的消息在它 05:44 结束那一轮之后才送达（05:45），第 2–4 项此刻才开始；消息只在收件方一轮结束时投递，轮内压缩不会投递。
- 主会话对第 1 项的独立复核（输出在 `/tmp/f3-main-s5a/v2/out/`）。做法：把交付的 17 个文件拷出一份快照（sha256 与报告 H.11 的表逐个相同），放进 `git archive 86fea651c server` 导出的树里跑，不占用它正在改的临时 worktree。
  - `gofmt -l` 无输出；`go vet ./internal/budget/` 退出 0；`go test -count=1 -v` 退出 0，99 PASS / 0 FAIL / 0 SKIP；`go test -race -count=1 -v` 退出 0，同样 99 / 0 / 0，没有 DATA RACE；`go test -race -count=5 -shuffle=on` 退出 0；`go list -deps` 里本仓库的包只有它自己；非测试文件没有浮点（`math` 只用到 `MaxInt64` / `MinInt64`）。没有跑整个 server 的 `go build ./...`，也没有跑第 4 步的库用例，这两项留到拷回时做。
  - 读改动：`ledger.go` 相对第一轮只多了预留上的一个 `holdSet`（每个 hold 原因一位）和 `applyHold` 里的判定，`hasUnknown` 删除；与报告里的 diff 逐行相同。逐处核对了未知列表的全部写入点与删除点：带 hold 原因的项只由 `applyHold` 加入，只由结算删除（结算后先看状态，位留着无妨）；用量一侧的三处删除按范围或条目身份匹配，碰不到 hold 的项；缺口条目的原因被校验限制在两个非 hold 的常量上。
  - 新旧对拍（主会话自己写的生成器，`/tmp/f3-main-s5a/v2/zz_mainsession_transcript_test.go`，不进仓库）：2,000 个种子、每个 160 次调用，共 320,000 次 `With` 调用、520,488 条条目，其中接受的 hold 39,884 条、因原因已记录而 stale 的 14,499 条、因已结算而 stale 的 9,315 条。第一轮的 `ledger.go` 与交付的 `ledger.go` 逐步记录回执、错误文本、`Len()` 与投影，两份摘要文件逐字节相同（sha256 `6a7edd4d…`）。对照：删掉“记下这一位”那一行的变异在种子 1 第 16 步就失败（同一预留同一原因在未知集合里出现两次）。
  - 基准（held 形状，`-count 3` 取中位数，负载 3.1–4.0）：重放 1 千 / 1 万 / 10 万条目，改前 0.82 ms / 48.6 ms / 5.54 s，改后 0.39 ms / 3.37 ms / 45.3 ms；装载（解码加重放）改前 2.79 ms / 68.2 ms / 4.93 s，改后 2.34 ms / 23.2 ms / 236.9 ms。与报告 H.8 一致。
- 读补充报告之后写进 `design.md` §10.4 的两条（5b 要照做）：全量重放与尾部都把全部条目交给一次 `With` 调用（逐行调用每次整体克隆，是平方的）；超过缓存上限的根每次装载按每条目约 2.3–2.5 µs 付费。没有采纳、留作记录的一条：把五个 hold 原因收成一张表供校验、`isLifecycleReason`、`holdBit` 三处共用（H.10 第 1 条）；现在由 `TestEveryHoldReasonHasItsOwnBit` 钉住，漏配只会让未知项重复、不影响金额，S1 不改。

### 2026-11-18

- 第 4 步复核草稿里的 N3、N4 已裁定并写进 `design.md` §10.2 与 §6：预算步骤里的工作空间取自 issue 一侧（普通 task 按主键读 `issue.workspace_id`，quick-create 取服务端存下的 `context.workspace_id`），不取自 agent、runtime 或 task token；在途扫描的 issue 分支改成连 `issue` 表过滤工作空间，quick-create 分支保留 agent 连接并把“带父任务的 quick-create，其 agent 与父任务同工作空间”写成前置条件；quick-create 的判定与 `parseQuickCreateContext` 共用一份定义。计划对比在 6 万行的库上做（没有探针索引，暖缓存）：改前 1.2–1.7 ms，改后 1.3–1.5 ms，同一条索引路径。迁移编号 866–880 当天复查无冲突。其余几条（N1、N2、D1、N6、索引提议、静态守卫）等复核报告定稿后一并裁定。
- 第 6 步调研 `f3-adapter-usage` 完成，报告 `research/step6-adapter-usage-semantics.md`（907 行）。只读代码与厂商文档，没有运行任何 agent CLI，没有调用任何 provider。主会话抽查：厂商页面自己重取了一遍，引用的句子都在所引的行上；`claudeBlockedArgs` 确实不拦会话参数；result 用量确实整体替换此前的累加；daemon 里的两处启动点、合成结果的分支、自定义 runtime profile 的界线都对照源码读过。
- 证据表裁定，写进 `design.md` §11.6：只有 `claude` / normalizer `1` 一行，新会话已证明只含本次调用，续用会话未证明，成本未证明实报。其余 23 个实现不进表。厂商源码仓库某次提交不采信为 codex 的入表证据。qwen 离入表最近，记了原因。
- 同时定下、写进 §11.5、§11.6、§11.8、§11.10、§11.11 的几条，6a 与 6b 要照做：
  - `resumed` 为 false 的条件：没有续用标识，而且 daemon 级参数与 agent 自定参数都为空。
  - `adapter`：自定义 runtime profile 的命令写 `custom-<provider 键>`，永不进证据表。
  - `ended` 按结果的来源与状态取四种情形；首包不改 `executeAndDrain`。
  - 模型目录发现、版本探测、openclaw 配置命令不算调用。
  - 证据表可注入，生产表另有一条用例钉住。
  - 条目的 `adapter_version` 列存 `<adapter>/<版本>`；操作行 `result` 记下每次调用的 `resumed`、`ended` 与套用的证据表结论。
- 这次裁定带来的后果，提交前用户可以推翻，要向用户说明：
  - (13) 首包生产环境里实际支出 A 恒为 0。没有一个 adapter 的成本被证明是提供方实报。
  - (14) 金额只有服务端估算，而且只来自 claude 的新会话调用。同一 issue 上的后续执行通常续用会话，它们和其余 23 个实现的用量都被隔离，根显示为未知。
  - (15) 配了自定启动参数的 claude agent（daemon 级 `MULTICA_CLAUDE_ARGS` 或 agent 的 `custom_args`）一律按续用处理，得不到估算。
  - (16) 没有可用 `result` 事件的 claude 调用，token 数可能不准，只影响估算。
  - (17) 要扩大覆盖（codex、续用会话、ACP 各家），需要真实 CLI 运行的抓取，会消耗额度，本任务没有授权；或者改 adapter、做账单对账，都在首包范围之外。
- 6b 提前派出（`f3-daemon-6b`），与第 4 步复核、5a 第二轮并行：它不依赖第 5 步的服务端代码，只改 daemon。位置是新建的临时 worktree `/Volumes/artisan/code/2026/mc-f3-budget-6b-20261118`（detached，`86fea651c`），F3 worktree 没有动过 `internal/daemon`、`pkg/protocol`、`pkg/agent`，做完按补丁拷回。为此把三样东西定死并写进 §11：能力常量名 `DaemonCapabilityBudgetUsageV1`、入口路径、daemon 对各状态码的处理。报告写到 `research/step6b-daemon-implementation.md`。服务端入口还不存在，它的用例对着假服务端跑；不运行任何 agent CLI。
- 第 4 步出口通过。独立复核报告 `research/step4-binding-check.md`（结论：通过，带一处低严重度的产品修正和 11 项待裁定）。
  - 复核方的改动（都在 F3 worktree，未提交）：`issue_budget.go` 的 `manage` 对负的 `expected_policy_revision` 先行拒绝 `invalid_request`（D1，此前从意图摘要的编码里冒出普通错误）；`issue_budget_test.go` 加 8 条用例和 1 个子用例，修了一处会挂到超时的用例；`issue_budget_binding_test.go` 三处更新改成保留父任务，让“只在父任务变化时绑定”有一次红的运行；新增并发压力用例 `TestIssueBudgetConcurrentWritersKeepEveryTreeBound`（12 棵树，启用落在写入的不同位置，一半的树上两个启用相争）；新增不连库的静态清点守卫 `TestIssueBudgetWriterInventoryIsComplete`。
  - 变异：128 个变异加一个恒等对照，全部经 `go test -overlay` 在新克隆上跑，不写 worktree。交付时的用例放过 22 个（service 19、handler 3），补用例后全部被杀。压力用例单独判：它能发现真实的锁环（`40P01`）和启用漏取 fence，发现不了还没遇到对手的半个顺序违规，所以语句顺序用例仍是锁顺序的主要守卫。守卫的 18 种改法全红。
  - 接点完整性：没有语句、Go 调用方、裸 SQL、触发器、规则或数据库函数在清点之外创建 issue、写 `parent_issue_id` 或删除 issue。
  - 全量是复核方跑的，主会话没有复跑全量：`internal/service` 1644 PASS / 0 FAIL / 18 SKIP，`internal/handler` 3028 / 0 / 53，各两遍，第二遍走仓库的 agent CLI 守卫入口，0 次 CLI 调用。链接到被改包的 35 个包里跑绿 30 个，2 个没有测试，3 个没跑（`cmd/migrate`、`internal/migrations`、`cmd/backfill_codex_usage_cache`）。skip 都有原因（没配 Redis 等），没有一条是数据库 skip，没有一条预算用例被跳过。
- 主会话对第 4 步最终状态的独立复跑（输出在 `/tmp/f3-main-s4v/`）。模板 `f3_budget_s4v_tmpl` 由 `f3_budget_base_865` 克隆并迁移到 `880_issue_budget_ready`：账本 868 行，6 张表，13 个索引，0 外键，0 无效索引。每组用一份新克隆，用后即删。
  - 13 个文件的 sha256 与报告第 16 节逐个相同；迁移 866–880 的 30 个文件串接摘要 `082fb2ea…` 相同；`git status --porcelain` 52 行，HEAD `86fea651c`。
  - `go build ./...` 退出 0；`go vet`（`internal/service`、`internal/handler`、`internal/budget`、`pkg/db/...`、`cmd/migrate`、`internal/migrations`）退出 0；`gofmt -l` 空。
  - `./internal/service/ -run TestIssueBudget`：31 PASS / 0 FAIL / 0 SKIP；加 `-race` 同样 31 / 0 / 0，没有 DATA RACE。
  - handler 的预算选择（`TestIssueBudget|TestDeleteMemberIssueKeepsBudgetRows|TestWorkspaceDeletion|TestDeleteIssue|TestBatchDeleteIssues`）：26 / 0 / 0；加 `-race` 同样，没有 DATA RACE。
  - 复核方没跑的两个包：`./cmd/migrate/...` 67 / 0 / 1（跳过的是既有的 pg_bigm 用例）；`./internal/migrations/... -run TestIssueBudget` 3 / 0 / 0。`cmd/backfill_codex_usage_cache` 没跑：它的集成用例自己建库、库名不带任务前缀，而它只链接生成包里新增的文件。
- 对复核 11 项的裁定，已写进 `design.md`（§5 第 5 条的增补、§6、§9.4、§10.2 的补充、§10.13、§12.7）：
  - N1（隔离级别只靠服务器默认值）：预算 service 自己的 `transact` 在第一条语句声明 READ COMMITTED，5b 做；其余到达接点的事务与 finalize 不改开启方式，保留“读到成员行之后断言”。部署前提是数据库默认 `read committed`。
  - N2（混合版本窗口）：首包不加开关或兼容路径，规则是全部实例换成带接点的版本之后才启用预算根；升级或回退之后跑报告 5.4 的两条只读查询核对（第二条是复核在裁定之后补的，见下面 14:4x 的记录）。
  - D1 接受 `invalid_request`。N6：5b 给意图摘要钉一条固定向量。N18、N19：设计里的两份清单各补一项，主会话对照源码核实过。
  - 在途扫描的专用索引不加（6 万行各形态 8 ms 以内，30 万行 16–30 ms）。静态守卫保留，改接点签名或增删写入口时显式更新它的清单。
  - 报告 13.2、13.3 列出的、后面各步会改到的第 4 步用例和代码，分派到 5b、5c 和第 7 步（§10.13 的同日补充）。其中一项留给主会话：5b 实测绑定方排队时一条 task 被拒几次，再决定领取被拒之后要不要加短延迟。
- 保留下来的两样东西：
  - 可达性工具的源码与输出拷到 `research/step4-reachability/`（`main.go`、`go.mod`、`go.sum`、`out.txt`；依赖 `golang.org/x/tools v0.48.0`，用法 `cg <server 目录>`）。第 8 步复核“没有 REPEATABLE READ 事务到达接点”时重跑，届时把 finalize 用到的预算函数加进它的接点清单。
  - 数据库 `f3_budget_s4c_plan`（136 MB，22 个工作空间、6.5 万 issue、6 万行队列、5.7 万成员行）留给 5b 量在途扫描改写之后的计划，只当克隆模板用。任务结束时与其余 `f3_budget*` 库一起删。
- 这次裁定带来的后果，提交前用户可以推翻，要向用户说明：
  - (18) 预算要求数据库的 `default_transaction_isolation` 是 `read committed`。不成立时，每棵树上的第一个洞是静默的（子任务没有成员行，或一次执行既没有 attempt 也没有缺口）；之后这棵树上读到成员行的写入和领取会失败，是响亮的。
  - (19) 全部实例升级到带接点的版本之后才能启用预算根。有根之后回退服务端会重新打开窗口。升级或回退之后要跑两条只读查询（漏掉的绑定、漏掉的墓碑），结果都应为空。
  - (20) 任务外的选项：启动时检查数据库的默认隔离级别。它管的是整个产品（issue 编号分配、删除一个 iteration 归属刚被并发改过的 issue，在别的级别下本来就会失败），本任务不做。
- 5a 第二轮第 2–4 项的代码在 08:11 之后没有再变（实现方此后只在做变异、基准和补充报告，补充报告此刻还没落盘）。主会话没有等报告，先核对代码并拷回：
  - 读改动（对照主会话 11-18 早上留的第 1 项快照）：`ledger.go` 只是 `Apply` 改成包内的 `apply`、新增只读的 `Reservation` 与状态名映射、`Digest` 挪走；`model.go` 多了 `ErrInvalidStoredForm`、`ReservationStatus`、`ReservationProjection`；新文件 `stored.go`（`EntryFormVersion`、两个上限、`EncodeEntry`、`DecodeEntry`、`Digest`）；`decimal.go`、`price.go`、`usage.go` 逐字节未变。`DecodeEntry` 的严格性靠“解出来再编码必须与原字节相同”，重复键、多余键、空白、非法 UTF-8、尾随内容都因此被拒。
  - 在 `git archive 86fea651c server` 导出的树里验证（`/tmp/f3-main-s5a/v3/out/`，21 个文件）：`gofmt -l` 空；`go vet` 退出 0；整个 server `go build ./...` 退出 0；`go test -count=1 -v` 112 PASS / 0 FAIL / 0 SKIP；`-race` 同样 112 / 0 / 0，没有 DATA RACE；依赖只有标准库；非测试文件没有浮点；导出的方法里没有 `Apply`。
  - 拷回 F3 worktree 的 `server/internal/budget/`（旧的 12 个文件备份在 `/tmp/f3-main-s5a/v3/f3-budget-before/`），`diff -r` 与临时 worktree 相同。在 F3 worktree 里（输出在 `/tmp/f3-main-s5a/v4/` 与 `/tmp/f3-main-s4v/post5a_*`）：`go build ./...`、`go vet ./...` 退出 0；纯计量 112 / 0 / 0，`-race` 同；新克隆上 service 的预算用例 31 / 0 / 0、handler 的预算选择 26 / 0 / 0（含复核 N11 点名的 `issue_budget_delete_test.go`），`-race` 各再跑一遍同样，没有 DATA RACE。
  - 还没做：读补充报告、基准复测、删除临时 worktree。做完才给 5a 打勾。
- 设计补了一条规则（`design.md` §10.2“没有事务时的规则”）：`TaskService` 没有 `TxStarter` 时 `runInTx` 不开事务，预算步骤此时不取锁、不写预算行，只查成员行；查到成员行就让 finalize 失败并返回专门的错误，没有成员行照旧。生产构造只有一处，传的是连接池；受影响的只会是用字面量构造 service 的测试，5b 第二轮要报告有多少现有用例走这条路径。
- 发现一个验证缺口并补上：仓库按 Go 1.26.x 构建（`server/go.mod` 写 `go 1.26.6`，CI 是 `1.26.x`，镜像是 `golang:1.26-alpine`），而本机默认的 `go` 是 1.27.0，此前第 2–4 步的全部验证只在 1.27.0 上跑过。主会话用模块缓存里的 1.26.9（`GOTOOLCHAIN=go1.26.9 GOPROXY=off`，不联网）在 F3 worktree 上补跑（输出在 `/tmp/f3-main-s4v/go1269*`）：`go build ./...`、`go vet ./...` 退出 0；纯计量 112 / 0 / 0，`-race` 同；新克隆上 service 的预算用例 31 / 0 / 0、handler 的预算选择 26 / 0 / 0，`-race` 各再跑一遍同样；`internal/migrations` 的预算用例 3 / 0 / 0；没有 DATA RACE。整包（service 1644、handler 3028）没有在 1.26.9 上跑，留到第 8 步的收尾验证，那一步两条发行线都跑。此后每一步的出口验证都要带上 1.26.9。
- 5a 装载路径的基准由主会话复测（`git archive 86fea651c` 导出的树加 5a 的 21 个文件，Apple M4，负载 3–5；`/tmp/f3-main-s5a/v3/out/bench-load*.txt`）。连同严格解码的装载，每条目：Go 1.27.0 是 3.9–4.6 µs（10 万条 0.40–0.43 s），Go 1.26.9 是 5.4–6.2 µs（10 万条 0.57–0.62 s）。差别在 JSON 解码：不做重编码比对的解码 1.27.0 约 2 µs、1.26.9 约 4 µs。重放本身两条线相同（首包形状 10 万条 42–43 ms）。`design.md` §10.4 里“5 万条目约 0.12 s”是 1.27.0 加宽松解码的数字，按仓库实际的发行线应是约 0.3 s，等 5a 补充报告读完一并改。mixed 形状仍是平方的（10 万条重放 3.6 s），原因是结算与累计用量对未知列表的扫描，这是 §10.4 已经写明留给 S2 的那一项，首包的生产路径到不了。
- 复核方在主会话裁定之后又补测了三件事（报告 12.3 的五次追加运行、5.4 的第二条查询；主会话 14:3x 读了它与 11:32 快照的差异，对照测试源码、墓碑查询和 `Reopen` 的拒绝条件核实），`design.md` §5、§6 同步更正：
  - 更正一句事实：别的默认级别下返回 500 的不是“删除 iteration”，而是删除一个 iteration 归属在等 fence 期间被并发改过的 issue；起因是 40001，去掉墓碑接点结果相同，与第 4 步无关。
  - 第三种静默残留：默认级别不是 READ COMMITTED 时，删除与启用竞态会留下没有墓碑的成员行，删除照常返回 204；被删的是根 issue 时根不会被标成已删除。墓碑是不先读取的更新，断言拦不住；不为它给每个删除事务加一次往返，仍归部署前提。
  - 核对查询是两条：漏掉的绑定（父任务是成员、子任务不是）和漏掉的墓碑（成员行没有墓碑、issue 已不存在）。旧实例删除成员 issue 必然产生第二种，不是竞态。两条都只探测、不修复。
  - 留给 5b 之后“领取被拒要不要加延迟”那项决定的输入（复核方的阅读结论，没有跑）：默认级别不对时，finalize 里的断言会让有预算的 task 每次领取都失败，领取、500、requeue、广播、唤醒、再领取，没有节流，只要配置不改就一直转。它是报告 13.3 第 3 点的无界形式；产品里别的持续性领取失败已经是这个模式。
- 第一条 6b 车道（`trellis-implement`）在 15:2x 被主会话停掉，五个小时没有写出任何文件。原因是派发方式：hook 把 `implement.jsonl` 里的 13 个文件共 273 KB 整段注入到它的上下文，它起步就占了约 137k token，读完设计与源码就被压缩，压缩后重读，一共四次。它的阅读没有丢：最后一份工作摘要里有全部接点的行号与原文、测试夹具的写法和一份完整的实现方案，主会话抽成 `/tmp/f3-main-s6b/prior-hook-points.md` 与 `prior-plan.md` 交给下一条车道。教训已记进记忆条目（`trellis-context-budget-is-adjustable`）：大任务的车道用 `general-purpose` 派发（不经 hook 注入），提示词只指向磁盘上的说明书，说明书按小窗口写（分阶段阅读、笔记落盘、二十次工具调用内出报告初稿）。`config.yaml` 与两份清单不动。
- 5b 在 14:2x 按这个方式重派（`f3-finalize-5b`，说明书 `/tmp/f3-main-s5b/brief-5b.md`），分两轮：第一轮做表结构、管理操作、绑定锁、账目装载（阶段 A–D），主会话复核后再发第二轮（finalize 十步、两个调用点、入口清点）。16:30 时它的笔记与报告都在按阶段更新。
- 5a 补充报告 P.10 的九项决定由主会话裁定，规则写在 `design.md` §10.4（三条新规则加一处数字更正）：装载耗时的数字按两条发行线实测更正；存储形式依赖 `encoding/json` 的编码字节稳定，以三条钉字节的用例为闸，Go 升级后变红就不发布、改写包内自己的编码器，不改期望值；`*a = *b` 的整账目赋值不拦，5b 复核时在 service 包里 grep；两个上限只由 `EncodeEntry` 知道，`With` 不重复检查；其余五项（先量 ID 长度再判、单一哨兵错误、第 2 步形式的行被拒、`ReservationProjection` 的四个取值）照实现。自写编码器、无分配解码器、不透明句柄三项提议不做。
- 复核方（`f3-binding-check`）在写完报告第 18 节之后只剩排版检查，16:3x 由主会话停掉以腾出名额。第 18 节（R1–R8）读完：R1–R3、R5、R6、R8 与主会话已经改进设计的内容一致，只补了 R3 的一个细节（§5 第三种残留里“一直不响的只是被删的这一行，树的其它成员照样会响”）；R4 留作 5b 之后那项决定的输入；R7 提醒 `f3_budget_s4c_plan` 的预算表是第 4 步的形状，5b 改过 866 之后要看 5b 的查询计划得另建克隆。
- 重派 6b 之前，主会话对着源码（`daemon/daemon.go:8486-8940`、`:5122-5262`、`:1959-2071`，`health.go:86-95`，`cancel_receipts.go:181-210`，`handler.go:705`）裁定了上一条车道提出的出入，写进 `design.md` §11.10：
  - `ended`：带知识上下文时等到的 backend 结果（`:8897-8901`）算 backend 自己的；`:8905` 合成的知识失败算合成（`interrupted`、用量为空）；“返回知识检查失败的结果”只指 `Execute` 报错那一处；`settleKnowledge` 改写后的状态按交回时的算。设计原句把整段 `drainCtx` 分支都算成合成，是错的，已更正。
  - `handleTask` 在 `runner.run` 之前的两处提前返回不建记录、不发批（其中一处连 provider 都不知道，填不出 `adapter`），记为已知局限。
  - `unknown` 调用的 `observed_at` 取启动意图时间；错误代码取应答的 `code` 字段（6a 的错误应答用 `writeErrorCode`）；不跟随重定向。
  - 新增一条上一条车道没有的规则：记录只发回建它的服务端（记录里记服务端地址，不同就不发、留到 7 天上限），免得 profile 改指别的服务端之后把上一个服务端的模型名与 token 数发过去。
  - 恢复排在 `listenHealth` 绑定端口之后；两个 daemon 用不同健康端口共用一个 profile 目录属于配置错误，记为已知局限。
- 6b 在 16:3x 重派（`f3-daemon-6b2`，`general-purpose`，说明书 `/tmp/f3-main-s6b/brief-6b.md`，临时 worktree `/Volumes/artisan/code/2026/mc-f3-budget-6b-20261118`），同样分两轮：第一轮做常量与字段、回传记录与组批、调用的采集（阶段 A–C，生产路径不打开日志，行为与现在完全相同）；主会话复核后第二轮做能力声明与发送、后台重发与重启恢复。验证增加两项：Go 1.26.9 上重跑构建、vet 与测试；Windows x64 与 Linux 的交叉编译（daemon 随桌面端发到 Windows，本机跑不了，只能以编译为准）。
- 5a 收尾（19:59）。子代理 19:45 自己结束；补充报告 `research/step5a-metering-implementation.md` 的 P.1–P.12 读完（文件 196 KB，“Addendum: second pass”之后是现状，P.11 列出它取代了前文哪些说法）。报告末尾 21 个文件的 sha256 与 F3 worktree 的 `server/internal/budget/` 逐个相同，临时 worktree `mc-f3-budget-5a-20261117` 里除这 21 个文件没有别的东西，已用 `git worktree remove --force` 删除。它另外写了一条记忆 `local-go-is-newer-than-the-repo-toolchain`（本机 `go` 是 1.27.0、仓库用 1.26.x，怎样不下载就用 1.26.9），内容属实，保留；记忆目录除新增这一条之外没有别的变化（对照 `/tmp/f3-s4/memory-backup-20261117/`）。
- 这份报告不登记进 `implement.jsonl` / `check.jsonl`：注入钩子对每个文件只取前 65,536 字节，而这份报告的前 64 KB 正是已被取代的第一轮文字（`Apply` 还是导出的、89 个用例、只在 1.27.0 上测）。规则都已经在 `design.md` §10.4，后面各条车道需要的 API 表写在各自的说明书里。
- 5a 在收尾时多测了一项，主会话在 Go 1.26.9 上复测得到相同数字，据此更正了 `design.md` §10.4：按条目数计的缓存上限给不出“约 40–65 MB”的内存上界，因为根本身有固定开销（只有一条预留的根占 2.8 KB，5 万条目全在单条目的根里要 138 MB）。改成按权重计：每个根的权重是条目数加 2，上限 5 万指权重之和，账目状态的上界约 70 MB；5b 用 10.2 的身份格式实测之后换成实测值。这条在 5b 第一轮（账目装载与缓存）复核时带给它。
- 趁 5b、6b 第一轮还在跑，主会话备好了后面要用的三份材料（都在 `/tmp`，重启会丢，丢了按 `design.md` 重写即可）：5b 第二轮的续派消息 `/tmp/f3-main-s5b/round2-5b.md`、6b 第二轮的续派消息 `/tmp/f3-main-s6b/round2-6b.md`、5c 的说明书草稿 `/tmp/f3-main-s5c/brief-5c.md`（同样两轮：先对账函数与 sweeper 接入并交出 hold 原因映射表，主会话确认后再做 `off` 规则与 quick-create 差异）。6b 第二轮新增一项：线上格式的两份共享样例 `server/pkg/protocol/testdata/budget_usage_v1_batch.json` 与 `…_batch_empty.json`，6b 的组批用例与 6a 的严格解码用例都对着这两份字节，两边分头实现也不会悄悄走样。
- 18 日夜里主会话在两条车道运行期间做的复核与裁定（记在 19 日一节）。

### 2026-11-19

- 5b 第一轮的阶段 A–C（表结构与查询、管理操作、绑定锁的绑定方一侧）主会话读完全部生产代码与用例，没有发现缺陷。三条意图摘要的固定向量在 Go 之外重算，相同。绑定锁的键与 fence 同种子不同文本；仓库里所有种子为 0 的咨询锁键逐个调用方查过，没有哪一个能拼出这两段文本，剩下的只是 64 位哈希碰撞，后果是多等一次，不会记错账。记录在 `/tmp/f3-main-s5b/review1/notes.md`。
- 5b 阶段 D（账目装载与缓存，`internal/service/issue_budget_ledger.go`）生产代码读完，与 §10.4、§10.5 相符，一处改规则，写进 §10.4 末条：
  - “账目不可用”只由全量重放判定。从缓存状态加尾部装载时校验不成立，先丢弃缓存、在同一次装载里全量重放，仍不成立才算不可用。原实现在缓存与库对不上时（库被还原、运维修过行而进程还留着旧前缀）会把一个健康的根报成不可用，finalize 随之写下永不算已覆盖的 `ledger_unreadable`。丢弃时记一条 warn。带进 5b 第二轮。
  - 5b 报告的三项待裁定都接受：R1 读取方装载失败只丢自己用过且仍在缓存里的状态，持锁方丢掉现有的；R2 第 5 步里不持锁的读取不发布，第 7 步的 `show` 要不要发布由第 7 步定（§12.4 允许）；R3 条目行的 `attempt_id` 只取自条目的预留，无预留 attempt 的用量条目由 6a 显式传入（§11.6）。
- 6b 第一轮的阶段 A–C 主会话读完三份生产文件、四处 diff 与三份测试文件（`budget_usage_test.go`、`budget_usage_capture_test.go`、`budget_usage_runtask_unix_test.go`），接点与 §11.10 相符。主会话在仓库的 agent CLI 守卫脚本下跑了 `TestBudgetUsage*`：33 PASS / 0 FAIL / 0 SKIP，0 次本机 CLI 调用（假 provider 是测试自建的 shell 脚本）。三处小改带进第二轮：`snapshot` 对空 attempt 不安全；`adapter_version` 不合规时应当不带该字段（§11.10 的 2026-11-19 补充）；日志写失败后留下的旧文件在重启后可能以同一个 key 发出不同字节，发送侧要把 `operation_conflict` 当永久错误处理并有专门用例。
- 6a 派发前定下的细化，写进 `design.md`（线上格式不变）：
  - §11.5：同一批里 `invocation_id` 不得重复；时间按 `time.RFC3339` 的解析为准；控制字符指 `unicode.IsControl`，按解码后的字符串判，原始字节先整体验 UTF-8；键名逐字节匹配（`encoding/json` 不分大小写，要自己查）；必填字段给 `null` 拒绝；请求体超 256 KiB 答 413，调用数或用量项数超限答 400。
  - §11.6：重叠与成本是两条独立的轴，被隔离的用量条目照样按四条规则定成本并带估算与快照；估算算不出来（价格换不成整数费率或溢出）按查不到价格处理。
  - §11.4：用量批的意图摘要是域前缀 `budget-operation-intent-v1\n` 加一个恰有五个键的 JSON 对象（`kind`、`workspace_id`、`scope_kind`、`scope_id`、`payload_digest`），用自己的结构体；摘要可以在 Go 之外手工算出，6a 用这样算出的定值钉住。
- 6a 的说明书写完并通读一遍（`/tmp/f3-main-s6a/brief-6a.md`，704 行，两轮：先严格解码、证据表、价格源、归一化这些不连库的纯代码，主会话复核后再做入口事务、查询与连库用例）。还有五处占位要等 5b、5c、6b 的结果：两份共享样例的摘要、5b 的账目接口、建 attempt 的测试夹具、sqlc 二进制的位置、起点的用例计数。
- 这次裁定带来的后果，提交前用户可以推翻，要向用户说明：
  - (21) daemon 组批时不校验模型名、调用数、用量项数。模型名超过 120 字节或含控制字符、一次执行超过 32 次调用或 256 条用量项时，服务端整批拒收，daemon 丢弃记录，那次执行靠 hold 保持未知。claude 的 adapter 产生不了空模型名（两条路径都跳过没有模型的用量），生产证据表里只有它，所以这条边界在首包的生产数据上基本走不到；写进 spec。
  - (22) 还原数据库之后必须重启全部服务端进程。(“全量重放判定”能发现大多数缓存与库对不上的情形，但缓存的 revision 不大于根行、内容不同而六个投影数字恰好相同的状态查不出来。) 第 8 步写进 spec。
  - (23) 一条被隔离的用量条目仍然带着“当时按什么价估过”的快照，但不贡献任何金额。读的人会在条目上看到估算值而在根的合计里看不到，`show` 的文案（第 7 步）要说清。
- 5b 阶段 D 的用例文件（`issue_budget_ledger_test.go`，15 个顶层用例、带子用例 56 个）主会话读完，没有发现缺陷；54 个变异全部被杀（5b 的记录，尚未独立抽查）。读用例时补定三件事，都放进 5b 第二轮的发现稿（`/tmp/f3-main-s5b/findings-draft.md`）：
  - 持根锁的调用方发现缓存 revision 大于根行（库被还原）而丢弃缓存时，现在一声不响；改成与“全量重放判定”同一条 warn，原因单列。已写进 §10.4 末条。
  - 缓存的权重只管条目数，不管字节。5b 实测（Go 1.26.9，§10.2 的身份格式）：服务端会写的各种形状每单位权重最多 1,838 字节，按 2,000 计 5 万权重约 100 MB；但存储形式允许到 65,536 字节，全是这种条目时是 3 GiB，§10.4 说的“严格上界”并不成立。裁定：所有写入都经过的 `appendEntries` 拒绝存储形式超过 2,048 字节的条目（首包最大的条目是带估算的用量条目，主会话当时实测 1,093 字节，04:40 补测后更正为最大 1,708 字节，见本节后面“5b 第一轮交回”一条；装载不设这道上限，别的版本或运维写下的行照读）。硬上界等 5b 第二轮用“全是上限条目”的一格实测后写进 §10.4。
  - `TestIssueBudgetLedgerStatesAreNeverAssignedThrough` 是语法规则：service 包里凡导入纯计量包的非测试文件，不许有经指针的赋值。5c 与 6a 都要在这个包里写这样的文件，两份说明书的“5b 接口”一节已加上这一条。
- 第 5 步独立复核要查的点开始攒在 `/tmp/f3-main-s5k/check-points.md`（11 条）。其中一条是类型挡不住的：同一事务里追加之后不得再装载（把追加返回的根行交给装载，会读到本事务未提交的行并发布；回滚后缓存里留下的状态只有投影对不上时才会被发现）。
- 又一条提交前用户可以推翻的后果：
  - (24) 服务端写入的单条预算条目不超过 2,048 字节。以后要往条目里放自由文本（例如更正的理由）的写入方会在这里被拒，得先决定文本放哪、上限提到多少，以及缓存上界跟着变成多少。
- 迁移编号撞车（2026-11-19 02:45 查到）：共享检出里出现了未提交、未跟踪的 `866_collaboration_information_metadata` 与 `867_collaboration_information_ready`（11-18 23:28，别的会话的工作，本任务不碰）。F3 的 866–880 与它们同号。现在不改号：5b 正在这棵树里跑，而且对方还没提交，号还会动。第 8 步合并前按当时共享分支的最大号整体平移 F3 的 15 个迁移，同时改三处按号引用的地方（`server/cmd/migrate/main.go` 13 处、`server/cmd/migrate/issue_budget_test.go` 15 处、`server/internal/migrations/issue_budget_test.go` 22 处），平移后在模板的全新克隆上重跑迁移与结构用例。本机的 `f3_budget*` 库都是按旧号建的，届时全部重建。
- 第 7 步提前拆开（03:00–04:30，主会话）：5b、6b 各占一条车道，第三条空着，而第 7 步的调用端只依赖线上形状，不依赖服务端代码。
  - `design.md` 新增 §12.9（HTTP v1 线上形状：两个写入口、按 key 查询、成员查询、`show`、观察与执行两个列表的请求与应答，逐字段写出类型与可空规则）与 §12.10（拆分）。形状从已有的 Go 类型推出（`IssueBudgetOperationResult`、`budget.Projection`、`budget.ReservationProjection`、表的列），不新造概念。
  - 7a 服务端：事务内复核、读取侧与分页查询、八条路由、`no-store`、§12.8 除 CLI 之外的全部用例；在第 5、6 步整步复核之后。7b 调用端：CLI（§12.6）、Core 的 `show` schema 与 API 方法（§12.7）、内置 skill 文档与 source map；只依赖 §12.9，在自己的临时 worktree 里与第 5、6 步并行。
  - 两半共用一个样例目录 `server/internal/handler/testdata/issue_budget_http_v1/`：7b 先按 §12.9 写出，拿它当假服务器的应答；7a 的用例把每个文件严格解码进自己的应答结构、再编码回相同字节，并断言真实应答的键集合相同。样例与 §12.9 不一致时以 §12.9 为准。
  - §12.6 补充：每个子命令都有 `--output table|json`（默认 `table`，`json` 原样打印收到的字节）；`enable` 与 `set-limit` 两个金额参数都必填（数值或 `none`）；`operation` 可按 `--root` 或 `--issue` 定位；读取也做本地的人类凭证检查；写入只有三种结局（已提交、被拒、未知），未知时不重试、不换 key，提示用 `operation` 查。
  - 7b 的说明书 `/tmp/f3-main-s7b/brief-7b.md`（468 行，两轮：第一轮样例、金额换算、九个子命令；主会话复核后第二轮做 Core schema 与 skill 文档）。临时 worktree `/Volumes/artisan/code/2026/mc-f3-budget-7b-20261119`（内容与 02:24 的 F3 快照相同，56 个既有改动不许动）。04:31 派出，车道名 `f3-callers-7b`。
- 5b 第一轮交回（04:27，报告 `research/step5b-finalize-implementation.md` 1,185 行，末行 `STATUS: round 1 complete`）：
  - 5b 自己的验证（每条命令退出码 0）。Go 1.27.0：gofmt / build / vet 干净；纯计量 112 / 0 / 0；service 加 handler 的 `-run 'Budget'` 114 / 0 / 1，带 `-race` 相同且没有数据竞争（唯一的 skip 是名字里碰巧带 Budget 的 Redis 限流用例）；`./cmd/migrate/...` 67 / 0 / 1；`./internal/service/...` 1673 / 0 / 18（基线 1644 / 0 / 18，多出的 29 个是新用例）；`./internal/handler/...` 3028 / 0 / 53（与基线相同，没有数据库 skip）；sqlc v1.31.1 重新生成后树不变；866–880 迁移通过。Go 1.26.9：纯计量 112 / 0 / 0，`-run 'Budget'` 114 / 0 / 1，带 `-race` 相同。它建的十二个 `f3_budget_s5b_*` 库已自行删除并复查。
  - 第一轮没有跑的：`./cmd/server/...`、daemon 与 protocol 包（按说明书在第二轮）、整包 `-race`、整包在 1.26.9 上。
  - 主会话的独立验证：把 02:24 的快照物化成 `/Volumes/artisan/code/2026/mc-f3-verify-5b`，03:53、04:22、04:33 三次与实时的 F3 树逐文件比对，76 个路径内容全同（5b 02:24 之后只写报告，没有再动代码）。不连库的一段两条发行线都过：44 个改动的 Go 文件 gofmt 干净，`go build ./...`、`go vet ./...` 退出码 0，纯计量 112 / 0 / 0。连库的一段 04:50 起在后台跑（`/tmp/f3-main-s5v/verify.sh … r1 db`，十二组，一次一组，库名 `f3_budget_s5v_*`），结果见下一条记录。
  - R4（缓存上限管条目不管字节）按前面定的 2,048 字节上限答复。04:40 补测最坏的转义：模型名是 120 个 `<` 时每个字符在 JSON 里占 6 字节，带估算的用量条目是 1,708 字节（两条发行线相同），这是合法 v1 上报能产生的最大条目，上限之上还有 340 字节；四个 token 数与金额取 int64 最大值、模型名不需转义时是 1,108 字节（前面记的 1,093 用的是 16 位数字）。规则与依据写进 `design.md` §10.4 末条；5b 第二轮另加一条不连库的用例钉住“最大的合法用量条目不超过上限”。
  - 第二轮 04:47 发出（`/tmp/f3-main-s5b/round2-5b.md`，258 行）：先改“全量重放判定”与字节上限两处（各自先写用例），再做阶段 E–G。领取重试的 laps 测量要等主会话的连库验证结束（标记文件 `/tmp/f3-main-s5v/r1-db.done`），免得两边抢同一个 PostgreSQL 容器把时间测歪。
- 主会话对 5b 第一轮的连库验证（04:49–05:12，验证树 `/Volumes/artisan/code/2026/mc-f3-verify-5b`，每组一个从 `f3_budget_base_865` 克隆并迁移到 880 的新库，一次一组，跑完即删；每组退出码 0）：
  - service 的 `TestIssueBudget`：60 / 0 / 0；handler 的预算与删除相关选集：26 / 0 / 0。Go 1.27.0 与 1.26.9 各一遍，再各带 `-race` 一遍，四组数字相同，没有数据竞争。
  - `./internal/migrations/...` 103 / 0 / 0；`./cmd/migrate/...` 67 / 0 / 1。
  - 整包：`./internal/service/...` 1673 / 0 / 18，`./internal/handler/...` 3028 / 0 / 53，与 5b 报告的数字逐项相同。
  - sqlc：用 `/tmp/f3-main-tools/sqlc`（v1.31.1）在一份临时副本里重新生成，143 个生成文件与验证树逐字节相同；目录里另外 4 个文件是仓库里手写的（`attachment_row.go` 等），不是生成物。
  - 内存基准在 Go 1.26.9 上复现：服务端会写的形状里最贵的一格 1,838 字节 / 权重（上限处 87.7 MiB），`form-limit` 一格 64,890 字节 / 权重（3,094 MiB），与 5b 的数字相同。
- 主会话自选的变异抽查（05:25–05:35，验证树，Go 1.26.9，`-overlay` 喂给编译器所以不改树；对着 service 的 `TestIssueBudget` 60 个用例）：账目缓存与装载的 13 个变异杀掉 12 个（同 revision 不返回缓存、晚到的发布盖掉新状态、上限与淘汰的边界各偏一、读取方失败时丢掉别人的状态、淘汰不减权重、同 revision 仍读尾部、读取方发布、去掉末尾 seq 校验、去掉投影校验、接受非规范的预留文本、去掉追加后的条目数校验）。活下来的一个：持根锁的装载失败时把“丢掉缓存里现有的状态”换成“只丢自己用过的那个”。两者只在一种交错下不同：本进程更早一笔事务的 `Committed()` 晚到，落在持锁方查缓存之后、装载失败之前。那个晚到的状态与持锁方用过的出自同一族，同样不可信；它若恰好与根行同 revision，全量重放的结果还会因为“revision 不更大就不替换”而发布不进去。规则（R1）已在 §10.4 末条，缺的是用例；05:40 发给 5b，随第二轮的“全量重放判定”一起补两条用例并把这个变异列进去。
- 新增的、提交前用户可以推翻的后果：
  - (25) 预算 HTTP 接口的错误体是 `{"error":"<code>","code":"<code>"}`，两个键同值；金额与各种 revision 是十进制字符串，计数与 `protocol_version` 是 JSON 数字；请求严格解码（未知字段、缺字段、重复键、结尾多余内容一律 400）。这是首包对外的形状，定下之后已安装的桌面端与 CLI 会按它解析，以后只能加字段。
  - (26) `show` 的“未知”分两处给：`unknown.by_reason`（按原因的计数）与 `unknown.pre_activation`（启用前已在跑、未被覆盖的执行数）。只要其中一处不为零，`total_known` 就是 false，`remaining_ticks` 是 null：界面拿不到“剩余额度”，只能显示未知。
  - (27) CLI 的写入没有自动重试。网络错误、5xx、读不懂的 2xx 一律报“结果未知”，由人拿同一个 key 用 `operation` 查；CLI 不替人换 key 重发。
  - (28) 第 7 步拆成两半并让调用端先做，靠的是 §12.9 不再变。7a 做服务端时如果发现形状必须改，要回头改 7b 的样例与解析。
- 7a 说明书的准备（05:40–06:10，主会话）：对着处理器、路由与 service 的源码核对 §12，改了三处设计（都标 2026-11-19）：
  - §12.9 更正：错误体 `{"error":"<code>","code":"<code>"}` 用 `writeErrorCode` 写不出来，它把 map 交给 `json.Marshal`，键被排序，`code` 排在前面。预算处理器用字段顺序为 `error`、`code` 的结构体经 `writeJSON` 写。7b 的样例与解析不受影响（样例本来就是 `error` 在前）。
  - §12.1 补充：检查顺序定为“人类凭证与角色 → 路径 id 与 key 的写法 → 请求体严格解码 → issue 访问与查找 → service”，403 先于 400，请求体先于任何预算表读取。issue 访问沿用 `loadIssueForUser`，但给它一个丢弃正文、只记状态码的 `ResponseWriter`，再把状态映射成预算自己的错误体（404 → `not_found`；5xx → `internal_error`；其余拒绝在启用入口是 422 `not_eligible`，别处是 404）；不拆 `loadIssueForUser`。路径 id 用预算自己的校验解析，不用 `parseUUIDOrBadRequest`（它写的是句子）。
  - §12.1 同一处：三条以 issue 定位的路由永远先过 issue 访问，所以根任务被删之后它们答 404（包括启用的按 key 查询），墓碑只能经以根定位的路由读到。
  - §12.4 补充：首包的读取侧不向账目缓存发布，`show` 与执行列表用 5b 的不持锁装载，用完即弃。
  - 写入的事务内复核放在 `transact` 里、隔离级别声明之后的第一条语句（在 fence、根锁与按原 key 查找之前），普通读、不加锁。
- 7a 的说明书 `/tmp/f3-main-s7a/brief-7a.md` 写到阶段 C（两轮：第一轮线上类型与样例往返、事务内复核、读取侧，不出路由；主会话复核后第二轮做处理器、`no-store` 中间件、八条路由与 §12.8 的用例）。阶段 D、E 与第 4–9 节待写；派发要等第 5、6 步独立复核与 7b 的样例合进 F3 树。
- 6b 第一轮交回（06:06，报告 `research/step6b-daemon-implementation.md` 1,117 行，末行 `STATUS: round 1 complete`）：阶段 A–C，新增 38 个 `TestBudgetUsage…` 用例，没有改任何既有用例；这棵树编出的 daemon 行为与原来相同（还没有任何代码打开回传日志）。
  - 6b 自己的验证：说明书第 8 节八项两条发行线都过。Go 1.27.0 第二遍 3082 / 0 / 5，Go 1.26.9 3082 / 0 / 5，基线 3044 / 0 / 5，多出的正好是 38 个新用例；1.27.0 第一遍有 3 个 `TestReasonixBackend…` 超时（5 秒上限，机器当时被别的车道占着，单跑 9 次全过，与本次改动无关）。`-race` 无数据竞争。变异 178 个杀掉 177 个，8 个对照全过；活下来的一个是 `openBudgetUsageJournal` 里列目录那一步，POSIX 上它前面的探针写入已经要求同样的权限，只有 Windows 上它才是唯一的检查，而 Windows 代码本机只能交叉编译。
  - 主会话的复核（06:15–06:30）：报告的状态、文件、验证、变异、偏离、待裁定与备注各节读完；11 个文件的 SHA-256 与报告逐个相同；已跟踪文件的 diff 与 02:24 快照逐字节相同；`budget_usage.go` 与主会话审过的那一版相比只差两段注释；快照之后新增的 4 个用例与加强的 6 个用例读完，没有发现问题。主会话自己在 6b 的树上跑：gofmt 无输出；两条发行线 build、vet 退出码 0；`-run '^TestBudgetUsage'` 两条线都是 38 / 0 / 0，带 `-race` 相同且无数据竞争；agent CLI 守卫脚本下 38 / 0 / 0，没有任何替身被启动；`windows/amd64` 的 build 与 vet、`linux/amd64` 的 build 退出码 0。
  - 偏离 1–8 接受（其中 7：建记录的首次写入失败时删掉文件；8：读取拒绝非规范文件名、非普通文件、超过 4 MiB、版本或 attempt id 或批 key 不对的文件）。
  - 6b 的待裁定项：进程死在“记录文件落后于内存”的窗口里时，下一个进程会按文件少报调用；首次启动意图没写进去时报出来的是一个空批，而 §11.5 把空批定义为“没有启动任何 provider 调用”。裁定写进 §11.10（第二条 2026-11-19 补充）：启动意图写入失败就尽力删掉记录文件；恢复从不上报“没有批字节、也没有任何启动意图”的记录，不论谁建的；本进程建的、不在在途表里的无批字节记录同样只删不报；目录里的临时文件名恢复时跳过，7 天上限那一轮顺手清掉过期的。
  - 第二轮 06:38 发出（`/tmp/f3-main-s6b/round2-6b.md`，先做 `adapter_version` 与上面这条裁定，再做阶段 D–F；另加两份共享线上样例 `server/pkg/protocol/testdata/budget_usage_v1_batch*.json`，6a 的严格解码器要接受同样的字节）。
  - 6b 报告里的一条教训：车道在自己的回合里收不到消息，也没有理由把设计的同一节读第二遍；它是碰巧搜到 04:55 的补充才知道的。此后回合中途加进设计的规则，一律在继续车道的那条消息里全文写出。这次的第二轮消息照此办理。
- 新增的、提交前用户可以推翻的后果：
  - (29) 预算路由的 403 先于 400：没有权限的人发一个格式不对的请求，得到的是 `budget_forbidden`，看不出请求哪里不对，也看不出那个 issue 有没有预算根。
  - (30) 根任务被删之后，以 issue 定位的三条路由一律 404。可见的边角：启用已经提交、应答丢了、根任务紧接着被删，这时按 key 查询与原样重发都得到 404，操作的人看不到“当初成功了”；不会重复提交。墓碑状态只能用根 id 读。
  - (31) 协作类 issue 上启用预算答 422 `not_eligible`，其余预算路由答 404；现有 issue 接口对同一情形给的是说明文字，预算接口不透出那段文字。
  - (32) 首包里读取（`show`、执行列表）不预热账目缓存：一个只被人查看、很久没有领取的大根，每次查看都按库里的条目重放一遍。
  - (33) daemon 的回传记录文件在首次启动意图写不进去时会被删掉，那次执行的用量如果随后进程崩溃就不再上报，靠服务端的 hold 保持未知。“还没启动任何 provider 就崩溃”的 attempt 在重启后同样不上报空批：崩溃留下的文件证明不了“什么都没启动”。
  - (34) 已知局限：回传目录从某次执行的第 N 次调用（N > 1）起变得不可写、文件也删不掉、daemon 又在送达之前死掉，下一个进程会报出前 N−1 次调用而缺第 N 次。首包不结算，金额不受影响；S2 要从一批里推出“调用完整”之前必须先处理。
- 07:10–07:55 三条车道都在跑（5b 第二轮、6b 第二轮、7b 第一轮），主会话做不依赖车道的准备：
  - 第 5 步独立复核的简报框架写好，放在 `/tmp/f3-main-s5k/`：`brief-5k-common.md`（两位复核方都读：窗口规则、冻结副本、什么算证据、发现的写法、变异经 `-overlay`、硬规则、环境、报告）、`brief-5k1.md`（领取时的预算步骤 A1–A12、账目装载与缓存 B1–B11、锁与隔离级别 C1–C5，出口 B07/B08/B34/B35/B35a/B45–B47 与 B53、B55 的领取半边）、`brief-5k2.md`（纯计量包 P1–P6、对账与 sweeper、`off` 期间、quick-create 差异、未知集合、表结构与迁移、用例有没有被削弱、范围，出口 B36–B38、B52、B55 其余）。派发前要填的占位符与做法记在同目录 `dispatch-notes.md`。
  - 拆成两位复核方并行、且在冻结副本上做的理由：第 4 步的复核用了约 9 小时；第 5 步的代码与规则更多，一位复核方读不完；在副本上做，6a 可以同时在活的 worktree 里推进。代价：复核发现的问题要在 6a 的轮次边界合进活的树；复核方只加新用例文件、不改现有文件，合并就是拷文件。
  - 第 4 步结束时的源码没有留快照（最早的快照是 2026-11-18 17:20，5b 已经开工两小时）。“第 3、4 步的用例有没有被削弱”这一点因此只能对照当时 `-v` 输出里的用例名单和那份最早的快照，简报里照实写明。
  - 为 5c 的 hold 原因表做了主会话自己的清点（`/tmp/f3-main-s5c/reason-table-main.md`，不事先给车道）：把 task 行写成终态的 sqlc 查询共 21 条，Go 里没有绕过 sqlc 的原始 SQL；daemon 经 `FailAgentTask` 报上来的原因在 `pkg/taskfailure/failure.go`。留给确认时定的四个问题：`FailStaleTasks` 写的 `timeout` 是服务端 sweeper 不再等待，进程是否结束没有证明，记 `timeout` 还是 `lost_contact`；只会命中未派发 task 的那一族取消与过期；没见过的 `failure_reason` 字符串；task 行 status 的完整取值。
  - 清点时发现设计 §10.3 没有覆盖的一种情形，补进 §10.3（2026-11-19 补充）并全文写进 5c 的简报：task 行的终态说的是它最后一次领取，而 open attempt 可以属于更早的一代（之后那次领取没有写下 attempt：当时根是 `off`，或那次领取的预算事务失败、原行 requeue 后没被再领就结束）。这样的 attempt 记 `superseded_unproven`，不按终态映射。核实过的依据：`claim_generation` 只在 `ClaimAgentTask` 与两条 `ReclaimStaleDispatched…` 里加一，`RequeueAgentTaskAfterClaimFailure` 不加；领取的事务（`claimTask` / `ClaimTaskForRuntime`）先于 `FinalizeTaskClaim` 单独提交。
- 新增的、提交前用户可以推翻的后果：
  - (35) 对账给“代数落后于 task 行”的 open attempt 记 `superseded_unproven`，不看 task 的终态。可见的差别只在观察记录的原因一栏；金额不变（hold 在首包从不释放）。
- 08:00–08:35 三条车道仍在各自的轮次里写代码（5b 在写领取收尾的实现文件 `issue_budget_finalize.go`，6b 先写了发送阶段的用例文件，7b 在写九条命令与文本输出），没有到达边界。主会话继续做不依赖车道的准备：
  - 7a 的说明书（`/tmp/f3-main-s7a/brief-7a.md`，865 行）通读完，与设计 §12 对得上；改了一处过时的文档大小。说明书用到的两条事实对照源码与设计核过：第三个头 `X-Actor-Source` 确实存在而且 §12.1 点了名；§12.3 的 409 代码正好七个。fixture 目录现有 26 个文件，与说明书一致。八处占位照旧等 5b、5c、6a、7b 的结果。
  - 提前审了 7b 的线上形状读取文件（`cmd_issue_budget_wire.go`，06:15 之后没有再动）：逐字段对照 §12.9 的必填、可空、可省略，以及 attempt 表的列定义（`runtime_id`、`agent_id` 非空）与纯计量包按预留读出的 JSON 名，没有发现出入。读取方式是每个字段先只记下“出现过没有、是不是 null、是不是出现了两次”，之后再按规则解码，所以缺字段、不该有的 null、重复的键、类型不符都判成“读不懂”，不拿零值顶替；嵌套对象逐层同样处理。记一条备注（不是发现）：`encoding/json` 匹配键名不分大小写，`Replayed` 会被读成 `replayed`；读取方从宽，值仍然是服务端给的。其余文件还在变，等第一轮交回再审。
  - 第 6 步的独立复核同样拆成两半，daemon 一半不等服务端：说明书写好了（`/tmp/f3-main-s6k/brief-6kd.md`，约 42 KB，复核方 `f3-check-6kd`）。理由：6b 交卷之后空出一个名额，而 6a 要等 5c；daemon 一半只在线上格式上与服务端相接，复核它不需要服务端的代码，也不需要数据库。它在 6b 的临时 worktree 上做（车道结束后那里没有别人写），只加 `_check6kd_test.go` 结尾的新用例文件，变异经 `-overlay`。要点八组：谁声明协议（含“attempt id 先校验再拼文件名”）、回传记录、每次调用记下什么、组批（复核方自己按 §11.5 写一个不借 daemon 类型的严格校验器）、发送与状态码分类、重发与重启恢复（自己列判定表与“进程可能死在哪里”的清单）、任务路径不受影响与锁、两条发行线与交叉编译。已经裁定过的 16 项列在说明书第 6 节，不再当发现报。五处占位等 6b 交卷后填：文件清单与摘要、报告的章节位置、主会话复测的用例计数、第二轮里新接受的事项。
- 08:35–09:00 仍没有车道到达边界：5b 把 daemon 声明的协议接到两个领取调用点、把 attempt id 放进领取应答，正在写领取收尾的用例；6b 发送阶段的实现写完，在跑全部用例；7b 在做第一轮命令的变异检查。监视器 08:46 到期没见到边界，重新布置了一个（09:42 前后到期）。
  - 第 7 步的独立复核也拆成两半，调用方一半不等服务端：说明书写好了（`/tmp/f3-main-s7k/brief-7kb.md`，约 37 KB，复核方 `f3-check-7kb`）。它在 7b 的临时 worktree 上做，只加 `_check7kb_test.go` / `.check7kb.test.ts` 结尾的新文件；Go 的变异经 `-overlay`，TypeScript 没有 overlay，变异在复核方自己的临时目录里改副本、做完删掉并用 `git status` 证明没留东西。要点九组：A 共用的 fixture（复核方自己按 §12.9 判，车道的检查器只算一条声明）、B 金额换算、C 发请求之前（只给人用、不代填 key、头、路径转义）、D 一次写只上线一次与三种结果、E 读应答（含服务端文本字段里的控制字符与换行对表格输出的影响：设计 12.6 还没有规则，等复核结果再裁决）、F Core（schema、`parseWithFallback` 脱敏与 `null` 回退、没有多加页面/hook/store/query key）、G 内置 skill 的文字与 source map 以及文字一改仓库还要求什么一起动、H 用例与仓库规则（不出本机、没有被削弱、每组必备用例都有一个杀得死的变异、两条 Go 发行线）、I 运行（离线安装加 `--frozen-lockfile`，前后对比 `git status`，不许动锁文件）。已经裁决、不算发现的六条写在说明书第 6 节。占位六处（文件清单、清单摘要、第二轮消息、报告章节表、主会话的基线计数、第二轮接受的事项），等 7b 两轮交回后填。
  - 09:03–09:25 提前审了 7b 的命令与文本输出两个文件（`cmd_issue_budget.go` 955 行、`cmd_issue_budget_print.go` 382 行；车道已在做变异检查，文件基本稳定。当时的副本留在 `/tmp/f3-main-s7b/early-0903/`，SHA-256 前八位依次 `392881ed`、`901503bf`，第一轮交回时只看差异）。对照设计 12.6（含 2026-11-19 补充）与 12.9 逐条核过，没有发现缺陷：每个子命令先做人工凭据检查再看参数；请求自己组装，不带 agent / task 身份头，不跟随重定向（307/308 不会把 POST 正文再发一次），写请求去掉 `GetBody` 且不带 `Idempotency-Key`，两条 HTTP 线路上都不会被传输层重放；三种结局与设计一致（201，或 200 且 `replayed`，并且 kind 与 root 对得上，才算已提交；401 或带 12.3 代码的错误形状才算拒绝；其余一律未知，未知时只给按原 key 查询的命令）；读不懂的应答在两种输出格式下都不显示任何部分，错误文字里只有字段名；`member_count`、`in_flight_gap_count` 缺省按 0 是设计 12.9 明文允许的唯一一处；服务端给的文本进表格前过一道 `budgetText`（有不可见字符就整段加引号转义，游标含这类字符时不拼进下一页命令），说明书里没写的“控制字符与换行”一点车道自己做了。
  - 审出一条要补的说明（不是缺陷，第二轮顺带做）：CLI 既有的退出码分层是 网络=2、401/403=3、404=4、400/422=5、其余=1。写命令因此：已提交=0；被拒绝=3/4/5，或 409 那七个代码=1；结果未知=1（车道有意不用 HTTP 错误类型包装未知，免得一个不是预算 handler 发的 404 得到“未找到”的退出码）。409 拒绝与结果未知同为 1，脚本只能靠输出首行的 `refused (…)` / `outcome unknown:` 区分。设计没有对退出码作规定；不为此新增退出码分层（那是整个 CLI 的约定），只要求在命令的帮助文字里写明“退出码为 0 才表示已提交，其余要看首行”。
- 09:25–10:55 仍没有车道到达边界，主会话继续提前审与备料：
  - 提前审了 6b 的发送阶段（`budget_usage_delivery.go` 及 `client.go`、`managed_transport.go`、`daemon.go` 的接线；副本在 `/tmp/f3-main-s6b/early-0930/`）：应答分三类（2xx 送达；400/404/409/413/422 终局拒绝；其余保留重发）、只发给记录自己的 `server_url`、不跟随重定向、能力位在记账目录就绪之后才声明。没有发现缺陷。车道自己定的七项（15 秒发送上限、应答只留前 64 KiB、`code` 也要求代码格式、保留只记 Debug、删不掉的记录在内存里记为已了结、`server_url` 按字符串精确比较等）接受，写成 6kd 复核说明书的“已接受”第 17–22 条草稿（`/tmp/f3-main-s6k/accepted-round2-draft.md`）。
  - 审了 5b 对 13 个既有测试文件的改动：全部是给 `FinalizeTaskClaim` 的调用多传一个参数（`TaskClaimBudgetProtocolNone`），四个 handler 测试文件多了一条 import；23 行增、19 行删，没有断言被改。
  - 写好两个基线脚本（`/tmp/f3-main-s6k/baseline.sh`、`/tmp/f3-main-s7k/baseline.sh`：清单与 SHA-256、两条 Go 线的 gofmt/build/vet/用例、guard 下的运行、交叉编译；每个计数同时给顶层与含子用例两种口径，免得车道与复核方的数字对不上）和 7b 第二轮的消息草稿（`/tmp/f3-main-s7b/round2-7b.md`）。
- 10:55–12:30 **5b 车道饿死，停掉并换车道**。
  - 现象：08:53 之后没有写过任何 Go 文件；十二次压缩，上下文约 20.8 万 token，每次工具调用约 3.5 分钟；每次压缩后要重读 82 KB 的笔记，读完又到压缩点。领取收尾的代码（`issue_budget_finalize.go` 581 行、`task.go` 与两个 handler 文件的接线）已写完、能编译，但一条用例都没有。11:31 停掉它时，它刚写出用例文件的头 240 行（只有辅助函数，未用的 import 让 `go vet` 报错；留给接替车道）。
  - 处理：与 6b 那次相同，换一条任务窄、自带全部事实的新车道 `f3-finalize-5d`（说明书 `/tmp/f3-main-s5d/brief-5d.md`，约 24 KB；报告另起一份 `research/step5d-finalize-tests.md`，不往 112 KB 的旧报告里追加）。它只做服务层这一半：两处小的代码补充、22 组用例（F01–F22）、15 个变异、四项验证。handler 一半（两个领取端点带/不带能力头、`budget_attempt_id`、绑定进行中的应答）、入口清点、重领圈数实测、报告收尾，等它做完再派一条同样窄的车道。
  - 停之前的状态：`go build ./...` 两条 Go 线都过；快照 `/tmp/f3-main-snap/f3-20261119-1210-5b-stopped`（74 个路径）。5b 的库 `f3_budget_s5b_r1` 已删；5d 从主会话的空模板 `f3_budget_s5v_tmpl` 克隆（模板建于 866 最后一次修改之后）。
  - 这一段损失约两个半小时的车道时间。教训与 6b 那次相同，补一条：车道的笔记文件本身也要有上限（5d 的说明书规定 150 行以内、写满就重写），否则笔记越详细，压缩之后越读不完。
- 5b 笔记里“需要裁定”的九项，主会话的裁定（写进设计 10.2 的 2026-11-19 补充、10.5、10.7；提交前都可以推翻）：
  - 账目不可用时，同一 task 之前仍为 open 的 attempt 原样留着：接受，写进 10.5。
  - **删除的 context 父任务记什么**（设计自相矛盾：10.7 说带 `root_deleted`，10.2 第 9 步把 `root_deleted` 定义在根任务的成员行上，V18 说“`root_deleted` 或成员 tombstone 观察”）：`root_deleted` 只看根任务的成员行；依据的成员行已 tombstone 而不是根任务的那一行时，不写 `would_refuse`，写一条 `attribution_difference` / `member_deleted`。10.7 的句子改了。
  - 成员行指向的根行不存在、普通 task 的 issue 行读不到、quick-create 的 context 解析不出：都不写 attempt、执行照常，分别记 error / warn：接受。
  - 领取因绑定进行中被退回、再领时代数加一，绑定方给上一代记的缺口从此不被覆盖：接受为已知界限，写进 10.2 补充，用例钉住。
  - **预留让 A+R+H 超出 int64**（首包预留从不释放、预留额没有上限；纯计量包报 `ErrOverflow`，finalize 失败、领取反复 requeue，一个只观察的根挡住执行）：领取路径自己兜住，改写成未定额预留并记一条 `coverage_gap` / `reservation_overflow`；管理 API 不设上限。
  - `attempt_open` 的意图摘要与操作结果的形状、两种观察的 detail 键：按代码的写法定下来，写进设计；摘要有两条主会话在 Go 之外算出的黄金向量（写在 5d 的说明书里）。写设计时更正了自己的一处错：代数在 JSON 里是字符串里的十进制数，不是 JSON 数字。
  - 有没有测试把已打开的事务当作 `TxStarter` 传进来（那样 `Committed()` 会在外层提交之前发布缓存）：主会话查了，全仓只有 `task_batch_claim_partial_test.go:87` 一处，那条用例里没有预算根，不会走到发布。生产构造只有连接池一种。
- 新增的、提交前用户可以推翻的后果：
  - (36) 删除了 quick-create 的 context 父任务（它不是根任务）之后的领取，观察记录里是一条“归属差异 / 成员已删除”，不是“将被拒绝”。将来的强制对这种领取怎么办，留给 F3-S2。
  - (37) 根声明的预留额大到让累计占用溢出时，那次领取照常执行，记成未定额预留加一条永不覆盖的缺口，根从此显示未知。要恢复只能关掉开关或调小预留额；已经记下的缺口不会消失。
  - (38) 启用根或把任务移入预算树的那一刻，恰好在做领取收尾的 task 会被退回重领，根上留下一条不会被覆盖的在途缺口，尽管那一代从未交给 daemon。
- 12:30–14:25 5d 开工，主会话备 5e，三条车道被一次 API 故障打断后续上：
  - 5d（`f3-finalize-5d`）12:35 前后派出。到 13:34 用例文件里有 6 条用例（摘要的两条黄金向量、F01–F04），四次运行的日志在 `/tmp/f3-s5d/logs/`。
  - handler 一半的说明书写好了：`/tmp/f3-main-s5e/brief-5e.md`（约 17 KB，车道名 5e，报告 `research/step5e-claim-endpoints.md`）。内容：11 组用例 H01–H11（两个领取端点带/不带能力头、`budget_attempt_id` 有与无、能力头的解析表、绑定进行中时单端点答 200 `{"task":null}` 且与“没有任务”逐字节相同、task 回到 queued、什么都没写、锁放开后下一次领取拿到它；批量端点同理；非绑定的收尾失败仍是 500；RPC 路径；关闭的根不改变应答；日志级别 info）、7 个变异、两份清点（设计 10.11 的入口；`RequeueTaskAfterClaimFailure` 的各调用点）、重领圈数实测（绑定方持锁 10 ms / 100 ms / 1 s 各五次，再加两条排队任务的一行；只报数字，要不要加延迟由主会话定）。它只加新的 handler 用例文件，不动服务包。worktree 一栏留着占位，派发时填。
  - 写说明书之前主会话读了 5b 写的 handler 接线（`daemon.go` 的 `claimBudgetProtocol`、两个端点的收尾调用与绑定分支、`agent.go` 的应答字段、RPC 路径上能力头的抄送），没有看出缺陷；是否真对，由 5e 的用例和之后的独立复核来说。
  - 监视器误报一次：5d 的报告骨架末行写成 `STATUS: blocked: in progress, not finished`，被监视器当成交卷。监视脚本改成忽略含 `in progress` 的状态行（`/tmp/f3-main-s5d/watch-lanes3.sh`），并在续跑消息里让车道把占位行改成 `STATUS: in progress`。
  - **14:01 一次 API 502 同时打断了三条车道**（5d 停在 13:38，7b 停在 13:40，6b 停在 13:55；都没有到达边界，磁盘上的东西没有丢）。这是服务端的临时故障，不是车道失败。14:20 逐条发消息续上，各自从自己的笔记接着做；监视器重挂一个。损失约 25–45 分钟的车道时间。
  - 名额：三条车道都在跑，5e 等其中一条交卷后再派（它在关键路径上，排在两位不在关键路径上的复核方 6kd、7kb 之前）。
- 14:25–17:30 5e 备齐并派出，6b 收窄后交卷，未知集合查询的计划由主会话测了：
  - **5e 的工作树**：`/Volumes/artisan/code/2026/mc-f3-budget-5e-20261119`，14:33 的 F3 树快照（74 个路径，`/tmp/f3-main-snap/f3-20261119-1433-for-5e`），两条工具链下 `go build ./...` 与 `go vet ./internal/handler/` 均为 0。它只加 handler 的用例文件，不依赖 5d 之后加的两处代码；用例文件之后拷回 F3 树。说明书里的行号在这棵树上逐个核过。
  - **5c 说明书**填了两处占位（5b/5d 留给它的接口表、离线 sqlc 的路径），还差基线计数一处，等 5d、5e 验完再填。接口表里写了 `attribution_difference` 的常量名，5d 确实按这个名字加了（`issueBudgetObservationAttributionDifference`，`issue_budget_finalize.go:553` 在用）。
  - **设计 10.4 的两个内存数字**从 5b 报告 6.1 抄进了设计（每单位权重 1,838 B 实测、按 2,000 B 记；写入上限形状 2,960 B 实测、按 3,200 B 记），连同它们没有覆盖的情形。
  - **6b 第二轮收窄（16:05）**。车道第 14 次压缩之后，15:06 起的 50 分钟里只读不写，在为第二轮的变异生成器做准备。主会话停掉它，改派一份窄任务（`/tmp/f3-main-s6b/stage-f-narrowed.md`）：跑一次全包、改三处报告、交卷；第二轮的变异检查移交独立复核方，验证矩阵由主会话自己跑。16:45 交卷，`STATUS: complete`，报告 131 KB。
  - **6b 在 F 阶段去掉了自己加的 15 秒发送上限**（连同另外三处“没有测试能分辨”的代码）。现在一次上报的时间上限是 daemon 客户端给每个请求的 30 秒（`client.go:136`，`reportBudgetUsage` 按值拷贝客户端，上限随之保留）加上发送所用的 context。主会话核过：全仓只有这一个带超时的客户端，子客户端也出自 `NewClient`。数字接受（一轮重发最坏 32 × 30 秒）；**没有测试钉住这个上限，不接受**：车道自己在报告 11.4 (e) 里写了“没有”。裁定为必须补测试，交给复核方写成它自己的用例文件，合并时并入。
  - **主会话在 6b 树上跑的基线**（16:49–17:12，`/tmp/f3-main-s6k/base-r2/`，21 个文件的摘要在 `manifest.txt`）：两条工具链 gofmt/build/vet 全 0；`^TestBudgetUsage` 78 条顶层用例（含子用例 280）在两条工具链、带 race、以及 CLI 守卫下全过；`./internal/daemon/...` 1829/3801 通过、2 条平台跳过（两条工具链各一遍，默认工具链再带 race 一遍）；`./pkg/protocol/...` 61/451；`./pkg/agent/...` 1232/1997、跳过 3/4；Windows 与 Linux 的交叉编译和 vet 为 0。没有失败。
  - **6kd 复核说明书填完**（`/tmp/f3-main-s6k/brief-6kd.md`，896 行）：文件清单与摘要、报告的章节表、基线数字、第二轮已接受的 7 项（17–23），以及新加的 5.1 节“第二轮必查变异清单”（应答分类、处理中标记、已结清集合、服务器归属、保留期与三类记录、每轮上限与游标、启动顺序、F 阶段四处删除、上报必须结束）。车道自己列的“读代码看来没有测试钉住”的 9 处（11.4 (f)），要求复核方逐条跑变异并分三类。
  - 两份线格式样例的 SHA-256 填进了 6a 的说明书（`0d76f057…7dc0`、`bfe8d8ba…cdf8`）。
  - **5e 派出（17:00）**，车道名 `f3-claim-5e`。提醒它重领圈数的计时放到最后做，并写明当时的负载（主会话的基线跑到 17:12）。
  - **未知集合查询的计划**（5b 拆分后这件事没人接，主会话自己测）。造数库 3,000 个根、约 50 万 attempt、42 万观察；数字和结论写在设计 10.12 的补充里。裁定：
    - 现有索引确实扫过该根的全部观察（3 条缺口、20 万条将被拒绝的根：4,613 页、约 29 ms），设计写的条件成立，**加部分索引** `issue_budget_observation_gap_idx (budget_root_id, reason) WHERE kind = 'coverage_gap'`（同一个根 11 页、0.02–0.14 ms）。键里带 `reason` 的写法比只含根的写法在 20 万条缺口时快（45–48 ms 对 76 ms，后者落盘排序）。
    - 代价与未覆盖缺口行数成正比这一点首包不改（5,003 条 1.2 ms，20 万条 45–48 ms）；会逐次增长的来源是旧 daemon、账目不可读、预留溢出三种领取。接受，写进规格，补救办法留给 F3-S2。
    - 仓库没有设置 `jit`，这次也不加；规格里写一句排查提示。
    - finalize 另外三条查询各走一条唯一索引（4 页），不需要新索引。
    - 交给 5c 做（它的说明书加了“阶段 E”，排在第一轮最前）：一条并发索引迁移，排在就绪检查之前；就绪检查加一列“应有的谓词”。比较谓词的写法主会话在 PostgreSQL 17.11 上试过：去掉括号、空白和 `::text` 之后比较，接受三种等价写法，拒绝另外三种（探测库已删）。
- 新增的、提交前用户可以推翻的后果：
  - (39) daemon 一次用量上报最多等 30 秒（不再是 15 秒）；对着一个把每个请求都挂住的服务器，一轮重发最坏要 16 分钟，这期间别的记录排队。
  - (40) 设了额度的根上，领取时那条“未知集合”查询的代价随未覆盖缺口的条数增长，没有封顶。旧版 daemon 在同一棵父任务树上领取上万次之后是几毫秒到几十毫秒一次。
  - (41) 观察表多一条部分索引（只含缺口），迁移总数加一。
- 17:30–19:30 6b 的独立复核拆成两半并备好，5e 起步慢但没有停它，7b 第一轮的四项裁决定了意向：
  - **车道每一轮大约 4 分钟**。三条车道的请求记录都是 3–4.5 分钟一条（19:00 前后量的），不是某一条车道的问题。按这个速度，一份完整大小的说明书要 10 小时以上才做得完，所以之后的复核一律拆半、说明书收窄。
  - **6b daemon 一半的复核拆成两位复核方**，说明书由一份源文件（`/tmp/f3-main-s6k/brief-6kd.md`）经 `/tmp/f3-main-s6k/split/split.py` 生成，改源文件后重跑脚本即可：
    - `f3-check-6kd1`（`split/brief-6kd1.md`，约 39 KB）：记录与批次。用 6b 自己的工作树（已冻结），草稿目录 `/tmp/f3-s6kd1/`，只加 `_check6kd1_test.go` 后缀的用例，报告 `research/step6-check-daemon-1.md`，发现编号 `KD1-…`。全包运行、CLI 守卫、交叉编译都归它。
    - `f3-check-6kd2`（`split/brief-6kd2.md`，约 45 KB）：声明、发送、重发、恢复，含 5.1 节的必查变异清单和 30 秒上限的补测。用**自己的一份副本** `/Volumes/artisan/code/2026/mc-f3-check-6kd2-20261119`，免得两位复核方加的用例文件互相弄坏对方的编译。副本出自 19:16 的快照 `/tmp/f3-main-snap/6b-20261119-final`：状态与原树一致，21 个文件的摘要全对，两条工具链下 `go build ./...` 与 `go vet ./internal/daemon/` 均为 0。后缀 `_check6kd2_test.go`，报告 `research/step6-check-daemon-2.md`，编号 `KD2-…`。
    - 两位都还没派：三个名额被 5d、7b、5e 占着。风险较大的第二半先派。
  - **主会话通读了 6b 第二轮的两份代码**（`budget_usage_delivery.go` 361 行、`budget_usage_recovery.go` 207 行，及 `budget_usage.go` 的 `seal()`），没有看出缺陷，只留下一个疑问交给第二位复核方（说明书的 F7a）：一条记录封存之后、发送协程给它打上“处理中”标记之前，重发的一轮可以先拿到它；若它已超过保留期，这一轮会把它丢掉，于是 `deliverBudgetUsage` 注释里“不论多旧都会试发一次”可能不成立。是否真能发生、要不要改，等复核方的变异和用例说话；这是疑问，不是已确认的缺陷。
  - **5e 读了 100 分钟才开始写**（56 次工具调用、没有落盘）。它的说明书没有“二十次调用之内必须落盘”这一条，是主会话的疏漏。没有停它：查它的记录，它还在第一个上下文窗口里（约 158K token，未压缩），不是压缩后空转。18:41–18:44 落盘了笔记、运行脚本、报告骨架和第一组用例（H01，`daemon_claim_budget_test.go`）。它的决定里有一条要记：领取的“空结果缓存”靠 Redis，handler 用例里是 nil，所以 H06 里缓存那一半只能引代码、不能跑。
  - **监视器两次到点没有等到边界**（16:13、17:55），都不是失败；各查一遍状态行和记录后重挂一个。当前这一个挂到 19:37 前后。
  - **7b 第一轮报告里要主会话裁的四项，意向如下**（等它写出 `round 1 complete`、主会话核过样例和基线之后，随第二轮消息发给它）：
    - R1 “结果未知”与 409 拒绝共用退出码 1：不加新码，靠 stderr 前缀区分，帮助文本里写两句，按行钉住 `cli.ExitCodeFor`（第二轮说明书第 2 节已经这么写了）。
    - R2 多出的三份样例（`operation-reopen-request.json`、`observations-empty.json`、`attempts-empty.json`）：留下。样例目录的契约是 26 个文件；空列表必须是 `[]`，不是 `null`。
    - R3 `--mode` 在本地校验（`off|observe`）：留下。
    - R4 游标和每页条数走查询串（`url.Values.Encode`），不是路径段：以设计 12.2 为准。说明书里那句相反的话是主会话写错的。
    - 另有一条读法先接受为已知限制、写进规格：写操作遇到不带预算错误形状的 4xx（包括处理器前面返回的 403/404）一律报“结果未知”，而不是“被拒绝”。其余 35 条读法等第一轮交卷时逐条过。
  - 名额的顺序没变：5d 交卷并验完就派 5c（关键路径）；5e 腾出的名额给 `f3-check-6kd2`；之后是 `f3-check-6kd1`、`f3-check-7kb`、6a。
- 19:30–20:15 7b 第一轮由主会话验过并接受，第二轮（Core 与内置 skill）已发：
  - **主会话自己的核对**（19:45–19:50）：四份命令文件对比 09:03 留的副本，只有 `cmd_issue_budget.go` 的两行不同（一句注释、下一页提示的措辞），其余三份逐字节相同；十个文件的 SHA-256 与报告第 5 节一致。26 份样例：主会话照设计 12.9 写的检查器（不是车道的生成器）零发现，摘要与车道留的清单一致。基线（`/tmp/f3-main-s7k/base-r1/`）：两条工具链 gofmt/build/vet 全 0；`TestIssueBudget` 34 条（含子用例 824）在两条工具链和 race 下全过；整包在 CLI 守卫下 524/2110，无失败无跳过。数字与车道报告的逐项相同。
  - **车道自己发现设计在它干活期间动过**，并逐节重读核对，结论是命令遵循的规则没有变。主会话从会话记录里核了：第 12 节在它开工之后有四处补充（错误正文由结构体输出、handler 判定顺序与 issue 加载器的正文不外传、读取侧首包不发布缓存、`rootId` 自己解析），全是服务端怎么产出应答，样例和读取一方不受影响。
  - **四项裁决按 17:30–19:30 那条的意向定了**，36 条读法全部接受。另接受两条已知限制并写进规格：非成员的写操作得到“结果未知”（成员校验中间件的 404/403 不是预算 handler 的正文，命令不认它是拒绝，宁可说不知道）；新 CLI 对着没有预算路由的旧服务端同理。车道还指出设计 12.1 里接受下来的那个边角在命令上的表现（启用已提交、应答丢失、根任务随后被删，按原 key 查询得到 404），维持不变。
  - **20:10 停掉 7b 并发出第二轮消息**（`/tmp/f3-main-s7b/round2-7b.md`）。停的原因：代码 17:22 起就没再动，之后两个多小时它在一遍遍复查报告，第九次压缩之后 50 分钟没有落盘；它笔记里列的收尾事项（改三处引用、两句话、写第 6、7 节、状态行）由消息规定“最多五次编辑、不再重读”。第二轮在 D、E 两个阶段之前先补三处小测试：每行钉住退出码（R1）；成员校验中间件的两种应答原样进“结果未知”的表，并配一个变异；服务端文本里的转义序列和换行不能在终端上画东西（它的读法 27，此前没有测试）。
  - 监视器 19:38 第三次到点（没有边界），重挂一个到 20:36 前后。
- 20:15–21:30 5d 交付并验收，两项裁决已定，5c 已派发：
  - **交付前的预审（20:15–20:45）**：读了 5d 的两处代码新增（溢出回退、`member_deleted` 归属差异观察）和 `appendEntries`，没有发现缺陷：`appendEntries` 先编码全部行、再问纯函数包（`ledger.With`），之后才发第一条语句，所以被拒的那一批没有留下任何行、也没有动根行，回退时同一批 hold 可以原样重发。把我自己想到的变异体写成清单（`/tmp/f3-main-s5v/mutants-5d-main.md`）。同时读完了 5e 的测试文件（H01–H11 加缓存那一半，1,055 行，没看到缺陷）；7a 任务书的样例摘要占位换成了摘要文件（`/tmp/f3-main-s7a/fixtures-v1.sha256`，26 行）并写明空列表是 `[]`；5c 任务书最后一个占位换成“基线由主会话在冻结副本上测、写到 `/tmp/f3-main-s5c/baseline.txt`”，这样 5c 的派发不必等我的全量验证。监视器 20:37 到点（无边界）重挂；7b 20:19 按消息收掉了第一轮（报告 109 KB）。
  - **5d 21:09 交付**（`research/step5d-finalize-tests.md`，36.6 KB）：30 条 `TestIssueBudgetFinalize…`（三个测试文件，共 3,549 行）、F01–F22 对照表、37 个变异体（杀 36，E20 幸存）、一项待裁决、九条偏离。代码只有预审过的那两处新增。
  - **主会话的验证**：20:46 的快照（`/tmp/f3-main-snap/f3-20261119-2050-5d`）物化成 `/Volumes/artisan/code/2026/mc-f3-verify-5d`，并入 5e 的测试文件；旧的 `mc-f3-verify-5b` 核对过没有独有文件后移除。五个文件的 SHA-256 与报告一致，交付后未再变。脚本 `/tmp/f3-main-s5v/verify-5d.sh` 不重建模板库（5e 正在克隆它，且建模板以来迁移没有变：868 个文件、头 880）。结果（`/tmp/f3-main-s5v/s5d-summary.txt`）：两条工具链 gofmt 0/65、build/vet 0、`internal/budget` 112/0/0；服务层 `TestIssueBudget` 95/0/0（go1.27.0、go1.26.9 及两者的 race）；处理器层预算相关 38/0/0（同样四种，含 5e 的 12 条 `TestClaimBudget…`——这是 5d 的代码与 5e 的测试第一次在同一棵树上跑）；迁移 103/0/0，`cmd/migrate` 67/0，1 条跳过。写这条时 service、handler 两个整包还在跑。
  - **我自己的变异体**（`/tmp/f3-main-s5v/mutants-5d.py`，8 个，都是车道清单里没有的）：杀 6（溢出缺口的 detail 置空、不重试、重试仍带金额、清掉错误但什么也不追加、缺口只在有上限的根上写、Error 日志整条去掉）；幸存 2：去掉 `!reserve.Unsized &&`（等价，不带金额的预留不可能溢出，预期如此），以及**去掉 Error 日志的第五个字段 `attempt_reserve_ticks`**。报告偏离 7 说加了这个字段，但测试辅助函数只查四个，是个真的小缺口。
  - **裁决一：没有 `TxStarter` 的服务上被拒的 claim 留下任务令牌——维持现状。** 没有事务时，finalize 任何一步失败都会留下前面各步写的东西（令牌之后还有 daemon 令牌、评论回执），这是那条路径本身的性质，不是预算步骤带来的；预算步骤在那条路径上确实什么也不写，这正是“无事务规则”承诺的。生产环境一律带 `TxStarter`（`internal/handler/handler.go:560`）。把拒绝提前，等于给只有测试才走的路径再造一套步骤顺序。F14 继续钉错误和预算表不变，不钉令牌数（钉下来就把一个偶然固化了）。规格里写一句。
  - **裁决二：E20 背后的不可达分支保留，并用直接调用钉住。** 事务路径里“issue 行不存在”这一支（`issue_budget_finalize.go:245-248`）经 `FinalizeTaskClaim` 到不了：`lock_issue_execution` 先把这样的 claim 判成 blocked，`agent_task_queue_issue_id_fkey` 的级联又让任务随 issue 一起消失。但“查无此行”总得有个答案，另一个答案是让 finalize 失败、任务永远回队列，更糟；而且本仓库的方向是去外键。所以分支留下，测试直接调用 `finalizeTaskBudget`（在内存里构造一个 issue 不存在的任务值），注释里说明为什么只能直接调。车道的判断是“为它弯折用例不值得”，它考虑的是让 DELETE 穿过 finalize 自己的事务，直接调用不属于那种弯折。
  - **两个小尾巴交给 5c 的“阶段 0”**（任务书已加，约十次工具调用，超过十五次就记下进度继续往下）：给 `requireIssueBudgetOverflowLog` 补第五个字段并用变异体证明；写上面那条直接调用的测试并用变异体证明。之后 `TestIssueBudgetFinalize` 是 31 条，服务层 `TestIssueBudget` 是 96 条。
  - **留给第 5 步独立复核（5k1）的一个问题**：带评论回执（`recordCommentReceipt=true`）时预算步骤仍在最后。5d 的语句顺序断言只覆盖不带回执的 finalize（辅助函数的注释自己写明了）。今天把预算步骤挪到回执之前不会多拿一把锁（任务行早已锁住），所以不是缺陷，但 `task.go` 注释里那句“with or without a comment receipt”没有测试。由 5k1 查处理器层是否已钉住，没有就在它的后缀测试文件里补。
  - **5d 报告里的一条后果，进规格**：被拒的那一代 claim 留下的 `in_flight_legacy` 缺口在首包里永远不会被覆盖，未覆盖缺口按整个根计数，所以该根只要有上限，之后每次 claim 都会记 `unbounded_unknown`。
  - **21:26 派发 5c**（`f3-lifecycle-5c`，第一轮：阶段 0 → E → A、B）。派发前把 5e 的测试文件拷进了 F3 工作树（SHA-256 `bf7626d5…7f38`，与 5e 工作树一致）；5e 还没交付，它若再改这个文件由我重新拷入，任务书里说明了这是工作树里唯一由主会话写的文件。监视器改为 `7b2 5e2 5c1`，到 22:21 前后；顺手修了脚本的一个漏洞：第一轮的 `STATUS: round 1 blocked` 此前匹配不到。
  - 名额：7b（第二轮）、5e、5c，三个，满。5e 交付后名额给 `f3-check-6kd2`。
- 21:30–次日 00:50 5e 交付并验收，四项裁决已定，`f3-check-6kd2` 已派发；整包里那条协作用例的失败查清了：
  - **5e 21:34 交付**（`research/step5e-claim-endpoints.md`，28.8 KB）：12 条 `TestClaimBudget…`（含子用例 30），一个测试文件 1,054 行，生产代码没有改。变异体：任务书的 7 个（第 6 个拆成单端点、批量两处）加自拟 3 个，全杀。另有入口清点 11 项、requeue 调用点表、绑定期间的圈数实测。它的三个数据库已删。这些测试在主会话的验证里已经跑过（文件 21:24 拷入冻结副本）。
  - **主会话验证的收尾**（`/tmp/f3-main-s5v/s5d-summary.txt`）：handler 整包 3040/0/53（含子用例 5985），205.9 s，与 5e 自己的数字相同。service 整包 **1707/1/18**（4099/3/18），1310.6 s；失败的是 `TestStageExecutionMetricAmbiguityInvalidatesExistingRollback`（`collaboration_stage_execution_test.go:181: stage_execution_fact_missing`），18 条跳过都是 `REDIS_TEST_URL not set`。
  - **这条失败与预算代码无关，证据如下。**
    1. 用例文件和它测的 `collaboration_stage_execution.go` 在基线提交与 F3 冻结副本里 SHA-256 相同。F3 在 service 包里改的 12 个已有文件中只有 `collaboration_edit.go` 属于协作，改的是改父分支，这条用例不经过。
    2. 被测代码带墙钟预算：回调之后记录结果 5 s，派发窗口 30 s，夹具里的有效期 20 s 和 60 s。
    3. 22:16–22:25 在冻结副本上单独连跑五遍，2 过 3 败，子用例耗时 15–180 s。当时 5c 正在同一个仿真 PostgreSQL 上跑迁移套件。
    4. 23:12–23:14 在基线提交的干净导出（`/private/tmp/f3-main-pristine-86fea`，模板 `f3_budget_base_865`）与冻结副本上交替各跑四遍，每遍一个新克隆：8/8 通过，子用例 2.4–9.7 s（`/tmp/f3-main-s5c/flake-ab/`，脚本 `flake-ab.sh`）。
    - 结论：数据库慢到超出用例自己的墙钟预算时它才失败。没有做到的：没有在干净基线上复现出失败，复现需要同样的负载。已写进 5c 的基线文件：遇到它就在自己没有别的套件在跑时单独重跑，两次结果都报告。属于任务外发现，收尾时报给用户。
  - **5c 的基线**（`/tmp/f3-main-s5c/baseline.txt`，脚本 `mk-baseline.sh`，22:32 写完，23:20 补了上面那段）：`-run 'Budget'`（service 加 handler）161/0/1（含子用例 283/0/1），go1.27.0、go1.26.9、`-race` 三次相同，没有 `DATA RACE`，跳过的一条是 Redis 限流用例。`cmd/server` 整包 477/0/9（1123/0/9）。其余数字取自验证脚本的日志。
  - **5e 的四项裁决**，已写进设计 10.10、10.11 的“2026-11-20 补充”。
    1. 缓存那一半的用例（`TestClaimBudgetBindingInProgressInvalidatesEmptyVerdict`，内存里的 Redis 替身）保留：默认测试集里只有它覆盖“放回之后缓存的‘没有任务’失效”。
    2. 绑定期间的重领**不加延迟**。上界：持锁 1 s 时单端点约 91 圈，批量端点两条 task 约 43 圈（86 次放回）；正常的绑定持锁 10–100 ms，0–9 圈。每圈每条 task 两条 info 日志、一条广播，什么都不丢，没有按代数或放回次数的上限。下限留给 F3-S2。
    3. 重领第二次经过预算步骤：10.2 的取代规则，5d 的三条用例已钉。
    4. 名单之外的入口：心跳带的 daemon 工作与云端节点命令不是执行；服务端自己的模型调用是真实支出，但不在首包对象内，**不进 `not_applicable`**（v1 已冻结），规格写明“根的合计不含它们”，产品问题收尾时交给用户。
    - 另：能力标记只读第一行同名头，写进规格，不改。
  - **写补充时多想到的一条后果**（设计 10.10）：圈数让“误报”更容易出现。在途扫描落在某一圈里时，被放回的那一代留下一条不会被覆盖的 `in_flight_at_bind`，该根从此是“未知”。上限是每个绑定方、每条被绑定的 task 一条。首包接受，收窄办法留给 F3-S2。
  - **21:55 派发 `f3-check-6kd2`**（第 6 步 daemon 一半的独立复核之二；派发前清单 21/21 一致）。到 00:45 它读了 27 个文件、跑了 26 条命令，刚开始跑基线，报告还是开头的 1.6 KB。没有压缩，也不是空转。
  - **5c 的进度**：22:11–22:24 做了阶段 E 的迁移部分，新建 `880_issue_budget_observation_gap`（部分索引），就绪检查改名为 `881_issue_budget_ready`。**后果：模板库 `f3_budget_s5v_tmpl`（头是 880 的就绪检查）只对应 5d 交付时的树。** 验 5c 第一轮要另建一个模板，不重建这个，冻结副本还在用。00:46 它在清点任务状态的写入方（阶段 A、B），报告 12.8 KB。
  - **7b 第二轮**：阶段 D（Core 的 schema 与测试）走了红、红、绿三步，00:44 重写了测试文件。
  - **5e 的工作树已移除**（00:50）。移除前逐个文件比过：它自己的测试文件在 F3 里逐字节相同；与 F3 不同的 8 个文件在 F3 里都是 5d、5c 之后改的新版本；两份旧的 `880_issue_budget_ready` 在冻结副本里还有；未跟踪目录 `internal/budget/` 21 个文件全部相同。
  - 监视器 22:31、00:06 两次到点都没有边界。脚本加了 `6kd1`、`6kd2`、`7kb` 三个映射，00:47 重挂 `7b2 5c1 6kd2`，到 01:42 前后。
  - 三条车道都慢，工具调用之间隔 4–20 分钟（三条车道加主会话同时在跑）。没有空转的迹象，不干预。
  - 名额：7b（第二轮）、5c（第一轮）、6kd2，三个，满。之后的顺序：`f3-check-6kd1`、`f3-check-7kb`（7b 交付后）、5k1 与 5k2（5c 交付后）、6a。
  - **01:00 看了一眼合并面**（第 8 步用）：共享分支 HEAD `6a7f76cf1`，比 F3 基线 `86fea651c` 多 36 个提交、1,101 个路径，迁移已提交到 868（`866_collaboration_information_metadata`、`867_collaboration_information_ready`、`868_collaboration_runtime_output`）。与 F3 工作树 79 个路径的交集只有三个文件：`service/issue.go`（对方改 `CreateInTx` 开头的协作根检查，F3 在后面加建子任务的绑定）、`service/task.go`（对方改 `ClaimTasksForRuntimes` 的重领探测，不改重领的代数规则，也不碰 `FinalizeTaskClaim`）、生成的 `models.go`（重新生成）。7b 的树另有 `packages/core/api/client.ts` 与 `cmd/multica/cmd_issue.go` 两个。都是不同的函数或不同的段落，目前看不到语义冲突。F3 的迁移（866–881）第 8 步要改到 868 之后，以届时的最大编号为准。任务目录仍未被别的会话提交。
- **01:05–02:15 等车道期间：预审 5c 的前两个阶段，给后面的车道备料；5c 自报读了主会话的原因表**
  - **5c 的阶段 0 与阶段 E 预审过，没有发现缺陷**（看的是工作树里的差异，不是它的报告）。阶段 0：`requireIssueBudgetOverflowLog` 现在要求日志带 `attempt_reserve_ticks=<那个超大值>`；新用例 `TestIssueBudgetFinalizeStepGoesOnWhenTheIssueRowIsGone` 直接调预算步骤，传一个没有对应行的 `task.IssueID`，要求不报错、不签发 attempt、一条带 `task_id` 与 `issue_id` 的 WARN、库里什么都没变。`issue_budget_finalize.go` 没动。阶段 E：新迁移 `880_issue_budget_observation_gap`（`(budget_root_id, reason) WHERE kind = 'coverage_gap'` 的部分索引），就绪检查顺延为 `881_issue_budget_ready`，清单加第五列 `predicate`；`cmd/migrate/main.go` 的并发索引清理表、三处测试相应更新。
  - **就绪检查的一个已知边界，接受**：881 比对谓词时把空白、括号与 `::text` 全部去掉，字符串字面量里面的也去掉。名字与键都对、谓词字面量只在这些字符上不同的索引（例如 `'coverage_gap '`）会被放过。后果只是那条按原因计数的查询用不上索引，不影响正确性。第 5 步复核（5k2）的“已接受事项”里写明，第 8 步写进 spec。
  - **5c 自报了两件事**（它的笔记 `/tmp/f3-s5c/notes.md` 的 MUST REPORT 两段，交付时会写进报告）：
    - 它在起草原因表之前读了 `/tmp/f3-main-s5c/reason-table-main.md`。那是主会话自己的清点，本来要拿来和它交的表对比，不该让车道先看到。这是主会话的疏忽：文件放在简报让车道去读的同一个目录里。01:50 把这份表和 `terminal_writers.py` 移到 `/tmp/f3-main-review/`，没有任何简报提到这个目录。后果：“两张表独立得出、互相印证”不成立了。仍然算数的是：主会话在验收时逐行对代码；5k1 复核时读的是代码，不读主会话的表。
    - 它为了查 `agent_task_queue.status` 的取值，对模板库 `f3_budget_base_865` 本身执行过一条只读的目录查询（没有写入，连接随即关闭）。规则是不碰模板库；它自己记下了不再这样做。接受。
  - **主会话为验收原因表先读清楚的几处**（F3 树，`server/`）：
    - 两个领取入口都是先拼应答、再调 `FinalizeTaskClaim`（`internal/handler/daemon.go:1921`、`:3689`）。启动前的失败与取消（`failClaimedTaskBeforeLaunch` → `FailClaimedTask`、runtime 没有 owner 时的 `CancelClaimedTask`）都发生在 finalize 之前；`settleKnowledgeFinalizeRefusal` 发生在 finalize 失败（已回滚）之后。所以这些路径碰不到当前代的 attempt，行上更早的 open attempt 走 10.3 的代数规则（`superseded_unproven`）。
    - `RequeueTaskAfterClaimFailure` 的十个调用点都在 finalize 没有提交的路径上；finalize 成功之后两个入口都不再 requeue。所以处于 `queued` 的行没有当前代的 open attempt，`ExpireStaleQueuedTasks`（`queued_expired`）与 `FailExpiredRuntimeReconnectRetries`（`runtime_reconnect_timeout`）只会遇到更早的代。验收时再确认一件事：有没有同一行从 `dispatched` 回到 `deferred` 的路径。
    - `status = 'failed'`、`failure_reason = 'timeout'` 有两个写入方，行上分不开：sweeper 的 `FailStaleTasks`（`error = 'task timed out'`；只在 daemon 证明不了自己活着时触发，按 10.3 是 `lost_contact`）；daemon 自己上报的 agent 超时（`internal/daemon/daemon.go:8139`，字面量 `timeout`）与准备阶段超时（`taskRunFailureReason`，`:5392`），按 10.3 是 `timeout`。`FailAgentTask` 不留下“这是 daemon 报的”的标记。只有 daemon 才会写的超时类字符串：`codex_semantic_inactivity`、`codex_resume_oversized`、`idle_watchdog`、`agent_error.agent_timeout`、`runtime_cli_timeout`。
    - `status = 'failed'`、`failure_reason = 'cancelled'` 存在：daemon 把一次以取消告终的运行经 `/fail` 报上来（`daemon.go:5598`、`:5698`）。
    - `FailAgentTask` 把 daemon 给的原因文本原样存下（先过 `NormalizeDaemonReason`），所以会有表里没有的字符串；这条查询不会写出 NULL。
  - **待裁定（等 5c 的表）**：`('failed','timeout')` 记 `lost_contact`（共用的字符串按最弱的断言记；代价是最常见的 agent 超时在 `show` 里显示成失联）还是按 `error` 是否等于 sweeper 的常量区分；未知字符串默认记什么。倾向前者与“未知记 `lost_contact`”，并要求两条守卫用例：扫查询文件里的 `failure_reason = '<字面量>'`，表里没有的就红；遍历 `taskfailure` 的原因清单，表没裁定的就红。主会话的预审笔记在 `/tmp/f3-main-review/5c-prereview.md`。
  - **备料**：
    - `/tmp/f3-main-s5v/verify-5c.sh <工作树> <标签> [quick]`：5c 加了迁移，旧模板 `f3_budget_s5v_tmpl`（头是 880 的就绪检查）只和冻结的 5d 副本对得上；这个脚本从给定工作树的迁移另建 `f3_budget_s5v_tmpl2` 再跑。`run.sh`、`mk-template.sh` 改成从 `F3_TMPL` 取模板名。
    - 5k 的三份简报（`/tmp/f3-main-s5k/`）：已定的内容填了（五条车道的声明清单、10.4 的两个内存数字、绑定拒绝不加延迟及其后果、迁移 866–881 与共享分支自己的 866–868、已接受事项、留给 5k1 的问题：预算步骤排在最后这一条有没有连同评论回执一起钉住）。快照路径、清单、基线数字等占位符要等 5c 两轮都交付。
    - 6a 简报：`FIXTURES_FOR_ATTEMPTS` 换成 service 包现有帮手的表（建根、建 attempt、读账目、双连接、日志断言）；接口草稿里的 `PENDING` 去掉。还差 `6A_BASELINE_COUNTS`。
    - 7a 简报：`FIXTURES_FOR_HANDLER_TESTS` 换成 handler 包现有帮手的表，身份一栏写的是具体函数（`newRequest`、`newRequestAs`、`asAgent`、`taskActorReq`、`newDaemonTokenRequest`、两个任务令牌帮手），并写明中间件里的检查（成员资格、no-store、路由接受哪些凭据）直接调 handler 测不到，要像 `cmd/server` 的用例那样走路由，报告里逐行说明是在哪一层证明的。还差 `6A_STRICT_DECODER`、`7A_BASELINE_COUNTS`。
  - 01:43 监视器到点，没有边界；重挂 `7b2 5c1 6kd2`，到 02:38 前后。三条车道都在动：5c 在收尾阶段 A 的清点（任务行怎么消失、失败原因常量），7b 在阶段 E（内置技能文档），6kd2 在跑变异（03 组，发送路径）。
- **02:15–04:10 原因表定案；预读 5c 的阶段 A；第 8 步的改号脚本；7b 的一次测试运行写了默认开发库**
  - **原因表定案，改了上一条里的“倾向前者”**（设计 10.3 的“2026-11-20 补充”，提交前可推翻）。5c 的表第 1–7、9、10、11 行照它起草的确认。第 8 行（`failed` 加 `timeout`）**不再一律记 `lost_contact`，按错误文本拆开**：`error` 恰好等于 sweeper 写的常量 `task timed out`（`FailStaleTasks`）记 `lost_contact`，其余文本（含空）记 `timeout`。
    - 为什么改：验收行 B36 要求超时算在它自己的原因下，而 daemon 上报的裸 `timeout` 正是最常见的超时；一律记失联会让 `show` 里几乎看不到 `timeout`。安全方向没有丢：daemon 恰好发来同样的文本时，记成的是较弱的那个原因。
    - 代价是和一段消息文本耦合，所以要两条用例钉住：一条不连库，查询文件里名叫 `FailStaleTasks` 的语句把 `failure_reason = 'timeout'` 与 `error = '<常量>'` 一起写，别的语句都不写这个字面量；一条连库，用生成出来的真 `FailStaleTasks` 让任务失败，再对账。
    - 另加一条守卫：查询文件里凡是置 `status = 'failed'` 并写字面量 `failure_reason = '<x>'` 的语句，`<x>` 必须在表里。它和 5c 已有的“`taskfailure.AllReasons()` 每个都得放进表”是一对。
    - 表不是两边独立得出的（上一条已记）。每一行主会话都对过代码，第 5 步复核再对一遍。
    - 给 5c 的第二轮消息草稿在 `/tmp/f3-main-review/round2-5c-draft.md`，裁定部分写完，验收部分等它交第一轮。
  - **预读 5c 阶段 A 的代码**（03:27，`internal/service/issue_budget_lifecycle.go` 298 行与查询 `ListIssueBudgetOpenAttemptsOfRoot`；只看了代码，还没对用例），**没有看到缺陷**。它只取根锁，锁后才列 open attempt，任务行不加锁读；`off` 的根跳过；账本装不上时记 error、什么都不改；有预留才写 `hold/<attempt>/<原因>`，关闭必须恰好影响一行，所有 hold 一批追加。验收时要问的：
    - `closed`、`tombstoned` 而开关没关的根照样对账（代码只看 `mode`，与设计一致），有没有点名这两种状态的用例；
    - `issue_budget_finalize.go` 03:00 的改动是什么；
    - 查询要多返回 `error` 列（上面的拆分需要）；
    - 同一行有没有从 `dispatched` 回到 `deferred` 的路径；
    - 它的库是怎么建的。
  - **第 8 步的改号脚本备好了**：`python3 -I /tmp/f3-main-s8/renumber.py <工作树> <新的首号> [--apply]`。它找出 `NNN_issue_budget*` 的迁移，要求编号连续、上下行齐全，首号不高于树里别的迁移的最大编号或有撞号就拒绝；改写已修改与未跟踪文件里的迁移名，以及 F3 迁移文件注释行里的裸编号；分两步改名。不带 `--apply` 只打印计划。在 F3 树上试到 869：16 个迁移名 866..881 → 869..884，5 个文件 54 行要改，32 个文件要改名（`/tmp/f3-main-s8/dry-869.txt`）。`--apply` 在临时副本上跑通过，副本已删。**注意**：F3 工作树的基线里没有共享分支的 866–868，首号要按第 8 步当时共享分支的最大编号定（今天是 868）。
  - **预读 7b 第二轮的 Core 一半，没有看到缺陷**：`packages/core/api/issue-budget-schemas.ts` 的 schema 与五份 `show-*.json` 夹具逐字段对得上；`getIssueBudget` 走 `parseWithFallback`，解析不了回 `null`，拒绝仍是 `ApiError`；测试文件开头是 `// @vitest-environment node`。没有 UI 用到它（设计 12.7）。
  - **预读 6kd2 报告的前半**：A1、A5、E1、E2 都是 `holds`，变异全杀。它记下一条方法上的坑：接线用例是从磁盘解析 `daemon.go` 的，只 overlay `daemon.go` 会读出七个假的幸存者，要把用例文件的路径也一起 overlay。之后对 `Run` 做变异的复核都要照做。
  - **事件：7b 的一次测试运行写进了默认开发库 `multica`**（它的报告“Needs a ruling”R5；它发给主会话的消息没有送达，是主会话读它的报告时看到的）。
    - 经过：03:13:08–03:14:21，7b 在阶段 E 之前自己加了一次基线，跑了整个 `server/internal/service` 包，没有设 `DATABASE_URL`。简报要的只是点名的五条用例，并且写明“不连库”。这个包里有五个帮手在变量为空时回退到 `localhost:5432/multica`（`newResolveOriginatorPool`、`newTaskClaimRacePool`、`newCancelFinalizePool`、`newHeadShaDedupPool`、`newPluginStoragePool`），那里正好有服务在听。254 条顶层用例能走到这些帮手：155 过、38 败、61 跳过。
    - 车道的处置是对的：停下，写进报告，脚本改成只跑点名的五条，**没有去查也没有去清那个库**。
    - **主会话的只读检查**（03:39–04:05，只有 `SELECT`，没有写入）：
      - 5432 上听的是共享容器 `multica-postgres-1`。`multica` 库 53 MB，检查时没有连接，迁移头是 `763_collaboration_knowledge_delivery_history`（比各棵树落后一百多个迁移，所以那次运行里有 50 条 `collaboration_stage_child` 不存在的错误）。
      - **没有登记的开发环境在用它**：`~/.multica/dev/envs` 与 `archived-envs` 里没有一个文件的库名是它；主检出的 `.env` 指向 `multica_multica_0_4_37_492`。库名 `multica` 只出现在 `.env.example` 和代码的回退地址里。
      - 库里是测试夹具：一个 2026-09-11 建的 `test-workspace`，17 个用户的邮箱域都是 `*.test`、`*.invalid`、`example.com`，没有 issue、agent、task、评论。从 9 月 11 日起至少 11 天有同类运行留下的行（例如 `autopilot_rule_version` 2,845 行、`issue_status` 989 行、`agent_invocation_target` 8,726 行）。也就是说，没设变量的运行落到这个库上是早就有的事，不是这次才开始。
      - **这次留下的行**（`created_at` 或 `captured_at` 落在运行窗口内）：共 666 行，分布在 9 张表：`collaboration_config_invalidation` 647、`autopilot_rule_version` 8、`channel_task_delivery` 3、`user` 2、`issue_source_context` 2，`channel_chat_session_binding`、`channel_installation`、`chat_draft_restore`、`issue_status` 各 1。
      - 运行之前就在、现在还在的行，没有一行的 `updated_at` 落在窗口内（查了所有同时有两个时间列的表）。
      - **没有查清的**：有没有更早留下的行被这次的清扫类用例删掉。没有运行前的行数可比，统计视图的累计值也跨了多次运行。能说的只是：被删的只可能是同一类测试残留。
    - **裁定**：
      1. 车道不检查、不清理，维持。
      2. **主会话也不清理。** 这个库不是本任务的；删行还是整库重建由用户决定，收尾时连同上面的事实报给用户。留下的 666 行不影响任何在用的环境。
      3. 之后每条车道的每一条 `go test` 都**显式**带 `DATABASE_URL`：要库的指向车道自己的克隆；不要库的指向一个连不上的地址，`postgres://nodb:nodb@127.0.0.1:9/nodb?sslmode=disable&connect_timeout=2`。04:08 在主会话的验证树上证过：五个帮手各取一条用例，5/5 跳过（`database unreachable … connection refused`），没有一条通过或失败（`/tmp/f3-main-review/nodb-recipe.out`）。
      4. 回退地址不止 service 包。`server/` 下带同一个回退的测试文件：service 5 个，`internal/integrations/` 7 个（dingtalk、ghsnapshot、lark、wecom 3 个、channel/engine），`internal/middleware/workspace_test.go`，`internal/scheduler/stale_steal_test.go`，三个回填包（`taskusagebackfill`、`issueactivitybackfill`、`attributionbackfill`），`cmd/migrate/migrate_concurrent_test.go`，`cmd/backfill_codex_usage_cache/integration_test.go`。handler 包与 `cmd/server` 的用例没有。写进所有还没派发的简报，第 8 步写进 spec。
    - 属于任务外发现与事件，收尾时报给用户。
    - **善后（04:12–04:45）**：
      - 裁定已发给 7b（它本轮结束时才会收到，所以它会先写 `STATUS: complete`，再多跑一小轮改报告；快照要等这一小轮结束）。
      - 五份还没派发的简报加了这条规则（脚本 `/tmp/f3-main-review/add-db-rule.py`，各留 `.bak-20261120b`）：6a、7a、5k 公共部分、7kb（第 4 节第 6 项原文就是“不连库跑 service 包里点名技能的用例”，正是这个坑，已改成点名加连不上的地址）、6kd1（只许跑 daemon 与 `pkg` 的包）。给 5c 的第二轮消息草稿也加了。
      - 顺带看清一件事，写进了简报：`internal/handler` 与 `cmd/server` 的 `TestMain` 在变量为空或库连不上时直接以 2 退出，所以这两个包里“不需要库的用例”也得连着自己的克隆跑。7a 简报里“handler 包，不连库”的说法指的是用例本身不碰库。
      - 另两条在跑的车道核对过：自 11 月 16 日（本任务开始）起，`multica` 库里除这次的窗口外没有任何新建的行。5c 的 `go test` 都经它自己的脚本（`/tmp/f3-s5c/gotest.sh`），库名不是 `f3_budget_s5c_*` 就拒绝；但脚本有一个 `-` 模式会把变量清空，第一轮验收时要它改成连不上的地址。6kd2 只跑 `./internal/daemon/`，那里没有连库的代码。
      - 记忆条目 `db-backed-go-tests-skip-silently` 更新了：原文说 handler 的 `TestMain` 会回退并以 0 退出，现在已不成立；补上了这次的坑与配方。
  - **预读 7b 的阶段 E（内置技能文档，03:36–03:48 停笔）**：`SKILL.md` 加一句（`issue budget` 只给人用，命令与服务端都拒绝 agent 与 task 凭据，读也不行），细节放在 `references/report-snapshots.md` 新的一节，source map 加一节四行。措辞与设计 12.1、12.3 一致。验收时要问的两点：source map 第三行把来源写成本任务的 `design.md`（服务端一半还没实现；先例是同一文件第 322 行引用 F2 的任务文档），7a 落地后要改成 handler 及其用例；`SKILL.md` 的行数上限用例（500 行）它跑过没有。
  - **5c 自己读到了设计 10.3 的补充**（04:11、04:20 两次重读设计），04:25 起已在第一轮里做按错误文本的拆分：查询加了 `error` 列并重新生成，原因表、真 `FailStaleTasks` 的用例、扫查询文件的守卫都在写。第二轮消息里的裁定部分届时改成对照检查，不再是新要求。
  - 监视器 02:39、03:56 两次到点都没有边界，各重挂一次（现在这一个到 04:51 前后）。04:45 三条车道都在动：7b 在更新报告（126 KB），5c 在跑阶段 A 的用例，6kd2 在读发送路径的测试帮手。
- **04:45–06:00 等车道期间：备好下一批派发；主会话自己的脚本也按新规则改了**
  - 监视器 04:52、05:48 两次到点仍无边界，各重挂一次（现在这一个到 06:47 前后）。05:52 三条车道都在动：7b 在收尾第二轮报告（151 KB，05:40 还在改措辞）；5c 在写第一轮报告（21 → 33 KB）并做阶段 B 的变异；6kd2 在跑第 7 组变异。
  - 简报里已能定的占位符填了：7kb（第二轮消息的路径；已裁定事项 7–14）、6a 与 7a（5d 落定的账本接口一节、离线 sqlc 的路径）、5k2（原因表改为指向设计 10.3 的补充，并说明“你做的是第一张独立的表”；四条已接受事项）。还空着的都要等车道的冻结清单与基线数字。
  - `/tmp/f3-main-s7k/freeze-7b.sh`：对 7b 的树只读，写出调用方一半的文件清单与摘要（16 个文件加 26 个固定样例，共 42 个），并核对样例摘要。05:00 试跑：42 个文件，样例 26/26 一致。7b 最后一小轮结束后重跑，届时的清单才算数。
  - 5c 的 `gotest.sh -`（清空变量的模式）查清了：一共用过三次，都带 `-run` 点名。一次是 `./internal/migrations/`（这个包没有回退），两次是 service 包里它自己那条不连库的守卫用例。加上库里的证据（自 11 月 16 日起窗口外没有新建的行），5c 没有碰到回退库。第一轮验收时仍要它把这个模式换成连不上的地址。
  - 主会话自己的脚本也查了一遍。`/tmp/f3-main-s7k/baseline.sh` 原来是 `unset DATABASE_URL`，而且第二轮模式会按传入的正则跑 service 包，与 7b 出事的那条命令是同一个形状；这一支还没用过。05:45 改成全程导出连不上的地址，并拒绝空的或匹配整包的正则（备份 `.bak-20261120d`）。`/tmp/f3-main-s5v/verify-5c.sh` 里连库的步骤都经 `run.sh` 在命令上显式给克隆的地址，不带变量的只有 `./internal/budget/...`（没有连库的代码）；也加了同一个导出，让规则对主会话字面成立。
- **06:00–06:30 5c 阶段 A 先审；7b 的树上先量第二轮基线**
  - **5c 的阶段 A 在 05:54 收口（它已转去做阶段 B），主会话逐行读了产品代码**：`issue_budget_lifecycle.go`（311 行）、查询 `ListIssueBudgetOpenAttemptsOfRoot`、`issue_budget_finalize.go` 相对 5d 冻结版的差异（只多了来源 `lifecycle` 与四个尝试状态共五个常量，没有改任何语句）。判断顺序与设计 10.3 的补充一致：行没了 → 状态未终结（保持 open）→ 代数低于任务行 → `completed` → `cancelled` → `timeout` 且错误文本恰为 `task timed out` → 原因表 → 兜底。事务只拿根锁；账本读不出时记一条 ERROR 且什么都不改；`Committed()` 在 `transact` 无错返回之后。它自报 18 条用例、24 个产品代码变异加 3 个查询文件变异全部被杀。**代码按现状接受。**
  - 要它在第二轮补的四条（已写进 `/tmp/f3-main-review/round2-5c-draft.md` 第 2 部分），都不改对账的语句：
    1. `admission_state = 'closed'` 的根、根任务已删除的根各一条用例（对账只看 `mode`，这两种根照常对账，现在没有用例点名）。
    2. **未定额的预留**：根没有声明每次尝试的预留（`attempt_reserve_ticks` 为 NULL，也就是没人配置过的根，生产里最常见）时，结束的尝试应写一条挂起、不动任何 tick、该原因的未知计数加一。现有用例都声明了 100，另一条是完全没有预留的尝试，中间这种缺了。
    3. 经真实入口 `TaskService.FailTask` 失败一条 `timeout` 任务（daemon 式的错误文本），要求挂起原因是 `timeout`（现在原因表的用例都是直接改行）。
    4. 一次对账没有上限（查询没有 LIMIT，一个事务在根锁下关掉这个根所有已结束的尝试）：写进报告的已知限制，本包不加上限。
  - 任务状态“终结后不再变”是无锁读的前提，5c 的报告逐条列了证据（没有任何查询把终结行改回去；`claim_generation` 只在三条认领语句里加一，从不减）。主会话的判断：即使以后有语句复活终结行，只要复活后先重新认领再运行，账也不会错（旧尝试要么已被对账关掉，要么被新认领的收尾以 `superseded_unproven` 关掉，都只有一条挂起），所以不为此加守卫。
  - **7b 点名的连库 handler 用例由主会话在自己的克隆库上跑了**（7b 的树迁移文件与 5d 冻结版逐字节相同，用模板 `f3_budget_s5v_tmpl`）：`TestClaimTaskByRuntime_BuiltinSkillsFollowOnboardingScope`、`TestResolveTaskSkillBundles_OnboardingRequiresTheSessionsOwnAgent`、`TestWriteMeasuredJSONByteIdenticalToWriteJSON`，go1.27.0 与 go1.26.9 都是 3/3（含子用例 31/31），克隆已删（`/tmp/f3-main-s5v/7b-skill-hdl-{d,g}.txt`）。
  - **7b 树上的第二轮基线**（`/tmp/f3-main-s7k/base-r2-counts/summary.txt`，全程带连不上的地址）：`gofmt` 两个版本 0 行；`go build ./...`、`go vet ./cmd/multica/...` 两条工具链退出 0；`TestIssueBudget` 36/875（顶层/含子用例；第一轮是 34/824），两条工具链与 `-race` 相同；守卫下整包 `./cmd/multica/...` 526/2161；样例目录检查器退出 0；Core：vitest 92 条通过、typecheck 与 lint 退出 0；service 包点名的五条技能用例 5/27，没有跳过。7b 的产品文件自 03:48 起没再变；它最后一小轮结束后重跑 `freeze-7b.sh`，摘要有变就重量。
  - 7kb 简报剩下的四个占位符由 `/tmp/f3-main-review/fill-7kb-b.py` 一次填（文件清单、摘要路径、报告章节对照、基线数字），已空跑通过，等 7b 收尾后 `--apply`。
  - 06:25 车道：7b 在把验证一节并进报告（它的产品工作与验证早已做完，这一小时都在核对报告措辞；因为已接近收尾，没有打断）；5c 在做阶段 B（扫描器那一步）；6kd2 在跑第 7 组变异，报告 28 KB。

### 2026-11-20（续）

- **06:30–09:10 7b 收口并冻结；R6、R7 裁定；7kb 简报填完；6kd1 派出**
  - **R6 裁定：钉住。** 技能文档里“`multica issue budget` 只给人用”那一句由用例钉住：`server/internal/service/builtin_skills_test.go` 的 `TestWorkingOnIssuesSkillCoversIssueLoopContracts` 的 `mustContain` 多一行（7b 的文件清单因此多一个文件）。车道给了去掉那句话时变红、放回后变绿的两次运行。
  - **R7 裁定：按它的提议。** source map 新增的那一行现在引用任务目录里的 `design.md`（共享检出里未跟踪、worktree 里没有）；7a 合入时由主会话改成指向 handler 与它的用例。
  - **7b 又空转了约三小时**：产品工作 03:48 就结束了，之后一直在核对报告措辞。07:50 前后主会话把它停下，发了一条完整的收尾清单（八项，至多 15 次工具调用，并说明“这条消息就是全部”，因为它收件箱里还压着两条旧消息）。它用五次工具调用在 08:14 收尾，报告 177,342 字节、1,602 行，末行 `STATUS: complete`。
  - **主会话对最终的 7b 树做的核对**：相对上一次冻结只多那一行；车道的 `skill-tests.sh` 改成带连不上的地址、只跑点名的五条；`freeze-7b.sh` 的清单加上那个测试文件后重跑，**调用端这一半是 17 个文件加 26 个样例，共 43 个**，样例 26/26 与冻结摘要一致；基线重量一遍，与上一条记录的数字相同；其余 76 个脏路径与派发时的状态逐行相同（8 个 `M`、68 个 `??`），修改时间都早于车道开工，所以车道只动了自己的文件。
  - 报告里留着三处已过时的句子（车道自己说明了）：开头一段（第 6 行）还说 R5 待裁定；第 6 节有两行说“第 27 种读法”和成员检查的函数体没有用例（第二轮起都有了）。7kb 的简报会点明。`pnpm knip`（CI 里的一项）没有跑，合入时补。
  - 7kb 简报：`fill-7kb-b.py --apply` 之后 761 行、没有占位符；已接受事项加了第 15 条（R6）与第 16 条（R7）。派发前还欠两处修改：第 I 节仍写着 `./cmd/multica/...` “不带 `DATABASE_URL`” 跑（要改成带连不上的地址，基线就是这么量的）；补一句上面三处过时句子的说明。
  - **6kd1 在 08:20 派出**（第 6 步 daemon 半边复核的第一部分，冻结的 6b 树，摘要 21 个文件全部一致）。
  - 监视器：06:47、07:59 到期无边界；08:18 报 `7b2: STATUS: complete`；09:16 到期无边界。
- **API 520 两轮中断；监视器补上“车道停下”的检测**
  - 第一轮：5c 07:33、6kd2 07:37、7b 07:48 的回合被 HTTP 520 结束。约 35 分钟后主会话发消息恢复了 5c 与 6kd2。
  - 第二轮：6kd1 08:38（刚读完简报）、5c 08:48、6kd2 08:49。**直到 10:35–10:40 才恢复，三条车道各空了约 1 小时 50 分。** 原因：监视器只找报告里的 `STATUS` 行，看不见这种停法；车道在 ListAgents 里只显示 idle，转录最后一行是带 `isApiErrorMessage` 的 assistant 行。
  - 修法：`/tmp/f3-main-s5d/watch-lanes4.sh`（加 `lane-stopped.py`）。除原来的边界外，被监视车道的转录满足下面任一条就退出并说明：最后一条 assistant/user 行是 API 错误行，或是一个已结束的回合（`stop_reason` 不是 `tool_use`），且 150 秒没有新写入；或者 45 分钟没有任何写入。自测：已结束的 7b 转录判为“停下”，三条在跑的车道判为“在跑”。12:39 起后台运行（`5c1 6kd2 6kd12`）。
  - **更正**：上面 04:45–06:00 那条说 5c 的脚本在变量未设的情况下跑了三次，实际是**六次**（它自己的清点 `/tmp/f3-s5c/runs-audit.out`）：`./internal/migrations/ -run TestIssueBudget` 一次、L13 经 `gotest.sh` 两次、L13 经 `treemut.sh` 三次（查询文件变异 a-t01..a-t03，在临时拷贝里）。六次都点名了不开数据库的用例，这两个包也没有 `TestMain`，没有任何一次碰到回落地址。它的两个脚本的 `-` 模式已改成设连不上的地址。
- **10:40 阶段 B 的五条裁定（随恢复消息发给 5c，让扫描那一步只建一次；提交前可推翻；已写进 `design.md` 10.3 的同日补充）**
  - 起因：5c 的笔记说它打算按设计的字面建查找语句（不连根行、没有 LIMIT、没有每个根的时限），再在报告里要裁定：`off` 的根与账目读不出的根每一轮都会被找到，每一轮处理的根数没有上限。
  - **B-1 自己的循环，不进 30 秒的存活检测那一轮。** `runPeriodicSweep` 在一轮没跑完时丢掉后面的 tick；现有代码正因为这一点把逐行开事务的工作挪出了那一轮（`runtimeGCSweepInterval` 的注释，`runRuntimeGCSweeper` 与 `runSourceContextSweeper`）。对账每个根要一把根锁和一个事务，不能让它拖慢离线判定与任务回收。所以：一个与 GC 循环同类的函数，同样 30 秒一轮，在 `main.go` 里挨着 GC 循环启动；`runRuntimeSweeper` 的阶段清单不动。它没有观察者，失败不进 `taskObservation.record`；阶段指标照旧（`RuntimeSweepStageBudgetLifecycle`）。设计原来那句“现有的周期性 sweeper 里加一步”由主会话改。
  - **B-2 `off` 的根**：查找语句连上根行，把 `mode = 'off'` 的排除掉（普通读，不取锁）。根锁下的那次检查保留，因为两者之间开关可能变。
  - **B-3 每轮有界**：查找语句按根 id 排序并带 LIMIT（循环旁的具名常量，500，与相邻的批量常量同值）；每个根的对账有自己的 5 秒时限（与 `runtimeGCOperationTimeout` 同值），查找语句同样；某个根失败或超时记一条带根 id 的 WARN 后继续下一个；根与根之间遇到取消就停。本包不做游标，也不给整轮设时间预算。
  - **B-4 账目读不出的根**：按阶段 A 的做法（每次访问一条 ERROR、什么都不改、下一轮再被找到）。有了 B-3，每轮至多 LIMIT 条这样的日志与重放。写进报告的已知限制：这样的根在修好之前每轮都要在根锁下完整重放一次（量出一次的代价）；排在前面的不可推进的根达到 LIMIT 时，后面的根要等它们修好。期间不丢东西：尝试保持 open，open 尝试的预留与它的 hold 一样计入占用。
  - **B-5 两处“已结束”必须是同一个集合**：查找语句的条件与 `issueBudgetAttemptEndOf` 的判断（三个终态，或任务行没了）。两者一旦不同，某个根每轮都被找到却什么都关不掉。要它说明靠什么钉在一起；没有就加一条 L13 式的守卫（读查询文件、不开数据库）。
  - 查询计划按裁定后的形状（连根行、排序、LIMIT）在它的种子库上重量，有无 881 索引各一次。
  - **12:40 它的笔记里的进度**（主会话尚未核对）：查找语句与 sqlc 生成；`ReconcileLifecycles(ctx, maxRoots, each)` 与 `reconcileLifecycleRoots`；指标常量；`cmd/server/budget_lifecycle_sweeper.go` 与 `main.go` 的一行；service 用例 R01–R07（`issue_budget_lifecycle_round_test.go`）；`cmd/server` 用例 W01–W05（`budget_lifecycle_sweeper_test.go`，其中一条不连库的用例钉住“不在存活检测那一轮里跑”）；指标用例一条。计划：有索引 6.5 ms / 2,469 个缓冲页，无索引 108 ms / 9,790。还剩：变异、882 编号下的迁移用例、两项测量、报告。
  - 迁移：阶段 B 新增 `881_issue_budget_attempt_open`（部分索引 `issue_budget_attempt_open_idx`，`WHERE state = 'open'`），就绪检查挪到 `882_issue_budget_ready`。**F3 的迁移现在是 866..882 共 17 组**，第 8 步改号后是 869..885（`renumber.py` 自己发现文件组）。主会话的 `verify-5c.sh` 从 worktree 的迁移自建模板，不受影响。
- **平台行为：子代理压缩上下文之后，会被附上主会话读过的文件**
  - 证据（转录里的 `attachment` 行，`/tmp/f3-main-s5d/attached-files.py` 与 `leak-check.py`）：5c 在 08:31 与 11:38、6kd2 在 10:59 各自压缩之后，被附上了三份它们从没打开过的文件的**全文**：`/tmp/f3-main-review/round2-5c-draft.md`（主会话给 5c 第二轮的草稿）、`/tmp/f3-main-review/fill-7kb-b.py`、`/tmp/f3-main-s5v/verify-5c.sh`；另有 `implement.md` 的一条只有路径、没有内容的引用。这三份正是主会话当时最近用 Read 工具读过的文件。两条车道之后都没有去读 `implement.md`（转录核过）。
  - 两条车道都自己发现并写了下来：5c 的笔记“披露 3”（没有据此行动，仍只做第一轮）；6kd2 的笔记第 266 行（不是它的、没有用，报告第 9 节会有一行）。
  - 影响：5c 提前看到了第二轮要补的四条（本来就是要发给它的指示）；6kd2 看到的是第 5 步的材料与两个脚本，与它复核的 daemon 半边无关，它的发现的独立性不受影响。02:15 之前那次“5c 读到主会话的原因表”不是这个原因（那次是它自己用 `cat` 读的，转录里没有对应的附件行）。
  - **从现在起的做法**：只给主会话看的材料（`/tmp/f3-main-review/` 下的草稿、裁定前的清点、复核方不该先看到的结论），主会话一律用 shell 命令读写，不用 Read/Write/Edit 工具，直到不该看到它的车道全部结束。复核方的简报加一句：平台附上了你没打开过的文件时，不要用，并在报告里说明。
- **13:00–18:00 阶段 B 预审通过；API 第三轮中断；监视器三次到期都没等到边界**
  - **阶段 B 预审（主会话通读了产品代码与测试，第一轮不提修改）**。文件与 14:30 的 SHA-256 前 20 位：`cmd/server/budget_lifecycle_sweeper.go`（新，72 行，`75945dad5632a3b49afb`）；`main.go` 第 770–774 行在 GC 循环与 source-context 循环之间启动它，`runtime_sweeper.go` 未动；`pkg/db/queries/issue_budget.sql` 第 341 行的 `ListIssueBudgetRootsWithEndedAttempts`（`1085bd8cf15e37645fa8`）；`internal/service/issue_budget_lifecycle.go` 追加 `ReconcileLifecycles` 与 `reconcileLifecycleRoots`（384 行，`cf455bab818139e5b863`，阶段 A 的代码没有改）；`internal/metrics/business.go` 的阶段名 `budget_lifecycle`；迁移 `881_issue_budget_attempt_open`（部分索引），就绪检查挪到 882。测试：服务层 R01–R07（`issue_budget_lifecycle_round_test.go`，`86066bc616fcf2777419`），`cmd/server` 的 W01–W05（`budget_lifecycle_sweeper_test.go`，`c3728d5ebd868124c0ce`），指标一条，迁移一条。五条裁定逐条对上：独立循环、查找语句排除 `off`、按根 id 排序加 LIMIT 500、每个根与查找语句各 5 秒、两个“已结束”集合由 R02/R03 钉住。
  - **预审留给第一轮验收的四个观察**：
    1. W04、W05 跑的是覆盖全库的一整轮，`requireNoForeignWork` 在别的工作区有“已结束而未关”的尝试时直接失败（“run this package on a database of its own”），不去写不属于自己的行。根只在显式激活时建（`issue_budget.go` 第 500 行是唯一插入点），普通测试不留预算行；风险只在预算测试自己的清理。读代码的结论是各自都清（服务层夹具按工作区清六张表；handler 的 `newClaimBudgetScene` 按工作区清；绑定与并发测试按根 id 清；`testutil.Fixture.Insert` 的行随清理删除），**还没有用一次运行证明**；验收时在整包跑完之后、删库之前直接执行查找语句数一遍。
    2. R06 依赖时间（每根 2 秒，上界 20 秒），在这台模拟 amd64 的共享库上要在负载下看一眼。
    3. W03 从磁盘读源文件，变异走 `-overlay` 时测试文件自己的路径也要进 overlay。
    4. 整轮没有总时限（裁定 B-3 如此）：500 个被占住的根 × 5 秒约 42 分钟，只推迟本循环自己的下一轮；日志最坏每 30 秒 500 行。两条都已让 5c 写进已知限制。
  - 5c 自报（未经主会话验证）：阶段 B 变异 51 个全杀（无库 9、服务层 21、`cmd/server` 15、迁移 5、另 1）；迁移测试在 882 编号下 10 通过 0 失败 0 跳过；查找语句两种写法的计划数字在报告里（建成的连接加 DISTINCT：有索引 6.5 ms，无索引 108.3 ms）。
  - **API 第三轮中断**：5c 在 16:04 连续九次请求无应答后以 520 结束回合，16:19 才从队友的空闲通知看到（当时没有监视器在跑），16:21 发消息恢复，指向它笔记的“After compaction 7”，并追加上面第 4 条的两个已知限制。整个下午 API 都慢，单次请求 3–8 分钟，6kd2 有一次 55 分钟的空档之后自己继续了，所以“长时间不写”不等于停下，要用 `req-gaps.py` 看。
  - **监视器**：v4 在 13:34、15:22、17:22 三次到期，都没有等到边界；17:22 到 17:56 之间没有监视器（到期通知晚到），三条车道这段时间都在写报告，没有损失。17:56 起 `5c1 6kd2 6kd12` 重新挂上。教训不变：到期当下就重挂。
  - **7kb 简报定稿（793 行，没有占位符，等空槽派出）**：第一节写明每条 `go test`（含变异）都带无库地址；第 16 项注明 source-map 那一行在 7a 合入前引用的是未入库的 `design.md`；第 17 项列出车道报告里三句过时的话，不算发现；第 18 项记 `pnpm knip` 车道没有跑。主会话在 7b 树里试跑了 `./node_modules/.bin/knip`（输出 `/tmp/f3-main-s7k/knip-7b.txt`，792 行）：退出码 1，原因是这棵树只做了部分安装，读不了 `playwright.config.ts` 和几个 vitest/electron 配置，报出的未用文件与 734 条未声明依赖都与本任务无关，没有一条匹配 `issue-budget` 或 `IssueBudget`，`git status` 前后一致。**有意义的 knip 要完整安装，留到 7b 合并时跑**；复核方不跑。
  - **5c 第二轮的草稿**挪到了只走 shell 的路径（旧路径只剩一行占位），内容待第一轮验收后补齐：阶段 B 的结论、把“三次”更正为六次、第一轮验收看到的问题，以及一句“你通过附件见过这份草稿，以发出的消息为准”。
  - **`verify-5c.sh` 还缺两步**，第一轮验收前补上：`./cmd/server/ -run 'TestBudgetLifecycle'`（两条 Go 线加 race）和不碰库的 `./internal/metrics/`。
  - 记忆新增两条（跨会话有用）：子代理压缩后会被附上主会话 Read 过的文件；API 5xx 会无声结束子代理的回合。
- **18:00–18:35 观察 1 的运行证明；5c 第一轮验证里一处与本任务无关的失败**
  - **W04/W05 同库检查（主会话自己的克隆库，18:30，go1.27.0，`/tmp/f3-main-s5v/run-shared.sh`，输出 `pre-shared-*.txt`）**：从工作树迁移到 882 的模板克隆一个库，不删库连跑四段：`internal/service -run TestIssueBudget`（121 通过）→ handler 的预算测试（38 通过）→ `cmd/server -run TestBudgetLifecycle`（5 通过）→ 同一条再跑一遍（5 通过）。每段之后用查找语句去掉 LIMIT 数一遍（`probe-ended.sql`）：已结束而未关的根 0，预算根 0，尝试 0。结论：聚焦的预算测试清得干净，W04/W05 在它们之后同库能过，自己重跑也能过。**整包**（service、handler、`cmd/server` 全部测试）跑完是否留行，由完整验收时每步删库前的同一个探针回答。`verify-5c.sh` 已补上缺的两步，并接进了探针和这段同库序列。
  - 5c 的阶段 B 报告已全部读完（成本、五条裁定逐条落实、13 条测试、51 个变异、迁移、运行、十条已知限制、退出行对照表），与预审一致，第一轮不提修改。它新量出的两件事进了已知限制：一个账本装不下缓存或加载不了的根，重放超过 5 秒就永远不会被循环对账（实测 4 万条最慢 2.65 秒）；每个等锁超时的根会让 pgx 关掉一条连接、连接池再开一条。前一条 5c 列为“需要裁定”，主会话的意见：首包只观察，尝试不关只影响尝试状态与原因计数，预留照样算占用，**作为已知限制接受，记给 F3-S2**（强制之前要么把每根时限做成随账本大小变，要么让重放可以续做）。
  - **5c 自己的第一轮验证**（`/tmp/f3-s5c/verify-r1.summary`，18:20 跑完）：九步里八步退出码 0（预算选择 187/0/1 顶层、两条 Go 线各带 race；service 整包 1734/0/18，那条基线里会抖的协作测试这次过了）。一步失败：Go 1.26.9 那次把 `internal/migrations`、`cmd/migrate`、`internal/metrics` 放在**同一条 `go test` 命令**里，`cmd/migrate` 的 `TestCollaborationStageMigrationsRunnerUpgradeRefusalRollbackAndReupgrade` 在应用 `819_collaboration_stage_ready` 时报 `could not open relation with OID …`（SQLSTATE XX000）；同样的命令在 go1.27.0 下通过。两个迁移测试包并行跑在一个克隆库上，一个在建删索引，另一个的就绪检查正在对索引目录调用 `pg_get_indexdef`，这是包间并发撞出来的，819 与那条测试都不是本任务改的。5c 自己已这样判断并在逐包重跑取证；主会话的验收本来就是每包一库。这条归到“任务外发现”：就绪检查这一类迁移（819，以及照同一写法的 882）在检查期间遇到别的会话删表删索引会报内部错误，生产迁移单独执行时不会遇到。
  - **两条“需要裁定”的裁定（18:45，已写进 `design.md` 10.3 补充的最后一条；提交前可推翻）**：① 每根 5 秒对整本重放的账本：保持建成的样子，列为首包已知限制，记给 F3-S2 与账本快照一起定（候选：只给等锁设上限的 `lock_timeout`，它还会连带消掉“超时关一条连接”；或让时限随账本大小变）。② 跳过与失败的根数不进指标：首包不加。`design.md` 第 190 行补了编号说明（校验迁移 882，共 17 条 866–882，第 8 步改到 869–885）。
  - **平台行为的更正（18:50 用三个探针文件验证）**：不只是 Read 工具，**单独一条 `cat 文件` 的 Bash 命令也会被登记为主会话“读过”**（之后改动该文件，平台会回一条“changed on disk since you last read it”）；放在复合命令里的 `cat`、`sed -n`、Python 读取不登记。主会话 18:26 正是这样看过给 5c 的第二轮草稿，发现后立即挪到新路径、旧路径放占位并 `cat` 了一次占位。核对三条车道的转录：新路径没有被任何车道附上；旧路径自 16:28 起附出去的都是占位（5c 18:09、6kd2 16:28 与 18:25、6kd1 18:06）。**补记一条此前漏掉的**：6kd1 在 12:55 的压缩之后也收到过旧草稿的全文（之前记的是“6kd1 没有收到”，那是 12:40 查的），内容是第 5 步的，与它复核的 daemon 半边无关。5c 在 18:09 还被附上了主会话压缩后恢复的两条记忆笔记（内容无害，但说明清单就是主会话的已读文件状态）。做法改为：不给车道看的材料，不用 Read/Write/Edit，也不用单独的 `cat`；看用 Python 一行或 `sed -n`，写用 heredoc。记忆笔记已更正。
- **18:50–19:25 主会话的第一轮完整验收（`r1c`）在冻结副本上开跑；聚焦部分的结果**
  - 监视器 `b4f3k20t9` 18:52 到期（没有边界），一分钟内重挂 `bcc857ieu`（`5c1 6kd2 6kd12`，到 19:48）。
  - **5c 对 v6c 那处失败的证明**（它的笔记）：在不含本任务改动的导出（`git archive 86fea651c server`）上，两条 Go 线都能用“校验期间另一会话删掉一个关系”复现 819 的失败；对本任务的 882 做同样的持锁删除，5/0/0 通过，且没有后端在等，882 不受影响。归入任务外发现。
  - **不等状态行先验收**：代码哈希自 14:30 未变，5c 余下的只是写报告。19:10:31 快照 `/tmp/f3-main-snap/f3-20261120-1912-5c-round1`（90 个路径），物化为 `/Volumes/artisan/code/2026/mc-f3-verify-5c1`（状态一致，870 条迁移，末条 882）；19:11:49 起跑 `verify-5c.sh … r1c`（汇总 `/tmp/f3-main-s5v/r1c-summary.txt`）。到边界时比对实时工作树与快照，一致则 `r1c` 就是边界验收，不一致只补验变了的部分。
  - **聚焦部分 19:22 跑完**（每步一个克隆库，一条命令一个包）：
    - 不碰库：两条 Go 线都是 gofmt 0 行/74 个文件、build 0、vet 0、`internal/budget` 112/0/0、`internal/metrics` 50/0/0。
    - `internal/service -run TestIssueBudget` 121/0/0（两条线，加 race 再两条线）；handler 预算测试 38/0/0（同样四次）；`internal/migrations` 整包 104/0/0（两条线）；`cmd/server -run TestBudgetLifecycle` 5/0/0（两条线，加 race 再两条线）。每步之后探针都是 0 个根、0 个尝试。
    - 同库连跑（service → handler → cmd/server → cmd/server 再一次）：121、38、5、5，全程没有残留行，与 18:30 那次一致。
    - `cmd/migrate` 整包：go1.26.9 70/0/1（基线 67/0/1，多出的 3 条是本任务的 `issue_budget_test.go`）；**go1.27.0 69/1/1**，失败的是 `TestCollaborationCommandHistoryMigrationRepairsInterruptedIndex`（`migrate_collaboration_command_history_test.go:61: index … validity = true, want false`）。
  - **这处失败与本任务无关**（主会话读测试源码后的判断，还没有用重复运行量化）：测试只跑 736 一条迁移；本任务对 `cmd/migrate/main.go` 的改动只是往 `concurrentIndexCleanups` 里加了 15 个键（867–881）。测试的做法是取消构建的上下文，随即回滚挡路的写事务，再断言索引无效；pgx 的取消请求是异步发出的，回滚若先到，服务端那条后端拿到锁就把索引建完了（表里只有一行），于是读到“有效”。这是测试自身的竞争，在这台模拟 amd64 的容器上更容易露出来。归入任务外发现；`r1c` 跑完之后在同一棵树和原始导出上各重复若干次，量一下频率。
  - **整包（go1.27.0，各用自己的克隆库，19:42 全部跑完）**：`./cmd/server/...` 482/0/9（基线 477/0/9，多出的 5 条是阶段 B 的循环测试）；`./internal/service/...` 1734/0/18（基线 1707/1/18；多出 26 条，基线里那条会抖的协作测试这次通过，与 5c 自己的整包计数相同）；`./internal/handler/...` 3040/0/53（与基线相同，这一轮没有新增 handler 测试）。三次之后探针都是 0 个根、0 个尝试：**整包也不留预算行**，W04/W05 的整轮守卫在仓库入口 `scripts/test-go.sh`（`-p 1`，同一个库，`cmd/*` 在前）下不会被别的包的残留触发。
  - **那条协作迁移测试的频率（19:46，`/tmp/f3-main-s5v/flake-736.sh 40`，输出 `flake-736-summary.txt`）**：冻结副本 go1.27.0 40/0、go1.26.9 40/0；不含本任务改动的导出 go1.27.0 40/0、**go1.26.9 39/1**，失败的是同一行同一句话。加上 `r1c` 的两次整包运行，共 162 次里 2 次，其中一次在没有本任务改动的树上：是任务之前就有的抖动，归入任务外发现（原因是主会话读源码的判断，未证明）。
  - **`r1c` 的结论**：第一轮的树在两条 Go 线上构建、vet、gofmt 干净；预算相关的选择、整包 `internal/migrations`、三个整包都通过；唯一的失败与本任务无关。监视器 `bcc857ieu` 19:49 到期（无边界），`b66bzdaj0` 接上（到 20:45 左右）。给 5c 的第二轮消息已把验收结果填好（不再有占位符），并补一句“第 8 节第 3 项那条两个包的命令改为一条一个包或加 `-p 1`”。等 5c 的状态行，到时比对实时工作树与 19:10 快照。
  - 第 8 步备忘：`concurrentIndexCleanups` 的键带迁移编号，重编号时要一起改。已在冻结副本上空跑 `renumber.py … 869`（`/tmp/f3-main-s8/dryrun-r1c.txt`）：17 个文件名段 866..882 → 869..885，改 5 个文件 57 行（含 `cmd/migrate/main.go` 的 15 行），重命名 34 个文件。脚本改不到的只有一处单独出现的编号：`pkg/db/queries/issue_budget.sql:351` 的注释“…`issue_budget_attempt_open_idx` (881)”及其生成文件，第 8 步把括号里的编号删掉（名字足够），再跑一次 `sqlc generate`。`internal/budget/hold_test.go:666` 里的 869 是刻度数，不是编号（`/tmp/f3-main-s8/bare-numbers.py` 的输出）。
- **19:50–20:40 6a 第一轮改为与 5c 第二轮并行（主会话的决定，提交前可推翻）；6kd2 的发现已预读**
  - **为什么并行**：剩下的关键路径是 5c 第二轮 → 第 5 步复核 → 6a 两轮 → 6a 复核 → 7a 两轮 → 7a 复核 → 第 8 步。6a 第一轮（严格解码、证据表、价格来源、归一化）全是纯代码，只新增 `internal/service` 下的文件，测试不碰库，依赖的只有 11-18 起没动过的 `internal/budget` 和基线里的 `internal/metrics/pricing.go`，与 5c 第二轮（`off` 开关、quick-create 差异观察）没有交集。等 5c 第二轮做完再派，白等一整轮。
  - **做法**：从 19:10 的快照物化出独立工作树 `/Volumes/artisan/code/2026/mc-f3-budget-6a-20261120`（20:15 再核一次，实时工作树与快照 110 个路径逐文件一致），只另外拷入 6b 的两个夹具文件（SHA-256 与简报一致），起点快照 `/tmp/f3-main-snap/f3-20261120-6a-start`。并行版简报 `/tmp/f3-main-s6a/brief-6a-par.md`（869 行，由 `mk-par-brief.py` 从原简报按“恰好一次”的锚点生成，原简报未动）：第一轮**只新增文件，不改任何现有文件**，需要改现有文件就写进“需要裁定”；6b 的代码只读、不在它的树里；每条 `go test` 一个包；整包加 `-timeout`；附件规则；三处已证明与本任务无关的抖动（协作阶段指标测试、736 索引修复测试、819 校验遇并发删表）遇到就重跑一次、照实报告、不查原因。第二轮在哪棵树上做，等两轮之间的评审时再定。合并时把它新增的文件拷进主工作树即可。
  - **6a 的基线（就是 `r1c` 的数字，另补量三项）**：`-run 'Budget'` 按包分开，service 145/0/0（255/0/0）、handler 42/0/1（68/0/1），两条 Go 线相同（`/tmp/f3-main-s6a/base-run-summary.txt`）；`internal/middleware` 整包 73/0/10（126/0/10）。跳过的测试名单写在 `/tmp/f3-main-s6a/base/skips-*.txt`，service、handler、cmd/server 三份与 5c 基线的名单逐名相同。
  - **空槽的顺序**：6a 第一轮 → 7kb → 6b 的修复轮。6kd2 一报完成就派 6a。
  - **6kd2 的发现一节已读（等状态行后正式裁定）**：没有 blocker；3 条 major（KD2-12 `begin` 的锁没有测试、KD2-01 30 秒上限没有测试、KD2-02 服务器地址规则的测试放过两个变异）都是代码对、测试缺，各带补测；8 条 minor 里 7 条同样是补测，KD2-06（F7a：过了 7 天上限的批次在“轮询先到”的次序下一次都不发就丢）是一句话与代码不符，可选 6 行修复或改措辞；KD2-10 是规则空白（记录不带账号，同一 profile 换账号后旧批次以 404 丢失）；6 条 note 不改。主会话的倾向：补测全部并入车道文件；KD2-06 取那 6 行修复；KD2-10 首包记为已知限制并作为产品问题交给用户；KD2-11（车道一条测试 200 次里失败 13 次）按复核者给的改法修。


### 2026-11-20 Codex 并行续接（用户要求推进至归档）

用户在 Codex 会话要求检查 F3 并继续至归档。Codex 发现原 Claude 会话仍在推进，因此保留其所有 live worktree，另建 `mc-f3-completion-20261120` / `codex/f3-completion-20261120`，从已冻结的 5c r1c 树接入。已在这棵树实现 6a 服务端用量入口、7a 人工管理 API，并集成冻结的 6b/7b；5c C/D 正在本树补齐。调用端独立复核发现 Core 的预算错误文本日志泄露，已加 7 条红绿回归并修复，221 条 Core/API 用例通过。当前仅在 `f3_budget_cx_*` 自有克隆库串行测试；原树和原报告未改。具体分工、证据与联络请读 [codex-coordination-20261120.md](research/codex-coordination-20261120.md)。原会话请勿重复派发 6a/7a 或提前归档；可在该文件追加交接。最终需统一验证和按路径提交后再归档。


### 2026-11-20 最终接管与本地集成完成

- 所有步骤1–8已完成；先前未勾选项按最终代码与实际证据补齐。原始失败和未完成的Claude复核记录保留。
- 功能提交584846f95，集成提交8126073f6；共享本地分支已快进到完全相同的已验证源码，7916哈希无差异，13200项无关工作副本文件未变。
- 合并后normal/race共22个包运行通过，571个预期顶层名称、142个映射名称、91个既有名称逐项PASS；Core5355、Views7610全量通过；build/vet/lint/typecheck/knip通过。
- 规范补齐部署/回退与两条只读探针；内置skill/source map已同步。最终记录与证据入口：[codex-final-delivery-20261120.md](research/codex-final-delivery-20261120.md)。
- 仅归档本子任务；父roadmap保留in_progress，F3-S2/F4/F5和M1剩余切片开放。
