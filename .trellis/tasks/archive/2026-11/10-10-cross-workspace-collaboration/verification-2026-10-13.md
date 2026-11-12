# 本轮实现验证（2026-10-13）

结论：技术复审已完成，当前权限/数据、账号读取、队列内部控制和 core 兼容基础通过下列检查。**P1–P8 尚未全部实现，89 条完整 AC 仍未通过，创建入口保持未注册。** 本报告区分基础回归与端到端交付，不能将其当作发布批准。

## 新规则的实现落点

| 复审项 | 本轮代码与验证 | 剩余工作 |
| --- | --- | --- |
| AU01/AU05 非终态与来源成员退出 | 授权器拒绝外部 handler 借普通状态编辑重开；退出来源空间仍保留参与/creator 能力；撤销参与后只按当前来源成员权读取 | 完整生命周期/状态编辑服务、真实账号安全失效 writer |
| AU02 评论版本/附件交付 | schema 复用 comment.revision，并提供 deleted_at、附件交付及多引用存储；旧客户端数字版本不改 | 评论历史/私有附件/清理服务接线 |
| AU03 机器搜索 | 搜索谓词、排名、comment snippet、列表私有筛选/排序/计数受可信 principal 控制；机器只得最小引用；正文和评论独有 sentinel 负例通过 | plugin/IM 出站、导出及全部系统/执行 writer 清单 |
| AU04 线程共锁 | 本轮仍关闭未适配的旧评论动作 | 线程 resolve/unresolve 与 edit/delete 共锁的完整实现 |
| EX01/EV 停止证明 | Stop 不改变历史 fence/nonce；发过许可、有运行/成功矛盾或任一未结屏障都不能恢复；仅确定从未许可可结算 | 原生封闭执行范围、不可重用启动账本、认证的 sealed proof、真实 FIFO promotion/claim |
| CL 客户端协议/隐私 | 旧数字版本与协作 int64 字符串分开；拒绝错单/错来源及迟到身份响应；schema、HTTP 错误与 JSON 语法错误均不泄漏私有日志 | 通知、Query/WS/撤权缓存、双端页面及已安装旧客户端验证 |

## 本轮改动和简化

- 服务端：collaboration_queue.go 和 collaboration.sql 新增借用同一事务的队列数据操作；入队、取消、Stop 和拒绝 Resume 共用操作回执、锁后授权及有界重试。不引入新调度器或依赖。
- 权限/查询：issue.go、collaboration_boundary.go、issue.sql 的机器查询在匹配前裁剪；普通任务继续原路径。生产 router 的新测试验证真实 JWT/PAT 与 task-token 身份，而非仅手工注入 handler 上下文。
- 客户端：client.ts 用同一协作读取封装处理身份代次、取消和畸形响应，复用已有 UUID 比较与 schema；没有创建平行状态存储。
- 验证：扩展 collaboration_test.go、collaboration_boundary_test.go、collaboration_queue_test.go、core API/schema 测试；sqlc 生成文件与最终 SQL 对齐。规范和 progress/implement/acceptance/task 元数据同步真实状态。

## 执行证据

测试使用隔离库 multica_collaboration_20261013_87773；环境文件位于 /tmp/multica-collaboration.Qp1TA6/env，仅 source。迁移回退演练在该库内新建的私有 fixture schema 运行，不影响应用 schema。日志目录统一为 /tmp/multica-collaboration.Qp1TA6/。

| 检查 | 最终结果 | 日志 |
| --- | --- | --- |
| go test ./internal/migrations -run TestCollaborationSchemaRetainsIdentityAndExecutionBarriers -count=1 -v | 1 组通过；无 FK/invalid index、唯一 session/run、未知 proof 屏障、忙 writer/有数据拒绝 down、空 schema 回退 | verification-migrations.log |
| go test -race ./internal/collaboration ./internal/middleware（可信身份及协作授权定点选择） | 6 组通过 | verification-principal.log |
| go test -race ./internal/service -run TestCollaboration -count=1 -v | 19 组、29 子项通过 | verification-service-final.log |
| go test -race ./internal/handler（协作、旧入口、普通搜索/批量/删除/状态分类控制样本） | 74 组、29 子项通过 | verification-handler-final.log |
| go test ./cmd/server -run '^TestCollaborationRoutesUseAuthenticatedAccountWithoutSourceMembership$' -count=1 -v | 1 组通过；含 JWT/PAT、真实 task token、撤权、无来源 membership、创建路由未注册 | verification-router.log |
| pnpm --filter @multica/core test api collaboration --maxWorkers=2 | 35 文件、855 测试通过，其中协作 44 项 | [客户端报告](research/verification-compatibility-2026-10-13.md) |
| pnpm --filter @multica/core lint / typecheck | 均通过 | 同上 |
| pnpm typecheck | 9/9 个任务成功（5 缓存）；不含 Mobile | typecheck-workspace.log |
| go vet ./internal/collaboration ./internal/auth ./internal/middleware ./internal/service ./internal/handler ./internal/migrations ./cmd/server | 通过 | vet-collaboration.log |
| make sqlc + 生成目录 SHA256 前后比较 | 最终再生成无变化 | sqlc-final.log、verification-summary.json |

最终 Go 计数：101 个顶层测试、58 个子项；0 FAIL、0 SKIP，使用 -race 的测试无竞态报告。没有运行整仓 make test/pnpm test 或 Web/Desktop E2E；已有相关领域和普通任务控制样本不替代完整 P8。

Handler 最终选择表达式：

```text
Test(Collaboration|Legacy.*Collaboration|WorkspaceWithCollaboration|BuildSearchQuery|SearchIssues|BatchUpdate|BatchDelete|DeleteWorkspace|ListOpenIssues|CustomTerminalStatusCountsAsTerminalInSQL|ListEndpointsCarryStatusCategory|ListFilterByCategoryReturnsCustomStatusIssues)
```

Principal 最终选择表达式：

```text
Test(CreatorKeepsLifecycle|ExternalHandlerCannotReopen|CollaborationAccess|MachineAndRestricted|PrincipalRequires|Auth.*Principal|AuthenticatedPrincipal)
```

## 失败到通过的证据解释

- 客户端先复现 14 个失败，随后追加取消/JSON 损坏竞争再复现 1 个失败；修正后相关 855 测试通过，详见客户端报告。
- 队列独立复核补出 6 个红例：零许可但有运行/成功/未知进度的矛盾、两类不一致屏障、无 command 回执的零序号。修正后全 service 定点回归通过，详见 [队列报告](research/verification-queue-review-2026-10-13.md)。
- 权限新测试一度错误断言 status=todo 必须返回零条，实际返回的是合法普通任务。按普通任务行为不变的契约，修正为准确验证普通样本 ID/数量，同时继续要求协作记录缺席；最终 handler 全部通过。
- 两个代理在收尾时限流。主会话接管 authority/handler/router 检查，剩余 checker 接管队列修正和最终 service 回归；未把失败轮次计作通过。

## 下一阶段必须保留的限制

1. Account epoch 目前比较逻辑已验证，真实 disable/reset/revoke 的推进和持久失效 fanout 未接线；fixture 直接 UPDATE 不能证明真实禁用恢复安全。
2. 运行中的 run 均由测试 fixture 建立；尚无共享 promoter、claim/finalize/permit、窄业务/终结 token、监督器账本或 proof 接收。真实原生停止确认未完成，Resume 目前只实现持久拒绝。
3. 私有附件、线程/评论、出站机器隔离、人类流转、有限代管、双端界面和旧安装客户端仍按 implement.md 完成。
4. 本任务保持 in_progress；本轮没有提交、部署或开启创建。并行会话的桌面下载/发布/归档变更不属于本轮验收。
