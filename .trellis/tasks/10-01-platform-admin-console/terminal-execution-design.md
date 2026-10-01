# 终端与执行详细设计

状态：拟定方案，未实现；覆盖 PRD R06–R10、R14、AC04–AC08、AC12。首期按 100–1000 个受管理安装实例设计。
源码事实见 `research/terminal-current-state.md`；平台权限与账号撤销见 `security-design.md`，容量与指标口径由主设计统一。

## 1. 对象边界与首期能力

后台管理的“终端”定义为一个部署内、一个 OS 用户持有的 Multica 安装身份，不等同物理电脑，也不等同某次登录。
界面称“受管理安装实例”，可显示管理员命名的“终端名称”；同一机器多个 OS 用户形成多个实例，物理资产合并留待后续。
首期允许：查看运行元数据、取消单次执行、停止接单、恢复接单。排除任意 shell、屏幕控制、批量动作、重跑、进程强杀接口及跨机器迁移。
平台写权限不会授予私有正文、日志、路径、原始错误文本和产物访问权；操作结果只返回结构化 code 和脱敏摘要。

## 2. 身份层次

| 标识 | 来源与用途 | 是否可作为管理认证 |
| --- | --- | --- |
| `user_id` | 现有登录账号，同一人可登录多个终端 | 只证明人；平台角色另验 |
| `install_id` | 现有客户端使用统计 UUID，可能随 StorageAdapter 清理变化 | 否，仅统计别名 |
| `device_id` | 现有 Electron userData 身份，用于旧设备登录路径 | 否，不能继承为平台权限 |
| `daemon_id` | 现有 `~/.multica/daemon.id`，同 HOME 的 CLI profiles 共用 | 否，只是声明标识 |
| `installation_id` | 拟新增服务端 UUID，绑定安装公钥与部署 | 必须同时验证持钥证明 |
| `binding_id` / `binding_epoch` | 拟新增服务端安装—daemon—工作空间绑定及单调代次 | 控制请求必须匹配当前有效绑定 |
| `runtime_id` | 现有工作空间中的执行引擎/provider/profile | 从已验证绑定和 DB 反查，拒绝任意列表认领 |

同一 OS 用户下桌面 profile 和 CLI 共享新管理身份文件，按后端部署 ID 分区：`~/.multica/management/{deployment_id}/installation.json`。
此文件保存服务端安装 ID 和密钥，权限 0600、原子写入；以排他创建锁解决同时启动的首轮生成竞态。文件读取/签名由 main/daemon 执行，不向 renderer 暴露私钥。
保留数据重装沿用原实例；删除管理身份、换 OS 用户或换 HOME 产生新实例。管理员可记录 replaced_by 关系，但不自动继承控制权和旧绑定。
多人轮流登录同一 OS 用户时实例稳定；参与用户从验证过的会话记录，负责人与最近登录人分开。账号换人需重新授权绑定，不能继承旧账号权限。
不同 backend 的相同 `daemon_id` 不代表同一个管理对象；同一内部组织的多个工作空间通过绑定关联同一实例，统计按 installation_id 去重。

## 3. 安装接入与绑定证明

使用 Node/Go 标准库 Ed25519，服务端保存公钥，避免新依赖。该证明确认凭据持有者，不宣称硬件可信或防止 OS 管理员复制私钥。
1. 已登录桌面 main 生成安装密钥。人类密码会话申请 enrollment challenge；服务端从会话和默认组织推导 user/organization，不接受请求自报归属。
2. challenge 记录部署、user、auth_version、公钥指纹、随机 nonce、有效期（建议 2 分钟）和未消费状态；客户端签名固定版本的规范字节串。
3. 服务端事务重验账号版本、nonce、签名；一次性消费 challenge，创建或返回同一公钥实例。重放仅返回既有结果，不能换组织/用户/公钥。
4. daemon 用自己的现有认证证明工作空间访问权，同时提交安装签名的绑定 challenge。两份证明绑定同一个部署、workspace、daemon、公钥、nonce、auth_version。
5. 服务端锁安装与绑定，确认 workspace 属于组织、用户有访问权、daemon 原绑定未冲突，再赋予 binding_id/epoch。连接凭据附带绑定范围，不能签发人类或平台凭据。
6. 新 daemon 注册的 runtime 经服务端绑定上下文关联，不按请求里的 installation_id 建立关系；已绑定 daemon 的 legacy register/heartbeat/claim 也必须验证当前绑定，不能换 PAT 冒充。

签名输入固定包含 protocol version、deployment_id、HTTP method/path、nonce、expires_at、body hash；避免跨接口/部署重放。
绑定 challenge 通过 main 启动 daemon 的继承匿名管道传递；已有 daemon 只允许经已认证本地 IPC 交接。无法证明本地对端时要求受控重启，不依靠 localhost/IP/hostname 判断可信。

首期绑定的桌面登录主体与daemon凭据主体必须是同一user_id，auth_version均为该账号当前版本；不隐式代理另一用户。两个主体不同、workspace不可访问或当前daemon范围不匹配均拒绝。以后共享执行节点需要单独授权模型，不能放宽本规则来兼容。

challenge请求按purpose区分：`enroll`输入public_key；`bind`输入installation_id、workspace_id、daemon_id、public_key及客户端持有的expected_binding_epoch（首次可空）。服务端校验安装组织、当前人类账号、成员关系和候选daemon范围，返回challenge_id、nonce、expires_at及signature_payload。daemon实际认证在兑换时再次验证并与签名主体一致，发行challenge不等于绑定已获准。

`signature_payload`为服务端生成的UTF-8 JSON字节的无padding base64url，字段固定且总是出现：`protocol_version,purpose,challenge_id,deployment_id,organization_id,user_id,auth_version,installation_id,workspace_id,daemon_id,public_key_fingerprint,expected_binding_epoch,nonce,expires_at,method,path,body_sha256`；不适用字段为null。UUID为规范小写，auth_version/epoch用十进制字符串，expires_at为UTC Unix秒十进制字符串；purpose/method/path使用闭集常量。

服务端按上述字段顺序、无空白、无重复key序列化一次并保存原字节hash及相关结构化范围。main/daemon先解码核对purpose、部署、主体、目标和时效，再直接签收到的原字节，不各自重新序列化JSON。兑换时服务端读取原challenge重新核对同一payload/hash，Ed25519验证签名，校验proof外的请求参数与签名内一致，事务消费nonce。body_sha256针对发行阶段固定的规范业务参数字节，不含signature和传输凭据，响应给出该字节串供双方核验；无法匹配即拒绝，不做宽松JSON等价比较。
管理协议的每次请求使用当前绑定凭据并校验 source auth_version；WS 连接在入站操作与出站提示前重新验证。重连重新取得短期 nonce/session，不复用旧连接授权。
同时证明不同公钥但声明相同 daemon_id 时返回 `binding_conflict`，保持现有归属，进入诊断；不得“最后注册者覆盖”。换绑必须由旧安装持钥者或有权限的具名管理员发起并审计。
可疑克隆不按 hostname 自动合并；凭据丢失走重新接入及旧绑定撤销。仅持有 install_id/device_id/daemon_id、管理员名下普通 PAT 都不能夺取绑定。
普通注册和未绑定旧 daemon 保持原流程。旧运行时在管理台显示“未关联”，不推测资产归属，也不开放终端级控制。
首轮接入只确立接入时刻以后的身份，不追溯证明旧任务来自该安装。旧运行时的名称/IP/daemon_id 即使与新端一致，也只是迁移候选。
复用既有 runtime 前必须有该 runtime 当前有效且 daemon 范围受限的凭据证明，以及安装签名；单凭用户 PAT + 自报 daemon_id 不允许认领别处的 runtime。
缺少旧端持有证明时，不自动迁移历史 runtime；由具名管理员记录旧对象退役并让新端接入。新对象接入后，旧对象的历史执行保持“未关联”，避免错误合并。

## 4. 三条状态轴与新鲜度

| 轴 | 展示值 | 判定证据 |
| --- | --- | --- |
| 客户端活跃 | 活跃 / 不活跃 / 未上报 | 新桌面签名上报的服务端接收时间；现有日活仅用于历史统计 |
| daemon 可达 | 可达 / 不可达 / 未知 | 当前有效绑定的 heartbeat/连接及既有 runtime liveness；不是窗口是否打开 |
| 执行准备度 | 可接单 / 停止接单 / 环境不可用 / 无权限 / 未知 | admission、有效绑定、可用 runtime、凭据和现有 claim gate；实际任务仍受队列与并发条件约束 |

每个轴附 `observed_at`、`source`、`freshness`、`reason_code`；没有探测数据用 null/unknown，不把缺失变成零、离线或正常。
“可接单”只表示具备基础条件；忙碌数量/剩余并发单独展示，不能保证任意任务立即领取。
桌面实时上报建议 60 秒 ±20% 错峰，活跃窗口 180 秒；程序休眠/退出之后按窗口变为不活跃，不要求成功发送退出包。
daemon 保留现有默认 15 秒 heartbeat、60 秒 DB 刷新、30 秒 batch tick、150 秒 stale gate；管理功能不得单独拉长其中一项。
连接失效、运行环境健康和任务终止宽限是不同规则；沿用现有 sweeper 对在途任务的保护，管理台不基于“窗口关闭”取消执行。
机器维度信息按安装去重上报，不让每个 provider 重发完整资产；1000 台新增客户端上报约 17 次/秒，另计现有 runtime 心跳。
数据库或 liveness 读取失败时展示状态不可用及最后已知时间，不发布全体终端离线告警。

## 5. 数据模型（拟新增）

所有 ID 使用服务端 UUID；下表为逻辑字段，不直接指定迁移编号。组织实体沿用总架构命名。

| 表 | 关键字段与约束 |
| --- | --- |
| `organization` / `organization_workspace` | 组织及 workspace 归属；现有 workspace 不改成客户实体 |
| `managed_installation` | id, organization_id, public_key, key_fingerprint, key_version, lifecycle, responsible_user_id?, display_name, desktop_version?, os?, client_seen_at?, admission, admission_version, replaced_by?, created_at, updated_at |
| `installation_daemon_binding` | id, installation_id, workspace_id, daemon_id, principal_user_id, auth_version, binding_epoch, state, capability_version, authenticated_at, last_seen_at, revoked_at?；历史撤销记录保留 |
| `admin_operation` | id, organization_id, actor_id, actor_auth_version, kind, target_installation_id?, target_task_id?, binding_id?, binding_epoch?, execution_fence?, root_operation_id?, idempotency_key, payload_hash, reason, state, version, ack_deadline?, accepted_at, applied_at?, confirmed_at?, result_code?, reconciliation_state |
| `admin_audit_event` | id, operation_id?, actor_id, organization_id, action, target_kind/id, phase, sanitized_before/after, result_code, request_id, recorded_at；只追加 |

`admin_alert` 在监控设计定义，关联 operation_id；操作账本不承担任务调度职责，不替换 `agent_task_queue`。
短期 enrollment/binding challenge 需持久表（`installation_challenge`），保存 nonce hash、请求 hash、会话版本、过期/消费/结果；不依赖服务内存以承受重启与双实例并发。
参与用户历史以独立 `installation_user` 关联记录验证来源与 last_seen_at；不保存桌面登录密码，不把参与者自动变成负责人。
安装别名仅记录已证明接入方报告的 install_id/device_id，带来源与时间，不设为授权查找键；历史日活无法可信回填绑定。
新执行增加 `submitted_installation_id?` 和已解析的执行安装/绑定快照；创建端从验证会话附带的证明确定，Web/旧端保留 null，禁止依据随意 header 归属。
`execution_fence` 为既有 task ID + runtime ID + dispatched_at；queued/deferred 使用服务端锁定状态版本。retry 子执行用各自 task ID，绝不把取消套到后来重试。

索引族：安装 `(organization_id, lifecycle, id)`、`(organization_id, updated_at, id)`；唯一公钥指纹 `(organization_id, key_fingerprint)`；安装用户 `(installation_id,user_id)`。
绑定：active `(workspace_id,daemon_id)` 唯一；`(installation_id,state,workspace_id)`；runtime 通过已验证 workspace/daemon 匹配，不另存可分叉的 runtime ID 列表。
操作：`(organization_id,actor_id,idempotency_key)` 唯一；`(state,ack_deadline,id)`；`(target_installation_id,accepted_at,id)`；`(target_task_id,execution_fence,kind)`及`(root_operation_id,state)`普通索引。不同管理员对同一task/fence允许各自存在操作记录，不加会阻止记录第二个请求的唯一约束。
审计：`(organization_id,recorded_at,id)`、`(operation_id,phase)` 唯一防止重复阶段事件；challenge nonce hash 唯一并索引 expires_at。
状态/时间/类型以 CHECK 与服务层共同约束。所有索引用独立单语句 `CREATE [UNIQUE] INDEX CONCURRENTLY`；禁止外键、级联及内联 UNIQUE 隐式建索引。
关联删除由应用事务处理；有操作/执行/审计引用的安装只退役不硬删。敏感内容保留策略不会删除核心操作结果和主体标识。

## 6. 接口契约（拟新增）

| 接口 | 认证和请求 | 返回 |
| --- | --- | --- |
| `POST /api/installations/challenges` | 有效人类密码会话；enroll/bind判别输入、目标范围见§3 | challenge_id, nonce, deployment_id, expires_at, signature_payload及规范业务参数字节 |
| `POST /api/installations/enroll` | 原会话 + challenge_id, signature, desktop_version, os | installation_id, key_version；幂等接入 |
| `POST /api/daemon/installation-bindings` | daemon 原认证 + 安装签名、绑定 challenge、workspace_id | binding_id, binding_epoch, capability_version；绑定凭据只在受控通道返回 |
| `POST /api/installations/{id}/heartbeat` | 安装证明 + 有效用户会话；boot_id, sequence, version, declared_capabilities | server_time, next_report_after；同 boot 重复/倒序不倒退状态 |
| `GET /api/admin/installations` | 平台读权限；cursor, limit≤100, organization, state, version | items, next_cursor, as_of；三个状态轴 |
| `GET /api/admin/installations/{id}` | 平台读权限 | 元数据、参与用户、bindings、runtime 摘要、能力、可用操作 |
| `GET /api/admin/tasks` | 平台读权限；task/source/user/installation/runtime/status/time 筛选 | 分页执行元数据，不含正文与日志 |
| `POST /api/admin/installations/{id}/admission` | 平台写权限；admission=`stopped\|accepting`, expected_admission_version, reason；Idempotency-Key | 202 + operation_id, state, applied_at, confirmation |
| `POST /api/admin/tasks/{id}/cancel` | 平台取消权限；expected_execution_fence, reason；Idempotency-Key | 202 + operation_id, state, ack_deadline |
| `GET /api/admin/operations/{id}` | 平台读权限且组织范围匹配 | 操作阶段、结果、证据时间、可重试的查询提示 |
| `GET /api/admin/operations?idempotency_key=...` | 平台读权限；固定当前actor与授权组织范围，不接受其他actor冒查 | 0或1条自己的原操作；没有记录时可用同key重试，已有记录时返回原结果 |
| 现有 task 状态/取消 ack | 原 daemon 认证与绑定证明；ack 增加可选 operation_id, binding_epoch, execution_fence, outcome | 沿用原返回，平台操作结果在服务端关联；无需新增取消兑换接口 |

入参严格限制大小、枚举和 UUID；错误 401=凭据失效、403=权限不足、404=对象不可见、409=版本/绑定/幂等冲突、410=过期、503=认证或存储暂不可用。
幂等键统一来自Idempotency-Key header，与规范化非秘密请求hash、actor、组织绑定；同键同请求返回原operation，不同非秘密payload返回409。未知提交结果按key查询；0条不证明请求绝不会提交，重试仍用原key让唯一约束协调，不能另换键。凭据恢复排除密码的专门规则见security-design.md。
管理列表使用脱敏 DTO，`content_access` 根据当前业务权限计算；日志链接仍访问原鉴权 API，不能加入 admin 绕过分支。
请求端仅提交目标 ID，organization、安装归属、runtime/fence 从服务端读出并交叉校验。前端禁用按钮不是授权判据。
HTTP 与 WS 共用 service、认证上下文和 schema；新 capability 名暂定 `managed_installation_v1`、`admin_cancel_ack_v1`。

取消请求示例（Idempotency-Key 由调用方稳定保存到查询结果确定）：

```json
{"expected_execution_fence":{"runtime_id":"<uuid>","dispatched_at":"<RFC3339>"},"reason":"执行持续无响应，已联系负责人"}
```

操作响应示例：

```json
{"operation_id":"<uuid>","state":"applied","result_code":"awaiting_daemon_confirmation","applied_at":"<RFC3339>","ack_deadline":"<RFC3339>","confirmed_at":null}
```

状态响应须包含当前 task 状态与独立的 confirmation 字段；`task.status=cancelled` 与 `confirmation=unconfirmed` 可以同时成立。
queued/deferred 的 fence 使用服务端返回的 target version，不能要求客户端虚构 dispatched_at。时间由服务端生成，客户端不能选择无限期 deadline。
收到 operation 终态后 Query 更新操作/终端/执行缓存；中间态用有界轮询，WS 只加速失效。离开页面不终止已经受理的后台操作。
账号退出或角色撤销清空相关管理 Query 缓存，后续轮询经过当前会话重新鉴权，不能依靠浏览器残留的角色缓存。

## 7. 操作生命周期与取消

首期操作主状态：`applied → succeeded`，目标已终态时 `succeeded` + `already_terminal`；确定执行失败可进入 `failed`，所有主终态不可反向改写。
受理记录、目标状态和 applied 阶段在一个事务提交，不创建需要终端兑换的 pending cancel。未来尚未应用的异步命令才增加 pending/expired，当前不实施。
`applied` 表示服务端已提交动作；`succeeded` 必须说明完成依据：服务端门禁已生效、排队任务取消，或 daemon 已确认停止。
取消在途执行使用 ack_deadline（建议 10 分钟）作为确认等待窗口；到期仍维持 applied，`reconciliation_state=unconfirmed`，不能宣称进程已停止，也不恢复已取消的任务。
终态操作迟到证据只追加审计，不改变原 result_code；同一证明重放不产生多份审计。task 本身沿用终态 CAS 与结果字段不覆盖规则。

1. 管理员提交取消：事务锁定账号/授权、target task 当前状态与 fence。completed/failed等终态写`already_terminal`的无变更结果；cancelled且已有管理根操作时按下述并发请求规则关联。原业务路径已cancelled但没有管理确认记录时只返回`already_cancelled_unverified`，明确本地进程状态未知，不伪造停止成功。
2. queued/deferred 且从未投递的任务，在同一事务内复用显式用户取消写入，结果 `cancelled_before_dispatch`，无须虚构终端 ack。
3. dispatched/running/waiting_local_directory：同一事务立即复用显式用户取消状态/聊天指针写入，写 intent + applied + audit；是否在线不改变这个语义。
4. 事务提交后才广播现有 task:cancelled，唤醒现有取消监测；daemon 沿用状态轮询及重连协调，即使通知丢失也能发现持久终态。
5. daemon 复用现有进程取消、日志刷出、工作目录保存和 `AckTaskCancelled`；扩展可选 operation_id、binding_epoch、execution_fence、outcome。
6. ack 事务验证同一次已应用取消、当前有效绑定及 task.cancelled；保留已有分支/错误写入的 CAS，原子写 confirmed/result/audit；然后完成原 deferred chat 与广播协调。

已取消任务是持久期望状态；即使超过 ack_deadline，daemon 重连仍按原任务协议停止旧执行。这是已提交状态的协调，界面不可称为“取消请求已过期”。
未来未应用命令若加入 expires_at，必须在服务端提交和终端执行前均校验，过期不重放；不能把这个 TTL 套到已提交取消或 admission 策略。
取消响应丢失时按相同 operation 查询/重试，服务端返回已应用结果；daemon 从任务终态恢复停止流程。断电重启无法证明旧进程状态时只记录“不再发现此执行”，不能伪装正常 ack。
完成回调与取消并发由 task CAS 决胜：完成先提交则取消 `already_terminal`；取消先提交则迟到 complete/fail 不覆盖 cancelled 和原有结果。
到期后再次点击取消返回原 task 终态和可查询的原操作；不会自动重跑、撤销取消或生成新执行。

### 多管理员取消同一执行

每个不同actor/key拥有自己的admin_operation和请求审计。第一个在task行锁下应用取消的操作成为根，root_operation_id为空；同task/fence后续请求仍在该锁下查根，创建自己的操作并以root_operation_id指向它，只复用取消事实，不再次修改task或发送新的业务执行。根已确认则新请求直接引用该确认；根未确认则新请求同样显示applied/unconfirmed。

因此A和B的响应即使都丢失，各自都能按自己的key找回自己的operation ID；重试同key不再建行。根operation_id随现有daemon取消回执传递，合法ack确认根后，协调器按root_operation_id幂等同步子请求状态。根确认和自己的审计同事务；子请求同步可分批恢复，在跟随者尚未同步时查询同时返回根的确认状态和自己的审计更新时间，不返回互相矛盾的成功/失败。

请求审计唯一性为各自operation_id+request phase；根及每个子请求的状态阶段事件分别以自己的operation_id+phase去重。daemon证据只属于根一次，不复制成另一次终端执行。扫描器崩溃重启可从根终态补齐跟随者审计，不丢B的发起人和原因。task/fence锁决定根，应用层校验root的组织、kind、target与fence一致且禁止链式/循环root引用。

## 8. 停止接单与并发边界

停止接单是服务端准入策略，终端离线也可提交：同一事务更新 admission=stopped、admission_version+1，写 succeeded 操作和审计。
界面显示“服务端已停止新领取”，daemon 是否已看到此版本另列 `observed_admission_version`。断线不会使服务端门禁失效。
恢复接单同样比较 expected_admission_version；冲突返回 409，避免管理员旧页面覆盖另一管理员的停止决定。
不得取消/暂停/迁移现有在途执行；停止提交前已成功领取的任务保留资格，可能在之后才启动，页面提示“在途执行继续”。
在领取事务中对已验证安装行加共享锁，门禁写入取排他锁，以提交顺序定义边界；资格判定不能只靠 handler、内存缓存、WS 提示或客户端自觉。
现有 `ClaimTask`、单 runtime、HTTP batch、WS `tasks.claim`、旧 batch fallback、dispatched reclaim 必须汇入相同门禁。
dispatched reclaim 只准恢复停止生效前已持久领取的同一 fence；不得通过 reclaim 新增投递代次绕开停止。实现需保存“首次授权 admission_version”，明确已投递回执，无法证明则不重新投递。
deferred promotion 可以保持业务排队，但后续 queued→dispatched 必须过门禁。自动重试产生的新 task 同样禁止领取，不把停止当执行失败触发更多重试。
内部调用不能跳过：SQL claim/reclaim 也校验服务端解析的 binding/installation 与 admission。已绑定终端切换旧协议/PAT 不会脱离安装门禁。
绑定撤销/换绑也必须锁相同安装行并递增 epoch；在绑定查找与加锁间变化时重查，禁止用过期关联执行操作。
锁顺序需与既有 session→agent→task 协议整合：领取先解析候选但不信任，取得安装共享锁后按原资源顺序加锁，再重查 agent/runtime/binding。多安装按 UUID 排序。
需要密码源版本锁的事务先锁凭据主体，再安装，再既有资源；停止操作不反向申请 task 锁。取消事务复用现有聊天锁序，提交前重验授权，不在持锁期间做网络调用。
恢复后提交再失效 EmptyClaim/ReclaimCheck 提示并唤醒 daemon；丢提示由有界轮询发现，不能使永久缓存阻止恢复接单。

## 9. 原子性、通知与恢复

新增 operation coordinator 复用现有服务：提供 `CancelTaskInTx`/阶段写入和提交后 action 描述；旧公开取消入口继续包装它，不复制聊天收尾与恢复输入逻辑。
intent、目标状态变更、该阶段审计在一个 DB 事务中提交；任何必需写入失败全部回滚。审计失败不能返回管理成功。
通知、WS publish、容量唤醒在 commit 后执行；通知失败不把已经提交的 API 改报失败。响应包含 operation ID，方便确认未知提交结果。
操作扫描器按索引分批加载 applied，使用行锁 `SKIP LOCKED` 或 version CAS；更新下一次扫描时间，禁止长事务等待网络。
确认截止只改变 reconciliation_state；applied 的业务收尾复用现有幂等协调服务，新增阶段标识防止重复计数/审计。通知失败和服务重启均可恢复。
已应用取消的 ack 与审计必须使用同一事务，原 branch/error write 的“已有值不可覆盖”保持不变；原始路径/错误不进入 admin DTO/audit。
daemon 本地只保存必要的 operation_id/fence/结果待确认记录，原子写盘；服务器接收确认后清理。旧绑定队列不自动迁移给新用户或新安装。

旧 cancel ack 缺少 operation_id 时仍完成原聊天/分支收尾；只在服务端能唯一对应 task、当前绑定和 execution_fence 时关联管理结果，否则保留未确认。
同一 task/fence 的两个并发管理员取消复用同一个有效取消事实；各请求保留自己的 actor/reason 审计，不重复运行取消副作用。
操作结果不因负责人变更、用户更名或安装退役被改写；审计保留当时主体 ID 与展示名快照，当前名称另行查询。
安装端主动上报的 error/message 视为不可信内容，管理视图优先用服务端枚举 code；原始诊断只保存在原受限业务通道。
后台“确认已停止”仅表示协议约定的 daemon 收尾证据，不等于硬件级检测所有子进程；受损系统或不可达机器保持未确认。

## 10. 兼容、灰度与源码落点

先上表和只读管理 API，再发布支持证明的桌面/daemon，再开放控制。未具备双端协议与身份证明时，明确显示“未关联/不支持”，不猜测成功。
旧执行仍可由有平台取消权限的管理员使用原服务端取消；缺少新 ack 能力时返回 `confirmation_unavailable`，不影响其原轮询取消行为。停止/恢复接单只对已证明关联的安装开放。
客户端使用已有 `parseWithFallback` + zod；新增字段可选、未知枚举落到 unknown；服务端不改变旧心跳/领取响应的必需字段。
回退管理 UI 可行；一旦 stopped 门禁生效，后端回退不能静默移除门禁。需先恢复全部 admission 并完成审计/挂起操作处置，或使用保留门禁的回退版本。
持久台账、审计与运行中的操作不得因 down migration 丢失；新版本启动检查必要索引存在，缺失时禁止控制写入并提示运维修复。

| 代码落点 | 计划变更 |
| --- | --- |
| `apps/desktop/src/main/device-identity.ts`（仅参考）、新增 `managed-installation.ts` | 保留旧 device_id；实现受管理密钥和持久化，main IPC 暴露有限签名/接入能力 |
| `apps/desktop/src/main/daemon-manager.ts` | 安全交接绑定 challenge、能力协商，避免把凭据放 CLI 参数和日志 |
| `server/internal/daemon/identity.go`、`client.go`、`daemon.go` | 保留 daemon_id；接入绑定、原取消 ack 扩展与本地恢复 |
| `server/internal/handler/daemon.go`、`daemon_rpc.go`、`daemon_ws.go` | 验证注册/心跳/claim 的绑定上下文，共用控制服务，不复制 HTTP/WS 规则 |
| `server/internal/middleware/daemon_auth.go`、`server/internal/auth/` | 当前绑定与源账号版本验证；拒绝绑定替换和凭据降级 |
| `server/internal/service/task.go`、`server/pkg/db/queries/agent.sql` | 取消事务复用、claim/reclaim 门禁、版本/fence 与提交后副作用 |
| 新 `server/internal/service/admin_operation.go`、`managed_installation.go` | 幂等协调、生命周期、绑定和组织校验 |
| 新 `server/internal/handler/admin_installation.go`、`admin_execution.go` | 分页/筛选/脱敏 DTO、操作 API；router 接平台权限 gate |
| `server/migrations/`、`server/pkg/db/queries/`、generated | 实体、独立并发索引、sqlc；生成文件只通过生成命令更新 |
| `packages/core/api/`、查询 hooks；`packages/views/` | 契约解析、服务端分页和 Query 状态；管理页面不存入 Zustand |

## 11. 必须覆盖的验证矩阵

| 类别 | 关键用例 |
| --- | --- |
| 身份 | 伪造 ID/同名主机不绑定；nonce 重放/过期、错误部署、错误私钥、账号版本撤销拒绝；并发 enroll 仅一个安装 |
| 生命周期 | 多 OS 用户分开；多工作空间/provider 去重；保留/删除数据重装；同 OS 换账号不继承权；旧 daemon 未关联 |
| 绑定 | 抢占已有 daemon 被拒绝；换绑 epoch 撤销 HTTP/WS 旧连接；legacy register/claim 不绕过绑定；本地 IPC 无凭据拒绝 |
| 状态 | 窗口关闭 daemon 正常、窗口活跃 runtime 不可用、休眠、未上报、心跳缓存故障/DB 故障；不把未知当正常 |
| 取消 | queued 即时取消；在线取消/ack；离线立即取消但 ack 到期未确认；重复请求/不同 payload 同 key；A/B同时取消、两响应均丢失按各自key恢复；根确认与子请求同步中断；完成与取消 CAS |
| 结果 | 迟到 ack 不覆盖 completed/failed；原 branch/错误保留；任务 retry 新 ID 不受旧取消；旧绑定 ack 不改变新对象 |
| 门禁 | 停止与领取同时发生；单/batch/WS/旧端/fallback/内部 ClaimTask；deferred/reclaim；恢复缓存失效；在途继续 |
| 事务 | intent/audit/取消写入故障全回滚；commit 后通知失败轮询恢复；服务重启恢复；双扫描器不重复；未知 commit 查同 ID |
| 隐私 | 平台取消权限能取消但不能读取私有正文/日志/路径；分页/详情/错误/导出均无敏感内容泄漏 |
| 容量 | 1000 安装、多 runtime、随机/集中重连、双管理员冲突、100 万历史执行分页；记录实测，不以目标代替证据 |

Go 回归在 service/handler/daemon 原目录，DB 使用 testutil；TS 解析矩阵放 core，页面只验代表性状态与操作回执；真实代理不进入默认测试。
