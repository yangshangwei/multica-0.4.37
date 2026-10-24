# Limited collaboration administration

Apply to R38/R40 administration, its read/operation endpoints and the shared
Web/Desktop controls. This exception never grants ordinary issue content,
participation, transfer, resource organization or owner-consent authority.

## Identity and disabled accounts

Require administration enabled, password mode, a complete current human
password session, super-admin role and the active source organization mapping.
Workspace owners, ordinary participants and platform observers do not qualify.
Keep the real administrator throughout history/audit; never substitute the
creator or add an administrator participant.

Disabled means persisted `disabled_at` or the existing emergency denylist.
List, detail, lifecycle and double-disabled successor rules share that meaning.
Missing passwords, required password changes, source departure or ordinary
participation loss do not themselves enable R38. SQL list filtering receives
the same normalized read-only denylist snapshot before LIMIT; do not expose
internal email/filter values in the public DTO.

## Reads and scope

The dedicated prefix is `/api/admin/collaboration/issues`. List/detail contain
only identifier, source/C/H summaries, status/revision, queue state and unsettled
run count. Detail adds the existing status-option contract. Every list, detail
and candidate response carries `scope`, the organization authenticated in that
same read transaction. A later `/admin/me` request cannot safely correlate an
already returned response.

Candidates are existing effective participants, available only for a closed,
double-disabled issue. Fill a visible page with bounded keyset reads after
eligibility filtering. Its cursor must name the last returned eligible user;
never return a hidden-user cursor or an empty page with a continuation cursor.
Clients validate organization, issue, revision and page continuity before cache
or selection. Refresh never selects a replacement successor automatically.

## Atomic writes and recovery

Only complete/cancel/reopen are admitted. Capture UUID request key, revision,
expected status, target status/options version, reason and conditional successor.
The reason is nonempty, NUL-free UTF-8, at most 1,000 bytes after trimming;
status keys are bounded to 128 bytes. JavaScript character counts are not byte
limits. Only double-disabled reopen accepts and requires a successor; even
explicit null is invalid in other scenarios.

Keep the platform advisory fence, sorted user locks, workspace before
organization/mapping locks, then the existing catalog/W01/iteration/issue and
collaboration locks. Recheck complete session, role/scope, effective disable,
candidate membership and every captured version under the locks. Do not retry
an unknown commit as a new operation.

Reuse transaction-local `changeHandler("admin_reopen")` and `changeLifecycle`.
The two existing operation ledgers, successor, to-do, information-request
supersession, status/queue, audit, history and outbox commit together. Reopen
retains the original creator, cancelled commands, paused queues and old proof
barriers. A business lifecycle result does not prove any process ended.

`admin_operation` retains the real actor/organization/request and audit result;
`collaboration_operation` stores the exactly correlated minimal receipt. The
dedicated original-key GET rechecks current administrator/source authority.
After creator restoration, POST rejects even an old key; the original GET still
recovers an existing result. Restoration never permits a new exception write.

## Shared client behavior

Use the admin Query scope and synchronous session cache boundary. Preserve the
exact original input before POST and across remount/empty lookup. Do not persist
server receipt data or automatically issue a write/new key. A confirmed receipt
may retain its original intent until explicitly dismissed, allowing GET recovery
when fresh detail is refused after creator restoration.

Web uses `/admin/collaboration`; Desktop uses the existing account tab with
`administration=collaboration`. That mode mounts only the dedicated page and
keeps separate mementos. The shared page itself checks full admin access. It
cannot borrow normal collaboration capabilities or a cached role.

Canonical tests: `admin_collaboration*_test.go`, core `admin/collaboration-*`,
shared views `admin/collaboration/`, and both
`e2e/collaboration-administration-*.spec.ts`. Test role/disable/candidate/version
changes under lock, required-write rollback, uncertain commits, original GET
recovery, exact scope and native account-cache teardown. Browser evidence must
come from an independent password/admin-enabled task environment.
