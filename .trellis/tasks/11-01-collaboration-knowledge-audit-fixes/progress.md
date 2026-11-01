# 知识交付审计缺陷修复：进度

核对时间：2026-11-01 19:53 +0800。依据是各分道分支的实际提交、`research/evidence/` 文件和 workflow 日志，不依据会话汇报。

## 负责人变化

- 会话 -14 [2a1270] 建好了环境、PRD、设计和 6 份方案比较，13:36 起一直报网关 401/502，没有提交修复代码。
- 13:55 用户让会话 -54 接手：先修 H1，再修其余缺陷，然后跑 make check、合入、RK1。-54 在 `2057b5890` 保存了 -14 的设计，H1 完成后于 15:46 启动四分道 workflow（`kfix-lanes`）。-54 的主循环 16:37 起报 502/EPROTO；workflow 仍在运行。
- 18:30 会话 -5c 曾提议由自己接手集成。用户 2026-11-01 19:11 在会话 -fe [ad8790] 中另定了分工（由 -fe 转达），以此为准：
  - -54：全部代码和测试代码、四条分道、workflow 写的 `research/evidence/`、`.trellis/spec/` 和内置 skill 文档（按 AC9 随各自的代码提交一起改），以及分道合入集成分支、最终验证。
  - -5c：只维护本任务目录下的 `prd.md`、`design.md`、`implement.md`、`progress.md`、`task.json`。发现需要改代码的问题就发给 -54。-54 开始往集成分支合入分道之后，-5c 每次提交前都先和 -54 对齐，并跑 `git status`、`git diff`，避免两边同时提交。
  - `9c056ad68`（admin 测试固定时钟）是 -5c 在这个决定之前提交的，保留不动。
  - 合入前复核：-fe。
- 合入授权：用户 2026-11-01 14:56 在会话 [4a018c] 同意"复核结论为 pass 或 pass-with-notes 后可以直接合入 `codex/cross-workspace-collaboration`"。

## 缺陷状态

| 分道 | 缺陷 | 提交 | 复核 |
| --- | --- | --- | --- |
| claim | H1 | `4cf4d45a2` | 3 份，pass_with_notes |
| claim | H1-followup | `6f7a601ce` | 2 份，pass_with_notes |
| claim | M1 | `2dd633df4`（瞬时错误重新入队）、`3110295d0`（finalize 拒绝时结算任务） | 复核中 |
| claim | H3、M3 | 排队 | — |
| launch | H2 | `680ad868f` | 3 份，pass_with_notes |
| launch | M2、L6、L7 | 实现中，M2 的改动未提交 | — |
| write | L1、L2 | `c930ac847`（时钟接缝）、`0cf5f7918`、`3aee019aa` | 1 份，pass_with_notes |
| write | L3 | `931446b23` | 与 L4、L5 一起复核 |
| write | L4、L5 | 实现中，L4 的改动未提交 | — |
| web | M4、M5、L8 | `500781f4c`、`2e28c2c98`、`a5e8941b7` | 2 份，pass_with_notes |
| web | L9 | `60ca94822` | 与 L12 一起复核 |
| web | L12 | 实现中，改动未提交 | — |
| web | L10、L11、L13 | 排队 | — |

- 21 项中已有 10 项提交。其中 7 项已复核完毕，没有阻塞项；M1、L3、L9 的复核还没完成。每项都有 `<ID>-red.txt` 和 `<ID>-green.txt`，放在各自分道的 `research/evidence/` 下。
- 19:53 时 launch、write、web 三条分道各有一组正在实现，改动还没提交。-54 准备把工作交给新会话。workflow 只在 -54 的会话里运行，`resumeFromRunId` 也只能在同一个会话里用。所以 -54 一关，这三组改动就会停在半成品状态，新会话不能直接续跑这次 run。
- 各分道的完成门和整条分道的复核都还没有运行。

## 集成预检（只读）

- 四条分道之间只有 claim 和 launch 改了同样两份文件：`.trellis/spec/server/collaboration-knowledge-execution.md`、`docs/collaboration-knowledge-runtime.md`。`git merge-tree` 合并两条分道的当前 tip，没有冲突。
- 共享分支自 `6aed33aad` 起新增 20 个提交，改了 17 个代码和 spec 文件；-a8 的 `codex/runtime-audit-fixes-20261101` 改了 12 个文件。两者与四条分道都没有文件重叠。
- 修复分支和四条分道都不含隐私改写前的 `5059db461`。

## 复核留下的问题

复核结论都不是阻塞，workflow 不会处理这些问题。按 PRD 的"全部修复、测试通过"，第一组建议在合入前处理，每项先写失败的测试，由 -54 实施（清单已发给 -54）；第二组需要负责人决定。

建议合入前处理：

1. 末尾的取值参数会吞掉托管的 `--settings`（claim 复核，medium；实现者也列为未解决）。custom_args 或默认参数以 `--model`、`--append-system-prompt` 等需要取值的参数结尾时，CLI 会把 daemon 追加的 `--settings` 当成这个参数的值，托管路径变成位置参数，关闭继承 skill 的设置因此失效。追加 `--settings` 的逻辑早于知识交付（`765ab186a^` 已有）。H1 之前，这类参数会被白名单顺带拒绝；H1 之后它们按普通参数执行，问题因此暴露。修法在"claim 时拒绝末尾的取值参数"和"改为单个 `--settings=<path>` token"之间选择。先用本机 CLI 确认实际解析行为，只看 `--help`，不调用 API。
2. M5：面板自己内存里那次丢失的提议，恢复后必须关闭表单，但没有测试覆盖。一旦回退，草稿会保持打开，再点一次提交会产生重复的候选（web 复核，medium）。
3. L8：分段错误归因只固定了一部分。读不到来源时应报 `sourceChanged`，prepare 时失去授权时的归因，都还没有测试（web 复核，medium）。
4. H2：daemon 端新增的三处接受检查，包括 `validateKnowledgeClaim` 中的来源条件 `task.droppedPriorSessionID == d.Session.NativeSessionID`，删掉以后没有测试会失败（launch 复核，low，两份复核都提到）。
5. 文档：
   - 操作文档没有写"旧 daemon 遇到新 API 时，每一轮仍报 `knowledge_context_changed`"；
   - `docs/collaboration-knowledge-runtime.md:69` 把拒绝范围写得比代码宽：实际只作用于 v1 内置 Claude invocation，而且漏了"取值拼成子命令名时也会拒绝"；
   - spec 第 18 行的 `resume_unavailable` 单元格没有写明可以重放；
   - 一处测试注释已经过时（`{"--", "--continue"}` 那一行）。
6. L1：`ApproveKnowledgeCandidate` 返回的任何 `ErrNoRows` 现在都会变成合成的 40001 重试。非过期原因造成的未命中，3 次重试后变成 503，以前是 404。另外，spec 写的"重试一定得到 409"只在重试预算还有剩余时成立（write 复核，low）。
7. M5：`recover()` 在点击时用 `current?.value.items` 判断候选是否属于本面板。如果点击时列表因刷新被擦除，或者面板正停在后续页，本面板自己的候选会被当成别处的（web 复核，low）。

需要负责人决定（来自设计阶段，实现时按设计的默认处理）：

- 长 chat 下每次 claim 的 O(N) 历史读取成本；owner 命名空间里一次私有披露会让该 owner 跨工作空间永久拒绝（H3）。
- 旧 daemon（`inv == nil`）custom_args 里的 `--continue`、`--resume` 不在第一步拒绝。spec 第 91 行要求保持旧 daemon 的行为（H1）。
- 子命令黑名单只覆盖 2.1.316 帮助里列出的子命令，加上 `remote-control`。H1 之前所有裸操作数都会被拒绝（H1-followup 复核，low）。
- 1.5s 的服务端忙等预算；`ObserveTaskKnowledgeLaunch` 是否也加重试（M2）。
- observation 写入方是否加固（H2 设计的待决问题 1）。
- 审批预算检查仍用应用时钟（L1 实现者提出）。

## 与本任务无关、但挡住验证门的问题

- `packages/views/admin/executions/list-controls.test.tsx` 中 "keeps current-state scope only for a legal queued filter (2026-10-01T00:00:00Z)" 从 2026-11-01 起必然失败：它写死 `time_from=2026-10-01`、不设 `time_to`，而 `validateAdminFilters` 以当前时间为终点，31 天窗口一过就返回 `rangeTooLong`，不再跳转。所有分支都有这个问题，会让 web 分道完成门和 `make check` 失败。修复分支上的 `9c056ad68` 把这个用例的 `Date` 固定为 2026-10-02，计时器保持真实。修复前 1 个失败、7 个通过，修复后 8 个全部通过。证据在 `research/evidence/admin-list-controls-clock-{red,green}.txt`。
- `server/internal/service/empty_claim_cache.go` 在基线上就不符合 gofmt，本任务不改。
- launch 分道第二次完整运行 `internal/daemon/...` 时，3 个 repocache 测试报 "stop process tree: operation not permitted"；第一次运行和单独重跑都通过。不在已知 flake 列表里，集成后的完整运行中再观察。

## 下一步

1. 等 workflow 跑完：M1 → H3+M3，M2+L6+L7，L3–L5，L9+L12 → L10+L11+L13，然后是各分道完成门和整条分道的复核。
2. -54：按 claim → launch → write → web 合入修复分支，再合入共享分支当前的 tip；sqlc 零漂移；所有包各用一份新克隆库完整运行，并发相关的包加 `-race`；core、views 的 typecheck、lint、test。
3. -54：处理上面"建议合入前处理"的各项，每项先写失败的测试。
4. -fe：合入前复核。make check 和门控 e2e 的时间要与真实模型窗口协调。
5. -54：清理本任务创建的库：`kfix_*` 克隆库，包括 `kfix_write_svc_r2`；模板库在最后删除。
6. -5c：每批分道结果出来后，把实际结论和偏离写进 `design.md` 和本文件。