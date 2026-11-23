# S2 server increment 2a — independent check

- Reviewer: `/root/s2_server_check`; reviewed on 2026-11-14.
- Verdict: **changes requested for the frozen increment**. Two concrete findings are routed to the existing server owner. This is neither whole-S2 nor A01–A12 acceptance.
- Ownership: read-only review. Only this report and its JSON companion were written in the repository. No product, test, contract, specification, database, git index or runtime was modified by this reviewer.
- Primary source: `/Users/artisan/.multica/acceptance/pay241-20261114-resume/s2-2a-source`. All 45 bound source/contract hashes matched the increment report. The coordinator's larger `s2-2a-verification-source` snapshot supplied unchanged dependencies for static verification; the active 2b workspace was not substituted for frozen 2a.

## Findings (fixed)

None in this review. The server owner is actively implementing 2b, so even local fixes were routed instead of racing their writes.

## Findings (not fixed)

### S2A-C01 — register the stage indexes for interrupted-build recovery (P1)

**Location:** `server/cmd/migrate/main.go:122`, and S2 concurrent-index migrations 806–818 and 822–824.

The migration runner's `concurrentIndexCleanups` map has no entries for these 16 S2 indexes. The existing pure `TestEveryConcurrentUpBuildHasCleanup` fails on every one of them. A concurrent build interrupted after PostgreSQL creates an invalid index leaves that relation behind; the next normal `migrate up` cannot recover it through the repository's established cleanup hook. The new 822–824 files use unconditional `CREATE UNIQUE INDEX CONCURRENTLY`, so the retry hits the already-existing invalid relation instead of rebuilding it. Successful empty-database migration and the later ready check do not exercise this recovery path.

The observed test also reports eight unrelated `report_snapshot` indexes (795–799, 801–803). Those are explicitly outside this finding's S2 change list and must not be silently mixed into the stage commit.

**Required follow-up:** register all 16 S2 index names using the existing map; retain the original failing run; verify the stage registrations and actual invalid-index retry without weakening the repository-wide test or dropping unrelated findings. Include future 2b indexes when they are introduced.

**State:** routed; the coordinator reproduced the same failure independently and assigned the S2 repair to `/root/s2_server`. No corrected version was reviewed here.

### S2A-C02 — a source-denied child digest discloses private description guesses (P2)

**Location:** frozen `server/internal/service/collaboration_stage_view.go:352` and `server/internal/collaboration/stage.go:148`; wire §13.7 already records the limitation.

R1 returns both the ordinary child's title and `SHA-256(canonical {title, description})` to an authorized collaboration reader even when `can_open=false`. The description itself remains source-private. An observer can therefore distinguish a small set of candidate descriptions entirely offline. This is more than equality across two server responses: for the synthetic title `Synthetic payment exception`, comparing the two guessed descriptions `manual refund rejected` and `manual refund approved` uniquely recovers the latter from the exposed digest. The proof uses synthetic strings and the exact ASCII canonical encoding of the current helper; it does not read any private business description.

R4's new source-denied projection correctly removes baseline/evidence/history, but it does not remove this existing R1 disclosure. A statement that the digest contains no plaintext is insufficient to close this boundary.

**Coordinator resolution:** amend the frozen contract with a dated §13 entry: R1 child `content_digest` is explicitly `null` when source access is denied and remains hex64 for source readers. Disable unsupported fresh A5 child selection for the null variant while preserving existing title/ID references and minimal history. No new crypto/key infrastructure or fake digest is needed. The coordinator assigned the server and shared client follow-up.

**State:** open/routed against frozen 2a. Closure requires the later server projection, strict client null handling, and source-revocation/malformed-response regressions; those changes are not retroactively part of this reviewed snapshot.

## Additional verification request

**V01 — original-decision expiry at the final A9 boundary.** In frozen `collaboration_stage_review.go:553–582`, the dependency expiry check precedes `checkStageArtifactCurrent`, another database-clock read for CI, and the caller credential check. The final clock/caller check does not revisit the original dependency's expiry. A deterministic delay after line 557 should establish whether a short-lived dependency can expire before `AdvanceCollaborationStage` while the caller and CI remain valid. This reviewer did not run a DB-backed interleaving, so this is recorded as an unresolved verification request, not a reproduced third defect. The coordinator received the exact path and recommended test seam.

## Verified boundaries

- A6 binds an immutable code candidate to the configured implementer, root requirement digest/lifecycle, child's semantic version, exact PR/head, integration identity/trust and optional M1 artifact observation. Ordinary status changes do not change that semantic identity.
- CI consumption requires a complete, nonempty current-head check set, every check completed/success, and a nonfuture observation younger than 30 minutes. Its digest excludes refresh time. Unsupported VCS repository/check identity does not inherit the UI's permissive normalization.
- A7/A9 use the retained original signer credential, participant/account/configuration revisions and source membership-row identity. A rejoin cannot revive an approval. Current role and original implementer separation are both checked. Private signer metadata has generated `json:"-"` treatment.
- R4's source-denied branch returns null baseline, unavailable observations, empty report/decision/execution arrays, false/null signability, identifier-free blockers and a fixed withheld-group count. Reason-only return does not manufacture successful evidence; supplied optional references still receive same-candidate/read checks.
- A8 appends a revocation and retains the original decision. The root workflow transaction and original-key receipt contracts are reused. Reopen resets the business stage/revision; approval consumption checks lifecycle and policy.
- The GitHub producer rejects unknown/null/malformed/nameless check nodes; older mirror observations and stale derived links are refused, and equal-time/different-head ambiguity invalidates strict snapshot completeness.
- Stage policy activation remains false in the reviewed increment. QA/report, release/execution, all final writers, activity/readiness and complete A01–A12 acceptance remain later increments.

These statements combine code-path inspection with the increment's retained targeted evidence. They do not add a real GitHub integration, real signer, payment, deployment or Web/Desktop acceptance claim.

## Verification

| Check | Result | Evidence and limits |
| --- | --- | --- |
| Frozen input identity | PASS | 45/45 source/contract files match; original implementation logs and coordinator hash audit are retained. |
| Lint/format | PASS | `gofmt -l` on the frozen increment's Go files; exit 0, empty output. |
| Go type/static analysis | PASS | `go vet` on frozen service, handler, ghsnapshot and server command packages; exit 0, empty output. No TypeScript files are in this review. |
| Independent migration-map test | FAIL | One named test failed, zero skipped. It reports 16 S2 and eight unrelated missing registrations. This was a pure selector with database/MULTICA/PG/Redis environment removed and private Git-external state. |
| Targeted service/handler/producer tests | Retained evidence | Implementation report: service normal/race 107 named each; handler normal/race 72 each; pure snapshot race 17. Not rerun on an actively shared database by this reviewer. |
| Additional coordinator producer verification | PASS | Frozen ghsnapshot full package with race: 42 named, zero failures/skips; task-owned package database. |
| Additional coordinator migration verification | PASS, bounded | Actual empty 825→821 down/up and retained synthetic code-history refusal at 825 before index/ledger removal. This does not replace interrupted-build recovery. |

Raw reviewer files are under `/Users/artisan/.multica/acceptance/pay241-20261114-resume/s2-2a-check/`; exact argv, hashes, counts and coordinator references are in the JSON companion. An initial read command used a nonexistent `collaboration_access.go` path; it was corrected to `collaboration.go` before inspection and caused no writes. No whole-package service/handler suite, full race campaign, actual router/password journey, positive selected-artifact journey, browser or Desktop launch ran in this review.

After the findings are fixed, recheck only the changed contracts and implicated behaviors, then retain the required integrated acceptance. Capture the final stage contracts in a dedicated server spec when 2b–2d settle; task reports alone should not become the long-term authority for the released protocol.
