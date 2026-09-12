# 变更说明验收记录

验收日期：2026-09-13。范围：本任务的桌面页面、同部署日志 API、提交记录生成、发布与离线交付。需求与编号见 [prd.md](./prd.md)。

## 验收结论

本功能专项验收通过。没有遗留的本功能缺陷；独立代码与视觉评审均已批准。全仓库 E2E 收集存在一个未改动的历史插件测试阻断，单独记录如下，不能宣称 `make check` 全部通过。

| 标准 | 证据 | 结论 |
|---|---|---|
| A1 帮助入口、桌面标签、Web 路由、更新提示 | 真实 Electron 使用产品 preload、renderer、router；点击帮助进入页面，标题/图标、当前桌面版本、更新提示内部跳转均验证；12 项通知/外壳测试覆盖提前到达事件和无工作区状态 | 通过 |
| A2 阅读结构、语言、窄窗口、键盘 | 桌面/窄窗口/深色/Electron 截图；16 项 reader 测试；四语言资源与搜索/帮助测试；独立视觉评分 94/100，检测器无发现 | 通过 |
| A3 来源与版本真实性 | [官方来源](./research/official-reference.md)、[提交依据](./research/fork-change-summary.md)、[seed 来源](./research/seed-provenance.md)；最新稳定版按有效数字版本比较；复查远端 Releases 仍为空 | 通过 |
| A4 已打开页面与新会话看到发布 | 临时 Git 仓库生成 v0.0.1/v0.0.2，原子发布到真实 Go API；已打开页面在 60144 ms 后显示第二版，服务器 PID 84289 和启动时间不变，页面 timeOrigin 不变；新浏览器会话读到同一记录；公网访问被阻断 | 通过 |
| A5 加载、失败、兼容与恢复 | Go 文件/响应校验；Core malformed/unknown-enum/QueryObserver 测试；真实运行中将文件损坏，再恢复，并模拟 HTTP 失败；旧内容保留且提示，恢复后提示消失 | 通过 |
| A6 生成与累计历史 | 临时 Git 测试覆盖祖先选择、合并提交、dirty 隔离、trailers、同版本冲突、稳定/预发布、旧版本重跑与历史保留 | 通过 |
| A7 构建/发布/离线链路 | 43 项发布工具测试全部通过，包含真实 Docker/Compose、无宿主 Node、断网安装、目录引号/逗号、持久化与失败回滚；过期构建产物在远程写入前拒绝；工作流依赖图与同字节产物经过独立审查 | 通过 |
| A8 Trellis 与验证证据 | PRD、设计、实施清单、三个子任务、来源研究、两轮代码评审、视觉评审、本记录与变更边界审计齐备 | 通过（全库限制见下） |

## 实际执行的检查

- `pnpm typecheck`：9 个任务成功；后续受影响 Core/Views 检查成功。
- TypeScript 全量测试：8,109 项通过（core 1,947；views 5,250；desktop 592；web 260；docs 60）。一次并行运行中已有 MCP 测试超时；该文件单独 29 项及随后完整串行调度均通过，没有更改该测试或降低断言。
- `pnpm lint`：0 error；保留仓库已有 warnings，新增/修改功能文件的定向 lint 无问题。
- `go vet ./...`：通过。
- `make check` 的 Go 阶段：完整 Go 测试与 agent CLI guard 通过，包含 `pkg/agent`。最初继承的 Git 配置和本验收服务的 `LOG_LEVEL=error` 干扰了两个已有默认环境测试；清理测试子进程环境后通过，没有修改这些生产模块或测试。
- 后端专项：69 个 case；reader race 检查通过；最后的 router fixture 改为明确清空五个外部 provider key，避免依赖并行任务的 messaging 开关。
- 独立 closeout：Core 41 + Reader 16 + Diagnostic 15，共 72 项通过。
- `MULTICA_RUN_DOCKER_CHANGELOG_SMOKE=1 node --test scripts/*changelog*.test.mjs`：43 项通过，无失败/跳过；独立审查额外验证最终 CSV 挂载路径用例。
- `pnpm exec playwright test e2e/changelog.spec.ts e2e/changelog-desktop.spec.ts`：最终 3 项通过。使用独立结果目录与测试数据库，没有执行用户安装的智能体 CLI。
- `pnpm --filter @multica/desktop build`：通过。
- Web production build：在隔离的 Web 源码副本中执行同一 `next build --webpack`，复用现有依赖/共享源码；通过并包含 `/[workspaceSlug]/changelog` 路由。
- Helm lint/render、Node/Shell/YAML 检查、`git diff --check`：通过。
- 从暂存区单独导出 server 源码（不含并行任务的未提交改动），运行 changelog、handler、cmd/server 的 Changelog 测试：全部通过。独立提交不依赖其他任务的修改。

## 全仓库 E2E 的既有阻断

完整 `make check` 到达浏览器收集阶段后失败：

```text
SyntaxError: ... surface-document does not provide an export named buildSurfaceDocument
```

`e2e/plugin-surface-document.spec.ts:2` 仍导入 `buildSurfaceDocument`，而 `packages/views/plugins/surface-document.ts` 当前导出 `buildSurfaceFrameDocument`，输入契约也不同。这两个文件和 `playwright.config.ts` 的工作区 diff 为空，本任务没有改动插件行为或弱化旧断言。此问题不属于变更说明功能；全仓库 E2E 不能记为绿色。变更说明三项真实浏览器/Electron 测试已独立执行通过。

## 运行与发布边界

- Electron smoke 使用真实产品 preload、renderer、路由和外壳，原生 daemon/安装更新等与阅读无关的服务被替换为隔离测试端点；未启动真实模型任务、修改用户偏好或执行真实二进制升级。
- 开发服务器曾缓存新增包导出前的元数据。按项目环境脚本仅重启 Desktop 组件后，`/src/routes.tsx` 从 500 恢复 200；保留原数据库与用户配置，当前桌面开发环境正常。
- 没有创建真实生产 tag、推送分支、发布生产 Release 或部署生产服务。自动化路径由本地 Git/HTTP fixture 和真实离线容器验证。
- 内网发布产物仍须经现有交付渠道到达部署；安装/升级脚本负责校验和发布，客户端随后自动刷新。
- GitHub 多附件替换不是整体事务；中断恢复与重新准备/构建见 `.github/RELEASING.md`。不会用回退 seed 覆盖已有累计历史。

## 本地证据位置

`.omx/state/desktop-changelog/` 保存 `live-publication-evidence.json`、`changelog-e2e-verified.log`、构建/类型/lint/Go 日志、截图、`design-detector.json`、`ralph-progress.json` 和 `staging-summary.md`。临时服务、数据库和合成版本与用户实际部署分离；合成 feed 在测试后恢复。

评审：[后端与页面](./research/code-review.md)、[发布链路](./research/release-code-review.md)、[视觉](./research/visual-review.md)。
