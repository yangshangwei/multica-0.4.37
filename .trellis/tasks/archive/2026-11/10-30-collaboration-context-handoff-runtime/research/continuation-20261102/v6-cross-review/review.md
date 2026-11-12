# 实际选定 v6 全量验收脚本交叉复核

结论：**changes-required，阻塞项 1；不签发启动许可。** 本报告针对选定的 `collaboration-resume-20261101/adapter-v6` 和 final4 的 `run-make-check-v6.py`，不沿用另一个 `resume-20261102` 变体的复核结论。

## 未修复的阻塞问题

**AV6-CROSS-1（中）：canonical 环境文件仍被 Make 当代码解释。**

- 选定 launcher 的 `parse_generated_environment` 只拒绝 `$(` 和反引号；接受单引号包装的 `${shell ...}`。
- writer 使用 `shlex.quote` 生成 Bash 安全的字符串，但 `make check ENV_FILE=<canonical.env>` 会使根 `Makefile` 第 8 行 `include $(ENV_FILE)` 读取它，第 29 行 `export` 又将这些 Make 变量展开给子进程。Make 的单引号不是转义机制，`${shell ...}` 是合法函数表达式。
- 在完全自有的无凭据配置和小 Makefile 上，保留相同的 `include` / `export` 并执行 `/usr/bin/true`，成功创建了一个自有临时标记，退出码 0。实际原环境、选定 launcher、业务 Makefile、数据库均未执行。证据：`make-loader-probe.json`。
- 现有 15 项 launcher 测试替换了 make 执行，剩下的 canonical 测试仅经 Bash 读取，因此这些测试全部通过也未覆盖本边界。
- 必须在进入任何 Make include 前拒绝其不支持的字面表达式，或明确分离 Make 与 Bash 的配置输入；然后以真实微型 Make include/export 增加 `$()`、`${}`、单引号、转义和字面美元的测试。修复不得运行原环境中的表达式或改业务 Makefile。

此处是已复现的 parser / 传递合同问题，不宣称实际固定环境中存在恶意内容。当前环境已绑定原始哈希，不据此诊断产品故障。

## 已确认关闭与保留的说明

| 原问题 | 选定 v6 结果 |
| --- | --- |
| AV5-1：API10/1 污染 Go 默认值测试 | 已关闭。API/non-test 保留 10/1；真正 `go test` 分支和递归分支都先移除两键。14 个 Go/Playwright 替身契约全部通过 |
| AV5-2：原始 dotenv 经 shell 执行 | 原始文件读取已改为 data parsing，Bash 替换/重定向测试通过；因 AV6-CROSS-1，端到端 data-only 合同仍未关闭 |
| AV5-N1：clearance 未核对当前 bytes | 已关闭。启动前实际比较候选 HEAD、go/pnpm bytes、launcher 自身 bytes、原 source config bytes；篡改与 missing/blocking review 测试均在 Popen 前拒绝 |
| 私有 HOME/cache / 配置根 | 保留说明。选定 launcher 不设置私有 package HOME/GOCACHE/GOMODCACHE/GOPATH，实际 source 也没有这四键，故使用 caller 值。launcher 会清除所有继承的 `MULTICA_TASK_*`，再只加入必要隔离键，没有强加 CONFIG_ROOT/WORKSPACES_ROOT，测试自己的 HOME 仍有效。此复核不证明 caller 的 account/home 隔离；如全量 owner 需要此隔离，应在执行上下文中登记私有 HOME 及显式 cache/GOPATH，不能使用全局 task-config 根替代 |

## 其他核查

- 真正 compile-only `go test -c` 和 non-test 命令直接转发；`-args -c`、`-run -c` 和 `-c=false` 不能冒充 compile-only 绕过 TestMain 的隔离。
- mode1 只接受记录中的本机 `multica_check_*_go` 模板，验证实际模板名，再逐包建立唯一 clone、记录具名测试与退出码；递归 mode0 要有匹配 `ccheck_*` 的 allocation record，未知/远程/默认库均拒绝。此轮全部是 fake psql / fake go，无实际 SQL。
- launcher 要求 clean candidate、已有独立复核、真实 hashes 相等，拒绝重复 record/log/safe-source；安全副本以 O_EXCL、0600 创建；检查临时目录归属/0700/非 Git。
- renderer URL 由两条已审环境键一致取得，要求无凭据的 loopback HTTP(S)，而非误用旧 19844；完整环境及配置值未打印。实际源配置里知识开关关闭，外部模型设置为空。
- 脚本保留固定 make/check 参数、Go 的 race/-p/-parallel/timeout，pnpm 只补缺失的 JSON reporter 和经过 guard 的已分配 API/renderer。
- 同源 CLI/renderer 的实际构建/监听证明、共享连接余量、最终源码前后稳定性仍是单一全量 owner 的运行义务。现有生产 API/Web source/config/进程检查提供启动边界；本报告没有接触服务、Redis 或 GUI，也没有把文件路径视作平台通过。

## 复核时实际哈希

- go：`b8c5d5adbece4f781a00a230b4f6ffbbfed195f4e44826d53997d72b3f160079`
- pnpm：`7a416eb60f1bdbe6bdf7421d2f61346d74d0e2575ae8a5236e450627db63ff48`
- launcher：`7e2592d1b6e0a795ce9078f4c97d04255f635d392ff7e16a154309aa87695516`
- Go contracts：`ddeb1c96fbefa7aaa4f88b8b0478f313eda4abc4eb26eff369c2b5e284aed66a`
- launcher contracts：`4fc48dc97ca194c7cb4df1eead377b2fd134054b7c4e43e9d1cb58c75390f5c6`
- 原 source env：`99475d2e36917761e88bab9687073ddb182c131241d8d815b7eb18850572dfe2`

## Verification

- `python3 .../adapter-v6/check-go-isolation-test.py`：14 pass、0 fail，exit 0。
- `python3 .../adapter-v6/check-launcher-test.py`：15 pass、0 fail，exit 0。
- 微型 Make 反例：已确认 marker 被执行，exit 0；不是完整验收通过。
- Lint/TypeCheck：未运行，产品/TypeScript 未修改，完整验收仍由 sole owner 执行。
- 本代理仅写入分配的 `v6-cross-review/` 报告与证据；选定脚本/配置/共享台账未修改。新 SHA 必须另行复核，旧 SHA 上的结果不能作为修复后 clearance。
