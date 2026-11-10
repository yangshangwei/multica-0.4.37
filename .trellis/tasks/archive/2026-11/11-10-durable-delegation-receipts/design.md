# 技术设计

## 复用及事务边界
在普通CreateComment完成现有身份、任务能力和payload校验后接入可选幂等路径。冻结请求digest、发起agent/user和issue作用域；短事务插入request claim与评论（含受限能力的既有不派发语义），同键唯一约束只允许一个胜者。不要复制含mutex的TaskService，也不要把Composio overlay、广播和唤醒纳入长事务。

唯一确定提交者沿现有computeCommentAgentTriggers/enqueueCommentAgentTriggers执行。初始持久状态unknown，最终写入真实逐目标回执；崩溃/commit歧义/收尾失败留unknown，无自动重试。部分结果只可基于已持久事实，不用后续最新任务查询推测。评论ID在claim事务冻结；原始目标从规范化mention固定，不因重放重新解析当前roster。

## API与身份
POST /api/issues/{id}/comments可选Idempotency-Key；无键路径原样。GET /api/issues/{id}/delegation-receipts/{key}读取原回执，Cache-Control:no-store。响应增量delegation_receipt含key/state/comment_id/targets；trigger_outcomes保留即时契约并增量task_id/executing_agent_id等字段。最终字段由实现与router/CLI双方同步确认。恢复不得绕过当前认证与资源权限；同一agent重启的新task可读取但不能把其他人的key当自己提交。请求digest覆盖所有影响路由/保存的字段，规范化确定且严格键解析。

## 真实任务关联
fresh enqueue传递TaskService返回task.ID；coalesced使用原子UPDATE RETURNING task.ID；deferred必须来自持久登记成功的具体任务ID，不能仅HasActiveTask布尔推测。不得按latest查询补造ID。多目标共享leader各保存原target UUID与同一真实task。成员替换不能在replay产生新派发。

## 迁移、兼容、回退
无FK/级联，索引独立CONCURRENTLY及失败清理/形状检查沿用已有迁移模式。保留历史回执；rollback不可删除非空证据表。旧客户端无键/编辑不变；没有前端流程或新依赖。新增CLI显式参数与只读恢复，默认不生成新的重试循环。未知是安全保留态，不是失败即允许再发。

## 实现前收敛补充
幂等namespace包含原始originator，避免同agent替另一人执行时读取私人历史。source task只冻结留证，不参与重启后的相同agent身份匹配。首包带键创建拒绝attachment_ids（400且零写入），避免现有事务外附件绑定和临时签名URL被错误当成不可变回执；无键附件行为保持。GET仅有权原主体读取，当前成员/任务能力/原originator权限仍需复核。

逐目标范围沿用trigger_outcomes，仅显式agent/squad mention。无显式提及的幂等评论仍保留保存幂等性，不以空targets冒称隐式路由任务的持久身份关联。

## 用户确认的离线语义
2026-11-10本会话用户选择：保留暂时离线时入队等待，持久记录当时离线；验证未绑定/不可用拒绝。可用性必须源自受理期间实际观察，记录时间与状态，不能在GET或重试时根据当前状态重算。不能向无权私有目标泄露runtime细节。
