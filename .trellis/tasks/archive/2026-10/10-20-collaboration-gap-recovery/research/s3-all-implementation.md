# S3a: issue-scoped `@all` selection

Date: 2026-10-21. Implementation slice of `10-20-collaboration-gap-recovery`.

## Change boundary

- Gap: the server already accepts `[@all](mention://all/all)`, but the pure insertion helper rejects non-UUID identities and the shared comment chooser has neither `@all` nor visible human/AI groups.
- Authority stays in the existing protected issue mention query and server preview/send checks. `@all` represents current human participants; it is not a member UUID or an AI target and does not depend on AI execution availability.
- Expected edits: `packages/core/collaboration/comment.ts` and `comment.test.ts` for the explicit syntax branch and canonical identity/authority cases; `packages/views/collaboration/comment-composer.tsx` and `comment-composer.test.tsx` for the grouped insertion, preview invalidation, suppression and final send wiring.
- The coordinator owns the three bilingual `composer.mentionPeople`, `composer.mentionAI` and `composer.mentionAll` keys, any shared specs, production Web/Electron builds and visual-verdict evidence.
- No new endpoint, query, dependency, state store, editor, invite/grant path or AI target kind. No SQL/service changes. Description/comment editing semantics remain owned by the existing server contract. Existing content/draft/attachment/reply/epoch identity and original request recovery remain intact.
- No cleanup or refactor is required. Extend the existing component happy path instead of duplicating the server notification/authority matrix in DOM tests.

## Verification plan and ownership

1. Add failing core syntax/identity regressions and extend the shared component integration test before changing production code.
2. Add the minimal special-case helper branch and labelled groups using the existing buttons/change handler.
3. Run scoped core/views Vitest, affected-file ESLint, core/views typechecks and diff whitespace checks. Record actual red/green outcomes below.
4. Production Web, actual Electron, viewport/zoom and visual-verdict are pending coordinator verification. This slice alone does not close AC-I04 or the parent AC86/90/91/97.

## Execution evidence

The new tests were written and run before either production file changed.

| Phase | Command | Actual result | Log |
| --- | --- | --- | --- |
| Red | `pnpm --filter @multica/core test collaboration/comment.test.ts` | Exit 1; 2 failed, 15 passed. The two canonical `@all` cases threw `CollaborationUnavailableError` from the old UUID-only helper. | `/tmp/multica-s3-all-core-red.log` |
| Red | `pnpm --filter @multica/views test collaboration/comment-composer.test.tsx` | Exit 1; 1 failed, 5 passed. The mixed-delivery integration could not find the `People` group. | `/tmp/multica-s3-all-views-red.log` |
| Green | `pnpm --filter @multica/core test collaboration` | Exit 0; 52 files, 817 tests passed; no failures or skips. Includes all 17 comment tests. | `/tmp/multica-s3-all-core-green.log` |
| Green | `pnpm --filter @multica/views test collaboration` | Exit 0; 15 files, 60 tests passed; no failures or skips. Includes all 6 composer tests. | `/tmp/multica-s3-all-views-green.log` |
| Static | `pnpm --filter @multica/core exec eslint collaboration/comment.ts collaboration/comment.test.ts` | Exit 0; no lint diagnostics. | `/tmp/multica-s3-all-core-eslint.log` |
| Static | `pnpm --filter @multica/views exec eslint collaboration/comment-composer.tsx collaboration/comment-composer.test.tsx` | Exit 0; no lint diagnostics. | `/tmp/multica-s3-all-views-eslint.log` |
| Types | `pnpm --filter @multica/core typecheck` | Exit 0. | `/tmp/multica-s3-all-core-typecheck.log` |
| Types | `pnpm --filter @multica/views typecheck` | Exit 0. | `/tmp/multica-s3-all-views-typecheck.log` |
| Whitespace | `git diff --check -- packages/core/collaboration/comment.ts packages/core/collaboration/comment.test.ts packages/views/collaboration/comment-composer.tsx packages/views/collaboration/comment-composer.test.tsx` | Exit 0. | Tool output; no diagnostics. |

The canonical helper cases cover locale-independent `all/all` output without AI submission authority, rejection of empty/uppercase/UUID/path aliases, and refusal to interpret `all` as a member/agent/squad UUID. Existing projection/label escaping tests remain intact.

The shared component test `groups people and AI, inserts @all without losing explicit targets, and sends the reviewed delivery` uses the real query/helper/composer stack with mocked HTTP: separately named visible groups, explicit person/agent/squad insertion, `@all` insertion invalidating an existing preview, server-projected human recipient display, no comment send during previews, per-agent suppression preserving all Markdown, one final comment, and exactly the two explicit AI target versions and original observed authority. Existing shared-draft identity, suppression/grouping, Chinese continuation and reply regressions also pass.

The coordinator supplied the actual labels `People` / `人员`, `AI targets` / `AI 目标`, and `@all · All participants` / `@all · 全体参与者`. These are rendered through the existing i18n layer; the authored mention remains exactly `[@all](mention://all/all)` in both languages. All three keys are coordinator-owned changes.

Checks ran on Darwin arm64, Node v22.23.2 and pnpm 10.28.2, against the working tree based on `8ecb98883504443a6ec1a5fb4a4c6295a985ae04`. pnpm printed the existing warning about root `pnpm.onlyBuiltDependencies`/`pnpm.overrides` configuration placement; no dependency or package-manager changes were made.

Verified working-tree SHA-256 values:

| File | SHA-256 |
| --- | --- |
| `packages/core/collaboration/comment.ts` | `8ca85a66a2a4bdfc349f6f1788f24b1ed84fd00a419d6720acd0458dc862d366` |
| `packages/core/collaboration/comment.test.ts` | `8837f9c5eca6df789ddd662033443b110696cd4f2d265948bece84d22b059d1e` |
| `packages/views/collaboration/comment-composer.tsx` | `d2944b577271c5ce98bcedfddecc00d3a50211b366f628ba4320222259db23fb` |
| `packages/views/collaboration/comment-composer.test.tsx` | `1fa9f468340eef4f872d474943babbbddd0b7ab9984d5cb2fbd708c6240db3dc` |

## Remaining verification and handoff

- Production Web and actual Electron mixed-mention flows, wide/narrow/200% layout, and visual-verdict evidence are **pending coordinator verification**. No browser or native-product acceptance was claimed here.
- Server notification/current-participant/actor-exclusion/edit-delta semantics remain in their existing canonical tests, including `TestCollaborationMentionParserKeepsTypedIdentityAndExplicitTargets`, `TestCollaborationCommentPreviewIsReadOnlyAndKeepsIndependentPeopleAndTargets`, and `TestCollaborationCommentEditMentionDeltaUsesCurrentParticipants`. This child did not rerun Go/database tests or start any runtime/provider.
- Root integration still owns app typechecks, locale parity, independent review, broad static checks and final acceptance attribution. AC-I04 and parent AC86/90/91/97 remain open until their complete evidence is collected.
- No shared barrels, schemas, description editor, runtime, SQL, environment, task state or other agent's changes were edited. Nothing was committed or archived by this child.
