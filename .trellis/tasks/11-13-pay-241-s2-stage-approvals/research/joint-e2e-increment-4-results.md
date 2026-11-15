# S2 joint E2E authoring — increment 4

Owner: `/root/s2_e2e_resume`, 2026-11-14.

**The required shared M1 form follow-up is authored and frozen for independent review. Actual scenario bodies executed remain0.** Scenario3 no longer creates or consumes its positive M1 authorization with direct API helper calls. It uses one original authorization across the real shared Web and Desktop forms. This is not runtime A11/A12 or whole-S2 acceptance.

## Implemented authoring change

1. After the existing physical-build/report/release/deployment/verification path and Web delivery submission, the creator opens `Accept and complete` in Web. The shared `Completion authorization` region selects the actual source child and artifact, sends the real root completion preview, displays the exact artifact/commit, chooses the latest allowed expiry and explicitly signs with `Authorize exact completion`.
2. The first Web final-completion attempt is deliberately aborted **before** API forwarding. The fixture captures its original body, raw-body hash and operation key. A real original-workflow GET404 must leave the request pending, with exactly one authorization POST and one browser completion attempt. The root's persisted effects must remain unchanged.
3. A fresh Desktop profile authenticates the same actual creator account without copying Web storage. Its shared form selects and previews the same source/artifact, and its server-derived binding must equal Web's. It then uses the existing-reference UI to read the actual issued authorization ID returned to Web.
4. Desktop explicitly clicks `Accept and complete`. Its final body, body hash and idempotency key must equal the interrupted Web intent. Desktop must receive the original successful workflow result and a separate exact consumed-M1 GET, including the matching submission and collaboration revision. It creates no second authorization.
5. Web checks its original request again. The original workflow GET and consumed-M1 GET must agree with Desktop's receipts before the pending recovery control disappears. Web must not send another authorization or completion POST. The authored journey has two explicit browser completion attempts but only one API-reaching final write.

The frozen submission form exposes no separate acceptance-note input. Both platforms keep its default empty `acceptance_note`; no new product field or fake input is introduced. Authorization reason remains an explicit human input.

API helpers remain only for the existing negative controls, including a valid M1 authorization whose self-completion is refused for missing stage evidence. The separate R38 root and its refusal remain intact; positive R38/self-form runtime acceptance is not claimed by this cross-client submission journey. The release/rollback/late-result scenarios, actual tar/Git/CLI source bindings, eight generation slots and original480-second scenario budgets are unchanged. New waits retain the existing20-second client deadline.

## Files and preserved evidence

The five-file freeze is at:

`/Users/artisan/.multica/acceptance/pay241-20261114-resume/s2-e2e4/source`

Three files changed from the independently checked E2E3 source:

- `e2e/collaboration-stage-execution.spec.ts`: shared Web/desktop authorization, interrupted original request, existing-reference completion and dual-receipt recovery.
- `e2e/fixtures/collaboration-stage-execution.ts`: exact shared preview/response capture and bounded pre-forward fault helper.
- `e2e/fixtures/collaboration-stage-control.node-test.mjs`: two new fault-driver controls.

The unchanged `collaboration-stage-control.ts` and `collaboration-stage-artifact.node-test.mjs` are also bound in the freeze. **E3-C01's strict full-byte USTAR numeric validation and corrected-checksum regressions remain preserved.** The original five E2E2 files remain byte-identical. All E2E3 reports, original freezes and failed logs are untouched.

No Core/Views/server/platform/SQL/migration/dependency/Git file was changed. No public fixture route, simulation option, new completion writer, hand-filled artifact, cross-platform storage copy or automatic resend was added.

## Verification

| Check | Result |
| --- | --- |
| First new fault-driver tests |9 existing tests passed;2 new tests failed before the helper existed. Source and raw failure retained. |
| Final pure fixtures |13 passed,0 failed/skipped: the existing11 tests including USTAR hardening, plus2 new pre-forward/key controls. |
| Isolated TypeScript |Passed on the declared checked3d/E2E graph. |
| Scoped ESLint |Five actual files reported,0 errors/0 warnings. |
| Playwright discovery only |Four scenarios in two files; zero bodies executed. |
| Source/dependency audit |2,552 bound source files,638 loaded type files, zero live-workspace/foreign-snapshot source escapes and zero hash differences. |

The fake Page/Route tests prove that the fault driver calls no forwarding API, retains the exact original body/key, aborts once and removes its interceptor. A mismatched completion operation key is refused without repairing or forwarding it. They do not execute a browser, M1 API or final writer. Existing socket tests use local fake Unix sockets; artifact tests use an independent Python USTAR writer. No actual product API, database, browser, daemon, Git fixture, external provider or model account ran in this increment.

The static graph starts from client3d's2,532-file freeze, overlays its three independently checked corrections, takes only declared E2E dependencies from the approved E2E3 reconstruction, and adds this owned increment. It does not import the live mixed Core/Views/knowledge graph. The client3d corrections concern absent-route classification, pagination and reference-flow regressions; UI selectors, empty acceptance-note behavior and existing-grant consumption are unchanged.

Every raw log has a direct exit sidecar and recorded command. Counts overlap across reruns and are not additional acceptance cases. The JSON companion binds the before/current/frozen files, checked client/E2E baselines, configs, source graph, failed test-first source and final audit.

## Remaining acceptance

Independent final authoring review is next. The coordinator must then bind the completed server2d/client source, fresh API/CLI/Git/base-Host binaries, production Web and matching Desktop, and run all four scenarios plus the full A01–A12 plan. Preserve original failures and denominators, stop owned processes and restore gate booleans with monotonic epochs. Source-issuer/consumer API authority and every actual writer still require their separate backend evidence. Real S4 cases and parent acceptance remain open.
