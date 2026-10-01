# S06 技术设计边界

本切片采用父任务设计，以下引用为本任务技术契约，不复制另一套易分叉规则：

- [detailed-design.md](../10-01-platform-admin-console/detailed-design.md)
- [capacity-plan.md](../10-01-platform-admin-console/capacity-plan.md)
- [test-spec.md](../10-01-platform-admin-console/test-spec.md)

## 数据与边界

角色/组织/安装/task分别保留各自身份；服务端完成授权，query/schema执行平台作用域；私有内容延续原业务权限。所有新表无外键/级联，每个索引使用独立单语句并发迁移。

## 依赖与集成

S02、S03、S04 的相关契约及验证通过；具体依赖见父 implement.md。

本任务不占用迁移编号。router注册、公共导出、locale和sqlc生成由当批集成人统一处理。已有文件若被别的切片修改，应合并兼容而不还原其内容。

## 风险验证

- [ ] 时区/窗口/分母0/缺用量一致，Token不伪装实际账单。
- [ ] 失联、排队超时、失败告警幂等补偿、认领冲突、恢复与新episode。
- [ ] DB/检测器故障不误报全体离线，observer所有写操作拒绝。

功能回退遵循父 migration-rollout.md；关闭页面不取消已生效的权限、账号禁用或接单策略。
