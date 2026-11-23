# Independent supplemental evidence review — 2026-11-02

**PASS WITH NOTES for the exercised diagnostic and narrowly scoped macOS subgates. Required performance/platform/trust/business gates remain open.**

This review used the complete native task context, approved task artifacts and current execution plan. It was read-only outside these two review files and started no service, test or database operation. The reviewer did not modify product/harness code, other agents’ evidence, progress or the conditional matrix.

## Findings (fixed)

No product fix was required or authorized. The coordinator already narrowed the current macOS runtime report’s network statement: renderer request observation begins after firstWindow and is not an OS-level egress trace. That limitation is accurate and should stay.

## Findings (not fixed)

1. **Windows baseline prerequisite is conditional.** research/remaining-platform-handoff-1.md currently says the stable v0.5.1 EXE and checksum must also exist for its all-flags command. The actual workflow downloads the feature-capable 0.5.2-rc.11.1 baseline only when windows_business_acceptance=true (workflow lines220–231); the stable previous_tag download is guarded by business!=true (lines233–249). Update the handoff wording so stable prerequisites apply only to that latter lane. This document is coordinator-owned, so the reviewer reported the correction without editing it.
2. **The first macOS runtime fixture did not reach a normal authentication form.** Both runtime screenshots show the server-authentication-configuration error. The synthetic /api/config has auth_mode=password but omits password_auth_available=true; schemas default the latter false and LoginPage correctly rejects that combination. This is a fixture limitation, not a demonstrated product regression. Main/version, isolated config/preload IPC and localStorage retention assertions remain valid. Do not use this run as evidence of a valid password form, real login, authenticated session migration or business flow. Either document this scope explicitly, or fix only the synthetic response in a new numbered run if normal UI readiness needs proof.

Both notes were sent to the coordinator. They do not justify altering the original successful or failed evidence.

## Performance verification

The preserved task-only diagnostic test adds a new virtual Go file through an overlay; no existing file is replaced. Current candidate HEAD, source diff hash and frozen P95 test hash match the recorded pins, and the virtual target remains absent on disk. Every compile/diagnostic command stdout/stderr hash and the saved test/metrics hashes match.

The request context marker restricts tracing to one actual HTTP closure; setup/start/preview are outside the span. Grouped counts independently reconcile to 3,053 standalone statements, 1,000 active-task reads, 2,000 participation reads, and seven membership batches containing 7,000 statements. BatchQuery events contribute counts only. Actual installed pgx SendBatch uses BatchTracer, and batch end fires at result closure; no second QueryTracer duration is summed for each batched query. Query totals, batch totals, connection acquisition and residual arithmetic agree with the recorded 2,396.832ms handler time.

The diagnostic correctly reports client-observed spans rather than pure PostgreSQL execution, process-wide rather than per-goroutine allocation, no Lock-only observer, first-operation/cache and shared-host limitations. The PostgreSQL amd64 image/aarch64 engine observation is grounded in the separate environment evidence. It does not establish the exact cause of the original two slow tail samples or a product fix. Original P95 2,171ms >2,000ms remains FAIL; the single diagnostic is not added to the original test inventory or used to close that gate. The recorded diagnostic database cleanup shows zero connections; this reviewer did not connect to any database.

## macOS verification

The file-only lane’s24 assertions and16 command exits are successful; all32 recorded command stream hashes match. Exact dirty-baseline limits, candidate ASAR/main/CLI pins and separate signing/trust boundaries are stated correctly.

The later actual runtime lane independently establishes two immutable packaged Main launches at the same task installation path, exact old/new versions, isolated Electron home/userData, real preload configuration/preferences/daemon-state IPC reads, and exact candidate recovery of the old renderer’s synthetic localStorage marker. It does not inject authentication or replace IPC handlers. The configuration bytes remain unchanged. See the authentication fixture limitation above.

The before/final multica:// default path and entire same-bundle registration lists are equal. Only the newly added task app registration was removed. A fresh independent read-only NSWorkspace query still matches the original /Applications/Multica.app default and registration set. A fresh process-table check found no process with the task runtime prefix; lsof found no listener on the synthetic port58786 (expected exit1, empty stdout), and no task app registration remained. No user installation was launched or overwritten.

## Remaining-platform handoff

The explicit candidate version, GitHub-hosted disposable Windows guard, source/ref drift check, accepted-baseline checksum, unsigned-versus-trust distinction, Linux native-controller limitation, iOS wrapper commands, and real Provider authorization boundary match the repository. No remote run was dispatched during review. The Windows conditional prerequisite note above is the only concrete command-contract mismatch found.

## Verification

- Product lint/typecheck: **not repeated**; this review changes only task reports and no product source/dependency.
- Product tests: **not rerun** by reviewer; actual saved diagnostic/runtime results and source were checked independently.
- Evidence: raw hashes, JSON, query/batch arithmetic, original failure, source/frozen-test preservation and fresh native cleanup/protocol checks passed.
- git diff --check: PASS.
- No product simplification/refactor, dependency change, commit or publication.

Machine-readable findings, artifact hashes and exact review observations are in research/next-evidence-review-1.json. These conclusions are scoped to the reviewed hashes; new numbered runtime evidence requires a supplemental review.

## Supplemental review: coordinator resolutions and runtime-2

Both actionable notes above are now addressed without rewriting prior raw evidence:

- The Windows handoff explicitly limits the stable previous_tag installer/checksum prerequisite to business=false. Its all-flags business=true command depends on the feature-capable0.5.2-rc.11.1 artifact, as the workflow actually does.
- A new numbered next-mac-runtime-2 preserves the first run and corrects only the task synthetic password capability. Its source diff adds password_auth_available=true and real username/password/form visibility assertions on both actual applications, without changing immutable app bytes or test timeouts. The candidate screenshot was visually inspected and shows the normal Sign in form. Both actual old/new Main launches, runtime configuration/preload IPC, retained localStorage and config bytes still pass. No authentication request was submitted; real business login/credentials/automatic updater remain outside the exercised subgate.

The saved runtime-2 driver hash matches run.json; all nine recorded command steps exit0. Both stages report the exact old/new versions and isolated home/userData, no daemon autoStart and no automaticUpdates. A fresh independent process-table check finds neither runtime prefix, lsof shows no listener at the new synthetic port60790 (expected exit1), and a fresh NSWorkspace query equals the entire original protocol default/registration set. The task runtime registration is absent.

Final verdict remains PASS WITH NOTES for the scoped evidence, with no unresolved evidence defect in these reviewed supplemental subgates. Original performance FAIL and unexecuted platform/trust/business acceptance gates remain open. The authentication-error observation in the first review remains historically accurate for runtime-1.
