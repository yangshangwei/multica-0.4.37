# 设计

复用 ci.yml 的临时 PG17/pgvector + Redis 服务合同，在 Release verify 配置 multica_release_test，与 DATABASE_URL 一致。迁移前确认 current_database，防止隐式 fallback；保持原 DAG、govulncheck 和例外合同。

包装脚本最前置检查 URL 是否非空白；保持串行包、race/timeout 和 agent CLI guard。handler/router 初始化失败返回非零，避免日志输出原 pgx 错误带凭据；使用短连接探测 context，成功后恢复普通 fixture context。testutil 保留无 DB 的 HTTP helper 测试路径，去默认开发库 fallback；显式数据库失败则终止。

新增 stdlib Node 离线 precondition 测试：go test -c 编译三个包，exec 测试二进制时只给合成配置，清空 PG*，指向独占临时目录下不存在 socket，设置超时和只读 options；核验失败类型、无凭据输出及未执行 fixture。缺 URL 的 testutil 是明确可选路径，与必需两包区别验证。

文件所有权：.github/workflows/release.yml、ci.yml；scripts/test-go.sh、test-go.test.sh；scripts/changelog-workflow.test.mjs；新 scripts/go-test-database-preconditions.test.mjs；handler_test.go、cmd/server/integration_test.go、internal/testutil/db_test.go；若现有 guard 回归依赖旧文本需相应更新但不减弱断言。

后续完整 expected-test inventory 独立建设，当前不得把关掉 TestMain 假绿等同全仓零跳过。各源文件位于独立 worktree，DB 负例在执行前排除继承连接变量。
