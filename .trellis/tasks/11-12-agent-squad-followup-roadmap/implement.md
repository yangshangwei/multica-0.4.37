# M1-S1 实现与验收计划

状态：in_progress；用户已批准 M1-S1，执行与验证进度按下列清单和 research 记录。目标是完成普通单事项的证据查看、人工授权、精确完成和原操作查询；技术采用现有 Go/Chi/sqlc writer、React Query/core/views 与 Go CLI，不加依赖。

## 0. 实现前门禁

- [x] 沿用现有任务；阅读 AGENTS.md、CLAUDE.md、trellis-start/continue、当前任务及两项完成合同。
- [x] 核对 planning、未激活、缺 design/implement、空 manifests；建立本轮源码研究，而非沿用历史通过。
- [x] 完成 PRD 收敛；requirements 区分首包和后续入口，design 固定具体 wire/交互/恢复方案。
- [x] 独立规划复核的问题全部处置，校验本目录路径/JSON/JSONL/验收映射；本轮最终摘要交由用户审阅。
- [x] 用户后续明确批准最新规划摘要及 M1-S1 实现；只批准首包，不表示整条路线或真实运行放行。
- [x] 再核对 Git/并行文件 ownership、schema/API/源码变化、当前任务身份与本轮审批是否仍适用。重大变更回到规划审阅。
- [x] `python3 .trellis/scripts/task.py validate .trellis/tasks/11-12-agent-squad-followup-roadmap` 成功后，才执行 `python3 .trellis/scripts/task.py start .trellis/tasks/11-12-agent-squad-followup-roadmap`；不得为派研究代理提前 start。

当前门禁依据 `.agents/skills/trellis-brainstorm/SKILL.md` Planning Flow 8–9：最终规划摘要之后，后续明确批准才允许 start；`.trellis/workflow.md` Phase 1.4 要求复杂规划三文件与真实上下文。首包完成时不要把整条 roadmap 归档为 completed。

## 1. 后端证据发现与预览

责任：一个 trellis-implement 子代理拥有这一步的 server 文件；执行前加载 trellis-before-dev 与上下文。不得同时安排另一个代理修改 router 或 delivery service。

拟新增：

- `server/internal/handler/delivery_completion_preview.go` 与对应 `_test.go`：两个只读接口、严格输入、当前权限/事务/响应。
- `server/internal/service/delivery_completion_preview.go`：原服务能力必要组合；避免复制 artifact observer/issuer authority。
- `server/pkg/db/queries/delivery_evidence_candidates.sql` 及 sqlc 生成文件：issue-scoped 有界分页，保留原 artifact 可见性过滤。
- `server/cmd/server/delivery_completion_preview_integration_test.go`：真实 Auth/router 和任务自有 DB 集成。

拟修改：`server/cmd/server/router.go`；只在复用需要时最小调整 `delivery_evidence.go`、`delivery_completion_authorization.go`、`artifact_authority.go` 的 service 私有 helper。没有重写授权服务、gate 或 migration 的预授权。

执行次序：

1. [x] 添加红测试：任务分页/不可见候选/可见集合 cursor、隐藏行不能产生空页或 has_more/计数泄漏、有界扫描不足明确 unavailable；封闭 body、自定义 done、过期和缺字段、有限凭据 max expiry、无业务写但允许原锁且零新增事实。
2. [x] 在已核对的独立 package DB 执行指定红测试，记录原失败与源码身份；不要跑旧历史脚本或默认 DB。
3. [x] 实现 candidates 的有界查询/原权限筛选；实现 selected preview 的同事务组合，确认 policy/issuer/consumer/source/PR 读取没有逃逸到 pool。
4. [x] 运行新测试和现有 authority/observer/authorization 回归；审查 no-store、错误码、JSON 字段、标准 reader 中 unknown 的含义。
5. [x] 未发现需新增索引或 schema；保留无 migration 设计。

出口：Web/CLI 所需规范请求片段与期限均可从新读接口得到；新读不签发、不消费、不调用真实 Provider。先验证接口，再安排依赖客户端。

## 2. Core 合同、原意图与完成协调器

责任：一个 trellis-implement 子代理拥有 core；P1 wire 冻结后执行。现有类型 `UpdateIssueRequest.completion` 原样复用。

拟新增：`packages/core/issues/completion.ts`、`completion-schema.ts`、`completion-queries.ts`、`completion-draft.ts` 及相邻 `.test.ts`，`packages/core/api/delivery-completion.test.ts`。文件拆分按实际规模收敛，不为每个 state 建新抽象。

拟修改：`packages/core/api/client.ts`、`api/schemas.ts`、`types/api.ts`（仅缺失响应类型）、`issues/mutations.ts`、`issues/index.ts`、`platform/session-cleanup.ts`，必要 export/package.json。复用现 cache coordinator、StorageAdapter、draft cleanup；不直接导入浏览器存储。

1. [x] 红测试覆盖 canonical wire、explicit null、safe int64、malformed 成功/unknown flags、scope mismatch、private key 零透传。
2. [x] 加 API schema/方法：candidates/preview、原创建/by-key/get/revoke；新 bound PUT 解析最小 receipt 并比对原意图。
3. [x] 红测试覆盖原 draft 保存失败/读回不符零请求、重复点击、重挂载、创建/完成/撤销 unknown 查询、GET 404/issued/conflict 不重写、登出清理、旧响应身份隔离。
4. [x] 实现纯状态转换与 scoped 单飞、原意图存储、lookup-only 恢复；创建未知只可找回原授权且不自动提交，完成/撤销未知查到 issued 保持 unknown。创建、完成、撤销分别记录派发阶段。
5. [x] 仅 bound completion 路径关闭 optimistic done；已知 receipt 与当前 issue 分开 reconcile，不能回退新版本。保留原不带 binding 的乐观编辑。
6. [x] 定向测试、core lint/typecheck 通过；P3/P4 接口稳定后再接线。

## 3. Web/Desktop 共享交互

责任：trellis-implement 拥有 views 与对应 locale；core 仅由 P2 owner 修订，避免并行冲突。

拟新增：`packages/views/modals/issue-completion-authorization.tsx` 及测试。

拟修改：`packages/views/issues/actions/completion-hint-gate.ts`、`use-issue-actions.ts`、`surface/use-issue-surface-actions.ts`、`components/issue-detail.tsx`，对应测试与 `packages/views/locales/en/issues.json`、`zh-Hans/issues.json`。普通 hint modal 保留非授权语义。app routing 不新增；若挂载差异需 app 修改，先列明平台用途。

1. [x] 添加接线测试，验证 enforce 路由到新的授权 flow，并让 bound completion 经 Core writer 送入实际 PUT。
2. [x] 接入本包所有显式普通 status-only 入口，复用一个选择/预览/授权/提交界面；排除 drag/batch/create/mixed/collaboration。
3. [x] 加两个明确动作、当前证据/期限/理由、关闭与撤销区别、原操作查询入口、缺证/拒绝/未知/历史完成但已重开状态。
4. [x] 覆盖重复动作、unmount、scope/身份变化、存储失败、迟到响应、callback 不提前执行。
5. [ ] 完成实际 enforcing Web/Desktop renderer 的键盘/焦点/窄窗与视觉验证。当前已运行 Desktop 使用 off/observe Demo 项目，授权弹窗不能安全地通过现有数据触发；禁止为截图擅自修改共享策略/事项。
6. [x] views 定向测试、lint/typecheck 通过；共享代码无 next/react-router-dom，Core 保持 package 边界。

## 4. CLI 与内置指导合同

责任：可在 P1/P2 合同稳定后与 P3 并行，由另一 trellis-implement 代理拥有 `server/cmd/multica/` 与下面内置文档；不要修改 P1 service/router 或 P3 locale。

拟新增：`server/cmd/multica/cmd_issue_completion.go` 与 `_test.go`；修改 `cmd_issue.go` 命令注册/缺授权提示。

同步：

- `server/internal/service/builtin_skills/multica-working-on-issues/SKILL.md`
- `server/internal/service/builtin_skills/multica-working-on-issues/references/working-on-issues-source-map.md`
- `server/internal/service/builtin_skills/multica-creating-agents/SKILL.md`
- `server/internal/service/builtin_skills/multica-creating-agents/references/creating-agents-source-map.md`

1. [x] 以 httptest/fake client 写 wire/flag/stdout/stderr/exit 红测试；未连接真实 API/Provider。
2. [x] 实现 prepare/authorize/submit/show/revoke，私有 operation 文件、身份作用域、写前派发标记、专用 --accept 和原 key 恢复。
3. [x] 证明未知原操作只查询不重写、文件锁/完整性失败不写、重定向不跟随；授权返回与 completion receipt 必须精确匹配。
4. [x] 原 status/update 不自动签发，原 --no-start 合同不变，新 submit 不接受 --no-start/mixed updates。新 JSON/table envelope 不改变旧命令输出。
5. [x] 更新两份内置 SKILL 与 source map，保持人类签发边界、hint 非授权、Reporter/Reviewer 权限不扩展；不改实例模板。
6. [x] CLI 全包测试/vet/build、内置 skill 合同通过。

## 5. 验收矩阵：测试归属与证明

下列名称为拟新增场景，不是已通过记录。纯状态矩阵在 core canonical 测一次，UI 测接线/可访问性与命名回归；DB 套件证明真实事务而非 mock 所期待的行为。

| 验收 ID | 主验证层 | 必需证据 |
| --- | --- | --- |
| S1-01/02 | server handler/router | >一页可见候选、cursor/has_more 不暴露隐藏行、扫描不足明确 unavailable、selected preview 字段一致、数据库时间/TTL、零新增评价/授权/状态事实；准确 pass/hold/unknown |
| S1-03/04 | core + UI/CLI 一条真实链 | 提示确认零授权、明确接受单次原 POST、target_position:null、本人 consumer、精确 status+completion 原 PUT |
| S1-05 | DB service/handler + core 拒绝映射 | preview→authorize 与 authorize→PUT 两个窗口中的 task/project/policy/PR/artifact/credential 变化；late failure 全事务回滚 |
| S1-06 | core + CLI transport | POST 已提交但响应丢失、POST 未到达、POST 在途；均保留原 key；GET 找回/404/409/503 不另签 |
| S1-07 | core + CLI + DB fault injection | PUT 已提交响应丢失、未提交、在途/锁冲突；GET issued 不再 PUT；receipt 精确匹配才消除 unknown |
| S1-08 | handler/router + UI 查询接线 | 原 receipt 回放零二次通知/queue/status；随后 reopen 当前读更高版本；当前失权不能回填历史 done |
| S1-09 | core、跨宿主 UI 与 DB race | 单意图双击单次派发；并发不同意图精确版本最多一次真实完成，另一项拒绝，不伪装已被它完成 |
| S1-10 | UI/CLI + DB race | 签发前取消零写，签发后关闭不撤销，显式 revoke；consume/revoke 两种胜出顺序和 response loss；卸载无后续写 |
| S1-11 | server/core | 自定义 active done、archived/unknown、done→done、重开后新完成；旧授权不能签新一轮 |
| S1-12 | views integration + CLI | Web/实际 Desktop renderer 同组件接线、键盘/焦点/窄窗；CLI JSON/exit/stderr 每种结果一致 |
| S1-13/15 | core 存储/session + router | private key 零透传，原 artifact authority；保存失败不写、refresh 恢复、账号/server/workspace 切换不泄漏 |
| S1-14/16 | core/CLI compatibility + 既有 v1 回归 | malformed 2xx/structured404/legacy404 区分、skip=false、无后门；被排除入口仍走原门禁 |

## 6. 验证命令与环境

下列产品命令在各实现切片就绪后按记录运行；先前规划阶段仅做过文档验证。DB-backed 测试每个 package、normal/race 各用新建且迁移完成的任务自有 DB；先核对 actual database、源码和 allocation，不从 `.env`/旧脚本猜。失败日志/数据库保留，不能清理审计事实求绿。协调者分配环境，子代理仅获精确包/用例权限。

典型前端定向命令（新增文件后补入实际名字）：

```sh
pnpm --filter @multica/core exec vitest run api/delivery-completion-hint.test.ts issues/completion-hint.test.ts api/delivery-completion.test.ts issues/completion.test.ts issues/completion-draft.test.ts
pnpm --filter @multica/views exec vitest run issues/actions/completion-hint-gate.test.tsx issues/actions/__tests__/use-issue-actions.test.tsx modals/issue-completion-hint.test.tsx modals/issue-completion-authorization.test.tsx
pnpm --filter @multica/core lint
pnpm --filter @multica/core typecheck
pnpm --filter @multica/views lint
pnpm --filter @multica/views typecheck
```

Go 编译/静态检查与 CLI 确定性验证：

```sh
go -C server build ./internal/handler ./internal/service ./cmd/server ./cmd/multica
go -C server test ./cmd/multica -count=1
go -C server vet ./cmd/multica ./internal/handler ./internal/service ./cmd/server
```

确认专属环境后，分包分次执行（每次单独实际环境；命令不是环境准入）：

```sh
go -C server test ./internal/handler -run 'Test.*DeliveryCompletion' -count=1 -json
go -C server test -race ./internal/handler -run 'Test.*DeliveryCompletion' -count=1 -json
go -C server test ./cmd/server -run 'Test.*DeliveryCompletion' -count=1 -json
go -C server test -race ./cmd/server -run 'Test.*DeliveryCompletion' -count=1 -json
```

所选 regex 必须与实际 `func Test...` 清单逐个比对，附 run/pass/fail/skip 明细；没跑到不算通过。此外按改动执行 service observer/authorization/receipt、controlled artifact 及 builtin skill 合同；精确用例在实现 diff 后登记，不能用只编译代替。禁止用 `go test -run '^$'` 当编译（会运行 TestMain）。sqlc 查询变更执行 `make sqlc` 并审生成 diff。

最终由 trellis-check 独立审 scope/code/spec、跨层数据流、no-auto-retry、权限及 artifact authority；根协调者完成真实 Web/Desktop 共享流程的确定性环境验收。E2E 用生产模式 Web 与单独 fixture DB；不以 Next dev 冷编译错误判断产品。Desktop 要有实际 renderer/运行壳证据，单 Web 测试不冒称 Desktop 已测；安装包/Windows/发布不在范围。

按项目工作约定执行受影响包测试与 `pnpm lint`、`pnpm typecheck`、`pnpm test`、`pnpm knip` 及 Go 受影响全包检查；全仓现有失败需与开始时并行 diff 分开记录，不修无关代码。`make check` 仅在自有环境与其完整执行图被核对后使用，不能附带真实 Provider、Hosted CI 或发布。检查通过后不无理由反复扩大测试。

## 7. 交付、回退与下一包

- [x] 本包验收结果链接、失败/跳过与限制写到任务 research；不得引用旧阶段测试数字作为本包通过。
- [x] 更新完成 evidence/hints spec 与 CLI 内置合同，只记已验证行为；保留 skip=false 与独立协作/R38 语义。
- [ ] 保留其他工作区改动；提交仅列本包明确文件，遵循 Lore intent/约束/验证 trailers。不提交其他代理材料，不整体 git add。
- [x] 任一包失败可回退本包客户端/只读入口接线；原 v1、授权终结和私有证据不删除。不切 off 绕过 gate。
- [ ] M1-S1 验收后回到本任务更新剩余矩阵，继续 F2 报告快照规划；不能把 roadmap 或独立真实运行任务整体关闭。

实际执行使用 Trellis 原生 implement/check 子代理并注入本任务 jsonl；每个 dispatch 以当时 `task.py current` 的路径开头并明确唯一 ownership。用户批准后激活任务；P1 后端、P2 Core、P3 共享 UI、P4 CLI 和最终检查均已完成各自实现/检查。实现报告见 `research/m1-backend-resume.md`、`research/m1-clients-resume.md`、`research/m1-cli-resume.md` 和 `research/m1-final-check.md`。

## 8. 2026-11-13 最终执行状态

| Lane | 结果 | 证据 |
|---|---|---|
| P1 Backend | handler 76/76 normal 与 race；router 4/4 normal 与 race；零 skip；build/vet 通过 | `research/m1-backend-resume.md` 与各独立 DB JSON/JSONL |
| P2 Core | 全量 338 files / 4,546 tests；当前复验 4 files / 21 tests；lint/typecheck 通过 | `research/m1-clients-resume.md` 与本轮复验 |
| P3 Shared Views | 当前复验 5 files / 84 tests；typecheck、受影响文件 lint 通过 | `research/m1-clients-resume.md` |
| P4 CLI | `go test ./cmd/multica`, vet/build、3 项内置 skill 合同测试通过；未知提交后的 `issued/not_checked`、错误收据和过期授权 fail closed | `research/m1-cli-resume.md`、`research/m1-final-check.md` |
| Visual/renderer | 未验证：当前 Desktop Demo 项目策略不为 enforce，未到达授权 modal；不可用该环境宣称 UI 通过 | `research/m1-final-check.md` |

Views 全包只在修复前运行一次，78 项失败中受影响 gate/modal/surface 与八个 timeout suites 后续单独通过；全包未在修复后重跑。M1-S1 产品实现和确定性测试已完成，但实际 enforcing renderer 验收仍是阻止将首包标为完全接受的事项。
