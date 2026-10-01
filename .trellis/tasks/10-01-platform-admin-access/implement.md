# S01 实施分解

状态：计划，下面步骤未执行。需求见 prd.md；依赖：无；本期入口切片。

## 负责文件与模块

- `server/internal/handler/admin_auth.go（新增）`
- `server/internal/service/platform_admin.go（新增）`
- `server/internal/handler/auth_password.go`
- `server/internal/auth/password_session.go（保留公共锁的PAT续期能力）`
- `server/cmd/server/platform_admin.go（新增）`
- `server/cmd/server/router.go`
- `server/pkg/db/queries/admin.sql、organization.sql（新增）`
- `server/migrations/（组织、角色、账号禁用字段、audit/operation基础与独立并发索引）`
- `packages/core/admin/（schema、API、Query作用域）`
- `packages/views/admin/admin-shell.tsx（新增）`
- `apps/web/app/admin/layout.tsx、page.tsx（新增）`

括号中的新增路径为计划落点；实现前按真实导出和模块规则核对。不能编辑 generated 文件代替源SQL。

## 步骤

S01先实现通用admin_operation幂等创建和当前actor/组织范围内的按key查询；响应丢失仍能找回角色操作。S04与S05均依赖本基础，不互相等待通用查询接口。

1. 读取父设计及本任务JSONL，确认上游验证记录、当前修改和拟触及文件所有权。
2. 将下面三类行为分别写成权威层回归用例，先运行确认是预期行为失败。
- [ ] JWT/cookie与所有PAT/机器凭据直接及转换路径的授权矩阵。
- [ ] 两管理员互降、授予角色旧会话失效、无workspace登录和最后管理员保护。
- [ ] 审计失败事务回滚，普通桌面/CLI登录和PAT续期兼容。
3. 实现最小后端/协议路径，参数/schema/权限/事务先完成，再接入Query与页面。
4. 对应回归通过后测试失败路径、竞态与旧版本兼容；不添加绕过授权的测试专用生产分支。
5. 按父 test-spec.md 运行所属包 test/typecheck/lint/static检查；DB/协议变更运行对应Go和race验证。
6. 对照父AC做纵向验收，写 verification.md；共同模块变更由集成人串行检查。
7. 更新已证实的spec知识并按Lore协议形成可审阅提交；保留未测项，不推送或部署未授权环境。

## 验证命令

使用[父测试规格](../10-01-platform-admin-console/test-spec.md)中适合变更层的命令。新测试选名以实际文件为准；运行前准备task-owned数据库和生产模式Web。S07负责全量集成与容量/原生证据，不能用子任务定向通过替代全部验收。
