# 多账号跨空间业务闭环测试报告

日期：2026-11-09 至 2026-11-10（北京时间）。已有主线独立复现 **4/4 通过**；新增业务场景 **S1 外包交付、S2 客服、S3 异常治理均通过（3/3）**。三个最终结果分别来自 B6、B5、B3，没有宣称同一次调用跑完全部场景。最终源码审计、端口释放与数据库闸门恢复已验证。既有扫描 15 项公开标识符误报已逐项复核，实际凭据形态命中 0；原扫描报告保留。独立事实复核通过；最终字节签名见[复核记录](final-evidence-review-20261110.md)。测试与证据仍为未提交工作，任务暂不归档。机器台账见 [results.json](results.json)。

API、PostgreSQL、Web、CLI、调度与 daemon/Provider 子进程真实运行，Provider 输出由测试控制，账号和业务素材均为合成数据。每个场景分别核对运行终结与业务交付；排队、回执或模型式文本本身不算验收完成。本报告不替代 AC43、E5、H3a、E0b/K1–K3 的真实模型验收。

## 候选与环境

- 产品候选：`b10da8fec9dab057f9830bd336369d42e27b8c85`；快照树 `d53bae89c9894de1676d52a5797f680a7b469227`。使用 `git archive` 的隔离源码；只同步本任务新增 spec、fixture 和测试配置。
- 数据库：`multica_qa_biz_20261109_b45ace53198f`。API/Web/renderer 端口为 `59765/59766/59767`；password 认证，托管安装启用，同一台 macOS、同一 OS 用户。运行 PID、部署身份、构建 hash、workspace 链接在 [allocation.json](environment/allocation.json)，各构建记录在 `environment/`。
- R1/R2/R3、B1/B3/B5 的知识注入关闭；B4/B6 开启。API 切换历史保存在 allocation 的 `api_config_history`，没有通过修改产品实现改变结果。
- Playwright 单 worker、自动重试为 0；测试修正后使用新 run ID，所有失败均保留。
- 前置源码审计发现 `next-env.d.ts` 是 Next 构建生成的声明差异，保留副本后恢复候选字节，运行构建未变。见 [声明恢复记录](environment/generated-declaration-restoration-20261110.json)；其后 [源码审计](environment/source-audit-002132.json)核对 27,607 个受管理文件，漂移为 0。[最终审计](environment/source-audit-003345.json)再次核对同样 27,607 个文件，漂移为 0。

## 场景、预期与实际

### S1 外包交付与知识复用：通过

拓扑：甲方 A 拥有来源空间；乙方经理 B 拥有独立交付空间、运行环境及 Leader/成员小队；工程师 C 没有任何工作空间，作为外部参与者处理任务；竞争乙方 Y 无权访问。完整账号/空间/运行 ID 见 [S1 证据](b6-outsourcing-20261110/test-results/collaboration-business-sce-e8bed-nd-reuse-approved-knowledge-chromium/s1-outsourcing-evidence.json)及机器台账。

预期链路是 A 发起 → B 邀请并转交 C → B 提供小队、C 采纳、B 逐资源授权、C 显式恢复 → Leader 派活、成员回写、Leader 续作 → C 交付、A 退回、C 修订 → C 从共享上下文提出知识 → B 无权批准、A 批准并完成验收 → 同来源空间二期运行收到该候选。

[B4 原始报告](b4-outsourcing-20261110/playwright.json)已执行到小队三段运行、首轮退回和 Leader 修订，随后卡在共享上下文抽屉关闭后打开知识候选的测试定位。失败截图中抽屉已关闭、外层知识按钮可见；记录无页面错误、清理错误，测试仍为失败。[B6](b6-outsourcing-20261110/playwright.json)完整复验 1/1 通过：一期最终 done，第一条交付 returned、第二条 accepted；B 批准知识返回精确 `403 knowledge_permission_denied`，A 在页面确认版本并批准。二期实际运行收到候选 `e7f6900e-2de3-4ff0-bdbf-8882cf2f7a73`，来源任务、版本和内容 SHA256 相符；Y 对两期读取均被拒绝。

本场景共 5 次 Provider 运行：Leader、成员、续作、修订 Leader、二期单 Agent；均实际结束，5 个账本均为 `proof_acknowledged / host_exit_observed`，daemon exit 0。页面错误和清理错误为 0，场景恢复到进入时闸门状态；最终环境闸门另见收尾记录。二期通过公开 API 创建以建立复用上下文，授权、恢复、提及走 UI；只验二期知识注入与运行成功，没有声称二期业务也已提交并验收。

### S2 客服工单、附件与重开返工：通过

拓扑：客服 S 拥有客服空间；研发 D 拥有研发空间、运行环境和排障 Agent；无关账号 X 用于拒绝检查。最终账号、空间、安装、运行环境及 Agent 的 ID 全部在 [S2 证据](b5-support-20261110/test-results/collaboration-business-sce-7a501-reopened-ticket-is-reworked-chromium/s2-support-ticket-evidence.json)。

| 预期 | 最终实际 |
| --- | --- |
| 手动工单、请求/返回信息、共享两份附件 | 页面完成创建及信息往返；客户日志与样例 CSV 均交付 |
| 手动模式拒绝 AI；只有发起人可改策略 | 资源提供返回 `409 task_manual`；切为 assisted 后队列仍暂停；研发改策略返回 403 |
| 授权后仍需显式恢复；运行接收原始附件 | 恢复前无 Provider visit；恢复后两份文件 SHA256 与上传内容相等 |
| 运行成功不等于业务已验收 | 首次运行 `success` 且有退出证明时，任务仍未 done；随后提交、客服验收 |
| 重开后重新分派、显式恢复、再次交付 | 重开保持客服处理人；客服通过 UI 明确转回研发，通道仍暂停；恢复后第二次运行、第二次提交和验收 |
| 交付可追溯、外人不可见 | 两条 accepted 历史保留，最终 done；X 读取被拒绝 |

[B5](b5-support-20261110/playwright.json) 1/1 通过。两次实际 Provider 运行均关闭，daemon exit 0；两个退出账本均为 `proof_acknowledged / host_exit_observed`。页面错误和清理错误为 0，场景恢复到进入时闸门状态；最终环境闸门另见收尾记录。截图位于同一 test-results 目录。

### S3 运行中撤权与异常治理：通过

拓扑：负责人 L 拥有项目空间；O1/O2 各有独立空间、运行环境和 Agent，向同一协作任务提供资源。完整身份与过程见 [S3 证据](b3-support-governance-20261110/test-results/collaboration-business-sce-4837c--and-the-issue-still-closes-chromium/s3-governance-evidence.json)。

| 预期 | 最终实际 |
| --- | --- |
| O1 在途撤权停止真实进程，保留排队输入 | `grant_revoked`、`grant_required`、`stop_requested`；未 release 的运行终止，排队指令保留 |
| 另一拥有者 O2 不受影响 | O2 独立运行成功；其授权与目标未随 O1 撤权停止 |
| 再授权不等于恢复；未证明停止不能复跑 | 再授权仍 hold；停止证明未到时恢复返回 `409 stop_unconfirmed`；证明到达、显式恢复后保留输入只执行一次 |
| 移除 O2 同时隔离执行、等待与访问 | 在途运行停止，等待指令取消，读取返回 404；removed 通知无标题且不可打开 |
| 取消与重开保留控制边界 | 取消后恢复返回 `409 issue_closed`，合作方重开返回 403；负责人重开，显式恢复后执行、提交、验收 done |

[B3](b3-support-governance-20261110/playwright.json)中 S3 通过。O1 共 3 次、O2 共 2 次真实 Provider visit；两次在途停止均未放行便终止，另外三次正常放行。5 个账本均有 `proof_acknowledged / host_exit_observed`，两个 daemon exit 0，页面错误和清理错误为 0，场景恢复到进入时闸门状态；最终环境闸门另见收尾记录。

## 历次运行与发现

| 批次 | 结果 | 说明 |
| --- | --- | --- |
| [R1 closure](r1-closure/playwright.json) | 1 通过 | 三个拥有者的交付闭环，历史独立复现 |
| [R2 host](r2-host/playwright.json) | 2 通过 | 小队三段执行、运行环境切换，历史独立复现 |
| [R3 memory](r3-memory/playwright.json) | 1 通过 | 两个运行配置通过真实 run-local CLI 交接共享上下文，历史独立复现 |
| [B1 support/governance](b1-support-governance/playwright.json) | 2 失败 | S2 请求体混入执行字段；S3 多目标控件未限定 |
| [B3 support/governance](b3-support-governance-20261110/playwright.json) | S3 通过，S2 失败 | S2 首次交付和两次运行已执行，但重开后遗漏处理人显式回交 |
| [B4 outsourcing](b4-outsourcing-20261110/playwright.json) | S1 失败 | 抽屉关闭动画与重新选择导航产生竞态 |
| [B5 support](b5-support-20261110/playwright.json) | S2 通过 | 补齐客服显式转回研发的真实 UI 步骤 |
| [B6 outsourcing](b6-outsourcing-20261110/playwright.json) | S1 通过 | 新批次完整重跑，覆盖知识批准、一期验收、二期注入 |

三个最终场景合计 12 次 Provider visit，其中 10 次正常完成、2 次按撤权/移除操作预期终止；12 个运行都保留退出证明。B3→B5→B6 的新增 spec 只修改对应场景，fixture SHA256 均为 `14d9be8fd6a6b60a85c8afd1d1b71aaa38744cf6b330c453631b60f98207e0d2`。三次源码同步记录见 [B3](environment/test-sync-002333.json)、[B5](environment/test-sync-002811.json)、[B6](environment/test-sync-003148.json)。

保留的全部运行共 11 次测试尝试：7 通过、4 失败、0 跳过、0 自动重试；7 个不同测试的最终选定结果均通过。四项执行失败属于测试设施或场景契约，当前没有据此确认产品缺陷：

1. S2 用执行授权 helper 生成普通工作流请求，附加的 `execution_view_version` 被严格解码器拒绝（400）。修复为正确字段，保留精确 403 权限断言，没有放宽产品权限。
2. S3 将单目标控件 helper 用于两个目标，`Resume target` 定位重复；未等待的响应 promise 又在页面关闭后掩盖点击失败。新 fixture 按资源 ID 限定，联结点击/响应并断言目标 ID。
3. S2 误以为重开自动把责任交回研发。真实语义保持验收后的客服处理人；新增 UI 转交并验证 AI 仍暂停，不改产品行为。
4. S1 在 Escape 后立即用瞬时可见性选中关闭中的抽屉，点击期间按钮变隐藏。修复等待抽屉确实隐藏再打开知识候选。B6 完整重跑通过，知识拒绝断言同步收紧到确切 403/code。

环境方面，前置审计的生成声明漂移已留痕恢复；收尾发现逐场景的 restore 仅恢复进入时状态，曾继承已开启的闸门。因此 `gate_restored=true` 不代表恢复最初部署基线。主会话停止自有服务后，单独将本任务数据库恢复为 `creation_enabled=true / enabled=false / host_execution_enabled=false`，epoch 单调增加至 14，见 [最终闸门恢复](environment/final-gate-restoration-20261110.json)。知识注入已切回 off。既有扫描 15 项公开标识符误报已逐项复核，实际凭据形态命中 0；[原扫描报告](credential-scan.json)保留，最终复核明细另列于 [扫描复核](credential-scan-adjudication-20261110.json)。这是逐项复核后的结论，原扫描程序结果仍为 15 项，不将其改写为原始零命中。各失败批次的原始 JSON、截图与清理结果保留，不将修正后的通过追溯写入旧批次。

## 覆盖边界与复现

| 范围 | 已归档 Codex 主线/本任务复现 | 新业务覆盖 |
| --- | --- | --- |
| 多拥有者人类交付、转交、退回与验收 | R1 通过 | S1/S2/S3 均通过 |
| 同一拥有者小队三段执行 | R2 通过 | S1 三段执行与后续返工通过 |
| 原生 host 运行环境切换 | R2 通过 | 沿用独立复现，无新增声明 |
| 共享上下文跨运行接续 | R3 通过 | S1 候选批准与二期注入通过 |
| 手动转 AI、两文件哈希、验收后重开返工 | 原主线未包含完整链路 | S2 通过 |
| 运行中撤权、移除参与者、内容隔离、取消重开 | 原主线仅有已结束运行撤权 | S3 通过 |

不覆盖真实模型推理质量或真实供应商准入、不同拥有者混合一个小队、跨物理机/操作系统隔离、离线恢复、原生 Electron/Desktop 操作、移动端、安装包、全站无障碍、压测或生产部署。构建 renderer 只是 host fixture 的环境前置，不代表桌面业务验收。

以下命令须先核实本任务私有环境归属并使用新的 run ID；原批次不得覆盖。环境脚本由主会话管理，不直接复用过期 PID。

```sh
Q=.trellis/tasks/11-09-collaboration-business-scenario-qa/research/environment/qa_env.py
python3 "$Q" verify
python3 "$Q" sync
python3 "$Q" restart-api --knowledge off
python3 "$Q" run --provider host QA_RUN_ID=business-support-governance-new-id pnpm exec playwright test e2e/collaboration-business-scenarios.spec.ts --grep "S2 support ticket|S3 governance" --config playwright.qa.config.ts
python3 "$Q" restart-api --knowledge on
python3 "$Q" run --provider host QA_RUN_ID=business-outsourcing-new-id QA_KNOWLEDGE_CONTEXT=on pnpm exec playwright test e2e/collaboration-business-scenarios.spec.ts --grep "S1 outsourcing" --config playwright.qa.config.ts
```

收尾证据：[最终环境清理](environment/final-cleanup-20261110.json)确认 API/Web/renderer 三个端口均释放，6 个测试二进制哈希与初始分配一致，2 个新测试文件与执行快照一致；知识 off，执行与 host 闸门关闭，创建开启，数据库及私有日志保留。`allocation.json` 的 PID 为历史身份记录，不是当前运行状态。定向静态验证命令见[验证记录](static-validation-20261110.md)，独立复核与最终字节签名见[复核记录](final-evidence-review-20261110.md)。提交归档仍待单独收尾。

最终场景截图：[S1 甲方验收](b6-outsourcing-20261110/test-results/collaboration-business-sce-e8bed-nd-reuse-approved-knowledge-chromium/s1-phase-one-accepted-client.png)、[S2 返工后再次验收](b5-support-20261110/test-results/collaboration-business-sce-7a501-reopened-ticket-is-reworked-chromium/s2-reaccepted-support.png)、[S3 治理后完成](b3-support-governance-20261110/test-results/collaboration-business-sce-4837c--and-the-issue-still-closes-chromium/s3-final-lead.png)。
