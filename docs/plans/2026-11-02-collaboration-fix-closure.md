# 首轮协作修复交付闭环推进计划

> 执行方式：本会话按 Trellis implement/check 原生子代理逐项实施和独立复核；沿用现有任务，不创建重复任务。

**Goal:** 在同一冻结候选上取得完整验证证据，续作原 E0–E6 实验，并准确更新首轮任务及父任务的验收状态。

**Architecture:** 产品候选沿用已合入的协作实现。确定性验证使用独立 worktree、任务专用数据库和私有启动配置；原模型实验继续使用原 baseline/after 源码和原框架，两个证据系列分别记录。

**Tech Stack:** Go、sqlc、PostgreSQL、pnpm/Turborepo、Next.js、Electron、Playwright、Trellis。

## 当前事实与改动边界

- 集成分支：`codex/cross-workspace-collaboration`，接管时 HEAD `7af9ab990`。其他任务的未提交改动保留。
- 已准备的独立候选：`/Volumes/artisan/code/2026/mc-collaboration-final4-20261102`，HEAD `64a9994ec549e8dff73ab14456dc9a9f4b63f7c7`，工作区干净。产品 source ID `9f72fc214b8e81f6fc3c1969be5e3304b7f158f60a6f7305f1531d55574d80ee`。
- 第三次完整检查失败：296 pass / 47 fail / 89 skip / 7 未运行。后续定向修复的成功不代替完整复验。
- 原实验：E3 after trial 1 无效；下一入口 trial 2，剩余调用上限 14。E1/E2a 无效旧槽位保持原编号，不补成成功样本。
- 最小行为缺口是验证启动配置和测试入口是否真实覆盖同一候选，以及剩余实验是否按原登记生成可核对的结果。先修改任务计划/证据；如私有启动器或适配器有阻塞缺陷，修复其来源并验证，产品代码仅在实际复验确认回归后才另列最小补丁。
- 本轮预期写入：本计划、任务 `implement.md` / `task.json` / context manifests / `research/continue-20261102/`，以及必要的任务私有验证脚本。其他任务的工作区、业务默认数据库、历史原始证据、原生产品断言及生产权限规则均不在改动范围内。

## 1. 同步续作入口与归属

- [x] 更新现有 `implement.md`：E3 after trial 2、剩余 14 次；保留历史段落。
- [x] 用 `python3 .trellis/scripts/task.py start .trellis/tasks/10-30-collaboration-fix-closure` 激活现有 in_progress 任务。
- [x] 保存接管 HEAD、dirty 文件、候选指纹、工具版本、调用预算和当前进程的只读记录。
- [x] 将当前数据库隔离、E2E 环境、最新验收记录纳入 implement/check manifests。

## 2. 解除完整验证前置阻塞

- [x] 独立复核 Go/pnpm 适配器及最终启动器，核对哈希、非测试转发、`-c` 解析、递归测试分配记录、原 Playwright 断言与 Agent CLI guard；final-v9复核0阻塞。
- [x] 运行任务私有fake契约，final-v9全部86项通过，无真实账号或业务数据库访问；保留原v5/v6/v8红证据。
- [x] 检查并修复API连接池、HOME/cache与组织publisher配置对普通Go测试的污染；失败真实复现后做最小修复，v9技能catalog原断言7/7通过。
- [x] 独立复核 final639 RKI6 normal/race 的原 JSONL、private public/trace、来源及统计。结论 pass-with-notes、0 阻塞；fake/sim、H6 manual_review_required 与旧失败保持原分类。既有付费入口继续关闭；本轮不启动新 RK1 系列。
- [x] 独立复核无阻塞后生成精确哈希放行记录。历史v6为Go `8963ebcc…dd9a9`/launcher `3938151d…f3e34`、83fake及交叉复核；当前v9为Go `70ca24bb…34fce`/launcher `68dfc4c4…bf475`、86fake及实际catalog7/7pass，final-v9独立pass-with-notes/0阻塞；各版本放行和运行证据分别保存。

## 3. 重跑完整确定性验证

- [x] 核对 candidate clean、source ID、已安装工具/依赖、私有 env、Git 树外 0700 TMPDIR、renderer URL 和静态构件哈希。
- [x] 只读核对共享PostgreSQL连接余量；当前Go包用新模板克隆/独立HOME，API10/1，Redis16422/renderer19845单一owner，各完整run分开配置和证据。
- [ ] 由统筹串行执行冻结候选的私有启动器，实际调用仓库 `make check`；保存命令、原日志、阶段状态、所有 Go 包 JSONL、Playwright 原 JSON 报告和退出码。
- [ ] 若失败，保留原 run；先从具体失败和源码确认原因。测试基建问题交 implement 子代理最小修复，行为回归补正确层的失败测试；独立复核后登记新的完整 run，不覆盖旧结果、不拼接通过项。
- [ ] 验证覆盖 migration/sqlc/build/vet、静态检查、TS/Go 测试、production Web 及相关原生 Electron。条件性 skip 和未运行项逐项保留，不写成通过。
- [ ] 默认整套结束后，在同一候选的独立password部署补 C3 Web/native/offline 四项及runtime handoff三项，要求实际7pass、0skip/retry。默认legacy整套会跳过它们，不能凭整套exit0关闭此门。
- [x] 核对当前sqlc版本证据：source `45efa093c` 的 `sqlc@v1.31.1 generate` exit0/无漂移，配置、迁移、queries、generated到候选64a全部0diff，复用该证据。746 SQL及具名回归未变，整套需确认该回归和prefix/direction测试实际pass。

## 4. 续作原冻结实验

- [x] 核对授权原文及 14 次预算，与 RK1 25 次分账；原源码 baseline `c58f79874cfe2e6318e3dce067d7b6bac24e943f` / after `835e7b046c6841e0945a59326bc4ddcbae4a0df7`、历史框架 SHA-256 `f6f62e55888fa2d3e733ff12dec6054e07f877828363ad0d503ada6db07aa0bd` 保留。
- [x] 对 OEP-1/OEP-2 做追加式登记及最小停止规则补丁。两条新隔离目录保持原产品 HEAD，统一 successor 框架 `ff2ac795d7ec3a6ca9664276200fe61f2526fe40c496ad7bc137b42099bd08c0`；两 arm 各32项 fake 通过，独立复核 pass-with-notes、0 阻塞。原 prompt、判分、模型、超时、旧试次和原三文件框架保留。
- [ ] 在两条 frozen source 上验证 fake entrypoint、固定 CLI/model/provenance、正式 E0 记录、私有证据目录和独占执行窗口；不自动更换 CLI/model、超时、评分或原场景。
- [ ] 依原登记串行执行 E3 after trial 2/3，再 E4 → E5 → E6，逐场景核销实际启动次数与剩余上限。
- [ ] 每一场景独立核对原 prompt/HTTP/tool/outcome/lifecycle/proof 与语义标准；首个不完整结果立即停止其后真实调用，记录产品/模型/环境类别及可证实范围。
- [ ] 如原账号、固定 CLI/model 或独占窗口不具备，保存具体阻塞及准备结果，不消费其他系列预算、不用 fake 代替真实结果。

## 5. 核销交付与收尾

- [ ] check 子代理复核本轮全范围差异和证据；主会话更新问题 → 修复 → 验证 → 限制映射。
- [ ] 更新 task metadata、首轮验证报告以及父任务 AC0/AC3 关联状态；其他子任务/平台/账号验收不随本任务自动关闭。
- [ ] 仅提交本任务经过复核的文件，使用 Lore 决策记录。用户已授权本地提交/复核后的本地合入；公开推送、外部 CI、部署、发布不在本次范围内。
- [ ] 仅在完整验证通过且原实验按登记完成、必要语义结论审查结束后归档。仍有环境阻塞或未运行门时保持 in_progress，并明确下一步入口。

## 验收证据

每次 run 单独记录 source/framework/launcher/CLI/config/artifact 指纹、所用任务环境、实际命令、开始/结束、退出码、具名 pass/fail/skip/未运行项。凭据留在私有配置，报告只写配置哈希、键名或无凭据的身份信息。历史失败继续保留。

## 本轮执行补记

- 用户已授权跨会话协调。runtime 会话同意由首轮会话单独执行 final4 完整检查，共用同源证据；其 final5 准备保留，未启动重复全套/API/Redis/Provider。知识会话只核销自己的独立验收项。
- 独立v5预审的AV5-P1/P2/P3及后续Make/package/flag/global--问题已在历史v6修复并复核；实际Go publisher配置污染经当前v9最小修复、86fake与7/7原断言验证，final-v9独立复核0阻塞。各旧版本、changes-required和失败证据保留。
- 本轮验证独占 Redis 容器 `multica-fix-closure-20261102-r4`（16422）和 renderer（19845）；旧19844服务归属别的会话，不作本轮可写资源。构件哈希与已登记产品一致。
- 原实验产品源码与历史三文件框架一致，注册 CLI 仍为2.1.316。原 E5 harness 的 canary失败停止规则已用独立 successor 修复并复核，OEP-1/OEP-2 核销；实际 E3–E6 调用仍等待完整检查和独占窗口。
- v6完整检查2026-11-02 14:47–15:02 UTC，PID91222，exit2/3命名失败保留。普通Go的publisher配置污染经真实7项红绿与v9独立86fake复核修复。
- v9新完整run15:36 UTC启动，PID20588；15项静态/13,082项TS再次通过，cmd/server完整包735pass/0fail/6skip，其余Go/race执行中。日志`make-check-v9.log`/JSON及每包新克隆分开保留，最终整套状态仍待结束。
