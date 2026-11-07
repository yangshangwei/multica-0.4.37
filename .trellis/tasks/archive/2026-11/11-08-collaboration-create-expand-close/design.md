# Description growth and desktop close boundary

## Source-backed findings

Expansion currently resizes only DialogContent. CreationContents is a block-flow body, and the description wrapper is space-y-1 around a field-sizing-content textarea with64pxminimum; the new body height becomes empty space after advancedfields. Independent Impeccable layout assessment confirms this; mechanical layout detector returned[] and did not detectthe structural defect.

Close events work in the prior Chromium fixture. Desktop source evidence shows a48pxWebkitAppRegion drag bar, while expanded Close lies aroundy24–52. Currentpopup has no no-drag boundary. Native Electron Quartz HID evidence confirmed the hypothesis: baseline Close got no DOM events and dragging its center moved the owned window32×24px; the same events after popup no-drag close correctly with stationary window. Keep the correctly wired DialogClose.

## Smallest implementation boundary

Expected product file creation-form.tsx, with relevant creation/AI integration tests only if behavioral assertions needed. Keep BaseUI DialogClose behavior and add a local no-drag boundary to the popup/header if reproduction confirms desktop drag interception. Chromium browsers ignore app-region. Do not edit shared Dialog globally without evidence and scope review.

## Spatial thesis

Expanded body becomes a flex column with gap-4 instead of space-y-4; natural-height direct children do not shrink. Description is the only grow item with auto basis and no shrinking. Inside it, label and AI controls keep naturalheight while textarea grows with auto basis/shrink-0 and retains field-sizing-content/min64. Extraavailablebody space therefore becomes editing space; additional sectionsconsume thatfree space beforebodyoverflow. Default/standalone layout stays existing blockflow. An explicit dataattribute + scoped descendant variants or a presentational expandedprop is acceptable; do not branch two mounted trees or use size-basedkeys.

## Risk and rollback

Flex sizing must preserve scroll semantics on longtext/AIquestions/files/advancedopen; realgeometry rather thanDOMclass tests proves it. Desktop no-drag must cover actual hit region but retain popupfocus/ref/lifecycle. Scopedrevert restores previouslayout without migrations.
