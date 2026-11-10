# Research: 续会话完成入口与 D1 主体边界复核

- Query: 复算前序 45 文件清单，核实当前完成事务、私有评估主体、协作/R38 责任及 D4 条件范围；查找 D1 是否已有批准。
- Scope: internal；只读源码研究，仅新增本文件，不改写前序审计或 manifest。
- Date: 2026-11-10
- Worktree: `/Volumes/artisan/code/2026/mc-delivery-completion-enforcement-20261110`
- 本轮读取时间：2026-11-10T14:55:55Z 附近。主会话另行核对 Git HEAD/blob；本研究未执行 Git 操作。

## Findings

### 源码清单复算

直接读取 `completion-source-manifest.json` 的所有路径，以工作区原始字节复算 SHA-256 与长度：

| 项目 | 本轮结果 |
| --- | --- |
| 原清单源码文件 | 45 |
| SHA-256 与字节数均匹配 | 45 |
| 缺失 | 0 |
| 内容或字节数不匹配 | 0 |
| `source-head-verification.json` 中的源码记录 | 45 |
| 两份历史清单的对应哈希不一致 | 0 |

这是工作区字节与前序记录的复核，不是本轮 HEAD blob 检查、运行测试、事务验收或强制门禁交付。原清单仍绑定其原始 `a37bee7cd` 研究基线，不把这次匹配改写为原清单在新 HEAD 上的验收。

本轮另读取了 `server/internal/service/issue_public.go` 的独立内容写入路径：4,320 字节，SHA-256 `aff7e1ac31fcb4576f111b40128aafd3e1cf5f96e12b249e82973c2d792d423a`。这是补充定位；不将原 45 文件清单偷偷扩充或声称该文件经过前序冻结。

### D1 已有事实与未决定部分

1. **评估属于原人类。** `server/internal/handler/delivery_evaluation.go:75` 先用真实认证上下文解析主体并拒绝非 human；`:151` 的查询绑定 `workspace + evaluator_user_id + request_key`，对应 `server/pkg/db/queries/delivery_policy.sql:41`。知道他人的 key 不能读取其评估，更不能把他人身份传给机器完成事务。
2. **恢复不改变原观察。** `delivery_evaluation.go:201` 解码并验证原绑定；`:208` 用锁定的当前 issue/project 版本复用原观察器验证当前权限；`:228` 对曾绑定 execution 丢失返回拒绝；`:232` 返回保存的 observation 和新算 applicability。`:235` 构造响应未授予完成权；`:239` 保留原 hold/unknown、期限及精确制品比较。
3. **提示仍无准入权。** `server/internal/handler/delivery_completion_hint.go:76` 仅允许当前 human，`:112` 接收现有事务进行提示检查，`:161` 按目标 `Category` 决定适用范围，`:175` 仅恢复当前人类的原评估。响应 permission bool 保持 false。非 done、无项目和 off 分支不查提供的 key。
4. **系统完成没有人类验收主体。** `server/internal/service/issue_status_write.go:26` 独立持有写事务；`:31` 记录 `type=system, id=null, user_id=null`，`:85` 实际写 status。`server/internal/handler/github.go:1898` 调该入口；`server/internal/handler/vcs_webhook.go:274` 复用 GitHub 聚合完成。当前没有项目政策锁或完成授权事实。
5. **普通协作已有发起人责任。** `server/internal/service/collaboration_workflow.go:388` 拒绝非人类 principal；`:371` 的 complete/cancel/reopen 权限要求当前 `ControlAI`、member creator 且 `creator_id == actor`。`server/internal/service/collaboration_lifecycle.go:424` 复核精确当前 submission、可用交付、当前承担者、review 类别、无信息请求；`:462` 起的自完成明确记录自身结果与 accepted decision。
6. **R38 是另一项管理事实。** `server/internal/service/admin_collaboration.go:68` 要求已启用的 password/admin 环境、完整人类 JWT/password session、平台管理员及匹配组织；`:171` 要求创建者实际禁用；`:225` 直接调用 `changeLifecycle`，不调用 `prepareCompletion`。`:215` 使用真实管理员创建 collaboration operation。不能把这条路径当作发起人验收、workspace owner/admin 管理政策或已批准 skip。
7. **尚无 D1 批准。** 当前任务 `research/decision-register.md:18` 和 `:26` 明确回答待收；前序提示归档的 `research/enforcement-decisions.md:15` 将跨人类/机器消费列为待决；父任务 `progress.md:80` 仍等待最终人类责任及代办回答。未发现已经实现的共享评估/完成代办授权模型。候选 A/B/C 均未获批准。

若用户后来选择受限代办，还须明确其适用入口；不能借该选择默默扩大既有独立协作的 human/initiator 权限或伪造 R38 actor。D5 仍需收敛。

### 实际 writer 与可复用事务边界

| 入口 | 当前真实写入与边界 |
| --- | --- |
| 普通更新 | `server/internal/handler/issue.go:3571` 解析 actor 并构造事务内复核；`:3771` 开实际写事务；`:3885` UpdateIssue；`:3916` commit。workspace/member/catalog/W01 在 issue 前，project 在 issue 后用 NOWAIT。 |
| move | `server/internal/handler/issue_move.go:17` 白名单允许 status/project；`:141` 调普通 UpdateIssue。前置 anchor 读取不是最终写事务。 |
| batch | `server/internal/handler/issue.go:4961` 逐项处理；`:5118` 每项调用普通更新并各自提交；`:5123` 存在提前 return 或继续下一项；`:5215` 仅返回 updated 计数。不是全批原子。 |
| 创建即 done | `server/internal/service/issue.go:382` CreateInTx；`:429` 持 catalog 锁重解自定义状态；`:443` 起解析父任务并可继承项目；`:465` 锁实际项目；`:516`/`:538` 两个 INSERT 使用传入 status，新 issue ID 此时生成。 |
| 评论转子任务 | `server/internal/handler/source_context.go:667` 手动子任务允许输入状态；`:677` 解析；`:746` 进入 IssueService.Create，再进入 CreateInTx。 |
| GitHub/VCS | `server/internal/service/issue_status_write.go:42` 明确 RC；`:46` 起持 workspace/catalog/W01/iteration/issue 锁；`:63` 拒绝独立协作；`:85` status 写入。完成候选由 `handler/github.go:1898` 的当前 PR 聚合复核决定。 |
| 独立协作 | `server/internal/service/collaboration_lifecycle.go:340` → `prepareCompletion` → `changeLifecycle` → `recordStatusChange:547`/UpdateIssueStatus:552。`collaboration_workflow.go:388` 拥有事务、回执与重放，commit 故障不重试。 |
| R38 | `server/internal/service/admin_collaboration.go:144` 持有独立事务与双账本；`:225` 直接生命周期操作；`:251` commit。必须在该原事务中接入，不能假定经过普通协作的 prepareCompletion。 |

观察/政策/凭据 helper 已接收 `pgx.Tx`，但仍为 handler 私有实现并依赖请求上下文：

- `server/internal/handler/delivery_policy.go:147`、`:164` 使用 project 行锁串行政策修订，包括首个政策行不存在时；`:58` 仍仅接受 off/observe，owner/admin 管理决定已经确定。
- `server/internal/handler/delivery_evidence.go:157` 复用现有观察算法；`:192` 的四个 PR/link 表 SHARE NOWAIT 锁不能被仅锁已有行替代；`:324` 使用数据库时间复核。
- `server/internal/handler/agent_artifact.go:62`、`:114`、`:188` 分别处理原认证主体、当前凭据与用户/成员权限；不得用 header 或外部 hint 结果代替。
- `server/internal/handler/delivery_policy.go:65` 及 `agent_artifact.go:79` 的入口负责 schema readiness、250ms lock timeout 等。借用实际 writer 事务时必须保留等价前置保障。

后续实现只能在明确合同后下沉或窄接口复用这一份观察逻辑；service 不能反向依赖 handler，不能复制第二观察器，也不能把前一事务的 matching 当准入。具体接口或授权模型未在本轮选定。

### D4 条件范围的精确限定

原研究提到项目删除批量 detach，需要补充以下现有 guard；这是原研究表述的限定，不是本轮新增功能或产品缺陷：

- `server/internal/handler/project.go:958` 查询项目关联 issue 是否含独立协作；`:963` 在存在协作 issue 时返回 `409 project_has_collaboration_issues`，要求先通过协作流程移出项目。因此不能写成“删除项目会自动 detach 协作 issue”。
- 通过该检查后的普通项目删除仍在 `server/internal/handler/project.go:978` 调用 `DetachProjectIssues`；`server/pkg/db/queries/project.sql:122` 定义的 SQL 为 `UPDATE issue SET project_id=NULL, revision=revision+1, updated_at=now() WHERE project_id=$1 AND workspace_id=$2`。它不改 status，也没有 done 排除条件。
- 协作项目变更实际经 `server/internal/service/collaboration_edit.go:34` 的 EditIssue，ProjectID 等字段选择 `edit_source_structure`；`:100` 更新字段，`:108` 更新结构。`server/internal/service/collaboration_source_fields.go:167` 的 prepareSourceStructure 在 `:180` 仅锁有变化的有效目标项目。
- `server/internal/collaboration/access.go:100` 的 Edit 及 `:101` 的 EditSourceStructure 不依赖 open；已 done 的具权协作仍可编辑。跨账号参与者与源 workspace 成员是不同权限，结构编辑另有 sourceMemberLocked。
- 独立内容路径 `server/internal/service/issue_public.go:42` 的 UpdateContent 自持事务，`:104` 调 UpdateIssueContentOnly；`server/pkg/db/queries/issue.sql:616` 对真实 title/description 变化增加 revision，未改 status。若 D4 纳入已 done 内容变化，这条不能漏接。

因此 D4 的重验范围必须按变更事实决定，不能只覆盖普通 move；也不能在用户尚未决定时把上述已 done 编辑一律阻断。

### 排除入口复核

对非测试生产 Go 的 `UpdateIssueStatus`、`UpdateIssue`、`CreateIssue`、`CreateIssueWithOrigin`、`CreateInTx` 调用重新搜索，没有发现超出原矩阵的新增完成 writer。复读的关键排除条件：

- `server/internal/service/collaboration_lifecycle.go:61` 按 action/category 限制，只有 complete 可取 done；通用 change_status 及定向 handoff 不能绕过。
- `server/internal/service/collaboration_creation_fields.go:86` 拒绝终态创建；不是 CreateInTx 的另一种 done 来源。
- `server/internal/handler/issue_iteration_assignment.go:164` 对 iteration 更新使用字段白名单，`:180` 只接受 iteration/expected_revision/reason/allow_completed，拒绝与 status 混写。
- `server/internal/service/task.go:5957` 恢复只请求 todo；`server/internal/handler/triage.go:609` 创建固定 backlog。
- `server/internal/service/issue_status_write.go:100` 提供持久状态类别的严格解析，包含已归档自定义状态；不能把会退化到原 key 的 Effective 当准入解析。

该搜索与复读是源码证据，不能替代每个实际 writer 的运行验收。

## Files found

- `research/completion-source-manifest.json`、`research/source-head-verification.json`：原 45 文件清单与原 HEAD 核对记录，保持未修改。
- `server/internal/handler/delivery_evaluation.go`、`delivery_completion_hint.go`、`delivery_policy.go`、`delivery_evidence.go`、`agent_artifact.go`：私有主体、观察复用、政策/凭据和提示边界。
- `server/internal/handler/issue.go`、`issue_move.go`、`source_context.go`：普通/批量/move/评论子任务调用链。
- `server/internal/service/issue.go`、`issue_status_write.go`：创建及系统完成实际事务。
- `server/internal/service/collaboration_workflow.go`、`collaboration_lifecycle.go`、`admin_collaboration.go`：独立协作发起人和 R38 的不同主体及事务。
- `server/internal/handler/project.go`、`server/pkg/db/queries/project.sql`：协作删除 guard 与普通项目 detach。
- `server/internal/service/collaboration_edit.go`、`collaboration_source_fields.go`、`issue_public.go`：D4 条件范围的独立结构/内容 writer。

## Related specs

已阅读 `.trellis/workflow.md` 的规划/研究/状态约束，并核对：

- `.trellis/spec/server/delivery-completion-hints.md`
- `.trellis/spec/server/delivery-policy-evaluations.md`
- `.trellis/spec/server/delivery-evidence-observation.md`
- `.trellis/spec/server/controlled-artifact-execution.md`
- `.trellis/spec/server/collaboration-delivery.md`
- `.trellis/spec/server/collaboration-administration.md`

## External references

无。此问题由当前本地源码、规格与任务决策记录回答；未调用外部资料或真实供应商。

## Caveats / Not Found

- D1/D2 候选仍未批准。没有实施、任务启动、授权 schema 或 enforce/skip 选择。
- 本研究未运行 Go/前端/DB/认证路由/浏览器/性能/供应商测试；真实模型调用 0，DB/容器动作 0。历史测试计数不当作本轮覆盖。
- 未改产品代码、前序证据、spec 或任务状态；只新增本文件。主会话负责 HEAD blob、提交关系、总体状态与最终验收。
- 原清单只绑定被记录的 45 文件；补充读取的独立内容 writer 不应被误报为原清单的一部分。后续产品变化仍须重新冻结并验收全部受影响入口。
