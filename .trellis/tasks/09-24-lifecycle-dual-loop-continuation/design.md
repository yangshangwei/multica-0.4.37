# 技术设计

运行时继续复用 `CreateLifecycleHandoff`，不新增表或外部连接器。请求边界增加结构化窗口和评测 artifact 字段，服务层继续提供 fail-closed 判定。

事故学习分两种输入：`existing_prevention_tasks` 只引用已有 issue 编号；`prevention` 用于创建或幂等复用一个带 owner/priority/acceptance signal 的子 issue。已有任务路径只写源 issue 的证据和审计评论，不生成空的 follow-up。

复用 follow-up 时，解析目标 issue 后要求 `parent_issue_id == source.ID`。已有事故预防任务若不是子 issue，必须通过 `existing_prevention_tasks` 引用，不得伪装成普通 follow-up。所有查询仍带 workspace 条件。

观察窗口使用 RFC3339 起止时间；结束时间不得早于开始时间，窗口必须标记完成。服务契约仍只消费完成布尔值，但 handler 在写回 metadata 时保存完整边界，便于审计和后续 provider 适配。

Agent 评测增加 `artifact_digest`，与 baseline/candidate/skill/MCP 版本共同构成来源。缺少 digest 或任一必需类别时保持 unknown；安全 case 的拒绝仍由既有 validator 判定 hold/pass 语义。

不执行生产动作，不读取凭据，不新增 MCP。回滚点是单文件 handler/service 测试和字段变更，旧客户端省略新字段时按 unknown 处理。
