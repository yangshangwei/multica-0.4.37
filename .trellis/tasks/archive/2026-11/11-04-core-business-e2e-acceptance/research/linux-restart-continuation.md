# Linux restart diagnostic continuation

## Change boundary
The prior original Linux full-Main run failed with only a finally close timeout. These four test files now preserve the original error and independently attempted cleanup errors, and capture payload-free transport/phase events. No product source, installed artifact, frozen checkout, retry count, or timeout value changes. Earlier dirty fixture changes remain intact.

- `collaboration-installed-main.ts`: fixed-schema timeline projection, client command start/result/timeout metadata, immediate controller event collection,512-event cap, failure-preserving cleanup helper. Native close attempts each independent cleanup and retains original command errors.
- `.mjs`: stdout stage records are flushed before restart close/launch/refresh/metadata waits. Stop-daemon/application shutdown are separately observable. Commands remain serial and use the original execution methods.
- `collaboration-installed-main.spec.ts`: primary error retained before status capture; safe timeline attached before that capture; disconnect, native close and final timeline attachment all attempted. Multiple errors use AggregateError with the first original error as cause; a sole error keeps its original object.
- `.node-test.mjs`: adds failure ordering/all-cleanups regression plus actual protocol-reader event/timeout and secret-projection checks.

## Event privacy and interpretation
Only numeric command id, allowlisted command name, fixed phase/outcome, nonnegative monotonic elapsed milliseconds and source are retained. Input, response, URL, account identity, error message and arbitrary event fields are excluded from the timeline. Unknown fields are dropped; invalid values reject the event. Client and controller clocks have different origins; order events within a source and correlate by command id. The first512 events are retained and an explicit truncation flag reports overflow.

The controller flushes each start event before awaiting the potentially blocked operation. A restart that never resolves therefore still leaves `restart.close started` (or the later entered stage) in the parent's continuously consumed stdout. The parent's command timeout remains120s. Capture20s, close30s and existing termination budgets remain unchanged.

## Read-only hypotheses ranked
1. **Product quit cleanup waits on a stalled daemon lifecycle queue.** Frozen `apps/desktop/src/main/daemon-quit.ts` synchronously prevents before-quit and awaits `runExclusive`, even when autoStop=false; only afterward does it call quit again. If this is the blocker, the next record ends at restart.close, with no restart.launch. A targeted product lifecycle observation would then distinguish queue work from preference loading.
2. **Old Main exits but new Main startup/firstWindow waits.** The original controller records no intermediate stage, and the120s outer timeout covers both close and launch. If this is the blocker, restart.close completes and restart.launch starts without completion. Investigate single-instance/profile/process lifetime only on that evidence.
3. **New app launches but account restoration/refresh never reaches the expected issue.** Prediction: restart.close and launch complete, refresh starts and stalls or fails. Then inspect authenticated window state and real route/request events, not daemon shutdown.

Playwright1.58.2's local Electron custom close handler invokes app.quit and waits for its app-close event. This supports the relevance of before-quit lifecycle semantics; it does not prove hypothesis1. The code does not replace app.close with app.exit, kill arbitrary processes, or increase the deadline.

## Validation state
After root explicitly confirmed H1 release, all eleven focused Node checks passed (zero skipped/failed); syntax, scoped diff check and one-case Playwright collection passed. No browser test body, database, service, build or Provider operation ran. Commands executed:

```text
node --experimental-strip-types --test e2e/fixtures/collaboration-installed-main.node-test.mjs
node --check e2e/fixtures/collaboration-installed-main.mjs
```

Before runtime execution, inspect the exact patch/hashes in `linux-restart-continuation.json`, independently review the privacy/exception behavior, copy only the approved files to the frozen harness, refresh its source proofs, and run one new-numbered original Linux journey on a fresh task database. Raw first failures and completed historical acceptance results stay unchanged.

Independent review identified two controller-side loss paths. Both are now covered by actual-function tests: a failing event write retains the earlier operation error as AggregateError cause; serialized controller aggregates retain member stacks through the existing credential scrubber. The original nine-test/V1 evidence is preserved, and the two-file increment is separate because root already applied V1 to the frozen checkout.
