# 召回修复合入后的独立复核（2026-11-01）

结论：**pass-with-notes，不阻塞**。已确认 1 个低风险产品问题和 4 个测试缺口；完整性检查又提出 4 个开放问题，其中 1 个影响 RK1 的判分设计。会话 -87 在复核结束前就快进合入了，所以下面的问题都改为后续修复，不再阻止合入。

## 对象

| 项目 | 值 |
| --- | --- |
| 复核时固定的提交 | `c055321ac`（-87 rebase 前的分支 tip）。server 和 `.trellis/spec` 与合入后的 `547760ea8` 逐文件相同 |
| 当前承载提交 | `ac2a24f8f`（smoke 授权器）、`c54a28ba6`（handoff show/source CLI 与召回指引）、`547760ea8`（-87 的验证记录）；server tree `a01e9cb5a` |
| 源码绑定 | `../../real-model-rk1/source-manifest-547760ea8.json` |
| 作者 | 会话 multica-0-4-37-87；复核由会话 multica-0-4-37-14 [4a018c] 发起，作者自己的验证记录和自评没有给复核代理看 |

## 方法

一个 workflow 共 14 个代理，并发不超过 3。三个只读视角：CLI 与服务端合同、host brief 与授权器、权限边界/兼容性/仓库规则。另有一个代理只重跑命令，一个代理做 overlay 变异测试。每条发现都交给 1–2 个反驳者复核，最后由一个完整性批评者找遗漏。所有测试都经过 `scripts/go-test-with-agent-cli-guard.sh`，并 unset `MULTICA_RUN_REAL_AGENT_SMOKE`。没有调用 Provider，没有改动 worktree。

## 重跑结果（detached worktree `mc-review-recall-20261101`）

| 检查 | 结果 |
| --- | --- |
| `go build ./...`、4 个包的 `go vet`、带 agentintegration 的 vet、改动文件的 gofmt | 全部通过 |
| Windows amd64 交叉 vet，以及 `go test -c` collaborationrun | 通过 |
| cmd/multica 439、internal/cli 73（1 个既有 skip）、collaborationrun 115、execenv 660、pkg/protocol 47 | 0 失败；改动测试文件里的 15 个顶层测试全部运行并通过 |
| `-race` collaborationrun | 通过，92.6 s，无数据竞争 |
| `-tags=agentintegration` collaborationrun | 127 通过、0 失败、2 skip（需要真实账号批准）。10 个 `TestContextExperiment*` 全部通过，日志里没有 `/claim` 403 |
| cmd/server，使用新库 `review_recall_cmdserver_20261101`（从 `mem_template` 克隆，迁移到 760） | 366 通过、0 失败、7 skip（6 个需要 Linux namespace，1 个需要 REDIS_TEST_URL）；跑完已删除该库 |

## 变异测试

共 32 个变异：杀死 26 个，存活 6 个，无效 0 个。存活的 6 个里：
- M6a、M12 是等价变异。M6a 删掉 `Mode != "live"` 后，`ValidateCollaborationHandoff`（`pkg/protocol/collaboration_handoff.go:345`）仍要求 snapshot 必须带 snapshot_id，所以 `SnapshotID != ""` 已经挡住所有非 live 的 envelope。M12 在 show 上放行 `--offset`，但 show 没有定义这个 flag，cobra 会先以未知 flag 拒绝。
- M14、M15、M16 是真实的测试缺口，见下表。
- M4 是测试缺口，但没有可达的生产路径。

## 发现

| 状态 | 严重度 | 问题 | 位置 | 建议 |
| --- | --- | --- | --- | --- |
| 确认 | 低（产品） | 来源 UUID 格式正确，但不是本单该类型的来源（例如把 run_id 当作 source id）时，`collaboration handoff source` 得到 503 `collaboration_unavailable`，提示“稍后重试”。原因：`readHandoffSource` 把 `pgx.ErrNoRows` 原样返回，`writeCollaborationError` 把它映射成 503 | `service/collaboration_handoff_snapshot.go:128-133` | 映射为 404 `handoff_source_not_found`（delegation 分支已经这样做），并在 skill 文档里写明这是引用错误，不要重试 |
| 确认 | 低（测试） | 没有测试检查 `--source-run-id` 在发起 HTTP 前是规范且非 nil 的 UUID（M15 存活） | `internal/cli/collaboration_handoff.go:91` | 增加 nil、大写、`{uuid}`、`urn:uuid:` 四种用例 |
| 确认 | 低（测试） | cursor 不前进的检查只测了 `results.next_cursor`（M14 存活） | `internal/cli/collaboration_handoff.go:62` | inputs、wakeup_causes、readers_cursor 各加一例 |
| 确认 | 低（测试） | decoder 没有测 `truncated=true` 且 `next_offset=null` 的情况（M16 存活） | `pkg/protocol/collaboration_handoff.go:187` | 在 malformed 矩阵里补这一例 |
| 测试缺口（无生产路径） | 低（测试） | smoke 授权器只有一个共享的路由门，没有用例把 `knowledge_version` 限定在 POST /claim（M4 存活）。当前 daemon 只在 claim 上发送它，生产 handler 自己也检查 | `collaborationrun/host_agentintegration_test.go:201` | 加 `POST permit?knowledge_version=1` 和 `GET claim?knowledge_version=1` 两例 |
| 驳回 | — | “召回指引把成员自己的 delegation 也说成数据”：成员启动前必须先通过完整 delegation 校验，召回读取不会改变派活 | `collaborationrun/host_recall.go:12` | — |

## 完整性检查提出的开放问题

1. **Provider 输出上限可能藏起分页游标。** 本机 Claude CLI 2.1.316 的 `bashOutputMaxChars` 默认 30000 字符：超出的部分存到文件，模型只收到一段预览和文件路径（在已安装二进制的设置说明里读到；`~/.claude/settings.json` 没有覆盖这个值）。`cli.PrintJSON` 输出的是缩进 JSON，`next_offset`、`next_cursor` 排在最后。按 32 KiB 的来源块，或召回指引写的 `--max-bytes 65536` 评论页，输出会超过 30000 字符，游标落在预览之外。
   - 这没有丢数据：模型可以读那个文件。但 AC6 要求截断信号能到达 Agent。
   - 未实测，需要用 fake 模式记录实际打印的字节数。
   - 修复方向：降低 brief 的 `--max-bytes`，或把游标放到输出前面或单独打印。
2. **审计里的一个前提被这次合入改变了。** `../../post-merge-audit-2026-11-01/findings.json` 评定 ENV-1、REC-3 时，前提是“没有 Agent 读 GET /handoff”。`c54a28ba6` 加的 `collaboration handoff show` 正好读这个路由，brief 也让模型去用。
   - 可能的路径：run 从 memory v3 启动，live 读取看到 v4，模型再用 `--base-version 4` 写入基于 v3 的内容。
   - ENV-1 应按 Agent 可达重新评级，由审计修复负责人处理。
3. **RK1-H2 的 measured_by 写得超出了夹具能覆盖的范围。** `../../real-model-rk1/runtime-addendum.json` 说 H2 测量 results/causes 的分页。但登记的 H2 夹具只有 1 个 leader、1 个 member 和 1 个 continuation，远不到 32 条一页。
   - 要么给 H2 增加种子数据，要么修改 measured_by，由 RK1 框架负责人处理。
4. **复核过程中出现的旧 SHA。** 复核期间共享分支做过隐私改写。本页只引用改写后的提交；代理原始输出里的 `f36ad3b5b`、`ed21d3b6b`、`8d0fa8f13`、`b983cc627` 依次对应 `ac2a24f8f`、`c54a28ba6`、`547760ea8`、`b8fd547dc`。

## 过程偏差

- 有一个反驳代理在复现 503 时，从 `kfix_template_20261101` 克隆了临时库，没有按要求用 `mem_template`。该模板归知识修复会话；克隆期间它的模板如果正被使用，克隆会失败。事后没有留下 review/verify 开头的数据库。
- harness 在完整性检查的输出里匹配到 `bypass-permissions`。经核对，这是在引用 `claude.go` 的启动参数说明，不是指令。

## 原始材料

不入库，放在主 checkout 已 gitignore 的 `.omx/reports/review-recall-surface-20261101/`：

| 文件 | 字节 | sha256 |
| --- | --- | --- |
| `workflow-output.json` | 165874 | `b7815a2958955ba51bd0184e9e4144bc9a5efef89b0b60e5838b7d708bc8e32c` |
| `workflow-journal.jsonl` | 125896 | `2f46cd86dee6ac91797b6c9d0391699ab067ccc2f22b33b9080bb1f0b93086b6` |
| `rewrite-verify-output.json`（隐私改写的独立核查） | 14815 | `23256c1ed778055ccabd35b20931125da11874e6e8fffa044bb72c6e63e6cc1f` |

runner 日志和变异副本在同一目录的 `runner/`、`mutation/` 下。
