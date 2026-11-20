# #38 验收记录：可读地址替换保留当前事项的历史展开

冻结 Web 回归、原 E2E、真实 Electron 渲染器及 #37 联验均已通过。独立最终审查无剩余发现；本记录不将单测或原用例的修前通过当作失败复现。

## 根因与修前证据

历史 trace 的 `call@674` 在 UUID 地址点击历史，面板已展开并显示 Loading；`call@676` 等待链接结束时，地址变成同事项的可读编号，面板关闭。可读页面请求早于点击开始，随后才提交替换。此次没有重新操作原 trace。

新隔离环境先运行原 `iterations-i1-history.spec.ts` 标题，结果 1/1 通过；它不是 RED。冻结新 Web 文件分别扣住真实可读页面的 RSC 响应和该事项的参与历史读取，在实际 summary 点击事件记录 UUID pathname，先放行页面、仍扣住历史，再断言面板保持展开。

同一新文件 SHA256 `d314e1229304b706ee6c8248eae907a102d3e7889d354aa04a37e11e8a21e80f` 在原 #38 产品上连续两次稳定失败，均命中第 169 行：可读页面已提交，当前原生 `details.open=false`。旧元素已断开连接，确认路由父段重挂载。本地组件已有 workspace/UUID key，无法跨父路由卸载保存状态。A→父事项 B 的真实站内负对照两次均通过。

真实 NavigationProvider、canonical query 与路由 key 重挂载的 Views 回归也连续两次失败于第 84 行，嵌入式本地展开对照保持通过。早期浏览器 fixture 的两个父链接定位冲突已独立保留为 fixture 证据，不计为产品 RED。

## 实际修改与归属

八个产品文件：

- `packages/core/issues/stores/iteration-participation-store.ts`：每个挂载的导航表面保存当前工作区/UUID 的一条临时展开意图，包含源/规范 pathname 与 generation。
- `packages/core/issues/stores/index.ts`：导出该 Core store 及生命周期函数。
- `packages/core/platform/session-cleanup.ts`：现有会话/Server 清理入口重置所有已挂载的表面并推进 generation。
- `packages/views/navigation/context.tsx`、`index.ts`：Context 只注入 Core StoreApi；挂载时注册，卸载时注销，pathname 提交时同步。
- `packages/views/issues/components/issue-detail-route.tsx`：解析真实身份、先选择当前记录再规范化地址，给路由详情传受控展开值及回调。
- `packages/views/issues/components/issue-detail.tsx`：向普通事项的参与历史传递该受控值，保留既有工作区/事项组件 key。
- `packages/views/iterations/iteration-participation.tsx`：原生 open 与 Query enabled 使用同一个有效布尔值；非路由详情继续使用本地展开状态。

状态只属于当前事项，切 B、离页、登录、身份变化、当前事项不可用或会话结束会清除；旧 generation 的回调不能恢复上一条记录。历史行、权限、取消和重取继续归 Query，游标继续本地持有。没有修改 NavigationAdapter 类型、Next/Desktop 接线、权限或后端协议，没有本地持久化或所有已访问事项的状态表。

测试文件：新增 Core store 生命周期矩阵、补现有 DOM-aware session-cleanup 集成测试、新增 Views 路由重挂载与 embedded 对照；新增冻结 Web 双例与 Electron A→B 单例。相关文件为 `iteration-participation-store.test.ts`、`session-cleanup.test.ts`、`issue-detail-route.iteration-history.test.tsx`、`e2e/iteration-history-navigation.spec.ts`、`e2e/iteration-history-navigation-desktop.spec.ts`。原两个 E2E 保持原字节。

Core `iteration-operations.md` 与 Views `state-management.md`/索引记录这些可执行边界。产品与测试的完整指纹见 `38/green/owner-verification-manifest.json`、`38/native-test/ready.json`。

## 环境、命令与结果

证据根目录为 `.artifacts/e2e-fix-20261119-002358-fc3f5ec4/`。私有源码目录是普通同级目录 `/Volumes/artisan/code/2026/multica-e2e-fix-20261119-002358-fc3f5ec4`；起始 HEAD 为 `72c207f72d67af979e6f6222a44d397036f4c79b`。全新库为 `e2e_fix_37_38_20261119_002358_fc3f5ec4`，API/Web 端口为 54150/54151，独立 HOME、上传及临时目录。

#38 修前 Web 构建为 #37 最终构建 `UkGXK1ed0UWW3CgSnEhGw`；两项最终合并构建为 `C2Kji8hfWmojMSLxyjsnu`。已核对 9014 个源码输入、protected SHA、API 健康身份、legacy auth、数据库 ready 与实际 CSS/构建一致。旧成功构建保存在本轮 `builds/37-preserved-final-web`。

Web 使用 `next build --webpack`；Desktop 使用 `electron-vite build --outDir <本轮绝对输出目录>`。二者都退出 0。Electron 使用真实编译 preload、renderer、router 与 shell，通过已有 `project-p1-desktop` 的本地 HTTP/WS 代理接入私有 API。受控 main fixture 禁止 daemon/agent 启动；验收没有启动智能体。

| 阶段 | 证据相对根目录 | 结果 |
| --- | --- | --- |
| 原标题修前运行 | `38/red/original` | 1 passed；不是 RED |
| 冻结 Web RED ×2 | `38/red/stable-browser-01`、`stable-browser-02` | 同一第 169 行失败；A→B 对照通过 |
| 路由重挂载 RED ×2 | `38/red/unit/route-remount-final-01`、`-02` | 第 84 行失败；embedded 对照通过 |
| 路由重挂载 GREEN | `38/green/unit/route-remount-types-correction-02` | 2/2 |
| Core 三个相关文件 | `38/green/unit/core-focused-01` | 26/26 |
| Views 五个相关文件 | `38/green/unit/views-related-01` | 53/53 |
| Web navigation | `38/green/unit/web-navigation-01` | 15/15 |
| Desktop navigation / issue-window | `38/green/unit/desktop-navigation-01` | 38/38 |
| Core/Views lint、typecheck | `38/green/static/`、owner manifest | exit 0；Views 27 条既有警告在未改文件 |
| 匹配源码的两端构建 | `38/green/web-build`、`desktop-build`、`build-admission.json` | exit 0；Desktop 输出 495 个文件已记录指纹 |
| 冻结 Web 双例 GREEN | `38/green/targeted-browser` | 2/2，页面提交且历史仍扣住时 nativeOpen=true、旧元素已断开 |
| 原标题 / 原整文件 | `38/green/original`、`original-full-file` | 各 1/1，全部后续冻结/筛选/删除断言执行 |
| 既有 Electron 两整文件 | `38/green/native-related` | 5/5，英文/中文、冻结历史、设置反馈、原子结转 |
| 新 Electron A→B | `38/green/native-targeted` | 1/1，同文档 B 可读虚拟路由、面板关闭、零 B 历史读 |
| 三份新 E2E 严格类型检查 | `joint/green/e2e-typecheck` | exit 0 |
| #37/#38 原及新四文件联验 | `joint/green/web-final` | 14/14，0 skip/flaky/retry |
| 独立最终审查 | `38/check/final-review.json` | 8 个产品文件、测试、规范与原始检查输出指纹匹配，无剩余发现 |

所有 Playwright 运行使用 Chromium、`--workers=1 --retries=0 --reporter=line,json --trace=on` 与新结果目录；浏览器运行顺序串行，没有自动重试、超时放宽或断言削弱。命令、退出码、标题、输入 SHA 与 trace 路径见各阶段 receipt/report 和 `joint/verification-summary.json`。原 #37 文件自身的 retain-on-failure trace 策略仍保留。

## 验收范围

本项与 #37 的最后联验已通过；不代表原 52 个失败标题全体通过。未跑全量 E2E、打包后的 Electron main/安装器或真实智能体。Electron 本轮证明真实 renderer/preload/router 行为，不能扩大为安装包验收。

没有操作保留库、KEEP-1、原报告/trace、导出或备份。两条收集错误仍归 PAY-241 S3；#51/#52 仍与既有创建弹窗任务核对，#51 不构成跨区串值证据。只清理本轮拥有的进程，保留新库、私有源码及红绿证据。
