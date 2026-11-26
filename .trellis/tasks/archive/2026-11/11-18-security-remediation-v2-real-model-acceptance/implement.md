> 当前执行路径（用户2026-11-20最新明确决定）：改用官方 `codex exec` 直接执行，不使用 app-server SDK 客户端。真实完整指令、平台身份和证据不变；此路径证据独立标为 official-cli-direct，旧SDK失败不抹去。不伪造UA/clientInfo，不探测替代模型通道；同账号同模型、已有不限累计预算授权。原文中“只允许app-server/拒绝exec桥”是被本次用户决定替代的旧方案；直连必须运行真正官方CLI，不能只改头部或名称。

> 当前授权覆盖（2026-11-20续）：用户明确不限制本任务真实模型调用次数与总运行时长，要求持续推进到成功。使用当前Codex配置 model=`api666-zzzcoding/gpt-6-astra` / provider=`magpie` / effort=`max` / account alias=`current-magpie-config`。以下旧有限预算/等待具体参数段落是历史，已由本段替代。每次调用仍记账；20分钟单次watchdog仅用于检测卡死，可诊断后重试，不是用户累计预算。真实入口仍须完成实际生产路径及隔离验证，不能凭本授权绕过。

# 执行计划：Codex 真实验收

> 当前执行检查点（2026-11-20 续做审计）：下文早期准备步骤和零调用状态保留为历史。当前已有官方 CLI P1/O1 真实通过样本、C1 装置失败与待分析师队列、D1 脚本；真实后续准入关闭。以本文末尾及 `research/resume-evidence-audit-20261120.md` 为当前待办，不重新询问已明确的调用、提交或清理授权。

## 已完成的无模型准备

- [x] 读取父任务 AC9/AC10、子任务一设计与平台能力、Codex home/sandbox、普通 daemon driver 和原生 app-server 源码。
- [x] 把“未授权占位”更新为“已授权 Codex，待具体执行参数”。
- [x] 四个合成仓库与四例结构化预期写入 `fixtures/`，16 个输入文件冻结 sha256。
- [x] `scripts/prepare.py` 提供无模型源校验、本地夹具测试、固定镜像离线容器探针；没有真实入口、没有账号读取、没有供应商发现。
- [x] 本地夹具验证：代码 2/2 过；依赖 2/3 过，唯一失败是预设的超长标签漏洞。此失败是预期基线，不是失败测试被忽略。
- [x] 离线容器预检：两例基线结果吻合，7 个角色目录/无网络探针通过（r3）；前两轮失败分别保留。
- [x] 独立复核 R2 补齐合成 scanner 与 D1 前后输出/间接依赖对照，16 个输入重新冻结；旧冻结保留。
- [x] 本地启动账本与网关控制实现，9/9 合成测试通过，完整 real 注册仍被准备版无条件拒绝。
- [x] 普通 daemon Claim seam 的 overlay-only Go probe 通过；真实入口无 opt-in 的跳过是预期守卫证据，不算模型验收通过。
- [ ] 实际运行拓扑（含网关、守护进程与角色交接）无模型全链路验证。
- [x] 独立规划评审有条件通过；见 `../11-18-security-remediation-v2-template/research/review-20261120.md`。`execution_admission=false`，仍须解决强制工具离线边界和真实装置前置。

## 步骤 1 集成与参数注册

1. 等父任务实际集成子任务一；记录集成目标 branch/commit、产品提交列表和 ancestor 检查，不能把 efdb597d6 变基或本地 commit 当已合入。
2. 一次性向用户补齐必要参数：精确模型（现配置候选 `api666-zzzcoding/gpt-6-astra`）、账号来源别名、最大物理启动数、最长总时长及单次超时。已有 Codex 授权不重问。预算参考路径约 31 次，不能由实现者自行授权。
3. 用任务私有目录创建注册与持久账本。内容见 design 第 2、7 节；账户真实 credential 留网关，不进注册摘要。
4. 先冻结原生 adapter/CLI/镜像/协议/指令/source hashes，再编译与记录真实入口文件/二进制。Go 真实入口需 agentintegration + `MULTICA_RUN_REAL_AGENT_SMOKE=1`，必须先验证注册再做 CLI lookup。

## 步骤 2 环境和夹具

```bash
python3 .trellis/tasks/11-18-security-remediation-v2-real-model-acceptance/scripts/prepare.py --local-tests
python3 .trellis/tasks/11-18-security-remediation-v2-real-model-acceptance/scripts/prepare.py --container-probe
```

脚本固定已有 Python 镜像 digest，`--pull=never`，每个子命令超时 20 秒；新探针容器有 task label 和唯一名，超时只清理自身。第一版在共享 Docker 异常时 60 秒超时，原始失败保留于 `research/preflight-raw.txt`，后续实现已加有限等待和精确清理。源校验不依赖 Docker。

实际部署前还必须：

1. 用明确归属的独立数据库，按最终迁移版本准备，校验 `current_database()`，不接触其他任务库；独立 API pool 限额和身份健康检查。
2. 每个角色独立容器/网络与私有 workspace，供应商网关只允许冻结端点，API 保留原始 task token；不共享 root HOME 或 Docker socket。
3. 按已验证的 native managed requirements + stdio MCP 路线装配真实 task-token CLI broker，最终角色镜像重新证明普通 shell 无网和平台通道 scope；无模型原生机制已补证，实际装配仍为独立复核 R3 的真实准入阻断。然后按 design 第 4 节实际运行全部正负探针，包括角色间访问、出站绕过、只读 reviewer、网关关闭后拒绝、未授权路径拒绝。
4. 从最终版本创建全新工作空间副本，核对小队/leader template_version=2、现行受限 reviewer 版本、7 个内置座位，C1 额外试点 verifier；配置所有人实际 invocation 权限与 runtime binding。
5. 把合成 repo 复制到实验私有目录并创建 Git 基线；不把期望表或评分工具放入角色容器。交接只转移成员实际产物，记录 patch/test 的 hash 和作者。
6. 使用任务私有根级 `project_doc_max_bytes` 覆盖最终指令和祖先文档总量，验证 task Codex home 复制值及完整 AGENTS。验证时不先启动真实模型；首次真实样本另留实际读取完整性证据。
7. 用测试自建假可执行文件跑真实装置的声明周期/预算/停止/角色路由，只计 scripted 证据；默认测试保留 agent-CLI guard。

## 步骤 3 复核和最终冻结

- 独立检查本目录方案、四例预期、预算守卫、全量平台 oracle、容器 mount/network 证据和两个版本 pin。
- 解决阻断后写 `research/planning-review.md`（由独立检查者产出），把真实入口与私有注册 sha256 写入 implement 记录。
- 重算 `fixtures/freeze.sha256.json` 仅限未开始真实调用之前；保留此次规划版冻结和后续变化原因。第一份真实注册记录所采用的 hash，不覆盖原试验。
- `task.py start` 只在启动条件实际齐全后执行。准备进度可写入 planning 任务，不假设完整真实环境已建好。

## 步骤 4 串行真实验收

1. 首先 P1，确认能遵守停止；再 O1 的无效回复/有效搁置；再 C1 的独立 verifier 闭环；最后 D1 依赖与迁移审查。
2. 每次前先原子预留一次额度并记录期限；启动 wrapper 和网关执行硬上限。任何不完整样本、证据落盘失败或边界突破都停止，不隐式重试。
3. 负责人由操作者以平台人类身份发预登记决定，仅在所需 reviewer 结果全部出现后放行；不替智能体写结果。
4. 每个步骤读实际 tasks/comments/receipts/activity/status，保存完整差异；供应商会话只保存在仓库外，结束后摘要+sha256。
5. 计数/时间耗尽立即停止该任务 owned 进程并断开网关，报告已得证据及未覆盖，不能重置账本续跑。

## 步骤 5 对照与交付

- 按 `expected-actions.json` 每项给真实观察与引用，判定 proved/disproved/incomplete/not-covered；若有缺失真实 terminal/usage，明确 unknown。
- 统计 verifier 工作量与越界，单次试点不给内置角色升格结论。
- 对照子任务一 design 第 11 节的 v1 失败项；契约、脚本和模型三列分别填写，完成父任务 AC10 集成复核。
- 指令行为需变更时停止本系列，回父任务新增模板版本；另立 registration/预算后再测。
- 整理脱敏证据和 hashes 后清理本任务容器、网关、凭据副本、数据库和私有工作目录；仅按所有权清单清理，留分支/提交和持久证据。

## 验证记录与当前阻点

| 项目 | 状态/证据 |
|---|---|
| 源冻结 | `fixtures/freeze.sha256.json`；16 个输入 sha256 |
| 本地无模型准备 | `research/preflight-local-20261120.json`，fixture validity 通过；OS 隔离未由此证明 |
| 离线容器初次 | `research/preflight-raw.txt`，Docker 超时；无供应商调用 |
| 离线容器最终 | `research/preflight-container-r3-20261120.json`，两例基线及 7 角色探针通过；不证明实际网关和 daemon 完整隔离 |
| scanner 控制 | `research/synthetic-scanner-before-20261120.json` 与 `synthetic-scanner-fixed-control-20261120.json`；临时 lock 修改验证 synthetic scanner，不是模型修复 |
| 本地预算/网关 | `research/runtime-controls-r4-20261120.log`，9/9 过；r1 socket 超时和 r2 定向恢复均保留；只证明本地请求/进程限额，不证明远端断线后不计费 |
| ordinary driver 准备 | `research/ordinary-driver-probe-20261120.log`，合成 HTTP Claim 1 PASS；真实入口无 opt-in 1 SKIP（守卫证据，非验收） |
| 真实账号/模型/次数/时长 | 已授权 Codex；具体参数尚待用户一次性补齐 |
| 实际合入 | 父任务协调，未填写可验证集成提交前保持未满足 |
| 真实启动/用量 | 0 次；token 0（没有启动，而不是缺失 usage 当 0） |
| 模型结论 | 四例与 verifier 全部未覆盖 |

真实入口完成实现与带网关的生产环境仍需步骤 1–3。准备版 `runtime_guard.py` 无条件拒绝 real 模式；tagged Go 入口有 opt-in 仍拒绝，防止未验证的布尔字段/哈希被当作合入或隔离证明。不能将本目录 no-model 准备脚本宣称成真实运行已就绪。

新增可复用控制的验证命令：

```bash
python3 -B -m unittest discover -s .trellis/tasks/11-18-security-remediation-v2-real-model-acceptance/scripts -p test_runtime_controls.py -v
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -overlay "$PWD/.trellis/tasks/11-18-security-remediation-v2-real-model-acceptance/research/ordinary-driver-overlay.json" -tags=agentintegration ./internal/daemon -run '^TestSecurityRemediation(OrdinaryDriverPreparation|RealModelEntry)$' -count=1 -v -timeout=2m
```

overlay 只把本任务文件虚拟挂到 daemon 包，不创建/修改产品文件。当前 overlay JSON 的绝对路径绑定本 worktree；迁移到其他检出时需重建路径并保留新哈希，不直接复用旧运行身份。

## 原生工具边界续做

- [x] no-model native app-server ordinary/network-widening/dangerFullAccess 对照；最终 managed network=false 阻止工具出站。
- [x] 同一 app-server 的 stdio MCP 调用可访问合成 API，原始合成身份保留；scope 外调用拒绝、API 调用数1、modelTurns0。
- [ ] 按本方案实现真实 Multica CLI task-token broker并接最终 production driver；这是已授权后续工作，不需要追加实施许可。
- [ ] 最终策略的完整工具/角色探针及独立复核，再解除准备版真实禁用。

记录：`research/native-boundary-findings-20261120.md`。本轮启动了本地 Codex无模型进程，不能写“没有调用任何 Codex CLI”；真实模型调用与供应商账号访问仍为0。


## 2026-11-20 实际合入更新

子任务一已实际fast-forward合入codex/cross-workspace-collaboration，集成提交50045c12cc6eb32a11e032b68717ec844640f4b8包含产品15bcd2e5e、0ffc3572c、aa592194b。前置1现在满足。准备规划及无模型机制经独立评审；仍需精确型号/账号/有限预算、真实task-token受管CLI通道（不得绕过受限reviewer外部MCP拒绝）、完整生产driver及最终隔离正负探针。真实调用授权无需再次请求。

下一次继续从主检出的本任务目录读取最新记录，按需要新建隔离验收worktree；本轮产品worktree移除后不再硬编码其路径为运行入口。overlay路径需按新checkout生成，旧research overlay只是历史证据。实际model turns/provider requests/account accesses仍0，本地Codex无模型进程另列。


本轮产品worktree已于完成全部提交和实际集成后移除；全部准备脚本、fixture、研究和控制输出已在主检出保存。真实验收的继续工作使用新隔离环境，当前task仍planning且execution_admission=false，原因是具体运行参数和生产受限身份通道尚未闭合，不是调用未授权。


## 2026-11-20 真实执行结果与外部权限阻塞

实际已执行3次P1平台尝试，前一次在原生启动前拒绝；后两次共2个原生进程、7个POST模型请求，6次官方客户端限定拒绝、1次并发限制。完成模型行为回合0，usageunknown。已正常保留原生客户端身份及协议头；不修改clientInfo伪装，不尝试其他模型路由绕过限制。模型目录读取1次仅证明可见，不能作为SDK授权。真实入口现关闭，待明确SDK支持配置。

P1fake05和O1fake03完整实际平台脚本验证通过；原生只读transport r12、6个会话安全控制、6个CLI通道控制、12个预算/网关控制通过；C1/D1候选4角色文件系统/Git权限探针与6个实际Gitbundle交接测试通过，完整C1/D1driver仍未实现。详见research各记录。没有生产源码/模板行为修改，不发生新模板版本。


## 官方CLI阶段最终检查点

P1与O1真实样本完成并独立语义通过；P1自动action-only检查过严原失败保留，按原冻结protocol-consistent要求评估，非修改要求刷通过。C1首轮原生completed但wrapper误读leader未挂载测试产生evidence_failed，合法productanalyst排队保留；D1仅脚本。后续实现代理被工具内容筛查拒绝，未绕过。已完成明确的本地wrapper维护，不代表C1续跑获准。

50任务控制与11backend契约通过，原生CLI隔离、完整平台脚本、实际模型用量和逐例语义报告均保存。无完整makecheck重跑：仅任务装置与记录，无生产TS/Go逻辑变更。真实任务仍in_progress，AC9/AC10未完成。检查点/清理见research/cli-checkpoint-cleanup-20261120.md。

## 2026-11-20 续做审计与剩余步骤

- [x] 读取实际 AGENTS/CLAUDE、Trellis 现有任务、相关规范，重新查询 HEAD；保留分支在新 `mc-secrem-v2-audit-20261120` worktree 处理，主检出其他会话改动不带入。
- [x] 9 个 dump 名称/大小/hash/完整离线解码通过；C1 原 failed leader、queued analyst、委派来源与原平台快照一致；D1 真实库无 task/comment/issue。没有恢复数据库。
- [x] 冻结 16 个输入与 P1 successor/O1/C1 已保存源码副本逐字节 hash 核对；首个 P1 原 backend 字节缺失的限制继续保留。
- [x] 找到原 `cli_harness` 工具拒绝：2026-11-20 06:21:37Z，possible cybersecurity risk，回执要求需更宽访问时申请 Daybreak。具体触发子步骤未披露；不凭会话更换、改写请求或其他执行通道重试。
- [x] 把当前状态与 SDK 历史计数分开，记录 v1 失败项部分验收对照，AC9/AC10 仍开放。
- [ ] 被拒 C1 续接取得适用的工具方正式访问或复核结果；本轮未取得。
- [ ] 在此前置之后完成合法可选分析分支支持及其独立验证，保留原失败与待办；未在本轮实现。
- [ ] 新环境恢复、当前源/配置绑定、无模型全链与角色隔离的独立准入，然后才恢复 C1 真实运行；D1 单独准入后执行。
- [ ] 全部真实案例与 verifier 工作量齐备后完成 AC9/AC10。需要修改模板则回父任务走新版本。

本轮独立历史审计见 `research/resume-p1-o1-audit-20261120.md`，已有维护控制验证见 `research/resume-maintenance-audit-20261120.md`，根会话 dump/源码/工具限制核验见 `research/resume-evidence-audit-20261120.md/json`。全部只证明所列审计或维护范围。没有新的真实调用、准入、生产源码或模板修改。未运行全仓 make check/lint/typecheck；最终记录检查与本地集成回执单独保留。

本轮审计已实际合入 `14057a80f`（并行基线先到689fcb2d8，零路径交集后变基）。50 Python、11 Go命名结果和范围内静态检查通过；8,798个其他脏路径在合入窗口内指纹不变。审计worktree及新建测试状态已清理，任务与分支保留；最终回执见 `research/resume-checkpoint-cleanup-20261120.md`。未完成项和工具访问前置不变。

## 2026-11-20 后续维护：临时测试目录归属

当前基线 `1861f2aab` 已包含上述集成提交。继续同一任务，在新建的
`mc-secrem-v2-maintenance-20261120T124811Z` worktree 处理；不复用旧运行环境。

已有审计指出两个控制测试仍向历史证据根创建临时目录。实际源码确认
`test_codex_exec_code_container.py` 与 `test_contained_git_role_handoff.py`
各有一个固定的历史根。这里只修正这两个测试的临时状态分配，使用调用者选定的
独立临时目录并规范化路径；保留测试场景、断言、不可变镜像和生产执行策略。
这项维护独立于被拒绝的 C1 分析师续接，不修改或执行续接实现，不生成新准入。

- [x] `trellis-implement` 修改上述两个测试文件；清理只作用于测试新建目录。
- [x] 在仓库外新建的私有 HOME/临时目录下执行原有50项控制测试，全部通过；命令、退出码和原始输出保留。50个测试方法AST不变，29个Python文件解析通过，16个冻结输入hash一致。未重跑历史模型样本或全仓测试。
- [x] `trellis-check` 独立核对差异、原断言保留、规范路径与清理；46份原件hash及16个冻结输入一致。历史目录顶层元数据前后一致，源码检查确认不再选择历史根；不将其扩大成全部历史字节或中间写入审计。
- [x] 主会话记录访问查询的实际结果、用量无增量、AC9/AC10仍开放；维护提交 `9e74d1c39` 已合入，13,243个其他脏路径在整合窗口内指纹不变。本轮worktree和测试状态已清理，证据保留；见 `research/maintenance-checkpoint-cleanup-20261120.md`。

工具方正式访问/复核尚无新增证据。禁止以本维护的通过、用户不限预算授权或新会话替代该前置。
