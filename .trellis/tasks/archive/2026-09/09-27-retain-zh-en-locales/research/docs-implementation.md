# Documentation implementation and verification

## Scope

- Active public-doc languages are `en` and `zh`; removed Japanese/Korean
  translations, menu/home copy, search tokenizers and unused Japanese-only CSS.
- Deleted the 94 inventoried documentation resource files: 90 MDX pages and
  four metadata files. All 90 old page/root routes had English source counterparts
  before deletion; see `retired-docs-routes.json`.
- Added config-relative permanent redirects from `/ja/:path*` and
  `/ko/:path*` to `/:path*` under the existing `/docs` basePath.
  Retired prefixes in existing Markdown links remain intact so redirects work.
- Updated English/Chinese conventions and the docs-bundle spec. Regenerated
  the Chinese embedded bundle (36 pages, 58 assets) without removing pages.

## Tests and build

1. Red: focused static-param, hreflang and redirect/menu tests failed in six
   expected assertions against the four-language implementation.
   Log: `/tmp/multica-retain-locales-docs-red.log`.
2. Green: `pnpm --filter @multica/docs test --maxWorkers=2` passed
   **9 files / 62 tests**.
   Log: `/tmp/multica-retain-locales-docs-green.log`.
3. `pnpm --filter @multica/docs build` passed with Next.js 15.5.18, including
   type validation and production generation (95 static pages).
   Log: `/tmp/multica-retain-locales-docs-build.log`.
4. `go test -p 2 -parallel 2 ./internal/docs ./internal/handler -run Docs -count=1`
   passed through the CLI guard against isolated database `multica_locale_backend_1790477109326`
   (docs 1.116s, handler 0.859s).
5. Scoped `git diff --check` passed. The full integration owner runs global
   lint/static/build checks; no second global check was started in this lane.

## Actual production-server checks

Started the built app with
`pnpm --filter @multica/docs exec next start --hostname localhost --port 51424`.
The task-owned listener was PID **84586**, at **http://localhost:51424**.

- All **90** recorded old routes returned **308 → canonical English page 200**.
- Every route retained a Chinese query value and duplicate query parameters.
- Three trailing-slash variants reached their canonical 200 pages without loops.
- Unknown retired slugs returned 308 → normal 404.
- Sitemap advertises only `en`, `zh`, `x-default`.
- English and Chinese search returned 200 with nonempty results and no
  retired-language result URLs.
- Chromium verified exactly two language options, English → Chinese → English,
  page reload, correct HTML language and retained hreflang.

Evidence: `docs-http-results.json`, `docs-browser-results.json`,
`screenshots/docs-language-menu-{baseline,current}.png`,
`screenshots/docs-zh-agents-{baseline,current}.png`.
HTTP checker: `/tmp/multica-retain-locales-docs-http.py`.

## Baseline issues isolated

**Existing hydration error:** the browser initially reported React #418 on
English pages. To distinguish a regression, extracted docs from commit
`e595d2313` into `/tmp/multica-docs-baseline-1790478704537`, reused installed dependencies,
built the original four-language app and served it separately on port 51425.
The exact same navigation/reload script produced **baseline = 2 errors,
new build = 2 identical errors; newly introduced errors = 0**. Chinese navigation
worked in both. SSR/client text differs at the automatic previous/next page
footer; unchanged Fumadocs `PageFooter` resolves this through `usePathname`,
while the unchanged English middleware rewrites the prefix-free URL to `/en`.
This task leaves the existing hydration issue unchanged.

**Existing generated-content drift:** regeneration also synchronized three
unchanged Chinese source pages that were ahead of their committed bundle:
`agents-create` (14-role roster), `self-host-quickstart` (offline install command
and mounted skill templates), and `skills` (categories/tags). Their source
files were not edited by this task; generated output now matches the authoritative
source. The fourth generated diff, `developers/conventions`, is intentional here.

**Local test environment:** binding Next to `127.0.0.1` caused its localhost
normalization to treat English middleware rewrites as external requests and loop.
Using `--hostname localhost` on the same build resolved this. Python's inherited
system proxy also returned 502 for loopback; HTTP verification explicitly disabled
proxies. Neither problem required an application-code change.

## Visual verdict

Compared baseline/current English menu and Chinese page at 1440×1000:
**98 / pass**. Layout, font order and spacing remain consistent; removal of the
two retired menu items is intentional. Pointer-hover differences between captures
do not change layout. The verdict is archived in
[docs-visual-verdict.json](docs-visual-verdict.json).

The baseline server and temporary extraction directory were cleaned up after comparison.
Its successful build log remains in `docs-baseline-build.txt`. The isolated backend
test database was dropped without FORCE; the current docs listener is handed to
the integration owner for final review and cleanup.
