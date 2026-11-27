# 初始化整合进度 — 2026-11-27

用户确认执行整合建议。复用当前任务和现有worktree，无新任务。工作区：/Users/artisan/.codex/worktrees/material-repository-initialization/multica-0.4.37；分支codex/material-repository-initialization。

已合入codex/git-engineering-material-workflow（221a6aedf含23d7d0721），合并提交73e187139。仅开发日志索引冲突，保留199/200两个会话，并修复自动合并导致的会话尾部错置。产品代码无文本冲突。工作区已有server/.artifacts未跟踪资料保持不动。

实施边界：将现有纯materialrepo内核接到运行时操作、可信目录读取、项目根级持久化与独立人类审核；加入ready门槛、维护候选和共享UI。复用队列及载体，不拓宽普通PRD文件权限，不新增依赖，不调用真实模型或写远端仓库。

原生Trellis实施代理：backend拥有服务/handler/router/SQL/generated与公共API；runtime拥有协议/daemon/provider读取/materialrepo维护与CLI；frontend拥有core/views/locales/e2e。主会话拥有任务/规范/内置技能/最终集成。先冻结contracts.md，各lane不得并发改同一共享文件。最后派独立全范围检查，验证后本地提交并合回原功能分支；保留原分支未提交父任务文件和其他产物。

## 合同与已识别集成风险

公共API和运行时扩展已冻结于contracts.md；旧描述增加字段使用omitempty保留历史摘要。新material-repository-v1能力阻止旧daemon绕过初始化/规则预读。普通需求继续使用原文件集合，初始化允许服务器catalog内20个明确文件。

复查重点包括：目录读取与回执体积上限一致、无关二进制文件摘要保留、完整规则漂移回执撤销当前发布资格、已采用bundle的维护版本查找、审核记录恢复后的阶段推进、同一候选退回/撤回后重新批准、禁用/切换运行时保留历史。缺失或不完整证据不降级为成功。

## 第一轮联调证据

运行时代理已在新隔离库完成两provider真实队列驱动的确定性链：完整目录检查→20文件初始化候选→选定测试运行时发布→人类维护批准→审核记录同步/合并→核验ready→普通PRD发布与方案人工启动/同issue重试。此为中间通过，独立全范围检查和最终源界面验收仍待完成。

联调发现并修复新增RawMessage经数据库jsonb重排后描述摘要不稳定；新字段按规范JSON规范化，旧v1描述摘要保持原值。规则漂移负回执的持久化/丢确认恢复也在补回归，避免根状态已阻塞但私有待重放回执永久悬挂。

## 独立检查与维护补齐

独立检查发现了重新检查可能替换在审候选、维护错误状态随事务回滚、inspect失败后的只读恢复分支不匹配等问题，正在修复。另发现维护只有同版无差异路径，按本次完整交付范围补入实际bundle升级：保留原1.0.0全部文件字节，新1.0.1仅更新AGENTS规则说明和material.yaml身份。两版本都明确支持，旧仓可原样采用，真实维护变更需独立审核合并；不创建虚构产品版本。最终浏览器矩阵扩为每provider的初始化/维护×Web/Desktop，共8条旅程。

主会话已执行六个非DB包完整默认测试并通过（daemon、CLI、material、materialgit、materialrepo、protocol），私有证据multica-init-default-y847n13c。该次早于最终catalog升级，升级变更由运行时代理新增竞态和集成验证，不冒充最终源全量结果。

## 审查中的最终验证边界

独立审查已补全：保留在审候选不重复PR、无task的调度失败可重试、inspect恢复仍只读、未知/畸形目录不伪造规则漂移、维护失败阻塞持久化、原始plan proof绑定、旧bundle实际采用与升级历史、同绑定设置变更后重新人类批准。后续又发现首次dispatch前重复check/maintenance会替换意图，已加原意图保护，旧测试运行保留为中间证据。最终源验证尚待独立报告，不提前冻结浏览器运行。

## 独立检查完成与生产界面修正

独立全范围检查已完成，无遗留阻塞项。最终服务/普通队列竞态64项通过0跳过，专用Redis两项真实运行；两provider的初始化/普通/人工/漂移与真实1.0.0→1.0.1维护组合链竞态通过，源码稳定。详情见integration-independent-check.md。

首轮GitLab生产界面4条中2条首次初始化通过，2条维护自身也已成功，但后续普通PRD等待人工方案的30秒断言超时；失败截图和DB证明正确状态随后已出现。共享provider fixture累积20文件目录，完整读取成本使后续用例超时。保留原失败，改为每条用例独占新provider仓与新数据库，保留原时限/断言，不跳过维护后普通流程。没有为此更改产品代码。

最终隔离GitLab4/4通过。GitHub初始初始化通过；维护用例在已有catalog采用阶段暴露GitHub测试HTTPS服务按chunk转字符串导致中文UTF-8被截断的问题，产品正确拒绝目录内容冲突。修复仅在测试fixture，加入强制跨字符分块回归；GitHub四条用例在新库/新模拟仓重新跑。GitLab已通过证据与此无关fixture修复明确区分，保留全部原失败。
