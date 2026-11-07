# Current-branch end-to-end acceptance

## Goal
Prove observable business completion and identify regressions, unauthorized access, inconsistent state and failed recovery on the current branch. Deliver reproducible evidence and an honest acceptance verdict.

## Background
- Target: codex/cross-workspace-collaboration, HEAD 822c15785449b6c1cef699f6fd4d8e1f5483bd5e. Initial worktree was clean.
- Verified origin/main and merge-base: dd87e0b33c1d6a64d488f7ee9ecd0a65984a68ff. Scope includes all business differences from this baseline, not only recent ledger commits.
- Host: macOS arm64; Chromium is the repository default. Root commands exclude Mobile.
- Existing runtime/context/knowledge task ledgers remain authoritative requirements and historical evidence; their passes do not substitute for this run. Unmerged worktree repairs do not count.

## Requirements
R01: Record immutable source, branch, worktree, tool and environment provenance.
R02: Map requirements/changes to risks, named cases, outcomes and evidence.
R03: Own all databases, ports, files, synthetic users, services and deterministic runtime fixtures; use production Web builds.
R04: Exercise identity/workspace, issues, cross-account collaboration, direct/squad execution, stop/revoke/recovery, startup handoff/memory/recall, knowledge, realtime/idempotency (P0).
R05: Exercise projects/iterations/triage, chat/resources/automation, admin/flags, Web/Desktop, compatibility and failure UX (P1).
R06: Run applicable static/unit/integration/E2E suites; preserve first failures, exact case identity, skips and reasons.
R07: Report actual browser/native/package/OS/real-provider coverage separately, with evidence.
R08: File defects with severity, prerequisites, reproduction, expected/actual, sanitized requests and suggestions. Do not repair product code during acceptance.

## Acceptance criteria
AC01: Every identified business area has named coverage or an explicit blocked/skipped rationale.
AC02: Every planned case has PASS, FAIL, BLOCKED or SKIPPED; result counts reconcile to raw runs.
AC03: A full acceptance pass requires every agreed P0/P1 item executed and passed with no open critical defect. Any missing platform, provider prerequisite or critical failure prevents a full-pass claim.
AC04: Dedicated synthetic data includes at least two workspaces, three roles/users, two independent sessions and two runtime identities where supported by existing fixtures. Insufficient setup is recorded as a gap.
AC05: Test artifacts include scope, coverage matrix, execution report, defect list, evidence index and remaining risks.

## Constraints / out of scope
- No business refactor, feature change, merge of pending fixes, production deployment or external messaging.
- No real Provider/account call: this request supplies no fresh account/model/call-count/cost authorization. Prior budgets and paused experiments must not be restarted. Record those acceptance cases BLOCKED.
- Other operating systems and signed installers require available owned runners/artifacts; do not infer their success from Chromium or source tests.
- Preserve failed state and raw logs; never lower assertions, add retries or report only successful reruns.

## Resolved decisions
The user authorized task creation and planning. The initial request authorizes isolated test execution and necessary test-only gaps, with product defects reported for later repair. No product behavior decision is required.

## 2026-11-07 authorized defect repair continuation

The user explicitly requested diagnosis and repair of the remaining2 E2E and4 frontend unit failures. This supersedes R08/no-product-fix only for QA-20261107-001,003,004 and necessary regression coverage. Preserve earlier acceptance evidence and unrelated platform/provider exclusions.

Acceptance: canonical diagnostic-path and Skill presentation suites pass without weakened assertions; production Web and real Electron full detail journeys pass the original exact620 restoration assertion on the repaired candidate; relevant focused regressions plus lint/typecheck pass. No paid model calls or unrelated refactors.
