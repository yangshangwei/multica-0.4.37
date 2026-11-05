# Cross-workspace collaboration experience audit and repair

## Goal
Audit every existing cross-workspace collaboration surface with Impeccable, explain business/visual/usability/simplicity findings, implement the repair plan, and independently review the result.

## Background and authority
User requested audit, repair plan, implementation and review until complete; then requested continued progress and approved task creation. Existing Web/Desktop UI is shared in packages/views/collaboration, plus administration, source-workspace entry and managed knowledge panels. Native mobile has no collaboration UI. Existing unrelated uncommitted work must be preserved.

## Requirements
- R1: Inventory workbench (current/participating), creation, detail/workflow/delivery/discussion, execution/resources, attachments/children/context/knowledge/activity, notifications, source entry and administration.
- R2: Independent design/business and technical assessments with severity, evidence and page coverage. Distinguish measured findings from visual inference.
- R3: Preserve typography, semantic tokens and compact product identity while repairing verified hierarchy, responsiveness, focus, accessible names and state defects.
- R4: Preserve account/workspace authority, server capabilities, exact acceptance, idempotent recovery, drafts and platform navigation.
- R5: English and Simplified Chinese parity; no new dependencies or parallel business-state models.
- R6: Search/pagination, empty/error/loading, long text, keyboard, narrow viewport and touch interactions must remain truthful and usable.

## Acceptance criteria
- AC1: Every existing surface has an audit disposition; each accepted repair maps to changed files or a justified no-change review.
- AC2: Review and repair plan exist before product edits.
- AC3: Behavioral repairs have meaningful regression coverage; affected tests pass.
- AC4: Views lint, relevant type checks, locale parity, static checks and collaboration tests pass; unrelated baseline failures are recorded separately.
- AC5: Fresh browser evidence covers wide/narrow layouts, both locales/themes and representative task, creation and administration interactions. Distinguish fixture/mocked UI checks from real API lifecycle acceptance.
- AC6: Independent final review has no unresolved in-scope P0/P1/P2 findings. Do not infer native/provider acceptance from shared UI checks.

## Constraints and scope
No backend permission redesign, migration, native feature expansion or new dependency. The user already authorized execution of the repair plan; after concrete planning/review, proceed without another routine permission loop.
