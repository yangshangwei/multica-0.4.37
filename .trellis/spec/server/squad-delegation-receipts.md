# Immediate squad delegation receipts

## 1. Scope / trigger
Ordinary issue comment add/edit used to delegate by agent or squad mention. This contract consumes existing server outcomes; it does not replace dispatch, persist receipts, or change issue acceptance. Cross-account collaboration remains a separate protocol.

## 2. Signatures
- `CommentResponse.trigger_outcomes[]`: `target_type`, `target_id`, `status`, `reason_code` on create/edit responses only.
- CLI `warnCommentTriggerOutcomes(io.Writer, map[string]any)` called by ordinary comment add/update, before the table-mode return.
- `squadOperatingProtocolHeader` and runtime mention briefing teach the leader to inspect the immediate JSON response.

## 3. Contracts
Preserve the complete stdout JSON and the successful comment write exit code. Write a bounded warning to stderr for blocked, malformed or unknown outcomes; do not echo arbitrary server text into terminal control sequences. No extra HTTP request or automatic repost.

`queued`, `coalesced`, `deferred` establish admission or input coverage only. They do not establish execution, completion or acceptance. Validate target type/UUID, nonempty status/reason; identical duplicate targets count once, contradictory duplicates remain unconfirmed. Unknown reason strings accompanying known handled status remain compatible.

The leader knows intended roster targets and must match every exact type/UUID. Missing/malformed/unknown/conflicting entries are unconfirmed. For a delegation-only turn use `action` only when every target has a handled receipt; otherwise use existing `failed` and explain the handled subset and unresolved targets in reason. This is agent guidance, not a server-derived proof field. Maintain one comment per turn, no blind retry on response loss, human `done`, and owner/guest status boundaries.

## 4. Validation / error matrix
| Response | CLI | Leader |
| --- | --- | --- |
| Field absent (old server) | Preserve success/JSON, no generic warning | Intended delegation remains unconfirmed |
| Empty array / ordinary no-mention comment | Preserve success | Compare intended targets if any |
| Valid queued/coalesced/deferred | No warning | Admission only |
| Blocked or partial | Warning; successful saved-comment exit | Failed evaluation plus handled subset |
| Unknown/malformed/conflicting | Warning; no panic or extra POST | Unconfirmed, no claim of full success |
| Lost response | Existing transport error | Inspect saved state; do not blindly repost |

## 5. Good / base / bad cases
Good: exact roster targets match their immediate receipts and partial admission is visible. Base: old server or no-mention comment keeps its existing command behavior. Bad: treat HTTP201 or squad `action` alone as proof that every member ran.

## 6. Tests required
CLI fake HTTP tests exercise actual add/update commands, assert one request, complete stdout JSON, stderr warning and retained successful write semantics, including table output and existing HTTP errors. Brief composition tests cover exact targets, uncertainty, partial handling, one-comment rule and owner/guest boundaries. Runtime mention tests distinguish requested work from execution. DB-backed handler package tests still require their own explicit database even for text-only assertions.

## 7. Wrong vs correct
Wrong: exit nonzero after a committed comment solely because a target is blocked, encouraging duplicate retries; or reconstruct the original receipt by reading the saved comment.
Correct: retain successful write result and warn separately. Preserve immediate receipt JSON. Ordinary comment reads/broadcasts do not contain historical outcomes; persistent recovery needs a separately reviewed contract.
