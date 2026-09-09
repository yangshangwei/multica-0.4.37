# Design

完整方案与取舍见仓库根 `docs-in-app-design.md`。本文只切执行顺序。

五条 lane，依赖顺序即执行顺序：后一条需要前一条的产物才能验证。L1–L4 是纯增量（应用里多一个只读页面），L5 才改既有行为，所以 L5 单独一个提交，回滚只需 revert 它。

## L1 内容管道

**边界**：`scripts/generate-docs-bundle.mjs` + 测试。只读 `apps/docs/`，只写 `server/internal/docs/content/`。

读 `apps/docs/content/docs/*.zh.mdx` 与 `meta.zh.json`，产出每页一个 JSON（`slug`、`title`、`description`、`body`、`toc`）、一个 `manifest.json`（导航树 + 标题/描述），图片从 `apps/docs/public/images/docs/` 复制。

**全案风险最高的一步。** remark 默认不解析 HTML 块内部的 markdown，所以 `<Callout>` 必须降级为 `remark-directive` 的容器指令（`:::warning`）而非 `<div>`——否则 292 个提示框里的链接、加粗、行内代码原样显示。未登记的 JSX 标签报错退出，否则文档站新增组件时应用内会安静地少一段内容。

先写测试再写生成器：这一步的正确性无法靠肉眼看产物判断。

验证：`node scripts/generate-docs-bundle.mjs && git diff --exit-code server/internal/docs/content/`

## L2 后端

**边界**：`server/internal/docs/embed.go`、`internal/handler/docs.go`、`cmd/server/router.go` 注册三行。

`embed.FS` 放在只被 server 导入的包里——`cmd/multica` 不能背上这 4.5MB。`slug` 用 manifest 白名单校验：嵌入 FS 只阻止跳出 FS，不阻止越权读取自身其他内容。

验证：`cd server && gofmt -l . && go vet ./... && go test ./internal/handler -run Docs -count=1`

## L3 core

**边界**：`packages/core/docs/`（`paths.ts`、`schema.ts`、`queries.ts`）。

`docsHref(slug, anchor?)` 是 L5 的前置。schema 走 `parseWithFallback`。文档是服务端状态 → TanStack Query，不进 Zustand。

验证：`pnpm --filter @multica/core typecheck test`

## L4 渲染与两端路由

**边界**：`packages/views/docs/`、`apps/web/app/[workspaceSlug]/docs/`、桌面端 `routes.tsx`、四个 `locales/*/docs.json` + `i18n/resources-types.ts`。

不动 `RichContent`——它的 docstring 明确禁止长出 surface 专属分支。文档共用 `packages/ui/markdown` 原语，自己一套组件树。图片 src 重写、github-slugger 生成 heading id、内链拦截走 navigation 都在这层。

路由放 workspace 作用域：web 根 `/docs` 一旦配了 `DOCS_URL` 就被 `next.config.ts:55` 的 beforeFiles rewrite 在 Next router 之前接走。

验证：`pnpm --filter @multica/views test typecheck`，然后桌面端手工走一遍完整链路。

## L5 寻址替换与入口重指

**边界**：`packages/views` 9 个文件、3 个测试文件、`onboarding_shim.go`、`install-runtime-issue.ts`。

9 处硬编码改调 `docsHref()`。`help-launcher.tsx` 只改「文档」一项，「更新日志」「桌面端」保持外链。

`runtime-docs.test.ts`、`telegram-tab.test.ts`、`help-launcher.test.tsx` 按 `multica.ai` 形态断言，会红，同批改。新增锚点 parity 测试覆盖 `#事件过滤`、`#自定义运行时配置`——这两个锚点现在只被字符串断言保护，改完必须有测试证明它们在 bundle 里真实存在。

Go 与 TS 两侧的智能体文档 URL 一起改。

验证：`pnpm typecheck lint test` + `make test` 全量。

## 回滚

L1–L4 增量，留在树上无副作用。L5 单独提交，revert 即恢复外链行为，无数据需要迁移。
