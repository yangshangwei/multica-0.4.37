# P2：准确交付来源与自动知识消费有效性

状态：本消费lane已实现并完成具名验证。继承已批准 `2026-11-14-v1` 范围；不代表worker、公共UI或完整产品已验收。未提交、推送、部署或调用真实模型账号。

## 变更边界与共享契约

原缺口是knowledge source只解析memory/comment/run_record，消费链路未检查自动采纳的来源生命周期、精确验收与有效性证明。现已在既有source/resolver/locks、普通与协作prepare/authorize/history和daemon适配中补齐，不增加业务队列入口。

主会话授予本lane：`collaboration_knowledge_sources.go`、新的delivery/validity服务文件、`collaboration_knowledge_resolve.go`/`locks.go`/`material.go`、相关`task_knowledge*`、`collaboration_knowledge_runtime.go`、`daemon/knowledge_context.go`、`collaborationrun/host_knowledge.go`及窄版本接线`client.go`/`host_runner.go`/`host_ledger.go`/`host_session.go`，以及runtime handler知识版本query。SQL限新的`collaboration_knowledge_delivery.sql`及既有service查询的两条metadata SELECT、对应sqlc产物。公共candidate handler/view、生命周期入队、router与worker由其他lane拥有。30个本阶段改动文件及SHA256完整列于 `p2-consumption-source-sha256.json`；material构造本阶段无需再改，P1的严格latest2 contribution读者已支持delivery。

已提供的共享helper：

```go
func knowledgeDeliveryCanonical(result, basis string) (string, error)
func knowledgeAcceptedDelivery(ctx context.Context, q *db.Queries, issue db.Issue,
    submissionID pgtype.UUID, deliverySeq, acceptedSeq int64,
) (knowledgeDeliverySnapshot, bool, error)

// Snapshot: Source protocol.CollaborationKnowledgeSource;
// Result, Basis, Canonical string; DeliveryEpoch int64;
// AcceptedByUserID pgtype.UUID.

func knowledgeAutomaticValidity(ctx context.Context, q *db.Queries,
    row db.CollaborationKnowledgeCandidate,
) (knowledgeAutomaticProof, bool, error)

// Proof: Binding protocol.TaskKnowledgeValidity; EvaluationID pgtype.UUID;
// Dependencies []protocol.CollaborationKnowledgeSource.

func (s *CollaborationService) KnowledgeUsageStatus(ctx context.Context,
    q *db.Queries, row db.CollaborationKnowledgeCandidate,
) (string, error) // active | paused_review | inactive
```

这些helper借用当前查询/事务，不授予ACL。调用方仍检查当前viewer/consumer/source/audience。

delivery hash使用固定字段顺序的完整`{result,basis}` Go JSON结构；quote字段选择result或basis，与union身份分开。读取精确submission及delivery/accepted event，不用最新结果、评论或timestamp替代。

自动有效性由不可变`ApprovalOrigin == automation`决定。候选仍approved、精确candidate/version/hash/target、head/evaluation/job绑定、当前project、已关闭source epoch、当前最新正式lifecycle boundary及精确accepted交付均须成立。evaluation的`support_dependencies`是严格、最多8条的`[]protocol.CollaborationKnowledgeSource`，必须属于同一来源issue并包含本次精确accepted delivery；不能把比较材料的`input_source_closure`替代为正面依据。原candidate.sources和新支持来源共同进入锁与当前可用性检查；比较来源不成为正面依赖。

有效性引用的generation/check attempts必须真实存在、succeeded、属于该job/batch、阶段正确并互相匹配execution revision；检查请求的policy/source/target快照必须与immutable evaluation一致，evaluation支持依赖必须属于该检查实际支持集合。860引入显式重试的新策略快照后，不能将已有evaluation与job的原始policy或当前execution policy比较：已有有效知识不会因另一个批次重试而失效。

已有有效知识不要求当前项目policy仍开启或当前任务仍assisted；这些开关只约束新生产/复查。原触发验收者与retained job reader身份必须一致，但不额外引入其永久管理角色。人工批准的自动提议按人工采纳处理，不施加自动重开暂停。

在30项真实DB并发矩阵稳定通过后，主会话明确授权daemon默认协商latest2，已启用普通daemon和host runner两个生产声明点。原v1常量、显式v1请求/读者、回执和协作固定DATABASE slot保持。host本地namespace按其retained version使用knowledge:v1/v2：v1原字节不变，已有记录的协议删除/改写会因key不匹配而拒绝，不是换数据库slot隐藏历史。

普通/协作server的claim、存储、prepare、authorize、observations沿精确协商版本传播。`KnowledgeConsumerFacts.ProtocolVersion=0`只表示已有可信内部调用的legacy1；运行事务显式传递当前claim版本。v1不能静默省略或消费自动条目/新来源。含自动validity的已曝光会话若当前复查失败，返回`knowledge_context_changed`，不自动换新proof或fresh session；已有human-only失效会话行为保留。用户明确建立的新fresh binding可以准备当前有效proof。

来源锁顺序保持：用户/凭据→工作空间/成员→目标revision→来源→候选→共享validity head及准确材料→运行时/task。delivery handoff及双event与原memory/comment/run材料都被精确锁定；head的evaluation ID/revision在锁后再次匹配预读提示。先前未锁定的支持材料不能在`ValidateKnowledgeResolutionLocks`时成为正文。

## 迭代证据

- `p2-delivery-red-4af7654e.json`：真实正式交付/验收后，当前 `KnowledgeSourceAvailability` 返回false；1个具名FAIL、无skip，源码稳定。证明准确delivery来源尚未实现。独立测试DB，未调用真实模型。
- `p2-validity-red-dc658aa5.json`：delivery/hash回归6项已过；自动proof夹具误用有id的Insert创建无id目标counter，setup失败，已改为InsertNoID。保留原失败，不能算产品缺陷。
- `p2-validity-red-corrected-3d42ee6f.json`：3项预期RED，v1未明确拒绝自动候选、缺失proof仍被注入；并行实现期间source_stable=false，只作迭代证据。
- `p2-consumption-green-8108323a.json`：13 PASS / 1 FAIL，源码稳定；唯一失败是夹具在已完成任务上调用memory更新，被正确以issue_closed拒绝。资料准备已移到正式交付前。
- `p2-daemon-knowledge.jsonl`：首轮110 PASS / 12 FAIL。原因是两个旧纯夹具构造请求时省略版本，新fixture正确echo后成为0；以及新拒绝用例误写failed而非既有启动前blocked状态。补齐夹具版本、修正状态断言，未弱化生产检查。后续 `p2-daemon-knowledge-corrected.jsonl` 为122 PASS、0FAIL/SKIP，含v2普通进程真实stdin及v1新形态零进程。

## 最终具名验证

| 证据 | 结果 |
| --- | --- |
| `p2-consumption-matrix-43927f54.json` | 新独立DB，`-race`，30/30 PASS、0 FAIL/SKIP，source_stable=true，结束DB连接0 |
| `p2-source-isolation-final-4e575904.json` | 新独立DB，`-race`，2/2 PASS、0 FAIL/SKIP，source_stable=true，结束DB连接0；同项目其他来源独立性及更新后的精确来源夹具 |
| `p2-daemon-v2-activation.jsonl` | `Knowledge|HostLedger`，138 PASS、0 FAIL/SKIP；test-created provider、CLI guard、Git外HOME/TMP、无账号凭据 |
| 静态/编译 | `go -C server build ./...`、`go -C server vet ./internal/service ./internal/daemon ./internal/daemon/collaborationrun`通过；30个拥有文件diff --check与gofmt清洁 |
| Tagged包 | `go test -c -tags=agentintegration ./internal/daemon/collaborationrun`仅编译通过；知识query许可同步1/2、3拒绝，未运行任何真实账号测试 |

DB矩阵证明：

- delivery完整result/basis与转义、非ASCII、U+2028/U+2029、双event/submission/hash准确，重开不删除合法历史材料。
- approved自动候选必须有完整proof并使用v2；缺少head/job/evaluation不能退化成legacy知识。
- 重开与真实prepare/authorize两种提交顺序，消费者使用非参与者的独立成员，观察真实PostgreSQL阻塞，避免共同用户锁掩盖来源锁缺失。没有运行outbox drain。
- 重开后的cleanup observation保留；下一次physical authorization不能沿用原proof。
- 新支持memory进入锁提示，清除后旧manifest拒绝；重新验收本身不自动恢复，新增validity才能恢复。
- 旧会话在重开/重验后明确拒绝；新明确fresh binding可取得新revision。人工批准的自动提议不暂停，同项目另一来源自动知识保持有效。
- 当前项目policy关闭或任务变manual不追溯撤销历史有效知识。

daemon/host测试证明：普通和协作真实测试进程收到准确delivery正文，授权绑定v2manifest及实际stdin hash，physical/native/termination观察保留2；host重启读取v1/v2本地ledger且拒绝降级或抹去知识历史；既有会话隔离、原回复恢复、无持久会话与受控CLI保护继续通过。

## 完成边界

这些是实际数据库、事务竞态、严格协议和test-created进程的产品组件证据。模型记录为受控fixture；不证明真实模型语义质量。Web/原生Desktop完整业务旅程、完整worker生产与UI集成由主会话后续P6整合；此lane不提前改判那些AC，也不宣称已经部署或打包安装验证。
