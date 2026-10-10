# Review evidence — 2026-10-10

## Provenance

The user supplied a Tasks-tab screenshot, requested impeccable analysis, then explicitly requested a Trellis task for the four proposed improvements.

Independent assessments:
- A: `/root/iteration_design_review` — screenshot/source design review.
- B: `/root/iteration_design_evidence` — detector/source evidence, withheld from synthesis until A finished.

Full report: `.impeccable/critique/2026-10-10T11-37-46Z__packages-views-iterations-iteration-page-tsx.md`.
The supplied screenshot is preserved as `research/task-tab-before.png`.

## Confirmed observations

| Finding | Source anchor | Preservation requirement |
| --- | --- | --- |
| Unbounded search consumes spare width | `iteration-issue-list.tsx:140` | Keep search, filters and grouping discoverable |
| Many layers precede first task | `iteration-page.tsx:125-129`; `iteration-issue-list.tsx:138-161` | Preserve whole-period versus matching-count semantics |
| Applied conditions disappear on collapse | `iteration-issue-list.tsx:147-158` | Preserve unknown/historical values |
| Full reset only at zero matches | `iteration-issue-list.tsx:172` | Preserve existing reset/grouping contract |
| Blocked and todo share empty circle | `iteration-issue-list.tsx:165-167` | Text already exists; preserve historical categories |
| Missing metadata leaves separators | `iteration-issue-list.tsx:167` | Unknown identity is not unassigned |
| Original-scope option is unconditional | `iteration-issue-list.tsx:108` | Started zero-baseline periods are valid |

The 2048 x 1088 screenshot's first task begins around y=790. These are image coordinates, not calibrated CSS pixels. Anchors describe the reviewed working tree and may move.

The iteration-operations spec requires actual lifecycle start facts, snapshot-owned values, independent whole-period statistics, complete selected-source filter metadata, retained tab state and cursor resets. Reuse those contracts.

The adjacent overview has a lightweight filter control but also lacks closed-state active-condition feedback. Its visual pattern may inform this task; repairing that screen is out of scope.

## Assessment and limits

Context loading: the canonical iteration-operations spec measured 33,006 bytes at task creation, above the 32,768-byte per-file injection limit. Both manifests therefore inject the core index and this note. Implementers and reviewers must read the complete current canonical spec directly before work; this summary does not replace it.

- Mode: Operate. Static heuristic score: 25/40. Four P2 issues, no confirmed P0/P1 blocker.
- Detector: `impeccable detect --json packages/views/iterations`; exit 0, JSON `[]`. TSX pattern matching does not verify rendered usability.
- Native UI tool failed with `CUA_REPL_ENABLED_SURFACES is required`.
- Runtime status: API/Web ownership mismatch; Desktop stopped.
- Keyboard interaction, narrow rendering, computed contrast and tests were not executed.
- Branch: `codex/projects-p1`, with unrelated existing edits to preserve.
- No application code changed during the review.

Keep the incumbent identity and three tabs. Improve toolbar density, phase context, visible conditions and task-row hierarchy without adding product actions or changing backend behavior.
