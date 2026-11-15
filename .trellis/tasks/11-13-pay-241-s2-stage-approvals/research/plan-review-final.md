# S2 最终规划复核

结论：**可开始服务端首增量实施**。`ready_for_implementation=true`，`blocking_findings_open=0`，无待决产品问题。客户端并行以服务端交付冻结的逐动作 wire 合同为前提；这项既定实施依赖不增加审批步骤。该结论不表示产品已经实现、政策可以启用或 S2 验收通过。

- 审查人：原生 `trellis-check`，`/root/s2_final_plan`，2026-11-13。
- 显式目标：`.trellis/tasks/11-13-pay-241-s2-stage-approvals`。hook 的 S3 路径不用于选择本次任务，未切换主会话 task 指针。
- 已读 AGENTS/CLAUDE、13项 check/implement context、PRD/design/implement、两轮旧评审及报告合同，并核对下述实际源码入口。未启动代理、产品进程、数据库、模型或 Git。
- 结论绑定本次修订后的规划 SHA-256，完整值见同目录 `plan-review-final.json`。旧两轮评审保持原样；其中“等待用户答复”是历史状态，不是新的阻挡。

## F01–F06 处置

| 项目 | 最终结论与实施依据 |
| --- | --- |
| F01 签字人代际与配置权 | 已解决。PRD确认source owner/admin且当前任务有权配置；design:19/80以受权配置确定实现责任人，提交者独立归因；决定绑定原签字人的participant/account/credential及配置代际、有效期，消费重查原签字人。移除再加入或凭据恢复不复活旧签字。原workflow锁前收集当前参与者、锁后重读集合，配置不能引入未参与者。 |
| F02 语义版本与CAS | 已解决。design:81分开已审需求/代码/构建身份、后续追加证据和每次操作CAS；同样覆盖普通child。正常状态/处理人变化不使内容批准自己失效；前序决定和精确submission保持独立引用，重开需要新lifecycle确认。 |
| F03 最终事务检查 | 已解决。design:58/82/147明确两条新完成分支、M1 off/observe/enforce及实际writer都要在commit前检查私有同事务stage guard和数据库后采样时间。guard验证允许的before→after转换，不能把合法完成推进的lifecycle误认为外部漂移。原M1授权及历史replay保持独立。 |
| F04 建议物化与child | 已解决。design:87固定proposal家族、current pending、唯一物化、1–20child与CreateInTx全原子。人工手建保留；机器拆解需可信内部准入。design:137明确首次启用/重开对原child显式确认、不重建或派发；:147列出实际创建/修改/enqueue/Start入口、持久来源映射和NOWAIT交叉锁约束。 |
| F05 测试与外部事实 | 已解决。独立mcb endpoint/CLI，服务器推导attempt；真实comment→command→first claim冻结subject及Git preparation；每run/attempt一份不可变run_event报告，原key最小ACK、typed日志、失败自动投影、QA真人选择均有合同和验证矩阵。GitHub要求当前head、完整非空检查、completed/success及30分钟内观察；VCS仓库身份不足时unsupported。S2仅实现外部事实内部登记/消费门禁，合成正例不等于S4真实连接器/支付/部署。 |
| F06 共享客户端与恢复 | 已解决到可实施程度。design:112明确本包补M1协作typed消费、GET-only stage恢复、严格匹配与安全activity/通知/realtime；:118按动作划分confirmation，项目政策与issue operation不混用。implement:34规定服务端先交付逐动作字段/路径/错误/到期边界，再启动客户端并行。尚未产生wire实现文件，不把此先后依赖写成已经完成。 |

第二轮 P01/P02/P03由用户两次明确答复及PRD收敛关闭；T01由采用的报告合同关闭；T02的动作变体、作用域、恢复状态和真实writer接线已补到设计，具体DTO/路由清单是首个实施产物。这里没有新增职业规则、通用工作流或外部执行器。

## Findings (fixed)

- File: `design.md`
  - Issue: 正文还保留“配置或提交者”“Finish必要时”“客户端若发现需扩张”“CI时效待定”，与已确认规则和后文冲突；统一confirmation也要求了部分动作根本没有的ID。
  - Fix: 统一到受权配置的实现责任人、必做的最终guard、明确M1客户端归属、30分钟completed/success；补child语义版本、合法完成转换、动作变体及实际入口表。项目策略沿用项目事务边界保留原key/digest，不伪造issue/candidate/decision或修改旧operation scope。
- File: `implement.md`
  - Issue: CLI仍写“是否需新增命令待协议决定”，未把最终报告合同与第二轮锁/入口要求落实到首增量交付。
  - Fix: 明确服务端owner负责报告CLI/protocol/runtime/帮助，先冻结wire后交客户端；把普通人工child控制组、child语义版本、root变化与enqueue/Start两种竞争顺序加入定向验证。
- File: `research/test-report-contract.md`
  - Issue: 头部仍说兼任/自评未答；验证要求CLI在HTTP前发现格式合法但与服务器冻结subject不同的绑定，超出当前run identity环境可知范围。
  - Fix: 同步已确认产品事实；将CLI本地格式/上下文检查与服务端实际subject等值检查分开，后者必须拒绝且零event。未放宽绑定要求，也未给CLI添加猜测候选或用户凭据回退。

## Findings (not fixed)

无遗留规划阻挡。以下是实施和验收依赖，不是未答产品问题，也不计为通过：

- 服务端先完成 `implement.md:34` 的逐动作wire清单（含期限、边界值、严格JSON、原key恢复），客户端随后接同一冻结版本。所有writer门禁和schema readiness完成前，不能开放政策启用。
- 实际产品尚未实现；需按A01–A12完成正常/竞争/回滚、strict schema、原key未知恢复、M1叠加及production Web/实际Desktop验证。既有service50/handler31只是基线，不折算S2新增通过。
- S4负责真实支付、仓库级CI、部署/回滚适配器与真实证据；无合格执行事实时生产停在等待。S2可用标明synthetic的受控生产数据形状证明内部消费合同，不将父矩阵整例改为PASS。
- `.trellis/spec/`本轮不写“已实现”合同；实现后同步实际阶段协议、runtime CLI/报告、完成事务及共享读取约束。保留其他任务与旧评审记录。

## Verification

- 规划静态检查：PASS。implement/check各13条真实路径，无缺失/重复；PRD含A01–A12；父矩阵仍为24个唯一`TC-01`至`TC-24`；修订文档代码围栏/UTF-8/结尾换行及过时措辞检查通过；JSON报告与当前SHA重新核对。
- 源码事实：核对`workflowInTx`/`workflowRoleAllowed`/用户集合、Prepare/Finish、`ChangeLifecycle`/`SubmitAcceptance`/R38、普通writer、`checkIssueExecution`/enqueue/Start、CreateComment→command、`claimPromptSnapshot`、run_event存储、既有GET恢复与ordinary-only Core schema。产品文件未修改。
- Lint: NOT_RUN（仅规划文档审查，没有产品实现）。
- TypeCheck: NOT_RUN（同上）。
- Tests: NOT_RUN（产品/DB/Provider测试）；仅执行文档、清单和哈希静态校验，不借用历史测试成绩。
- 审查脚本首次用`TC01`格式匹配父CSV失败；读取实际`case_id`后改为逐行核验`TC-01`至`TC-24`，数据未改变。少量不存在路径的只读检索已改用已确认路径；未据检索错误推断产品缺失。

改动限于上述三份规划文档及本次两份报告；没有新依赖、产品简化、数据库操作、Git操作或其他任务写入。下一步是主会话激活S2并交给原生实现owner。
