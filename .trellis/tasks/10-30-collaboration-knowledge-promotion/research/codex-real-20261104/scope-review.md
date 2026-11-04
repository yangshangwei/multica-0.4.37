# Codex continuation scope review

- Date: 2026-11-04
- Scope: read-only review of the continuation proposal, compatibility report, authorization record, installed protocol record and relevant product source. Only this report was written.
- Verdict: **the identified product-support gap is correct; the proposal is suitable for a scope decision, but needs the contracts below before implementation admission.** No Codex product behavior or real-model acceptance is established by this review.

## Confirmed facts

1. `KnowledgeLifecycleSupported` and `ValidateKnowledgeSessionArgs` in `server/pkg/agent/knowledge_launch.go`, the ordinary lifecycle gate, service retained/current claim validation, and collaboration negotiation currently admit only built-in local Claude. Codex generic process supervision does not imply managed-knowledge support.
2. Codex uses app-server JSON-RPC. `codex.go:902` enters the generic lifecycle, `:1512` starts/resumes a thread before `turn/start`, and `:1882` implements resume fallback. Its outer execution loop can create another process after initialize/catalog failure. The proposed adapter and server/daemon/host work is necessary; an executable replacement would test a different path.
3. The ordinary configuration namespace currently hashes Claude-specific arguments/environment (`knowledge_context.go:78`); a Codex namespace must reflect its actual config/storage sources. The frozen invocation additionally needs the effective Codex options, including service tier and thread persistence policy where used.
4. The installed protocol record proves that Codex 0.165.0 exposes `ThreadStartParams.ephemeral`. It does not prove that the application sends the field, that a response honors it, or that the resulting thread is not recoverable after process termination. `ThreadResumeParams` has no corresponding field. Current `codex.go:1952` sends `persistExtendedHistory: true`; that is not a substitute for an ephemeral policy.
5. The new authorization record accurately retains the user-selected Codex and local configured model, caps physical measured executions at 15, records 0 executions, and leaves the entry non-runnable. E0b/K1/K2/K3 counts are 1/4/2/8, with three separately labelled fake seeds. These counts are not underlying model HTTP-request counts. The retained real execution package and completed fake reports remain historical evidence, not Codex acceptance.

## Required additions to the implementation contract

### R1 — Define the exact RPC input and native-session publication boundary

The plan mentions native ACK but should explicitly require this order: physical launch authorization → actual thread/start or exact thread/resume → validation of the returned native thread ID → synchronous server observation ACK → actual knowledge-bearing `turn/start`. Missing, mismatched or unacknowledged thread identity must send no knowledge-bearing turn and publish no reusable pointer.

`codexTurnInput` can prepend a continuity notice after thread selection (`codex.go:1870`). Either the final prompt must already contain the exact admitted notice, or the adapter must prove that the actual RPC input is byte-equivalent to the prepared prompt. A new thread after rejected resume cannot be treated as the old session or receive the old input automatically. Define whether any pre-input initialize retry is allowed; if allowed, each physical process needs a new authorization and ledger entry. The simplest initial knowledge path is no automatic retry. Preserve existing non-knowledge recovery behavior.

Required controls: denied first start; wrong/missing thread ID; failed ACK; rejected resume; changed fingerprint; injected continuity text; attempted second process after a knowledge failure; positive exact resume. Assert process counts, `turn/start` counts and pointer/observation ordering separately.

### R2 — Preserve private failure/result semantics, not only successful session ACK

This is absent from the current high-level plan. `agent.Result` already has `KnowledgeFailure` and `KnowledgeOutput`, and the Claude adapter populates them after terminal lifecycle failures. Codex currently returns generic failures and attaches sanitized stderr/error notification text (`codex.go:1444,1581,1713,3299`). Generic pattern sanitization does not prove that private knowledge text is absent from diagnostics.

The Codex knowledge path must preserve already observed usage, process termination and private output in the existing private evidence fields, return the precise terminal knowledge reason, and keep provider stderr/private error bodies out of generic runtime logs. This must include asynchronous `error` notifications, thread/turn request errors and process-exit failures. Keep ordinary diagnostic behavior unchanged when knowledge is absent.

Required controls: a unique private sentinel in stderr and RPC errors cannot appear in generic logs; missing ACK and revoked second attempt retain earlier usage/termination without replay; precise failure classification reaches existing task retry exclusions.

### R3 — Make provider/version/configuration compatibility explicit before broadening admission

Use a minimal supported Codex invocation contract covering actual argv and JSON-RPC settings. An allowlist change alone is insufficient. Review config/profile selectors and default/custom options against the installed CLI; bind the same effective provider/model/account-storage configuration from negotiation through the physical adapter. Do not serialize credential values into evidence.

Ordinary and host semantics differ: ordinary K2 needs controlled persisted native history; host K3 requires fresh, nonpersistent threads with no reusable pointer. The host capability must refuse an older CLI or an unsupported/ignored persistence setting rather than remove it. Reuse `PrepareCollaborationCodexHome` and current containment helpers, but verify their current custom-provider credential/config behavior; no host-home mechanism has been revalidated in this review.

Retain the strict existing protocol shape when possible. If any wire descriptor/session field changes, follow the documented protocol-version bump and older-daemon compatibility rules; merely adding a new provider string does not by itself require a migration. Preserve downgrade/history checks for Codex once exposure exists. No database schema or UI change is currently demonstrated as necessary.

### R4 — Capture Codex traffic and start counts as Codex, with a durable stop cursor

The existing harness reads Claude stream-json and counts its pass-through stand-in as fake. The new capture path must preserve the bidirectional JSON-RPC stream, bind the actual thread and turn, and require an authoritative terminal turn event. Do not score a standalone answer or a translated pretend-Claude result as product proof.

Record the actual underlying Codex process start before it can make model requests, including incomplete attempts; separately record wrapper processes and fake seeds. A failed final evidence write must prevent a later unaccounted launch. Preserve the 15-execution cap without automatic extra health checks, retries or model substitutions. Existing finite source/role fixtures and semantic assertions are reusable, but expected answers must not enter the real child's input/config/environment except through the admitted knowledge route.

## Minimal ownership and execution order

| Slice | Files / responsibility | Exit evidence |
| --- | --- | --- |
| A — Codex adapter | `server/pkg/agent/codex.go`, `knowledge_launch.go`, `launch_lifecycle.go`, necessary `ExecOptions` field in `agent.go`, adjacent agent tests | Fake app-server tests for exact input/session ACK, no replay, private result/log handling, invocation/persistence; normal and race |
| B — Ordinary integration | `server/internal/daemon/knowledge_context.go`, `server/internal/service/task_knowledge.go`, `task_knowledge_prepare.go`, adjacent daemon/service tests | Codex fresh/resume/revoke/rebind and unsupported-client history matrix; explicit fresh task-owned DB where needed |
| C — Collaboration integration | `server/internal/daemon/collaborationrun/host_knowledge.go`, `host_runner.go`, `server/internal/service/collaboration_knowledge_runtime.go`; existing host-home helper only if a demonstrated gap requires it | Actual ephemeral RPC and unsupported-version refusal; run-scoped input/authorization and no persisted pointer; ordinary behavior unaffected |
| D — Harness | Existing six `rk1k` files and tagged daemon driver; coordinator-owned separate registration/launcher/ledger | Full Codex fake normal/race; exact 1/4/2/8 + three fake-seed counts; private durable evidence faults; no model requests |
| E — Independent verification / execution | Read-only review of A–D, current source/CLI/config/account binding; main owns real window and ledger | Fixed E0b → K1 → K2 → K3, stop at first incomplete result, distinct model/input verdicts |

Freeze A's option/lifecycle interface first. B and C can then proceed in independent file sets; D consumes those settled interfaces. Do not concurrently edit the shared agent contract from multiple lanes. The main session owns specification/operation-guide updates, final integration and the user scope decision. Required docs include `collaboration-knowledge-execution.md`, `collaboration-context-experiments.md`, `docs/collaboration-knowledge-runtime.md` and the current task artifacts; update built-in skill sources if their documented provider behavior changes.

## Findings fixed

None. Product edits and changes to the proposed plan were outside this review's write scope; all design additions are reported for the coordinator.

## Findings not fixed

R1–R4 are implementation-design requirements, not claims of regressions in the current Claude-only product. The product expansion decision remains with the main session/user. Real model connectivity and runtime ephemeral behavior remain unverified.

## Verification

- Source/plan consistency: reviewed against the cited concrete paths; no product execution.
- Lint: not run; report-only scope, no product edits.
- Type-check: not run; report-only scope, no product edits.
- Tests: not run; no database or real-agent execution authorized for this reviewer.
- No global Codex configuration or credentials were read. No old ledger or evidence was modified.
