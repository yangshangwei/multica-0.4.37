# Research: 模拟小队的独立 handler 契约测试

- Query: 在不调用真实 Codex、OpenCode 或任何模型的前提下，用哪些现有 handler tests 独立验证派活语法、private originator、成员回唤、防循环及 activity 归因；如何保证测试数据库隔离且没有静默跳过。
- Scope: internal; source-only research. Main session executes verification.
- Date: 2026-10-27
- Active requirement: 用户已明确改为模拟验证。先前 `codex-routing-continuation-20261027.md` 中的真实 runtime 续跑候选仅是历史研究，不能作为继续调用模型的授权。

## Findings

### 1. 数据库前置条件是硬条件

`server/internal/handler/handler_test.go:44` 的 `TestMain`：

1. **只读取 `DATABASE_URL`**；空值默认连接 `postgres://multica:multica@localhost:5432/multica?sslmode=disable`（`:46`）。`TEST_DATABASE_URL` 或 `TEST_DATABASE` 都不会替代它。
2. `pgxpool.New` / `pool.Ping` 失败会打印 `Skipping tests: ...` 并直接 **`os.Exit(0)`**（`:52`、`:57`）。此时可能只有 package-pass，而没有任何 test-run / test-skip 事件。
3. 不初始化 schema、不自动跑 migration。它直接创建测试 user/workspace/runtime（`:85`、`:104`）。
4. Setup 先执行 cleanup；cleanup 删除固定 slug `handler-tests` 的 workspace 和固定 email `handler-test@multica.ai` 的 user（`:170`）。因此即使 `-run` 只选纯 helper test，也不应连接用户运行中的 API 数据库。
5. 固定 fixture namespace 不适合同一数据库内并行启动两个 handler test 进程；选 `-p 1 -parallel 1`，并且不和其他数据库测试共用这份测试 DB。

使用一份**新建、独立、可丢弃、已迁移**的数据库。主会话应先记录其专用 container/server、端口、唯一 database name 和 `current_database()` 返回值，确认不等于 API 实际 database；不要从工作区 `.env` 推导测试目标。新库迁移前应没有应用业务表，避免误把一份运行库当作测试库。

仓库 CI 使用 `pgvector/pgvector:pg17`（`.github/workflows/ci.yml:490`）；这是可复用的测试数据库镜像基线。全量 migrations 包含必需的 pgcrypto / pg_trgm，以及可条件跳过的 pg_bigm / pg_cron 路径，直接使用与 CI 同一 PostgreSQL image 可减少不相关环境差异。这批选定的 handler tests 不需要 Redis。

正确的 schema 初始化入口是在 repo root 执行：

```bash
: "${SQUAD_CONTRACT_DATABASE_URL:?Set the explicit disposable test database URL first}"
DATABASE_URL="$SQUAD_CONTRACT_DATABASE_URL" go -C server run ./cmd/migrate up
```

`server/cmd/migrate/main.go:837` 同样只读取 `DATABASE_URL`，没有值时也会默认连接本机 `multica`（`:851`），所以显式传值不可省略。它按 `internal/migrations.Files` 查找、排序所有 up migrations（`server/internal/migrations/migrations.go:49`），错误退出非零。`make test` / `make migrate-up` 会加载本 checkout 的环境并调用 `ensure-postgres.sh`（`Makefile:383`、`:392`），不适合作为本次独立测试库命令。

### 2. 已核实的 24 个顶层测试

下列名单通过只读扫描当前 `server/internal/handler/*_test.go` 验证，正则恰好匹配 24 个顶层函数。没有执行 `go test -list`：它仍会先进入本包 TestMain，不能用作无数据库副作用的枚举工具。

| 验证面 | 现有测试与真实断言 |
|---|---|
| 普通 `@name` vs 完整链接 | `TestCommentMentionsAnyone`，`squad_comment_trigger_test.go:18`：literal `@alice` 为 false，agent/squad/member/all 完整链接为 true，issue cross-reference 不算路由 |
| 畸形 UUID 不 panic、不派发 | `TestPreviewCommentTriggers_MalformedMentionIDDoesNotPanic`，`comment_trigger_preview_test.go:1199`：5 个 case 同时验证 preview blocked/target_unavailable 和 POST 不给 assignee 排任务 |
| 服务端 roster 链接可往返解析 | `TestBuildSquadLeaderBriefing_MentionsRoundTrip`，`squad_briefing_test.go:357`：每个输出的 agent/member UUID 能被真实 `ParseMentions` 读回 |
| 精确小队链接创建 leader 任务 | `TestCreateComment_SquadMentionStampsSquadIDOnLeaderTask`，`squad_id_enqueue_test.go:19`：POST 后真实 queued row 同时带正确 squad_id 与 is_leader_task=true |
| Claim 角色与注入一致 | `TestClaim_LeaderTaskFromCommentMention_InjectsBriefing`、`TestClaim_NonLeaderTask_NoBriefing`，`squad_briefing_claim_test.go:133`、`:158`：直接调用 claim handler，检查 actual instructions / leader flag / leader_role_resolved |
| Private invoke 拒绝边界 | `TestShouldEnqueueOnComment_PrivateAgentGate`，`agent_access_test.go:700`：owner 可调用，普通 member、非 agent owner 的 workspace owner、无 allowed originator 的 A2A 均拒绝 |
| Squad mention 不绕过 private | `TestCreateComment_SquadMentionPrivateLeaderBlocksPlainMember`，`squad_comment_trigger_test.go:720`：评论保存成功但 private leader 的 queued count 仍为 0 |
| 真实 HTTP leader → private worker → private leader | `TestCreateComment_WorkerAgentCommentWakesPrivateSquadLeader_MUL4015`，`squad_worker_comment_wakes_leader_test.go:201`：leader HTTP comment 创建 worker row，originator 必须继承原 owner；worker 以自己的 task/parent 发结果后恰好创建一个 leader row |
| 已排 leader 不重复 | `TestCreateComment_WorkerAgentCommentDoesNotWakeLeader_WhenLeaderTaskPending`，同文件 `:109`：成员结果不能把已有一个 queued leader 变成两个 |
| 终态 leader self mention 不伪造成功 | `TestCreateComment_SquadLeaderSelfMentionCompletedTaskDoesNotFakeSuccess`，`comment_trigger_outcomes_test.go:259`：返回 blocked/self_trigger_suppressed，不能回 deferred，task 总数不变 |
| Completion 不循环重播普通 worker 评论 | `TestCompleteTask_DoesNotReconcilePlainWorkerReplyOnSquadIssue`，`comment_reconcile_test.go:383`：completion reconciliation 对 plain agent reply 不再创建 follow-up |
| No-action 的终态/评论约束 | `TestCompleteTask_SquadLeaderNoActionDoesNotSynthesizeComment`、`TestCompleteTask_SquadLeaderNoActionCanonicalizesTaskID`、`TestCompleteTask_SquadLeaderActionStillSynthesizesComment`、`TestCreateComment_SquadLeaderNoActionRejectsComment`，`squad_no_action_test.go:107`、`:122`、`:137`、`:152`：no_action 不合成 comment，大小写 task UUID 一致，action 正控制仍产出 comment，no_action 后新 comment 返回 409 |
| Activity 的当前 task/leader/tenant 归因 | `squad_evaluation_provenance_test.go` 的 8 个 `TestRecordSquadLeaderEvaluation_*`：guest/child leader 接受并保存正确 actor/squad/outcome；non-leader、无 squad、错 issue、错 actor、跨 workspace、已更换 leader 均拒绝且不伪造记录 |

完整且固定的 expected 顶层名单：

```text
TestCommentMentionsAnyone
TestPreviewCommentTriggers_MalformedMentionIDDoesNotPanic
TestBuildSquadLeaderBriefing_MentionsRoundTrip
TestCreateComment_SquadMentionStampsSquadIDOnLeaderTask
TestClaim_LeaderTaskFromCommentMention_InjectsBriefing
TestClaim_NonLeaderTask_NoBriefing
TestShouldEnqueueOnComment_PrivateAgentGate
TestCreateComment_SquadMentionPrivateLeaderBlocksPlainMember
TestCreateComment_WorkerAgentCommentWakesPrivateSquadLeader_MUL4015
TestCreateComment_WorkerAgentCommentDoesNotWakeLeader_WhenLeaderTaskPending
TestCreateComment_SquadLeaderSelfMentionCompletedTaskDoesNotFakeSuccess
TestCompleteTask_DoesNotReconcilePlainWorkerReplyOnSquadIssue
TestCompleteTask_SquadLeaderNoActionDoesNotSynthesizeComment
TestCompleteTask_SquadLeaderNoActionCanonicalizesTaskID
TestCompleteTask_SquadLeaderActionStillSynthesizesComment
TestCreateComment_SquadLeaderNoActionRejectsComment
TestRecordSquadLeaderEvaluation_AcceptedOnNonSquadAssignedIssue
TestRecordSquadLeaderEvaluation_AcceptedOnChildIssueBoundTask
TestRecordSquadLeaderEvaluation_RejectsNonLeaderTask
TestRecordSquadLeaderEvaluation_RejectsLeaderTaskWithoutSquadID
TestRecordSquadLeaderEvaluation_RejectsCrossIssueTaskAndNamesTaskIssue
TestRecordSquadLeaderEvaluation_RejectsForeignAgentWithoutLeakingTaskIssue
TestRecordSquadLeaderEvaluation_RejectsForeignWorkspaceTaskWithoutLeakingItsIssue
TestRecordSquadLeaderEvaluation_RejectsAfterLeaderChange
```

### 3. 精确执行命令和禁止真实模型

在 repo root、专用测试 DB 完成迁移后执行。下面变量只属于这次测试，不改变 shell 全局数据库目标：

```bash
: "${SQUAD_CONTRACT_DATABASE_URL:?Explicit disposable test DB is required}"
: "${SQUAD_CONTRACT_EVIDENCE_DIR:?Use an existing dedicated evidence directory}"
SQUAD_CONTRACT_REGEX='^Test(CommentMentionsAnyone|PreviewCommentTriggers_MalformedMentionIDDoesNotPanic|BuildSquadLeaderBriefing_MentionsRoundTrip|CreateComment_(SquadMentionStampsSquadIDOnLeaderTask|SquadMentionPrivateLeaderBlocksPlainMember|WorkerAgentComment(WakesPrivateSquadLeader_MUL4015|DoesNotWakeLeader_WhenLeaderTaskPending)|SquadLeader(NoActionRejectsComment|SelfMentionCompletedTaskDoesNotFakeSuccess))|Claim_(LeaderTaskFromCommentMention_InjectsBriefing|NonLeaderTask_NoBriefing)|ShouldEnqueueOnComment_PrivateAgentGate|CompleteTask_(DoesNotReconcilePlainWorkerReplyOnSquadIssue|SquadLeader(NoAction(DoesNotSynthesizeComment|CanonicalizesTaskID)|ActionStillSynthesizesComment))|RecordSquadLeaderEvaluation_(AcceptedOn(NonSquadAssignedIssue|ChildIssueBoundTask)|Rejects(NonLeaderTask|LeaderTaskWithoutSquadID|CrossIssueTaskAndNamesTaskIssue|ForeignAgentWithoutLeakingTaskIssue|ForeignWorkspaceTaskWithoutLeakingItsIssue|AfterLeaderChange)))$'
DATABASE_URL="$SQUAD_CONTRACT_DATABASE_URL" \
  bash scripts/go-test-with-agent-cli-guard.sh -- \
  go -C server test ./internal/handler -count=1 -p 1 -parallel 1 \
  -timeout=5m -json -run "$SQUAD_CONTRACT_REGEX" \
  > "$SQUAD_CONTRACT_EVIDENCE_DIR/handler-contract-tests.jsonl" \
  2> "$SQUAD_CONTRACT_EVIDENCE_DIR/handler-contract-tests.stderr.txt"
```

必须保存上述整个命令的实际 exit code。若使用 `tee` 展示日志，启用 `pipefail`，不能把 tee 的成功当测试成功。

`scripts/go-test-with-agent-cli-guard.sh` 会把 `scripts/agent-cli-command-names.txt` 的 Codex、OpenCode 等命令名替换成 PATH 前端 sentinel，执行时记录 redacted marker 并退出 126；只要发生任何环境 agent CLI 调用，wrapper 最终都会失败。它拦截的是 PATH 命令查找，不是通用网络/绝对路径沙箱。

所选测试没有启动 daemon/backend：`handler_test.go:68` 建内存 hub/event bus；HTTP 由 `httptest.NewRecorder/NewRequest` 驱动，任务是测试 DB 中的行；claim fixture 也只是调用 `testHandler.ClaimTaskByRuntime`（`squad_briefing_claim_test.go:18`）。`completeRunningTask` 使用本地 test helper 注入合成 daemon auth context（`squad_no_action_test.go:70`），不是对真实 API 发请求。因此这套测试不依赖模型账号、不消耗额度，也不需要模型凭据。

不要加 `-tags=agentintegration`、`MULTICA_RUN_REAL_AGENT_SMOKE=1`，不要扩大到真实 provider smoke tests。本轮不需要修改任何产品测试来获得模拟验收。

### 4. 必须证明没有跳过

通过条件全部满足才可写“24 个 handler 测试通过”：

- Guarded command 实际 exit code 为 0。
- JSONL 中上述 24 个名称**逐个**出现 `Action=run` 和 `Action=pass`；仅计算不带 `/` 的顶层 Test，并分别检查精确名称集合。嵌套 subtest 数量不能充当 24 个顶层测试。
- 不存在任何 `Action=skip` 或 `Action=fail`，包括 package 和 subtest。
- Output / stderr 不含 `Skipping tests:`、`no tests to run` 或 `unexpected agent CLI invocation:`。
- 所选 package 有末尾 `Action=pass`；若进程中途结束、JSON 被截断或只有 package-pass 而无 24 个顶层事件，拒绝宣称通过。
- 保留测试数据库身份、迁移命令/结果、源码版本，以及完整日志路径。不会把“没有 DB 所以 exit 0”或 `-run` 拼错零命中登记为成功。

可以让主会话把上面的固定 expected text block保存成 evidence 中的 `.txt`，再用本地 Python 解析日志，等价检查如下：

```python
events = [json.loads(line) for line in log_path.read_text().splitlines() if line.strip()]
expected = set(expected_path.read_text().splitlines())
assert len(expected) == 24
assert command_exit_code == 0
assert not any(e.get("Action") in {"skip", "fail"} for e in events)
top = lambda e: bool(e.get("Test")) and "/" not in e["Test"]
ran = {e["Test"] for e in events if top(e) and e.get("Action") == "run"}
passed = {e["Test"] for e in events if top(e) and e.get("Action") == "pass"}
assert ran == expected and passed == expected
assert any(e.get("Action") == "pass" and not e.get("Test") for e in events)
output = "".join(e.get("Output", "") for e in events) + stderr_path.read_text()
assert not any(marker in output for marker in [
    "Skipping tests:", "no tests to run", "unexpected agent CLI invocation:",
])
```

这是供执行方放入完整上下文的判定片段，不是产品新测试；它需要明确提供 log_path、expected_path、stderr_path 和实际 command_exit_code。

## Files Found

- `server/internal/handler/handler_test.go` — 整包 TestMain、DATABASE_URL 默认值、静默退出和测试 fixture 清理。
- `server/internal/handler/squad_comment_trigger_test.go`, `comment_trigger_preview_test.go`, `squad_id_enqueue_test.go`, `squad_briefing_test.go`, `squad_briefing_claim_test.go` — mention 语法、实际 enqueue、claim 角色和 briefing。
- `server/internal/handler/agent_access_test.go`, `squad_worker_comment_wakes_leader_test.go` — private invoke、真实 provenance 与成员返回。
- `server/internal/handler/comment_trigger_outcomes_test.go`, `comment_reconcile_test.go`, `squad_no_action_test.go` — 终态、自触发、防重复和 no_action。
- `server/internal/handler/squad_evaluation_provenance_test.go` — 当前 task/actor/leader/issue/workspace 的八项 activity matrix。
- `server/internal/handler/daemon_test.go:110` — `newDaemonTokenRequest` 测试身份 helper；不需真实凭据。
- `server/cmd/migrate/main.go:837`, `server/internal/migrations/migrations.go:49` — 显式测试 DB 的 migration 入口。
- `scripts/go-test-with-agent-cli-guard.sh`, `scripts/agent-cli-command-names.txt`, `scripts/test-go.sh` — 禁止默认测试调用环境 agent CLI，以及数据库 package 的顺序运行约定。
- `.github/workflows/ci.yml:490`, `:553` — CI PostgreSQL image 与先 migration 后测试顺序。

## External References

未查询外部来源；所有命令和测试名称来自当前仓库源码。未执行模型、CLI runtime、数据库或 API 操作。

## Related Specs

- `CLAUDE.md` 的 Testing / Verification：默认测试禁止访问已安装 agent CLI；数据库 fixture 使用 testutil；真实 smoke 需独立 build tag 和显式 opt-in。
- `.trellis/spec/server/builtin-templates.md`：精确 mention / 真正成员任务 / author/source_task 证据分离。
- `.trellis/spec/server/security-boundaries.md`：private 权限与 human principal 边界。

## Caveats / Not Found

- `TestCommentMentionsAnyone` 覆盖普通 `@name` 与完整链接，但这批现有 handler tests 没有单独命名的“精确缺失方括号”case。它由实际 parser 形状决定；若 scripted scenario 加这一负例，应记录为模拟场景证据，不声称现有 handler test 已专门覆盖该字符串。
- Handler tests 用合成身份调用 handler，不能替代完整路由中 task-token middleware 的独立 auth 测试。任务 scripted daemon/API 验证应另外回读其真实 task token 来源，但仍不得调用真实模型。
- 模拟 provider 的成功不证明真实模型遵守指令，也不证明第三方模型 API/网络出站限制。最后结果应称“模拟链路及服务端契约通过”，真实模型试点保持未验收。
- 本 researcher 只完成了源码匹配、固定 expected 名单和前置条件研究；实际 24 项测试、迁移和 no-skip 判定由主会话执行后才能写为通过。
