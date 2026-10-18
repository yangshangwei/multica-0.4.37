# 协作数据与持久约束

2026-10-17：M01–M11 的完整数据契约，承接[设计 D02](../design.md#d02-持久数据与约束)。已落实的迁移/目标/组/控制见[当前代码规范](../../../spec/server/collaboration-targets.md)，其旧记录隔离按 M10 待删，按目标控制将由 lane 与目标 hold 取代；评论原子提交、完整运行协议等仍待实施，不把本附表全部视为已交付。

账号用全局 user UUID；新增 epoch/seq/generation/revision 为 BIGINT 十进制字符串，协作版本名 collaboration_revision/expected_collaboration_revision。旧 Issue/comment.revision 数字 wire 不变，新 DTO 不写旧缓存。不用 Number/字典序，拒绝小数/科学计数/越界，时间不作执行身份。

## 聚合、目标与执行

| 数据 | 核心字段及职责 | 持久约束 |
| --- | --- | --- |
| issue_collaboration | issue_id、revision、authority_epoch、lifecycle_epoch、active_information_request_id | issue_id 唯一；creator/handler 只读原 issue，生命周期 epoch 保持单一权威 |
| issue_participant | issue_id、user_id、epoch、joined_at、revoked_at、revocation_reason | 每 issue/user 一行；离开留墓碑，重加提升 epoch，不删除历史 |
| issue_handoff | id、issue_id、actor/from/to、purpose、note、requester、parent_request_id、previous_status、resolution | 追加事实；return 引用具体请求，不猜测上一处理人 |
| collaboration_session | id、issue_id、queue_state、control_version、target_set_version、event_seq、pause_reason | 每 issue 一个永久全局控制器；queue_state=active/paused/closed，生命周期与 issue_collaboration 原子联动；没有权威 active_grant_id/active_run_id |
| collaboration_target | id、session_id、kind=agent/squad、resource_id、target_epoch、config_version、资源名册、hold_reason（initial/grant_required/config_invalid/switch_hold）、revoked_at | session/kind/resource_id 唯一；类型及资源 UUID 不可替换；重授权不另建绕过暂停的目标；hold 只挡本目标输入，新建、撤权、epoch/配置失效、promote 发现失效与 Switch 均写 hold，仅明确 Resume 释放，重新授权不自动释放（R25） |
| collaboration_lane | id、session_id、root_agent_id、queue_state、control_version、group_generation、next_command_seq、active_group_id | session/root_agent_id 唯一；@A 与以 A 为队长的小队共用（M06）；每 lane 一条外部 FIFO、至多一个未结 group；Stop target 作用于所属 lane，授权/配置失效与切换只置目标 hold |
| collaboration_grant | id、issue_id、target_id、config_version、owner_user_id、owner_auth_epoch、execution_workspace_id、agent_id、runtime_id、installation/binding、epoch、consented_at、revoked_at | 每行覆盖明确目标配置/资源，owner 分别确认；队长 grant 不覆盖整队，owner/runtime/目标不可原地替换 |
| collaboration_command | id、target_id、lane_id、group_id（promote 或 M11 并入前为空）、input_event_id、seq、author_user_id、participant_epoch、author_auth_epoch、lifecycle_epoch、内容/附件快照、state、reason | lane/seq、input_event/target 分别唯一；lane 在入队时按目标当前配置确定；小队换队长时 waiting 同事务转入新队长 lane 末尾并 switch_hold，Resume target 后才可 promote（E07）；内容、目标、作者和入队身份不可被评论修改覆盖；一条评论可关联多条目标 command |
| collaboration_execution_group | id、lane_id、batch_role=agent/squad、squad_target_id、generation、批次内各 target 的 target_epoch/config_version 与冻结名册/grant 集合（M11 并入时追加）、state、expansion_sealed_at、forced_disposition、settled_at | 批次 command 经 command.group_id 关联（M07）；每 lane 至多一个 settled_at 为空的 group；group 持有 lane 位置直到成员与 continuation 全部结清 |
| collaboration_run | id、group_id、parent_run_id、step_key、step_kind=agent/leader/member/continuation、task_id、实际 agent/grant/资源及 epoch、fence、claim_generation、launch_nonce、progress/proof/outcome/disposition、settled_at | task_id、group/step_key 分别唯一；一组可有多条 run，取消 session 级未结 run 唯一性；parent 只指同组既有 run，禁止循环或串组 |
| collaboration_delegation | id、group_id、parent_run_id、source_comment_id、member_agent_id、成员 run、state | source_comment/member 唯一，仅本组存活 run 评论的 @成员生成（M08）；重放/编辑不增 run；队长 continuation 每组至多一个排队中，再唤醒即合并；只表达组内工作，不创建普通 issue 子任务 |
| collaboration_run_event | run_id、producer_event_id、seq、type、payload、来源归属、结果/用量事实 | producer key 唯一；来源侧持久化，不依赖 engine task/message 存活 |

session.event_seq 只排历史；lane.group_generation 在新根 group 占位时递增，session.target_set_version 在目标集合改变时递增。评论/进度/组内扩展不改控制版本。控制 scope=target/issue，绑定版本与观察到的 active_group/generation；停止与组内扩展共锁，见[执行协议](execution-contract.md)。

同评论重复 typed target 只建一条 command。@A 与 @S（队长 A）各建 command 但进入同一 lane 与批次，合并为一次以 S 身份的队长运行（M06）。lane 忙碌或暂停期间到达的 command 均 waiting；promote 按 FIFO 取合格输入为一批（M07），遇第二支合格小队截断，hold 或配置/授权失效者被跳过、不挡他人；显示名不作身份。实际 agent/机器容量及会话/目录互斥按 E01/E04，不能由 target_id 绕过。

command 固定目标而非旧 grant；group 在 promote 冻结完整配置/名册/grant。撤资源保留 waiting，换资源、代次校验、不复活旧工作与不迁移 proof 按 E07。

## 操作、内容与通知

| 数据 | 字段及职责 | 约束 |
| --- | --- | --- |
| collaboration_operation | scope_kind=create/issue、scope_id、actor_user_id、request_id、kind、payload_hash、结果/版本、resolved_issue_id | scope/actor/request_id 唯一；创建按来源空间，其余按 issue；payload 含 control scope=target/issue、目标/配置版本及执行代次；成功/拒绝均持久 |
| collaboration_event | issue_id、event_seq、actor、事件及安全投影、outbox 状态 | issue/seq 唯一；变更、审计及通知意图同事务 |
| collaboration_notification | 稳定 id UUID、recipient_user_id、issue_id、event_seq、kind、mention 标志、source_kind/source_id、最小投影、read_at、archived_at | recipient/issue/event_seq 唯一；原事务固定接收者；评论定位到评论，正文定位到正文。打开时重验当前权限，不复制私有正文，不在投递重试时重算参与名单 |
| 协作普通评论 | 复用 comment、既有 revision BIGINT（迁移 351），新增 deleted_at | R43 留 UUID/作者/回复锚点/墓碑，只清展示正文；DTO 用 comment_revision 字符串。前后正文/版本/actor/时间写 collaboration_event，不改 command/重触发 |
| 线程与回应 | 复用 comment.resolved_*、comment_reaction、issue_reaction | R57 在共同 issue 锁后验权，清旧解决+设新解决同事务；不同目标并发也最多一个解决记录；回应仅修改本人，不触发执行/通知 |
| collaboration_attachment | id、source issue 或私有 draft、uploader、storage key、固定版本、delivered_at | 独立于评论的永久交付归属；每次绑定/读取验同单，draft 一次消费，不接受任意公网 URL |
| collaboration_attachment_reference | attachment_id、reference_kind、reference_id、reference_revision、removed_at | comment/command/run 引用固定文件；引用身份唯一，删展示只标记引用；交付归属与其他引用保留，GC 不删已交付文件 |
| 终结凭据记录 | run/group/target/fence/launch、原 installation 验证身份、token hash、proof、receipt id/digest | 独立于当前账号/runtime/task 存活；只服务该 run 收尾，确认后留幂等墓碑；父 run 的证明不能结清其他成员 |
| 协作身份与准入 | collaboration_account_fence、collaboration_runtime_enrollment、collaboration_execution_gate | 版本来源、注册、撤销与冻结按 E06/E07；不以空 epoch/binding 兜底 |

创建先占 source workspace/creator/request_id，原子建立 issue/协作/参与/paused session/事件/回执；draft 重验 uploader/purpose/未绑定。同 key 改正文、同 draft 二次消费拒绝；丢响应按原 scope 找原单，创建/正文提及不执行。

输入同事务保存评论/input_event、逐目标结果、合法 command、通知/回执；事务失败全回滚，目标拒绝可与其他接受结果同存。预览取消仅改本次意图，原键不重新解析/派发，仅有评论权者不获 command。

R52/R55 仅通知当前参与者且排除操作者，同事件去重；@all 仅展开人员，不抑制显式 AI。非参与者不获邀/订阅/通知，@小队不加人。编辑只通知新增有效人员，不执行。

## 迁移与验证边界

M10：旧单会话执行路径直接删除。迁移 571–650 从未进入 main 或任何发布，新增前向迁移删除 legacy_unresolved 的 command/grant/run 及其关联记录，删除 legacy_* 列与约束，不做映射、隔离或兼容读取。已提交的迁移文件不改写；运行过 620–650 的开发/测试库直接执行新迁移。人类流转、参与、交接、通知、附件和账号 epoch 数据保留。

无 FK/REFERENCES/级联或隐式建索引的 inline PRIMARY KEY/UNIQUE。所有索引各用单语句 CREATE [UNIQUE] INDEX CONCURRENTLY 迁移，命名约束只附着合格索引。验证索引后才切换 writer，不能仅删旧唯一索引就开放并发。

索引至少覆盖 source workspace 留存检查、participant(user_id, revoked_at, issue_id)、用户 assignee、target(session_id, kind, resource_id)、lane(session_id, root_agent_id) 唯一、command(lane_id, state, seq)、command(group_id)、group(lane_id) 未结唯一、run(group_id, settled_at)、run(parent_run_id)、delegation(source_comment_id, member_agent_id)、grant(target_id, config_version)、失效资源反查、待投递事件及 notification(recipient_user_id, archived_at, read_at)。唯一冲突产生可重放业务结果，不能忽略错误掩盖重复根执行。

失败重试、attempt/continuation 边界与手工重做按 E03/E04；E08 的兼容/并发/容量为 1/撤权/原生证明测试均待执行。
