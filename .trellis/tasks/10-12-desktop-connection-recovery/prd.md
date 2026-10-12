# Desktop connection recovery

## Goal and authorization
Let desktop users recover from an unavailable or incorrect server without signing in first. The user approved the proposed recovery-page design with “请按照建议优化” and explicitly authorized task creation/execution with “创建任务并推进”.

## Background
- App.tsx:338 replaces the main shell during auth recovery; App.tsx:351 uses the same page for workspace-list failure.
- pages/auth-recovery.tsx:19 exposes only Retry, leaving no in-app way to correct a broken address.
- platform/auth-initializer.tsx:262 retries with capped backoff; HTTP requests have no application deadline.
- main/index.ts:832 already probes a new endpoint before clearing the old session and reloading.

## Requirements
- R1: Show configured endpoint, truthful connection state and localized failure copy. Do not claim a retained login on a tokenless boot.
- R2: Change server remains available while waiting/retrying. Reuse the prefilled endpoint editor with Test, Save and connect, Back, and a notice that switching requires session verification.
- R3: Bound auth/config/device recovery requests to 10 seconds. Preserve credentials on transient errors and existing terminal-auth/epoch protections.
- R4: Show actual retry progress and countdown only for scheduled retries. Prevent duplicate manual attempts; keep initial workspace recovery actionable and able to resume when the server returns.
- R5: Background recovery cannot unmount the open editor or erase entered text. Back reveals current app/recovery state. New-endpoint validation failure preserves original configuration/session.
- R6: Expandable/copyable details contain only endpoint, client version, last failure time, sanitized category/status; never token, raw body or stack. Copy has success/failure feedback.
- R7: Preserve design tokens, translations, DragStrip, keyboard accessibility and narrow-window usability. Child issue windows hand off server editing to main because save is main-only.

## Acceptance criteria
- A1 (R1,R3,R4): Hanging/rejected boot reaches usable recovery, keeps credential; retries succeed after server recovery.
- A2 (R2,R5): Offline user opens prefilled editor, tests invalid/unreachable endpoint, returns with original session intact.
- A3 (R2,R5,R7): Valid endpoint switch uses coordinated reset/reload; no old request/token reaches new server. Child window can reach main editor.
- A4 (R5): Authentication succeeds while editor holds unsaved text; text/editor persist; Back shows recovered application.
- A5 (R4): Manual retry stays on recovery surface without overlap. Workspace failure neither implies invalid credentials nor promises an unscheduled retry.
- A6 (R6,R7): Details omit fixture secrets; copy, keyboard navigation, Chinese/English and narrow/normal windows work.
- A7: Targeted regressions, affected-package lint/typecheck, isolated Electron visual/interaction acceptance and independent review pass.

## Out of scope
Offline business-data browsing/editing; backend/auth protocol changes; dependencies; full settings redesign; packaging/deployment.

## Blocking questions
None. Direction and execution are authorized in this conversation.
