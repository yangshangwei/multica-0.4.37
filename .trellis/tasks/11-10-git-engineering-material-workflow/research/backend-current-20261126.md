# Research: 当前后端能力与 Git 物料首个闭环

- Query: 对照 D1–D10/PAC1–PAC15，核对当前 main 基础上的复用点、凭据、持久化审批、后台任务和方案启动闸门。
- Scope: internal；只读源码与规范研究，只新增本文件。
- Date: 2026-11-26
- Source: 父会话已从 main 的 `2c2010f273eddd7a02fdff95da52c0ba4ed4eb99` 建立 `codex/git-engineering-material-workflow`；本研究未运行 Git 命令、测试、数据库或外部 API。

## Findings

### 1. 结论与真实缺口

仓库已有精确版本的人类审批、协作阶段门禁、原始请求回执、后台租约和完成证据机制，但没有 Git 工程物料领域的表、API、发布/合并执行器或版本基线。针对 `material_workflow` / `material_revision` / `git_material` / `engineering_material` 的源码与 SQL 搜索未找到现成模块。不要将协作阶段的 `materialize`（创建普通子事项）误认为 Git 物料入库。

当前范围至少需要一个专门的物料服务边界：绑定输出仓 → 保存稳定物料身份与候选版本 → 自动生成发布意图 → Git 分支/PR/MR → 人类批准当前版本 → 受控合并及内容核对 → 固定基线 → 创建/启动方案任务。现有能力可减少权限、持久化、队列与事项接入的重复设计，但不会使这条链路自动成立。

### 2. 可复用代码及不能直接复用的语义

| 能力 | 现有文件及定位 | 实际保证与限制 |
| --- | --- | --- |
| 普通项目、多仓资源 | `server/internal/handler/project_resource.go:74`, `:88`, `:452`, `:965`; `server/pkg/db/queries/project_resource.sql:10` | `github_repo` 虽名为 GitHub，URL 接受通用 Git；`resource_ref` 只有 URL/ref/default-branch hint，没有物料输出用途、路径权限、托管仓稳定 ID 或写凭据绑定。明确物料绑定不可静默回退到 workspace 仓库。 |
| 项目资源并发 | `project_resource.go:532`; `project_resource.sql:29` | 创建/修改资源通过 workspace/project fences 串行化，资源集变化会改变执行依据；新绑定应引用同一工作空间、项目的资源，读取事务内再次核对，不只信请求中的 URL。 |
| 版本审批 | `server/internal/service/collaboration_stage_actions.go:62`, `:331`, `:386`, `:448` | candidate/version/content digest、精确 head CAS、不可变决定、原始 key 回执已有实证测试。**现有 API 只针对 collaboration root，创建 proposal 本身也 human-only**；不能开放旧端点让 Agent 绕过合同。 |
| 方案拆解对象 | `server/internal/collaboration/stage.go:79`, `:100`; `collaboration_stage_actions.go:567` | 现有 proposal 是 summary + 1–20 个待创建普通子事项。接受后在同一事务生成 `todo`、未指派的子事项，独立映射、不 enqueue。不是 Markdown PRD、Git 设计物料或自动方案阶段。 |
| 阶段策略 | `server/internal/service/collaboration_stage_policy.go:22`, `:99`, `:192` | 项目 stage policy 缺省关闭；owner/admin 人类配置，变更不启动/停止任何工作。它明确独立于 Git 自动化开关，不能拿它替代 D8 的默认开启策略。 |
| 真实执行闸门 | `server/internal/service/issue_admission.go:12`; `server/internal/service/task_start.go:18`, `:76`; `server/internal/service/issue_task_transaction.go:51` | 统一入队检查和 queued→started 事务末端检查存在。D8 的等待手动启动/固定基线校验应在这些服务边界检查，不能只加前端按钮。 |
| 普通自动派发 | `server/internal/service/issue.go:943`, `:987`; `task.go:1235`, `:1399`, `:1484`, `:4244`, `:4342`, `:6374` | 现有被指派 Agent 的 `todo` 会被派发；只有 backlog 停车语义不足以满足 D8。仅创建时不 enqueue，无法防止后续指派、评论、恢复或 claim/start。 |
| DB 可靠后台任务 | `server/internal/service/collaboration_knowledge_automation_jobs.go:114`; `collaboration_knowledge_automation_worker.go:21`, `:60`; `server/pkg/db/queries/collaboration_knowledge_automation.sql:52` | 接受事务内写 job，`SKIP LOCKED` + lease token/generation + 截止时间 + 条件写回，外部调用在事务之外。知识提取的未知结果不能盲重试是可复用模式，**知识业务表/worker 不能直接变成通用 Git 任务队列**。 |
| 启动 worker | `server/cmd/server/main.go:706` | 可参照同一 server lifecycle/sweep context 注入独立物料 worker；供测试调用同一个 `ProcessNext`。内存事件只负责唤醒，不承担唯一权威。 |
| 完成门禁 | `server/internal/handler/github.go:1939` | PR close aggregate 经 `service.WriteIssueStatus` 进入事务完成检查，已不是 11-10 研究所述的裸完成条件。物料 PR 仍应使用独立关联/明确 reference-only，不产生 `Fixes <business issue>` closing intent。需求合并不应直接写父事项 done。 |

### 3. 凭据与 provider 的实际形态

**GitLab**

- `server/internal/integrations/vcs/vcs.go:127` 的 Provider 只有 webhook 分类/验签/解析、ValidateToken，没有读文档、提交、开 MR、评论或条件合并。
- `server/internal/integrations/vcs/gitlab.go:205` 使用实例 API 验证身份；验证成功不证明写入/合并权限。
- `server/internal/handler/vcs.go:53`, `:82`, `:159`：受 `VCSIntegrationEnabled`、`VCSSecretBox` 控制；连接已有 `AccessTokenEncrypted`、`WebhookSecretEncrypted`、连接者和 trust revision。Connect 先验证 provider token，再用现有 SecretBox 加密存储；公开响应不暴露 token。
- 可以在配置中引用工作空间内现有 connection，使用现有密钥机制解密；仍必须新增明确的目标仓和写入/合并授权配置以及运行前核对。只读接入被管理员选择为输出仓不能自动升级为写授权。
- 自建 GitLab 实例从已审核连接配置取值；不要让 PRD 或 Agent 结果提供 URL/token，或关闭 TLS 验证。

**GitHub**

- `server/internal/handler/github.go:678` 已有 App JWT 生成；`:817` 仓库浏览交换短期 installation token，`:841` 只请求 `metadata:read`，结束后撤销（`:923`）。这个 token 不能用来提交物料。
- `server/internal/integrations/ghsnapshot/client.go:60`, `:84`, `:129`, `:162` 有独立 App 凭据客户端、缓存、singleflight、短期 token 和受限错误输出。其 token 方法未导出，现有公开行为是 PR/CI snapshot；包职责明确是读取。
- GitHub 写入需要新窄 provider 能力与明确 installation/repository 绑定。可提取/共享既有认证实现或注入服务端凭据解析器，不能把 private key 或 installation token 发给 UI/Agent。不要为了复用而将现有 VCS 统一大重构。
- GitHub 默认地址是 github.com；没有现成依据声称 GitHub Enterprise Server 支持。

**建议窄合同（建议，未实现）**

在独立物料 provider 边界定义与工作流对应的动作，而不是通用任意 HTTP/Git 执行接口：`ResolveRepository`、`ReadFileAtCommit`、`EnsureCandidateReview`、`ObserveReview`、`SyncBusinessDecision`、`MergeApprovedHead`、`VerifyMergedContent`。每个操作只接受冻结的仓库身份、目标分支、物料路径/内容、预期 SHA 和操作 ID，凭据由服务端解析。合并必须核对完整变更集；未知原生门禁保持等待/阻塞，不能将 mergeable=true 当成所有保护规则满足。

远端查找恢复必须使用稳定的分支/操作关联键，并核对实际内容。网络超时保留未知状态；远端创建成功但响应丢失不应重复新建 PR/MR。观察结果中的 merge SHA 与受审 head 不必相等（squash），必须核对最终文件内容摘要。

### 4. 权限与事务模式

- `server/internal/handler/project_write_fence.go:47` 的 `projectHumanActor` 同时拒绝机器凭据及 agent actor；`:74`/`:86` 的事务辅助函数锁 workspace、当前会员，并且只在整个事务回滚后重试。**回调不能包含远端副作用。**
- `server/internal/service/artifact_authority.go:32`, `:40`, `:101`, `:191` 的 `ArtifactCredentialSnapshot`、`Actor`、当前凭据核对及 user/member fences 已覆盖 JWT/PAT/task/password session 的边界，可复用认证原语，而非借用 artifact 专有授权。
- `stagePolicyAuthority` (`collaboration_stage_policy.go:99`) 是人类当前身份、workspace/project 锁与 manager 权限的实例；它同时依赖 stage schema，不宜直接用于普通物料服务。抽用公共原语或沿用已有 project transaction 边界。
- `RequireHumanActor` (`actor_guards.go:96`) 只是路由 deny guard，服务端审批方法仍须检查已验证的当前 principal、成员资格和配置审核人。客户端提交 `reviewer_user_id` 不构成身份。
- 原始 request_id + body digest + CAS 有大量现成模式：`ReviseStagePolicy` (`:192`) 在同一项目锁下先查原始 key，相同 body 返回同一 receipt，不同 body 返回 idempotency_conflict。机器不得调用决定端点；只准结构化候选写入，不等同人工批准。
- 人类业务审核、远端评论同步、合并 intent、实际合并核对需要分别保留。权限撤销/仓库配置变更/候选更新必须阻止未发出的旧操作；已产生远端副作用只对账，不能抹掉历史。

### 5. 推荐最小纵向实现边界

以下是给父会话收敛首期范围的建议，不能冒充已获用户定案：

1. 在普通项目上保存物料配置：明确输出 resource/connection/repository/base/path root、指定人类审核人、D8 auto-plan（首次配置默认 true，显式 false 持久化）。保留不启用物料流程的旧项目行为。
2. 稳定物料主体与不可变 revision 独立持久化；允许路径由服务端从类型和 UUID 生成。冻结正文、来源 commit/path、验收条件和阻塞项；仅完整可评审候选进入一次 publish intent。不要把普通对话每次更新都当作发布。
3. DB 意图、可恢复 worker 和 GitLab/GitHub adapter 一起落地；实际 provider 写入不与业务 DB 伪装成跨系统原子事务。评论同步使用同一操作标记，审批者只需 Multica 账号。
4. 人类审批绑定 candidate + 当前远端 head + 配置 revision，执行器核对 current authority、完整路径集合和原生门禁，再条件合并；最终按 merge commit 读取完整文件验证摘要，记录 approved baseline。
5. D8 创建同一普通方案 Issue（todo）与独立、持久化的 stage intent。关闭后保持 `waiting_manual`；开启后读取/核对完整固定 PRD，按现有 Agent invoke authority 入队。手动启动清除该同一 intent 的门禁并复用 Issue；重新开启仅影响新阶段是最保守建议，需父会话记录最终决策。当前 proposal API 可作为**后续方案审核的参考模式**，但它只服务 collaboration issue，不能假定任意普通方案 Issue 已可调用。
6. 若本期声明“方案审核闭环”，必须额外实现方案产物候选/审核及编码 gate，不能仅建 todo 就称完成 PAC14。若包括跨模块编码、QA、整体验收，则需独立模块版本清单和流程合同；现有 stage/release 的人工与 runtime 证据不是自动多仓工程编排。

建议表至少区分：project binding/policy、stable material、immutable revision、immutable decision、durable operation/intent；可按实际设计将当前 heads 放在 material row，勿把所有历史塞进可变 JSON。唯一键包括 workspace/project/material identity 与 revision，operation idempotency key 在当前租约中持续保持。新表没有外键；每个索引独立 `CREATE [UNIQUE] INDEX CONCURRENTLY` migration，并补项目/工作空间删除的显式清理或保留拒绝。

### 6. 实现与验证入口

拟新增/接入文件（命名由最终设计确认）：

- `server/internal/material/`：稳定 ID/路径、候选校验、状态/版本与受控 provider 合同。
- `server/internal/service/material*.go`：事务持久化、当前权限、后台处理、方案 task/intent。
- `server/internal/handler/material*.go`，`server/cmd/server/router.go`：workspace/project scoped reads、候选写入、人类审核、配置与手动启动；schema/严格解析；`main.go` 生命周期 worker。
- `server/pkg/db/queries/material*.sql`、`server/migrations/`、生成 sqlc；显式 project/workspace cleanup 接口。
- `issue_admission.go`、`task_start.go` 及对应 claim/恢复边界：D8 持久化等待和固定基线，防止旁路启动。
- 所有 CLI/API/产品行为在 built-in skill 有说明时，同步 `server/internal/service/builtin_skills/*` 与 references 源码映射。

测试复用：

- `server/internal/testutil/db.go:30`, `:174`, `:247`, `:279` 的 Fixture、Issue/Task/Project builders；`http.go:22`, `:93` 的 JSONRequest/Call/Wants/JSON。不要照抄旧 `project_resource_test.go` 的 recorder/INSERT 样板。
- `collaboration_stage_test.go:38`, `:411`, `:622`, `:919` 展示完整版本、并发一次创建、丢失 commit 回执测试；复制行为模式，不能依赖该 fixture 的业务角色作为物料需求的授权。
- `collaboration_knowledge_automation_worker_test.go:187`, `:233`, `:798` 示范 fake HTTP 外部服务、真实持久化 worker、配置缺失、未知与租约过期。
- `collaboration_stage_admission_test.go:153` 验证共享 enqueue gate 与真实 start boundary，适合参考 D8 的关闭/重放/指派旁路矩阵。
- provider 使用 `httptest.Server` 验证真实 HTTP method/path/body、预期 SHA、完整分页和失败恢复；真实临时 Git 可验证固定文件内容。默认测试不得访问用户 Git/真实 Agent。
- 必须覆盖：跨 workspace/project、机器审批、自报审核人、当前人类撤权、旧 head、路径夹带/链接逃逸、目标配置变化、远端成功回执丢失、重复/乱序事件、false 自动启动持久化、手动双击、任务已创建未入队、旧批准不授权新方案/编码、合并不自动完成业务事项。
- `.trellis/spec/server/database-test-isolation.md` 要求显式 task-owned DATABASE_URL；handler/router TestMain 即使 `-run` 过滤也会建/清 fixture。不要使用默认共享数据库。研究阶段未运行测试。

## Related specs

- `.trellis/spec/server/index.md`
- `.trellis/spec/server/security-boundaries.md`
- `.trellis/spec/server/task-api-capabilities.md`
- `.trellis/spec/server/collaboration-stage-approvals.md`
- `.trellis/spec/server/collaboration-knowledge-execution.md`
- `.trellis/spec/server/database-test-isolation.md`
- `.trellis/spec/server/verification-environment.md`
- `CLAUDE.md`：迁移、UUID、数据库 fixture、API 兼容与 built-in skill 同步。

## External references

本轮只核对本地源码；GitLab/GitHub API 及官方行为资料沿用本任务 `research/gitlab-github-compatibility.md` 与 `research/primary-sources.md` 作为待刷新输入。实现 adapter 时须重新查官方原生审批/合并接口，不将旧研究或 fake provider 视为真实实例验收。

## Caveats / Not Found

- 未发现可直接开启的物料工作流实现或通用 Git 写入 provider；首个闭环需要真实新增服务和持久化。
- 本研究不证明运行环境已有 Git 写权限、可达的自建 GitLab、正确 App 权限或实际模型产物；这些是后续使用配置/集成验证。
- 初步通信曾将既有 stage 子事项写成 backlog，读到实际 `materializeStageChildren` 后已校正：实际为 **todo + 未指派 + 不 enqueue**。D8 仍需要独立持久化执行闸门。
- 未更改产品代码、执行测试或调用远端服务；不能据此标记 PAC 完成。
