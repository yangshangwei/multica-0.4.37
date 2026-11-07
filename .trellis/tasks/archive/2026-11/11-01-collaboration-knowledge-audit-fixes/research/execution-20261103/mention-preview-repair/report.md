# Mention / preview repair preparation

**Prepared for independent review; not applied and not runtime-verified.** The selected patch is `repair-v2.patch` against frozen candidate `85ef5d20578f63c7c4f879f035acbd414e80f306`. Only the two declared test files would change. The actual candidate bytes remain unchanged.

## Root cause and smallest repair

1. Actual failures 4 (Web), 7 (Chinese preview) and 10 (Desktop) still require Send to be disabled after a suppression change. Current `comment-composer.tsx:82–94,140,149` retires the old preview but keeps Send available: Send fetches a validated new preview and, for every explicit AI target including suppressed/refused targets, returns without writing until a second explicit action. `.trellis/spec/views/frontend/collaboration-detail.md:54–59` and the existing component tests at `comment-composer.test.tsx:122` describe the same contract.

   The patch exercises that contract rather than just flipping an assertion. The shared preview helper can use Send, observes the actual trigger-preview response, and asserts zero comment POST while obtaining impact confirmation. The partial-refusal journey also requires the old preview to disappear and all retained business state to stay equal before confirming. The Chinese flow observes the refreshed suppressed target, requires the explicit AI confirmation copy, checks zero POST/comment/execution change, then requires exactly one POST on confirmation.

2. Actual failure 5 waits for Edit issue without opening the current secondary-action disclosure. The existing `revealCollaborationEditingAction` in `e2e/fixtures/collaboration-editing.ts:135–141` already performs that user interaction and works in the accepted Web/native editing journeys. The patch reuses it in the shared mention helper; no duplicate platform code or new navigation helper is added.

The real owner-revocation barrier, exact original request key, receipt equality, no duplicate comment/queue/notification checks, outsider denial, durable notification anchors, and edit-recipient deltas remain unchanged. No timeout, retry, skip or API authorization expectation is weakened. No product implementation changes are proposed.

## Files and evidence

- Would change `e2e/fixtures/collaboration-mention-journeys.ts` and `e2e/collaboration-preview-web.spec.ts` only.
- `source-binding-v2.json` records original/proposed source hashes plus current implementation/spec/helper hashes.
- `red-evidence/` retains copies and hashes of v13 failure excerpts 4, 5, 7 and 10. These are failures from the original full run, not new reproductions by this agent.
- Proposed file copies live only below `.omx/private/mention-preview-repair-v13/proposed-v2/`.
- The first unapplied proposal and its static checks are retained. Revision 2 uses a scalar POST count and captures the execution snapshot immediately before suppression, avoiding a cross-locale snapshot comparison.

## Checks performed and limits

`syntax-check-v2.json`: TypeScript 5.9.3 parsed both proposed files with zero syntax diagnostics. This does not typecheck imports or execute test modules.

`patch-check-v2.json`: checked whitespace and applied the patch only to ignored scratch copies with Git discovery blocked; resulting hashes equal the proposed files. The first static attempt was an ineffective no-op because Git discovered the enclosing checkout and skipped paths; a hash assertion detected that. `patch-check-first-attempt.json` preserves it. Neither attempt changed the frozen candidate.

No lint/typecheck, Go, Playwright, API, database, service or Provider execution occurred. The patch is not green test evidence.

## Required follow-up

After independent review and preservation of v13's whole-run/final-audit results, root can apply the selected patch after checking the two original hashes. Run all three Web mention journeys, all three native Desktop mention journeys, and the Chinese/English preview journey: **seven cases, zero unexpected skip/retry**. `report.json` lists exact names and prospective command arguments. This preserves shared-helper positive controls as well as actual failing cases. The new complete candidate gate remains required after all repair lanes are integrated.
