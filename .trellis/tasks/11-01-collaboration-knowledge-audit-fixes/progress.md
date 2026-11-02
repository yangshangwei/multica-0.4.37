# 知识交付审计缺陷修复：进度

当前新轮：共享 `make-check-v9` 已于 **2026-11-02 15:36:09 UTC** 启动，仍在运行。v6 的原失败和新专用库定向红绿分别留证；新 source/适配器/launcher 独立复核通过，见 [v9 启动绑定](research/continuation-20261102/shared-check-v9-start.json)。定向 7 项通过不等于整套通过；下面 v6 失败为上一轮实际结果。

上一轮实际结果：共享 `make-check-v6` 已于 **2026-11-02 15:02:20 UTC** 失败结束，make exit 2。15 项静态、13,082 项 TS 和脚本回归通过；Go 到第 6 包 `cmd/server`，两个顶级 skill catalog 场景收到 503 `resource_store_unavailable`，含父/子项为 3 个 fail 事件，已执行 Go 共 1,889 pass / 7 skip；剩余 Go、API/Web/PW 未到达。见 [本轮实际结果](research/continuation-20261102/shared-check-v6-result.json)。下面“启动/running”段落是历史入口，不能继续作为当前状态。

唯一执行负责人已将原因定位到 API-only `MULTICA_RESOURCE_PUBLISH_DIR` 启用发布器，但 Go TestMain 直接 SQL fixture 没有内部组织。新 v9 只隔离 outer ordinary-Go 子进程的该变量，保留真实 API 和测试自有递归配置；先在新的专用库做实际红绿及独立复核，再跑新的完整检查。旧 v6 与失败库/日志保留。本知识会话只读核对初始化/目录契约，不启动第二套测试或改产品/断言。

历史启动记录（14:47:28 UTC）：21 条审计处置已独立核对，L7 的未复现/测试覆盖例外已写入 AC1；共享第 4 轮完整检查当时实际启动。唯一执行负责人为首轮闭环会话。选定 Go 适配器 `8963ebcc...` 已包含冻结 v8 和全局 `--` 分派微修、现有 Make/HOME/pool 改动；两份独立 pass-with-notes/0 阻塞和 RKI6 结果哈希已核对，见 [同源运行绑定](research/continuation-20261102/shared-check-binding.json)。原 v5/v7/v8 的失败或开放条件保留在各自版本报告中，不重标为通过。

本任务只有收到共享完整检查的实际成功结果、核对必要具名用例与跳过范围后才可收口。知识 K1–K3 的六个测试/driver 代码文件及 14 次知识/1 次独立 E0b 仅完成设计，尚未实施、分配预算或运行；launcher 是后续协调构件。这些真实模型/发布门归 #6 和父任务，不阻止本审计子任务在本地 AC 完成后独立关闭。

## 2026-11-02 本轮推进

用户要求列出计划并继续，当前入口为 [推进计划](advance-plan-2026-11-02.md)。已补齐并验证 implement/check 各 8 条上下文，恢复本已有任务；没有新建任务。第 4 候选 `64a9994ec` 工作树干净，产品、原 v5 适配器、launcher、私有配置、CLI 和 renderer 指纹均与登记一致。renderer 的旧本地 HTTP 辅助进程已退出，本轮确认端口空闲后恢复，实际页面字节与已固定构件一致。

RKI6 实际证据已独立复核为 pass-with-notes、0 阻塞：normal/race 各 62 条具名通过、无失败/跳过，原始证据与第 4 候选的源码/框架绑定一致；见父任务 `research/final-candidate-20261102/continuation-20261102/rki6-review.json`。v5 私有适配器复核为 changes-required：AV5-1 递归包选择未绑定 allocation.package；AV5-2 缺少显式 Go 路径仍运行测试。本轮隔离的 v7 修复又复现取值 flag / -test.* alias 中的 -c 被误判为 compile-only，正在补充最小修复与红绿证据。原 v5 和首轮 v7 结果保留，不改原失败。

完整第 4 轮由首轮闭环会话 `01a19c0d-cce1-73e3-99ee-2c3c9e9239ba` 单执行，知识侧提供独立修复/复核后复用同源结果，不另开完整检查或使用其 Redis16422/renderer19845。已确认原 v6 属于该会话且未被本会话覆盖；本会话只在 `adapter-v7-knowledge-advance-20261102` 写独立增量。renderer19844 的自有辅助进程在留证后已停止。第三轮已结束且失败（TS 13,082 通过、Go 82 包无失败、浏览器 47 项失败），不能继续使用旧“运行中”字段。原 source URL 的种子库已不存在，但 check.sh 会通过明确的 maintenance 连接新建自己的 API/Go 验收库，因此无需重建旧库。只读容量核对为 38/100 个连接，未改变共享配置或其他会话连接。见父任务 `research/final-candidate-20261102/continuation-20261102/preflight.json`。

下面保留此前分道与合流过程；本轮最新阶段以上述计划、实际复核与新的完整运行结果为准。真实 Provider 调用仍为 0，知识默认 false，真实模型、发布及默认库恢复继续按各自门记录。

最新入口：`research/combined-ready-for-main-integration.json`。21项审计、分道复核跟进及集成发现均已处置，无未处置阻塞产品项；最终整体候选验收未完成，任务保持in_progress。全量失败与复验分开记账，原service exit1不冒充passed。按既有用户授权由统筹本地合入主集成，然后在最终候选跑完整makecheck与门控验收。

## 2026-11-02 合流验证入口

知识四分道已按claim→launch→write→web顺序本地合入，并吸收最新主集成runtime，共同产品源码`45efa093c`，后续夹具/skill入口及执行计划测试增量`0702d5039`/`742abb217`。全部21项已有修复或测试处置，L7未复现而以实际进程终止测试处置，不能写成修了不存在的产品缺陷；CT1同时间戳跨页测试及IW1预算DB时钟跟进也已补齐。所有分道最终独立结论均pass/pass-with-notes，IW1增量独立pass。合并后的handler/router、前端全套和跨分道独立复核已完成；完整service原运行的3项失败已分别处置并复核，但原exit1不改为通过。当前具备本地合主集成条件，最终完整makecheck及同候选验收尚未通过。

- claim产品`30edf3e36`、H3/M3`94e790d2d`、CT1`5c3549001`；完整门/三视角证据见research/claim-ready-for-integration.json。
- launch产品H2/M2/L6及H2接受/L7补验，最终分道`f00c94962`；research/launch-ready-for-integration.json。
- write L5`ac6546772`、L1归因`d4bc37a86`、IW1`464540191`；research/write-ready-for-integration.json。
- web产品`ed0026525`，独立复核pass-with-notes，分道缺失的admin时钟修复`9c056ad68`在知识集成保留；共同源码Core全套4340项通过，Views全套运行中。
- 共同源码sqlc零漂移、Go build/vet全包通过，core/views lint/typecheck通过；DB-backed各包使用单独已迁移新库，完整证据research/integration-verification/。
- 默认库迁移事故`0aedf934e`已向用户披露并保留，不作为通过证据；当前不得down/清理默认multica，迁移由统筹先验证库身份后执行。
- 原workflow不跨会话resume。主集成runtime已合入`9e7f18785`；知识尚未合主集成，须待整体复核与同源门结束后由统筹独占合入。
- RK1 runtime25已批准/消耗0；原系列E3after trial2/剩14；K1–K3/E0b/安装版和AC43独立预算/环境待落实。生产开关false。

## 2026-11-01 23时续作接管

新统筹 Codex `01a1979f-c71b-7ea2-8734-185ab2bdc8f8` 按最新用户请求接管既有任务；旧workflow不跨会话resume。此前负责人及19:53段落保留为历史。已核实四分道HEAD：claim `3110295d0`、launch `3351c4c13`、write `fba1fca3f`、web `a9801fb07`；知识集成仍`8c0f37672`。21项已有14项分道提交，不能据此视为通过；launch两个dirty测试、write五个dirty文件和L5红测证据保留。

- claim由原生Trellis implement子代理`/root/knowledge_claim`接管：先补H3/M3，再处理托管参数复核问题。
- launch/write/web现有修改不覆盖，待全局额度释放后按既有分道顺序接管；本轮其他两个执行分道是runtime和RK1 fake，不再各自派生代理。
- 本统筹独占知识集成及主集成合流；分道仅可在自己的worktree提交。独立复核pass或pass-with-notes且无阻塞后，本地合入已有用户授权，无需重复询问。
- 不能删旧失败数据库；完成后是否清理另行评估，原文“清理kfix全部库”不作为当前执行入口。
- 合流后必须在同一候选源码联合验证，再跑真实模型。RK1 runtime 25次与原系列剩余14次分账，知识K1–K3/E0b未分配预算。生产知识开关默认false。

完整现场快照：主集成父任务`execution-resume-2026-11-01.json`。当前未关闭任何缺陷或验收门。

核对时间：2026-11-01 19:53 +0800。依据是各分道分支的实际提交、`research/evidence/` 文件和 workflow 日志，不依据会话汇报。

## 负责人变化

- 会话 -14 [2a1270] 建好了环境、PRD、设计和 6 份方案比较，13:36 起一直报网关 401/502，没有提交修复代码。
- 13:55 用户让会话 -54 接手：先修 H1，再修其余缺陷，然后跑 make check、合入、RK1。-54 在 `2057b5890` 保存了 -14 的设计，H1 完成后于 15:46 启动四分道 workflow（`kfix-lanes`）。-54 的主循环 16:37 起报 502/EPROTO；workflow 仍在运行。
- 18:30 会话 -5c 曾提议由自己接手集成。用户 2026-11-01 19:11 在会话 -fe [ad8790] 中另定了分工（由 -fe 转达），以此为准：
  - -54：全部代码和测试代码、四条分道、workflow 写的 `research/evidence/`、`.trellis/spec/` 和内置 skill 文档（按 AC9 随各自的代码提交一起改），以及分道合入集成分支、最终验证。
  - -5c：只维护本任务目录下的 `prd.md`、`design.md`、`implement.md`、`progress.md`、`task.json`。发现需要改代码的问题就发给 -54。-54 开始往集成分支合入分道之后，-5c 每次提交前都先和 -54 对齐，并跑 `git status`、`git diff`，避免两边同时提交。
  - `9c056ad68`（admin 测试固定时钟）是 -5c 在这个决定之前提交的，保留不动。
  - 合入前复核：-fe。
- 合入授权：用户 2026-11-01 14:56 在会话 [4a018c] 同意"复核结论为 pass 或 pass-with-notes 后可以直接合入 `codex/cross-workspace-collaboration`"。

## 缺陷状态

| 分道 | 缺陷 | 提交 | 复核 |
| --- | --- | --- | --- |
| claim | H1 | `4cf4d45a2` | 3 份，pass_with_notes |
| claim | H1-followup | `6f7a601ce` | 2 份，pass_with_notes |
| claim | M1 | `2dd633df4`（瞬时错误重新入队）、`3110295d0`（finalize 拒绝时结算任务） | 复核中 |
| claim | H3、M3 | 排队 | — |
| launch | H2 | `680ad868f` | 3 份，pass_with_notes |
| launch | M2、L6、L7 | 实现中，M2 的改动未提交 | — |
| write | L1、L2 | `c930ac847`（时钟接缝）、`0cf5f7918`、`3aee019aa` | 1 份，pass_with_notes |
| write | L3 | `931446b23` | 与 L4、L5 一起复核 |
| write | L4、L5 | 实现中，L4 的改动未提交 | — |
| web | M4、M5、L8 | `500781f4c`、`2e28c2c98`、`a5e8941b7` | 2 份，pass_with_notes |
| web | L9 | `60ca94822` | 与 L12 一起复核 |
| web | L12 | 实现中，改动未提交 | — |
| web | L10、L11、L13 | 排队 | — |

- 21 项中已有 10 项提交。其中 7 项已复核完毕，没有阻塞项；M1、L3、L9 的复核还没完成。每项都有 `<ID>-red.txt` 和 `<ID>-green.txt`，放在各自分道的 `research/evidence/` 下。
- 19:53 时 launch、write、web 三条分道各有一组正在实现，改动还没提交。-54 准备把工作交给新会话。workflow 只在 -54 的会话里运行，`resumeFromRunId` 也只能在同一个会话里用。所以 -54 一关，这三组改动就会停在半成品状态，新会话不能直接续跑这次 run。
- 各分道的完成门和整条分道的复核都还没有运行。

## 集成预检（只读）

- 四条分道之间只有 claim 和 launch 改了同样两份文件：`.trellis/spec/server/collaboration-knowledge-execution.md`、`docs/collaboration-knowledge-runtime.md`。`git merge-tree` 合并两条分道的当前 tip，没有冲突。
- 共享分支自 `6aed33aad` 起新增 20 个提交，改了 17 个代码和 spec 文件；-a8 的 `codex/runtime-audit-fixes-20261101` 改了 12 个文件。两者与四条分道都没有文件重叠。
- 修复分支和四条分道都不含隐私改写前的 `5059db461`。

## 复核留下的问题

复核结论都不是阻塞，workflow 不会处理这些问题。按 PRD 的"全部修复、测试通过"，第一组建议在合入前处理，每项先写失败的测试，由 -54 实施（清单已发给 -54）；第二组需要负责人决定。

建议合入前处理：

1. 末尾的取值参数会吞掉托管的 `--settings`（claim 复核，medium；实现者也列为未解决）。custom_args 或默认参数以 `--model`、`--append-system-prompt` 等需要取值的参数结尾时，CLI 会把 daemon 追加的 `--settings` 当成这个参数的值，托管路径变成位置参数，关闭继承 skill 的设置因此失效。追加 `--settings` 的逻辑早于知识交付（`765ab186a^` 已有）。H1 之前，这类参数会被白名单顺带拒绝；H1 之后它们按普通参数执行，问题因此暴露。修法在"claim 时拒绝末尾的取值参数"和"改为单个 `--settings=<path>` token"之间选择。先用本机 CLI 确认实际解析行为，只看 `--help`，不调用 API。
2. M5：面板自己内存里那次丢失的提议，恢复后必须关闭表单，但没有测试覆盖。一旦回退，草稿会保持打开，再点一次提交会产生重复的候选（web 复核，medium）。
3. L8：分段错误归因只固定了一部分。读不到来源时应报 `sourceChanged`，prepare 时失去授权时的归因，都还没有测试（web 复核，medium）。
4. H2：daemon 端新增的三处接受检查，包括 `validateKnowledgeClaim` 中的来源条件 `task.droppedPriorSessionID == d.Session.NativeSessionID`，删掉以后没有测试会失败（launch 复核，low，两份复核都提到）。
5. 文档：
   - 操作文档没有写"旧 daemon 遇到新 API 时，每一轮仍报 `knowledge_context_changed`"；
   - `docs/collaboration-knowledge-runtime.md:69` 把拒绝范围写得比代码宽：实际只作用于 v1 内置 Claude invocation，而且漏了"取值拼成子命令名时也会拒绝"；
   - spec 第 18 行的 `resume_unavailable` 单元格没有写明可以重放；
   - 一处测试注释已经过时（`{"--", "--continue"}` 那一行）。
6. L1：`ApproveKnowledgeCandidate` 返回的任何 `ErrNoRows` 现在都会变成合成的 40001 重试。非过期原因造成的未命中，3 次重试后变成 503，以前是 404。另外，spec 写的"重试一定得到 409"只在重试预算还有剩余时成立（write 复核，low）。
7. M5：`recover()` 在点击时用 `current?.value.items` 判断候选是否属于本面板。如果点击时列表因刷新被擦除，或者面板正停在后续页，本面板自己的候选会被当成别处的（web 复核，low）。

需要负责人决定（来自设计阶段，实现时按设计的默认处理）：

- 长 chat 下每次 claim 的 O(N) 历史读取成本；owner 命名空间里一次私有披露会让该 owner 跨工作空间永久拒绝（H3）。
- 旧 daemon（`inv == nil`）custom_args 里的 `--continue`、`--resume` 不在第一步拒绝。spec 第 91 行要求保持旧 daemon 的行为（H1）。
- 子命令黑名单只覆盖 2.1.316 帮助里列出的子命令，加上 `remote-control`。H1 之前所有裸操作数都会被拒绝（H1-followup 复核，low）。
- 1.5s 的服务端忙等预算；`ObserveTaskKnowledgeLaunch` 是否也加重试（M2）。
- observation 写入方是否加固（H2 设计的待决问题 1）。
- 审批预算检查仍用应用时钟（L1 实现者提出）。

## 与本任务无关、但挡住验证门的问题

- `packages/views/admin/executions/list-controls.test.tsx` 中 "keeps current-state scope only for a legal queued filter (2026-10-01T00:00:00Z)" 从 2026-11-01 起必然失败：它写死 `time_from=2026-10-01`、不设 `time_to`，而 `validateAdminFilters` 以当前时间为终点，31 天窗口一过就返回 `rangeTooLong`，不再跳转。所有分支都有这个问题，会让 web 分道完成门和 `make check` 失败。修复分支上的 `9c056ad68` 把这个用例的 `Date` 固定为 2026-10-02，计时器保持真实。修复前 1 个失败、7 个通过，修复后 8 个全部通过。证据在 `research/evidence/admin-list-controls-clock-{red,green}.txt`。
- `server/internal/service/empty_claim_cache.go` 在基线上就不符合 gofmt，本任务不改。
- launch 分道第二次完整运行 `internal/daemon/...` 时，3 个 repocache 测试报 "stop process tree: operation not permitted"；第一次运行和单独重跑都通过。不在已知 flake 列表里，集成后的完整运行中再观察。

## 下一步

1. 等 workflow 跑完：M1 → H3+M3，M2+L6+L7，L3–L5，L9+L12 → L10+L11+L13，然后是各分道完成门和整条分道的复核。
2. -54：按 claim → launch → write → web 合入修复分支，再合入共享分支当前的 tip；sqlc 零漂移；所有包各用一份新克隆库完整运行，并发相关的包加 `-race`；core、views 的 typecheck、lint、test。
3. -54：处理上面"建议合入前处理"的各项，每项先写失败的测试。
4. -fe：合入前复核。make check 和门控 e2e 的时间要与真实模型窗口协调。
5. -54：清理本任务创建的库：`kfix_*` 克隆库，包括 `kfix_write_svc_r2`；模板库在最后删除。
6. -5c：每批分道结果出来后，把实际结论和偏离写进 `design.md` 和本文件。
