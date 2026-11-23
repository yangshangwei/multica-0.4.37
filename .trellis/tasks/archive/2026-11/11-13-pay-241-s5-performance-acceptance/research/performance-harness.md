# Research: PAY-241 独立协作者与回写性能采样工具

- Query: 为 S5 准备最小、可复现的 100 独立账号状态传播及受控智能体回写采样方案。
- Scope: internal；只读研究与本研究文件，不修改产品、数据库或 Git，不启动 Provider。
- Date: 2026-11-13
- Status: PREPARATION_ONLY。S2 产品规则与 S1–S4 联合验收尚未全部完成，本文件不声称 S5 或 TC24 通过。

## Findings

### 1. 实际可复用入口

本仓库没有 `server/pkg/ws/`。浏览器 WebSocket 的权威实现是 `server/internal/realtime/hub.go`、`collaboration.go`；共享客户端对应 `packages/core/api/ws-client.ts`。不得按不存在的 pkg 路径假定认证或 ACK 协议。

| 能力 | 位置 | 已有合同 |
| --- | --- | --- |
| 注册独立密码用户 | `e2e/fixtures.ts:71` | `registerPassword(username,password,name)` 经 `/auth/register`，保留返回 token，返回真实 user ID |
| 真实密码登录 | `e2e/fixtures.ts:56` | `/auth/login`，返回真实 user ID 并更换该 TestApiClient 的 token |
| 取得已发凭证、授权 HTTP | `e2e/fixtures.ts:394`、`:399`、`:464` | `getToken()`、`requestJSON()`，每实例独立 token；不可给 100 个数组项放同一客户端 |
| 标准 host 账号 | `e2e/fixtures/collaboration-host-runtime.ts:407` | `createHostActor()` 复用注册并 POST `/api/me/onboarding/complete`，不要求打开浏览器 |
| 公开创建协作任务 | `collaboration-host-runtime.ts:513` | POST `/api/collaboration/issues`，指定 source workspace、当前真实 handler、description、attachment_ids |
| 公开邀请参与者 | `e2e/fixtures/collaboration-resources.ts:58`；`server/cmd/server/router.go:1695` | POST `.../participants`，每次精确当前 revision/epochs + target_user_id + Idempotency-Key |
| 参与者权威清单 | `server/internal/service/collaboration_participants_view.go:24` | GET `.../participants` 返回当前 unrevoked participants 与 `account_active`，没有分页；停用账号仍可能出现在清单，不能只看数量 |
| 状态选项与状态写 | `server/cmd/server/router.go:1705`、`:1766`；`packages/core/api/client.ts:1927` | GET `.../status-options?action=change_status`；PATCH `/api/collaboration/issues/{id}`，不是猜测的 POST `/status` |
| 转交责任 | `server/cmd/server/router.go:1696` | POST `.../handoffs`，`purpose=transfer`；该动作不接收 `expected_status/status_options_version` |
| 独立当前 Issue GET | `collaboration-host-runtime.ts:20`–`:21`；`core/collaboration/schema.ts:214` | `/api/collaboration/issues/{id}?execution_view_version=6`；revision 和 event_seq 是十进制字符串，按 BigInt 比较 |
| 受控 native host | `collaboration-host-runtime.ts:176`、`:218`、`:345` | 明确已构建 daemon/provider/CLI、安装身份绑定、等待实际 visit、脚本化 CLI 写、release 和真实退出证明 |

`requestJSON()` 返回 generic 类型，性能脚本仍应对需要作为证据的响应使用现有 schema 或明确严格断言；HTTP 200 自身不证明观察到了目标版本。此 helper 的 `API_BASE` 在模块加载时读取环境（`fixtures.ts:13`）；须先配置 `NEXT_PUBLIC_API_URL` 与 fixture API 同源，不能只给函数传一个不同的 apiUrl。

### 2. 100 个账号的可用准备方式与实际限制

推荐为每次验收生成唯一 run suffix 和 `p000`–`p099` 账号，创建 100 个不同的 TestApiClient，保存 `{actor_index,user_id,username}` 的非秘密清单。令牌和密码只留在内存或明确的权限受限临时文件，不能写入公开 JSON/日志。注册/登录后每个客户端调用 `/api/me`，确认 100 个 distinct ID，且 ID 与准备清单一一对应。

密码注册存在真实速率限制：`server/internal/handler/auth_password.go:132` 是每 IP 30 次/5 分钟；登录 `:176` 是每 IP 300 次/分钟、`:191` 是每账号 10 次/5 分钟。因此直接 Promise.all 注册 100 个会产生 429。可行的最小方式是把注册作为计时之外的独立准备阶段，按最多 30 个/窗口分四批；主会话继续其他工作，定时恢复准备，不把等待吞成性能样本。也可复用**本任务隔离数据库**中已预置并逐个真实登录的 100 个 fixture 用户，但必须记录 setup 是预置、不能宣称经公开注册验收。此次未找到专用的批量管理员账号创建 API；不要凭空调用，也不要清限流或冒用人用账号。

创建一个本任务自有 source workspace 和一个公共 API 创建的协作 Issue。creator/初始 handler 可是 actor 0，其余 99 人逐个公开邀请。每次邀请先以 creator 读取当前 Issue，再提交以下正文：

```json
{
  "target_user_id": "<this distinct user's UUID>",
  "expected_collaboration_revision": "<current revision>",
  "expected_lifecycle_epoch": "<current lifecycle>",
  "expected_participant_epoch": "<creator's current participant epoch>",
  "expected_account_epoch": "<creator's current account epoch>"
}
```

每次使用一个新、持久记录的 Idempotency-Key；同一次未知结果只能原键恢复。此处不要原样展开 `hostAuthority()`，它还带 `execution_view_version`，而普通 workflow body 的严格解码只认以上四个版本字段（`handler/collaboration_workflow.go:18`、`:129`）。邀请不能并发使用同一个预读 revision；`InviteParticipant` 在同一事务内调用真实 enroll、账号 fence、操作和通知（`service/collaboration_workflow.go:92`、`:180`）。当前未找到批量邀请公开入口，99 次顺序 setup 是合理的，不必为了 setup 新增产品 API 或直接插 participant 表。

所有 100 账号随后都要成功 GET 同一 Issue 且 `capabilities.read===true`；参与者清单必须恰好包含这些 distinct ID，并逐项 `account_active===true`。这些账号可作为跨工作空间参与者，不必把 100 人都加为来源 workspace member。资格仍由实际 `collaborationRecipientEligible()` 的当前启用/密码主体规则决定（`service/collaboration_recipient_eligibility.go:21`），遇到拒绝要登记准备失败，不能为凑数绕过。

### 3. WebSocket 认证、订阅和匹配

每账号创建单独 Node 22 原生 `WebSocket`，不必增加 `ws` 依赖或打开 100 浏览器页。项目要求 Node >=22（根 `package.json`），运行前确认 `typeof WebSocket==='function'`。构造 API 同源 `/ws?scope=account`，协议改为 ws/wss，不附 workspace 参数、不把 token 放在 URL。`hub.go:854` 会拒绝 account 与 workspace 参数混合。

精确握手：

```text
open → send {"type":"auth","payload":{"token":"<this user's token>"}}
wait → {"type":"auth_ack"}
send → {"type":"subscribe","payload":{"scope":"issue","id":"<issue UUID>"}}
wait → {"type":"subscribe_ack","payload":{"scope":"issue","id":"<same issue UUID>"}}
```

依据：`packages/core/api/ws-client.ts:104`、`:147`、`:261`；`hub.go:953`、`:1107`、`:1172`。无 cookie 的原生连接使用第一帧认证；不要把 `open` 当已认证。无 Origin 或同源 Origin 是服务器接受的原生路径（`hub.go:161`）。`subscribe_error`、auth error、close/error 或 ACK 超时均是独立账号未就绪，不得把缺少的观察者悄悄排除。

被测状态提示：

```json
{
  "type": "collaboration:changed",
  "event_id": "<durable UUID>:issue",
  "payload": {
    "issue_id": "<issue UUID>",
    "event_seq": "<positive decimal>",
    "collaboration_revision": "<positive decimal>",
    "authority_epoch": "<positive decimal>"
  }
}
```

`CollaborationChangeSchema` 是 payload 权威 schema（`core/collaboration/realtime-schema.ts:5`）。durable `event_id` 来源是 `service/collaboration_realtime.go:117` 和 `realtime/collaboration.go:177`；最终 issue scope 发布在 `service/collaboration_realtime.go:155`。`collaboration:workbench_changed` payload 是空对象，不含任务/版本，不能当成本次状态传播成功。

当前 relay 每 500ms 扫描至多 100 条事件（`service/collaboration_realtime.go:64`）。每条状态变化还会向每个参与账号发布 account hint，再发布 issue hint（`:133`–`:155`）；采样包括这个真实扇出成本，不通过直接调用 hub 或手工 DrainOutbox 绕过。最终物理发送前会再次认证和重查 scope（`hub.go:1218`、`realtime/collaboration.go:68`），100 个独立授权主体确实进入当前权限路径。

### 4. 预登记采样设计

建议保守的主口径为 **一个 API 写请求发出前 → 该观察者收到对应持久状态事件后，用自己的 token GET 并验证完整目标版本**。同时记录写响应确认点。主口径含写请求与事务耗时，较“commit→可见”更宽；本机不能从 HTTP ACK 精确知道数据库 commit 单调时刻，不将 ACK 命名成真实 commit。可另报 ACK→可见，但不能隐去响应之前已观察到事件的样本或凭后置起点得到虚假的更快结果。

所有时点来自一个 runner 的 `process.hrtime.bigint()`，统一相对 runStart 的毫秒数；UTC 只供审计关联，不做持续时间减法。每次写至少记录：

```text
event_index, writer_user_id, request_id, target_issue_id,
before_revision, intended_status, request_start_ms, write_ack_ms,
write_http_status, receipt_revision, event_id, event_seq,
observer_user_id, socket_id, first_event_ms, read_start_ms, read_end_ms,
observed_revision, observed_event_seq, observed_status,
latency_ms, event_latency_ms, read_attempts, duplicate_frames,
result, error_code, deadline_ms
```

步骤：

1. 等待 100 次身份核验、100 auth_ack、100 精确 issue subscribe_ack、100 个当前授权 GET；再做 5 次标记为 warmup 的真实状态变化。预热和正式样本在原始文件中都保留，但按预登记字段区分。
2. 正式 100 次状态变化；让 actor 0–99 **各担任一次真实当前 handler 并发起一次变更**，这样不只有一个作者和 99 个被动 socket。上一轮 handler 用公开 `handoffs` 转给下一人，转交回执与来源记录保留，但转交准备不混入本轮状态 p95。各 actor 以自己的 GET/version/status-options 发出 PATCH，在当前允许状态里交替 `todo/in_progress`，不使用 done/in_review 绕过阶段规则。
3. 每轮写前注册观察 waiter 并保存预期下一 revision；写回执确认目标 revision。事件可能早于写响应，应缓冲并记录其原始时刻，用实际 receipt revision 关联，不能丢掉早到事件。为避免丢掉中间版本，只在上一轮 100 观察者完成或其预登记截止后开始下一轮。
4. 每个观察者只对匹配 issue + revision + event_seq 的 `collaboration:changed` 开始自身授权 GET。校验 status 与完整 revision 等于该轮回执、读取的 `realtime.event_seq` 覆盖该持久事件。重复帧记数、按 event_id 去重，不重复创建样本。其他 account hint、先前 handoff event、未知帧不启动本轮成功判定。
5. 正式分母是 **100 events × 100 independent observers = 10,000 observer-event samples**。报告 pooled p50/p95/p99/max、每观察者的 100 样本分布、每事件最慢观察者分布，以及 HTTP/WS/GET 分段耗时。至少同时报告 pooled p95 和 100 次最慢观察者耗时的 p95，避免平均掩盖固定慢用户。
6. 每轮可留 10 秒最终收敛截止，预算仍为 2 秒；2 秒以上继续记录真实耗时并判定该样本超预算。socket 断开/授权失败/未收到帧/GET 未收敛均保留，不用纯 GET polling 兜底成成功。未知/失败不得设为 0；若缺失落入分位点，p95 标记不可给有限上界。即使有限成功样本 p95 合格，只要有未收敛，该次运行整体仍不满足完整传播验收。
7. 预登记中止策略：若准备不足 100 人，整场 NOT_RUN；若正式运行中止，所有剩余计划项保留 NOT_RUN，不把前几轮重新定义成“完整场景”。结束前再次逐人读取 final state，保存 distinct 身份/观察者/请求数量断言。

建议注册/开户 setup 并发最多 2–4，WS 打开并发最多 10；正式扇出 GET 允许 100 个独立观察者同时读取。连接、预热和写事件间隔都在 manifest 固定，不在看到 p95 后改测试负载。记录 CPU/RAM、操作系统、Node/Go/DB 版本、API pool 配置、是否 Redis/多副本、进程 PID、提交与脏文件 hash、源码匹配的 API/Web/Desktop 构建身份、并行测试负载。

该协议证明 **API/state convergence**，不包含 React Query、组件调度或像素绘制。若同时开三类角色 Web 会话和一个 Electron 页面，可另测 DOM 可见并保留来源，但必须写明只测少量真实 UI 客户端，不能把这部分外推为 100 个 UI 页面渲染性能。旧 4 账号 100 GET 或当前项目两页性能测试也不能替代这里的 100 身份 WS 分母。

### 5. 结果回写测量：现成 host fixture 的可靠边界

必须避开 S1 fixture 的误导起点：`collaboration-multi-account-provider.go:188`–`:209` 在发回 visit callback 之前已经执行 memory update 和 comment add；到 `:247` callback 才带回结果。故从 `daemon.release()` 开始量它已存在的 comment 会得到近零值，完全漏掉被测回写。

普通 `collaboration-host-provider.go` 更适合测量：

- `:364`–`:391` 等待 `a<JSON line>`，再调用当前 run-local CLI 执行；`runFixtureAction` 的 comment 路径在 `:61`–`:62`。
- `collaboration-host-runtime.ts:370` 的 `daemon.act(visit,[{kind:'comment',request_id,content}])` 在 socket 发送后等待真实 CLI 原请求回执。
- `daemon.release()` 在 `:351`–`:354` 发送 `r`；provider 随后在 `host-provider.go:394` 写出真实受控 final result，daemon 再向 API 写进度/终结事实。
- `e2e/collaboration-multi-account-closure.spec.ts:245` 已有最终权威读条件：精确 run ID、`progress_state='settled'`、`proof_state='host_exit_observed'`、`launch_count='1'`、`business_outcome='success'`。此处仅复用合同，当前性能必须重新运行。

建议分开报告两个指标，均用同一 runner 单调时钟：

**业务结果回写**：t0 在调用 `daemon.act()` 前；同时挂起 exact comment GET 观察，不先 await act 再开始计时。终点是某个当前有权的人类观察者 GET `/api/collaboration/issues/{issue}/comments/{request_id}` 成功且匹配内容摘要、issue ID、author_type=agent、确切 author_agent ID。CLI receipt 必须随后匹配同一 request/comment ID；run records 复核该 comment 归于实际 run/agent/runtime。GET route 的确存在（`router.go:1738`），无需遍历不断增长的所有评论再计时。可以每 25–50ms 重读一次并记录实际轮询间隔和首次可见区间；**一条回写是一个样本，多个 GET 尝试不是多个样本**。

**final result/终结回写**：t0 在 release 前；终点是权威 run 查询同时满足上述 settled、退出证明、success。该耗时包括本地 provider 输出、退出和 daemon/API 终结写入，应命名为受控 finalization convergence，不叫模型推理耗时。不能拿仅有成功退出事件或 callback socket close 代替 API 已可读的结论。

可预登记 1 个预热 run、20 个正式独立 run，每 run 5 次不同 request_id 的短结果写，共 100 次业务回写和 20 次 finalization 样本；每个正式 run 只有一次 launch。三个已有角色 fixture 可轮换作者资源，或先用一个明确受控资源，报告范围必须一致。provider callback 有 150 秒 deadline（`host-provider.go:346`），每 run 5 次结果允许保留失败同时避免一个长期 run 被 fixture 总时长污染。运行排队/启动、结果写、终结分别计时；调用额度/模型质量不属于该确定性数据。

每个样本保存 actual run/command/request/comment IDs、Agent/resource owner、人类发起身份、t0/回执/权威可见时刻、结果摘要 hash、同请求原结果证据。未收敛/拒绝/重复派发不丢弃；两项 p95 均按完整预定样本报告，业务回写目标 5 秒；finalization 单列同预算及事实，不混成有利平均。

现有 `daemon.act()` 等回执的 `expect.poll` 可能增加等待粒度，因此终点应由并行授权 GET 首次验证时刻决定，而不是 act promise resolve 时间。多个 daemon 的内部 `started_ms/released_ms` 各有起点（`:233`），不能直接相减；runner 自己统一记录调用边界即可。

### 6. 最小工具实现建议与文件归属

- 一个独立 `e2e/collaboration-performance.spec.ts`，复用现有 Playwright runner 但主测试不请求 browser fixture；不是新造 100 页面脚本。任务规划中显式命名 canonical suite，避免每天复制。
- 一个必要时拆出的 `e2e/fixtures/collaboration-performance.ts`，负责独立账号/WS observer、采样 journal 和 percentile。可以直接使用 Node 原生 fetch/WebSocket/process.hrtime，现有 pg/Playwright；不新增依赖。
- host helper 不需要为 t0 改产品，先用现有 createHostActor/enrollHostRuntime/adoptHostTarget/daemon.act/release。只有复用入口缺少必需的纯测量 hook 时才最小改 fixture，不能给产品写“快速跳过权限”开关。
- 原始 `samples.jsonl` 增量写入并 flush；manifest、setup identities、summary、failures、源码/构建/environment hashes 独立输出。只把 summary/索引和适量原始文本纳入阶段任务；密钥/完整凭证/含凭证 host ledger 不进入仓库。
- 结束时先关闭全部 socket、等待/停止受控运行及真实终结证明、关闭 test daemon，最后调用 fixture gate restore。原始失败文件和自有数据库保留供审阅，不删除其他任务数据。

状态传播测量无 Agent 时只需 collaboration enabled/creation enabled；host 测量使用 `openHostAcceptanceGate()`（`:106`），它记录原布尔值、恢复时递增 epoch，不倒退 epoch。旧 namespace fixture 必须保持 host gate false，不混用两种运行测试。优先各自显式独立 DB/lane，避免一个测试恢复 gate 干扰另一个仍运行的 fixture。

## Related specs

- `.trellis/spec/server/collaboration-realtime.md`：当前权限、持久 outbox、事件补读与 Query 收敛。
- `.trellis/spec/server/verification-environment.md`、`database-test-isolation.md`：API 和测试数据库、构建/进程匹配及精确许可。
- `.trellis/spec/server/collaboration-host-execution.md`：受控 host 真实进程与当前权限/终结合同。
- `.trellis/spec/web/frontend/e2e-run-environment.md`：production Web 验证；开发服务器冷编译不能混进产品性能判定。
- 父任务 `design.md` 第 5 节与 `test-matrix.csv` TC24：起止定义、完整样本和 100 独立观察者。

## Existing comparison, not acceptance evidence

`e2e/projects-p1.spec.ts:8`–`:85` 是两个真实页面、30 次项目统计变化的独立 canonical 量测；可参考逐样本原始输出和分位数算法，不能把其身份数、项目 overview 或旧 5 秒目标套到 S5。此次在 e2e/scripts 未找到现成 100 个独立账号 WS 状态测量工具。

## External references

没有新增 SDK/API 选型。本方案基于现有 Node >=22、项目公开 API、当前 WebSocket 协议和明示受控 fixture；实际平台/版本在执行 manifest 固定。

## Caveats / Not Found

- 这里是可实施采样设计，未创建账号、未拨号 WS、未写数据库、未启动任何 daemon/模型、未测 p95。
- 100 个账号的注册速率窗是实际准备约束；S2 尚待产品决定期间可安排独立准备，但不能跳过限流或把不足 100 人记为通过。
- fixture password 身份能证明真实平台授权路径，不能代表真实业务人员签字；受控 Provider 不代表真实模型能力。
- 本机单进程测量包含同机网络、API 和最终授权读取成本；不能据此外推跨地域、多副本或生产容量。
- API 收敛与 100 人 UI 渲染是不同指标；最终 TC24 和父任务状态由主会话依据预登记合同与实际完整证据判定。
