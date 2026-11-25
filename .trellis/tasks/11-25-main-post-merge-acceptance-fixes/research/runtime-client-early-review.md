# Research: Early runtime, skill import and client regression review

- Query: Inspect actual diagnostic/skill changes for semantic or safety regressions, and client changes for weakened business success conditions.
- Scope: internal; early read-only source review, not final integration verification.
- Date: 2026-11-25
- Worktree: `/Users/artisan/.codex/worktrees/main-acceptance-fixes/multica-0.4.37`
- Comparison: corresponding files in the original read-only `/Volumes/artisan/code/2026/multica-0.4.37` checkout. Files changed during review; findings below reflect the last inspected form, including fixes already received.

## Findings

### C1 — P1: Explicit issue reselection still bypasses remembered selection

**Client implementer:** `e2e/fixtures/collaboration-detail-journey.ts:825–826` now clicks Participating and then explicitly clicks the known issue from the account list. The earlier `goBack()` substitution has been removed, but this replacement still weakens the original scope-restoration condition. The original helper switched away/back and required the previously selected issue to reappear without selecting it again. If the application loses the selected issue during the switch, the new explicit issue click supplies that missing selection and can still pass the title/ID/scroll assertions.

Preserve the assertion that the selected issue is restored **before** any explicit issue click. Use the actual current product resume operation if it exists. If current navigation deliberately offers only a fresh list/explicit open, report that the old restoration contract no longer has an equivalent UI path; do not silently claim the new list-selection journey closes the original restoration coverage. A separate explicit navigation test may be useful, but it does not replace this assertion. Keep the scroll and single-account-tab checks.

This distinction is documented in `.trellis/spec/core/frontend/global-scope-navigation.md:38–46`, `:53–67`: remembered scope destinations, explicit links and Back/Forward are separate operations; the Collaboration heading now only discloses links. The finding is about lost test coverage, not proof of a product restoration bug.

### R1 — P2: A canceled probe can be reported as an invalid helper response

**Runtime implementer:** `server/internal/daemon/collaborationrun/controller.go:325–342` has two simultaneously possible terminal paths. `exec.CommandContext` can kill the helper when the probe context expires; its ready pipe then closes without a complete response. Both `ready` and `ctx.Done()` can be selectable. If `ready` wins, `:332–333` wraps only the read/decode error and overwrites the code with `invalid_helper_response`, even though the request context is already canceled/deadline-exceeded. The returned error then loses the cancellation identity that the other select branch preserves.

Capture the ready-boundary context state **before** Stop/Wait cleanup, retain it alongside the read error, and distinguish EOF/no reply from malformed JSON and an explicit helper refusal. Do not infer which event caused the other when their ordering is unknown. A bounded case with a closed ready stream and already-canceled context should retain `errors.Is(..., context.Canceled)` and avoid claiming a malformed helper message. Keep `ErrUnsupported` compatibility and fail-closed availability.

This is a statically reachable diagnostic ambiguity, not a reproduction of D05's historical cause. It matters because the new diagnostics are intended to distinguish environmental failure from cancellation.

### R2 — P2: Any stdin read error is labeled EOF

**Runtime implementer:** `server/internal/daemon/collaboration_host_router_test.go:340` discards the error from `io.Copy(io.Discard, os.Stdin)` and unconditionally records `stdin_eof` before canceling. `io.Copy` returns nil for normal EOF; an actual read failure also takes this branch. That turns an unobserved shutdown cause into an asserted EOF in the evidence used to attribute permit cancellation.

Keep cancellation behavior, but record `stdin_eof` only on nil error and a fixed content-free event such as `stdin_read_failed` otherwise. Do not stringify the error. A reader returning a synthetic error is enough to verify the distinction; no runtime policy or timeout change is needed.

## Already resolved or no additional actionable finding

- **Nil Unwrap entry is fixed.** `controller.go:123–128` returns only `ErrUnsupported` when the cause is nil. `controller_test.go:58–68` includes a nil-cause case and rejects nil unwrapped entries. Do not reopen the earlier version as a current finding.
- **Main heading OR is fixed.** `e2e/fixtures/collaboration-installed-main.mjs:528` now requires exactly `Current handling`. `packages/core/paths/account.ts:42` and `:81` default to `current`; the native scenario next opens an explicit `view=current` destination at `e2e/collaboration-installed-main.spec.ts:195`. The fresh-profile login's previous allowance for either Current handling or Participating is no longer present.
- **No additional actionable issue found in the inspected `skill.go` delta.** `gitHubAPIResponseError` emits fixed status/reason text, validates rather than echoes Retry-After, and does not include response bodies, cookies, arbitrary headers or request IDs. Bare403 stays unconfirmed. Tree failure still produces no imported skill, preserves `errImportSourceUnavailable`, and does not add retries or fallback. Existing ref-resolution sentinel behavior is untouched. This is a source-review conclusion, not a live GitHub result.
- The new Host transport classification uses fixed labels and keeps status0 separate from a received503. Monotonic timings are process-local; parent spawn/ready observations provide bounds, not exact clock synchronization. No new arbitrary transport-error text is exposed in those fields.
- Namespace logging calls the sanitized `NamespaceProbeError.Error()`; helper error codes are restricted to known names and canonical numeric errno suffixes. The retained wrapped cause is not printed by the new logger. No newly introduced secret exposure was identified in those inspected paths.

## Files found and related contracts

- `server/internal/daemon/collaborationrun/{controller.go,controller_test.go,controller_linux.go}` — new probe diagnostics, nil-unwrapping test and actual helper refusal/exit protocol.
- `server/internal/daemon/collaboration.go:154–170` — capability admission and safe probe logging.
- `server/internal/daemon/collaboration_host_router{,_boundary}_test.go` — transport facts, local event clock and diagnostic controls.
- `e2e/fixtures/collaboration-host-runtime.ts` — parent-side wait, ready, stdin and process-close timestamps.
- `server/internal/handler/{skill.go,skill_github_error_test.go}` — fixed GitHub response classifications and failure-preservation/redaction controls.
- `e2e/fixtures/{collaboration-detail-journey.ts,collaboration-installed-main.mjs}` — restoration substitution and now-exact login heading.
- `CLAUDE.md`, `.trellis/spec/server/agent-output-redaction.md`, `.trellis/spec/core/frontend/global-scope-navigation.md` — relevant state/ownership, diagnostic secrecy and navigation distinctions.
- Runtime/client/security child design documents — bounded ownership, original business assertions, allowlisted diagnostics and no unsupported causal attribution.

## Caveats / Not Found

- No product files were edited; no tests, services, Git operations or target scripts were executed. Only this report was written.
- Findings are concrete source-path issues, not claims that the original production failures were reproduced or fixed. Existing diagnostic test code was inspected, not run.
- This early review neither reopens accepted infrastructure controls nor substitutes for the later independent integration check after all lanes stabilize.
- External references: none needed/fetched; this review does not assess current external advisories or the security dependency upgrades.

## Source fingerprints at handoff

Captured 2026-11-25T10:16:48.592276+00:00.

- `server/internal/daemon/collaborationrun/controller.go`: `ac12e76adf0f42923fd01585e4588d6bfbac65e5021993754da5b2fe4af5c891`
- `server/internal/daemon/collaboration_host_router_test.go`: `0ffc8f5fbfbd24ba13b9dfbf0f93afee99423c967101fff8afe8aabeca48d746`
- `server/internal/handler/skill.go`: `1fa125efadfaabce7ce98b2281cec4540183d4dfd764babfbecb189ab53c0233`
- `e2e/fixtures/collaboration-detail-journey.ts`: `1dbe1b9bed3de59fc68158cafbf122e6ed54b629fa1aca831a1f026dbdc84aae`
- `e2e/fixtures/collaboration-installed-main.mjs`: `1dfaa8d95e4b4ddf3d5758b647a34595feb6b7be8cf51674fd05cffd56480d68`
