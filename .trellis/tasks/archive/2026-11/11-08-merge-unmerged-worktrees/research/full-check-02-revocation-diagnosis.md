# Full-check-02: P1-L14 revocation diagnosis

Initial status: awaiting final Playwright failure stack; read-only triage, 2026-11-08T12:18:45.133429+08:00.

Candidate c52eeb53f637f1f5f3dc19ba5d1ac51b974fb56c is still being checked. No candidate files changed, tests/builds launched, or fixture writes performed by this reviewer.

## Observed evidence

- Full log line8767: item232, `projects-p1-lifecycle.spec.ts:286`, P1-L14, failed in2.2s. Final error stack has not yet been emitted. The suite has441 cases and is still progressing.
- Adjacent P1-L16 member leave and owner removal cases passed, each including20 seconds of bounded-traffic observation. This supports the HTTP denial/teardown path working in those separate cases; it does not prove L14 draft erasure.
- The sole disk screenshot for L14 is blank. `p1Failure` runs in the test catch for the member page, then the fixture finally runs it for the unused default owner page, using the same `info.outputPath("failure.png")`. The second capture overwrites the useful first image. Inline `failure-state` and `page-errors` attachments may still appear in the final reporter output. There is no on-disk error-context or trace for this case at this reading.
- Failure duration is shorter than default retry assertions/timeouts. An immediate protocol/evaluate/response read exception or immediate status assertion is plausible; no exact root cause can be assigned until the stack identifies the failed statement.

## Static flow

The test stores the member draft, leaves via real API expecting204, subscribes to an overview404 response, clicks Refresh, reads its JSON code, then checks composer, goal, draft DOM and persisted localStorage erasure. Access handling uses `protectProjectRequest` to classify `workspace_access_denied`, marks workspace denial before removing queries, clears progress/description drafts, and causes ProjectDetail to render its permission-lost branch. Request failure and mounted local draft cleanup have existing unit coverage. None of these observations waives the failing browser case.

## Baseline/candidate source identity

```json
{
  "e2e/projects-p1-lifecycle.spec.ts": {
    "e541cfb43": "f492738b88c137fa42a3106f9332b9f8c58301fb",
    "c52eeb53f": "f492738b88c137fa42a3106f9332b9f8c58301fb"
  },
  "e2e/fixtures/project-p1.ts": {
    "e541cfb43": "8614fa5b7268458265428acb81a9d7356c7046b9",
    "c52eeb53f": "8614fa5b7268458265428acb81a9d7356c7046b9"
  },
  "packages/core/projects/access.ts": {
    "e541cfb43": "8cb9c9ca024c90e89506f2cfe66d0a4a4b8f4cf0",
    "c52eeb53f": "8cb9c9ca024c90e89506f2cfe66d0a4a4b8f4cf0"
  },
  "packages/core/projects/progress-draft-store.ts": {
    "e541cfb43": "eb2737440c1f04584bd96eb7fad7f1fa73329e80",
    "c52eeb53f": "eb2737440c1f04584bd96eb7fad7f1fa73329e80"
  },
  "packages/views/projects/components/project-overview.tsx": {
    "e541cfb43": "350a566b962ca8d28ba1b326ba6c2e80d338f18e",
    "c52eeb53f": "f64f5efc4047af473e360c7cecec33d50ab82371"
  },
  "packages/views/projects/components/project-progress.tsx": {
    "e541cfb43": "428efc981720bfd05903be700d5328840ff7bba1",
    "c52eeb53f": "e8742cf54b326910b4f3ee8e2891747bc54cd11d"
  },
  "packages/views/projects/components/project-detail.tsx": {
    "e541cfb43": "6a65f379b11452e987f0a0c8d429f3257902f414",
    "c52eeb53f": "37ae259c8dffdf42b6c9f1e21e8b7f83276c4cf0"
  }
}
```

## Next diagnostic step

Read the final original failure stack and inline attachments after the running suite finishes. Preserve original failure evidence before any later runner overwrites test-results. Only then select a focused reproduction on an owned environment; no concurrent replay while the complete run is active. Product changes must wait until the candidate run ends. Do not replace browser erasure assertions with an API-only pass.

## Final original stack and confirmed root cause

Run02 finished12:24:19 with exit2. Original log lines9538–9605 identify the cause:

- Primary error at lifecycle spec line300: strict-mode violation. The unscoped exact `Refresh` button locator matched both the Overview content control and a sidebar Refresh control.
- The intended Overview click never occurred. Thus this original failure did not reach the HTTP denial/draft-erasure assertions; it is not evidence that the product leaked a revoked draft.
- The response waiter then rejected when finally closed the member context. That secondary `Target page, context or browser has been closed` error is an effect of the failed click, not the first fault.
- Two failure-state attachments confirm the useful member project page capture followed by `about:blank` from the default fixture page. Both screenshot attachments refer to the same overwritten image path.

The competing sidebar control is conditional, explaining why the adjacent no-draft L16 cases could pass with the same originally broad locator. Do not infer randomness in authorization from those results.

## Authorized mechanical repair

After run02 completed and its artifacts were copied, main authorized editing only integration `e2e/projects-p1-lifecycle.spec.ts`. L14 now scopes Refresh to the section containing the exact Overview heading, matching the existing production DOM in `project-overview.tsx`. It does not select by ordinal or force clicks. The wait and click share Promise.all so either failure is observed and a later context close cannot produce an unhandled waiter rejection.

All original assertions are retained: real204 leave, exact project overview404 and workspace_access_denied body, composer removal, protected goal/draft disappearance, and persisted draft erasure. No product source, permission rule, timing budget, retries or skip changes. The fixture screenshot overwrite is documented but unchanged because it lies outside this narrow repair.

`git diff --check -- e2e/projects-p1-lifecycle.spec.ts`: pass. No browser test/build was run by this reviewer; main owns the focused production-Web replay and new complete gate. This diagnosis and test fix do not retroactively pass run02.
