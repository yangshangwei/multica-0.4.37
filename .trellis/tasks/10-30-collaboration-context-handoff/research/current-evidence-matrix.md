# 当前进度证据核对表

固定审计对象：/Volumes/artisan/code/2026/mc-collab-integ，HEAD 4b0cbfde7a2a03142e70ceb91294ec8d8f1ff074。集成 worktree 只有未跟踪的 .integ-check.sh；主工作区的 52 项未提交改动未被复制、重置或覆盖。原始材料快照、SHA-256 和来源时间见 research/raw/source-manifest.json。

| 项目 | 当前结论 | 分类 | 证据 | 还缺什么 |
| --- | --- | --- | --- | --- |
| 8 个修复提交 | 已全部在集成 HEAD，范围从 7bb077987 到 4b0cbfde7，基线为 a3475f20a；主工作区另有并行提交 85b98c3bb（6 个 core/frontend 文件，含自身测试）；它不在集成 HEAD | 已实现，最终集成未闭环 | raw/integration-8-commits.txt、raw/integration-8-commits.patch、git reflog | 最终 branch 的单一提交链、全栈验证 |
| 集成旧检查 | vet/build/tagged-vet/daemon/cmd/service/handler 日志均 exit 0 | 证据有效但版本边界有限 | mc-collab-integ/.integ-check.log 快照；日志没有记录 HEAD，时间早于最后整理 | 在最终 HEAD 重跑并记录 HEAD、命令、数据库、测试名 |
| A1 附件填充 | 修复了历史超过固定 32 的阻塞；仍先取 32 再按 runtime limits 整体拒绝，未引用文件可阻塞纯文字；引用超限应拒绝 | 实现偏差 | review-a-experiments.md；server/internal/service/collaboration_runtime_preparation.go:25-88,99-151,235-239,358-362；.trellis/spec/server/collaboration-preparation.md:18-32,58-62 | 以实际 runtime 的 FileCount/PerFile/Total/space 逐个补位，新增预算矩阵 |
| A2 评论转义 | 读取预算覆盖 JSON 最坏转义，解码后仍保留 64 KiB 内容限制 | 已修；链路证据不足 | d84f217a9；server/internal/handler/collaboration_runtime_business.go:14-35；collaboration_runtime_comment_size_test.go | 服务层 64 KiB+1 不落库；最终 HEAD 重跑 |
| A3 本地链接 | 协作 CLI 在 HTTP 前使用无普通 task 身份门槛的拒绝检查 | 已修 | b34701756；server/cmd/multica/cmd_collaboration.go:91-95,143；测试名写入 raw/role-cli-current-head.txt | 最终 HEAD 同测试，保留普通任务路径不变 |
| B1 迁移编号 | 两个 745 不会被迁移器漏执行，但违反仓库唯一数字前缀规则 | 实现/集成阻塞，已复现 | raw/migration-prefix-lint-current-head.txt；server/internal/migrations/migrations_lint_test.go:79-98；server/migrations/745_*；B 报告 | 未发布则换到下一个空闲前缀；空库、旧库、重跑、down/up |
| B2 重新授权 | 0ef4c7858 的权限路径保持 owner/participant/resource/binding/config/hold/explicit Resume；旧 NULL boundary、runtime visibility、service tier 等仍 Switch-only，但 UI 泛化为 owner 可重新确认 | 文案/范围偏差，无已证实越权 | B 报告；collaboration_authority.go:198-239,537-646；collaboration_control.go:390-482；labels.ts:60 | 按 can_consent/原因显示动作；Web/Desktop 真实恢复证据 |
| C1 成员提示 | host prompt 增加服务端 role/step，并提示 issue get 查 delegation comment | 窄修复通过；正文仍依赖模型主动召回 | C 报告；host_runner.go:496-526；host_prompt_role_test.go | 正式 E3 前后各 3 次；结构化派活正文/完成结果属后续 |
| C2 冒烟白名单 | memory/base 精确业务 token 路由已加入 authorizer；主定向测试 PASS | 确定性修复通过，真实入口未验证 | raw/authorizer-current-head.txt；host_agentintegration_authorizer_test.go | 运行精确 TestHostProviderAccountSmoke；记录 provider/model/CLI/launch/proof |
| C3 补充请求 | 表单已有正文、请求人、fallback return status 断言；服务/e2e 已验证提交链 | 实现/组件断言通过；接收方浏览器证据欠缺 | C 报告；workflow.test.tsx:147-160；collaboration-handoff-chain-web.spec.ts | 接收方页面提交前可见性断言；正常 status select 值；Desktop 记录 |
| E0 | 修复前历史记述显示 403 未启动，但 /tmp/exp-e0-base.log 已丢失；E0b 不走正式 authorizer | 缺少可审计证据 | raw/experiments-preregistration-2026-10-29.md、manifest present=false；C 报告 | 修复后精确 TestHostProviderAccountSmoke，前置门槛 |
| E1/E2/E4/E5/E6 | 只有预登记、未提交框架和两次排除的超时；没有正式结果 | 缺验证，部分依赖模型自觉 | 实验预登记、raw/host_context_experiment_test.go.txt | 先硬化框架记录与评分，再串行试验 |
| 实验框架 | 默认 trials=1；未可靠记录实际 model、完整 prompt、按序工具请求/响应和失败 verdict；E0b 与正式 smoke 分离 | 证据设计缺陷 | A 报告；未提交实验快照行 53,68-69,89-92,112,129,143,149-240 | 在假 Provider 下验证记录，再消费真实额度 |
| 协作上下文当前边界 | memory 是单份活文档，版本/署名/基线写入/权限已有；AI 写入脱敏；run record、ledger、provider 本机副本并存 | 设计已选，保留/撤权边界未闭合 | archive/2026-10/10-26-collaboration-memory/prd.md C01-C30/CA01-CA18；collaboration_memory*.go/sql；host_ledger.go | 定义 checkpoint、撤权后的可见范围、清除范围 |
| 成员/队长交接 | inputs 有 author/prompt；RuntimeContext 有 roster/dispatch_comment/200 comments；缺 member outcome、reader ids、wakeup reason、final answer/current handler | 设计/实现缺口 | collaboration_squad.go:47-59,146-205；collaboration_squad.sql:42-49；collaboration_squads.md | 结构化 handoff envelope 与 reader/结果投影 |
| 评论召回 | 最新 200、旧到新；无 total/truncated/分页/字节预算/作者显示名/AI 指令权标记 | 实现缺口 | collaboration_squad.sql:42-49、collaboration_squad.go:47-59、access.go:88-95 | before/limit/byte budget + truncation + provenance/trust metadata |
| 权限边界 | runtime token 与人类权限分离；评论作者可能只有 Comment 无 SubmitCommand，却进入窗口；prompt 目前无可信度标签 | 安全/设计缺口 | access.go:88-95；host_runner.go:515；claude.go:740 | 把权限标记作为数据，固定“只执行受权输入”的系统边界 |
| workspace/project 现有承载 | 普通任务已有 Workspace Context/Project Context 注入和 owner/admin 路由；项目更新有 revision/transaction；source context 明确拒绝协作 capture | 复用边界 | runtime_config_sections.go:184-198,414-444；router.go:1890-1892,2298-2304；source_context.go:63,274-280 | 知识候选单独建模，不直接让协作单写生效上下文 |
| 技能层 | skill 内容影响执行面，现有 invalidation/owner consent 机制；直接自动沉淀会扩大执行权限和传播范围 | 后续候选 | .trellis/spec/server/collaboration-switch-and-skills.md；旧 memory PRD C15/C22 | 首轮只论证，需单独授权、review、版本/撤回方案 |


## 2026-10-30 首轮实现更新

- A1、B1、B2、C3 已完成最小收尾；详见子任务 verification-2026-10-30.md。
- Final closure HEAD: 5491d1b22，包含 85b98c3bb、8 个修复、迁移重编号、runtime budget selector、B2/C3 UI/test changes。
- Fresh/reapply migration、sqlc、Go vet/build、DB service/handler collaboration、pnpm lint/typecheck/test 已有原始日志。
- E0–E6 仍是下一门槛；实验框架硬化进行中，真实模型尚未调用。
