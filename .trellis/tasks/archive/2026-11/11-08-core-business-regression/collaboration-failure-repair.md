# Four collaboration baseline failures: diagnosis and regression

Current baseline-1 retained four collaboration failures. Narrow fixture repairs followed the existing E2E environment contract; no product source, assertion threshold or timeout was changed. The complete four affected spec files then passed **7/7 tests, zero skips/retries/flaky results**, in **56.835 seconds**, exit 0.

## Observed causes and changes

| Original failure | Evidence and cause | Repair |
| --- | --- | --- |
| Command history focused capture: viewport ratio `0.9992349147796631`, expected 1 | Minimal `scrollIntoViewIfNeeded` alignment leaves a fractional clipped edge. This is the documented native-scroll-ancestor capture problem already handled in preparation-records Web tests. | Use `scrollIntoView({block:"center", inline:"nearest", behavior:"instant"})`; retain full-ratio, no-overflow, bounding-box and PNG-dimension assertions. |
| Mention refusal capture bottom `1000.25`, viewport height 1000 | Same fractional edge positioning after a viewport resize; target was visible but not wholly inside the screenshot. | Center through native scroll ancestors before settled-layout measurement; retain exact `bottom <= height` and all other geometry assertions. |
| Trigger preview cannot find an AI target button | Baseline DOM snapshot contains the collapsed `Mention` toggle, with no exposed participant/target group. The test attempted a hidden target before opening the current UI disclosure. | Call existing `revealCollaborationMentions(composer)` before selecting the three real targets. All preview, suppression, no-write and exact receipt assertions remain. |
| Stale lifecycle waits for missing `More actions` | Baseline second-page screenshot shows `Leave issue` directly in the primary action bar and no More actions button. The helper sampled `isVisible()` during post-submission rerender and permanently chose the wrong branch. | Wait for either the actual primary Leave action or the More actions disclosure to become visible, then select its real location. No forced click, reload, artificial request or sleep was added. |

Files changed by this repair:

- `e2e/collaboration-command-history-web.spec.ts`
- `e2e/collaboration-mention-journeys-web.spec.ts`
- `e2e/collaboration-preview-web.spec.ts`
- `e2e/fixtures/collaboration-control-gaps.ts`

The parent's earlier `collaboration-workbench-web.spec.ts` response-waiter fix is preserved unchanged: the response budget begins only immediately before releasing the deliberately held request. The new readiness wait repairs the separate helper branch failure seen afterward.

## Execution evidence

```sh
bash /Users/artisan/.multica/qa/core-20261108/browser.sh collaboration-repair-1 \
  e2e/collaboration-command-history-web.spec.ts \
  e2e/collaboration-mention-journeys-web.spec.ts \
  e2e/collaboration-preview-web.spec.ts \
  e2e/collaboration-workbench-web.spec.ts
```

The root authorized this focused run after baseline became terminal. Only the four changed test/fixture files were synced into the QA checkout, with before/after SHA-256 in `collaboration-repair/sync-1.json`. Production Web/renderer builds and API configuration remained as in baseline. The runner uses one worker and zero retries.

Private evidence root: `/Users/artisan/.multica/qa/core-20261108/`.

- Original failures: `baseline-1.log`, `baseline-1/artifacts/` screenshots, traces and error contexts.
- Complete passing replay: `collaboration-repair-1.log`, `collaboration-repair-1/results.json`, generated screenshots and geometry attachments.
- Exact named inventory and patch: `collaboration-repair/case-inventory.json`, `collaboration-repair/fix.patch`.

The original complete baseline remains failed; this seven-case replay does not retroactively change its result or establish full-suite acceptance. Broad post-fix verification remains owned by the root. No additional UI abstraction or product workaround was introduced.
