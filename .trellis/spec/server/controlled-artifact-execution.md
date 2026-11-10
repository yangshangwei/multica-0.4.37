# Controlled artifact execution

## 1. Scope and trigger

The `artifact-tar-v1` policy packages an operator-staged local synthetic release
candidate. `internal/artifact` owns pure canonical specifications and filesystem
packaging; `service/artifact_authority.go` owns the shared current-authorization
checks, while `handler/agent_artifact.go` owns HTTP admission and execution
transactions. It is a backend API contract, not a general shell sandbox, deployment
adapter, artifact registry or new approval UI. Existing generic approvals remain
self-reported and cannot be converted into controlled execution.

## 2. Signatures

| Boundary | Contract |
| --- | --- |
| `POST /api/agent-approvals/artifact-package` | Strict typed request and canonical UUID `Idempotency-Key`; atomically create pending approval and one unclaimed execution row |
| `GET /api/agent-approvals/{approvalId}/artifact-package` | Current authorized reader, `Cache-Control: no-store`, frozen specification and retained result |
| `POST /api/agent-approvals/{approvalId}/decision` | Human decision for a controlled row additionally binds `expected_spec_digest` |
| `POST /api/agent-approvals/{approvalId}/artifact-package/execution` | Only `expected_spec_digest`; one definite durable claim may invoke the packager |
| Existing `/execution` and `/cancel` | Controlled self-report refused; cancellation refused after claim; unbound behavior preserved |
| `artifact.New(Config)` | Requires operator `StagingRoot`, `OutputRoot`, `Identity`; unavailable configuration disables execution |
| `Packager.Prepare` / `Pack` | Verify trusted staging and roots, freeze spec, then package exact bytes for a committed execution ID |

Runtime configuration is `MULTICA_ARTIFACT_STAGING_ROOT`,
`MULTICA_ARTIFACT_OUTPUT_ROOT`, and `MULTICA_ARTIFACT_CONFIG_ID`. These are read by
server router setup. Configure before startup; paths and configuration identity
are never accepted from HTTP. Linux/macOS use descriptor-relative no-follow
operations; other platforms refuse the adapter.

## 3. Requests, provenance and lifecycle

Request fields are `agent_id`, `task_id`, `resource_kind`, `resource_id`,
`action`, `staging_id`, `source_revision`, `staging_manifest_digest`,
`input_manifest_digest`, `entries`, `target_environment`, `policy_version`.
Each entry has `path`, `size_bytes`, `sha256`. Fixed values are `issue`,
`package_release_candidate`, `local-synthetic`, `artifact-tar-v1`. Unknown,
duplicate and case-alias JSON fields, nulls and trailing documents fail closed.

Limits: 64 files, 8 MiB per file, 32 MiB total, 96 ASCII path bytes and eight path
components; 32 KiB request and 24 KiB canonical spec. Portable slash paths reject
empty/dot/traversal/absolute/volume components, reserved device names, exact or
case-fold duplicates and file/parent collisions. SHA-256 strings use 64 lowercase
hex characters. IDs use canonical nonzero UUIDs.

An operator seals `<staging-root>/<staging_id>/manifest.json` and its `files/`
directory. `artifact.StagingManifest` binds workspace, task/generation, agent,
resource, declared source revision and input manifest digest. The service account
and deployment operator are trusted; remote callers cannot write these roots.
Sealed staging directories/files use 0500/0400, output root 0700. No claim of
protection from malicious root or a same-UID operator is made. A revision string
is declared, not independently verified by Git; the manifest proves selected
input bytes and the operator's provenance statement.

Use the package's canonical helpers: fixed field order, compact UTF-8 JSON,
sorted entries, no HTML escaping/trailing newline, microsecond UTC timestamps.
Separate domain prefixes distinguish input, staging, request, invocation and
specification hashes. Request identity excludes generated time/IDs; the frozen
spec includes transaction-resolved principals, task/runtime/generation, root
identity and exact 15-minute DB admission expiry. Never hash PostgreSQL JSONB text.

Safe ordinary approval plans contain counts, declared revision, environment,
expiry and digests; full filenames/manifests/root identities remain in restricted
detail. The existing `production_release` risk category does not grant production
actions: the controlled action and environment remain fixed local packaging.

The public path revalidates verified credentials and current authority in the
admission transaction. Follow the reviewed lock graph: workspace KEY SHARE
NOWAIT; original task token when present; joined task/agent/runtime/issue and
originator/runtime-owner locks; invocation targets; independent human user/member
and original credential locks; managed binding fences when present; approval then
execution UPDATE NOWAIT. Member SHARE locks also fence role updates. Bound waits
for implicit uniqueness with a transaction-local lock timeout. Never retry an
uncertain commit or widen authority through supplied headers, a new credential,
another task, an empty capability or an approver whose authority was revoked.

| State transition | Durable fact and side effect |
| --- | --- |
| Create | Pending approval and unclaimed execution in one transaction; zero invocation |
| Decide | Human approves/rejects exact digest; no specification changes |
| Claim | Approved unclaimed row becomes `unknown` with original nonce before invocation |
| Definite claim commit | The sole owner may invoke the typed adapter once, bounded to 120 seconds |
| Verified success | Receipt `succeeded` and legacy approval `executed` commit together |
| Proven clean refusal | `failed`; no published output and confirmed cleanup |
| Panic/deadline/ambiguous commit or publication | Retained `unknown`; no automatic rerun or receipt recovery write |
| Replay | Original receipt or binding conflict, never a new execution |

After a definite claim commit the server owns the original bounded operation;
disconnecting the HTTP client does not cancel it or grant another owner. The
120-second execution context bounds that operation. Process loss, deadline or
uncertain finalization retains unknown; no background retry or takeover follows.

Internal finalization locks only retained approval then execution and checks the
original nonce/spec/state. It records prior admission and must not require live
workspace/task/agent rows, create activity after deletion or start another run.
Workspace teardown retains every bound approval by `approval_id` and all execution
rows, while legacy unbound cleanup remains. Missing workspace denies public reads
and new admission. An admitted operation may finish after deletion; the original
nonce owner still records both successful states atomically.

The adapter captures bounded verified input bytes once and writes those bytes in
sorted deterministic USTAR entries: regular 0644, UID/GID zero, epoch mtime, no
ambient metadata/PAX. It exclusively creates an execution-specific output,
syncs/closes/reopens/hashes, publishes without replacing an existing file, syncs
both directories and verifies final bytes and inode identity. A matching content
hash alone does not permit output-file substitution. Cleanup may remove only its
own original partial file; collisions or uncertain evidence are retained.

## 4. Validation and error matrix

| Condition | Required outcome |
| --- | --- |
| Missing/invalid operator configuration or partial schema | Refuse execution, no legacy fallback |
| Malformed/ambiguous spec, wrong fixed policy or input drift | Invalid contract, no new claim/output |
| Wrong current principal/task/resource, restricted profile, revoked approver | 403/404 and zero adapter invocation |
| Same key and canonical request | Retained original IDs/spec, no expiry extension |
| Changed key contents, digest, root identity or frozen binding | 409 conflict before invocation |
| Lock inversion/busy authority | Whole-transaction rollback/refusal; no server retry |
| Database time at/after admission expiry | 410; historical receipts remain subject to read authority |
| Claim commit uncertain | 503/unknown and zero invocation |
| Finalization/publication uncertain | 503/unknown, preserve claim and filesystem evidence |
| Existing file with unknown receipt | Not success and not permission to run again |

## 5. Good, base and bad cases

Good: an exact staged manifest is approved, one concurrent claimant produces one
deterministic archive, and current authorized readers retrieve the same receipt.
Base: absent configuration leaves the feature unavailable while generic approvals
keep existing behavior. Bad: turn a generic approved prose plan into a different
artifact action, report success through the old execution-note endpoint, or
reissue a side effect because the first request timed out.

## 6. Required tests and migrations

Pure tests live in `internal/artifact`; actual database/authority tests in
`internal/handler`, actual router/auth tests in `cmd/server`, and SQL/rollback
tests in `cmd/migrate` and `internal/migrations`. Use owned test databases per
package and the agent-CLI guard. Assert actual invocation counts and committed
row identities, both sides of revocation/cancellation/deletion races, immutable
old decisions, failure/unknown no-rerun, corrupt binding refusal and unbound
compatibility. Filesystem tests cover replacement and I/O uncertainty, not only
path syntax. Pure helper tests do not replace router credential evidence.

Migrations 766–770 create the table without FK or inline unique/primary indexes,
then three separate concurrent unique indexes, then validate their exact usable
shape. Index build failures have registered cleanup hooks. Highest down migration
770 first takes an ACCESS EXCLUSIVE NOWAIT fence and refuses any retained row;
766 repeats the fence before table drop. Separate concurrent index drops require
maintenance across the whole rollback. Never remove retained evidence to make
rollback possible, or run an old application lacking controlled-row guards
against these rows. Disable admission while retaining guards for operational
rollback.

## 7. Wrong versus correct

Wrong: mark legacy approval executed at claim, trust a local archive as proof of a
successful DB commit, or reclaim an expired unknown operation.
Correct: persist unknown before the side effect, finalize receipt and legacy
success in one transaction, and use read-only lookup to recover only an actually
committed result. Unknown is terminal for execution.
