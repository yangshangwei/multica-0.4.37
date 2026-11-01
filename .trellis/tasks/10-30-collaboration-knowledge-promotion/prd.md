# 知识候选逐层沉淀

## Goal
让协作单结论成为带出处、范围和验证证据的候选，经人确认后进入项目或空间上下文。

## In scope
issue-scoped candidate、提议/确认权限、独立目标集合 revision、版本/纠错/过期/撤回、持久目标失效和后续注入过滤。

## Confirmed authority and dependencies

- 用户已确认：发起人或当前处理人提出；目标项目负责人/管理员确认项目知识；目标 workspace owner/admin 确认空间知识；技能层不纳入本轮。依据：会话 `01a18a36-e482-7840-9900-3ff8174b4ce7` 用户轮次 14。确认操作必须由人执行。
- G0/G1 已冻结在 runtime 产品提交 `0e1ee8e1e`、确定性证据提交 `2b861347e`：来源/版本/purge/availability、handoff 与实际应用输入绑定。依据 `../10-30-collaboration-context-handoff-runtime/verification-integration-2026-11-01.md`；真实模型与更广平台矩阵仍单列开放，实验窗口结束于 upstream 502，本次不重开。任务已进入 in_progress；数据/协议、Core、普通启动基础与共享 UI 已分批实施，人工 API/失效和执行接线按独立证据收口。真实模型、平台与最终启用仍各自验收。
- 代码核对：Project Context 使用 project description/resources（`runtime_config_sections.go:417`）；workspace 使用 `workspace.context`（`:184`）。项目更新有 expected revision，workspace 当前直接写 context，不能假定两者都有可复用的候选版本/撤回服务。
- 确认不得把来源协作单的可见范围无条件扩散到目标所有运行；正文注入需同时校验候选状态、有效期、来源可见权与目标适用范围。撤回/过期必须影响下一次注入，不只修改候选 UI 状态。

## Out of scope
未经确认写入生效上下文、自动写入技能层、向量检索、跨 Server。

## Acceptance criteria

| ID | 可观察验收 |
| --- | --- |
| AC1 | 未确认候选不改变 base context 或任何实际注入；外部当前处理人可向来源空间提出，隐藏任意目标不可枚举。 |
| AC2 | 只有当前有来源读权且有目标确认权限的人可批准；Agent lead、机器凭据、普通评论权不能代替。 |
| AC3 | 正文不可原地修改；纠错经新版本确认后原子替代旧贡献，版本/出处/验证声明可追溯。 |
| AC4 | 同 scope/actor 的 request UUID 与 memory/command 等跨 kind 共用命名空间；新意图新 UUID，原 key/hash 幂等，跨 kind 复用拒绝；过时审阅版本拒绝；失去来源或目标角色后仍可凭 candidate ID 与原 request ID 读取本人内容无关回执。 |
| AC5 | 过期、撤回、supersede 和来源不可用立即影响后续消费；不依赖后台 sweeper；来源 purge/delete 后人类列表、详情和目标视图也不暴露派生正文。 |
| AC6 | 普通 issue、第一方 Chat、quick-create、autopilot 和协作运行使用当前可信 originator/runtime owner/audience；不以 AccountableUserID 或 installer 补出授权。 |
| AC7 | 真实 fake-executable 输入只含当次获准贡献；private body 不拼入稳定 base brief/project sidecar；披露及启动各有当前授权校验。 |
| AC8 | 精确 Provider session 的累计 exposure 决定是否可复用；旧客户端/不支持 backend 也不能恢复已知失效或不可证明的 managed history；启动拒绝不自动重放业务输入。 |
| AC9 | Shared Web/Desktop 展示提案、人工决定、版本、目标贡献与冲突；无 source Read 的目标管理员也能收到只有目标身份/修订的失效提示。source 清除/ACL 事务只持久化 scan intent，不新增 target/candidate/revision 等待；worker 可重放、去重、删除后继续。提示到达立即清除旧正文；漏帧/离线最长 30 秒可见期，expiry 与晚响应不能延长它。 |
| AC10 | 迁移/sqlc/Go/前端/定向平台检查有版本化证据；源码和索引遵守无 FK、CONCURRENTLY 规则；不自动写 skills。 |
| AC11 | 知识父 workspace/project 存活锁用 KEY SHARE NOWAIT；lead 权限才用 project SHARE，角色仍锁 member。独立 revision 不占 issue_counter/project 普通编辑热点；真实并发回归证明无新增锁反转，初始化竞争与准备失败都有有界处置。 |

正式设计见 `design.md`，分批实施与具名测试见 `implement.md`。K1–K5 与 F1–F4 已完成设计复核；实施阶段的独立审查和修正记录分别保留，不把计划通过当作产品验收。G0/G1 的确定性依赖已满足；G2 是知识实现的启用门，不能循环要求未实现的功能先通过才允许实施。G3 的所有权、迁移、隔离环境已分配，见 `research/execution-ownership-2026-11-01.json`；普通 C2/C3、协作 C4 与 Web/macOS 确定性门已通过；生产开关仍默认 false，真实 Provider 和发布矩阵另行验收。
