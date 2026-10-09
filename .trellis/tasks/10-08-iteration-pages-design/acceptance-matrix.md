# Final acceptance evidence matrix

Status: functional evidence is recorded for the isolated candidate. The final combined browser rerun and Git handoff remain pending current permissions; see verification.md. Historical failed runs are preserved, not overwritten.

| Criterion | Observable requirement | Canonical evidence | Final result |
| --- | --- | --- | --- |
| AC-01 | 总览中较晚计划在较早计划上方；当前仅一条且默认展开；切到历史标签后较新记录在上。点击三类行分别打开当前、计划、冻结历史详情，原有直达链接仍有效。 | timeline/catalogue and navigation tests; actual overview/history navigation | Recorded; see verification.md |
| AC-02 | 10 月 1–7 日与 10 月 10–16 日之间显示“2 天未安排”；相邻、重叠、同日期、跨时区、缺页或筛选状态不误报间隔。不会生成冷却期设置。 | core timeline boundary matrix; locale date grouping and screenshot checks | Recorded; see verification.md |
| AC-03 | 超过一个 API 分页的周期仍按正确顺序显示；当前不会因位于后续页而被漏掉。失败提供重试；无周期与筛选无结果有不同反馈。 | complete catalogue/cursor tests and navigation pagination regressions | Recorded; see verification.md |
| AC-04 | 总览只对展开的当前周期请求统计，未来／历史行不逐行加载详情。图的每个已绘制日期及数值均来自响应，表格与图一致。 | current-only detail request and shared chart table tests; keyboard disclosure E2E | Recorded; see verification.md |
| AC-05 | 计划详情空状态符合参考图的左对齐和留白；添加可选择并确认现有任务，新建可确认本周期归属。成功后出现任务；取消、失败或结果未知不伪装成成功。 | manual scoped-create tests; actual picker/create membership E2E; planned-empty screenshot | Recorded; see verification.md |
| AC-06 | 三个详情标签页可由键盘切换；任务筛选与分组仍有效，任务筛选不改图表；切换标签不丢失当前筛选、编辑草稿或待恢复操作。 | detail draft/tab navigation tests and overview tab/panel keyboard regression | Recorded; see verification.md |
| AC-07 | 已完成、有效范围、剩余均为任务数；零分母显示不适用；统计失败显示错误或带提示的上次成功值，不显示 0%。 | history/graph tests including null ratio and failed-read cases | Recorded; see verification.md |
| AC-08 | 当前任务后来改名、换负责人或重开，历史表格、曲线及冻结完成率保持原样；历史对比入口明确区分当前值；名称／说明更正不重算快照。 | Go canonical history tests and Web/Electron immutable snapshot flows | Recorded; see verification.md |
| AC-09 | 普通人类成员保留维护周期的既有权限；机器凭据不会因新按钮获得权限。冲突须重新确认，响应丢失沿原身份恢复；状态已变化或功能已关闭也仍能恢复。 | handler/service permission/conflict tests; durable command/recovery E2E | Recorded; see verification.md |
| AC-10 | 未启用及旧服务状态不展示写入口，已关闭仍可读历史；未知状态／模式只读且有说明。与父设置页集成后可从业务页到真实设置入口，再回到历史。 | unsupported/disabled/unknown-mode tests; settings and retained-history navigation | Recorded; see verification.md |
| AC-11 | 中文／英文、1440px 与 900px 桌面窗口、深浅色中无横向页面溢出；图右侧统计可换行；按钮、标签页、对话框有焦点反馈，图表可通过数据表理解。 | locale parity and real light/dark 1440px/900px screenshots; chart table keyboard access | Recorded; see verification.md |
