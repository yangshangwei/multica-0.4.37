# S2 client increment 3a — independent check

Snapshot: 2026-11-14T03:40:42.582632+00:00; wire SHA-256 `73a3498c34bf205e0cd78d7226b0ca32e35ed7b438b54213a750141115032c82`.

**CHANGES REQUIRED. Client 3a is not ready for eventual joint integration yet.** This is a client incremental review, not S2, A01–A12 or parent acceptance. No product file, server file, wire contract, dependency, Git state or runtime environment was changed. The coordinator asked that semantic fixes return to the client owner.

## Findings (fixed)

None. All findings require behavioral decisions/changes and were reported to the coordinator.

## Findings (not fixed)

### C01 · P1 · Issue denial deletes the original recovery identity (D4)

**Source:** `packages/core/collaboration/stage-intent.ts:140`, `packages/core/collaboration/draft.ts:68`, `packages/core/collaboration/access.ts:66`, `packages/views/collaboration/collaboration-detail.tsx:71`.

**Contract:** wire-contract.md §3.5; design.md F06.

**Reproduction:** Send a stage write and lose its response; revoke issue access and receive the structured denial. clearCollaborationAccess calls clearCollaborationDrafts, which removes the entire stage intent. Reload the denied detail: no original key remains and the early return renders no stage recovery control.

**Impact:** An operation may have committed, but its original account can no longer inspect the permitted content-free O1 receipt. Retaining the full body would instead retain revoked private content.

**Recommendation:** Split the persisted stage record into a sendable intent and a content-free recovery-only identity. On issue denial erase body/prose and retain only actor/server/issue/action/key plus matching IDs/versions/digests/expiry. Recovery-only records must never reach POST/resend. Render a small GET-only recovery surface from the denied CollaborationDetail branch, without an issue DTO. Keep account/logout teardown destructive for both forms. Reuse stage-intent, stage-actions recovery, and the existing account-only guard; do not create another operation endpoint.

**Coverage:** stage-intent.test.ts currently asserts full erasure, so the green test encodes D4. Replace it and add denied-detail recovery and account-switch cases.

### C02 · P1 · Source denial leaves other candidates’ private cache readable

**Source:** `packages/core/collaboration/stage-queries.ts:127`, `packages/views/collaboration/stage-code-panel.tsx:25`, `packages/views/collaboration/stage-code-panel.tsx:53`.

**Contract:** wire-contract.md §8.6 source-denied variant; PRD A10.

**Reproduction:** Read candidates A and B, leaving B cached. Lose source membership, keep issue participation, and obtain A’s exact baseline:null denial. Select B while its refetch is held. B still has available cached R4 data, so its PR/build/decision reasons render until the new response arrives. An earlier B request has no source-denial epoch fence either.

**Impact:** Private source details can reappear after this issue has already established that source evidence is withheld. The current selected form cleanup test does not cover sibling caches or late responses.

**Recommendation:** Introduce a source-evidence erasure/fence for this issue’s R4 entries on the exact denied projection. Cancel without rollback, remove private baselines/reasons/reports/facts across cached candidates, reject older in-flight responses, and require a current authorized read before restoring evidence. Preserve allowed issue identity and original-key recovery.

**Coverage:** Add a two-candidate Query/Views regression and a denied-A/late-B response race.

### C03 · P2 · Reason-only return incorrectly requires source evidence

**Source:** `packages/core/collaboration/stage-code-presentation.ts:27`, `packages/core/collaboration/stage-actions.ts:76`, `packages/views/collaboration/stage-code-panel.tsx:53`, `packages/views/collaboration/stage-code-forms.tsx:40`.

**Contract:** wire-contract.md §8.3; wire-contract.md §8.6; wire-contract.md §13.9.

**Reproduction:** Use an active designated reviewer/QA with current issue participation but no source membership. R1 supplies current roles/stage and R4 supplies the exact current candidate identity with baseline:null. A return with reason, null expiry, all-null evidence and [] dependencies is allowed by the approve-only evidence contract, yet presentation returns an empty action set, preparation rejects missing baseline, and the form exits before a return can be composed.

**Impact:** A legitimate human cannot return work solely because they cannot read the source evidence, even though return grants no approval and can carry no evidence.

**Recommendation:** Derive reason-only return authority from current role/stage/candidate identity independently of source-read/approval eligibility. Offer a content-free return form; require source authority only for optional evidence/dependencies actually supplied. Keep approve and source rendering blocked.

**Coverage:** Add source-denied current-role return positive case and wrong-role/optional-hidden-evidence negative cases.

### C04 · P2 · Fresh discovery cannot invalidate stale R4 controls

**Source:** `packages/core/collaboration/stage-code-presentation.ts:29`, `packages/core/collaboration/stage-actions.ts:107`, `packages/views/collaboration/stage-code-forms.tsx:121`.

**Contract:** wire-contract.md §5.1 code_candidates; PRD S2-R06.

**Reproduction:** Hold cached R4 A as current, then refresh issue/R1 together so discovery marks A superseded or stale and B current. Candidate identity/lifecycle/role revision in cached R4 A is unchanged. deriveCollaborationStageCodeState ignores the matching discovery entry and still offers approval/advance; preparation can capture the new workflow revision with A’s obsolete current state.

**Impact:** The UI knowingly offers an old approval/transition despite a newer authoritative candidate discovery. Server rejection remains the final protection; this is not evidence of a server authorization bypass.

**Recommendation:** Require coherent R1 CandidateRef/state and R4 identity/state for authorizing controls, and refetch the selected record after discovery changes. Preserve independent revocation of superseded, unconsumed decisions.

**Coverage:** Add a R1-new/R4-old canonical presentation case and an open-form refresh regression.

### C05 · P2 · Policy read access is treated as policy write authority (D8)

**Source:** `packages/views/collaboration/stage-panel.tsx:306`, `packages/views/collaboration/stage-panel.tsx:326`, `packages/core/collaboration/stage-actions.ts:158`.

**Contract:** wire-contract.md §4.1–4.2; PRD confirmed role rules.

**Reproduction:** A source-workspace member who is not owner/admin opens Project stage policy. P1 legitimately returns enabled:true (or disabled plus activation_available:true). The panel computes disable/enable only from those fields, renders an active form, and prepares PUT without current source administration authority.

**Impact:** Ordinary members see an administrative action that is guaranteed to be refused; activation_available describes rollout availability, not that viewer’s permission. The backend still enforces authority.

**Recommendation:** Use the existing explicit source-workspace membership/permission query (e.g. useCurrentMember(memberListOptions) with the captured workspace and current successful authority) for policy-write affordances. Keep read-only policy facts visible to allowed readers. Do not use managed-only configure_roles to hide first enablement. The existing `useCurrentMember` wrapper exposes loading but not query errors; the wrapper alone is not reliable after a failed refresh. If its underlying source-member query cannot provide current authority in this account scope, request a server-owned P1 `can_manage`/`revise_policy` hint; do not expand the frozen wire in the client.

**Coverage:** Add owner/admin/member/unknown/denied UI cases without inventing a new policy wire capability.

### C06 · P2 · Failed R1/P1 refreshes leave stale forms writable

**Source:** `packages/views/collaboration/stage-panel.tsx:79`, `packages/views/collaboration/stage-panel.tsx:94`, `packages/views/collaboration/stage-panel.tsx:306`, `packages/views/collaboration/stage-panel.tsx:307`, `packages/views/collaboration/stage-forms.tsx:278`.

**Contract:** client-increment-3a-results.md delivered behavior 7; core/frontend/collaboration-workbench.md current authority boundary.

**Reproduction:** Load valid R1/P1 and open a form. Make a background R1 or P1 refresh throw a transient network/500 error while the other queries remain cached and successful. TanStack Query retains data; both view/policy and locked ignore query.error, so stage/policy submissions remain enabled. The same issue exists for A4 after R3 failure. R4’s query.isError guard covers only R4.

**Impact:** The panel displays an authority-read error while still allowing writes from the failed read’s old state. Form input should survive, but submission should wait for a successful refresh.

**Recommendation:** Keep retained inputs and show retry, but propagate required-read error state to each form’s read/prepare lock. Apply consistently to R1, P1 and R3; preserve recovery GET even when content reads fail.

**Coverage:** Add a transient-refetch failure with retained data for stage, policy and proposal decision controls, then successful explicit refresh.

### C07 · P2 · Contradictory revoked approval is still selected as valid

**Source:** `packages/core/collaboration/stage-schema.ts:179`, `packages/core/collaboration/stage-code-presentation.ts:45`.

**Contract:** wire-contract.md §8.4 / §8.6; PRD S2-R08 strict new responses.

**Reproduction:** Start with stageCodeReviewDecisionFixture, retain state:"valid", and add a well-formed non-null revocation. The R4 schema accepts both, and validDecisions does not inspect revocation; a business owner/reviewer can still get the QA advance or use the decision as a preceding approval.

**Impact:** A malformed new response exposes an approval as authorizing even though its same projection explicitly records revocation. This is a response-drift/client boundary issue, not a claim that the server emits that combination.

**Recommendation:** Reject the contradictory state/revocation combination in the pure schema and exclude revoked records defensively from validDecisions. Add the small canonical malformed-response regression.

**Coverage:** Existing schema tests cover CI shape and unknown states, but not decision terminal-fact contradictions.

## Verification

- All **24/24 client source hashes** and **24/24 log hashes** match the 3a report. The JSON artifact records every hash plus additional reviewed source bindings. Reported HEAD is provenance from the implementation report; no Git commands were run.
- Recorded final evidence is **Core 273 passed / Views 113 passed**, no failures/skips. Recorded Core/Views typecheck and scoped lint pass; these logs were read and hash-verified.
- Independent runtime checks are temporarily deferred at the coordinator’s request during the registered S5 r02 measurement window. Findings above are source-based reproductions; no new test-pass claim is made.
- Existing positive coverage confirms exact A6–A9 variant matching, microsecond expiry confirmation matching, original-key GET-only recovery while a full intent survives, failed-report QA return with readable source, and current selected-form source erasure. It does not cover the cases above.

## Increment boundary and follow-up

Client3b remains responsible for M1 collaboration completion, generic submit/lifecycle/R38 stage-blocker consumers, activity/notifications, and T4 detail after server2b freezes it. Those are recorded gaps, not new 3a demands. Production Web/Desktop and all joint S2 acceptance remain unverified.

The coordinator has confirmed C01’s intended boundary: erase private body on issue denial, keep only a recovery identity and reachable GET-only controls, and erase everything on account switch/logout. After fixes, sync the collaboration specs with this recovery distinction and the source-evidence cache fence.

A separately reported observation in the actively edited server2a R4 builder used `rollback` as StageBlocker.stage, outside the frozen stage enum. It was sent directly to the coordinator/server owner and is not treated as a stable client finding or a server review verdict.
