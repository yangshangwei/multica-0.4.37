# Research: M1 effective delegation and critical-path CI

- Query: Map existing `trigger_outcomes` production/consumption and fake execution CI; propose independently verifiable M1 slices without duplicating authorization, queues, or runtime mechanisms.
- Scope: Internal, read-only source research. No tests, databases, models, remote services, or Git operations were run.
- Date: 2026-11-05
- Checkout: `/Volumes/artisan/code/2026/mc-agent-rnd-m1-20261105`; parent supplied baseline `5aef9c19ad3e764d09cff939edfded53f8c513bd`.

## Findings

### 1. Ordinary comment dispatch already has trustworthy immediate outcomes

`server/internal/handler/comment.go:75` declares additive `trigger_outcomes` on create/edit responses only. Each item is `{target_type, target_id, status, reason_code}` (`:89`). `target_id` preserves the explicitly mentioned agent or squad UUID, rather than the executing leader UUID. No private target name or owner is returned.

The actual flow is:

1. Persist the comment, construct and publish its ordinary response (`comment.go:1886`).
2. Resolve the effective originator and compute dispatch admission (`:1910`, `:1955`). The comment remains saved even if one mention is blocked.
3. Deduplicate by executing agent and call the existing enqueue/merge/reconcile machinery (`:2029`).
4. Project actual results back to explicit targets (`:2222`) and return them with the created comment ID (`:1916`).

The status vocabulary and meaning already exist:

| Result | Existing behavior | What a consumer must not claim |
| --- | --- | --- |
| `queued` | A fresh task was enqueued. | The task has started, completed, or passed review. |
| `coalesced` | The comment was merged into an existing queued task; shared-leader squad aliases are also honestly represented as coalesced. | A separate run exists for every mention. |
| `deferred` | An active task's completion/reconcile path covers the comment, with the existing coverage rules. | A fresh task already exists or the result is available. |
| `blocked` | Admission or enqueue failed; `reason_code` explains the stable category. | HTTP 201 means that this target received the assignment. |
| Missing/unknown | A malformed mention may not parse; unknown fields/statuses can come from version drift. | Absence of `blocked` proves successful delegation. |

`comment.go:2070` documents the proof requirement before returning success-shaped statuses; `:2132` merges same-head queued work, `:2164` durably registers a lost-race input on an active task, and `:2205` returns truthful failure on non-convergence. Do not rebuild this queue logic merely to consume its result.

`server/internal/dispatch/reason.go` is the canonical stable reason vocabulary. It intentionally conceals nonexistent versus private targets using `invocation_not_allowed`. A new consumer should not infer extra entity information from that category.

Existing regression owners include:

- `comment_trigger_outcomes_test.go:30`: mixed allowed-agent/private-squad mentions save one comment and return partial outcomes; no blocked task is enqueued.
- `comment_trigger_outcomes_test.go:104`: unavailable/private target enumeration safety.
- `comment_trigger_outcomes_test.go:211`: two squads sharing one leader create one task and report queued/coalesced rather than two independent queued runs.
- `comment_trigger_outcomes_test.go:254`: completed leader self-mention does not fake success.
- `comment_duplicate_enqueue_race_test.go:75`, `:130`, `:207`, `:438`: concurrent queued/claimed winners, changed HEAD, honest coverage refusals.
- `comment_merge_failclosed_test.go:18`: failed merge preserves the original attribution snapshot.
- `squad_worker_comment_wakes_leader_test.go:31`, `:109`, `:201`: member updates, pending leader suppression, and private-leader wake behavior.

These files were inspected, not executed in this research assignment.

### 2. CLI already preserves the JSON; the leader protocol does not require consuming it

`server/cmd/multica/cmd_issue.go:622` defaults comment add to JSON. `runIssueCommentAdd` receives a raw JSON map and prints that complete map (`:2115`–`:2126`). Comment update does the same (`:2155`–`:2169`). `server/internal/cli/client.go:363` decodes the HTTP response without a bespoke comment projection. There is no missing transport field to add.

However:

- The CLI always prints “Comment added” to stderr before considering per-target results (`cmd_issue.go:2120`). Table mode omits the JSON entirely (`:2123`).
- `server/internal/handler/squad_briefing.go:39` teaches correct roster mention syntax but moves directly to recording `action` (`:52`) and stopping once the delegation comment is posted (`:67`). It contains no `trigger_outcomes` inspection, target matching, partial-refusal handling, or missing-receipt branch.
- `RecordSquadLeaderEvaluation` correctly verifies current-task provenance and the live squad leader (`server/internal/handler/squad.go:1261`, `:1275`, `:1321`), but stores only the caller-supplied `outcome` and `reason` alongside task/squad IDs (`:1341`). This is an evaluation record, not server-derived proof of delegation.
- `server/cmd/multica/cmd_squad.go:490` sends only `{outcome, reason}` to `/api/issues/{id}/squad-evaluated`. No trigger comment or target receipt is bound to that record.

Useful existing guidance is already present in `builtin_skills/multica-mentioning/SKILL.md:123` and `multica-working-on-issues/SKILL.md:281`: read the outcomes, distinguish blocked, and do not equate a saved comment with a started agent. The main leader protocol should align with this rather than introduce a competing protocol.

`server/internal/daemon/execenv/runtime_config_sections.go:796` also describes an agent mention as enqueueing a new run unconditionally. Any wording correction must preserve the existing no-courtesy-mentions and one-comment-per-turn rules, ownership status rules, and Agent Identity precedence.

### 3. Web already consumes outcomes; ordinary dispatch has no general durable receipt reader

`packages/views/issues/hooks/use-issue-timeline.ts:324` warns for blocked or unknown outcomes after comment creation/edit. `packages/core/issues/comment-trigger-outcomes.ts:59` validates individual entries, and `:76` explicitly whitelists `queued`, `coalesced`, `deferred`. Quick actions use the same response shape. No parallel UI status subsystem is needed for the first slice.

The ordinary response contract is intentionally private and ephemeral:

- `comment.go:75` says only create/edit responses populate outcomes.
- `comment.go:3463` explicitly excludes them from broadcasts.
- Unchanged edit content produces no new outcomes (`:3434`).
- Existing queued task lineage, `trigger_comment_id`, planned comment coverage, `source_task_id`, and activity rows help later reconciliation, but are not a durable per-explicit-target receipt for blocked/coalesced outcomes.

Therefore response-loss recovery cannot claim that `comment list` will reproduce the original receipt. A new persistent receipt or server-derived evaluation binding needs a separately reviewed contract; do not change `action` from “acted/delegated” to “all mentions accepted” silently. Likewise, adding a nonzero exit after a successfully committed comment can cause blind retries and duplicate comments if callers interpret it as a failed write.

Ordinary comment add has a `--request-id` flag (`cmd_issue.go:623`), but its help explicitly scopes it to collaboration writes. The ordinary add body at `:2108` does not send it. Do not claim ordinary comment POST is idempotent by request UUID without implementing and reviewing that behavior.

### 4. MCP is not a second first-party comment transport here

The first-party runtime brief requires the Multica CLI for platform resources (`runtime_config_sections.go:830`). Searches found no separate ordinary `create_comment` MCP tool to update.

Existing MCP mechanisms serve different boundaries:

- `server/internal/daemon/remote_mcp_broker.go:272`: task-scoped approved remote tools, call/concurrency bounds, upstream forwarding.
- `server/internal/daemon/plugin_hook_mcp.go:194`: known plugin tool lookup, then `invoke(taskID, installationID, hookKey, arguments)`.

Do not build a new MCP comment API as part of this slice. A plugin/remote tool that indirectly changes Multica is a separate capability/boundary review; ordinary CLI outcome consumption does not prove control over arbitrary external tools.

### 5. Existing critical-path fixtures can be wired into PR CI

`.github/workflows/ci.yml:3` runs for push/PR to `main`. It has frontend build/lint/typecheck, Vitest, backend race tests with PG17/Redis, vulnerability checks, and native collaboration lifecycle verification. There is no Playwright job in this workflow. `e2e/**` and `playwright.config.ts` are not currently inputs to its frontend/backend filters. A PR targeting the specified integration branch also does not trigger this workflow under the current branch filter.

M0 already added database precondition checks and release service/migration setup. Preserve them; a new browser smoke should not rerun the entire release pipeline or claim hosted execution from local YAML tests.

Reusable evidence primitives:

| Existing surface | What it proves when executed | Limits |
| --- | --- | --- |
| `e2e/core-business-journeys.spec.ts:17` G1-01 | Browser assignment confirmation, exactly one task, real daemon claim/start/progress/messages/complete APIs, live result and persisted comment | Simulated daemon; no provider executable or model |
| Same file `:65` G1-02 | Failed run, inbox notification, explicit retry lineage, cancellation before claim | Same limit |
| `e2e/core-business-agent-loops.spec.ts:174` G6 | Plain comment, explicit agent mention, `/note`, returned `trigger_outcomes`, matching queue/claim and threaded reply | Happy-path ordinary agent mention, not a full squad receipt matrix |
| Same file `:345` G8 | Streaming transcript, cancellation, task-token revocation, late completion stays cancelled | Protocol/service behavior rather than OS process termination |
| `e2e/fixtures/core-journeys.ts:103` `SimulatedDaemon` | Synthetic runtime through real HTTP endpoints, explicit execution lifecycle | Not the Go daemon binary or paid provider |
| `.github/workflows/ci.yml:627` native host lane | Named synthetic host lifecycle/recovery tests on Linux/macOS/Windows | Collaboration host boundary, not ordinary browser dispatch |
| `scripts/check-collaboration-native-tests.mjs:41` | Rejects failures, missing required names, skipped required parents/subcases | Can reuse the inventory pattern, not its unrelated required test list |

`e2e/workspace-defaults.spec.ts:162` already covers project squad preparation and enqueue, but intentionally never claims work (`:217`). `e2e/progress-reporting.spec.ts:353` covers report-template adoption and explicit fixture delivery; its body states no daemon/model is executing (`:398`). Do not present these as LLM reasoning evidence.

`playwright.config.ts:5` selects one worker and zero retries and expects already-running servers. `e2e/env.ts:5` may load `.env.worktree` or `.env`, so a CI launcher must supply isolated explicit values and not accidentally inherit another checkout's file. `scripts/check.sh:124` provides a full isolated verification flow, but it reruns static/TS/all Go checks before browser tests: using it unmodified as a narrow PR smoke would duplicate existing CI jobs.

## Recommended independently verifiable M1 child slices

### D1: Immediate delegation acknowledgement consumption

Smallest useful first delivery, independent of least-privilege schema work:

1. Make the dynamically injected leader protocol explicitly request/inspect JSON and match every intended roster target by exact type/UUID to the returned outcome.
2. Define `queued/coalesced/deferred` as handled admission, not member completion; blocked/missing/unknown targets remain undelivered. Preserve one-comment-per-turn, no blind repost after an uncertain write, and no automatic acceptance/stage advancement.
3. Add a CLI warning summarizer for non-success target outcomes that preserves stdout JSON and successful-comment exit behavior; preserve ordinary no-mention comments and old-server responses. Explicit missing expected-target detection belongs where the leader knows its intended roster, not in a generic CLI parser that guesses intent from every comment.
4. Align existing mention/working skills and source maps; update generated docs only if their source content changes. Dynamic protocol changes apply to fresh claims; existing saved template instructions remain owner content.

Suggested ownership: `server/internal/handler/squad_briefing.go` and its tests; `server/cmd/multica/cmd_issue.go` and one adjacent outcome/helper test; bounded runtime mention wording and relevant builtin skill source maps. Avoid queue, identity, and lifecycle mutations. Coordinate the shared `cmd_issue.go`/handler ownership with least-privilege work before dispatch.

Acceptance: fake HTTP CLI tests for queued/coalesced/deferred/blocked/partial/unknown/missing response shapes; stdout retains exact response fields; one HTTP POST; no extra trigger calls. Generated leader owner/guest briefing tests require exact-target validation and rejection handling while retaining their current status boundaries. DB-backed existing outcome tests can be reused if any handler flow is touched. This proves the tool/protocol contract; it does not prove a real model always follows it or deliver a durable receipt.

### D2: Server-derived durable delegation receipt

Separate design and acceptance, required before claiming loss/replay-safe effective delegation:

- Reuse existing enqueue outcomes and task/comment provenance. Store the server-observed result keyed by source task + trigger comment revision + target, including the policy/observed scope needed to interpret it; never accept a caller's status or reason as proof.
- Define exactly how the receipt covers coalescing, deferral, roster/permission changes, duplicate requests and response loss. Bind evaluation to the receipt or expose a dedicated scoped read; do not reinterpret historical `squad_leader_evaluated` rows as verified dispatch.
- Preserve permission-safe private feedback: only authorized actors can recover receipts, and no target names/owner IDs leak through denied results.
- First decide whether extending existing activity storage is sufficient. A new table or composite transaction is justified only by demonstrated durability/uniqueness/atomicity requirements. No generic workflow engine or second queue.

Negative acceptance: forged outcome, foreign/stale task, target substitution, replay across comment revisions, duplicate concurrent requests, accepted comment with rejected member, lost response and reconnect, permission revoked before subsequent execution. Test both persisted receipt and actual queue/input coverage. Current separate comment-save/dispatch ordering must be documented; adding receipt persistence cannot retroactively make those operations atomic.

This slice changes acceptance/evidence semantics and needs the main session's explicit design impact statement before implementation. It can follow D1 while least-privilege work stabilizes; don't include it in D1's completion claim.

### C1: Fixed PR critical-path smoke with named evidence gate

Independent from D2; may reuse finalized D1/least-privilege checks once available:

1. Add a narrowly owned CI smoke job or workflow for both relevant frontend/backend changes **and** its own fixtures/launcher/config. Include the integration branch if the intended review target is `codex/cross-workspace-collaboration`; otherwise document that only PRs to main run it.
2. Use a fresh runner-owned PG17 database, migrations, explicit identity check, synthetic legacy login config, production Next build/start, one API and one browser worker. Reuse current launch/health/ownership helpers where practical without re-running all existing CI jobs. Never start the real daemon or discover installed provider CLIs.
3. Run fixed existing G1-01/G1-02/G6/G8 cases and retain Playwright JSON, API/Web logs, source/runtime identity, screenshots/trace on failure. Existing simulated-daemon tests are sufficient for the browser→API→queue→reply chain. A separate synthetic Go provider/process check owns the actual executable boundary; label both accurately.
4. Fail the job when expected cases are missing, skipped, retried to green, or reported empty; retain first failure. Reuse the native inventory check's pattern, not its test names.
5. Aggregate the smoke result into a declared repository gate; YAML alone does not configure GitHub branch protection. Upload evidence on failure and clean up only owned processes.

Suggested ownership: `.github/workflows/ci.yml` or one dedicated smoke workflow, one launcher plus launcher/workflow/inventory tests under `scripts/`, explicit smoke config/list under `e2e/`. Existing G1/G6/G8 fixture source stays canonical rather than being copied into duplicate specs. This lane does not need business-source mutations unless the selected smoke demonstrates a real defect.

Acceptance: local launcher failure tests (missing DB/browser, wrong DB identity, missing test, skip, fail, timeout), then one real production-Web/isolated-API execution of the selected existing tests. Hosted workflow execution remains untested until separately authorized remote work. No model account or production credentials required.

## Files found

- `server/internal/handler/comment.go` — persisted comments, mention resolution, dispatch, response projection.
- `server/internal/handler/squad_briefing.go` — system-level leader mechanics and status ownership.
- `server/internal/handler/squad.go` — current-task/live-leader evaluation authorization and activity writes.
- `server/cmd/multica/cmd_issue.go` — comment add/update JSON transport.
- `server/cmd/multica/cmd_squad.go` — evaluation CLI payload.
- `server/internal/dispatch/reason.go` — stable nonenumerating reason codes.
- `server/internal/service/builtin_skills/multica-mentioning/` — existing receipt semantics and implementation source map.
- `packages/core/issues/comment-trigger-outcomes.ts` — safe outcome parsing and handled-status whitelist.
- `packages/views/issues/hooks/use-issue-timeline.ts` — existing user warning consumption.
- `.github/workflows/ci.yml` — current filters, service tests, aggregate gates, native evidence gate.
- `e2e/fixtures/core-journeys.ts` — real API test clients and simulated daemon.
- `e2e/core-business-journeys.spec.ts`, `e2e/core-business-agent-loops.spec.ts` — reusable browser critical paths.
- `scripts/check.sh`, `scripts/dev-env.sh` — local isolated environment lifecycle and production Web startup.
- `scripts/check-collaboration-native-tests.mjs` — existing named-test inventory pattern.

## Related specs

- `.trellis/spec/server/security-boundaries.md`: authorization principals and existing lifecycle transaction boundaries.
- `.trellis/spec/server/project-execution-squad.md`: runtime/roster readiness, setup versus execution, no silent instance rewrite.
- `.trellis/spec/server/builtin-templates.md`: delivered versus accepted work; no automatic replacement of custom instructions.
- `.trellis/spec/server/database-test-isolation.md`: own fresh database, `TestMain` executes under `-run`, private filesystem state, named test evidence.
- `.trellis/spec/web/frontend/e2e-run-environment.md`: production Web, explicit startup identity and browser cache ownership.
- `.trellis/workflow.md`: independently verifiable child artifacts and implement/check separation.

## External references / versions

No external source was consulted or network request made. The versions here are repository-declared: CI Node 22, Go 1.26.x, PG17/pgvector, Redis 7; root package declares Playwright `^1.58.2`. A workflow update should use the locked repository dependencies, not infer current third-party release behavior from this research.

## Caveats / Not Found

- This report covers ordinary issue/squad paths. Cross-account collaboration has its own retained event/run/command ledger and CLI routing; do not graft ordinary receipts onto it or weaken its existing provenance.
- No general durable ordinary per-target dispatch receipt read was found. This is based on the concrete comment response contract and searches, not a claim that every historical migration was reviewed.
- Same-head queue/input coverage is already sophisticated; preserve it. `comment.go:2083` documents a best-effort different-head queued blocker corner. D1 must not overclaim durable execution from `deferred`.
- Existing Go handler tests have database prerequisites even when testing protocol strings. Use `go test -c` for compile-only work, or a coordinator-owned database permit for execution.
- No runtime, browser, provider, CI job, or test was executed. All acceptance commands above are proposed verification, not passing evidence.
