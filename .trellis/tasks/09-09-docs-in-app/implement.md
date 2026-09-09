# 执行记录：应用内文档（P0）

状态说明：L1–L5 已全部实现并按层提交到 `main`（2026-09-10 收尾）。原计划在 `feat/docs-in-app` 分支上工作，实际提交直接落在 `main`，该分支现无领先提交。本文件为事后补写的执行记录，对应 `prd.md` 验收项逐层落地情况。

## L1 内容管道

- [x] `apps/docs/lib/docs-bundle/jsx-to-directive.mjs`（`61c73d738`）：`<Callout>`（`warn`/`warning` 合并）降级为 remark 容器指令且体内 markdown 仍被解析；`<VideoEmbed>`、`<CommunityLinks>` 降级为叶子指令；未登记 JSX 标签报错退出。
- [x] `apps/docs/lib/docs-bundle/parse-page.mjs`（`0ef6b1dfa`）：解析 frontmatter、`meta.zh.json` 分组分隔符、标题与正文；配套 `parse-page.test.ts` 232 行矩阵。
- [x] `scripts/generate-docs-bundle.mjs`：从 `apps/docs/content/docs/` 生成提交进仓库的 bundle（45 页 zh 正文 + 图片资产）。
- [x] CI（`.github/workflows/ci.yml`）：重新生成 bundle 后 `git diff --exit-code` 校验新鲜度，沿用 `generate:reserved-slugs` 约定。

验证：`pnpm test` 覆盖 `generate-docs-bundle.test.ts` / `jsx-to-directive.test.ts` / `parse-page.test.ts`。

## L2 后端分发

- [x] `server/internal/docs/embed.go`（`0c96d792b`）：`go:embed` 进 server 二进制，manifest/page/assets 三端点 handler；slug 与资产路径走 manifest 白名单，不做字符串拼接。
- [x] manifest 带 `serverVersion`；ETag 命中路径。
- [x] `binary_scope_test.go`：CLI 二进制（`cmd/multica`）未嵌入文档内容。
- [x] `docs_test.go` 325 行：三端点正常/异常路径、白名单拒绝越权。

验证：`cd server && go test ./internal/docs ./internal/handler -run 'Docs' -count=1`（需可达 DATABASE_URL）。

## L3 core 寻址与解析

- [x] `packages/core/docs/`（`af68f1075`）：`schema.ts`（zod + `parseWithFallback`，配 `schema.test.ts` malformed-response 测试）、`queries.ts`（TanStack Query，不进 Zustand）、`slugs.ts`、`types.ts`。
- [x] `packages/core/paths/paths.ts`：`docsHref(slug, anchor?)` 唯一寻址入口。
- [x] `docs-anchor-parity.test.ts`：产品代码出现的每个锚点都能在 bundle 里找到对应 heading；task.json notes 记录的两个既有失效锚点（`#事件过滤` 词序、ko `#사용자-지정-런타임-프로필`）在 L5 一并修正并被 parity 测试锁定。

验证：`pnpm test --filter @multica/core`。

## L4 渲染与两端路由

- [x] `packages/views/docs/`（`91ea4362a`）：`docs-content.tsx`（共用 `packages/ui/markdown` 下层原语，不动 `RichContent`）、`docs-page.tsx`、`directives.ts`（Callout/VideoEmbed/CommunityLinks）、`sanitize.ts`、图片 src 重写为 `${apiUrl}/api/docs/assets/...`、heading id 用 github-slugger、内链走 `useNavigation().push()`。
- [x] web：`apps/web/app/[workspaceSlug]/(dashboard)/docs/[...page]/page.tsx`；desktop：`routes.tsx` 挂 session 路由。
- [x] `locales/{en,zh-Hans,ko,ja}/docs.json` 四语 UI 文案，注册进 `i18n/resources-types.ts`；正文 zh-only 为已知边界。

验证：`pnpm test --filter @multica/views`（`docs-content.test.tsx` 216 行：Callout 内 markdown 渲染、图片重写、内链 navigation）。

## L5 寻址替换与入口重指

- [x] `3a8f1050d`：产品内 9 处硬编码 `multica.ai/docs` 全部改为 `docsHref()`——help-launcher（入口重指非新增，去外链字形）、agents-page、agent-approvals-page、skills-page、runtimes-page/runtime-docs/runtime-profiles-dialog、collection-page、slack/telegram/dingtalk-tab、webhook-event-filter-section。
- [x] 智能体文档访问改内网：`onboarding_shim.go` WebFetch 指令、`install-runtime-issue.ts` 落 issue 正文的 4 处 URL 改指本部署。
- [x] 「更新日志」「桌面端」保持外链（发布资产不在本部署内）；`DOCS_URL` 既有 web 行为未动。
- [x] `runtime-docs.test.ts`、`telegram-tab.test.tsx`、`help-launcher.test.tsx` 等按 `multica.ai` 形态的既有断言同批改。

## 验收状态

PRD 全部验收项已随各层提交配套测试；桌面端完整链路（sidebar footer → 开 tab → 图片加载 → 内链跳转 → 锚点定位）在实现会话手工走通。收尾归档会话（2026-09-10）未重跑全量 `pnpm typecheck` / `pnpm test` / `make test`，以各层提交时的验证记录为准。

## Git Commits

| Hash | Layer | Message |
|------|-------|---------|
| `61c73d738` | L1 | feat(docs): add the JSX-to-directive transform for the in-app docs bundle |
| `0ef6b1dfa` | L1 | feat(docs): generate the in-app docs bundle from the docs site source |
| `0c96d792b` | L2 | feat(server): serve the embedded documentation bundle |
| `af68f1075` | L3 | feat(core): address and parse the in-app documentation |
| `91ea4362a` | L4 | feat(views): render the in-app documentation on web and desktop |
| `3a8f1050d` | L5 | feat(views): point every documentation link at this deployment |

## 回滚

按 design.md：bundle 生成物与端点是纯增量，回滚 = revert 上述 6 个提交；无数据库迁移、无 wire shape 变更、无路由路径变更。
