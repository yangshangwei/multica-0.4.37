# Workbench independent check

## Fixed

1. `packages/core/api/client.ts` imported cancellation schemas from an effectful
   module that reimported the API barrel. The cycle initialized `ApiError` as
   undefined in ordinary partial-import consumers. Extracted only schemas and
   pure receipt parsing/matching to `collaboration/command-schema.ts`; public
   collaboration exports remain intact. The unchanged MCP/autopilot tests
   reproduced four failures before and passed all 75 afterward.
2. Cancellation and Switch recovery guarded account scope even when issue access
   was still considered authorized. A fresh HTTP 403 therefore left protected
   issue/proposal caches visible. Recovery now uses the issue guard unless access
   was already denied. Content-free own-receipt recovery remains available after
   known denial. Canonical tests confirm action/comment/control already had the
   right guard and required no implementation change. Missing/malformed receipts
   retain their original request key; cancellation/Switch GET recovery never
   sends a replacement POST.
3. The cancellation handler serialized the entire zero-valued queue receipt
   after service redaction, whereas the strict client accepts only the seven
   permitted content-free fields. This was independently identified by the
   runtime reviewer and reproduced in an HTTP regression. The handler now uses
   the existing Switch response pattern. Service tests retain actual departure
   coverage; HTTP tests pin the serialized shape. TypeScript remains strict.
4. The real Web test seeded an empty AI target roster, which the actual mention
   catalogue correctly rejected as a corrupt configuration (503). Replaced only
   that fixture value with a valid historical roster referencing unavailable
   synthetic resources. No agent/runtime/grant/executable is created. Added an
   actual catalogue preflight and retained all existing product assertions.

## Reviewed without additional changes

- Current/participating account identity, retained selected detail after handoff,
  Web authentication/setup/password guards and ambient workspace clearing.
- Private schema parsing, exact decimal versions, source-only access, supported
  protocol 5 controls independent of rollout/readiness, and public submission gate.
- Account/issue epochs, query cancellation without rollback, private draft
  retention, original operation identity and late-response isolation.
- Activity authorization and values share a repeatable-read snapshot. Handoffs
  use explicit event-to-handoff identity, actor/purpose/issue checks, and safe
  field projection; old unlinked events remain generic. Unknown payloads redact.
- Waiting cancellation, comment history/anchors, target feedback and partial
  lane/hold results stay separate from actual execution confirmation.

## Files changed by this reviewer

- `packages/core/api/client.ts`
- `packages/core/collaboration/command-schema.ts`
- `packages/core/collaboration/command.ts`
- `packages/core/collaboration/command.test.ts`
- `packages/core/collaboration/index.ts`
- `packages/core/collaboration/switch.ts`
- `packages/core/collaboration/switch.test.ts`
- `packages/core/collaboration/control.test.ts`
- `packages/core/collaboration/comment.test.ts`
- `packages/core/collaboration/operation.test.ts`
- `server/internal/handler/collaboration_commands.go`
- `server/internal/handler/collaboration_commands_test.go`
- `e2e/collaboration-workbench-web.spec.ts`

Existing implementation changes outside these edits were preserved. The only
extraction is the cancellation schema leaf; no generic guard, public schema,
dependency, permission or ordinary-test mock was loosened.

## Verification

- `workbench-check-cycle-before.log`: original ordinary regression, 4 failed / 71 passed.
- `workbench-check-cycle-after.log`: same unmodified suites, 75 passed.
- `workbench-check-cancellation-before.log`: 1 failed / 14 passed (fresh recovery denial).
- `workbench-check-cancellation-after.log`: 15 passed.
- `workbench-check-recovery-before.log`: 1 failed / 29 passed (Switch recovery denial).
- `workbench-check-recovery-after.log`: all five helpers, 45 passed.
- `workbench-check-core-tests.log`: 33 files / 655 tests passed before the final
  Switch-only repair; the affected helpers were rerun as above.
- `workbench-check-views-tests.log`: 7 files / 68 tests passed.
- `workbench-check-web-tests.log`: account platform, 6 tests passed.
- `workbench-check-lint.log`, `workbench-check-typecheck.log`: core/views/Web
  passed. Views retains 27 existing warnings outside this lane.
- `workbench-check-core-lint-final.log`, `workbench-check-core-typecheck-final.log`:
  passed after the helper repairs.
- `workbench-check-handler.log`: isolated PostgreSQL + guarded Go race; five
  protocol/control/history/handoff handler/router tests passed, no skips.
- `workbench-check-history-command.log`: six service/handler history/cancellation
  tests passed, no skips. Includes actual departure and content-free own receipt.
- `workbench-check-redacted-before.log`: the new HTTP wire-shape regression
  failed with the unwanted zero-valued execution fields.
- `workbench-check-redacted-after.log`: four guarded Go race tests passed, no
  skips; includes cancellation/Switch HTTP shape and actual service departure.
- `workbench-check-handler-vet.log`: handler static analysis passed; gofmt and
  the owned-file whitespace diff check passed.
- `workbench-check-web-run1.log` and its screenshot/trace: preserved initial Web
  failure from the corrupt test roster, after successful cancellation recovery.
- `workbench-check-web-run2.log` / `.json`: one Chromium test passed in 7.0 s,
  no retries/skips. Actual account scope, old-comment focus, original-key recovery,
  mention/comment, handoff/history, new-handler access, retained participation,
  narrow layout, revocation, workspace-header isolation and no browser errors.
- `workbench-check-web-provenance.json`: reviewed API PID 92118 / port 52649,
  production Web PID 92705 / port 52650, build `5vA9q4KPddaHoty4Os0St`.
- `workbench-check-web-verdict.json`: scoped visual/functional pass with inspected
  1280x720 and 390x844 PNGs. No pixel-fidelity score: no reference image was given.

## Remaining scope

`workbench-gaps.md` remains the explicit P6 product boundary. No target/consent/
Switch authoring, attachments, creation, full participant/comment management,
source-board integration, account WebSocket delivery or P7 clusters were added.
Broader P1–P8 / 97-criterion acceptance remains unclaimed. No commits or service
restarts were performed by this lane.
