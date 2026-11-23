# S4 preparation backend foundation

- Active task: `10-20-collaboration-gap-recovery`.
- Date: 2026-10-21.
- Owner: `s4_preparation_backend`; protocol/service/handler/SQL/migration slice only.
- Source baseline: `8ecb98883504443a6ec1a5fb4a4c6295a985ae04`, with the existing shared worktree changes preserved.
- Environment: dedicated `service_s4` database provided by the coordinator; Go 1.26.6, darwin/arm64. Database URL remains in the coordinator's private wrapper.
- Status: bounded backend foundation implemented and verified; ready for independent review. Full S4 / AC-I06 remains open.

## Implemented contracts

1. `protocol/collaboration_preparation.go` defines independent preparation/bundle version 1, complete capability reports, exact run/resource/command/file/version correlation, bounded limits, fixed Git bundle/full OID metadata, private invocation, and typed preparation observations. Existing runtime identity/proof protocol remains 2. The private manifest and its digest stay on the negotiated runtime boundary; neither enters human records. Attachment content hashes, storage keys, encryption keys, host paths and account values are excluded. `command_ids` bind the actual prompt without adding a prompt-body hash.
2. `agent_runtime.collaboration_capabilities` is a dedicated protected column. Its complete registration stores the **post-registration** runtime configuration epoch. A companion trigger sorts before the existing runtime configuration trigger and requests one advance through the existing epoch/outbox contract. Ordinary registration metadata cannot replace this column. Current identical reports are no-ops; a later execution configuration change leaves the report visibly stale until explicit full registration with the current observed epoch. Existing owner consent/fingerprints remain authoritative.
3. Preparation-dependent promotion checks all selected frozen resources before creating a group/run/reservation. Registered prepared runtimes also validate their supported invocation settings. Unsupported preparation leaves the input waiting without inventing a permanent hold or changing queue controls. A healthy legacy text-only issue can still claim the same namespace.
4. First negotiated claim atomically freezes `prompt_snapshot` and `preparation_snapshot`. The file set contains the actual prompt's references and explicitly delivered issue files at that boundary. Replays retain exact versions and exclude later delivery. A previous legacy claim cannot be upgraded by retroactively injecting preparation. Claim response adds `preparation` only after explicit `preparation_version:1` plus matching bundle digest; old no-file responses keep their original six fields.
5. All preparation/byte/observation operations use the exact current `mcb_` run via `lockRuntimeBusiness`, frozen manifest, original binding and reservation checks. Attachment bytes use `h.collaborationAttachmentService()` and the real encrypted storage adapter. Storage I/O occurs after the first authorization transaction closes; the same mcb/manifest/version/row validation runs again before sending bytes. Existing two-slot buffering, private headers and controlled downloads are reused.
6. Observations retain the caller's original event ID, manifest, attempt sequence and nonce in `collaboration_run_event`. Repeated identical observations return the original correlation; changed payloads conflict. They do not write provider outcome, proof, launch permission or capacity release. The existing protocol-1 human record stream excludes preparation events so installed clients continue reading their known item kinds. An optional human preparation summary is a later S4 integration item.
7. A late delivery after admission is handled only for an untouched run with no claim, termination capability, attempt, business capability or runtime event. Roots can retain an independently unclaimed M11 continuation; members/continuations require full retained parent/group/delegation validation and no outgoing work. Only the refused run receives the existing affirmative server never-permitted fact. The shared post-proof effects settle its delegation, schedule the normal member wake and settle a group only when its full ledger permits it. The original run identity, other live runs and queue/hold controls remain intact. Rejected members and their independently rejected continuations leave the pending-derived set, so a healthy root actually gets admission. Required-write errors roll back all effects; unknown commits never become a reported successful refusal or an automatic replay.
8. `preparation.invocation` requires explicit `instructions`, `model` and `thinking_level` strings, frozen from the actual locked agent in the first prompt transaction. Empty model/effort selects the authorized runtime default. Replay uses the retained snapshot. Unsupported settings receive finite public category codes; raw values never enter errors/events/logs. Existing owner/configuration epoch checks still reject stale execution authority.

## Interfaces for the coordinator / daemon lane

| Route | Handler | Authority |
| --- | --- | --- |
| POST `/api/collaboration/runtime/capabilities` | `RegisterCollaborationRuntimeCapabilities` | Current managed daemon and full observed-epoch report |
| GET `/api/collaboration/runtime/capabilities?runtime_id=UUID` | `GetCollaborationRuntimeCapabilities` | Same managed runtime; current and recorded epochs for recovery |
| GET `/api/collaboration/runtime/runs/{runId}/preparation` | `GetCollaborationRuntimePreparation` | Current run mcb |
| GET `.../{runId}/attachments/{attachmentId}/content?version=N` | `GetCollaborationRuntimeAttachmentContent` | Same run, frozen file/version and final-send revalidation |
| POST `.../{runId}/preparation/observations` | `RecordCollaborationRuntimePreparation` | Same manifest/attempt mcb; original event ID |

The byte route returns `X-Multica-Run-ID`, `X-Multica-Attachment-ID` and `X-Multica-Attachment-Version` only after final authorization. The downloader must also verify status and exact byte length. Paths are `attachments/<UUID>/<decimal-version>`; display names never form filesystem paths.

Version-1 limits are explicit in registration: file count 1–32; per-file bytes 1–100 MiB; total input bytes 1–256 MiB; workspace bytes at least total input and at most 512 MiB; Git expansion 1–256 MiB; entries 1–65536; timeout 1–600 seconds. Typical fixture/local defaults are 32 / 100 MiB / 256 MiB / 512 MiB / 256 MiB / 4096 / 60. No server-side omission defaults enable support. Adapter IDs are `static_json_v1` and `claude_stream_json_v1`, version 1, fresh session and no fallback. Canonical provider/family comes from the existing registry. The server checks installed Linux OS; architecture is the daemon's explicit probe report because the existing installation table has no architecture column.

## Files changed in this lane

- `server/pkg/protocol/collaboration_preparation.go`, its test, and optional negotiated fields in `collaboration.go`.
- New `server/internal/service/collaboration_runtime_preparation{,_capabilities}.go` and corresponding tests.
- `server/internal/service/collaboration_runtime_invocation.go` and invocation/derived-preparation regressions.
- Minimal preparation integration in `collaboration_runtime_capabilities.go`; pre-promotion check and named object-refusal classification in `collaboration_runtime.go`.
- New `server/internal/handler/collaboration_runtime_preparation.go` and handler tests.
- `server/pkg/db/queries/collaboration_preparation.sql`; the known-kind filter in `collaboration_records.sql`.
- Migration `737_collaboration_runtime_preparation.{up,down}.sql` and `server/internal/migrations/collaboration_runtime_preparation_test.go`. No new indexes or foreign keys.
- Narrow approved fixture correction in `TestCollaborationSquadUnreadAuthorDepartureDoesNotStopOtherWork/consumed`: complete the actually offered member, re-list and claim its continuation, assert input consumption, then retain the original departure/Stop assertion.

The coordinator owns generated sqlc files, route wiring, shared schemas/API clients, built-in references and specs. This lane created no commits and did not archive a task.

## Failure-first and integration findings

Bounded reuse plan followed for the final derived-run fix: the existing post-proof effects
(own capability revocation/reservation release, delegation settlement, member
wakeup and group settlement) moved into one helper without changing the
accepted-proof path. The already passing runtime/squad regressions protect that
behavior. New RED cases first covered an unclaimed member/continuation blocking a
healthy root, a live sibling, an issued claim, a capable runtime receiving an old
request, and failed required writes. The helper now also serves only a
fully validated run with affirmative zero claim/attempt evidence. The existing
independent-issue test setup was extracted for reuse; its assertions remain in
the original test.

- Before implementation, `TestCollaborationRuntimePreparationLegacyRejectsFilesWithoutClaiming` failed because the legacy claim silently accepted delivered files. The paired no-file six-field wire control passed.
- Before the promotion guard, `TestCollaborationRuntimePreparationUnsupportedFilesDoNotReserve` failed because unsupported file work produced a reserved candidate.
- `TestCollaborationRuntimePreparationObservationKeepsInstalledRecordsReadable` reproduced the existing record reader's rejection of the new event kind. The narrow query predicate repair preserves the installed item contract.
- The first adjacent race run had 60 top-level passes and one old fixture failure: the consumed-author test searched only the first candidate page and never proved it had claimed a continuation. The approved fixture repair follows actual member priority and preserves all behavioral assertions.
- The late-delivery ordering (no-file promotion → public file delivery → legacy claim) reproduced a remaining reserved, never-claimed run. The final factual-refusal regression protects the input, original run identity, controls, issued claims and commit boundary.
- Derived RED cases reproduced a permanently pending member and a blocked healthy root. GREEN follows the actual member → wake → continuation → healthy-root schedule. A second RED case covered M11 joining before the reserved root's first claim; each untouched run now converges independently and both original inputs keep their original sequence. Live-sibling, issued claim/permit, capable-runtime old request, temporary evidence-read failure and required-write rollback controls retain their barriers.
- Invocation RED cases demonstrated discarded instructions/model/effort and admission despite unsupported configuration/enabled skills. The frozen invocation and pre-admission category gates now pass, including valid explicit empty settings and retention after subsequent configuration changes.
- Intermediate fixture corrections: toggle the actual runtime mode instead of setting its existing value; preserve chi route context when adding managed authentication; expect the actual revoked mcb 403; use a nonempty `testutil.Cols` insert. These were fixture defects, not evidence of a relaxed production contract.

## Verification

From `server/`, the dedicated wrapper injects the private database URL and pinned Go toolchain:

```bash
python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s4 bash ../scripts/go-test-with-agent-cli-guard.sh go test -race ./internal/service -run '^TestCollaboration(Runtime|Managed|Termination|RemovalOfNonRoot|GroupDepend|Squad|Batch)' -count=1 -json
python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s4 bash ../scripts/go-test-with-agent-cli-guard.sh go test -race ./internal/handler -run '^TestCollaborationRuntimePreparationHTTP' -count=1 -json
python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s4 bash ../scripts/go-test-with-agent-cli-guard.sh go test -race ./internal/migrations -run '^TestCollaborationRuntimePreparationMigration' -count=1 -json
GOTOOLCHAIN=go1.26.6 go test -race ./pkg/protocol -count=1 -json
GOTOOLCHAIN=go1.26.6 go vet ./pkg/protocol ./internal/service ./internal/handler ./internal/migrations
```

Before the final late-delivery follow-up, actual results were: adjacent service race **63 top-level + 67 subtests passed**; preparation-only race **15 top-level + 27 subtests passed**; handler race **2 top-level passed**; migration race **1 top-level passed**; protocol package, vet, gofmt and `git diff --check` passed. All named executions had zero skips. Earlier logs remain under `/tmp/multica-gap-recovery-20261021-h6phxwo9/` as `s4-backend-{service-race-final,preparation-race,handler-race,migration-race}.jsonl`; the expanded final results follow.

Final verification after invocation freezing, derived/M11 refusal convergence,
and shared post-proof effects:

| Scope | Actual result | Retained log |
| --- | --- | --- |
| Service race, preparation plus adjacent runtime/managed/squad/batch/group-dependency cases | 76 top-level + 82 subtests passed; 0 failed, 0 skipped | `s4-backend-final-service-race.jsonl` |
| Handler race | 2 top-level passed; 0 failed, 0 skipped | `s4-backend-final-handler-race.jsonl` |
| Protocol package race | 5 top-level + 12 subtests passed; 0 failed, 0 skipped | `s4-backend-final-protocol-race.jsonl` |
| Actual migration 737 in a disposable schema, race | 1 top-level passed; 0 failed, 0 skipped | `s4-backend-final-migration-race.jsonl` |
| `go vet` for protocol/service/handler/migrations | exit 0 | Tool execution recorded in the session |
| Owned Go files `gofmt -l`; worktree `git diff --check` | empty output, exit 0 | Tool execution recorded in the session |

All final logs are under the same private task directory listed above. The full
service selector includes the original proof/termination regressions after the
post-proof helper extraction; this is not a count of only newly added tests.

## Remaining scope

- This verifies service/protocol/handler state and real local encrypted storage on macOS, with synthetic managed identities and proof payloads. It does not verify a Linux kernel process tree, native provider execution, Git extraction, actual provider accounts or quota, or Windows/macOS containment support.
- The daemon must probe the actual explicit bundle, negotiate the full capability, stage exact bytes, validate final current authority, and preserve its existing nonce/permit/proof ordering. Those changes belong to the daemon lane.
- Native adapters must consume the already frozen invocation exactly. Unsupported settings below need a separately implemented configuration/materialization contract before their refusal can be removed.
- Add a separate optional human preparation/provenance summary and shared-client handling without exposing unknown item kinds to installed readers.
- Root owns full repository checks, router authentication integration, built-in guidance, Web/Electron validation and the complete acceptance ledger. Creation/execution rollout remains gated; full S4 and parent completion are not claimed here.

## Invocation support matrix

| Setting | Prepared v1 behavior / refusal code |
| --- | --- |
| Agent instructions | Exact UTF-8/NUL-free text, at most the existing collaboration text bound of 64 KiB |
| Agent model | Exact runtime-native safe token, explicit new wire limit 256 bytes; slash/colon/context-window modifiers are retained; empty means default |
| Agent thinking level | Exact token, at most 64 bytes and the existing pure provider literal gate; empty means default |
| Unsafe/oversize invocation | `unsupported_provider_invocation`; never truncate or coerce |
| Nonempty custom environment | `unsupported_provider_custom_environment` |
| Nonempty custom arguments | `unsupported_provider_custom_arguments` |
| Nonempty MCP overlay | `unsupported_provider_mcp` |
| Nonempty connected-app allowlist | `unsupported_provider_connected_apps` |
| Nonempty service tier | `unsupported_provider_service_tier` |
| Nonempty runtime configuration | `unsupported_provider_runtime_configuration` |
| Nonempty disabled-runtime-skill list | `unsupported_provider_skill_exclusions` |
| Nonempty autonomy level | `unsupported_provider_autonomy`; ordinary hand-authored agents default to empty |
| Any enabled agent-skill binding | `unsupported_provider_skills`; disabled bindings are not dependencies |
| Nonempty squad instructions | `unsupported_provider_squad_instructions`; the fixed agent invocation DTO does not represent this additional policy |
| Custom runtime profiles / unsupported provider-family or installed OS pairing | `unsupported_runtime_configuration` at capability registration |

Absent/null JSON and the actual empty object/array forms retain their no-overlay
meaning. MCP additionally accepts the existing empty `{"mcpServers":{}}` wrapper.
Permission/invocation visibility, ownership, binding and physical capacity stay
with the existing authority/reservation predicates. Presentation and template
provenance do not supply provider arguments. Runtime mode remains part of the
existing fixed resource identity; nonempty runtime-config overrides are refused.
