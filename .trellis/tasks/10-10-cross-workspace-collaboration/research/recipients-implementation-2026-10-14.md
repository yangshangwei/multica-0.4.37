# R29 recipient resolution implementation — 2026-10-14

The assigned backend slice is implemented and verified against the task-owned
recipient database and the actual generated SQL. This is evidence for the R29
resolver, not full P1/P6/P8 or AC45/AC46 end-to-end acceptance. No commits were
created. Existing unrelated changes were preserved.

## Files and ownership

This implementer added:

- `server/internal/service/collaboration_recipients.go`
- `server/internal/service/collaboration_recipients_test.go`
- `server/internal/handler/collaboration_recipients.go`
- `server/internal/handler/collaboration_recipients_test.go`
- `server/pkg/db/queries/collaboration_recipients.sql`
- This report.

The root session owns `router.go`, generated SQL, shared collaboration files and
integration checks. It registered `ResolveCollaborationRecipient` and ran
`make sqlc`. The core agent owns the matching API/schema changes. This implementer
did not edit those shared files, migrations, or dependencies.

## Implemented contract

`POST /api/collaboration/recipients/resolve` accepts a JSON object containing
`kind: username|email|user_id`, `value`, and exactly one present, non-null UUID
context: `issue_id` or `source_workspace_id`. A second context explicitly set to
null is rejected. Unknown fields, duplicate keys/aliases, trailing JSON,
non-object bodies and oversized input are rejected. Non-JSON content types
return `415 json_required`.

The response is exactly the dedicated shape:

```json
{
  "issue_id": null,
  "source_workspace_id": "UUID",
  "recipient": {
    "user_id": "UUID",
    "name": "display name",
    "avatar_url": "avatar URL or empty string",
    "external": true
  }
}
```

`external` is relative to the explicitly authorized source workspace. No email,
username, profile, credential state, confirmation token or write authority is
returned. UUID results use the canonical spelling produced by `util.UUIDToString`.
The lookup does not create operations, account fences, participants, memberships,
handoffs, grants, commands, events or notifications, and does not change the
handler. A successful lookup also works inside a PostgreSQL read-only transaction.

Authorization and reuse:

1. Both handler and service require the trusted authenticated principal to be a
   current human JWT/PAT actor. Machine, unknown, restricted and legacy agent
   actors are denied; the handler also rejects nonempty legacy agent/task hints.
   Client `X-User-ID` is not the authority.
2. Issue context reuses `ResolveIssueAccess`, followed by current Invite/Transfer
   capabilities. No read basis returns `404 not_found`; a readable nonmanager
   gets `403 forbidden`. Creation and execution switches are not consulted.
3. Creation context reuses `authorizeWorkflowAccount`, including current disabled
   state, emergency denylist and password-session checks. A query joins the
   explicit source workspace to the caller's current member row. Ordinary
   members may create under existing policy; no owner/admin-only gate was added.
   Only after this authorization is `creation_enabled` checked.
4. Context authority is checked again after matching. A transfer, removal,
   account disable, source departure or creation-switch closure during the
   lookup cannot return a candidate using the earlier authority.
5. `auth.NormalizeUsername` handles usernames. Email input uses the authentication
   normalization (`TrimSpace` then `ToLower`), validates a single full address,
   and is bounded to 254 bytes. UUIDs use the checked `util.ParseUUID` parser.
   Historical stored username/email values are matched with lowercasing and the
   same Unicode whitespace trim set. Queries use exact equality and `LIMIT 2`;
   there is no directory, name, prefix, substring or `LIKE` query.

Multiple normalized matches are `409 recipient_ambiguous`, including when one
match is disabled; the service never picks an arbitrary survivor. A missing,
disabled, emergency-denied or password-restricted candidate is
`404 recipient_not_found`. Password mode also requires a completed credential;
legacy accounts may have no password credential. Database failures propagate to
`503 collaboration_unavailable`, never an empty or permissive result.

The handler clears ambient workspace headers and sends `Cache-Control: no-store`
before its checks. It reuses the existing bounded `auth.PasswordLimiter` via
`Handler.PasswordLimiter`, with independent hashed actor/IP keys and budgets of
20 actor lookups and 60 IP lookups per minute. Query values are not limiter keys
or log fields. Default single mode works without Redis; configured shared mode
rejects on absent/failed Redis. Capacity is inherited from the existing 50,000
bucket bound. Exhaustion returns `429 rate_limited` with a rounded-up
`Retry-After`; limiter failure returns 503 with `Retry-After: 1`.

## Verification evidence

The initial new service tests failed to compile because the resolver API did not
exist. During implementation, the transferred-workflow fixture's embedded struct
literal was corrected before successful execution.

All Go invocations sourced the assigned isolated database file first:

```bash
source '/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-collab-fields-20261014-9l0n9hrj/recipients.env'
```

After the root session generated the actual workspace SQL, these exact commands
passed from the repository root. The two DB suites ran sequentially:

```bash
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race ./internal/service -run '^TestCollaborationRecipient' -count=1
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race ./internal/handler -run '^TestResolveCollaborationRecipient' -count=1 -v
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server vet ./internal/service ./internal/handler
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test ./internal/auth -run '^TestPasswordLimiterDoesNotEvictActiveBucketsAtCapacity$' -count=1
gofmt -l server/internal/service/collaboration_recipients.go server/internal/service/collaboration_recipients_test.go server/internal/handler/collaboration_recipients.go server/internal/handler/collaboration_recipients_test.go
```

- Service race suite: passed, 6.938 s; eight top-level resolver tests plus their
  normalization, availability, authority and fault cases.
- Handler race suite: passed, 2.058 s; all eight top-level tests and their strict
  input and limiter cases ran, including real DB success/failure controls.
- Vet: exit 0. Gofmt: no output.
- Existing limiter capacity regression: passed, 0.602 s. This ran before root
  generation; the auth test does not depend on the new SQL methods.
- Whitespace checks against `/dev/null` covered all five new code/query files,
  including untracked files that a normal `git diff --check` omits. Clean;
  `git diff --no-index`'s expected difference exit status was accounted for.

While the root session was recovering from an API resource error, the fields
implementer generated temporary SQL using the repository-pinned sqlc v1.31.1.
Both resolver suites passed with and without `-race`, plus vet, through its Go
overlay. Those temporary runs did not change generated workspace files. The
actual generated recipient file later matched that tested output byte for byte:
SHA-256 `bebfeff4b5486bf9c31ecbd1887ca8ff879d60170bcc8faa9f689f66042bd8ba`.

Covered behavior includes strict presence/null handling; exact matching and
historical normalization collisions; minimal private response fields; current
source-relative membership; trusted principal and password-session checks;
context rejection before candidate queries; real transfer/removal commits
injected between authorization and lookup; creation-switch independence for
existing issues; read-only/no-business-write behavior; query and header rotation
against the limiter; account rotation against the IP limit; local/shared limiter
failure; and database errors without private response leakage.

Default tests used the repository's agent CLI guard. No installed AI CLI or paid
provider account was accessed. Native UI, real authenticated router coverage and
the full `make check` pipeline remain the root session's integration work.

## Existing recipient-write restriction gap

The resolver rejects restricted candidates, but the existing shared workflow
`validTarget` currently checks only locked user existence, `disabled_at` and the
emergency denylist. A caller already knowing a target UUID can bypass resolver
availability checks through an invite or transfer. This is an existing writer
gap, not authority granted by a lookup. Shared workflow was left untouched as
instructed, and the finding was sent to root.

Affected responsibility/enrollment checks are `InviteParticipant`, `Transfer`,
`RequestInformation`, `ReturnInformation` and `SubmitAcceptance`. `validTarget`
also filters notification recipients, so changing that predicate globally would
silently change R52 notification policy.

Recommended scoped fix: add a transaction-borrowing recipient-eligibility helper
for those responsibility/enrollment checks. While the existing ordered user locks
are held, reuse the basic target check and read `GetPasswordCredential` through
the same transaction. Deny `must_change_password`; require a credential in
password mode; allow its absence in legacy mode; propagate other database errors
to rollback/503. Preserve each action's existing unavailable-target receipt and
keep notification filtering independent. `CreateInTx`'s explicit external handler
should use the same rule if closing every known-ID path.

Recommended regression: resolve an eligible target, then restrict it before a
direct-UUID invite/transfer; assert refusal and unchanged participation,
responsibility, handoff history and grants. Cover direct restricted/setup UUIDs,
complete password and legacy controls, shared-user-lock ordering against password
reset, credential-read failure rollback, and unchanged notification recipients.
