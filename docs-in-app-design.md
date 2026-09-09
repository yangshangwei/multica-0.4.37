# 应用内文档：内网部署的文档分发设计

## 背景与目标

产品里所有文档链接都指向 `https://multica.ai/docs`：`packages/views` 有 9 个文件硬编码了它（`layout/help-launcher.tsx:25`、`runtimes/components/runtime-docs.ts:9`、`autopilots/components/webhook-event-filter-section.tsx:22`、settings 的 slack/dingtalk/telegram 三个 tab、`agents/components/agents-page.tsx:268`、`agents/approvals/agent-approvals-page.tsx:70`、`skills/components/skills-page.tsx:185`，以及 `onboarding/templates/install-runtime-issue.ts` 会写进 issue 正文的 4 处），后端 `internal/handler/onboarding_shim.go:79` 还在指示智能体 `WebFetch` 同一个地址。内网部署访问不到公网，这些全部是死链。

Web 侧已有代理能力但指不到东西：`apps/web/config/runtime-urls.ts:67` 的 `resolveDocsUrl` 配合 `apps/web/next.config.ts:55` 的 beforeFiles rewrite 会把 `/docs/**` 转发到 `DOCS_URL`，`docker-compose.selfhost.yml:216` 也透出了这个变量，但仓库里没有 docs 镜像，compose 和 Helm 里也没有 docs service。桌面端则完全没有文档内容来源。

目标是让内网部署的桌面端和 web 都能读到与本部署版本一致的文档，内容保持 `apps/docs/content/docs/` 单一来源，不 fork，不新增运维组件。P0 只做中文正文。

## 设计取舍

内容有三个独立问题，混在一起会得出错的答案：内容运行时放哪（分发）、谁渲染（呈现）、产品链接怎么找到它（寻址）。

**分发：嵌进后端二进制。** 文档描述的是服务端行为，版本必须跟服务端走——装了 0.4.37 桌面端的机器连到 0.4.42 的后端时应该看到 0.4.42 的文档。后端是定义上每个客户端都能到达的唯一 origin，而桌面端没有后端本来就只是一个配置页，所以这不引入新的可达性依赖。一次覆盖 web、desktop 和未来的 mobile，运维不多一个要拉取、对版本、排错的镜像。成本约 4.5MB 进镜像，在 Alpine 运行时下是噪声。后端已有 `go:embed` 先例：`internal/service/builtin_skills.go:10`、`internal/handler/reserved_slugs.json`、`pkg/publicapi/v1/openapi.yaml`。

否决「把 Fumadocs 静态产物打进 Electron」：版本会漂移（文档描述客户端构建而非所连部署），帮不到 web 和 mobile，每个平台安装包各加 4.5MB，而离线收益是假的——离线时桌面端本来也用不了。

否决「补 docs 容器 + 配 `DOCS_URL`」作为终态：它只修 web，桌面端仍是死链。可以作为临时止血，不作为终态。

**呈现：原生渲染。** 内容离纯 markdown 只差三个组件（`<Callout>` 292 次、`<CommunityLinks>` 4 次、`<VideoEmbed>` 1 次），frontmatter 只有 `title` / `description`，链接和图片都是根相对路径；渲染栈已在仓库（`packages/ui/markdown` 的 react-markdown + remark-gfm + rehype-sanitize + shiki）。桌面端 renderer 跑在 `file://` 且 `webSecurity: false`（见 `apps/desktop/src/main/renderer-web-preferences.ts` 的说明），在其上内嵌一个站点是安全倒退，还要放弃 tab 系统、主题、i18n 和统一搜索。

**寻址：一个解析器。** `packages/core` 出一个 `docsHref(slug)`，上述 9 处全部改为调用它。这是直接消除死链的那一步。

## 用户流程

1. **进入**：sidebar footer 的 `HelpLauncher`（已存在于 `packages/views/layout/app-sidebar.tsx:878` 的 `SidebarFooter`）里「文档」一项由外链改为应用内跳转，去掉 `ArrowUpRight` 外链字形。「更新日志」和「桌面端下载」保持外链——它们指向的发布资产不在本部署内。
2. **阅读**：路由 `/{slug}/docs` 落在目录页，`/{slug}/docs/{page}` 落在具体页面。桌面端它是普通 session route，直接就是一个 tab，不需要 `WindowOverlay`。
3. **导航**：左侧目录树来自 `meta.zh.json` 的 `pages` 数组，保留其 `---分组名---` 分隔符语义。正文内部链接拦截后走 `useNavigation().push()`，外部链接仍开新标签页。
4. **上下文跳转**：产品内「查看运行时文档 →」这类链接带 slug 和锚点直接落到对应位置，锚点行为与公网文档一致。
5. **搜索**：P1。P0 先给目录树 + 正文。

## 状态与异常

| 状态 | 行为 |
| --- | --- |
| manifest 请求失败 | 阅读器显示错误态与重试；sidebar 入口保持可见 |
| slug 不存在 | 应用内 404 文案，提供回目录链接，不抛错误页 |
| page 响应字段缺失 | `parseWithFallback` 兜底，正文降级为纯文本而非白屏 |
| 图片 404 | 显示 alt 文本，不留破图占位 |
| 后端版本过旧（无 docs 端点） | 端点 404，sidebar「文档」项隐藏，其余项不受影响 |
| 锚点不存在 | 落到页面顶部，不报错 |
| 未登录 | 与其他产品 API 一致要求鉴权；设备鉴权模式天然可用 |

## 实现边界

### 内容管道

新增 `scripts/generate-docs-bundle.mjs`。读 `apps/docs/content/docs/*.zh.mdx` 和 `meta.zh.json`，产出到 `server/internal/docs/content/`：每页一个 JSON（`slug`、`title`、`description`、`body`、`toc`），一个 `manifest.json`（导航树 + 页面标题/描述），图片从 `apps/docs/public/images/docs/` 复制过去。产物提交进仓库，CI 校验新鲜度（生成后 `git diff --exit-code`），沿用 `generate:reserved-slugs` 的既有约定。

JSX 降级为 remark 指令在这一步做，**这是全案最容易做错的一处**。remark 默认不解析 HTML 块内部的 markdown，所以把 `<Callout>` 直接降级成 `<div>` 会让 292 个提示框里的链接、加粗和行内代码原样显示。必须转成容器指令，由 `remark-directive` 解析：

```
<Callout type="warning">见 [运行时](/daemon-runtimes)</Callout>
   ↓
:::warning
见 [运行时](/daemon-runtimes)
:::
```

`type` 只有三种取值，其中 `warn`（12 次）与 `warning`（180 次）合并。`<VideoEmbed>` 和 `<CommunityLinks>` 降级为叶子指令（`::video-embed{provider="bilibili" id="BV1..."}`）。生成器对未知 JSX 标签必须**报错退出**，不能静默丢弃——否则文档站新加一个组件时，应用内会安静地少一段内容。

### 服务端

`internal/docs/embed.go` 持有 `embed.FS`。这个包只能被 server 导入，不要让 `cmd/multica` 的 CLI 二进制也背上 4.5MB。

`internal/handler/docs.go` 提供三个端点，在 `cmd/server/router.go` 注册：

```
GET /api/docs/manifest              → 导航树 + 页面标题/描述 + serverVersion，带 ETag
GET /api/docs/page?slug=agents      → 单页
GET /api/docs/assets/images/docs/*  → 图片，长缓存
```

`slug` 必须用 manifest 的白名单校验，不能拼接文件路径——嵌入 FS 也一样，`embed.FS` 不阻止 `..` 之外的越权读取自身内容。manifest 带 `serverVersion` 让客户端可以显示「文档对应 x.y.z」。

### 客户端数据层

`packages/core/docs/`：`paths.ts`（`docsHref(slug, anchor?)`）、`schema.ts`（zod + `parseWithFallback`，按 `CLAUDE.md` 的 API 兼容规则，并配 malformed-response 测试）、`queries.ts`（TanStack Query；文档是服务端状态，不进 Zustand）。

### 渲染与路由

`packages/views/docs/`：`docs-page.tsx`（布局 + 目录树）、`docs-content.tsx`（渲染器）、三个指令组件。

**不动 `RichContent`。** 它的 docstring 明确写了自己是产品内容的唯一渲染器、API 要保持窄、不能再长出 surface 专属分支。文档是第二种内容类型，走自己的组件树，共用 `packages/ui/markdown` 这个下层原语。

渲染时把图片 src 从 `/images/docs/x.webp` 重写成 `${apiUrl}/api/docs/assets/...`：桌面端在 `file://` 上，根相对路径解析必然失败。heading id 用 github-slugger（Fumadocs 用的同一个）生成。locale 前缀语义复用 `apps/docs/lib/locale-link.ts` 的 `prefixLocale`。

平台接线：`apps/web/app/[workspaceSlug]/docs/` 和桌面端 `routes.tsx` 各加一条 session route。路由放 workspace 作用域而非根 `/docs`，因为 web 根路径的 `/docs` 一旦配了 `DOCS_URL` 就被 beforeFiles rewrite 在 Next router 之前接走。`reserved_slugs.json:40` 已保留 `docs`，无需改动。

### 入口与 i18n

`help-launcher.tsx` 的「文档」项改为应用内跳转。`help-launcher.test.tsx` 现在断言 `DOCS_URL` 外链，要一并改。

内容 P0 只出中文；**UI 文案仍需四语言齐全**——`packages/views/locales/{en,zh-Hans,ko,ja}/docs.json` 新增命名空间（目前 25 个，没有 docs），并注册到 `packages/views/i18n/resources-types.ts`。文案覆盖目录页标题、错误态、空态、404 和「文档对应版本」。zh 之外的 locale 打开文档时读到的是中文正文，这是 P0 的已知边界，文案里要讲清楚而不是假装没有。

### 智能体的文档访问

`onboarding_shim.go:79` 让智能体 WebFetch 公网文档，内网下必然失败，改为读本部署的 docs 端点。`install-runtime-issue.ts` 那 4 处会落进 issue 正文的 URL 同理。

## 配置与安全约束

- 鉴权与其他产品 API 一致。文档不含机密，但没必要扩大未认证面。
- `slug` 与资产路径都走白名单，不做字符串拼接。
- 渲染保留现有 `rehype-sanitize`。内容是一等来源，但管道不能因此变成 HTML 直通。
- `DOCS_URL` 现有 web 行为不动。应用内文档覆盖之后它只剩「operator 想额外挂一份完整站点」的意义，去留放到 P2 决定——现在删掉会打破一个已有测试覆盖的行为。
- 生成产物只读，运行时不写盘、不落缓存目录。

## 测试与验收

- 生成器：Callout 三种 type、体内 markdown、frontmatter 缺失、`meta.zh.json` 分组分隔符、未知 JSX 标签必须报错退出。
- bundle 新鲜度（CI）：重新生成后无 diff。
- 覆盖完整性：`meta.zh.json` 里每个 slug 都能解析到页面。
- **锚点 parity**：产品代码里出现的每个锚点都能在 bundle 里找到对应 heading，含现存的 `#事件过滤`（`webhook-event-filter-section.tsx:22`）和 `#自定义运行时配置`（`runtime-docs.ts`）。这两个已被 `runtime-docs.test.ts` 和 `telegram-tab.test.ts` 按 `multica.ai` 形态断言，改链接时这些断言会红，必须一起改。
- Go：三个端点的正常与异常路径、slug 白名单拒绝越权、ETag 命中。
- core：`docsHref` 输出、malformed-response 兜底。
- views：Callout 内 markdown 正确渲染、图片 src 重写、内链走 navigation 而非整页跳转。
- 桌面端手工验收：sidebar footer 入口 → 开 tab → 图片加载 → 内链跳转 → 锚点定位，走完整链路。
- `pnpm typecheck`、`pnpm lint`、`pnpm test`、`make test`。

## 分阶段发布

1. **P0（本任务）**：内容管道、后端 embed、core 解析器、views 渲染、两端路由、sidebar footer 入口重指、9 处死链替换、智能体文档访问。做完内网不再有死链。
2. **P1**：搜索（把 `apps/docs/app/api/search/route.ts` 的 CJK / 日文 tokenizer 抽成共享纯函数模块，两边共用，避免分词规则漂移；45 页规模下客户端建索引即可，不需要新后端设施）、正文 ToC、目录树折叠与当前项高亮。
3. **P2**：其余三个 locale 的正文、mobile、版本偏移提示、`DOCS_URL` 去留。
