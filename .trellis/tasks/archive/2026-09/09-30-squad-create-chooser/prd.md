# Squad creation chooser

## Requirements

Replace the squad page header dropdown with a modal matching the supplied New Skill chooser reference. Keep the existing template and custom squad creation destinations and labels. Show a title, concise introduction, and two full-width option cards with muted icon tiles, descriptions, and right chevrons. Use existing shared dialog primitives and semantic tokens for web and desktop. Support English and Simplified Chinese, narrow viewports, dismissal, and keyboard navigation.

## Acceptance criteria

- The New Squad button opens the chooser instead of a dropdown.
- Each option opens its original creation modal and dismisses the chooser.
- Escape and Close return focus to the trigger; selecting an option leaves focus to the next modal.
- Layout matches the skill chooser's width, spacing, typography, and card treatment.
- Existing squad discovery, creation, and locale checks pass, with lint/typecheck and screenshot evidence recorded.

## Bounded implementation plan

1. Verify existing creation wiring before editing.
2. Replace the header dropdown with a small local chooser component, preserving empty-state shortcuts.
3. Add bilingual descriptions and update the existing creation interaction test.
4. Run focused tests, lint, typecheck, mechanical visual analysis, and browser verification at wide and narrow widths.
