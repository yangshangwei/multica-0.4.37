# Administration UI audit remediation

Status: implemented and locally verified on 2026-10-04. All F01–F11 findings
are closed for the audited scope. See
[verification and remaining limits](2026-10-04-admin-audit-remediation-verification.md).
Changes remain in the working tree; no release or deployment was performed.

## Scope and decisions

Implement all eleven findings in `.omx/reports/admin-ui-audit-2026-10-04/audit.md`.
Preserve the sidebar work, authorization/content redaction, unknown values,
operation idempotency and server validation. No dependencies or database changes.

- Use UTC consistently as the default display zone, label displayed timestamps,
  and retain an explicitly selected zone during list/detail navigation.
- Preserve same-directory list filters/cursors through a validated `return_to`
  query parameter. Never accept external or unrelated return destinations.
- Give known invalid dates, UUIDs, zones and range constraints inline field
  feedback. UTC date-only account ranges include the selected final calendar day
  and cap today's final boundary at now so the server accepts it.
- Keep touch targets at least 44px for narrow/coarse-pointer administration UI.
  Narrow lists prioritize names, statuses and actions; retain secondary details.
- Separate first receipt loading/failure from refreshing known receipts. Reject
  trimmed-empty reasons visibly without submitting any operation.
- Put actionable current observations before historical statistics and reduce
  audit rows to a useful summary plus expandable technical details.

## Ownership

1. Root: shared filter/date/return helpers, shared filter controls/pagination,
   UTC formatting, scoped touch styling, locale integration, final verification.
2. Execution lane: execution/issue lists and details, operation forms/receipts.
3. Account lane: account list/detail, workspaces and audit pages.
4. Observation lane: installation pages, alerts, health and overview.

Page lanes own their tests. Locale additions are proposed per lane and merged
by root to avoid simultaneous writes. Existing unrelated work is preserved.

## Verification

Write regression coverage at the canonical layer before behavioral edits:
pure boundary matrices in core; form wiring, ARIA feedback and loading/focus
states in views. Run affected admin suites, locale parity, typecheck and lint.
Use the retained audit fixtures only in the browser for populated/empty/error
states; do not send real account or execution mutations. Recheck all 17 pages
at desktop and mobile widths, including dark/English, and replay the six audit
probes with corrected expectations. Inspect in a bounded batch, repair findings
once, then confirm and produce before/after evidence.

## Final review follow-up

- Clear draft feedback on URL changes and native form reset, including a reset
  that leaves the current URL unchanged. Four failing regression cases were
  reproduced before the correction.
- Match Go's UTF-8 query-length limit and canonicalize accepted time-zone aliases
  before submission. Four failing boundary cases were reproduced before the fix.
- Apply the existing touch-link sizing to expanded audit receipts and execution
  lineage links; exercise populated examples in the browser.
- Preserve the full page matrix and add targeted confirmation for these cases.
