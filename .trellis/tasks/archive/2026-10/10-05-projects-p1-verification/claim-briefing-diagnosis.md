# Claim briefing 测试时序依赖

2026-10-05。针对全仓 race 中 `TestClaimTask_LeaderGetsBriefing` / `TestClaimTask_NonLeaderGetsNoBriefing` 的空 body 报错完成窄诊断；未修改 ClaimTask 生产保护或 health/performance 实现。

## 根因

两个测试借用了 TestMain 在套件开始时创建的共享 runtime。NonLeader 注释声称使用独立 runtime，但其 `createHandlerTestAgent` 实际通过 `handlerTestRuntimeID` 重新取得同一共享 runtime。

认领的既有保护要求 `status='online'` 且 DB heartbeat 不超过150秒（`service/task.go:198` 的 RuntimeClaimFreshnessSeconds；`queries/agent.sql` ClaimAgentTask）。单独执行时新鲜，长套件执行到此处时可能已经失效。全仓失败的 handler package 用时221.486s，符合这个时长依赖条件。不能通过修改生产阈值或跳过检查修复 fixture。

实际返回是 HTTP200 `{"task":null}`，不是空 HTTP body。测试 helper 用 `json.Decoder` 消费 recorder buffer 后再打印 `Body.String()`，因此丢掉了诊断正文。

## 证据

1. health 私库只跑原两测试，带 race / CLI guard，均通过：2.326s，`.omx/p1-claim-briefing-isolated.log`。
2. 新增诊断回归，仅将套件共享 runtime 的 last_seen_at 置为10分钟前并在结束恢复；调用两个原测试，均真实返回 task:null 而失败：1.298s，`.omx/p1-claim-briefing-aged-red.log`。没有执行其它 P1 fixtures，故不需要 P1 残留即可复现。
3. 在 `git archive 0af59c5d5` 的原始 server 与原始schema私库上，用 Go `-overlay` 只注入同一诊断测试；两个子场景同样失败：1.283s，`.omx/p1-claim-briefing-baseline-red.log`。原 Claim handler、task service和这两个测试在P1前后无相关变化。证明是既有 fixture 的潜在时长依赖，不宣称已运行原基线的整套 race 或其历史 CI 一定失败。
4. 修复后相关 Claim/Briefing/Protocol 前缀 **56个顶层测试通过**，含共享 runtime 已过期的新回归及两个子场景：race 4.772s，`.omx/p1-claim-briefing-green.log`。

health 私库命令（cwd=server）：

```sh
set -a
source ../.env.worktree
source ../.omx/projects-p1-test-env/health.env
set +a
../scripts/go-test-with-agent-cli-guard.sh go test -race ./internal/handler -run '^TestClaimTask_|^TestClaim_|^TestBuildSquadLeaderBriefing|^TestSquadOperatingProtocol' -count=1 -v
```

基线工作目录 `.omx/projects-p1-performance/baseline-src/server`，source该lane的baseline.env；使用 `go test -race -overlay=.omx/p1-claim-briefing-overlay.json`（实际命令使用绝对路径）运行新增过期fixture回归。overlay映射到诊断时保存的文件，不改写原archive。

## 最小修正

只改 `server/internal/handler/squad_briefing_test.go`：两测试通过 `dbfx.Runtime` / `dbfx.Agent` 创建各自就绪数据；不刷新共享runtime、不削弱生产门槛。helper改为从 `Body.Bytes()` Unmarshal，保留错误正文。新增共享runtime过期回归，防止再借用套件心跳。

`go vet ./internal/handler` 与差异检查通过；没有执行真实 agent CLI。原 C 性能结果依赖的生产源码未改变。本次没有重跑整个 handler package，完整父级回归由父任务继续。
