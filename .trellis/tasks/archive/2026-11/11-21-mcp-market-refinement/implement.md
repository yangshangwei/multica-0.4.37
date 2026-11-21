# Bounded refinement plan

## Boundary and cleanup plan

The gap is presentation density and advance guidance in the existing shared MCP catalog. Presentation lives in `mcp-market.tsx`; standalone context lives in `mcp-market-panel.tsx`; type-tab target geometry lives in `marketplace-page.tsx`. Keep all creation, assignment, inventory and query ownership intact.

Expected source scope:
- `packages/views/mcp/mcp-market.tsx`: group filter rows, compact card metadata, factual preparation hints and action wording.
- `packages/views/mcp/mcp-market-panel.tsx`: short standalone flow explanation.
- `packages/views/marketplace/marketplace-page.tsx`: local coarse-pointer sizing only; retain other ongoing edits.
- `packages/views/locales/{en,zh-Hans}/settings.json`: paired UI strings.
- Focused tests in the existing MCP suite; existing marketplace tests protect navigation/draft behavior. Do not add class-mirroring tests.
- `.trellis/spec/views/frontend/mcp-market.md`: record final presentation/metadata contract.

No backend, API, core store, shared Tabs primitive, Skills page, squad behavior, dependency or unrelated cleanup changes.

## Sequence

1. [x] Baseline existing behavior: MCP catalog/panel, marketplace, locale parity (127 passing).
2. [x] Complete isolated layout and mechanical assessments; preserve original source snapshots for attribution.
3. [x] Dispatch the approved bounded implementation to a Trellis implement agent. Use existing tests to preserve behavior and add focused coverage for meaningful new metadata/guidance behavior.
4. [x] Build a task-owned fixture using real shared React components and compiled UI CSS. Capture one batched wide/narrow, locale/theme/touch round; verify keyboard, filtering and dialog return paths.
5. [x] Record a visual verdict before any further UI edits. First verdict 87/revise led to one native category-scroll / shorter-copy correction batch. Final confirmation follows.
6. [x] Dispatch Trellis check against the final diff; run targeted tests, lint/typecheck, locale parity and detector. Record exact scope and any baseline limitations.
7. [x] Sync the MCP presentation spec, record task evidence and finish without touching other work.

## Functional verification follow-up

The final visual pass passed at 92/100. The native-strip keyboard check exposed Chromium focusing the center of a partly visible long chip without revealing its whole label. A minimal experiment confirmed that CSS scroll margins did not change this, while native `scrollIntoView({block: "nearest", inline: "nearest"})` on focus did. Repair that narrow behavior and rerun the existing failing browser check. This is a functional regression repair, with no further visual-polish round. Consolidate repeated type-tab target classes on the page-owned TabsList while preserving exact geometry and all pre-existing squad-template lines.
