# Follow-up diagnosis

## Newest evidence
The first repair 3fc0cacbd fixed retained Dialog hidden attributes. The user's new feedback establishes a separate remaining OS pointer failure: Escape and backdrop work, X does not. Previous validation used accessibility element activation and was insufficient proof of OS mouse hit testing.

Native CUA reproduction after first repair: actual Electron app at localhost:5666; screenshot 1442x768; drawer X at [1420,20]. `multica.click([1420,20])` moved the pointer visibly to the X but the entire populated Activity drawer remained. This is different from the prior empty retained shell bug.

## Root cause / discriminating prediction
DesktopAccountNavigation renders a 48px high shell bar with WebkitAppRegion:drag across the window. CollaborationDetailAuxiliary is a top-0 full-height portaled Dialog; its close control sits over that region and does not set no-drag. Native dragging intercepts pointer events before the DOM. Accessibility activation and synthetic DOM clicks bypass this, explaining the false positive. Existing CollaborationCreationEntry in creation-form.tsx already applies style={{WebkitAppRegion:'no-drag'} as CSSProperties} to DialogContent. Its spec explicitly rejects synthetic clicks as native evidence.

## Bounded repair
- packages/views/collaboration/detail-auxiliary.tsx: apply the same no-drag boundary to the owning resource DialogContent; do not replace DialogClose wiring.
- packages/views/collaboration/detail-auxiliary.test.tsx: assert the actual mounted resource popup exports this Electron contract; preserve existing close/backdrop/draft tests. DOM tests cannot prove OS hit testing, so coordinator must repeat actual coordinate click after repair.
- Main updates collaboration detail spec and prior verification claim, captures a short break-loop retrospective and native evidence.

## Check plan
New contract regression red before fix. Auxiliary+detail suites, changed-file lint, Views types, diff check. Main native CUA coordinate X repeat + Escape/backdrop, and independent Trellis check. Do not operate UI, restart app or commit from sub-agents.
