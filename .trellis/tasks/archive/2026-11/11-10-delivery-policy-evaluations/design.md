# 技术设计

## 最小边界
新增project_delivery_policy追加版本与delivery_evidence_evaluation不可变观察，复用artifactTx、artifactPrincipal/artifactUsers/artifactCredential、evaluateDeliveryEvidence及lifecycle/project锁。旧POST evaluate和既有执行器不替换、不复制；必要只抽取请求校验以供新入口复用。没有前端消费者，旧响应无需增加字段。

## 政策API
GET/PUT /api/projects/{id}/delivery-policy，human-only且当前workspace/project有权。GET返回最新版本或revision0/off默认。PUT严格有界JSON：expected_policy_revision(nonnegative)、mode(off|observe)、policy_version(delivery-artifact-observe-v1)、stage(done)、reason(去空白后1..1000字符)。政策管理仅当前workspace owner/admin；project lead身份不授予写权。project row UPDATE NOWAIT串行追加，当前revision比较通过才插入revision+1；无op仍是显式新修订以保留操作，超时后GET核查revision，不能盲目更换预期修订。独立policy revision不改project.revision，避免旧项目编辑协议受影响。历史行保留actor/time/reason。无历史默认off不补造版本0行。

## 持久评估API
POST /api/issues/{id}/delivery-evidence/evaluations，必填UUID Idempotency-Key；JSON等于旧观察body再加expected_policy_revision。此入口要求project非空、当前policy revision匹配且mode observe。GET /api/issues/{id}/delivery-evidence/evaluations/{key}只恢复当前验证人类主体在该workspace的原键；同主体跨issue复用key冲突。namespace(workspace_id,evaluator_user_id,request_key)。严格body不接受verdict、producer、skip和authority自报。

原始observation嵌套保存原deliveryEvidenceResponse，不再计算/延长其时间。外围含evaluation_id、request_key、evaluator_user_id、policy_revision、stage、observation、applicability、usable_for_completion:false。applicability为当前检查结果(status: current|stale|expired|unknown, reason, checked_at)，不是授权。初次pass可current，原observation.verdict为unknown时始终不变，applicability可因更优先的版本/政策/期限条件显示stale/expired；原hold不能升级。

新创建在同一artifact事务：当前身份→workspace/issue/project锁→政策共享读取→同键查重/唯一约束→evaluateDeliveryEvidence→插入不可变快照→确定commit后返回。无外部副作用，无自动事务重试，NOWAIT/短lock_timeout冲突409。提交结果不明503；实际落库可由GET/同键恢复，未落库同键可重新计算但不能宣称原请求已成功。

## 恢复及失效
读取先锁当前issue/project并复核人类当前凭据及原evaluator身份；不能先遇旧版本409就返回旧快照，这会跳过原制品授权。使用临时当前issue/project版本构造recheck请求（原approval/SHA/digests不变），调用现有evaluateDeliveryEvidence做全部当前artifact权限检查和当前事实计算，再独立比较原政策、原project/issue版本、有效期和当前观察。授权错误拒绝整个读取；DB/commit故障非成功，锁竞争409。版本/政策off或revision变化→stale；expiry→expired；原unknown或当前unknown→unknown；原pass且新pass、全部原绑定相同→current，仍不是完成许可。原hold保持stale/unknown而非升级。所有GET/replay零评估写入、无Pack/队列。

已删除issue/workspace公开读取拒绝，保留表中历史且不注册到清理删除路径。已删除project的政策GET返回404；若issue仍存在（删除项目将其移出并增加revision），历史评估在完整当前artifact授权后可读stale。无项目的新评估拒绝；移出项目同样可恢复stale。原observation含execution_id而当前执行行消失时，必须拒绝返回旧快照（404/409 retained_binding_missing），不能把观察器早期evidence_missing当成已完成历史权限校验。原本没有execution的unknown可在当前调用者/资源权限通过后恢复unknown；若该approval后来出现则必须经过其完整作用域及权限检查。

## 迁移和回退
776起新增表，无FK/级联或隐式PK索引。唯一索引(workspace,project,revision)、(workspace,evaluator,key)、evaluation id等必要索引逐文件CONCURRENTLY；末尾校验确切形状并登记失败清理。highest down先ACCESS EXCLUSIVE NOWAIT锁表拒绝有任何政策/评估；最终DROP再防护。整个多步降级停止准入，不删历史规避。业务回退可追加off且保留历史；不是危险动作授权。

## 后续门禁
单更新/批量更新/创建done/GitHub/独立协作均未阻断。未来提示→阻断必须覆盖全部writer并在实际状态变更事务复核政策、身份和证据，不能信任本包历史current或pass作为可复用令牌。skip的责任/理由/权限需独立合同。

## 规划独审收敛
- 恢复与新准入分支顺序：当前caller/workspace/issue身份检查及锁→按actor/key找已存行→对比原request digest/issue（冲突409）→完整当前artifact复核→只读返回原观察及当前适用性。只有不存在原行时才执行project非空/当前mode observe/expected_policy_revision等新准入检查。关闭或修改政策不能使合法旧键失去恢复入口。
- 新表独立schema readiness：每次新policy/evaluation事务检查表、唯一索引确切键顺序/唯一性/有效可用性（与迁移末尾同一合同），缺失或错误形状503且零写入；artifactTx只检查旧artifact表，不可误当新表就绪。
- 显式比较原execution/producer/task/generation/spec/artifact/receipt identity与当前观察；即使两次verdict都是pass，绑定改变仍stale。历史unknown不因新证据重算为pass。
