# 迭代设置与业务页最终独立复核

日期：2026-10-10。复核基线：`cfa0254ebf643924da9a20eacbecd50aac7d3fee`，加本报告所列设置权限重试修正。复核覆盖父任务 `10-08-iteration-workspace-settings` 与子任务 `10-08-iteration-pages-design`，并保留之后已提交的时区单入口、任务选择器、进展与范围变化增强。没有按旧原型回退后续实现。

**结论：聚焦复核发现并修复 1 项权限恢复缺陷；没有发现其他需要修改产品代码的阻断问题。** 本报告只证明列出的代码路径、组件和核心回归。最终同源码 Web/Electron 构建、Go 数据库验证、完整 `check.sh` 与视觉验收由主会话执行，结果应归入父子任务最终 verification；本报告不以旧浏览器通过记录替代该步骤。

## Findings (fixed)

### P2：启用被拒绝后，已恢复读取权限的设置页可能变为空白

- 文件：`packages/views/iterations/iteration-settings-tab.tsx`、`packages/views/iterations/iteration-settings-tab.test.tsx`。
- 触发：管理员发起启用时权限发生变化，服务端返回 403。核心命令正确清除受保护缓存和被拒绝请求，但设置页的 `denied` 继续读取持久的 `enable.error`。用户重试并成功读取 capability/settings 后，旧错误仍令 `readable=false`。若后台先恢复成功读取，query error 消失，页面还会失去 Retry 入口。
- 修正：将被拒绝的 enable error 作为错误/重试区域的后备来源；只有当前 capability 与 settings 都重新读取成功后，才 reset 该已拒绝 mutation。失败读取继续隐藏受保护内容。没有重发启用，没有改变权限、核心请求身份或未知结果恢复。
- 回归：新增同页“拒绝启用 → 一次离线重试 → 成功重试 → 只读设置恢复”和“后台重新读取成功 → 保留 Retry → 只读设置恢复”。两条都断言启用仅发送一次、没有请求操作回执。
- Red：`permission-retry-red.log` 在成功重试后找不到 switch；`permission-background-red-confirmed.log` 在 query status 已是 success 后找不到 Retry。后台测试第一次将两次 refetch 放在一个 React act 中，设置查询尚未重新启用，失败属于测试安排；已拆开 act，原始 `permission-background-red.log` 保留但不作为产品缺陷证据。
- Green：设置测试文件 19/19 通过；最终 views 聚焦回归 22 文件/391 项通过。

该修正删除“旧失败继续充当当前权限”的错误约束，复用既有 React Query reset 与页面 retry，无新增状态、请求、依赖或公共接口。

## 当前实现与验收映射

| 要求 | 当前代码事实 | 本轮验证/对应规范层 |
| --- | --- | --- |
| ITS-01、02；IP-11 | `settings-nav.ts` 在分拣台后注册迭代；共享 `SettingsPage` 使用现有 `?tab=iterations`；`IterationSettingsTab` 使用 Settings 组件，业务页只有设置链接。 | settings-nav、settings-page、settings-tab、navigation、locale parity 测试。 |
| ITS-03、05 | handler 能力返回 implemented supported/manual/atomic_handoff；enabled 来自保存设置。service 默认 false/revision 1，enable 只更改设置并确认保存时区；部署 gate/Available 已移除。 | 阅读 handler/service/main 与部署文件；core schema/client 回归。Go default/preserved-state/replay/authority 测试存在，由主会话本轮 DB 管线执行。 |
| ITS-04、11；IP-10、12 | 设置 UI 限人类 owner/admin；服务端在事务内重新确认稳定人类与角色。周期维护仍是人类成员能力。恢复组件在 enabled/support/detail tab 外挂载，核心按 actor/server/workspace 保存完整原命令并使用授权 epoch。 | settings-tab（含新拒绝恢复）、command、access、prepare、operation、navigation、recovery 测试；未改变服务器权限。 |
| ITS-06、07 | disable 读取当前版本并向服务端请求完整预览；原因/确认、complete/invalid/等待锁定沿用既有流程。后端 active 快照、planned 取消、归属清理与回执/outbox 原子提交。 | operation/settings 回归；阅读生命周期 apply/closure 合同及现有服务测试。没有用当前可见任务页计算影响。 |
| ITS-08 | 后续 `7e1eb8058` 已将共享时区编辑收拢到 General settings；迭代设置展示 `effective_timezone` 并链接单一编辑器，enable 使用展示的保存值；旧周期保存自己的时区。 | workspace-planning-timezone、core planning-timezone、settings-tab、navigation 测试。按当前合同验证，不恢复迭代内第二表单。 |
| ITS-09、10、12 | 手动模式只读；侧边栏需 supported/manual/enabled；disabled 设置保留历史。命令和 WS 更新 iterations、triage settings 与共享时区；时区修改刷新项目与迭代。 | sidebar、settings、command、realtime 与 mounted WS-instance 回归；新启用只在提交后通过 writeIterationResult 发布，receipt replay 不重复发布。 |
| IP-01、02、04 | catalogue 顺序读完全部元数据；完整目录派生计划/历史倒序与全局 upcoming；仅 active 行查询详情。无 active、筛选无结果和读取失败有独立反馈。 | catalogue、timeline、navigation：跨页 active、多计划、stale 重启、过滤、无逐行详情。 |
| IP-03 | 间隔使用校验后的日历日；完整无过滤集合且无刷新/错误才展示，重叠、未知、取消、时区不可比较时抑制。 | timeline 的日期/排序/边界矩阵；catalogue 的重复身份/cursor、workspace 身份与授权检查。 |
| IP-05、06、07 | 稳定 wsId:id 详情边界；保存时区始终显示。真实零任务计划提供添加已有/新建；任务筛选保持挂载，编辑 Dialog keepMounted。创建携带所选 ID/已审核 revision 并在原 POST 事务归属。 | navigation、details、add-existing、assignment、form、create-issue、create-issue-dialog 测试。 |
| IP-08、09 | 图/表复用相同 statistics；冻结优先 snapshot。单日显示实际日期与值，不制造趋势；有效/原始完成率分母分开。任务数与事件数、未完成关闭去向与周期内范围变化分开。 | progress、history、chart-closeout、scope、activity、events、navigation 测试；当前 task 改动不重算冻结图表。 |
| IP-10、12；ITS-12 | 首读可重试；缓存暂时失败保留草稿；确切权限/删除隐藏受保护内容。待确认操作不靠 live enabled/status 猜测成功。标签与 panel 关联并保留任务筛选。 | navigation、form、operation、command/access，以及本轮权限恢复 regression。 |
| ITS-02、12；IP-12 | 使用共享 Web/Desktop 视图、双语、语义样式、原生披露/图表表格与现有 Tabs/Dialog。 | 本轮组件和 locale parity 通过；像素布局、实际键盘/触控尺寸、深浅色宽窄窗口及真实双端证据仍归主会话最终 browser/visual run。 |

读代码还确认：关闭/历史访问没有依赖部署标志；真实更新与出站通知提交后发布；操作/任务/运行执行语义保持；客户端未引入 Zustand 服务端数据镜像；旧服务能力回退与 `workspace_access_denied` 仍区别处理。未发现本次需新增迁移或依赖的理由。

## Findings (not fixed) / 主会话收尾

没有留下已确认的产品源码问题。以下属于主会话拥有的文档与整体验收，不在 reviewer 写入范围：

1. **父计划中的旧时区双入口描述。** 初读时父 PRD ITS-08/AC-08、design、implement 和 closeout-plan 仍描述迭代页独立时区表单及脏草稿锁开关。`7e1eb8058`、现有代码与 core spec 已采用 General settings 单入口。主会话已经开始同步这些当前文档，应保留旧 verification 原貌。
2. **子设计中的后续语义细化。** 当前 `6d3bf906d` 与 core spec 已明确阶段感知范围展示：计划/开始前取消显示 Planning adjustments；真正空承诺启动不等于未启动；冻结快照拥有阶段与标题；新增/移除/重入/取消/重开/净变化在 Scope changes，交付比例与 started 在 Progress；单日 chart 只展示真实日期与值及完整数据表。子 PRD/design/implement 的“完整统计都放 Progress”等旧描述应对应更新，不能将已批准增强判成缺陷。
3. **新权限恢复规则同步。** 需要 core iteration-operations spec 记录“被拒绝的 mutation 必须仍可重试，当前受保护读取均成功后才清除旧拒绝错误”。已向主会话交接，主会话负责规范写入。
4. **最终源码与浏览器证据。** 最终冻结构建必须包含这两个修正文件。父任务 10-09 权限阻塞、当时测试数量、旧浏览器失败等都是历史；本轮复核不能把它们改写成当前失败或当前通过。主会话负责新 `check.sh`、Web/Electron、视觉 verdict 和任务归档。

## Verification

所有日志在 `.omx/state/iteration-settings-final-20261010/review/`。`source.json` 记录 HEAD 与两处修正文件 SHA-256；`settings-retry.patch` 记录实际补丁。

| 检查 | 结果 | 日志 |
| --- | --- | --- |
| Core 聚焦回归，maxWorkers=2 | 17 文件 / 304 项通过，退出 0；之后 core 源码未改 | core-tests.log |
| Views 修正前聚焦基线，maxWorkers=2 | 22 文件 / 389 项通过 | views-tests.log |
| 新缺陷 red / green | 两条预期失败已记录；最终设置 suite 19/19 通过 | permission-*-red*.log、permission-retry-final-green.log |
| Views 修正后聚焦回归，maxWorkers=2 | 22 文件 / 391 项通过，退出 0 | views-tests-final.log |
| Core typecheck / lint | 均退出 0，core lint 无 ESLint 警告 | core-typecheck.log、core-lint.log |
| Views 修正后 typecheck / lint | 均退出 0；lint 为 0 error / 27 条既有非迭代 warning | views-typecheck-final.log、views-lint-final.log |
| Diff whitespace | `git diff --check` 通过 | 命令返回 0 |

实际命令：

```sh
pnpm -C packages/core exec vitest run iterations api/iteration-client.test.ts api/iteration-schemas.test.ts projects/planning-timezone.test.tsx realtime/use-realtime-sync-ws-instance.test.tsx --maxWorkers=2
pnpm -C packages/views exec vitest run iterations settings/components/workspace-planning-timezone.test.tsx settings/components/settings-nav.test.ts settings/components/settings-page.test.tsx modals/create-issue.test.tsx modals/create-issue-dialog.test.tsx layout/app-sidebar.test.tsx locales/parity.test.ts --maxWorkers=2
pnpm --filter @multica/core typecheck
pnpm --filter @multica/core lint
pnpm --filter @multica/views typecheck
pnpm --filter @multica/views lint
```

Views lint 既有 27 条非迭代警告，以及 pnpm 对旧 package.json 配置位置的提示另行保留；未为本次任务改环境/依赖或清理无关代码。这些测试数量是实际运行的行为覆盖，不是行覆盖率。未在 reviewer lane 重复执行全仓库套件、Go DB、构建或浏览器；它们由主会话统一运行。
