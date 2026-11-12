# Runtime 归档阶段复核 — 2026-11-12

本轮范围为尚未提交的 Runtime 续作准备代码。核心产品此前交付结论不变；本报告不认定真实续作完整可用，也不改判任何历史失败。

## Findings (fixed)

- **RC1 — `run_continuation.py:214` 的 `--check-only` 未阻断未来真实分派。** 旧脚本只记录该参数；若未来准入均满足且显式真实开关开启，它仍会建立调用记录并执行 Go 真实入口。补充成功预检后的立即返回，保留预检失败时拒绝。所有 gate 当前始终 false，本轮未触发真实路径。
- 新增可重复的隔离 `check_only_regression.py`：准入、记录写入、进程调用均 mock。旧脚本副本运行 5 例为 3 pass / 2 fail；修复后 5 / 5 pass。覆盖全开 gate 下 check-only 不调用、无真实 opt-in 的纯检查、拒绝、普通运行拒绝及普通运行 mock 分派。
- 保留修复前脚本副本与 SHA，不改历史 registration、授权或结果。当前 launcher SHA 已改变，因此旧登记失配是明确的预期状态。

## Findings (not fixed)

### RC2 — H6 双启动真实续作接线尚未完成

`rk1_scenarios_recall_test.go:180,185` 在一个 H6 trial 内顺序调用两次 `f.plan` / `f.run`。新的 `rk1RuntimeContinuationPlanBinding.bind`（`:752`）在每次 plan 时 reserve，但 `reserveSlot`（`:582`）要求上一 slot 已 Complete。真实路径直到整个 `rk1Series` 返回才调用 `FinishRealTrial`（`:1079`），所以第二跳无法合法 reserve。

此外，`FinishRealTrial` 对每个 measured run 分别 glob，要求恰好一个 launch（`:993`），随后却用外层 run 的 `index` 访问 `launches[index]`（`:1001`、`:1016`）；双启动的第二项会越界。`finish`（`:640`）又只接受最后一个 sequence，不支持整轮后按先后逐项终结。当前第二跳更早被拒绝，不表示后两个分支正确。

这属于**尚未执行的真实验收框架**，不是产品 handoff 回归。按主会话指示不扩展生命周期设计，移交后续验收任务先修复并补 fake 双跳/终止/借记组合验证。不能把旧 `physical_accounting_and_order` 的手动逐 slot 完结测试当作这条真实接线已覆盖。

### RC3 — 归档后路径需要新入口和绑定

Python 的 `HERE.parents[4]`（`:25`）、旧任务路径常量、旧 `run_rk1.py` 引用，以及 Go Real/Controls 的旧 `.trellis/tasks/<runtime>/...` 路径都受目录移动影响。冻结 JSON 同时保存旧绝对路径和字节 hash。

本轮不对历史记录做路径替换、不创建旧路径别名、不重置预算。继任任务必须另建归档感知的入口，按 hash 读取归档后的历史证据，记录新源码/框架/CLI/config/数据库/窗口/permit/gateway 绑定。旧入口与登记封存为准备证据，保持真实开关关闭。

### RC4 — 旧登记不再适用于已修正的 launcher

旧 launcher SHA：`4b3799485dfbe4103b7f44ab130920e0a0a1ad5607a70573c6d09de671ece54e`。
修复后 SHA：`6d6911cefb409baef7c41efa83cb3dcf46cce2f32dda4769dde420f685a809c2`。

旧 registration/sidecar 保持匹配，所有历史五哈希仍匹配。按 loader 的当前合同，旧登记将拒绝新 launcher。没有为了取得绿色 Controls 结果而改冻结登记，也没有执行 DB-backed Controls 的一次已知拒绝运行。

## Verification

- Lint / 静态：四个 owned Go 文件 `gofmt -l` 无输出；`go vet -tags=agentintegration ./cmd/server` exit 0；Python AST 解析通过。
- TypeCheck / 编译：`go test -tags=agentintegration -c` exit 0，仅编译，没有执行 TestMain。工具链为 Go 1.27.0 darwin/arm64。
- 本轮测试：Python mock 回归 5 / 5 pass，红绿日志保留。
- 历史证据核对：19 个 RK1 Go 文件整体 SHA 为 `71f7ca86b99d1e12cc71980d4f5c506e5c6d15b6e56ebab8731ea0d43609bad8`，与冻结登记一致；既有 fake normal/race 日志字节也与记录相符。本轮没有修改 Go，因此未重复 DB-backed fake 全套；这些只作为对应冻结版本的历史证据，**不声称本轮新测通过**。
- 当前 Controls 未执行；完整真实 H6 接线未验证且存在上面已确认的框架缺口。全仓测试不在本次有界复核范围。
- Provider / 模型调用 0，数据库访问 0，permit/window 使用 0；真实 gate 四项全 false，历史 12 / 25 借记与余 13 不变。

精确文件与 SHA 清单见 `runtime-check.json`；冻结完整性见 `runtime-integrity.json`。可提交四个 owned Go 文件与 launcher 的准备实现，提交/结项措辞必须保留“真实续作准备”边界，RC2–RC4 由继任真实验收任务承接。未提交、未归档、未改变任务状态。
