# Runtime enforcement slice (service_impl)

Worktree: `/Volumes/artisan/code/2026/multica-platform-admin`. Scope: daemon HTTP/WS registration, runtime/task authorization, claim/finalization fences and immutable installation snapshots. Parent owns installation cryptography, proof transport, binding lifecycle, auth-level bound-token validation, migrations and sqlc generation.

Status: bounded runtime-enforcement implementation and package verification complete. Parent/native-client integration remains pending.

Implemented:

- `service/managed_runtime.go`: current namespace/binding/epoch/principal checks; fresh runtime owner lookup; user-before-namespace registration/claim fences; task execution binding checks.
- Runtime/profile upserts use those locks and authoritative source owner. Existing foreign non-NULL owners cannot be replaced even before binding; existing NULL owners remain NULL. New rows may use the authenticated principal. Both builtin/custom/failed-profile paths follow this rule. Legacy merges refuse changes of ownership, including NULL-to-known, and retain the original vanished-target fence error. Bound registration skips hostname-based legacy history merging; legacy merge cannot consume a managed namespace.
- Daemon runtime/task access helpers, inline heartbeat/batch/deregister paths and WS runtime scope gates call current managed validation. Token workspace takes precedence over UserID when listing daemon workspaces.
- `daemonws.Hub.SetRuntimeAuthorizer(h.AuthorizeDaemonConnection)` is parent-wired; it rechecks upgrade and every inbound/outbound authorization using preserved authenticated context. Bound RPC runtime sets are pinned.
- Fresh claims capture execution installation/binding/epoch in the claim transaction; stale reclaims preserve historical snapshots. Finalization repeats runtime/binding checks. Bound task transactions retain user/namespace fence.
- Source SQL additions propagate only `auth.SubmissionInstallationFromContext`. Manual reruns and manual user retries use a fresh proof or NULL; they never inherit a prior machine. Only automatic retries, continuations and delegated recovery inherit the original submission snapshot when no new proof exists. Scheduled autopilot without a manual actor leaves submission NULL. No execution identity is copied into retry rows.
- New task-token and derived Remote MCP daemon-token provenance records the locked source binding ID/epoch, independent of historical task execution metadata. Caller-provided binding fields are cleared; derived workspace/daemon must match the source. Parent migration 493 and auth enforce that provenance.

Verification:

- Broad managed/claim/finalize/daemon regression passed (service 6.171s, handler 6.958s) before the final ownership hardening.
- Final service race passed 19.667s and daemonws race passed 2.348s after ownership hardening.
- Final handler race passed 73.049s with `go test -race ./internal/handler -run '^(TestManaged|TestClaim|TestFinalizeTaskClaim|TestDaemon|TestLegacyRegistration|TestLegacyCustomProfile|TestMergeLegacy|TestRuntimeProfileDeleteLock|TestPassword|TestHeartbeat|TestRPC|TestNotify)' -count=1`.
- `go vet ./internal/service ./internal/handler ./internal/daemonws` and `git diff --check` passed.
- New tests prove first-claim execution capture, historical no-backfill, current task/Remote MCP token provenance, mismatched derived-token rejection, first-bind vs legacy registration serialization, fresh owner checks, token workspace listing, WS cross-scope/outgoing owner checks, revoked/pinned RPC, manual A->Web NULL / A->desktop B source semantics, and automatic continuation inheritance.
- Ownership regressions prove foreign upsert denial, legacy-merge laundering denial, NULL-owner preservation, and parity for custom/failed profiles. Same-owner legacy/profile registration tests remain green.
- Existing partial-success injection now targets the second agent claim instead of the second transaction begin. Its assertion still requires returning the first committed task after a later claim fails.

Remaining integration limits:

- No real daemon, installed agent CLI, or native device was controlled by these tests. Parent/S07 owns production/native and load acceptance.
- Historical NULL ownership is intentionally not adopted by registration; any controlled ownership transfer needs its own explicit authorization path.

Parent auth now handles bound mat_ business-token provenance using token binding columns; runtime callbacks require the current bound MDT. The service transaction namespace fence is daemon-token-only so agent business writes keep their existing permission model.

## Revoked namespace downgrade regression

Parent review found that `ValidateManagedNamespace` treated a missing active binding as legacy authorization even when revoked binding history remained. A current owner PAT could therefore register or claim in the revoked namespace.

- Red: `TestManagedRuntimeRevokedNamespaceCannotDowngradeToLegacy` reproduced successful owner-PAT, unbound-daemon and no-session validation, accepted registration, and an actual queued-to-dispatched claim after revocation (service test failed as intended).
- Fix: when no active binding exists, consult the existing `GetLatestInstallationBinding`; any binding history rejects the legacy fallback. Storage errors remain errors. Never-managed legacy namespaces retain their existing behavior, and a credential carrying a revoked binding still fails as an invalid session. No migration or SQL change was needed.
- Green: `go test -race ./internal/service -run '^TestManaged(Runtime|Claim|Registration)' -count=1` passed in 7.828s. The regression asserts the rejected claim leaves task state and dispatch timestamp unchanged.
- Handler coverage adds revoked-history PAT registration, heartbeat and single-runtime claim rejection (403). Two legacy ownership tests had simulated legacy by revoking a managed binding; their fixtures now use a fresh, never-managed daemon namespace while preserving foreign/NULL-owner assertions.
- `go test -race ./internal/handler -run '^Test(Managed|LegacyRegistration|LegacyCustomProfile|MergeLegacyRuntime)' -count=1` passed in 13.199s after that fixture correction. `go vet ./internal/service ./internal/handler` and scoped `git diff --check` passed.
- Both commands used explicit `DATABASE_URL=postgres://multica:multica@localhost:5432/multica_platform_admin_s01_test?sslmode=disable` from the worktree root via `go -C server`.

Changed files for this repair: `server/internal/service/managed_runtime.go`, its test file, and `server/internal/handler/managed_runtime_test.go`. This closes a legacy downgrade path; parent still owns rebuilding and native/cross-slice acceptance.
