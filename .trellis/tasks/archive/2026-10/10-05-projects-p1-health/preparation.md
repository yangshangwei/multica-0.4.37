# Health 实施准备

2026-10-05（准备时点）：已核对本 worktree 的 CLAUDE、T1/server 规范、父 design/api-contract/test-spec、health 子任务及实际 536 新表。当时尚未启动产品实现、未运行产品测试；后续 FG/HG 实施结果见 [verification.md](verification.md)。

## 单一采集入口

拟提供 `projecthealth.Collect(ctx, q *db.Queries, project db.Project, now time.Time) (Collection, error)`。调用者通过 foundation 的 `runProjectTransaction` 在首条 SQL 设置 REPEATABLE READ，并完成 workspace → member fence/locking read → project shared lock；所有读取和进展发布使用传入的同一 q，采集函数不开第二事务。

`Collection.Statistics` 是 API 合同的非递归 `ProjectStatisticsSnapshot`；`Collection` 内部同时保留四种风险的稳定 issue ID 集合。overview 与分页复用本次采集；progress 只使用 Statistics。验收摘要由独立查询生成，不加入统计快照或 digest，避免递归增长及更正干扰发布时间。

`sql-proposal.sql` 提供四个窄查询，交 foundation 落入唯一 sqlc 源。正式输入不取正文/metadata；下钻只对本次精确风险分页 ID 取完整 Issue，二次同租户/项目/正式准入条件作为防御。最新发布时间由 MAX 固定 published_at 取得。最近验收与当前描述验收各取最新一条再 UNION；不按 conclusion 偏好 passed、不按 created_at 排更正。

现有目录/引用查询可先复用：ListIssueStatusEntries(include_archived=true)、ListMembers、ListAllAgentsAnyKind、ListAllSquads、ListSquadMemberPreviewRows、ListAgentRuntimes。成员 identity 使用 member.user_id，不是 member.id；agent/squad 引用不读取查看者 invoke 权限。Squad 有效性要求未归档 + 同空间有效 leader；环境另检查 leader 与完整 agent roster 的 runtime 绑定/在线状态。大量无关 workspace agent 输入是否需要新窄查询交性能实测判断，不先添依赖或索引。

## 纯逻辑与 digest

- 目录按 key/category 处理含 archived 自定义 key；内置类别直接可识别，未知 key 保留 N/U，unknown_status 增加，complete=false、health=unavailable。任何查询失败返回错误，不用 0 伪造成功。
- N/F/C/U 与旧 done_count=N-U 分开；B/O/A/R 只从 U 取得；四集合并集去重，父子按实际 ID 分别统计。
- D 由 planning_timezone 算得，无配置明示 UTC。due<D；paused 项目仍逾期，completed/cancelled 仅结束项目逾期标签，不抹去未结束任务风险。
- 七个日历日由日期差计算，不使用 168 小时；无进展时从 in_progress_since 开始（migration 基线保留来源），有进展时固定首发时间，更正不重置。
- digest 包含租户/项目身份、project revision、时区配置及 D、已排序 key/category 目录、正式成员影响统计字段、有效引用/环境相关底层字段与最近发布时间；排除 calculated_at、姓名/显示名/图标、正文及排版。相同集合数量但成员换人、due/assignee/目录/时区变化必须换 version。
- cursor 绑定 workspace/project/signal/version/last_id；signal 只支持四值，分页最大 100。版本改变返回新第一页和 refreshed=true；禁止接受个人过滤作为统计边界。不完整统计拒绝精确分页。

## RED → GREEN 验证矩阵

1. 纯 Go：6 done + 2 cancelled + 2 todo → N10/F6/C2/U2/0.8，empty ratio=null，仅 cancelled 不等于验收成功。
2. 真实 DB：五 admission × actual/candidate project × workspace，正式父子均保留；候选 project 不计；已有 T1 接受回归保持执行行 0。
3. 真实 DB：blocked+昨日+未分配同一任务三卡各1/union1；today/null/terminal date 不计逾期；四精确下钻 + 多页及变更刷新。
4. 真实 DB：member 已退出、agent 未归档但 runtime 离线/私人不可调用、archived agent、squad 无效 leader/成员运行环境失效；有效引用与环境分离。
5. 纯逻辑 + DB：archived custom in_review、unknown 状态；目录加载错误无法显示绿色；同计数不同 ID 和仅显示名称变动的 digest 对照。
6. 纯 Go：UTC 缺省、Asia/Shanghai 日期、America/New_York 春秋 DST、D-6/D-7、重开/迁移/更正/项目结束状态。
7. 真实 DB：同一 RR 采集两次期间另一连接修改任务/目录/引用，当前事务版本一致，下次事务版本改变；read 权限撤销不从旧 RR 快照恢复。
8. 真实 DB：最新验收在旧描述但当前描述存在较早 failed，两摘要独立；更正不改变首发排序，作者离开/删除使用最小历史身份。

测试使用 `.env.worktree` 的独立 DATABASE_URL、`scripts/go-test-with-agent-cli-guard.sh` 和既有 dbfx/testutil；不会执行真实 agent CLI。HG 必须有纯逻辑、真实 DB 回归、共享接线和窄 go vet 的实际结果。
