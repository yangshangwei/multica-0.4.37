# M1 项目政策与持久评估 — 交付记录

在c3bc9abd1基线上，复用已验收的观察器、lifecycle、当前人类身份及受控制品审批/执行回执，新增项目交付政策修订与不可变评估账本。当前只支持off/observe、固定delivery-artifact-observe-v1检查和done观察阶段。政策由当前工作空间owner/admin管理，项目负责人身份本身不赋予修改权。

## 接口与行为

- GET/PUT `/api/projects/{id}/delivery-policy`：默认revision0/off，CAS追加修订；保留负责人、理由和时间，同值设置仍是新的明确操作。
- POST `/api/issues/{id}/delivery-evidence/evaluations`：原观察请求加expected_policy_revision，必需UUID Idempotency-Key；同人类主体同键同请求恢复原评估，不同请求409。
- GET `/api/issues/{id}/delivery-evidence/evaluations/{key}`：在当前身份和原制品权限下恢复原始观察，独立返回current/stale/expired/unknown适用性。新合法凭据可恢复，同键不会刷新时间或把原hold/unknown提升为pass。

恢复先查原键，再区分新准入，因而关闭/更改政策仍能读取有权历史。旧版本不匹配不能提前返回旧pass：先用当前锁定资源版本运行完整原制品权限复核，再标记失效；原执行绑定消失拒绝历史内容。持久化和观察同事务提交，失败或不确定commit不返回成功；实际提交可同键恢复。无额外打包、排队、评论或状态变更。

所有响应usable_for_completion=false，current/pass均不构成完成或发布许可。原stateless evaluate API仍不写账本。off不是skip豁免；本包无skip、提示、UI/CLI或任何完成入口阻断。普通更新、批量更新、创建即完成、GitHub自动完成、独立协作及自定义done类别均是后续全入口门禁范围。

## 冻结后验证

26份产品/合同/测试文件冻结，最终全部零失败/跳过：

| 范围 | 普通 | race | 说明 |
| --- | ---: | ---: | --- |
| Handler |236|236|64顶层；95新政策评估+140旧观察/制品/审批+1删除清单 |
| 真实认证路由 |80|80|28顶层；46新增+34旧观察/制品 |
| 实际迁移与注册合同 |37|37|9顶层；34新增+3全局cleanup注册检查 |

技能合同21、迁移静态合同1、runtime扫描1，共23通过。全后端build/vet、gofmt/diff和sqlc再生幂等通过，26文件零漂移。具名计数包含子测试，普通/race和迭代结果不能累计成独立覆盖。实际路由以真实JWT/password/PAT认证、task凭据受控打包，核对tar内容和SHA、持久行、重建路由恢复、实际到期及撤权；没有浏览器/全仓/真实供应商验收声明。完整索引见verification-2026-11-10.json；独审另见research/independent-review.md/json。

## 保留的失败与修正

6次非零原始运行全部保留。产品/路由编译失败来自sqlc的nullable pgtype.Bool判断；handler首轮另有编译失败，下一轮使用不存在的guest角色夹具；router首轮独立审批人缺少合法调用授权，失败发生在旧审批入口，补齐显式授权后新库通过。它们不算产品行为red→green证明。

基线实际删除清单测试证明旧agent_artifact_execution、comment_delegation_receipt未分类；本包新增retain_audit分类覆盖这两表与新两表，要求workspace_id并保留现有删除行为。独审还纠正迁移测试对更新后ON CONFLICT DO NOTHING的过时23505预期，改核对ErrNoRows与行/索引快照不变。最初迁移34通过发生于生成查询变更前，不代替最终37普通/race记录。

## 留证与运维边界

迁移776–780无FK，三个唯一索引各自CONCURRENTLY；运行时与迁移核对完整索引形状。项目政策与评估随issue/workspace删除留存，公开读取仍拒绝；项目删除后政策GET404，存活的已移出任务可在完整当前权限下恢复stale。非空或并发写入回滚被实际拒绝，多步降级须全程停止准入，不可删除历史来降级。

PR head只是数据库已观察事实；有效窗口沿用原artifact准入期限，不是通用制品有效期。全局短PR表SHARE NOWAIT锁及PAT异步辅助写可能合法409，没有吞吐承诺。恢复目前限原人类评估者；共享审计视图、提示和强制门禁后续单独交付。

专属PG停止前连接0，14个数据库与全部原始日志保留；未借用或清理历史验收环境。真实模型调用0，AC43/E5/H3a/知识预算及停止规则不变，无推送/部署/外部消息。本子任务归档不关闭完整M1、M2/M3或六项业务。

实现提交`4ec66e93c`。子任务验收完成，归档与本地集成记录随后追加；父任务保持in_progress。

## 本地集成完成

实现`4ec66e93c`、归档`3ae2f0495`、交接`53a7a25c3`已快进合入`codex/cross-workspace-collaboration`。主工作区1,048个不重叠已有路径逐字节/权限/存在状态保留；4个重叠spec/父任务/journal文件完成三方合并，旧日志完整保留。26份冻结源码与业务分支工作区一致，详见research/local-integration.json。没有推送或部署。
