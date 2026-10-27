# Research: 安全试点重试的真实派活与 leader 角色来源

- Query: 为什么本轮安全试点 leader 直接实现并代写成员报告；怎样用现有 API/CLI 验证一次真实 leader → member → leader 交接。
- Scope: internal；只读仓库源码及任务证据，不调用运行服务、不读取凭据、不修改产品/工作空间配置。
- Date: 2026-10-27

## Findings

### 1. 证据结论与已确认原因

`pilot/evidence/real-pilot-summary-20261027.json` 保留十二个 issue/子任务的评论、timeline 和执行历史。`POLA-4` 原始任务 `01a17b3b-c767-7d5d-9d5e-ec99b9846125`、`POLA-5` 原始任务 `01a17b59-c293-78a2-8972-73922aa8078c` 均记录 `is_leader_task=true`，作者为 leader `70f335f0-1f27-47f5-ac3f-4defb309a736`。它们直接实现了实验室补丁，没有在父 issue 派发成员。

按 `util.MentionRe` 的同一模式扫描四个原始案例的全部评论，得到零条可解析的 `mention://agent/…` 或 `mention://squad/…` 链接。`POLA-4` 三条标称“迁移审查员”“测试工程师”“安全/代码审查员”的报告，实际 `author_id` 都是 leader，`source_task_id` 都是 `01a17b3f-9dac-7e38-a08d-775dca44f236`。角色标签不构成派活或独立审查证据。

由此可以确定 **没有发生有效的精确 mention 派活**，且 **leader 越过了自身协调职责**。但原始回合的 claim payload 和实际 OpenCode `AGENTS.md` 未保存在现有研究输入中，不能直接把初始失败归因为“模型看到完整 briefing 后忽视它”，也不能断言产品丢失了 briefing。

### 2. 纠正回合还切换成了普通 agent 回合

`POLA-4` 的纠正回合 `01a17b3f-9dac-7e38-a08d-775dca44f236` 没有 `is_leader_task=true`，它由操作者回复 leader 已有评论触发。

这与当前源码的路径一致：

1. 无精确 agent/squad mention 的 member 回复先匹配父评论 agent 作者（`comment.go:2702`）。
2. `routeReplyToParentAuthor` 返回 `commentTriggerSourceThreadParent`，不附带 squad（`comment.go:2766`、`comment.go:2784`）。
3. `enqueueSingleCommentTrigger` 的 thread-parent 分支调用 `EnqueueTaskForThreadParent`（`comment.go:2599`、`comment.go:2605`）。
4. `EnqueueTaskForThreadParent` 显式传入 `isLeader=false` 和空 `squadID`（`service/task.go:1359`）。

因此“回复某个恰好是 leader 的 agent”并不等于“以 squad leader 身份激活”。这个纠正回合不能用来证明完整 squad briefing 下的模型行为。`POLA-3` 的纠正回复同样是普通 agent 回合。

下一次协调请求应显式 mention **小队**，而不是仅回复 leader 的评论。`comment.go:2664` 优先处理精确 agent/squad mention；`comment.go:3080`、`comment.go:3145` 会把 squad mention 解析为 `commentTriggerSourceMentionSquadLeader`，再调用 `EnqueueTaskForSquadLeader`（`comment.go:2585`、`service/task.go:1376`）。

### 3. 产品已有的 briefing 和 stop-after-dispatch 契约

- `handler/squad_briefing.go:23` 的 `Squad Operating Protocol` 要求 leader 协调、禁止自己实现，明确普通 `@name` 不触发成员；派发后记录 activity 并立即结束回合（同文件 `:28`、`:39`、`:50`、`:67`）。
- 同文件 `:120` 的硬规则要求 `[@Name](mention://<type>/<UUID>)`，禁止缺少合适成员时默默自己完成，规定一个任务只能选择 mention 或已分配 `todo` 子 issue 其中一条触发路径。
- `buildSquadLeaderBriefing` 拼接 Operating Protocol、含 UUID 的 Squad Roster、工作空间 squad instructions（同文件 `:180`）。
- claim 按 `task.IsLeaderTask && task.SquadID`，在确认仍是该小队 leader 后，把 briefing 追加到 `resp.Agent.Instructions`；缺 briefing 时清掉响应中的 leader 标志（`handler/daemon.go:2481`、`:2499`、`:2522`）。
- daemon 将 claim 的 `Agent.Instructions` 原样作为 `TaskContextForEnv.AgentInstructions`（`daemon/daemon.go:6937`、`:6960`）；`execenv/runtime_config_sections.go:109` 把它写入 `Agent Identity`。
- OpenCode 的运行时文件是工作目录中的 `AGENTS.md`（`execenv/runtime_config.go:158`）；backend 使用 `--dir` 和对应 `PWD`，系统 brief 不另行通过 `--prompt` 传递（`pkg/agent/opencode.go:84`、`:93`、`:142`）。
- `taskIsSquadLeader` 使用服务器 `leader_role_resolved` 能力；旧服务器才回退到 Instructions 中的标题推断（`daemon/prompt.go:839`）。历史队列行上的 `is_leader_task` 只证明 enqueue 意图，不能替代当时 claim 角色和运行时文件。
- leader 使用独立协调工作目录是预期行为，不能单凭 CWD 不在 `/work/lab` 判定 briefing 缺失（`daemon/local_directory.go:107`、`:121`）。它通过项目资源得知实验室绝对路径；这不赋予实现权限。

当前 `maintenance-lead/INSTRUCTIONS.md:3` 明确“负责分配工作，不亲自升级”，`:27` 还禁止亲自升级、运行测试或读公告。试点采用的简短 squad instructions 也没有授权 leader 代写成员报告。因此，不需要放宽自治或更改共享角色来修正试点。

### 4. 精确派活格式、CLI 和 API

`util/mention.go:16` 只解析 Markdown link 的 `mention://` 格式。它不把普通 `@安全审查员`、姓名、报告标题变成目标。CLI `runIssueCommentAdd` 把读取的内容直接写入 body 的 `content`，没有姓名到链接转换（`cmd/multica/cmd_issue.go:2049`、`:2101`）。

由 **正在运行的 leader 自己** 在当前 issue 中发出的最小派活评论：

```text
[@安全审查员](mention://agent/aae540c4-6bbe-42b6-a362-c62e2e434318) 请只读复核 supply-chain.json 的 hold/unknown、缺证 owner 和验收信号；不要修改文件或访问外部服务。用你自己的身份回复本线程。
```

先把文字写到当前 task 目录内的 UTF-8 文件，再执行：

```bash
multica issue comment add <issue-id> --content-file delegation.md --output json
multica squad activity <issue-id> action --reason "已派安全审查员只读复核缺证边界"
```

若 leader 由评论触发，第一条必须附 `--parent <本轮提供的 trigger comment UUID>`。禁止沿用上轮的 parent；服务端会核对 trigger/coalesced comment（`comment.go:1672`、`:1808`）。派发成功后结束顶层回合，不能睡眠等待、继续读实验室或代写成员结果，单并发 daemon 才能释放槽位给成员。

等价 API：

```http
POST /api/issues/{issue-id}/comments
Content-Type: application/json
X-Workspace-ID: <workspace-id>
Authorization: Bearer <current task credential>
```

```json
{
  "content": "[@安全审查员](mention://agent/aae540c4-6bbe-42b6-a362-c62e2e434318) 请只读复核缺证边界并用自己的身份回复。",
  "parent_id": "<仅评论触发时填写本轮 trigger comment UUID>"
}
```

不需要 `mentions`、`agent_id` 等额外 request 字段；由 `content` 解析。评论 create/edit 响应中的 `trigger_outcomes` 报告每个精确目标的 `queued/coalesced/deferred/blocked` 和 `reason_code`（`comment.go:75`）。它只在 create/edit 返回，不应从后续 GET 缺该字段推断未派发。应保存当时响应，再验证真实 run 和 author。

评估接口是 `POST /api/issues/{issue-id}/squad-evaluated`，body 为 `{"outcome":"action","reason":"…"}`；handler 核对调用 task 和当前 leader，不应拿普通 agent 回合强行调用（`cmd_squad.go:464`、`handler/squad.go:1212`）。

### 5. 作者、来源和私有角色授权

- 当前 task credential 固定 actor/task；`X-Agent-ID` 单独不能伪造其他成员。`resolveActor` 对 task token 信任服务端确定的身份，其他路径须验证 task 属于 agent（`handler/handler.go:933`）。
- agent 发出的评论写入自己的 `source_task_id`；这个字段保留 originator 继承链，不能靠标题或正文角色名更换作者（`comment.go:1759`、`:1839`）。
- `canInvokeAgent` 对 agent-to-agent 按最上层 human originator 授权；private 仅 owner 可调用，无 admin/A2A 绕过（`agent_access.go:25`、`:73`）。不存在“加入 squad 即可调用”的授权。
- `invokeOriginatorFromRequest` 从当前非终态 task 读取 `originator_user_id`，终态 task 不得继续派新工作（`agent_access.go:234`）。当前试点原始 human 为 `1e486130-bb06-4e6e-8be9-dd58c8eb83be`，但是否仍是每个 private target owner 应在新一轮通过现有读接口重新核验，不扩大 Access。
- 成员收到 mention 后创建自己的 run，task attribution 从触发评论的 `source_task_id` 继承已存在的 human（`service/task.go:1411`）。成员无精确 mention 的结果评论在 squad-owned 父 issue 上会唤醒 leader（`comment.go:2683`、`:2932`）；leader 自己的普通结果评论受 self-trigger guard 保护。

### 6. 仅修改试点 squad instructions 的候选增补

下面是 **尚未实际验证的试点补充文案**，应通过正常 squad update 和当前内容/时间前置条件应用到试点小队，不改共享 agent instructions、内置模板或自治等级：

```text
Leader 每轮只做协调，禁止编辑/回退实验室文件、运行测试、实现修复、模拟成员身份或替成员写报告。成员报告只有 author_id 属于该成员且 source_task_id 指向该成员真实任务时才有效。

每轮只派一个下一步。必须把对应完整 mention 链接放进当前 issue 的一条评论；普通 @姓名、文中列角色名和本地报告文件都不算派活。使用当前 task 的 CLI 凭据。评论触发回合必须 --parent 本轮 trigger comment。读取 create-comment 响应；只有 queued/coalesced/deferred 且最终有对应成员 run 才算投递。blocked 时报告 reason，不放宽 Access。

可粘贴的成员链接：
分诊：[@产品分析师](mention://agent/c400dd9b-689c-4cf6-8224-682fd8d0cee8)
安全：[@安全审查员](mention://agent/aae540c4-6bbe-42b6-a362-c62e2e434318)
诊断：[@诊断工程师](mention://agent/328e2d0c-338a-46f1-aec9-0f0ff14dccfb)
实现：[@实现工程师](mention://agent/2e1aa5ca-4b7d-4278-a0bb-e47f7820ebff)
验证：[@测试工程师](mention://agent/da164006-db36-4041-8898-d88f14fb23cf)
兼容性：[@迁移审查员](mention://agent/08377736-afba-4140-b64c-67705fc5ba46)
正确性：[@代码审查员](mention://agent/0275de41-ff8b-4e4c-965b-d33a3bc2dbae)

成功派发后调用 multica squad activity <本轮issue-id> action --reason "<下一owner和理由>"，然后立刻结束回合。不得等待、跑测试或继续派下一个成员。成员回复后再决定下一步。没有下一步时记录 no_action 并静默结束；记录失败遵守现有单评论例外。不得标 done。

本轮若仅验证安全审查派活，先由安全审查员只读核对 supply-chain.json 的 hold/unknown，待真实成员回复后只记录 no_action，不再启动修复或后续角色。
```

注意这些 UUID 只属于本试点工作空间，不能固化进产品模板。后续模板必须依赖实时 Squad Roster。

### 7. 建议的单次路由验证

1. 新建或复用一个 squad-owned 的只读路由案例，明确只审 `supply-chain.json`、禁止补丁或外部调用。先按上面的候选文案更新 **试点 squad**，保留修改前值用于恢复。
2. 操作者的启动评论只放小队链接，不能同时放精确成员链接或代码中的示例链接，否则 parser 会直接把成员作为 human mention 入队，无法证明 leader 派活。

```text
[@安全漏洞修复小队（试运行）](mention://squad/ac1a40c2-a655-4453-b86d-745796a454bc) 本轮只验证真实成员交接。请按试点规则把 supply-chain.json 的只读 hold/unknown 复核交给安全审查员；派活后记录 activity 并立即结束，不自己审查、不改文件。成员回复后仅记录 no_action，父任务不标 done。
```

3. 捕获这一轮实际 runtime briefing 的脱敏片段（Agent Identity、Squad Operating Protocol、Squad Roster、Squad Instructions）及 task role 来源，区别“内容注入”与“模型遵守”；不要把 credential/env 文件复制进证据。
4. 断言 leader 评论作者为 `70f335f0-…`，包含安全审查员精确链接；create 响应有该 target 的非 blocked outcome；leader 先终态再由安全审查员 run 执行。
5. 断言 member task 的 agent 为 `aae540c4-…`、触发评论为 leader 的那条委派；成员评论同一作者且 `source_task_id` 属于该真实 run。
6. 断言成员结果触发新的 leader 回合；leader 记录 `no_action` 后退出，没有代写成员报告、没有后续循环；实验室 diff/hash 未变化。
7. 将该新增证据与已有补丁/QA结果分开记录。一次路由成功不弥补原先缺失的 `diagnostician → implementer → QA` 全链，也不关闭网络最小权限或阶段二准入。

## Files Found

- `server/internal/handler/squad_briefing.go`：leader 协调、精确 mention、回合结束和 roster 拼接。
- `server/internal/handler/daemon.go`：claim 时的 leader 角色核对与 briefing 追加/降级。
- `server/internal/handler/comment.go`：mention 解析、thread-parent/assigned-squad fallback、actor 与 source task 记录。
- `server/internal/util/mention.go`：唯一的 Markdown mention regex。
- `server/cmd/multica/cmd_issue.go`：评论 content 原样传输，没有姓名转换。
- `server/internal/service/task.go`：普通回复任务与 squad-leader 任务的 role flags、delegation attribution。
- `server/internal/handler/agent_access.go`：private invoke 与 human originator 授权。
- `server/internal/daemon/execenv/runtime_config.go`、`runtime_config_sections.go`：OpenCode AGENTS.md 和 Agent Identity 注入。
- `server/pkg/agent/opencode.go`：`--dir`、`PWD`、文件 brief 的接入点。
- `server/internal/service/builtin_agent_templates/maintenance-lead/INSTRUCTIONS.md`：既有 leader 不亲自实现/测试的角色边界。
- `pilot/evidence/real-pilot-summary-20261027.json`：真实 actor、task 和手工补交成员结果的来源。

## Related Specs and Regression Anchors

- `.trellis/spec/server/builtin-templates.md`：模板来源、工作空间副本和自治边界。
- `.trellis/spec/server/security-boundaries.md`：human/machine 主体与不可借用 owner 权限。
- `TestClaim_LeaderTaskFromCommentMention_InjectsBriefing`、`TestClaim_NonLeaderTask_NoBriefing`、`TestClaim_LeaderSwappedAfterEnqueue_NoBriefingAndNoLeaderRole`。
- `TestCreateComment_SquadMentionTriggersLeader`、`TestCreateComment_SquadLeaderThreadParentTaskDoesNotSelfTriggerAssignedFallback`。
- `TestCreateComment_WorkerAgentCommentWakesPrivateSquadLeader_MUL4015`、`TestCreateComment_WorkerAgentCommentWakesSquadLeader_MUL4015`。

## External References

未使用外部资料。OpenCode `1.18.21` 和试点模型名称来自本任务已存证据；本结论基于当前仓库接入代码，不宣称已分析 OpenCode 上游加载器的实现或模型内部决策。

## Caveats / Not Found

- 本研究没有运行服务、重放模型或更改小队。候选指令与下一轮流程未验证。
- 缺少初始 task 的实际 claim 响应、task 所见 AGENTS.md 快照；原始 leader 直接实现的更深原因仍为“instruction delivery/compliance 待判定”。现有代码已覆盖这些注入路径，不应在无证据时改产品代码。
- 本地任务 evidence 中的 GET comment 历史没有 create-time `trigger_outcomes`，因此只能证明现有文本无可解析链接；不能重建某条过去请求在服务端的瞬时入队结果。
- 已完成的独立成员子任务证明成员能够执行各自职责，不能证明 leader 自主派活。它们由操作者直接创建，attribution 为 direct_human。
- `comment.go:2982` 的一段旧注释称 private agent 可由 admin invoke，但实际 `agent_access.go:25`、`:86` 明确无 admin bypass；以实际授权函数为准。
