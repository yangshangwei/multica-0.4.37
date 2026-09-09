# 应用内文档（P0）

## Goal

让内网部署的桌面端和 web 读到与本部署版本一致的文档，消除产品内全部指向 `https://multica.ai/docs` 的死链。内容保持 `apps/docs/content/docs/` 单一来源，不 fork，不新增运维组件。完整设计见仓库根 `docs-in-app-design.md`。

## Requirements

1. **内容随后端分发。** 文档内容由构建期生成器产出，`go:embed` 进后端二进制。嵌入包只能被 server 导入，`cmd/multica` 的 CLI 二进制不得因此增重。
2. **生成器忠实降级 JSX。** `<Callout>`（292 次，三种 type，`warn` 与 `warning` 合并）降级为 remark 容器指令，体内 markdown 必须仍被解析；`<VideoEmbed>`、`<CommunityLinks>` 降级为叶子指令；遇到未登记的 JSX 标签必须报错退出，不得静默丢弃。
3. **产物可校验。** 生成物提交进仓库，CI 重新生成后无 diff（沿用 `generate:reserved-slugs` 约定）。
4. **端点最小且不可越权。** manifest / page / assets 三个端点。`slug` 与资产路径用 manifest 白名单校验，不做字符串拼接。manifest 带 `serverVersion`。
5. **响应按 API 兼容规则解析。** zod schema + `parseWithFallback`，配 malformed-response 测试。文档是服务端状态，走 TanStack Query，不进 Zustand。
6. **渲染不动 `RichContent`。** 文档走 `packages/views/docs/` 自己的组件树，共用 `packages/ui/markdown` 下层原语。图片 src 重写为 `${apiUrl}/api/docs/assets/...`（桌面端在 `file://` 上，根相对路径解析必然失败）；heading id 用 github-slugger；内部链接走 `useNavigation().push()`。
7. **寻址收敛到一处。** `packages/core` 出 `docsHref(slug, anchor?)`，`packages/views` 9 处硬编码全部改为调用它。
8. **入口重指而非新增。** `help-launcher.tsx:25` 的「文档」项改为应用内跳转并去掉外链字形；「更新日志」「桌面端」保持外链（其发布资产不在本部署内）。
9. **智能体文档访问改内网。** `onboarding_shim.go:79` 的 WebFetch 指令与 `install-runtime-issue.ts` 落进 issue 正文的 4 处 URL 改指本部署。
10. **内容 zh-only，UI 文案四语齐全。** `locales/{en,zh-Hans,ko,ja}/docs.json` 新增命名空间并注册进 `i18n/resources-types.ts`；非 zh locale 读到中文正文是已知边界，文案里讲清楚。
11. 不加数据库迁移、不改 wire shape、不动既有路由路径、不动 `DOCS_URL` 现有 web 行为、不碰 mobile。

## Acceptance

- [ ] 生成器测试：Callout 三种 type、体内 markdown（链接/加粗/行内代码）、缺 frontmatter、`meta.zh.json` 分组分隔符、未知 JSX 标签报错退出。
- [ ] bundle 新鲜度：CI 重新生成后 `git diff --exit-code` 干净。
- [ ] 覆盖完整性：`meta.zh.json` 里每个 slug 都能解析到已生成页面。
- [ ] 锚点 parity：产品代码出现的每个锚点都能在 bundle 里找到对应 heading，含现存 `#事件过滤`、`#自定义运行时配置`。
- [ ] Go：三端点正常与异常路径、slug 白名单拒绝越权、ETag 命中、CLI 二进制未嵌入文档内容。
- [ ] core：`docsHref` 输出、malformed-response 兜底。
- [ ] views：Callout 内 markdown 正确渲染、图片 src 重写、内链走 navigation 而非整页跳转。
- [ ] `runtime-docs.test.ts`、`telegram-tab.test.ts`、`help-launcher.test.tsx` 三处按 `multica.ai` 形态的断言已同批改。
- [ ] 桌面端手工验收：sidebar footer → 开 tab → 图片加载 → 内链跳转 → 锚点定位，走完整链路。
- [ ] `pnpm typecheck`、`pnpm lint`、`pnpm test`、`make test` 通过。

## Source

仓库根 `docs-in-app-design.md`；本轮对 `apps/docs`（45 页 × 4 语言 / 1.9M 正文 / 2.6M 图片 / 三个 JSX 组件）、`apps/web` 的 `DOCS_URL` 代理链路、桌面端 `runtime-config` 与 `renderer-web-preferences`、`packages/ui/markdown` 渲染栈的实地勘察。
