# Research: main 合并后的后端验收风险与执行入口

- Query: 当前 main 的完整验收需要覆盖哪些后端业务、安全、并发、恢复与环境契约；怎样复用既有私有执行器建立新的可审计 campaign。
- Scope: internal；只读产品、脚本、规范和既有验收记录，不执行测试、启动服务、访问账号或操作 Git。
- Date: 2026-11-24
- Current source: 主会话提供的 `main / 103e2bc32`。本研究没有自行刷新 Git；最终提交身份由主会话冻结和复核。

## Findings

### 1. 合并证据只能作为风险线索

归档任务 `.trellis/tasks/archive/2026-11/11-23-cross-workspace-main-readiness/task.json` 记录 PR #6 的源提交 `662d66943`、合并提交 `0302e4cb9`，二者 Git tree 一致；任务记录、会话日志随后进入 main。详见该任务 `research/merge-continuation-20261124.md`。

同一归档任务的 `READINESS.md:40`、`research/backend-results.md` 记录旧候选的 86/86 默认包完整 race 执行，以及 25,564 个具名 PASS、24 SKIP；浏览器为 538 个独立身份、464 PASS、74 SKIP。它们不是 `103e2bc32` 本次 campaign 的通过数，不能直接继承或改标签。完整 PR CI 成功也不能替代本次实际生产构建、用户流程和条件模式验证。

最近合并修复了四类具体风险，应作为本次优先回归：

| 风险 | 最近失效原因 | 本次应验证的原业务断言 | 主要源码/测试锚点 |
| --- | --- | --- | --- |
| 私有历史分页不可用 | CI 首个 Go 进程缓存公开默认 JWT 签名密钥，历史 cursor 正确拒绝并返回 503 | 新的私有 key 在首个 Go/API 进程前就绪；历史分页、签名、actor/filter 绑定、撤权继续严格 | `server/internal/auth/jwt.go:18`；`server/internal/service/collaboration_information_history.go:350`；`server/internal/handler/collaboration_information_test.go:49`；`.github/workflows/ci.yml:548` |
| 延后提醒回执与存储不一致 | 首次回执保留纳秒，PostgreSQL 只保存微秒 | 首次回执、列表、undo、关键通知重放表示相同 deadline；旧 undo 不覆盖新版本 | `server/internal/handler/inbox_snooze_test.go:36`、`:102`；`server/internal/inboxstate/state_test.go:129`、`:139` |
| UTC JSON 回读丢失执行结果 | `time.Local` 与 JSON 的 `time.UTC` 位置指针不同，但 instant 相同 | 合法完整 invocation UTC 往返继续匹配；1 ns deadline 变化及任意其他绑定变化仍拒绝 | `server/internal/service/collaboration_stage_execution_internal_observation_test.go:42`；`server/internal/service/collaboration_stage_execution_serialization_test.go:15` |
| completion preview 受无关历史污染 | fixture 初始零计数错误地统计全局保留的交付历史 | 新 fixture 范围无记录；无关 consumed 历史合法保留；整个 preview 前后全局业务快照完全不变 | `server/internal/service/collaboration_completion_preview_managed_submission_test.go:117`；`server/cmd/server/collaboration_stage_completion_preview_integration_test.go:95` |

额外保留 stage fixture 数据库时钟回归：`collaboration_stage_execution_fixture_clock_test.go:11`、`:47` 必须使用数据库 observation clock；生产未来时间拒绝不放宽。

### 2. 当前标准入口及覆盖边界

| 入口 | 已核实行为 | 完整验收时的限制 |
| --- | --- | --- |
| `make test ENV_FILE=<private file>` | `Makefile:383` 确认数据库、迁移，然后执行 `scripts/test-go.sh --race` | 同一 URL 被整个 Go 包序列共用，不能单独满足最新规范要求的每包新库 |
| `bash scripts/test-go.sh [--race]` | `scripts/test-go.sh:31` 无显式 DATABASE_URL 时在 go list 前退出；`:49` 普通包 `-p 1 -parallel 2`，Agent 包尾段 `-p 2 -parallel 2`；40 分钟包 timeout | 只接受无参数或 `--race`；没有 JSON/uncached inventory，也未自动每包分库 |
| `make check ENV_FILE=<private file>` | `scripts/check.sh:177` 至 `:224`：静态检查、TS、脚本、Go race/vet、独立 API、生产 Web/Desktop build、Playwright 单 worker 零 retry | 非 mobile 的静态/TS；Go 共用一个 check Go 库；并非全部 auth/feature 模式 |
| `scripts/go-test-with-agent-cli-guard.sh` | 使用 sentinel PATH 和调用 marker，任何 ambient Agent CLI 调用令入口失败；命令错误码透传 | 必须保留；不能为真实 Provider smoke 去掉普通测试保护 |
| `go -C server vet -p 2 ./...` | 静态 Go 验证，不是用户流程测试 | 应在同一固定 source/toolchain 执行；不能计入 E2E PASS |
| `go -C server tool govulncheck ./...` | `.github/workflows/ci.yml:609` 的官方入口；CI 使用 `1.26.x` 最新补丁 | 当前补丁版本与测试版本分别报告；旧工具链可达 stdlib 提示不能被称为当前补丁结果 |

`scripts/check.sh:118–123` 明确强制 `legacy` auth，并关闭 managed installations、knowledge context、platform admin 和 device auth。因此即使一次标准 `make check` 全绿，也没有证明以下 live 模式：

1. password auth + platform admin：注册策略、登录/改密、受限恢复密码、派生 PAT/task/daemon 凭据撤销、admin 角色与最后管理员保护。
2. managed installations + knowledge context：安装/运行来源、当前 owner、knowledge v1/v2、真实 daemon claim/launch/recovery。
3. device auth：首个账号、工作区创建、会话恢复、禁用后的拒绝。
4. 各模式中关闭开关的负向对照，不能只验证 enabled 模式。

条件用例需各自分配新的隔离 API/数据库和准确 flags，或明确保持 NOT_RUN/BLOCKED；不能通过把 required gate 设置成假或删 skip 得到“全覆盖”。

### 3. 后端测试源规模与分层

只读源码静态解析发现 `server/` 下 1,783 个 `_test.go` 文件、77 个有测试文件的目录、11,967 个标准 `testing.T` 顶层声明。其中 service 1,890、handler 3,147、router 563、migrate 75、migrations 105、Agent 1,294、daemon 1,041。这个计数未解析当前平台 build tags 和动态子测试，不是实际执行数；当前平台实际包/test denominator 应重新从 `go list` 与 Go AST 生成。

应保留以下三种证据并分别计数：

- 纯 helper/protocol/state 单元和 property/boundary tests。
- 新数据库上的真实 handler/service/router/WS integration；HTTP handler 单测不能冒充完整 UI 旅程。
- 生产 Web / 原生 Electron / daemon CLI + API + DB 的 live 流程；fixture Provider 仅代表确定性运行闭环，不能代表真实模型行为或平台发布验收。

### 4. 广覆盖业务矩阵

每个 P0/P1 流程至少包含成功、拒绝、边界、幂等/并发、失败恢复五个维度；读写拒绝同时检查无副作用。角色维度包括人类 owner/admin/member、无成员、外部当前 handler、通用机器和受限机器；资源维度至少两个工作区、两个项目、跨账号与同时间戳分页。

| 范围 | Happy path | Negative / boundary | Concurrency / recovery | 现有锚点及需 live 验证的部分 |
| --- | --- | --- | --- | --- |
| API readiness 与 startup | 健康、ready、迁移完成 | DB 不可达、错误 auth 配置、默认生产密钥拒绝 | 只有确认所属 pid/commit 的 listener 可复用 | `cmd/server/integration_test.go:215`、`:233`；live API health/source identity |
| 登录与会话 | legacy、password、device 支持的方式分别成功；JWT/cookie/PAT | 错密、关闭注册、受限账号、CSRF/simple form、invalid/revoked session | 改密与并发 mint 竞态；旧 JWT/PAT/daemon/task 即失效；503 可重试 | `handler/auth_password_test.go:50`、`:170`、`:425`、`:437`、`:518`；live 双客户端撤销 |
| 人类/机器 principal | 人类 mint、合法 task 自有访问 | task/cloud PAT 不得 mint 人类 JWT/PAT；伪造 actor headers 无效 | 同 owner 不能借人类签署/完成权限 | `middleware/auth_principal_test.go:13`；`security-boundaries.md`；`cmd/server/collaboration_machine_completion_refusal_test.go:24`、`:102` |
| workspace / tenancy | create、slug、成员、邀请、切换、leave、owner delete | reserved/conflict slug、非 owner 删除、跨 ws ID/UUID/identifier、失去成员 | 切换后无 query/WS/cache 泄漏；leave/delete 与到达事件竞争 | `handler/handler_test.go:2089`、`:2106`；`cmd/server/workspace_scope_guard_test.go:26`；live 两工作区/两账号 |
| dependency 错误语义 | 有权限资源可读 | 不存在/失权 404，DB fault 503，禁止把503当撤权 | 网络/DB 故障重试后恢复，不误删工作区/账号状态 | `middleware/workspace_access_error_test.go:30`；live fault/reconnect |
| project / iteration | 创建项目、lead、资源、迭代状态与任务分配、历史 | 跨项目/ws、已关闭迭代、非法 timezone、越权设置 | 并发 closure/assignment、取消确认、release 和历史不可被新写覆盖 | `cmd/server/project_permission_route_test.go:57`；`cmd/server/iteration_lifecycle_routes_test.go:14`；`handler/iteration_i1_p95_test.go` |
| ordinary issue 生命周期 | CRUD、parent/subissue、默认 todo、labels/property、member/Agent/squad assignment | UUID malformed、同标题 duplicate、cross-ws parent/project、不完整 assignee pair | revision 冲突、重复 enqueue、删除/恢复显式 dependent cleanup | `handler/handler_test.go:344`、`:612`、`:803`、`:1375`、`:2896`；live list/detail/count |
| comment / replies / attachment | 新建/编辑/删除、thread、reaction、mention、附件 | foreign parent/comment/ws、revision、UTF8/JSON/body limits | thread cursor 同 timestamp 不重不漏；重放/幂等通知；编辑竞态不重复执行 | `handler/comment_list_test.go:720`；`handler/comment_cap_thread_test.go:391`；live author/admin/Agent matrix |
| **现存 comment admin 权限债务** | 合法人类 admin 管理他人评论 | owner/admin 所属机器不得借角色管理他人评论 | 拒绝不得改 body/附件/revision/通知/队列；作者字段不等于操作者证明 | `handler/comment.go:3331`、`:3333`、`:3592`、`:3594` 仍无独立人类条件；任务 `11-20-comment-admin-human-authority/prd.md` 未实施。需本次真实路由复现，单列风险，不以其他绿灯关闭 |
| inbox / snooze | read/unread、archive、group snooze、remind now、undo、expiry | foreign item、旧 revision/undo token、过去/非法时间 | critical source 只消费一次；跨 snooze replay 不唤醒；批处理不误碰 future snooze | `handler/inbox_snooze_test.go:51`、`:102`、`:204`；`inboxstate/state_test.go:321`、`:354`；live UI readback |
| collaboration 创建与 authority | 选择 external recipient、source、initiator、原 key 创建 | machine、失效 source/recipient、能力关闭、scope/版本歧义 | 不确定 response 用 original-key receipt 恢复；不重复 materialize | `cmd/server/collaboration_delivery_routes_test.go:11`；live default/localized creation 与跨账号 |
| collaboration workbench / stage | 确认 requirement、计划/stage、审批、execution、formal delivery/accept/reopen | wrong stage/version/source、机器冒签、过期批准、未来 observation | concurrent revoke/read；one-time consume；UTC wire；未消耗 invocation 不授权 | `cmd/server/collaboration_stage_completion_preview_integration_test.go:380`、`:671`；service stage suites；live 双方闭环 |
| command/history/record | FIFO、cancel waiting、当前/历史 run 和原文件/评论 | tampered cursor、private ordering/status/source leakage | 同时队列、retained old history、新 preview 只读、切 target 仍保留原执行 | `handler/collaboration_command_history_test.go:31`；`handler/collaboration_execution_test.go:218`；live timeline/reload |
| completion enforcement | 有效人类 grant+exact delivery 才能完成 | source revision/hash、过期/撤销 grant、机器从人类继承权限 | authorization/terminal/issue status 同一事务；失败回滚；原 receipt 不重写 | `cmd/server/collaboration_machine_completion_refusal_test.go:24`、`:102`；live ordinary/collaboration completion |
| knowledge proposal / target | propose、approve/reject/withdraw/correct、project/workspace adoption | v1/v2 shape、exact revision/hash、target/source 权限、过期、purged/deleted source | 人类决定赢过迟到系统结果；reopen 暂停旧 proof；容量不偷删、不复活撤销 lineage | `handler/collaboration_knowledge_final_matrix_test.go:235`；`service/collaboration_knowledge_test.go:45`；live adopted search/read |
| project knowledge automation | formal acceptance 唯一 job、0–3候选、可用素材与出处 | ordinary done/run success 不触发；无project/manual/disabled不触发；budget/quote/source closure 拒绝 | uncertain attempt 保留；explicit retry 新 execution，publication quota不重置 | `.trellis/spec/server/collaboration-knowledge.md`；`e2e/project-knowledge-automation.spec.ts` 的 live job/status/policy |
| knowledge runtime | claim→prepare→exact manifest→launch | withdraw/expiry/purge/owner revoke；version upgrade/downgrade；不可读 comparison | late lock NOWAIT、launch 前当前权限重查、原 manifest 不被改请求替换 | `service/collaboration_knowledge_runtime_test.go:127`、`:168`、`:234`、`:281`、`:334`；managed+knowledge live daemon |
| realtime / WS | issue/account/knowledge events、Query恢复 | final-send 撤权、bad/ambiguous envelope、cross account/source | 排队 frame 遇 PAT/password撤销拒绝；断网重连/re-subscribe；持久 outbox重放不重复 | `cmd/server/collaboration_realtime_test.go:22`；`cmd/server/collaboration_knowledge_realtime_test.go:119`、`:232`、`:290`；live 两客户端 |
| upload / resources / downloads | scoped upload、metadata、原文件下载/导出 | traversal、foreign attachment、capability expire/revoke、oversize、secret日志 | 原 deliver 文件不因display删除消失；uncertain commit有recoverable receipt | `handler/collaboration_attachments_test.go:59`、`:140`；live 原生下载和内容/出处 |
| runtime / Agent / squad | 假 executable 的队列claim、启动、stdout/file、complete；项目 execution squad | 无runtime、stale heartbeat、disabled/offline、容量、错误 provenance | queue/task cancel、daemon restart、duplicate claim、原授权/原output恢复 | `cmd/server/project_execution_squad_test.go:11`；daemon/Agent suites；live fixture Provider进程闭环 |
| webhook / autopilot / integrations | sandbox签名/trigger、schedule preview、订阅者 delivery | bad token/signature、route principal、quota、跨originator invoke | 同key重放不重复run；lookup DB fault无body/write；取消/通知失败可poll恢复 | handler/autopilot suites；真实第三方消息/付款不自动触发，使用本地fixture |
| migrations / upgrade | 新库全部up、现有受支持旧schema升级、ledger/index有效 | retained security/history down拒绝、noFK/cascade、并发index各独立statement | interrupted invalid-index修复、runner锁、explicit app dependent cleanup | `cmd/migrate/migrate_collaboration_command_history_test.go:14`；resource-offer `:159`；只能私有schema/库演练 |
| performance / resource | 既有 I1 P95、搜索timeout、容量/连接预算、长列表分页 | 超限/慢查询/timeout明确失败、没有通过诊断替换阈值 | 原threshold/warmup保留；锁/重试负载有记录；无孤儿任务/连接 | `handler/iteration_i1_p95_test.go`；`handler/search_timeout_test.go:41`；独立低干扰campaign |

### 5. 最小安全环境契约

1. 固定本次新 source、toolchain、脚本与构建产物身份；BEGIN/END hashes 都要复核。发现新提交保留旧run，冻结新候选并按实际差异重验。
2. backend PostgreSQL/Redis 与浏览器数据库分离。每个 DB-backed 顶层包收到单独的 freshly migrated clone；仅不同 workspace ID 不能隔离 singleton gates/TestMain cleanup。
3. 测试前要求显式 recorded expected database，实际 `SELECT current_database()` 必须一致。`cmd/migrate` 缺 URL 尚有默认fallback，所以 URL校验必须在 migrate 前。客户端存在、endpoint/port/container owner先证明。
4. 每个顶层包私有 HOME；HOME/TMPDIR/TMP/TEMP/GOTMPDIR 的实际解析链在 Git 外，无symlink、非可信可写祖先。macOS `/var` 应解析到可信 `/private/var`；用户属0775 `/Volumes/artisan`不是 secure artifact fixture root。Go1.27 的 `T.TempDir` 直接受 GOTMPDIR影响。
5. 明确固定 GOCACHE/GOMODCACHE/GOPATH，普通测试 process umask022；配置/日志/private dir仍0600/0700。
6. 新32-byte随机 JWT_SECRET 在首个Go/API进程前建立；值仅存私有配置，不进研究/报告/Git。普通Go测试移除 API `DATABASE_MAX_CONNS`、`DATABASE_MIN_CONNS`、`MULTICA_RESOURCE_PUBLISH_DIR`、全局task root等；不能让API预算改变测试行为。
7. 同一个PG cluster仍共享 max_connections；记录 aggregate clients，给task-onlyAPI小pool并按预算并发，绝不改共享max_connections或踢不属于本任务的连接。
8. default测试保持Agent CLI sentinel；`agentintegration` + `MULTICA_RUN_REAL_AGENT_SMOKE=1` 的真实账号/额度测试需单独明确授权。`server/pkg/agent/real_agent_smoke_integration_test.go:1`、`:12` 为tag和运行前gate。
9. UTC合并回归必须在 `TZ=UTC` 和非UTC（本机Asia/Shanghai）两种环境；原 deadline精度、不等值拒绝、future-clock拒绝不变。
10. 原失败/raw/db保留；修正环境后新run、新库。包exit0不能替代实际命名test terminal，empty TestMain/compile-only不能算业务PASS。

### 6. 可复用私有执行器：最新整包 campaign

**优先复用实现模式，不能直接运行旧目录。** 当前本机已确认以下文件存在，均为2026-11-23旧candidate资源：

| 文件 | 已核实职责/入口 |
| --- | --- |
| `.omx/private/qa-work/main-readiness-20261123/backend/run_backend.py` | `STATE=脚本父目录`，读取 `tools.json` 和 `secure-temp.json`；`python3 <new-dir>/run_backend.py setup` 分配有owner-label的native ARM64 PG/Redis、独立JWTkey并写private allocation；`... execute` 依次 build/migrate/wrapper-tests/guarded race/vet/vulnerability |
| 同目录 `bin/go` | 仅admit旧wrapper固定test语法；每个canonical package克隆DB和私有HOME；添加 `-json -count=1`，清API-only变量，actual child umask022；写 `packages/<sha256(pkg)前12位>/{receipt.json,test.jsonl,stderr.log}` |
| 同目录 `summarize.py` | 对新 `expected-tests.json` 核对rawjson、package身份、top/named终端、missing/unfinished/duplicate、buildfail和stream SHA256；产出 `test-summary.json` |
| 同目录 `tools.json` | source、candidate、state、real_go、psql、缓存路径。旧candidate是 `cb192014b`，不能复制其值作为当前身份 |
| 同目录 `expected-tests.json` | schema是 `{package,test_files,top_functions}` 的数组；旧86个package，必须从新source重新生成 |
| 同目录 `source-initial.json`、`final-audit-cleanup.json` | 起始源码hash inventory、最终source/allowed delta、命名inventory完整性、PG/Redis owner/clients/stopped proof |

`backend-results.md` 与 `verification-evidence-audit.md` 对旧脚本输出做了独立逐包核算，原fail/interrupted与accepted replay保留在不同目录。这说明实现模式可用，不等于新版源码/最新规范的自动clearance。

新campaign应先完成：

1. 分配全新的私有state/evidence目录，不复用旧allocation、packages或日志。旧runner会把existing complete/exit0 package视为retained pass而跳过；新目录必须为空。
2. 主会话固定 `103e2bc32` 全SHA的隔离source；将runner/adapter/accounting复制到新目录，只改本次private路径/身份。重新记录脚本hash，并冻结本次 argv允许范围。
3. 生成新 `tools.json`（新source/candidate/state、当前实际安装Go/psql、pinnedcache）、新 `secure-temp.json`（新Git外可信链的actual path/UID/mode）。`tools.json`无需旧密码；`setup`自行产生新key/容器。
4. 使用固定Go的 `go list -json ./...` 获得当前platform的 TestGoFiles/XTestGoFiles，再用Go AST生成expected Test/Fuzz/Example/benchmark分类。排除 `TestMain` harness；不能用 `go test -list` 当只读inventory，因为它会运行TestMain。
5. 在运行前生成 source/tool/adapter/config/environment/proof hashes，独立审查并登记本次clearance；setup前后的端口/container/db proof分别保留。旧clearance不匹配103e2bc32。
6. 执行全套新包race；如计划需要normal与race对照，分别新库/新输出，不能写入同一packages目录。大型包40m timeout和parallel预算保持，background运行允许主会话持续更新进度。
7. 新parser/independent checker核对全部expected包和实际top/named/leaf identity；保留每次fail/skip/replay来源，禁止从不同执行拼出一个假的通过子树。
8. 最后复核HEAD/所有产品bytes/toolchain/config、source edges和实际resource owner；匹配complete命令退出与inventory完整性后再作验收结论。只停止确属本任务的无client资源，库与logs保留。

**复用前必须处理的当前规范差距：** 最新简化 `bin/go:8` 在 `BACKEND_PACKAGE_CHILD=1` 时直接forward，没有旧严谨adapter的recursive package/allocation精确许可校验；runner没有强制candidate/config/clearance准入，summarizer对BuildEvent只按action分类而未完整验证ImportPath/Output形状。若用于新任务，要在独立审查中限定并核对这些process边界，或复用下面的更完整契约；不能把“旧run通过”当作新脚本已满足当前规范。

### 7. 更严谨的历史 launcher / clearance / final audit 路径

这些文件已在本机核实存在，用于核对契约，不可借用旧clearance运行本次source：

- `/Volumes/artisan/code/2026/mc-collaboration-final-owned-20261103/.omx/private/final-check-v11/run-make-check-v11-provenance.py`
- 同目录 `owned-bindings-v11-provenance.json`
- 同目录 `independent-review-clearance-v11.json`
- 同目录 `check-source-v11.env`、`check-source-v11.safe.env`（含私有配置，不读取/打印其值）
- 同目录 `make-check-v11.json`（旧run）
- `.omx/private/collaboration-resume-20261101/adapter-v9/check-bin/go`、`pnpm`（v11声明byte-identical的adapter源）；v9有精确selection、compile-only、recursivepackage允许和环境scrub回归。
- `.trellis/tasks/archive/2026-11/10-30-collaboration-fix-closure/research/continue-20261103/v11-review/{review.md,review.json}`
- 同research目录 `final-state-audit/{final_state_audit.py,final_state_audit_test.py,boundary.md}`

旧v11根命令是 `python3 <old-root>/.omx/private/final-check-v11/run-make-check-v11-provenance.py`；内部调用 `make check ENV_FILE=<old-root>/.omx/private/final-check-v11/check-source-v11.safe.env`。review绑定candidate `304f74dd...`；任何当前source/state/root变更均需本次新review/clearance，不能编辑旧文件或运行该旧命令。

核心clearance字段：`candidate_sha`、`launcher_sha256`、`adapter_sha256`、`source_env_sha256`、`bindings_sha256`、`resource_ownership_sha256`、accepted verdict、`blocking_findings_open=0`。必须在实际执行前匹配真实bytes，执行后另做final audit。接受条件是 `command_exit==0 && final_identity_audit.pass && inventory_complete && resource_ownership_verified`。

旧v11final auditor当前hardcode旧ROOT/REVIEW/paths；只提供实现参考，不能原样运行并认为当前已审计。允许的Next生成import例外必须精确、内存normalize并分别记录raw/normalizedhash，绝不扩为任意源码改动。final filesystem audit自身也不证明live DB/API/listener或用户流程。

### 8. 新环境就绪后的准确验证命令形状

下面命令由协调者在**已经加载经过数据解析的该包私有env、核对recordedDB、固定source/toolchain**之后发给子agent；DB配置本身不出现在报告。一个顶层包一个fresh clone，normal/race/UTC replay各有新run：

```sh
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -json -count=1 -race -timeout=40m -p 1 -parallel 2 ./internal/service
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -json -count=1 -race -timeout=40m -p 1 -parallel 2 ./internal/handler
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -json -count=1 -race -timeout=40m -p 1 -parallel 2 ./cmd/server
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -json -count=1 -race -timeout=40m -p 1 -parallel 2 ./cmd/migrate
bash scripts/test-go.test.sh
bash scripts/check.test.sh
bash scripts/dev-env.test.sh
go -C server vet -p 2 ./...
go -C server tool govulncheck ./...
```

完整package分母来自本次实际 `go list`，以上四个重包并非全部后端。Agent/daemon/CLI/protocol/migrations/auth/storage/realtime等其余包需完整wrapper或同样的新逐包runner覆盖。默认Linux/macOS/Windows差异以实际runner能力和build条件登记，缺平台/feature/外部依赖明确列SKIP或NOT_RUN。

## Files Found

- `CLAUDE.md`：测试分层、真实Agentgate、migration/noFK/UUID/error约定。
- `Makefile:260`、`:383`：完整check和Go入口。
- `scripts/check.sh:61`、`:118`、`:166`：新check allocation/key/feature模式与执行阶段。
- `scripts/test-go.sh:31`、`:49`：显式DB、序列与Agentbudget。
- `scripts/go-test-with-agent-cli-guard.sh`：sentinel命令保护。
- `.github/workflows/ci.yml:495`、`:534`、`:548`、`:609`：pgvector17、最新Go1.26 patch、随机key/masking、vulnscan。
- `.trellis/tasks/archive/2026-11/11-23-cross-workspace-main-readiness/research/{backend-results.md,verification-evidence-audit.md,merge-continuation-20261124.md}`：近期实际失败类别、环境修复与旧acceptance边界。
- `.trellis/tasks/11-20-comment-admin-human-authority/prd.md`：现存未实施权限债务。
- 上述矩阵中的测试/源码文件以及第6–7节实际存在的private执行器与review。

## Related Specs

- `.trellis/spec/server/index.md`
- `.trellis/spec/server/database-test-isolation.md`
- `.trellis/spec/server/verification-environment.md`
- `.trellis/spec/server/security-boundaries.md`
- `.trellis/spec/server/collaboration-foundation.md`
- `.trellis/spec/server/collaboration-stage-approvals.md`
- `.trellis/spec/server/collaboration-delivery.md`
- `.trellis/spec/server/collaboration-knowledge.md`
- `.trellis/spec/server/collaboration-knowledge-execution.md`
- `.trellis/spec/server/collaboration-realtime.md`
- `.trellis/spec/server/inbox-snooze.md`

## External References

无外部检索。本任务应以固定仓库的CI约定（Node22、Go1.26.x最新patch、pgvector/pgvector:pg17）和实际安装版本为依据；本研究没有证明当前工具、Docker image架构、网络或账号可用。

## Caveats / Not Found

- 本研究没有执行测试/构建/迁移/服务、修改环境、访问真实模型/账号、操作Git或旧campaign资源；没有新测试PASS声明。
- 所有旧运行数和privateclearance仅归属各自旧source，不能继承给103e2bc32。
- 当前commentadmin机器权限风险有源码与独立任务证据，本研究未live复现；应记录为已知债务并验证，不能自行实现另一个规划任务。
- 现有一次标准check没有全部mobile、feature模式、真实Provider、Windows/Linux发布验证；条件矩阵和最终报告需要如实保留这些区别。
- 当前新campaign的source/tool/resource/admission都仍由主会话建立；研究给出可复用路径及缺口，未签发runtimeclearance。
