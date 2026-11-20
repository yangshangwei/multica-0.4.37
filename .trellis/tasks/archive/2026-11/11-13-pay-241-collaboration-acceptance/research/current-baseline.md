# PAY-241 当前基线与缺口

核对日期：2026-11-13。方法：只读源码、规范、测试和保留证据；未执行产品测试、读取凭据、调用 Provider、连接支付沙箱或改变旧任务状态。

## 1. 证据层次

1. 用户原方案是目标，24 条用例完整保留。
2. `artifacts/qa/pay-collaboration-20261113/retained-files/test-case-matrix-v2.csv:2` 是旧演练的修订分类：8 PASS / 1 CONTROLLED_PASS / 9 PARTIAL / 2 GAP / 4 SIMULATION_ONLY。
3. `artifacts/qa/pay-collaboration-20261113/report.md:11` 的 API 是 `ade2298e2`，Web 是当时工作区；`README.md:17` 说明开发进程。这不是当前版本统一构建的证据。
4. 旧演练 PASS 是其实际执行断言的结论，不能自动覆盖原用例所有措辞。例如 TC03 使用 urgent，TC23 验证请求/返回补充信息；它们不证明 P1 精确映射或全部四项阻塞必填信息。
5. 本任务的 `test-matrix.csv` 将旧分类与本轮结果分列；本轮全部为 NOT_RUN。源码存在、测试已编写、旧环境曾通过、当前实际运行通过，是四种不同事实。

## 2. 已有能力与不可替代的缺口

| 主题 | 当前事实、限制与依据 | 规划含义 |
| --- | --- | --- |
| 状态 | `server/internal/issuestatus/issuestatus.go:40` 定义七类；`server/internal/service/collaboration_lifecycle.go:62` 限制普通 change_status/reopen 进入 in_review；`collaboration_delivery_test.go:20` 保护交付验收语义 | 主协作任务的 in_review 不直接充当代码评审；业务阶段单独表达 |
| 人工身份 | `server/internal/collaboration/access.go:23`、`:82`、`:109` 根据验证主体区分人/机器与发起人/处理人/参与者；`server/internal/handler/workspace.go:547` 只有 owner/admin/member | 职业名称不是授权；需要阶段审批人配置决策 |
| 最终完成 | `server/internal/service/collaboration_lifecycle.go:475` 与 `.trellis/spec/server/collaboration-delivery.md:105` 要求发起人接受精确提交 | 产品/项目经理任意一方关闭，不是现有默认权利；不能把历史产品账号验收推广到全部业务角色 |
| 事务完成门禁 | `server/internal/service/delivery_completion.go:294` 与 `delivery_completion_authorization.go:40` 约束 enforce 策略下的完成授权；`.trellis/spec/server/delivery-completion-enforcement.md:5`、`:85` 限定证据含义 | 复用版本绑定/人工授权/幂等事务；不能据此宣称 QA、发布、线上验证已完成 |
| 阶段门禁 | `server/internal/handler/issue.go:4241`、`server/internal/service/collaboration_lifecycle.go:422` 处理现有状态/交付；旧 TC07/08 明确 GAP | PR/CI、人工代码评审、QA签字、发布和回滚需独立合同与否定测试 |
| 建议审批 | `server/internal/handler/agent_approval.go:22`、`:344`、`:358`、`:432` 有人工 approve/reject 与执行条件 | 不等同于“修改建议→接受指定版本→一次性生成子任务”，也不是宿主命令沙箱 |
| 版本冲突 | `server/internal/service/collaboration_workflow.go:395`、`:568` 校验协作版本/epochs；`server/internal/handler/issue.go:3426`、`:4218` 的普通 expected_revision 可选 | 协作和普通子任务分别验证，不能声称所有客户端一律无覆盖 |
| 实时恢复 | `server/internal/service/collaboration_realtime.go:94`、`:117` 与 `.trellis/spec/server/collaboration-realtime.md:40`、`:58` 提供保留 outbox 和重新读取 | 验收最终快照与持久时间线完整，WS 帧本身不是永久事件日志 |
| 作者展示 | `artifacts/qa/pay-collaboration-20261113/README.md:13` 记录公共评论 author=null；`.trellis/spec/web/frontend/collaboration-closure-qa.md:15` 保持 author 为人类摘要，具体 Agent 来自运行/资源 | PAY-6 是显示能力缺口；不能给人员 author 字段塞入 Agent 以破坏合同 |
| 审计回放 | `server/internal/handler/activity.go:15`、`server/internal/handler/handler.go:951`、`server/internal/service/collaboration_activity.go:37`、`:70` 及 `collaboration_command_history_test.go:95` | 联合活动、评论、指令、运行、交付和授权回执导出；单一活动列表不足以证明全部操作可回放 |
| 阻塞信息 | `server/internal/service/collaboration_handoff.go:20`、`:59`、`:93` 保留说明、处理人和恢复状态 | 解除条件/预计解除时间没有完整必填合同，不能因有 blocked 状态就判全部通过 |
| 外部业务 | 当前仓库未发现电商支付/订单/履约后端；旧演练只有本地支付模型 | 支付沙箱和发布系统另接真实证据；平台不自行冒充支付服务 |

## 3. 可复用自动化

- `e2e/collaboration-multi-account-closure.spec.ts:22` 已有产品/研发/QA独立会话与原生测试 poller；`:70` 明确支付证据为合成；`:235` 起验证失败→修订→人工验收。
- `.trellis/spec/web/frontend/collaboration-closure-qa.md:7` 将该场景指定为 canonical synthetic closure；复用并扩展，避免复制一套逐日期专用 E2E。
- `e2e/collaboration-editing-web.spec.ts:9`、`collaboration-workbench-web.spec.ts:183`、`collaboration-offline-result-notifications.spec.ts:15`、`collaboration-live-revoke-web.spec.ts:14`、`collaboration-handoff-chain-web.spec.ts:12` 分别覆盖编辑/冲突/通知/撤权/转交。
- `packages/core/collaboration/command.test.ts:119` 保留原幂等键；`packages/core/api/ws-client-account.test.ts:48` 与 `collaboration/run-handoff-queries.test.ts:112` 覆盖连接和运行恢复。
- 后端复用 access、delivery、lifecycle、completion_authorization、issue_revision、realtime、authority、fifo、command_history 与 workspace_preservation 测试。用共享 `internal/testutil` fixture，新建断言留在行为所在层。

旧脚本不足：`03-agent-and-ui.spec.ts:154` 的 TC13 是 `check(..., true)`，只记录说明；`04-recovery-and-sync.spec.ts:26` 只恢复一条评论；`:36` 仅四账号100并发GET；`:67` 注入进程退出而非真实模型超时。这些不能升级成完整权限、丢包恢复、100人同步或真实超时通过。

## 4. 执行前置

- `.trellis/spec/server/database-test-isolation.md:3`：新任务独立数据库，不能使用旧验收环境；`-run` 不会跳过 TestMain 的数据操作。
- `.trellis/spec/web/frontend/e2e-run-environment.md:11`：production Web；`playwright.config.ts:7` 单 worker / 零重试，`:20` 不自动启动服务。
- `e2e/fixtures/collaboration-host-runtime.ts:83`：隔离 password API、deployment identity、明确 Web/renderer origin、测试 daemon/provider/CLI 路径。
- 测试默认使用自建受控 Provider；不得解析或运行环境中已有登录的真实 Agent CLI。真实运行需单独明确范围与当前准入。
- 旧 `README.md:27` 要求重跑使用新工作空间；历史 PAY-1及PAY-2–6记录保留。新的 Trellis 任务不是再次创建业务 PAY-241 的证明。

## 5. 与现有任务的关系

| 已有记录 | 本任务边界 |
| --- | --- |
| PAY-2 | 真实支付与履约幂等；本规划阶段作为外部验收依赖保留 |
| PAY-3 | 职业角色和发布验收门禁；进入阶段审批设计包，不以现有 owner/admin/member 代替 |
| PAY-4 | PR/CI、发布和回滚集成；对齐 Git 工程物料规划及真实仓库规则 |
| PAY-5 | 100独立协作者与真实模型超时；分别安排确定性恢复/负载测试和真实 Provider 验收 |
| PAY-6 | 具体智能体评论作者展示；建议首包候选 |
| `11-12-agent-squad-followup-roadmap` | M1已交付完成门禁直接复用，F2等并行改动不由本任务修改或认领 |
| `11-12-collaboration-real-runtime-acceptance` | 不重置调用预算，不消除历史失败，不把受控Provider等同真实模型通过 |
| `11-10-git-engineering-material-workflow` | Git物料定义/获批版本与Multica讨论/运行的权威划分沿用；不暗含授权研发代码自动合并 |

源码行号来自本次只读核对；跨会话改动可能使行号移动。进入实施前核对对应符号与最新差异，不能用旧文件哈希覆盖并行成果。
