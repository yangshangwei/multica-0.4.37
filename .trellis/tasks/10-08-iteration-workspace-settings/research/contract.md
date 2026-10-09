# 现有合同核对

2026-10-08，只读检查当前工作树；不代表新方案已经实现。原生研究子代理与主会话分别核对后端合同和共享设置界面。

## 设置与界面

- `packages/views/triage/triage-settings-tab.tsx`：使用 SettingsTab / SettingsSection / SettingsCard / SettingsRow，owner/admin 修改；分拣台采用草稿 + 保存。
- `packages/views/settings/components/settings-nav.ts`：工作空间组已有分拣台，导航基于静态声明和 URL `?tab=`。
- `packages/views/settings/components/settings-page.tsx:249`：共享设置页挂载分拣台内容，Web 和 Desktop 可共同消费。
- `packages/views/iterations/iteration-page.tsx:136`：当前折叠配置含时区、启用和 IterationOperation disable。
- `packages/views/layout/app-sidebar.tsx:892`：当前迭代入口仅检查 supported。
- `packages/views/common/timezone-select.tsx:79`：已有 IANA 时区选择器，保留当前值、浏览器值及完整可用列表。

## 能力与权限

- `server/internal/handler/iteration_settings.go:51`：supported/atomic_handoff 来自 iterations_i1；enabled = available && settings.Enabled，manual=true。
- `packages/core/api/iteration-schemas.ts:18`：安装的客户端已有稳定能力响应形状；保持字段、类型、schema_version=1 和现有路径。
- `server/internal/featureflags/keys.go:48`：迭代不是通用前端公开 flag，不需增设前端标志。
- `server/migrations/550_iteration_tables.up.sql:5`、`server/internal/service/iteration.go:109`：设置默认 false/revision 1，缺行读取有相同默认；无需数据迁移。
- `iteration_settings.go:63,74`：启用时稳定人类身份及当前 owner/admin 校验。
- `iteration_management.go:29`、`iteration_lifecycle_preview.go:130`、`iteration_lifecycle_apply.go:22`、`iteration_closure.go:241`：关闭的预览、提交、重放均保留管理员/人类检查。

## 部署 gate 退出清单

生产位置：`server/internal/featureflags/keys.go:10,77`；handler 的 `iteration_settings.go:51,90`、`iteration_management.go:42`、`issue_iteration_assignment.go:75,103,219`、`triage_iteration.go:25,77`；service 的 `iteration.go:18,65`、`iteration_lifecycle_metadata.go:92`、`iteration_notifications.go:47`；`server/cmd/server/main.go:655`。

必须保留 workspace enabled 门槛：`iteration_lifecycle_metadata.go:102`、`issue_iteration_assignment.go:106`、`triage_iteration.go:43,80`。

配置/现行操作文档：`.env.example:211`、`docker-compose.selfhost.yml:118`、`scripts/offline-build-changelog.test.mjs:12`、`scripts/offline-changelog.test.mjs:202,224`、`docs/offline-upgrade.zh-CN.md:33,293,304,306`、`.trellis/spec/server/offline-delivery.md:24`、`.trellis/spec/server/iterations.md:141,150,176`、`.trellis/spec/core/frontend/iteration-operations.md`。历史计划和旧发布证据保持原样。

`docs/qa/core-e2e-test-plan-2026-10-08.md:52` 也包含环境假设，但当前是其他工作的未跟踪文件，实施时应协调所有权，不直接覆盖。

## 启用与时区是独立操作

- `server/internal/service/iteration.go:25,53–105`：启用只接受 request_id、expected_revision、confirmed_timezone；验证时区、设置版本、锁定的共享规划时区。冲突返回 iteration_preview_stale；只修改设置，不创建周期。
- `server/internal/handler/project_timezone.go:77–129`：PUT planning-timezone 独立事务，body 为 planning_timezone string|null，null 回到 UTC；仅人类 owner/admin。
- `iteration_lifecycle_metadata.go:145–168`：创建迭代复制当前共享时区，既有周期保留自己的时区。
- `packages/core/projects/p1-mutations.ts:20`：当前时区更新只 invalidates projects，需要同时刷新 iteration settings。
- `iteration_settings.go:96` 与 `iteration_management.go:87`：当前启用直接写 JSON，缺少其他生命周期操作的提交后 iteration:updated 发布；应补齐跨客户端更新。
- `packages/core/iterations/command.ts:215`：当前本地命令完成只 invalidates iterations/issues；分拣台迭代能力也要更新。

## 关闭与恢复

- `iteration_lifecycle_preview.go:166,227,235`：预览针对全部 active/planned 周期和完整任务集合，超过 2,000 条任务返回 413，不能分批关闭冒充原子成功。
- `iteration_closure.go:125–198`、`iteration_lifecycle_apply.go:89–115`：active 结束为 completed 并冻结快照，planned 变 cancelled。
- `iteration_closure.go:51–109`、`server/pkg/db/queries/iteration.sql:47`：清除当前任务指针，保留参与记录、范围事件、结转次数；任务状态/项目/负责人/执行均保持；回归 `iteration_closure_test.go:785`。
- `iteration_lifecycle_apply.go:31–138` 与 `operation_execute.go:74–91`：状态、快照、归属、持久回执、通知 outbox 同一事务。
- `iteration_lifecycle_apply.go:102–112`：每位去重接收者一份关闭摘要；已关闭空间停止逾期提醒，历史通知可继续投递。
- `iteration_list.go:96`、`iteration_history.go:44–104`：历史读取要求成员权限，不依赖 enabled。
- `iteration_operations.go:88–110`、`operation_execute.go:51–74`：读取回执独立于当前启用状态，重放先查已完成记录再考虑当前设置；不能重发旧意图改变后来状态。
- `packages/core/iterations/command.ts:176–215`：沿用 enable 和 disable:workspace scope，持久化原始请求，先 GET 恢复，确认未找到再同身份 POST。
- `packages/views/iterations/iteration-recovery.tsx:17`：设置页也必须挂载，不能被禁用或能力检查隐藏。

## 升级风险

过去数据库 enabled=true 但被部署标志隐藏的空间，会在移除 gate 后按保留状态重新可用；通知 worker 也可能继续处理已存在 outbox。升级说明必须准确说明，不把这些空间强制改成 false，更不能把全体空间默认改为 true。
