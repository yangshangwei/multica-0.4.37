# S2 client increment 3b1 results

Status: **UNIT COMPLETE — INDEPENDENT CHECK PENDING** (2026-11-14T06:17:06.426857+00:00). This is the bounded generic stage-refusal feedback increment; it does not accept S2, A01–A12, or the parent task.

- HEAD: `8e824b3035c4f17a9a9dfa0504f1294fbcc9b20b` plus this client overlay and other owners' work.
- Wire at report time: `a71d71570cf1f337d0f8c299e77e307e6d37516dd516eaa6e0882548c5669520`. Implemented the already-frozen section 10.2. The newly published T4/test_subject and R1/A5 privacy fields belong to the next batch.
- Read `client-increment-3a-recheck.md/json` before editing. Reviewer C08 changes in `stage-panel.tsx`, its test and `stage-recovery.test.tsx` were preserved byte-for-byte against the 3b1 baseline.
- The coordinator explicitly widened ownership to the project DELETE error feedback and corresponding tests/locales. Those project files were clean before this increment. No delete-impact fields were invented.
- No git write, dependency, API/DB/browser/Desktop/provider process or full build. Root owns specs and commits.
- Frozen source copy for read-only review while 3b2 proceeds: `/Users/artisan/.multica/acceptance/pay241-20261113-01/snapshots/client-3b1-20261114T062238Z`. Its `482` files and hashes are in `source-manifest.json`; the report JSON binds that manifest.

## Delivered behavior

1. **Generic operations:** the existing operation schema retains `stage_blockers` and `stage_blockers_truncated` for `stage_incomplete`, requires status409 for the new refusal codes, and rejects absent/empty/malformed blocks, hidden-evidence identifiers, contradictory confirmations or blockers attached to `stage_policy_exit_required`. Unknown blocker codes remain unknown and blocking.
2. **One blocker contract:** generic and R38 parsers reuse the existing StageBlocker schema through shared refusal-field validation. No second blocker type, request store or workflow engine.
3. **Delivery and fields:** the existing delivery refusal registry recognizes both new codes. Submit, self-complete, exact-submission completion and source-field PATCH keep original result/basis/files/selections while retiring only a valid stored refusal. GET recovery keeps the historical blocker snapshot and status rather than substituting current stage state.
4. **Malformed versus missing GET:** `getCollaborationOperation` throws unavailable when a stage-marked payload fails parsing, so it cannot become a null receipt and enter the legacy resend fallback. `action.ts` and generic HTTP404→same-request behavior are unchanged; existing404 regressions pass.
5. **R38:** source inspection confirmed the existing administration error envelope is `{code,error,request_id}` and lifecycle errors roll back. The new parser consumes that envelope plus stage fields, retaining actual status and safe blockers while dropping raw error prose/transport trace. First known refusal can end the attempt. A rejected retry of an old unknown request keeps the original key/input and GET recovery; its stage refusal is only the latest attempt's diagnosis. No stored failure receipt is invented. Success receipts carrying contradictory stage blockers are unavailable.
6. **Shared feedback:** workflow errors render existing blocker labels/renderer, status and a historical-refusal caption. Administration distinguishes known refusal from a rejected retry with unknown original outcome and keeps the reason visible. Known refusal has a refusal heading, with no misleading pending hint.
7. **Project DELETE:** actual409 `stage_policy_exit_required` produces the localized instruction to disable stage gates. It produces no success feedback/navigation. Delete-impact schema and metadata remain unchanged.

## Recovery and authority boundaries

Stage parsing grants no completion, deployment or release authority. Server guards remain authoritative. If server2d changes R38 to a persisted failure variant, its exact DTO must be frozen before client consumption. No M1 preview/candidate route, target union, T4 detail, test_subject field or activity projection was guessed in this batch.

The newly announced R1 nullable child digest was intentionally not mixed into3b1. Root subsequently authorized privacy+3b2 against the new freeze; review this snapshot for3b1 rather than assuming the live workspace remains at these hashes.

## Verification

Logs are `/Users/artisan/.multica/acceptance/pay241-20261113-01/logs/s2-client-3b1-*.log`. Exact commands and log hashes are in the JSON; each has a directly captured exit.

| Log suffix | Exit | Result |
| --- | --- | --- |
| `core-red` | 1 | 25 failed | 32 passed (57) |
| `core-green1` | 0 | 57 passed (57) |
| `admin-red` | 1 | 2 failed | 1 passed | 45 skipped (48) |
| `views-red` | 1 | 3 failed | 59 skipped (62) |
| `core-green2` | 0 | 105 passed (105) |
| `views-green1` | 1 | 1 failed | 2 passed | 59 skipped (62) |
| `types1` | 0 | typecheck / lint |
| `views-green2` | 0 | 4 passed | 59 skipped (63) |
| `core-final` | 1 | 1 failed | 364 passed (365) |
| `views-final` | 1 | 1 failed | 135 passed (136) |
| `types-final` | 0 | typecheck / lint |
| `core-lint` | 0 | typecheck / lint |
| `views-lint` | 0 | typecheck / lint |
| `result-final2` | 0 | 37 passed (37) |
| `core-final2` | 0 | 365 passed (365) |
| `views-final2` | 0 | 136 passed (136) |
| `types-final2` | 0 | typecheck / lint |
| `core-lint-final` | 0 | typecheck / lint |
| `views-lint-final` | 0 | typecheck / lint |

- Initial Core red:25 failures/32 controls passed. R38 red:2 failures/1 control passed with45 filtered. Views red:3 failures. These are retained.
- First Views green exposed an assertion matching the hidden retained form and visible receipt reason; it now checks the visible `dd`.
- The broader run caught a fixture with current submission but null `latest`, violating the original delivery summary; the fixture now uses the same current/latest submission. It also caught a duplicated admin refusal hint; the receipt omits its pending hint when refusal is known, leaving the existing single alert.
- Final Core: **10 suites,365 passed,0 failures/skips**. Final Views: **6 suites,136 passed,0 failures/skips**, including workflow uploads, administration, project detail and locales.
- Core/Views typecheck passes. Lint has0 errors. The project detail's two hooks warnings at unchanged lines153/261 predate this diff; only an import and DELETE onError callback changed there. They were not broadly refactored.
- Scoped `git diff --check` passed. No unrelated tests were broadened after these checks passed.

## Source changes

24 code/test/locale files changed against `s2-client-3b1-before.json`. JSON contains before/final hashes and unchanged C08/generic-recovery hashes.

| File | Final SHA-256 |
| --- | --- |
| `packages/core/admin/collaboration-api.test.ts` | `5f244b5390fea37656af298823f10f4146212d90f58f16469f0f7febfa366cca` |
| `packages/core/admin/collaboration-queries.test.ts` | `6ceb6f8807a3494be44e06cf2203bd8c20277210aca37e8c0fc8074cb96710c1` |
| `packages/core/admin/collaboration-queries.ts` | `c7c555c90ef255e236b59714f64a3b559d0b30fade31f7f514e9ba3cc9073307` |
| `packages/core/admin/collaboration-schema.ts` | `e53d0b3759c5289b1710d8b3ef87cfe60311ae785343db34376edfe1a9e41396` |
| `packages/core/api/client.ts` | `ac1b6e284fa9656df64401b21dceb55a987bb41b58b404f317b81989f549cecd` |
| `packages/core/collaboration/delivery-schema.ts` | `ebdcd5409168fefd2212627cce519a78935243a1a63d61b09b1962a4a781a965` |
| `packages/core/collaboration/result-operation.test.ts` | `2ed734e46bab3c840a76bdc8900d4423bddf2b233cfcf1aa2d1dc8d9a12163b5` |
| `packages/core/collaboration/schema.ts` | `a4af82f7b8ebd82001cab485a4e8e5b29efb42eebd8cde8027550bccd5e0bbc6` |
| `packages/core/collaboration/stage-generic-refusal.test.ts` | `d9f676c7841f53f5be536e34da89266b0c3edfd6883fab30750bd83d4634adc6` |
| `packages/core/collaboration/stage-schema.ts` | `0477374825e303e150c02f7c9f9a0a39225a734b64f9dcab8e0d0c78ef4dcc68` |
| `packages/views/admin/collaboration/collaboration-controls.tsx` | `7b466f6fd1ebae60051f18e1b440a4a47b5b583955c3af479fec8bb1f3ab399a` |
| `packages/views/admin/collaboration/collaboration-page.test.tsx` | `dc2197ebeee1d2f522fdf31290dd24be6e5722a7743eb12649d88a908a50b578` |
| `packages/views/admin/collaboration/collaboration-receipt.tsx` | `4495916d81fba37f1a17a2bb570c441ded97c8901e8218a27a4d6ebe85457ac1` |
| `packages/views/collaboration/stage-refusal-notice.tsx` | `60da053703d00db6bdf024ef2acfe88213dc9842bdc624e26dba801515b379ac` |
| `packages/views/collaboration/workflow.test.tsx` | `f9228b8d05787188f6d084726416fb33d55923a9774191904e36ae710126506b` |
| `packages/views/collaboration/workflow.tsx` | `d75193bad8733a722bccfad7e5fe494031104de053e54268c6efe126ea1a90ea` |
| `packages/views/locales/en/admin.json` | `8a0ed9e8fd8c26bb8d1c229d0ba17fbd23533a3389d5ee76b089f61badcef923` |
| `packages/views/locales/en/collaboration.json` | `62323fc52147fb1671514dcd45c786808cef9ab22db360b17b63f98485427be0` |
| `packages/views/locales/en/projects.json` | `706047bb82df160817e9b967cdd52abe273f6e539bfb70189b4b4289f6dd07cc` |
| `packages/views/locales/zh-Hans/admin.json` | `249c3cd5dc24aa056ec35bb3981fed5f050f3fcbafb3fd3ef894bac790d4ed09` |
| `packages/views/locales/zh-Hans/collaboration.json` | `1884a19c96fb3491053b977e5768c33873888d47ec9d8c351ee49d3d436b9ae2` |
| `packages/views/locales/zh-Hans/projects.json` | `b82a9ee88f942e05b14216a229b2b65bf3b442c90afa6168e2f4357afd82b7ff` |
| `packages/views/projects/components/project-detail.test.tsx` | `7ff54f1f26f4f2743ea68d1b1c97cb09ef560b06ef5c43d58cc08b44b8ef7788` |
| `packages/views/projects/components/project-detail.tsx` | `f7e96f1fdfde135d9580b00b179771dc44fa9b268b183e861256cca3607a6d32` |

These artifacts are `research/client-increment-3b1-results.md/json`. Previous A5,3a,fix and recheck reports remain unchanged.

## Remaining work

Independent3b1 review uses the frozen snapshot. Privacy+3b2 will consume the just-frozen T4/create-only test_subject and nullable R1 digest. M1 completion and activity still need server2d's exact contract; delete-impact blocking metadata remains unfrozen. Real API/DB, matching production Web/Electron and A01–A12 remain joint acceptance work.
