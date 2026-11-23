# 外部配置变更的持久失效收敛

## 行为差距与边界

660 已用 `collaboration_config_epoch` 将 agent、runtime、squad 的执行配置与名称、头像、心跳分开。准入读取的配置 fingerprint 包含这些 epoch，能立即拒绝旧授权；目前配置 writer 没有留下可靠的后续 hold / 停组工作。成员与 invoke 子表更改也没有完整的 epoch 收敛接线。

本切片复用资源 epoch 和既有资源 / 账号失效模式：writer 同事务记录窄失效意图，后台每次处理一个来源，复用来源 NOWAIT 锁、`stopInvalidatedWork`、`stopGroup` 与持久证明。只撤旧配置的 grants，设置其目标 `config_invalid`，不暂停通道、不改原 run / proof 身份。已成功重新配置并授权的新 fingerprint 不因旧意图被撤销。

实际入口包括 `UpdateAgent`、`UpdateAgentEnv`、`ArchiveAgent` / `RestoreAgent`、`UpdateSquad`、小队成员添加 / 移除 / 角色更改、`DeleteSquad`、runtime 注册 / visibility、`workspace_revoke` 归档及 invoke 清理。SQL trigger 覆盖这些单条、批量及借用事务的 writer；不逐个复制来源扇出逻辑。仅 `UpdateAgent` 的 row / nullable clear / invoke 列表须收成一个事务，避免中间权限集合被授权；原 no-op invoke 列表保持原行身份。

原资源 teardown 仍走既有原子失效，不延后清理至此消费者。消费者始终运行，不受创建 / 执行开关限制，取消服务上下文即停止。无新增依赖或通用失效框架。

## 主会话迁移提案

迁移编号由主会话在 FIFO 迁移之后分配。下列为需要的最终形状；已提交的 660 不改写。

```sql
CREATE TABLE collaboration_config_invalidation (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    resource_kind text NOT NULL CHECK (resource_kind IN ('agent','runtime','squad')),
    resource_id uuid NOT NULL,
    config_epoch bigint NOT NULL CHECK (config_epoch > 0),
    reason text NOT NULL CHECK (reason IN ('configuration_changed','resource_deleted')),
    last_issue_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_attempted_at timestamptz,
    reconciled_at timestamptz
);
```

三个索引各单独 `CONCURRENTLY` 单语句迁移，并登记现有恢复注册表：

```sql
CREATE UNIQUE INDEX CONCURRENTLY idx_collaboration_config_invalidation_id
    ON collaboration_config_invalidation(id);
CREATE INDEX CONCURRENTLY idx_collaboration_config_invalidation_resource
    ON collaboration_config_invalidation(resource_kind,resource_id,config_epoch,id)
    WHERE reconciled_at IS NULL;
CREATE INDEX CONCURRENTLY idx_collaboration_config_invalidation_poll
    ON collaboration_config_invalidation((COALESCE(last_attempted_at,created_at)),id)
    WHERE reconciled_at IS NULL;
```

无需外键、级联或隐式索引。`id` 是稳定意图身份；不使用时间作为执行或幂等身份。每次 epoch 变化追加一条，consumer 保留已完成墓碑。删除追加 `OLD.collaboration_config_epoch + 1` 的 `resource_deleted` 意图，删除后仍通过保留的 target / grant / group / run 找到来源。

替换 `collaboration_advance_resource_config()`：保留 660 的字段判别；runtime 的判别补 `visibility`（private/public 改变执行权限）；允许内部 child-table hook 显式请求 `OLD.epoch + 1`，其他 epoch 改写拒绝。最终 epoch 为 old + 1 当且仅当已有字段变化或明确请求单次推进，其他写保持 old。新增 AFTER UPDATE hook：仅新旧 epoch 不同时 INSERT 意图。三个资源表 AFTER DELETE hook 写删除意图。纯 name/avatar/status/capacity/heartbeat 及 no-op 更新无意图。

新增 child hook：

- `agent_invocation_target` INSERT / DELETE / 有变化 UPDATE：涉及的 old/new agent UUID 去重排序，以 `FOR UPDATE NOWAIT` 锁 parent，然后 `UPDATE agent SET collaboration_config_epoch=collaboration_config_epoch+1`。已有查询先取得 parent 锁；反向手写 / 删除路径遇到冲突立即回滚，不持子表等 parent。与当前 fingerprint 一致，invoke row 身份 / created-by 变化也算变化。
- `squad_member` INSERT / DELETE / 有变化 UPDATE：只对 old/new `member_type='agent'` 的 squad UUID 去重排序，锁并推进 parent epoch；人类成员作为元数据不推进。agent 角色变化计入执行配置；完全相同 UPDATE 不推进。
- 子表 parent 已删除时无 parent epoch 可写：资源自身 DELETE hook 已留下意图，不能尝试复建 parent。
- 经主会话确认纳入 `agent_skill` 绑定与 enabled：`TaskService` 的 `ListAgentSkills` / `ListAgentSkillsByIDs` 确实将该集合用于执行指令和 bundle 授权。复用 agent parent hook，并把四个既有 junction 写查询放在 parent 先锁的 CTE 下。skill 自身内容 / 发布版本的跨资源传播另需专项确认，不在此处宣称完整覆盖。

回滚需要按现有保护方式锁定意图表，存在保留数据就拒绝；空库维护时删除新增 hooks/table 并恢复 660 的函数。升级已有库时应为仍有当前 target / 未结执行的 agent/runtime/squad 生成当前 epoch 的启动扫描意图，避免仅处理升级之后的新变化而遗漏此前已漂移的配置；该扫描比较真实 fingerprint，不盲撤当前合法授权。

## sqlc 查询契约

文件建议 `server/pkg/db/queries/collaboration_config_invalidation.sql`，主会话生成。

1. `LockNextCollaborationConfigInvalidation(excluded_intent_ids uuid[]) :one`：未 reconciled、非 excluded；同 resource_kind/resource_id 有较小 `(config_epoch,id)` 未完成者不能越过。按 `COALESCE(last_attempted_at,created_at),id` 取首个 `FOR UPDATE SKIP LOCKED`。
2. `RecordCollaborationConfigInvalidationAttempt(intent_id uuid) :exec`：独立短事务只更新时间，先 `FOR UPDATE SKIP LOCKED`；不改游标。用于失败 / 未知 commit 后公平调度。
3. `NextCollaborationConfigInvalidationIssue(resource_kind text, resource_id uuid, last_issue_id uuid?) :one`：从下方稳定关系 UNION 来源，LEFT JOIN issue 保留损坏来源；按 issue UUID 取游标后的首个。
   - 当前未 revoked target：agent / runtime 在 target.roster 对应字段；squad 是 target.kind / resource_id。
   - 保留 grant：agent_id / runtime_id；squad 通过 grant.target_id 的 typed target。
   - 未结 group：agent / runtime 在 group.roster；squad 包含 root target 或同 batch 的输入 target（混合批次需覆盖非 root）；可经新冻结 target 集合代替 batch 关联。
   - 未结 run：agent_id / runtime_id；squad 经 group / 输入的固定 typed target。无 grant 的 run 仍须发现。
4. `AdvanceCollaborationConfigInvalidation(intent_id uuid, issue_id uuid) :execrows`：仅未完成且旧 cursor 更小，更新时间；期望恰好 1 行。与 hold / grant / stop / event 同事务。
5. `CompleteCollaborationConfigInvalidation(intent_id uuid) :execrows`：置 reconciled_at、last_attempted_at；期望恰好 1 行。

来源锁完全复用 `lockCollaborationResourceSource`，先将 next row 适配成现有 `ListCollaborationResourceIssuesRow`。配置判断前用现有 `lockCollaborationResources` 获取完整 roster 的资源 / 成员 / invoke / namespace NOWAIT 锁，然后调用 `collaborationConfigurationSnapshot`。不另开 pool 授权查询。

## 初始实现与验证安排

生产文件预计 `collaboration_config_invalidation.go`、必要的 `collaboration_invalidation.go` 小幅复用、`handler/agent.go` 与 `agent_permission.go` 的局部事务修复，`handler/handler.go` / `cmd/server/main.go` 按现有 sweep 生命周期接线。测试真实驱动 SQL writer 与 HTTP agent/squad writer，观察 epoch/intent 原子、立即拒新授权、后台 hold/撤 grant/停组、原证明、无关目标、no-op 显示、更晚的新 consent 保留、子表 / 来源锁竞争、失败公平性及重启接续。主会话负责迁移与生成。

隔离 PostgreSQL 位于主会话提供的 configuration DB；Go 使用 go1.26.6 和已有 agent CLI guard。尚未运行新增回归，不把此设计记成已完成。
