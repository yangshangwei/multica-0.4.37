# 收件箱延后实施计划

## Execution boundary

本任务当前为 `in_progress`。交互方向、建任务与最终实施规划均已获用户明确批准；主会话已运行 `task.py start` 并调度 Trellis 原生子代理。执行进度和验证结果记录于 `progress.md`；共享接口见 `research/implementation-contract.md`。

最小行为差距是：共享收件箱没有个人通知组延后状态、服务端到期唤回及相应缓存/交互。行为分别由服务端通知存储和投递、Core API/Query、Views 共享页面拥有，不能只在行组件拦截。

当前工作副本有大量其他任务的改动。实施前对下述范围单独记录基线，子代理明确文件所有权，保留其他改动；主会话负责共享接口、集成、规范更新和收尾。无需另建重复任务。

## Ordered checklist

### 1. Freeze contracts and baseline

- [x] 在启动后读取当前任务三个文档及已整理的实现/检查上下文；确认目标文件在共享工作副本中的最新状态。
- [x] 固定 `inbox_snooze` 组身份、首次建行的事务级 fence、schema 字段、三类列表参数、mutation 回执、撤销冲突和个人 WS 事件契约。
- [x] 为既有通知组身份、最新组已读语义、评论落点、归档和普通评论重放保留/补充规范层回归，先观察相关新行为测试失败。
- [x] 准备任务拥有的本地 PostgreSQL 测试数据库，与正在运行的 API 数据库分开；保存精确测试选择、工具链和启动身份，不启动用户已安装的智能体 CLI。

### 2. Backend persistence and lifecycle

所有权：一个 `trellis-implement` 子代理负责 `server/migrations/` 中本任务迁移、`server/pkg/db/queries/inbox.sql`、对应生成代码、`server/internal/handler/inbox*.go`、router 接线及本任务后端测试。确切迁移编号在写入前重新检查。

- [x] 增加无外键的计划表、最小关键来源回执表、分别并发创建的身份/到期索引及双向迁移；运行 sqlc。
- [x] 实现延后、改时间、现在提醒、服务端快照撤销和版本冲突，确保通知状态与计划同事务提交。
- [x] 增加 `/api/inbox/snoozed` 列表投影；对主列表、归档列表、未读与跨工作空间计数统一组过滤、最新行语义及提醒排序。扫描暂停时已到期项仍可见为未读；用户标读原子消费到期标记。
- [x] 主列表批量操作使用有效已读投影并排除未来延后组；全部已读消费尚未物化的到期标记，归档已读保留其重新未读提醒。手动归档、自动失败归档、工作空间/任务/账号删除及成员撤权/退出正确清理/失效计划，更新已有 workspace delete manifest 检查。
- [x] 实现有界、重启可恢复的到期扫描与条件消费；无活动通知或失权时不发送提醒。
- [x] 后端回归覆盖权限隔离、时间非法/已过期、归档目标、无任务通知、组内普通新增、并发改时间、撤销/到期/归档竞争、扫描暂停后逐项/全部已读不重开、归档已读保留到期未读及服务端重启扫描。

### 3. Notification delivery and personal events

仍由后端所有者执行；主会话审核实际生产路径清单后再进入此步，防止只修改一种通知来源。

- [x] 对 `notification_listeners.go`、服务端 task/autopilot 结果、自动化失败监控、待分诊、项目及迭代通知复用一致的延后状态转换；通知插入与首次建立计划也取得同一事务组 fence，提交后才发布。
- [x] 新增 `mention_scope` 来源标记，区分直接 `@member`、`@all`、`@squad`；只允许已确认的直接提及/本人分配/执行失败唤回，保留当前接收人、通知偏好、静音和普通评论持久化去重契约。
- [x] 为评论/描述直接提及、分配、原执行失败及委派恢复耗尽固定原来源/接收人/组回执；保留/传递 comment UUID、committed issue revision、真实 task UUID 和耗尽说明 UUID 等当前生产路径遗漏的元数据，不重写历史通知身份。
- [x] 增加延后时已提交 revision/可见来源 ID 基线，接纳有来源证据的旧随机 ID 行；无元数据历史来源通过确切可见来源识别并消费，不因重新投递而唤回，也不创建伪造提醒。不能用事务开始/更新时间证明来源已提交。
- [x] 延后普通通知继续保存且不产生普通系统提醒；有效新重要通知原子提前唤回，重复源事件不再唤回。去重也覆盖未延后时的投递及唤回后重新延后的场景。
- [x] 添加最小个人组状态事件和路由；同时补齐已有 `inbox:unread` 的个人事件注册/接收人路由。
- [x] 执行通知监听器与数据库回归，分别覆盖普通投递抑制、评论/描述直接提及与群体提及、分配/失败/耗尽说明唤回、未延后时的消费去重、唤回 → 重新延后 → 原事件重放仍延后、有元数据接纳/无元数据可见基线、来源事务先开始后提交、首次延后/通知插入两种 fence 顺序及自动归档不复活。

### 4. Shared Core

所有权：一个 `trellis-implement` 子代理负责 `packages/core/types/inbox.ts`、API client/schema、`packages/core/inbox/`、相关 realtime 和 `packages/core/paths/tab-subject.*`。API/WS 契约先与后端所有者对齐后再写。

- [x] 增加可选投影字段、严格 mutation receipt schema、API 方法、已延后 key/options 与 mutation。
- [x] 新操作等待服务器成功；失败保留缓存和目标。成功及撤销冲突正确失效三类列表、数量和账户摘要。
- [x] 分组、过滤、计数、原评论落点和提醒排序保持一致；任务 projection update/delete 覆盖已延后缓存。
- [x] realtime 使用事件源工作空间；当前工作空间之外的操作同样刷新，并遵循服务端系统提醒资格。
- [x] 加入焦点/重连与 30 秒恢复刷新；持久化状态留在 Query，纯时间帮助函数留在 Core。
- [x] Desktop tab subject 对三类列表保持不同身份，同时保留既有 active/archive 标识。
- [x] 测试 API 请求及 malformed response、旧字段缺省、组计数/过滤、缓存失效、冲突、时区/日历/DST 与标签身份。

### 5. Shared Views

所有权：一个 `trellis-implement` 子代理负责 `packages/views/inbox/`、`packages/views/locales/en/inbox.json`、`packages/views/locales/zh-Hans/inbox.json` 和其组件测试，以及 Desktop Snoozed 标签标题的 `packages/views/layout/tab-presentation.tsx` 查询分支与回归。先依赖冻结的 Core 接口；不与 Core 所有者同时编辑其文件。

- [x] 扩展三类视图、URL 恢复、已延后 footer 数量/空态和列表排序说明。
- [x] 行内时钟、右键、触屏菜单、详情动作共用延后选择器，五项快捷时间和自定义时间完整接线。
- [x] 更改时间、现在提醒、归档与撤销回执可用，失败保留 picker/输入/通知；成功后再续接选中项。
- [x] 防止弹层键盘操作触发背后通知导航，避免手动或到期唤回立即被旧详情标读。
- [x] 验证无悬停设备、44px 点击区、Escape/焦点恢复、深浅色、窄屏、长标题及跨年时间。
- [x] 组件测试聚焦接线、可访问性、成功/失败路径；纯时间矩阵只保留在 Core。

### 6. Integration and final gate

主会话协调一个 `trellis-check` 子代理独立审核全范围并自修安全的局部问题。

- [x] 核对 Storage → API schema → Core Query → 共享 UI → WS/reconnect 的完整读写链。
- [x] 在独立数据库运行后端相关 normal/race 回归及 `go vet`；每个声明的测试都有实际执行结果，跳过或编译失败分别记录。
- [x] 运行范围内 Vitest、TS typecheck、lint 和静态边界检查；修复本任务问题后再运行要求的较宽检查。
- [x] 在任务拥有的 production Web 与实际 Desktop renderer/原生 Electron 上完成真实交互链，使用测试账号和 TestApiClient。
- [x] 链路覆盖延后 → 刷新 → 已延后 → 改时间/撤销/现在提醒，以及到期、普通新评论、直接提及与跨工作空间/跨设备同步。
- [x] 视觉检查按桌面/窄屏、深浅主题批量执行；每次视觉迭代写入 visual-verdict，再做下一次修正。预览结果不能代替实际产品截图。
- [x] 保留迁移、归档、撤权、旧响应与并发撤销的证据；新依赖数量为零。
- [ ] 主会话更新非显而易见的通知延后规范，按 Lore 提交本任务原子改动，质量门通过后再归档和记录进度。

## Validation commands

实施时先按新增测试的真实名称收窄 Go/Vitest 范围，再在正确环境执行以下标准检查。不要在未配置测试库时盲跑后端测试，也不要将 `next dev` 用于 E2E。

```bash
python3 ./.trellis/scripts/task.py validate .trellis/tasks/11-20-inbox-snooze
make sqlc
pnpm --filter @multica/core test
pnpm --filter @multica/views test
pnpm typecheck
pnpm lint
pnpm check:ui-exports
make test
make check
```

带 `DATABASE_URL` 的精确 Go 命令、production Web/Electron 启动命令及实际测试选择在准备环境后记录到 task-local verification 记录中。全部工具调用通过现有 pnpm/Makefile 源命令；新建 `.test.ts` 中不需 DOM 的纯逻辑测试使用 node 环境。

## Risk and rollback points

- 第一处风险边界是组身份/版本/schema；冻结前不开展多个写代理，契约变动由主会话协调。
- 第二处是多来源通知和现有重放去重；来源/接收人身份首次持久投递才可唤回，随机通知 ID 不证明新源事件，不以消息文本或客户端 `mentioned` 猜测直接提及。
- 第三处是通知计划与归档/撤权/到期竞争；必须先有 canonical 事务/竞态测试，再加入后台扫描。
- 第四处是跨工作空间事件与 Desktop 标签恢复；必须证明源工作空间刷新和独立标签身份。
- 若服务端阶段不完整，不启用 UI 成功路径；若需回滚先解除活动隐藏计划，再移除查询/迁移。
- 外部部署、全量无关失败或新增产品决策另行记录，不通过改动无关业务或放宽断言掩盖。
