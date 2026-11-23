# P1 API 与跨子任务数据合同

状态：已实现 v1，本轮 P1 实施验收完成（2026-10-06）；部署状态独立记录。health/progress/ui/foundation 共同依赖本文件；[design.md](design.md)定义业务、锁与存储，[test-spec.md](test-spec.md)定义证明。本文是线协议唯一来源；没有新增产品范围。

## 1. 通用表示与端点

JSON 使用 snake_case。UUID为标准小写带连字符文本；timestamp为UTC RFC3339Nano；date为YYYY-MM-DD；版本为正整数，计数为非负整数。所有示例使用类型记法，`T|null`字段必须存在，不能省略后由fallback补“成功”。新增接口成功响应必须携带并校验workspace/project身份；旧Project对象新字段可选，保持混合版本兼容。

| 端点（`P=/api/projects/{project_id}`） | 成功响应 / HTTP |
| --- | --- |
| `GET /api/workspaces/{valid_ws_id}/project-capabilities` | 200 `{workspace_id,schema_version:1,overview,updates,description_cas,planning_timezone}`，四能力为boolean |
| `GET/PUT /api/workspaces/{ws_id}/planning-timezone` | 200 `{workspace_id,planning_timezone:string|null,effective_timezone,configured}`；PUT输入`{planning_timezone:string|null}` |
| 既有 `PUT P` | 200 原ProjectResponse加`revision,description_revision`；输入沿用design §4；旧计数不改义 |
| `GET P/overview` | 200 `ProjectOverview`；统计输入无法取得仍能确认身份时200 complete=false，各未知计数null；身份/DB整体不可用503 |
| `GET P/health/issues` | 200 `RiskPage`；signal/limit/cursor/version见§2 |
| `GET P/updates` | 200 `{workspace_id,project_id,items:ProjectUpdate[],next_cursor:string|null}` |
| `GET P/updates/{update_id}/revisions` | 200 `{workspace_id,project_id,update_id,items:UpdateRevision[],next_cursor:string|null}` |
| `GET P/updates/{update_id}/revisions/{revision}/executions/{task_id}` | 200 `{workspace_id,project_id,update_id,revision,task:AgentTask,messages:TaskMessagePayload[]}`；实际执行与消息供既有执行记录对话框读取 |
| `POST P/updates/preview` | 200 `UpdatePreview`，无发布/通知写入 |
| `POST P/updates` | 首次201、重放200 `UpdateWriteResult` |
| `PUT P/updates/{update_id}` | 更正首次/重放均200 `UpdateWriteResult` |
| `GET P/delete-impact` | 200 `{workspace_id,project_id,project_revision,issue_count,formal_issue_count,resource_count,update_count,autopilot_count,preserves_issues:true,preserves_executions:true}`，仅管理员 |
| 既有 `DELETE P` | 204，无payload；项目缺失404，不复活已删除记录 |

能力路径在现有URL成员授权组（`server/cmd/server/router.go:1692`）添加；禁止使用旧`/api/projects/capabilities`，旧router将其当id并返回400（`:2136`、`handler/project.go:231`）。仅对已成功读取的同一工作空间，以合法UUID调用专用capability端点得到404才判unsupported；若工作空间详情随后也404/403，走删除/撤权。400是请求错误，401认证失效；workspace不可见或成员失权在真实通用中间件保留404及workspace not found，并附workspace_access_denied。该code直接走scope失权，不能降级为unsupported。无此code的专用能力404须先确认workspace仍可读；操作/来源级403按下表保留本地输入，网络/数据库不可用/畸形响应为探测失败，不冒充失权或空功能。

## 2. 非递归统计、概览与风险页

```text
ProjectStatisticsSnapshot = {
 workspace_id, project_id, project_revision, snapshot_version,
 calculated_at, reference_date, timezone, timezone_configured,
 complete:boolean, incomplete_reasons:string[],
 counts:{total,completed,cancelled,open,blocked,overdue,unassigned,in_review,
         risk_union,unknown_status,execution_environment_unavailable},
 closure_ratio:number|null, project_overdue:boolean|null, lead_valid:boolean|null,
 latest_update_at:timestamp|null, progress_age_days:integer|null,
 health:unavailable|empty|risk|attention|clear, reasons:string[]
}
ProjectOverview = {workspace_id,project_id,description_revision,
 statistics:ProjectStatisticsSnapshot,
 latest_acceptance:AcceptanceSummary|null,
 current_description_acceptance:AcceptanceSummary|null}
AcceptanceSummary = {update_id,revision,published_at,conclusion:passed|partial|failed,
 description_revision,applicable_to_current_description:boolean,author:HistoricalMember}
RiskPage = {workspace_id,project_id,signal,items:Issue[],total:integer,
 snapshot_version,refreshed:boolean,overview:ProjectOverview,next_cursor:string|null}
```

statistics只含计算事实，**不含进展对象、验收摘要、description文本或另一个snapshot**。完整统计counts均整数；不完整时已算出的数可保留，未知项必须null（unknown状态仍可得到N/U），closure_ratio仅分母/分子已知且N>0时有值，health=unavailable。全部来源失败不伪造N=0。

`latest_acceptance`从kind=acceptance稳定记录按`published_at DESC,id DESC`选最新，返回其current_revision；更正时间不改变顺序。`current_description_acceptance`仅在该集合中过滤description_revision=当前版本后按同序选最新，不能偏爱passed跳过较新的failed。两者可能不同：界面分别标“最近验收（旧版，请复核）”与“当前描述验收”，不能让较新的旧版更正覆盖较早当前版结论。无当前版则当前结果为null；图标等变更不影响版本匹配。

snapshot_version为design §5全部聚合输入的canonical digest，排除calculated_at、可见标签与排版。RiskPage只接受四signal；按`issue.id ASC`稳定keyset，默认50/最大100；cursor封装版本、signal、last_id并绑定ws/project（可解码校验，不是授权凭证）。按[ADR-05](research/pagination-adr.md)，版本变化仍refreshed=true，但有效cursor继续当前集合中严格大于last_id的位置；无cursor才返回第一页。overview、total、items始终属于本请求同一事务。next_cursor带当前version，客户端随后发送该版本；不混接旧页。UI的遍历变更提示保持到成功的显式从头刷新，失败不清；total>0的空后缀不称为项目无风险。新/重新入险的低ID需从头刷新才能包含，不宣称跨页是同一冻结快照。统计不完整时拒绝声称精确下钻，503并返回当前可授权overview用于错误提示。

## 3. 进展、历史与署名

```text
HistoricalMember = {id:UUID,name:string|null,avatar_url:string|null,
 availability:active|departed|deleted}
Acceptance = {conclusion,scope:string,explanation:string|null,
 description_revision,description_snapshot:string}
UpdateRevision = {
 workspace_id,project_id,update_id,revision,editor:HistoricalMember,created_at,
 kind:progress|risk|acceptance,body:string,
 health_judgment:on_track|attention|risk|null,
 correction_reason:string|null,evidence:EvidenceView[],
 statistics_snapshot:ProjectStatisticsSnapshot|null,acceptance:Acceptance|null
}
ProjectUpdate = {workspace_id,project_id,id,author:HistoricalMember,published_at,
 current_revision:integer,current:UpdateRevision}
UpdateWriteResult = {workspace_id,project_id,request_id,update_id,result_revision,
 replayed:boolean,result:UpdateRevision,author:HistoricalMember,published_at}
```

列表按`published_at DESC,id DESC`keyset，默认20/最大100；修订按revision DESC，默认20/最大100。cursor分别含最后时间/id或revision且绑定所有父身份。不因更正将旧发布移到列表首部；不支持单条硬删除。create返回revision1；correct返回下一revision；重放返回request当时的result_revision，不谎称它仍是current_revision；之后再次更正可通过列表取得最新版。

作者/修订者批量读取全局历史署名`GetUsersByIDs`（`server/pkg/db/queries/user.sql:9`），响应仅id/name/avatar，**不返回email**。有user无当前membership为departed；user已删除则id保留、name/avatar=null，UI稳定显示“已删除成员”。记录已授权后才查署名，不新增可任意枚举用户的接口。

## 4. 证据身份、版本与当前授权

```text
EvidenceInput = {kind:issue|execution|url,id:UUID|null,url:string|null}
EvidenceVersion =
 issue     -> {kind:"issue",id,revision:integer}
 execution -> {kind:"execution",id,state_version:integer,result_digest:string}
 url       -> {kind:"url",url,verification:"unverified"}
EvidenceView = {input:EvidenceInput,observed_version:EvidenceVersion,collected_at,
 availability:available|changed|deleted|inaccessible|unverified,
 current_version:EvidenceVersion|null,label:string|null,href:string|null}
```

| kind | 收集/比较版本 | 授权与回放读取 |
| --- | --- | --- |
| issue | 当前`issue.revision`（models.go:976）；同ID不同revision需复核 | workspace成员 + `GetIssueInWorkspace`，采用`loadIssueForUser`租户解析规则（handler.go:1102），事务内query器；历史不要求仍归该项目，移出仍可标changed；不以项目ID绕过原来源授权 |
| execution | `agent_task_queue.state_version` + SHA256 canonical `{status,result,error,completed_at}`，覆盖结果改变却未增加状态版本的路径；result原文不持久复制到引用 | `GetAgentTaskInWorkspace`；chat来源仅会话creator可读；其余读取agent并按`canAccessPrivateAgent`的owner/admin/public_to目标规则，绝不用canInvokeAgent代替读权限。证据：chat.go:1757、:1825；agent_access.go:157。P1提取接受qtx的只读判断，无调用/执行副作用 |
| url | 保留规范HTTP/HTTPS URL，无内容版本；不加入fetch时间 | 仅安全协议/URL检查，不抓取或宣称验证；外部内容变化无法自动识别，UI显示未验证 |

证据收集、正式提交及每次读取历史独立授权。提交使用成员fence与locking membership read；来源实体/agent/会话/适用allowlist行在同一RR事务中`FOR SHARE NOWAIT`锁定再判定，变更/删除导致40001或55P03整笔重试，避免RR旧授权冒充当前授权；锁按kind/id排序。无法找到适用授权行时拒绝，不通过缺行推断已授权。读取历史不返回无权来源label/href/current_version，只有原身份/采集时刻与inaccessible；所属workspace撤权则整条记录拒绝，不能用历史快照绕过。

2026-10-05 集成审查补充：内部 issue/execution 证据的 `href=null`，客户端依据 `input.kind/id` 与记录的工作空间身份导航；issue 使用既有工作空间路径，execution 使用上表只读执行证据端点并复用现有执行记录对话框。只有外部 URL 证据提供 HTTP(S) href。执行端点要求指定修订确实引用该执行，在同一 RR 事务重新校验项目、当前成员、private-agent/allowlist 或 chat creator 权限后返回真实任务及消息；未引用／父身份不匹配返回404，当前无权返回403。客户端严格核对 workspace/project/update/revision/task 身份及消息结构。此补充不新增任务页面，不复用仅验证工作空间的旧消息读取接口扩大证据权限。

## 5. Preview、提交与canonical hash

```text
UpdateDraft = {operation:create|correct,update_id:UUID|null,expected_revision:integer|null,
 kind,body,health_judgment:null|enum,evidence:EvidenceInput[],
 acceptance:{conclusion,scope,explanation:null|string}|null,
 expected_description_revision:integer|null,include_statistics:boolean,
 correction_reason:string|null}
UpdatePreview = {workspace_id,project_id,draft:UpdateDraft,
 evidence_versions:EvidenceVersion[],description_revision:integer|null,
 recipients:HistoricalMember[],statistics_snapshot:ProjectStatisticsSnapshot|null,
 preview_hash:string,previewed_at:timestamp}
WriteInput = {request_id:UUID,draft:UpdateDraft,preview_hash:string,
 evidence_versions:EvidenceVersion[]}
```

correct draft的kind与验收描述版本从原记录固定，不允许改kind/绑新目标；passed/partial的依据条件仍检查。新验收必须expected_description_revision=当前项目版本；旧验收correct沿用原适用版本并显式标旧版，更正不要求把旧版本变当前。acceptance.description_snapshot仅服务端生成。create的include_statistics=false保存null；correct为false保留上版snapshot原值（不重新采集），true则按当前preview重采；hash的statistics_version仅在true时有值。

规范化固定：body/理由/范围/说明统一CRLF→LF后trim，保留中间空白、不做Unicode NFC转换；UUID小写；URL用标准解析保留query/fragment、拒绝用户名密码和非HTTP(S)；所有optional显式null；enum严格检查；evidence保留用户顺序，重复同kind/id或相同URL首个保留。JSON对象key按Unicode码点字典序，数组顺序保留，整数十进制；canonical serializer由服务端单一helper定义，用固定测试向量验证，不新增依赖。

`preview_hash = SHA256(canonical({contract_version:1,workspace_id,project_id,actor_user_id,draft,evidence_versions,acceptance_description_revision,recipient_ids_sorted,statistics_version}))`。无统计时statistics_version=null；有统计时为本次snapshot_version，**不包括calculated_at/collected_at/previewed_at**。非验收的acceptance_description_revision=null；更正验收使用原版本。收件者从正文解析并当前授权，客户端不能提交任意recipient_ids。

正式create/correct在RR写事务重新规范化、授权、计算所有比较项和统计；hash或evidence_versions不相符则409 preview_stale，提供已授权的changed_fields与新preview，UI重新确认后用新request_id提交。include_statistics=true时实际发布snapshot必须与preview的statistics_version相同，否则409；成功保存本次写事务的`calculated_at`和actual observed snapshot，不能原样信任客户端preview数字或用第二事务贴入。include_statistics=false时任务无关变化不导致统计冲突。

请求幂等hash独立于previewhash：SHA256 canonical `{contract_version,workspace_id,project_id,actor_user_id,operation,update_id,draft,preview_hash,evidence_versions}`。同request/hash先回放历史result，不再因新preview状态重发通知；仍先确认项目/访问权。不同hash409 idempotency_conflict。hash只是变化检测与去重，不是签名/权限凭证。

最小流程fixture：预览普通进展`body="本周完成登录流程"`、空evidence/null acceptance/不附统计，返回hash H；POST携request R/H返回201 result_revision1；完全重试R/H返回200 replayed=true且相同id/revision；更正文案需preview operation=correct+expected_revision1+理由，再以新request R2提交返回revision2；重放R仍返回revision1，不隐式覆盖revision2。

## 6. 错误合同与响应校验

P1 handler新增错误沿用现有`{error:string}`基础，加`code:string,field_errors?:{field,message}[],current?:object,retryable?:boolean`；通用认证中间件401可能只有error，仍按HTTP认证失败处理。current只含当前授权范围内数据。错误不能被parseWithFallback当成功。成功身份/必填revision/完整snapshot枚举不合法均是协议错误，保留输入并显示重试；旧Project可选新字段不影响旧CRUD。

| HTTP | 固定code与payload | 客户端行为 |
| --- | --- | --- |
| 400 | `invalid_request`，可选field_errors（UUID/signal/cursor/日期语法） | 不当unsupported、不自动改值 |
| 401 | `unauthenticated` | 清认证/保护缓存并登录 |
| 403 | `forbidden`（工作空间／成员访问已撤销） | 清保护内容/候选/草稿，停止重试 |
| 403 | `project_evidence_forbidden`（private-agent／chat来源失权，附evidence字段提示） | 保留本人草稿；清除该证据查询，移除失效引用后重新预览；历史仅脱敏该来源 |
| 403 | `project_updates_disabled`（P1新发布／更正已关闭） | 保留草稿并显示只读限制；当前授权的历史及完全相同已成功请求仍可读取／重放 |
| 403 | `project_permission_denied`（操作角色／机器actor限制） | 显示操作权限不足；不据此撤销整个工作空间访问 |
| 404 | `workspace_access_denied`，保留既有 `error:"workspace not found"`（不存在与无权同形状） | 真正scope失权：清保护Query/草稿/候选并挡住迟到响应；不当unsupported或project删除 |
| 404 | `project_not_found`/`project_update_not_found` | 停止相应提交；已删除项目只保留本人未提交文本供复制，不能恢复对象；仅无scope拒绝code的专用能力路径按§1判unsupported |
| 409 | `project_description_conflict`/`project_revision_conflict`/`project_update_revision_conflict` + current | 保留输入、显示最新版本、显式合并 |
| 409 | `project_update_preview_stale` + `{current:{changed_fields,preview}}`；`idempotency_conflict`无保护正文 | 复核后新意图；不得自动覆盖 |
| 422 | `validation_failed` + field_errors | 依据/字数/证据/IANA/日期顺序逐项显示 |
| 428 | `project_description_revision_required` | 明确客户端版本限制、保留输入 |
| 503 | `project_health_unavailable`或`project_write_retry_exhausted`，retryable=true，Retry-After:1 | 不展示最新成功；写沿用原request_id重试 |

测试必须包括真实旧router请求合法ws子资源404及旧错误路径`/api/projects/capabilities`400；新端成员200/非成员404且workspace_access_denied，操作权限不足403且project_permission_denied；400/401/403/404/network/malformed矩阵；所有DTO缺identity/错误workspace、未知kind、不安全整数、负counts、递归/畸形snapshot不得假成功。列表历史署名/多验收排序/旧更正及三种证据版本变化使用同一合同fixture。

真实路由验证：planning-timezone PUT经过成员级路由，handler在锁内重新检查owner/admin并返回操作级403；成员离开后project/overview读取在通用中间件返回稳定scope404。数据库读取故障保留503且不发失权code，避免服务故障擦除用户状态。对应project_permission_route_test.go与workspace_access_error_test.go。
