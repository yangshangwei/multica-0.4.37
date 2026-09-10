# Implement — 内网部署文档裁剪

执行顺序按依赖排列：先删产品深链（避免删页后代码引用悬空），再删文档源 + meta，再改写/裁剪，最后再生成 bundle 并全量验证。每步后跑窄验证，最后跑全量。

## Step 1 — 产品代码深链与菜单清理

- [ ] `packages/core/docs/slugs.ts`：删除 `slackBot`、`telegramBot`、`dingtalkBot` 条目（保留 `agents`、`autopilots`、`daemonRuntimes`、`installAgentRuntime`、`skills`）。
- [ ] `packages/views/settings/components/slack-tab.tsx`、`telegram-tab.tsx`、`dingtalk-tab.tsx`：移除指向已删文档页的"查看文档"链接分支，Tab 其余行为不动。
- [ ] `packages/views/layout/help-launcher.tsx`：删除"更新日志"与"下载桌面版"两个菜单项及 `CHANGELOG_URL`/`DOWNLOAD_URL` 常量与相关 import；更新文件头/行内注释以反映内网形态。
- [ ] `packages/views/onboarding/templates/install-runtime-issue.ts`：移除公网 curl/PowerShell 安装命令与 Codex/Kimi 官网外链；保留站内 `/docs/install-agent-runtime` 指引。

验证：`pnpm typecheck`（类型捕获 DOCS_SLUGS 删条目的悬空引用）。

## Step 2 — 删除 9 个文档页 + meta 条目

- [ ] 删除 `.zh.mdx` 文件：`slack-bot-integration`、`telegram-bot-integration`、`lark-bot-integration`、`dingtalk-bot-integration`、`community-maintained`、`github-integration`、`cloud-quickstart`、`mobile-app`、`channels`。
- [ ] `apps/docs/content/docs/meta.zh.json`：删除对应条目；移除已空的"---社区---"分组头；"---集成---"分组只保留 `vcs-integration`；"---客户端---"分组只保留 `desktop-app`。
- [ ] grep 全部剩余 `.zh.mdx` 确认无被删 slug 的站内链接；已知修复点：
  - `index.zh.mdx`：`/cloud-quickstart` → `/self-host-quickstart`
  - `self-host-quickstart.zh.mdx`：对 `cloud-quickstart` 第 3-5 步的引用 → 内联改写
  - `vcs-integration.zh.mdx`：grep 确认是否反向引用 `github-integration`，有则删除该提法

验证：`node scripts/generate-docs-bundle.mjs`（双向校验通过即结构正确）。

## Step 3 — 整页改写（4 页）

- [ ] `install-agent-runtime.zh.mdx`：安装链接表改为"工具名 + 检测命令 + 内网安装说明"；公网官方链接降级为标注"内网不可达"的参考；补自定义 endpoint/私有模型服务说明。**不得改页面标题结构中被深链的部分**（本页无锚点深链，但保持 slug 不变）。
- [ ] `self-host-quickstart.zh.mdx`：clone 地址、`docker compose pull` 镜像来源、CLI 安装命令、SELF_HOSTING.md 外链 → 内网占位符（`<内网镜像仓库>/...`）+ "由管理员提供"说明；"公网"措辞改内网语境。
- [ ] `tutorial.zh.mdx`：GitHub 主线 → 内网 Git 服务示例（用 `<内网 Git>` 占位）；`pnpm dlx shadcn@latest` → 内网包源说明；Unsplash/picsum → 本地占位图说明。
- [ ] `desktop-app.zh.mdx`：安装/自动更新/校验章节 → 内网离线分发流程；保留"连接自托管实例"、守护进程、标签页章节。

验证：每页 grep `multica.ai|github.com|raw.githubusercontent.com|npmjs.com|expo.dev`，确认剩余出现均有"内网不可达/离线参考"标注。

## Step 4 — 局部裁剪（7 页）

- [ ] `index.zh.mdx`：删 `VideoEmbed` 与 `CommunityLinks` 段（含 import）；"从这里开始"改指 `self-host-quickstart`。
- [ ] `auth-setup.zh.mdx`：删 Resend、Google OAuth 章节；保留 SMTP/固定验证码/注册限制。
- [ ] `environment-variables.zh.mdx`：删/标注 Resend、Google OAuth、CloudFront、Composio 行；PostHog 行给 `ANALYTICS_DISABLED=true` 指引；保留服务端 LLM 与 S3/MinIO 说明。
- [ ] `skills.zh.mdx`："从 URL 导入"与"更新已导入的 skill"两节改为内网来源说明（本地导入 / `multica skill import --file` 为主路径）。
- [ ] `cli.zh.mdx`：安装章节改内网分发；处理 `skill import --url` 与 Multica CLI skill 段落。
- [ ] `troubleshooting.zh.mdx`：GitHub Issues 链接 → "联系内部支持渠道"；Resend 排查分支删减。
- [ ] `developers/contributing.zh.mdx`：clone/install 指向内网镜像。

顺手优化：`auth-tokens.zh.mdx` 示例域名 `api.multica.ai` → `api.example.com`（仅示例替换，无行为影响）。

验证：`node scripts/generate-docs-bundle.mjs` 再跑一次。

## Step 5 — Bundle 再生成 + 提交产物

- [ ] `node scripts/generate-docs-bundle.mjs` 最终运行；确认 `server/internal/docs/content/pages/` 中被删页 JSON 消失、manifest 分组正确。
- [ ] `git status` 检查 `server/internal/docs/content/` 变更完整（含删除文件）。

## Step 6 — 测试更新与全量验证

- [ ] 更新失败测试（预期影响面：`docs-anchor-parity.test.ts`、`help-launcher.test.tsx`、settings tab tests、onboarding 模板测试；Go 侧 `server/internal/docs` embed 测试如有分组/页面断言一并更新）。测试断言跟随产品行为变化，不放宽断言。
- [ ] `pnpm typecheck`
- [ ] `pnpm test`
- [ ] `make test`（Go，含 docs embed）
- [ ] 死链终检：对 `server/internal/docs/content/` grep 被删 9 个 slug，结果为零。

## Step 7 — 收尾

- [ ] Review gate：改写页文案 diff 人工过一遍（中文风格、占位符一致性）。
- [ ] Spec update（Phase 3.3）：若发现值得沉淀的约定（如"内网 fork 文档裁剪规则"），写入 `.trellis/spec/docs/`。
- [ ] Commit（原子提交建议按 Step 1 / 2+5 / 3+4 分组，conventional prefixes：`docs(...)` / `refactor(views)` / `chore(docs-bundle)`）。

## 回滚点

每个 Step 是独立可 revert 的提交边界；Step 2 之后 `generate-docs-bundle` 必须能跑通才能进入后续步骤。
