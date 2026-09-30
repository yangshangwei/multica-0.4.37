# Squad default icons implementation plan

Goal: use Lucide icons and existing Skill category colors for default squad avatars, retaining circular squad containers and square Skill tiles.

Approved design: map built-in template keys to their default emoji, Lucide glyph and category tone in the views layer. Only an absent avatar or the matching template default emoji uses this presentation; uploaded images, other emojis and unknown templates retain existing avatar behavior. Existing stored defaults and new templates share the same rendering, without database changes.

1. Add regression coverage for default-template icons, custom avatar preservation and unknown-template fallback.
2. Add a shared squad avatar renderer using existing avatar size and Skill color tokens. Route list, detail, profile, generic actor and template previews through it. Let the upload control accept a presentation preview without changing persistence.
3. Run focused tests, package typechecks/lint, inspect a rendered preview in light/dark themes, and review the final diff.

Known data limit: the API does not distinguish a manually reselected original template emoji from an untouched default. Both display as the default icon.

## Expanded icon standard

User requested all agent/squad/skill default and creation icons use Lucide. Replace avatar emoji suggestions with an allowlisted Lucide picker, preserve uploads and reaction emoji, render legacy avatar emoji as Lucide, and persist new choices as `icon:<name>`. Share the registry in UI (no core imports), validate backend markers and update default creation. Keep mobile rendering compatible using its installed SVG renderer. No database migration. Validate marker parity, rendering, picker persistence/error states and existing creation flows. Remove the redundant squad-only SVG rendering in favor of the common avatar renderer.

Completion scan also includes project icon create/edit/display: retain the existing `icon` field, validate new markers, convert legacy raw emoji on display, and preserve project icon footprints.
