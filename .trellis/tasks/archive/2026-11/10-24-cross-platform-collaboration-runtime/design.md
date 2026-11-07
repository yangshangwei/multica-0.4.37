# 技术设计：普通本机执行能力承载协作任务

> 2026-10-26，in_progress。已按用户确认方向实施；代码与验证进度见 [验收记录](verification.md)。新模式默认关闭，旧 namespace 强保证仅适用于原模式。

## 设计选择

新增普通本机执行模式，复用现有 Provider 的启动、模型参数、输出解析和平台进程管理。协作层继续拥有授权、输入快照、真实身份、会话键、启动许可、审计、队列和结束回执。

权威的消息字段、校验矩阵和状态定义在 [代码契约](../../spec/server/collaboration-host-execution.md)；业务要求见 [PRD](prd.md)，用户文案见 [copy.md](copy.md)。

不调用普通任务整体 runTask 来绕过协作入口，不为每个平台重新实现全部 Provider，不重写整个 daemon.go。旧 Linux 执行只因兼容旧运行和旧客户端而继续保留原语义；新模式不是它的自动 fallback。

## 复用与职责

| 层 | 复用 | 新适配职责 |
| --- | --- | --- |
| Provider | Backend.Execute、ExecOptions、Session、已有消息/结果/用量解析 | 所有真实启动与等待的生命周期观测；内部 fallback 也受协作启动门禁约束 |
| 环境 | execenv 的目录、工作树、临时目录、Provider home 准备辅助函数 | 协作专属上下文与键；明确文件输入；逻辑上的业务/收尾凭据分离 |
| daemon | 已注册的 runtime/executable、机器容量、watchdog、消息收集 | 协作执行适配器、平台持久账本、协作结果上报，不调用普通 StartTask/ReportProgress |
| Server | 成员/资源授权、目标、通道、合批、小队、身份与回执幂等 | 新模式准入、逐启动许可、模式对应终态谓词、兼容及重新授权 |
| Web/Desktop | 现有任务、讨论、目标控制、共享记录 | 新能力/终态解析、真实范围展示、授权说明与旧客户端限制 |

## 数据流

~~~mermaid
flowchart LR
  A[协作者提交带目标 ID 的指令] --> B[Server 校验参与者与本单授权]
  B --> C[固定作者、输入、目标与执行模式]
  C --> D[daemon 领取并准备独立协作环境]
  D --> E[每次实际启动登记并取得许可]
  E --> F[复用普通 Provider 执行]
  F --> G[收集结果、用量、启动及终态观察]
  G --> H[Server 幂等记录并按模式结算]
~~~

1. 保留现有 comment/input 的原子写入及 request UUID。提及存 typed resource ID，名称不决定权限。
2. capability、runtime 配置版本、目标配置、授权及 Provider 组合在 claim 时固定；输入作者逐条固定。
3. 本地账本在任何可能执行任务内容的启动之前持久化启动身份。准备子进程同样归属本 run，不得脱离停止控制。
4. 授权 Server 许可后才执行；相同启动身份只能真正启动一次。获取不到新许可时不能内部重试。
5. Provider 的业务结果与终态观察走独立记录；回执丢失只重放，不重新执行。
6. 模式对应的终态满足后结算该 run；小队组必须等每个成员/续作各自满足，不以队长结束代替全组结束。

## 会话、目录和身份

稳定协作会话键包含：
collaboration / deployment / resource-owner / agent / issue-session / channel / provider-runtime / role / session-generation。

小队成员再加入 execution-group / member identity，不能与队长或该成员在另一组的会话混用。同一组的队长续作保留适用身份；普通 target aliases 共用调度 lane 不等于无条件共用 Provider 私人会话。

- 会话指针由 daemon 的本地身份映射保存，用户文本、输出或随意传入的 session ID 不能选择其他会话。
- 实际拥有者、runtime/provider、账号来源、角色或不兼容配置改变时推进 session-generation。普通人类转交不改变这些身份。
- 合法指针不可恢复时建立新会话，从本单已授权快照重建并记录连续性丢失；不寻找“最近一次会话”。
- 稳定工作目录按上述键管理；运行临时目录再加入 run/launch。目录外访问遵循普通 Provider 和系统用户权限。
- 同一稳定会话/工作目录身份同时只允许一个受管理执行持有使用权，使用现有调度/环境锁并以持久账本重建未结束占用。两个不同合法 run 争用同一身份时一个执行、一个等待；不能占住稀缺机器槽等待目录锁而阻塞持有者收尾。unknown 不提前释放使用权，daemon 崩溃导致 OS 文件锁释放也不代表逻辑占用结束。
- 代码仍来自明确的 Git 提交或附件，不自动把私人未提交目录复制给协作任务。
- 逐指令存 actual author；Agent owner、runtime/resource owner 和实际 Provider 单独存。合批不能只检查或显示第一位作者。
- 派生成员、续作及 fallback 都关联原指令集合和父 run；Agent 在文本里写的身份不是授权依据。

## 权限与审计

按单 owner consent、参与者、目标配置、额度与暂停规则保持。提供给 Provider 的 Multica 接入仅包含协作业务身份，不主动注入普通 mat_、个人 PAT/JWT 或 daemon 管理凭据。

收尾 token 只供监督器取停止意图及上传本 run 的最小结束回执；撤权后不可继续用它上传正文、附件或结果。宿主进程与监督器通常同属一个系统用户，因此这种区分是应用层协议和参数管理，不能再声称同用户恶意进程无法读取监督器文件。

授权确认对象包含本机执行模式、Provider/版本、已有工具权限和有效配置指纹。已启用的 MCP、hooks、账号或其他工具均按普通配置与适配器支持情况处理；托管且可检测的权限扩展必须使原同意失效。宿主其他配置不作密封冻结或全机监测承诺，遵循已接受的普通本机信任范围；不要为这一要求新增全机配置监测。

审计写入 request、command、input author、target、run、parent run、launch、attempt、资源来源、配置版本、模式、停止操作者/范围、业务结果、用量和结束观测。共享记录仅投影获准的内容及最小来源摘要，不上传原始环境变量、凭据、完整启动参数和私人会话；同系统用户可用凭据的风险在授权中明确。

## 执行模式与结束语义

新模式 host_process_supervision_v1 的范围是 Provider/适配器已登记的受管理执行，不提供 Linux namespace 的封闭后代保证。

| 事实 | 取值/例子 | 对调度的意义 |
| --- | --- | --- |
| 业务结果 | success、failure、结果缺失/记录不完整 | 不单独证明执行结束，退出码零也不自动生成成功 |
| 受管理执行 | 未启动已确认、host_exit_observed、unknown | 决定新模式能否结算、释放逻辑容量、参与 Resume 判定 |
| 后代清理 | tracked_scope_complete、best_effort、unverified | 说明实际观察范围；不能单独推导受管理执行结束或全后代消失 |

host_exit_observed 要求所有登记的直接启动已等待结束或经验证的适配器生命周期终结、启动登记已封口且不可能再 fallback。没有未知启动，适配器声明负责的范围中没有已知仍在运行的执行。

普通 Result.ProcessExited 的含义因 Provider 而异，不能将它当作唯一统一门禁：有些实现已经 cmd.Wait 但未填写该字段。实现需复用集中 launch/wait 事件或添加经过验证的生命周期回调。业务完成文本、Result 通道关闭和 stdio EOF 均不足以单独证明终态。

能力声明列出允许的监督范围及普通降级，例如 Job 绑定失败后退至直接进程管理；每次 launch 记录实际范围。允许的降级属于同一宿主能力的已知范围，不伪装成 Job 成功，也不临时切换为另一执行模式。超出声明的范围或无法观测直接执行终态则拒绝/保留未确认。

普通模式可在上述受管理执行结束后结算，即使逃离进程组的后代未受验证。已知受管理执行仍在运行、启动后未登记、账本损坏或身份不确定时仍为 unknown。不给 unknown 增加“强制忽略并恢复”入口。

gateway/远端 Provider 模式必须有对应远端结束确认；本地客户端退出不能冒充远端工作结束。没有可验证契约的组合标为不可用，不假装已支持全部普通 Provider。

## 停止、网络与恢复

| 情况 | Server | daemon / UI |
| --- | --- | --- |
| 有权者停止目标/整单 | 先封闭扩展并挂起对应 lane/targets | 取消该范围内所有已登记启动；显示停止请求已受理 |
| 暂时超时、断网或 5xx | 不释放未确认 reservation | 已运行执行可继续；同步重试；无新许可不新增 launch/派生 |
| 明确撤权、任务结束、身份失效 | 拒绝新业务访问和启动 | 取消执行，业务结果不越权补传，收尾身份仍可交回执 |
| 可靠终态已持久化但回执丢失 | 等原回执接受后推进队列 | 重放同身份回执，不重新执行 |
| daemon 重启且旧终态未知 | 保留原 run、nonce 与恢复屏障 | 尝试恢复原终态观察；无可靠证据则显示执行状态未确认 |
| 终态确认后用户未恢复 | 保持挂起 | 重连、收到回执、重新授权都不自动恢复 |

本地物理槽在可靠终态观察持久化后可以释放；Server 的逻辑 reservation 仅在接受同身份、同模式回执后释放。下一批仍须经过 Server 准入，不能借这一区别越过旧 run 屏障。

恢复只消费下一批有效等待输入，不重新执行被停止/取消的命令。没有终态证据的未知执行不自动重试；系统重启、PID 查不到或 Provider 报错不是默认的结束证明。未来若要增加经验证的进程实例/系统重启恢复，必须作为明确的证据类型实现测试，不能用猜测解除屏障。

## 防重复与内部重试

区分用户 request、逻辑 run 和实际 launch。一次 Backend.Execute 可能内部启动多次，外层 fresh-session retry 也可能再启动；均进入同一登记/许可/停止门禁。这里登记的是 daemon/Provider 适配器直接创建的进程及其准备 helper；模型工具自行派生的操作系统后代只按普通监督范围记录清理事实，不宣称逐个拦截或实现工具副作用 exactly-once。

新增 v3 的逐启动终态接口 POST /api/collaboration/runtime/runs/{runId}/launches/{launchId}/termination。它使用原 run 的收尾凭据，幂等记录该 launch 的未启动或真实结束观测，不封闭整个 run、不结算执行组、不释放 run reservation。撤权后仍可完成这一窄用途收尾。

Server 在同一事务中校验 attempt N 的终态已持久化、当前授权/停止状态及 run 未封口，再许可 N+1。所有启动都结束且不再 fallback 时，才提交最终 host_exit_observed 并结算 run。最终回执的封口序号必须覆盖 Server 已发出的全部许可，不能跳过一个未知 attempt。旧 runtime v2 保持单 attempt 语义。

每个 launch 使用新 attempt sequence、nonce 和 launch ID；相同身份重放不再次启动。前次真实启动尚未确认结束时不许可后次。语义明确的“无法恢复旧会话”允许在前次受管理启动已结束、当前授权有效且未停止时新建会话尝试；网络不确定和普通业务失败不能被转换成自动重试工作。

把普通 Provider 的现有重试次数作为上限，不新增自动重试策略。停止封口与每次实际 spawn 串行化；permit 和 spawn 之间若收到停止，必须阻止后续 spawn。已越过不可回退的启动点则标记真实启动疑点并等待原观察，不烧第二个 nonce 重跑。

防重复保证的是系统不因请求/回执重放而再次启动同一执行身份，不是任意 shell、Git 推送、MCP 操作或模型自行重试的业务副作用 exactly-once。

## 协议、存储与客户端兼容

- 新宿主消息使用 runtime protocol 3；旧 Linux protocol 2 继续按原严格消息形状及回执语义处理，不全局替换版本常量。
- 复用 containment 字段表达固定能力，新增 host_process_supervision_v1；新的终止状态为 host_exit_observed。旧 exited_proven 不重释为普通进程结束。
- 新执行读取视图采用 v6；新客户端同时解析 v5/v6。通过显式读取能力协商选择版本，不能以某个版本字符串猜测支持。
- 未声明 v6 能力的客户端，对包含宿主模式的执行图提供合法的 v5 unavailable 投影，隐藏该图的 AI 操作能力，不把新节点强转成旧证明。人类任务读取/评论继续可用。旧版至少展示已有的执行不可用状态，服务端返回稳定升级拒绝码；不要求已经安装的旧二进制认识新的文案。直接 AI 命令/控制入口拒绝不兼容模式。
- 混合评论沿用 AC91：保留评论与有效人员通知，仅对旧端不兼容的 host AI 目标返回 blocked/升级拒绝回执且不入队；兼容的其他目标照常逐项判定。原请求恢复/重放保持原拒绝，不在升级后自动执行，调用者须明确另行提交。完成/取消任务、退出、移除等人类流程继续按原权限执行，其引发的系统收尾不属于被禁用的“直接 AI 操作”。
- 查询参数 execution_view_version=6 作为读取协商；未提供按 v5。新模式的人类 AI 写请求携带 execution_view_version=6，并由对应严格 schema 校验；该字段仅声明客户端理解的语义，不授予权限。
- 采用新终止状态后必须以前向迁移扩展 CHECK 约束，更新相关 SQL/sqlc/Go/前端枚举和所有结算谓词。复用已有 containment text；在现有 collaboration_attempt 上新增 launch_descriptor JSONB 与 terminal_observation JSONB 保存 v3 的许可描述和逐启动终态。新模式令 launch_id 与 UUID 格式的 attempt_nonce 相同，避免新增重复身份列。旧行保持 NULL 并按原模式解释，不改历史 migration。
- 本地 ledger 升为 v2，保存模式、进程实例身份、所有 launch 及真实观察。旧 ledger v1 只按原 Linux 语义读取；Windows 补文件权限、原子持久化和跨进程锁。
- 宿主准备采用新的受校验能力/manifest 分支，复用冻结配置及附件/Git规则；不得设置 preparation=0 绕过输入约束，不要求对宿主 CLI 做 Linux ELF bundle 沙箱验证。

## 升级与回退

1. 先部署可读取新旧协议的服务端、迁移和客户端，宿主能力保持关闭。
2. 交付通过相应 Provider/OS 原生验证的 daemon；显式注册其实际能力与配置版本。
3. 从强隔离切换普通本机执行触发资源重新授权。旧等待输入保留，授权后仍须明确恢复。
4. 已 claim/permit 的旧 Linux run 固定原模式至收尾；旧 unknown、nonce 墓碑和停止屏障不清空。
5. 回退时关闭新宿主准入，保留新旧回执读取/收尾能力。存在新状态数据时不得直接降回不认识该状态的旧服务端二进制，也不得通过 down migration 删除历史。

## 源码依据

- server/pkg/agent/agent.go:17：Backend、ExecOptions、Session 的可复用边界。
- server/internal/daemon/daemon.go:6838、7610、7938、8434：普通 runTask、凭据、外层 fallback、消息/结果收集的耦合。
- server/pkg/agent/launch.go:112；server/pkg/agent/codex.go:902：集中启动构造和 Provider 内部重试，不能只统计 Execute 调用。
- server/pkg/agent/claude.go:354、735；server/internal/daemon/execenv/codex_sandbox.go:177：不同结束字段与普通权限配置。
- server/pkg/agent/proc_windows.go:95；server/pkg/agent/proc_other.go:27：Job 降级与 Unix 进程组，不能等同 namespace。
- server/internal/daemon/collaborationrun/runner.go:17、81、160；ledger.go:302：具体 Process、固定 Linux 能力及原终止证明。
- server/internal/service/collaboration_runtime_capabilities.go:125；server/pkg/protocol/collaboration_preparation.go:275：当前模式准入。
- server/pkg/db/queries/collaboration_runtime.sql:230：结算与容量谓词需随新状态一起修改。
- server/internal/service/issue_admission.go:46：普通业务入口不能直接接收协作单。
- packages/core/collaboration/execution-schema.ts:98：严格 v5 投影和枚举需要版本兼容。
