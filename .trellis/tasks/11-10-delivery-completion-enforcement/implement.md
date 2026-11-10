# 执行计划

## 当前合同与门禁

D1–D5 已由用户逐项确认，D6/D7/技术明细已按用户明确的推荐执行委托收敛。PRD 完成一次去重收敛，研究绑定真实源码与兼容消费者。先由独立 Trellis check 审查合同，再 task start；不再申请已有流程许可。

## 顺序

1. [x] 恢复两个分支/独立 worktree、前序实现/归档证据、主目录保护与当前源码。
2. [x] D1–D7 与 skip 明细、普通子任务协作证据适配、逐项 batch/旧客户端合同收敛。
3. [x] 独立合同审查与 context validate；确认所有产品范围与公开版本/权限限制。
4. [x] P0：专属 PG 与独立 baseline handler/router DB，先运行既有真实制品/观察/评估/提示回归。然后唯一核心下沉至 service，原 HTTP 适配器保持契约，normal/race 与 build/vet/格式验证。
5. [ ] P1：冻结 typed issuer/consumer/grant/Prepare/Finish 接口；新增授权/消费/撤销/迁移事实、schema readiness 与留证迁移，真实认证/实际 tar 证明一次消费和原恢复；仍禁止生产启用 enforce。
6. [ ] P2：普通 update/move/batch、CreateInTx/comment-child、项目调整/删除与 core/views 部分结果恢复；先记录真实失败测试，再实现并逐包验证。
7. [ ] P3：真实 VCS 消费者与信任修订，协作 exact ordinary-child/source/submission/self/R38 的原事务接入与正负实际回执。不得伪造普通 task 或使用另一个 observer。
8. [ ] P4：仅在全入口闭合后开放 enforce、评估/hint 模式兼容、内置技能/source map；全套 normal/race 各包新库，实际 tar/SHA、真实认证/状态/回执、竞态、rollback/unknown、迁移及客户端验证。
9. [ ] 冻结每个受影响文件、具名测试库存和全部原始日志；独立 Trellis check，主会话最终 build/vet/sqlc/format/static 及适用 lint/typecheck/测试。
10. [ ] 主会话更新 spec、父子状态、六业务计划和 journal；仅本包 Lore 提交，独立检查 staged diff 成功后再提交；安全本地集成并复核主目录旧路径 byte/mode/existence。
11. [ ] 仅将已独立验收的实现包记为交付；P4 前不得归档 enforcement。M1 与六业务保持开放，v1 skip 不可用及真实模型停止规则明确保留。停止仅本包资源并保留数据库/日志。

## 文件所有权与并发

- 主会话：PRD/design/implement、decision register/context、spec、任务状态、验收环境/日志库存、路由最终集成、git/journal/主目录保护。
- P0 core implement：service/artifact_authority.go 与 service/delivery_evidence.go 等唯一核心；handler/agent_artifact.go、delivery_evidence.go、delivery_evaluation.go、delivery_completion_hint.go、delivery_policy.go 的调用适配（policy 模式/schema 不变）。不得改 writer/模式/schema。具体导出接口完成后先报告给主会话。
- P1 ledger implement：新 service 授权/消费文件、handler 新授权端点与新 SQL/迁移/注册/generated；不得覆盖 P0 文件或普通/协作 writer。共享类型由主会话协调唯一所有者。
- P2 ordinary implement：handler/issue.go、issue_move.go、source_context.go、project.go、service/issue.go 及对应测试；集中处理 batch 输出。
- P2 client implement：packages/core/types/api.ts、api/client.ts 与相邻 schema、issues/mutations.ts；views batch toolbar/surface/run-confirm 及有意义测试；无需新验收 UI。
- P3 collaboration implement：service/collaboration_lifecycle.go、workflow/delivery/edit/source_fields/admin_collaboration.go、相应 handler/decoder 与测试。
- P3 VCS implement：service/issue_status_write.go、handler/github.go、vcs_webhook.go 与信任版本 SQL/测试；共享 SQL/迁移交 ledger 所有者统一生成。
- Trellis check 独立审查期间暂停冲突写入；不因 slot 限制同时让两人改相同文件。最多三条子代理并行，不递归派发。

## 验收资源和失败记录

每个 DB 测试包、baseline/final 与 normal/race 各新库；只用本次新专属 PG，不启动/借用/清理旧环境。账户/临时执行状态置于 Git 外，独立 caches，禁止 ambient agent CLI；编译使用 go build 或 test -c，不能用空过滤运行 TestMain。每次命令记录实际 exit、原始输出、源码/工具/DB 身份与具名库存，失败不可覆盖。

单次回归集合参考前序冻结记录但重新记录本轮实际执行，normal/race 不相加。P0 行为保持回归不当作新 gate 验收。真实供应商调用为 0，不推送、部署或发送外部消息。
