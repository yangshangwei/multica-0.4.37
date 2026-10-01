# S07 实施分解

状态：计划，下面步骤未执行。需求见 prd.md；依赖：S01、S02、S03、S04、S05、S06 的相关契约及验证通过；具体依赖见父 implement.md。

## 负责文件与模块

- `e2e/platform-admin.spec.ts（新增）`
- `server/internal/migrations/platform_admin_migration_test.go（新增）`
- `各切片权威测试文件（按失败所属层补充）`
- `apps/docs/content/docs/（管理后台使用与部署文档）`
- `SELF_HOSTING.md`
- `本子任务 verification.md、release-checklist.md、容量与原生验证报告`

括号中的新增路径为计划落点；实现前按真实导出和模块规则核对。不能编辑 generated 文件代替源SQL。

## 步骤

1. 读取父设计及本任务JSONL，确认上游验证记录、当前修改和拟触及文件所有权。
2. 将下面三类行为分别写成权威层回归用例，先运行确认是预期行为失败。
- [ ] task-owned password模式生产Web E2E，并保留classic全量回归。
- [ ] 1000安装、100万执行、10观察者、集中重连的实测报告。
- [ ] 旧/新原生客户端、迁移索引/回填、灰度/回退、权限撤销竞态完整证据。
3. 实现最小后端/协议路径，参数/schema/权限/事务先完成，再接入Query与页面。
4. 对应回归通过后测试失败路径、竞态与旧版本兼容；不添加绕过授权的测试专用生产分支。
5. 按父 test-spec.md 运行所属包 test/typecheck/lint/static检查；DB/协议变更运行对应Go和race验证。
6. 对照父AC做纵向验收，写 verification.md；共同模块变更由集成人串行检查。
7. 更新已证实的spec知识并按Lore协议形成可审阅提交；保留未测项，不推送或部署未授权环境。

## 验证命令

使用[父测试规格](../10-01-platform-admin-console/test-spec.md)中适合变更层的命令。新测试选名以实际文件为准；运行前准备task-owned数据库和生产模式Web。S07负责全量集成与容量/原生证据，不能用子任务定向通过替代全部验收。
