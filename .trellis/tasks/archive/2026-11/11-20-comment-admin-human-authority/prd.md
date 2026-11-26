# 评论管理权限：机器身份不得借用管理员权限

状态：2026-11-25 用户授权纳入 main 验收修复；等待父任务内部规划审查通过后实施。2026-11-20 原规划授权及证据保留。来源：[安全漏洞修复小队 v2](../11-18-security-remediation-squad-v2/prd.md)，子任务一 design.md 14.2 平台项 A。

## Goal

禁止机器凭据通过其所有者的工作空间管理员角色修改或删除他人评论，同时保留合法人类管理员操作及智能体自身允许的评论操作。

## 已核实事实

在基线 efdb597d6，`server/internal/handler/comment.go` 的 UpdateComment 与 DeleteComment 均以 resolveActor 判断作者，以 workspaceMember 的 owner/admin 判断管理员。管理员分支未单独要求人类凭据。编辑保留 author_type/author_id 并递增 revision，故仅凭原作者字段不能证明放行内容仍由本人写下。现有 RequireHumanActor 与 actor guard 是复用入口，实施前仍须追踪真实鉴权中间件和全部机器凭据来源，不能仅凭传入 actor_type 判断。

## Requirements

- 人类 JWT/合法人类 PAT：作者可按现有规则操作本人评论；当前工作空间 owner/admin 可按现有产品规则管理他人评论。
- 普通人类成员不得管理他人评论；跨工作空间、已失去成员资格、已撤销凭据保持拒绝。
- 所有机器凭据（任务、守护进程、智能体凭据及仓库实际支持的其他机器来源）不得因绑定的人是 owner/admin 而获得管理他人评论的权限。
- 智能体本人评论的修改/删除仍受当前运行、来源、受限能力和相关产品规则约束；不得把整个接口改成人类专用而破坏允许的自身操作。
- 不能靠伪造 actor headers、作者显示名、任务发起人或所有者身份洗成人类权限。
- 拒绝请求不得改变内容、附件、revision、删除状态、通知、执行队列或审计记录；合法修改继续保留作者归属，记录真实操作者（现有审计能力不足时设计中明确差距）。
- 安全小队的严格 revision 放行规则不随此修复放宽：人类管理员仍可以合法改写评论。

## Acceptance Criteria

- [x] 完整凭据 × 角色 × 本人/他人 × 修改/删除矩阵，经真实路由和鉴权层验证。
- [x] owner/admin 所属机器对他人评论编辑、仅附件修改、删除均拒绝；无副作用。
- [x] 合法人类 owner/admin 可管理他人评论，作者字段不变、revision 与冲突语义正确。
- [x] 普通成员、自有智能体评论、受限审查员、过期/终结 task、跨工作空间回归通过。
- [x] 绕过路由直接调用处理器时不能借用管理员角色；复用既有身份判据。
- [x] 新鲜任务测试库上整包 handler/cmd-server、必要服务层与定向 race、go vet 通过，普通测试有 CLI 防护。

## 范围与交付

只修改评论写权限、必要鉴权复用、测试及规范；不变更安全小队模板、不扩展评论可见性、不改数据库关系。独立分支交付。此任务不能被“指令已禁止 leader 编辑评论”替代。

## 2026-11-25 acceptance-fixes execution authorization
Parent: ../11-25-main-post-merge-acceptance-fixes. Work exclusively in the assigned worktree. No commits, pushes, deploys, new dependencies, real Agent CLI/model execution, changed deadlines/retries/skips, or altered original evidence. New public evidence: artifacts/qa/main-acceptance-fixes-20261125/comment; private resources: .omx/private/acceptance-fixes/comment.

Generic upload comment_id binding is a separate product contract investigation; do not infer comment-management authorization or change it without a confirmed requirement. Full handler and relevant cmd/server normal/race plus real HTTP credential regressions are required, on unique owned databases/HOME/ports.
