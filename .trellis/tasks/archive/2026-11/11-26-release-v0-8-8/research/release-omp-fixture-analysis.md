# Release OMP 测试夹具失败分析

日期：2026-11-26。只读源码诊断与私有实验，未修改冻结仓库、产品实现或 v0.8.8 标签。

## 结论

Release CI 45721755253 的 `TestOmpExecuteDefaultsToOmpBinary` 暴露了一个确定存在的测试夹具缺陷：测试将 PATH 替换为只包含自建 `omp` 的目录，但 shell 脚本用外部 `cat` 消费 stdin。该目录没有 `cat`；shell 打印错误后继续输出模拟成功事件并以 0 退出。若退出早于后台 prompt writer 完成，产品正确地将 EPIPE 记为失败。

这不是已经证明的产品执行故障，也不能通过忽略 `pi.go` 的 prompt write error 修复。保留同一不可变 SHA 原样重跑 CI 在证据上合理，但可能再次遭遇这个竞态；本机重复通过不是缺陷消失的证明。重跑成功时仍应记录“已知测试夹具缺陷，后续提交修复”，不能宣称零已知测试缺陷。若选择修复源码，须新提交/新版本流程，不得移动 v0.8.8。

## 冻结来源

- HEAD 与 `v0.8.8^{commit}` 均为 `d2c9240eaf9f768446faba93cac625bd63e9cf53`。
- `omp_test.go`、`pi.go`、go.mod/go.sum 和 agent CLI guard 的工作区字节与该提交 Git 对象逐项相同，实验后重新核对未变。
- CI 原失败日志显示约 1ms 的执行后，结果为 failed，错误为 `omp prompt write failed: write |1: broken pipe`。未读取业务凭据或调用已安装的真实 agent CLI。

## 因果证据

1. `omp_test.go` 的脚本显式执行 `cat > /dev/null`，随后两条 `printf` 并 `exit 0`；测试随后 `t.Setenv("PATH", fakeDir)`。
2. 实际 `/bin/sh` 在仅含假 CLI 的 PATH 下执行相同结构，stderr 确认找不到 cat，仍返回 0。让父进程在已观察到子进程退出后写入原小 prompt，真实写入得到 errno 32 / EPIPE。这是确定的因果控制，不冒称原 CI 使用了人为延迟。
3. 私有对照仅将消费行改为 shell 内置 `while IFS= read -r line || [ -n "$line" ]; do :; done`。同样 PATH 下，子进程在 stdin 写入前仍等待，写入及 EOF 后正常退出。
4. 原测试只写短 prompt，调度可能让写入先完成，所以缺陷可出现也可通过。产品并发消费 stdout 和写 stdin 是正常设计；原 `pi_test.go` 的 fixture 注释也明确要求 fake 等待 stdin EOF，避免虚假的 prompt-write 失败。

## 精确测试与对照

所有 Go 命令均使用 `GOTOOLCHAIN=go1.26.11`，通过 `scripts/go-test-with-agent-cli-guard.sh` 执行，平台为 **macOS darwin/arm64**。pkg/agent 的 TestMain 不依赖数据库；没有启动全库测试或修改任何测试预算。

| 输入 | 参数 | 实际具名结果 |
| --- | --- | --- |
| 冻结原测试 | 普通，`-count=100` | 100 PASS / 0 FAIL / 0 SKIP |
| 冻结原测试 | `-race -count=100` | 100 PASS / 0 FAIL / 0 SKIP |
| 私有 builtin 读取对照 | 普通，`-count=100` | 100 PASS / 0 FAIL / 0 SKIP |
| 同一私有对照 | `-race -count=100` | 100 PASS / 0 FAIL / 0 SKIP |
| prompt 写入失败负例 | 原不消费脚本，大 prompt，`-count=20` | 20 PASS，断言结果必须 failed 且包含 prompt write failed |

原样调用示例：

```bash
GOTOOLCHAIN=go1.26.11 bash scripts/go-test-with-agent-cli-guard.sh -- \
  go -C server test -race -json ./pkg/agent \
  -run '^TestOmpExecuteDefaultsToOmpBinary$' -count=100 -timeout=3m
```

私有对照通过 Go `-overlay` 仅映射到私有 `omp_test.go` 副本，原产品 `pi.go` 不变。builtin 对照只改一行 shell 消费语句；负例只改 prompt 大小及该结果断言，证明产品仍拒绝写入失败，不把模拟成功事件当成任务完成。负例中原 Fatal 文案仍写 expected completed，但实际检查条件已明确要求 failed + prompt write failed；此私有实验不作为待提交测试。

本机 200 次原样通过不能替代 Linux Release CI 的实际重跑，也未统计跨平台失败概率。普通/race 四组都真正运行 100 次具名测试，非编译成功或静默 TestMain 退出。

## 证据位置与复核

私有证据目录：`/Volumes/artisan/code/2026/multica-0.4.37/.artifacts/release-v0.8.8/omp-fixture/`。

- `source-binding.json`：固定 SHA 与五个原文件摘要。
- `shell-proof.py` / `shell-proof.json`：真实 shell、等待退出、EPIPE 与 builtin 读取对照。
- `original-normal.*` / `original-race.*`：原样测试完整 JSONL、stderr、退出及具名计数。
- `builtin-*`：仅私有一行修正的副本、overlay、普通/race 证据。
- `write-failure-negative-*`：负例副本、overlay、20 次真实拒绝写入失败的结果。
- `run-repeats.py`、`summary.json`、`evidence-manifest.json`：精确命令、平台、退出码、计数及全部证据文件长度/SHA-256。

本代理未重新触发 CI、未 commit/push、未移动标签。CI 是否原样重跑及后续版本修复由发布主会话负责；不把私有 overlay 生成物混入正式构件。
