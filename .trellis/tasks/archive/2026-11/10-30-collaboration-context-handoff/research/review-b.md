# B 组独立审查：资源拥有者重新授权与迁移

审查人：Codex 原生子代理 independent_b。审查日期：2026-10-30。
固定代码：`/Volumes/artisan/code/2026/mc-collab-integ`，HEAD `4b0cbfde7a2a03142e70ceb91294ec8d8f1ff074`。
范围：`0ef4c7858`、`4c2140b54` 相对 `a3475f20a` 的最终行为；迁移、SQL/sqlc、配置授权和恢复边界。
操作：源码/断言独立检查，以及一个无需数据库的迁移 lint 测试。未修改产品、迁移或数据库，未调用真实模型。集成工作区检查后仍只有原有未跟踪 `.integ-check.sh`。

## 结论

**需要收尾修改后才能合入。** 已实测一个静态验收阻塞：新迁移数值前缀 745 重复。重新授权的主要权限路径保持资源拥有者身份、当前账户/参与代次、资源绑定、config version、hold 和显式 Resume；本次未发现改变路径中的可证实越权。另有界面说明与可恢复范围不一致，以及最终版本数据库/浏览器验证不足。

这里的“未发现越权”是代码审查结论，不代表最终 HEAD 的完整数据库、并发、旧 token 和 Web/Desktop 验收已经通过。

## B1：新增重复迁移前缀导致现有 lint 失败（合入阻塞）

- 分类：实现/集成偏差；已复现，不依赖模型。
- 文件：`server/migrations/745_collaboration_owner_reconsent.up.sql:7`、`server/migrations/745_drop_user_cloud_waitlist.up.sql:4`。两个 migration 的 up/down 都共用数值前缀 745。
- 仓库现有约束：`server/internal/migrations/migrations_lint_test.go:17` 明确禁止向历史集合增加碰撞，`:79` 的 `TestMigrationNumericPrefixesStayUniqueAfterLegacySet` 在 `:95`–`:97` 对新碰撞报错。
- 真实复现（在上述固定 HEAD 的 `server/`）：

```text
env -u GIT_CONFIG_COUNT go test ./internal/migrations -run '^TestMigrationNumericPrefixesStayUniqueAfterLegacySet$' -count=1 -v

=== RUN   TestMigrationNumericPrefixesStayUniqueAfterLegacySet
    migrations_lint_test.go:96: migration prefix 745 is reused by [745_collaboration_owner_reconsent 745_drop_user_cloud_waitlist]; use the next unique prefix instead
--- FAIL: TestMigrationNumericPrefixesStayUniqueAfterLegacySet (0.00s)
FAIL
FAIL    github.com/multica-ai/multica/server/internal/migrations    0.477s
FAIL
exit code: 1
```

- 准确影响：静态验收失败，迁移命名不符合当前规则。**不是迁移器用数字版本去重而跳过迁移**：`server/internal/migrations/migrations.go:60`–`:70` 按完整文件名排序，`:94`–`:99` 的 `ExtractVersion` 只剥除 `.up.sql`/`.down.sql`；`:79`–`:91` 的 AllVersions 收集两个完整 basename。
- 最小处理：如果 reconsent 迁移尚未正式发布，给它分配当前下一个空闲数值前缀，同步 `server/internal/migrations/collaboration_owner_reconsent_test.go:12` 的常量和相关规范中的编号；不要把 745 加进历史豁免。若任何正式环境已记账，先核对完整版本名与实际列/函数，再设计幂等前向修复，不能仅重命名已发布迁移。
- 验收：上述单测通过；实际空库升级、已有库升级、重复执行、触发器与新增列检查另做。修前日志不覆盖此验收。

## B2：恢复提示仍把部分 Switch-only 场景说成可直接重新授权

- 分类：设计范围已明确，但界面表述与实现有偏差；无需扩权即可修复。
- 代码边界：`server/internal/service/collaboration_authority.go:233`–`:239` 只允许记录过 boundary 且 boundary 未变化的目标直接重新授权。迁移 `745_collaboration_owner_reconsent.up.sql:10`–`:14` 特意保留所有旧目标 NULL boundary。`collaboration_reconsent_test.go:284`–`:298` 明确断言这类目标连 instructions 变动也必须走 Switch。
- 界面：`packages/views/locales/zh-Hans/collaboration.json:11`（英文对应 `:11`）写“配置已变更，资源拥有者可重新确认……若执行方式或工具权限有所扩大……配置切换”；`packages/views/collaboration/labels.ts:60` 对所有 host `config_invalid` 使用该提示。该选择没有检查是否存在 boundary。
- 可观察后果：升级前已登记的目标，或边界变更但并非权限扩大的目标（例如 `service_tier`、runtime visibility），会看到“可重新确认”，而 `Resources` 实际不会给 `can_consent`（`server/internal/service/collaboration_resources_view.go:99`–`:145`）；前端授权按钮只在 `can_consent===true` 时出现（`packages/views/collaboration/resource-panel.tsx:69`）。请求 API 也会以 `target_configuration_changed` 拒绝（authority `:577`–`:579`）。
- 推荐收尾：提示直接限定为“已记录边界且仅修改说明/模型/思考级别时可由资源拥有者重新确认；其他变化或旧目标需发起人/处理人 Switch”。更好的按目标动作提示可后续复用服务端已有 `can_consent` 和不可用原因，避免推断权限。不要为了让提示成立而自动给旧目标填当前 boundary 或放开 Switch-only 变更。
- 验收：覆盖“新目标三类允许变更”、“旧 NULL boundary 目标”、“真实边界变化”三种界面说明/按钮/服务端结果一致；Web/Desktop 各走一次真实恢复路径。

## 已检查的正向行为

| 检查点 | 最终 HEAD 依据 | 结论与限制 |
| --- | --- | --- |
| 重新授权只收实际拥有者 | `server/internal/service/collaboration_authority.go:537`–`:558` | 必须有 SubmitCommand，版本匹配、目标未撤回，选中资源的 OwnerUserID 必须等于真实 actor；未新增发起人/处理人门槛，普通完整参与者中的资源拥有者可走此路径。 |
| 检查在写入前并且持锁 | authority `:560`–`:587` | 锁目标 grants、资源、绑定，重算当前配置并校验当前 installation/binding；拒绝发生在刷新前。锁冲突仍应回滚整事务。 |
| 窄范围变更可证明 | migration up `:28`–`:45`；authority `:198`–`:229` | instructions/model/thinking 仅推进 config epoch；其他受管执行列或子 writer 显式推进也推进 boundary。boundary digest 仍覆盖完整 roster、成员、runtime epoch、membership、invocation targets。 |
| 未登记历史边界保持拒绝 | authority `:233`–`:239`；test `:284` | NULL boundary 不猜测历史，安全地保留 Switch-only；这是设计取舍，交付声明必须说明。 |
| 不复活旧授权 | authority `:588`–`:605`；`server/pkg/db/queries/collaboration_authority.sql:25`–`:32` | 撤掉目标旧 grants，守 expected config 和 boundary 后只推进 config_version；不改 target_epoch/roster/boundary；旧身份不变成新 grant。 |
| 小队每个资源须重新确认 | authority `:622`–`:628`；test `:226`–`:281` | 第一个拥有者只确认自己的资源；其余资源仍 grant_required。测试以同一 owner 拥有 leader/member 演示每个资源，但未直接证明两个独立 owner 的全新 reconsent 流程。 |
| 重新授权不执行、不 Resume | authority `:632`–`:646`；test `:103`–`:156` | 保持 config_invalid 并推进 hold version；poll 无 run、waiting input 保留；显式 Resume 后新 claim 带新 invocation。 |
| 旧 run 业务 token 无法借新授权继续 | `server/internal/service/collaboration_runtime.go:336`–`:371` | hold 即拒绝；即使之后恢复，target/run config version 和旧 grant epoch 仍需相等；因此新 consent 不使旧 run 通过当前授权。这里是代码推导，需真实 token 回归。 |
| 异步 invalidator 不撤新授权、不漏旧组 | `server/internal/service/collaboration_config_invalidation.go:198`–`:236`、`:272`–`:275`；test `:304`–`:330` | 当前 fingerprint/version 相符的 grants 保留，旧组仍 stop；单测只用 synthetic service 路径，不证明实际 Provider 已终止。 |
| Resume 屏障保留 | `server/internal/service/collaboration_control.go:390`–`:410`、`:416`–`:482` | 当前授权/资源校验和完整 group/run 账本检查仍执行；过时 hold 的通用回归在 `collaboration_hold_test.go:174`–`:204`。 |
| 新 Switch 记录 boundary | `server/internal/service/collaboration_switch.go:398`–`:405`；test `:215`–`:220` | committed proposal fingerprint 必须已等于当前快照，随后保存其 boundary；后续窄变更可重新授权。 |
| schema 与生成字段静态对应 | migration up `:7`、`:13`；`server/pkg/db/generated/models.go:161`、`:1152`；generated `collaboration_authority.sql.go:306`–`:355` | 新增 nullable pgtype.Int8/Text 与迁移一致，Refresh 参数/WHERE/Scan 与 SQL 对应；尚未运行 sqlc 重生成后逐字节比较，不能宣称 generated 完全无漂移。 |

## 设计取舍与 PRD 的关系

1. 旧 PRD R13（`.trellis/tasks/10-10-cross-workspace-collaboration/prd.md:37`）给出的原则是执行模式/托管工具权限扩大导致授权失效；本次实现用三字段 allowlist 作保守可证明范围，并没有实现任意语义上的“权限不扩大”自动分类。runtime 及 service_tier 等仍 Switch-only 应作为首轮范围限制写出。
2. 这次编辑仍会暂停受影响目标、停止旧组并撤旧授权。改进的是恢复人和恢复途径，不是让说明编辑对执行完全无影响。
3. config_version 改变会换 Provider session generation，因而后续新会话的连续性仍依赖共享上下文，不能宣传为无损原会话恢复（`server/internal/service/collaboration_runtime_capabilities.go` 的 session_generation 和 `server/internal/daemon/collaborationrun/host_session.go` 的会话键）。
4. 模型能否理解新说明、续接旧发现，不由授权状态机保证，必须用正式实验另证。

## 断言与验证缺口

本次新执行仅有 B1 的迁移 lint；以下列的是已读断言及下一轮应执行的验证，不冒充通过记录。

- `TestCollaborationConfigurationReconsentRecoversNonWideningAgentEdits`：覆盖三字段和改回原样、hold/config/grant 精确变化、同 request replay、当前 consent 重复、late invalidator、waiting input、stale target、新 claim invocation。
- `TestCollaborationConfigurationReconsentRefusesWideningAndIdentityEdits`：覆盖 permission_mode、custom_args、skill binding、runtime visibility 拒绝及 Switch 恢复。其他边界列以 trigger 总体代码为主，未逐列跑矩阵。
- `TestCollaborationConfigurationReconsentRecoversSquadMemberInstructions`：同 owner 的两个资源重签，不等于两个 owner 与真实发起人/当前处理人分离的 UI 路径。
- `TestCollaborationConfigurationReconsentKeepsSwitchOnlyForTargetsWithoutBoundary`：验证保守升级限制。
- `TestCollaborationConfigurationReconsentBeforeConsumerStopsOldConfigurationGroup`：确认 stop intent，不证明 managed process 已结束，也不直接用旧 business token 调用 memory/comment/outcome。
- `TestCollaborationConfigurationBoundaryEpochAdvancesOnlyThroughItsWriter`：拒绝直接 boundary 赋值、窄变更不推进 boundary、显式子 writer 与 service_tier 推进。
- `TestCollaborationOwnerReconsentMigrationSeparatesBoundaryEpoch`（`server/internal/migrations/collaboration_owner_reconsent_test.go:36`）：只在最小临时表上应用单个迁移并手工安装 trigger，测试 up/down/reapply；不是整库生产迁移器升级证明，也不验证两个 745 的流水线 lint。

最终收尾至少要在唯一 final HEAD 上完成：迁移编号 lint、空库/旧库通过实际 runner 升级、实际 trigger/列/ledger 检查、sqlc v1.31.1 重生成无漂移、完整 reconsent 与 invalidation/hold/authority/switch 定向套件、旧 token 三类写拒绝、两个不同拥有者的小队重签、非发起人非当前处理人的资源拥有者恢复，以及 Web/Desktop 提示与动作一致。

## 建议最小合入门槛

- [ ] B1 以独立小提交解决，保留旧迁移原始证据。
- [ ] B2 纠正文案或提供按目标服务端决定的恢复路径；明确历史目标限制。
- [ ] final HEAD 的数据库/生成产物/授权与恢复验证没有 skip 伪通过；日志记录 HEAD、命令、环境、测试名、退出码。
- [ ] 原有 account/participant/owner/binding 校验、只通过显式 Resume 恢复、旧 run 收尾与 unknown 屏障不因测试修复而放宽。
