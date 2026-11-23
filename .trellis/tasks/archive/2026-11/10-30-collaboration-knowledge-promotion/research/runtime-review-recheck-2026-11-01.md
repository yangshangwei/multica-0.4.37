# Runtime privacy recheck — 2026-11-01

Verdict: **RR1 resolved at its original daemon stderr sink; new residual RR2/P2 remains open. G2 remains open.** The original review reports are preserved.

## RR1 disposition

The seven-file correction removes the raw Claude stderr forwarding sink for both positively negotiated knowledge paths. `Config.PrivateInput` is a per-backend logging policy, set from the admitted descriptor at ordinary and collaboration backend construction. It does not change shared logger state, execution support, credentials, invocation/session semantics or lifecycle authority. Legacy backend instances keep existing logging.

The private backend still retains the existing 2048-byte stderr tail and applies the existing common-secret/control-character sanitizer. Provider failure carries that bounded diagnostic in the authorized error/outcome path. An ordinary knowledge refusal carries it in `KnowledgeOutput`, retains its canonical content-free `Error`, and leaves failed `Output` empty. Success retains its original final output. Arbitrary source text remains private evidence despite sanitization.

The new tests exercise actual approved stdin echoed to stderr in all five ordinary/collaboration success/failure/refusal cases. The separate agent test alternates private/legacy/private runs through the same logger and verifies lifecycle logs survive. Retained red logs show all five private-log assertions fail before the correction; the same cases pass in green and race runs. Main's strict observation ACK, C4 mandatory persistence policy and ordinary claim-fence wiring are unchanged.

## RR2 — P2: preserved private stderr is re-logged by ordinary server failure handling

Two remaining generic server logs consume the authorized failure text:

- `server/internal/service/task.go:5157`: `slog.Warn("task failed", ..., "error", errMsg, ...)`.
- `server/internal/handler/daemon.go:4755`: `slog.Info("task failed", ..., "task_error", req.Error, ...)`.

Concrete path: a negotiated ordinary Provider receives an approved private stdin sentinel, echoes it to stderr and exits nonzero. The corrected Claude backend retains the bounded tail in `Result.Error`. `runTask` returns it as blocked `TaskResult.Comment`; `reportTaskResult` sends it to the real `/fail` endpoint. The two server logs above then persist that private text at ordinary Warn/Info levels. The stderr sanitizer deliberately preserves arbitrary private body text, so it does not close these sinks.

The new stderr fixtures stop at `runTask` with fake API responses, and therefore cannot observe real server logs. The newly passing ordinary composite does call real failure handlers, but its current failure scenarios use content-free knowledge refusals rather than a Provider failure carrying echoed private stderr.

Minimal owner fix: omit raw error text from these two log records whenever the returned/persisted task carries knowledge metadata. Use the authoritative task row, rather than trusting only a client flag or failure classification; ordinary Provider failures do not necessarily have a `knowledge_*` reason. Retain the original task error and authorized transcript/outcome unchanged. Add a real service/handler or ordinary composite regression that captures the server logger and proves private stderr is absent while the bounded failure evidence is retained. No product fix or test execution was performed by this reviewer.

## Source and retained-evidence audit

All seven correction source hashes and all six dependency hashes match current files; before/after correction snapshots agree. The three hashes changed among the previous runtime review's captured source set are exactly `claude.go`, ordinary `daemon.go` and collaboration `host_runner.go`. The additional `agent.go` field and three new test files are explicitly included in the correction's seven-file ownership inventory. The current strict ACK implementation and tests still match their previously reviewed hashes.

All 15 correction artifact hashes and all 40 ordinary-composite artifact hashes match. The ordinary composite's complete 4219-file source snapshot matches current files. Its final raw race log has all 27 listed pass events across 14 top-level tests, with no failure or skip. This closes the previously unrun **ordinary server/database/daemon composition** acceptance slice for its stated deterministic scope. Actual HTTP handlers, live daemon WS claim with no HTTP fallback, real transactions, fake Provider stdin, original-key retries and refusal/cleanup behavior are wired in the tagged fixture.

Counts below are top-level passes / pass events including subtests and are not additive unique-test counts.

| Retained suite | Passes | Skips / failures |
| --- | --- | --- |
| Correction agent regressions | 93 / 195 | 1 platform case / 0 |
| Correction full ordinary daemon | 888 / 1676 | 2 platform cases / 0 |
| Correction full collaborationrun | 113 / 332 | 0 / 0 |
| Correction agent race | 114 / 229 | 1 platform case / 0 |
| Correction ordinary daemon race | 56 / 256 | 0 / 0 |
| Correction collaborationrun race | 71 / 168 | 0 / 0 |
| Ordinary real-server/DB/daemon composite race | 14 / 27 | 0 / 0 |

The skipped cases are explicitly retained in the JSON inventory: root/sudo behavior under a non-root runner, the Linux collaboration batch runner, and Windows drive-root behavior. No new stderr regression was skipped. Correction build/vet/whitespace commands and ordinary tagged static checks report exit zero.

## Remaining gates

RR2 is an open implementation privacy finding. The C4 real HostExecutor/server/database composite was still running at this recheck, so no full G2 pass is issued. Final platform/source acceptance and any separately authorized real Provider/account/model matrix also remain distinct. This reviewer ran only read-only source/evidence inspection and wrote this report plus its JSON companion; no database, test, Provider, product edit, commit or sub-agent execution occurred.
