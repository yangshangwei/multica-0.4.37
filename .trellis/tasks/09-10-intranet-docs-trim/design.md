# Design — 内网部署文档裁剪

## 1. 改动面总览

| 层 | 路径 | 改动 |
|---|---|---|
| 文档源 | `apps/docs/content/docs/*.zh.mdx` | 删 9 个文件、改写 4 个、局部裁剪 7 个 |
| 导航 | `apps/docs/content/docs/meta.zh.json` | 同步删除条目；清理空分组（"社区"整组、"集成"只剩 vcs-integration） |
| Bundle 产物 | `server/internal/docs/content/` | 由生成脚本重建（pages/*.json、manifest.json、assets） |
| 产品深链 | `packages/core/docs/slugs.ts` | 删 `slackBot`/`telegramBot`/`dingtalkBot` |
| 设置页 | `packages/views/settings/components/{slack,telegram,dingtalk}-tab.tsx` | 移除"查看文档"链接分支 |
| 帮助菜单 | `packages/views/layout/help-launcher.tsx` | 删"更新日志"与"下载桌面版"外链项 |
| 引导模板 | `packages/views/onboarding/templates/install-runtime-issue.ts` | 删公网 curl 命令与外链 |
| 测试 | 上述组件的 `.test` 文件 + `apps/docs/lib/docs-bundle/docs-anchor-parity.test.ts` | 同步更新 |

## 2. 关键决策

### D1 删除方式：删源文件 + meta 条目，而非生成器加开关

`buildNav()` 双向强校验（meta 有页无 → 报错；页有 meta 无 → 报错"unreachable in the sidebar"）。唯一正确做法是两处同步删。不加 include/exclude 开关：内网部署是本 fork 的唯一形态，开关是兼容层（CLAUDE.md 禁止）。

### D2 channels 页删除而非保留机制说明

五个平台全部不可达；"消息通道暂不可用"这类占位页对内网用户没有信息量。若产品后续支持内网 IM，再按需新增页面。集成分组保留 `vcs-integration`（内网核心页）。

### D3 渠道设置 Tab：只删文档链接，不动渠道功能

渠道 Tab 的功能代码（密钥配置、连接状态）是另一个任务的范围。本任务只移除指向已删文档页的"查看文档"链接，避免死链。注意三个 Tab 的链接写法是条件渲染（`workspaceSlug ? docsPage(...) : ...`），删除链接分支而非整段 JSX，保持 Tab 其余行为不变。

### D4 help-launcher：删两个公网菜单项

- "更新日志"（multica.ai/changelog）：内网死链，直接删除菜单项。
- "下载桌面版"（multica.ai/download）：同理删除。原代码注释解释了为何保留外链（release assets 不在部署内），该注释随菜单项一并更新——内网部署下这两个入口没有意义。
- "文档"（站内路由）与"反馈"（本部署 `/api/feedback`）保留。

### D5 改写页的公网链接处理原则

- **必需性依赖**（安装命令、clone 地址、镜像拉取）：必须替换为内网等价物。内网等价物没有唯一答案（各公司镜像地址不同），写法用占位符 + 说明（如 `<内网镜像仓库>/multica`），并加一句"由管理员提供"。
- **参考性外链**（工具官方文档）：可保留为普通链接，但需明确标注"内网不可达，仅离线参考"。install-agent-runtime 的安装链接表改为列出工具名 + 检测命令，链接列改为说明性文字，避免一张 26 个死链的表。

### D6 交叉引用修复点（删页后必查）

| 文件 | 引用 | 处理 |
|---|---|---|
| `self-host-quickstart.zh.mdx` | `cloud-quickstart` 第 3-5 步 | 把"快速上手第 3-5 步"的内容内联或改写为自托管语境步骤 |
| `index.zh.mdx` | `[完成第一次运行](/cloud-quickstart)` | 改指 `self-host-quickstart` |
| `tutorial.zh.mdx` | GitHub 建仓/PR、Desktop 下载 | 随 R2 改写 |
| `vcs-integration.zh.mdx` | 反向引用 github-integration？ | 实施时 grep 确认（扫描显示方向是 github→vcs，但需验证） |
| 其他保留页 | 被删 slug 的任意站内链接 | 实施时对全部 `.zh.mdx` grep 被删 slug |

站内链接形式：`/slug`、`/slug#锚点`。`resolveDocsHref` 把不存在的 slug 仍解析为 docs 路由，点击后 404——所以死链只能靠 grep 预防，没有运行时校验。

### D7 Bundle 产物与 CI

`server/internal/docs/content/` 是提交产物，CI 用它证明二进制内容未漂移。删除页面的 `pages/*.json` 随再生成消失；`assets/` 中被删页面独占的图片不会被自动清理（生成器只做增量 copy + 不删旧文件？——实施时验证：生成器 `rmSync(outputRoot)` 后全量重建，所以会自动清理）。manifest.json 的 groups 随 meta 重建。

## 3. 风险

| 风险 | 缓解 |
|---|---|
| 遗漏交叉引用 → 站内 404 | 验收标准含"被删 slug 引用为零"的 grep 检查 |
| `DOCS_SLUGS` 删条目后 parity 测试或设置页测试失败 | 测试更新列入 R4，跑 `pnpm test` 全量验证 |
| 改写文案不符合产品中文风格 | 遵循 `developers/conventions.zh.mdx`；改写页 diff 由人工 review |
| 生成器/嵌入层 Go 测试依赖具体页面 | `server/internal/docs` 有 `embed_test.go`/`binary_scope_test.go`；跑 `make test` 验证，必要时同步调整 |
| Go bundle 与 TS manifest 形状测试锁死分组标题 | meta 分组名变化（"社区"组消失）若被测试断言，同步更新 |

## 4. 回滚

纯内容 + 少量前端代码改动，无 DB、无 API 变更。`git revert` 整个提交即可。
