# 阶段一试点结果

状态：`phase1_real_results_routing_gate_open`（2026-10-27 重试后）。任务保持 `in_progress`，阶段一尚未验收，阶段二未启动。

## 当前结论

用户要求“再试一下”后，沿用原 daemon/runtime 和隔离容器完成了真实模型任务。短探测中 `mimo-v2.5-free`、`muse-spark-1.2-contributor-free` 返回 `UnknownError`，`opencode/nemotron-3.5-lightning-free` 返回 `READY`，随后作为 daemon fallback 完成案例。没有改写共享 agent 的模型、角色来源、自治等级、Access 或 skills。

四个父案例 `POLA-3`～`POLA-6`、八个成员子任务 `POLA-7`～`POLA-14` 最终均为 `in_review`。依赖和授权补丁在隔离实验室中通过六项测试，可移植回放也得到“原始基线四通过、两个预期失败 → 应用补丁后六项全通过”。供应链缺证案例保持 `hold/unknown`，没有变成风险已解决。

这些结果证明模型曾可执行、合成补丁有效，并有真实独立审查。最后的只读派活验证又遇到匿名模型限额，因此不能把前面的成功理解为服务持续可用。小队完整自主路由仍未通过：`POLA-4`、`POLA-5` 都由 leader 直接实现，`POLA-4` 后续三个角色报告实际也由 leader 代写。操作者随后手动创建独立成员子任务来补救；这个过程不能替代 leader 按角色派活、诊断与实现分离的验收。随后用试点 v2 指令证明了一次实际 leader → 安全审查员委派，但成员没有结果，派发 activity 也存在误记，不能宣称完整往返已通过。

## 环境、来源与清理

| 项目 | 实际值或证据范围 |
|---|---|
| 工作空间 | 北辰 `d6eaf4b9-07bc-4aa4-b3ee-5a4bc733949c` |
| 小队 | 安全漏洞修复小队（试运行）`ac1a40c2-a655-4453-b86d-745796a454bc` |
| 项目 | 安全修复试点 2026-10-27 `e9f1b898-709a-4aee-9b75-5d72cf55e950` |
| 原 daemon / runtime | `019fef71-c14f-757e-9ad2-a9750d306456` / `39443bc2-1bf7-4515-8bdd-4684a41d64c9` |
| 容器 | `multica-security-pilot-20261027`，无 host HOME、Docker socket 或卷挂载；重试结束后已停止 |
| 运行身份 | daemon 为 root；provider 为 `node` UID 1000。首次恢复曾验证 provider 不能读取 root 开发 CLI profile |
| CLI / 模型 | OpenCode `1.18.21`；本轮 fallback `opencode/nemotron-3.5-lightning-free`，本地 OpenCode 凭据数仍为 0 |
| 代码来源 | 复用从 `61ff581d2` 构建的 daemon。首次环境快照中的 API `/health` commit 为 `564e3c51c`，不是本轮 checkout 的 API 验证 |
| 实验室 | `/work/lab`，分支 `pilot`，已提交基线 `3c00fabd4415a6497a90fda491fd49129d758183`；当前三文件补丁未在实验室 Git 中提交 |
| 项目资源 | `local_directory` + `in_place` 绑定原 daemon；普通成员读取共享实验室，leader 初始任务使用独立协调 cwd 后访问绝对实验室路径 |
| 内容版本 | `roles-20261027.json` 保留模板版本、指令摘要；`skill-content-versions-20261027.json` 保留六个去重 skill 摘要 |

最终清理见 `evidence/routing-final-cleanup-20261027.json`：停止前原 daemon 四个工作空间 namespace 的 pending/active task 为 0，容器最终 `exited`，临时开发认证副本已删除。最后一次启动中新建且无 agent 绑定的猎户座 runtime `1fced76f-4b16-4962-a42a-e5b27860d4eb` 已通过普通无 cascade 删除接口清理，历史 runtime/角色绑定保留。`evidence/retry-cleanup-20261027.json` 保存的是之前案例执行结束后的清理，不是最终路由验证后的状态；其中删除的临时 runtime 为 `4a181074-c97c-474e-86cf-1e60616e257b`。

默认 Docker bridge 的出站访问未限制；独立文件系统和 role 指令不构成网络最小权限证明。角色共用容器内 UID，也没有角色间文件系统隔离。没有注入生产凭据或个人模型账号凭据；API/模型目的地约束仍是未完成门槛。

## 案例与真实交接

当前完整评论、activity、任务结果及 `source_task_id` 收在 `evidence/real-pilot-summary-20261027.json`，以下各项状态均为 `in_review`。

| 案例 | 实际结果 | 验收边界 |
|---|---|---|
| 生产升级演练 `POLA-3` | leader 成功提交人工介入和 incident/release 交接草案；同一 leader 补交 `exploitability: unknown`、`evidence_state: unknown`、`production_impact: no` | 原评论把模拟语境写进枚举值，已补正三个字段；这是无真实生产影响的桌面演练，没有实际派发生产小队或执行生产动作，不代表所有分诊字段均已完成统一校验 |
| 组件/依赖 `POLA-4` | leader 将虚构依赖 v1 升至 v2，适配 `.label → .text`；操作者手动派出迁移、QA、安全、代码审查子任务 | 补丁正确；初次实现和后续代写报告违反预期分工，不能记作自主路由通过 |
| 程序代码 `POLA-5` | leader 在审计副作用前加入 `workspace_id` 拒绝检查；操作者手动派出独立 QA、安全、代码审查子任务 | 六项测试全通过；诊断工程师和实现工程师本人的交接尚未演练 |
| 其他风险 `POLA-6` | leader 和独立安全审查员确认来源摘要、生产者证明、组件清单缺失，保留 `hold/unknown`，owner 为 `pilot-build-owner` | 完成缺证识别和补证交接；供应链安全仍未知，不能以测试通过关闭风险 |

独立成员子任务对应如下；每项均有真实对应 agent 的完成任务和评论，区别于父 `POLA-4` 中仅在正文标角色名的 leader 评论。

| 子任务 | 父案例 | 实际角色与结果 |
|---|---|---|
| `POLA-7` | `POLA-4` | `migration-reviewer`，`.label → .text` 兼容与回滚审查完成 |
| `POLA-8` | `POLA-4` | `qa-engineer`，当时依赖三项通过、授权两通过一失败；合计五通过一失败 |
| `POLA-9` | `POLA-4` | `security-reviewer`，依赖受影响性和供应链未闭合边界审查完成 |
| `POLA-10` | `POLA-4` | `code-reviewer`，代码与回滚审查完成；“无 diff”的原结论经精确命令复核后补正 |
| `POLA-11` | `POLA-5` | `qa-engineer`，最终六项测试通过 |
| `POLA-12` | `POLA-5` | `security-reviewer`，工作区拒绝与审计顺序复核完成 |
| `POLA-13` | `POLA-5` | `code-reviewer`，首次证书校验错误失败，单独重试后完成 |
| `POLA-14` | `POLA-6` | `security-reviewer`，确认 `hold/unknown`、缺失材料、禁止网络执行和补证验收信号 |

## 路由与证据质量发现

1. `POLA-4` 初始任务 `01a17b3b-c767-7d5d-9d5e-ec99b9846125` 直接由 leader 修改实验室。后续 `01a17b3f-9dac-7e38-a08d-775dca44f236` 写出“迁移/QA/安全和代码审查”三个报告，评论的 `author_id` 和 `source_task_id` 仍属于 leader；这些报告不计独立成员审查。独立子任务由操作者手动派发，不证明小队完成了自主派活。
2. `POLA-5` 的 leader 任务 `01a17b59-c293-78a2-8972-73922aa8078c` 同时诊断和实现。真实 QA/复审后来已补齐，但 `diagnostician`、`implementer` 本人的执行和交接仍未完成。修改后的 `workspace_reader.py` 只新增两行拒绝检查；部分报告误述新增行数或修复前审计行为，需以实际 diff 和测试输出为准。
3. `POLA-3` 首条交接用 `simulated`、`insufficient - synthetic drill` 代替枚举。纠正评论 `01a17b39-dc15-7c15-99a5-e7ff9f841f05` 来自原 leader agent 的普通回复任务 `01a17b39-5e21-734b-b929-de16cb671120`；该任务没有 `is_leader_task=true`，不能当作小队 leader 协调回合。模拟场景与真实状态必须分别记载；本次真实 `production_impact` 为 `no`，模拟设定不构成现实可利用证据。其他案例的统一分诊字段仍需逐项检查。
4. `POLA-10` 首次代码审查称“无未提交的修改、所有 hash 符合基线”，与实际 diff 矛盾。操作者要求 `git -c safe.directory=/work/lab status --short` 后，同一 reviewer 的任务 `01a17b4d-ba37-7b33-a02c-2d369e7b474e` 补交了实际变更结论。`POLA-7` 的“所有基线 hash 相符”仍不准确：至少 lock 与 application 已改变。
5. `POLA-8` 的总数“五通过一失败”正确，但把授权子集写为“四/五通过”；实际授权模块仅三项，当时是两通过一失败。`POLA-9` 将版本锁定描述成防止未授权变更的保证也过度推断；锁文件选定版本不等于写入权限控制。独立身份本身不能保证报告内容正确，这些误述仍须补正。
6. `POLA-13` 首次任务 `01a17b63-95f7-7ec5-81b6-dc2e5952f6f7` 返回 `unknown certificate verification error`、结果为空；任务 `01a17b66-85ec-71e4-add3-257fbe9d28f0` 重试完成。失败记录保留，不计作审查结论，也不通过关闭证书校验规避。

## 最后一次真实派活验证（2026-10-27 12:05～12:10）

仅试运行小队的指令通过 `expected_instructions` 与 `expected_updated_at` 成对前置条件更新为 `runtime/pilot-instructions-v2.md`；原始内容及更新快照见 `evidence/pilot-instructions-before-retry-routing.json`、`evidence/pilot-instructions-routing-update.json`。没有修改产品代码、共享角色指令、8 人 roster、自治或 Access。

研究 `research/retry-routing-20261027.md` 区分了两条触发路径：操作者只回复 leader 旧评论会激活普通 agent 回合；显式的 `[@小队名](mention://squad/<UUID>)` 才走小队 leader 路径。真正的 leader 必须自行发出 `[@成员名](mention://agent/<UUID>)` 评论，不能用正文角色名或操作者直接 @ 成员代替。

`evidence/retry-routing-validation-20261027.json` 保留了以下真实结果：

1. 第一条显式小队触发创建了 leader 任务 `01a17b88-e954-7c44-9942-3e59edd330b9`。其实际 `AGENTS.md` 投影含 Squad Operating Protocol、roster 精确链接、leader 角色及 v2 指令标记，见 `evidence/routing-briefing-projection-20261027.json`。这只证明该新回合收到内容，不能追溯证明早期实现/代写回合收到同样 briefing。该 leader 生成的链接缺少 `[]`，没有产生成员任务。
2. 明确纠正 Markdown 格式后，真正的 leader 任务 `01a17b8a-8931-7032-9a6e-e64761e9432d` 发布了有效链接 `[安全审查员](mention://agent/aae540c4-6bbe-42b6-a362-c62e2e434318)`，触发评论为 `01a17b8a-bf22-7003-936c-b9b2dfcca12e`。
3. 独立安全审查员的真实任务 `01a17b8a-bf48-71f4-8b3e-18e190fe89bb` 精确关联该评论，`attribution.source=delegation`、`precise=true`，`delegated_from_task_id` 指向上述 leader。leader 在 12:07:54 结束，成员在同一秒开始。这证明成员由 leader 委派且单并发槽位已释放，区别于前面的操作者直接派发子任务。
4. 本次成功派发时，leader 却记录了 `no_action`，与应记 `action` 的指令不符。活动记录不能被描述成完全正确，也不能据此认定整轮协调闭合。
5. 成员首次模型调用日志返回 `Free usage limit reached`，随后于 12:09:35 显式取消，`result=null`。没有成员交付，也没有成员结果唤醒后 leader 收口的证明。前面的 `POLA-14` 成功审查是独立历史子任务，不可移作此次委派的结果。

本轮实验室补丁保持不变。当前已证实一段真实派发链，未证实成员返回与 leader settlement；模型额度、错误 activity、诊断/实现分离和网络门槛继续开放，阶段一不通过。

## 补丁验证与范围

- `evidence/lab-diff-20261027.patch` 只修改三个合成文件：`application.py` 的 `.label → .text`、依赖锁定与本地源摘要切换至 v2、`workspace_reader.py` 在 `audit.append()` 前拒绝跨工作区读取。仓库 `fixtures/` 继续保留原始故意失败基线。
- `evidence/lab-final-verification-20261027.txt` 记录共享实验室实际状态、基线 ref、结构校验通过和六项测试全通过；供应链结论仍为 `hold/unknown`。
- `evidence/portable-replay-20261027.json` 从仓库原基线复制临时目录，先验证六项中的两个预期失败，再应用保存的真实补丁得到六项通过。基线摘要比对仅三个目标文件改变；测试、vendor、供应链材料未改。此回放证明合成补丁能复现，不是小队路由或生产漏洞修复证明。
- `validate_fixture.py` 的“local source hashes match”只证明它检查的本地依赖字节/结构一致，不能解读成所有基线文件未变、生产者可信或供应链已闭合。
- 本轮验证覆盖任务 JSON/JSONL、证据一致性、文档格式、Python AST、shell 语法及基线摘要；`go test ./internal/util -run 'Test(ParseMentions|HasMentionAll)$' -count=1` 通过，核对现有 mention 解析契约。未修改产品代码，没有运行全仓 TS/Go lint、typecheck 或 E2E。首次恢复阶段的 service 注册表窄测是历史验证，本轮未重复运行。

## 首次恢复与模型阻塞历史（2026-10-27 00:33～00:44）

首次恢复时，`POLA-3` 的三个任务均失败：`01a1790f-907a-7e0d-9510-c3fc31880671` 因 root 创建的 `0700` task 临时目录导致 provider UID 1000 写入失败；包装器所有权修复后，`01a17914-2ec8-7c0a-915b-2b4de2013179` 和一次自动 retry `01a17916-ff9c-793f-9a39-e9b0d4b9c847` 遇到匿名模型额度限制并在三分钟上限后失败。无工具 `hy3-free` 探测也曾返回 `UnknownError`。这一阶段没有成功角色结果，不能把当时系统错误评论当作审查结论。

历史记录保留在 `evidence/production-escalation-*.json`、`evidence/model-errors-20261027.json`、`evidence/cleanup-20261027.json` 与 `evidence/runtime-after-stop-20261027.json`。本次成功重试使用新的 `real-pilot-summary-20261027.json`、`retry-model-probes-20261027.json`、`retry-cleanup-20261027.json`，不覆盖旧失败记录。

更早的普通跨 runtime agent copy 曾丢失 `autonomy_level` 和 `template_key`，副本已移除并归档；本次继续沿用用户选择的“恢复旧 runtime”，没有重绑共享 agent。上轮两项创建 handler 测试曾报 `runtime not found in this workspace`，它们使用独立 DB fixture，不能与真实桌面 daemon 是否在线互相印证；本次没有改测试掩盖这个历史失败。

## 下一步与阶段二门槛

1. 由试点操作者恢复可用模型额度，继续验证 v2 指令下成员返回与 leader 收口，并纠正派发记录为 `action`。一次真实委派已留证，但取消且无结果的成员不能被算作交付；显式小队 mention 与普通 thread reply 的角色差异按研究记录执行。旧回合的 briefing 接收情况仍未知，不追溯补造根因。
2. 在现有失败基线的隔离副本上演练 `diagnostician → implementer → QA`，确保各成员自己的 task/评论可追溯；保留当前补丁和旧误分流记录。
3. 由原报告 owner 更正未解决的摘要、测试计数、权限含义与分诊枚举误述；操作者回读原始命令和最终文件验证更正。
4. 由运行时操作者落实并验证 API/模型出站目的地约束。`pilot-build-owner` 继续补供应链来源材料，缺证时保持 `hold/unknown`。
5. 只有上述缺口和四例完整验收满足后，才另行启动阶段二。当前没有新增内置模板、角色、数据库、产品依赖或生产执行通道。
