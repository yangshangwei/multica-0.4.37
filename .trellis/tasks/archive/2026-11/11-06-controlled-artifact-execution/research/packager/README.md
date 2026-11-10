# Canonical specification and local packager lane

Implemented in `server/internal/artifact/` only. The package is pure with respect to database, account and network access. Authorization and durable one-time claims belong to the coordinating API transaction; a filesystem collision never recovers or proves a prior successful execution.

## Integration surface

- `ParseRequest(io.Reader)` validates every required request and entry field, duplicate/unknown/null JSON, exact field names, portable paths, sizes, digests, fixed action/environment/policy, and returns sorted entries.
- `RequestDigest(Request, Binding)` binds the normalized request to verified initiating principal/workspace. `CanonicalSpec` and `ParseSpec` bind exact normalized bytes, runtime/generation/originator, opaque root identity, normalized invocation and six-digit UTC DB timestamps. `Prepare` fixes expiry at DB creation time plus 15 minutes.
- `New(Config{StagingRoot, OutputRoot, Identity})` pins real root/ancestor identities; absent or invalid operator configuration returns `ErrDisabled`. Other admission problems return `ErrInvalid` without raw paths.
- `Prepare(ctx, request, binding, databaseTime)` verifies the operator manifest and exact input bytes before returning a spec. `ValidateStaging` can recheck that binding. `RootIdentity` is an opaque hash, not a pathname.
- `Pack(ctx, executionID, spec)` is invoked only after the sole owner sees definite durable claim commit. It revalidates the frozen roots and staged bytes, writes deterministic USTAR from a bounded verified snapshot, publishes exclusively, syncs/reopens/rehashes and returns `Outcome` (`succeeded`, `failed`, `unknown`). It never retries.
- `DecodeStrict` serves small decision/execution bodies; callers still validate required values. It rejects case aliases, duplicate/unknown/null keys, trailing JSON and bodies over 32 KiB.

Operator layout: `<staging>/<staging_id>/manifest.json` plus `<staging>/<staging_id>/files/<approved-relative-path>`. Staging root, generation and all directories are sealed `0500`; regular files/manifest are `0400`; output root is `0700`. Manifest keys are workspace_id, agent_id, task_id, task_generation, resource_id, source_revision and input_manifest_digest. Manifest digest uses `StagingDigest` over canonical semantic bytes. Source revision is declared, not independently Git-verified.

Successful output is `<output>/<execution_id>/artifact.tar`, with a private execution directory and an opaque execution ID/digest in the public outcome. `linkat` exclusively publishes without overwriting; removal of the temporary link precedes the final single-link verification. Partial or final output is preserved on uncertain cleanup/publication/sync/close/reopen/deadline/panic. A replaced partial is never deleted by failure cleanup.

## Validation

Exact source hashes and counts are in `result.json`; raw red and green records are retained alongside it.

- macOS arm64: 15 top-level tests / 78 named PASS, zero fail/skip, both normal and race; focused vet passed.
- Linux amd64: the exact cross-compiled static binary ran in the cached pgvector image, read-only root, no network, separate executable tmpfs; 78 named PASS, zero fail/skip. This is an emulated Linux kernel/container run on the macOS host, not a native host-platform claim.
- Windows amd64: cross-compilation passed for the fail-closed unsupported-platform implementation; no Windows execution claim.
- Red evidence includes actual acceptance of a same-byte replacement output inode, JSON case aliases, and deletion of a replaced partial. Regression fixes retain those failures and subsequently pass.
- First Linux attempt exited 125 before tests because Docker could not bind the host's private temporary path. The successful run streamed the same binary bytes over stdin to a container-only tmpfs; the failed preparation is retained.

Tests used the coordinator's Git-external state directory and `scripts/go-test-with-agent-cli-guard.sh`. No database, real agent CLI, provider/model, authenticated account or network was used. No dependencies were added.

## Limits

This lane does not prove the DB claim/approval/receipt transaction or route authority. Privileged mount injection and malicious root/operator/same-UID processes are outside the trusted-operator boundary. Linux race and Windows execution were not run. An independent reviewer must review this implementation before the overall child is accepted.
