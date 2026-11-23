# PAY-241 新会话交接：r15 实际验收后的 GET 503 诊断

记录时间：2026-11-16，Asia/Shanghai。现有 S2 任务继续进行；本文件是本次交接的最新入口，旧进度中的“运行中”和旧 PID 已被下面的停机记录替代。

## 1. 接续任务与授权

- 工作区：`/Volumes/artisan/code/2026/multica-0.4.37`
- 分支：`codex/cross-workspace-collaboration`
- 交接时实际 HEAD：`1a5e537b0531aa51bd7e3eded83bbb801f1c5133`。有其他任务的改动和提交，不重置、不覆盖。
- **继续现有任务**：`.trellis/tasks/11-13-pay-241-s2-stage-approvals`，`in_progress`。不要新建 PAY-241/S2 任务，不从头规划。
- 用户已授权自主实现、修复、检查和本地验收。普通可逆下一步直接执行，不重复询问流程许可。本轮继续保持：不提交 Git、不新增依赖、不调用真实模型账户、资金或生产部署。
- 主会话协调环境、集成、规范和验收；实现/检查采用 Trellis 原生子代理，写域明确。每条派发以 `Active task: .trellis/tasks/11-13-pay-241-s2-stage-approvals` 开头。
- 先读本文件、当前 `task.json` 和相应 `prd.md`/`design.md`/`implement.md`；按需加载规范和证据。无需把所有历史日志或旧报告重新读一遍。

## 2. 最新实际结果与未解决问题

**r15 原四场景：1 PASS、1 FAIL、2 NOT_RUN，retries0。S2、A11/A12 和总验收尚未通过。**

| 原场景 | r15 结果 | 实际覆盖 |
| --- | --- | --- |
| `collaboration-stage-approvals.spec.ts:27` | PASS | Web/Desktop 角色、建议精确版本、代码评审、撤权/原键恢复、源权限，以及实际 CLI 报告、失败保留、QA 退回与重测后新批准 |
| `collaboration-stage-execution.spec.ts:233` | FAIL | 两个本地产物实际创建201→人工批准200→执行200，tar 和回执成功；随后代码评审表单 Reason 持续禁用，原20秒 fill 超时 |
| 同文件迟到部署场景 | NOT_RUN | 首失败规则停止，不能算环境条件 skip 或 PASS |
| 同文件阶段证据/M1/管理退出场景 | NOT_RUN | 首失败规则停止，最终完成仍待实际运行 |

当前首要诊断点：`e2e/fixtures/collaboration-stage-execution.ts:336` → `stageApprovalUI` → `StageJourney.releaseReady:163`。`Approve code review` 表单和 Reason textarea 已存在，但 fieldset 禁用；截图显示 **“Could not load collaboration. Try again.” / Refresh**。

保留的服务日志显示：同一个 reviewer 的候选 GET 先200后503，约15秒后再次503；P/Q 对同候选有200。表单按当前读取错误锁定符合现有防护，不能以旧的缓存200绕过它。

**尚未采集失败503的原始响应正文及底层 SQLSTATE，根因未证实。** 不要先归因于角色、自评、候选未选择、RR序列化、连接容量或背景任务；不能去掉 disabled、强制 fill、改权限、提高超时或加自动重试求通过。

诊断相关身份只用于对照原证据：

- case2 root：`01a1e224-a6de-7ea9-9d56-3c009b9c1460`
- candidate：`01a1e224-a964-769b-af29-70fbc780f8eb`，version1
- P：`486de1ce-aa43-49df-94b1-f711a5374e83`
- R：`a56c5bfb-ff4f-464c-b926-e1b59c30ac00`
- Q：`0a4c2faa-13e5-40ef-a3f2-9db05e528d75`

**未创建被动诊断副本/draft，未执行新的诊断 case。** 两个原代理已进入交接 hold，不再自动推进。

## 3. 已完成且可复用的检查

下面是不同层的精确范围，不把重叠测试数相加，也不把 scoped 通过冒充全 ROOT 验收。

| 检查范围 | 已接受证据 |
| --- | --- |
| P2 | `P2_CODE_ACCEPTED_R09_SCOPED`，阻挡0；可显式启用，项目政策仍默认关闭 |
| 联合 service | normal/race 各116顶层、321命名、263叶通过，`JOINT_SERVICE_ACCEPTED_R09_SCOPED` |
| 当前 handler/router 入口 | handler normal/race 各99/326/275；router normal/race 各54/208/174，四命令全部通过，零skip/fail/源漂移 |
| r11 当前服务来源复用 | 所选99/54测试、生产依赖及 service 文件与已接受e4字节一致；未选legacy/RK/迁移器整体检查不由此覆盖 |
| 前端产品修复 | 639参数记录、37文件；5 typecheck、2 scoped lint、2新生产构建通过；1676产物、90 NFT核验 |
| r13 Host target helper | 真实当前targetId→resource_id定位，支持squad；Node红6PASS/3FAIL→绿9PASS，六入口noEmit与两文件lint通过 |
| 最终运行产品 | server、multica、stage-router.test、host-daemon.test、host-provider、migrate 六项编译通过 |
| packager配置 | 真实生产源码 `artifact.New`：旧缺Identity配置禁用、新配置成功；未调用Prepare/Pack/DB，独立接受 |

本次已修复并窄整合：

1. `workflowAuthority` 显式选四个人工工作流版本字段，保留 Host 的 `execution_view_version:6` 独立合同。
2. Core 补齐六个已冻结执行/回滚 blocker 与 stored409状态，Views补对应双语原因和回归。未知码、derive、权限、CAS和原键逻辑保持。
3. `hostControl` 按当前公开target读取真正resource_id，再唯一选择目标行按钮，保留issue范围和精确请求/回执target断言。
4. 运行环境补 staging 子目录与必填 `MULTICA_ARTIFACT_CONFIG_ID`。旧缺目录/503失败和旧generation完整保留。

规范 `.trellis/spec/server/collaboration-stage-approvals.md` 已补充六码同步、stored409/恢复、真正未知码和 workflow/Host 请求边界。当前 SHA256：`c63287709e126159533a7042acf552a9f0341dda9ed3aac63b708e8850ba4aec`。

## 4. 证据定位与来源绑定

下面缩写仅用于读文件，展开后都是实际绝对路径：

```text
BASE=/Volumes/artisan/.multica-acceptance/pay241-20261115-continuation/resume-r05
RUNTIME=$BASE/runtime-current-r11-c54-r01
ACC=$RUNTIME/acceptance
CHECK=$BASE/final-a01-a12-r09-r02-check/current-r11-source-check-r01
```

| 文件（相对上面缩写） | SHA256 |
| --- | --- |
| `ACC/four-scenarios-r15-r01/result.json` | `d236ece490d9fca4702f617fc1cd8b71540254796d1885442de3bb22b66e97c7` |
| `ACC/execution-after-r15-r01.json` | `dc4c268ca0e1ed9be30ec5f23ced1aa791ea70ec420ba998a81f8b33d096b226` |
| `CHECK/r13-runtime-independent-r01/r15-latest-runtime-after-independent-handoff-review-r01.json` | `1345a5d58616459dd273632d925efda0798579edc8221d3757b854eed72359b1` |
| 同目录 `r15-artifact-proof-and-form-lock-source-review-r01.json` | `0fb32a29068841018193f3d0d33d32e4434ac36a418f0c88d9708b2ea177435b` |
| 同目录 `r14-code-and-runtime-subset-a-w-q-matrix-r04.json` | `440345e1d35e6ad3a44e66e61307ee3818503025a8e4d53aead98fbb4da838c1` |
| `BASE/server-current-r11-c54-r01/manifest.json`，5039文件 | `0a10ec899822babed7cd3ae5d9bc5c732672129479b2b66e5aba47b8eb68c092` |
| `RUNTIME/build-r01/build-manifest.json` | `321f74b5a9755ed348b7b198ae2b134233dee59c8276c5ba128e4a4c25b69c57` |
| `BASE/frontend-contract-r10-r01/current-overlay-r11-r02/input-binding-production.json` | `3af323729f85212f329315ad14bc0998c6bedcfde6d96203bb09f53fa57b638c` |
| `ACC/fixture-r13/input-binding-r13.json` | `665fb42dde8054add9a80cf0afd5c3a0d806e767c531d8661e696c3141e3b6a0` |
| `ACC/artifact-config-repair-r15/browser-env-r15.json`，私有0600 | `ea9b97b8d56f4265e3da5d2582fe9b31fcea783ea5ffb74e456c7bba686cd76b` |
| `CHECK/r13-runtime-independent-r01/r15-real-artifact-constructor-independent-accepted-r01.json` | `ac7f2b1463cbc8a8f4f359ed5fa947ee5b5ebb3b393a690dd7e2e3559fa70bb8` |

生产 Web BUILD_ID：`zAPjouCl2eCGWczJwKrjb`。

精确物理源码：

- server：`$BASE/server-current-r11-c54-r01/source`
- production frontend：`$BASE/frontend-contract-r10-r01/current-overlay-r11-r02/source`
- actual r15 test frontend：`$ACC/fixture-r13/source`

server/CLI/harness **编译元数据是c54**，后来实际ROOT HEAD不同已做字节来源核验；不要重写旧构建标签来假称最新HEAD，也不要仅因HEAD变化重建未变的依赖。知识新增的 `knowledgeintegration` tag测试未进入PAY默认Go编译范围，新会话仍需核查实际新差分。

r15 case2证据在 `ACC/four-scenarios-r15-r01/test-results/collaboration-stage-execut-4904f-separate-on-Web-and-Desktop-chromium/`，包含 `stage-execution-evidence.json`、三个PNG及原日志/DOM引用。服务端实际读取错误日志：`ACC/artifact-config-repair-r15/harness.stderr.log`。没有503正文/SQLSTATE时不能凭这些状态行推根因。

ROOT整合收据：

- 七个客户端路径：`$BASE/p2-verification-r07-r01/current-coordinator-r01/root-seven-path-integration-r11.json`，SHA `78866e6afbe5135967633e3897cee07b9223cf77e3af573fc1d6b70a5932695e`
- 两个Host测试文件：同目录 `root-host-control-r13-integration/receipt.json`，SHA `ad69ac37ef3e5b2a9db37f77d005a80ed70ef0d52bf792c62112e545911feac6`

## 5. 新会话的运行起点：已停机，全部历史保留

用户请求切会话后，ROOT完成所属服务停机：

- 旧bridge86282/API86283、Web23116、renderer23153 **全部退出**。
- 64922、64923、64924 **无监听**，PAY当前活动DB套件0。
- 原Playwright5433/recorder5424已退出；无新诊断或测试在途。
- 数据库 `multica_pay241_20261116_s2_runtime_r11` 保留，停机后其他连接0。
- 原门禁布尔恢复 `enabled=false / creation_enabled=true / host_execution_enabled=false`，**epoch10**。
- 10个Host run全部settled；未知/leased queue、未释放reservation、不完整artifact execution均0。
- **3条running合成producer记录保留**，不是Host工作泄漏，不要删除/改status来使原始count为0。
- **2条成功artifact execution与实际tar输出保留**，每个tar2048字节；旧批准、回执、报告与输入不清理。

停机证据：

- `ACC/new-session-handoff-r15/owned-services-stopped.json`，SHA `fb75ef62b00c66ac70bec7d46dd4dfc032c2d8b4a57ebb83f6402185307c90b0`
- 同目录 `database-after-stop.json`，SHA `25b0e2342fe366ad47b619a4d698c5495595987e3d90b534df049819d15b1c52`

私有可信状态：`/Users/artisan/.multica/pay241-r11/runtime-r01`。staging根0500、output根0700；其祖先必须继续符合原安全校验。

- 旧8 staging generation保留：1个前轮已sealed，7个未用。
- r15新8 generation保留：前2个sealed并各有一次成功执行，后6个未用。
- **下一次完整四场景必须分配全新8个UUID**，不能拿已有generation列表从slot0重跑。保留所有旧目录/inode/manifest/files；在API停止时临时放开staging根写权限、建新空0700子目录、恢复0500，然后构造并启动。
- output执行目录由packager按execution_id创建；不要错误要求新staging_id对应output目录预存在。

最新私有配置是 `ACC/artifact-config-repair-r15/browser-env-r15.json`，含已改密管理员和显式空模型key。**不得打印凭据、token、JWT或数据库密码**，不继承ambient账户。初始管理员不可再次初始化/改密；恢复API后用已有私有凭据核验即可。

启动模式必须继续使用 `stage-router.test` 与既有JSONL/socket bridge；它自身就是实际API，**不能同时启动regular server**。重启参考 `ACC/artifact-config-repair-r15/bridge-config.json`、`restart-plan.json`，新日志/ready/退出收据使用新目录，不覆盖旧记录。

## 6. 新会话立即执行的顺序

1. 用Trellis继续现有S2，核本交接、任务指针/Git变化、停机事实和需要的源码规范。不重复创建任务或重跑已接受639/116/99/54检查。
2. 对**原case2表单禁用/GET503**做有边界的根因调查。先读取最新source review、三张截图、表单锁规则和日志；设计最小被动记录，捕获**原reviewer身份**的issue/stage/exactcandidate真实200/503正文及必要底层读错误/SQLSTATE。保留原fill、角色、请求身份、20秒/480秒，不用admin冒充reviewer。
3. 如需重现场景，在新私有诊断副本中只加被动记录，先静态/独立审查，再明确一次原case2诊断命令。诊断通过也不增加四场景验收计数。若需服务侧被动错误采集，只能最小test-only接线，不能改变隔离级别、重试、权限或业务顺序求通过。
4. 分配新8 generation、新配置/日志/输出目录，恢复匹配API/Web/renderer，核真实listener、BUILD_ID、管理员及全新before。constructor配置/根身份未变可引用已接受证据；新source或新问题才补必要检查。
5. 根因修复通过独立检查后，以新的来源绑定**跑原完整四场景**，workers1/retries0/maxFailures1、原480秒/20秒不改。首失败停后续，保全4分母和NOT_RUN，保留每次失败；不自动重跑同键、已sealedgeneration或旧许可。
6. 四场景和after通过后，独立逐项评审12A/18W/12Q、更新父矩阵/规范/任务记录。S2未实际接受前不归档、不进入S3全面恢复实施。

当前父任务24例仍 **0 PASS、7 PARTIAL、17 NOT_RUN**。TC08/12/13已有可记录的评审/QA进度，后续按每行原要求和实际证据更新；同首case多次PASS只计一次，S4真实支付/履约/远端CI/发布人签字不由合成场景覆盖。S1与S3 timeout-proof已归档，S3完整恢复、S4真实输入、S5最终候选与总验收保持开放。

## 7. 并行知识任务与资源协议

知识协调线程：`01a1df7d-d12f-74b2-a959-72af2045a06e`。用户已授权会话间协调，可读取/向该线程发送任务与资源状态；不要向真实用户/外部渠道发送消息。

- **全局最多2套数据库测试**；PAY最多1，知识最多1。PAY现停机套件0，新会话恢复前向知识线程更新占用。
- 知识最近冻结SOURCE `8877eb98…`/HEAD1a5，10407files/32links；它自己的TAG修复与接受仍独立。
- 知识仅tag测试 `server/cmd/server/collaboration_knowledge_host_composite_test.go` 已窄整合；默认Go/PAY生产依赖未因该tag改变。不要把TAG失败或通过混入PAY。
- 修改共同source/spec/fixture前通知知识协调方，尊重其短COPY窗口；私有诊断、read-only、外部配置准备可继续。完整SOURCE冻结不等于停止所有任务，但新差分必须明确登记。
- 旧temp资产missing、旧运行失败、旧源/私有构建及JSON收据全部保留，不恢复或删改他任务文件。

本轮所有代理已hold。新会话可按工具实际可用性恢复原代理或建立明确责任的新实现/检查代理；**不得假定旧代理仍执行或重发已完成命令**。
