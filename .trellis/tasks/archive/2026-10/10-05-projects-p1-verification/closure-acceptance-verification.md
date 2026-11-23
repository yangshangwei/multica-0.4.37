# G3：范围闭合不等于项目验收

2026-10-05，真实 health 私库回归通过。只增加测试，未修改生产代码。

`server/internal/handler/project_health_test.go` 中新增 `TestProjectHealthClosedScopeDoesNotImplyAcceptanceOrChangeProjectState`，通过真实 ProjectOverview handler 和 PostgreSQL 检查：

| 当前正式任务 | N/F/C/U | closure_ratio | 最近／当前描述验收 | 持久项目状态 | 新进展/验收记录 |
| --- | --- | ---: | --- | --- | ---: |
| 两条 done | 2/2/0/0 | 1 | 均null | planned不变 | 0 |
| 两条 cancelled | 2/0/2/0 | 1 | 均null | planned不变 | 0 |
| done + cancelled | 2/1/1/0 | 1 | 均null | planned不变 | 0 |

这条证明覆盖真实验收表查询与持久项目状态，不用纯Compute结果代替AC-04/09要求；UI对应展示由UI lane验证。

命令（cwd=server，先source `.env.worktree`，再覆盖health私库环境）：

```sh
set -a
source ../.env.worktree
source ../.omx/projects-p1-test-env/health.env
set +a
../scripts/go-test-with-agent-cli-guard.sh go test -race ./internal/handler -run '^TestProjectHealth' -count=1 -v
```

结果：exit0，**14个health顶层测试全部通过**，新三种终态子场景全部通过，2.949s；CLI guard未报告真实agent调用。日志 `.omx/p1-health-closure-acceptance.log`。gofmt与差异检查通过。测试-only变更不影响已记录的生产二进制性能证据。
