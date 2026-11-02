# 可靠上下文交接与召回：推进计划（2026-11-02）

状态：执行中。用户要求“请列一个推进计划，根据计划继续”，本轮恢复已有 `in_progress` 任务，不创建新任务。开始时间：2026-11-02 12:48 UTC。

## 最新执行入口（2026-11-03 本地）

v9 实际完整检查 exit 2：静态 15/15、TS 13,082、Go 82 包/20,852 pass/0 fail/22 skip；浏览器 334 pass/16 fail/89 skip，零重试。原 16 项已定向修复和复验，尚不能替代下一轮完整检查。

v10 选定干净候选 `118fc53ea8eb8ebc53b4f9c1131e3e7910eacce9`，工作树 `/Volumes/artisan/code/2026/mc-collaboration-e2e-repair-20261103`。本任务重算 7,960 个产品文件，指纹仍为 `9f72fc214b8e81f6fc3c1969be5e3304b7f158f60a6f7305f1531d55574d80ee`；17 文件 runtime 框架 hash 未变，当前 RKI6 normal/race 各 62 pass 的独立证据可按这个字节范围复用。绑定准备见 [v10 记录](research/continuation-20261102/coordinator/v10/binding-preparation.json)。这不是最终付费登记。

原执行会话连续 Azure resource 错误后，知识协调会话 `01a19c0e-5cc4-77e0-b2d3-bd2842d39899` 正恢复同一个 v10 唯一执行；它已完成 32/32 离线预检、同源 renderer/preload 安装核对，并收尾独立入口审查。v10 已于2026-11-02 19:56:51UTC启动，PID92594；独立入口复核pass-with-notes、0阻塞，实际结果待返回。见 [启动观察](research/continuation-20261102/coordinator/v10/run-start-observation.json)，先前未启动快照保留。恢复状态见 `../11-01-collaboration-knowledge-audit-fixes/research/continuation-20261103/v10-execution-recovery.json`。Redis 16422、renderer 19845 和失败数据库继续由该执行者保管。

剩余顺序：v10 完整通过 → 同候选密码模式 C3 四项及 runtime 三项实际验收 → 最终源码/框架/CLI/配置/构件登记 → 原冻结系列余 14 次与 runtime25 分别在协调串行窗口执行 → 必要 AC 与规格、交付记录。恢复执行者目前只承接 v10；后继窗口需明确交接，其他会话不能提前启动。runtime25 消耗 0；原系列下一 E3 after trial 2。已有实验预算授权继续有效，外部 push/CI/部署/发布仍未授权。

主分支随后合入了source-task搜索与创建/接收人修复（`4ba3a868`、`55bb7802`），目前`c6da153`与冻结v10是同父分支。12个生产路径变化，runtime handoff/checkpoint/recall/claim/provider实现未变；但C3与offline的接收人控件有重叠，sqlc查询/生成结果也已变化。v10证据只对118fc成立；最终如合入这批变化，需补对应创建/source-search/接收人路径复验、合流118fc的两处E2E修复并重新绑定实际源码/构件。详见 [主分支影响复核](research/continuation-20261102/coordinator/v10/mainline-impact-review.json)，不把新旧版本的部分结果拼成整轮通过。

## 目标与本轮初始基线

关闭本子任务必要的最终验收门，保留原始失败、未运行项和父任务的独立发布门。需求仍以 `prd.md` / `design.md` 为准。本计划补充 `implement.md`，不改验收标准。

- 集成入口：`7af9ab990`；已准备的完整验收工作树为 `/Volumes/artisan/code/2026/mc-collaboration-final4-20261102`，HEAD `64a9994ec`，初查工作树干净。
- 产品源码与 `639bf6ef8` 等价，登记指纹为 `9f72fc214b8e81f6fc3c1969be5e3304b7f158f60a6f7305f1531d55574d80ee`。开始前重新验证实际文件、脚本和环境。
- 先前完整验收 exit 2；其后的局部修复与通过结果不能替代新一轮完整验收。
- v5 脚本和当前 RKI6 normal/race（各 62 pass）的结果独立复核未完成；之前的 429 不作为复核结论。
- RK1 runtime 已登记预算 25 次，消耗 0；原冻结系列的剩余 14 次独立保留。真实运行前核对最终绑定、停止规则、串行窗口和现有授权范围。
- 主工作区已有其他会话的未提交改动。本轮不将这些改动带入固定候选，不撤销或提交其他会话的改动。

## 推进顺序与责任

| 步骤 | 责任与写入范围 | 完成证据 |
| --- | --- | --- |
| 1. 恢复入口与隔离检查 | 主会话；本计划及 `research/continuation-20261102/` 下协调记录 | 当前 HEAD/文件指纹、工作树状态、实际环境与服务归属；保留旧失败库与日志 |
| 2a. v5 验收脚本独立复核 | 原生 `trellis-check` 子代理；仅 `research/continuation-20261102/adapter-review/` | 检查原 v4 问题、v5 实现/11 项契约、非测试转发、按包新库和临时目录隔离；给出绑定脚本 SHA 的 verdict 和未关闭问题 |
| 2b. 当前 RKI6 结果独立复核 | 另一原生 `trellis-check` 子代理；仅 `research/continuation-20261102/rki6-review/` | 重算源码/框架/原始记录哈希；核对两轮 62 项、19 类夹具、22 个公开样本、负向判据、实际 fake 启动与真实调用 0 |
| 3. 新一轮完整候选验收 | 首轮闭环会话 `01a19c0d-cce1-73e3-99ee-2c3c9e9239ba` 单独执行 final4；本会话复核证据、维护 runtime 子任务 | 2a/2b 与实际选定 v6 无阻塞项后签发绑定脚本 SHA 的 clearance；保存真实退出码、命令、具名测试、平台/环境证据，共享这一完整运行 |
| 4. 失败定位与必要修复 | 按具体失败派 `trellis-implement`，分配独立文件/工作树；再独立复核 | 保留失败；分清配置/容量/夹具与产品问题；改动后登记新候选并完整重跑必要验收，不拼接旧结果或放宽断言 |
| 5. 真实模型启动门 | 主会话；本任务实验登记与台账 | 固定最终源码、框架、CLI、配置与证据；确认 runtime 25 次授权和现有外部操作约束；按登记顺序串行执行获准范围，首次不完整即停并保留结果 |
| 6. 状态与交付记录 | 主会话；本子任务进度/状态与父验收矩阵的对应项 | 全部门禁具备证据才标完成；父任务 Windows/升级/账号等范围保持独立；提交仅包含本轮拥有的文件并采用 Lore 协议 |

## 执行约束

1. 依赖步骤串行。2a/2b 仅审阅与无数据库契约验证，可并行；共享 PostgreSQL 和原生 GUI 的实际运行保持单一执行 owner。
2. 不运行未登记的真实智能体命令。普通验收经现有 agent-CLI guard，真实 Provider 调用另计已批准预算。
3. 测试必须使用记录中明确分配的环境。新一轮 DB-backed 包使用各自新建且迁移的任务库；不能复用失败全套数据库、清理保留历史或更改共享数据库限额。
4. API 连接池上限 10、下限 1，仅用于任务 API；测试进程不继承会改变默认值测试的键。开跑前检查连接余量，不能终止其他会话连接或服务。
5. 不更改既有业务默认关闭的知识开关、模型判分、断言或超时以求通过。所有修复须有具体失败与根因证据。
6. 本轮修改任务规划/验收记录时保留历史结论。不存在或不完整的结果只能记为未运行/待验，不计通过。

## 进度

- [x] 已恢复原任务，现有 implement/check manifests 验证通过。
- [x] 已确认准备候选工作树干净；主工作区改动另行保留。
- [x] v5 脚本独立复核：changes-required；AV5-1 API 参数污染与 AV5-2 Shell 环境执行两项阻塞，原版本保留。
- [x] RKI6 当前结果独立复核：pass-with-notes，0 阻塞；1,794 项只读核查通过，normal/race 各 62 pass。
- [x] 本会话 v6 修复：17 项隔离、11 项启动器契约，原有 7 项 dbstats 纯单测、6 项 Python 编译通过；v5 的 4 项真实配置单测失败作为红证据保留。
- [x] 本会话 v6 独立复核：Go/pnpm pass；launcher changes-required，AV6-1 Make include/export 边界仍开放。17/11 契约和既有 7 项纯 Go 红/绿证据已复核。
- [x] 实际选定的跨会话 v6 首次独立复核：Go14/launcher15 fake pass，AV5-1/N1 关闭；launcher changes-required，AV6-CROSS-1 证明 `${shell ...}` 仍由 Make 执行。
- [x] 唯一执行 owner 完成最终修复与冻结；原 cross reviewer 一次复验83项纯/fake契约全部通过，pass-with-notes、0阻塞；Make/参数/包许可/实际hash/HOME问题关闭。见 `research/continuation-20261102/v6-cross-review/recheck/review.json` 和 `clearance.json`。
- [ ] 新一轮完整候选验收。
- [ ] 必要修复与复核（由实际失败决定）。
- [ ] 最终源码/框架/CLI/环境绑定与真实模型启动门。
- [ ] 真实模型必要场景验收。
- [ ] 完成状态、父矩阵、规格与交付记录。

## 本轮入口更新

- 新整轮候选：`/Volumes/artisan/code/2026/mc-collaboration-final5-20261102`，冻结 HEAD `7af9ab9905c98c77bb091223ebe687c639475bb4`；7,960 文件产品指纹与 `639bf6ef8` 相同，工作树产品干净。
- 旧准备环境文件的实际 hash 与历史登记不同，新副本重新冻结 `check-source-input.env` 并记录实际 hash；旧登记不覆盖。CLI 与 renderer 的实际 hash 均匹配原同源证据。
- 本轮私有临时目录为 `/private/tmp/multica-final-check5-lre1jag3`，0700、owner 501，位于所有 Git 工作树之外。
- 截至环境准备时，PostgreSQL 总连接 39/100。实际开跑前重新检查余量；保留其他会话的 API/连接。
- 当前假模型结果复核见 `research/continuation-20261102/rki6-review/review.json`；v5 阻塞证据见 `research/continuation-20261102/adapter-review/review.json`。

## 跨会话单次验收

2026-11-02 用户授权的首轮闭环协调会话联系本会话，双方确认避免 final4/final5 重复完整运行。

- 单一完整验收 owner：`01a19c0d-cce1-73e3-99ee-2c3c9e9239ba`，使用 final4 / `64a9994ec`。它维护首轮任务及父矩阵相关 AC0/AC3；本会话维护 runtime 子任务。
- 实际选定脚本为 `.omx/private/collaboration-resume-20261101/adapter-v6/check-bin/{go,pnpm}` 和 final4 `.omx/private/final-candidate/run-make-check-v6.py`。本会话另建的 `resume-20261102` 版本保留独立红/绿证据，不混入它的执行绑定。
- 选定版本的交叉复核写入 `research/continuation-20261102/v6-cross-review/`，按实际文件 hash 签发结论；完整运行 ID/PID/结果由唯一 owner 提供后再登记到本任务。
- 环境文件、Redis 16422、renderer 19845 由唯一 owner 管理。本会话不启动重复服务、测试或 flush Redis。final5 未执行完整 check、没有 API/Redis/Provider 进程；其 renderer 启动在检测到占用后拒绝，未停止任何其他进程。
- final5 的副本、CLI、renderer 和私有临时目录保留为准备证据；它们不代表实际验收已运行。
- 原冻结实验 14 次由首轮会话续跑，RK1 runtime 25 次仍独立保留；真实模型必须在完整门禁通过及协调的串行窗口中运行，不能并发挪用预算。
- 当前选定版本 Go SHA `b8c5d5adbece4f781a00a230b4f6ffbbfed195f4e44826d53997d72b3f160079`、launcher SHA `7e2592d1b6e0a795ce9078f4c97d04255f635d392ff7e16a154309aa87695516` 的 cross review 为 changes-required。新修复必须重新登记，不覆盖本轮失败/探针。
- 环境加载新合同的规范草案保存在 `research/continuation-20261102/coordinator/verification-env-spec-note.md`；共享 spec 的更新交由唯一执行 owner 协调，避免覆盖其他会话的既有改动。
- 唯一 owner 已一次性冻结实际最终字节：Go `8963ebcc…4dd9a9`、pnpm `7a416eb6…ff48`、launcher `3938151d…1f3e34`、source env `99475d2e…2dfe2`；修复方83项fake和6项语法检查通过。原 cross reviewer 正只复验这些最终字节，旧变异/失败保留。两份最终复核无阻塞后才执行完整验收；完整验收之后补密码模式C3四项及runtime三项，再安排原14与runtime25的独立真实窗口。
- 新实际完整运行已于14:47UTC启动、15:02UTC结束，真实exit2。静态15项与TS13,082项全部通过；Go `cmd/server` 两个技能模板用例GET目录503（3个fail事件包含父测试），其余Go/浏览器尚未完成。日志与失败包数据库保留；唯一owner排查catalog配置/路径后登记新完整运行，不拼接旧结果。实际记录为 final4 `.omx/private/final-candidate/make-check-v6.json`。
- v9 新完整运行已于15:36UTC启动，PID20588；native复核0阻塞、86fake与实际7项catalog回归通过，main只读2行/路径diff核对通过。修复仅在ordinary mode1 Go package child去掉API-only `MULTICA_RESOURCE_PUBLISH_DIR`，mode0/test-owned/API/non-test/compile保留；全部v9记录/home/registry路径独立，旧v6失败保留。当前仍running，不计完整通过，记录为 final4 `.omx/private/final-candidate/make-check-v9.json`。
