# P0 后端协议回归与 P1 接口清单

日期：2026-11-14。范围：批准版本 `2026-11-14-v1` 的最小失败回归和当前源码检查。此轮未修改生产实现，未运行数据库测试、真实模型或部署。

## 已建立的失败证据

新增 `server/pkg/protocol/collaboration_knowledge_automation_test.go`，只使用现有 Go 类型/入口，编译成功后产生实际行为失败：

| 测试 | P0 结果 | 实际缺口 |
| --- | --- | --- |
| `TestKnowledgeAutomationPublicVersionsPreserveLegacy` | v1通过、v2失败 | 公共提议读者固定v1 |
| `TestKnowledgeAutomationDeliverySourcePreservesExactIdentity` | 失败 | 无delivery来源，无法保留submission/双event/hash |
| `TestKnowledgeAutomationRuntimeVersionsPreserveLegacy` | v1通过、v2失败 | 普通和协作运行请求都固定v1 |
| `TestKnowledgeAutomationManifestBindsReacceptedValidity` | 失败 | 新有效性字段被当前manifest解码静默丢弃，重验前后hash相同 |
| `TestKnowledgeAutomationManifestRejectsAmbiguousOrLegacyValidity` | 五个子例均失败 | 缺失/重复/未知字段、零epoch及v1携带validity被静默接受 |
| `TestKnowledgeAutomationDeliverySourceRejectsAmbiguity` | 通过 | 旧读者拒绝新来源；P1后继续约束v1、缺失身份、顺序及混合类型 |
| `TestKnowledgeAutomationPublicRequestCannotChooseSystemIdentity` | 通过 | 公共请求不得指定automation身份；不因v2开放系统写入口 |

命令：`go -C server test ./pkg/protocol -run '^TestKnowledgeAutomation' -count=1 -json`。
退出1，顶层5 FAIL / 2 PASS / 0 SKIP。原始证据：`p0-server-protocol-red.jsonl`。

原有知识协议基线另运行：

`go -C server test ./pkg/protocol -run '^(TestKnowledge(Proposal|Approval|Complete|Exact)|TestTaskKnowledge|TestCollaborationKnowledgeRuntime)' -count=1 -json`

退出0，15个顶层测试通过，包含子例共47 PASS、0 FAIL、0 SKIP。原始证据：`p0-server-protocol-legacy.jsonl`。两次运行均移除继承的 `MULTICA_LLM_*`、`OPENAI_*`、`ANTHROPIC_*`、`DATABASE_URL` 与真实Agent开关；`pkg/protocol` 无TestMain，不访问数据库。只对新增文件运行gofmt。

## 已与主会话及Core实施者对齐的字段

delivery source：

```json
{"kind":"delivery","issue_id":"11111111-1111-4111-8111-111111111111","submission_id":"22222222-2222-4222-8222-222222222222","delivery_event_seq":"11","accepted_event_seq":"15","sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
```

v2 manifest entry在既有candidate/version/content_sha256/source_issue_id/target后添加可选有效性元组：

```json
{"validity":{"revision":"1","accepted_event_seq":"15","submission_id":"33333333-3333-4333-8333-333333333333","validated_close_epoch":"3"}}
```

v1不能携带该元组；v1序列化不增加null字段，原hash/回执不变。主会话确认使用可选指针时，比较必须按字段内容，不能让原来的 `==`/`slices.Contains` 退化成指针身份比较。

系统身份属于公共candidate metadata：`proposal_origin` / `decision_origin`（human或automation）、nullable human UUID、`automation`内job/policy/rule来源。运行时contribution不需要伪造人的身份；由外层版本检查其来源和validity能否表达。Core实施者负责系统projection正例；公开请求拒绝冒充系统的后端回归已建立。

## P1及后续接入点

| 归属文件 | 必要变更/约束 |
| --- | --- |
| `server/migrations/` | 当前最大编号843；主会话暂留844起。新增四领域表、候选origin/nullable actor与不可变规则、普通metadata CHECK允许1/2。每索引单独CONCURRENTLY，禁止FK/cascade。不能改写747历史迁移以假装已升级。 |
| `server/pkg/db/queries/collaboration_knowledge.sql`及新automation查询 | 现有创建/批准/拒绝/撤回SQL均按human UUID写入；新增内部系统原语并保留旧默认。候选SELECT显式列表也要包含新字段。 |
| `server/pkg/db/queries/task_knowledge.sql:17` | `SetTaskKnowledgeDelivery`当前固定写`knowledge_context_version=1`，需显式按保留记录版本写。 |
| `server/pkg/db/generated/`、`server/sqlc.yaml` | 运行sqlc同步类型，继续内部delivery/version的`json:"-"`通配覆盖；系统job/evaluation私有材料不能进入通用JSON。 |
| `server/pkg/protocol/collaboration_knowledge.go` | 显式支持公共1/2；delivery各必需字段、版本/顺序/同issue/hash；human请求仍不接收origin。contribution严格解析来源并由外层版本拒绝v1新形态。旧回执按原版本恢复。旧测试把2当未知的例子应改用真正未知3。 |
| `server/pkg/protocol/task_knowledge.go` | manifest严格对象/版本/entry/validity解析；保留v1字节；新增v2支持常量，不能全局替换1。 |
| `server/pkg/protocol/collaboration_knowledge_runtime.go` | prepare/bundle显式双版本；manifest与外层版本一致，v1无法表达的新来源或validity必须拒绝。 |
| `server/internal/service/task_knowledge.go:67` | DB版本、JSON version、descriptor、prepared manifest、authorization、observations按同一显式版本严格读取；`encodeTaskKnowledge`当前也固定1。所有已保留历史仍需读全。 |
| `server/internal/service/collaboration_knowledge_runtime.go:21` | 保留`knowledge:claim:v1`/`knowledge:prepare:v1`键；其内容显式双版本，不换键使旧读者误判无历史。claim/prepare/authorize/status版本与原请求绑定。 |
| `server/internal/service/collaboration_knowledge_material.go`、`collaboration_knowledge_sources.go` | 当前contribution构造会通过仅v1的proposal校验；正式delivery需复用 `collaboration_delivery.go`确切交付/accepted领域读取，hash完整result/basis结构。 |
| `server/internal/service/collaboration_knowledge_view.go` | 当前提议者是string、决定actor必填，projection固定1；按协商版本输出origin、系统审计、usage_status且维护restricted/source_unavailable材料遮蔽。v1完整范围不能漏自动行假报空集。 |
| `server/internal/handler/collaboration_knowledge.go`、`collaboration_runtime.go:171`、router CORS | 人类header/body协商一致；当前runtime query只有`1`，知识允许1/2但handoff/recording不能跟随放宽。 |
| `server/internal/service/collaboration_knowledge_resolve.go:92,149,156,363` | 当前manifest固定1且entry按`!=`比较；`knowledgeEligible`无source epoch/accepted有效性条件。P2统一接入当前自动validity，并让原manifest保持原证明。 |
| `server/internal/service/collaboration_knowledge_locks.go:20` | 当前只收集原candidate.sources；有效性证明的新增支持来源必须纳入同一锁/当前可用性集合。 |
| `server/internal/service/task_knowledge_prepare.go:146` | union当前固定1、使用`slices.Contains`；新指针元组需值比较，同candidate不同证明不能默默替代；prepare/replay/authorize均经过统一复查。 |
| `server/internal/service/collaboration_lifecycle.go:529,613` | reopen推进epoch；正式accepted event在finishStatusChange同事务创建。这里写轻量job/scan即可，不加入target锁或模型调用。 |
| `server/internal/handler/workspace_delete_manifest_test.go:97` | 新表必须逐项归类；已有candidate/invalidation保留、target删除语义继续有效，不能cascade删除系统历史。 |
| `server/internal/daemon/knowledge_context.go`及`collaborationrun/host_knowledge.go` | 新daemon明确协商2，响应/manifest/launch都校验对应版本；已披露v2历史不能无知识降级或另开会话重放业务。 |

## 本轮文件所有权与剩余验证

仅新增四文件：上述协议测试、本记录、red原始JSONL、legacy原始JSONL。没有改动既有源文件、规划或其他任务。

P0只证明协议缺口和既有纯协议基线。source重开与prepare/authorize双提交顺序、人工撤回优先、后台工作/模型、DB迁移、HTTP版本完整范围、旧slot读取、真实daemon输入和Web/Desktop旅程仍需后续在主会话分配的独立数据库/环境执行；不可把本轮红/绿结果称作产品AC已完成。
