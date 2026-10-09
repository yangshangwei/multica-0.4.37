<!-- Approved by user on 2026-10-08: 确认无误，请继续推进. Implementation is authorized for business pages and the independent settings page shown in the final preview. -->
# 迭代业务页实施计划

状态：实施收尾中；用户已于 2026-10-09 授权按父任务 [closeout-plan.md](../10-08-iteration-workspace-settings/closeout-plan.md) 完成修复、验证、提交和归档。此前的规划状态由该授权取代。

## 阶段门槛与所有权

- 本子任务最终 PRD、设计与交互原型已获批准；主会话按父任务 closeout-plan.md 完成实施与统一验收。
- 父任务继续拥有设置页、设置预览、启停与部署 gate 合同，以及最终跨任务集成。与父任务共同触及 `iteration-page.tsx`、共享操作组件、locale 时先协调文件所有权；不能覆盖正在进行的工作。
- 当前最小差距：已有周期列表／详情把配置、表单、图表、事件、任务纵向堆叠；改成时间线和分标签详情，业务语义保持。
- 本子任务不写后端、迁移、自动化、移动端、全局任务创建重构或依赖更新。新建任务仅增加既有手动创建的周期上下文。

## 执行顺序

### 1. 锁定既有行为与目标断言

- [ ] 阅读 `CLAUDE.md`、`iteration-operations.md`、父设置合同与本任务三份文档；确认最终原型对应当前设计，列出待改文件。
- [ ] 先运行现有 iterations 组件／核心回归，记录基线；行为测试直接补在其规范层，不复制核心状态矩阵到 DOM 套件。
- [ ] 在核心增加时间线派生回归：计划／历史倒序、同日期稳定顺序、全局当前／upcoming 身份、完整目录、过滤后空结果、2 天间隔、重叠／嵌套／取消／未知模式／跨时区／DST／闰日／跨年抑制或日历计算。
- [ ] 为页面添加目标行为断言：当前展开、计划／历史无详情请求、无 active 不请求详情、元数据多页完成前不推断当前／间隔、统计失败不为零。

### 2. 读模型与总览

- [ ] 复用 `iteration-catalogue.tsx` 完整受保护目录；确有共享需要时把 query options 放到 `packages/core/iterations/index.ts`，保持缓存键，补重复游标／重复身份与 stale cursor 测试。
- [ ] 用核心纯函数派生顺序和间隔；不要倒排服务器升序首屏冒充全局时间线。总览只保留一份目录读取及一次当前详情 query。
- [ ] 在共享 iterations 目录实现时间线，迁移名称／日期／状态筛选及历史显示分页。已有完整目录上筛选不改变全局身份，背景刷新失败保留数据并禁用间隔推断。
- [ ] 新建迭代入口复用 `IterationForm`，保留默认日期、Unicode 名称验证、冲突基线和成功后导航。不要把表单默认值转成设置项。

### 3. 详情结构与空状态

- [ ] 重组详情头部、操作区和任务／进展／范围变化标签；按 `wsId:id` 保持隔离，表单与恢复状态不受标签切换销毁。
- [ ] 提取并复用 `IterationHistory` 的统计／图表／表格，让总览当前预览与详情共享同一来源，保留全部现有统计指标。
- [ ] 原始承诺虚线；只绘制服务端点；空图有说明；图例与数据表可读。历史始终优先 snapshot.statistics 和 snapshot.events。
- [ ] 在 `IterationIssueList` 接入已确认真实零任务空状态，区分筛选无结果／加载／失败；保留范围、筛选、分组、分页和当前对比。
- [ ] 对 `IterationAssignment` 增加必要触发方式，打开即进入选择流程但仍要求完整预览和确认；目标为当前周期，不产生自动移动。

### 4. 现有动作与周期上下文

- [ ] 对现有 `IterationForm`／`IterationOperation` 只作所需呈现扩展；开始、结束、取消、删除、显式交接仍经过原权限、预览、pending 和恢复管线。
- [ ] `CreateIssueModal` / `ManualCreatePanel` 增加工作空间校验的周期初值，在创建 UI 明确显示并加载可用 revision；保持单次 `POST /issues` 创建与归属事务。
- [ ] 失效周期、已关闭工作空间、修订冲突或权限变更阻止误归属，保留创建输入；不会静默降级为未分配任务。关闭／重新打开不同周期不复用旧归属。
- [ ] 验证恢复在 WebSocket 先更新状态、页面关闭后重进、enabled=false、能力变动时仍可用；复用核心 canonical tests，页面只证明挂载与连接。
- [ ] 历史更正只保留名称／说明；快照和日期不变。保持未知 enum／mode 只读以及可选历史字段 unknown 表达。

### 5. 双语、设置集成与视觉验收

- [ ] 同步 `locales/en/projects.json` 与 `locales/zh-Hans/projects.json`，遵守项目术语和 plural 规则；使用 tokens、现有 Tabs/Dialog/Menu/Button，不新增 UI 框架。
- [ ] Web 与桌面原地址回归；不增路由，仍检查桌面标签名／图标、复制链接、返回与工作空间切换。
- [ ] 与父任务协调旧配置折叠区移除及真实 settings 链接，避免死链接和重复设置控件。父设置未实施时记录此父级集成验收项，不扩大本子任务实现范围。
- [ ] 逐页验证中文／英文、1440px／900px、深浅色、键盘焦点、图表表格可访问性及长名称溢出。每次视觉迭代运行项目要求的 visual-verdict 并保存证据。

## 测试分层与覆盖

| 位置 | 要证明的行为 |
| --- | --- |
| `packages/core/iterations/calendar.test.ts`；必要的新 `timeline.test.ts` | 日期、稳定排序、间隔覆盖与筛选语义的完整矩阵，使用 node 环境。 |
| 核心目录 query 新／既有测试 | 升序多页到完整目录、重复游标／ID、cursor_stale 重启、不拼旧页、AbortSignal 与 workspace 身份；不引入 N+1。 |
| `packages/views/iterations/iteration-page.test.tsx`、`iteration-navigation.test.tsx` | 时间线呈现、一个当前详情读取、刷新保留、不同实体状态重置、链接、disabled／unsupported／未知状态和恢复挂载。 |
| `iteration-details.test.tsx`、`iteration-history.test.tsx` | 标签键盘、空状态与过滤无结果区分、图表表格一致、历史不被当前变更重算、null 比例和错误显示。 |
| `iteration-form.test.tsx`、`iteration-operation.test.tsx`、`iteration-assignment.test.tsx` | 触发器改造不破坏现有预览、修订冲突、输入锁定、完整影响和恢复。 |
| `packages/views/modals/create-issue.test.tsx` | 周期初值显示／工作空间边界、正确 revision、失效归属不静默清除、创建失败保留草稿、成功一次创建。 |
| `packages/core/iterations/{command,access,prepare,realtime}.test.*` | 原请求恢复、身份 epoch、事务预览与普通任务事件刷新仍通过，保留 canonical ownership。 |
| `e2e/iterations-i1.spec.ts`、`iterations-i1-history.spec.ts`、`iterations-i1-desktop.spec.ts` | 总览到三类详情、添加／新建到计划、手动开始／结束与冻结、关闭后历史、桌面直达和实时统计；用 TestApiClient 建立场景。 |

权限回归必须涵盖人类普通成员、owner/admin、机器凭据、撤权和跨工作空间。后端协议不变，复用原有服务测试，不为 UI 变化复制一套后端生命周期测试。真实新端点或字段若意外变成必要条件，应停在范围复核并补 schema／malformed-response 测试，不能静默扩大本计划。

## 验证命令与证据

实施时按实际最终变动运行，不把下面的命令列表当作已执行：

```bash
pnpm --filter @multica/core test -- iterations api/iteration-client.test.ts api/iteration-schemas.test.ts
pnpm --filter @multica/views test -- iterations modals/create-issue.test.tsx
pnpm typecheck
pnpm lint
pnpm test
pnpm knip
pnpm exec playwright test e2e/iterations-i1.spec.ts e2e/iterations-i1-history.spec.ts e2e/iterations-i1-desktop.spec.ts
```

E2E 使用本 checkout 的隔离、生产构建 Web 环境；先遵守 `.trellis/spec/web/frontend/e2e-run-environment.md`，桌面代理必须转发 WebSocket upgrades。父级全量集成需要时执行 `make check`，记录实际环境和已有失败，不用其他任务的结果冒充本次证据。

本轮规划验证只检查：文档无占位符、需求到验收可追溯、所有 JSONL 路径存在且行格式正确、原型与设计一致。生产 lint/typecheck/test 尚未执行；主会话负责原型验证。

## 回退点与交付记录

每个阶段保持可独立回退：目录派生 → 总览布局 → 详情拆分 → 创建上下文。发现历史变化、恢复丢失或父任务冲突时先撤回该阶段的布局／接线，保留原协议、持久命令和用户草稿；不可通过清空缓存状态掩盖错误。

完成时记录实际文件、复用／删除的旧结构、执行过的验证和剩余风险。无需改数据库；超大目录的服务器倒序分页优化为明确后续项，不在本次添加批量统计接口。父级设置链接集成如仍未完成，单独报告，不能把完整用户旅程宣称为已交付。
