# S02 技术设计边界

本切片采用父任务设计，以下引用为本任务技术契约，不复制另一套易分叉规则：

- [terminal-execution-design.md](../10-01-platform-admin-console/terminal-execution-design.md)
- [capacity-plan.md](../10-01-platform-admin-console/capacity-plan.md)
- [detailed-design.md](../10-01-platform-admin-console/detailed-design.md)

## 数据与边界

角色/组织/安装/task分别保留各自身份；服务端完成授权，query/schema执行平台作用域；私有内容延续原业务权限。所有新表无外键/级联，每个索引使用独立单语句并发迁移。

## 依赖与集成

S01 的相关契约及验证通过；具体依赖见父 implement.md。

本任务不占用迁移编号。router注册、公共导出、locale和sqlc生成由当批集成人统一处理。已有文件若被别的切片修改，应合并兼容而不还原其内容。

## 风险验证

- [ ] 接入签名/nonce/部署/账号版本、绑定抢占与并发接入。
- [ ] 同机多OS用户、多profile、多runtime、换账号、清数据与保留数据重装。
- [ ] 窗口关闭/引擎不可用/休眠/未知探测、旧端未关联不误开放控制。

功能回退遵循父 migration-rollout.md；关闭页面不取消已生效的权限、账号禁用或接单策略。
