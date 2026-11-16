# 自动知识整理：AI 资源与后台执行研究

日期：2026-11-14。范围：只读当前工作区代码，未运行测试、真实 AI 或外部请求；本文是规划证据，不是已实现能力。

> 后续决策：2026-11-14 用户已明确“采用这个统一服务端模型方案来自动整理知识”。方案 A 的配置与费用归属已进入 PRD D8；下文关于“待用户确认”的文字保留为本研究产生时的选型记录，不再是当前未决问题。项目策略与规则授权仍待收敛。

## 结论

最小可行路径是复用 `server/pkg/llm` 的服务端无工具 JSON 生成能力，新增独立的持久整理工作及授权检查。这样验收完成后无需重开协作队列、恢复原 Agent 会话或要求参与者电脑在线。

这仍是**待用户确认的产品选择**：现有部署级模型配置服务于四类辅助功能，并没有授予系统自动发送完整协作材料的权限。需要明确新的自动整理用途、材料范围和费用由部署配置承担。若要求使用项目自己的 CLI 智能体，则需要另一套明确的资源选择及持续授权，不能默取“项目第一个小队”。

## 可复用的服务端模型层

| 已有事实 | 证据 |
| --- | --- |
| 所有 API 进程自行发起的模型调用都经 `pkg/llm`，使用 OpenAI 或兼容网关；与 daemon 执行 CLI 的账号完全分开 | `server/pkg/llm/client.go:9`、`:20`、`:86`；`.env.example:126` |
| 配置入口是 `MULTICA_LLM_API_KEY`、`MULTICA_LLM_BASE_URL`、`MULTICA_LLM_DEFAULT_MODEL`，router 注入 Handler 的可共享 Client | `server/cmd/server/router.go:457`；`server/internal/handler/handler.go:539` |
| API key 和 base URL 均为空时必须零外部请求；只设置 model 也不启用 | `server/pkg/llm/client.go:64`；`server/pkg/llm/outbound_contract_test.go:52` |
| 现有描述优化直接调用 `GenerateJSON`，限制 20,000 字符输入、45 秒和 8 个并发，并严格检查输出；不会派发 Agent | `server/internal/handler/issue_description_assist.go:23`、`:58`、`:76`、`:131`、`:172` |
| 推荐创建助手是人类主动请求的只读建议，拒绝 Agent/task-token 主体；结果不授权执行、不修改默认值 | `server/internal/handler/issue_creator_recommendation.go:49`、`:78`、`:135` |
| 推荐返回的依据必须是所给材料精确子串；生成完成后重新检查权限和材料是否变化 | `server/internal/handler/issue_creator_recommendation.go:19`、`:145` |
| 新增调用者必须同步出站材料清单及文档，静态测试会检查所有调用者 | `server/pkg/llm/outbound_contract_test.go:126`；`server/pkg/llm/client.go:31`；`.env.example:139` |

可复用的是模型 Client、严格 JSON 解码、精确引文校验、超时和模型后权限复查模式；不能后台伪造一次人类描述优化或推荐 HTTP 请求。整理资料与结果写入应由新的领域服务负责。

无配置时建议保留 `waiting_configuration` 等可解释状态，向项目负责人提供一次性配置入口，普通成员完成任务不受影响。这个状态名称是建议，不是已有枚举。

## 项目与协作 AI 配置不能直接当作默认整理授权

1. 项目保存的是**有序小队候选列表**。`configured` 仅代表设置成功，不证明运行时在线；选择多个候选也不会广播任务。它用于新任务的默认分配，不会自动改派既有任务。证据：`server/internal/handler/project_execution_squad.go:23`；`.trellis/spec/server/project-execution-squad.md` 的 Authority / UI semantics。
2. 现有小队配置已经区分绑定权限与调用权限：模板运行时的私有机器仅拥有者可绑定，现成小队需检查实际 Leader 的调用权限。证据：`server/internal/handler/project_execution_squad.go:150`、`:175`、`:199`、`:220`。
3. 协作资源输入必须验证当前完整 roster、配置指纹、每项资源拥有者 grant、参与者 epoch 及当前拥有者状态。一次旧授权或目标登记不等于永久执行资格。证据：`server/internal/service/collaboration_authority.go:379`、`:398`、`:419`；`server/internal/service/collaboration_targets.go:259`。
4. 资源支持、在线/容量及拥有者同意是独立条件。证据：`server/internal/service/collaboration_runtime.go:32`。
5. 普通自动化的无现场用户执行已有可借鉴模式：重新解析触发器授权人、检查当前成员/调用权限，记录 originator、accountable、规则版本和触发证据。证据：`server/internal/service/autopilot.go:1154`、`:1177`。

因此不能按“任务最后用了谁”“项目第一支小队”“当前登录管理员能看到谁”直接选整理资源。若选 CLI 路线，至少需要一个明确的项目整理资源配置、可撤销的自动调用授权和独立执行身份；其 owner/原始授权人必须可读取被发送的材料。

## 验收触发点和已关闭队列

- 协作完成的可用事实是精确 `delivery_decision`：外部交付绑定当前 `SubmissionID`，且当前处理人为验收人、状态为 `in_review`、无未完成补充信息请求；自验收也先生成正式交付，再绑定 accepted。证据：`server/internal/service/collaboration_lifecycle.go:464`、`:476`、`:495`、`:508`。
- 完成或取消会关闭协作 session、取消等待命令并停止全部目标；重开也让目标维持暂停。证据：`server/internal/service/collaboration_lifecycle.go:523`、`:549`、`:553`、`:564`。
- 生命周期完成事件与精确决定在同一工作流事务写入，事件包含 `delivery_decision`。建议以这一持久事实的身份及版本建立唯一整理工作，不仅订阅易失的进程内通知。证据：`server/internal/service/collaboration_lifecycle.go:613`、`:622`、`:632`；`server/internal/service/collaboration_workflow.go:404`。
- `manual` 策略会阻止提交指令、恢复、资源提供/采纳/授权等操作；切回 assisted 不会恢复执行。证据：`server/internal/service/collaboration_ai_policy.go:21`、`:30`、`:40`。新的服务端整理也应显式检查这一产品策略，不能因不走 daemon 就绕过纯人工选择。

验收之后往原评论追加 @Agent 并自动 Resume，不是可用捷径。预生成可发生在提交后，但最终采纳仍须绑定被正式验收的那次交付，不能把退回版本的草稿自动继承给后一次验收。

## 持久后台工作模式

已有知识失效工作与 webhook worker 可借鉴：

- 来源写事务只追加一个稳定 cause 的持久工作，随后短事务扫描/投递；知识工作每轮有数量和时间上限。证据：`server/internal/service/collaboration_knowledge_invalidation.go:20`、`:32`、`:88`、`:109`、`:137`。
- webhook 队列及租约在 PostgreSQL，重启/副本接管能重新领取；本地通知只用于提速；并发有上限，处理方法可在测试中同步驱动。证据：`server/internal/handler/webhook_delivery_worker.go:19`、`:25`、`:52`、`:113`。
- 聊天标题的 detached goroutine 是允许静默丢失的装饰功能，不能直接作为需要追溯、重试和去重的知识工作可靠性模型。证据：`server/internal/handler/chat_title.go:75`、`:92`、`:103`。

建议新工作记录验收身份、来源项目、策略版本、材料版本/摘要、重试计数、下一次可用时间及租约；模型调用放在数据库事务之外，保存候选/采纳前再检查来源、项目关系和策略。主任务只原子保留整理意图，模型离线或出错不回滚已提交的验收。

## 普通任务与协作任务差异

普通任务状态更新走 `Handler.UpdateIssue` 的事务及可选 delivery completion enforcement；它没有自动等价于协作的 `delivery_decision/SubmissionID`。证据：`server/internal/handler/issue.go:4072`、`:4145`；`server/internal/service/delivery_completion.go:178`、`:294`、`:306`。

`EventTaskCompleted` 属于一次执行事件；现有自动化监听也将任务事件与 issue 更新分开处理（`server/cmd/server/autopilot_listeners.go:19`、`:54`）。不能拿它替代本需求的业务验收。建议第一期仍限定已有明确验收链的协作任务；普通任务扩展另定验收契约。

## 方案比较与待决策

| 方案 | 优点 | 代价及授权要求 |
| --- | --- | --- |
| A：部署级 `pkg/llm`，新持久整理服务 | 一次配置、无工具、不依赖个人机器；最小调用链 | 使用部署 API 费用；必须明确允许该层接收限定协作资料，登记新增消费者及用量/预算 |
| B：项目指定 CLI Agent | 可用团队已有模型账号及内网运行环境 | 新增资源选择、源权限、机器在线、持续调用授权、独立队列与会话；不能复活业务队列 |
| C：业务 Agent 在交付时附带结构化经验 | 可减少单独模型请求，交付中已有语境 | 纯人工交付或旧 Agent 无产出；无法独立保证资料完整，仍需服务端检查，难以覆盖全部自动整理需求 |

建议 A 为首期，C 作为后续可选输入优化；B 不在首期并行实现，避免三种执行身份同时进入新流程。

需要用户决定的最高价值问题：**是否接受系统用统一配置的服务端模型自动整理项目协作资料，并由该配置承担调用费用？** 如果选择 B，则资源授予方式和无资源时体验成为下一轮规划问题。此选择不会改变“消费已采纳知识仍受运行时支持约束”的已有事实；生成与消费是两条独立模型路径。
