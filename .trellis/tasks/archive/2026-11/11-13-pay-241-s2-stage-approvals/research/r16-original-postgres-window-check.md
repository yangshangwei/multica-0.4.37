# Research: Original r15 PostgreSQL lock-error window

- Query: Independently match retained PostgreSQL row-lock errors with the original r15 candidate GET failures, without replaying a request or assigning an unrecorded lock holder.
- Scope: Retained files and existing source only; writes limited to this report and its companion JSON.
- Date: 2026-11-16.

## Findings

### Original error evidence is now available

The peer-retained `postgres-r15-failure-window.log` is exactly1,630bytes with SHA256 `0af05198c488f63b190d5fbeafb9cfbc8461bff7a416d2f925961611cee7aee8`, matching the supplied binding. Its summary has SHA256 `2739a1fae1174a6c50ca636a8b6d6a84898be677fa629f78ae9971f9cd8c71e5` and records a successful read of the original PostgreSQL log window from02:16:55Z through02:17:14Z. This researcher did not re-run that command.

The raw window contains exactly two `ERROR` records:

| PostgreSQL timestamp (UTC) | Failing backend PID | Actual recorded error and statement |
| --- | --- | --- |
| 2026-11-16 02:16:57.543 | 39390 | `could not obtain lock on row in relation "issue"`; `LockLifecycleIssue`, `FOR NO KEY UPDATE NOWAIT`. |
| 2026-11-16 02:17:12.570 | 39491 | The same relation, named query and lock clause. |

Both statements show `WHERE id = $1 AND workspace_id = $2`. Neither arguments nor the locked row UUID are printed. Neither explicit SQLSTATE nor the blocking backend PID is recorded. The PIDs above identify the **failed** query backends, not lock holders.

### Match to original HTTP history

The original `artifact-config-repair-r15/harness.stderr.log` is2,200,943bytes with retained-file SHA256 `69acce9e5821dc032e8409eeaa328face09ddb88bea957505aae7720836539c6`. Its HTTP prefixes render the matching r15 UTC events eight hours later; the prefix itself has no timezone or subsecond precision. The table aligns those local rendered seconds to UTC, without inventing millisecond request timestamps.

All rows below use the same exact route:

`/api/collaboration/issues/01a1e224-a6de-7ea9-9d56-3c009b9c1460/stage/code-candidates/01a1e224-a964-769b-af29-70fbc780f8eb`

| Log line | Rendered HTTP time / aligned UTC second | Actual actor | HTTP | Request ID | Duration |
| --- | --- | --- | --- | --- | --- |
| 8512 | 10:16:56 /02:16:56Z | Product `486de1ce…` | 200 | `22dd27cf` | 103.931ms |
| 8557 | 10:16:57 /02:16:57Z | Reviewer `a56c5bfb…` | 200 | `7594e192` | 141.863ms |
| 8602 | 10:16:57 /02:16:57Z | Reviewer `a56c5bfb…` | 503 | `ced94b1e` | 12.227ms |
| 8603 | 10:16:57 /02:16:57Z | QA `0a4c2faa…` | 200 | `eaba2e3f` | 60.162ms |
| 8621 | 10:17:11 /02:17:11Z | Product `486de1ce…` | 200 | `82789b00` | 72.186ms |
| 8656 | 10:17:12 /02:17:12Z | Reviewer `a56c5bfb…` | 503 | `d589f97e` | 16.582ms |
| 8657 | 10:17:12 /02:17:12Z | QA `0a4c2faa…` | 200 | `db95fa5e` | 47.918ms |

Thus the two original PostgreSQL row-lock errors occur in the exact two rendered seconds containing the original reviewer's candidate GET503 responses. A successful same-candidate QA GET follows each failed reviewer response in log order. The reviewer's earlier same-candidate GET200 is also retained.

This is strong original-window correlation. It is not a request-ID→PostgreSQL-PID join: HTTP records have request IDs but no backend PID, while PostgreSQL records have backend PIDs but no request IDs or parameter UUIDs. Request start/end times are not captured with comparable subsecond precision, so durations do not establish which query held a lock first.

### Exact candidate and source-child identities

The original `stage-execution-evidence.json` identifies:

- Collaboration root: `01a1e224-a6de-7ea9-9d56-3c009b9c1460`.
- Source workspace: `18602f78-e225-4465-8c50-42834f332cdd`.
- Project: `f0e7079d-b901-4a45-a884-e0419be2c3cf`.
- Candidate: `01a1e224-a964-769b-af29-70fbc780f8eb`, version1, digest `ab2b969d0940ae8ab3208f824fafe7c729877f7a9e5fd3a83315b3edbfda6387`.
- Selected source child: `01a1e224-a7a2-794e-a81a-fd0ec5b7de7f`.
- Selected artifact approval: `239a4aa4-42f2-4feb-8136-08b9a8d45ad6`; execution `01a1e224-a8dc-7e19-a6fa-5ed44b9b8e85`.
- Original reviewer: `a56c5bfb-ff4f-464c-b926-e1b59c30ac00`; QA reader: `0a4c2faa-13e5-40ef-a3f2-9db05e528d75`.

The actual product candidate POST at02:16:55.913Z binds that child and artifact to the returned candidate identity and HTTP200 `stage_code_candidate_created`. The actual advance POST at02:16:55.994Z returns HTTP200 `stage_advanced`, `code_review`, stage revision2. The subsequent failed Reason fill did not send an approval POST.

The source trace establishes that this artifact-bearing R4 invokes `checkStageArtifactCurrent` → `stageArtifactObservation` → `EvaluateDeliveryEvidenceInTx` → `LockLifecycleIssue` on the selected child. The raw PostgreSQL log does **not** independently prove that `$1` was this child rather than another `issue` row; retain that distinction until the peer's exact traced parameters are available.

### Confirmed cause boundary and remaining uncertainty

The original PostgreSQL window confirms actual issue-row `NOWAIT` lock contention during r15. This materially strengthens the earlier source-only contention hypothesis. It does not, on its own, prove that QA was the holder, that a particular reviewer request used backend39390/39491, or that no concurrent writer held that row.

Two legitimate artifact-bound R4 reads can contend because the shared observation path uses a child `NO KEY UPDATE NOWAIT` lock, then approval/execution `FOR UPDATE NOWAIT` locks. The paired reviewer503 / QA200 history is consistent with such read-read contention. A lifecycle, metadata or other writer using the same row can also produce the recorded error. No holder query, lock mode, transaction boundary, commit/release time or explicit original SQLSTATE distinguishes these possibilities here.

The peer coordinator owns a trace that can bind the fresh actual reviewer request, backend PID, child/workspace UUIDs, first failing statement/error, holder identity and transaction release to complete that distinction. No new replay or probe is needed from this researcher.

### Existing deterministic regression target

Use the existing `stageArtifactReportFixture` and `stageSourceFenceStarter`, with the established `GetCollaborationAttempt` one-shot barrier after the first artifact observer holds its locks (`server/internal/service/collaboration_test_report_artifact_test.go:231`, `collaboration_stage_source_fence_test.go:15`). A second legitimate same-candidate R4 read through a copied service with the raw pool avoids wrapper recursion and arbitrary sleeps. This can prove read-read contention separately from a competing writer. Preserve the member/credential revocation and snapshot-rejection regressions; do not implement a lock-policy repair until the peer captures the actual request/holder chain.

Related spec: `.trellis/spec/server/collaboration-stage-approvals.md:48` requires current frozen-artifact authority, repeatable-read permission fences and rejection of an invalidated snapshot. The first research report contains the full source path and scoped byte bindings; it was preserved unchanged by this follow-up.

## Caveats / Not Found

- No explicit original SQLSTATE, original503 response body, SQL parameter UUID or lock-holder PID is present in these files. Do not label an inferred SQLSTATE as a recorded one.
- QA GET200 does not by itself prove that its response contained readable artifact evidence or that QA held the blocking lock.
- This slice performed only file reads, hashes and report writes; no API, database, browser, build, test or source change.
- Original canonical r15 remains1PASS/1FAIL/2NOT_RUN. This proof adds no canonical scenario or A11/A12 PASS.
