# Independent implementation review

Verdict: **APPROVE**

Reviewed the phase 2 production changes in `server/internal/handler/issue_creator_recommendation.go`, the `loadedInvocationDecision` extraction in `agent_access.go`, `packages/views/modals/creator-recommendations.tsx`, `packages/core/issues/creator-selection.ts`, the `defaultActor` store additions, and the project/default wiring in the actor picker and agent create panel. Supporting inspection included API/schema/mutation changes, router/authentication, the installed TanStack mutation implementation, and focused existing test coverage.

## Resolved finding

- **Mutation lifetime:** `creator-recommendations.tsx` originally cleared only local component state. TanStack keeps an observed mutation's variables and response even with `gcTime: 0`, retaining the submitted text after suggestions were dismissed. The final code resets the mutation observer on clear/cancel, unmount, and current-request settlement. Settlement is guarded by request identity, so an older request cannot reset a newer one. Inspected the final fix; the leader also reports a real QueryClient regression test that failed before the fix and passes afterward.

## Review evidence

- The server rejects agent/task-token principals before catalog/model work, requires workspace membership, filters active runtime-bound user agents using invocation permissions, and admits squads only through eligible leaders.
- Model candidates contain temporary references, actor type, and saved-description excerpts. Output accepts at most three unique known references with exact nonblank evidence bounded to 240 runes. No entity writes or model tools are introduced.
- After generation, membership and the eligible catalog are read again. Changed descriptions and newly ineligible objects are dropped before returning evidence.
- The client captures the request's scope, text, project, actor, identity, and descriptions. It aborts invalidated requests, ignores stale responses, and rechecks current eligibility and unchanged descriptions before explicit adoption.
- Creator seeding waits for unresolved higher-priority candidate data and preference hydration. The final helper skips malformed actor types/IDs. A valid current choice survives later query/default/project changes.
- Project candidates derive from configured execution squads and intersect the eligible catalog. Shortcut groups deduplicate and remain within the eight-item budget; full project browsing remains available.
- Default/footer navigation, cancellation, dismissal, and successful recommendation adoption move focus to surviving controls before removing their initiating controls.

No unresolved correctness, security, or regression findings were identified in this bounded review.

## Verification limits

This was a source review; no product files were edited, no tests were run by this reviewer, and no independent browser/visual verification was performed. The leader owns final test execution and reports the targeted frontend and backend suites passing. This approval is limited to the phase 2 scope above, not the unrelated changes present in the worktree.
