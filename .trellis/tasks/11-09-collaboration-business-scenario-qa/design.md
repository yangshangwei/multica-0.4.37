# 技术设计：业务场景闭环测试

> 机制核实与代码出处：`research/scenario-feasibility.md`。

## 1. 边界

- 只新增：`e2e/collaboration-business-scenarios.spec.ts`（3 个场景测试，串行）和 `e2e/fixtures/collaboration-business-scenarios.ts`（本任务专用 helper）。如果现有 Provider 无法覆盖某个场景，再新增 `e2e/fixtures/collaboration-business-provider.go`，只在测试 fixture 里实现模拟行为。
- 只读复用：`collaboration-host-runtime.ts`、`collaboration-host-squad.ts`、`collaboration-handoff-chain.ts`、`collaboration-offline-handoff.ts`、`collaboration-control-gaps.ts`、`collaboration-disclosures.ts`、`collaboration-memory.ts`、Codex 的 `collaboration-multi-account-closure.ts`（已提交到 `6e103fd4a`）。不修改其中任何文件。
- 不改产品代码、迁移、已有测试、`package.json`、lockfile。

## 2. 隔离环境（任务私有）

| 项 | 取值/做法 |
|---|---|
| 私有根 | `/Volumes/artisan/code/2026/qa-private-collab-biz-20261109-<rand>/`（0700），内含 `source/`、`bin/`、`state/`、`logs/`、`home/`、`uploads/`、`registry/`、`environment.json`（0600） |
| 源码快照 | 用 `git archive <HEAD>` 解包到 `source/`（不含 .git，不受工作区里其他会话的未提交文档修改影响），再执行 `pnpm install --frozen-lockfile`（优先离线），核实 workspace 链接都指向快照 |
| 数据库 | 共享集群 `multica-postgres-1`（localhost:5432）上新建 `multica_qa_biz_20261109_<hex>`。创建前确认不存在、连接余量不少于 15；创建后核实 `current_database()`，再迁移到最新 |
| 端口 | API 59765 / Web 59766 / Renderer 59767（规划时已核实空闲），启动前再查一次 |
| API | `go build -ldflags "-X main.commit=<sha>" ./cmd/server`；password 认证、single 限流、托管安装开启、device/cloud 关闭；连接池 max 8、min 1 |
| Web | 在快照内执行 `pnpm --filter @multica/web build` 和 `next start --port 59766`（production） |
| Renderer | `electron-vite build` 后由 `scripts/check-desktop-renderer.mjs` 托管（host fixture 要求有真实 renderer URL，不填无关地址） |
| 测试二进制 | `go test -c ./internal/daemon`（真实 poller）、`go build ./cmd/multica`（CLI）；Provider 分别为 host、memory、closure，记录哈希 |
| Playwright | 任务专用配置：单 worker、0 重试、trace 关闭、JSON 报告；每次运行用新的 `QA_RUN_ID`，输出到证据目录 |

环境变量与 Codex 已验证的配置一致，只换端口、数据库、部署 ID 和路径。环境文件当作 JSON 数据读取，不 source dotenv。启动服务后由 `services.py verify` 核实：`/health` 的 pid 和 commit、`/api/config`（password、managed、deployment_id）、登录页里的 API/WS 地址、renderer 返回内容与构建产物一致、每个端口的监听者属于本任务的进程组。

协作闸门：只在本任务数据库里由 fixture 临时打开（`openHostAcceptanceGate`），测试结束恢复原状，并记录前后状态。

## 3. Provider 策略

同一个 Playwright 进程里所有 daemon 共用一个 `COLLABORATION_HOST_PROVIDER`，所以每批运行固定一个 Provider：

| 批次 | Provider | 内容 |
|---|---|---|
| repro | closure（Codex 的 `collaboration-multi-account-provider.go`） | Codex 主线 |
| repro-host | host | Codex 补充用例：squad、switch |
| repro-memory | memory | Codex 补充用例：memory-runtime |
| biz | host（或本任务 business Provider，见第 4 节） | 三个新场景 |

## 4. 场景设计

### 4.1 批次与部署配置

| 批次 | API 知识开关 | Provider | 内容 |
|---|---|---|---|
| R1 | 关（与 Codex 一致） | closure | Codex 主线 `collaboration-multi-account-closure` |
| R2 | 关 | host | Codex 补充 `collaboration-host-squad`、`collaboration-host-switch` |
| R3 | 关 | memory | Codex 补充 `collaboration-memory-runtime` |
| B1 | 关 | host | 场景②、场景③（`--grep`） |
| B2 | 开（`MULTICA_KNOWLEDGE_CONTEXT_ENABLED=true`，只重启 API） | host | 场景① |

每批使用新的 `QA_RUN_ID`。如果某批要重跑，用新的 run ID，旧结果保留。场景① 用 `QA_KNOWLEDGE_CONTEXT=on` 自检：没有设置时 `test.skip` 并写明原因，报告里不算通过。

### 4.2 新文件

- `e2e/collaboration-business-scenarios.spec.ts`：3 个 `test`，标题前缀分别为 `S1 outsourcing`、`S2 support ticket`、`S3 governance`。每个测试自己创建账号和资源，彼此不共享状态。
- `e2e/fixtures/collaboration-business-scenarios.ts`：
  - `bizActor`（password 账号，可选是否建工作空间）、`bizRuntime`（工作空间 + 托管运行环境 + 真实 daemon，可选 Agent/小队）；
  - `offerVendorResource`（拥有者提供 agent/squad，管理方采纳，返回每个 roster 资源的 consent 输入）；
  - 界面封装：`authorizeInUI`、`readIssue`、`workbench`、`notifications`；
  - `rawRequest` 拒绝用例、证据记录器（步骤、事实、截图、清理，写证据前做凭据形态检查）。
- 复用、不修改：`collaboration-host-runtime.ts`、`collaboration-multi-account-closure.ts`（`simpleWorkflow`、`decideClosureResult`、`rawClosureRequest`）、`collaboration-handoff-chain.ts`（`directedFormThroughUI`）、`collaboration-offline-handoff.ts`（`selectCollaborationRecipient`）、`collaboration-control-gaps.ts`（`commentThroughUI`）、`collaboration-knowledge.ts`（`fillKnowledge`/`submitKnowledge`/`chooseCandidate`/`approveKnowledge`）。

### 4.3 场景① 外包交付

| 角色 | 空间 | 资源 |
|---|---|---|
| 甲方产品负责人 A | 甲方产品部（来源空间） | 二期时可以没有资源 |
| 乙方项目经理 B | 乙方交付中心 | 运行环境 B，小队 S（Leader“乙方架构师”、成员“乙方开发”） |
| 乙方外包工程师 C | 无 | 无 |
| 竞争乙方 Y | Y 空间 | 无 |

1. A 在界面创建一期任务：AI 辅助，处理人 B，附需求说明。核对 B 的 workbench `current` 中有该任务，且 B 收到 `assigned` 通知。
2. B 在界面添加 C 并转交给 C。核对：C 的工作空间数为 0；C 的 `current` 有该任务；B 的 `current` 没有、`participating` 有；C 收到 `participant_added`/`transferred` 类通知。
3. B 提供小队 S，C（当前处理人）采纳。在 B 授权前，可提及目标里没有 S。B 在界面逐个授权 roster 资源。C 显式恢复。
4. C 在界面 @S 下达需求。真实执行三段：
   - Leader（`act` 派活评论 @成员）；
   - 成员（`act` 回写实现说明）；
   - 续跑（`act` 总结评论 + `squad_activity`）。
   三段的 agent、运行环境、拥有者都属于 B，输入作者是 C。组状态为 `succeeded`，任务状态不是 `done`。
5. C 提交结果，A 退回给 C（原因：缺少无障碍验收说明）。C 再 @S，Leader 直接补写说明并记 `squad_activity`（no delegation）。C 再次提交。
6. 知识：C 在界面基于 Leader 总结评论提出候选，目标为甲方空间。B 调用确认接口被拒绝（记录状态码与 code）。A 在界面确认。
7. A 验收第二次提交，任务变为 `done`。交付历史为 2 条：一条 returned，一条 accepted。
8. A 创建二期任务（处理人 B），B 提供单个 Agent“乙方架构师”，A 采纳，B 授权，B 恢复，B 提及该 Agent。真实运行的 `knowledge_observation.contributions` 应包含步骤 6 的候选。放行并结算。
9. Y 读取一期和二期任务返回 404，响应里不含标题。

### 4.4 场景② 客服工单

角色：客服主管 S（客服中心）；研发 D（研发部，运行环境 D，Agent“研发排障”）。

1. S 在界面创建任务：手动处理，处理人 D，附 `customer-log.txt`。
2. D 在界面请求补充信息，S 在界面返回信息，并用评论共享 `sample-export.csv`（如果界面评论不支持附件，就用公共 API 上传草稿并评论，报告里注明）。
3. 手动模式下，D 提供资源被拒绝（`task_manual`），可提及目标为空。
4. S 切到 AI 辅助，回执 `ai_policy_changed`，队列仍为 paused。
5. D 提供 Agent，S 采纳，D 在界面授权。D 发送 @指令后，在显式恢复前没有运行；D 恢复后才执行。
6. 真实运行的 `attachment_hashes` 包含两份文件，sha256 与上传内容一致。放行后结算 `success`，任务不是 `done`。
7. D 提交结果，S 验收，任务 `done`，`submit_command=false`。
8. S 重开：目标 hold 为 `issue_reopened`；D 发 @指令后没有运行；D 恢复后运行一次；D 再提交，S 再验收，任务 `done`。交付历史为 2 条 accepted，重开事件可追溯。

### 4.5 场景③ 异常治理

角色：负责人 L（L 空间）；合作方 O1、O2（各自空间、运行环境和 Agent）。

1. L 创建 AI 辅助任务，处理人 O1，邀请 O2。O1、O2 提供 Agent，L 采纳，O1、O2 在界面授权，L 恢复。
2. O1 发两条指令：A1 和 A2。A1 真实运行，暂不放行；A2 排队等待。
3. O1 在界面撤回授权。核对：
   - 回执 `grant_revoked`，通道 `stop_requested`，hold `grant_required`；
   - A2 仍为 `waiting`；O2 的目标不变。
   - daemon 观察到停止后终止 Provider：socket 关闭，`released_ms` 为空。A1 运行结算为 `host_exit_observed`，指令为 stopped。
4. O2 @自己的 Agent 下达 B1，真实执行成功，不受 O1 影响。
5. O1 重新授权后 hold 仍在，在显式恢复前 A2 不运行；O1 恢复目标后，A2 真实执行成功。
6. O2 发 B2（运行中、不放行）和 B3（排队）。L 在界面移除 O2。核对：
   - 回执 `participant_removed`；B2 被停止，进程真实终止；B3 被取消；
   - O2 读取任务返回 404；O2 只收到 `removed` 通知（不可打开、无标题）；O2 的 Agent 不再在可提及目标里。
7. L 取消任务：回执 `cancelled`，所有人 `submit_command=false`，O1 的 AI 评论和恢复都被拒绝。L 重开，O1 恢复，O1 的指令真实执行。O1 提交，L 验收，任务 `done`。

### 4.6 判定原则

- 每段真实运行都核对：输入作者、agent、运行环境、拥有者、结算状态和退出账本。
- 拒绝类断言核对状态码和 code，并确认状态快照没有变化。
- 时序相关的地方（停止证明、续跑排队）用 `expect.poll` 等待实际事实，不用固定 sleep。

## 5. 证据与报告

- 证据目录：`artifacts/qa/collaboration-business-scenarios-20261109/`。包含 `environment/`（allocation、构建/迁移日志、哈希、README）、`<run-id>/`（playwright.json、test-results、截图）、每个场景的 `*-evidence.json`（0600，提交前做凭据形态扫描）、`results.json`、`report.md`。
- 每个场景的证据文件记录：拓扑（账号/空间/Agent/运行环境/安装的 ID）、按步骤的事实（请求回执、运行 ID、状态、通知、列表视图）、Provider 观测（输入、附件哈希、知识注入、启动与退出账本）、清理结果、页面错误。
- 判定规则：Agent 运行成功与业务完成分开断言；入队或回执不等于执行；跳过、阻塞、环境失败不算通过；首轮失败保留，修正后用新的 run ID 重跑，两次结果分别报告。
