# 添加电脑实现验收

2026-10-11；按已批准设计完成共享 Web/Desktop 功能实现。

## 交付

- `packages/views/runtimes/components/connect-remote-dialog.tsx`：首次不预选系统，三系统终端提示、两步命令、复制反馈、地址/安装缺项、分段令牌帮助、等待/迟迟未连接/恢复重试，以及真实电脑成功详情。
- `connect-remote-commands.ts`、`connect-remote-machine.ts`：分别集中命令与地址校验、电脑身份与归属判断，删除原来由通知 payload 直接切换成功的路径；不新增持久状态或依赖。
- `packages/core/api/{client,schemas}.ts`、`config/index.ts`、`platform/auth-initializer.tsx`：按系统配置链路、旧响应兼容、清理旧值、可靠运行时列表解析。
- `packages/core/docs/slugs.ts`、两套 runtimes 文案，以及对应回归测试。
- `server/internal/handler/config.go`、`.env.example`、Compose/Helm/离线分发配置，以及中英文安装/部署说明和三份重新生成的部署内文档。
- `.trellis/spec/views/frontend/connect-remote-computer.md` 与规范索引、离线交付规范。

## 自动检查

最终主会话执行：

```bash
pnpm typecheck
pnpm --filter @multica/core lint
pnpm --filter @multica/views lint
pnpm --filter @multica/core exec vitest run api/schema.test.ts api/client.test.ts config/index.test.ts platform/auth-initializer.test.tsx --maxWorkers=2 --testTimeout=15000
pnpm --filter @multica/views exec vitest run runtimes/components/connect-remote-dialog.test.tsx runtimes/components/connect-remote-commands.test.ts runtimes/components/connect-remote-machine.test.ts locales/parity.test.ts --maxWorkers=2 --testTimeout=15000
git diff --check
```

- 全仓 9 个包类型检查通过，包含 Web、Desktop 和 Docs；最终 core/views 定向检查亦通过。
- 核心 276 项、弹窗/辅助函数/语言对齐 128 项，共 404 项定向测试通过。
- core lint 无警告；views lint 为 0 错误、27 条未修改文件中的既有警告；最终修改文件的 ESLint 无错误。
- 独立检查还覆盖既有电脑分组回归和文档目录，最终 86 项 UI/helper/machine、4 项目录测试通过。

配置实现者执行且检查者复核：

- 独立已迁移数据库中，20 项 `TestGetConfig|TestURLHostEquals` 和 12 项 Docs handler 测试通过，未因数据库缺失跳过；使用 `scripts/go-test-with-agent-cli-guard.sh`。
- `go test ./internal/docs/...`、`go vet ./internal/handler` 和修改文件 `gofmt` 检查通过。
- `bash scripts/selfhost-config.test.sh` 通过；临时独立 Helm v3.19.0 的官方 SHA-256 已核验，`scripts/helm-config.test.sh` lint/render 通过。
- `node --test scripts/offline-changelog.test.mjs scripts/offline-build-changelog.test.mjs`：58 项通过，3 项需明确开启的真实镜像测试跳过。
- `node scripts/generate-docs-bundle.mjs`：37 页、58 个资源；仅预期的 CLI、环境变量、自托管快速入门三份页面有变更。
- Docs 生成器/锚点 5 项测试通过；脚本语法、全局 diff 空白检查通过。

## 浏览器和视觉

使用独立源码快照的 Next.js 16.2.6 `next build --webpack` + `next start`，
最终构建 ID 为 `il_bB-nDE36RiFnsyMd5g`；37 个有改动的构建相关文件与主工作区逐字节一致。
API 与 WebSocket 使用隔离浏览器 fixture，未写入真实账号、工作空间或运行时。

`browser-check.mjs` 的最终中英文流程均通过：首次未选、键盘选择、三系统命令、完整 setup 剪贴板、分段配置/交互登录/启动剪贴板、390px 滚动到帮助末尾与可见页脚、Escape 返回触发按钮焦点、回环地址、内网 Windows 安装缺项/显式配置、控制字符地址、缺少工作空间的首次列表恢复、畸形通知、旧电脑新增 provider、HTTP 503/重试，以及新电脑实际名称/系统/运行时成功详情。两套流程各观察到 47 次运行时查询，页面异常均为零。

浏览器报告、22 张截图、源码哈希和构建启动记录位于
`.omx/reports/add-computer-platform/`。与原图对照的视觉评分 94/100，通过；
Impeccable detector 返回空结果。最终源码复验保持相同界面，窄窗口可滚动到所有帮助命令且页脚可操作。

## 检查发现与修正

1. 控制字符地址在 `trim()` 后误判为无配置：先检查原始输入，再决定云端默认值。
2. `127.api.example.com`、`0.packages.internal` 被 IPv4 前缀规则误拒绝：仅对规范化的 IPv4 地址执行该检查。
3. 缺失 `workspace_id` 被降级为空字符串，形成不完整电脑基线：在 API 边界拒绝缺失、空白或畸形工作空间身份。
4. 帮助链接绕过文档目录校验：注册并使用共享 `DOCS_SLUGS`。

前 3 项均验证了失败到修复通过的回归；其他旧版可选显示字段和未知枚举仍保持兼容。

## 验证边界

未执行三系统真实双机安装、真实 Agent CLI、真实账号令牌登录、原生 Electron 运行或辅助技术验收。Web 浏览器覆盖真实共享组件；Desktop 由相同组件、路径/导航测试和类型检查覆盖。离线检查未运行三个 opt-in 真实镜像用例，也未发布或部署。

电脑识别保守地保留主机名身份，避免重装 daemon 或新增 provider 被误报；同名新电脑或首次可靠基线取得前已经出现的电脑可能继续显示等待，可在电脑列表中确认。这是缺少单次接入标识时的既定边界。
