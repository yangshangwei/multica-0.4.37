# BK-R03-02 — history query parsing, 2026-11-18

The R05 candidate fixes the [R03 finding](block-backend-r03-source-independent-review-20261118.md) for three human reads only: history collection, exact handoff, and the repair's affected-request list. Each still resolves current issue read authority first. An unrelated reader receives `404 not_found`; an authorized reader receives `400 invalid_information_query` for a malformed query, even when another pair is valid.

Previously `URL.Query()` silently dropped malformed pairs. The lists now parse the raw query once with `url.ParseQuery`, reject any parse error, and use the parsed values for the existing limit and cursor checks. Exact detail rejects parse errors and any supplied field. Valid pagination and service logic are unchanged. Creation routes were not changed or verified by this fix.

## Candidate files

Paths are relative to `/Volumes/artisan/.multica-acceptance/pay241-s3-clients-20261118-x4c5mjep/`.

| Changed path | R05 SHA-256 |
| --- | --- |
| `source-block-work-r05/server/internal/handler/collaboration_information.go` | `71ae10ad9cf06e05ba608a5639e7a97e34bb322e2b6eaeae06f9aad6f700fe5f` |
| `source-block-work-r05/server/internal/handler/collaboration_information_test.go` | `4524f6695bd24f902aca3aa85a0f1114b2e8405638352d7e5219bfe087dd845b` |

`TestCollaborationInformationHistoryWireAuthorityAndQueries` extends the existing isolated fixture. Controls cover cursor continuation to the older handoff, exact detail without query fields, the normal non-repair lookup denial, and an unrelated human on all three reads. The test hash is unchanged from RED; `gofmt -l` listed neither file.

## Coordinator verification

Receipts below are relative to `verification/block-package-continuation-r05/` under the same evidence root. Counts are named tests, including parents.

| Run | Outcome | Receipt |
| --- | --- | --- |
| Handler RED, before product edit | 9 passed, 6 failed: malformed-only on all three reads, mixed pairs on both lists, and the parent. Exact detail already rejected the mixed case. | `handler_red/result.json` |
| First handler normal, retained | 2 passed, 1 failed. The opening valid GET returned `503 collaboration_unavailable` instead of 200 before any new subtest ran. | `handler_normal/result.json`, `handler_normal/stdout.jsonl` |
| Fresh serial handler normal | 15/15 passed | `serial-normal/handler_normal/result.json` |
| Fresh handler race | 15/15 passed | `handler_race/result.json` |
| Service normal and race | 74/74 passed in each mode | `service_normal/result.json`, `service_race/result.json` |
| Candidate API build and vet | Both exited 0 | `candidate-build/build-receipt.json`, `candidate-build/vet-receipt.json` |

All listed test runs recorded zero skips and zero remaining connections. Final gate epoch was 2 for handler runs and 3 for service runs. The fresh passes do not erase or explain the first normal failure. This is package and build evidence only: no live API, browser, or runtime acceptance is claimed, and full S3 remains open.
