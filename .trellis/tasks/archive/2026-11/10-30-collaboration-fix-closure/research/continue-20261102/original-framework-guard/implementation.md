# 原实验 E5 停止规则与追加式续作登记

## 实施范围

仅在两条新 detached worktree 的 `host_context_experiment*.go` 覆盖层修改：

- baseline：`/Volumes/artisan/code/2026/mc-context-guard-baseline-20261102`，产品 HEAD `c58f79874cfe2e6318e3dce067d7b6bac24e943f`。
- after：`/Volumes/artisan/code/2026/mc-context-guard-after-20261102`，产品 HEAD `835e7b046c6841e0945a59326bc4ddcbae4a0df7`。
- 根任务新增 `original-series-run/prerun.json` / `run_scenario.py` 及本目录证据。原 baseline/resume worktree、旧框架、旧样本、业务源码和其他任务未提交工作均保留。

最小缺口是 `runSeries` 原先只在 runtime/证据错误后停止；E5 `finished/fail` 会继续后续 Provider trial，再由入口在整批结束后报告行为失败。

## 改动

1. 在原 `env.run` 已持久化失败样本且未返回 incomplete 错误后，`E5_injection` 的原 `fail` 判据立即停止剩余 trial。返回专用 blocking-behavior 错误，入口明确报告行为失败，保留 `finished/fail` 原分类。
2. test-created fake Provider 增加受控 canary 输出；四项完整 Runner fake 回归覆盖 canary 单次启动、safe E5 三次启动、incomplete 仍为 runtime 错误、其他场景行为失败仍保留三轮。现有 E6 配对/恢复和 incomplete 停止回归继续通过。
3. 新追加式登记只包含剩余 14 次：E3 after trial2/3（2次）、E4 baseline1–3（3次）、E5 baseline1–3（3次）、E6 baseline1–3/hop1–2（6次）。两个 source HEAD、模型、CLI、thinking/tier、场景、判分、超时和原槽位均沿用原登记。
4. 新 runner 核对两个 source HEAD、准确四文件集合/各文件哈希/合并哈希、dirty scope、固定 CLI 文件哈希和原 cursor。`--check-only` 不启动 Go 或 Provider；实际执行还要求主会话的清晰放行记录。场景目录拒绝覆盖，command/log 仅写追加目录，已有 trial 不替换。

## 红绿证据

- red：旧 `runSeries` 对同一 test-created canary 实际启动 **3 次**，canary-stop 回归失败；另外三项控制回归通过。原日志 `e5-stop-red-baseline.jsonl`，统计 **4 run / 3 pass / 1 fail / 0 skip**。
- green：两条新 arm 串行运行 `^TestContextExperiment`，各 **32 run / 32 pass / 0 fail / 0 skip**。命令、时间、原 JSONL 哈希见 `fake-green-{baseline,after}.json`。
- runner：四项注册 `--check-only` 通过；旧 E3 after trial1 和未知选项拒绝；无 execution clearance 的实际入口在创建场景目录或启动 Go 前拒绝。共七项静态契约，`runner-preflight-contracts.json` 保留实际输出。
- Go 文件执行 gofmt；Python runner 用 AST 语法检查。未运行全量 Go/DB、lint/typecheck、真实 Provider、正式 E0 或账号探测。

## 指纹与原证据保留

- 原框架：`f6f62e55888fa2d3e733ff12dec6054e07f877828363ad0d503ada6db07aa0bd`，原两条 worktree 的三文件哈希已再次验证相符。
- successor 框架：`ff2ac795d7ec3a6ca9664276200fe61f2526fe40c496ad7bc137b42099bd08c0`，两条新 arm 四文件字节一致；各文件哈希见 `successor-framework.json`。
- runner：`da39ef5ec1f0c6ae1607f16dacf82cd247bb1d70804a1880112127bd258f5042`。
- preregistration：`2f77e27d357f2b5cf9b2eedc386b07981e4ed50e8252c69227a51b733ad50dd1`。
- 仅 framework 变化，不将新产品修复回灌旧对照；后续样本必须记录 successor hash。最小 diff 见 `framework-stop.diff`。

## 主会话的实际执行入口

独立复核通过、完整候选验收通过、当前授权及独占窗口确认后，主会话可在 `original-series-run/execution-clearance.json` 填写以下字段。此文件未由实施子代理创建：

```json
{
  "owner_session": "01a19c0d-cce1-73e3-99ee-2c3c9e9239ba",
  "authorization_confirmed": true,
  "wholecheck_passed": true,
  "exclusive_window_confirmed": true,
  "blocking_findings_open": 0,
  "remaining_series_call_cap": 14,
  "framework_sha256": "ff2ac795d7ec3a6ca9664276200fe61f2526fe40c496ad7bc137b42099bd08c0",
  "runner_sha256": "da39ef5ec1f0c6ae1607f16dacf82cd247bb1d70804a1880112127bd258f5042",
  "prerun_sha256": "2f77e27d357f2b5cf9b2eedc386b07981e4ed50e8252c69227a51b733ad50dd1"
}
```

首次入口：

```sh
python3 .trellis/tasks/10-30-collaboration-fix-closure/research/continue-20261102/original-series-run/run_scenario.py E3_member_scope after 2
```

之后仅由主会话依登记顺序串行调度并核销预算。首个 incomplete 或 E5 canary 行为失败后停止全部后续场景；`command.exit_code` 不代替原 JSONL 的语义判定。历史 E1/E2a 的无效槽位、原 E3 after trial1 和历史 E0 保持原证据范围。
