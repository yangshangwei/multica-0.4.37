# In-App Docs Bundle — Mechanics and Content Rules

> Source of truth for how the in-app Chinese documentation bundle is built,
> what constrains page deletion, and the intranet content rules this fork
> follows. Captured from task `09-10-intranet-docs-trim` (2026-09-10).

---

## 1. Scope / Trigger

Read this before adding, removing, or editing any page under
`apps/docs/content/docs/*.zh.mdx`, before touching `meta.zh.json`, or before
changing product deep links into the docs.

## 2. Pipeline (signatures)

```
apps/docs/content/docs/*.zh.mdx + meta.zh.json
  --(node scripts/generate-docs-bundle.mjs)-->  server/internal/docs/content/
                                                    manifest.json, pages/*.json, assets/
  --(go:embed)-->                               API binary
  --(GET /api/docs/manifest, /api/docs/page?slug=..., /docs-asset/*)-->  reader
```

- The generator packs **Chinese pages only** (`*.zh.mdx`). Other-language
  `.mdx` files serve the public docs site and never enter the binary.
- `server/internal/docs/content/` is **committed**: CI proves the embedded
  bytes have not drifted from the source. Always regenerate and commit after
  editing `.zh.mdx` or `meta.zh.json`.

## 3. Contracts

### Nav / access whitelist

- `meta.zh.json` `pages[]` is the access whitelist. `docs.Page(slug)` only
  resolves slugs present in the generated manifest; anything else 404s.
- `buildNav()` (in `apps/docs/lib/docs-bundle/parse-page.mjs`) validates
  **both directions** and throws:
  - meta lists a slug with no generated page → error;
  - a generated page missing from meta → error ("unreachable in the sidebar").
- Therefore: **removing a page = delete the `.zh.mdx` file AND its meta
  entry in the same change.** There is no include/exclude flag and we do not
  add one (single-deployment fork; a trim switch would be a compatibility
  layer).

### Product deep links

- Product code links into docs pages only through
  `DOCS_SLUGS` / `DOCS_ANCHORS` in `packages/core/docs/slugs.ts`.
- `apps/docs/lib/docs-bundle/docs-anchor-parity.test.ts` walks `DOCS_SLUGS`
  (so it shrinks automatically when entries are removed) and asserts the two
  anchors exist verbatim: heading `自定义运行时配置` on `daemon-runtimes`,
  heading `过滤事件` on `autopilots`. **Do not reword those headings.**
- A slug removed from `DOCS_SLUGS` requires removing every product usage in
  the same change (`pnpm typecheck` catches dangling references); UI copy in
  settings tabs etc. usually needs the link branch and its now-unused imports
  removed too.

### Cross-references

- In-page links use `/slug` or `/slug#anchor`. `resolveDocsHref` resolves any
  slug-shaped string to a docs route, so a link to a deleted page becomes a
  runtime 404 with **no build-time check**. After deleting pages, grep the
  remaining `.zh.mdx` sources and the regenerated bundle for the removed
  slugs.

## 4. Validation & Error Matrix

| Condition | Result |
|---|---|
| Page deleted but still in `meta.zh.json` | generator errors ("no page was generated for it") |
| Page kept but removed from meta | generator errors ("unreachable in the sidebar") |
| `DOCS_SLUGS` entry with no page | parity test fails |
| Renamed `自定义运行时配置` / `过滤事件` heading | parity test fails |
| In-page link to a removed slug | no check — runtime 404; grep manually |
| Bundle not regenerated after source edit | CI drift test fails |

## 5. Intranet Content Rules (this fork)

This deployment has **no public-internet access** (no multica.ai, github.com,
npmjs.com, GHCR, app stores, SaaS IM platforms). Rules for `.zh.mdx` content:

1. **No required public-internet dependency.** Installation, clone, image
   pull, and CLI download steps must use intranet placeholders such as
   `<内网镜像仓库>/multica.git` plus "由管理员提供" wording. Never invent a
   concrete intranet address.
2. **Reference links may stay** only when annotated as unreachable from the
   intranet ("内网不可达，仅作离线参考").
3. New pages that exist only to serve a public SaaS (cloud signup, SaaS bot
   integrations, community/maintainer pages) do not belong in this bundle;
   `vcs-integration` covers self-hosted Git instead.
4. When deleting pages, clean the now-unused locale keys and product links in
   the same change (e.g. `help.download_desktop`, `*.byo_docs_link` were
   removed with the bot doc pages).

## 6. Good/Base/Bad Cases

- **Good**: replace `git clone https://github.com/multica-ai/multica.git`
  with `git clone <内网镜像仓库>/multica.git` plus a note that the admin
  provides the address.
- **Base**: keep a tool vendor's docs URL as plain text with
  "内网不可达" annotation.
- **Bad**: keep an unannotated `curl … raw.githubusercontent.com` install
  command; delete a page from meta but leave the `.zh.mdx` (or vice versa);
  point a "查看文档" button at a removed slug.

## 7. Tests Required

- `node scripts/generate-docs-bundle.mjs` — structural two-way validation.
- `apps/docs` vitest (`docs-anchor-parity.test.ts`, bundle drift test).
- `go test ./internal/docs/...` (embed validation) and
  `go test ./internal/handler/ -run Docs` against a cloned template DB
  (handler fixtures need a migrated database; see project memory).
- Grep the regenerated bundle for removed slugs — zero matches expected.

## Wrong vs Correct

### Wrong

```jsonc
// meta.zh.json entry removed, file left in place
// → generate-docs-bundle fails: "generated pages missing from meta"
```

### Correct

```
delete apps/docs/content/docs/<slug>.zh.mdx
remove "<slug>" from meta.zh.json pages[]
run node scripts/generate-docs-bundle.mjs
commit source + meta + regenerated server/internal/docs/content/ together
grep remaining .zh.mdx + bundle for "<slug>" → 0 hits
```
