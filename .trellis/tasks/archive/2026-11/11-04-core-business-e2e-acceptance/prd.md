# Core business end-to-end acceptance

User approved the plan in this conversation on 2026-11-04 and explicitly requested execution. This authorizes task creation and execution; no additional planning approval is needed.

## Scope
Candidate: 1a32ca36c2c4f4455a01bdd39d0e0ab4db20cccc, with product-tree dirty inventory captured before execution. E01 authentication; E02 workspaces/membership; E03 issues; E04 triage; E05 agent execution; E06 failure/retry/stop/recovery; E07 comments/inbox/realtime; E08 cross-workspace human collaboration; E09 authorized collaboration execution; E10 account/workspace/server isolation; E11 projects; E12 iterations; E13 chat; E14 automations; E15 skill/MCP; E16 context/knowledge; E17 platform administration.

## Acceptance
- Every required journey has an actual result tied to candidate, configuration, platform and exact test identity.
- Preserve first failures, reruns, skips, blockers and not-run cases separately. Historical results are not current evidence.
- Verify persistence, permissions, realtime observers, execution identity and recovery, not just page visibility or process exit.
- Distinguish simulated protocol, actual daemon/controlled provider, real model, native application and installed-upgrade evidence.
- Preserve existing 2000ms iteration closure P95 gate; no assertion weakening or retry-to-green.
- No unclosed critical business/security/data-integrity failures; missing mandatory environment means incomplete acceptance.
- Preserve unrelated working files, processes, databases and prior evidence.

## Boundaries
Test execution, isolated fixture setup, evidence and narrow testing-infrastructure fixes are in scope. Product defects are recorded and narrowly repaired only with confirmed cause and regression evidence. No release, external messages, production mutations, new dependencies, global configuration edits or use of unspecified paid-model quota.

User clarification: this run completes controlled execution; real commercial-model acceptance is explicitly deferred and must be reported separately as NOT_RUN. No model-account or paid-quota access.

## Candidate-specific accepted performance deviation
User decision relayed from thread01a1a7a6 on2026-11-05: “这个2155可以接受”. Accept this1a32candidate2155.025ms closure result as an exception. Keep original2000ms test andFAIL evidence; do not change global thresholds or future-candidate criteria. No further attribution/optimization for this accepted gate.
