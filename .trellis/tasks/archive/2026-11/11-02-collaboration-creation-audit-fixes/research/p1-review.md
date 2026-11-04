# P1 static checkpoint

- Date: 2026-11-03.
- Role: native trellis-check, read-only checkpoint while the implementer completes P2.
- Context: loaded the saved native hook output and checked the task artifacts, current source, relevant Core/picker contracts and inherited source-picker behavior.
- Verdict: **No blocking R1/R2 defect found in static inspection.** This is not the final quality or browser verdict.

## Findings (fixed)

None. Product, specs, task lifecycle, preview and other-session files were left untouched; this review record is the only write.

## Findings (not fixed)

No P1 correction is requested from this checkpoint.

The final pass must include two pending checks, rather than treating this static checkpoint as completion:

- R3: verify actual compiled opaque focus ring/outline colors and clipping in both themes, including portaled account/parent search and exact-lookup controls. The main session owns this browser evidence.
- R5: the P1-stage option uses `ariaLabel={account.name}`. This overrides descendant text, so the full-ID identity supplement must also enter the option's label or description when P2 is finished. At review time the corresponding P2 regression is intentionally red; this is an implementation watchpoint, not a P1 blocker.

## Static evidence

| Contract | Current implementation assessment |
|---|---|
| Scope-limited mutation lock | `creation-form.tsx:57` derives the current Server/API/viewer creation prefix. `useIsMutating` also requires the expected key length and terminal `create`/`recover`; ordinary issue, options and other-account mutations do not match. `delivery-access.ts:7` confirms the creation key shape. |
| Success retirement interval | `creation-form.tsx:60` keeps the source locked while the mutation is pending even after Core removes its operation/form bundle. Core still owns retirement in `creation.ts`; no view-side acknowledgement or duplicate receipt owner was added. |
| Uncertain and content-free recovery | `creation-form.tsx:53` includes contentful creation and upload records in the lock. Idle `input:null` creation records are excluded from that durable lock, while the parent original-key GET is a matching pending recovery mutation. The source selector is disabled and its change handler rejects updates when locked (`:86`). |
| Existing account/source fences | The form retains its unmount `active` ref and `isCollaborationScopeCurrent` success guard. The parent recovery retains its current-scope guard. Core retains its actor/request/source correlation and access-epoch checks. `SelectedCreationSource` still rejects retained options on `isCollaborationDenied` and preserves only current-key Query data after a transient error (`:118`). |
| Stable option ownership | `account-picker.tsx:68` uses a per-instance `useId`, and option IDs combine that listbox ID with the normalized account identity (`:133`). Both filtered and sectioned lists retain that named ID; multi-select is explicit (`:146`). IDs are not index-based. |
| Selection and disabled semantics | Opted-in `PickerItem` retains its native button and gains option/selected/disabled attributes. `aria-selected` follows the committed account selection, not arrow highlight. Disabled native buttons are excluded by the existing candidate selector. Non-opted-in callers retain their old roles/tab behavior. |
| Active descendant synchronization | `property-picker.tsx:169` updates the active ID only for an actual option owned by the supplied listbox. The same paint/sync paths clear it for invalid index, external replacement/removal, empty results and close. The primary textbox exposes controls/active descendant only while open (`:282`). |
| Keyboard and paging | `property-picker.tsx:224` retains the IME/Safari-229 guard, committed-DOM candidate sync, typed-search precedence, unique fallback suppression and nearest scrolling. Account `navigationResetKey` stays stable for append-only pages, while DOM identity comparison still invalidates removals/reorders. Footer/retry buttons remain outside arrow candidates; persistent search focus is restored before ephemeral footer actions disappear. |
| Inherited source-picker changes | `searchMaxLength`, the parent popup, selected-parent identity, local pending/error/retry/empty feedback and retry-input ref remain. The protected `optionsError`/retry/placeholder wiring and denial/transient fallback were not rolled back. |
| Focus scope | `form-styles.ts` uses the existing semantic `faint-foreground`, overrides focused descendant ring/border color, and supplies native-search/option outlines through opt-in `contentClassName`. Both account and parent portals receive the local class. No global primitive/token edit is introduced by this checkpoint. |

## Coverage inspected

The current regressions name delayed creation completion, uncertain original-key recovery/rejection, idle content-free recovery and active GET locking. Account regressions name stable selected/disabled IDs, IME/Safari-229, removed/empty active options and append-only paging/footer focus. Existing generic PropertyPicker regressions still exercise external resets, typed first-match priority, early Enter, append-only preservation and non-opted-in unique fallback/ref behavior.

## Verification

- Lint: **not run**; the parent explicitly reserved the final pass until P2 is complete.
- TypeCheck: **not run** for the same reason.
- Tests: **not run by this checkpoint**. The implementer reported 9 creation-form tests and 17 account/shared-picker tests passing before beginning the intentionally red P2 regressions; the parent independently reported 43 canonical Core tests passing.
- Browser/AX/geometry/physical-device/native checks: **not run by this checkpoint**; main-session browser verification and the explicitly separate native boundaries remain.

The final trellis-check handoff must re-read current bytes after all P2 work, review the full R1–R8 scope, run the required checks and report any remaining findings with actual results.
