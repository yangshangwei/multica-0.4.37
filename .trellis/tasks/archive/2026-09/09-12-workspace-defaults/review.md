# Contract review

Independent review found five concrete gaps; all are integrated into design.md before implementation: successful project response after preparation failure, selection revision across transactions, invoke vs wire/autonomy permission boundaries, requested vs actual runtime, and handwritten project search/resources responses.

Baseline: corepack pnpm install --frozen-lockfile --offline passed using 10.28.2. Existing views tests modals/create-project.test.tsx and onboarding/onboarding-flow-mode.test.tsx: 2 files, 6 tests passed. jsdom emitted its existing canvas getContext warning; no real agent CLI ran.

## Implementation spec review
Independent review identified and verified fixes for: shared invocable squad selection, complete current roster readiness, malformed local-directory constraints, and deleted-default recovery. Project-local ordinary issue creation now uses lowest-precedence defaults with atomic assignee-pair override. Final bounded spec result: PASS.

## Code quality review
One P2 was found and fixed: a failed membership query on automation configuration must retain suggested assignee and offer retry. No backend/security finding was reported.

## Verification scheduling
The second full concurrent run produced three 5-second test timeouts (two skill dialog cases and one untouched MCP settings case), while typecheck/lint passed. A browser cold render also exhausted its test budget. Run heavy unit and browser verification separately before finalizing; do not weaken assertions.

Quality recheck: PASS. Membership retry now preserves defaults until data succeeds.

Final API compatibility self-check caught canonical UUID responses being rejected for uppercase/compact project references. Two regressions reproduced the issue; identity comparison now canonicalizes UUID text forms while keeping opaque identifiers exact. Wrong project/workspace response tests remain in place.

Browser instability root cause confirmed in the isolated Next dev-server log: it reached the used-memory threshold and restarted during project navigation. Restarted only this task environment with NODE_OPTIONS=--max-old-space-size=6144 on the 16GB host. Browser checks now wait for the actual project region as well as its URL. Production behavior/assertions are unchanged.
