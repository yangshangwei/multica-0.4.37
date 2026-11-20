# PAY-241 S2 client A5 release results

Status: **CLIENT A5 UNIT COMPLETE** (2026-11-14T02:43:13.471152+00:00). This replaces the interrupted IN PROGRESS skeleton. It is not an A01–A12 or full S2 acceptance report.

- Current HEAD: `482cd7f683739aca72050b6b09fdfe724742a898` plus this lane's uncommitted overlay and other owners' work. The external creation-page commit does not include this lane's files.
- Frozen contract: `research/wire-contract.md`, sha256 `ced0cd66c729093c96259f42adee8a788868dde12851679a990266bd8dba1b4c`, section 13 items 1–5. The contract was unchanged during these checks.
- Ownership: shared Core/Views stage files and en/zh-Hans stage text only. No git writes, new dependencies, real API, database, browser, Desktop or model execution.
- Initial manifest: `/Users/artisan/.multica/acceptance/pay241-20261113-01/logs/s2-client-a5-resume-before.json`. Machine-readable before/after hashes, command text and log hashes are in `client-a5-release-results.json`.

## Behavior and boundary

1. R1 now requires per-child `code`, `code_state`, `pull_requests` and `depth`. Code references contain provider, PR UUID and a full lowercase 40- or 64-hex head. The schema distinguishes null code from an omitted field, rejects malformed references and depth/attachment contradictions, and preserves future code-state strings as unknown with disabled actions. Descendants at depths 1–64 are eligible when attached; detached mapped issues stay visible but cannot be selected.
2. A5 requires an explicit code choice on every selected entry: null or one exact linked PR/head from R1. No code is inferred from free text, a PR title, a short hash or an unlinked reference. Existing current approvals preselect the child and its still-current code (or previously explicit none); a changed/unavailable code requires a new choice.
3. The shared form pins the scope it displayed when opened. It keeps selected full heads through polling and blocks submission when the selected content/head or policy/role/scope/lifecycle revision changes. Reopening is required to review the fresh baseline. Newly selected children cannot submit until a code choice is made. Empty scope remains an explicit valid confirmation.
4. Removed the temporary A5 hold after the request and R1 shapes matched the frozen contract. Existing managed/current/write-context/capability gates still apply. Code state is rendered separately from scope approval and machine execution admission; a changed head does not imply a reopened scope or an execution refusal.
5. Added the declared `stage_scope_code_unavailable`/409 receipt and localized explanation. Scope-change and child-unavailable explanations reflect head changes and governed descendants.
6. `matchesCollaborationStageConfirmation` was inspected and retained: A5's actual minimal receipt remains action + issue + decision + exact lifecycle + next scope revision. The contract adds no child/code echo. Original-key transport correlation, strict receipt parsing and GET-only recovery are unchanged. An A5-specific regression preserves the original full head and key through transport failure, reload, GET404 and exact successful recovery, with one POST total.

## Files changed in this resume

Fourteen code/test/locale files changed. Existing `api/client.ts`, `collaboration/index.ts`, `draft.ts`, shared detail mount and non-stage files were not changed in this resume.

| File | Final sha256 |
| --- | --- |
| `packages/core/collaboration/stage-schema.test.ts` | `0e1e46d2064b3effbe476c7e0e7e3f689a82b9d7ecdd1b2cff91186f2a1abbf8` |
| `packages/core/collaboration/stage-actions.ts` | `8c7830e646193938ee0db2f6987d4e10a74d07b2d85c100b6efce77a8b608e07` |
| `packages/core/collaboration/stage-presentation.ts` | `c8155318a4f0bf3ad1fb1580e8bb25a5d0be32f3bb1f365c7388cb2d9384ff98` |
| `packages/core/collaboration/stage-test-fixtures.ts` | `808ffcd4aabb489ea12059d2c983a7b2ee2f84f25ea42ef79d84ebad3ce5fb47` |
| `packages/core/collaboration/stage-schema.ts` | `79c499e65f9eb7a2dee5907845dc9a70a854de7db745f0c4ec0221bd3a28586f` |
| `packages/core/collaboration/stage-actions.test.ts` | `e306622d17ec20add9a385e33c3f2885a00ef4bf4e0286ccd48d9a107fb391a1` |
| `packages/core/collaboration/stage-intent.test.ts` | `bc5d3290cd52fbfd68d1a8b384f90161c0d0850c1e768288d7dffadeb556717e` |
| `packages/core/collaboration/stage-presentation.test.ts` | `f80ce30463ea4800e8e7823e4e24028e4b520f76fb51ec55e52119cc523aba63` |
| `packages/views/collaboration/stage-panel.tsx` | `7a4b7b5ab4f1c47ba91000389e01015818be098423b6ef05d42f4df6112a8ea6` |
| `packages/views/collaboration/stage-labels.ts` | `a38161de3e04fe8f370310a41496e0a0a2ac95c41e09744b3ac950cf3eb0a4f3` |
| `packages/views/collaboration/stage-forms.tsx` | `b02e6a9bcefcbbe8c160c7c7a5555afbf07aab08ac6763b4287455cd5fafc610` |
| `packages/views/collaboration/stage-panel.test.tsx` | `6b47267b9343246673e798dc5911b07e52e69b9b69556b63d7fe425c8bd18461` |
| `packages/views/locales/en/collaboration.json` | `32d4a8b9897ee9e23cc0267d305238725be0e0be5ce5431dc7a58253f15a0f7f` |
| `packages/views/locales/zh-Hans/collaboration.json` | `5cbfc6c8c3f5bd398204f653578fb70dd48d0a7dda467fff4d100609608695b8` |

The report files themselves are `research/client-a5-release-results.md` and `.json`. The prior increment-1 report remains historical and still records its original A5 hold.

## Red and green evidence

Logs are under `/Users/artisan/.multica/acceptance/pay241-20261113-01/logs`. Every command has an appended direct exit code; earlier failures were retained.

| Log | Exit | Result |
| --- | --- | --- |
| `s2-client-a5-resume-core-red.log` | 1 | 163 passed / 23 failed / 0 skipped |
| `s2-client-a5-resume-views-red.log` | 1 | 0 passed / 2 failed / 17 skipped |
| `s2-client-a5-resume-core-green1.log` | 1 | 230 passed / 1 failed / 0 skipped |
| `s2-client-a5-resume-views-green1.log` | 1 | 100 passed / 2 failed / 0 skipped |
| `s2-client-a5-resume-core-green2.log` | 0 | 232 passed / 0 failed / 0 skipped |
| `s2-client-a5-resume-views-green2.log` | 0 | 102 passed / 0 failed / 0 skipped |
| `s2-client-a5-resume-typecheck1.log` | 0 | Completed |
| `s2-client-a5-resume-core-lint1.log` | 0 | Completed |
| `s2-client-a5-resume-views-lint1.log` | 0 | Completed |

- First Core red: 23 failures demonstrated the superseded child parser, paused action, old request shape and missing refusal code. First Views red: both A5 tests could not find the held confirmation action. The 17 skipped tests were intentionally excluded by the red test-name filter; final runs have no skips.
- First green attempt: Core's old 200-child size fixture still omitted `code`; it was updated to explicit null. The two Views failures exposed Base UI label concatenation; the checkbox now has an explicit issue identifier/title accessible name. These are preserved as `green1`, not relabeled as passing runs.
- Final Core: six stage suites, **232 passed**, zero failures/skips. Final Views: stage panel, shared detail and both locale suites, **102 passed**, zero failures/skips.
- Core and Views `tsc --noEmit` completed with exit 0. Both stage eslint runs completed with exit 0 and zero code warnings. The package-manager warning about the existing root `pnpm` configuration remains unrelated.
- Scoped `git diff --check` passed. No unrelated test suites were broadened after these relevant checks passed.

## Tests changed

- `stage-schema.test.ts`: required R1 code/depth fields; governed descendants; malformed/null/unknown states; explicit null, full SHA-1/SHA-256, strict A5 references and refusal status. Existing confirmation tests retain the unchanged minimal receipt contract.
- `stage-presentation.test.ts`: released capability, required explicit code choices, no fabricated/substituted heads, descendant selection and unknown-state refusal. Replaced the obsolete hold assertions.
- `stage-actions.test.ts`: exact A5 body plus original-code/key preservation and GET-only recovery after reload. Existing access/account/epoch/refusal matrices still pass.
- `stage-intent.test.ts`: adapted the existing maximum-child fixture to mandatory explicit null.
- `stage-panel.test.tsx`: replaces the held-button case with the full null + descendant/PR happy path and a live-query refresh that preserves the selected head and sends nothing.

## Remaining work and limits

- No real-server JSON, browser layout, Electron interaction or simulated-human sign-off was verified in this slice. The coordinator owns joint production Web/Desktop and A01–A12 verification.
- A6–A9/R4/T4, M1 collaboration completion, stage blockers and activity/notifications belong to the next dispatched client batch; none was implemented here.
- No server behavior was inferred from these stubs. Scope confirmation is not code review, CI success, deployment, payment or a completed parent acceptance case.
- No blocker remains for this bounded A5 client increment. Await coordinator's third-batch dispatch.
