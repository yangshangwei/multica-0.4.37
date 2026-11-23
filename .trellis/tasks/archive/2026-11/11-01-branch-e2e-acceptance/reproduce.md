# 复现与交接

本次原始环境、进程/构建身份、命令、私有配置路径和退出码均在 `.omx/reports/branch-e2e-822c15785-20261101/runs/*/execution.json`。私有连接和测试密码不复制到报告。

## ENV-1

先在拥有的专库及密码模式API部署目标源码，构建并启动生产Web，配置 `DATABASE_URL`、`COLLABORATION_E2E_API_URL`、`NEXT_PUBLIC_API_URL`、`PLAYWRIGHT_BASE_URL`、`E2E_PASSWORD_AUTH=1`，均为本地测试地址。固定源码和API健康提交后执行：

```sh
python3 replay-env1.py --source-root /path/to/frozen-checkout --env-file /path/to/private-test.env
```

脚本默认强制822c完整SHA。复验后续候选必须明确增加 `--expected-head <new-full-SHA>`，新结果不能覆盖本轮。脚本创建全新输出目录，不访问真实Provider；退出非零是本轮预期的产品回归失败。它seed retained run后真实写memory/API并从生产UI断言，不代表真实模型摄取。

## 其他失败

按 `coverage.csv` 的spec和完整标题，在同一认证/开关配置下用 `pnpm exec playwright test e2e/<spec> --grep '<完整标题>' --workers=1 --retries=0`；运行前先读对应执行的private_env_file、live config、gate及renderer来源。旧收件人定位器和动态权限时间比较属于已记录测试失配，不可通过降低断言或隐藏首败修补报告。

专库/旧失败库与固定worktree保留，服务已停止。不要恢复任何旧真实Provider实验或沿用别的任务预算。
