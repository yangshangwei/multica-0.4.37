# S05 技术设计边界

本切片采用父任务设计，以下引用为本任务技术契约，不复制另一套易分叉规则：

- [security-design.md](../10-01-platform-admin-console/security-design.md)
- [detailed-design.md](../10-01-platform-admin-console/detailed-design.md)
- [migration-rollout.md](../10-01-platform-admin-console/migration-rollout.md)

## 数据与边界

角色/组织/安装/task分别保留各自身份；服务端完成授权，query/schema执行平台作用域；私有内容延续原业务权限。所有新表无外键/级联，每个索引使用独立单语句并发迁移。

## 依赖与集成

S01 的相关契约及验证通过；具体依赖见父 implement.md。

本任务不占用迁移编号。router注册、公共导出、locale和sqlc生成由当批集成人统一处理。已有文件若被别的切片修改，应合并兼容而不还原其内容。

## 风险验证

- [ ] A操作B只撤销B，JWT/PAT/daemon/task/plugin与WS一致失效。
- [ ] 禁用保留历史，恢复不复活旧凭据，临时密码仅可强制改密。
- [ ] 复核失败/并发改密/最后管理员/CLI受控恢复/秘密不持久化。

功能回退遵循父 migration-rollout.md；关闭页面不取消已生效的权限、账号禁用或接单策略。
