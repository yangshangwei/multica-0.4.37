# Settings audit remediation and regression verification

## Goal

Make the existing shared and desktop settings reliable under failed reads, failed or overlapping writes, keyboard use, and narrow windows. Close F01–F17 from `.impeccable/audit/2026-10-13-settings/report.md` without changing the product's settings information architecture or visual identity.

The user explicitly requested creating a Trellis task and implementing/verifying the audit recommendations on 2026-10-14. The preceding audit report supplies the agreed scope, recommended behavior, and evidence; this task transcribes that scope into implementation artifacts rather than starting a new product-design interview.

## Background and evidence

- Audit: 20 visible settings tabs, 17 grouped findings (8 P1, 9 P2), 15 groups with runtime evidence and 2 source-only groups.
- Evidence and source anchors remain in the audit report and its `verification.json`.
- Native baseline: Electron 39.8.7 on macOS, 1440px and 900px windows, 900×1000 at 200% native zoom (450×500 CSS pixels).
- Findings distinguish read errors from successful emptiness, denied access from unavailable permission data, and frontend fixtures from real backend writes.

## Requirements and acceptance criteria

| ID | Required observable outcome |
| --- | --- |
| F01 | A persistently failed status catalog reaches a stable retryable error; mounting category rows does not create an unbounded request loop. |
| F02 | Catalog, member, workspace-permission and provider reads distinguish pending, failure, genuine empty/unconfigured, and transient refresh failure. Retry remains reachable; unknown membership never masquerades as confirmed non-admin access or enables unsafe leave actions. |
| F03 | Label, property and Quick Action menus are visible and pointer/keyboard reachable at 900px and 450 CSS px, with or without the main sidebar. |
| F04 | A property dialog with at least 12 options keeps its title/name and final Save/Cancel reachable at 900×700 and 450×500 CSS px. |
| F05 | The eight verified controls expose purpose-specific accessible names; provider fields changed in this task follow the same label contract. |
| F06 | Shortcut recorders expose their current binding or unassigned state; decorative keycaps do not remove the only accessible value. |
| F07 | Warning text and connected badges meet 4.5:1 text contrast in both themes using existing semantic foreground tokens. |
| F08 | Transient refresh failures preserve authorized MCP/triage/provider content and edit-node identity; definitive access loss still hides protected content. |
| F09 | A failed profile/workspace avatar save keeps the crop workflow and does not announce success. |
| F10 | An empty required name displays an associated validation error and an explicit unsaved state; active form drafts survive validation and can be corrected and saved together. No new cross-route draft system is required. |
| F11 | All audited submit/confirm paths reject duplicate pending submissions. Failures retain the target and a usable retry path. |
| F12 | All token-result close paths obey the saved-token acknowledgement, including Escape, close button and backdrop. |
| F13 | GitHub setting controls cannot silently accept and discard another operation during a pending save. |
| F14 | Removing a repository waits for persistence; a failed removal retains the row/confirmation and supports retry, including the last row. |
| F15 | A pending timezone write cannot be overtaken by an accepted second selection or overwrite a later accepted choice. |
| F16 | PAT list/create responses pass a schema boundary, malformed data never crashes the view, and an unusable create result cannot look successful. Credential values are not logged. |
| F17 | Manual update results are announced through an appropriate live region without moving focus. |

## Scope and constraints

- Shared settings, directly related triage/MCP/provider views, desktop-only settings, PAT API parsing, existing locale namespaces, and colocated regression tests.
- No new dependencies, backend/database changes, endpoint changes, deployment, release, or broad UI redesign.
- Feature gates remain unchanged. The separately gated plugin deletion appendix E01 and new billing/plugin delivery are deferred; do not enable these products to claim coverage.
- Preserve status ordering/archive protection, notification recovery, integration navigation/focus, endpoint setup, and existing package boundaries.
- Preserve unrelated working-tree changes, especially the concurrently edited API client and collaboration backend.
- Runtime verification uses disposable profiles and memory fixtures; do not send real invitations, create credentials, charge accounts, install plugins, or start agents.

## Completion

All F01–F17 have an implementation and a documented regression/verification result. Relevant package tests, lint, type checks and native geometry/AX/contrast probes pass, or an unrelated baseline failure is separately evidenced. Update project recovery conventions when the fix establishes a reusable contract; retain the original audit and publish a remediation result alongside it.
