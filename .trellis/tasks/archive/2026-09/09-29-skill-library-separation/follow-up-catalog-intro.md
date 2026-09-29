# Remove redundant catalog introduction

## Request and bounded cleanup plan

The user supplied a screenshot and requested removing the catalog heading and the independent-copy introduction beneath it. The tabs identify the view; search, source filters and results should lead the panel.

1. Run existing catalog/session and locale tests to protect browsing behavior.
2. Delete only the introduction wrapper, both conditional heading variants and its description; remove their now-unused English/Chinese locale keys.
3. Update the existing browser assertion to identify the search control, then run targeted tests, lint/typecheck, static checks and one desktop/narrow browser pass.

Keep creation-preview guidance, source empty states, tabs, filters and copy behavior unchanged. This is a small follow-up to the delivered task, not a new feature.

## Result and verification

Removed the introductory wrapper, heading variants and paragraph from the shared catalog, plus their six locale entries across English and Chinese. Search/source filters now lead the panel; the creation preview still explains independent copies.

- Before and after: 3 existing catalog/session/locale suites, 80 tests passed.
- Scoped ESLint and views typecheck passed.
- Impeccable mechanical detector returned no findings; `git diff --check` passed.
- The existing real browser discovery/copy/focus/persistence scenario passed in 13.0 seconds against the local development Web server after route compilation. Initial attempts exceeded the production-oriented test's navigation limits while Next compiled the list and detail routes; application code and test timeouts were not changed for those warmups.
- Wide and 390px screenshots reviewed, visual verdict 96/100. Evidence: `.omx/reports/skill-library-intro-20260929/final/`; logs: `/tmp/skill-intro-after.log`, `/tmp/skill-intro-lint.log`, `/tmp/skill-intro-typecheck.log`, `/tmp/skill-intro-browser-final.log`.
- No production rebuild/deployment or native Electron run for this cosmetic follow-up. No known issue in the scoped change; unrelated workspace edits excluded.
