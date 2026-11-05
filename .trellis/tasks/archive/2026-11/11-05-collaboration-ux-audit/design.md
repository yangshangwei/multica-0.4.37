# Repair design

## Boundaries
Use the existing shared Web/Desktop views and semantic UI primitives. Core continues to own protected queries, capabilities and recovery identities; Server behavior and native platform wiring remain unchanged. Native mobile has no existing collaboration screen and receives an explicit not-applicable audit entry.

## Direction
Operate mode: compact, calm task UI with a clear next action, visible progress and predictable recovery. Retain the existing sans typography, surface tokens and primary/secondary action model. Make loaded search scope explicit; keep metadata secondary; use responsive stacking and natural line wrapping. Do not add decorative metrics, gradients, dashboard cards or a new navigation model.

## Planned contracts
1. Initial auxiliary query failure is not empty success. Context and file evidence expose loading/failure/retry without leaking denied data. Retained original requests remain discoverable under all enclosing disclosures.
2. An explicit action preparing a result from a comment must reveal the resulting form, focus a useful field, and return focus appropriately on cancel. It never submits a result automatically.
3. Workbench counts distinguish loaded and total rows, search matches and pending reads. Clear search retains list/view context. Pagination remains explicit; no speculative server search API.
4. Shared focus/touch/wrapping rules apply to account, source and portalled collaboration controls through existing styles/components; no global redesign.
5. Admin status labels use localized product labels rather than raw enum values. Metadata-only administrative access must remain separate from ordinary issue reads.

## Compatibility
No API or storage schema changes. All current URLs, source/notification anchors, immutable receipt keys, permission fences and strict result acceptance remain intact. English and Simplified Chinese strings are updated together.

## Review and rollback
Separate implementation ownership by component family. Review source and live evidence after a complete edit batch; repair measured defects together and confirm. Revert only this task's file changes if necessary; never reset the working tree or include unrelated task/runtime work.

6. Recipient discovery must fence debounce/placeholder candidates and filter pinned accounts for a nonempty query. Never commit a stale identity with Enter or pointer.
7. Date formatting follows the chosen app locale using the existing useLocale hook.
