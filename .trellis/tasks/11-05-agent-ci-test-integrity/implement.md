# 实现与验证

1. 添加包装脚本空配置拒绝与 workflow 数据库/迁移顺序回归；运行证明旧配置失败。
2. 增加离线实际二进制前置条件回归，go test -c 仅编译，不用 go test -run '^$' 充当编译。
3. 修复 wrapper、三个 TestMain、Release services/env/migrations；保持成功 fixture 和 cleanup 语义。
4. 主 CI backend 过滤加入 Release、新测试等必要文件；显式 Node22，执行相关回归。
5. 运行 bash scripts/test-go.test.sh、node --test scripts/changelog-workflow.test.mjs、node --test scripts/go-test-database-preconditions.test.mjs；脚本使用 task-owned Git 外 tmp，Go 编译并发有界。
6. bash -n、gofmt、Go vet/compile 对受影响包、git diff --check。证据应有实际用例数和退出码；不启动 DB 或全测试，不触发线上发布。向主会话报告，无自行提交。
