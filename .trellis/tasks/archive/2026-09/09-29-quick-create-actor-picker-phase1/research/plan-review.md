# Independent phase-1 plan review

Date: 2026-09-29. Review only; no product implementation or global task-state changes.

## Verdict: APPROVE

The PRD, design and execution plan form an actionable, bounded phase-1 implementation contract. No unresolved plan issues found. The two low-severity documentation findings and hydration-readiness clarification below were resolved and independently rechecked. Approval applies to the plan, not to unimplemented behavior or release readiness.

## Scope and evidence

Read `CLAUDE.md`, the task's `prd.md`, `design.md`, `implement.md`, both research notes, both JSONL manifests and task metadata. Reused the research evidence rather than repeating its repository-wide investigation. Checked the existing `PropertyPicker`/`PickerItem`, quick-create store, workspace-aware storage and both parent submission branches to resolve concrete interface and lifecycle questions.

Both JSONL manifests parse successfully, contain nine unique entries and reference existing files. Task artifacts are intentionally absent from these spec/research manifests: `.claude/hooks/inject-subagent-context.py` injects PRD/design/implementation artifacts separately, and the `trellis-implement`/`trellis-check` fallback instructions explicitly read them. Existing test paths named in the plan were checked for existence; new test paths are marked as new.

## Contract review

| Requirement | Assessment | Evidence / implementation implication |
| --- | --- | --- |
| R1: Compact access with full discovery | Pass | PRD fixes 3 favorites / 5 recents, excludes every favorite from recents, and provides access to overflow favorites and the complete directory. `design.md:109` makes pagination presentational, not a search-domain limit. |
| R2: Independent local favorites | Pass | Typed actor references stay in the existing core preference store; no server objects or unrelated Chat/sidebar pins. Primary and favorite controls are sibling buttons; favorite actions do not select or submit. |
| R3: Accepted-submit history | Pass | Both ordinary and comment-source requests use the existing shared accepted-success boundary. Submission captures actor identity before awaiting; recording updates last actor and MRU together. Failed, blocked, canceled or merely selected actors do not become recent. |
| R4: Truthful, robust descriptions | Pass | Saved actor description is authoritative; squad description remains distinct from its leader. `design.md:74` normalizes malformed fields before preview/search; inert text rendering avoids interpreting description markup. |
| R5: Search and type filtering | Pass | Full description search, existing name-pinyin semantics, stable ranked results, complete-directory search from every browse mode and explicit reopen/clear behavior are specified. |
| R6: Eligibility, lifecycle and keyboard | Pass | Existing eligibility and seed precedence remain owned by the parent. Loading/error/empty states differ. External result changes invalidate stale index/sole-result fallback, while typed search retains first-match Enter behavior; same-render precedence and pre-effect Enter are explicitly tested. |
| R7: Scale and responsive access | Pass | All-mode pagination preserves tail access. Real browser input-to-render measurements with 550 actors, declared environment and P95 target are required; pure helper timings cannot substitute. |

The steps distinguish canonical pure/store tests from component wiring and real-primitive keyboard tests. Scope explicitly excludes recommendations, taxonomy, new backend contracts, new dependencies, mobile and a general picker rewrite. Preserving unrelated working-tree changes is an explicit execution requirement.

## Resolved review notes

1. **P2 — API naming: resolved.** `design.md:89` now explicitly names `api.quickCreateIssue` and `api.createCommentSubIssue` at their shared accepted-success boundary, matching the inspected parent implementation.
2. **P2 — Primary-button accessibility surface: resolved.** `design.md:119` now uses native primary-button text containing the name, type and summary, plus screen-reader-readable selected-state text. It explicitly avoids assuming `PickerItem` forwards arbitrary ARIA attributes or introducing option/listbox semantics. No additional primitive prop is required for this choice.
3. **Hydration readiness clarification: resolved.** `design.md:97` now requires scope-matched readiness using workspace UUID, user and session generation; each read also captures a read generation so stale completions cannot publish current readiness. Component rendering and favorite/history write entrances check this condition. `implement.md:38` adds the mirror-before-microtask and stale-hydration regressions. This directly covers the interval where the old scope can still report `persist.hasHydrated() === true`.

## Representative implementation simulations

- **Persisted favorites and scope changes:** Build state from fixed defaults plus valid allowlisted fields; recover malformed JSON at the store-local read boundary before merge; avoid a persisted reset that overwrites the destination key. Resolve favorites only against authorized current queries. The plan has direct tests for empty/corrupt destinations, return navigation, cleanup and old tuples.
- **Search plus an external reorder:** Typing marks a pending real-search highlight; filter/favorite/eligibility changes invalidate stale numeric selection. Resolve their same-render precedence deliberately, suppress sole-result Enter after external invalidation, and preserve highlight on append-only pagination. Real PropertyPicker tests protect existing IME/empty-value behavior and non-opted-in callers.
- **Deferred accepted creation:** Capture the submitted typed actor, workspace UUID, user and nonpersisted session generation before either API branch. At the shared success point, compare current identity/status and preference readiness before atomically recording. Skip only the stale preference write; do not convert an accepted request into a failure. Test both branches, including same-user logout/relogin and changing the current actor while awaiting.

## Remaining implementation risks

- The clarified scope/readiness contract still needs implementation evidence: exercise immediate reads/writes before the rehydration microtask and reject stale read completion, in addition to empty/corrupt destinations and session cleanup. The plan now specifies these cases; this review does not establish they already pass.
- Restoring focus after a favorite moves between React section parents needs a mounted-DOM test. Stable actor keys alone cannot preserve a node across different parents.
- Cached partial data, long descriptions, narrow popovers and desktop focus behavior remain unverified until implementation and browser checks run. The 100 ms P95 target is a measurement requirement, not a result established by this review.
- The live working tree already contains unrelated edits in shared target files; source anchors are snapshots and must be rechecked at implementation start.

No unit, integration, browser or performance tests were run during this planning review. Only document/context checks and targeted source inspection were performed.
