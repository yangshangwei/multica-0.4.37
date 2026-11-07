# 协作详情工具栏与资料抽屉

## Goal
Implement the user's selected option one: a compact auxiliary toolbar near the task summary opens a right-hand drawer, keeping requirements, delivery and discussion stable and focused.

## Requirements
- R1: Replace the bottom disclosure stack with labelled icon/text buttons for issue details, attachments, shared context, knowledge, sub-issues and activity. Preserve current permissions and all existing functionality.
- R2: Open one right-side drawer with fixed title/issue context, category navigation and independent scrolling. Wide layout approximately 640–720px; narrow layouts fill viewport width. Use existing semantic tokens, restrained borders, clear selected state, 44px touch controls and reduced-motion support.
- R3: Preserve editors, proposals and active uploads across close/reopen and category changes. Hidden content is inert and absent from keyboard/accessibility navigation. Do not mount duplicate panels.
- R4: Show truthful known counts only (attachments and projected children); unknown counts are omitted. Main toolbar explicitly surfaces panel errors/pending recovery without stealing focus or hiding the recovery path.
- R5: Reference-files link opens the files drawer. Explicit memory/version links open correct panel and focus its real target. Repeated navigation works; scope resume preserves saved main scroll. Comment/run/description anchors close obstructing drawer and focus the main target.
- R6: Close/Escape returns focus to actual opener. Nested previews and confirmations close independently. Denied access and issue/viewer/server changes must not leak old panels or drafts into another identity.

## Acceptance
- [x] Six toolbar entries replace bottom disclosures; ordinary work remains the primary surface.
- [x] Drawer and its nested controls work with keyboard, Chinese/English, narrow widths and enlarged text; background height/scroll stay stable.
- [x] Close/category switching preserves draft input and ongoing work; hidden panels are not focusable.
- [x] Pending/error reminders lead directly to recovery; counts never treat unavailable reads as zero.
- [x] Reference and repeated memory/version navigation work, as do main anchors and permission loss.
- [x] Relevant unit tests, typecheck, scoped lint/static detection and native visual smoke pass with recorded limits.

## Scope
Shared Web/Desktop collaboration view only, UI composition and panel reporting props, translations, tests, specs. Reuse existing API/query/state and dialog primitives; no new dependencies, backend changes or mobile app changes. The user explicitly chose option one and requested implementation; proceed after recording this concrete plan.
