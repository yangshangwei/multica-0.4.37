# I1 验收与测试规范

本文件是待执行规格，不是通过记录。产品需求编号均见源 iterations-prd.md；每项未来必须记录 fixture、command、commit、result。

| ITR-AC | 规范场景 | Canonical 层 / 子任务 |
| --- | --- | --- |
| 01,02 | 默认关、确认时区、成员配置拒绝 | handler DB / foundation,lifecycle |
| 03,04,29 | 多计划、两连接同时开始唯一active、空基线null比率 | handler DB / lifecycle |
| 05 | 过期只提醒、执行/状态不变 | service clock+DB / closure |
| 06 | pending/rejected/duplicate/未知准入跨入口拒绝 | handler/service DB / lifecycle |
| 07,08,09 | 不动执行与项目；单归属双边日志；历史拒补 | handler DB / lifecycle |
| 10,11,12,13,14 | 固定O、A—J算例、移出后外部done不计、父子独立、全取消/空null | pure stats node/Go canonical / history |
| 15,16 | 仅R结转、终态释放、H次数1→2重试仍2 | closure DB / closure |
| 17,18,28 | 目标取消、响应丢失、预览后done，整体冲突/重放 | 两连接+故障注入 DB / closure |
| 19,20 | 执行继续、之后重开删除换项目不漂移 | service+DB snapshot digest / history,closure |
| 21 | 曾有关联空计划不能删除，纯改名可删除 | handler DB / lifecycle |
| 22,23 | 整空间禁用、任一冲突保持全部原状 | 两连接 DB / closure |
| 24,25 | 默认时区仅新计划、不随查看者位移 | pure time+API / foundation,clients |
| 26 | 旧PUT缺字段不清归属；未知响应安全降级 | handler+schema fixture / foundation,clients |
| 27 | 撤权后历史/通知标题计数附件不可读 | DB race / closure |

## 必须追加的工程场景

1. Start 前计划内变 done/cancelled 的明确 retain/remove、early today 保持14日历天、expired拒绝、active开始日锁定、改结束日理由、历史更正不改 snapshot。
2. 非O任务首次期中加入→移出→重入得到added_unique=1/reentry_events=1；O成员重入不增加新增；计划期关联后start前移出、start后首次入期仍新增1。当前范围空、完成后移出/重入、移出/删除不缩O、普通换期不清累计结转，三次提示；图表 baseline/日末/延期一致，事件稳定顺序；净有效变化分母0返回null；blocked无启动记录不计started，执行启动无状态变更也计started，移出重入按新参与重置。
3. End-and-start 任一目标/原计划成员/权限变更全回滚，新O包含原计划+结转；多个目标仅一个 next active；全部R必须有去向，重复/遗漏/额外终态目标拒绝。
4. 整体 disable 包括超出第一页的每个未来计划；超部署上限413无任何变化；batch move同样不截断。
5. 同 request不同payload409；事务提交响应丢失→GET/同ID重试返回原结果；GET404不换新ID；create/edit/delete/enable也重放；原实体删除后授权本人可回放，撤权不能回放保护内容。
6. 关闭、移动、状态更新、daemon完成、任务删除、状态目录重分类、项目删除、成员撤权和开始竞态，每对两连接 barrier 验证线性化；所有 writer inventory 条目至少一个真实路径断言。运行中 issue-task 计数和进程身份不变。
7. 两连接在获取 workspace fence 前后反转到达，证明没有 issue→iteration fence 反序；55P03/40P01重试不重复日志，超限503。测同空间普通任务写吞吐和锁等待，不能只测迭代空载。
8. Outbox 持久后进程崩溃、投递后标记前故障、再次发送、接收者在投递前撤权均最多一份可见摘要；逾期跨本地日期最多每日一次；started_by撤权无个人提醒；不发送外部消息。
9. America/New_York 春秋 DST、Pacific/Apia 跳日、午夜 gap/fold 时区真实 tz fixture；边界为日历下一天首有效时刻，非24h；保存 Local/偏移/不存在IANA拒绝；旧计划时区不可改。
10. T1 accept+iteration+project 单事务（目标冲突时仍 pending）；accept_and_execute仍一次执行；CSV迭代未采用；新任务、CLI、旧桌面/mobile显式迭代写缺确认428。WM-AC-02/04/05/06/07/08/10/11/12 对应纳入。
11. 当前 project 删除保留 iteration 指针/历史；P1 到位后对其专属cleanup另做联合fixture。跨空间移动带参与历史409；工作空间删除按现有权限清除本空间全部迭代记录，无 orphan/no FK。
12. 空/用过/部分迁移/invalid concurrent index/recovery/down guard/并发writer隔离库演练；P1共享时区已用时I1不能drop；任何归属、审计、操作存在拒绝down。
13. 核心DTO malformed/缺身份/错workspace/未知enum/负数/非安全整数/错误日期；capability 404与401/403/network区别，不能 fallback写成功。旧服务不支持新字段明确限制。
14. start today/handoff 在本地午夜前预览、午夜后提交必须409并返回新日期；两个事务开始顺序与取得fence顺序相反且跨午夜，事件采用锁后采样，sequence和日末图表一致；注入时钟回拨不倒排事件。
15. Web和Desktop真实全流程：创建→安排→开始→完成一项→预览结转→历史；键盘完成、loading/403/409/unknown result保留输入、长名称、图表数据表。Mobile不依赖core hook，仅兼容读取/编辑；新页面后置不等于可绕过。

## 验证分层与运行

纯统计/规范化/时间 helper 只在一个 canonical 测试层做全矩阵；组件只验证接线与用户交互，避免重复helper矩阵。DB采用 server/internal/testutil 的 dbfx/Call，真实Postgres两连接而非内存替代。e2e 使用 TestApiClient 准备/清理数据。默认测试不得调用本机真实agent CLI；用 fake executable/已有fixture，执行不干扰断言不需真实账户。

迁移、全Go、TS类型/lint/unit、双端e2e和 make check 的命令/输出未来写 verification.md；1,000任务与更大禁用总集合记录硬件/DB/数据分布/冷热/P95/锁等待/查询计划/实际选择阈值。性能建议2秒不是现成通过线，实施测基线后冻结验收线。可观测性统计 operation conflicts/replays/transaction retries/outbox pending/dead_letter，日志不得带任务正文或凭据。
