# Desktop changelog staging audit

Prepared 2026-09-12T21:47:47.098828+00:00 against HEAD `d96d4012770e971b4e03ab070e034ab928c115a8`.

**Result: 87 approved product/spec files staged; 5540 insertions and 97 deletions.** No commit was created. The index was empty at the initial check and at application time. Trellis task/journal files remain unstaged for the lead.

Product-only index tree: `1ee63b52ae07f63195c5c0c408a7c5c6c65f3906`. Staged patch SHA-256: `fb270db90c20469f76f606c3fd84e2ad2efc47837e23416a60afccee69988c7d`.

## Verification

- `git diff --cached --check`: exit 0, no output.
- Staged paths exactly match the approved manifest; all index entries outside the manifest are unchanged.
- SHA-256 and mode comparison confirms all 113 snapshotted dirty/new source files were preserved during staging. This excludes Trellis task/journal evidence being edited by the lead.
- 11 shared files still have the unrelated messaging hunks unstaged; no fully owned feature file has unstaged content left.
- Staged Go wiring and router-test blobs match gofmt output.
- Parsed embedded docs retain the HEAD content exactly after removing the new changelog section and its single TOC entry.
- The staged patch contains no MessagingIntegrationsDisabled, messagingIntegrationsEnabled, or MULTICA_MESSAGING_INTEGRATIONS_ENABLED reference.
- The router test now clears the five existing provider keys. The lead applied this isolation fix before staging; the lead also restored the original executable bit of update-notification.test.tsx. Both current versions are staged.
- Functional test reruns belong to the lead; this pass verifies index scope, formatting, and preservation.

## Shared-file hunk proof

| File | Staged lines | Included scope |
| --- | --- | --- |
| `.env.example` | +6/-0 | Only the six-line CHANGELOG block after PORT. |
| `SELF_HOSTING.md` | +51/-1 | In-App Changelog section and Google/docs/changelog offline bullet correction only. |
| `apps/docs/content/docs/environment-variables.mdx` | +17/-0 | Only the Changelog section. |
| `apps/docs/content/docs/environment-variables.zh.mdx` | +17/-0 | Only the 变更说明 section. |
| `deploy/helm/multica/templates/backend.yaml` | +16/-2 | Only the changelog directory mount and volume wiring; its whole current diff is in scope. |
| `deploy/helm/multica/templates/configmap.yaml` | +1/-0 | CHANGELOG_FILE only. |
| `deploy/helm/multica/values.yaml` | +8/-0 | backend.changelog.existingClaim and backend.config.changelogFile only. |
| `docker-compose.selfhost.yml` | +2/-0 | Directory mount and CHANGELOG_FILE only. |
| `package.json` | +3/-1 | Only the two changelog command aliases; its whole current diff is in scope. |
| `scripts/helm-config.test.sh` | +24/-0 | Only changelog defaults, explicit-path rendering, and directory-mount tests. |
| `server/cmd/server/router.go` | +4/-0 | Four added lines in three hunks: If-None-Match, CHANGELOG_FILE config, authenticated route/comment. Existing config alignment and messaging behavior remain HEAD-equivalent. |
| `server/internal/docs/content/pages/environment-variables.json` | +6/-1 | Only the matching body insertion and one TOC entry; every other parsed field and prior body/TOC content remains HEAD-equivalent. |
| `server/internal/handler/handler.go` | +11/-4 | Changelog import, config field, reader member, and initialization only. |

## Staged file manifest

```text
M	.dockerignore
M	.env.example
M	.github/RELEASING.md
M	.github/workflows/release.yml
M	.gitignore
A	.trellis/spec/guides/changelog-release-flow.md
M	.trellis/spec/guides/index.md
M	Dockerfile
M	SELF_HOSTING.md
M	apps/desktop/src/renderer/src/App.tsx
M	apps/desktop/src/renderer/src/components/desktop-layout.tsx
M	apps/desktop/src/renderer/src/components/update-notification.test.tsx
M	apps/desktop/src/renderer/src/components/update-notification.tsx
M	apps/desktop/src/renderer/src/routes.tsx
M	apps/docs/content/docs/environment-variables.mdx
M	apps/docs/content/docs/environment-variables.zh.mdx
A	apps/web/app/[workspaceSlug]/(dashboard)/changelog/page.tsx
M	deploy/helm/multica/templates/backend.yaml
M	deploy/helm/multica/templates/configmap.yaml
M	deploy/helm/multica/values.yaml
M	docker-compose.selfhost.build.yml
M	docker-compose.selfhost.yml
M	docs/offline-upgrade.zh-CN.md
A	e2e/changelog-desktop.spec.ts
A	e2e/changelog.spec.ts
A	e2e/fixtures/changelog-electron.cjs
M	package.json
M	packages/core/api/client.ts
A	packages/core/changelog/index.ts
A	packages/core/changelog/queries.test.ts
A	packages/core/changelog/queries.ts
A	packages/core/changelog/schema.test.ts
A	packages/core/changelog/schema.ts
A	packages/core/changelog/selection.test.ts
A	packages/core/changelog/selection.ts
A	packages/core/changelog/types.ts
M	packages/core/diagnostics/diagnostic-context.ts
M	packages/core/package.json
M	packages/core/paths/paths.test.ts
M	packages/core/paths/paths.ts
M	packages/core/paths/route-icons.ts
M	packages/core/paths/tab-presentation.test.ts
A	packages/views/changelog/changelog-page.test.tsx
A	packages/views/changelog/changelog-page.tsx
A	packages/views/changelog/index.ts
M	packages/views/i18n/resources-types.ts
M	packages/views/layout/help-launcher.test.tsx
M	packages/views/layout/help-launcher.tsx
A	packages/views/locales/en/changelog.json
M	packages/views/locales/en/layout.json
M	packages/views/locales/index.ts
A	packages/views/locales/ja/changelog.json
M	packages/views/locales/ja/layout.json
A	packages/views/locales/ko/changelog.json
M	packages/views/locales/ko/layout.json
A	packages/views/locales/zh-Hans/changelog.json
M	packages/views/locales/zh-Hans/layout.json
M	packages/views/package.json
M	packages/views/search/search-command.test.tsx
M	packages/views/search/search-command.tsx
M	scripts/build-offline-upgrade.sh
A	scripts/changelog-lib.mjs
A	scripts/changelog-test-helpers.mjs
A	scripts/changelog-workflow.test.mjs
A	scripts/generate-changelog.mjs
A	scripts/generate-changelog.test.mjs
M	scripts/helm-config.test.sh
A	scripts/install-changelog.mjs
A	scripts/install-changelog.sh
A	scripts/install-changelog.test.mjs
M	scripts/offline-bundle.sh
A	scripts/offline-changelog.test.mjs
M	scripts/offline-installer.sh
M	scripts/offline-upgrade.sh
A	scripts/publish-changelog.mjs
A	scripts/publish-changelog.test.mjs
A	scripts/release-changelog.mjs
A	scripts/release-changelog.test.mjs
A	server/cmd/server/changelog_test.go
M	server/cmd/server/router.go
A	server/internal/changelog/content/changelog.json
A	server/internal/changelog/feed.go
A	server/internal/changelog/feed_test.go
M	server/internal/docs/content/pages/environment-variables.json
A	server/internal/handler/changelog.go
A	server/internal/handler/changelog_test.go
M	server/internal/handler/handler.go
```

## Preserved unrelated work

The remaining unstaged source changes include messaging API schemas/config, auth initialization, integration UI and tests, template/catalog UI and tests, server main/config/DingTalk, and the messaging portions of shared deployment/docs/router files. Unrelated untracked tests and Trellis tasks are also untouched. No remaining dependency on those changes was found in the staged feature.

```text
.env.example
.trellis/tasks/archive/2026-09/09-12-intranet-messaging-integrations/check.jsonl
.trellis/tasks/archive/2026-09/09-12-intranet-messaging-integrations/implement.jsonl
.trellis/tasks/archive/2026-09/09-12-intranet-messaging-integrations/verification.md
SELF_HOSTING.md
apps/docs/content/docs/environment-variables.mdx
apps/docs/content/docs/environment-variables.zh.mdx
deploy/helm/multica/templates/configmap.yaml
deploy/helm/multica/values.yaml
docker-compose.selfhost.yml
packages/core/api/schemas.ts
packages/core/config/index.ts
packages/core/platform/auth-initializer.test.tsx
packages/core/platform/auth-initializer.tsx
packages/views/agents/components/agent-overview-pane.test.tsx
packages/views/agents/components/agent-overview-pane.tsx
packages/views/agents/components/agents-page.test.tsx
packages/views/agents/components/builtin-agent-catalog.tsx
packages/views/agents/components/tabs/integrations-tab.test.tsx
packages/views/agents/components/tabs/integrations-tab.tsx
packages/views/autopilots/components/autopilot-template-catalog.tsx
packages/views/autopilots/components/autopilots-page.test.tsx
packages/views/common/builtin-template-catalog.tsx
packages/views/settings/components/integrations-tab.test.tsx
packages/views/settings/components/integrations-tab.tsx
packages/views/skills/components/builtin-skill-catalog.tsx
packages/views/skills/components/skills-page.test.tsx
packages/views/squads/components/builtin-squad-catalog.tsx
packages/views/squads/components/squads-page.test.tsx
scripts/helm-config.test.sh
server/cmd/server/main.go
server/cmd/server/router.go
server/internal/docs/content/pages/environment-variables.json
server/internal/handler/config.go
server/internal/handler/dingtalk.go
server/internal/handler/dingtalk_test.go
server/internal/handler/handler.go
```

## Audit artifacts

Temporary audit directory: `/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-changelog-index-v4nwehbd`. It contains the initial index/status, source hashes and modes, per-file staged blob manifest, exact staged patch, shared-file candidate patch, staged name/status and line counts, and remaining unstaged names.

The only worktree file written by this staging pass is this summary. The lead may now stage finalized changelog Trellis evidence and journal entries, verify the combined index, and commit.
