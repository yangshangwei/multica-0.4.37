# Run-bound Agent output attachments

## 1. Scope / Trigger

Apply to Agent file writes, encrypted output provenance, runtime file reads,
stored outcome references and shared file rendering. Human draft uploads keep
the [content contract](./collaboration-content.md). The existing runtime
claim/permit/attempt, comment event, receipt, notification and proof owners remain
authoritative; output does not add a retry ledger or authorize another launch.

## 2. Signatures

- `POST /api/collaboration/runtime/runs/{runId}/comments/attachments` uses the
  current `mcb_` capability. Multipart fields are `comment_id`, `content`, optional
  `parent_id`, and exactly one `file`; identity UUIDs remain the original intent.
- `multica issue comment add <bound-issue-UUID> --content <text>
  --request-id <original-UUID> --attachment <CWD-file> --output json` uses that
  endpoint. The existing JSON comment command keeps its no-file wire.
- The success/replay response is HTTP **201** with the original comment receipt
  and `attachment`. Metadata adds nullable `uploader_agent_id`, `run_id` and a
  canonical positive signed-int64 decimal `attempt_seq`.
- Migration 868 extends `collaboration_attachment` with those provenance fields
  and human/Agent exclusivity constraints. Agent rows have null human
  `draft_id`, `uploader_user_id`, `intended_issue_id` and `upload_request_id`, and
  permanent issue delivery. No foreign key, cascade, new index or retry table.
- The stored outcome adds server-owned `output_attachment_ids`; incoming
  `CollaborationRunOutcome` and Provider/Runner/ledger wire remain unchanged.
  Public comment/result records project optional `attachment_ids`.

## 3. Contracts

The actual run row and issued capability determine Agent, issue, source and
current attempt. Caller multipart fields cannot supply uploader/run/attempt,
object key, digest or encryption material. Validate current business authority
before external storage and again inside the final runtime transaction.

Reuse encrypted private `Storage`. The immutable `comment:<UUID>` input includes
body, parent, display name, sniffed content type, length, SHA-256 and attempt.
JSON/no-file and multipart/file are different inputs. Matching original-key
replays preserve the exact winning file, object, comment, references and receipt.
Changed immutable fields conflict. The borrowed locked comment writer owns all
existing notification, squad and public-event effects in the same transaction
as the file row and issue/comment references.

Each file is at most 100 MiB. The existing run lock limits committed outputs to
32 for that exact run/current attempt. Resolve matching replay before checking
capacity; it consumes no additional slot. Do not use human-draft counts or
truncate an outcome's reference set.

Every prepared object has independent identity/key material. Delete only this
attempt's object after a known loss, replay or rollback. Retain a potentially
inserted object only for the existing `collaborationRuntimeCommitError` unknown
commit marker. Reset insertion ownership for each internal transaction callback
retry. An uncertain write keeps the original intent and key.

The first outcome transaction freezes all committed outputs for its run/attempt,
ordered by `(created_at,id)`, without a limit or later-list reconstruction. The
shared `decodeCollaborationStoredOutcome` separates the complete original
request from stored file references and serves both ACK paths, records,
checkpoint/retention/seal, reports and handoff. Original ACK compares the complete
original request; caller output IDs never enter that wire. Legacy absent output
refs remain absent. New outputs after an outcome commits are refused, including
a file prepared before that outcome won the final lock.

Runtime reads require current mcb/attempt authority and either exact own output
provenance or an existing frozen input reference. They grant no issue-wide file
fallback. On no-snapshot runs, unknown and unfrozen files share not-found.
Reauthorize after reading/decrypting and before response headers or bytes.
Current authorized humans retain existing issue/native reads. Private keys,
digests and storage URLs remain outside public metadata.

The CLI pins its invocation CWD with `os.OpenRoot`, opens nonblocking, requires a
regular bounded file and rejects URL, multiple, missing, nonregular, oversized or
escaping inputs before HTTP. This is invocation-CWD containment; the existing
five-field run context contains no immutable run-root anchor. No redirect,
profile fallback, journal or replacement request ID is introduced. Typed success
must correlate full original comment/Agent/run/file identity and metadata.

## 4. Validation & Error Matrix

| Condition | Required result |
| --- | --- |
| Anonymous, human or malformed bearer | 401 `invalid_collaboration_capability` |
| Original live intent and matching file replay | 201; same receipt and one stored object/reference set |
| Same UUID with changed file/body/parent/metadata or JSON/file alias | 409 `comment_conflict`; winner preserved |
| A 33rd distinct output for the exact run/attempt | 409 `attachment_limit`; prepared loser cleaned |
| Public capability/author/grant withdrawal | 403 `business_capability_required`; original read/write retries refuse |
| Direct SQL grant-removal handler control | Existing 409 `grant_required`; distinguish from public withdrawal |
| Outcome wins after bytes were prepared | Refuse final write, clean only losing object, keep frozen refs |
| Unknown business commit after an insertion | Keep potential object and original intent |
| Mixed/half/null/duplicate/noncanonical stored output provenance or refs | Reject malformed data; no invented identity or partial ref set |
| Human draft recovery receives an Agent row with null draft | Unavailable/unconfirmed; no throw, intent retirement or fresh key |

## 5. Good / Base / Bad Cases

Good: a real Agent CLI loses the committed response, repeats the same UUID/file
and recovers one original object and receipt. Base: old JSON comments, human
drafts and outcomes without outputs keep their existing contracts. Bad: record
the resource owner as the human uploader, rebuild old results from current files,
or delete potentially committed storage on every transaction error.

## 6. Tests Required

`collaboration_runtime_output*_test.go` owns O01–O08 identity, encryption,
same/different-key contention, required-write rollback, uncertain-object
ownership, withdrawal during storage/read, old attempts, explicit new-run old
fact preservation, full-32 replay and outcome-first cleanup. Stored decoder
tests cover every consumer, original ACK, legacy absence and malformed refs.
Handler/router tests pin actual status/body/header boundaries. Migration tests
exercise real upgrade SQL, exact XOR shape and retained/uncommitted downgrade
refusal. CLI tests own CWD/file/redirect/full receipt correlation.

Core schema/recovery tests own real Agent nullable provenance, legacy humans,
bounded unique comment/result refs and wrong-kind draft recovery. Shared Views
reuse protected attachment preview/download for input/comment/result records;
legacy records without refs issue no file read. Current-client acceptance must
use actual test-built Provider/CLI output, production Web and matching native
renderer/preload, with original tokens, objects, references and all failed rounds
retained. See [recovery QA](../web/frontend/collaboration-recovery-qa.md).

## 7. Wrong vs Correct

Wrong: add a human draft UUID or new request ID to recover a run output.
Correct: preserve original `comment:<UUID>`, actual Agent/run/attempt provenance
and current authority through the existing transaction and receipt.

Wrong: use an outcome's current attachment list as its historical result.
Correct: decode the original stored server-owned frozen reference set and keep
the unchanged original outcome request separate for ACK comparison.
