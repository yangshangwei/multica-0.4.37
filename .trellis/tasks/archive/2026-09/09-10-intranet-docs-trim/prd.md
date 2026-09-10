# 内网部署文档裁剪：删除/改写内网不可用文档页

## Goal

将内嵌中文文档（`apps/docs/content/docs/*.zh.mdx` → `server/internal/docs/content` bundle）裁剪为纯内网部署可用版本。本部署的服务器、桌面客户端与浏览器均无法访问公网；保留指向公网资源的内容对内网用户是死链或误导。

## Background

- 内嵌文档机制：`scripts/generate-docs-bundle.mjs` 只打包 `*.zh.mdx` + `meta.zh.json`，产物提交进仓库，由 Go 二进制 embed。`Page()`/`Asset()` 按 manifest 白名单提供访问。
- `buildNav()`（`apps/docs/lib/docs-bundle/parse-page.mjs`）对 meta 与页面双向强校验：删页必须同时删 `.zh.mdx` 文件与 `meta.zh.json` 条目。
- 产品代码深链 8 个 slug（`packages/core/docs/slugs.ts`），`docs-anchor-parity.test.ts` 校验两个中文标题锚点（"自定义运行时配置"、"过滤事件"）。
- 渲染层（`packages/views/docs/`）无自动外载，公网入口只有内容中的 `VideoEmbed` 与 `CommunityLinks` 指令。

## User Decisions（2026-09-10 确认）

1. 内网对飞书/钉钉无出站 → 四个 IM bot 页全部删除。
2. 无 GitHub Enterprise → `github-integration` 删除（由 `vcs-integration` 承接）。
3. 无内部 iOS 构建链路 → `mobile-app` 删除。
4. `channels` 页一并删除：五个 IM 平台（飞书/Slack/钉钉/企微/Telegram）在内网全部不可达，"集成"分组只留 `vcs-integration`。（倾向性方案已告知用户，用户未反对；如需保留说明页，用户会另行提出。）

## Requirements

### R1 整页删除（9 页）

删除文件 + `meta.zh.json` 条目：

- `slack-bot-integration`、`telegram-bot-integration`、`lark-bot-integration`、`dingtalk-bot-integration`
- `community-maintained`（"社区"分组整组移除）
- `github-integration`
- `cloud-quickstart`
- `mobile-app`（"客户端"分组只剩 `desktop-app`）
- `channels`

### R2 整页改写（4 页，骨架保留，公网获取通道替换为内网方式）

- `self-host-quickstart`：git clone / GHCR 镜像拉取 / curl 安装 CLI / SELF_HOSTING.md 外链 → 内网镜像仓库与内网分发；修复对 `cloud-quickstart` 的交叉引用；"公网"措辞按内网语境调整。
- `install-agent-runtime`（被产品深链，不可删）：公网"官方安装说明"外链表 → 标注内网离线安装/内网镜像方式；补充自定义 endpoint/私有模型服务的说明。
- `tutorial`：GitHub 建仓/PR 主线、`pnpm dlx shadcn@latest`、Unsplash 占位图 → 内网 Git + 内网包源示例。
- `desktop-app`：multica.ai/download 安装、自动更新源、GitHub releases 校验、微软误报申诉 → 内网离线分发；保留"连接自托管实例"等章节。

### R3 局部裁剪（7 页）

- `index`：删 `VideoEmbed`（B 站）与 `CommunityLinks` 段；"从这里开始"改指 `self-host-quickstart`。
- `auth-setup`：删 Resend 与 Google OAuth 章节；保留 SMTP（含内网匿名 relay）章节。
- `environment-variables`：Resend / Google OAuth / CloudFront / Composio 行删除或标注内网不可用；PostHog 给出 `ANALYTICS_DISABLED=true` 指引；保留服务端 LLM（`MULTICA_LLM_BASE_URL` 指内网推理服务）与 S3/MinIO 内网说明。
- `skills`：删/改"从 URL 导入"（GitHub/ClawHub/Skills.sh）与"更新已导入的 skill"两节。
- `cli`：安装章节（curl/brew）改内网分发；处理 `skill import --url` 与 Multica CLI skill 段落。
- `troubleshooting`：GitHub Issues 链接 → 内网支持渠道占位；Resend 排查分支按邮件方案删减。
- `developers/contributing`：clone/install 命令指向内网镜像。

### R4 产品代码连带处理

- `packages/core/docs/slugs.ts`：删除 `slackBot`、`telegramBot`、`dingtalkBot` 三个条目（其页面已删）。
- 设置页 `slack-tab.tsx`、`telegram-tab.tsx`、`dingtalk-tab.tsx`（`packages/views/settings/components/`）：移除指向已删文档页的"查看文档"链接；渠道功能代码本身不动（超出本任务范围）。
- `packages/views/layout/help-launcher.tsx`：删除"更新日志"（multica.ai/changelog）与"下载桌面版"（multica.ai/download）两个内网死链菜单项；保留"文档"（站内）与"反馈"（本部署 API）。
- `packages/views/onboarding/templates/install-runtime-issue.ts`：移除公网 curl 安装命令与 Codex/Kimi 官网外链，文档指引保留站内 `/docs/install-agent-runtime`。
- 同步更新受影响的测试（`help-launcher.test.tsx`、settings tab tests、onboarding 模板测试、`docs-anchor-parity.test.ts`）。

### R5 Bundle 再生成

运行 `node scripts/generate-docs-bundle.mjs`，提交再生成的 `server/internal/docs/content/`（被删页面的 JSON 同步消失）。

## Constraints

- 仍被产品深链的页面不得删除：`agents`、`autopilots`、`daemon-runtimes`、`skills`、`install-agent-runtime`。
- 不得改写标题"自定义运行时配置"（daemon-runtimes）与"过滤事件"（autopilots），锚点一致性测试盯着它们。
- 英文/日文/韩文 `.mdx` 不动：它们只服务公网 docs 站，不进 bundle。
- 中文文案遵循 `apps/docs/content/docs/developers/conventions.zh.mdx`（CLAUDE.md 指定）。
- 不修改 `generate-docs-bundle.mjs` / `buildNav` 机制本身，不加裁剪开关。
- 不引入兼容层/降级路径：内网部署是唯一目标形态。

## Acceptance Criteria

- [ ] `node scripts/generate-docs-bundle.mjs` 成功，无孤儿页/meta 失配错误；产物已提交且与源一致。
- [ ] 内嵌文档导航（manifest）中不再出现 R1 的 9 个 slug；`Page()` 对它们返回 404。
- [ ] 保留页面中无指向已删 slug 的站内死链（对 `apps/docs/content/docs/*.zh.mdx` 与 bundle 产物 grep 验证被删 slug 引用为零）。
- [ ] `docs-anchor-parity.test.ts` 通过且仍覆盖 `DOCS_SLUGS` 全部条目。
- [ ] 设置页三个 bot Tab 无指向已删文档的链接；`help-launcher` 无公网菜单项；相关组件测试更新并通过。
- [ ] `install-runtime-issue.ts` 模板中无公网 URL。
- [ ] `pnpm test`（TS/Vitest）与 `make test`（Go，含 `server/internal/docs` embed 测试）通过。
- [ ] 改写后的 4 页无指向公网获取通道（multica.ai、github.com、raw.githubusercontent.com、npmjs.com、expo.dev、GHCR）的必需性依赖；公网外链若保留仅为参考性提及并明确标注内网不可达。

## Out of Scope

- 渠道（飞书/Slack/钉钉/企微/Telegram）功能代码的移除或禁用。
- 英文等多语言文档的同步裁剪。
- `apps/docs` 公网文档站的任何构建/部署调整。
- 服务端配置层面的网络策略调整（如 PostHog 关闭开关的服务端默认值）。
