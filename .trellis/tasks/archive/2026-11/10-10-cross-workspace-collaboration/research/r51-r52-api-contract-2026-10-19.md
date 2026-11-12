# R51/R52 additive API contract

Server implementation contract, 2026-10-19. Existing creation/execution gates and
execution view protocol 5 are unchanged. This document coordinates core/views.

## Ordinary children

The issue detail adds an optional block (omission on older servers means support
unavailable, not an empty list):

```ts
ordinary_children?: {
  items: { id: UUID; identifier: string; title: string; can_open: boolean }[];
  source_workspace_slug: string | null;
}
```

The complete same-source formal ordinary child set is ordered by issue number
ascending. Independently shared children are excluded. Source members get true
and the current source slug; outside participants get false/null. No other child
fields or source catalogs are returned. Authority and children use the detail's
existing repeatable-read snapshot.

Postcommit ordinary changes publish `collaboration:children_changed` to each
affected shared parent's issue scope with **exactly** `{issue_id: UUID}`. This
transient hint has no event sequence/revision/epoch and does not advance access
epochs or clear drafts. Existing polling/focus/reconnect recovers missed hints.

## Recorded run read

`GET /api/collaboration/issues/{id}/runs/{runId}/records?cursor=...&limit=...`

Auth-only route, no ambient workspace authority; current human source membership
or participation required on **every page**. Missing/unauthorized issue reads are
404 `not_found` before any run lookup. After issue authorization, a wrong/missing
run is 404 `run_not_found`; it does not revoke the otherwise valid issue view.
Malformed paging is 400 `invalid_records_query`.
Database or malformed retained data returns unavailable. All responses are
`Cache-Control: no-store`. Older route absence uses the existing optional-route
unavailable behavior; it never falls back to ordinary task messages.

`limit` defaults to 50, range 1–100. Cursor is opaque, run-bound and bounded to
1024 characters. It preserves input/event/within-event position, so a recorded
event with many messages can cross a page without loss. `next_cursor: null`
means that this read reached the end; refetching observes later events. The cursor
freezes the upper event sequence and prompt identity. Pages also target a 256 KiB
encoded content budget; one larger legal historic result is returned intact.

```ts
type RecordedRunPage = {
  protocol_version: 1;
  issue_id: UUID;
  run: CollaborationExecutionRun; // existing exact v5 run shape, no new fields
  resource: {
    agent_id: UUID;
    runtime_id: UUID;
    resource_owner_user_id: UUID;
    execution_workspace_id: UUID;
    grant_id: UUID;
    provider: string | null;
    model: string | null;
    account_source: "runtime_configuration" | null;
  };
  recording: {
    status: "not_recorded" | "partial" | "complete";
    inputs_recorded: boolean;
  };
  usage: {
    input_tokens: DecimalInt | null;
    output_tokens: DecimalInt | null;
    cache_read_tokens: DecimalInt | null;
    cache_creation_tokens: DecimalInt | null;
    total_tokens: DecimalInt | null;
    cost_usd: string | null;
  } | null;
  items: RecordedRunItem[];
  next_cursor: string | null;
};
type RecordedRunItem =
  | { id: string; kind: "input"; command_id: UUID; author_user_id: UUID;
      body: string; attachment_ids: UUID[] }
  | { id: string; kind: "message"; role: "assistant" | "user" | "system";
      body: string }
  | { id: string; kind: "tool_call"; call_id: string; name: string; body: string }
  | { id: string; kind: "tool_result"; call_id: string; body: string; is_error: boolean }
  | { id: string; kind: "comment"; comment_id: UUID; body: string }
  | { id: string; kind: "squad_activity"; outcome: "action" | "no_action" | "failed";
      body: string }
  | { id: string; kind: "result"; outcome: "success" | "failure"; body: string };
```

All displayed entries have stable nonempty IDs (at most 200 characters). Inputs
precede event entries; events retain server sequence, with each event's messages
before its final result. Inputs come only from that run's frozen prompt. A null
prompt is `inputs_recorded:false`, never the mutable batch. Comment entries use
the retained event input even after the displayed comment is edited/deleted.
No transcript is inferred from console noise or private provider reasoning.

Resource IDs are the source-owned run identity. Provider/model/account source
are a one-time allowlisted configured-resource snapshot, never live resource catalogs
or proof of a provider's actual billing account. Provider/model are null or
nonempty identifiers of at most 128 UTF-8 bytes. Missing historic provenance is
null. No installation/binding secrets, credentials,
private directories, raw runtime config or owner email are returned. Usage is
explicitly recorded telemetry: absence is null, including absent individual
counters; zero is shown only when recorded. DecimalInt is canonical nonnegative
signed-int64 decimal text. cost_usd is nonnegative canonical decimal text with
1–19 integer digits and up to 10 fraction digits, no sign/exponent/leading zeroes
except zero itself; it is not a computed estimate.

`not_recorded` means no typed runtime record was supplied and no recording gap
is flagged; legacy final bodies still appear as result entries. `partial` means a supplied record is partial or
the persisted run marks missing recording. `complete` describes recorded process
coverage, never business success or proof. Run state/outcome/proof remain separate.

Run discoverability is already complete: `ListCollaborationRuns` and
`ListCollaborationExecutionGroups` have no LIMIT and the v5 detail exposes their
complete authorized sets. This increment adds no second run-list owner.

## Supported runtime outcome extension

Existing `{outcome,body}` provider stdout and existing outcome requests stay valid.
Optional `record` adds `{version:1, completeness:"complete"|"partial",
messages:[...] , usage?: ...}`. Messages are the message/tool_call/tool_result
variants above without server-generated `id`. All objects are strictly decoded;
unknown fields, duplicates, invalid discriminants and inconsistent tool fields
refuse ingestion. Maximum 256 messages; each body at most 65,536 UTF-8 bytes;
combined outcome plus record JSON at most 1 MiB; request envelope at most 2 MiB.
Body alone retains the service's 1 MiB legacy limit. Each call_id/name is nonempty
and at most 128 UTF-8 bytes. No provider/account/resource identity is reporter-set.
Only the narrow business capability ingests this extension; proofs carry no
content. Replays compare normalized complete payloads and cannot replace records.

## Group result notifications and anchors

Exactly one newly proven terminal group transition records one logical event
`ai_completed`, `ai_failed`, or `ai_stopped` (cancelled/forced-stopped groups).
Intermediate leader/member turns do not notify completion. Results alone and
unconfirmed Stop do not settle; all applicable proofs/delegations must converge.
Proof without outcome remains failed/incomplete; late success cannot replace it
or an earlier forced disposition.

The existing notification DTO retains `source_kind:"issue", source_id:null`
and adds optional `run_id:UUID` equal to the group's representative run: final
allocated leader turn, or direct root run. Older clients still open the issue;
new clients use `run_id`. Migration 729 stores it in a nullable `run_id` column
while retaining the existing source kind. It is **not**
the member that happened to submit the last proof.
`kind` uses the three values above, `is_mention:false`. Open via
`/account?view=participating&issue=<issue UUID>#run-<run UUID>` and read the run
through the protected route. The existing denied-notification projection strips
the title/anchor (including `run_id`) and sets `can_open:false` after access loss.

AI/system result events have no human actor, so all current eligible participants
are recipients, including the input author and resource owner. Existing distinct
human actions retain true-operator self-notification exclusion. Recipients are
frozen in the terminal transaction; retries neither re-expand recipients nor
create ordinary inbox/subscription rows.
