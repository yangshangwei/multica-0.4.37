# Research: Codex 续跑的真实小队交接与运行时边界

- Query: 用户将底层运行时切换到 Codex 后，怎样沿用当前试点身份完成 leader → member → leader 回环，并准确判断运行时、目录与归因是否一致。
- Scope: internal; current source inspection, no live API mutation or model invocation.
- Date: 2026-10-27
- Task: `10-26-security-remediation-squad`; 阶段一尚未通过，阶段二不属于本次研究的实现范围。

## Findings

### 1. 先区分两套执行模型

本试点历史记录使用普通工作区 issue / squad / `agent_task_queue` 路径。Leader 的权威标识是 task 的 `is_leader_task` 与 `squad_id`；成员是自己的普通 agent task。这里不存在必须填写的 `squad_member` task kind。当前接口公开 `is_leader_task`、`squad_id`、`trigger_comment_id`、`delivered_comment_ids` 和 `attribution`，后者包含 `source`、`delegated_from_task_id`、originator 和 evidence reference（`server/internal/handler/agent.go:429`、`:472`、`:504`、`:534`、`:566`）。

另一套跨工作区协作执行使用 `collaboration_run.step_kind`、execution group、冻结 roster、business capability、`leader_turn`，对应 `/api/collaboration/runtime/runs/{runId}/...`。其成员和续轮有独立 admission/proof；`server/internal/service/collaboration_squad.go:154`、`:263`、`:333` 及 `.trellis/spec/server/collaboration-squads.md:12` 描述的是这套契约。不能把那套 squad-activity endpoint、request-id 或 settlement 规则套到本试点的普通任务上。

CLI `multica squad activity` 确实会先判断是否处在 collaboration runtime，但普通试点走 `/api/issues/{issue-id}/squad-evaluated`（`server/cmd/multica/cmd_squad.go:464`）。切换模型 provider 本身不会改变业务执行模型。

### 2. 启动真正 leader 的最小路径

读取实时的 squad、roster、issue、agents、runtime 和项目资源，确认仍是目标试点且有权执行后，在父 issue 发布**仅包含目标小队精确链接**的启动评论：

```text
[@安全漏洞修复小队（试运行）](mention://squad/ac1a40c2-a655-4453-b86d-745796a454bc)
本轮继续未完成的只读供应链交接验证。请由安全审查员本人复核缺证结论，保留 hold/unknown 与下一 owner；派发后记录 action 并结束。成员结果返回后回读证据，记录 no_action 并结束。本轮不修改实验室代码或执行生产操作。
```

```bash
multica issue comment add <issue-UUID> --content-file start.md --output json
```

依据：

- 精确 mention 由 `util.MentionRe` / `ParseMentions` 解析，必须是 `[@Label](mention://agent-or-squad/UUID)`；普通姓名、漏掉方括号都不会触发（`server/internal/util/mention.go:17`）。Parser 不把代码块中的示例链接当作无害文字，所以启动评论不要附完整 roster 或可解析的成员示例。
- Explicit agent/squad mention 优先于 thread-parent / assignee fallback（`server/internal/handler/comment.go:2664`）。Squad mention 解析 leader 后调用 `EnqueueTaskForSquadLeader`（`:2585`、`:3080`、`:3145`），产生 `is_leader_task=true` 和 `squad_id`（`server/internal/service/task.go:1376`、`:1425`）。
- Human 仅回复 leader 旧评论会走 `routeReplyToParentAuthor` → `EnqueueTaskForThreadParent`，后者明确创建 `isLeader=false`、无 squad 的普通 agent 回合（`comment.go:2702`、`:2766`；`task.go:1359`）。这不能验证完整小队 briefing。
- 创建评论响应的 `trigger_outcomes` 能说明 explicit target 是 queued/coalesced/deferred/blocked，但它不等于任务成功。仍需回读 task 记录（`comment.go:1914`、`:1955`）。

启动评论应由当前拥有这些 private agents 调用权的 human 发出。A2A 后续继承同一 human originator；workspace admin 或“加入了 squad”都不会绕过 private invoke 限制（`server/internal/handler/agent_access.go:13`、`:73`）。

### 3. Leader → member 的实际执行步骤

以下命令必须由**正在运行的 leader**使用当前 task 身份执行。操作者代发同样正文只证明 human 触发，不能作为 leader delegation：

```text
[@安全审查员](mention://agent/aae540c4-6bbe-42b6-a362-c62e2e434318)
请只读复核 supply-chain.json 的来源摘要、生产者证明和组件清单缺口；不改文件、不访问外部服务。保留 evidence_state=unknown、hold 和 pilot-build-owner 的补证责任。用本轮 trigger comment 作为 --parent 发布本人结果后结束。
```

```bash
multica issue comment add <issue-UUID> --content-file delegation.md --parent <THIS-turn-trigger-comment-UUID> --output json
multica squad activity <issue-UUID> action --reason "已派安全审查员复核缺证，等待成员结果"
```

随后 leader 必须立即结束。单并发 daemon 若 leader 睡眠等待或继续工作，成员拿不到执行槽位。规则在 `server/internal/handler/squad_briefing.go:23`、`:50`、`:67`；当前试点 v2 指令又做了相同限制。

如果本轮是 assignment-triggered、没有 trigger comment，第一条不传 `--parent`。如果有 trigger/coalesced comment，必须使用**本轮**允许的 ID；沿用上一轮 parent 或创建顶层回复会被 409 拒绝（`server/internal/handler/comment.go:1794`）。文件需位于当前 cwd，或明确采用 CLI 支持的 stdin 模式；不要把旧工作目录文件误当本轮输入（`server/cmd/multica/cmd_issue.go:614`）。

评论会由服务器记录真实 `author_type=agent`、`author_id=leader-ID`、`source_task_id=leader-task-ID`；task token 强制这些身份，正文角色标签不能替代它（`comment.go:1759`、`:1839`）。提及成员产生 distinct task，其 `attribution.source=delegation`、`delegated_from_task_id=leader-task-ID`、`trigger_comment_id=delegation-comment-ID`（`server/internal/service/task.go:1415`、`:1425`）。

一轮只选择一种派活方式。创建 `todo` 且 assigned-to-agent 的子 issue 已自动触发；再在父 issue mention 同一个 agent 会执行两次（`squad_briefing.go:145`）。本次回环优先使用同一父 issue 的 mention 路径。

### 4. Member → leader 何时自动发生

**可靠的正常路径是成员本人通过 `multica issue comment add` 在同一个、仍 assigned-to-squad 的父 issue 发布结果**，回复其当前 trigger comment。结果正文不要无意包含其他 explicit agent/squad/member/all mention，也不要加 `/note` 前缀。

```bash
multica issue comment add <same-issue-UUID> --content-file result.md --parent <member-turn-trigger-comment-UUID> --output json
```

该 HTTP create 路径先保存 `source_task_id`，随后同步调用 `triggerTasksForComment`。对于没有 explicit mentions 的 agent-authored comment，保留了一条窄路径：如果 issue assigned-to-squad，就调用 `routeAssignedSquadLeaderFallback` 唤醒其 leader；leader 自发普通结果受 self-trigger guard 保护（`server/internal/handler/comment.go:1914`、`:2683`、`:2932`）。结果评论会继承成员的 human originator，让 private leader gate 通过；对应回归为 `TestCreateComment_WorkerAgentCommentWakesPrivateSquadLeader_MUL4015`（`squad_worker_comment_wakes_leader_test.go:201`）。

自动回唤有以下边界：

- Issue 只是 @squad 邀请的 guest case、并未 assigned-to-squad 时，成员普通结果不会走上述 assignee fallback。生产试点父 issue 应先核对实时 assignee。
- 结果含 explicit mentions 时，explicit 路由优先，不会同时附加 assigned leader。`@all`、member mention、`/note` 可以抑制 implicit 路由。
- 成员的“完成”必须有可回读的结果。普通 `CompleteTask` 会在没有已发评论且 output 非空时合成 fallback comment（`server/internal/service/task.go:4474`、`:4516`）；但 `createAgentComment` 只保存和广播（`:7186`），未调用 HTTP `triggerTasksForComment`。Completion reconcile 又故意排除 plain agent comment 的 implicit leader fallback（`server/internal/handler/daemon.go:4091`、`:4209`）。因此**不能以 task=completed 或终端 final output 推定一定发生了 leader 回唤**。要求成员主动发布结果，并回读新 leader task。
- 成员若已经发过一条 progress comment，完成时 fallback 检测可能认为已有评论而不再补最终报告（`task.go:4492`）。只发“开始检查”不构成最终证据。
- 保持单一执行链，减少 leader 处在 dispatched 阶段时收到普通成员回报的竞争；pending/coalesced 结果依实际 task 与 delivered receipt 判断，不能数评论推算 run 次数。

### 5. 最终 activity 与状态

Leader 回唤后先读取成员实际证据，再决定下一阶段或收口：

- 真正派出下一成员、或做了收口状态动作：记 `action`，不能记 `no_action`。
- 此次只确认现有 `hold/unknown` 已有 owner、不需继续动作：调用 `no_action` 并静默结束。
- Activity 失败：按已注入 protocol 的一评论例外留痕；不可无限重试或代写成员。

```bash
multica squad activity <THIS-turn-issue-UUID> no_action --reason "已回读安全审查员结果，缺证由 pilot-build-owner 补齐，本轮无后续操作"
```

普通 endpoint 是 `POST /api/issues/{issue-id}/squad-evaluated`，JSON 为 `{"outcome":"action|no_action|failed","reason":"..."}`。它要求当前 actor 就是 task.agent，task 属于该 issue，`is_leader_task=true`、有 `squad_id`，且 actor 仍为该 squad 的当前 leader（`server/internal/handler/squad.go:1212`、`:1282`、`:1303`、`:1342`）。因此操作者不能代记旧 leader 的正确 activity。历史 `no_action` 误记应保留为历史；新一轮正确执行提供新证据。

`no_action` 会禁止该 task 再在本 issue 评论（`comment.go:1827`）并抑制 terminal fallback（`task.go:4483`）。若还需评论或更新状态，应先完成动作并记 `action`，不要先记 `no_action` 再解释自己为什么完成。

普通 squad-owned leader 的系统协议是派活期间 `in_progress`，整体验收后 `in_review`，`done` 留给人工/已有集成（`squad_briefing.go:98`）。试点 v2 明确本轮父案例留 `in_review`。研究不改变这份已采用的试点限制，更不能把供应链 hold 当风险已解决。

### 6. Codex 绑定与目录核验

Provider 切换和恢复执行前，应分别读取这几层；这不是同一个“在线”判断：

1. 8 个 agent 的 ID、runtime_id、template_key/version、autonomy、permission_mode、model/thinking/service tier、skills 摘要。
2. 各 runtime 的 workspace、provider、daemon_id、owner、online/heartbeat 与真实模型结果。
3. 项目 local_directory 的 resource ID、`local_path`、`daemon_id`、`execution_mode`。
4. 所有关联旧/新 runtimes 的 pending/active tasks，避免恢复 daemon 消费历史无关队列。

可以使用的现有命令/API：

```bash
multica agent get <agent-UUID> --output json
multica agent update <agent-UUID> --runtime-id <Codex-runtime-UUID> --output json
multica project resource list <project-UUID> --output json
multica project resource update <project-UUID> <resource-UUID> --local-path <isolated-lab-absolute-path> --daemon-id <Codex-daemon-UUID> --execution-mode in_place --output json
```

这些是可用形状，**研究没有执行它们，也不建议在实时状态未知时批量重绑共享 agents**。用户如果已经切换，先验证其结果即可。

- `UpdateAgent` 通过最小 PUT body 支持 runtime_id；不要求重写 instructions/Access（`server/cmd/multica/cmd_agent.go:736`、`:753`）。服务器限定同 workspace、private runtime 仅 owner 可使用，并同步 runtime_mode（`server/internal/handler/agent.go:1985`）。
- 跨 provider 更新会清除已知不兼容旧 model；带 `/` 的 OpenCode model ID 属于 runtime-specific，会在 Codex 静态 catalog 不接受时清空（`agent.go:2075`；`server/pkg/agent/models.go:433`、`:486`）。Unknown/custom 值可能保留，explicit model 不由该 fallback 清除。思考强度/服务等级存在不兼容值会要求明确清空或替换，不可假设自动迁移（`agent.go:2085`、`:2127`、`:2179`）。
- **旧 queued task 不会随 agent.runtime_id 自动移动**。Claim recheck 若 task runtime 与现 agent runtime 不同，拒绝执行并要求 retry（`server/internal/handler/daemon.go:2219`）。新队列行从当前 agent.runtime_id 创建（`task.go:1425`）。
- **手动 rerun 不是新自主 delegation 证据**。它保留历史 agent/leader/squad/trigger，可恢复工作，但新 attribution 是 `direct_human`，另记 `rerun_of_task_id`（`task.go:5608`、`:5696`）。若要关闭原任务，可记录为 human recovery；若要验收自主路由，用新 @squad 请求让 leader 真正重派。
- 旧 OpenCode session 不会直接交给新 Codex runtime。Claim 只在 source/current runtime 相同且会话安全时传 prior_session_id；prior_work_dir 仍可能作为 best-effort 目录复用候选传递（`daemon.go:2708`、`:2731`、`:2755`）。因此必须确认源码/fixture 目录的存在和内容，不能只看“新会话”。
- Local directory 根据 **daemon_id** 匹配，不根据 runtime_id。不同 provider 若同一 daemon，目录仍匹配；若 Codex 在另一台/另一 daemon，旧 `/work/lab` 会被忽略，任务转普通工作目录路径（`server/internal/daemon/local_directory.go:156`、`:181`）。保持每 project/daemon 最多一个资源；更新已有资源时用 `project resource update <project> <resource>`，不要制造冲突（`server/cmd/multica/cmd_project.go:82`；`server/internal/handler/project_resource.go:538`、`:677`）。
- Leader 不绑定用户 local_directory，不持有路径写锁；使用独立协调目录是正常行为（`local_directory.go:107`、`:122`）。它能从项目资源知道实验室绝对路径，但这不授权亲自实现。
- Codex 接收 daemon 写入实际 cwd 的 `AGENTS.md`；app-server `thread/start` 的 `developerInstructions=nil` 是避免重复注入的既有实现，不代表丢失 briefing（`server/internal/daemon/execenv/runtime_config.go:156`；`server/pkg/agent/codex.go:1946`）。当前源码注释的实测基线为 codex-cli 0.144.6；实际安装版本和本轮注入内容仍需实时采集。
- 构建 squad from-template 会复用共享角色并保留旧 runtime；generic agent copy 丢失 template/autonomy provenance。不要用这些路径伪造“相同角色已迁移”（`.trellis/spec/server/builtin-templates.md:140`）。

### 7. 可接受的回环证据矩阵

使用普通只读接口回读完整分页/线程数据，并保存脱敏投影：

- `GET /api/issues/{id}/task-runs`
- `GET /api/issues/{id}/comments`
- `GET /api/issues/{id}/timeline`
- `GET /api/agents/{id}`、`GET /api/squads/{id}`、`GET /api/squads/{id}/members`
- `GET /api/projects/{id}/resources`

普通 router 注册位置：`server/cmd/server/router.go:2204`、`:2214`、`:2301`、`:2344`。精确过滤范围和分页以当前 handler 参数为准，不把首屏当完整历史。

| 阶段 | 必须出现的真实记录 |
|---|---|
| 启动 | 当前 human 的 squad mention；非 blocked target outcome；对应 leader task 在当前 Codex runtime，`is_leader_task=true`、squad_id 正确 |
| Briefing | 本轮 claim delivered role 或实际 AGENTS.md 的 Agent Identity/Operating Protocol/Roster/Squad Instructions 脱敏投影；不要保存 credential/env 全文 |
| 派发 | Leader 本人评论，精确一名成员链接，source_task_id 指向此次 leader run；本 run 的 squad_leader_evaluated outcome=action |
| 成员执行 | 独立 member agent 的真实 task，trigger 是 leader 派发评论；attribution.source=delegation，delegated_from_task_id 指向 leader；终态和实际工具结果 |
| 成员报告 | Author 是该成员、source_task_id 属于上述 task；当前 parent 正确；测试/哈希/结论与原始命令吻合 |
| 返回 | 新 leader task 的 trigger/delivered_comment_ids 覆盖成员报告，并保留可审计 human chain；不是操作者补点 rerun |
| 收口 | 新 leader 根据报告记录正确 action 或 no_action 后终态；没有代写、无继续循环/无遗留 active task；父 issue 不标 done |
| 只读供应链用例 | Lab source/diff/hash 无变动；unknown/hold 与 owner 仍在；网络目的地和身份隔离另有实际拒绝证据 |

供应链只读回环通过只能关闭“真正成员返回/leader 收口”这一项。组件升级、原因未知的诊断 → 实现 → QA、迁移审查、证据正确性和网络最小权限仍需各自真实验证，不能由一个只读 case 推导阶段一整体通过。

### 8. 当前工作区保留旧角色的私有 Codex 试点副本

主会话随后提供了本轮 live API 事实：八个历史角色仍绑定 offline OpenCode runtime `39443bc2-1bf7-4515-8bdd-4684a41d64c9` / daemon `019fef71-c14f-757e-9ad2-a9750d306456`；在线 host Codex runtime `c64f60ac-bbda-4831-8dc0-ad2376c261a2` 属于另一 daemon `80193da5-b315-8122-93e8-0482d01eb4d5`。这些是主会话的实时读取结果，本 researcher 没有自行访问 live API。

在保持北辰工作区和历史角色原状的前提下，公共 API 有一项限制：

- 七个 listed 成员可通过 `POST /api/agents/from-template` 指定唯一 `name` 创建第二份 agent；template key/version/autonomy/instructions/role skills 由服务器决定，private 是默认值（`server/internal/handler/agent_template.go:97`、`:137`、`:203`、`:220`）。
- `maintenance-lead` 是 unlisted，单角色 from-template 明确拒绝（`:137`）。Squad from-template 没有 force-new 或 role override 参数（`server/internal/handler/squad_template.go:114`），且 `resolveTemplateAgentInTx` 按 workspace+template_key 无条件复用已有可接入角色，不会迁移它的 runtime（`:477`、`:508`）。所以同一工作区没有“历史 leader 完全不变，同时创建另一个具有同一内置 template provenance 的 leader”的公开路径。
- 不应临时 archive/改名旧 leader 诱导模板重复创建，更不能直接改数据库、伪造 provider 或给 generic copy 填造假的 template_key。

**可行的当前工作区方案是：七个 listed 模板角色副本 + 一名诚实标记为 custom 的私有试点 coordinator。** 它重用实际 maintenance-lead 的角色内容和已批准的 Coordinator 等级，但不声称自己是该模板直接创建的角色。无需新增产品模板或改任何历史 agent 的 runtime/Access/指令。

#### 8.1 创建 custom coordinator 并显式声明权限

通过当前已授权的 human API 身份提交。先用正常 `POST /api/agents` 创建一个有唯一名称、专用 runtime、private Access 的 custom agent，例如：

```json
{
  "name": "安全修复协调员（Codex 试点）",
  "description": "本轮隔离试点专用协调员；角色内容来自历史例行维护负责人，非内置模板创建。",
  "instructions": "<从历史 maintenance-lead GET 响应读取的实际完整 instructions>",
  "runtime_id": "<专用 Codex 试点 runtime UUID>",
  "permission_mode": "private",
  "max_concurrent_tasks": 2,
  "skill_ids": []
}
```

`skill_ids` 应以历史 leader 实际绑定为准，不从示例推定为空；当前内置 maintenance-lead v3 的 RoleSkills 确实为空（`server/internal/service/builtin_agent_templates_roster.go:355`），但 workspace copy 可能曾自定义。不要复制 custom env、个人 MCP、未知 runtime args 或凭据。角色 instructions 原样复制，pilot-specific 限制继续放在这一个 squad 的 instructions 中。

**创建 body 没有 `autonomy_level`，此字段不能在 generic create 时设置。** `CreateAgentRequest` 不包含它，`CreateAgent` 固定传空 provenance（`server/internal/handler/agent.go:1239`、`:1327`）。创建成功后，在该新 agent 尚未入 squad、尚未有任务、专用 daemon 尚未消费工作时，立即提交：

```http
PUT /api/agents/<new-custom-leader-id>
Content-Type: application/json

{"autonomy_level":"coordinator"}
```

`UpdateAgent` 接受 `coordinator`，但该字段只允许 human credential actor 变更；task/cloud machine credential 会得到 403（`agent.go:1932`）。这是显式设置本次已设计的 Coordinator 契约，不是声称 generic copy 自动继承权限。不要创建/运行任何 task，直到 GET 回读同时证明：private、正确 runtime/owner、`autonomy_level=coordinator`、template_key 为空、instructions SHA-256 匹配历史来源、skills IDs/hash 符合预期。

此两步只修复了“普通复制丢失 autonomy 且被误认为已有权限”的问题；**模板 provenance 仍然不同，而且必须一直保留这一事实**。建议证据中记录 `creation_kind=custom` 与独立的 `derived_from` 对象（source_agent_id、source_template_key/version、instructions_sha256、skill 摘要、读取时间），不把它写成 agent 的真实 template_key。比较行为结果时说明“同角色内容的自定义 coordinator”，不要写“八个内置模板角色原样复制”。

#### 8.2 创建七名真实模板成员

逐个 `POST /api/agents/from-template`，使用七个 key：`product-analyst`、`security-reviewer`、`diagnostician`、`implementer`、`qa-engineer`、`migration-reviewer`、`code-reviewer`：

```json
{
  "template_key": "security-reviewer",
  "runtime_id": "<专用 Codex 试点 runtime UUID>",
  "name": "安全审查员（Codex 试点）",
  "permission_mode": "private",
  "language": "zh"
}
```

每份返回立即记录真实 template key/version/autonomy/runtime/permission/skills，不接受创建半成功后继续启动。Skill materialization 按 workspace+name 复用已有 skill 并保留其内容（`agent_template.go:279`、`:305`）；不会强行把 workspace 调优内容恢复成内置最新版。它保护历史内容，但不保证逐字等于当前内置 skill。用实际 skill ID、version 与内容摘要验证，不为本试点改写共享 skill。

#### 8.3 仅更换试点 squad 的 roster / leader

普通 `UpdateSquad` 支持 leader_id 和 instructions 同一 PUT，并可带成对 `expected_instructions` / `expected_updated_at`；leader rotation 在事务内会自动加入新 leader，但**不会移除旧 leader**（`server/internal/handler/squad.go:382`、`:408`、`:476`）。成员增删是独立 endpoint，整组操作不是单事务。

可靠执行顺序：

1. 保存当前试点 squad、完整成员 role notes、instructions/updated_at、项目 resource 和旧/新相关 task 队列；确认试点无 pending/active 任务。保持旧隔离 daemon 停止，并让新专用 daemon 不消费任务。不要触发修复、恢复旧 daemon ID 或改历史共享 agents。
2. 创建并核验上述八个新角色；失败时不切换 squad。
3. 为这个试点 squad 添加七个新成员，按旧 seat 保留 role note：`POST /api/squads/{squad}/members` body `{"member_type":"agent","member_id":"<new-id>","role":"<seat note>"}`（`squad.go:1000`）。此操作只改 squad_member，不改 agent。
4. 删除七个旧非 leader seats（CLI `multica squad member remove <squad> --member-id <old-id> --type agent` 的具体 flags 应按本地 CLI help 核对）；不要删除/归档 agent 本体。HTTP 是 DELETE 同一 members endpoint，JSON 指定 member_type/member_id（`squad.go:1101`）。
5. 重新 GET squad 获得当前完整 instructions/updated_at；将 v2 指令里八个旧 agent UUID 逐一替换成新 ID，其他已批准规则保持一致。一次条件 PUT 同时设置 `leader_id=<custom-id>` 和新 instructions。读取失败/409 时停止切换，重新核对差异；不能盲目覆盖。
6. PUT 自动加入 custom leader 后，再移除旧 leader 的 squad_member seat。RemoveSquadMember 对当前 leader 明确拒绝，所以必须先换 leader（`squad.go:1128`）。
7. 回读最终 exactly-eight roster、唯一 leader、每个精确 mention 目标、各自治等级和 private Access；重查队列仍为空。更新仅该试点项目的 local_directory owner/path 到新隔离实验室，留存旧 resource snapshot。然后才启用新 daemon 并触发新 @squad 请求。

若要进一步降低既有 squad 配置改动风险，八个角色也可以新建一个明确命名的 pilot squad；但须只将指定试点 issue 的 assignee 指向它，并记录历史关系，不能假定新 squad 自动接管旧任务。当前方案中保留原 squad ID 能继续使用原试点父案例；人类评论中的原 @squad 链接仍正确，成员链接需全部更新。

回滚只恢复本试点 squad 的 leader/roster/instructions 与项目 resource。先确保新 agent tasks 已终止/空闲，再反向配置；不重写历史 run、旧角色或其他 squads。新角色若保留应清楚标明试点归属，若清理应在主会话确认无排队/使用者后处理。

#### 8.4 防止误消费历史任务

- 新容器应使用新的稳定 daemon 身份和新 provider runtime，不借用旧 OpenCode daemon ID，也不绑到 host 普通 Codex runtime 后声称已隔离。所有新角色只绑专用 runtime；历史角色继续绑旧 runtime。
- 按 runtime_id / daemon_id 列出全部新注册 namespace，再确认没有非试点 agent/task。新 identity 没有旧 runtime task 队列，提供执行范围分离；只选择 profile.workspace_id 不足以限制账号发现。
- `ListDaemonWorkspaces` 明确 user PAT/JWT 返回账号所有工作区，workspace-scoped daemon token 才返回绑定工作区（`server/internal/handler/daemon_workspace.go:18`）。本研究未找到普通公开的“为本试点随意铸造 workspace daemon token”接口，不能直接插入 token 或改认证类型以限制范围。
- Runtime 在线/没有 agent 绑定不保证模型可用；先在专用边界内做一次受控只读角色任务，确认实际 provider、网络目的地、目录和来源，再启动长链。

以上方案满足“保留八个历史角色不动、复用其角色行为、权限不超过既定等级、运行当前工作区试点”的语义；它**不满足新 leader 与原 leader 具有相同内置 template provenance**，而是显式记录一个 custom 来源差异。阶段一验收和后续固化的总结必须保留这项限制。

## Files Found

- `server/internal/handler/comment.go` — HTTP comment provenance、精确 mention、leader/member 路由、parent/no_action 防漂移。
- `server/internal/service/task.go` — task enqueue、attribution、completion fallback、rerun 归因。
- `server/internal/handler/daemon.go` — runtime/role claim recheck、briefing 注入、跨 runtime session 隔离与完成 reconciliation。
- `server/internal/handler/squad.go` — activity 的当前 task / leader 双重授权。
- `server/internal/handler/squad_briefing.go` — 协调、一次派发、activity、停止与父任务状态契约。
- `server/internal/handler/agent.go` / `server/pkg/agent/models.go` — runtime rebind、provider 模型/思考强度的迁移规则。
- `server/internal/daemon/local_directory.go` / `server/internal/handler/project_resource.go` — 本地实验室 daemon owner 与目录互斥边界。
- `server/pkg/agent/codex.go` / `server/internal/daemon/execenv/runtime_config.go` — Codex app-server 与本轮 AGENTS.md 上下文来源。
- `server/cmd/multica/cmd_issue.go`, `cmd_squad.go`, `cmd_agent.go`, `cmd_project.go` — 上述 CLI 的真实参数和 endpoint。
- `server/internal/handler/squad_worker_comment_wakes_leader_test.go`, `squad_briefing_test.go`, `squad_no_action_test.go`, `squad_evaluation_provenance_test.go` — private-originator 回唤、claim briefing、no_action 和 leader task 授权的回归锚点。

## External References

本研究没有查询外部资料；provider 事实限于当前仓库的 Codex adapter 与其源码注释。没有检查账户、实际安装版本、网络代理或模型费用。

## Related Specs

- `.trellis/spec/server/builtin-templates.md` — role provenance、runtime 恢复、精确交接证据。
- `.trellis/spec/server/security-boundaries.md` — 机器凭据和 human authority 的边界。
- `.trellis/spec/server/collaboration-squads.md` — 用于区分跨工作区执行模型，不能替代本试点普通 squad 规则。
- `pilot/runtime/pilot-instructions-v2.md`, `pilot/trial-results.md`, `research/retry-routing-20261027.md` — 已采用的试点指令与未闭合历史。

## Caveats / Not Found

- 这是源码研究，不是实测验收；没有运行真实模型、读凭据、变更 API、启动 daemon、跑测试或修改产品代码。
- 用户所说 Codex 切换的实际范围（全部八个角色、仅 leader、同 daemon 还是新 daemon）必须由主会话回读当前数据确认。旧 run-state.json 是历史，不能覆盖较新的用户设置。
- 当前源码支持链路不证明当前开发 API/daemon 已运行该版本。采集实际 API health commit、daemon binary/provider 版本以及本轮 task runtime。
- 本研究没有证明 OS 用户/容器隔离或 API/模型出站限制。Codex provider 名称和 private Access 均不能代替这些证据。
- 不应改写旧失败任务或 activity 来制造通过；保留历史，新增正确的证据链。
