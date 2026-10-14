# Native return-focus regression

The final production Electron acceptance reproduced an A02 gap after the
committed implementation. All 16 Chinese/English, light/dark, wide/compact/
narrow/native-200% layout cases pass. The empty-list return path still loses
focus after its initial restoration, so the pre-fix verdict remains revise/88.

## Reproduction and cause

1. In the empty automation list at 886 × 540, select Repository maintenance.
2. Tab to Documentation check and press Enter. The gallery saves category,
   142px scroll and `documentation-check` as its pending focus target.
3. Return with the in-flow Back control, the Desktop keyboard history shortcut,
   or the navigation adapter's history operation.
4. The card receives focus, then React deletes it within the next route commit.
   The final list gallery has no pending target and leaves focus on BODY.

Desktop's session URL updates before its RouterProvider projection commits.
The outgoing template page reads the destination URL without `?template=` and
briefly mounts its own picker. The restoration adapter already resolves to
the destination `/autopilots` pathname. This temporary picker consumes that
destination's focus entry, then disappears when the actual list route mounts.

`focus-route-order.json` records the focused card under the still-present
"New autopilot" shell and proves that exact node is disconnected 100ms later.
`focus-blur-stack.json` identifies React `commitDeletionEffectsOnFiber`, with
no explicit `HTMLElement.blur()` call. A return within the creation-picker
pathname keeps the same focused node connected. These observations separate
the product lifecycle gap from the screenshot/keyboard driver.

## Bounded repair and regression

The template page should not mount a picker or configuration for a pathname
outside its own flow while a platform navigation is committing. Keep all hooks
unconditional, then guard the rendered flow using the existing trusted
`paths.newAutopilotTemplate()` pathname. This prevents the outgoing page from
consuming destination state without changing the navigation contract, store,
global focus ownership, or the one-shot restoration adapter.

The canonical page regression must stage the two updates independently: first
publish the destination URL while the old page remains mounted, then replace
it with the real destination catalog. Assert that the handoff survives the
intermediate stage and that the final card receives focus with category and
scroll retained. Existing same-path Back, dialog-owner and success/failure
creation regressions remain applicable.

Native confirmation must wait through the final route commit and assert that
the focused card remains connected, in addition to checking activeElement.
The native toolbar's own focused Back control is an intentional other focus
owner; it must retain focus rather than being overridden by gallery restore.
