# 本轮覆盖与证据独立对账

## 最终快照24与AC结论

验收工作可以完成，**产品验收FAIL，不建议发布**。当前所有必需已授权本地原始身份均已尝试，没有新增待执行必需项；失败链路的未执行下游步骤及平台BLOCKED保持明确。Parent负责在原任务路径标记completed；不archive、不commit，避免破坏绝对run-context与历史证据绑定。

最终矩阵在 `coverage-audit/snapshot-24/coverage-matrix.{json,md}`，完整AC在 `snapshot-24/final-ac-review.{json,md}`。默认538仍400PASS/138SKIP，原138最新117PASS/21FAIL/0NOT_RUN，全失败历史114PASS/24FAIL；合并原538最新517PASS/21FAIL。T24现在PASS，仅指性能阈值与**当前最终清理**；CLI03历史fallback cleanupFAIL单独保留，不回写为成功。

最后globalreceipt为PASS_RESOURCE_CLEANUP_ONLY：stop exit0、stop前otherclients0、runtimePG与backendPG/Redis均停止，3个container/data volumes保留；182历史service ports关闭、0ownedprocess/lock/extra campaigncontainer，原9protocol注册与Applications默认URL一致。两套runtime的物理source/tool/assets/workspace bindings经独立只读复核仍匹配32724tracked文件，0generateddiff；未连接数据库或重跑产品测试。两次历史audit exit1继续保留。

AC01–AC05满足本次验收工作的计划/来源/处置/矩阵/证据交付标准；AC02明确保留原CLI清理失败。AC06满足FAIL报告交付，**绿色通过的条件不满足**。最终report/defects已保留CLI/Linux、UNKNOWN根因和L10限制；parent只需引用snapshot24最终T24状态完成交付。所有25领域/11lane状态列于final-ac-review.md，历史快照不覆盖。Reviewer未改task状态/parent报告/产品文件。

## 历史快照23（以下原文保留，非最终结论）


## 当前快照23：所有原始browser身份已执行（2026-11-25T08:39:47.632036+00:00）

默认538保持400PASS/138SKIP。原138最新117PASS/21FAIL/0NOT_RUN；全失败历史口径114PASS/24FAIL。Linux03wrapper准入失败与Linux04实际Main登录后45秒旧Collaboration标题失败分别保留，Linux04后续控制/历史链路未运行。CLI03仍240005ms timeoutFAIL及fallback卷删除cleanupFAIL。

本轮新增Linux外层closure审计，将7个绝对路径最终hash（含nested与private registry）和真实清理绑定到原身份；17个离线回归PASS、ASTPASS。独立解码原始boot/timeline，实际Linuxarm64packagedMain、file renderer、当前ASAR/executable/CLI哈希成立，init完成、login标题失败、close.application完成；协议前后原9个registration与Applications默认URL完全一致。见snapshot23/linux04-independent-audit.json。

以下是全部25领域/11lane最新状态；T24/AC02仍待最后globalreceipt，不把局部closure或旧PG0client当最终stop。

| ID | 范围 | 状态 |
| --- | --- | --- |
| T01 | 启动、配置、迁移 | PASS |
| T02 | 认证与账号 | PASS |
| T03 | 工作区、组织、成员 | PASS |
| T04 | 任务生命周期 | PASS |
| T05 | 项目与小队默认 | PASS |
| T06 | 迭代 | PASS |
| T07 | 筛选、搜索、分页、导航 | PASS |
| T08 | 评论、附件、引用 | FAIL |
| T09 | Inbox、Triage | FAIL |
| T10 | Agent、角色、小队 | PASS |
| T11 | 协作创建与信息 | FAIL |
| T12 | 协作审批与交付 | FAIL |
| T13 | 协作执行与控制 | FAIL |
| T14 | 知识全生命周期 | FAIL |
| T15 | 全局权限与隔离 | FAIL |
| T16 | 实时与一致性 | FAIL |
| T17 | 设置与资源 | FAIL |
| T18 | 管理、安装与观测 | FAIL |
| T19 | 桌面原生与持久化 | FAIL |
| T20 | 移动端 | BLOCKED |
| T21 | CLI、daemon、安装/升级 | FAIL |
| T22 | 多语言、主题、可访问性 | FAIL |
| T23 | 文档、插件、自托管 | BLOCKED |
| T24 | 性能、规模、清理 | NOT_RUN |
| T25 | Chat 会话与消息 | PASS |
| L0 | 静态、TS、Go、脚本、UTC | FAIL |
| L1 | 默认 legacy 与补充 | FAIL |
| L2 | password | FAIL |
| L3 | admin/managed | FAIL |
| L4 | device auth | PASS |
| L5 | namespace/control | FAIL |
| L6 | host/knowledge/output | FAIL |
| L7 | packaged/installed Main | FAIL |
| L8 | Firefox/WebKit | BLOCKED |
| L9 | Mobile | BLOCKED |
| L10 | 100-user performance | PASS |

Parent更新后的report/defects已复核：count、CLI/Linux/Stage、failureuncertainty及L10限制准确，陈旧队列文本已移除。尚待globalresourceparagraph、最终snapshot/T24及parentchecklists状态。见snapshot23/updated-report-review.json。任务状态未改，产品结论FAIL；没有进一步测试计划。

## 历史快照22（以下原文保留，非当前结果）

## 当前独立快照22（2026-11-25T08:03:41.680925+00:00）

本节取代下方历史快照21/20/17的“当前”结论。默认538唯一身份（400PASS/138SKIP）完整不变。138个原SKIP最近完整执行为 **117 PASS /20 FAIL /1 NOT_RUN**；保留所有历史失败口径为114PASS /23FAIL /1NOT_RUN。两种口径差异来自snooze、informationDesktop和Web delayed-read的后来完整PASS，原失败不删除。

完整逐身份和T01–T25/L0–L10矩阵位于 `coverage-audit/snapshot-22/`。此时唯一没有完整执行的原身份是 `collaboration-installed-main.spec.ts` 的Linux full Main；CLI03是已有FAIL身份的新重放，尚不提升。Linux03的准备独立review已接受准确digest `f920df29b833a1a7c73f507cdf5b6c5ab6b4184ba53f6871a04cc614c7337c1f`，完整Main尚未在此快照完成。

### Runtime最后队列与adapter独立对账

`runtime/continuation-final-ledger-05.json`与11个lane的collection/raw report逐身份、名称、terminal、retry0、expected count、exit、completion SHA全部匹配：**25次执行、24唯一身份、12PASS/13FAIL**。这不是新增25个原分母：Web delayed-read先FAIL后PASS出现两次。原plan05十lane23次，加delayed二例，共25。

- business-scenarios02为完整3PASS，取代当前覆盖中的未准入状态；business01的raw3PASS/无command exit仍永久保留INTERRUPTED，不回填。
- information-desktop02为完整1PASS，原合并注册预算失败保留。
- delayed02为Web1PASS/Desktop1FAIL。Desktop已通过participant-last-basis子场景，source-only membership loss在六个匹配拒绝文本的strict locator失败，最后source-loss-participation-survives子场景NOT_RUN；不能从前一子场景推断整个身份PASS。
- memory-runtime与order-export两个Host prelaunch/permit失败根因仍UNKNOWN，Provider未启动；不把status0解释为收到503，不把pool pressure直接当根因。
- cross-client/lane-close/lifecycle/offline-notifications共11个身份为2PASS/9FAIL。9个失败在namespace readiness/admission边界，实际底层probe原因未记录；与独立backend默认策略EPERM不是同一证明。

本次**所有四个namespace adapter**均另行复核，未只信runtime完成标记：独立review/input digest、当前runner/daemon/provider/permit/launcher哈希，private registry中的精确name/ID/image/owner、create/copy/start/rm事件、anonymous volumes，以及process/container cleanup。实际创建数分别1/4/2/2，全旅程预计2/8/4/4；少创建是中途FAIL的后果，不能升级全链路通过。所有9个实际container与9个anonymous volume均有原清理回执且通过当前成功docker inventory证明不存在；无fallback termination/removal，process remaining为空。

三个between-lane PG回执核对为other/active clients0、无active lane locks。最终ledger仍明示PG running=true以供CLI/Linux，不能当最终shutdown。delayed外层adapter的inputs/review及所绑定runtime completion也匹配。详见 `snapshot-22/plan05-independent-audit.json`；该新独立回执明确绑定历史adapter当前字节，未伪造原completion未记录的历史SHA。

### 失败与完成语义

20个最近失败身份继续分为：资源发布1、manual-detail2、macOS packagedMain1、installedCLI1、PAYcreation2、snooze1、Host prelaunch2、namespace readiness9、Desktop delayed locator1。产品评论机器principal越权另有真实写入证据；原Go race NOWAIT与扫描问题另列。不得把20个身份说成20处产品漏洞。完整列表在 `remaining-local-gaps.json`。

Linux owning-layer仍为原9中的2PASS/7BLOCKED，以及另1root子用例PASS；不重复父terminal、不推断race。不重复执行已验证前端lint/typecheck；审计代码未改，16个离线回归证据保持适用，新的身份/计数/hash核对通过，tracked diff为空。

任务可以在全部必需已授权本地尝试有结果、不可运行范围有明确BLOCKED理由、全局来源/资源终审和报告交付完成后，以**产品FAIL**结论完成“验收工作”。本review不改task状态，也不把失败测试勾成PASS。已生成 `snapshot-22/final-report-handoff.json`，逐节指出report、implement、continuation、PRD与task状态的陈旧信息及最后需要更新的条件；这些parent文件未被本review改写。

当前仍等待CLI03、LinuxMain03及全局source/tool/asset/workspace-edge与全部owned资源终审。其后需新label最终快照，不能复写snapshot22或把本次局部cleanup视为终审。

## 历史快照21（以下原文保留，非当前结果）


## 当前独立快照21（2026-11-25T07:22:53.264410+00:00）

本节取代下方历史快照20/17的当前结论。默认分母仍为538唯一身份、400PASS和138SKIP。原138跳过的最近完整执行现为 **110 PASS / 12 FAIL / 16 NOT_RUN**；保留所有历史失败的口径为109PASS /13FAIL /16NOT_RUN。逐项证据、T01–T25/L0–L10矩阵和remaining-local-gaps均位于 `coverage-audit/snapshot-21/`。

本次新完成：recovery-extras-five为4PASS/1FAIL，memory-runtime为1FAIL，order-export为1FAIL，order-export-leave为1PASS。新增的三个FAIL不是三个已证明的产品缺陷：Web delayed-read在共享PausedCommentResponse准入处缺少 `PAY241_S3_RESPONSE_STAGE_HOLD_ACCEPTED=1`，需要parent接受native-response02后单独准入重放；memory-runtime与order-export都在原30秒首个Provider visit等待中观察0次，尚未判定产品/环境根因。memory同期permit503与pool pressure仅是观测，不直接归因。保留原deadline和全部原始失败。

business-scenarios01仍为INTERRUPTED，raw3PASS未获original exit/completion，不计PASS。cross-client01读取时正在执行、没有completion，同样不提前升级覆盖。

### Linux owning-layer结果已独立核实

`coverage-audit/linux-go-evidence-20261125.json`重读实际Go JSON和每次命令退出码，核对完整准确名称及一次run/terminal、原stdout/stderr摘要、准入digest与最终文件字节：5288项source、2064项dependency及18项tool/binary/runner均匹配。

- 原Linux9身份：**2PASS /7BLOCKED**。两个通过是Codex signaled/reaped retry与Pi stdin pipe，均为Linux normal模式的原测试。
- 另一个原root-only skip子用例通过。它有父+子两个PASS terminal，但只关闭 **1个原子用例**，不能另算两项行为。
- UID1000和UID0的namespace prerequisite均exit2、`operation not permitted`；有效capabilities0、NoNewPrivs1、Seccomp2。七个原namespace test body未执行，属于**本次已准入default-policy配置BLOCKED**，不是产品FAIL，也不证明所有Docker配置均不可用。没有privileged/unconfined重试，不应自行放宽策略。
- 专项没有DB或外部网络；已核对owned-label容器清理命令exit0及空原始输出。原normal/race SKIP仍保留，normal补测不能代表race通过。

T13/T21的Linux缺口已从可准备NOT_RUN改为上述有证据的配置BLOCKED，L0保留详细计数。Windows/pg_bigm等其他边界不受本次通过影响。

### Linux Main独立review

`linux-main-acceptance/independent-review-linux-main02-20261125.json` 对准确inputs digest `3fea09eb8aff9acc2661c6ccde98ffd7e4e6b4678555893a1bee4987e922b0ab` 给出BLOCKED：整checkout只读mount仍暴露 `.omx/private` 与 `.env`；实际Docker返回小写 `error: no such object`，三处大小写敏感检查拒绝真实不存在结果。七个offline检查、28个input绑定、80个package条目和独立ASAR的47855 packed/2unpacked/497产品成员均通过，但不能关闭这两个准入问题。Parent已派原负责人修复，等待linux03新输入重新独立review；本审计未改implementer文件或启动Main。

所有产品tracked字节未修改。审计代码相对snapshot20没有改变，保留原16个offline回归PASS证据；本轮另验证矩阵身份和Linux raw/hash。任务仍in_progress，最终资源/来源closure与最后覆盖快照等待执行者完成。

## 历史快照20（以下原文保留，非当前结果）


## 当前独立快照20（2026-11-25T07:06:07.720260+00:00）

本节取代下方历史快照17的“当前”结论；历史文字和所有旧证据原样保留。基线仍为 `103e2bc329a9fb845d9f45c20b2a9bb3ebb54625`，本次没有 tracked 修改、产品修改、服务启动或执行准入签发。任务保持 in_progress，最终矩阵仍须在各执行者结束后用新标签刷新。

`coverage-audit/snapshot-20/` 已逐项重读原始 collection/report、身份和名称、一次执行/零重试、来源指纹、completion 绑定 SHA 与监听端口/gate 收尾。默认 **538 唯一身份 = 400 PASS + 138 SKIP** 不变。138个原跳过身份的最近完整执行为 **105 PASS / 9 FAIL / 24 NOT_RUN**；保留任何历史失败的保守口径为 **104 PASS / 10 FAIL / 24 NOT_RUN**。这两种口径不混加，不以成功片段拼出整套通过。完整逐身份与 T01–T25、L0–L10 映射在 `original-skip-dispositions.json` 和 `coverage-matrix.{json,md}`，待执行清单在 `remaining-local-gaps.json`。

相对snapshot17，新证据关闭11个原身份：stage-approvals1、runtime-output2、runtime-recovery4、knowledge-deployed1、stage-execution3。knowledge-deployed的on/off是**同一个原身份的两种配置**，不计成两个分母。Stage02的三例均为实际PASS、exit0；独立核对 `runtime/stage-02` 与 `stage-acceptance/stage-02` 的completion、原始命名结果和adapter清理，没有源/摘要/清理不一致。

本轮发现并修正审计器的两个本地问题：

1. T14的知识开关缺口原来从不读取已完成的on/off证据。现在解析hash绑定原始Playwright中的 `deployed-knowledge-gate-evidence`，核对实际DB/API/health、已批准知识内容、真实Host输入、三类事件计数（on各1、off各0）、单次launch、sealed ledger、host_exit_observed、provider reaped、daemon退出，以及三个源文件和daemon/provider/CLI实际SHA。两种配置均成立才关闭该配置缺口；单个PASS或标签命名不能替代它。见 `knowledge-variants-audit.json`。T14仍有业务场景/共享memory运行尚未完成。
2. 无completion的原始报告原来只显示“没有执行证据”。现在把完整原始身份/terminal及SHA留在 `unaccepted_raw_inventory` 和逐身份 `unaccepted_attempts`，与可计入的observations分开。`business-scenarios-01`状态是 **INTERRUPTED**，虽raw3PASS，但原command exit和completion缺失，三个身份仍是NOT_RUN。`slot-handoff-native-20261125-0645.json` 绑定的before/after及全部原始文件SHA重新匹配，after记录API/Web/renderer端口关闭、PG其余客户端0。恢复清理不能追认原command exit；需新label完整重放。

原审计源码/测试/map及本说明的修改前副本在 `revision-19-before/`；修改计划在 `revision-19-plan.json`。保留snapshot18/19，未覆盖任何旧snapshot；快照19对应审计器原字节已按其记录SHA还原保存于revision-20-before。

### 给执行协调者的剩余本地清单

| 原始场景 | 未获完整执行身份数 | 下一步 |
| --- | ---: | --- |
| business-scenarios | 3 | 新allocation/label完整三例重放；保留中断证据 |
| cross-client execution | 1 | 已审Linux namespace adapter，完成准确身份及容器清理 |
| lane-close controls | 6 | Web/Electron全部六例，不拼接通过片段 |
| lifecycle controls | 2 | 两客户端完整移除/重连/原proof路径 |
| offline-result notifications | 2 | 两客户端真实登录、通知来源和去重 |
| memory-runtime | 1 | 两个native runtime profile与实际run-local CLI |
| order-export / order-export-leave | 2 | 各一条完整执行与离开/移除路径 |
| recovery-extras | 6 | 先五例；Desktop delayed protected-read须native-response02的实际准入证据 |
| installed Linux full Main | 1 | 当前包/ASAR/CLI、Docker child接口和独立review后完整Main |
| **总计** | **24** | 不含下列9个已失败身份的修复重放 |

九个最近失败身份仍为：resource publishing1、manual-detail2、packaged host-installed1、information-recovery Desktop1、installed-cli1、PAYcreation2、snooze跨工作区mention1。已观察原因分别为旧导航/标题/工作区菜单selector、注册速率预算或copied relay执行前置失败；不能归纳为九处已证明的产品缺陷。应由原负责人提交最小private fixture/environment修正并独立review，保持完整断言与原时限，新label重放。installed-cli清理后已没有原copied文件UID/mode/mountflags，不能用host0700猜测此前根因。

**已证明的产品问题**仍是普通机器凭据借owner/admin角色修改他人人类评论。原完整Go race的合法M1 setup发生NOWAIT表锁冲突，其竞争持锁者/根因尚未确定；14项govulncheck可达发现是扫描证据，不是线上利用复现。公共skills GitHub HTTP403是外部服务失败，没有响应头证据证明一定限流。这些分类与browser fixture failures分别保留。

### 默认138以外：Linux Go9与root-only

新增 `coverage-audit/linux-go-feasibility-20261125.json` 记录九个**确切Go身份、所属包和源SHA**。它是只读准备建议，**不是执行准入，也没有测试PASS**。已读原normal命名JSON、源码与既有Linux image前置回执，并只执行 `docker image inspect` 确认cached digest为linux/arm64、默认root；未启动容器。

- `pkg/agent` 两例（Codex signaled/reaped retry、Pi stdin pipe）可在普通Linux容器执行，只需 `/proc`、`/bin/sh`、cat/sleep/stat和原测试自行生成的fake executable。镜像内具体utility仍需执行前探测。
- `cmd/server` 六例与 `internal/daemon` poller一例可在本机准备，仍需真实nested user/PID/mount/IPC/UTS namespace、UID/GID maps、mount/pivot_root及静态测试二进制。已有browser adapter使用privileged与seccomp/AppArmor unconfined只是配置先例，不能推断当前namespace admission已通过或自行放宽容器配置。先独立审查，保留 `MULTICA_REQUIRE_COLLAB_NAMESPACE=1` 与route opt-in，使前置不满足FAIL而非SKIP。
- 安装的固定Go可先用 `GOOS=linux GOARCH=arm64 CGO_ENABLED=0 GOPROXY=off GOTOOLCHAIN=local` 编译三个包测试二进制及CLI；实际缓存缺失应停止并报告，不下载新依赖。仅router TestMain需要新owned migrated DB、private JWT及验证过的数据库身份；另两个包的所选场景是本地fixture。服务时段由parent协调。
- `TestClaudeRootSudoPreflight/root_bypass_without_sandbox_errors` 是纯preflight函数，可以单独在disposable container UID0执行，无需host sudo或真实Claude。旧“本机非root所以不可执行”不能作为本地验收最终封口。
- 静态namespace测试是normal模式；`-race`依赖CGO，不能从静态通过推断race通过。Linux build-tag-only新增身份也不在原九个skip分母中，若宣称完整Linux包验收需另收集清单。
- 两个Windows身份仍需要Windows kernel；pg_bigm镜像缺扩展仍是依赖缺口。browser/CLI成功绝不关闭Go身份。

T13、T21和L0矩阵已明确携带这项可准备但未执行的Go缺口，避免单凭macOS平台解释或browser通过提前封口。

### 本轮验证与范围

16个离线回归全部通过：原8例，加上缺失/损坏/非object嵌入附件拒绝，及中断raw PASS拒绝准入、on/off精确内容/事件、错误DB、未reap、非boolean标志和缺单侧证据拒绝。Python AST通过；现有环境没有ruff/mypy/pyright，因此没有声称新Python lint/typecheck通过。原产品non-mobile lint/typecheck及mobile两项static执行的exit0和原始log SHA已重新核对，未重复运行产品检查。见 `snapshot-20/verification.json` 与 `offline-regressions.log`。

目前四个历史不完整记录保留：comment-authority-legacy01、legacy-broad01、native-response01、stage01；新完整重放不删除它们。各lane的清理回执不等于campaign最终live shutdown，T24继续NOT_RUN；其他平台/浏览器/签名/updater、Helm/PowerShell等缺口保持独立。最终快照仍等待parent在剩余runtime/native/CLI及Linux Go检查结束后要求刷新。

本快照读取时 `runtime-supplement/recovery-extras-five-01` 已开始但尚无completion，维持未准入；不会因执行中或中间terminal升级覆盖。

## 历史快照17说明（以下原文保留，非当前结果）


基线：`103e2bc329a9fb845d9f45c20b2a9bb3ebb54625`。本记录使用同一 campaign 的实际原始回执，不继承历史候选的通过数。当前仍有补测在执行。

## 对账方法与当前结果

`artifacts/qa/main-post-merge-e2e-20261124-080504/coverage-audit/audit_coverage.py` 独立解析每个 lane 的原始 `collection.json` 与 `playwright.json`，核对 file/title/project、完整身份集合、一次执行/零重试及终端。另核对 completion 绑定的实际文件 SHA、候选与源码指纹、原/终 gate booleans 和监听端口关闭回执，再与原 `named-inventory.json` 比较。

默认 L1 的 **538 个唯一身份、400 PASS、138 SKIP** 完整成立，138 个原始跳过分布于 68 个文件，没有丢失、重复或未知身份。每一项都已落入 `original-skip-dispositions.json`，包含原始理由、补测模式、所有实际尝试、当前证据和缺失前置条件。原始 138 个 SKIP 不被修改。

本轮恢复后的快照为 `snapshot-17/`：原始跳过的最近一次执行为 **94项PASS、9项FAIL、35项NOT_RUN**；保留所有历史失败的口径为93 PASS、10 FAIL、35 NOT_RUN。差异来自snooze首次allocation失败的身份在第二次已通过，而第二次另一身份因旧workspace菜单selector超时。默认538个身份、400 PASS与138 SKIP分母保持不变；补测不增加原始分母。快照读取时 `runtime-supplement/stage-approvals-01` 尚缺最终completion；运行负责人随后确认其已通过并继续下一场景，因此这不是最终快照。之后必须使用新 `--label` 对账，不能复写历史状态。

失败历史与最新执行结果分别保留：逐身份`status`采用保守“任何原始失败仍在”的口径，`latest_execution_status`单独记录最近一次完整执行。领域/lane矩阵的当前status使用最近执行，加上明确的未解决缺陷与缺口；`retained_history_status`另保留全部失败历史。后来的通过不会静默删除原失败。需要由责任人结合具体环境修正/产品缺陷结论判断某个原始失败是否已闭合，不能拼接不同运行中的成功片段为整套通过。

`coverage-map.json` 将原始身份映射到 T01–T25，并形成每个快照的 `coverage-matrix.{json,md}` 与 L0–L10 表。跨领域的同一用例允许引用多次，**这些领域计数不能相加作为独立测试总量**。非浏览器证据保留单独路径和摘要；本审计不把文件存在或准备阶段通过提升为实际浏览器/原生通过。

## 已核实的失败与计数语义

- 原始 runtime 的 `inventory_complete` 同时混入“存在测试断言错误”的判断。例如 snooze 与 full Main 已记录全部计划身份，但该字段为 false。新审计把身份完整性与测试通过分开，保留原字段和原 problems；完整失败身份不等于漏记用例。
- `inbox-snooze-01` 是 1 FAIL、5 serial SKIP。fixture 在合法 actor/业务 UI 前拒绝缺失的专门 allocation，而且该 fixture 要求 password；先前泛用 `--snooze` 仅设置开关不足以准入。原失败保留，准确 allocation 的新 lane 才能关闭其覆盖缺口。
- `installed-main-02` 是 1 FAIL、零 skip/retry。真实包已进入密码表单/登录，后续控制器等待旧 `Collaboration` heading 失败，后面的 Stop/restart/Resume 没有证明。不能把已通过启动/登录子步骤提升为 full Main PASS。native 独立回执记录源码/包未变、原协议默认 URL 和九个注册路径恢复、自有进程退出。
- 既有 `legacy-broad-01` collector 失败和 `comment-authority-legacy-01` probe 接口失败缺少预期输出，作为明确的准备失败保留；这不使后来已独立核验的完整 lane 失效。
- 评论机器权限真实 HTTP 缺陷、原整包 Go race 失败和安全扫描发现仍按各自原始证据保留。窄范围诊断通过不关闭这些发布阻塞。

## 需要继续执行的本地范围

快照17中35个未执行原始身份为：business-scenarios 3、cross-client 1、Linux installed-main 1、knowledge-deployed 1、lane-close 6、lifecycle 2、memory-runtime 1、offline-result 2、order-export 2、recovery-extras 6、recovery-runtime 4、runtime-output 2、stage-approvals 1、stage-execution 3。运行负责人按已审配置顺序推进；knowledge-deployed同一身份还须分别记录开关开启和关闭，单个PASS不能关闭配置矩阵。最新确切列表在 `snapshot-17/original-skip-dispositions.json`。

L10已完成三个原始阶段。独立解析对应原始collection/report、completion绑定的样本与清理证据，得到3个原始身份PASS；这不是新增浏览器UI覆盖。100账号/100事件/100次写回、原阈值和暖身排除规则由独立算术回执复核。T24仍保留全campaign最终资源清理缺口，不能把性能子场景的清理提升为所有运行均已结束。

T02/T15 的真实 cookie 缺失/伪造/其他会话 CSRF与恶意 HTTP/WS Origin子范围已由`security-boundary-02`关闭：1个私有Playwright测试、11个边界见证PASS，嵌套/外部completion摘要和清理均成立。它不增加默认538身份分母；T02其余密码迁移身份尚待执行，T15仍有真实评论机器越权失败。首轮security01表面9 PASS、2 FAIL中，恶意browser CORS PASS受到Playwright合成预检污染，只有8项独立真实见证可保留，不能以9计算。详情见`security-boundary-probe.md`。

快照17的9个最新失败身份及未完成边界如下。这些失败不能归纳为9处已证明的产品故障。

| 原始场景 | 数量 | 已观察到的失败 | 尚未完成的链路 |
| --- | ---: | --- | --- |
| 管理资源发布真实上传 | 1 | 原生consumer等待旧Skills导航链接 | 完整市场同步、恢复及撤回 |
| manual-detail Web/Desktop | 2 | fixture等待旧Workspaces/Collaboration侧栏按钮 | 完整scope-restoration与后续旅程 |
| packaged host-installed | 1 | 真实登录后等待旧Collaboration标题 | Stop、暂停后重启、显式Resume及最终历史 |
| information-recovery Desktop | 1 | 合并Web/Desktop的36次注册超过单API的30次/5分钟预算 | Desktop F未完成、G未运行；不能用先前A/B/C/E片段拼接整例通过 |
| installed-cli | 1 | copied relay执行权限前置失败 | 第一次合法旧CLI写入及三类共9次拒绝写入 |
| PAYcreation Web/Desktop | 2 | 创建前等待旧Collaboration标题 | 完整创建和不可变创建身份 |
| snooze跨工作区mention | 1 | fixture点击当前工作区按钮而未打开Switch workspace菜单 | 持久snooze、普通/群mention保持休眠、个人mention/分配唤醒及跨工作区未读 |

snooze首次allocation遗漏另作为历史测试准入失败保留。真实机器评论凭据越权具有实际受保护写入证据；race锁冲突机制已观测，但竞争持锁者和根因尚未知；安全扫描的可达发现也不等同线上利用复现。机器账本在每次失败观察上保存`failure_classification`，保留产品证明与未知边界。

Linux完整Main是本机可准备的额外原始L7场景。当前 `linux-main-acceptance` 仅有计划及缓存image前置探测：Linux arm64、Node22.23.3、Playwright1.58.2、Xvfb和空HOME检查通过，没有当前包、ASAR完整性、launch准入或实际Main回执。需完成当前源码打包和独立审查后才能执行，不能因尚未打包就宣称平台不可用。installed-cli02的volume已按canonical流程清理，未留copied文件UID/mode/mountflags，故不能仅凭host上的0700权限确定根因；新的最小复制/执行诊断属于独立前置工作，不重写原失败。

原生 iOS、其他已缺浏览器和对应 Windows/签名/updater 条件保持明确平台缺口。196 个 mobile headless tests、renderer fixture、ad-hoc 包组装分别只证明其实际层次。

## 验证和资源边界

本审计的 8 个离线反例通过，包含丢身份、重复身份、同 ID 改名、重试、失败但身份完整、后续通过保留失败以及 SKIP 不升级。Python AST 检查通过。审计没有修改产品或既有测试，没有重复执行已通过的全仓 lint/typecheck；同一源码既有前端静态 receipts 继续作为产品检查依据。新增私有安全探针自己的 lint/typecheck 另有实际执行证据。

主会话已提供 `backend/root-cleanup-result-20261125.json`：自有 backend PG/Redis 已停止，停止前排除审计连接后的客户端为零，数据卷保留。runtime/performance 资源尚有后续执行；本审计只读取历史 cleanup 回执，不宣称整个 campaign 的当前进程/容器最终清理完成。

刷新命令：

```sh
python3 artifacts/qa/main-post-merge-e2e-20261124-080504/coverage-audit/audit_coverage.py --label snapshot-NN
```

报告完整性与产品是否可发布分别判断。计划矩阵有明确状态，不代表所有计划项都已通过。

## 协调中断观察

在后续顺序运行的handoff等待期间，原生agent与主会话的实时agent inventory均显示`errored: This request was blocked by our safety systems. Reason: Potentially unintended activity.`。该状态没有给出具体被拒绝操作，不能据此判断产品缺陷或权限根因。runtime与coverage agent仍在运行，后续仅继续此前已有独立accepted review的明确本地范围，不自行签发新的review或复做未知的被拒绝操作。已接受的旧CLI readonly/network-none兼容性检查与只读证据对账继续服从现有资源排他约定；尚未执行的阶段和最终主会话整合保持未完成状态。

只读检查还确认`native-response-01`在`context_paths`准入即拒绝：原runner的`--probe-only`上下文令renderer为null，probe要求真实分配renderer origin，因此没有`probe-result.json`也没有原生transport试验体。它不能授予`PAY241_S3_RESPONSE_STAGE_HOLD_ACCEPTED`；Desktop Extras第六个延迟protected-read用例继续待准入。该接口修正需要原负责人与独立审查恢复，当前未越权修改native代码或伪造通过标记。


## 终审草案补充：等待Linux04与全局收尾

已生成 `coverage-audit/acceptance-final-review-draft-20261125.json`。HEAD仍为103e2bc32、tracked diff为空，但这不是最后物理来源/工具/制品终审。Linux03在owner-only synthetic文件mode准入前失败，未启动Main；Linux04以精确0400/0500修正独立接受并进入实际运行，草案读取时无completion。任务完成继续HOLD，parent负责状态。

剩余四道门：Linux04真实Main边界与原始terminal、最后全局source/resources审计、新immutable覆盖矩阵、parent最终report/defects/checklist交付。CLI03已独立核实240005ms timeout及historicalcleanupFAIL，9relay starts不能记全例通过；后来20containers/10volumes不存在也不改写fallback删除失败。AC01计划充分，AC03/05已记录部分可核实，AC02/04/06最终闭合须上述门。可最终完成验收工作并交付产品FAIL，但不宣称绿色条件成立。
