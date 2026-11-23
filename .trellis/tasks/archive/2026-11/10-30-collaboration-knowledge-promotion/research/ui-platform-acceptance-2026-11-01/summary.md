> Capture update: complete packet revision 2 is ready for full re-review after VR-E01–03. See [recapture-summary.md](recapture-summary.md), [review-gallery.md](review-gallery.md) and [capture-packet.json](capture-packet.json). All earlier evidence remains retained.

# UI/platform acceptance — 2026-11-01

Final result: **8/8 independent cases passed together in 49.253 seconds**, with zero skips, flaky outcomes or automatic retries. Strict E2E TypeScript, scoped ESLint and whitespace checks passed. Production Web and real native macOS arm64 Electron use the frozen sibling checkout.

## Source and ownership

Base `53a1662586555d14ea04efb3b951744a4993fa60` plus an explicit stable copy of 117 server source changes, including 61 new files, from the implementation checkout. Every copied file has before/after/copy SHA-256 in `snapshot-source.json`; the final check found no drift. `snapshot.patch` plus `snapshot-new-files/` retains the complete overlay. This is preliminary UI acceptance of those bytes, not final combined G2 source.

Changed implementation files: `e2e/collaboration-knowledge.spec.ts` and `e2e/fixtures/collaboration-knowledge.ts`. No product file, dependency, shared existing helper, task plan, commit or runtime experiment ledger was changed. The new fixture reuses TestApiClient, memory authority, existing retained-run records and the native routing/IPC harness. Shared journey assertions run on Web and Desktop.

## Verified behavior

- External current handler with no source membership proposes from comment, shared-context memory and retained run records, using lawful current source destinations. The comment/current-project path makes zero project/member catalog requests.
- Human keyboard confirmation sends the exact displayed version/state revision/content hash. Correction creates version 2, supersedes atomically on approval, and workspace derivation retains its distinct source link. Project/workspace base editors remain separate and unchanged.
- Open source and target views clear purged required material. A target-only administrator receives content-free target revision hints, loses source text/link/reject controls, and successfully withdraws without recovering hidden source. Withdrawn restricted audit rows remain legitimately visible.
- Ambiguous delivery retains the original candidate/request/payload, stays locked after an unsealed refusal, and recovers only through GET after reload. A definitive unapplied refusal unlocks editing and the changed intent uses fresh IDs.
- An explicit 12-second expiry hides active material at its measured deadline while refresh GETs fail and online state remains unchanged. The earlier offline variant proved disconnect clearing only and is not expiry evidence.
- Native Electron executes the same source/review/version/project/workspace journey, observes a live authenticated account socket, and clears an open workspace contribution after purge with a real target hint. Daemon starts and ordinary engine tasks remain zero.
- Complete panel coverage at 1380 and 640 logical pixels is recorded as overlapping viewport images with scroll geometry, PNG dimensions, hashes and traces. See `review-gallery.md` for the final set.

## Failures retained and resolved

There were 39 actual case executions across development runs: 29 passed and 10 failed; these are repetitions of the same acceptance cases, not 39 independent tests. One separate test-module initialization attempt executed zero cases. All failures, current-source captures and redacted traces are retained under `runs/` and logs.

Fixture/environment corrections: source invalidation can arrive after a proposal response, so confirmation waits for its actual source cursor and ensuing refetch; project properties use an existing narrow drawer and retain DOM inside a collapsed desktop panel; the workspace settings tab is General; withdrawn target audit rows are retained; native WS required the task renderer origin in CORS_ALLOWED_ORIGINS. A test-only attempted import of a non-exported schema was removed after typecheck/initialization failure. Target hint listeners now attach before socket creation, and the expiry case measures expiry rather than disconnect. No product defect or product repair was established by these failures. Early interrupted native trace archives are explicitly labeled; valid final traces are complete.

## Boundaries and cleanup

This proves real HTTP/UI projection and controls with synthetic actors. The run-record setup uses the existing deliberately seeded retained-history fixture; it does not prove runtime ingestion. Native evidence uses a real Electron compositor, production renderer/preload/native routing and IPC, with test-owned daemon services. It is not packaged-installation or full Main bootstrap acceptance. Authentication is fixture setup, not a new login-form acceptance claim. No real Provider/account/model or user-installed agent CLI was used. No next-input injection, model understanding, Windows/Linux matrix, release readiness or final G2 enablement is claimed. Main still owns independent screenshot review and any rerun required by later combined source changes.

All owned API/Web/static listeners are stopped; no native process/profile remains. The dedicated DB and frozen checkout are retained. Execution/host gates were false throughout; final cleanup also closed the restored human-creation gate. `gate-safety.json` records zero ordinary tasks and before/after gates. `cleanup.json`, `commands.md`, `environment.json` and final running records identify ownership and exact restart/destroy commands. No old runtime environment was stopped or modified.
