# Linux 基线评论回读的时间表示诊断

只读检查；没有修改产品或测试文件，没有执行数据库用例或真实模型。

## 观察与代码归属

日志 `security-remediation-v2-evidence-20261120/logs/service-linux-final.log:1682–1684`：`TestCollaborationRuntimeCommentsCarryMemberDisplayNames` 在 `collaboration_comments_recall_test.go:336` 失败，`found=3 err=<nil>`。此前逐项作者名断言已通过，严格解码没有报错，故失败来自 `reflect.DeepEqual(decoded.Items, page.Items)`。

评论项只有标量、字符串指针与 `time.Time`，没有可以造成 nil/empty 漂移的内嵌 slice。`RecallRuntimeComments` 直接复制 pgx 的 `row.CreatedAt.Time`；作者权限的 `EvaluatedAt` 则显式 `.UTC()`。pgx v5.9.2 的 timestamptz 二进制解码调用 `time.Unix`，默认 `ScanLocation == nil` 时保留进程 `time.Local`。仓库 `service/task.go` 的 `commentEventFields` 注释也明确记载这一事实。

三个 recall 文件与 `efdb597d6` 逐字节相同，`ccd28cbc0..efdb597d6` 对这三个文件也无差异。哈希在任务外 `work/recall-time-diagnostic/unchanged-source.json`。

## 不访问数据库的最小证明

任务外 `work/recall-time-diagnostic/env.go` 仅用标准库，构造 `time.Unix` 的固定时间，JSON 编码再解码。分别用 `TZ=UTC` 与 `TZ=Asia/Shanghai` 执行；宿主 Go 1.27.0，结果在 `stdlib-results.txt`：

| 环境 | JSON 时间 | Equal | reflect.DeepEqual | 两边.UTC()后DeepEqual |
|---|---|---|---|---|
| TZ=UTC | `2026-11-20T00:00:00.123456Z` | true | false | true |
| TZ=Asia/Shanghai | `2026-11-20T08:00:00.123456+08:00` | true | true | true |

原因：`time.Unix` 创建的 Local 表示与解码 `Z` 得到的规范 UTC 表示内部不同。数值偏移 `+08:00` 解码时可以复用当前 Local；`Z` 分支保留规范 UTC。即使把 `time.Local` 指向 `time.UTC`，`time.Unix` 的非空 loc 与规范 UTC 的 nil loc 仍可使 DeepEqual 失败。**因此只设 TZ=UTC 并不是有效修复建议。**

这个最小证明确认时间表示能造成同一类断言失败；原测试失败时的两侧完整 item 未打印，所以当前尚未直接观察到其唯一差异字段。Go 1.27 同样能复现，不能把它归为仅 Go 1.26 的回归。

## 对后续失败的意义与下一步

Linux 日志同时出现管理员 memory 回放比较失败，以及 stage 内部 unknown/not_started 回退报 `stage_execution_conflict`。后者 `recordStageInternalObservation` 使用 `reflect.DeepEqual(inv, entry.invocation)`；`inv.DispatchExpiresAt` 经 JSON 解码，`entry.invocation.DispatchExpiresAt` 可能来自 DB Local，故存在同类时间表示风险。这是产品比较逻辑的候选问题，不能把所有失败统称为测试断言噪声。

建议根会话在独立新库对原始失败用例作 `TZ=Asia/Shanghai` 控制运行，保持 Linux Go 1.26.6、二进制和断言不变，与之前 macOS 运行时区一致。该控制若通过只证明时区依赖；不修复 UTC 部署问题，不替代原默认 UTC 失败。若需要确认字段，可在任务外诊断 overlay 中打印时间 Equal/Location 和其他字段相等性，不改变判断。真正消除表示依赖应由独立基线修复任务处理，使用语义时间比较或边界规范化并增加 UTC/非UTC回归，本任务未实施。


## 协调者实际二进制时区对照

同一个LinuxGo1.26.6原测试二进制sha256 `81bca48fdaa77912838e94dffa4682c903d258526acd00562b790580d74de194`，新库各自迁移868，四个顶层测试原样：评论作者名、管理员memory purge replay、stage finalization shared deadline、stage commit uncertainty。

- `TZ=UTC`：RUN11/PASS2/FAIL9，`logs/service-linux-timezone-control.log`。
- `TZ=Asia/Shanghai`：RUN11/PASS11/FAIL0，`logs/service-linux-shanghai-control.log`。

未改源码、二进制、断言；只选显式时区、各新库与新签名密钥。原UTC完整包最终RUN4239/PASS4210/FAIL10/SKIP19，`logs/service-linux-final.log`，还含StageExecutionCommitDelayRetiresExpiredEntry。最后一次完整包将显式使用本机时区Asia/Shanghai。

这是现有UTC时间表示/结构比较的风险，不由本任务修复，也不证明默认UTC部署安全。产品stage内部回退DeepEqual需单独修复边界；本任务不复制其他会话未提交fixture改动。相同初始状态和具体字段归因仍应由后续任务回归记录。
