# M1 持久化委派回执 — 交付记录

在38d07b6ed基线上复用现有trigger_outcomes与派发引擎，新增可选UUID Idempotency-Key的普通评论创建和只读回执恢复。CLI支持 `issue comment add --idempotency-key <uuid>` 与 `issue comment receipt <issue-id> <key>`。同键同请求返回原评论和结果，不再派发；内容冲突409。请求/评论原子持久化，只有确定提交的唯一请求调用现有队列，收尾不确定保留unknown。

逐目标保存原agent/squad UUID、实际task/执行agent/覆盖小队、queued/coalesced/deferred/blocked及原因。目标替换不改历史；暂时离线继续入队，runtime_observation保存实际任务绑定运行时的状态与观察时间，恢复在线或更换绑定后读回仍不变；未绑定/不可用明确拒绝，私有目标不泄露运行时详情。恢复读取需要当前主体与originator权限；新合法任务可追溯同agent原回执，不能借旧凭据或其他人的key扩权。

## 独立验收范围

| 检查 | 普通 | race |
|---|---:|---:|
| Handler（205顶层，含旧评论/能力/小队回归） |342|342|
| 真实JWT/PAT/task认证路由 |20|20|
| 真实迁移/回滚及索引清理合同 |5|5|

所有最终轮次0fail/skip。具名计数含子测试，各轮不累加成独立覆盖。CLI98、runtime/技能合同33、迁移静态合同1、runtime调用扫描1通过；全后端build/vet、sqlc再生幂等、36文件冻结hash、gofmt/diff检查通过。13份最终验证记录见verification-2026-11-10.json。无前端变更，没有全仓/浏览器/真实供应商执行验收声明。

## 审查修复与失败保留

- 后台completion与cancelled-survivor补偿曾可能绕过unknown；共同入队与传播入口现在核对原始已受理target type/UUID及执行者。读取和重试不会接管，自动补偿也不能将原拒绝目标变成授权。
- 原派发允许只针对对应comment的内部permit；显式编辑是新的授权legacy操作，不改原回执。不同评论不能继承permit。
- 实际覆盖小队只作历史事实；不将它误作直接agent提及的未来路由约束。真实red证明旧比较丢失followup，修复后在实际completed后生成正确直接任务。
- reporter读取另一同项目任务时，先锁目标再读取当前project。真实red为项目移出并发提交后仍GET200；修复后读事务期间移动55P03，随后移动成功则GET403。
- 初始unknown切片独立保存，最终写入失败不返回未落库的成功状态；精确任务身份从实际操作返回值取得，不查询最新task推断。
- 空header拒绝且零写入；受限task读取仅放行精确GET路径并检查资源scope。
- broader migration合同发现down清理错误注册（只DROP无CREATE），修复保留up注册；非空回滚与锁冲突均实际验证。
- 所有15次非零运行原样保留。夹具/TestMain编译/sqlc时序及早期dispatched不能Complete的失败不算产品red；CLI零选中首次运行也不算行为证据。成功但修复前的轮次保持历史，不替代最终03。

## 边界与运维

首包仅可选、无附件、普通创建中的显式agent/squad目标。无键创建、编辑、quick-action及历史评论不自动拥有持久幂等；无显式提及的空targets不能证明隐式任务或保证隐式补偿。禁止自动换新key重发。queued/deferred/recorded均不是执行完成、验收或发布授权。

运行时观察是入队后对实际task.RuntimeID的时点采样，不保证连续在线。unknown需人工核查，不能自动重跑。迁移771–775无FK，两个索引各自CONCURRENTLY；回滚须全程停准入/维护且不能删证据来降级。

专属PG容器已停止，连接0，23个数据库与原始日志保留（含6个未运行的保留库）；主工作区已有变更本地集成时逐项保护。真实模型调用0，无push/deploy/外部消息。M1完整项目政策/持久评估/全done入口门禁、M2快照→总预算→调度、M3仍开放；AC43/E5/H3a/知识停止规则及预算不变。
