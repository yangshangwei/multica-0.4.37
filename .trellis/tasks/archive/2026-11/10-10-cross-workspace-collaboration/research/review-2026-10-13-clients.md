# Research: 最新需求的客户端、通知与兼容一致性复审

- Query: 检查 R41–R58 新规则及 R36 来源成员退出与账号工作台、通知、旧客户端的接线。
- Scope: internal；主会话直接复审。两次原生研究代理限流，标准研究代理服务不可用，没有把这些失败调用视作独立审阅。
- Date: 2026-10-13
- 输入：当前 PRD R01–R58、AC01–AC89、D09–D11、P3/P6/P8、数据契约、旧客户端审阅；§49 只用于恢复已写明的提及编辑规则。

## Findings

### CL01：R55 评论编辑新增提及的投递契约未从讨论进入主设计

讨论 §49 明确“评论编辑只通知新增的提及者”，但 D09 的一般事件规则是通知全部参与者，R52 清单又只列新评论；P3 的提及实现项未固定新旧差集。直接复用新评论 fanout 会重复通知全体，完全跳过 edit 又会遗漏既定行为。

修订：新评论事件给除操作者外的当前参与者发一条通知，提及者只改变该通知的展示种类；评论编辑在锁内比较前后有效人类参与者提及，仅新增的接收人产生 mention_added 通知，不发全体编辑通知。正文中的 R55 提及同样使用当前参与者和新增差集，不把普通编辑变成全体广播。操作者排除、@all 当前参与者展开、@AI 不执行、非法 UUID/agent/squad 不产生能力；同 event/recipient 只一条。投递重试沿用原事件，不因后来重加参与关系重算收件人。

数据契约需明确通知 UUID、recipient/event 唯一性及 mention 展示字段，R57 线程/表情操作不混入通知事件。

验证落点：AC83/AC86；新评论同时提及只一条通知，编辑 A→A+B 只通知 B，删除提及不通知，@all、操作者、撤权、重放和旧来源 inbox 无重复。这是补全已确认规则的工程合同，不重新提出产品选项。

### CL02：R57 API 路由与通知路由身份需要落定

D03/P3/P6 已允许 resolve/unresolve 和本人表情回应，D09 端点族却未列出；通知 route 使用 {id}，数据表只定义 recipient/issue/event_seq。实现前需固定稳定 UUID 和单一端点族，避免 shared view 回到受来源 membership 限制的旧入口。

修订：在协作 issue scope 下增加线程 resolve/unresolve、评论 reactions、本单 reactions 的端点；全部使用本单访问依据及实际人类账号，表情删除只删本人。沿用现有 comment/reaction 记录，不构造第二个普通 issue。通知列表/已读/归档均按真实 recipient 所有权授权。

验证落点：AC88 由 P3 主验证；AC83 由 P6 集成。来源外参与者能操作，有来源评论权者能操作，第三人和机器不能操作；无新 command/task/通知。错单评论 UUID 和他人 reaction 不能越权。

### CL03：来源成员失效必须重算账号视图投影，不能只做空间重定向

`packages/core/realtime/use-realtime-sync.ts:1320` 当前 member:removed 清来源空间缓存并重定向；它不维护未来账号 scope 的授权快照。R36 明确发起人退出来源空间后仍保留生命周期权，AC80/81/89 又要求结构字段失权。如果 account view 只听协作参与关系撤销，失去来源成员身份后可一直显示旧字段能力。

修订：来源 membership 变更须触发相关账号/issue scope 的投影失效。账号页重验来源成员和参与关系两种依据，保留仍合法的生命周期与协作历史，清除来源目录/结构编辑缓存，不把 workspace loss 当作 participant removal。仍需挂载、聚焦、重连快照补偿；来源空间晚卸载不能覆盖 account 镜像。

验证落点：AC27/55 的服务端权限由 P2 负责；AC31 的双端接线覆盖打开详情时退出来源空间，生命周期按钮保留、结构编辑消失，刷新/重连结果一致。账号组不被空间组清理误删。

### CL04：受保护 API 的 schema 降级必须显式关闭原始载荷日志

`packages/core/api/schema.ts:50–61` 在未传 redact 时记录 `result.error.issues` 和 `received: data`。D10 要求所有协作 API schema 校验，D11 又要求日志只含 ID/结果码；复用默认调用会把畸形私有评论、指令或附件能力 URL 写入客户端日志。

修订：协作内容、通知、受控附件/能力和操作回执解析统一传 `{redact: true}`，失败返回 typed unavailable，不记录正文、token、URL、请求载荷或 Zod 收到值。可选显示字段单独降级，关键身份/权限/版本失效不得解释为普通任务、空列表或已停止。

验证落点：P6 schema malformed-response tests 捕获 logger，确保所有错误输出没有私有 sentinel；旧 `IssueSchema.revision` 数字保持原样（`packages/core/api/schemas.ts:1324`），新协作十进制字符串与其缓存隔离，普通+协作列表不整页清空。

## Coverage and retained decisions

- R01–R07/R19/R20/R29/R33/R34/R36/R39/R46：账号待办、参与入口、角色动作及原子创建；CL03 补来源 membership 失效的双端接线。
- R08–R18/R23–R28/R30–R35/R37/R38/R40：客户端只消费服务端能力、控制版本和真实 proof 状态；安全性由执行/权限报告及 P4/P5/P7 的真实测试证明，不能以 UI 按钮代替。
- R21/R22：同 Server、Web/Desktop 完整接线，Mobile/旧客户端仅允许服务端许可的投影和动作；不新增跨 Server 或 Mobile 功能。
- R41/R42/R47–R51/R54/R58：角色与来源成员的差异、字段/版本/来源选项、最小引用、外部摘要及看板拖动在现有设计中一致；保留 P1 规范矩阵与 P6 接线分层。
- R43/R44/R52–R57：CL01/CL02 补提及、通知和线程端点；私有 draft、移除展示后留存、禁止评论转普通子任务及机器隔离仍按已定契约。
- 旧客户端审阅 C01–C06 的已修规则仍成立：无 cookie 原生下载短能力、明确新建入口、不同列表 query key、旧数字 revision、新状态选项版本及账号页不依赖 workspace list 成功。

## Caveats / Not Found

未修改产品或运行产品测试，没有把工程接线缺口说成已有功能故障。认证/SQL/运行监督的完整证明由另外报告及实现承担。所有上述修订保持 58 项需求与 89 项验收编号，无需新增产品选择；不能据此宣称全功能已验收。
