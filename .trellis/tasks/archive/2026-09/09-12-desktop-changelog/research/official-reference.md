# Official changelog reference

Retrieved https://multica.ai/changelog on 2026-09-12 UTC with HTTP 200; also rendered in Chromium. Screenshot: `/tmp/multica-changelog-reference.png`. The source has monthly release navigation, a long-form timeline, version/date headers, descriptive titles, and New Features / Improvements / Bug Fixes groups. Latest displayed upstream entry is v0.4.43 dated 2026-09-11; v0.4.37 is dated 2026-08-31.

The requested application surface should preserve that reading structure while using the existing app tokens, Help menu and desktop tabs. Chinese UI must follow `apps/docs/content/docs/developers/conventions.zh.mdx`.

## Upstream v0.4.37 baseline (translated summary)

- New: Huawei Cloud CodeArts runtime; native iPad support; more reliable multi-server WeCom reply delivery.
- Improved: large issue lists and agent startup with many skills; localized onboarding, squads, editor, statuses and priorities; lower server database overhead.
- Fixed: streaming chat scroll; narrow analytics tables; Codex startup; daemon recovery; older self-hosted upgrades; workspace deletion warnings; long and queued runs being cancelled prematurely.

These are attributed upstream release notes, not a claim that this fork has subsequently released. The fork's reachable local tag is v0.4.37; upstream v0.4.38–v0.4.43 are not ancestors of current HEAD. Do not display those upstream versions as installed or fork-published.

## Source and publication honesty

Fork repository `yangshangwei/multica-0.4.37` has no published GitHub releases in the public Releases API at inspection. Current committed changes must be labeled unreleased until the new release pipeline publishes them. Dirty working-tree changes may inform the task research but must not enter generated published notes.
