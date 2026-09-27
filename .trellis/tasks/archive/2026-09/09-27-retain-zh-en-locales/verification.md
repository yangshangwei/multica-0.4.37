# Retain Chinese and English — verification

Implementation based on `e595d2313` in `/Volumes/artisan/code/2026/multica-retain-zh-en-locales`. Independent client review: APPROVE; integration owner also reviewed backend/docs boundaries. All task acceptance criteria are satisfied, with pre-existing findings isolated below. Feature commit: `372576209b64e69e79d89ad637291147c6fbc400` on `main`. No release tag or production deployment was created.

## Changes and simplifications

- Active UI languages: `en`, `zh-Hans`; content/doc Chinese identifiers remain `zh`.
- Deleted all 152 inventoried retired resources: 54 shared JSON, 4 website/case-study files, 94 public-doc files. Removed retired registries, selectors, template/onboarding copy and unreachable locale-only styles.
- One small core normalizer reuses the exact retired-preference rule across Web, Desktop and login synchronization. One backend boundary normalizer preserves old client requests without relaxing unrelated validation.
- Existing content, IME/Unicode support, template identities, null/omission semantics and user preferences retain their documented behavior. No database schema or bulk content migration.
- Source groups: `packages/core/i18n`, `packages/views/locales/settings/onboarding/workspace`, `apps/web`, `apps/desktop`, backend handler/template registries, docs, tests and current maintenance specs. Full path inventory: [changed-files.json](research/changed-files.json).

## Evidence

| Gate | Result |
| --- | --- |
| Red/green regressions | Retired picker/sync, menu/template mapping, Desktop first-connect and backend language/data cases failed for expected old behavior, then passed. See lane evidence. |
| Full static checks | Lint, typechecks, UI export checks passed. Existing warnings remain. |
| Full TypeScript tests | **8,537 passed**: core 2,057; views 5,439; Web 261; Desktop 718; Docs 62. |
| Go and script checks | **66 Go package results passed with race checks**, followed by `go vet`; repository script regressions passed. |
| Production Web browser checks | **9 passed**, zero retries, 40.9s. Includes old Japanese/Korean preferences, both retained choices across reload, older-server preference response, and template/content/identity regressions. |
| Desktop build | CLI bundle and Electron main/preload/renderer production build passed. |
| Real Electron acceptance | **2 passed**, zero retries, 18.1s. Actual Japanese/Korean system signals and saved choices resolve to English; Chinese/English selection, process close/reopen and saved preferences succeed; no renderer errors or real agent starts. |
| Public Docs build/tests | Production build passed; 62 tests passed. |
| Old docs HTTP compatibility | All **90** retired page paths: initial 308, English destination 200, query preservation. Roots, trailing slashes, missing pages, search and SEO checked. |
| Manual screenshots | Chinese/English Desktop settings inspected; layout and controls remain usable. [Chinese](research/desktop-chinese.png), [English](research/desktop-english.png). Docs baseline comparison scored 98/pass. |
| Whitespace/resources | `git diff --check` and retained JSON parse/namespace checks passed; no inventoried retired resource remains. |
| Original checkout preflight | Tracked patch applies cleanly; new source/test files have no conflicts. All 260 task-owned changed/new source paths were synchronized and byte-compared successfully. |

Commands: `bash scripts/check.sh e2e/localized-template-defaults.spec.ts e2e/squads-design.spec.ts e2e/retained-languages.spec.ts --project=chromium` (with installed libpq bin on PATH); Desktop build through `dev-env.sh exec`; real Electron test through that same isolated environment. Logs: `/tmp/multica-retain-zh-en-check.log`, `/tmp/multica-retain-zh-en-desktop-build.log`, `/tmp/multica-retain-zh-en-desktop-test.log`.

## Existing findings and runtime notes

- **Docs React #418 is pre-existing.** An independent four-language build of `e595d2313` and this build both produced two identical English-page hydration errors in the same navigation/reload scenario; Chinese was unaffected. New errors: zero. No unrelated product fix was added. See [docs browser comparison](research/docs-browser-results.json).
- **Knip baseline is unchanged:** nine unused files, one runtime dependency and one dev dependency reported in both checkouts. No new finding; unrelated cleanup was not performed.
- Regeneration synchronized three previously stale Chinese help outputs (`agents-create`, `self-host-quickstart`, `skills`) alongside the intentional conventions update. Those three source pages were not modified.
- First Electron attempt used a random renderer origin rejected by the isolated API's CORS policy. Curl confirmed missing allow-origin headers; using the configured frontend origin fixed the harness. A preflight guard now fails fast before test-account creation. No application CORS changes.
- `make up` automatically collected two expired agent test environments before an isolated API restart. Current-workspace data was unaffected. This was disclosed; subsequent component restarts avoid global GC. See [runtime effects](research/runtime-side-effects.md).

## Integration and handoff

Synchronized all 260 changed/new source paths to `/Volumes/artisan/code/2026/multica-0.4.37`; byte comparison passed and unrelated `.impeccable/` was preserved. Restarted only this checkout’s API/Desktop using named component helpers (no global GC). Fresh API health passed (PID 30300), and the live Desktop renderer at port 5666 served exactly `["en", "zh-Hans"]`. Regenerated ignored MDX indexes for the original checkout. Task-owned API, static renderer and Docs verification servers were stopped. Feature code is committed as `372576209b64e69e79d89ad637291147c6fbc400`; the user authorized pushing it with the completed task records to `origin/main`. See [fresh runtime proof](research/original-runtime.json). Detailed evidence: [backend](research/backend-implementation.md), [client](research/client-implementation.md), [docs](research/docs-implementation.md), [independent review](research/client-review.md).

## Commit preparation

The committed application sources match the verified worktree. The only excluded
generated difference is `apps/web/next-env.d.ts` switching its route-types import
between development and production build directories; its tracked baseline was
restored. Unrelated `.impeccable/` files were excluded. The source commit contains
259 paths; task archive and journal are separate bookkeeping commits.
