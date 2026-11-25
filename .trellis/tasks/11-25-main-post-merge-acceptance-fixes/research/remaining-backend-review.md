# Remaining backend independent final review

**Accepted: the selected81 normal +81 race package executions on the recorded frozen candidate, with six explicit skipped tests per mode.** This is bounded backend evidence, not whole-product or current-checkout acceptance. The original failing runs are preserved.

| Evidence set | Packages per mode | Named PASS | Named FAIL | Named SKIP |
| --- | ---: | ---: | ---: | ---: |
| Original normal / original race, each | 81 | 11,118 | 36 | 7 |
| Corrected normal / corrected race, each | 5 | 1,048 | 0 | 0 |
| Selected normal / selected race, each | 81 | 11,155 | 0 | 6 |

Selection replaces only artifact, storage, auth, middleware and WeCom with corrected receipts. It does not add both versions, the independent diagnostic, or historical passes. Each selected mode has5,078 top-level PASS/5 SKIP and10,354 leaf PASS/6 SKIP. Nine packages have no tests: their terminal package SKIP proves compilation only and contributes no named PASS. The five delegated service/handler/cmd-server/daemon/collaborationrun packages are excluded.

## Findings (fixed)

- **RB01 — inherited umask.** File: `.omx/private/acceptance-fixes/remaining_backend.py`; observed at `server/internal/storage/local_atomic_test.go:81`. The original077 mask changed requested0644 uploads to0600. Parent's new `correct_remaining.py` pins child022 while retaining private0700 HOME/TMP. Full storage normal/race each99 PASS.
- **RB02 — artifact ancestor ownership.** File: `server/internal/artifact/files_unix.go:94`. `/Volumes/artisan` is UID501/mode0775, which fails the unchanged secure-root ancestor guard. Corrected task TMP is under canonical `/private/tmp`. Full artifact normal/race each78 PASS. No product guard or assertion was weakened.
- **RB03 — missing shared-limiter opt-in.** File: `server/internal/auth/password_limiter_integration_test.go:18`. Original supplied only REDIS_TEST_URL. Corrected supplies MULTICA_TEST_REDIS_URL against the allocated Redis; auth normal/race each117 PASS, including TestPasswordLimiterSharedAtomicAdmission.
- **RB04 — actual Redis DB ownership.** Auth/middleware/WeCom override URL indices to11/13/15 and flush those DBs. Corrected execution uses its own verified Redis container, so it cannot collide with the original runner's indices. Middleware each138 PASS; WeCom each616 PASS.
- **RB05 — completion versus acceptance.** Original summary.complete is execution-count evidence only. This review independently verifies raw terminal events, expected packages, hashes, source and resources; parent does not treat runner exit0 or complete=true as test success.

This reviewer changed only the two review artifacts and executed the separately authorized pure artifact/storage diagnostic. Harness fixes belong to the parent. No source changes or new tests were needed.

## Findings (not fixed)

**Six retained skips, each mode:**

1. `cmd/migrate::TestCommentContentBigramRequirementMatchesRealPGBigm` — owned PostgreSQL lacks pg_bigm.
2. `internal/cli::TestCLIConfig_UnknownFieldsArePreserved` — existing unconditional known limitation: encoding/json drops unknown fields on round-trip.
3. `pkg/agent::TestACPManagedTerminalPreservesWindowsUnicodeShellOutput` — Windows-only cmd code-page behavior.
4. `pkg/agent::TestCodexExecuteRetriesAfterSignaledProcessIsReaped` — Linux-only signal/reap semantics.
5. `pkg/agent::TestPiExecuteAttachesStdinPipe` — requires Linux procfs.
6. `pkg/agent::TestClaudeRootSudoPreflight/root_bypass_without_sandbox_errors` — root-only preflight on this nonroot host.

These are unchanged historical gaps, not PASS and not newly added skips. No source/build-selected tests are omitted. Real-agent build-tag tests remain excluded by the task's no-real-agent contract.

**RB06 — current-checkout identity qualification.** The frozen candidate is stable, but the current working-on-issues SKILL.md has a later prose edit. It is embedded by `service/builtin_skills.go:16`, not external documentation. Go dependency inspection places service in production/test closures for cmd/multica, integrations/channel/engine, dingtalk, lark, slack, telegram, wecom and scheduler. Current binaries therefore must not be claimed identical to tested frozen inputs. The wording preserves the human-principal and revision-conflict semantics; focused builtin-content/packaging verification plus new final build/source binding is sufficient follow-up, not an automatic81-package behavioral replay. The other changed recorded file is e2e/fixtures/inbox-snooze.ts, outside these Go tests. Parent owns final candidate integration; this review leaves those files intact.

## Verification

- Raw reconciliation:162 original and10 corrected package receipts; all raw stdout/stderr hashes and counts match. Every test event has the expected package; one terminal per started named test, no unknown actions, malformed events, duplicate/missing terminals or incomplete package endpoints. All guard stderr files are empty.
- Inventory:81 selected +5 delegated equals86 historical packages, with no omissions/additions. All162 historical raw hashes also match retained receipts. All81 normal/race named sets and result maps match after narrowly identifying two PID-labelled and one temporary-path-labelled subtest; exact raw names are retained in JSON.
- Source inventory:Go list's actual darwin/arm64 selection yields5,083 top-level tests/fuzz targets; every normal/race package matches. Nine no-test packages are tracked separately. Source assertions were unchanged.
- Frozen source/tool:10,344 files retain exact bytes/modes and no symlinks; final before/after manifests match and the final hash matches the runner receipt. Compiler reports go1.26.11 darwin/arm64; tool, allocator and both runner hashes match admission. Corrected source manifest equals the original.
- Resources:independent read-only audit verifies exact PostgreSQL owner/image/port/volume and corrected Redis owner/image/port. All172 distinct package databases exist; all have zero clients at the final observation. No database is shared across package executions. Databases/logs remain retained; cleanup is parent-owned after all lanes.
- Lint: not independently run; reviewer changed report artifacts only. Parent owns lint/vet gates.
- TypeCheck: no separate project typecheck run; Go package compilation is evidenced by these executions. Parent owns broader checks.
- Tests: selected normal/race each11,155 PASS/0 FAIL/6 SKIP. Original36 failures and7 skips per mode remain recorded; bounded diagnostic artifact78/storage99 PASS is separately retained.

Passing original executions for the other76 packages remain useful because their frozen source/assertions, complete command selections, original40m/p1/parallel2 budgets, guard, raw/historical identities and resource receipts are all verified. This accepts those particular executions under their recorded environment, not every possible environment nor the original harness as a whole.

Machine-readable counts, source identity, raw hashes, package mapping, historical deltas, skip reasons, corrected receipts and independent resource checks are in `remaining-backend-review.json`.
