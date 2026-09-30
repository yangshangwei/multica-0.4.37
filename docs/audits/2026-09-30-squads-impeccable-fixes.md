# Squad discovery: audit fixes and browser verification

The squad list now exposes keyboard-accessible actor links, visible profile-card details, readable active filters in both themes, selected-filter summaries, progressive columns, and 44px coarse-pointer controls. Workspace/template labels are shorter. Saved squad descriptions and column preferences retain their original values.

## Changed files

- `packages/views/squads/components/squads-page.tsx`: native leader/creator links, theme-safe filter states, localized status summary, responsive columns and touch controls.
- `packages/views/squads/components/create-squad-chooser.tsx`: touch target sizing.
- `packages/views/agents/components/agent-profile-card.tsx`: always-visible detail link with a focus indicator.
- `packages/views/locales/{en,zh-Hans}/squads.json`: concise tabs, filter summaries and column-width explanation.
- `packages/views/squads/components/squads-page.test.tsx`: four new regression cases for keyboard navigation and filter feedback.
- `e2e/squads-audit.spec.ts`: real-browser interaction, contrast, layout, touch and performance checks.
- `e2e/squads-design.spec.ts`: updated labels and current creation-dialog semantics.
- `.trellis/spec/views/frontend/component-guidelines.md`: documents the squad-specific column tiers and accessibility contract.
- `docs/plans/2026-09-30-squads-audit-fixes.md`: approved scope and verification plan.

Removed the obsolete full-table minimum-width calculation and the profile-card hover-only visibility rules. Reused AppLink, Button, ListGrid, design tokens and the existing view store. No dependency, global color-token, server or stored-content changes were needed.

## Browser results

Tested the actual optimized Next.js 16.2.6 build in Chromium against an isolated eight-squad workspace on the local API. The runtime fixture has no daemon process or agent CLI attached. Fixture workspaces were deleted after each run.

Both browser suites passed:

- Tab from squad name to leader; Enter opens the correct agent page without invoking the row's squad destination.
- Creator link keyboard navigation; browser Back restores the list.
- Keyboard-opened member preview exposes a visible, focused details link.
- Leader filtering, visible selected-name/result summary, and keyboard clearing.
- Sorting, row menu open/close and focus restoration.
- New-squad dialog, template navigation, project setup and persisted display preferences.
- English/Chinese locale switching preserves user-authored names and descriptions.
- At window widths 320, 390, 768, 900, 1024 and 1440px, both document and table scroller fit their available width, including with all columns enabled.
- Coarse-pointer filter/display targets measure 44×44px; creation and row actions also meet the 44px target.
- No uncaught page exceptions. Recorded aborted requests are navigation/prefetch cancellations, not failed assertions or HTTP error responses.

Measured active-filter text contrast after transitions settle:

| Theme | Contrast | AA normal-text threshold |
|---|---:|---:|
| Light | 16.97:1 | 4.5:1 |
| Dark | 13.96:1 | 4.5:1 |

## Performance measurements

Conditions: local production build, Chromium, 1440×1000 viewport, no network throttling, one browser-cold navigation followed by two warm navigations. Three samples describe this machine and fixture, not field percentiles. API resource sizes are cross-origin and unavailable to Resource Timing, so transferred-byte totals exclude those bodies.

| Metric | First navigation | Warm 1 | Warm 2 | Median |
|---|---:|---:|---:|---:|
| TTFB | 34.9ms | 8.2ms | 10.0ms | 10.0ms |
| FCP | 712ms | 456ms | 428ms | 456ms |
| LCP | 756ms | 484ms | 448ms | 484ms |
| CLS | 0 | 0 | 0 | 0 |

An additional warm-cache sample with 4× CPU slowdown produced FCP 720ms, LCP 840ms and CLS 0. Opening the filter menu took approximately 154ms including Playwright actionability/observation overhead. Observed Event Timing durations reached 48ms; these lab interactions are **not field INP**. Opening and applying a leader filter at normal CPU took approximately 435ms including the entire multi-step automation sequence.

The first load fetched approximately 1.95MiB of observable resources, including 1.66MiB of encoded JavaScript across the shared application shell and this route. LCP and CLS are healthy in this local test, but the script payload remains above a lightweight-page budget and deserves separate bundle analysis before making weak-device/slow-network claims. No bundle-size reduction is claimed by this UI patch.

The existing development server had two environmental inconsistencies: its frontend port was outside the backend's CORS allowlist, and its cached agent-link behavior did not match current source. A temporary local proxy enabled development baseline capture; final acceptance used a separate production build and server without stopping the existing developer services. Development LCP was 1.45–2.33s, but the environment differs, so **do not interpret production-versus-development timings as a performance gain caused by this patch**.

## Verification

- 85 focused Vitest checks across squad-page, profile-card and locale-parity suites passed.
- `pnpm typecheck`: 9 package tasks successful.
- ESLint passed for changed view components/tests.
- Both Playwright suites passed against the isolated production build.
- Production build completed successfully. Existing CSS optimizer warnings concern the supported `::highlight()` pseudo-element; no new build errors.
- Impeccable detector returned `[]` for the three changed UI components.
- `git diff --check` passed.
- Independent static review found no additional correctness issues in the task diff.
- Batched desktop, compact, touch and dark-theme screenshot review passed; visual verdict 93/100.

## Evidence

Local artifacts are retained under `.omx/reports/squad-browser/`:

- `production-performance.json`: raw navigation and resource samples.
- `production-interaction.json`: contrast, layout, touch, CPU slowdown and interaction measurements.
- `production-zh-desktop.png`, `production-zh-touch.png`: localized screenshots; saved English fixture names remain unchanged intentionally.
- `production-dark-filter.png`, `production-320.png`: theme and smallest-width evidence.
- `before-performance.json`: development baseline, with the comparison limitation above.

Visual verdict: `.omx/state/squads-audit/ralph-progress.json`.

## Remaining limits

No real-device Safari/Firefox, assistive-technology session, production-network field monitoring or large-workspace load test was performed. The chosen column tiers intentionally hide optional columns in narrow containers; the display popover explains this and preserves preferences for wider windows. User-authored descriptions are not automatically summarized or rewritten; the full text remains available in squad details and the description title. Existing unrelated working-tree edits were preserved.
