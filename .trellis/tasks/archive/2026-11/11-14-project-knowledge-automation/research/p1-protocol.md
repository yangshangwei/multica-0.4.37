# P1：知识双版本协议与保留历史读者

2026-11-14。完成范围是主会话分配的协议、严格保留历史读取及版本边界；不是自动整理/系统采纳实现完成或产品AC全部通过。无提交、部署或真实模型调用。

## 已完成行为

- 公共提议、批准、决定及原操作回执明确支持1/2，继续拒绝未知版本、额外/重复字段和客户端指定automation身份。原v1回执保持原字段与序列化字节。
- 新增准确delivery来源：`issue_id`、`submission_id`、`delivery_event_seq`、`accepted_event_seq`、`sha256`。事件序号为规范正int64十进制串，accepted必须晚于delivery；拒绝混合comment/run/memory字段。v1提议和运行响应不能带delivery。
- manifest严格解析完整对象、entry、可选validity；v1不允许validity，v2人工entry省略该字段，显式null也拒绝。自动有效性绑定`revision/accepted_event_seq/submission_id/validated_close_epoch`；四字段各自影响manifest hash。
- entry比较按validity内容而非指针地址。相同候选的不同证明不能在会话历史合并中被替换或重复保留；明确拒绝。完整manifest/contribution校验要求版本一致、候选顺序/身份对应，并对v1新来源执行拒绝。
- 普通持久记录允许外层1/2：v1只含v1 claim；v2可以原样保留v1 claim并容纳v2 claim。每个claim的manifest、authorization、observation版本必须与其descriptor相同；数据库版本必须与JSON外层相同。编码使用原外层版本，不改写旧hash。
- 普通prepare/authorize/observe在锁内要求请求版本等于原claim版本；拒绝时没有metadata/pointer写入。v1无法重新领取已协商v2的保留claim，也不能在读取v2会话后通过“新会话”兜底继续。
- 普通history union保留最高已存在版本；当前claim不能降级其累计历史。所有`SetTaskKnowledgeDelivery`调用显式携带已解码外层版本，类型为`pgtype.Int4`，继续保留sqlc内部字段`json:"-"`。
- 协作保留槽位仍为`knowledge:claim:v1` / `knowledge:prepare:v1`。slot内claim/preparation/authorization支持严格双版本读取；preparation的请求/manifest必须同版本并匹配hash。已有claim恢复要求原协商版本；status返回原preparation版本。

三个原`*KnowledgeVersion`常量仍为1；新增`*KnowledgeLatestVersion=2`。没有改daemon生产协商、没有启用新的v2 claim writer，也没有新增系统采纳公开入口。

## 提供给后续实施者的API

| API | 用途 |
| --- | --- |
| `IsKnowledgeProtocolVersion(int) bool` | 只接受1/2 |
| `CollaborationKnowledgeLatestVersion`、`TaskKnowledgeLatestVersion`、`CollaborationRunKnowledgeLatestVersion` | 后续显式协商2；不要替换旧常量或重写原回执 |
| `CollaborationKnowledgeSource.SubmissionID/DeliveryEventSeq/AcceptedEventSeq` | delivery精确来源；quote的result/basis字段归属模型statement，不加到来源union |
| `TaskKnowledgeValidity` | `Revision`、`AcceptedEventSeq`、`SubmissionID`、`ValidatedCloseEpoch` |
| `TaskKnowledgeManifestEntry.Validity` | 可选指针，v1必须无、v2人工省略 |
| `EqualTaskKnowledgeManifestEntry(a,b)` | 含validity值比较；不要用entry `==` 或 `slices.Contains` |
| `ValidateTaskKnowledgeManifest(m)` | 严格版本、canonical identity、排序/唯一性、有效性形态 |
| `ValidateTaskKnowledgeManifestContributions(version,m,items)` | 完整外层版本与贡献的可表达性和对应关系；供普通/协作daemon共享 |
| `ValidateCollaborationKnowledgeContributionVersion(c,version)` | contribution没有自身version，必须依据其外层校验v1可表达性 |
| `ErrCollaborationKnowledgeProtocolUnsupported` | 新HTTP协商/投影边界可使用的固定错误码 |

没有给被嵌入的`CollaborationRunKnowledgePrepare`添加`UnmarshalJSON`，避免该方法被提升后吞掉bundle/authorization字段；具体外层具有严格解码器，独立请求继续由HTTP严格body读取加`ValidateCollaborationRunKnowledgePrepare`验证。

## 文件责任

本子代理修改的生产文件：

- `server/pkg/protocol/collaboration_knowledge.go`
- `server/pkg/protocol/task_knowledge.go`
- `server/pkg/protocol/collaboration_knowledge_runtime.go`
- `server/internal/service/task_knowledge.go`
- `server/internal/service/task_knowledge_prepare.go`
- `server/internal/service/task_knowledge_observe.go`
- `server/internal/service/collaboration_knowledge_runtime.go`

测试文件：更新`collaboration_knowledge_test.go`中“未知协议”反例为3，扩展P0 `collaboration_knowledge_automation_test.go`；新增service `task_knowledge_version_test.go`与`task_knowledge_version_integration_test.go`。

主会话曾先为prepare/observe的三处数据库写入添加显式版本，之后将这两个完整文件的版本边界责任交给本代理；这些已授权整合被保留。SQL/generated/模型/Core/Views没有由本代理修改。源文件SHA256见`p1-protocol-source-sha256.json`。

## 测试证据

| 阶段 | 结果及证据 |
| --- | --- |
| P0协议RED | 原`p0-server-protocol-red.jsonl`保留：5顶层FAIL / 2PASS；P1没有覆写该证据 |
| P1存储RED | `p1-storage-red-f350805f.json`：9具名运行、6PASS / 3FAIL，稳定源码；v2读取及按值union缺口 |
| P1真实事务RED | `p1-version-boundary-red-88c32401.json`：prepare/authorize错误提交、v1读v2后错误fresh fallback；5具名运行、1PASS / 4FAIL。`source_stable=false`，记录为迭代观察，不能作为冻结最终证明 |
| 纯存储GREEN | `p1-storage-unit-green-final.jsonl`：11具名PASS、0FAIL/SKIP；这些函数不接数据库，按主会话明确允许的无DATABASE_URL精确纯测试选择运行 |
| 全部protocol最终 | `p1-protocol-final.jsonl`：61个顶层、含子例451PASS，0FAIL/SKIP |
| protocol/daemon/host兼容 | `p1-runtime-compatibility.jsonl`：75顶层、含子例210PASS，0FAIL/SKIP。通过agent CLI guard，以Git外独立HOME/TMP运行，只选Knowledge测试，无数据库及真实模型凭据 |
| 冻结服务端GREEN | `p1-service-green-ff3e22d5.json`：15顶层、含子例25PASS，0FAIL/SKIP，`source_stable=true`，独立DB，结束连接数0 |
| 静态/编译 | 所有11个拥有的Go文件gofmt清洁；`go -C server build ./...`、`go -C server vet ./pkg/protocol ./internal/service`退出0 |

最终service数据库：`mca_knowledge_service_8bf207f48ab0`；migration ledger为847条、最新859；完整Server源码冻结hash：`5a4aaf79b2f15cecaf31bb7a31188590fdec26fe49f445ac527b02588d23c3ad`。

最终service选择覆盖纯storage/union，以及实际`TestKnowledgeRequestsMustMatchRetainedClaimVersion`、`TestKnowledgeV1CannotResumeV2Exposure`、claim finalization、prepare原请求丢响应恢复、session pointer/exposure原子绑定、来源变更后launch拒绝、native observation原回执，以及collaboration原始输入/精确manifest/协商不可升级降级。测试使用真实领域服务/隔离数据库；v2历史由测试夹具模拟，未开启生产v2写者。

## 后续必须整合的边界

- `knowledge_resolve.go`、`collaboration_knowledge_locks.go`仍由后续阶段添加自动validity读取与新的支持来源闭包；用上述值比较helper替代原entry `!=`/`slices.Contains`。消费依据候选不可变`ApprovalOrigin == automation`，不是最新`DecisionOrigin`或`ProposalOrigin`。
- delivery正文读取仍待领域实现：hash统一规范化的完整`{result,basis}`结构，精确引文独立选result/basis；需转义/Unicode回归，Frontend可消费服务器提供的准确ref。
- 公共HTTP header/body协商、完整范围v1不可表达拒绝、系统身份/usage_status projection及Core/UI仍待后续slice；仅DTO双版本不等于HTTP/安装版客户端已完整切换。
- 普通/协作fresh writer和daemon仍保持v1，等待完整读者、权限/有效性/历史检查接好后才显式启用2。协作`lockCollaborationKnowledge`与fresh claim仍保守限制1；P2切换时需按原claim精确版本绑定请求。
- P2/P3完整自动job、模型规则、重开双提交顺序、人工决定优先、完整重验分批、真实Web/Desktop旅程未由本slice覆盖。真实模型语义可靠性、真实账户及部署仍在已批准边界之外。
