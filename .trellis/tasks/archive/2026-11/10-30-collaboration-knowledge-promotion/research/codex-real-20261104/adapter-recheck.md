# Independent A1/A2 repair recheck

- Date: 2026-11-05 Asia/Shanghai.
- Scope: exact frozen adapter manifest after A1/A2 repair; no product edits, tests, model/CLI/account/database commands or broad re-review.
- Verdict: **A1 and the original A2 status leak are fixed. One remaining private-log field omission requires correction before final adapter acceptance.**

## Fixed findings verified

### A1 — RPC-bound current turn

`codexClient.handleResponse` now matches the outstanding RPC ID and, specifically for strict `turn/start`, calls `bindKnowledgeTurn` before publishing the RPC result to its waiter. A valid nonempty `result.turn.id` establishes the current turn; RPC errors, invalid/missing IDs and an overflowed pending buffer produce a typed refusal. The strict request-error path no longer converts a previously queued completion into success.

`handleNotification` dispatches strict knowledge through `handleKnowledgeNotification` before the legacy mutable gate. That new path checks the armed exact thread and a valid turn identity before retention or state changes. Pre-response events cannot affect output, usage, errors or completion. They are retained within 256-event / 4 MiB bounds, then only events matching the response turn ID are replayed. Events for another turn or thread, missing provenance, unsupported notification families, and events after completion are excluded. The legacy gate remains for nonknowledge behavior.

The new regression sequences cover correct early notifications, old early/late notifications, old then current, missing/invalid response IDs, RPC error after early completion, wrong/missing RPC correlation IDs, provenance noise, event-count overflow and byte overflow. They assert that old answers do not enter either the public result or private evidence. The before-repair red log contains seven actual failed controls, including the original false-completion and unknown-status examples.

### Original A2 — Unsupported terminal status

For private execution, unsupported `turn.status` now logs the fixed category `unsupported`; the raw diagnostic is retained in the internal turn error and subsequently private `KnowledgeOutput`. The `unknown_status` control requires a failed public result, no sentinel in logs/public error, and retained sentinel in private evidence. Additional guards normalize semantic activity and suppress provider-controlled permission/unknown-method diagnostic values.

## Remaining finding (not fixed)

### A2-R — Two lifecycle logs still expose the raw opaque turn ID (blocking privacy omission)

Locations in the reviewed frozen `server/pkg/agent/codex.go`:

- line 1767: semantic-inactivity warning logs `"turn_id", c.turnID`.
- line 1860: `first_item_wait` telemetry logs `"turn_id", c.turnID`.

The repair added `codexDiagnosticID(private, id)` and correctly uses it for first-turn timeout and terminal logs, but these two fields were missed. `bindKnowledgeTurn` deliberately accepts a bounded opaque nonblank turn ID, not a canonical UUID. Thus a valid-shape RPC response using `turn.id="private-sentinel"` and matching started/completed notifications is admitted, and the normal `first_item_wait` log exposes that value. A matching incomplete turn also exposes it on semantic inactivity. Sanitizing only the terminal log does not close the generic logging boundary.

Required minimal repair: apply the same `codexDiagnosticID(b.cfg.PrivateInput, c.turnID)` at both fields. Add a fake response whose opaque turn ID contains a private sentinel, with normal completion and a bounded inactivity case, and assert no generic log contains the sentinel. Preserve raw identity in the private protocol evidence; do not rewrite the identity used for correlation. The main session was notified immediately. No product fix was performed by this reviewer.

## Frozen source and verification evidence

All six source files matched the new `adapter-source.json`. The pre-repair manifest is retained in `adapter-source-before-a1-a2.json`.

| Item | Observed evidence |
| --- | --- |
| `codex.go` | SHA-256 `02baee07c47d8b52e5b509314ec7fb6efe297eebdf3d8b173f6db7a132fdc1f3` |
| `codex_knowledge.go` | SHA-256 `bf244f4675fdea9f9e304bfad32c33982ec7186cc8f6130ccbb7e1673da301cc` |
| `codex_knowledge_test.go` | SHA-256 `ea85d108e4e735dacf7bed342a023b0c8d1ddbe815a87db37f81bb41d56edead` |
| Red controls | `adapter-review-red.log`: 7 failed subtests; SHA-256 `de18321268f1eb2150b510a3521b2416ea3866c3c9e83badc14729adad062c32` |
| Normal | `adapter-review-normal.log`: package pass, 5.482 s; SHA-256 `7950981f795b0ed0d095e1ed059ef9084f414eddeafa74ec629856f84fe9c70b` |
| Race | `adapter-review-race.log`: package pass, 47.416 s; SHA-256 `7701294aa612f826525aba14834ed60fa681ecd86f222217ab55bd4cd82e729c` |
| Formatting | Fresh `gofmt -l` over six manifest files: exit 0, no paths |
| Diff whitespace | Fresh `git diff --check` over six manifest files: exit 0 |
| Vet | `adapter-review-repair.json` records exit 0; not rerun by reviewer |

The ordinary/race logs are terse package summaries, so they establish the saved package outcomes, not a newly recounted verbose test inventory. The source inspection confirms that the new controls are present. No duplicate test execution was launched while C prepared integrated normal2.

## Handoff

Correct A2-R, retain the original red/green records, freeze fresh source hashes and rerun the narrow private-ID controls under normal/race. Final integrated fake evidence must bind that final source. The current source does not yet receive an unconditional adapter pass; this recheck does not claim real-model, installed ephemeral-policy or release acceptance.
