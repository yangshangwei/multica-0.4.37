# Committed source decision ledger — 2026-11-08

Comparison base: `e541cfb436fed9c6733174479718b651e2cf030d`.

## Directly unmerged branch tips

| Branch | Tip commits outside base | Initial decision |
| --- | ---: | --- |
| `agent/agent/task1` | 1 | Drop: empty baseline commit |
| `codex/acceptance-evidence-822c15785-f417` | 2 | Selective: evidence/spec only |
| `codex/collab-recipient-picker-integration-20261030` | 6 | Selective: inspect E2E/evidence; implementation commits are patch-equivalent |
| `codex/collaboration-context-experiments` | 1 | Selective: test-only context experiment |
| `codex/collaboration-context-resume-20261031` | 5 | Selective: planning/evidence only; four commits patch-equivalent |
| `codex/collaboration-final-owned-20261103` | 1 | Skip: patch-equivalent |
| `codex/collaboration-fixture-repair-20261103` | 1 | Skip: patch-equivalent |
| `codex/knowledge-advance-notes-20261102` | 1 | Selective: evidence/spec only |
| `codex/knowledge-codex-20261104` | 3 | Skip: all three patch-equivalent |
| `codex/projects-p1-merge-readiness` | 2 | Selective: journal/evidence only |
| `codex/runtime-acceptance-20261104` | 17 | Selective: one non-equivalent evidence commit; rest patch-equivalent |
| `codex/verify-collaboration-host-runtime-20261026` | 1 | Selective: CI/spec/verification only |

## Non-equivalent commit candidates

These are not automatically approved for product integration; each remains subject to
changed-path review and targeted verification:

- `5696ee916`, `fc920522f` — acceptance evidence and environment/spec material;
- `d6a70ed29`, `4aa77e08e`, `0041aa4b2` — recipient picker follow-up/evidence and merge aggregate;
- `32b741d0a` — context experiment test;
- `835e7b046` — handoff planning artifacts;
- `cc530a558` — knowledge evidence/spec material;
- `6d2be85fb` — journal material;
- `f033051ca` — runtime evidence runner and supporting records;
- `92bb6d86b` — CI/spec verification material.

The recipient picker implementation (`751567567`) and Knowledge/Codex implementation
(`429b1b6b4`) are patch-equivalent to current commits (`9ab2b8558` and `88f1cc8d4`
respectively) and must not be cherry-picked again.

## Review rule

The ledger records Git-level reachability and patch equivalence. A candidate may be
marked `merged` only after its relevant source/test behavior is verified in the clean
integration worktree. Evidence-only material can be marked `archived` without entering
the product commit set.
