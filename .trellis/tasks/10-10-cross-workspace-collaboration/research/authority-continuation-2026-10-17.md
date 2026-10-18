# 2026-10-17 资源授权续作

本分工落实 E01/E02/E07 与 AC96 的服务端前提，真实调度、许可和原生终结证明仍由后续阶段完成。

## 改动边界与顺序

1. 先用数据库回归固定实际 owner 校验及配置修改后不得复活旧授权的缺口；保留既有队列、停止和撤权测试。
2. 在目标服务及新的资源授权服务中实现 creator/handler 登记、当前参与 owner 确认、完整名册和当前授权判断。评论服务借同一个事务调用；目录只读取本单目标。
3. 迁移 660–679 由本分工保留。复用现有 managed/enrolled binding 代次；仅为没有配置代次的 agent/runtime/squad 增加行内代次，排除显示及心跳字段。目标固定配置摘要，grant 固定 owner 参与代次及明确 consent 元数据，旧 grant 不自动升级。
4. 统一停止及失效的 command → group → run/delegation 锁序；仅做保持业务语义的锁序调整，以确定性双连接竞争和原停止矩阵验证。
5. 提供目标登记/owner 确认 handler，路由由内容分工整合。sqlc 由主会话统一生成。隔离数据库验证后写入证据。

不添加账号或空间成员关系，不暴露额外资源目录，不自动恢复队列，不签发 launch/proof，不调用真实 AI，不提交代码。

## 共享接口

- `AuthorizeTargetInputInTx(ctx, tx, issueID, target) (code, error)`：caller 已按 user/workspace/issue/session/target 顺序加锁，使用同一事务，grant/resource 后置 NOWAIT 防止与旧资源 writer 反向等待。空 code 仅表示可保存 waiting 输入。
- `AuthorizedMentionTargets(ctx, q, access)`：借已授权的一致读快照，仅返回完整有效授权目标。paused/busy/offline 与运行支持分别表达，`execution_available=false`。

## 当前证据

待本段实现及检查后更新；本文件不声称 AC96 或完整运行协议已验收。
