# 下一会话交接

本包实现ca0c8efd3，在05de761de之后推进显式普通任务完成提示API；已独立验收，归档d3b0b17db、Session151交接609219ed3并已安全快进业务分支；其后有本地集成证据提交，先核对最新HEAD。先检查业务分支最新HEAD，不回退。来源mc-delivery-completion-hints-20261110已完成，不当待续实现。

POST /api/issues/{id}/delivery-evidence/completion-hint，human-only，三个字段expected_issue_revision/target_status/evaluation_key（null或UUID）。复用原政策/评估/观察器/current artifact权限，不选latest、不重算原评估、不写状态。custom done按category；原pass+current才matching；off/no-project/non-done不适用；所有success enforced/usable_for_completion=false。无UI/CLI自动提示、skip或全入口阻断。独立协作返回原unsupported。

最终13文件冻结，handler普通/race各325（76顶层）、真实路由各103（37顶层），全0失败/跳过；合同50（service37/status11/migration1/runtime1）；build/vet/sqlc/gofmt/diff通过。独审核对29日志含9final+4fail及源码13/13。NUL真实503→400red/green；两轮夹具及格式失败保留。10库/日志保留，multica-completion-hints-20261110-pg在0连接后停止，不借用/清理它。

下一包完整强制门禁先读research/completion-writers.md与enforcement-decisions.md。须覆盖普通/批量逐条事务、move、create-done/commentchild、GitHub/VCS、协作精确验收/自完成/R38管理例外、custom done。原人类评估共享/机器消费、create-done、已done编辑/换项目以及skip边界尚待决定。owner/admin管理项目政策已定，无需再问。skip严格推荐已发问但未获答案，不能当批准或实现。

M2快照→父总预算→能力/负载调度；M3NFR/评测/供应链/模板知识。六业务与M1仍in_progress。真实模型调用0；AC43未通过完整供应商小队；E5用19余8停止，H3a用12/25余13停止，知识E0b两次403用2/17余15且CLI/hash/native gateway需重绑。无新增预算/停止例外，无推送/部署/消息。
