# Bounded independent daemon cross-check

Reviewed on 2026-11-20 in `/Volumes/artisan/code/2026/mc-f3-completion-20261120`. Read-only product review by the 7a implementer after reviewing `codex-daemon-review-resolution.md`. No daemon production/test changes made.

## Verdict

No new functional blocker found in the requested journal, capability, payload and recovery paths. The bounded review agrees with the adopted KD2 resolutions; it does not claim the earlier full mutation study was repeated.

## Checked against design 11.10

- Startup opens the journal after binding the health listener and before preflight/claim-capable traffic. The probe uses temporary write, file sync, rename, directory sync, delete and list. No usable journal means no protocol capability; existing managed workspace clients and future clients both inherit the atomic readiness flag.
- `handleTask` creates the record immediately before runner entry and seals after runner return. The two documented early exits (missing runtime and aborted local-directory wait) remain outside capture. HTTP budget submission runs in the background and the legacy `ReportTaskUsage` path remains separate.
- Both actual provider invocations in `runTask` use `executeAndDrainRecorded`, including fresh-session retry before the legacy usage merge. Model discovery/version/configuration processes do not become invocations.
- An invocation ID and launch intent are recorded before `backend.Execute`. `resumed` derives from the passed resume ID and extra/custom argument slices. Custom runtime profiles use `custom-<provider>`; notes are trimmed/validated and invalid versions omitted. `UsageNormalizerVersion` is1 for current providers and Claude derivation has the bump-contract comment.
- Result provenance is explicit: backend channel value, failed Execute, or synthetic result. Backend completed/failed returns its own usage, other backend outcomes are interrupted, synthetic outcomes carry no usage, and Execute failure becomes returned with empty usage. Scope uncertainty therefore becomes a durable gap server-side.
- The journal owns one batch key per canonical attempt. Private record version must equal1, files are regular and size-bounded, snapshots deep-copy slices/maps/bytes, records never mutate after sealing. Wire arrays are nonnull, model ordering is deterministic, ticks/tokens remain decimal strings, and cost-only rows are retained.
- Sealed batches persist as byte arrays and retries pass those bytes directly to the request body without decoding/reencoding. Recovery of earlier unsealed records builds unknown outcomes, uses the current producer epoch, persists the new bytes before the first send, and retains the original batch key.
- Delivery is a dedicated HTTP path with no redirect following or runtime/task404 cleanup side effects. It reads the entire response before accepting any2xx or permanent400/404/409/413/422 result; network/incomplete/3xx/401/403/408/425/429/5xx keep the record. Logs retain only safe error codes, not response text. Client timeout remains the request bound.
- Recovery only admits canonical record filenames; unsealed/no-launch and same-process-unheld leftovers are removed without asserting an empty execution. Running records remain owned by their task. Launch-intent write failure removes stale disk state best-effort while preserving live memory.
- Recovery compares exact server URL and selects the record’s managed workspace credential. Unavailable binding retains the report. A 32-record round-robin bound and30-second passes avoid starvation; in-flight claim marks prevent duplicate concurrent sends. Failed deletion is remembered as settled and retried without re-reporting.
- Retention is seven days for retryable records. Expiration-first may drop an old sealed report before its direct send; direct-send-first may deliver. Both orders are explicitly tested and neither changes the server hold.

## Executed verification

Guarded no-DB targeted run:

```sh
DATABASE_URL=postgres://invalid:invalid@127.0.0.1:1/f3_no_database?sslmode=disable \
  bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test ./internal/daemon \
  -run '^TestBudgetUsage(CapabilityNameIsFrozen|NormalizerVersionIsOne|RecordReadRejectsForeignFiles|BatchGolden|SealBuildsOnce|LaunchIntentWriteFailureRemovesTheRecordFile|RecoveryDoesNotSendABatchItCouldNotPersist|RecordBeingCreatedIsNeverTakenForALeftover|ReportGivesUpByTheClientsOwnBound|ExpiredSealedBatchIsDroppedWhenRetentionPassWins)$' \
  -count=1 -json
```

Result: EXIT0, 18 pass / 0 fail / 0 skip. Evidence `/tmp/f3-codex-7a/daemon-crosscheck.jsonl`. Includes new missing/zero private-version assertions. No provider/installed agent CLI invocation. Main retains the broader298-case normal/race evidence.

## Existing limits and one wording note

- A live process may send from memory after seal persistence fails; restart can rebuild different envelope bytes under the same key and receive permanent409. This is explicitly accepted by the 6b ruling and existing regression, not a new defect.
- Same-server account switching may lose a pending report to final404 (KD2-10); seven-day retention and filesystem failure also bound coverage. S1 retains unknown holds and makes no settlement/completeness claim.
- The comment on `budgetUsageResultFromFailedStart` says “no provider run exists” and `budgetUsageInvocationOutcome` says “used nothing”. The implementation correctly emits a no-usage gap; an Execute error alone does not prove no process started (design11.10). Optional wording correction: “Execute returned an error; no usable result” to avoid a future release-proof inference. This is not a functional blocker.
