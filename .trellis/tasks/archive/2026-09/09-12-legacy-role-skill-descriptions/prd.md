# Translate legacy built-in skill descriptions

## Problem

The user screenshot shows version 1 default descriptions for release-check and architecture-decision-record still in English. The presentation resolver only recognizes the latest defaults, so it treats these historical defaults as customized text. Both screenshot strings match the original templates in commit ca3a79d18.

## Acceptance criteria

- Both historical default descriptions display accurate Chinese translations across every existing shared surface.
- The original English descriptions remain displayed in English UI and are still searchable alongside the Chinese purpose text.
- Latest defaults continue to translate; customized descriptions, renamed skills, unrelated records, stored content, and invocation IDs are preserved.
- Reproduce the screenshot using the exact historical text before applying the fix and verify it afterwards.

## Implementation and verification

Add explicit description_v1 locale entries for these two built-ins and recognize only their exact historical default descriptions in the existing presentation resolver. Keep provenance checks unchanged. Add pure and list regressions, run affected skill/selector and locale parity tests, typecheck, lint and static analysis, then inspect the actual components using legacy fixture data in the existing isolated browser harness.

No backend, database, runtime instruction, or layout changes are required.
