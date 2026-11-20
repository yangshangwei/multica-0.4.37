# Independent visual acceptance

Disposition: **ship**. Visual verdict: **94/100**, threshold 90.

Reviewed on 2026-11-20 by a fresh Codex native sub-agent using the Impeccable
finish-reviewer contract. This is a scoped extension of the existing Multica
interface. The approved reference pins the badge, two-part composition and
compact cards; existing semantic tokens remain the rendering authority.
PRODUCT.md, DESIGN.md and new-world comp/build artifacts are absent and are
not requirements of this approved extension.

## Persistence

The PRD, design and implementation plan preserve the approved composition,
server-template constraint and existing-list behavior. All 18 required PNGs
decoded and were visually inspected in both English and Simplified Chinese.
No required capture was missing, blank or malformed.

Stable copies are under `.impeccable/review/automation-welcome/`. The original
reference is retained there as `reference.png`; `capture-manifest.json` records
the PNG and measurement hashes. The nine captures per locale are:

- `desktop`: 1440×1000.
- `wide`: 2048×1000.
- `compact`: 1024×768.
- `mobile` and `mobile-templates`: 390×1000, top and scrolled template content.
- `small`: 320×900.
- `desktop-dark`, `keyboard` and `populated`: 1440×1000.

## Fidelity

| Promise | Finding | Evidence |
| --- | --- | --- |
| Left introduction, right template group | Match | Desktop, wide and compact captures retain the composition and action hierarchy. |
| Typography | Authorized adaptation | Existing display/body/card roles follow the approved token reuse. |
| Material | Match | Neutral cards, Lucide icons and restrained shared elevation retain the reference treatment. |
| Ground | Match | Reference and light captures sample RGB 251,251,251; inherited dark canvas samples RGB 17,17,20. |
| Badge and actions | Match | Both locales retain the approved blue badge and creation/documentation actions. |
| Six compact templates | Authorized adaptation | Real server templates replace unavailable illustrative content; actual schedules and complete-catalog access remain. |
| Narrow layout | Authorized adaptation | The introduction precedes a single card column at 390px and 320px; bottom captures expose the remaining cards and catalog link. |
| Keyboard focus | Match | Both keyboard captures show a foreground ring; genuine Tab/Shift+Tab metrics report a connected, active, focus-visible element and 2px ring. |
| Saved automations | Match | Both populated captures show the existing table with a paused automation. |

`metrics-en.json` and `metrics-zh-Hans.json` report no horizontal overflow in
the welcome region or document, and no page errors. Compact measurements were
refreshed after the responsive sidebar settled.

## Ceiling

Reached for the approved scope. The first viewport makes the feature purpose
and creation action findable. Additional artwork, ornament and authored motion
were not promised.

## Material fixes

None. Earlier evidence issues concerned pointer-mode keyboard captures and a
shared production output overwritten by another session. Both were corrected
by recapturing on the task-owned production output, without application edits.

## Keep

Preserve the approved badge, headline and actions, six neutral compact cards,
truthful schedules, complete-catalog link, keyboard focus and populated-list
branch. This evidence covers production Chromium and the shared views; it does
not claim packaged Electron, native mobile, physical touch or spoken
screen-reader acceptance.
