# Research: PAY-241 S2 总验收依赖的 S4 真实环境前置核对

- Query: 截至现在核对 E01–E07 真实外部输入、最小缺字段、可离线推进的合同检查和安全填写清单。
- Scope: internal；只读仓库/任务资料与已经登记的私有 runtime/manifest 安全投影；不实施或激活 S4。
- Date: 2026-11-15
- Captured at UTC: 2026-11-15T03:41:04.327Z；展示时区 Asia/Shanghai（UTC+08:00）。
- Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`。唯一写入本目录的本报告及 JSON；不改 S4、PRD、父矩阵、代码或运行配置。
- Result: **E01–E07 全部 NOT_RUN；已读资料没有绑定真实 PAY payment/fulfillment sandbox、native repo/CI/build、非生产 release/rollback/metrics 与指定业务签字人。** 缺登记不等于证明用户没有这些资源。

## Findings

### 1. 读取范围与找到的资料

| 文件 | 用途 |
| --- | --- |
| `.trellis/tasks/11-13-pay-241-s4-external-integrations/prd.md:13` | E01–E07 与真实沙箱/非生产边界；没有配置不能模拟通过。 |
| `.trellis/tasks/11-13-pay-241-s4-external-integrations/design.md:7` | 五类实际目标、权威账本与回调/时间协议。 |
| `.trellis/tasks/11-13-pay-241-s4-external-integrations/implement.md:5` | 接入准备已完成；真实执行/检查仍开放。 |
| `.trellis/tasks/11-13-pay-241-s4-external-integrations/research/integration-readiness.md:94` 及 `.json` | I01–I10 intake；十组实际 target/owner/authority 未填写，七例 NOT_RUN。 |
| `.trellis/tasks/11-13-pay-241-s4-external-integrations/research/integration-readiness-followup-20261114.md:9` 及 `.json` | 历史 DTO-only/未修复描述；不能作为 11-15 现行实施结论。 |
| `research/server-increment-2b-acl-fix-check.md:4`、`research/server-increment-2c-followup-check.md:4`（当前 S2） | 2b ACL 与 2c C01/C02 窄修复已独立关闭；不是完整 S2/S4 验收。 |
| `research/joint-build-preparation-20261114.md:3` 及 `.json`（当前 S2） | 已有本地生产模式 Web/原生 Desktop 前端构建；实际联合场景 0/4。 |
| `.env.example:537`、`.github/workflows/ci.yml:3`、`.github/workflows/release.yml:3` | 公开配置/流程确实存在，但未指定 PAY 实际目标、执行 run 或授权。 |

完整已读文件路径、用途与当前 SHA-256 在 companion JSON 的 `files_found`；源码哈希仅识别读取字节，不代表已经部署或真实验收。

### 2. 已登记私有 runtime/manifest：实际安全核对

精确路径来自 `.trellis/tasks/11-13-pay-241-s2-stage-approvals/implement.md:17`、`.trellis/tasks/11-13-pay-241-s2-stage-approvals/implement.md:20`、`.trellis/tasks/11-13-pay-241-s2-stage-approvals/implement.md:28` 与 companion JSON；没有遍历用户账号/凭证目录，也没有读取其他配置文件。

- `/Users/artisan/.multica/acceptance/pay241-20261114-resume/resume2-coordinator-state.json`：记录时间 `2026-11-14T15:03:42.976861+00:00`，登记 `configs/runtime-r01.json`；明确真实 S4 E01–E07 未运行。
- `/Users/artisan/.multica/acceptance/pay241-s2-20261114-r01/configs/runtime-r01.json`：44 个键；只公开 allowlist 的安全开关/字段存在性。`MULTICA_TEST_STAGE_HARNESS=1`、`MULTICA_AUTH_MODE=password`；四个 GitHub App 字段明确为空，VCS enable 与 `METRICS_ADDR` 未声明，Cloud URL/邮件入口为空。未发现支付/订单/履约/发布/回滚/指标的业务键或具体目标登记；不是账户/加密连接的全量盘点。
- `/Users/artisan/.multica/acceptance/pay241-s2-20261114-r01/allocation.json`：记录时间 `2026-11-14T11:05:13.976160+00:00`；`api_started=false`，仅短 socket、0500 staging、八个 0700 槽位的隔离分配。**这是历史分配字段，不是现在的进程/服务在线审计。**
- `frontend-build-r01/input-binding-production.json` 与 `frontend-artifacts.json`：实际存在；3,127 输入文件、1,961 链接、1,586 构建输出，Web build ID `SaRnyLTvwkVOxOqh_KuHE`，登记输入/输出哈希与公共构建报告相符。这是本地 Multica 前端构建，不是 PAY 远端 CI 制品；最终 API/CLI/Host 和运行级关联仍需 S2 coordinator 完成。
- 凭据/DSN/URL 认证部分、HOME/环境原文、proof 原文均不复制。本报告只保存安全投影 SHA-256，不保存 credential-bearing runtime 原文或其整文件哈希。

### 3. 旧 follow-up 与当前事实必须分开

| 旧时点描述 | 11-15 当前证据 | 仍不能推导 |
| --- | --- | --- |
| S2B-C01 首红、尚未修复 | `research/server-increment-2b-acl-fix-check.md:4` 已关闭；`server/internal/service/collaboration_stage_evidence_read.go:15` 使用可加权限锁的 RR 读事务，`:84` 沿用原制品 observer。 | 不代表真实签字/所有 S2 writer/整个联合验收通过。 |
| 2c 只有 DTO，账本/消费者未实施 | 已有 `StageExecutionSources`、Prepare/receipt/observation/metric、三类持久账本；`research/server-increment-2c-followup-check.md:4` 关闭 C01/C02。当前 follow-up 三文件与登记冻结哈希全部一致。 | 不代表生产已配置真实适配器或已部署/回滚。 |
| 指标等版本歧义仍待实施 | `server/internal/service/collaboration_stage_execution_metrics.go:118` 与 `server/pkg/db/queries/collaboration_stage_execution.sql:69` 保留歧义；typed `rollback_observation` 已存在。 | 真实 source/native order/query/rule/sample/新批准仍未提供。 |
| 原 GitHub upsert 会接受旧 head | `server/pkg/db/queries/github.sql:141` 现有时序围栏、同时间异 head 使 strict CI 不完整；`server/internal/handler/github.go:1588` 拒绝后不派生关联。 | E01 真实 provider 重投/撤权仍 NOT_RUN；不把旧静态风险继续写成现行已复现缺陷。 |
| 没有可用构建或“production”意味着真实发布 | 本地 Web/原生 Desktop 前端构建已有；匹配 CLI 和最终 API/Host 运行仍待绑定。 | `next build` 的 production mode 不是生产部署，也不提供 PAY 原生 CI/build 对应关系。 |

### 4. I01–I10 当前最小缺项与责任

ID 是 intake 编号，**不是新环境变量名**。已知本地源/构建不必让用户重新提供；真正的业务系统/目标/人员由其 owner 提供。

| ID | 当前 readiness | 最小具体 missing | 谁提供 |
| --- | --- | --- | --- |
| I01 | PARTIAL_LOCAL_ONLY | 最终 API/CLI/Host/Web/Desktop digest/version、实际 S4 run 与 workspace/project/root/child/candidate/revisions。 | S2 coordinator 生成最终绑定；业务 owner 补实际环境身份。 |
| I02 | UNBOUND_EXTERNAL | provider/instance/version、remote repo ID/path、local/remote integration IDs/trust、允许 branch/PR namespace、native rules 与 read/replay/revoke/cleanup grants。 | 仓库 owner/admin。 |
| I03 | PARTIAL_LOCAL_ONLY | 外部 workflow revision/required-set、native run/job/check/attempt、source-vs-merge SHA、immutable build digest/来源/retention。 | CI owner；已有本地构建由 coordinator 复用。 |
| I04 | NOT_SUPPLIED | 实际 PAY provider/官方 sandbox API/version、sandbox tenant/merchant/endpoint/instrument_ref/amount/currency、create/pay/query/补偿与资金账本读权、idempotency scope/TTL。 | PAY 支付系统 owner。 |
| I05 | NOT_SUPPLIED | order/fulfillment service/version/contract、payment linkage、订单转移/履约效果的两个权威账本、dedup/queue/failure/recovery/补偿控制。 | PAY 业务/履约/消息 owner。 |
| I06 | NOT_SUPPLIED | callback routing/auth/replay 合同、private signing reference/generation、event/attempt 身份、限制原文存储、UTC skew/单调时钟/真实 45s 注入点与 deadline/grants。 | provider/callback/消息 owner。 |
| I07 | NOT_SUPPLIED | 非生产 env/remote instance/tenant/target/domain、官方 release/rollback API-or-CLI/version、conditional execution + original query、当前部署/conditional version/新旧 digest、executor/授权/health query。 | 非生产 operator。 |
| I08 | NOT_SUPPLIED | metric provider/stream/native-order/freshness、query/labels/window/baseline/threshold/rule/version/digest、原 sample refs/time/hash、fault/restore grants。 | observability/operator。 |
| I09 | NOT_SUPPLIED | 指定 implementer/reviewer/QA/release/creator IDs、当前 source membership/participant/role/account revisions/expiry、test-or-formal 标识；QA≠release。 | 项目/业务 owner；不能用职业或测试开关替代。 |
| I10 | IMPLEMENTED_LOCAL_NOT_EXTERNAL_BOUND | 最终 2d 全 writer/运行源绑定审查；实际 trusted adapter/version/integration/trust/config/target mapping；原键/observations/metrics/受限证据当前读权。 | S2 owner/coordinator 后续完成；真实适配器计划/实现/检查由对应 owner 承接。 |

### 5. E01–E07 状态与最小执行前提

公共前提是 I01 的最终运行身份与 I09 的明确人员/授权。运行后才产生的 order/payment/fulfillment、operation/fact/report/submission IDs 是**待产生证据**，不是让用户手填成功结果。

| Case | Status | 除公共前提外的最小缺项 |
| --- | --- | --- |
| E01 / TC06–08 | **NOT_RUN** | I02 的实际 repo/integration/trust/允许 branch/PR；I03 native required-set/run/attempt→head/build digest；I06 真实 signed replay/旧 head/撤换连接授权。 |
| E02 / TC09 | **NOT_RUN** | I04 PAY official sandbox/tenant/endpoint/instrument_ref/金额币种；I05 三域身份/权威查询；I06 callback/auth/截止时间。 |
| E03 / TC10 | **NOT_RUN** | E02 目标；同一事件合法重投三次设施及 event/attempt 语义；资金/订单/履约各自 dedup scope/TTL/效果键/账本读权。 |
| E04 / TC11 | **NOT_RUN** | E02 目标；真实 45s 注入点/时间容差/UTC/单调计时/deadline；超时后原操作查询、乱序/消费失败/恢复/补偿控制与授权。 |
| E05 / TC17 | **NOT_RUN** | I07 确切非生产目标/官方调用与条件执行/原查询；matching native build/新旧 digest；真实 source/consumer、不同 QA/release、拒绝零调用/执行观察入口。 |
| E06 / TC18 | **NOT_RUN** | E05 输入和成功原部署/立即前一 digest；I08 真实指标 stream/query/window/rule/threshold/sample；允许 anomaly/restore/rollback、精确人类批准与健康查询。 |
| E07 / TC19–20 | **NOT_RUN** | 实际 E05/E06 facts 与 verification run/report；精确 final candidate/delivery/submission；当前有效决定；2d/M1 所有实际完成 writer 的最终证据。 |

### 6. 本轮已做的必要离线合同核对（不是 S2 测试重跑）

L01–L08 都是 `SOURCE_CHECK_OK`：JSON/引用/字段、源入口与哈希的静态核对；**不等于行为 PASS 或 E01–E07 通过**。

- L01：原 intake 十组/七例/引用完整；两份旧 readiness 仍为 NOT_RUN。
- L02：`server/internal/service/collaboration_stage_execution_types.go:14` 六方法内部 source；proof `json:"-"`；source 仅在 `server/cmd/server/collaboration_stage_execution_control_test.go:336` 的测试构造显式注入，正常构造无实际实现。
- L03：`server/internal/service/collaboration_stage_execution.go:33` 分开 pending/unknown/not_started 与 fact；`server/pkg/db/queries/collaboration_stage_execution.sql:109` 仅 succeeded/failed。`PrepareStageExecution` 在 definite commit 后一次 entry，原键恢复不再调用。
- L04：`server/internal/service/collaboration_stage_execution_identity.go:28` 强制 conditional/original-query 与稳定物理身份；指标 `ambiguous` 持久状态及当前 query/rule/expiry 合同可读。
- L05：当前 GitHub 时间/head 围栏已替代旧风险描述；不做重复 2a 回归或真实重投。
- L06：GitHub query/storage 仍缺 native run/job/attempt/build identity；VCS CI 无 repository/run；`server/internal/service/collaboration_stage_evidence.go:116` strict gate 仅 completed/success，display skipped/neutral 不能当批准依据。
- L07：`server/internal/handler/cloud_billing.go:520` 转发到 Cloud billing；不会据此选择 PAY payment adapter。公开 `METRICS_ADDR` 和 desktop-updates 配置也是本产品能力/样例，不是 PAY metric/非生产部署目标。
- L08：登记 runtime 安全投影、实际输入/输出 manifests 与 2c 三文件哈希核对；未启动 API/浏览器/Provider，也未执行任何 S2 产品测试。

**可继续、无需远端调用的工作**：主会话完成已有 2d/匹配构建/最终合同核对；外部 owner 提供版本化官方合同及非敏感 schema 后，离线核对三域 idempotency/TTL、回调身份、native CI provenance、conditional/original-query、metric 顺序与现行 typed 字段是否能对应。合同未提供的项保持 WAITING_FOR_INPUT，不猜供应商接口、目标或配置名。不得以重新跑 S2 合成测试取代这些缺项。

### 7. 简洁可填写 safe-input checklist

仅填写名称、ID、官方文档/合同版本、受限证据引用和 scope grant；`private_ref` 只指向 owner 控制的凭据，不贴值。禁止 bearer token、secret、DSN/带认证 URL、PAN/CVV、proof/原回调体。下面空白不是通过证据；JSON 同名 checklist 保留空值供 owner 填写。

```text
I01 final_stack/run: source/API/CLI/Host/Web/Desktop digest/version; run/候选/业务环境 IDs =
I02 repo: provider/instance/version; repo ID/path; integration/trust; branch/PR; native rules/grants =
I03 CI/build: workflow/required-set; run/job/check/attempt; head/merge semantics; digest/provenance =
I04 PAY sandbox: actual provider/contract; tenant/endpoint/instrument_ref/amount/currency; charge reader/grant =
I05 order/fulfillment: services/version/linkage; order/fulfillment ledgers; dedup/queue/failure/recovery =
I06 callback/time: routing/auth/replay; private_ref/generation; event/attempt; clocks/45s/deadline/grant =
I07 nonproduction: env/instance/tenant/target/domain; contract; conditional/original-query; digests/executor/grants =
I08 metrics: source/native order; query/window/baseline/threshold/rule/version; sample refs; fault/restore grant =
I09 people: implementer/reviewer/QA/release/creator IDs; membership/revisions/expiry; test-or-formal; QA≠release =
I10 trusted boundary: actual adapter/version/trust/config/target; final2d review; original-key/evidence ACL refs =
```

### Related specs / external references

- 已读 `.trellis/workflow.md`、`.trellis/spec/server/collaboration-stage-approvals.md`、`security-boundaries.md`、`verification-environment.md` 与 root `CLAUDE.md`：原签字/来源权限、运行身份、受控事实不可冒充真实事实。
- 本地官方 API 引用：`server/internal/integrations/ghsnapshot/client.go:162` GitHub installation token 与 GraphQL client；`server/internal/handler/github.go:917` REST header version 2022-11-28；`server/internal/integrations/vcs/gitlab.go:205` GitLab /api/v4/user。只读源码，**没有下载官方文档或调用 API**；真实 provider/instance/payment/fulfillment/release/metrics 版本仍待提供。
- `server/go.mod:3` Go 1.26.6；依赖版本不是外部业务 API/version 的证明。

## Caveats / Not Found

- 未做网络/API（含本地产品 API）、账号/账本/数据库/生产/付款/模型调用、凭证目录遍历、Git 操作、环境/secret 轮换或重复 S2 测试。只读明确登记的私有文件安全投影，不复制凭据值。
- runtime/coordinator/allocation 是带时间戳的既有记录；未做当前进程/remote connectivity 审计，不能把 historical `api_started=false` 当作此刻必然没有 listener。四个 GitHub 字段为空只对选定 S2 test runtime 成立，不证明所有账户/加密连接不存在。
- 文档、局部 source freeze 和当前工作区仍可能并发变化；3 个 follow-up 字节匹配不等于当前整仓/最终 2d 已冻结或所有已部署组件匹配。记录的 source head 来自 manifest，未读取 Git。
- 旧 follow-up 的 2b/2c 描述仅作为历史；已有局部独立结果和本地构建不填 S4 PASS。原 2c follow-up `55P03` 首次 race 原因仍未知，未删除或重新归因。
- E08 独立真实证据检查也未运行。S4/父 PAY 完整外部验收保持开放；S2 本身是否满足 A01–A12 由主会话最终证据判断，本研究不改其 PRD/状态或父矩阵。
