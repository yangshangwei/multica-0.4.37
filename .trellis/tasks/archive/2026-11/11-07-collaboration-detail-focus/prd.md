# 协作详情主流程聚焦与操作弹框优化

## Goal
Make collaboration detail easy to scan and act on: understand the requirement, discuss work, submit a result, and review it without expanding the page for workflow input.

## Background and evidence
- User explicitly requests impeccable audit, a new Trellis task, a concrete optimization plan and implementation of discovered issues; this authorizes task creation and execution after written planning.
- Shared Web/Desktop detail lives in packages/views/collaboration/collaboration-detail.tsx:84. WorkflowForm is inserted under the toolbar in workflow.tsx:105; More also expands the toolbar at :89.
- Participants exposes role/permission prose above requirements. Auxiliary disclosures are split between main content and sidebar; task fields remain permanently visible.
- Workflow feedback and recovery at workflow.tsx:99 must travel with a modal form, otherwise an inert backdrop hides the only recovery controls.
- SourceCollaborationActionDialog already wraps workflow; do not create nested action dialogs.

## Requirements
R1. Workflow actions open one accessible bounded dialog, keeping underlying detail geometry and scroll stable. Ordinary discussion stays inline.
R2. One role-specific primary action; promote only directly relevant next actions, with other operations in an overlay menu. Keep capabilities and Core-derived state authoritative.
R3. Main content presents requirements, current result/decision, discussion. Secondary task context/fields, participants/permissions, files, shared context, knowledge and audit are compact and intentionally discoverable.
R4. Mention choices and file selection in comments open on demand; retained file uploads/recovery remain visible. Preserve single-action human comments and explicit AI confirmation.
R5. Preserve exact submission/version, original-key recovery, denied access behavior, drafts, attachment privacy and repeated notification anchors. Active work, errors and recovery must never disappear behind closed disclosures or a dialog backdrop.
R6. Both locales, keyboard, narrow layouts and enlarged text remain usable; no new dependencies or platform API imports.

## Acceptance criteria
- [x] Opening/closing workflow actions causes no underlying layout growth; focus enters named dialog and returns to actual trigger; Escape works; active submission cannot silently dismiss.
- [x] Failure and original-key recovery are operable within the dialog; success closes it; result draft survives dismissal; source action entry has only one dialog.
- [x] Toolbar and first viewport emphasize next action, requirements and current responsibility; secondary metadata/permissions do not dominate.
- [x] Comments still publish through existing preflight; optional mention/file tools remain available; saved upload attention reveals its controls.
- [x] Existing collaboration regression suites, scoped lint/typecheck and static audit pass, with fresh desktop/narrow browser screenshots and documented evidence.

## Scope
Shared Web/Desktop collaboration detail and its existing components, tests, bilingual strings and Trellis spec. No backend contracts, workflow rules, mobile redesign, dependency installation, or edits to unrelated pre-existing changes.
