# Questionnaire save order and session fencing

The final browser run reproduced a real persistence race. Trace requests show the Engineer snapshot with `use_case=[]` started at `17:47:02.197Z` and took 128.715 ms; the later `ship_code` snapshot started at `.250Z` and took 48.803 ms. The newer choice committed first, then the old full questionnaire overwrote it. Local UI still showed the checked choice. The runtime-skip branch completed without flushing the latest answers.

The server replaces the complete questionnaire JSONB; sending partial field patches would lose other fields. The fix stays in the existing frontend flow and save boundary:

- Apply local answer changes at the event boundary, with no request side effects in React state updaters.
- Serialize complete snapshots through one flow-local promise chain, preserving both same-field and different-field selection order while keeping interaction immediate.
- Both runtime completion paths drain prior snapshots and confirm the final snapshot before completion. A final save failure keeps the flow available for retry.
- Capture API session/user identity before a queued write or completion flush; refuse work after identity replacement. The core save response also checks that identity before writing the refreshed auth user or analytics.

Changed files: `packages/views/onboarding/onboarding-flow.tsx`, its mode/completion tests, `packages/core/onboarding/store.ts` and new `store.test.ts`. No backend API, schema, dependency or test assertion was weakened.

Deterministic evidence under `/Users/artisan/.multica/qa/core-20261108/onboarding-repair/`:

- `trace-questionnaire-order.json` retains only questionnaire fields and timing, plus original trace SHA.
- `red-2.log`: four regressions failed on the original flow — concurrent snapshots, StrictMode duplicate request side effects, and both completion paths racing pending saves.
- `green-3.log`: all 29 view cases passed, including final-save failure/retry and preventing queued writes under a replacement session.
- `session-guard-red.log`: both late-response identity cases failed on the original core save; `session-guard-green.log`: all 3 core cases passed.
- Targeted ESLint found no errors; one unchanged pre-existing `handleWelcomeSkip` dependency warning remains.

The independent auth reviewer found no blocking issue in the queue/final-flush/session-fence increment. The parent owns combined checks, production rebuild and original browser replay; component results alone do not establish end-to-end acceptance.

Combined auth/onboarding verification after reciprocal review: 22 core tests across 2 files and 49 view tests across 5 files passed (`combined-core.log`, `combined-views.log`). Final targeted lint for the five onboarding files passed with no errors and the single unchanged pre-existing missing-`t` dependency warning.
