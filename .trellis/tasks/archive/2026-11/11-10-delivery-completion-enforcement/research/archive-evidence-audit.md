# Research: 已归档 M1 增量与原始证据核对

- Query: 核对最新完成提示包及前序政策、证据观察、持久委派包的已交付范围、冻结源码、验收记录、失败保留、集成和剩余合同，避免重复实施或错误关闭父任务。
- Scope: internal；只读历史资料、源码、原始日志和容器状态。未运行测试、数据库命令、供应商调用或 Git 操作；仅写本研究的 Markdown/JSON。
- Date: 2026-11-10
- Baseline: `a37bee7cd1c05122383a6c40e157ef9ed5890bf4`，新 worktree `mc-delivery-completion-enforcement-20261110`。

## Findings

### 已核实结论

最新完成提示包已实施、独审、归档和本地集成。前序证据观察、持久委派及项目政策/持久评估也已交付，不能沿用早期文档中的“待实施”描述。完整完成门禁、skip、自动 UI/CLI 提示及 M2/M3 仍开放。本次证据复核未发现缺失日志、哈希漂移、计数矛盾或未解释的源码差异。

主会话执行了 Git 核对：用户列出的 18 个实现、归档、交接和集成提交全部为当前 HEAD 祖先；通过 `git show HEAD:<path>` 重新计算最新提示包 13 个路径的 SHA-256，全部匹配。研究角色未自行运行 Git。主会话产物分别为 `.omx/private/enforcement-intake/commit-ancestry.json` 和本任务 `research/prior-head-verification.json`；其完整内容和文件哈希已纳入本研究 JSON。

### 最新完成提示包

完整读取 `archive/2026-11/11-10-delivery-completion-hints/` 的 handoff、delivery、PRD、design、implement、progress、task、verification，以及 research 下的独审 Markdown/JSON、源码清单、测试库存、失败说明、本地集成与清理记录。JSON 均完整解析，未只读取概述字段。

- `task.json` 为 `completed`，实现 `ca0c8efd3`、归档 `d3b0b17db`、交接 `609219ed3`、集成证据 `a37bee7cd`。子任务明确记录 `full_m1_complete=false` 和 `full_delivery_gate_complete=false`。
- 冻结 13 文件：9 个变更的产品/测试/spec/skill 路径，4 个复用核心路径。新 worktree 和原来源 worktree 均 13/13 匹配。主工作区只有 `.trellis/spec/server/index.md` 与冻结值不同，符合已保留的用户额外条目；产品、测试、技能、复用 helper 均匹配，不应清理该索引。
- 29 份原始运行日志全部存在，SHA-256、记录摘要、独审绑定、具名终态与运行库存均匹配。其中 9 份最终检查、4 份失败；每个具名 `run` 都恰有一个终态。所有最终检查始于冻结时间之后。
- 源码函数名、命令选择表达式、原始日志与 `test-inventory.json` 完整相符：handler 每轮 76 顶层、325 具名；真实认证路由每轮 37 顶层、103 具名；合同 24 顶层、50 具名。
- 四个最终数据库包/普通与 race 轮次各使用不同数据库。历史清理记录报告 0 连接后停止容器、保留 10 库和日志；本次只读 `docker inspect` 复核 `multica-completion-hints-20261110-pg` 仍为 `exited`、`running=false`。未重启或连接数据库，因此未重新枚举已停止数据库中的内容。

| 最新最终范围 | 普通 | race | 具名拆分 |
| --- | ---: | ---: | --- |
| Handler | 325 | 325 | 89 提示 + 236 原合同 |
| 真实认证路由 | 103 | 103 | 23 提示 + 80 原合同 |
| 合同 | 50 | 未独立声明该轮 race | 21 技能 + 16 其他 service Resolve + 11 状态 + 1 迁移静态 + 1 runtime 扫描 |

普通与 race、迭代与最终轮次均不得相加为独立覆盖。额外 27 项不是 27 个状态测试。全后端 build/vet、sqlc、格式检查记录为 exit 0；本次仅复核这些历史证据，没有声称重新执行。

四份失败原样保留并检查其错误文本：`router-iter01` 的状态键长度约束、`router-iter02` 的编号计数器夹具、`handler-nul-red01` 的预期 400 实际 503、`backend-format02` 的临时括号格式错误。只有 NUL 是实际产品行为 red；其父测试和子测试两个 fail 不代表两个独立缺陷。原日志和记录没有改写。

### 前序三包

完整读取三包 delivery、design、verification 和独审 Markdown/JSON，并逐项解析其运行记录与日志。

| 增量 | 实现 | 原始运行记录 | 最终检查 | 保留非零运行 | 最终具名结论 |
| --- | --- | ---: | ---: | ---: | --- |
| 项目政策与持久评估 | `4ec66e93c` | 34 | 10 | 6 | handler 236、router 80、迁移/注册 37，各普通/race；附加合同 23 |
| 交付证据观察 | `45af89016` | 32 | 8 | 5 | handler 140、router 34，各普通/race；技能 21、runtime 1 |
| 持久委派回执 | `f4782b863` | 74 | 13 | 15 | handler 342、router 20、迁移 5，各普通/race；CLI 98、guidance 33、静态迁移 1、runtime 1 |

四包合计检查了 169 份运行记录及其原始日志。这是档案完整性检查数量，不是新执行的测试数量。每份日志的存在、哈希、具名终态计数和记录库存一致；独审中的最终/失败绑定也相符。三包原来源 worktree 分别匹配 26、6、36 份冻结源码。它们与当前源码的差异都能绑定到后续已验收包的源码清单，包括 router、skill/source-map、观察器、迁移注册和 generated models；没有发现无法解释的漂移。

历史证据包仍写“政策、持久委派待实施”，持久委派包仍写“政策/持久评估待实施”，这些都是当时的剩余范围，已由后续包覆盖。其“完整门禁尚未实施”仍适用于当前 HEAD。

### 日志与业务范围

已读 `.trellis/workspace/artisan/journal-3.md` Sessions 149、150、151，并逐段比较新 worktree 与主工作区：三段均完全一致。新 worktree 起始行分别 902、924、950；主工作区分别 1008、1030、1056，差异来自保留的既有日志，不能用行号偏移推断历史丢失。

当前已交付：受控 local-synthetic tar、stateless 证据观察、显式目标持久委派、off/observe 政策修订、人类私有持久评估、显式普通任务完成提示 API。仍待产品决定的合同包括跨主体证据消费/最终验收责任、skip、创建即完成、已 done 编辑/换项目、协作源项目与精确 submission/R38 关系，以及旧客户端和批量逐项失败协议。owner/admin 管理项目政策已决定，不应重复问。

M1 不能因这些子任务归档而完成。M2 仍按确定性报告快照 → 父级总预算 → 能力/负载调度；M3 保留持续 NFR、模型评测、供应链、模板/知识治理。AC43、E5、H3a、知识真实验收及停止/预算规则未因本次核对变化。

## Files Found

| 路径 | 用途 |
| --- | --- |
| `.trellis/tasks/archive/2026-11/11-10-delivery-completion-hints/handoff.md` | 最新实现/归档/集成和下一包边界 |
| 同目录 `delivery.md`、`verification-2026-11-10.json` | 最终计数、失败分类、范围和运行记录入口 |
| 同目录 `research/final-source-manifest.json`、`test-inventory.json` | 13 文件 SHA-256 与源码/执行库存 |
| 同目录 `research/independent-review.md/json` | 独审 PASS 及每个最终/失败记录的哈希绑定 |
| 同目录 `research/local-integration.json`、`cleanup.json` | 1,048 非重叠路径保留、4 文档合并及历史环境停止留证 |
| `archive/2026-11/11-10-delivery-policy-evaluations/` | 政策修订、私有人类评估及迁移 776–780 |
| `archive/2026-11/11-10-delivery-evidence-gate/` | 固定观察器及当前权限/数据库 PR 事实边界 |
| `archive/2026-11/11-10-durable-delegation-receipts/` | 显式目标、原 task 绑定、unknown 不自动重派及迁移 771–775 |
| `.trellis/workspace/artisan/journal-3.md` | Sessions 149–151 的增量和仍开放范围 |
| `research/archive-evidence-audit.json` | 本次逐文件/运行审计结果、来源与限定，0 个未解释异常 |

## Code Patterns

- `server/cmd/server/delivery_completion_hint_integration_test.go:48`：快照覆盖 delivery、execution、approval、evaluation、task、comment 和 policy；断言额外业务写入为零。
- 同文件 `:63`：真实 JWT/password fixture，持久 evaluator/key/evaluation/producer/receipt 身份逐项比对，重建 router 后使用新合法凭据恢复相同原观察。
- 同文件 `:335`：先调整合成夹具冻结窗口，再经过真实审批与打包，等待实际 DB 时钟到期；比较原观察、制品字节与全部业务快照，未通过重放续期。
- 同文件 `:433`：明确验证无 evaluation 仍可普通/custom done、批量 done、创建 custom done，并读取实际持久状态。它是兼容性证据，直接说明该包没有强制门禁。
- 同文件 `:472`：独立协作在普通 hint POST 返回 `409 unsupported_collaboration_action`，无原观察泄漏或业务写入。
- `server/internal/handler/delivery_completion_hint_test.go:60`：原观察/账本快照不变且 packager 调用为零，复用旧恢复合同。

## Related Specs

- `.trellis/spec/server/delivery-completion-hints.md`：显式提示是非授权基础能力；未来所有实际 writer 事务仍须复核。
- `.trellis/spec/server/delivery-policy-evaluations.md`：owner/admin 政策管理、原人类 namespace、完整当前权限恢复、历史保留及 `usable_for_completion=false`。
- `.trellis/spec/server/delivery-evidence-observation.md`：数据库已观察 PR head 不是远程实时 HEAD；unknown、期限和当前原主体权限。
- `.trellis/spec/server/controlled-artifact-execution.md`：审批前冻结、一次 claim、未知不重跑、确定性 tar 和删除后留证。
- `.trellis/spec/server/durable-delegation-receipts.md`、`squad-delegation-receipts.md`：queued/recorded 仅表示受理/保存，不代表模型执行或交付完成。

## External References

无外部网络引用或新文档研究。本次依据仓库、私有原始日志和主会话 Git 证据；历史环境记录为 Go 1.27 darwin/arm64 与 PostgreSQL 17 amd64，仅支持既有功能验收结论。

## Caveats / Not Found

- 研究角色未执行 Git；祖先关系和 HEAD blob 核对明确归属主会话，结果已保存在本任务。其余归档文件、源码工作区字节、运行日志和库存由本角色直接重算。
- 本次未重跑历史测试，也未新做全仓、浏览器、性能、部署或供应商验收。不得把归档重验称为新门禁验收通过。
- 只读 Docker 状态不证明已停止数据库中每一行仍存在；数据库数量与零连接依据历史清理记录，未为了核对而重启环境。
- 原始日志在本机私有临时目录，本次均存在；提交中的相对记录与绝对日志路径并不等于远端也保存了日志。
- 本研究没有解决或批准待决业务语义，不授权 skip、证据共享、机器消费或停止规则例外。用户未回答的推荐仍是推荐。
