# P1 首轮评审修订记录

日期：2026-10-05。仅修订规划；未实施产品、未运行产品测试，等待Architect→Critic复审。作者只修改父`prd.md`、`design.md`、`implement.md`、`test-spec.md`，新增`api-contract.md`与本记录；子任务与metadata由父代理同步。

| 审查项 | 修订落点 | 新增可证明条件 |
| --- | --- | --- |
| AR-01 / Critic 1 | design §5/6；api-contract §4/5；test-spec P1-FR-05/06 | txStarter.Begin后第一SQL SET TRANSACTION RR READ WRITE；所有读取/preview/写使用同qtx；成员/来源locking read检查并发撤权，40001/40P01/55P03最多3次整笔重试；双连接阶段屏障验证无混合快照 |
| AR-02 / Critic 2 | design §3/6/8；implement §2/5；test-spec P1-FR-10/11/12 | 无锁候选→RC单条正序锁→inbox/outbox同commit，5秒轮询/指数退避/12次deadletter，main.go注册与ctx退出；投递/删除/撤权两顺序及双worker/崩溃无环；自动化实际paused/pause_reason/trigger.enabled语义 |
| AR-03 / Critic 3 | 新api-contract §2—6；design交叉链接；test-spec AC-05/18与FR-06/09/13 | 独立非递归snapshot；完整进展/修订/写回放/preview DTO，稳定排序/nullable/identity；latest/current-description验收区分；三类证据版本/授权、canonical hash向量与统计版本变化；历史署名最小字段 |
| AR-04 / Critic 4 | api-contract §1/6；design §4；implement §3；test-spec FR-13 | 改工作空间合法UUID子资源project-capabilities；真实旧router错误路径400、新子资源404；400/401/403/404/network/schema区别，404先确认workspace仍可读 |
| CR-01 | implement §1与§2；父代理同步子任务 | depends_on只指已完成整任务的启动前置，本方案均[]；preparation/start/integration gates分别列出FG/HG/PG/UG证据；foundation共享整合持续到下游归并完工，FG不等于归档 |
| CR-02 | implement §6/8；test-spec MO、FR-13/15及§7 | 读取mobile CLAUDE/package/vitest；mobile/data新增API/parser/Query/realtime/权限node测试，独立typecheck/lint/test命令；明确根检查排除mobile，保留原范围不加移动编辑 |
| 架构分页建议 | design §9；test-spec §6 | 活跃0/1/10次每秒变更各10分钟，测刷新比/第二页到达率与耗时/连续重置；先基线后预算，不虚构已达标、不提前加B |
| 父代理迁移边界 | design §9；test-spec §6 | 有保留数据或规划配置/版本变更时down guard拒绝；writers停止+排他锁内复查；空库/填充/并发/部分索引失败隔离fixture；不承诺恢复旧FK |

依赖gate交接：FG=基础schema/锁/事务/DTO与局部测试；HG=health API/采集函数；PG=progress API/outbox；UG=平台真实集成。每项都有commit/命令/结果后生效。五项depends_on=[]；foundation start=实施授权，health/progress/ui start=FG，verification start=FG用于真实服务集成；所有任务的测试准备均可在已评审合同后开始，verification基线第一波即可。完成gates按implement §1，不能用准备开始或FG代替完整产品完成。

自检：四父规划+API合同均须<32KiB，相互链接存在；保留PRJ-001—014、PRJ-AC-01—27及18补充FR；I1真实集成仍显式待办。本记录不是审查通过声明。
