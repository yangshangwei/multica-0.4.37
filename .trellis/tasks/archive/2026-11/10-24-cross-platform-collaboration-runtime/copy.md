# 普通本机协作执行：授权与状态文案契约

> 文案契约，2026-10-26。共享界面及中英文 locales 已按本契约实施，仍须完成整体验收。仅适用于 host_process_supervision_v1；旧 Linux 运行继续展示与原证据匹配的历史信息。

## 授权确认

| 位置 | 简体中文 | English |
| --- | --- | --- |
| 执行方式 | 普通本机执行 | Standard host execution |
| 授权标题 | 允许本任务使用此智能体 | Allow this issue to use this agent |
| 授权正文 | 允许本任务当前及以后加入的有权参与者向此智能体提交指令，并控制本任务中的执行。执行使用所选运行环境的 AI 账号和工具权限，与普通任务相同。 | Allow authorized participants in this issue, including those who join later, to submit instructions to this agent and control its executions in this issue. Execution uses the selected runtime's AI account and tool permissions, as ordinary tasks do. |
| 本机访问说明 | 本任务使用独立会话和工作目录。工作目录不构成操作系统沙箱；智能体仍可访问普通配置允许的本机文件、账号和工具。 | This issue uses a separate session and working directory. The directory is not an operating-system sandbox; the agent can still access host files, accounts, and tools allowed by its ordinary configuration. |
| 停止说明 | 停止会阻止本次所选范围继续派发，并请求结束该范围内已启动的执行。按普通任务机制清理进程，不保证清除脱离管理范围的后台程序。 | Stopping prevents further dispatch in the selected scope and requests cancellation of executions already started within that scope. Cleanup follows ordinary task behavior and does not guarantee removal of background processes outside the managed scope. |
| 确认按钮 | 授权本任务使用 | Authorize for this issue |
| 重新授权原因 | 执行方式或工具权限已变更，需要资源拥有者重新确认本任务授权。已有等待指令会保留，确认后仍须明确恢复。 | The execution mode or tool permissions changed. The resource owner must confirm this issue's authorization again. Waiting instructions are retained; resuming still requires an explicit action. |

确认区域同时显示智能体名称及拥有者、所选运行环境及资源拥有者、Provider 和账号来源的安全摘要。运行环境名称复用 runtimeDisplayLabel 等既有 helper；不展示密钥、私人完整路径、原始命令行或账号文件。

## 状态与操作结果

| 实际事实 | 简体中文 | English |
| --- | --- | --- |
| Server 已接受停止，尚未收到执行端终态 | 停止请求已受理，等待执行端确认 | Stop requested; waiting for the runtime |
| 执行端断线且有停止请求 | 执行端暂时离线，停止尚未确认 | Runtime offline; stop not yet confirmed |
| 本地取消/等待进行中 | 正在停止本次执行 | Stopping this execution |
| host_exit_observed，停止导致本次受管理执行结束 | 本次受管理执行已停止 | Managed execution stopped |
| host_exit_observed，正常结束 | 本次受管理执行已结束 | Managed execution ended |
| host_exit_observed 且 best_effort，确有清理尝试记录 | 受管理执行已结束，并已尝试清理相关进程；未验证管理范围外的后台程序 | Managed execution ended and cleanup was attempted; background processes outside the managed scope were not verified |
| unverified 后代清理（独立事实） | 后台进程清理情况未验证 | Background-process cleanup was not verified |
| 未观测到可靠终态 | 执行状态未确认，暂不能恢复 | Execution state unconfirmed; cannot resume yet |
| 已结束但结果没有有效记录 | 执行已结束，结果记录不完整 | Execution ended; result record incomplete |
| 普通网络错误，未观察到停止 | 连接暂时中断，本机执行可能仍在继续 | Connection interrupted; host execution may still be running |
| 明确恢复成功 | 已恢复，将处理下一批有效指令 | Resumed; the next eligible batch will be processed |
| 能展示升级原因的客户端收到升级拒绝码 | 当前客户端不支持此执行方式，请升级后操作 | Upgrade this client to control this execution mode |
| 会话不可恢复，合法建立新会话 | 原会话未能恢复，已根据本任务记录建立新会话 | The previous session could not be resumed; a new session was created from this issue's records |
| 适配器/版本组合未通过准入 | 此运行环境暂不支持协作执行 | Collaboration execution is unavailable for this runtime |
| 新版执行视图的派发开关关闭 | 新的 AI 执行暂未开放。 | New AI execution is disabled. |

业务“成功/失败”、任务单“完成/取消”和上述执行状态分别展示。清理字段不能覆盖受管理执行的 unknown 状态；尚未确认结束时，主状态仍为“执行状态未确认”。旧安装版不保证认识新的升级文案，至少保留其既有“执行不可用”显示；升级原因由支持该错误码的客户端及诊断入口展示。标题不能把无有效结果的进程结束显示为任务成功；结束回执不自动恢复队列。

新版执行视图在派发开关开启时不显示全局“共享 AI 执行暂不可用”；具体目标仍按其实际授权和运行状态说明原因。开关关闭只说明新的执行暂未开放，不覆盖已有执行的终态、停止控制和记录。

## 审计展示

- 指令：由 B 提交；智能体：A 的开发助手；运行资源：C 提供的环境（相同时可合并显示）。
- 执行和重试属于原任务及原指令；展示实际尝试次数，不把重试改写成资源拥有者重新提交。
- 小队成员执行显示父执行及原始指令来源；AI 文本中的自称不能覆盖这些字段。
- 名称相同仍按稳定 ID 区分；展示拥有者有助于识别，不以名字授权。

## 新模式禁止沿用的表述

| 不适用表述 | 应表达的事实 |
| --- | --- |
| 只能访问本任务目录 | 目录独立，但本机访问遵循普通权限配置 |
| 已确认全部子进程和后台程序退出 | 受管理执行已结束，并显示实际后代清理范围 |
| 撤权后立即停止所有本机操作 | Server 立即阻止新授权操作；执行端收到后请求停止 |
| 断线即停止 | 本机可能仍在执行，停止/结果等待同步 |
| 日志流关闭，所以执行已停止 | 必须先有实际生命周期终态观察 |
| 执行完成，所以任务成功 | 业务结果、任务单状态与执行终态分别判断 |
| 升级不影响原授权范围 | 从隔离模式切换普通本机权限需要重新确认 |
| 保证智能体无法接触宿主凭据或监督器文件 | 协议不主动注入宽域凭据，不提供同系统用户进程间的强隔离 |

## 实施时需要同步的入口

- packages/views/locales/en/collaboration.json、zh-Hans/collaboration.json。
- shared views 的资源授权、执行面板、控制回执、运行记录、准备状态和恢复错误。
- core 对新旧执行视图的 schema、默认状态及未知枚举处理。
- 相关协作内置 skill/CLI 指南、运行环境诊断、安装/升级交付说明。
- 旧 Linux 历史记录按其原模式显示；禁止全局替换导致历史强证明失真。

本文件是文案来源；已实施入口和剩余平台验收见 verification.md。
