# Shared knowledge UI integration brief

Mode: Operate. This is an extension of existing collaboration/project/workspace surfaces, with settled product scope and permission policy. Preserve current visual identity, navigation, type/spacing tokens and reusable controls. No new global product/design system, illustration, dependency, AI summarizer or skill automation.

## Context already loaded

Impeccable context ran once at the knowledge worktree targeting `packages/views/collaboration/collaboration-detail.tsx`. Its package-local detector found no PRODUCT.md/DESIGN.md; manual source inspection establishes the incumbent UI. This is a scoped extension, so continue from existing code and the approved task, without another purpose/style interview or concept tournament. Read its craft-floor immediately before UI edits and use bounded capture/review passes.

Visual/code anchors: `CollaborationMemoryPanel` and `CollaborationExecutionPanel` mounted at collaboration-detail.tsx; `ProjectDescription` in projects/components/project-detail.tsx; workspace context editor in settings/components/workspace-tab.tsx; shared semantic/token CSS. The runtime child final-browser/runtime-handoff-final-web.png is a source-bound **component crop**, useful for density/type evidence but not a full-page/responsive proof. Capture complete new source/target flows for final QA.

## User-visible scope

- Source collaboration: list candidate versions, propose/correct/derive, inspect actual source/version/evidence, and human approve/reject/withdraw according to current server capabilities.
- Targets: independent managed-contribution sections in existing project detail and workspace context/settings; leave the human base editor separate. Web and Desktop use the same views.
- Proposal form uses lawful existing source references and current visible source workspace/project. A nonmember current handler must be able to propose to those source destinations without querying private project/member catalogs. Other destinations use existing normal visibility selectors.
- Source selection should reuse current memory/comment/run record readers and exact server identities/revisions/content hashes. No arbitrary JSON editor as the normal user path and no guessed record/hash. Inspect existing readers and coordinate with A2 if a real source-selection API gap exists; do not silently restrict all proposals to nonempty memory.
- Require readable full detail before approval; pin the actually reviewed version/hash/state revision into the intent. Changes to that review identity invalidate any confirmation state. 409/428 requires reread; an unknown result uses original candidate/request recovery. A new edited intent gets a new UUID.
- Approved is permission to use the contribution, not proof the stated fact was verified or injected into every task. Show expiry/applicability/source and verification claims honestly. Skills remain out of scope.

## State and privacy

Use the committed Core hooks/actions/cache/recovery contracts. Store only selected IDs, form input and client intent as local UI state, not copied server projections. Expired/revoked/unavailable queries must remove old body before reloading; a modal cannot keep a hidden-source body alive through local state.

Source-unavailable pending candidates may be rejected only with the server's human source-read/target-confirmation grant; restricted target-only rows may withdraw but cannot reveal source identity/body or reject. All meaningful actions await the exact receipt; known refusal receipts are not success toasts. Old unsupported servers do not render a fabricated empty collection.

Detail `history_truncated` is required. Detail history is capped at50; source collection pages all versions, so older entries remain reachable. Recovery stores only actor/server/candidate/request/action identities, never draft body or server data. No optimistic publish/delete/route change.

## Verification boundary

Core owns parsing/budget/lease matrices; shared component tests own meaningful interaction/accessibility and named wiring regressions. Avoid duplicating Core matrices through DOM mounts. Translate en/zh-Hans consistently and follow conventions before new wording. Use NavigationAdapter/AppLink; no next/router imports or new business stores in views.

Final acceptance must use production Web and native Desktop with known source and test-owned data. Cover external-handler proposal, human review/precise approval, correction/expiry/withdraw/source loss, target-only withdrawal, uncertain original receipt recovery, keyboard/long-content/overflow and actual clearing. Actual next-input injection belongs to C3/G2 evidence and cannot be claimed from UI alone. Run one mechanical Impeccable detector pass after implementation, then independent screenshot review with at most one correction batch unless a concrete unresolved defect requires more.
