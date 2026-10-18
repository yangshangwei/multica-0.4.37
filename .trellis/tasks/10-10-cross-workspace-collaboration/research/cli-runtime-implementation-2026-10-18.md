# Existing CLI commands through the real collaboration daemon

The bounded static Linux CLI/daemon slice is implemented and verified. No public
creation/execution rollout gate was enabled. General provider adapters, Git/file
preparation, other operating systems and complete Web/Desktop acceptance remain
outside this slice.

## Command and environment contract

Exactly five environment variables select provider execution context:

- `MULTICA_COLLABORATION_TOKEN`: the admitted run's `mcb_` business capability.
- `MULTICA_COLLABORATION_SERVER`: the admitted Server base URL.
- `MULTICA_COLLABORATION_CONTEXT`: complete `CollaborationRunIdentity` JSON.
- `MULTICA_COLLABORATION_ISSUE_ID` and `MULTICA_COLLABORATION_RUN_ID`: must match
  the corresponding IDs in that JSON.

Partial identities, unknown/duplicate identity fields, invalid UUIDs/versions,
mixed run/issue IDs and credential-bearing Server URLs fail before profile
resolution or an ordinary request. `MULTICA_COLLABORATION_RUNNER_CONFIG` is host
configuration and does not select provider identity.

The existing commands use only the run-scoped business endpoints:

```text
multica issue get <issue-uuid> --output json|table
multica issue comment list <issue-uuid> --output json|table
multica issue comment add <issue-uuid> --content <text> --request-id <uuid>
multica issue comment add <issue-uuid> --content-stdin --request-id <uuid>
multica squad activity <issue-uuid> action|no_action|failed --request-id <uuid>
```

Comment add also accepts `--parent <comment-uuid>`; activity accepts `--reason`.
The two writers require `--request-id` only in collaboration mode. It maps
directly to `comment_id` or `event_id`. After an uncertain response, retry the
unchanged command with the same UUID; the CLI never creates a replacement key.
An authorization rejection instructs the provider to stop, never to log in as
the resource owner. Ordinary CLI behavior remains unchanged.

Issue get returns the current authorized run context, including its immutable
inputs and frozen roster. Comment list returns that context's newest 200
comments, without querying ordinary workspace actor directories. Read/write
responses must match the admitted run/issue identity. Attachments, content-file,
comment filters, profile/server/workspace overrides and other business/local
operations are rejected in this mode. Static help/version output grants no
additional access.

## Runtime and proof contract

Runner configuration accepts optional absolute `cli_executable`. The namespace
controller verifies it as a static executable for the current Linux architecture
using the same checks as the configured provider. It copies the CLI read-only to
`/multica`; there is no PATH discovery of a user-installed executable. The
existing private root, PID containment, privilege drop and exit-proof behavior
are preserved. Only business capability and immutable identity enter the
provider environment; managed and termination credentials stay outside it.

The combined native test supplies real server-issued managed credentials over a
test-owned helper's stdin. The helper registers through the actual router and
drives production `runBatchPoller` with `MaxConcurrentTasks=1`. Its transport
observer checks the real machine slot and active-task count before every claim.
A forwarding proxy observes committed proof responses without replacing any
endpoint. The provider executes the test-built CLI for context, duplicate comment
replay and activity; an actual human @leader input joins while the member runs.
Three distinct runs, permits, native namespace exits and sealed proofs complete
one squad, with the final leader outcome preserved and the slot returned.

## Files changed in this slice

Production:

- `server/cmd/multica/{main.go,cmd_agent.go,cmd_issue.go,cmd_squad.go,cmd_collaboration.go}`
- `server/internal/cli/collaboration.go`
- `server/internal/daemon/collaboration.go`
- `server/internal/daemon/collaborationrun/{controller.go,controller_linux.go}`

Tests:

- `server/cmd/multica/cmd_collaboration_test.go`
- `server/internal/cli/collaboration_test.go`
- `server/internal/daemon/collaboration_router_test.go`
- `server/cmd/server/{collaboration_squad_routes_test.go,collaboration_daemon_cli_routes_test.go}`

No dependency, router, database query or migration changes were needed for this
slice. The preceding server work remains documented in
[the squad completion report](squad-completion-2026-10-18.md).

## Verification

All Go commands use `GOTOOLCHAIN=go1.26.6` and
`scripts/go-test-with-agent-cli-guard.sh`. No installed/paid AI CLI or account was
used. Evidence directory:
`/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-cwc-resume-20261018-23vtl_pb`.

- `squad-cli-red.log`: reproduced ordinary credential/client and endpoint
  fallback before implementation.
- `squad-cli-full-race.jsonl`: full `cmd/multica` and `internal/cli` suites,
  1,265 passing entries, zero failures. One pre-existing skipped test is
  `TestCLIConfig_UnknownFieldsArePreserved`, documenting the existing JSON
  unknown-field round-trip limitation.
- `squad-cli-final-scopes-race.jsonl`: final collaboration command tests,
  23 passing entries, zero skips/failures; includes JSON/table comment display,
  incomplete/wrong scope, unsupported operations/files, revoked capability and
  lost-response recovery with the original write identity.
- `squad-cli-vet.log`: CLI, daemon, controller and server packages pass vet.
- `linux-cli-daemon-combined.log`: root-built static Linux artifacts pass
  `TestCollaborationSquadCLIAndDaemonCapacityOneRoutesLinux` in 5.13 seconds,
  zero skips, real PostgreSQL/router/CLI/daemon/namespace proofs.
- `git diff --check`: clean.

The main session owns the broad changed-package regression, independent review,
built-in skill/source-map updates and remaining task-wide acceptance. This report
does not mark all 97 acceptance criteria complete.
