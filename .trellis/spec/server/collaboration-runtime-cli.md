# Collaboration CLI and daemon execution

> 2026-10-26: ordinary host execution has a separate [implementation contract](./collaboration-host-execution.md) and default-closed admission. The Linux namespace guarantees below apply only to the retained legacy branch; do not apply them to host execution. Platform/account acceptance remains tracked by the active runtime task.

## 1. Scope / Trigger

Apply when changing the existing CLI commands inside a collaboration run or
their Linux daemon/controller integration. This is an explicit contained
executor contract. Negotiated attachment/Git preparation and provider bundles
follow [collaboration-preparation.md](./collaboration-preparation.md); real
provider versions and other OS runtimes still require separate acceptance.

## 2. Signatures

Supported commands are `issue get`, `issue comment list`, `issue comment content`,
`issue comment add`, `squad activity`, `collaboration memory show`,
`collaboration memory update`, `collaboration handoff show` and
`collaboration handoff source`, using the bound issue UUID. The three writers
require `--request-id <uuid>` in collaboration mode. `collaboration memory` and
`collaboration handoff` exist only inside a run; outside one they refuse instead
of reaching an ordinary profile. Runner configuration accepts optional
absolute `cli_executable`, verified and mounted read-only at `/multica`.

- `collaboration handoff show <issue> [--cursor <c>]` → GET
  `/api/collaboration/runtime/runs/{runId}/handoff[?cursor=]`; prints the
  `CollaborationHandoffEnvelope` JSON.
- `collaboration handoff source <issue> <source-id> --kind <k> [--source-run-id
  <r> --version <v>] --sha256 <h> [--offset <n>]` → GET
  `.../handoff/sources/{sourceId}?kind&sha256[&source_run_id&version][&offset]`;
  prints the `CollaborationHandoffSourcePage` JSON. Both accept only `--output json`.

Five variables identify the provider run: `MULTICA_COLLABORATION_TOKEN`,
`MULTICA_COLLABORATION_SERVER`, `MULTICA_COLLABORATION_CONTEXT`,
`MULTICA_COLLABORATION_ISSUE_ID`, `MULTICA_COLLABORATION_RUN_ID`.
`MULTICA_COLLABORATION_RUNNER_CONFIG` is host configuration, not run identity.

## 3. Contracts

The context is complete `CollaborationRunIdentity` JSON. IDs, positive versions,
run/issue correspondence, Server URL and `mcb_` capability must validate before
profile resolution or ordinary requests. Unknown or duplicate identity fields
refuse. Runtime commands use only the run-scoped business endpoints; they never
borrow the resource owner's user/daemon token or workspace actor directory.

Comment writes accept content or stdin and an optional parent UUID. Activity
accepts action/no_action/failed and a reason. Content and reason are limited to
64 KiB of decoded text by the service; because the CLI's `json.Marshal` writes
`<`, `>` and `&` as six bytes, both routes read up to six times that limit plus
a 64 KiB envelope, like shared-context updates. Filters, attachments, comment
content-file, profile/server/workspace overrides and other operations are
unsupported except for the explicit recall operations below. Legacy `issue get`
and `issue comment list` without paging flags return the authorized run context
and its newest 200 comments. Every response must match the admitted run/issue.

Explicit `issue comment list --limit --max-bytes --cursor` reads
`.../runtime/runs/{runId}/comments` and returns a page envelope rather than the
legacy array. Limit is 1–100 (default 50), max_bytes is 4096–1048576 (default
65536). `bytes` counts the serialized UTF-8 JSON response, including metadata,
escaping and its trailing newline. Pages select newest-first and display oldest
first within that page. A run-bound, opaque next_cursor reaches older comments;
it is distinct from the content cursor below. Total counts currently visible,
non-deleted runtime comments, not only the newest 200.

An oversized first comment is an explicit `content_state=omitted` placeholder
with its exact comment_revision, author, content_bytes and content_ref; the
list cursor advances past it. `issue comment content <issue> <comment>
--revision <revision> [--cursor <cursor>] [--max-bytes <bytes>]` reads contiguous
UTF-8 chunks for that revision until complete=true. An edit returns
`stale_comment_revision`; discard incomplete assembly rather than joining
revisions. Deletion or authority loss refuses current content reads. Even an
empty page must fit its envelope; `budget_too_small` requires a larger budget,
not a retry of the same query or a non-advancing empty page.

Page/chunk JSON prints next_cursor/next_offset and completeness metadata before
its potentially long items/content/source text. Those response-only field orders
do not change startup snapshot encoding or hashes. The negotiated host brief
uses an explicit 16384-byte budget for comment pages and content chunks (the
protocol default stays 65536). Provider shells can still preview only a prefix
of a tool result: read their saved full output before claiming the page/chunk
text was read; seeing a cursor alone proves no content exposure.

Member comments carry optional current `author_name` labels in both runtime
reads. Reuse the human content-account lookup, including external members;
agent authors and unresolved accounts omit the name. CLI decoding preserves
the label but accepts older responses without it. Names and the current
author_authority role/can_submit_command are separate from historical command
admission. Every recalled comment remains trust=data_only, including text by
an author who can currently submit commands; only frozen inputs/delegations
carry execution intent.

Shared-context update replaces the whole document from `--base-version` (a
canonical non-negative decimal) with `--content-stdin` or `--content-file`; the
file must resolve inside the working directory and there is no override. There is
no inline `--content`. Runtime-local file links are refused through
`rejectLocalPathLinks`, because a run carries no MULTICA_AGENT_ID and the gated
`guardLocalPathLinks` would skip it. Each refusal names its next action:
`stale_memory_version` gives the current version and the runnable `memory show
<issue-uuid>` command; `memory_too_large` gives the submitted size and limit; a
413 from the request read bound (no receipt) says to shorten and re-check with
`memory show <issue-uuid>`; `memory_write_conflict` says the request ID was used
with another base version or content. Capability rejection and unknown results
keep the shared guidance above.

Handoff reads are the runtime recall path for what the startup snapshot bounds
(design §3). `handoff show` is the live projection (`mode=live`, no
`snapshot_id`); help text separates it from the frozen startup snapshot. Its
cursor is opaque to the CLI: only its length (≤1024) and base64url alphabet are
checked before HTTP; the server binds it to the run. A page whose next or
readers cursor echoes the request cursor is refused as non-advancing.
`handoff source` checks before HTTP that the source ID is a canonical UUID, kind
is `handoff|delegation|provider_result|squad_activity`, a handoff kind carries
no run/version while other kinds carry a canonical run UUID and positive
version, sha256 is lowercase hex and offset is non-negative. The response is
decoded by `protocol.DecodeCollaborationHandoffSourcePage` (strict fields, data
trust per kind, contiguous `next_offset`) and must echo the requested issue,
run, source ID, kind, run, version, sha256 and offset. Text is redacted while
sha256 names the original, so never compare them.

The caller retains the original request UUID and payload across uncertainty.
The CLI does not generate a replacement key or add a disk operation journal.
This does not introduce idempotency to ordinary task commands. Capability
rejection means stop; it never invites logging in as the resource owner.

The legacy controller verifies the explicit CLI using its static executable
and architecture checks; a prepared bundle must declare the full executable
closure instead. No installed executable discovery occurs. Only business
capability and immutable identity enter the provider environment; supervisor
termination and managed credentials remain private. Existing PID containment,
privilege drop, nonce ledger and proof sealing stay authoritative.

Production `runBatchPoller` acquires the real machine slot before claim. Each
leader/member/continuation has separate admission and proof. A database waiting
state or parent proof cannot release a child's slot or barrier.

## 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Partial/mismatched/duplicate run identity | Reject before profile or ordinary request |
| Another issue UUID or Server override | Reject without widening capability |
| Unsupported file/command/filter | Reject before ordinary side effects |
| Memory update with bad base/request ID, external file or local link | Reject before HTTP |
| Handoff cursor outside the opaque alphabet/size, or a bad source reference | Reject before HTTP |
| Handoff envelope for another issue/run, snapshot mode or a non-advancing cursor | Refuse; print nothing |
| Source page for another reference/offset, or malformed/non-contiguous chunk | Refuse; print nothing |
| Stale/too-large/reused-key memory write | Name the next command; no automatic retry |
| Uncertain write response | Original UUID and payload remain the retry identity |
| Revoked business capability | Stop; no credential fallback |
| Missing/nonstatic/wrong-architecture explicit CLI | Controller refuses support/start |
| Unknown process or unsealed attempt | Retain machine capacity and proof barrier |

## 5. Good / Base / Bad Cases

- Good: a leader replays one comment UUID, creates one delegation, exits, then
  its member and continuation acquire the same capacity-one machine in turn.
- Base: ordinary CLI commands retain their existing credential and API behavior.
- Bad: treating host runner configuration as provider identity, discovering an
  installed CLI, or supplying a fresh request UUID after a lost response.

## 6. Tests Required

`cmd_collaboration_test.go` and `internal/cli/collaboration_test.go` cover scope,
ordinary behavior, malformed identity, unsupported operations and retry identity.
`cmd_collaboration_memory_test.go` covers registration/allowlist, run-scoped
show/update, pre-HTTP refusals, refusal guidance and out-of-run refusal.
`cmd_collaboration_handoff_test.go` covers handoff registration/allowlist, root
guard flag refusals, out-of-run refusal and one walk across result pages and
source chunks. `internal/cli/collaboration_handoff_test.go` owns query encoding,
pre-HTTP reference checks and identity binding;
`pkg/protocol/collaboration_handoff_source_page_test.go` owns the decode matrix.
`TestCollaborationRuntimeCommentsCarryMemberDisplayNames` covers both current
runtime reads, member/agent attribution and strict recall decoding;
`TestCollaborationRuntimeCommentKeepsOptionalDisplayName` covers legacy CLI
retention and old-server omission. Recall service tests own 200+ comments,
UTF-8/escaping/envelope budgets, placeholder progress, revision-bound chunks
and current authority; CLI tests own explicit-envelope wiring and pre-HTTP guards.
Controller/ledger tests retain namespace and nonce/proof coverage.
`TestCollaborationSquadCLIAndDaemonCapacityOneRoutesLinux` exercises actual
registration/router/PostgreSQL, production slot polling, test-built CLI commands,
M11 input from another author and three separate native proofs. Require the
explicit test executable paths; never resolve a user-installed AI CLI.

## 7. Wrong vs Correct

Wrong: use a human profile when one run variable is missing, or submit the same
uncertain comment with a new identifier.

Correct: reject incomplete run context and preserve the caller's original
request UUID; use only the admitted run's business API and contained executable.


## Current packaged prior-CLI compatibility evidence

`e2e/collaboration-installed-clients.spec.ts` runs exact locally delivered
v0.6.7 OCI `/app/multica` bytes, immutable image digest and genuine client version/
platform/OS headers. A network-none/read-only container uses private stdio to
forward only the expected legacy issue method/path to the task API. The ordinary
PUT positive succeeds; old status/comment/rerun writes refuse in paused,
permitted-stop-unconfirmed and removed-author states with exact resource,
participant, history, queue and control snapshots unchanged. Author removal
retains its independent source read basis solely to reach the shared boundary;
own original content-free receipt remains identical. Prior package publisher
signature and old Desktop release trust are separate from this no-bypass claim.

Current canonical no-file legacy Claim wire stays unchanged; unsupported file
preparation rejects before claiming. Actual Linux daemon network interruption
and original natural proof recovery preserve pause and removed/cancelled author
input across client/daemon reconnect; proof never implies automatic Resume.
The packaged prior CLI case plus those exact current protocol/daemon contracts
supply AC32. No fixture starts a commercial provider or changes rollout gates.
