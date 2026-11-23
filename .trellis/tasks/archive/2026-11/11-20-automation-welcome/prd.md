# Automation welcome empty state

## Request and authorization

The user supplied a reference image and asked to use Impeccable to implement a
welcoming automation page when no automations exist, while opening the existing
management list when automations exist. On 2026-11-20 the user approved creating
the task and proceeding after reviewing the two-column design and behavior.

## Requirements

- A successfully loaded, unfiltered empty automation list shows a spacious
  welcome surface matching the reference's composition: left introduction and
  actions, right compact template cards in two columns on wide screens.
- Use the headline “让智能体按时自己开工”, concise accurate schedule/webhook and
  output-mode guidance, “新建自动化” and “阅读文档” actions.
- Show up to six real server templates with their titles, descriptions and
  actual localized schedules. Include an entry to browse the complete catalog.
  Do not fabricate the reference's unavailable daily news template.
- New automation opens the existing blank creation dialog. Documentation opens
  the existing workspace documentation route. Template selection opens the
  existing configuration flow and only creates after explicit form submission.
- Existing automations continue to open the existing list, including paused
  automations and filters with no results. Loading/error are never first-use.
- Template loading/error/empty states preserve blank creation and document access;
  failed template loading offers retry.
- Support English and Simplified Chinese, light/dark themes, keyboard operation,
  44px narrow/coarse-pointer targets, long text and narrow containers.
- Preserve template-back navigation, scroll and focus handoff behavior.

## Acceptance

- Shared component tests prove state routing, CTA behavior, real template
  selection, complete-catalog access and recoverable template failures.
- Existing catalog/configuration regression suites continue to pass.
- Browser evidence shows the real shared page at desktop and narrow sizes,
  including Chinese and dark mode, without horizontal clipping.
- Focused lint, views typecheck, locale parity and affected tests pass.

## Scope

Shared web/desktop views and bilingual strings, focused browser coverage and the
existing automation discovery spec. No backend templates, API contracts, global
design tokens, dependencies or native mobile UI changes.
