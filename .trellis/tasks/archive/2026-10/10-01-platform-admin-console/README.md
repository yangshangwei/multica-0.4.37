# 平台管理后台规划任务

父任务：`10-01-platform-admin-console`。实施已获用户授权，S01–S06 已完成实现，S07 集成验收正在推进；父任务保留聚合追踪状态。

当前实施工作区：`/Volumes/artisan/code/2026/multica-platform-admin`，分支 `feat/platform-admin-console`。真实进度见 [S07 验证记录](../10-01-platform-admin-acceptance/verification.md)。下文原规划快照保留为历史依据。

用户已确认：内部企业先落地、后续多客户；管理员复用普通用户的账号和认证，独立平台权限；首期100–1000台桌面终端。

新会话先读 [implementation-handoff.md](implementation-handoff.md)，其中记录当前实现与验收状态。[handoff.md](handoff.md) 保留首次规划交付的历史快照。

## 建议阅读顺序

| 文档 | 主要内容 |
| --- | --- |
| [prd.md](prd.md) | 需求、角色、分期、容量目标与13项验收 |
| [design.md](design.md) | 方案选择、核心决定、端到端链路和设计分工 |
| [architecture.md](architecture.md) | 服务、包、账号、组织与数据边界 |
| [detailed-design.md](detailed-design.md) | 六个首期菜单、页面、接口、DTO、指标与告警 |
| [security-design.md](security-design.md) | 平台授权、凭据转换、撤销、账号恢复与审计 |
| [terminal-execution-design.md](terminal-execution-design.md) | 可信安装、daemon关联、状态、取消和接单门禁 |
| [capacity-plan.md](capacity-plan.md) | 100–1000台、速率模型、分页和实测目标 |
| [migration-rollout.md](migration-rollout.md) | 数据迁移、启用顺序、灰度和安全回退 |
| [test-spec.md](test-spec.md) | 验收矩阵、权威测试层、故障和兼容证据 |
| [implement.md](implement.md) | 实施依赖、共享文件所有权和验证节奏 |
| [research/current-state.md](research/current-state.md) | 用户决定、源码事实与复用点 |
| [research/design-review.md](research/design-review.md) | 独立设计审阅结果及处理记录 |
| [verification.md](verification.md) | 本轮文档与任务结构验证，区分未来功能测试 |

## 已创建的7个子任务

1. [S01 身份权限与后台入口](../10-01-platform-admin-access/prd.md)
2. [S02 可信终端台账与上报](../10-01-platform-admin-installations/prd.md)
3. [S03 全局任务与执行查询](../10-01-platform-admin-executions/prd.md)
4. [S04 执行取消、接单门禁与回执](../10-01-platform-admin-controls/prd.md)
5. [S05 账号管理、恢复与撤销](../10-01-platform-admin-accounts/prd.md)
6. [S06 总览、告警与服务健康](../10-01-platform-admin-observability/prd.md)
7. [S07 集成验收、容量与发布演练](../10-01-platform-admin-acceptance/prd.md)

每个子任务都有独立prd/design/implement和上下文JSONL，依赖同时写在文档与task.json。父任务维护共同契约；子任务不能自行改变身份、权限、表名或操作语义。

## 当前交付范围

设计选择复用现有Go/Next/PostgreSQL，不新增依赖，不拆新控制服务；主菜单八个、首期开放六个。P2能力策略/独立分析/升级批次和P3多客户均为路线图，不混入首期验收。

工程参数为拟定目标而非实测。开始实现时先评审最新方案，再启动依赖满足的S01；当前父任务保留planning，实施与生产发布尚未发生。
