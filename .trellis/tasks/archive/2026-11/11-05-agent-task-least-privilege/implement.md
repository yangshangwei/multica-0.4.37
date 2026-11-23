# 实施与所有权

1. 研究当前代码并独立评审 PRD/design，记录问题和解决；主会话确认具体语义后 start。
2. trellis-implement 独占本子任务产品代码：task_token migration/query/generated；auth capability types/guard；middleware Auth/password auth；TaskService.FinalizeTaskClaim；daemon claim shared builder与必要两个入口；comment dispatch guard；四个模板／role skill及对应 source map；针对性测试。主会话拥有任务树、spec、Git提交、数据库分配。其他 lane 不得编辑这些路径。
3. 先以真实路由/数据库用例记录越权基线，再最小实现。新增接口自身编译错误不能冒充 red。测试不得运行用户 agent CLI。
4. 主会话提供独立 PostgreSQL17/pgvector 容器，按包分配数据库；子代理只能执行被许可测试，不创建/迁移/删除DB。Go 使用-p1与GOMAXPROCS=2，控制连接与CPU。原始输出放私有临时目录，任务保留脱敏汇总与源指纹。
5. gofmt、sqlc生成一致性、go build、go vet，定向middleware/auth/service/handler/router测试及必要race。仅真实命名pass计入，不把skip/编译当行为通过。失败保留，修复后新编号。
6. trellis-check 独立检查执行能力／撤权／越权／兼容和工具绕过，写 independent-review.md；发现缺口返回实现者。
7. 主会话更新可执行spec、验收结果、父任务进度；Lore格式原子提交。父任务保持in_progress，M1整阶段未验收前不宣称M2/M3完成。

## 验证范围
必须覆盖 auth 两种模式、真实claim两个入口、expired/deleted/old-generation/terminal、originator撤权、跨project、UUID和标识符／重复query、任意修改、评论三类唤醒、未知路由、旧实例和human control、额外broker credential拒绝。新权限必须真实连到API和claim，不以纯helper测完结束。

独立评审已完成；plan-review.md 的6项约束及design R1/R2为强制实现与验收合同。所有权追加必要handler canonical loader/issue-comment读取与现有plugin daemon backstop、comment SQL及其生成代码。不得触碰另一lane的squad_briefing/CLI/execenv mention段。
