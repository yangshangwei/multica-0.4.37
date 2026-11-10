# 技术设计

## 最小变更边界
新增server/internal/handler/delivery_completion_hint.go及对应测试、router注册和内置skill/source-map。必要复用deliveryEvaluationInTx(create=false)、recoverDeliveryEvaluation、currentDeliveryPolicy、deliveryPolicyTx（包含独立policy/evaluation schema及完整索引就绪）/principal/users/credential、LockLifecycleIssue和project SHARE NOWAIT；不得复制观察器。无SQL迁移。status使用issuestatus.Resolve和现有custom status catalog共享锁（事务lock_timeout确保有界）。

## API合同
POST /api/issues/{id}/delivery-evidence/completion-hint。严格最多4096字节JSON，恰好三个必需字段：expected_issue_revision（非负int64）、target_status（非空string，复用Resolve标准化）、evaluation_key（规范非零UUID或显式null）。不接受Idempotency-Key为执行含义；本API不写账本。

200响应：issue_id、issue_revision、target_status（标准化key）、target_category、project_id（nullable）、policy_revision（无项目0）、policy_mode（off/observe）、hint（matching|attention|not_applicable）、reason、checked_at（DB UTC）、enforced:false、usable_for_completion:false；evaluation仅实际授权恢复时包含完整原deliveryEvaluationResponse。

分支：先当前身份/资源锁及expected_issue_revision；独立协作409 unsupported_collaboration_action；目标状态严格Resolve（无效/已归档400 invalid_delivery_completion_status，DB错误非成功）。非done→not_applicable/target_not_done；无项目→not_applicable/no_project；off→not_applicable/policy_off。所有这些不检查key存在、不返回评估。observe+done缺key→attention/evaluation_required；有key→调用原评估恢复路径，pass/current→matching/observation_still_matches，其他→attention并使用applicability.reason。缺/他人key按原404，跨issue同key409，授权拒绝403/404。不允许matching代表workflow许可或完整质量。

先workspace fence、对custom目标取得catalog共享锁（内置状态不取），再当前issue NO KEY UPDATE NOWAIT、current human/user/member/credential、current project SHARE NOWAIT，再读政策；复用原评估恢复可以重复取得同一事务已有锁。custom status同事务Resolve，catalog锁顺序与已有writer一致，基础设施错误不可吞掉。有evaluation时顶层checked_at必须等于evaluation.applicability.checked_at（原观察器的当前检查时间），不能另取更晚时间制造已复核到期的假象；无evaluation时用DB clock。返回前再验证当前凭据并确认commit。Cache-Control:no-store。无自动事务/执行重试；锁冲突409，commit失败不成功。HTTP请求是显式只读POST，无新增持久评估、pack、queue、comment或status write。

## 并发与未来全门禁边界
提示事务保护当前policy读取与artifact授权，结束后不保留许可；客户端完成前仍可能失效，必须由未来每个实际writer在自身状态变更事务内重新授权。项目政策PUT的project UPDATE锁与本次project SHARE互斥；custom状态归档遵循既有catalog锁。不能将本提示当作该未来原子边界。

未来完整门禁必须在单/批量、create-done（含comment-to-child）、move、GitHub/VCS WriteIssueStatus、独立协作验收/自完成/R38管理员路径分别接入；复用lifecycle与审批回执。旧客户端启用强制后也须得到明确非成功且无部分写入，不能绕过。unknown/缺失/过期/撤权不能放行；skip不是off，须有单独不可变审计和同事务一次消费。政策变更应与实际完成共用project锁，评估、policy版本、identity、期限、SHA/digests在写事务重新核对；有意适配服务边界，不能在handler另建执行引擎。原作者与完成人关系及协作责任例外仍需业务决定，未实现。

## 回退/兼容
新增显式endpoint不改既有payload/response。没有前端消费者，不添加虚假的TS schema。功能回退可停止调用或owner/admin追加off；历史留存，已有授权边界保留。后端旧客户端全路径行为仍按原合同；未声称已完成门禁。无表/索引变化，不需新回滚迁移。

## 规划独审收敛
入口使用deliveryPolicyTx，包含完整schema/index readiness，not_applicable也不绕过基础设施拒绝。测试缺失/无效索引拒绝且零写入；有evaluation时checked_at与applicability一致。原loadIssueForUser已经拒绝独立协作POST，无须复制第二套跨账号鉴权。

实施独审发现目标状态NUL可到达PG text编码并误报503，已通过真实handler回归复现。对标准化后的非内置key复用issuestatus.ValidateKey先校验语法，再由Resolve核对目录/归档；语法错误400，不吞数据库错误。所有不消费评估分支补齐当前身份撤权验证。
