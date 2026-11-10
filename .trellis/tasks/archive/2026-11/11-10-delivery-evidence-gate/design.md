# 技术设计

## 边界与合同
新增 POST /api/issues/{id}/delivery-evidence/evaluate。严格有界JSON，固定 delivery-artifact-observe-v1，输入期望issue revision、project ID/revision（无项目显式null）、approval ID、expected source revision、spec digest、artifact digest。字段最终由实现保持一致并写spec。只读观察，不新增持久化表或依赖、不修改done写入。Human JWT/PAT为首批读者；机器身份拒绝且不信任actor headers。复用loadIssueForUser与现有artifact身份锁，使用解析后UUID。

政策要求一个受控artifact-tar-v1/local-synthetic成功回执，完整 canonical spec绑定及规范化receipt校验。policy版本为固定服务端常量。时限保守采用artifact expires_at作为本版本观察窗口；超过窗口hold不抹除历史成功，不将打包准入期限解释为通用制品保质期。

## 当前候选与源码新鲜度
expected_source_revision为选定候选断言。GetIssueLatestLinkedPRHeadSHA为数据库已观察VCS候选；没有已观察head整体unknown；不同head或receipt source_revision不匹配hold。不能把task.context.head_sha快照或客户端声明当实时仓库HEAD。读取并锁定其选择依据，避免并发新PR关联或head更新形成错误pass；若无法稳定加锁，保守unknown，不能假称排除并发。

## 事务与权限
读事务内使用现有NOWAIT workspace/lifecycle/project/authority锁，锁冲突返回409、依赖失败503；提交确定后返回结果。复用artifact锁序并避免等待反向锁。当前reader必须是受控artifact合法human reader；另检查原producer/发起人/runtime owner/审批人的当前成员及调用权限和冻结task generation/runtime/issue。已结束task允许历史评估，但撤权仍影响pass。完整receipt验证必须通过Go canonical serialization，不hash JSONB文本。响应不暴露路径、文件名或凭据；no-store。

## 兼容与回退
新增独立API，旧客户端无需新字段、旧done语义保持；本API响应不被现有UI消费故不引入前端schema。禁用/移除新增路由即可回退，不删除旧审批/回执。未来block必须在普通单/批量/创建done、服务WriteIssueStatus及独立collaboration路径统一合同后实施。
