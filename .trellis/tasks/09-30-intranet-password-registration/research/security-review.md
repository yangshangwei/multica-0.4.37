# 密码注册与旧账号迁移安全复核

日期：2026-10-01。范围：当前工作树中的 server 认证、迁移、个人派生凭据与撤销实现。阅读了项目规则、security-boundaries.md、PRD、security-contract.md 和 test-spec.md。本轮只补验收测试，没有修改生产代码。

## 结论

静态审查与本轮新增测试未确认认证旁路、原 UUID 丢失或旧请求升级到新凭据版本的问题。这不是全量 test-spec 验收通过，也不是生产部署批准。

## 新增证据

| 测试 | 实际覆盖 |
| --- | --- |
| `TestPasswordMigrationClaimBoundaries` | 经 Auth middleware + GetMe 的旧 JWT iat/exp 缺失、字符串非法值、未来 iat、cutoff 等号/之后、exp/deadline 已到，以及合法旧会话只能处于 setup 状态 |
| `TestPasswordConcurrentBindingAndReplay` | 两个已认证旧会话同时绑定，单个成功，另一个 409；只保留版本 1 与原 UUID；再经 middleware 重放旧 JWT 为 401 |
| `TestPasswordBlockedAuthenticationCannotSurviveReset` | login、PAT mint、PAT renew、password change 四条路径。先锁用户行，通过 PostgreSQL `pg_blocking_pids` 确认实际锁等待，再在持锁事务内改密并撤销；等待请求均 401，无 cookie、无有效 PAT；新密码仍可登录 |
| `TestPasswordBlockedTaskAndDaemonMintRejectsReset` | 同一 finalizer 先成功签发带版本 1/原 owner 的 task 与 daemon token；随后确认另一次 finalization 实际等待用户锁，在重置提交后拒绝旧版本，旧/新 token 均不存在，任务保留 cancelled |
| `TestPasswordRecoveryPreservesUserAndRequiresChange` | 原 UUID 首次恢复与再次恢复；再次恢复撤销 PAT/task/daemon token、取消执行并离线原 runtime；保留 membership、agent 和其他 owner runtime |
| `TestPasswordRecoveryUsernameCollisionRollsBack` | 恢复未绑定旧用户时撞到另一账号用户名，命令失败；目标 UUID、名字和原 PAT 不变，不生成半成品 credential；已有账号 hash/version/限制状态不变 |
| `TestPasswordDaemonHeartbeatRejectsRevocationAndDatabaseFailure` | 正常 heartbeat 可消费测试待办；版本撤销或认证 DB pool 关闭后，heartbeat 关闭连接，处理器不再消费待办 |
| `TestPasswordRealtimeDatabaseFailureStopsProtectedEvents` | 正常推送后关闭独立认证 DB pool，再产生事件，旧连接关闭，收不到该事件 |

测试使用 `.env.worktree` 中经核对的隔离库 `multica_multica_password_registration_593`。命令经 `scripts/go-test-with-agent-cli-guard.sh` 包装，未运行真实 agent。普通新增 handler 测试及恢复测试均通过；最终串行 race 运行亦通过。

## 验证记录

- 新增 handler 用例普通运行通过，3.394s；恢复两项普通运行通过，1.578s。
- 一次 race 与普通 handler 测试误用同一隔离库并行，后启动进程清理了共享 handler workspace fixture，导致前者 FK/404 失败。该结果作废，不据此判断产品行为。后续改为串行 `go test -race -p 1 ./internal/handler ./cmd/server -run '^TestPassword' -count=1`。
- 最终串行 race 命令退出 0：handler 全部 `TestPassword*` 42.694s，cmd/server 全部 `TestPassword*` 5.894s；无数据竞争报告。
- 补充 finalization/恢复普通串行测试通过：handler 2.230s、cmd/server 1.744s；同一筛选的串行 race 退出 0，handler 3.834s、cmd/server 6.996s。
- `gofmt` 与所改文件 `git diff --check` 通过。

## 仍未由本轮证明的边界

- 没有 fake clock：cutoff 等号精确覆盖；exp/deadline 使用当前整秒，证明已到期拒绝，未独立证明严格纳秒等号或重启不延期。
- 绑定/签发竞态有意注入已认证 context，以验证事务内二次校验；不是完整路由或两个真实 API 进程的并发端到端演练。
- heartbeat 使用测试处理器和待办 channel；证明 gate 先于处理器，未执行生产 Redis pending queue 的完整消费链。
- DB 故障使用关闭独立连接池注入；未模拟生产网络分区、连接池耗尽或数据库重启。
- task/daemon finalization 锁竞态已补齐；独立插件安装和成员 callback 的版本隔离复用既有 `TestPasswordCallbackRevocationPreservesPluginActor`，不重复其矩阵。恢复命令无回显终端交互、生产 CPU/NAT/代理/多副本容量仍需另外验收。
- 全仓 `make check` 的结果由主任务记录；此文不能代替其最终退出状态。
