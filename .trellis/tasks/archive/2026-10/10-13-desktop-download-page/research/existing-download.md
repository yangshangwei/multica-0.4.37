# Existing Web download page review

Date: 2026-10-13. The user supplied a photograph of `http://10.100.160.112:3000/download` after the first planning summary and asked to inspect this existing implementation. This is additional evidence, not implementation approval.

## Identification and evidence limits

- Route: `apps/web/app/(landing)/download/page.tsx:26` renders the matching download page. `:9` declares a 300-second revalidation window.
- Composition: `apps/web/app/(landing)/download/download-client.tsx:38` combines LandingHeader, DownloadHero, AllPlatforms, CLI, Cloud, version footer and LandingFooter.
- Screenshot wording matches `apps/web/features/landing/i18n/zh.ts:3258` and the download footer text.
- A direct HTTP HEAD request to the supplied intranet address timed out after 5 seconds with no response. No deployed logs, release response, or deployment commit were obtained. Conclusions below describe the matching repository implementation and visible screenshot, not a verified production root cause.
- Distinguish the existing Next.js Web page from the Nginx-only download service. The earlier statement that Nginx lacks a human-facing index remains valid; a statement that the repository has no download page would be incorrect.

## Data flow and the screenshot's unavailable state

- `apps/web/features/landing/utils/github-release.ts:39` hardcodes `https://api.github.com/repos/multica-ai/multica/releases?per_page=5`.
- `:53` performs the fetch on the Next.js server. Caching is 300 seconds; an optional server-only GitHub token may affect rate limiting but is not a suitable solution for the required public-network-independent page.
- `:74` rejects non-2xx responses. `:96` catches request/parse errors and returns an empty release with `version: null`; an empty candidate set also returns an empty release. This selects the visible “版本获取失败——请前往 GitHub 查看” state. The screenshot alone does not establish whether network access, rate limiting, payloads or another fetch condition caused it.
- `apps/web/app/(landing)/download/download-client.tsx:18` hardcodes the GitHub releases fallback; this appears in the version footer and incomplete-platform escape hatch. Hosting this code at an intranet address does not automatically redirect its data source to the intranet installer store.
- `github-release.ts:120` prefers the first recent release whose complete desktop asset set exists. `parse-release-assets.ts:108` lists twelve required artifacts across Mac, Windows and Linux. If only a newer Windows build exists while an older full release exists, this algorithm can display the older version. It must not drive a Windows-only intranet launch.
- `parse-release-assets.ts` maps GitHub asset records into platform URL fields; it does not validate local files, hashes or publication state. Keep the existing desktop artifact publisher as the local truth source.

## Reusable presentation

- `components/download/hero.tsx:31` supplies a dark hero, centered serif heading and primary download action; `:240` defines the real gradient. Display glare and texture in the photograph are not intentional design assets.
- `components/download/all-platforms.tsx:28` supplies the light platform matrix and inline OS icons. Its row layout is a useful baseline for pending platform states.
- `components/download/os-icons.tsx` contains existing platform SVG geometry.
- `app/(landing)/layout.tsx:1` uses `next/font/google` for Instrument Serif. Next normally emits self-hosted font assets at build time; this is not evidence that the user's browser directly contacts Google. A standalone static derivative must ship permitted local font assets or use local font stacks, not copy a Next font loader into browser code.
- The components depend on React, `next/link`, locale context and landing styles. They are not files that can be copied directly into an Nginx directory and executed. Reusing their markup/layout/brand assets differs from retaining the Next runtime.

## Changes required by the already-confirmed scope

| Existing behavior | Required intranet behavior |
| --- | --- |
| GitHub server fetch and public release URLs | Validated current Windows x64 data from the local release process |
| Browser OS detection chooses different primary platforms | Windows x64 is the only available download; every promised future platform stays pending |
| Mac/Windows ARM/Linux format buttons, with unavailable tooltips | Explicit “待开放”; no Linux format commitment; no Windows ARM or 32-bit entry in this task |
| GitHub fallback and all-release links | Local failure/retry guidance; no public fallback or history selector |
| CLI installer command uses raw.githubusercontent.com | Excluded from this first-install desktop page |
| Cloud runtime waiting-list section | Excluded from the agreed scope |
| “内置 daemon，无需配置” | Wording must make the required business-server connection clear |
| No first-use guide in the current composition | Configured business address, copy action, Chinese steps, password signup/login, both workspace-entry paths and FAQ |

## Confirmed reuse decision

The user confirmed using this page as the basis for the standalone static intranet deliverable and preserving its dark download hero and light platform matrix. Use in-page navigation to downloads, installation and FAQ. Keep the already-confirmed operational requirements and local publisher contracts.

This resolves visual decision D13. The target remains an independent static intranet page; it does not authorize silently changing the existing live Next.js route. Implementation follows review of the updated complete planning artifacts.
