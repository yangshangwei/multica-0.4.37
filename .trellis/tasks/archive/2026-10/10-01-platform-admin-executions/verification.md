# S03 实施验证

状态：实现、定向验证和指定生产快照的浏览器/视觉验收已完成。父任务整体验收、后续客户端变更和S07容量/原生验收仍需独立证据，不据此宣称整个平台任务完成。

工作区：`/Volumes/artisan/code/2026/multica-platform-admin`。实施基线：`fa28e28a267bc8c7d76d9b8c55ef2d08e000514e`（S01）。本切片未提交；父任务统一集成公共 API、导出、路由、locale、sqlc 和迁移编号。

## 已实现

- `GET /api/admin/tasks`、`/tasks/{id}`、`/issues`；各入口重新执行平台读取权限，查询受内部组织映射约束，不依赖调用者 workspace header。
- 执行列表支持业务任务、执行、工作空间、运行时、责任人、执行安装 ID、状态、来源、业务编号/UUID 搜索、创建时间窗口与时区。业务任务列表仅接受自己实际支持的字段，不静默忽略执行过滤条件。
- 共享 `AdminListScope` / `ParseAdminListQuery`：默认 50、最大 100；执行默认及最大 31 天；`(created_at DESC,id DESC)` 游标使用 HMAC，绑定 actor、organization、resource、规范筛选与 as_of；24 小时后要求重新查询。拒绝篡改、跨角色主体/组织/资源、改筛选、重复参数、非法编码。S05 使用 Window=0 查询所有账号历史。
- 执行/业务任务读取使用 5 秒 context deadline；limit+1 判断后页。创建快照排除翻页间的新插入，状态及用量保持实时并在界面说明。
- 真实 source 分类：issue、autopilot_issue、chat、autopilot、quick_create、unknown；直接使用持久 attempt、parent_task_id、retry_of_task_id、rerun_of_task_id。历史未证明的安装归属保持 null。
- DTO 不返回 error/context/result/命令/本地路径/消息。失败码只允许 canonical taskfailure 分类，其余为 unknown。业务标题需当前 workspace member；会话链接同时检查创建者、workspace member、原会话智能体的当前访问权限，以及原公共会话投影（显式创建或已有非 command 消息）。原内容 API 未修改。
- 无 task_usage 行时 usage=null；有行才汇总 token。模型来自执行实际用量上报，多个模型时不伪造单一模型；不以当前 agent 配置冒充历史执行模型，不展示账单金额。
- 业务任务的 execution_count 与执行下钻使用相同 `[time_from,time_to)` 与 as_of 窗口。UI 区分业务任务和执行尝试。
- Web 三个页面：执行列表、执行详情、业务任务列表；URL 筛选、游标分页、加载/错误/空态、受限内容、实际尝试关系及缺失用量。详情 5 秒、列表 15 秒轮询，后台 tab 暂停；Query 继承 S01 actor/server/org 缓存隔离。

## 已运行证据

测试数据库：`multica_platform_admin_s01_test`，没有调用真实 agent、没有模型请求。仅 fixture 创建独立 workspace/runtime/任务，按 fixture cleanup 删除。S03 单测未使用生产浏览器数据库。

1. 后端先观察不存在的新方法/查询失败；后续严格口径测试实际发现历史截止时间默认窗口错误、非法 query 编码被忽略，以及模型取自当前配置而非执行上报，均修复并重跑。
2. `DATABASE_URL=<task DB> go test ./internal/handler -run 'TestAdminExecution' -count=1` 通过；6 个顶层测试及参数子用例覆盖真实 SQL/handler、同时间戳分页、分页间插入、组合筛选、全来源、attempt/业务数区分、组织隔离、私有会话、原 workspace 标题权限、缺失用量、失败码脱敏、模型来源。新增用例逐项对照原 `GetChatSession` 的403/200/404/200，覆盖失去智能体访问权、合法 owner、仅内部命令会话、public_to成员授权。
3. 同范围 `go test -race ... -count=1` 最新通过，`6.516s`。`go vet ./internal/handler` 通过。
4. `pnpm exec vitest run admin/execution-api.test.ts admin/execution-schemas.test.ts admin/execution-queries.test.ts`：3 文件、9 测试通过。覆盖 API 解析、缺字段/未知枚举/越权内容路径、Query key 与组织响应隔离、原样保留签名 cursor。
5. `pnpm exec vitest run admin/executions admin/issues`：3 文件、5 测试通过。覆盖列表筛选/游标/错误、详情真实关联与未知用量、业务任务下钻窗口一致。
6. `packages/core` 与 `packages/views` 的 `pnpm exec tsc --noEmit --pretty false` 在本切片实现后通过；S03 文件定向 eslint 通过。最后 core 重跑仍通过；views 的最新重跑被并行 S05 新增 `users.close` 文案尚未合并阻断，需父任务 locale 合并后重跑，不能把旧通过当整批最终结果。pnpm 的既有配置迁移 warning 不属于本切片错误。
7. impeccable 机械检查 `detect.mjs --json packages/views/admin/executions packages/views/admin/issues` 返回 `[]`。这不是浏览器 visual-verdict 的替代证据。
8. 生产Web运行 `pnpm exec playwright test e2e/platform-admin-executions.spec.ts --workers=1 --retries=0` 通过，1场景/0失败（最后3.5s）。初轮原生fetch断言错误 `status()` 已改为属性 `status`，不涉及应用修复；重跑完整场景通过。
9. `query-plan.json` 保存2000条terminal执行fixture的 `EXPLAIN (ANALYZE, BUFFERS)`；fixture处于事务中并已ROLLBACK。organization/workspace/runtime 查询分别12.118/14.516/0.674ms。runtime使用`admin_task_runtime_page_idx`，其他两条使用既有agent workspace/keyset索引。这是小规模实际查询计划，不是S07性能验收。
10. S05实施人独立检查发现标题必须沿用content_access与有效URL的组合条件、会话URL须加入原智能体/公共会话检查；已复现真实失败并修复。该实施人复核源码后确认两项关闭，其余未发现具体阻断项。

## 生产浏览器结果与后续边界

- 已验证生产快照：source `67dbc412bec52dd0282d75c6ddd6fab9e267a21ed35836ac1f9292b7fb5e8f1b`；Web build `8WNMN1kDY94T4HvL_O04V`，Web PID53784/API PID53545。Web为production `next start`，API为task-owned Go进程；完整配置/启动来源见 `.omx/reports/platform-admin/batch-s02-s03-s05/{web,api}.running.json`。
- 真实password登录、非成员原内容API404、私有DTO/标题、cursor与组合过滤、attempt/detail/null usage、仅增加原workspace membership后可读取原内容并出现原链接、无页面错误均通过。假runtime没有进程，执行夹具全部terminal；已清理专属workspace/account。
- 已检查desktop1440×1000、mobile390×844（含滚动至表格）、详情截图；无document横向溢出。表格横向滚动留在自身容器，移动导航换行属于现有样式的扩展。visual-verdict为92/pass，无必修视觉项。
- 证据在 `.omx/reports/platform-admin-executions/{browser-verification,visual-verdict}.json` 与同目录4张PNG；状态记录在 `.omx/state/platform-admin-executions/ralph-progress.json`。视觉参照使用既有S01管理页面截图，不是逐像素复刻目标。
- 最终批次整体检查及后续源代码变化由父任务接续。运行中客户端drain/本地IPC变更不包含在此已验证生产快照；不能借用此结果宣称那些路径已部署或原生验证完成。
- 100 万历史、1000 安装、多观察者 P95 与原生终端验证属于 S07，未运行，也未声称性能目标已达成。
- S04 取消/协调按钮不属于 S03；当前 allowed_actions 为空，不展示虚假动作。S02 安装快照由已证明身份路径写入，本切片只读相应列。

## 文件与协作边界

本切片新增 `server/internal/handler/admin_query.go`、`admin_execution.go` 与测试，`server/pkg/db/queries/admin_execution.sql`，`packages/core/admin/execution-*`，`packages/views/admin/executions/*`、`issues/*`，`apps/web/app/admin/tasks/*`、`issues/*`，`e2e/platform-admin-executions.spec.ts`。未修改原 workspace 内容权限或创建新依赖。共享 helper 复用于 S05，两个业务列表共享控件与状态展示。

只读交叉检查 S05 时发现 data_quality=partial 的用户扫描页被显示为最终空态，已报告 S05 实施人和父任务；本切片没有代改他人文件。

## 2026-10-02 状态覆盖补充

自查发现遗漏仓库既有的 `waiting_local_directory`，已增加执行 schema、服务端筛选枚举和中英文状态标签；不返回其私有 wait_reason/路径。回归已观察原错误：API筛选400、schema降为unknown；修复后真实SQL/handler定向回归通过（0.819s），S02/S03合并core检查18项及typecheck通过。共享locale与SQL已由父任务合并进上述生产构建；S02在途计数的独立回归记录由该切片及父任务汇总。状态文案增量在 `locale-waiting-additions.json`，完整locale片段也已更新。


## 最终批次复跑（2026-10-02）

父任务在source `2d7a14b1612dab58d77a70352a12d9f8ce7270a2a3d9af56ea47e9c02fb87905`、Web build `4HqC2LA83Zf-dnxnfTGP6`（API PID1467/Web PID2095）重跑S03+S05，2/2通过、0重试，合计7.84s。原始JSON reporter输出：`.omx/reports/platform-admin/final-s02-s03-s05/accounts-executions-playwright.json`（带dotenv前缀）；规范化S03结果：`.omx/reports/platform-admin-executions/final-playwright.json`。先前S03视觉92/pass结论未发现回归；此复跑仍不代表Windows或安装态客户端验收。
