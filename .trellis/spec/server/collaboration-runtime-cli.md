# Collaboration CLI and daemon execution

## 1. Scope / Trigger

Apply when changing the existing CLI commands inside a collaboration run or
their static Linux daemon/controller integration. This is an explicit contained
executor contract; general providers, Git/attachments and other OS runtimes need
their own acceptance before rollout.

## 2. Signatures

Supported commands are `issue get`, `issue comment list`, `issue comment add`
and `squad activity`, using the bound issue UUID. The two writers require
`--request-id <uuid>` in collaboration mode. Runner configuration accepts optional
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
accepts action/no_action/failed and a reason. Filters, attachments/content-file,
profile/server/workspace overrides and other operations are unsupported. Reads
return the authorized run context and its newest 200 comments. Every response
must match the admitted run/issue.

The caller retains the original request UUID and payload across uncertainty.
The CLI does not generate a replacement key or add a disk operation journal.
This does not introduce idempotency to ordinary task commands. Capability
rejection means stop; it never invites logging in as the resource owner.

The controller verifies the explicit CLI using the provider's static executable
and architecture checks. No installed executable discovery occurs. Only business
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
