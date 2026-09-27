# Independent client review

Verdict: **APPROVE** after the Desktop E2E environment guard was added.

This was a read-only review of the integration owner's changes in
`packages/core`, `packages/views`, `apps/web`, `apps/desktop`, and `e2e`.
The reviewer did not implement those application changes.

## Contracts checked

- Exact stored `ja`/`ko` choices normalize before candidate matching, so
  Chinese OS/header preferences cannot override an explicit retired choice.
- New clients normalize old backend responses before supported-locale filtering;
  sync persists English and converges after reload without altering absent or
  unrelated unsupported values.
- UI `zh-Hans` versus content API `zh` remains intact in Mika, templates,
  project configuration, website dictionaries and docs links.
- Only retired resource branches are removed; retained resources, English type
  sources, existing adapters and persistence keys remain.
- Kana detection, Chinese pinyin handling, empty-slug protection, IME/general
  Unicode regression inputs and retained font fallbacks remain.
- Desktop first-connect copy, update notices, HTML language, and native custom
  link labels fall back to English for retired OS locales.
- New Web E2E coverage includes an older-backend response fixture and reload
  bounds. Desktop coverage uses a temporary profile and disables real agent
  execution through the existing isolated Electron fixture.

## Finding resolved

P2: the new Desktop E2E initially required only the two `CHANGELOG_*` endpoints,
while `TestApiClient` takes its API/database from `NEXT_PUBLIC_API_URL` and
`DATABASE_URL` and otherwise falls back to development defaults.
The owner added both explicit environment requirements and an API-origin
equality assertion in `e2e/retained-languages-desktop.spec.ts:10`.
The reviewer re-read that change and confirmed it runs before login/data writes.

No remaining application correctness or compatibility blocker was found.
Final integration/runtime test evidence is owned by the integration agent;
this review does not claim to have rerun its full suite.

## Final Desktop E2E follow-up

Verdict remains **APPROVE** after read-only review of the final test changes:

- Explicit API/database requirements, matching API origins and loopback-only
  API/renderer checks run before fixture login or database writes (lines 10–19).
- The OPTIONS preflight uses the actual renderer origin and verifies the API's
  allow-origin response before starting authentication (lines 20–30). This
  exposes the observed environment mismatch instead of presenting it as a
  language regression. The passing run used the already allowed
  `http://localhost:13781` renderer origin; no product CORS policy was changed.
- The renderer's actual `desktopAPI.systemLocale` must equal each retired test
  locale (lines 59–61), proving the fixture's environment flag reaches preload.
- Page errors are collected after reopening as well as on the first window,
  and checked after the final language switch. Failure screenshots/body text
  are captured before the existing nested cleanup closes the application,
  deletes the fixture workspace and removes its temporary profile (lines 82–109).

Read `/tmp/multica-retain-zh-en-desktop-test.log`: both real Electron tests passed
(`ja` 8.8s, `ko` 8.6s; total 18.1s). Screenshots are under
`/tmp/multica-retain-zh-en-desktop-results`. No new blocking issue was found.
Only this review record was updated; application and test source were unchanged
by the reviewer.
