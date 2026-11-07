# 当前候选变更风险与覆盖缺口

**当前计划覆盖旧阻断和常规任务链路，但不足以核销新增的小队创建、知识恢复、Codex 输入边界、日志脱敏与 CLI 委派回执。** 以下是覆盖缺口，不是已确认产品缺陷。没有运行任何测试，也没有修改产品或运行环境。

对比 `1a32ca36c2c4f4455a01bdd39d0e0ab4db20cccc` → `4c817dee9397a42ac903f419af7804b11d2d863e`：apps/packages/server/scripts/e2e 有 231 个变更路径，加三个 workflow 和一个 Playwright config 共 235 个。完整映射与 blob SHA-256 见 [current-risk-review.json](current-risk-review.json)。

主会话告知：core-business 三文件、task-lifecycle、旧失败 Web/password 和新增 memory/workbench/handoff-chain/detail 在执行；后端 250 个 race 用例覆盖 service/handler/auth/middleware/router/collaborationrun；非 Mobile lint/typecheck 已通过、单元仍执行。本审阅没有把这些进度消息当作已核验 raw 结果。

## 最小新增现有浏览器库存

下列共 **14 个静态声明身份**，执行前仍须通过当前环境的实际 collection 确认。使用冻结候选的生产 Web、1 worker、0 retry、唯一结果目录。

| 优先级 | 现有测试 | 选取 | 补上的具体风险 |
| --- | --- | --- | --- |
| P1 | `squads-design.spec.ts` | 全文件 1 项 | 小队列表→新 chooser→返回、已存偏好、模板/项目与本地化导航 |
| P1 | `localized-template-defaults.spec.ts` | `Chinese squad staffing|a localized name collision`，2 项 | 新模板创建页真实提交、复用定制 Agent、真实成员列表、409 无部分写入、失败保留表单 |
| P1 | `agent-role-template.spec.ts` | `offers a template starting point and creates from a role`，1 项 | Agent 创建现在也复用新的 CreationPageShell，须验证真实成功导航 |
| P0 | `collaboration-knowledge.spec.ts` | 全文件 8 项 | 提案/准确版本审批/纠错/派生、purge、丢响应原键恢复、拒绝解锁、记录来源、target-only 撤权、过期刷新失败、原生审阅 |
| P0 | `collaboration-recipient-picker-web.spec.ts` | 全文件 1 项 | self/recent/search、最近协作真实持久状态、最小账户字段、嵌套 Escape、过时选择；旧 exact-picker 链路不能替代 |
| P0 | `collaboration-terminal-reopen-web.spec.ts` | 全文件 1 项 | 已关闭任务禁止 @agent/Resume、仅创建者 reopen、不复活旧执行、Stop barrier 保持 |

前五项使用对应 legacy 配置；最后一项必须是密码/managed/执行开启的受控环境。知识 suite 自己保存/恢复创建 gate、保持执行关闭，不能与其它改 gate 的 suite 并发。原生知识项需要匹配构建的 renderer；少前置不得记为 PASS。

示例选取（在既有拥有的环境中，不新起服务）：

```sh
pnpm exec playwright test e2e/squads-design.spec.ts e2e/collaboration-knowledge.spec.ts e2e/collaboration-recipient-picker-web.spec.ts --workers=1 --retries=0
pnpm exec playwright test e2e/localized-template-defaults.spec.ts --grep 'Chinese squad staffing|a localized name collision' --workers=1 --retries=0
pnpm exec playwright test e2e/agent-role-template.spec.ts --grep 'offers a template starting point and creates from a role' --workers=1 --retries=0
# Separate password/managed environment:
pnpm exec playwright test e2e/collaboration-terminal-reopen-web.spec.ts --workers=1 --retries=0
```

**现有库存的实际空缺：手工创建小队。** 搜索所有 e2e 只找到两条 chooser 链接和模板创建提交，没有 `/squads/new/manual` 表单的真实提交。新手工页面有充分单元覆盖（leader/member 去重、失败恢复、部分成员失败、等待后导航），但不足以证明新 Web/Desktop 路由与真实 API 接通。最小补证是：从 chooser 进入 manual，选自有 leader＋成员，提交后验证真实 API roster 与刷新，再走原生 Desktop 的新路由；只读 reviewer 不写这个新测试，由主会话决定执行或明确 NOT_RUN。

`squads-audit.spec.ts` 大部分重复 chooser/响应式与度量，不列最小新增。若使用，应设唯一 `SQUAD_AUDIT_PHASE=production-<run>`；`before` 分支会提早返回，默认 `after` 把产物文案标作 development。不能把这些度量当严格性能门。

## 后端缺口与精确入口

主会话列出的包范围没有包括 `pkg/agent`、`pkg/redact`、`cmd/multica`、普通 `internal/daemon`、`execenv` 和 `internal/migrations`。应先核对已有 raw 具名库存，已执行的身份无需重复。

| 优先级 | 变化 / 风险 | 最小已有入口 |
| --- | --- | --- |
| P0 | Codex 原生 thread/turn、配置绑定、续用/撤回与私有输出 | `pkg/agent` 的 `^TestKnowledgeCodex` 11 个顶层；测试可执行文件自充 fake provider，不使用安装账号 |
| P0 | 普通 daemon 的知识配置与实际输入、丢回执禁止重放 | `internal/daemon` 的 `TestCodexKnowledgeNamespaceBindsSourceConfiguration`，及 `TestKnowledge*` / `TestOrdinaryKnowledge*` 输入链路 |
| P0 | 结构化日志敏感键/JSON/令牌边界，截断前脱敏 | `pkg/redact` 整包；`internal/daemon` 的 `TestToolOutputPreview*` 与 `TestExecuteAndDrain_ToolOutputRedactedBeforePreview` |
| P0 | 评论保存成功 ≠ 委派成功；异常/重复回执及不盲目重发 | `cmd/multica` 的 `^TestIssueCommentDelegationReceipts`；真实 Cobra add/update＋受控 HTTP，完整 JSON 和 warning 分流 |
| P0 | in_review 不得提前放行 stage、stdout/stderr 文案避免重复写入 | `execenv` 的 `TestIssueWorkflowSeparatesDeliveryFromStageAcceptance` 与 `TestEveryBriefThatTeachesJSONOutputAlsoWarnsAgainstMergingStderr` |
| P0 | 765 回滚与并发 mint 不能丢失权限边界 | `internal/migrations` 的 `TestTaskCapabilityRollbackFence`；只在显式独立 Go 库运行，测试使用 private schema |

这些默认测试都应通过现有 Agent CLI guard。保持私有 HOME/TMP、关闭真实模型环境变量、限定并发和唯一输出；不要使用普通默认数据库。下面仅是已有入口推荐，本 reviewer 未执行：

```sh
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race -p 1 ./pkg/agent -run '^TestKnowledgeCodex' -count=1 -json
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race -p 1 ./pkg/redact -count=1 -json
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race -p 1 ./cmd/multica -run '^TestIssueCommentDelegationReceipts' -count=1 -json
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race -p 1 ./internal/daemon -run '^Test(CodexKnowledgeNamespaceBindsSourceConfiguration|Knowledge|OrdinaryKnowledge|ToolOutputPreview|ExecuteAndDrain_ToolOutputRedactedBeforePreview)' -count=1 -json
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race -p 1 ./internal/daemon/execenv -run '^Test(IssueWorkflowSeparatesDeliveryFromStageAcceptance|EveryBriefThatTeachesJSONOutputAlsoWarnsAgainstMergingStderr)$' -count=1 -json
# Explicit owned Go database required; never the API-serving database:
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race -p 1 ./internal/migrations -run '^TestTaskCapabilityRollbackFence$' -count=1 -json
```

已排后端需确认原始终态包含：router 的 `TestTaskCapabilityTargetedReporterListWindows`、`TestTaskCapabilityTargetedRuntimeOwnerAuthority`、`TestTaskCapabilityTargetedMutableRoleTextCannotChangeScope`、`TestTaskCapabilityReviewProductionRouter`；handler 的 `TestTaskCapabilityReviewSourceContextDoesNotReadOutsideScope` 和 CloudFront scoped-token refusal；service 五个 `TestCodex*Knowledge*`；host 的 `TestHostCodexKnowledgeRequiresEphemeralBeforePrivateTurn`。包被选中、构建成功或别的 TaskCapability 测试通过都不是这些具体用例已执行的证据。

委派语义还应确认 handler `TestCreateComment_MixedMentionSurfacesPartialTriggerOutcomes`、`TestSquadOperatingProtocolChecksDelegationReceipts`，service `TestSquadsSkillCoversLeaderRoutingContract`、`TestSquadTemplates_LifecycleEvidenceRouting` 已在当前具名库存。API scope 保证不等于主机 shell sandbox；旧实际编译 CLI 验收和真实模型样本不能改标本轮。

若本轮要对 **实际小队执行** 作当前声明，现有 `collaboration-host-squad.spec.ts` 是最小具名链路：真实受控 Host leader/member、capacity-one 合并、later instruction 与另一小队。core-business 的 HTTP daemon / G6 委派路径及 manual detail 的 `noAIWork` 不能替代它。部署知识开关 OFF/ON 的当前声明则分别需要 `collaboration-knowledge-deployed.spec.ts` 的显式环境，不建议把不准备条件的跳过算覆盖。

## 验证入口自身的新增变化

非 Mobile pnpm 测试不会自动跑新 Node/shell workflow 契约。推荐在非 Git 的私有 TMP 中运行：

```sh
node --test scripts/check-agent-critical-path.test.mjs scripts/agent-critical-path.test.mjs scripts/agent-critical-path-workflow.test.mjs scripts/changelog-workflow.test.mjs
bash scripts/test-go.test.sh
bash scripts/go-test-with-agent-cli-guard.sh -- node --test scripts/go-test-database-preconditions.test.mjs
```

最后一项只向测试拥有的不存在 Unix socket 验证缺省/畸形/不可达 TestMain 前置，拒绝继承 DB 凭据，但会编译三个测试包，应与重型运行协调。新的 critical-path 四个业务身份全部已经包含在主会话 core 三文件中；不为相同四例另启完整 launcher。契约测试与浏览器结果都不能声称 hosted GitHub CI 或 Release 已执行。

## R01–R08 / AC 核销边界

- **R01/R03**：候选冻结正确；运行结束仍需重核源码、Build ID、API/Web/renderer/CLI 配置及进程归属，专库与 gate 清理不能省略。
- **R02/R06**：本报告补当前变更→风险→具名测试；执行后必须逐例归档 PASS/FAIL/BLOCKED/SKIPPED，保留第一失败。推荐清单和静态数量不是已执行数量。
- **R04/AC04**：知识、撤权/恢复和实际 squad runtime 需上述独立证明；至少两工作区、三身份、双会话/双 runtime 的结果不能只由 fixture 定义推定。
- **R05**：当前子集不能证明全部项目/迭代/分诊/资源/自动化/admin 用例已复跑。未变区域可明确引用历史结果及本轮 NOT_RUN；若要完整新候选通过，仍须执行 agreed P1 inventory。
- **R07**：Linux FD 回归要实际 Linux 运行及匹配当前包；Darwin 或旧 `1a32+FD` 包都不替代。真实 Windows/iOS/Linux desktop、安装升级/签名公证/自动更新、外部集成和商业模型缺口保持开放。
- **R08/AC03/AC05**：覆盖缺口不是产品缺陷；出现真实失败再按原任务要求取证。本次无产品更改、无 Provider 调用。任一 agreed P0/P1 或必要平台/Provider 未满足时，最终只能声明已验证的确定性范围，不能写完整分支验收通过。

源码/测试读取、235 个变更路径、候选 blob 哈希与 9 组映射已核对。Lint/typecheck/业务测试由主会话负责，本只读切片未执行；仅新增本报告及配套 JSON。

## 审阅过程中主会话新增观察

主会话随后通知：Core 单元 4460 PASS / 1 FAIL，独立复现 14 PASS / 1 FAIL，问题指向 diagnostics 未映射 `/squads/new/manual`；原 detail journey 的滚动恢复预期 620、实际 476，正在诊断。本 reviewer 未读取这些新 raw 结果，因此这里只保留带来源的当前信息，不作为独立确认，也不外推为手工创建完整流程失效或已确定滚动根因。两项首败应保留，由主会话后续最终证据审阅衔接。
