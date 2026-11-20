# 执行计划：security-remediation v2

需求见 `prd.md`（本目录）与父任务 `prd.md`；方案见 `design.md`，下文的“第 N 节”都指它。本文件只写顺序、命令、风险点和回滚点。

钉点图（父任务 `research/v1-pin-source-map.md`）有 129 KB，超过单文件注入上限 65536 字节，所以没有放进本任务的 `implement.jsonl` 与 `check.jsonl`：放进去会被截断，还会挤掉其他条目。本次改动要用的钉点已经抄进 `design.md` 第 9.5 节和第 10 节；需要原始依据时按节直接读那个文件（第 1.1 节是 Go 测试，第 3 节是前端、文档与端到端，第 4 节是生成文档，第 6 节是升版本的语义）。

## 启动前检查

`task.py start` 之前逐项确认，任何一项不成立就停下来回到规划：

1. 用户已明确批准最新的规划摘要，并确认或改写了 K5、K6、K7（父任务 `prd.md` 的 Decisions）。
2. 用户确认了工作位置。建议：从当前分支 `codex/cross-workspace-collaboration` 的 HEAD 手工开一个 worktree 和新分支，在里面实现。`main` 不含 v1 模板，不能作基线。不要用会自动清理的隔离 worktree。当前检出里有其他会话的大量未提交改动，直接在里面改容易互相覆盖。
3. 重新核对锚点。`design.md` 和钉点图里的行号是 2026-11-18 的工作区行号：
   - `git log -1 --format=%H`，与钉点图开头记录的 `ccd28cbc0` 比较；有新提交时用 `git diff --stat ccd28cbc0..HEAD -- <第 1、9、10 节列出的文件>` 看哪些动过。
   - `git status --porcelain -- <同一批文件>` 应为空。不为空说明别的会话正在改，先弄清是谁。
4. 读 `apps/docs/content/docs/developers/conventions.zh.mdx`（改文案和中文文档之前必读）。
5. 读第 9.3 节表里带 * 的五个测试的测试体，按结果更新那张表。
6. 确认数据库测试能真正运行：按 `.trellis/spec/server/verification-environment.md` 与 `.trellis/spec/server/database-test-isolation.md` 准备好独立的测试库，跑一个已有的处理器测试，在 `-v` 输出里看到 `PASS` 而不是 `SKIP`。

## 工作环境

2026-11-18 用户批准后建立，六项启动前检查均已通过。

- 产品文件只在这个 worktree 里改：`/Volumes/artisan/code/2026/mc-secrem-v2-20261118`，分支 `codex/security-remediation-v2-20261118`，基线 `ccd28cbc0`。主检出里只改本任务目录下的规划文件。
- 任务私有目录 `/private/tmp/secrem-v2-20261118`：`env/` 放各测试库的连接配置（不进仓库，不打印），`logs/` 放测试输出。
- 跑 Go 测试一律用 `bash /private/tmp/secrem-v2-20261118/gotest.sh <库标识|nodb> <go test 参数>`。它核对库的身份，套上智能体 CLI 防护包装，把输出写进日志，并报告真实退出码与 `RUN/PASS/FAIL/SKIP` 计数。`nodb` 用于不需要数据库的服务层契约测试。
- 测试库由协调者从迁移到 865 的空模板克隆，每个依赖数据库的包一个，实现者不建库、不迁移、不删库。已分配：`handler_a`、`cmdserver_a`（正文与版本号那一步）；`handler_b`、`cmdserver_b`（路由与通知测试）；`handler_c`（协调者自己写的 g、l 两项）；`handler_d`（四个场景）。新增用 `bash /private/tmp/secrem-v2-20261118/setup-db.sh <标识>`。最终验证另开新库。收尾时全部删除。到 2026-11-20 一共建了 32 个，名字都以 `secrem_v2_1118_` 开头（`bash /private/tmp/secrem-v2-20261118/list-dbs.sh` 列出）：上面 6 个，端到端的 `e2e`，七轮最终验证的 20 个（`final_*` 到 `final7_*`），基线预检的 5 个 `tip_*`（已迁移到 868）。
- 基线证据：`handler_a` 上现有的路由、能力、小队模板测试 91 个用例全部运行并通过，无跳过（`logs/handler_a-20261118-140609-70802.log`）。
- 整包运行处理器或 `cmd/server` 时，在命令前加 `TMPDIR=/private/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/`。这台机器默认的 `TMPDIR` 经过符号链接，产物相关的测试会因此失败（报 `artifact adapter unavailable` 或 503），与本任务的改动无关。

## 实现顺序

每一步结束时跑它后面列的命令。步骤之间不合并提交。

### 步骤 1 先写会失败的契约测试

- 文件：`server/internal/service/builtin_security_remediation_templates_test.go`。
- 按第 9.2 节加：新增契约组、否定断言、共用小节逐字一致、v1 分诊块是前缀、`code` 小节的顺序、队长独有规则。版本断言先不动。
- v1 的五组标记和父任务状态测试一个字不改。
- 预期：新增断言失败，原有断言通过。把失败输出留作“先红”的证据。

### 步骤 2 写两份正文

- 文件：第 1 节“指令正文”一行的两个文件。
- 先写四个共用小节（第 2 节），在一份里定稿后原样复制到另一份；再写各自独有的部分。
- 逐条对照第 3 到第 8 节。删改 v1 句子时对照第 8.1 节的表和“要保住的 v1 标记”。
- 写完自查：
  - “只有任务实际分配给本小队，且注入的 Squad Operating Protocol 授予父任务状态管理权时”在每份里恰好一次。
  - 没有 `{{`。
  - 队长角色指令的四个必备标题都在。
  - 没有残留的 v1 回执与重试表述。
- 预期：步骤 1 的断言全部通过。

### 步骤 3 升版本号

- `server/internal/service/builtin_squad_templates.go:312` 与 `builtin_agent_templates_roster.go:375`：1 → 2。
- 同时改第 9.5 节表里的全部版本钉点：三个 Go 位置，加 `e2e/security-remediation-squad.spec.ts:98` 的 `template_version: 2`。版本号和它的钉点放在同一个提交里。
- 目录描述（`builtin_squad_templates.go:350-353`）在步骤 5 与文案一起改。

### 步骤 4 处理器层测试

- 新文件 `server/internal/handler/squad_security_remediation_routing_test.go`：第 9.3 节的 a、b、d；在 `squad_comment_trigger_test.go:133-218` 的表里加 g 的一行；在 `subscriber_test.go` 的 `TestSubscriberAPI` 里加 l 的子测试。
- 新文件 `server/internal/handler/squad_security_remediation_flow_test.go`：第 9.4 节的四个场景。
- 第 9.3 节的 k：受限令牌走真实的发评论路径，断言已订阅成员的收件箱；优先放在 `server/cmd/server`。
- 用 `dbfx` 夹具和 `testutil.Call`；不手写 `INSERT ... RETURNING id` 加清理，不手写录制器、状态检查、解码的样板。场景的断言写在测试文件里。
- 任何一条与设计不符：停，改 `design.md` 和正文，再继续。不改测试去迁就（第 9.3 节末尾写了两处最可能的调整）。

2026-11-19 进度与实现中的两项发现：

- a、b、d、g、k、l 六项行为都有测试钉住并通过，订阅步骤和“回复里 @小队”这种放行方式都保留。四个场景（第 9.4 节）已写完并通过，结果和没有覆盖的部分记在那一节。
- 发现一，调用权限（`design.md` 第 4.2 节末尾）：默认建出的小队只有建队的人能唤醒。处理：闸门评论的固定表述加一句提示；文档写明前置条件；场景里两种负责人各走一遍。
- 发现二，指令长度（`design.md` 第 8.5 节）：Codex 运行时默认只读前 32 KiB，v2 队长读不到花名册和小队指令。处理：队长角色指令加一条截断时停下的规则；文档写明要调大的配置项。守护进程侧的根治办法是产品逻辑改动，不在本任务范围，现已授权另建产品任务，见本轮记录。
- 发现一的后续：从目录“应用到项目”建出的小队固定是 `private`，只有“从模板创建”页面能选向工作空间开放（`design.md` 第 4.2 节）。
- 发现二的后续：队长是 v1、小队指令是 v2 的组合在 32 KiB 处丢掉放行校验，所以小队指令开头也加了一条截断时停下的规则（第 4.5、8.5 节）。
- 另外三处改了已批准的设计：期限表不适用“取更严格的一条”；变体任务由队长建；已有小队的升级改成在界面里手工复制，因为 v2 队长正文没有查看入口（第 8.4 节）。
- 这些变更已获2026-11-20明确处理授权。独立复核之后又有两批，完整清单（19 项，第 19 项是 2026-11-20 的收尾批次）和等用户决定的事项在 `design.md` 第 14 节。

### 步骤 5 文案及其副本

- `builtin_squad_templates.go:350-353` 的中英文描述；`packages/views/locales/en/squads.json:186`、`zh-Hans/squads.json:181`。
- 副本：`packages/views/squads/components/builtin-squad-catalog.test.tsx:138,144`；`e2e/security-remediation-squad.spec.ts:22,29`。

### 步骤 6 文档、产品 skill、规范

- 按第 10 节的表和“文档要写的内容”九条（实现中加了第九条：已有小队怎样升级）。2026-11-19 已完成，结果记在第 10 节末尾。
- 中文文档改完后运行 `pnpm generate:docs-bundle`，提交生成结果，不手改生成文件。

### 步骤 7 补对照表

- 把第 11 节“本轮证据的类型”一列补成具体的测试名与位置。2026-11-19 已完成；补的时候发现收口对账的几句没有测试钉住，加了 `reconciliation` 契约组，场景里加了评论作者与来源任务的核对（第 9.2、9.4 节）。
- 交付说明里写明：本轮证据是契约测试加脚本模拟；模型是否遵守指令未经真实运行验证。

### 步骤 8 全量验证与独立复核

- 跑下面“验证命令”的全部条目，记录每条的真实结果；没跑的写明原因。
- 质量检查用 `trellis-check`。复核与编写分开：由没有参与编写的一方再读一遍两份正文和测试，对照 `design.md` 找遗漏。

2026-11-19 的验证结果。Go 整包各用一个新建的任务库。日志在 `/private/tmp/secrem-v2-20261118/logs/`，端到端的在同目录的 `e2e/logs/`；都在任务目录外，重启后可能消失。

| 检查 | 结果 | 记录 |
|---|---|---|
| `./internal/service` 整包 | 运行 3995，通过 3977，失败 0，跳过 18（依赖 Redis 的缓存测试） | `final2_service-20261119-063021-4227.log` |
| `./internal/handler` 整包 | 运行 5996，通过 5943，失败 0，跳过 53（依赖 Redis 等，本任务之前就跳过） | `final2_handler-20261119-064746-12876.log` |
| `./cmd/server` 与 `./internal/docs` 整包 | 运行 1149，通过 1140，失败 0，跳过 9（仅 Linux，或需要专门的环境） | `final2_cmdserver-20261119-064747-13044.log` |
| `go vet`，上面四个包 | 退出码 0，没有输出 | `vet-final2-20261119.log` |
| `gofmt -l`，上面四个包 | 只报 `internal/service/empty_claim_cache.go`。本任务没有碰这个文件，HEAD 上同样不过 | |
| 生成文档包 | 重新生成前后目录哈希相同；与 HEAD 的差异只有 `squads.json` 和 `security-model.json`，没有未跟踪文件 | `docs-bundle-final2-20261119.log` |
| `pnpm --filter @multica/docs test` | 64 通过 | `docs-test-20261119-b.log` |
| 目录组件测试 | 11 通过 | `views-catalog-final-20261119.log` |
| `pnpm typecheck` | 9/9，退出码 0 | `typecheck-final-20261119.log` |
| `pnpm lint` | 6/6，退出码 0，0 个错误；31 条告警都在本任务没改的文件里 | `lint-final-20261119.log` |
| 端到端 `e2e/security-remediation-squad.spec.ts` | 2 通过（en、zh-Hans） | `e2e/logs/playwright-20261119-063614.log` |

说明：

- 整包里本任务的测试都确实运行并通过：四个脚本场景、三条路由测试、通知测试、服务层契约测试。
- 这是第二轮整包。第一轮在改占位（`design.md` 4.1）之前：service 3994 / 3976 / 18，handler 5996 / 5943 / 53，cmd/server 1149 / 1140 / 9（`final_*-20261119-04*.log`）。两轮只差新增的那一条契约测试。
- 目录组件测试、类型检查和 lint 是在最后几处改动之前跑的。那几处改动是占位、文档里队长指令的路径、一条契约测试和规范，只涉及 `.md`、`.mdx`、`.go` 和生成的 JSON，不在这三项的检查范围内，所以没有重跑。
- 端到端环境是手工起的：自己的数据库、worktree 里构建的 API、生产构建的 Web，没有用 `make up`。脚本在 `/private/tmp/secrem-v2-20261118/e2e/`。改占位后重新构建了 API 再跑。
- 文档里写到的界面路径用临时探针在这套环境上核对过，结果和由此做的两处修改见 `design.md` 8.4 和 4.1。探针不入库。
- 没有跑 `make check`，也没有开竞态检测跑整包。原因：改动不含产品逻辑；`make check` 要另起一套环境，在这台共享机器上要占用二十分钟以上。这是当时未运行范围的历史记录；本轮验证取舍见后文，不再等待授权。
- 全程没有设置 `MULTICA_RUN_REAL_AGENT_SMOKE`，没有带 `agentintegration` 标签，没有调用任何已安装的智能体 CLI。

### 步骤 8 续：独立复核与最终验证（2026-11-19 下午到傍晚）

独立复核用了三条只读复核线，都没有参与编写：正文线（两份正文对照平台代码）、检查线（`trellis-check`，对照规范、测试和验收标准）、文档线。复核记录在任务目录外的 `/private/tmp/secrem-v2-20261118/review/`（`bodies.md`、`check.md`、`docs.md`）。

| 复核线 | 发现 | 已处理 | 留下的 |
|---|---|---|---|
| 正文 | 15 项（F1 到 F15），没有阻断项 | F1 到 F9、F12、F13 已改正文并加钉点；冻结后指出的两处（表情回应让 `revision` 变大、G1 旧结果在 G2 充数）在最后一批处理 | F10 主体和 F11 等用户决定；F14、F15、“负责人按子串匹配”和 F4 的剩余部分（执行记录按 `trigger_comment_id` 找）留到下一次修订（`design.md` 14.2） |
| 检查 | 冻结树上 8 项（F1 到 F8）加冻结后的 N1。复核最后一批（final5 树）：0 个阻断，2 个应改（L4-a、L5-b），3 个次要（L4-b、L5-a、L5-c），5 处措辞（L2-a、L2-b、L2-c、L4-c、第 1b 项）加后来的 N2，另给了两个可选做法 O1、O2。复核收尾批次（final6 树）：0 个阻断、0 个应改、0 个次要，只剩 N2 | F1、F4、F6、F7、F8 已改；F3（设计文档落后于正文）由文档更新补上；F5 剩下的几句和 N1 在最后一批加了钉点；L4-a、L5-b、L2-a、L2-b、L2-c 在收尾批次改掉，第 1b 项（规范里一句话的精度）随正文变长、规范改写而不再成立；N2 在收尾批次的复核之后改掉（见“步骤 8 再续”） | F2 的剩余部分、L4-b、L4-c、L5-a、L5-c、O1、O2 等用户决定或留到下一次修订（`design.md` 14.2） |
| 文档 | 15 项（F1 到 F15），没有阻断项 | F1 到 F13 和 F15 已改；F14 的第 1、2 项已改；它最后留的两处措辞（英文页少一个链接、来源对照表里一句话把两个原因并成一个）在收尾批次改掉 | F14 的第 3、4 项，都是可选的，属于页面原有结构：英文页没有“怎样判断派活是否成功”一节；英文页写状态时只写标识符。F10 里关于小节顺序的建议没有采纳（`design.md` 第 10 节末尾） |

最后一批（17:15 到 18:42）先加钉点看到失败，再改正文、文档、内置 skill 和规范，然后把下面的验证全部重跑。24 个改动路径没有增减；这一批动了其中 10 个文件。最终树的哈希清单在 `work/frozen-tree-20261119-final5.sha256`，最晚的一次修改是 18:41:45（生成的文档包），早于下面每一项的开始时间。

| 检查 | 结果 | 记录 |
|---|---|---|
| `./internal/service` 整包（新库） | 运行 4001，通过 3983，失败 0，跳过 18 | `final5_service-20261119-184438-10842.log` |
| `./internal/handler` 整包（新库） | 运行 5998，通过 5945，失败 0，跳过 53 | `final5_handler-20261119-184440-11012.log` |
| `./cmd/server` 与 `./internal/docs` 整包（新库） | 运行 1149，通过 1140，失败 0，跳过 9 | `final5_cmdserver-20261119-190820-19298.log` |
| 契约测试 `-run 'SecurityRemediation'` | 68 个全部通过；先红 7 个失败，原有文字的变体 9 个失败 | `nodb-20261119-171849-83709.log`、`-171528-79022`、`-172338-90629` |
| 五条路由测试 | 全部通过，含新增的 `ChangedReleaseCommentKeepsItsAuthor`。这一条写出来就通过，21:31 起另用 `-overlay` 做了变体证明：去掉编辑的加一、去掉表情回应的加一时各自失败，还原后通过（`design.md` 9.3） | `handler_d-20261119-172153-87966.log`；变体 `-213113-16719`、`-213247-18012`，还原后 `-213358-19023` |
| `go vet`，四个包 | 退出码 0，没有输出 | `work/vet-final5.out` |
| `gofmt -l`，四个包 | 只报 `internal/service/empty_claim_cache.go`（本任务没碰，HEAD 上同样） | |
| 生成文档包 | 重新生成后与目录逐文件比较无差异，没有未跟踪文件 | `work/docs-bundle-drift5.out`（空） |
| `pnpm --filter @multica/docs test` | 64 通过 | `work/docs-test-final5.out` |
| 端到端 `e2e/security-remediation-squad.spec.ts` | 2 通过（en、zh-Hans），API 用最终树重新构建 | `e2e/logs/playwright-20261119-191311.log` |
| 指令编辑器不安全字符扫描 | 0 处 | `work/scan-editor-final5.out` |
| 简报长度测量 | v2/v2 76,192 字节，截在协议第 4 步里（`design.md` 8.5） | `handler_d-20261119-174036-95520.log` |

这一轮没有重跑的：`pnpm typecheck`、`pnpm lint`、目录组件测试和 Web 构建。它们上一次是在冻结树上跑的（9/9、6/6、11 通过）；最后一批只动了 `.md`、`.mdx`、`.go` 和生成的 JSON，不在这四项的检查范围内。`make check` 和竞态检测仍然没有跑，原因同上面的说明。

整包里本任务的测试都确实运行并通过：服务层 14 个顶层测试（契约测试 32 个子测试），处理器层四个场景加五条路由测试，`cmd/server` 的通知测试。跳过的用例里没有一条属于本任务。

### 步骤 8 再续：收尾批次（2026-11-20）

检查线复核最后一批（`review/check.md` 的 “Last batch”）之后做了收尾批次，内容见 `design.md` 14.1 第 19 项：

- 放行校验第 2 项的时间条件改成“晚于最近一次派出这位审查员的评论”（L4-a）。先改契约测试的字面量看到失败，再改两份正文。
- 两份文档写明“设为结论”也算改动（L5-b）；英文页补上到 “Leader re-trigger rules” 一节的链接；来源对照表把两个原因分开写。
- 三处测试注释更正（L2-a、L2-b、L2-c）；规范按重新测量的长度改写。

24 个改动路径没有增减，这一批动了其中 10 个文件。哈希清单在 `work/frozen-tree-20261120-final6.sha256`，最晚的一次修改是 01:09:09（来源对照表），早于下表的整包、定向、`go vet`、文档和端到端各项的开始时间。两份正文是 00:38:53 保存的，先绿的那次运行和两次长度测量在它之后、01:09:09 之前。

| 检查 | 结果 | 记录 |
|---|---|---|
| `./internal/service` 整包（新库） | 运行 4001，通过 3983，失败 0，跳过 18 | `final6_service-20261120-012445-37992.log` |
| `./internal/handler` 整包（新库） | 运行 5998，通过 5945，失败 0，跳过 53 | `final6_handler-20261120-012446-38172.log` |
| `./cmd/server` 与 `./internal/docs` 整包（新库） | 运行 1149，通过 1140，失败 0，跳过 9 | `final6_cmdserver-20261120-013204-43788.log` |
| 契约测试的两个新字面量，先红后绿 | 先红：一个测试连同两份正文各自的 `missing_review` 子测试，共 5 行 FAIL，报缺的正是这两句；改正文后 68 个全部通过 | `nodb-20261119-234207-20039.log`；`nodb-20261120-003853-80049.log` |
| 服务层定向（模板、内置 skill、自治等级） | 118 个全部通过 | `nodb-20261120-011138-19580.log` |
| 处理器层定向 `-run 'SecurityRemediation'` | 26 个全部通过（15 个顶层测试） | `handler_d-20261120-011242-20621.log` |
| `go vet`，三个包 | 退出码 0，没有输出 | `work/vet-final6.out` |
| `gofmt -l` | 改过的 9 个 Go 文件无输出；整包仍只报 `internal/service/empty_claim_cache.go` | |
| 生成文档包 | 重新生成后逐文件比较无差异 | `work/docs-bundle-drift6.out`（空） |
| `pnpm --filter @multica/docs test` | 64 通过 | `work/docs-test-final6.out` |
| 端到端 `e2e/security-remediation-squad.spec.ts` | 2 通过（en、zh-Hans），API 用这棵树重新构建 | `e2e/logs/playwright-20261120-015241.log` |
| 指令编辑器不安全字符扫描 | 0 处（9 个文件） | `work/editor-scan-final6.out` |
| 简报长度测量 | v2/v2 76,304 字节，截在协议第 4 步里；四个自治等级逐个测量（`design.md` 8.5） | `handler_d-20261120-004939-92368.log`、`handler_d-20261120-010737-13493.log` |

整包里本任务的测试都运行并通过，数目和上一轮相同；跳过的用例里没有一条属于本任务。

检查线对这棵树做了一次只读确认（`review/check.md` 的 “Closing batch”，03:33 结束），六个问题逐条给了结论：0 个阻断、0 个应改、0 个次要，只有一处注释措辞（N2）。

- 它自己做的：两次核对 24 个哈希；服务层定向 117 个全部通过（`nodb-20261120-032415-83663.log`）；逐字节比较四个共用小节；重新生成文档包（无差异）；拿规范、内置 skill 和文档里的每个数字对两份测量输出。
- 它没有做的：重跑自己的两个长度探针；整包、`go vet`、类型检查、lint、文档测试、目录组件测试和端到端；核对本目录下的任务文档。
- 它另记了两种情形，都只会让队长多等或多重派一次，没有建议改措辞：审查结果落在队长读评论和读执行记录之间；有人在结果之后又 @ 了这位审查员，而队长把这次提及当成派出。

N2 在这次确认之后改了：`builtin_security_remediation_templates_test.go` 里的一行注释“The same reviewer has one run per gate.”，收尾批次之后已不准确，换成检查线给的原句。只动注释，没有再请复核线看。改后的核对：

| 检查 | 结果 | 记录 |
|---|---|---|
| `gofmt -l`，这个文件 | 无输出 | |
| `go vet ./internal/service/` | 退出码 0，没有输出 | `work/vet-final7.out` |
| 契约测试 `-run 'SecurityRemediation'` | 68 个全部通过 | `nodb-20261120-034456-94985.log` |
| `./internal/service` 整包（新库） | 运行 4001，通过 3983，失败 0，跳过 18 | `final7_service-20261120-034658-95920.log` |
| 目录组件测试 | 11 通过 | `work/vitest-catalog-final7.out` |
| 类型检查（`turbo typecheck`，加 `--force` 不走缓存） | 9/9，退出码 0 | `work/typecheck-final7.out` |
| lint（`turbo lint`，同上） | 6/6，退出码 0，0 个错误；31 条告警分在 17 个文件里，都不是本任务改的文件 | `work/lint-final7.out` |

最终的哈希清单是 `work/frozen-tree-20261120-final7.sha256`，和 final6 的清单只差这一个文件（03:44:11 保存）；上表各项跑完之后 24 个哈希仍然一致。处理器和 `cmd/server` 两个包不编译这个测试文件，第一张表里它们的整包结果、端到端和其余各项仍然对应最终的树。

检查线复核最后一批时还指出一处没有覆盖：路由测试 `ChangedReleaseCommentKeepsItsAuthor` 的作者断言没有失败过的运行（之前的两个变体只去掉了 `revision` 的加一）。04:58 补了第三个变体，同样用 `-overlay`，不改 worktree：让编辑顺带改掉 `author_id` 时，测试在作者断言处失败（`handler_d-20261120-045836-27102.log`）；还原后 5 条路由测试全部通过（`handler_d-20261120-045957-28039.log`）。之后 24 个哈希仍与 final7 清单一致。

没有重跑的只有 Web 构建：上一次是 2026-11-19 15:01（`work/web-build-final4.out`，退出码 0）；本任务的改动里会进 Web 构建的只有两个语言文件，它们最后一次保存是 14:23:17。`make check` 和竞态检测仍然没有跑，原因同前。全程没有设置 `MULTICA_RUN_REAL_AGENT_SMOKE`，没有带 `agentintegration` 标签，没有调用任何已安装的智能体 CLI。

### 基线分支

worktree 的基线是 `ccd28cbc0`，没有变基过。到 2026-11-20 04:27，`codex/cross-workspace-collaboration` 在 `7588a3508`：领先 34 个提交，改了 1001 个文件（704 个在 `.trellis/tasks` 下），新迁移 866、867、868。

和本任务的关系：

- 24 个路径里有两个被基线改过：`packages/views/locales/en/squads.json` 和 `zh-Hans/squads.json`。基线（`b6b07c306`）在两个文件里各加了 16 个键（`instructions_tab.remote_changed` 和 `recovery.*`），没有改或删已有的键；本任务在每个文件里只改了 `catalog.summaries.security_remediation` 一个值。在副本上用 `git merge-file` 做三方合并，两个文件都没有冲突（`work/base-merge-0155/`），合并结果的键集合与基线最新提交相同，双方的值都在（`work/merge_keys_check.py`）。
- 前端这一侧没有在基线上跑过，下面是读差异得出的：目录组件 `builtin-squad-catalog.tsx` 和它的测试在基线上没有变；端到端用例走目录的“应用到项目”，最后打开小队详情页，基线改了详情页（`squad-detail-page.tsx`）和两个创建页面。
- 基线改了本任务的测试会用到的服务端文件：`cmd/server/notification_listeners.go`（评论的收件箱条目改走 `CreateCommentInboxItem`）、`internal/service/task.go`（`ClaimTasksForRuntimes`）、`pkg/db/queries/agent.sql`、`internal/handler/handler_test.go`（夹具清理）、`cmd/multica/cmd_issue.go`（`init` 里一行）。本任务写下的文字没有引用这些文件里的行号。

预检（2026-11-20 03:20 到 03:50）。不动两个检出：用 `git archive` 把基线最新提交的 `server`、`scripts`、`examples` 取到 `/private/tmp/secrem-v2-20261118/tip-tree`，盖上本任务的 15 个服务端文件（`tip-tree.sh`；基线改过或新增了其中任何一个文件时它会拒绝），新建的任务库迁移到 868（`migrate-tip.sh`），三个整包各跑一遍（`tip-run.sh`）：

| 包 | 结果 | 记录 |
|---|---|---|
| `./internal/handler` | 运行 6037，通过 5984，失败 0，跳过 53 | `tip-tip_handler2-20261120-033139-89643.log` |
| `./cmd/server` 与 `./internal/docs` | 运行 1188，通过 1179，失败 0，跳过 9 | `tip-tip_cmdserver2-20261120-033456-90736.log` |
| `./internal/service` | 运行 4231，通过 4211，失败 2（一个测试和它的一个子测试），跳过 18 | `tip-tip_service-20261120-033138-89471.log` |

本任务的测试在三个包里都运行并通过：服务层 14 个顶层测试（契约测试 32 个子测试），处理器层 15 个，`cmd/server` 的通知测试。预检用的是 final6 的文件；之后只改了一行测试注释（N2）。

预检里见到的失败都不来自本任务，前两条合入时会遇到：

- `TestBuiltinSkillsConformToTemplate/multica-working-on-issues`（service）：“SKILL.md body is 501 lines, over the 500-line L2 budget”。这个 skill 不在本任务的 24 个路径里，做判断的 `builtin_skills_test.go` 也不在；临时目录里的 skill 文件和基线最新提交的逐字节相同。`ccd28cbc0` 上它的正文正好 500 行，基线的 `89740ed14` 加了一行。04:00 主检出的工作区里这个文件有未提交的改动（净减 3 行，按测试的算法是 498 行），是谁改的没有查；它提交之前，变基后的 service 整包会带着这个失败。
- `TestCollaborationInformationHistoryWireAuthorityAndQueries`（处理器，基线的 `f6703b7e4` 加的）：没有设置非默认的 `JWT_SECRET` 时返回 503 `information_context_unavailable`。同一棵树上不设时失败（`tip-tip_handler-20261120-032808-86460.log`），设一个每次运行临时生成的值时通过（`-032815-86651`）；上表三行都是带着临时值跑的，这个值不打印、不保存。`ci.yml` 和 `scripts/` 里没有找到给 Go 测试设 `JWT_SECRET` 的地方。
- 第一遍（03:20）另有 5 个失败是预检自己的问题：临时目录里少了 `examples/`，`TestExamplePlugin*` 和 `TestSchedulePulseExampleManifestDrivesDurableRetry` 要读仓库根目录的 `examples/plugins/...`。补上之后通过。

合入之前仍要真正变基，再重跑整包、目录组件测试、类型检查、lint 和端到端。预检是文件覆盖，不是变基，基线也还在动。新迁移意味着测试库要从迁移到 868 的模板克隆。

### 验收标准对照

| 验收标准（`prd.md`） | 结论 | 依据 |
|---|---|---|
| AC1 契约组、否定断言、共用小节一致 | 满足 | 契约测试 68 个；`SharedSections` 四节逐字节一致 |
| AC2 复现的四条边界、先复现后修复、作者不得改复现测试 | 满足 | `reproduction` 组；`CodeRouteReproducesBeforeFixing`；场景 `CodeRouteWithVerifierAndVariant` |
| AC3 分诊块是 v1 的扩展、12 个新字段、期限表、提交绑定、变体 | 满足 | `TriageBlockExtendsV1`；`triage_v2`、`due`、`commit_binding`、`variants` 组 |
| AC4 v1 的契约组和父任务状态测试不动 | 满足 | 对契约测试文件的差异只删了四处版本钉点 |
| AC5 `design.md` 9.3 的每一行都有运行的测试 | 满足 | 9.3 两张表；整包日志里各有 PASS 行，没有 SKIP |
| AC6 四个脚本场景，`design.md` 第 11 节补全 | 满足 | 四个 `TestSecurityRemediationFlow_*` |
| AC7 目录与文档的标记、7 个座位、试点指令样例和入座步骤 | 满足 | 组件测试 11 通过（最终的树）；端到端 2 通过（final6 树，之后只改了一行 Go 测试注释） |
| AC8 文档包无漂移，验证命令通过 | 满足，有两处说明 | 见“步骤 8 再续”的两张表；`make check` 和竞态检测没有跑；Web 构建在最后两批之后没有重跑 |

检查线对冻结树独立给出了同样的结论（`review/check.md` 的 “Acceptance criteria”）。之后的两批它各复核了一次（“Last batch” 和 “Closing batch”），范围是那两批改动的内容，没有重做这张对照表；表里在最后一批和收尾批次之后的结论是协调者按上面的运行记录核对的。

## 验证命令

Go 测试在 `server/` 下运行，带上指向独立测试库的 `DATABASE_URL`（见启动前检查第 6 条）。服务层的契约测试不需要数据库。

```bash
# 契约测试（步骤 1、2、3）
go test ./internal/service -run 'SecurityRemediation|TestSquadTemplates?_|TestSquadLeads_|TestAgentRoleTemplates?_|Autonomy' -count=1 -v

# 处理器层（步骤 3、4）：看 -v 输出里的 RUN / PASS，不能有 SKIP
go test ./internal/handler -run 'SecurityRemediation|TestShouldEnqueueSquadLeaderOnComment|TestCreateComment_Squad|TestTaskCapability|TestClaim_' -count=1 -v
go test ./cmd/server -run 'TaskCapability|Notification' -count=1 -v

go vet ./internal/service/... ./internal/handler/... ./cmd/server/...
gofmt -l internal/service internal/handler cmd/server
```

仓库根目录：

```bash
pnpm generate:docs-bundle
git diff --exit-code -- server/internal/docs/content
git ls-files --others --exclude-standard -- server/internal/docs/content   # 应无输出

pnpm --filter @multica/views exec vitest run squads/components/builtin-squad-catalog.test.tsx
pnpm --filter @multica/docs test
pnpm typecheck
pnpm lint

pnpm exec playwright test e2e/security-remediation-squad.spec.ts
```

- 上面的 `-run` 模式和 `--filter` 写法在实现时先各试一次，确认选中了预期的测试；选中数为 0 的命令不算通过。
- 把输出重定向到文件再看退出码，不要接 `tail`：管道会吞掉失败的退出码。
- 生成文档的漂移检查与 CI 相同（`.github/workflows/ci.yml:295-306`）。
- 端到端用例需要一套在运行的环境。不用 `make up`：它会回收其他会话的环境。改用手工启动的一套（自己的数据库、显式的 `DATABASE_URL`、worktree 里构建的 API 与 Web）。跑不起来时写明原因和已尝试的内容，不写成通过。
- 合入前按风险决定是否跑 `make check`。服务包在竞态检测下约需 13 分钟。
- 全程不设置 `MULTICA_RUN_REAL_AGENT_SMOKE`，不带 `agentintegration` 构建标签。

## 风险点

| 位置 | 风险 | 对策 |
|---|---|---|
| 两份 `INSTRUCTIONS.md` | 改写时碰掉 v1 的契约标记，或让那句状态管理权的话出现两次 | 步骤 1 先把测试写好；步骤 2 的自查清单 |
| 共用小节 | 两份文件各改一处后不再一致 | 逐字节比较的测试；只在一份里改，再整节复制 |
| `squad_comment_trigger_test.go` | 往已有的表里加行，可能和别的会话的改动冲突 | 启动前检查第 3 条；只加一行，不动其他用例 |
| 处理器层场景 | 依赖数据库；静默跳过会被当成通过 | 看 `RUN`/`PASS`；选中数为 0 不算通过 |
| 生成的文档包 | 每个页面正文是一整行 JSON，差异不好读 | 只审 `.zh.mdx` 的差异，生成结果靠漂移检查 |
| 目录文案 | 四处副本（两个语言文件、一个组件测试、一个端到端用例）漏改 | 步骤 5 一次改完，跑组件测试和端到端用例 |
| 当前检出 | 其他会话的未提交改动 | 在独立 worktree 里做；提交时按路径暂存，不用 `git add -A` |

## 回滚点

- 步骤 2 之后：两份正文加步骤 1 的测试是一个可独立回退的单元。
- 步骤 3 之后：版本号加钉点是一个单元。只回退它，正文仍是 v2 而版本记为 1，不要单独这样做；要退就连正文一起退。
- 步骤 4：只新增测试，回退不影响产品行为。
- 步骤 5、6：文案和文档各自成单元。
- 整体回滚见 `design.md` 第 13 节。

## 提交

- 2026-11-20用户已明确授权本地提交；无需再询问。未请求远端推送。
- 建议的拆分（每个提交自身能通过它涉及的测试）：
  1. `feat(squads): gate security remediation on a named human owner`：正文、版本号、契约测试、全部版本钉点（含端到端用例的那一处）。
  2. `test(squads): pin the comment routing the security gates rely on`：处理器层测试。
  3. `docs(squads): document security remediation v2`：文案及副本、文档、生成包、产品 skill、规范。
- 提交信息用 conventional prefix + Lore trailers，Tested/Not-tested按实际证据；不添加没有实际身份依据的署名。


## 2026-11-20 授权续做与最终交付记录

### 工作位置、快照与实际变基

- 开始时主检出 `codex/cross-workspace-collaboration` HEAD 是 **efdb597d6b80f930b5956de30bc8e4780e4023c7**；任务 worktree HEAD仍为ccd28cbc0，24个未提交产品路径。
- 主检出其他会话路径均保留。持久证据根目录：`/Volumes/artisan/code/2026/security-remediation-v2-evidence-20261120/`，权限0700。`initial/`保存两个检出status/HEAD/跟踪diff、全部脏文件哈希、原24路径tar与三任务完整副本。主检出的完整未跟踪文件清单大于status目录条目数，已按文件而非目录计数。
- 用 `git stash push --include-untracked` 保存任务全部改动，stash对象 **ac6f5c8a1cba3c8819d2aa276bd532600716e73b**；实际 `git rebase efdb597d6...` 成功，再按对象hash apply，没有pop或丢失备份。
- 恢复检查：24路径齐全；22个字节不变；en/zh-Hans squads.json仅与新基线合并，flatten后键集合等于基线，catalog security摘要保留本任务值，其余键逐个等于基线。没有带入主检出未提交修改。
- 三个最新任务目录从主检出复制到任务worktree；后续权威修改统一在任务分支。合入前核对主检出原副本与快照，任务记录单独提交，不能整仓暂存。

### 范围与最终决定

- design14.1按原编号逐项重写为最终处理/理由/用户影响/实现和验证；正文相关章节已同步，14.2逐条给出本轮落实或跟踪去向。
- comments双语+生成comments.json加入，产品路径由24增为27。没有新增handler/service产品逻辑、数据库迁移、前端组件逻辑或依赖。
- 试点前修订在首次提交冻结前纳入v2（父任务K12），后续真实验收发现的新指令变化需v3，不能同版本换正文。
- 两个独立产品规划：A `11-20-comment-admin-human-authority`，B `11-20-codex-project-doc-budget`；另建推送边界与表情术语跟踪项。均只交付规划，产品实现不混入本轮。

### 本轮环境与基线故障重新判断

- 原共享PostgreSQL在验证准备时被其他运行状态变化停止，127.0.0.1:5432拒绝连接；失败发生在只读身份探测，未写库。未启动/停止他人容器。
- 改用任务独占 `secrem-v2-1120-postgres` 容器、随机绑定loopback端口、私有随机口令；每个整包独立新库，迁移到当前868。测试前用实际连接URL核对current_database；每库私有签名密钥，TMPDIR解析为真实路径。普通测试全部套CLI防护，未启agentintegration或真实调用开关。
- `multica-working-on-issues` 行数问题在当前已提交基线上已修复：独立测试RUN/PASS2，日志 `nodb-20261120-070229-46786.log`（旧tmp logs目录）。没有复制主检出未提交修复。
- `TestCollaborationInformationHistoryWireAuthorityAndQueries`：新基线的check.sh已生成私有JWT_SECRET，本轮手工验证同样生成独立密钥；新库RUN/PASS13、FAIL/SKIP0，持久logs/probe-20261120-070201.log。没有放宽产品的默认密钥拒绝。

### 验证计划与make check取舍

最终产品树冻结清单为持久 `work/final-product-sha256.json`（27路径）。整包service、handler、cmd/server和internal/docs分别跑新库；go vet与改动Go格式；完整frontend lint/typecheck、目录组件、文档测试/生成/链接；最终API与生产Web构建跑en/zh-Hans两例。定向race覆盖本任务路由、脚本场景、通知及契约，数据库独立于普通整包。

本轮不执行完整make check：其固定步骤额外执行全仓TS、全仓Go race、Desktop构建及全部Playwright，与本任务无平台逻辑变更的边界不相称，也会占用共享机器大量资源；已阅读当前check.sh确认其行为。这里不是等待用户决定。用本轮明确的整包/定向race/生产Web验证替代，最终报告不宣称完整make check通过。

### 本轮新增契约证据

- 新规则先红：旧tmp logs/nodb-20261120-070157-43602.log，RUN75 PASS61 FAIL14 SKIP0，缺的正是新增文字。
- 首次绿色尝试遇到并发文档写入导致go:embed长度变化，编译失败、RUN0，保留nodb-20261120-070641-49929.log，不算产品测试失败或通过。
- 文件静止后服务定向：nodb-20261120-071234-55647.log，RUN/PASS101，FAIL/SKIP0。
- 最终长度：79,766 B（默认花名册/最小上下文），四等级均在协议第3步之前截断；两次overlay探针各RUN/PASS1。日志与测量表见design8.5当前段。

### 集成状态（动态记录）

已完成真实变基；尚未填写产品提交、任务记录提交及实际合入证据。该状态不能被解释为已合入；子任务二真实入口在合入及参数/环境门槛满足前关闭。子任务二规划、合成夹具和无模型预检可以继续。


### 本轮验证实绩（持续追加）

| 检查 | 实际结果 | 持久证据 |
|---|---|---|
| frontend lint/typecheck强制无缓存 | 17/17成功，0错误；既有告警未清理 | logs/frontend-static.log |
| 目录组件 | 11通过 | logs/catalog.log |
| 文档测试 | 64通过 | prior/logs/docs-test-final-20261120.log |
| 生成包 | 157文件逐字节无漂移 | prior/work/docs-final-verification-20261120.txt |
| 新/既有文档链接 | 改动六页74处local link/anchor通过 | 同上及prior/work/docs-links-revision-20261120.txt |
| 最终API与生产Web构建 | 两者退出0 | logs/api-build.log、web-build.log |
| 最终构建E2E | en、zh-Hans两例通过，0重试 | logs/playwright.log；e2e/results |
| go vet四包 | 退出0 | logs/go-vet.log |
| 改动Go格式 | 无输出 | logs/gofmt.log |
| cmd/server+internal/docs整包 | RUN1199/PASS1190/FAIL0/SKIP9；通知用例实际运行 | logs/cmdserver-20261120-072527.log |
| handler定向race | RUN/PASS38，FAIL/SKIP0 | logs/race_handler-20261120-074727.log |
| 通知race | RUN/PASS1，FAIL/SKIP0 | logs/race_cmdserver_native-20261120-080050.log |
| 服务契约race | RUN/PASS75，FAIL/SKIP0 | prior/logs/nodb-20261120-080050-22259.log |
| 独立复核 | 产品0阻断/0应改；复核者契约75/75，27hash一致 | research/review-20261120.md |

首轮handler整包在30m截止不完整，3946 RUN、3921 PASS、1 FAIL、7 SKIP。失败是未修改的基线聊天测试；独立定向3/3通过但未证明原因，见research/baseline-chat-check-20261120.md。首轮service在833 RUN（815 PASS、无测试FAIL）时由协调者终止，改用原生ARM集群；这次同样不算整包通过。两个原始日志完整保留。

环境修正：缓存的pgvector:pg17 tag实际amd64，引擎aarch64；第二轮使用已有arm64镜像digest `sha256:215ae4d47393a980e9ef7ff7760f7dad53ec3e84bceb9fbd45cef34e50b5ba0f` 创建独立容器与新库，timeout45m，原断言未改。架构事实不能单独证明聊天失败根因。原始两库保留至最终证据核查；不清理/重启其他会话服务。


### 整包复跑与未修改基线的异常

- handler原生arm64新库整包已通过：RUN6037/PASS5984/FAIL0/SKIP53，1017.024秒；日志 `logs/handler_native-20261120-080050.log`。逐项库存核对：所有SecurityRemediation、订阅新增用例和g行都有PASS，没有本任务skip。先前聊天失败在完整复跑通过，原因仍未被证明。
- service原生arm64新库整包在45m超时：RUN2651/PASS2614/FAIL19/SKIP0，日志 `logs/service_native-20261120-080051.log`。19个FAIL事件含父/子测试；主要是已有StageCompletion夹具产生unknown而非succeeded，均不在本任务产品改动中。不能记为完整测试通过。
- 精确前5个失败在service_diag新库原样PASS5/5，103.746秒；随后只加日志的私有overlay尝试更慢，未改变工作树，不把它作为产品修复。诊断见research/baseline-stage-completion-check-20261120.md。
- 环境控制性下一轮：用缓存的Linux arm64 Go1.26.6镜像（CI同工具链系列），只读挂载最终源、独立缓存和库，测试与DB共享Docker VM时钟，CLI防护仍在。源码、断言、平台时限均不放宽。该尝试的成功也不能自动证明旧失败根因。


### 最新已提交基线再同步（约09:00）

主检出前进到c56e2194ee45236b8c634843579b7f3e492b0db4（新增协作code-revision测试及任务记录；与27产品路径零交集）。主检出另有未提交stage fixture时钟修复，明确不带入。

先停止本任务尚在编译的Linux容器、保存全部产品与任务文档（含未跟踪）到stash `190670484a7e13b2171c7c3fb011bb6af7e3209f`，实际rebase到c56e2194e再apply。27产品sha256全部不变，任务记录完整恢复。前端、handler、cmd/server/docs及生产二进制的产品代码输入未变；新增service测试需包含在最终service整包。固定Linux只读源快照由c56e2194e的server/scripts/examples加任务server差异生成，防止并发任务记录写入影响编译。

最终service环境使用原生ARM PostgreSQL、任务独占tmpfs数据目录（768MiB），保持PostgreSQL默认fsync、产品超时及测试断言。只改变临时测试存储环境，避免共享持久卷I/O影响；最终Linux测试与DB同一VM时钟，Go1.26.6。原始不完整运行不被覆写。


Linux默认UTC完整service包跑完：4239 RUN、4210 PASS、10 FAIL事件（含父子）、19 SKIP。新增发现是已有time.Time Local UTC与JSON UTC表示的DeepEqual差异，涉及评论读回、memory replay和stage内部不确定回退。原二进制四组对照：UTC 11/2/9，Asia/Shanghai 11/11/0。最后完整service运行只固定TZ=Asia/Shanghai（本机时区），保持同一二进制/源码/断言。默认UTC环境仍有基线风险，不能以目标环境通过消除它；详见research/baseline-recall-time-check-20261120.md。


### 最终提交与清理约定

本轮按用户指定保留现有三个任务路径，任务记录单独提交，不使用自动归档/写共享journal脚本：父任务与子任务二尚未完成，主检出journal被其他会话修改，自动脚本会扩大暂存范围。子任务一完成状态与整包/实际合入证据写入task.json，保留路径便于后续AC10引用。

只清理本任务拥有的数据库、容器、临时文件、备份stash和最终不再依赖的worktree；提交分支保留。主检出正在变化的其他会话工作不stash、不暂存、不修改。真实模型参数未绑定前只做无模型工作，最终一次性补齐参数问题不重复询问调用授权。


### 最终验证通过与产品提交

最终service完整包：Linux arm64 Go1.26.6、TZ=Asia/Shanghai、fresh migrated868库，RUN4239/PASS4220/FAIL0/SKIP19；本任务75个全部PASS、无SKIP。日志 `logs/service-linux-release.log`、回执 `work/service-linux-release.json`，全包命名库存 `work/final-test-inventory.json`。19跳过是Redis18及root权限测试1，均不属于本任务。完整make check未跑，默认UTC失败仍是明确剩余风险；不宣称跨时区验证通过。

三笔产品提交（相对于c56e2194e，共27路径）：

- `15bcd2e5e7a95b69569e546236cdeeb3e87324f8`
- `0ffc3572ce0441489dd1502ddf2f3f061bb9a44d`
- `aa592194b233daf43878c3be52f76c845e0e5044`

第一笔只暂存正文/版本/契约与版本钉点；registry描述和E2E摘要通过精确index blob留到第三笔，没有修改已验证的worktree字节。第二笔处理器/通知/场景测试；第三笔目录、文档、生成包、skill、规范。每笔conventional+Lore，Tested/Not-tested包含实际环境与模型未验收边界。git diff--check通过，最后产品hash仍与冻结27路径一致。

任务记录提交独立完成后再实际fast-forward到基线分支；最终集成回执另写。三笔提交本身不能代替合入。
