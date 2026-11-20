# PAY-241 通知与运行恢复推进计划（2026-11-19）

> 本会话使用原生 `trellis-implement` / `trellis-check`，逐项实施和检查；继续现有 S3，不创建新任务。

**Goal:** 补齐普通订阅的真实交付和同评论事件去重，再推进受控运行通知与撤权，按实际证据更新 PAY-241。

**Architecture:** 沿用普通订阅、inbox、协作通知和原运行账本。普通评论通知用既有 inbox 主键约束保留一个持久化交付身份；协作运行仍沿用原 claim/permit/outcome/proof，不增加恢复系统。

**Tech Stack:** Go、PostgreSQL/sqlc、Next.js、共享 Core/Views、Electron、Playwright；不新增依赖或迁移。

## 当前进度与验收口径

| 阶段 | 当前证据 | 下一门槛 |
| --- | --- | --- |
| S1 | 已验收、提交、归档 | 保留原报告 |
| S2 | R33 完整代码和合成 A01–A12 独立接受，原四旅程 4/4、零重试 | 核对未提交改动归属，完成独立提交/归档 |
| S3 | 字段并发 R06、55 评论补读、Extras-A R05、阻塞 R08、普通评论去重/订阅 R05 和 Extras-B R03 已分别验收；TC-21/22 本轮通过 | Agent 输出附件、普通 UI 版本竞争/明确 R04 范围处置、R01–R09 最终来源检查 |
| S4 | 接入研究与清单已准备 | 真实支付/履约沙箱、仓库/CI、非生产部署/回滚、指标和审核人 I02–I09 |
| S5 | r02 工具演练已检查 | 最终候选上 100 用户 × 100 事件、100 次回写及业务签字 |

父矩阵起点：**4 PASS / 13 PARTIAL / 7 NOT_RUN**，通过 TC-05、TC-08、TC-15、TC-23。TC-16 的普通客户端可选版本边界和 TC-22 的普通订阅缺口继续保留。S4 真实输入及最终签字不能由受控运行替代。

## 本轮执行检查点

- 普通评论原事件去重已独立接受并本地提交 `8b1f3dcf6`。普通/竞态各 35 顶层、57 命名通过，原 RED 与日志观察器超时失败保留。
- 普通订阅 R05 已在新专库完整执行并[独立接受](../../.trellis/tasks/11-13-pay-241-s3-recovery/research/ordinary-subscription-r05-independent-check-20261119.md)：Web/Desktop **2 PASS / 0 FAIL / retry 0**，包含每端两条真实评论、原通知已读/锚点、物理重连/重载及退订后不再交付，阻挡项 0。
- Extras-B R03 已在另一新专库完整执行并[独立接受](../../.trellis/tasks/11-13-pay-241-s3-recovery/research/controlled-runtime-r03-independent-check-20261119.md)：**4 PASS / 10 子场景 PASS / retry 0**，实际 10 个受控 Provider turn、12 条原人工输入、0 个恢复启动；失败→新成功→Stop、原回执恢复和原 business token 撤权均核对，阻挡项 0。
- 两个新运行专库的 API 已正常停止、连接归零；本会话启动的 Web/renderer 也已正常停止。所有原失败轮次、运行账本和历史行保留。

TC-21/22 已按独立检查的定义场景推荐更新为 PASS；当前父矩阵为 **6 PASS / 11 PARTIAL / 7 NOT_RUN**。TC-14/20 只补充已覆盖证据，历史列和其他行保持原记录。随后依据 [R01–R09 缺口映射](../../.trellis/tasks/11-13-pay-241-s3-recovery/research/s3-remaining-acceptance-map-20261119.md)推进下一切片，完整 S3 仍保持进行中。

## 1. 普通评论事件去重：先复现，后最小修复

**Owner:** 唯一服务端 Trellis 实施代理；主会话分配数据库与执行命令，独立检查代理复核。

**Files:**

- 新增 `server/cmd/server/notification_comment_replay_test.go`。
- 修改 `server/cmd/server/notification_listeners.go`。
- 修改 `server/pkg/db/queries/inbox.sql`，重新生成 `server/pkg/db/generated/inbox.sql.go`。
- 主会话在对应 server spec 中记录已经验证的持久交付合同。

**Source:** 已接受的 R08 后端 `source-block-work-r05/server` 的新私有副本。修改前核对三个已有文件与 ROOT 的字节与模式；原候选、失败和他任务 WIP 保留。

1. 用 `testutil` 创建真实评论和订阅者，通过实际通知 listener 两次投递同一个不可变 `comment:created`。先在新专库运行 RED，核对重复 inbox 行和重复 `inbox:new`，保留原件。
2. 覆盖并发投递、新 bus 重放、read/archive 保持、既有随机 ID 记录、不同评论/收件人，以及作者、退订、Agent、非订阅者和静音控制。以原评论 UUID 和完整交付元组断言，界面合并行数不能代替数据库计数。
3. RED 明确复现后，仅为有效非零评论 UUID 派生域隔离的稳定 inbox ID。新增专用查询使用 `ON CONFLICT (id) DO NOTHING RETURNING *`，插入前保留匹配同原评论/收件人的旧记录；不更新、删除或复活已读/已归档通知。
4. `pgx.ErrNoRows` 表示本次没有新交付，跳过 `inbox:new`。其他通知保留既有随机 UUIDv7 writer。保留普通通知按当前订阅者投递的语义，不引入协作结果的冻结收件人规则。
5. normal、race 分别使用新专库完成新回归和相关旧通知/订阅回归，再检查 gofmt、编译、vet、sqlc 差异与独立审查。

```text
go test -json -count=1 -run '^TestNotificationCommentReplay' ./cmd/server
go test -json -race -count=1 -run '^TestNotificationCommentReplay' ./cmd/server
go vet ./cmd/server
```

上述命令只能通过已核对实际库名、源码、私有配置和工具的执行器运行。旧/new server 混合并发写入、历史重复行清理、其他泛化事件去重不在此增量通过口径中。

## 2. Web/Desktop 普通订阅的真实交付

当前结果：第1节已独立接受并提交。普通订阅 R01–R04 各保留 **0 PASS / 2 FAIL**：依次遇到运行 baseURL/标记转义、通知文案、多个 Send 控件和输入后 placeholder 消失导致的定位失败。R05 将输入与 Send 绑定到原编辑器的实际提交区域，在新专库单轮完成双端 **2/2 PASS、零重试**，已独立接受；七组 × 十七关系的快照、原收件人、两次评论、重连与清理均核对，阻挡项 0。

**Owner:** 独立 E2E Trellis 实施代理；只写新 spec/fixture，主会话负责运行环境。

**Files:** `e2e/collaboration-recovery-subscriptions.spec.ts` 和 `e2e/fixtures/collaboration-recovery-subscriptions.ts`；复用既有 recovery/native、认证、`TestApiClient` 和 SQL 核账工具。

1. 每端建立真实普通子任务和独立源工作区成员，通过实际 Subscribe/Unsubscribe 控件发请求，核对 `issue_subscriber` 的持久人工订阅与退订字段。
2. 另一真实账号发送无提及评论；核对原评论 UUID、所有 inbox 收件人/ID、真实通知按钮及评论锚点。作者、非订阅者和退订者作为明确控制。
3. 实际重连/重载后，原通知 ID 和已读状态保持；退订后不同评论不再交付。若使用 opt-in `Idempotency-Key`，单独标为 API 原请求恢复，不能替代第 1 节的原事件重放。
4. 先完成目标 TypeScript、ESLint 与 `--list`；服务端去重被接受后，在新浏览器专库、生产 Web 和匹配 Electron 单轮运行双端场景，workers=1、retries=0。
5. 独立核对全部持久记录、真实客户端来源、原始报告及仅自有资源的停止/连接归零；通过后仅更新覆盖到的 TC-22 子断言。

## 3. Extras-B 受控运行通知与撤权

**Files:** 沿用 `e2e/collaboration-recovery-runtime.spec.ts`、`e2e/fixtures/collaboration-recovery-runtime.ts` 两个已封存文件。仅在实际失败确证后修改其所属实现。

实际 R01 保留 4 个失败顶层、2 个失败子场景/8 个未运行子场景，初始 target Resume 的原回执均为 `409 issue_paused`，Provider/输入为 0。改为现有 issue-scope 初始激活后，R02 的双端撤权通过，两个通知场景因记录自动展开后按钮为 Hide 而失败，整轮仍为 2 PASS / 2 FAIL。只修正同一完整 run UUID 的 View/Hide 两个实际按钮名称后，R03 在新库完整通过 4 顶层/10 子场景并独立接受；原失败→新成功→Stop 历史及 R01/R02 原件全部保留。

1. 用实际 R08 server/client 树和当前共享 helper 组装新来源，不能引用不存在的 `source-block-integrated-r03`。
2. 核对三项明确编译的 executable：daemon test binary、`cmd/multica` CLI、受控 Provider。编译、readiness、配置与所有链接分别绑定；不查找用户安装的模型 CLI。
3. 为独立新专库分配 API、生产 Web、renderer/preload、部署 ID、私有状态根与池预算。API/WS/构建地址一致后进入测试体。
4. 单轮执行 **4 顶层 / 10 子场景 / 10 受控 Provider 启动 / 12 原人工输入**，保留原失败→新成功→Stop、原请求/账本重启、结果收件人及原 business token 撤权后的每个拒绝。
5. 记录实际 daemon/Provider/CLI PID 与原始凭据的哈希；完整 proof/终结、gate 恢复、监听者和连接清理通过独立检查后更新相应 TC-14/21/22 子断言。

```text
playwright test e2e/collaboration-recovery-runtime.spec.ts --workers=1 --retries=0 --max-failures=0 --reporter=json
```

只对绑定私有配置执行。静态检查、`--list`、人工上传和旧 timeout-proof 均不能充当这次实际运行。

## 4. 进度、提交与后续

每个增量独立验收后更新父矩阵、`session-progress.md`、父/子 `task.json` 和 spec；失败轮次按原结果保留，不跨轮拼接通过。提交只暂存本轮明确路径，使用 Lore trailer，不推送。完整 S3 的 R01–R09 均满足后才归档；S4 输入就绪后跑 E01–E07，再执行 S5 正式采样和 24 条总验收。

本轮的当前受控 L1/L2 已映射到 R03：四个已记录自然结果重放得到无内容 `200 outcome_recorded`，四个改变结果返回 `409 outcome_conflict`，十个新增迟到事件返回 `403 business_capability_required`；十二组重放/迟到核账前后相同、六组旧事实保持，恢复启动为 0。Stop/撤权的未记录本地结果被拒绝，不计作原已记录成功 ACK。新增输出写入入口仍须验证自己的身份、权限与副作用。

后续按以下顺序推进：

1. 明确 Agent 输出附件的现有运行写入入口，复用加密存储、原文件/请求身份、当前 attempt/capability 与已有回执，验证未知提交、并发重投、不同字节、撤权及迟到重试后的唯一对象和评论/结果引用。
2. 为普通 priority/assignee 的当前 UI 意图携带既有 numeric revision，验证两端同观察版本竞争的真实拒绝和 fresh 续接；旧无版本 API 保持兼容。若仅作 S3 R04 范围处置，父 TC-16 仍明确保留 PARTIAL。
3. 完成 R09 的最终来源、配置、程序、Web/renderer/preload、原 gate/PID/连接与 R01–R09 总核对，复用未受影响的已接受证据；S3 自身验收完成后归档。
4. S4 输入 I02–I09 就绪后执行 E01–E07，完成真实支付/履约、仓库/CI、非生产发布回滚和指标/审核人验证；再执行最终候选 S5 的 100 用户 × 100 事件、100 次回写与 24 条业务签字。

本轮开始 HEAD 为 `b1901a48f6990d39e9bd5a588d405342aaa2016f`，暂存区为空。原 11 月 19 日研究所记录的旧 HEAD 与环境保留为历史，本轮另分配来源/配置/专库，不默认消费旧分配。
