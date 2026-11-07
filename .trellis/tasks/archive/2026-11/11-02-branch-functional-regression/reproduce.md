# 复现入口

本轮产品候选7af9ab9905c98c77bb091223ebe687c639475bb4。当前所有测试服务已停止，不能直接执行旧浏览器命令后假定端口仍由本任务占用。

1. 先读test-plan.md、design.md、implement.md和report.md。每个`evidence/<run>/run.json`保存确切command、cwd、候选、数据库、配置hash、API/构件和harness指纹；各目录保存原始JSON、stdout/stderr及附件。
2. 本轮private配置和协调工具位于`.omx/private/branch-functional-regression-20261102/`，权限私有且未进入报告；不要输出凭据。旧run和phase目录不能覆盖；重新运行应新编号、新数据库/registry及真实端口/进程身份核对。
3. 生产Web build ID为`wYec8nnmn8Rukz4PiJAJ7`，renderer/API/WS来源曾逐一核对；复用只有在同源码、相同public地址和hash实际一致时才有效。当前根分支有已审阅的测试改动，不能只以HEAD相同描述harness字节。
4. Go完整包最新证据清单在`research/go-current-summary.json`；DB测试各用独立新迁移库，不能`go test`隐式命中默认库。tagged knowledgeintegration数据库前缀是已有fixture契约，见其源码和`research/knowledge-tagged-summary.json`。
5. 当前439库存对账：`python3 .trellis/tasks/11-02-branch-functional-regression/evidence/reconcile-e2e.py`，只读取已完成raw reporter更新本任务表。总执行记录包含原失败和skip，不把它当独立功能数量。
6. 复验测试修复应连同`research/*harness*.patch`、integration与review记录审查。不得删业务断言、增大超时、启用自动重试、伪造旧launcher clearance或使用用户Agent账号。

初始旧验收目录`.trellis/tasks/11-01-branch-e2e-acceptance/`保留原样；822c/639旧结果不代表当前版本。browser数据库可能被24hTTL GC收回，重建须新任务自有数据库，不恢复到默认部署。
