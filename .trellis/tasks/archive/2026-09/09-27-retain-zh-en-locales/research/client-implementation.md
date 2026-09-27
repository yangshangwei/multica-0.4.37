# Client implementation evidence

- Isolated checkout: `/Volumes/artisan/code/2026/multica-retain-zh-en-locales`.
- Baseline: core locale 11 tests, Web locale/docs fallback 12 tests, views retained/parity baseline 211 tests passed.
- Red: core picker/sync 9 failures and Web routing 6 failures exposed retained preference behavior; settings/template mapping 5 failures, context-menu 2 failures and endpoint setup 2 failures exposed retired UI behavior.
- Green: core picker/sync 24 tests, Web routing 9 tests, views affected suites 237 tests, Desktop affected suites 18 tests passed.
- Active UI locales narrowed to en/zh-Hans. Shared exact retired preference normalization is reused by Desktop choice, Web cookie negotiation and server preference sync. English fallback/type definitions retained.
- Removed 54 shared JSON and 4 website/case-study resources, retired dictionary branches, selectors, onboarding variants and unused language-specific CSS. General CJK fonts/input remain.
- Removed the now-unused UI locale argument from workspace slug derivation; retained kana/Hangul empty-slug protection and Chinese pinyin behavior.
- Updated existing localized template E2E coverage to retained locales; added browser cases for both retired preferences, retained switches/reload and older-server preference responses.

Full repository checks, production browser verification and final independent review are in progress. No source changes have been applied to the original checkout yet.
