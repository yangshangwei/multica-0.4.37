# S1 independent final review

**Verdict: PASS for S1; no unresolved code, spec, runtime or cleanup finding.** B01–B10 have the required evidence and S1 is eligible for completion. This does not complete the parent PAY-241 24-case acceptance or S2–S5. Reviewed source/spec/evidence hashes and explicit case mapping are in `check-review.json`.

## Scope and source identity

Browser acceptance used the isolated `c79dfe304` candidate plus exactly 12 S1-owned product/test files. All 12 final hashes match the current root and isolated source; the only additional tracked candidate change is the documented generated Next route-types import. The final E2E SHA-256 is `e2bbcda9340f930c20b49936b7f2b6d4a1a567d44314c101f11c50b5cb03db53`.

The unrelated concurrent F2 commit moved root to `fafb27b1d`. Its reviewed changes do not overlap the S1 projection, identity, access or byte-budget contracts. Separate final-root service/handler compilation and the four new top-level regressions (7 named tests/subtests) passed on fresh owned databases with unchanged S1 Go hashes. Browser evidence is still bound to the isolated candidate; it is not a claim of a repeated complete browser run at the newer root.

## Findings fixed

- Reviewer corrected vacuous exact `Edit`/`Delete` absence selectors to actual `Edit comment`/`Delete comment` names.
- Client owner extended the existing generic-label test to missing, empty and whitespace-only names in English/Chinese; the final 19-test list suite passed.
- Coordinator clarified that a fitting placeholder permits recovery, while metadata that still exceeds `max_bytes` returns existing `400 budget_too_small`. The four updated specs now accurately preserve the additive summary, human author, compatibility, access and installed-Main boundaries.
- E2E owner resolved the author-badge ambiguity with the permission-context role. The final `xpath=./div[time]` selects the real direct author header; exact name/badge and no-edit assertions remain intact. Two earlier failed B05 runs and their locator diagnoses remain retained.

## Contracts reviewed

The public summary is only trusted Agent ID/current name. Human `author` and all permissions remain unchanged. One existing batch lookup serves list/detail/recall after authorization, propagates real database failures and retains identity after archive/resource withdrawal/deletion. Actual serialized JSON budgets include the summary and preserve bounded placeholders/cursors/detail recovery. Core only discards malformed additive display metadata, and both shared list/reply paths use the same display selector without extra resource requests.

## Actual acceptance evidence

All five required browser cases have explicit passed results, zero skipped cases and zero retries:

- **B05/B06 — `b05-run3`:** one issue, three independent owners/resources/Agents, five observed controlled runs in 1/2/2 distribution, exact comment→run→Agent matches, three Web author/reply views, same-issue Electron author/reply verification, first delivery returned and second exact submission accepted by the initiator. All three owned daemons exited 0 with no active tasks; native app/profile and execution gates cleaned up successfully.
- **B08 — `supplemental-run2`:** Web and Electron each observe the offline information/acceptance and proven completed/failed/stopped AI results. Recipient source membership, source inbox and subscriber rows remain absent; revocation redacts protected records. Embedded source and Linux fixture binary hashes match current files, and transport/page errors are empty.
- **B09 — handoff in `supplemental-run1`, revoke in `supplemental-run2`:** directed human responsibility ends in creator-only completion; former-handler completion is 403. Live revocation requests stop and keeps its target held, preserves queued input, refuses missing consent/proof and enforces explicit Resume. This is the existing simulated-permit contract case, not a commercial Provider termination claim.

The first supplemental run used the wrong task-only gate profile. The unchanged namespace fixtures require `host_execution_enabled=false`; the coordinator corrected its wrapper, retained the failed run and reran the affected cases. The final original gate flags were restored. This is not a product-code fix or a retroactive pass of the failed run.

## Visual verification

The reviewer inspected all four actual screenshots. The **108-character** Agent name and reply text remain visible with textual Agent badges in 390px Web and real Electron 200% zoom. Recorded Range geometry proves 2 author lines and 3 reply lines on both clients, contained text and no local/document horizontal overflow. The parent visual verdict is 95/pass. The earlier 104-character estimate was incorrect; actual recorded length is 108.

## Verification and cleanup

Independent review checks: scoped Core/Views ESLint, strict E2E TypeScript before runtime locator follow-ups, diff whitespace checks and actual final evidence/hash audits passed. The E2E owner reran strict TypeScript/discovery for the final locator revision. No heavy checks were repeated solely for this final audit.

Reviewed execution evidence: Core 146 tests; Views four-file run 50 tests plus the updated list's 19 tests (overlapping counts, not 69 distinct tests); service 53 named tests/subtests and handler 9; focused race 6 service named/subtests and 1 handler; explicit Go vet; full isolated typecheck 9/9 and lint 6/6 (zero errors, existing unrelated warnings). Raw Go inventories and all five browser-report hashes independently match their manifests.

Host binary hashes, Web BUILD_ID, renderer index and preload still match preflight. Final independent IPv4/IPv6 probes confirm task API18970/Web13890/renderer6064 are closed, while human API18572 and IPv6 Web13492 remain running. Docker contains only the original PostgreSQL container. Task databases are retained as evidence; no user database was deleted.

## Remaining limits

Controlled Providers and synthetic human test accounts only; no real payment, commercial model or production release. Desktop proof is the Electron renderer, not an installed full Main. Parent full acceptance and subsequent packages remain open. Commit/task archival are coordinator workflow steps, not unresolved S1 verification gates.

## Documentation-only final delta

The final closure-QA additions correctly preserve the separately tested host/namespace gate profiles, exact prior-flag restoration, header-only Agent selector and recorded Playwright counterexample, and the namespace-under-host-cutover 409 behavior. Its updated SHA-256 is recorded in `check-review.json`. All 12 product/test hashes remain unchanged. No test rerun was needed; the S1 PASS verdict remains unchanged.
