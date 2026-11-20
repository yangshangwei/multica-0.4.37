# S2 joint E2E authoring — increment 1

Owner: `/root/s2_e2e`. Status: authoring complete and static checks passed; no browser or integration acceptance has run. Date: 2026-11-14.

## Change boundary

The existing multi-account closure covers generic collaboration but does not exercise stage roles, immutable proposals, code review and source-denied stage evidence through shared Web/Desktop controls. This increment adds one focused stage journey and its fixture using the existing password actors, TestApiClient and isolated Electron adapter. New files are limited to `e2e/collaboration-stage-approvals.spec.ts` and `e2e/fixtures/collaboration-stage-approvals.ts`; product, server, shared fixture and configuration files remain owned by their current agents.

The journey will record actual human UI operations and original-key recovery, synthetic PR/CI source facts, and source/privacy refusals. Policy enablement must use the production P2 endpoint; there is no SQL policy or approval bypass. It targets the A01/A02/A03/A04/A08/A10 subset, with Web/Desktop actions as partial A11 evidence only after a future real run. T4/QA, release/rollback, final completion writers and full A01–A12 acceptance are later increments.

This agent may author and run targeted static checks only. The coordinator owns the fresh database, source-bound production builds, API/daemon/browser launches, execution evidence and final gate restoration. Missing or failing configured dependencies must fail; the default unconfigured suite may opt out. No real models, money, remote GitHub/CI, deployment or production release are used.

### Authorized synthetic account setup clarification

Static inspection found that password registration creates an account without email, while the existing public workspace-invitation API requires email. There is no public email-setting operation in `UpdateMe`. The coordinator authorized a narrow fixture-only bridge: after public registration, assign deterministic `@example.invalid` addresses only to the exact newly registered test actors, matching both account ID and requested password username, and only while their current email is SQL NULL. The transaction refuses any mismatch or existing value and verifies the task database first. No password, account role or membership row is seeded or changed by this bridge. Source membership still uses actual public invitation/acceptance, DELETE and invitation/acceptance again. The coordinator must provide an API environment with outbound email disabled. This setup does not certify a public email/profile workflow.

## Checks and evidence

The new opt-in scenario is `stage roles, proposals and code review retain exact human decisions across Web and Desktop` in `e2e/collaboration-stage-approvals.spec.ts`. Its seven accounts are publicly registered synthetic accounts: product/business owner, implementer/current handler, code reviewer, QA approver, release approver, external participant and unrelated reader.

Authored behavior, pending actual execution:

- P2 is enabled through the shared UI and recovered through P3. Enabling must be available; the test fails if the server is still intentionally gated.
- Separate roles are configured through the shared UI; same-person implementation/review and QA/release API attempts are rejected. Stable business owner and current handler remain distinct.
- A proposal is filed, explicitly modified into version 2, then separately accepted. Original-key GET and exact-body replay retain one mapping. Another proposal is rejected without creating work. A human ordinary child remains creatable while policy is enabled.
- A5 selects the actual source-readable children and explicit code/no-code choices. A code candidate is created through the UI. Missing/failed synthetic CI blocks advancement; current passing synthetic CI permits code review. QA without a reviewer decision is rejected.
- External R1 readers receive `content_digest:null` for the source child under wire §13.14. R4 uses its exact protected variant; an unrelated account gets 404 and the shared UI offers no source approval.
- The isolated Electron reviewer performs actual shared UI approval. A real forwarded POST response is retained with its key and raw request/response hashes before the browser reply is aborted. Visible original-result recovery must issue GET only. Changed-body reuse of the key is rejected.
- Revocation, return/rework, changed PR head, and removed/rejoined source membership preserve old decisions without restoring their authority. A fresh explicit signature on the new candidate permits the Web action to enter QA.
- The fixture asserts zero collaboration commands and runs, records every captured stage request/response, takes Web and native screenshots after a future successful run, closes owned contexts/SQL/native processes, and retains database/history. It does not launch a daemon or provider.

| Check | Final result | Evidence |
| --- | --- | --- |
| Targeted TypeScript, project strict options | Exit 0, no diagnostics | `typecheck-4.log` |
| Targeted ESLint, existing Core rules with root E2E dependency context | Exit 0, zero warnings; 91 effective rules per file | `lint-4.log`, `effective-eslint-{1,2}.json` |
| Playwright discovery only | Exit 0; exactly one test in one file | `discovery-3.log` |
| Owned source identity | Both files match before/after final checks | `final-verification.json` |
| New-file whitespace check | Empty diagnostics; Git's no-index difference exit 1 retained | `static-final.json` |

Commands used existing Node 22.23.2, TypeScript 5.9.3 and ESLint 9.39.4. Private configs and logs are under `/Users/artisan/.multica/acceptance/pay241-20261114-resume/s2-e2e-increment1-check/`. The JSON companion binds the exact command/log/source hashes. No dependencies were installed, no existing product or fixture files were changed, and no Git write occurred.

First failures are retained: TypeScript initially inferred a UUID template-literal type for the default key, rejecting recovered string keys; the fixture now declares the key as `string`. ESLint initially rejected runtime imports through the Core package barrel from root E2E code; the fixture now imports the existing pure schema leaves directly. The first whitespace audit incorrectly required no-index exit 0; Git reports the new-file difference as exit 1 with no whitespace diagnostics. These are authoring/checker corrections, not product regressions. Playwright discovery emitted the existing pnpm package-field deprecation and dotenv banner; neither is hidden or interpreted as an executed test.

## Remaining gates

All full E2E cases remain **NOT_RUN**. Discovery is not execution; this report grants no A01–A12 or S2 acceptance.

The coordinator must first complete server 2b–2d and client follow-ups, including the frozen source-denied digest change, then bind fresh database/API/CLI/source/production Web/Desktop identities and run the scenario. The coordinator must verify outbound email is disabled before synthetic invitations. Actual rendered behavior and fixture SQL remain unexecuted; any failure must retain its original run and be repaired under the appropriate owner.

The scenario covers only the authored subset. Typed test reports/QA, release and rollback facts, M1/final completion and administrator completion need subsequent integration work in the same files once their served contracts are stable. Reuse the scenario helpers; do not turn this subset into a whole-phase pass or replace real S4 integration evidence with its synthetic source rows.
