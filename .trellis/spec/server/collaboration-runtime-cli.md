# Collaboration CLI and daemon execution

> 2026-10-26: ordinary host execution has a separate [implementation contract](./collaboration-host-execution.md) and default-closed admission. The Linux namespace guarantees below apply only to the retained legacy branch; do not apply them to host execution. Platform/account acceptance remains tracked by the active runtime task.

## 1. Scope / Trigger

Apply when changing the existing CLI commands inside a collaboration run or
their Linux daemon/controller integration. This is an explicit contained
executor contract. Negotiated attachment/Git preparation and provider bundles
follow [collaboration-preparation.md](./collaboration-preparation.md); real
provider versions and other OS runtimes still require separate acceptance.

## 2. Signatures

Supported commands are `issue get`, `issue comment list`, `issue comment add`,
`squad activity`, `collaboration memory show` and `collaboration memory update`,
using the bound issue UUID. The three writers require `--request-id <uuid>` in
collaboration mode. `collaboration memory` exists only inside a run; outside one
it refuses instead of reaching an ordinary profile. Runner configuration accepts optional
absolute `cli_executable`, verified and mounted read-only at `/multica`.

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
accepts action/no_action/failed and a reason. Filters, attachments, comment
content-file, profile/server/workspace overrides and other operations are
unsupported. Reads return the authorized run context and its newest 200
comments. Every response must match the admitted run/issue.

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
