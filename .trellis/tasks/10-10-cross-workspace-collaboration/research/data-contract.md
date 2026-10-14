# 协作数据与持久约束

本文件承接 [主设计 D02](../design.md#d02-持久数据与约束) 的完整数据契约；实现和检查均自动加载。以下内容自 D02 无损移入。

所有身份用全局用户 UUID。新增 epoch/seq/generation/协作 revision 使用 BIGINT 和无损十进制字符串；API 命名 `collaboration_revision`、`expected_collaboration_revision`，旧 Issue.revision 数字 wire 类型不变，新 DTO 不写入旧 Issue 缓存。比较不用 Number/字典序，schema 拒小数、科学计数和越界值。时间不承担排序或执行身份。

| 数据 | 核心字段及职责 | 约束 |
| --- | --- | --- |
| `issue_collaboration` | issue_id、revision、authority_epoch、lifecycle_epoch、active_information_request_id、created_at | issue_id 唯一；处理人和发起人只读现有 issue 字段，不双写另一套权威身份 |
| `issue_participant` | issue_id、user_id、epoch、joined_at、revoked_at、revocation_reason | 每 issue/user 一行；离开留墓碑，重加提升 epoch，历史不删除 |
| `issue_handoff` | id、issue_id、actor/from/to、purpose、note、requester、parent_request_id、previous_status、resolution | 追加交接事实；return 引用具体请求，不能从“上一处理人”猜接收人 |
| `collaboration_session` | id、issue_id、queue_state、control_version、event_seq、next_command_seq、resource_epoch、active_grant_id、active_run_id、pause_reason | 每 issue 一个永久逻辑 session；重开和换环境不另建并行 session |
| `collaboration_grant` | id、issue_id、owner_user_id、owner_auth_epoch、execution_workspace_id、agent_id、runtime_id、installation/binding、epoch、consented_at、revoked_at | 对完整资源组合授权；旧行不可换 owner/runtime，重新授权创建新记录 |
| `collaboration_command` | id、session_id、seq、author_user_id、participant_epoch、author_auth_epoch、lifecycle_epoch、内容/附件快照、state、reason | session/seq 唯一；内容、作者和入队身份不可被评论编辑覆盖 |
| `collaboration_run` | id、session_id、command_id、task_id、grant/epoch、execution scope、fence、claim_generation、launch_nonce、progress/proof/disposition、settled_at | task_id 唯一、command_id 唯一；session 至多一条 settled_at 为空的 run |
| `collaboration_run_event` | run_id、producer_event_id、seq、type、payload、来源归属、结果/用量事实 | producer key 唯一；内容留在来源侧，engine task 删除不删除这些事实 |
| `collaboration_operation` | scope_kind=create/issue、scope_id、actor_user_id、request_id、kind、payload_hash、结果/版本、resolved_issue_id | scope/actor/request_id 唯一；创建 scope_id 为来源空间，其余为 issue；拒绝结果也持久化 |
| `collaboration_event` | issue_id、event_seq、actor、事件及安全投影、outbox 状态 | issue/seq 唯一；变更与审计、通知投递意图同事务提交 |
| `collaboration_notification` | id UUID、recipient_user_id、issue_id、event_seq、kind、mention 标志、最小投影、read_at、archived_at | id 唯一且稳定；recipient/issue/event_seq 唯一；R52/R55 账号级通知，打开时按当前权限重验，不复制正文快照；原事务固定收件人，不在投递重试时重新展开参与者 |
| 协作普通评论 | 复用 comment 及既有 revision BIGINT（迁移 351）；新增 deleted_at | R43 复用现有版本递增，不新增第二版本源；新协作 DTO 用 comment_revision 十进制字符串，旧评论 revision 数字 wire 不变。删除保留 UUID/作者/回复锚点和墓碑并清当前展示正文，禁止旧硬删除；编辑前后正文、版本、真实 actor/时间写 collaboration_event，受本单权限保护 |
| 线程与回应 | 复用 comment.resolved_*、comment_reaction、issue_reaction | R57 在共同 issue 锁后重验访问，清旧解决+设新解决同事务，最多一个线程解决记录；回应按真实 actor 唯一，只删本人；均不触发执行/通知 |
| `collaboration_attachment` | id、source issue 或私有 draft、uploader、storage key、固定版本、delivered_at | 独立于 comment 的永久交付归属；每次绑定和读取重验同单，draft 只能消费一次，不接受任意公网 URL |
| `collaboration_attachment_reference` | attachment_id、reference_kind、reference_id、reference_revision、removed_at | comment/command/run 引用同一固定文件；唯一引用身份；移除评论展示只标记该引用，交付归属和其他引用保持；GC 不删除已交付文件 |
| 终结凭据记录 | run/fence/launch、原 installation 身份、token hash、proof、receipt id/digest | 不依赖当前账号/runtime 行存活；只服务本 run 终结，确认后留幂等墓碑 |
| 协作身份与准入 | 中立 `collaboration_account_fence`、显式 `collaboration_runtime_enrollment`、部署 `collaboration_execution_gate` | 版本来源、注册和更新者见执行协议 E06–E07；不以空版本/空 binding 兜底 |

创建操作先占来源空间/creator/request_id，原子分配唯一 issue 并消费私有 draft（uploader、purpose、一次绑定状态）。响应丢失可按创建 scope 找原 issue；同 key 改正文或同 draft 换 key 二次消费拒绝，不能创建第二单。

v1 不做服务端隐式失败重试；provider 内部尝试仍属同一 run，按执行协议 E04 监督。手工重做创建新 command。

不新增数据库 FK、REFERENCES 或级联。新表不写会隐式建索引的 inline PRIMARY KEY/UNIQUE；先建表，每个 ID 唯一索引、组合/部分唯一索引及查询索引分别用单语句 `CREATE [UNIQUE] INDEX CONCURRENTLY` 迁移。需要命名约束时只附着已有合格索引，不另建索引。迁移后核对索引有效性；未完成所有约束前禁止创建协作数据。

查询索引至少覆盖来源空间存在性检查、participant(user_id, revoked_at, issue_id)、未完成的用户 assignee、command(session_id, state, seq)、未结 run、待投递事件、notification(recipient_user_id, archived_at, read_at)。唯一性冲突返回可重放的业务结果，不用忽略错误掩盖两条活跃执行。
