# P1 架构复审

日期：2026-10-05。结论：**APPROVE（设计就绪，可交 Critic）**。这不是实现、测试、部署或产品验收通过；产品尚未实施，本轮未运行产品测试。保留首轮 [architect-review.md](architect-review.md)，本报告复核其 AR-01—04，以及修订引入的锁、DTO、迁移和任务衔接。

## 修正闭合

| 首轮问题 | 当前证据 | 复审结论 |
| --- | --- | --- |
| AR-01 发布同快照 | `design.md:71`—`:75`：所有采集接受同一 qtx，Begin 后第一条 SQL 显式 RR READ WRITE；不另开事务拼贴统计；member locking read 与来源 NOWAIT 防旧授权；事务失败有界重试。`api-contract.md:98`、`:122` 固定来源锁与实际采集快照；`test-spec.md:85`—`:86` 增加双连接阶段屏障。 | 已闭合。现有 `handler.go:59` 仅有 Begin，设计使用 Tx.Exec 设置隔离而不扩展全局接口，能够实现。RR 业务只读接口允许 locking read，因此没有误用 PostgreSQL READ ONLY。 |
| AR-02 outbox 锁顺序 | `design.md:99`—`:103` 明确不复制 T1 先锁队列；无锁候选扫描后逐条 RC 事务，workspace → recipient fence/member → project → outbox，锁内重查再提交 inbox/outbox；定义恢复、退避、终态与 worker 生命周期。`test-spec.md:91`—`:92` 增加删除、撤权、双 worker 与崩溃屏障。 | 已闭合。删除与投递都先 project 后 outbox，消除首轮指出的反向阻塞。worker 在现有 `server/cmd/server/main.go:652` 附近注册且由 foundation 持有共享文件，责任清楚。 |
| AR-03 跨域 DTO/证据 | `api-contract.md:28`—`:77` 定义非递归统计、进展、修订、重放、双验收摘要和历史署名；`:81`—`:124` 固定三类证据、版本/授权来源、规范化、preview/hash 与重放次序。`design.md:3` 明确其为唯一线协议。 | 已闭合。issue.revision 与 execution.state_version/结果 digest 分开；最新验收与当前描述验收各自稳定排序，不以更正时间替换首发时间；重放返回原结果版本，未冒充最新版。 |
| AR-04 旧端探测 | `design.md:63` 与 `api-contract.md:24` 改为合法工作空间 UUID 的 project-capabilities 子资源；禁止旧错误路径；404 需结合工作空间仍可读，400/401/403/network/schema 不混成 unsupported。`test-spec.md:93` 要求真实旧 router 契约测试。 | 已闭合。现有 `router.go:1692`—`:1699` 提供 URL 成员授权组；旧 `/api/projects/capabilities` 命中 UUID 400 的事实已准确记录，不再依靠新端路由优先级“修复”旧端。 |

## 修订一致性核对

- **RR 授权与版本**：项目、成员和证据采用锁内判断；成员/来源在 RR 建立快照后发生变化，不将普通旧 SELECT 当当前授权。`design.md:75` 明确 40001/40P01/55P03 整事务重试、原 request_id/payload 不变；业务 409 不静默重试。`api-contract.md:124` 将已成功幂等回放放在新 preview 比较之前，同时保留当前项目/访问权检查。没有新增权限来源。
- **历史与署名**：`api-contract.md:49` 排除统计递归，`:51` 明确多个验收的规则，`:77` 使用 `GetUsersByIDs` 的历史展示路径且不返回 email。已离开或删除成员的身份仍可表达，不依赖当前成员表的 inner join 丢失原作者。
- **执行证据读权限**：`api-contract.md:95` 复用现有 chat creator 与 agent 可见性语义，并要求事务查询器。已复核 `server/internal/handler/chat.go:1764`—`:1769`、`:1825` 以及 `agent_access.go:157`—`:178`；设计没有用 canInvokeAgent 替代读取权限，也不引入执行动作。
- **删除自动化**：`design.md:143` 使用实际 `status='paused'`、`pause_reason='project_deleted'` 与 `autopilot_trigger.enabled=false`，archived 保持不变；与现有 `server/pkg/db/queries/autopilot.sql:73`、`:89`、`:225` 的表示一致。进展专属数据删除、执行/任务历史保留的原范围未扩大。
- **数据 down**：`design.md:151` 明确保留数据/config/revision 的拒绝条件、停止 writers、锁内二次 guard、独立索引恢复与不承诺恢复旧 FK；`test-spec.md:110` 对应空库、填充、并发和索引失败。该路径是隔离演练与安全维护设计，未把破坏性 down 当常规生产回退。
- **分页可用性**：`design.md:155`、`test-spec.md:112` 新增持续变更时刷新比例、第二页到达率/耗时和连续重置次数；基线后锁定预算，没有只凭单请求 P95 宣称风险定位可用，也没有预先实现第二套统计存储。
- **任务衔接**：`implement.md:9`—`:17` 区分 FG/HG/PG/UG 与整个任务完成，foundation 在 FG 后继续负责共享整合。已读取五份子任务 `task.json`：`depends_on=[]` 均辅以明确 start/integration gates 和父文件证据引用；没有将下游启动依赖写成 foundation 必须完全结束，避免共享 SQL/worker 整合的循环等待。

## 剩余权衡与验证边界

首选 A 的最强反方仍是活跃大项目下频繁刷新妨碍翻页；B 能稳定重现点击时集合，但增加回收、当前权限重查和过期事实解释。修订保留 A，并把活跃分页纳入真实验证后再决定是否评审 B，综合路径成立。旧无 token 描述写返回 428 仍是明确兼容取舍；它保护 CAS，不能宣传成旧描述编辑无感兼容。

三个 pre-mortem 的 unit/integration/E2E/observability 路径保留；新增屏障与协议 fixture 对应首轮缺口。剩余风险应由已列出的真实数据库、混合版本和平台验证来判定，不需要本轮继续加入新的产品功能或通用基础设施。

本轮未发现必须再次回改的设计项。可按约定顺序进行 Critic 复核；最终规划完成仍由父任务审核任务结构、来源映射和评审结论。任何后续“通过测试/可发布”的声明，必须另取实际实现 commit、运行环境及测试证据。
