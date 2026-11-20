# 设计方向与实施前复核

复用 `server/internal/handler/actor_guards.go` 所拥有的机器凭据来源判据，管理员例外需要同时满足“当前人类凭据”及“当前工作空间 owner/admin”。作者路径保持独立：机器即使不能走管理员路径，也需经过自己的来源与能力授权。不得仅校验 resolveActor 返回 member，也不得把 machine 绑定的人当成实际操作者。

实施前追踪 auth middleware → actor provenance → workspaceMember → Update/Delete → 内容/附件/评论事件；列出路由已拦截与处理器仍需防御的矩阵。拒绝发生在请求写入之前。成功路径保持 revision 冲突和原作者字段语义。若要新增审计字段，应先明确迁移需求并单独评审，不能悄悄扩大本任务。

产品根因修复后，v2 放行仍检查 revision，因为合法人类管理员和非正文动作仍会改 revision。回滚以权限提交为单位，回滚前必须明确重新开放的机器权限风险。
