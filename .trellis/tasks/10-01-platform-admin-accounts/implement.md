# S05 实施分解

状态：计划，下面步骤未执行。需求见 prd.md；依赖：S01 的相关契约及验证通过；具体依赖见父 implement.md。

## 负责文件与模块

- `server/internal/handler/admin_user.go（新增）`
- `server/internal/service/platform_admin.go（扩展）`
- `server/internal/auth/password_revoke.go、password_session.go`
- `server/internal/handler/auth_password.go`
- `server/cmd/server/password_recovery.go`
- `server/pkg/db/queries/admin.sql、user.sql、password.sql`
- `packages/core/admin/user-queries.ts（新增）`
- `packages/views/admin/users/、administrators/（新增）`
- `apps/web/app/admin/users/、administrators/（新增路由壳）`

括号中的新增路径为计划落点；实现前按真实导出和模块规则核对。不能编辑 generated 文件代替源SQL。

## 步骤

1. 读取父设计及本任务JSONL，确认上游验证记录、当前修改和拟触及文件所有权。
2. 将下面三类行为分别写成权威层回归用例，先运行确认是预期行为失败。
- [ ] A操作B只撤销B，JWT/PAT/daemon/task/plugin与WS一致失效。
- [ ] 禁用保留历史，恢复不复活旧凭据，临时密码仅可强制改密。
- [ ] 复核失败/并发改密/最后管理员/CLI受控恢复/秘密不持久化。
3. 实现最小后端/协议路径，参数/schema/权限/事务先完成，再接入Query与页面。
4. 对应回归通过后测试失败路径、竞态与旧版本兼容；不添加绕过授权的测试专用生产分支。
5. 按父 test-spec.md 运行所属包 test/typecheck/lint/static检查；DB/协议变更运行对应Go和race验证。
6. 对照父AC做纵向验收，写 verification.md；共同模块变更由集成人串行检查。
7. 更新已证实的spec知识并按Lore协议形成可审阅提交；保留未测项，不推送或部署未授权环境。

## 验证命令

使用[父测试规格](../10-01-platform-admin-console/test-spec.md)中适合变更层的命令。新测试选名以实际文件为准；运行前准备task-owned数据库和生产模式Web。S07负责全量集成与容量/原生证据，不能用子任务定向通过替代全部验收。
