# Final bounded adapter review

- Date: 2026-11-05 Asia/Shanghai.
- Worktree: `/Volumes/artisan/code/2026/mc-knowledge-codex-20261104`.
- Scope: final A2-R repair and preservation of previously reviewed A1/A2/CROSS-1 fixes, against the refreshed six-file `adapter-source.json`.
- Verdict: **PASS — zero remaining blocking findings in the reviewed adapter scope.** This closes A1, A2 and A2-R. It does not certify the final integrated fake suite, installed Codex behavior, real-model results or release readiness.

## Findings fixed by the author and verified

### A2-R: all generic turn-ID logs now use the private diagnostic helper

The remaining semantic-inactivity log at `codex.go:1767` and `first_item_wait` fields at `:1860` now use `codexDiagnosticID(b.cfg.PrivateInput, c.turnID)`. A search of all `turn_id` fields confirms that the first-turn timeout, semantic-inactivity timeout, first-item telemetry and terminal-event generic logs consistently use the helper. The separate diagnostic-error formatter can retain the raw turn ID internally; private result publication keeps that error in `KnowledgeOutput` and returns a content-free public error.

The sentinel controls are meaningful: `TestKnowledgeCodexPrivateTurnIdentityLogs` runs a test-owned app-server with the opaque native turn ID `private-sentinel-turn`. Its success case requires the correct current answer and actual first-item telemetry. Its inactivity case emits progress without completion, requires the expected knowledge failure and actual semantic-inactivity telemetry. Both assert that the raw sentinel is absent from logs/public error and that the hashed diagnostic identity exists. Neither changes the ID used for RPC correlation.

The retained red log shows **both controls failing before the two-line repair**. Saved normal correlation/privacy and race privacy runs pass after repair. This verifies the originally missed paths rather than merely testing the hash helper in isolation.

### Earlier fixes remain intact

- A1 still establishes the authoritative turn ID in `handleResponse` before the RPC waiter sees success; early notifications remain bounded and filtered by exact thread/turn identity. Request errors and invalid response IDs cannot become successful execution via queued notifications.
- A2 still normalizes unknown terminal statuses before generic logging while retaining the raw diagnostic privately. Private semantic activity/error/permission/unknown-method diagnostics remain guarded within the previously reviewed scope.
- The configuration helper still includes effective configured `env_key` and `env_http_headers` digests; B's CROSS-1 finding remains closed by the reviewed A implementation.

## Findings not fixed

None within this bounded review. The broader integration and actual-provider acceptance limits remain explicit; they are separate gates, not unresolved adapter defects found here.

## Exact source and evidence

All six current files matched the refreshed `adapter-source.json`:

| File | SHA-256 |
| --- | --- |
| `server/pkg/agent/agent.go` | `d44ad98c4b7b6ac42a3c5039762fb4ebca50fdacf8f123f8d1bc74a0f4d4915e` |
| `server/pkg/agent/codex.go` | `77423e30b59f7e1fcd2d93532d4c5baa9040e4318b16102cb85e8c320af865f1` |
| `server/pkg/agent/codex_knowledge.go` | `bf244f4675fdea9f9e304bfad32c33982ec7186cc8f6130ccbb7e1673da301cc` |
| `server/pkg/agent/codex_knowledge_test.go` | `376e0297fdad86ce8fe8660b83a7f02ebc7b0d266c0b3e7a031737eec068e1ea` |
| `server/pkg/agent/knowledge_launch.go` | `7f8ccc92938b63fcc2b65d412bbb83b46f87e922c415ba5e57c9ddb84fb6ecdb` |
| `server/pkg/agent/launch_lifecycle.go` | `723efd4247436a5f808019f4d62ab2883b9ad90867ad257ee92d8218c760b86d` |

| Evidence | Observed result | SHA-256 |
| --- | --- | --- |
| `adapter-a2r-red.log` | success and timeout controls both fail on sentinel leakage | `2237a5b97e672b80223c6c1e49d8f5bfb457151d91839d205b10e796c7e67d92` |
| `adapter-a2r-normal.log` | package pass, 4.171 s | `42ae1fb0adda84c0d8c6dca0d94384b4dfa1a8476eaacff9c9a65fc6973d1ce8` |
| `adapter-a2r-race.log` | package pass, 8.417 s | `f81b1ee400a555bb7b77f992e0d53c1d0ede9e638e83dea36af291e3701bbe33` |
| `adapter-a2r-repair.json` | author records vet/diff exit 0 and mutation after C normal02 released | `b6cad3fef612e225b19343af101e92fafb01a2dc3c9970c1645b9c6c5411326a` |
| `adapter-source.json` | six-file manifest matched | `07e01e256fe44c1d5da5c82adf09d7f6fd41add88f059d6665222526baa356c6` |

## Verification

- Formatting: fresh `gofmt -l` over the manifest files returned exit 0 and no paths.
- Static whitespace: fresh `git diff --check` over the manifest files returned exit 0.
- Vet: recorded author result is exit 0; not duplicated by reviewer.
- Type-check/tests: saved normal/race package passes inspected, no fresh duplicate execution. The logs are package summaries, not a verbose inventory recount.
- No product edits, installed CLI/model/credential/database calls or commits. Only this report was written.

Final integrated fake evidence must use the exact accepted source above; older normal02 evidence precedes this repair and must remain separately labelled. Current real execution and rollout gates remain owned by the main session.
