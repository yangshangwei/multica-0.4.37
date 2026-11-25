# Verification and handoff — 2026-11-25

## Result

The approved categorized issue-detail mention picker is implemented in shared Views. Web and Desktop use the same opt-in implementation for comments, replies and comment/reply edits. Product source review has no remaining blockers. The user subsequently authorized commit and push; work commit is `f0a64fdf33a276b1411a7b7fe395908ed58c3dd9` on main.

## Changed files and reuse

- `packages/views/editor/extensions/categorized-mention-list.tsx` and its test: categorized directory/query lifecycle, typed-identity selection, scoped favorites, mention history, local expansion and focus-safe controls.
- `packages/views/common/actor-discovery.ts`: moved existing pure discovery helpers out of the creation-only model; creation callers import directly and preserve their selection/default/history semantics.
- `packages/views/editor/extensions/mention-ranking.ts`: shares existing issue conversion/ranking without a runtime component import cycle.
- `content-editor.tsx`, `extensions/index.ts`, `mention-suggestion.tsx`, `suggestion-popup.tsx`: explicit default-off opt-in and composition lifecycle. No backend or serialization changes.
- Comment/reply/edit hosts, their regressions, English/Chinese editor copy, and the focused Web/Electron fixture/suite.
- `.trellis/spec/views/frontend/issue-mention-picker.md` plus index: discovery, authority, data subscription, focus and testing contracts.

No new dependency, database migration or server change. Shared helper extraction removes duplicate search/ranking logic; other default/context mention and quick-create surfaces retain their behavior.

## Automated evidence

| Check | Observed result |
| --- | --- |
| Approved focused Views suites plus new categorized/host regressions | 10 files, **188 tests passed**, 15.09s |
| Views typecheck | Passed, including final member-observer/platform-hint follow-up |
| Views ESLint | Passed with 27 existing warnings; final affected-file lint had no findings |
| Web typecheck | Passed after production build completed |
| Desktop node + renderer typechecks | Passed |
| Production Web build | Passed; BUILD_ID `JxxRCtbumHw7PxRS_uwn4` |
| Dedicated desktop renderer/preload build | Passed using electron-vite in task-owned output |
| Playwright final suite | **3 passed**, 27.5s, one worker / zero retries |
| Task context validation + diff whitespace check | Passed |

Final Playwright command: `bash .omx/state/issue-mention-picker/verify.sh test --output=test-results/mention-accepted-final`.

Web English and Chinese tests exercise 550 synthetic actor rows, category/leader filters, paging beyond 20/50, full-description and pinyin search, member/issue targets, pointer filtering followed by uninterrupted typing, Tab/Enter insertion, Alt/Option+Down controls, Escape/reopen, persisted Markdown identity, unchanged creation preferences and unsent comments. English also verifies reply and both edit hosts. Native Electron executes the same discovery flow using the actual built desktop renderer/preload and a dedicated profile, with daemon starts and updater installs asserted zero. All three cases assert no server mutations from picker actions and no collected renderer/API errors. Intentional draft and mention-recency writes remain enabled.

The last test-only hygiene pass removed newly authored inline comments and expanded the local-directory unit test to browse every candidate. It does not change runtime source. See the follow-up result in progress.md.

## Environment provenance

- Separate registry environment `check-20261125123344-7383`, migrated isolated database `multica_check_20261125123344_7383_api`.
- API `localhost:18607`, health commit `a7bf6ba70`; product changes are frontend-only.
- Production Web `localhost:13527`, launcher46194/listener46219, source fingerprint `4e415e9385c7e7a6d80ea80849290dd09fb6f88143e237e0d0f59f5bf6177e9c` at build/start validation.
- Dedicated native renderer `127.0.0.1:5701`; index SHA256 `cc415240bf6307cf65e89153e8bd0e07f08ef91d71f1ed2ff075615421fa4868`.
- Private configuration and full service/build logs remain under `/Users/artisan/.multica/dev/envs/check-20261125123344-7383/`. Do not publish the secret-bearing env file.
- All three task-owned services were stopped after final acceptance. Synthetic workspaces and native profiles were removed by the fixture. The task database is retained under the allocated TTL environment.
- User backend18572 and desktop5666 still respond successfully; their processes were not stopped for QA.

## Review and visual evidence

Independent `trellis-check` review closed five concrete defects: removed-selection resurrection, page-scope resurrection, unrelated retry losing issue matches, missing active-descendant semantics, and unobserved member data updates affecting candidate permissions. The last one has a real QueryClient + actual mention factory regression. Source review and limitations are in `research/check-review.md`.

Inspected captures include English light, Chinese dark, 1280px/375px, and native full-list/filtered states under `research/screenshots/`. Fixed category/footer controls, leader filter, responsibility previews and explicit remaining count fit the available popup height. Visual verdict is recorded at `.omx/state/issue-mention-picker/ralph-progress.json`.

## Verification corrections and limits

- Test setup initially asserted serializer attributes that the React NodeView does not expose. The final suite checks the visible chip and persisted Markdown typed identity instead.
- Breakpoint screenshots reload at the target viewport, avoiding an unrelated responsive host remount race while retaining separate pointer/focus interaction tests.
- Native route.fulfill responses produced unusable status0 in this Chromium/fixture combination. Synthetic directory responses use a loopback HTTP fixture, with the same payloads and actual renderer requests. Web uses its original fulfill path; wildcard CORS headers are excluded because the Web API client includes credentials.
- Read-only POST issue-query endpoints are allowed through the test guard; actual comment/reply/edit writes remain blocked and counted.
- An early Web typecheck overlapped a build replacing `.next/types`; sequential rerun passed. Final checks do not use that failed concurrent run as evidence.
- IME ownership is covered by real-editor synthetic composition/keyCode229 tests. Native OS Chinese candidate-window behavior and screen-reader spoken output were not manually verified. No Windows/Linux native run or cross-device favorites claim is made.

## Task lifecycle

Implementation and verification are complete. After the verified handoff, the user explicitly requested commit and push. The functional work commit precedes separate task-archive and journal commits. This task was never PR-backed and landed directly on main, so archival uses the documented `--skip-branch-validation` exception; no fictitious feature branch is recorded. The archived task retains all planning, review, verification and screenshot evidence.
