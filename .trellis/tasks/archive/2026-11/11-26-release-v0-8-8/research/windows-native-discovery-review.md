# Windows discovery-only 原生发现器独立审查

日期：2026-11-26。**最终PASS：7文件可提交并进入新Windows原生验收，未发现剩余阻断提交问题。** 这是已确认测试环境缺少CLI的准备修复，不改变产品发现要求、权限或执行逻辑；原生完整旅程仍须新run证明。

## 根因证据范围

仅读取脱敏windows-decrypted-root-cause.json：parent已核验run45698773723接收者/AAD，private errlog映射到config.go:262的no_agent_cli_found，原密文/私钥/解密日志没有由本审查读取。静态源码确认LoadConfig发现零CLI且AllowNoAgents=false即退出；Linux容器已有发现用shell，而Windows native没有等价工具。

## 最小发现器与能力限制

新Go文件仅依赖fmt/os/regexp。必须有40hex源码戳；唯一成功argv为exact `--version`，输出2.1.5 (Claude Code)且exit0。空argv、--print含秘密、--version附加值、--fixture-capabilities等所有其他调用写exe旁固定`.unexpected`文本并exit97；写文件失败为98，戳无效为78。无模型调用、网络、任务协议、执行子进程或env/argv日志。没有复用具备实际synthetic任务能力的business/Host provider。

PS在已核验当前checkout的服务准备中用Go原生build .exe，ldflags写当前GITHUB_SHA，并用go version -m验证该戳。plan要求discovery与daemon/provider/CLI四路径大小写归一化后互异且位于任务state内；原driver普通文件/realpath/哈希循环包含新增binary。driver只执行版本检查，不执行模型。

## 环境与标记闭环

独立读agents_probe.go与BuiltinRuntimes：当前25个显式PATH probe+1个OMP descriptor，共26项。环境从这两个真实来源枚举，Claude只指向taskowned发现器，其余25项为phase内missing-providers绝对路径且driver逐项lstat证明ENOENT。Go probe/reresolve遇含路径分隔符的不存在override直接hard-miss，不尝试login shell、Codex bundle、CodeArts fallback或DSH执行；因此PATH中已有真实CLI不会被这些provider选中。PATH本身未改。

显式override位于process.env/setup.environment之后，实际Playwright→native controller→Electron→daemon继承相同限制；独立Host fixture使用自己受控provider，不被替换。参与账号在本旅程没有workspace，启动register只DetectVersion；agent.ListModels仅显式model RPC或任务选择触发，当前完整旅程不使用这两条，因此无需允许非version模型查询。将来若触发，marker应如实失败，不扩大能力迎合测试。

controller verifyDiscoveryOnly在Windows init、metadata/status和close核对exact realpath、SHA与marker不存在；driver开始前和完整journey/cleanup结束后核对marker及binary未变，并记录source+hash。非version调用的marker不会为重试清除；只要出现就不能宣告成功。该发现器与真正执行协作的Host provider独立。

初版把新字段强制于所有native入口会破坏既有Mac调用，本审查提出后已修正：Windows mandatory；Mac只有显式提供新discovery时才校验，未提供时保留原路径；Linux容器原shell/marker不变，未向native env写undefined。对应guard测试保留opt-out Mac行为。

## 独立验证

- 真实Go编译、go buildinfo源码戳、版本成功、四种非version调用exit97且stdout/stderr为空、固定marker恰好4行且无secret、再次version不抹marker均实测；没有用JS假版本替代本机二进制。
- 实际controller helper被VM调用：正确hash无marker通过、marker存在拒绝、binary修改拒绝、Mac非required分支保留。原容器发现器测试也通过。
- 最终相关config/workflow/installed-main/layout组合：45 PASS /0 FAIL /0 SKIP；其中discovery窄选4PASS。node --check、gofmt -l空、git diff --check通过。
- 本机为macOS，测试实际运行的是本机Go二进制；Windows PE及进程继承仍须下一native工作流验证。没有全仓typecheck/重型构建、没有真实provider账号调用，没有读取parent私钥。

## 最终文件摘要

| 文件 | SHA-256 |
| --- | --- |
| `e2e/fixtures/collaboration-discovery-only.go` | `a249278d4f7bbd3f240b9451eb95e85d0ffa7f9f81150f2ef1ff9d5e01060ae2` |
| `apps/desktop/scripts/windows-collaboration-server.ps1` | `9c3d169147c9244739b5bf674563e1f9a1dff58049f4503dc221c3799c297d5e` |
| `apps/desktop/scripts/windows-collaboration-config.mjs` | `e935f5f94f588f9570d94399446e0596d18d24ac76620b6ac9e4bd0468093b47` |
| `apps/desktop/scripts/windows-collaboration-config.node-test.mjs` | `ca826e4635f24272464774eacf123ae1424b6d98ed258189bed74b5a40f68bcd` |
| `apps/desktop/scripts/windows-collaboration.mjs` | `6a1e95c19d10d1134641989d7124919e68b859cc26bf4df6d4728dee574d2908` |
| `e2e/fixtures/collaboration-installed-main.mjs` | `5df3d6f76c0e13e1201116489e65314cda1de48cf49e231f6b83d5615df343cb` |
| `e2e/fixtures/collaboration-installed-main.node-test.mjs` | `c02c5db198c1a50228bfdc60a793c802181083ce6b044f97129edf3968b46180` |

仅新增本审查报告，未修改实现。正式版本不随包提供该发现器，它只服务受控验收。
