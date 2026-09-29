# Task creation and core workflow E2E — 2026-09-29

## Environment and scope

Tested the current working tree, including the recent AI assist and create-dialog changes, with a real Next.js web app, Go API, and isolated PostgreSQL database. The web app used a separate build directory to avoid the developer's existing Next.js lock; shared packages referenced this working tree. No production data or running desktop instance was used.

- API: localhost:18572; web: localhost:13492.
- Database: `multica_multica_0_4_37_492`, created only for this run.
- Model boundary: deterministic OpenAI-compatible service on loopback 14592, with real backend streaming/validation. No browser API route mocks in the new tests.
- Agent boundary: real quick-create 202 and database queue verification with a seeded runtime. No daemon or installed agent CLI executed.

## Coverage

| Area | Evidence |
| --- | --- |
| Email login | Fresh browser, email form, actual send/verify endpoints, OTP from isolated DB, workspace access, reload, logout, anonymous redirect |
| Vague feature input | Weekly-report requirement, real optimize endpoint, streamed result, automatic editor replacement |
| Clarifications | 0/1/2 questions, disclosure retains answers, delegate choice, merge, undo restores answers, re-merge has no duplicate |
| Draft protection | Edits during delayed generation survive; cancellation ignores late output; invalid provider output yields error; retry succeeds |
| Manual creation | Refined content saved by real create 201, read back via API, reopened in detail |
| Agent creation | Unanswered question does not block; real quick-create 202; queued agent ID and prompt match |
| Layout | Both modes at 1280×900,390×844,667×375 with long content; create button accessible and dialog height bounded |
| Task lifecycle | UI create, rapid title/body edits, status and priority changes, API+reload persistence, comment submission+reload, cancel delete, confirm delete, API 404 |
| Existing core suite | Board/list, date filtering, creation/detail/dismiss, login/logout, inbox/agents/settings navigation, empty-comment validation |

## Results

- New AI/layout suite: 7/7 passed against the real backend.
- New email/OTP suite: 1/1 passed; no injected browser token.
- New lifecycle suite: 1/1 passed in the final independent rerun (25.4s). The final combined run passed 8/9; its lifecycle reload assertion fired before the page finished rendering. After adding an explicit page-ready assertion, that scenario passed. Thus all 9 new scenarios have passing final verification, without claiming one combined 9/9 run.
- Existing 16-test baseline: 13 passed initially. Anonymous redirect was affected by default device auto-login; after disabling it, all 4 auth tests passed. All 3 navigation tests passed after cold compilation. Empty-comment test passed on the stable environment.
- One legacy comment-submit test remains stale: its locator is tied to placeholder text that changes when the composer activates. The new lifecycle test uses the stable active composer and verifies actual comment persistence. No existing product assertion was weakened to hide this failure.
- New TypeScript tests passed standalone strict type checking; provider passed `node --check`; `git diff --check` passed.

## Investigation notes

1. **Auth environment mismatch:** default device login created an authenticated device user without access to the test workspace, producing NoAccess instead of Login. Explicit `MULTICA_DEVICE_AUTH_ENABLED=false` restored the intended email-auth scenario.
2. **Cold page readiness:** early runs hit Next.js compilation and rendering delays. Tests now wait for the actual canonical detail page and a visible title after reload before interacting, rather than sleeping.
3. **Rapid description edit:** one early unstable run had no description PUT. Two subsequent stable runs persisted rapid title/body edits. No reproducible production cause was established, and product save logic was not changed.
4. **Legacy comment locator:** placeholder-based selection no longer reliably identifies the expanded composer. New coverage selects the active composer through its Send control.
5. **Responsive transitions:** dimensions are checked after the 300ms transition settles, using retrying assertions rather than a fixed delay or weakened size limit.

## Reproduction and artifacts

New tests:
- `e2e/issue-assist-flow.spec.ts`
- `e2e/task-lifecycle-qa.spec.ts`
- `e2e/auth-qa.spec.ts`

Provider and run instructions: `e2e/support/README.md` and `e2e/support/assist-provider.mjs`.

Local logs, screenshots and failure traces are under `.omx/qa/e2e/`, including `final.log`, `core-final.log`, `auth-baseline.log`, `baseline-warm.log`, and the corresponding result directories.

## Limits

This validates product plumbing and persistence, not the reasoning quality of a live commercial model. Real agent task execution, native Electron window behavior, and a physical phone's software keyboard were not exercised. The entire repository's unrelated E2E suite was not run.
