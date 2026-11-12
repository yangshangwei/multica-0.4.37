# 单智能体服务端续作边界

本会话已由用户再次确认讨论 §64 为当前要求。最新边界取讨论 §64–§66 与[当前单智能体范围](../continuation-single-agent-2026-10-17.md)：先完成 M05 的实际资源 owner consent、M10 和 M06/M07 单目标基础，再在 M09 内验证 M04 的单 lane 停止/恢复。多 lane 部分恢复与目标级 hold 在 M09 之后实施；旧的“整批授权失败则暂停整条通道”不能作为后续多目标规则。

## 删除计划（M10）

- 只通过新的前向迁移删除 `legacy_unresolved` 的 command/grant/run，以及这些旧 run 的专用终结与过程行；不重写 571–650。
- 删除 `legacy_*`、`legacy_unresolved` 列/约束和 session 的旧单 active_run/active_grant/counter 字段，删除对应服务分支、旧投影及查询。
- 当前 v2 输入、目标、执行组、run 的固定 nonce/fence/proof 均保留。人类单据、参与关系、交接、通知、附件和账号代次保持；650 的普通执行拒绝不变。
- 在隔离 schema 中先建立 571 的旧数据、升级到 620 的旧与当前数据并验证保留基线，再执行实际新增迁移，断言仅旧执行消失、当前未知 proof 仍存在。停止仍保留当前未知证明屏障。
- 删除旧数据不可凭 down 还原；down 明确拒绝伪造恢复，历史升级迁移的测试仅验证其自身日期边界。

## 输入与实际执行

- `command_id` 继续表示每条不可变人类输入；新增实际根 agent 通道和待执行批次。开始时冻结整批，取消/作者移除只移除 waiting 的相应输入，已开始批次整体收尾。
- 内容分工只调用 `AppendCollaborationInputInTx`，不自行插入批次或 command；同一事务保存评论、逐目标结果、输入及通知。
- 首段运行只接受直接 agent + 合法 managed MDT + 合格 Linux containment。legacy enrollment 仍可保留授权/等待，但不能启动；macOS/Windows 未证明封闭时拒绝能力。
- run 保留原始根输入作者及批次全部输入作者，promote/claim/permit 重验全部人的参与/账号/lifecycle 代次；不会用任意一位作者冒充整批授权。
- 单次 permit/封存 proof 使用共享 `protocol.CollaborationRunIdentity`，固定 deployment/installation/issue/session/target/channel/batch/group/run/fence/claim_generation/launch_nonce。termination token 只给 supervisor，独立于已撤销业务权限。

## 验证顺序

资源 consent/配置/owner 数据库负例 → M10 保留/删除迁移 → waiting 批次/单条取消/并发锁 → 当前 proof 屏障/单 lane 明确恢复 → managed claim/permit/独立收尾 → fake executable 的端到端停止及下一批。多 lane 部分恢复另按最新 hold 契约验收。所有测试仅使用本任务隔离数据库和显式 fake executable。

本记录是执行边界，不代表运行功能已完成。
