# Managed installation identity and runtime scope

Applies to `internal/installation`, managed auth/service/handler files and the
Desktop Main ↔ Go daemon integration. The platform task documents contain the
approved control design; this guide records the implemented identity boundary.

## Identity and enrollment

An installation is one deployment's identity held by one OS user. It is not a
physical device, telemetry install ID, hostname, runtime or login account. Store
the Ed25519 seed in the private shared management directory, never in renderer
state, command arguments or analytics. A separate stable daemon namespace is
combined with the authenticated user UUID to derive the wire daemon UUID; this
prevents an account switch from adopting another user's runtime namespace.

Enrollment and bind/renew sign the exact original server-issued canonical bytes.
The protocol checks purpose, method/path, deployment, organization, user/version,
workspace, daemon, public-key fingerprint, expected epoch, nonce, expiry and body
hash. Node and Go share wire fixtures. Do not reconstruct a semantically equivalent
JSON object and sign it. Canonical OS names are `macos`, `windows`, `linux`, `unknown`.

Desktop Main must map Node's `darwin` value to `macos` at this protocol boundary.
Do not change the runtime-discovery normalizer, which has a different contract.
The daemon profile `config.json` also contains credentials: Main writes it through
a unique `wx`/`0600` temporary file, syncs and closes it, then atomically renames it.
The managed Go reader rejects group/other POSIX permissions. Passing a mode only
to an ordinary overwrite leaves existing `0644` files unchanged, and truncating
the live path can break an open daemon reader. Windows ACL behavior requires
native verification; POSIX mode assertions are not evidence of Windows ACLs.

Regression: `apps/desktop/src/main/daemon-manager-managed-recovery.test.ts` covers
the platform mapping, new/existing profile modes and an already-open reader.

Challenges are persistent and single-consumption; only hashes of nonce/payload/body
are stored. The credential retry result is encrypted. A consumed successful bind
or renew may recover its original still-valid credential after challenge expiry;
it must not mint a replacement. Expired consumed enrollment can return identity
and binding hints but cannot mint a fresh metadata proof.

Same-authority profiles reuse binding identity/epoch and get separate credentials.
Authority changes/reactivation advance the namespace epoch. Never infer epoch
absence from a truncated hint list: enrollment is bounded, and the targeted
binding-hint endpoint requires the full human session plus matching current
installation proof. Revoked/old-version hints are recovery information, not
authorization. The actual bind still performs its locked compare-and-set.

## Runtime authorization

Use `ValidateManagedNamespace` / `ValidateManagedRuntime` for current runtime
scope, and the locked variants before registration/claims. Managed resource
traffic uses its workspace-scoped MDT; the global PAT is discovery only. Check
current binding, source account version, owner, deployment and membership at
HTTP/WS boundaries and again inside write transactions. WebSockets validate
incoming and outgoing messages; preserving the authenticated context matters.

Pass the caller's `RuntimeLookup` into managed validation and locking helpers.
For transaction paths, it must contain the transaction's queries, the existing
metrics collector and a source label. Using pool queries here can escape the
authorization transaction; direct `GetAgentRuntime` calls lose read accounting.

A namespace with revoked managed history stays managed. Looking only for an
active binding and treating no row as legacy permits a downgrade to PAT after
revocation. Check latest history before granting legacy behavior. The enrollment
feature flag only disables new enrollment; it cannot remove existing authority
checks.

Registration cannot change an existing runtime's non-null owner or convert an
unknown legacy owner into the authenticated caller. Hostname-based merge cannot
launder ownership or consume managed namespaces. Use the existing owner checks
for built-in, custom and failed-profile registration paths alike.

Locks preserve the shared user → namespace → existing resource ordering. Binding
also takes the workspace fence before locking the challenge, so workspace deletion
cannot deadlock while expiring challenges. Do not add a reversed task/credential
lock order in a new callback.

## Provenance and read models

The short-lived `mip_` proof uses a separately derived key and is attribution only.
It cannot authenticate a human. Validate it with the ordinary JWT/PAT, current
account version, deployment, installation lifecycle and key version before putting
an installation ID into request context. Arbitrary installation headers/body
fields are not evidence.

Submission identity and execution identity are separate. Manual retries/reruns
use the new request's proof or NULL; automatic continuations preserve the original
submission snapshot. First claim captures verified execution installation/binding.
Historical unknown execution identity remains NULL. Task-token binding provenance
is independent of task-history snapshots, so reclaimed legacy tasks still receive
properly scoped derived credentials.

Client heartbeat freshness is not daemon liveness. Boot/sequence/account-version
checks prevent replay from refreshing activity. Read DTOs expose separate client,
daemon and readiness axes; source failure is unavailable, not healthy or a mass
offline event. Missing model usage stays unknown. Private content remains behind
the original resource authorization.

## Execution controls and delivery generations

Admission is a persisted server policy. The shared installation lock precedes
agent/task claim locks, and SQL claim/reclaim queries repeat the policy check.
An already admitted managed execution may recover while stopped only with its
original binding and admission snapshot. Missing historical admission proof may
recover while accepting; do not fabricate proof for a stopped installation.

Keep three versions distinct: task state_version fences never-dispatched admin
requests; runtime + original dispatched_at identifies an in-flight execution;
claim_generation identifies one delivery attempt. Managed reclaim preserves the
execution identity but advances delivery generation. Finalization, comment
receipts, failed-response requeue and claim-only cancel/fail must compare that
generation inside their write transaction. Preserve the original claim's
trigger/comment snapshot after locking the current row; substituting the current
trigger would authorize a payload prepared from stale provenance.

Each actor/idempotency key retains its own operation and audit. A cancellation
root owns daemon evidence; followers inherit its effective result and recover
their own phase audits. A SKIP LOCKED batch shorter than its limit is not proof
all followers are synchronized: check for remaining differences before removing
the root's retry schedule. Definitive fence conflicts retain failed receipts so
a client can safely leave an obsolete request after original-key reconciliation.

Only an explicit, strictly decoded stopped receipt with current binding and
matching execution fence confirms a managed cancellation. Legacy field cleanup,
not_observed, completion-channel closure and expired acknowledgement windows do
not prove process exit. Local receipt storage remains scoped to the original
installation/user/version/binding/task/fence, including after restart.

## Verification sources

- Protocol and signed payloads: `internal/installation/*_test.go` and shared
  `internal/daemon/testdata/managed-*` fixtures.
- Enrollment/recovery/audit/lock races: `service/managed_installation*_test.go`.
- Runtime ownership, downgrade and claim fences: `service/managed_runtime_test.go`
  and `handler/managed_runtime_test.go`.
- Actual authenticated HTTP and task provenance: `cmd/server/managed_installation_test.go`.
- Client private persistence, cross-process locks, HMAC and session transitions:
  Desktop `managed-*.test.ts` and Go daemon/CLI managed tests. These tests do not
  replace installed-app or native Windows acceptance.

## Lost claim responses and recovery evidence

A committed dispatch whose response is lost must not be delivered again immediately. Recovery requires the dispatch to be older than the 90-second response-recovery window and its prepare lease to be absent/expired; the Redis hint adds a five-second margin before normal polling. A 90-second client observation timeout therefore cannot prove permanent loss. Reclaim advances `claim_generation` while preserving the original execution binding/epoch/admission, and must not reclaim a started task. Regression: `TestManagedBatchClaimLostResponseRecovery` exercises real managed HTTP claims and database eligibility with fixture timestamps; it does not measure Redis or wall-clock scheduling.

Do not casually turn cancellation cleanup into an unconditional detached requeue. Requeueing previously admitted dispatched work makes it queued again, where a concurrent installation stop can block fresh admission. Generation fencing alone does not settle this lifecycle difference.

Native transport tests must forward WebSocket Ping/Pong end to end and propagate downstream HTTP cancellation. The daemon refreshes its read deadline after each successfully read application message and on Ping/Pong; the server refreshes its read deadline on Pong. A proxy must forward control frames to preserve idle/fault behavior, but missing control forwarding alone does not establish the cause of a closure while application messages continue. Under load, correlate classified client/server failures instead of inferring the cause from a 60-second interval. Record proxy-observed ACK arrival separately from daemon processing or request-correlated RTT, and verify task/provider execution evidence independently of the driver's pass flag.

## Per-frame connection scope and RPC teardown

`AuthorizeDaemonConnection` validates the managed namespace/account/binding freshly for each call, then reads every current runtime and checks owner/workspace/daemon against that binding. Reuse is local to that single invocation; do not add a cross-frame authorization cache. Legacy paths and transaction-time claim locks remain independent. Regression: `managed_connection_scope_test.go` covers mutation between calls and the bounded query count.

A managed wakeup connection attaches and detaches its own `rpc`, never the daemon's unrelated default RPC. Pending sent calls remain uncertain; detachment must not authorize immediate duplicate claims. Regression: `TestManagedConnectionDetachFailsPendingRPCAndPreservesOtherTransport`. Classify connection errors without logging credential-bearing endpoints or peer text, and correlate workspace/phase/time; CLI RPC generation and proxy socket ordinals are different identifiers.

Windows management-ticket readers must include `FILE_SHARE_DELETE` as well as
read/write sharing, so a peer's atomic replacement or release can proceed while
the reader retains the old snapshot. Never treat an unreadable live ticket as
absent. Preserve Go's long-path behavior when using `CreateFile` directly.
Native regressions: `managed_lock_windows_test.go`; the Windows acceptance job
also repeats concurrent identity creation twenty times.
