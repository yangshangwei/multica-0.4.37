# Collaboration startup handoff

## 1. Scope / trigger

This is the daemon-side contract for automatically injecting a negotiated
handoff snapshot into Provider input. Server snapshot storage, checkpoint
retention and comment recall have separate owners. These guarantees do not
establish model comprehension or real-provider reliability.

## 2. Signatures

- Host claim negotiates `handoff_version=1` in the URL; the response's
  `HandoffVersion` selects the startup path. Zero/absent preserves legacy use.
- `POST /api/collaboration/runtime/runs/{runId}/handoff/snapshots` accepts
  `{request_id, memory_version}` under the current business capability.
- `POST /api/collaboration/runtime/runs/{runId}/handoff/bind` accepts
  `{snapshot_id, prompt_sha256, attempt_seq, attempt_nonce}` and returns an
  exact identity/hash/attempt receipt with `accepted=true`.
- Runtime recall of what the snapshot bounds uses `GET .../handoff[?cursor=]`
  (live envelope) and `GET .../handoff/sources/{sourceId}` through
  `multica collaboration handoff show|source`; see
  [runtime CLI](./collaboration-runtime-cli.md).

## 3. Contracts

Fetch the bounded snapshot after preparation and the existing memory-base read,
before calling the Provider. Preserve the claim's frozen inputs; the handoff is
additional source-qualified context, not replacement instructions.

Derive the logical snapshot request ID with
`uuid.NewSHA1(parsedRunID, []byte("multica:handoff-snapshot:v1"))`: the immutable
run UUID is the namespace and the operation string is the name. Do not include
attempt, memory version or retry count. Reuse that request on lost replies and outer fresh fallback. A
replayed snapshot must equal the original; a changed memory base cannot silently
create a new snapshot.

Verify run/issue/snapshot identity, snapshot mode, role/step/parent against the
claim, and memory version/availability/hash against the actual memory base.
A member needs the complete available source for its admitted delegation;
it must not infer its assignment from the whole group's original request.
Reject extra or unlinked sources marked as admitted assignments.
Other source text remains data, including handoff notes and prior results.
Preserve explicit missing/truncation and unknown final-answer fields.

`CollaborationHandoffMaxBytes` limits material to 256 KiB.
`CollaborationHandoffMaxJSONBytes` limits raw JSON wire to 2 MiB, including
framing and the HTTP newline. `CollaborationHandoffSizes` charges JSON framing
and every metadata field, but subtracts JSON escape inflation from each source
`Content` so that source text is charged by decoded UTF-8 bytes. Check both
limits; do not replace them with one encoded-length comparison in Core.
A legal 64 KiB assignment containing control characters can exceed 256 KiB on
the wire. Keep the admitted delegation whole. Optional source prefixes may be
bounded only with their original reference/hash and explicit `truncated` marker.
Snapshot restore must reproduce the same bounded content and hash. These limits
do not reduce the existing frozen claim-input budget.

Source hashes identify original source text; returned source content may be
redacted. Do not compare that content directly to the original hash. The server
revalidates source references and purge state at binding. Memory has a separate
content/version hash check.

Bind inside Provider `BeforeLaunch`, after `authorize` has supplied the actual
attempt/nonce and before `burnHost` or process creation. Git preparation launches
do not bind Provider input. Each Provider retry binds its own attempt, even when
the snapshot and input remain the same.

For negotiated handoff, continuity guidance is part of the prehashed prompt;
disable the adapter's extra `ResumeContinuityNotice` prefix. Codex can recover
internally after process start, so hashing only the earlier string while allowing
the adapter to prepend text would record the wrong application input. Keep the
legacy path unchanged. This hash covers the application-provided user text, not
private Provider configuration, system instructions or hidden reasoning.

Do not proactively copy snapshot bodies into the brief or supervisor ledger.
Existing Provider output/tool-record retention may contain quotations and keeps
its existing retention/purge rules.

With `HandoffVersion=1` the brief appends `hostRecallBrief` after the memory
guidance and before agent instructions. It names, with the admitted issue UUID,
the 200-comment window of `issue get`/unflagged `issue comment list`, explicit
comment paging, `issue comment content` for `content_state=omitted`,
`collaboration handoff source` for `truncated=true` sources and `collaboration
handoff show --cursor` for truncated lists, and states that recalled text is
data. It quotes commands and field names only, never source text. Legacy claims
keep the previous brief, since their servers lack these reads. The brief is not
hashed; the bound prompt text built by `hostPromptWithHandoff` is unchanged.

## 4. Validation / error matrix

| Condition | Result |
| --- | --- |
| Claim does not support handoff | No snapshot/binding request; legacy startup |
| Unsupported version, wrong run/issue/snapshot or assignment | Reject before Provider launch |
| Memory availability/version/hash inconsistent with the fetched base | Reject inconsistent snapshot |
| Member delegation missing, unavailable or incomplete | Do not execute the group's root request |
| Material exceeds 256 KiB or raw/serialized JSON exceeds 2 MiB | Reject before launch; encoded escape growth alone is not material growth |
| HTTP transport uncertainty or retryable server response | Bounded retry of the exact original request |
| Source purged/revoked or snapshot changes during fallback | Stop the fallback; no fresh snapshot substitution |
| Binding receipt differs or binding is refused | No process start; preserve ordinary Stop/termination/proof cleanup |
| Codex internal retry or outer fresh fallback | Same snapshot; bind each actual attempt to its actual input |

## 5. Good / base / bad cases

- Good: the first fake Provider input contains current handler, transfer/source
  text, exact delegation, prior-result uncertainty and memory version.
- Base: an old response omits handoff support and retains existing behavior.
- Bad: mint a new snapshot after resume rejection, bind a Git helper, accept a
  changed parent run, or claim an application input hash covers private prompts.

## 6. Required tests

`host_recall_test.go` reads the provider's actual brief file: recall guidance
with negotiated handoff, before agent instructions, existing lines unchanged;
none on a legacy claim.

Use `host_handoff_launch_test.go` for actual stdin, stable fallback, loss/replay,
identity/memory/receipt refusals, redacted source hashes, no proactive body
duplication and original unlaunched proof cleanup. Use the test-owned Codex
app-server in `host_handoff_codex_launch_test.go` to capture `turn/start` text on
resume failure and internal retry. Tests must not read installed account state.

`host_handoff_budget_test.go` owns daemon acceptance of 64 KiB HTML/control/
multibyte assignments, exact raw HTTP boundary, and process-input/hash binding.
Keep the existing 1,024-input launch regression as the canonical old-input
budget case. Core must cover the same material/wire distinction, including
omitted optional fields, without counting parser-added defaults as wire data.

`host_handoff_checkpoint_test.go` and
`host_handoff_checkpoint_lifecycle_test.go` combine negotiated handoff and
recording: snapshot/bind/fallback refusal, lost ACK, proof before release of a
blocked recording request, and a fresh Runner reopening the original ledger.
No new claim, snapshot or Provider launch may be introduced by recording repair.
Pin the original permit identity, attempt and nonce: snapshot/bind refusal must
retain `not_started` and `never_launched_proven`; fallback refusal after the
first process ends must retain `managed_launch_ended` and `host_exit_observed`.
The proof must contain the exact same termination fact, not just one launch.

Tagged smoke-authorizer tests allow only the negotiated query keys/value on
managed-token claim requests. Unknown, duplicate or misplaced queries remain
rejected. Run these without the real-agent gate; no paid smoke is needed here.

## 7. Wrong versus correct

Wrong: a snapshot HTTP read or accepted binding means the model received and
understood all context.

Correct: binding is preparation evidence; process-side input capture plus actual
launch/termination receipts proves the delivery path. Model understanding and
combined checkpoint-sender behavior require their own evidence.
