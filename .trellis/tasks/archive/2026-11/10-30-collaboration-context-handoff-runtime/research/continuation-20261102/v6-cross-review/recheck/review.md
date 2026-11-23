# 最终冻结的选定 v6：交叉复核

结论：**pass-with-notes；未关闭的阻塞问题 0**。仅放行已冻结字节上的既定验收脚本合同，不表示完整 `make check`、平台或模型验收通过。

复核人：`/root/rki6_result_review`（原生 `trellis-check`）；2026-11-02。原 changes-required、Make 红色反例与中间版本 `before-hashes.json` 保持原字节，本报告另记最终冻结版本。

## 精确绑定与冻结

| 对象 | SHA-256 / 身份 |
| --- | --- |
| 候选 | `/Volumes/artisan/code/2026/mc-collaboration-final4-20261102`；HEAD `64a9994ec549e8dff73ab14456dc9a9f4b63f7c7`，开头和结束时均干净 |
| 实际 go | `8963ebcc64d89881a6f1966f9ac7a09f1a9121dd32a4f0a9518493ed3b4dd9a9` |
| 实际 pnpm | `7a416eb60f1bdbe6bdf7421d2f61346d74d0e2575ae8a5236e450627db63ff48` |
| 实际 launcher | `3938151d67aa91346aa1898a7be7c561a94d16e86fb8c8d79ab9cb75c71f3e34` |
| 原始 source env | `99475d2e36917761e88bab9687073ddb182c131241d8d815b7eb18850572dfe2` |

实际路径仍为 `collaboration-resume-20261101/adapter-v6/check-bin/{go,pnpm}` 和 final4 的 `run-make-check-v6.py`，不是另一目录的变体。已核对首轮任务 `research/continue-20261102/adapter-fix/final-frozen-hashes.json` 的明确冻结记录。复核前后四项实际哈希完全一致，见 `frozen-before-hashes.json` / `frozen-after-hashes.json`。

## 已关闭的问题

- **AV5-1：关闭。** API/non-test 的 10/1 保持；真正执行测试及递归测试先清除 `DATABASE_MAX_CONNS` / `DATABASE_MIN_CONNS`，不改变默认值断言。
- **AV5-2 与 AV6-CROSS-1：关闭。** 原 source 只读解析为数据；拒绝 raw 命令替换及 normalized `$`、控制/换行字符，拒绝先于安全副本和任何 Make Popen。父写入器的 canonical 文件不能携带 `${shell ...}` 或转义后恢复的 `$(shell ...)`。旧纯反例在最终 helper 上拒绝 brace expression，真实自有 Make include/export 仅收到空配置，退出 0、没有 marker；原红色 marker 证据保留。
- **AV5-N1：关闭。** 启动前实际比较 candidate HEAD、go/pnpm、自身 launcher、原 source config 的字节哈希；missing/blocking review 或任一篡改都在启动前拒绝。
- **私有 HOME/cache：关闭原范围说明中的缺口。** launcher 给子进程建立 task-owned 0700 HOME、显式 `GOCACHE/GOMODCACHE/GOPATH`；mode1 每包创建自己的 0700 HOME，递归 mode0 保留测试自己选择的 HOME 与 cache。移除全局 CONFIG_ROOT/WORKSPACES_ROOT，不覆盖测试自己的后续 `t.Setenv`。
- **递归 package permit / REAL_GO：关闭。** 非空明确 Go/SQL/evidence context 是必要条件；mode0 allocation 要有 canonical package，读取当前选择的 `go list` 结果并与其单一 package 比较。其他目录、包、重复/多包、wildcard、非法/缺失 permit 以及递归 source overlay/modfile 在 test/SQL/exec 前拒绝。
- **compile scanner / global separator：关闭。** 已知 `-bench/-fuzz/-benchtime` 等值标志、`-test.*` alias 和完整已知注册表中的值不会把其值 `-c` 误认为编译选项；未知旗标拒绝。`-args/--args/--` 与 package 后的参数边界保留。全局 `go -- test` 仍进隔离检查；真正最后生效的 `-c` 和 non-test/version 原参数转发仍可用。

## 一次最终重放

原文件按字节复制到本复核的 `contracts/`；只因 `__file__` 改变，其默认临时目录归于本 scope。go/pnpm 链接仍指向实际冻结文件，launcher 合同只在自有副本中替换路径并 mock 整体执行。复制输入与原文件对应 SHA 见 `contract-inputs.json`。

所有重放使用 `/usr/bin/sandbox-exec -f contracts/deny-network.sb /opt/homebrew/opt/python@3.14/bin/python3.14 -B <contract>`：

| 契约 | 实际结果 |
| --- | --- |
| `check-go-isolation-test.py` | 17 pass，0 fail/error，exit 0 |
| `check-go-package-binding-test.py` | 38 pass，0 fail/error，exit 0；exec/SQL/Popen 全 mock |
| `check-go-global-separator-test.py` | 6 pass，0 fail/error，exit 0 |
| `check-launcher-test.py` | 22 pass，0 fail/error，exit 0 |

总计 83 个测试方法，0 失败。结果 JSON、原始日志及命令在本目录。原 Make reproducer 单独做相同问题的绿色核查，exit 0、parser rejected、marker=false，见 `make-loader-green.json`；不计入产品测试数。七份 Python AST 语法检查通过。

## 保留的范围说明

- 该 adapter 是固定 make-check 的私有入口，模式 1 的参数拆分只承诺现有 check 使用的旗标；不是通用 Go 命令行兼容实现。任何未来旗标/overlay 或 scope 改变须另行登记，而不是自动沿用本许可。
- parser 限定于已生成的任务配置：整行注释/空行可跳过，合法 quoted/escaped/empty/duplicate 值有契约；不接受含美元或控制字符的通用 dotenv 内容。canonical 后续 Bash 的字面符号保护保留，不把 shell quoting 当 Make escaping。
- 本复核没有执行实际 launcher、实际 Makefile、数据库/Redis/API/GUI、安装版或 Provider。共享连接余量、实际构建的 CLI/renderer 身份、完整阶段结果与结束源码稳定性仍由唯一全量 owner 验证。
- Lint / TypeCheck：此范围未运行，产品和 TypeScript 未改；这两个门由即将执行的完整候选验收提供。当前 RKI6 的独立复核另见原已完成报告。

本代理仅写本 `recheck/`，没有修改选定脚本/原始环境/共享台账。可将 `clearance.json` 作为这组字节的独立证据引用；真实调用预算、生产发布权限和产品完成状态保持原有门禁。
