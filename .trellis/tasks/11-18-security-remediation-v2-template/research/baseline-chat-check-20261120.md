# 基线聊天测试失败复核

初次整包日志 `security-remediation-v2-evidence-20261120/logs/handler-20261120-071745.log` 中 TestClaimTask_ChatDeliversAllUnansweredUserMessages 返回“还有青岛\n\n深圳呢”，期望只有深圳。该整包稍后在30分钟截止超时（3946 RUN，3921 PASS，1 FAIL，7 SKIP），不算完成。

独立只读诊断：精确原测试在probe库连续3次通过，无跳过，`logs/probe-20261120-074657.log`，7.187秒。daemon_test.go、daemon.go、chat.sql及生成查询未被本任务改动。旧式task查询按created_at,id排序，trailingUserMessages取最后非user消息后的tail。夹具分别用now()+1/+2/+3秒；普通慢调度不会逆转其顺序，数据库时钟回拨或未观察到的集合变化只是可能原因，未保存失败时的行时间戳，不能确定。

没有发现claim路径或相关trigger会改这些旧式行时间。未修改产品/夹具/断言，也未把3/3定向通过替代整包失败。改用缓存的arm64 pgvector镜像在本任务新的专有容器与新库重新跑完整包；保留原amd64容器及失败日志直到证据整理。原镜像在aarch64引擎中运行是环境事实，不能单凭这一事实证明它导致失败。

未来若复现，先用独立overlay记录该session的id/role/created_at/task_id及clock_timestamp，再决定根因。固定UTC基点可作为单独测试稳定性修复，本任务未实施。


## 精确锚点与诊断边界

- 原始失败见 `handler-20261120-071745.log:4795–4797`；用例位于 `server/internal/handler/daemon_test.go:3295`，失败断言位于 `:3348`。
- 精确复跑命令：`python3 /Volumes/artisan/code/2026/security-remediation-v2-evidence-20261120/run.py test probe ./internal/handler -run '^TestClaimTask_ChatDeliversAllUnansweredUserMessages$' -count=3 -v -timeout=5m`。退出 0，RUN/PASS 3，FAIL/SKIP 0。
- `git diff efdb597d6 --` 对 `daemon_test.go`、`daemon.go`、`server/pkg/db/queries/chat.sql`、生成 `chat.sql.go` 没有本任务差异；`git diff ccd28cbc0..efdb597d6 --` 对前三个文件为空。该路径先于本任务存在。
- `daemon.go:2951` 选择旧式消息读取，`chat.sql:815` 按 `created_at ASC, id ASC` 排序，`daemon.go:3811` 从最后一条非 user 消息后截取。这个用例没有设置 `chat_input_task_id`，因此 `service/task.go:3528` 的 task-owned reanchor 条件不成立。
- `chat.sql:895` 的 `ReanchorClaimedDirectChatInput` 与 `:982` 的后继消息重锚均针对 task-owned input；该夹具直接插入无 task_id 的消息。迁移中唯一相关更新触发器 `379_channel_context_mixed_version_guard.up.sql` 作用于 `task_id` 更新，不改这些行的时间。
- 假设排序：数据库墙钟回拨；未观察到的消息集合/行变化；读取实现的问题。只排除了普通慢调度足以造成次序逆转这一说法，没有重建原始失败时的行时间戳，不能把任何假设写成已证实原因。
