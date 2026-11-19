# PAY-241 S2 client increment 3a results

Status: **CLIENT 3a UNIT COMPLETE** (2026-11-14T03:25:59.165168+00:00). This is the A6–A9/R4 shared-client increment, not full S2 or A01–A12 acceptance. Stop at this boundary for coordinator review.

- HEAD: `0ba7405099ba4f8689878e5d957e9b0b1dc28eca` plus this lane's uncommitted overlay and other owners' work. No git writes.
- Final wire-contract SHA-256: `73a3498c34bf205e0cd78d7226b0ca32e35ed7b438b54213a750141115032c82`. Implemented server-owned section 13 items 8–10 and their updates to 5.1/8.3/8.6. The client did not edit the wire contract.
- Previous A5 MD/JSON and all red/green evidence remain unchanged. Before hashes for this increment: `/Users/artisan/.multica/acceptance/pay241-20261113-01/logs/s2-client-inc3a-before.json`.
- No real API, database, browser, Electron, model/provider, payment or deployment was run. No dependency added.

## Delivered behavior

1. **A6 code candidates:** create/revise uses the exact confirmed child, server-observed PR provider/ID/full head and optional existing artifact approval ID. The implementer comes from configured roles, never request input. The newest stale candidate can be revised; superseded versions cannot be revised as newest. The saved server candidate identity opens its R4 view.
2. **R1 discovery and R4 evidence:** candidate discovery is the frozen bounded identity/state list. An omitted discovery field means unavailable. R4 checks issue/candidate identity, strict evidence shapes, exact versions, bounded arrays, complete signer hints and meaningful CI observations. Future states disable actions. The exact source-denied variant has null baseline, unavailable/null observations, empty report/decision/execution arrays and anonymous blockers; malformed partial projections are unavailable.
3. **A7 human decisions:** current configured roles, stage, exact candidate and R4 signability govern approval. UI and preparation preserve code-review self-review denial and QA/release separation. QA picks a report and preceding approval; release binds the candidate's artifact digest and explicit environment; rollback selects a recorded deployment and its recorded previous artifact. Approval uses a finite UTC expiry within the server horizon. Matching does not conflate timestamps differing below one millisecond.
4. **Return is separate:** current role/stage may return failed or unsettled evidence with a reason and null expiry. Optional selected evidence/dependencies must still be readable and belong to the candidate. Return neither requires passing evidence nor manufactures an approval.
5. **A8 and A9:** revocation carries its own exact decision route identity, reason and original key. A superseded candidate's unconsumed decision remains revocable by the current original signer or authorized configuration administrator. Advancing uses exact candidate/stage revisions and explicit unlocking decision IDs. Verification requires the matching successful deployment fact; approval alone is insufficient. Rework requires a returned decision.
6. **Original-intent recovery:** all four new actions share the existing frozen JSON body/Idempotency-Key persistence and O1 GET-only recovery. Missing, malformed or incomplete receipts stay unresolved. Confirmation checks action, issue and the variant-specific candidate, parent, version, decision, expiry or stage revision. A8's decision ID is never smuggled into another route.
7. **Shared interface:** the existing shared stage panel/modal now contains code candidate selection, code/CI/build evidence, approval/return/revoke/advance forms, recent human decisions, runtime-reported test summaries and integration execution facts. Actual signing accounts remain separate from implementation ownership. Current permission loss erases an open form's source snapshot; a transient read failure disables the retained form rather than submitting stale evidence.
8. **Stage blockers:** strict content-free blocker parsing, refusal preservation and a reusable shared blocker view. Unknown blocker codes remain blocking. `stage_evidence_unavailable` rejects object IDs. `stage_incomplete` requires its bounded nonempty list and truncation flag before a stored refusal can settle.

## Scope boundaries retained

- **T4 is not invented.** Server-owned section 13.10 defers the detailed report/case/log/pagination DTO to server2b. This increment renders R4 report IDs, original run/attempt, case/failure counts, reported outcome and applicability, explicitly as runtime-reported. It does not fetch or synthesize detailed cases/logs.
- **Completion remains client3b.** Generic collaboration submit/lifecycle/R38 paths use the existing operation/delivery parsers, not stage-actions. Their stage-blocker adaptation, M1 collaboration completion preview/selection/authorization consumption, and activity/notification projections are pending 3b. The reusable schemas and view in this increment are the seams for that work.
- **Evidence selection is bounded by the frozen projection.** Artifact input references an existing approval UUID for server verification; it is not proof by itself. The rollback form offers the chosen deployment's recorded previous artifact; it does not invent a target or execution result. Discovery shows at most 50 newest candidates and reports truncation.
- No platform route changes or creation-page edits. Existing Web/Desktop detail mounting is reused. No second workflow, outbox, server-state store or execution entry point was added.

## Changed files

24 code/test/locale files changed relative to the A5 boundary. The JSON records both before and final hashes; null before-hash means newly created in 3a.

| File | Final SHA-256 |
| --- | --- |
| `packages/core/api/client.ts` | `e46abcb3fddae8aecffbca1160c2e796831505b8e855a67df9163ac80eba1ef2` |
| `packages/core/collaboration/index.ts` | `fed5f2e83af713acfaba751e58273e571ae506172d4b0024f0cf24bea3b84728` |
| `packages/core/collaboration/stage-actions.ts` | `51b3ce06114f8369e175693008aa0b7deea4a55ab8a253f74f3e2fa39f941f6e` |
| `packages/core/collaboration/stage-code-actions.test.ts` | `4e32f613e4bd47317d865de02edb02e03dfc24c60cfa2a3604286bdf06d1e97f` |
| `packages/core/collaboration/stage-code-api.test.ts` | `ce5b1c0b9f0c583db82bcf089cd53c8622f542c2f10ed88c54424eefb60cb3b9` |
| `packages/core/collaboration/stage-code-presentation.test.ts` | `9e826b4a8d4f413ad96893189b6d9ef8927815044259a6e35deb0edc1d31fe36` |
| `packages/core/collaboration/stage-code-presentation.ts` | `3bda07de8521308040e98dd1b8ba6674b3a71f3b4680a4055d10b88e67c79b9e` |
| `packages/core/collaboration/stage-code-schema.test.ts` | `32413efacf677bbcff613c74726c5755a6db5ebc3a0fa9c2a169d2ea5408a4c7` |
| `packages/core/collaboration/stage-intent.ts` | `e65d214a5b207f89b6f9d1ee64513b43650ebdbf54b5a9cef70a397cc0b1b6e7` |
| `packages/core/collaboration/stage-presentation.ts` | `753421496d1f4b6a13946aca7c2618de1ff439796e72d616975349dd0bae3e8b` |
| `packages/core/collaboration/stage-queries.test.ts` | `817db325e3c3deaaf86ac6b4ba876cf2dd31bf736af8c10c180d21c48b27af7e` |
| `packages/core/collaboration/stage-queries.ts` | `811fdd681d5e9774793d95cd835ae32e6c97e9701f223ae3097d31bf29f982a1` |
| `packages/core/collaboration/stage-schema.test.ts` | `d7fd073b6854352faaad98805338382033ed4f0b0b4ae61c65e34551fdf954f9` |
| `packages/core/collaboration/stage-schema.ts` | `84eb285caa3f7dee2af3757c63ddaf5bef37cb493c9e2554e055a70c179ce70c` |
| `packages/core/collaboration/stage-test-fixtures.ts` | `d3740570a0cb38f95c4d7d9920b29090acee636ab1e9f7ad5a84f5261021f243` |
| `packages/views/collaboration/stage-blockers.tsx` | `cd4a22d41ec0fc5ebbf42cc9964cb5bddd2c49b025ae1c3f4de2bbfec4dee5a8` |
| `packages/views/collaboration/stage-code-forms.tsx` | `b6c5763cc23523cc79f005992bb835a45d35edc7a9c80baf1cd27ab76bda5d5f` |
| `packages/views/collaboration/stage-code-panel.test.tsx` | `d84255260efb8d2e10fae4df7c946774001da253e61b5427488df5c7c2f1901e` |
| `packages/views/collaboration/stage-code-panel.tsx` | `4bebc2eac21225d8c207bbe11aadbd29393711b3272a30d1d92aa49de1a2c5b4` |
| `packages/views/collaboration/stage-forms.tsx` | `de1a2a07558684e729d788189399283e425e86182173a300a26ac36718354721` |
| `packages/views/collaboration/stage-labels.ts` | `616bedeb7dea0f02121bbfd270d2c8b3b3ed1bbf81771d4db579d15f505b9dcb` |
| `packages/views/collaboration/stage-panel.tsx` | `a9cfc878f4a3b88deb04ddfcaab83bbfdc225d54a90fe4a0f6229defd93bf11c` |
| `packages/views/locales/en/collaboration.json` | `24bf2a74390f7d97c8d95fca765f39637dfb21861d123d0ab3bd06a36120650d` |
| `packages/views/locales/zh-Hans/collaboration.json` | `5ddc720f8c6875e8f462f6284de3d4c2ddf3d05b880dbf4ffa399f780fa7e4f9` |

Additional result artifacts: `research/client-increment-3a-results.md` and `.json`.

## Verification and preserved failures

All logs: `/Users/artisan/.multica/acceptance/pay241-20261113-01/logs/s2-client-inc3a-*.log`. Exact commands and log hashes are in the JSON; every command has a directly appended exit code.

| Log suffix | Exit | Result |
| --- | --- | --- |
| `schema-red` | 1 | 19 failed (19) |
| `schema-green1` | 0 | 19 passed (19) |
| `presentation-red` | 1 | no tests |
| `presentation-green1` | 0 | 3 passed (3) |
| `core-types1` | 1 | Typecheck/lint |
| `actions-red` | 1 | 2 failed | 1 passed (3) |
| `freeze2-red` | 1 | 3 failed | 18 passed (21) |
| `core-code-green1` | 0 | 34 passed (34) |
| `views-red` | 1 | 4 failed (4) |
| `types2` | 0 | Typecheck/lint |
| `views-green1` | 1 | 1 failed | 22 passed (23) |
| `authority-red` | 1 | 1 failed | 6 passed (7) |
| `views-green2` | 1 | 1 failed | 26 passed (27) |
| `core-all1` | 0 | 270 passed (270) |
| `revise-red` | 1 | 1 failed | 3 passed (4) |
| `core-lint1` | 0 | Typecheck/lint |
| `views-lint1` | 0 | Typecheck/lint |
| `types3` | 0 | Typecheck/lint |
| `views-green3` | 0 | 113 passed (113) |
| `core-final` | 0 | 273 passed (273) |
| `views-final` | 0 | 113 passed (113) |
| `types-final` | 0 | Typecheck/lint |
| `core-lint-final` | 0 | Typecheck/lint |
| `views-lint-final` | 0 | Typecheck/lint |

Final results:

- Core: **10 suites, 273 passed, 0 failed/skipped**; includes all pre-existing stage suites and four new code-stage suites.
- Views: **5 suites, 113 passed, 0 failed/skipped**; code-stage panel, original stage panel, shared detail and locale checks.
- Core/Views typecheck passed. Stage files, Core API client and barrel eslint passed with zero code warnings. The existing package-manager configuration warning is unrelated.
- Scoped tracked-file `git diff --check` passed. No broader test suite, browser or build was run after these targeted checks passed.

Red evidence is retained as actual red evidence:

- First schema red: 19 failures from absent A6–A9/R4/blocker implementation. Presentation red: missing new module prevented suite import. Actions red: two missing new preparation paths, one refusal control passed.
- After server's second freeze, three tests failed for missing identity discovery/source-denied support and approve-only evidence requirements, then passed after alignment.
- Authority/revise regressions caught over-restrictive handling of an unconsumed decision on a superseded candidate and of the newest stale candidate. Those paths were repaired without granting stale approval or execution authority.
- First Views red: four missing feature paths. Early green attempts caught a duplicated checkbox accessible name and an over-specific report-text assertion. The label was corrected, and the assertion now matches the rendered outcome plus applicability. A5's checkbox label was also made a single issue identity/title to avoid duplicated accessible names.
- Early typecheck found the old schema test table exhaustively assumed only increment-1 actions. It now explicitly tests that increment; the new suite owns A6–A9.

## Canonical test coverage

- `stage-code-schema.test.ts`: R4/discovery/source denial, malformed/future shapes, exact A6–A9 intents/confirmations, microsecond expiry distinction, body limits, blocker privacy and GET refusal shape.
- `stage-code-presentation.test.ts`: designated signer, self-review, expiry, unavailable evidence, failed-report return, unlocking authority, matching deployment facts, revocation of superseded unconsumed decisions.
- `stage-code-actions.test.ts`: configured A6 observations, stale-candidate revision, exact approval evidence and expiry, original-key GET404/recovery without another POST.
- `stage-code-api.test.ts` and extended `stage-queries.test.ts`: routes, identity, malformed/unsupported responses, cancellation and current-access cache erasure.
- `stage-code-panel.test.tsx`: discovery, unavailable R4, candidate creation, code approval, QA failure return, A8/A9 wiring, exact release/rollback evidence and source denial during an open form. Other matrices stay in Core.

No claim here means real CI/payment/deployment integration, remote execution success, completed parent cases or a passed independent Trellis check. Coordinator review and client3b remain next.
