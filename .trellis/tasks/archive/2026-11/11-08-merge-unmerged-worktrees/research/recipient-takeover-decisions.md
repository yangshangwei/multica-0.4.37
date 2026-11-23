# Research: Recipient/UI source adjudication

- Query: Which staged recipient integration changes remain absent from the current branch, and which should be preserved, adapted, or rejected?
- Scope: Internal; source `/Volumes/artisan/code/2026/mc-recipient-integration-20261030-072838-a3fe`, source HEAD `2612465a016aa606cc7a2538e7890f1f4b76d995`; target `e541cfb436fed9c6733174479718b651e2cf030d`.
- Date: 2026-11-08

## Findings

This source is not a stale-copy-only batch. The archived source-HEAD→working-tree patch contains **43 code/test/locale paths** (39 excluding four locale JSON files). Every original hunk in 39 of those 43 paths still has exactly one matching preimage in current target bytes. The remaining four files are the evolved source-workflow integration and require selective adaptation. This is direct evidence of absent increments, not an inference from differing file hashes.

The accompanying JSON maps every path, its observed source/target hash, original hunk, target preimage line, decision and verification needs. No product code was changed.

| Logical increment | Paths | Decision |
| --- | ---: | --- |
| Input boundary/fill token separation | 23 | Merge as one coordinated batch, with actual contrast/visual checks |
| Project heading token corrections | 4 | Merge minimal substitutions |
| Restricted rich-content resource placeholders | 4 | Merge, retain restricted-resource safety assertions |
| Collaboration-aware batch action affordance | 2 | Merge, preserve transport guards |
| Embedded workflow success feedback | 4 | Selectively adapt success feedback; closing logic already superseded |
| Reaction accessible-name removal | 6 | Do not merge; preserve archived source, keep current action names |

### Input and fill tokens

Source `packages/ui/styles/tokens.css` splits `--input-fill` from `--input`, preserving old fills while increasing input boundary contrast; all 23 dependent stylesheet/component/test deltas remain unapplied. Current declarations are at `packages/ui/styles/tokens.css:31`, `:205` and `:296`; current consumer examples are `packages/ui/components/ui/input.tsx:12`, `packages/ui/components/ui/button.tsx:15`, and `packages/views/agents/components/inspector/model-picker.tsx:108`.

This is coordinated UI work, not safe to merge as one token-only edit. The source changes dark outline/tab decorative borders and usage-table separators to `input-fill` to preserve their old appearance, and read-only/disabled backgrounds must use the preserved fill tone. The old source adds comments claiming WCAG ratios but does not add a new border/focus contrast test. Current `apps/web/app/text-contrast.test.ts:37` explicitly excludes input/ring from its matrix; its passing status cannot certify these claims. Existing `/50` ring utilities remain in the source, so measure actual rendered border/focus combinations rather than claiming the token value proves all states. Run brand cascade tests and representative light/dark UI visual review before accepting.

### Project heading role tokens

Current project detail, overview, progress and risk-issue components still use `text-heading`, which is absent from the authoritative role scale. `packages/ui/styles/tokens.css:120` defines `text-title-sm` and `:123` defines `text-title`. Source uses those roles for the corresponding headings without changing behavior. All five source hunks match the current preimage.

### Restricted-resource placeholders

Current `RichLink` at `packages/views/rich-content/rich-content.tsx:219`, `RestrictedAwareImage` at `:420`, and restricted file-card path at `:434` return only existing label/alt/filename. Empty metadata therefore hides the fact that a resource was withheld. The source adds a localized inline `RestrictedResource` for links/images/files, plus two locale entries and parity assertions. It does not add links, images, lookup requests or resource URLs. Preserve the current test's negative assertions for `img`, `iframe`, private anchors and resolver calls (`rich-content-parity.test.tsx:198`) and add/use positive placeholders; include empty alt/name coverage when applying.

### Collaboration batch selection

Current `batch-action-toolbar.tsx:100` refuses via toast only after handler invocation; triggers at `:243`, `:254`, `:267` and delete at `:278` are disabled only for loading. Source adds `actionsDisabled = loading || containsCollaboration`, a persistent localized reason, and retains handler guards. The matching test at `batch-action-toolbar.confirm.test.tsx:112` currently expects a toast. Apply the narrow source change while retaining the current ordinary-status/assignment/iteration logic. Verify clear selection remains enabled and keyboard users can find the reason; test trigger disabled state and no network/confirmation path even if a mock invokes a callback directly.

### Embedded workflow feedback is mixed, not a whole-file merge

Current `workflow.tsx:40` defines dismiss through `onDismiss`; successful receipts at `:48` invoke dismiss. Current `source-bridge.tsx:72` passes `embedded`, `onDismiss={close}` and `onBusyChange`, already implementing the source's close-after-success intent with stronger single-modal/busy/focus behavior. Do not replace these files with the old `onDone` version.

The source's success toast after the outer dialog closes is still absent: current source bridge does not import toast, and the workflow result UI disappears when the source dialog unmounts. If retaining this source improvement, add a success-only callback to the current workflow and wire toast there. Never put the toast in `onDismiss`, which also serves cancellation. Keep the current denial, lost-result recovery, form focus and busy/Escape guards. Test success exactly once, and no success toast on cancel/failure/unknown result.

### Reaction labels should remain current

The source removes localized `Add {{emoji}} reaction` / `Remove your {{emoji}} reaction`, removes the count description association, and updates tests to emoji/count regexes. Current `discussion-actions.tsx:43` retains these action labels alongside `aria-pressed`; source deletion is not needed to fix a failing semantic contract. Keep the current localized action naming and existing tests. Archive the six-file alternate design with this explicit rejection instead of calling it already equivalent.

### Independent documentation

Six additional staged files are absent from target: `PRODUCT.md`, `docs/multica-introduction.html`, and four archived task records under `.trellis/tasks/archive/2026-10/10-25-multica-introduction/`. They are separately enumerated in JSON. Preserve them in the source archive and evaluate user-facing docs in the documentation batch; do not silently drop them or import old task metadata as product code.

## Files Found and Proposed Application

- `recipient-takeover-decisions.json`: complete 43-path evidence/decision map plus six documentation entries.
- `recipient-safe-source-candidates.patch`: **unapplied candidate patch**, containing only the 33 uniquely matching paths for token/fill, project typography, restricted placeholders and upfront batch disabling. It is a convenience for the implement agent, not proof that tests pass or permission to skip review. Four workflow paths require an adapted change, and six reaction paths are intentionally excluded.
- Source preserved patch: `/Volumes/artisan/code/2026/mc-merge-unmerged-archives-20261108/patches/recipient-integration-20261030-072838-a3fe.tracked.patch`.

## Related Specs

- `CLAUDE.md`: semantic tokens, role-named typography, shared Views/Core/UI boundaries, canonical test ownership.
- `.trellis/spec/views/frontend/collaboration-detail.md`: embedded workflow `onDismiss`/`onBusyChange`, single modal, request lock, retained feedback and recovery.
- `.trellis/spec/views/frontend/component-guidelines.md`: accessibility and shared component behavior.
- `.trellis/spec/web/frontend/e2e-run-environment.md`: production Web rather than development server for browser validation.

## External References

None used. Findings are grounded in repository files and the preserved patch; no external API or dependency decision is involved.

## Caveats / Not Found

- This research ran no product tests and performed no Git operations. It used the parent's fixed target identity and current file bytes, plus the previously verified source patch.
- The target may evolve after this snapshot. Recheck hunk preimages and hashes before application.
- Matching an original hunk proves the source increment is absent; it does not make that increment automatically desirable or verified.
- Rejected reaction changes remain available in the archive; the source worktree was not modified.
