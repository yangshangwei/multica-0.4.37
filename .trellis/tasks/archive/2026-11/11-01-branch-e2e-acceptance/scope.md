# Acceptance scope

Target: `codex/cross-workspace-collaboration` at `822c15785449b6c1cef699f6fd4d8e1f5483bd5e`. Initial working tree was clean; no pending product patch was omitted. Source is frozen in detached worktrees. Origin/main was verified with `git ls-remote` at `dd87e0b33c1d6a64d488f7ee9ecd0a65984a68ff`, also the merge-base. `baseline/` retains complete changed-file, commit, ancestry and worktree records.

The 6,820 changed paths include substantial retained task evidence; scope is derived from actual server/core/views/web/desktop/migration and fixture changes rather than document commit counts. Business areas cover all 14 requested matrix domains. `coverage-inventory.json` contains 437 unique named existing E2E cases, using full suite/title identity rather than ambiguous line numbers. Additional targeted cases and unavailable platform/provider gates are reported separately.

P0: identity/workspace boundaries; issue lifecycle; human cross-account creation, handling, transfer and information-return; Agent/squad admission and FIFO; stop/revocation/restart; startup handoff and memory versions; bounded author-attributed comment recall; human knowledge approval/publication/withdrawal; realtime and idempotency.

P1: projects/iterations/triage; chat/attachments/Skills/MCP/automation; administrator and deployment switches; Web/native Desktop/account and server switching; API/upgrade compatibility; input/error/focus/paging recovery.

Requirements, risk and test-file mapping: see the independently reviewed `research/review.md` in the Trellis task. Each named existing E2E case is retained in the machine-readable inventory and aggregate results. Go/TS boundary matrices are reported at their actual layer; seeded rows and simulated runtimes are never counted as model behavior.

Known gates: runtime audit REC-1–REC-4, ENV-1/2/4, RECALL-1/2 and knowledge audit H1–H3/M1–M5/L1–L13 remain open on the frozen source. `baseline/fix-ancestry.json` proves pending repairs are not ancestors. Current passes do not close a known invariant absent a regression.

No real Provider budget was granted for this acceptance. Accounts/models/call caps from other ongoing tasks are not reused; actual paid Provider calls remain zero. Windows native/installer, Mobile, signed release and mixed installed-version deployment acceptance require their own evidence.

The integration coordinator subsequently advanced the shared branch; those changes are not imported into the detached sources and are not included in this verdict. This acceptance result is not the final merged candidate's release approval.
