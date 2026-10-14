# 跨账号协作实施计划

- 日期：2026-10-14；状态：P1 数据/权限及标题正文日期编辑、P2 人类流转/生命周期、P4 账号失效消费和 P6 core 契约已有验证切片；其余工作仍按下列完整阶段推进。证据见 [progress.md](progress.md)。
- 依据：[PRD](prd.md)、[技术设计](design.md)、[执行协议](research/execution-contract.md)；源码索引见[研究摘要](research/design-contracts.md)。
- 分支：`codex/cross-workspace-collaboration`。O11、G01–G18 及发起人退出来源空间后的生命周期权限均已确认（discussion §54），最终摘要见 §53；用户复审及实施授权见 §55。
- 任务保留为规划与集成责任入口；下面按可验证阶段拆分工作，批准后可据此建立子任务，依赖以本文为准，不能从任务树推断。

## P0. 开工条件和契约固定

- [x] O11 已确认：双禁用时重开并指定已有有效协作者，无候选拒绝；仅增加 R40 的窄例外。
- [x] 发起人退出来源空间后，账号及协作身份有效时保留完成/取消/重开权限（discussion §54，2026-10-13）；只确认产品规则，尚未批准实现。
- [x] 按 grill-me 复核结果重新收敛 PRD、design/implement，检查文档和上下文并呈现新摘要（discussion §53，2026-10-12）。上一轮静态审阅和 O11 复核保留为历史证据，§32 被 §53 取代。
- [x] 用户于 2026-10-13 明确授权复审后实现（discussion §55）；技术复审收口后执行 `task.py start`，无需重复询问开工。
- [x] 记录实现时 git 状态、基础提交及他人改动；需要隔离时建立独立工作区，不 reset、stash 或提交本任务之外的文件。
- [x] 在涉及层加载最新 `.trellis/spec/`。确认当前 root CLAUDE、命名规范、sqlc/迁移规则和真实 AI 测试隔离约束。
- [x] 服务端实施/检查前必读 `.trellis/spec/server/security-boundaries.md` 全文及对应层规范；自动上下文改载任务研究索引，不能把该索引当作规范替代品。
- [ ] 固定 D02/D09 DTO、status_options_version、D06/E01–E08 launch/proof/准入/锁协议和 source/execution scope；新协作字符串版本与旧 Issue.revision 数字字段分开。

最小证明：没有未决产品选择；所有 R/AC 保留；实现/检查 context 无缺失文件或超限截断。真实实现前再分配迁移号，避免并行分支冲突。

## 阶段依赖和写入责任

```mermaid
flowchart LR
  P0[P0 规划批准] --> P1[P1 数据与权限骨架]
  P1 --> P2[P2 人类流转]
  P1 --> P3[P3 受控内容与实时]
  P1 --> P4[P4 运行监督与凭据]
  P2 --> P5[P5 FIFO 与旧入口收敛]
  P3 --> P5
  P4 --> P5
  P2 --> P6[P6 Web / Desktop 工作台]
  P3 --> P6
  P5 --> P6
  P2 --> P7[P7 有限代管]
  P5 --> P7
  P6 --> P8[P8 集成与发布门禁]
  P7 --> P8
```

P1 后 P2/P3/P4 可独立研究和实现，但共享服务、router 和 migration 由单一责任人整合；不能并行各自改 `task.go`。P6 的 schema/无空间外壳可按已固定契约提前开发，完整验收等待 P2/P3/P5。三项代管共享 UI 接入随 P7 完成。涉及核心行为的实施与检查使用 Trellis 原生子代理，主会话整合、处理产品选择并验证结果。

| 阶段 | 写入责任范围 | 高风险交界 |
| --- | --- | --- |
| P1 | `server/migrations/`、`server/pkg/db/queries/`、生成 DB 代码、新协作授权/DTO 服务 | 迁移号、所有权列、旧普通 issue 判别 |
| P2 | issue/comment/状态动作服务与 handler、账号工作台查询 | 与 P5 共用队列副作用接口，不先落半套 lifecycle |
| P3 | file/storage/attachment、realtime/scope authorizer、来源记录 | 公共上传、原生下载、执行空间清理 |
| P4 | protocol、agent supervisor、daemon、purpose middleware、凭据存储 | `daemon.go`/claim finalization 与 P5 共用接口 |
| P5 | task/run 调度与控制、旧 retry/claim/start/status writers、资源撤权 | `task.go`、runtime/account/workspace teardown，由该阶段统一整合 |
| P6 | core 协作模块、views 协作页面、Web/desktop 路由/标签接线 | core API client、desktop tab/coordinator、详情组件提取 |
| P7 | admin handler/service 和共享有限代管表单 | 原管理域锁、组织范围、O11 接手原子性 |
| P8 | 集成测试、内置技能/source maps、开发/发布规范 | 不改无关功能，不用发布开关代替服务端保护 |

## P1. 数据、对象授权及原子新建

- [x] 创建协作聚合、参与关系、交接、session、command/run、grant、操作/event/outbox、终结记录及账号 fence/运行 enrollment/部署 gate；只建 schema，不启用用户创建。
- [x] 所有新索引（含主键/唯一键）各用单语句 CONCURRENTLY 迁移；无 FK/级联，检查 invalid index 与安全 down 前置条件。
- [ ] 复用 `issue.assignee_id=user.id`，creator 保留真实用户；输出独立 source/execution scope，禁止虚构成员或代理 issue。
- [ ] 新增事务内单据授权器与最小 AccountSummary，读与普通评论采用来源成员 OR 本单参与关系；角色/操作能力分开。R53：授权器按人类白名单判身份，登录会话与个人 PAT 之外（含 task_token、cloud_pat、plugin、IM 及未分类来源）只得最小引用、写入拒绝；需要时由认证中间件显式标记人类来源，不能靠缺失 header 推断。
- [ ] 原子新建 issue、协作记录、参与关系和 paused session；标记从服务端关系派生，不允许旧 PATCH 启用。
- [ ] 创建使用 source/creator/request_id 的独立操作范围及无 issue ID 的回执查询，私有 draft 一次绑定；同 key 并发/丢响应只得到原单，同 draft 换 key 拒绝。
- [ ] 增加精确接收账号查询，区分用户名/邮箱/UUID、归一化/歧义/限流；提交动作仍重验。
- [ ] generic GET 列表最小协作投影、generic writers 拒绝未适配协作操作。R54：列表/详情/时间线/评论对非来源成员的 creator、handler、作者返回最小 AccountSummary（user_id、显示名、头像、外部标记），不含邮箱/用户名，并支持“外部协作者”筛选。创建开关持续关闭，不能先上线半套权限。
- [ ] R50 迭代归属（含创建时）：锁内重验来源成员与有效发起人/handler，复用 `PrepareMembershipChange`/`CommitMembershipChange` 借用事务、原因与版本契约，结转仍由服务端负责；外部 handler 不使迭代统计/预览/历史失败或泄漏账号详情。覆盖转交、离开来源空间、迭代版本变化和幂等重放。
- [ ] R51 普通子任务：普通任务以协作单为 parent 的写入沿用既有校验、锁序与 I1 记录，只把协作单当作引用，不取 session/participant/command 锁或推进协作版本；子任务执行与本单队列/grant 隔离。详情的子任务投影按查看者来源权限裁剪。覆盖跨空间拒绝、外部协作者读取、移出/删除子任务和并发转交。
- [ ] R58：协作单 `position` 允许来源成员经普通排序写入（看板、CLI `issue reorder`），不推进协作版本、不写历史/通知，机器身份按 R53 拒绝；`stage` 按 R49 规则锁内重验并留记录，child-done 阶段唤醒语义不变。
- [ ] R49 项目/父任务（含创建时）：锁内重验来源成员身份及有效发起人/handler，沿用普通任务的引用校验、锁序与 I1 事实记录；非来源成员写入拒绝，读取只得不可打开的最小引用。覆盖转交、离开来源空间、引用失效和幂等重放。
- [ ] 按 R41/R42/R48/D03 编辑字段（R48 开始/截止日期沿用普通任务解析与清空）：锁内重验有效发起人/handler、版本及来源定义/选项，值与历史/事件同事务，复用 D08 幂等；不重写 command 或触发 AI。仅投影本单所需选项，不开放定义管理；其余未确认写入暂拒绝，覆盖旧 API/Plugin/uploader/admin 旁路，不改普通任务规则。
- [ ] schema/DTO 明确 command seq、版本的字符串传输及 typed unavailable；冻结后供 core/daemon 阶段使用。
- [ ] 落实 2026-10-13 数据复审：comment 协作版本/墓碑及 collaboration_event 历史，附件永久交付/多引用，通知稳定 UUID 和收件人幂等；R53 搜索在匹配、排序、计数与 snippet 前裁剪，不能仅删响应正文。来源 membership revoke 保留独立协作身份与 creator/handler，不推进无关账号 epoch。

验证：数据库 fixture 建两个独立空间用户，跨空间只访问本单；第三人拒绝；来源普通成员只能读/普通评论。对新建注入中途失败并发删除，验证无半张单、无普通自动任务。R41 覆盖两类编辑者成功、转交/禁用/退出后旧权限失效、并发编辑冲突、失败原子回滚和事件重放；旧 command 输入保持不变。对 actor spoof、PAT/task token、人类确认、全局 UUID 与 member UUID 混淆建立负例。迁移从空库和当前升级库验证，`make sqlc` 后生成差异可复现。

## P2. 人类交接、状态与持久待办

- [ ] 实现普通转交、邀请/移除/退出；移除当前 handler 与回交 creator、指令/资源副作用接口同事务。
- [ ] R46 邀请/派单直接生效：锁内查 R12 角色和目标有效性，参与/历史/outbox 同事务，派单再变更 handler/待办；仅邀请不改责任。覆盖离线接收、通知故障、无权/失效目标、幂等重放和个人资源不被自动授权，不实现待接受阶段。
- [ ] 实现具体请求 ID 的“补充完成并交回”和“提交验收并转交”，目标失效不自动加回，过期请求不能覆盖后来的责任。
- [ ] 落实 D04 来源映射和 status_options_version 的生成/传输/锁内重算、补充请求链及 superseded；不伪造既有目录 revision，不静默改选。
- [ ] R47 手动改非终态：锁内重验有效发起人/handler、版本和 status_options_version，仅接受来源有效非终态；前后状态/操作者/事件同事务。不改 handler/待办/command，不触发 AI，不建、不消费或作废补充请求，in_review 不产生验收交接；覆盖转交后旧权限、目录变化、终态拒绝和幂等重放。
- [ ] R47 同时验证当前来源类别非终态且 session 未 closed；handler 的 done/cancelled→todo 等 PATCH 必须拒绝，只允许具权者明确重开。AC78 与 AC55 联合覆盖，无生命周期半更新。
- [ ] 完成/取消/重开进入同一领域服务，调用 session close/reopen 原子 primitive；旧 direct status/assignee writers 不绕过角色或 R35。含 GitHub/GitLab/Forgejo PR 合并自动推进 done（`server/internal/handler/github.go:1671`）与 `WriteIssueStatus` 系统写入：对协作单跳过，不能代替 R19 发起人完成。
- [ ] AC27/AC55 验证发起人退出来源空间后仍可完成/取消/重开，账号及协作身份须有效；来源结构字段权限及代管分别由 P1（AC80/AC81/AC89）、P7（AC66）主验证。空间退出清理不得误删本单参与关系或入口，运行资源失效仍按既有撤权规则处理。
- [ ] 账号工作台查询“当前处理/我参与的”，分页与计数同一谓词；不使用 inbox read/archived 或来源空间 membership 替代参与关系。
- [ ] 幂等操作记录保存原成功/拒绝和版本；通知投递意图与责任变更同事务，通知故障不丢待办。
- [ ] 来源空间删除在清理前检查全部协作记录；协作 hard delete/move/merge 拒绝，普通空间/任务行为回归。

验证：P→D→T→D 的阶段交接，D 请求 P 补充及嵌套补充逐层交回；普通评论不交回，AI 结束不整单完成。跨空间同名自定义 status 分别属于不同类别，按来源解释。对并发转交/移除/状态目录变化、重复操作、归档通知后重登验证最终责任及入口；失败事务不留下已移除却无人处理的结果。生命周期的真实队列副作用在 P5 联合验收，P2 不单独开放生产入口。

## P3. 受控附件、来源过程记录与实时

- [ ] 复审补项：R55 comment/body edit 仅新增有效提及者通知，与同一新评论事件按 recipient 去重；投递重试不重算后来名单。R57 与 edit/delete 共用 issue 锁串行两个目标的 resolve/unresolve，API 接线使用 D09 协作 scope。分别以 AC86/AC88 验证新增差集、并发解决、撤权竞争和零额外执行/通知。

- [ ] 协作上传从私有 draft 开始，创建时绑定唯一 issue；正文/评论/command/run 附件归属同一对象级授权。
- [ ] Web cookie/bearer 及原生下载均有可行认证通道：fetch 带认证，Web 无 cookie 的图片/下载/range 与 Desktop 使用重验短能力代理；不 fallback 公共 uploads/旧签名/S3。
- [ ] 来源侧保存 run event、结果、用量、历史 actor 和固定附件版本；执行空间清理不删来源事实或终结屏障。
- [ ] R43 评论编辑/删除使用同单授权和作者校验，内容/版本历史/删除墓碑/事件同事务；保留 ID/回复锚点，删除后拒绝新编辑。屏蔽旧 CancelTasksByTriggerComment/retrigger 和附件清理；命令/执行不受评论修改影响。
- [ ] R57 resolve/unresolve 与评论/任务表情回应：按当前评论权限（人类）授权，沿用每线程单一解决不变式，记录真实操作者；不改评论版本历史或 command，旧回应通知对协作单跳过，不进 R52。
- [ ] R56 评论子任务 preview/create（manual 与 agent）对协作单在任何 capture/复制前拒绝，不写 source-context、不复制附件、不启动 agent；普通任务不变。
- [ ] R44 区分评论展示引用和本单交付归属：移除展示后保留 issue 关联、固定文件和历史引用；旧删除/GC 不清理已交付文件，附件/历史入口仍受当前 ACL 保护。覆盖删评论/移除单附件、其他引用、刷新/跨端及撤权后拒绝。
- [ ] 引入账号连接和明确 issue scope，复用认证但不自动加入来源 workspace room；发送前重验访问依据，撤权丢弃队列中失效 payload。
- [ ] 实现持久 outbox、snapshot/event seq 补偿、按 viewer 的失效通知；不靠当前 workspace 推测事件归属。
- [ ] R52 账号级通知：collaboration_event 同事务为当前有效参与者（除操作者）写通知意图，recipient/issue/event_seq 幂等；被移除者仅收无内容的移除通知；旧 subscriber/inbox 监听对参与者跳过，同为来源成员者不重复收。P2/P5 的流转与 AI 事件接入同一事件清单，投递失败不回滚业务。
- [ ] R55 提及：服务端只把当前参与者解析为有效提及，标记 R52 通知为“提及了你”且同事件去重；`@all` 展开为当前参与者；旧 mention/@all/@squad 监听对协作单跳过，非参与者、agent、squad 提及不通知、不订阅、不触发执行；评论编辑只通知新增提及者。
- [ ] R53 出站隔离：plugin 事件桥与 IM（含企业微信 inbox 推送）对协作单不发或只发最小引用；普通 agent/子任务执行的 prompt 与上下文组装不带入父协作单内容。覆盖 CLI `issue get`/`comment add`、plugin 订阅、IM 推送和未分类身份来源。
- [ ] 两侧检查通用 task list/snapshot/message/usage/WS，执行空间普通成员不能从 engine task 得到来源私有内容。

验证：两条 read basis 的撤权分别覆盖详情、预览、下载、旧能力兑付和订阅；无效附件/comment/run ID 不串单。无 cookie 的真实 Web bearer 会话能预览/下载/range，撤权后重取拒绝。换/删执行资源后来源历史仍可读；断线丢事件由 snapshot 恢复。HTML/原生桥接沿用安全测试；已下载副本无法收回不作承诺。

## P4. 固定启动身份、监督器与窄凭据

- [ ] 新增 collaboration purpose 与 claim/finalize/launch/progress/proof 协议；普通同空间 claim 不放宽。
- [ ] 业务 token 绑定 issue/session/command/run/grant/账户与参与 epoch；全局路由默认拒绝，子进程无宽域 mat_/PAT fallback。
- [ ] 每次业务 token 请求调用动态能力谓词，写事务再验，旧 token 不因重新参与/重开/新 grant 复活；不能借 owner 的来源成员权降级。
- [ ] 按 E07 登记中立账号 epoch 更新者；password 使用真实会话版本，legacy 共享 runtime 有 owner 授权及持久 key challenge enrollment。保持原登录模式和 password-only 管理入口，不用空版本/空 binding 兜底。
- [ ] 在可能启动前预发并持久化仅 supervisor 持有的终结 token，绑定原 deployment/install/run/fence/nonce；撤权后仍只收尾。
- [ ] 本地账本使用跨进程锁、fsync、启动前 nonce burn、单调 attempt、最终 run seal；burn/fork/PID 回写窗口保守 unknown。内部尝试先证明前次退出再取新许可。
- [ ] supervisor 覆盖每次 provider launch、内部 fallback 及全部派生进程；共享能力须证明封闭 containment，Windows Job 绑定失败不启动，Unix 单独进程组不足以准入。覆盖 setsid/改变组/双 fork/提前关闭 stdio/PID复用，无法保证则不宣告能力；观测丢失保留 unknown，不能签发完整 proof。
- [ ] daemon owner、供应商账号和 command author 独立；grant consent/revoke 使用实际资源 owner。待执行指令不预绑旧资源，切换在成为队首时使用当前授权。
- [ ] 文件通过已提交 Git/明确附件准备；新环境重置供应商恢复指针，不搬运未提交目录、私人 session 或其他平台凭据。

验证：fake executable 起子孙进程、延迟/崩溃，逐点注入 burn/fork/PID/reap/seal 故障；另一 daemon 实例、重复 nonce/派发、迟到 fallback 不重复 fork。测 Windows Job 失败、token 路由矩阵、当前 grant/私有 owner 关系和认证×runtime enrollment/challenge 重放。撤用户/runtime 后只可提交旧 proof，不读正文/启动/续租/恢复；provider 无 supervisor 凭据。不用单个 ProcessExited mock 代替原生矩阵。

## P5. FIFO、停/恢复与全部旧 writer

- [ ] 实现 command 不可变输入/作者、数据库 seq、唯一 session/未结 run 约束与 promote 原子占位。
- [ ] R45 取消单条 waiting：当前有效参与者且为作者/creator/handler；与 promote 同锁，取消状态、操作者/时间及事件同事务。不变更原文/seq、active_run、queue_state/control_version，不依赖 grant/gate；in_flight 或终态拒绝，不自动 Stop。测试三类允许主体、他人/撤权/转交、paused/维护及 cancel↔promote 的确定性竞争和幂等重放。
- [ ] 按 E01 显式连接来源正式 admission、installation accepting、当前资源/绑定/账号和全局 gate；旧恢复例外不发新许可，capacity 不改 FIFO。
- [ ] E03 无许可占位由服务端原子撤派发并结算；永久 prepare 错误、临时投递故障和可能已有许可区别处理，不等不存在的 daemon 回执。
- [ ] Stop 先持久 paused+stop intent，旧执行 proof 只清对应屏障；control version 不受普通评论/进度干扰。
- [ ] 移除/退出、撤 grant、owner 离开、生命周期 close/reopen 与相应取消/暂停/停止副作用同事务。
- [ ] 禁用作者/owner 的 auth epoch 即刻拒新许可，持久失效 fanout 不因快速恢复丢失；按顺序收敛受影响单据。
- [ ] 拒绝的 Resume key 永远重放原拒绝；不存未来恢复意图。旧 stop fence 不套到后来新 run。
- [ ] 关闭隐式失败重试/旧 merge/reconcile/continuation/rerun 旁路；明确失败且可信结束才能在 active 下继续，手动重做新建 command。
- [ ] E05 分开 business_outcome/forced_disposition/proof；proof 先到缺结果记 outcome_unconfirmed，结果先到无 proof 不解锁，停止优先于 late success。
- [ ] 按 D08 清单覆盖 HTTP/WS、Plugin、CLI、队列 SQL、后台状态/孤儿/GC、整机停止及所有 teardown；同 purpose 进入同一 primitive。
- [ ] 固定逐 writer 锁序和 NOWAIT 交汇点，所有 helper 借同一事务；冲突有限重试、commit 不明查原操作。
- [ ] E06 gate 先关再扫描暂停；Resume/所有 active 写入持 FOR SHARE 到提交并比较 expected_execution_epoch。barrier 测覆盖读 gate→提交窗口及旧请求延迟到重新 enable；扫描完成才解除，不自动 active。关闭创建仍接收尾，未结 run 不因重启/超时/engine 清理消失。

验证：使用数据库事务屏障/同步通道制造 stop↔claim/finalize/start/complete、close↔enqueue、remove↔launch、revoke↔resume、workspace delete↔create、runtime teardown↔proof 竞争，不靠任意 sleep 猜时间。指令 1 运行、2/3 排队时停，4 在 paused 加入，证明前拒恢复，同 key 之后仍拒；新请求恢复仅执行有效 2/3/4，1 不重试。至少两个调度 worker 验证一个 session 只有一个未结 run，其他 issue 不受影响。

## P6. 共享界面与 Web / Desktop 路由

- [ ] 复审补项：private schema 一律 redact:true，畸形正文/短能力不进入日志；来源成员退出重验 account 视图，保留有效发起人生命周期权并移除来源字段能力，不能误删账号组。覆盖 CL03/CL04 的暖缓存与日志 sentinel 负例。

- [ ] core 增加协作 schema/API/queries/mutations/realtime，显式 deployment/viewer/issue scope；列表 key 含 view/归一化筛选排序，账号请求去 ambient workspace header。新版本字符串无损比较，不改旧 Issue wire。
- [ ] views 组合工作台、受限单据详情、精确查找/交接、参与人、普通评论/显式 AI composer、队列、授权/换环境和生命周期界面；复用可注入数据的现有组件，避免复制完整 IssueDetail。
- [ ] 接入 R41/R42/R48–R50 字段编辑及修改记录（R49/R50 选择器仅对来源成员显示，含新建表单；迭代切换/移出要求填写原因；R51 子任务列表对来源成员可打开，对外部协作者只显示最小引用）；来源普通成员不因此获得编辑权，角色/版本/来源选项冲突保留输入并重取，双端验证保存与历史接线。
- [ ] R55 共享编辑器的 @ 选择器注入本单参与摘要，只列当前参与者、`@all`、`@AI`；关闭 current workspace 的成员/项目/任务查找。R56 同时关闭“从评论/选区创建子任务”入口。
- [ ] R57 双端按能力显示解决/取消解决与表情回应，失败回滚本地状态并保留线程位置。
- [ ] R43 双端提供本人普通评论编辑/删除、历史及删除占位；依当前评论权限显示，冲突保留输入，不复活删除记录；其他人的回复继续可定位。
- [ ] R44 评论移除附件后，本单附件/历史仍提供受控访问；清楚区分移除展示与删除文件，不让客户端隐藏整个附件记录。
- [ ] R45 队列仅对有权者显示等待项取消，提交后按服务端结果更新并保留记录；状态已变时说明需使用停止入口，不能自动扩大操作为 Stop 或重排。
- [ ] 工作台及来源任务页有明确“新建协作任务”入口/表单：来源创建权、本人/精确目标、私有 draft 上传、原子提交、同 UUID 导航及未知结果恢复；失败保留输入，无 enable/convert。
- [ ] `/account` 集中构造/校验、登录返回地址、通知/分享/评论锚点；来源旧详情使用同一协作契约。
- [ ] Web 账号 guard 独立于 workspace guard；零空间仍可访问获授权单据，创建另选来源空间。
- [ ] Desktop App 启动/恢复不让 workspaceListUnavailable 阻塞已认证账号页；workspace list 500 仅在空间选择处重试，不能强制跳首空间/新建浮层；真实认证/强制改密仍阻断。
- [ ] Desktop 扩展持久账号 tab/group，route classifier、sanitizer、Coordinator、deep link/pop-out、资源键和 memento 一起适配；保留 workspace tabs/pins/MRU。
- [ ] 账号进入前清 workspace 镜像，旧布局晚卸载不能写回；A空间→account→B空间、StrictMode、重新登录/换 Server 和零空间启动均覆盖。
- [ ] 多实体动作 await 服务器，未知结果重用原 key；转出后详情不消失、通知归档不删参与入口。
- [ ] R54 来源列表/看板/时间线用 AccountSummary 显示外部人员并加“外部协作者”筛选；看板协作卡列内拖动按 R58 正常保存，拖到负责人列不乐观移卡，按能力打开转交入口或提示，状态列拖动仅 R47 非终态，完成/取消列拒绝并提示整单动作；普通任务拖动不变。
- [ ] R47 状态选择器只向有效发起人/当前处理人提供来源非终态选项，显示修改记录；选项变化时保留选择并重取，不静默改选，也不把完成/取消混入该入口。
- [ ] R52 工作台通知列表与未读数，链接到 `/account`；打开时按当前权限重验，已读/归档只改通知状态，失权后旧通知不能打开本单。
- [ ] R46 双端提交成功后显示已加入/已转交；仅邀请在参与列表可见，有派单才新增当前待办，不展示待接受按钮；接收人重登直接恢复服务器结果。
- [ ] 挂载/聚焦、账号 WS 重新认证/重连重验权限快照；撤权推进 epoch、cancelQueries(revert:false)、清 Query/draft/upload/preview/局部 transcript，阻止迟到响应，保留仍合法的来源成员投影。
- [ ] 展示真实队列/运行/proof 三种状态，键盘可操作、焦点返回、窄屏/缩放与共享 i18n；停止确认不依赖文案猜 task.status。
- [ ] 更新 root CLAUDE 与 Desktop spec：持久账号目的地和一次性 WindowOverlay 有各自分类，Zustand 仍只存客户端状态。

验证落点：纯 schema/权限/状态/路径在 core；shared component 测组合/提交/焦点；app 测路由/持久迁移。补旧 ordinary+shared 混合列表、新字符串 9/10 与超安全整数版本、current/participating 分离、暖缓存错过撤权再重连、getMe/workbench 成功但 workspace list 500。双端点击新建并带私有附件验证同一 UUID；Electron 隔离 profile/HTTP+WS/fake daemon 透传幂等/版本头，已有 tab 更新 ?issue=，重启保留布局但不泄漏前账号数据。

## P7. 有限管理员代管

- [ ] 使用当前平台管理域完整会话/角色/组织 fence，限制 creator disabled、来源空间在管理范围和非空原因。
- [ ] 实现专用 complete/cancel/reopen，记录真实 admin、原 creator、before/after 和 request ID；ordinary lifecycle 不增加 admin bypass。
- [ ] 实现 R40：仅双禁用的 reopen 必填 successor_user_id，锁内重验已有有效参与关系/账号和 revision；其他动作或非双禁用场景拒绝夹带改派，无候选拒绝。重开/handler/待办、失效补充请求链、前后身份审计及 outbox 同事务。
- [ ] R38 管理域下提供最小候选摘要；共享表单在 Web 管理入口和 Desktop 账号壳接线，R40 明确选人、无候选禁提交、失效后刷新但不自动改选。管理员不因此查看本单全文、获得普通转交权或加入本单。
- [ ] complete/cancel 调用同一 close，reopen 保持 paused/旧屏障，不隐式恢复账号、grant 或命令。

验证：creator 启用/禁用/操作中恢复、admin 缺角色/范围/原因、PAT/observer/空间 owner 均有明确结果；真实审计 actor 与 creator 不混。使用单独 password/admin-enabled 合成环境，默认 legacy-auth E2E 不能作为 AC65–66、AC69–70 证据。覆盖 creator=handler 和不同账号均禁用、有效 handler 保留、无候选/漏选/未参与/已退出/禁用候选、其他动作夹带字段；用事务屏障控制目标禁用/退出、原账号恢复及并发责任变更的先后，失败不留半次重开或成功事件。重复 key 不重复交接，换候选复用旧 key 拒绝；Web/Desktop 验证候选表单和持久待办接线，重开仍 paused 且不复活旧 command/grant。

## P8. 集成、文档与启用门禁

- [ ] 三账号、两个独立空间、Web+Desktop 完成人类派单/补充/阶段交接/验收，以及共享执行、撤权、换环境、完成重开。
- [ ] 同单授权矩阵扩展到直链、HTTP/WS、上传/下载/预览、来源列表及执行空间列表；证明两侧无正文泄漏。
- [ ] 故障注入：丢响应/重复事件、Server/daemon 重启、旧 provider 未结、owner 禁用/解绑/删除、未结凭据与日志不完整。
- [ ] 旧 Web/Desktop/Mobile/CLI/daemon 请求对协作数据无法绕过；普通任务原行为回归。未支持 Server/运行时显示不支持，禁止普通执行或公开下载 fallback。
- [ ] 更新受影响 CLI 行为、`server/internal/service/builtin_skills/` 的 SKILL.md 与 source maps，以及必要用户说明。提交 Lore 决策记录，不纳入他人改动。
- [ ] 验证加法迁移/功能关闭/兼容回退：开关关不丢读取与停止收敛；部署无旧 writer 后才能开创建，不能带活数据回退旧二进制或删表。
- [ ] 留存 capability 矩阵、数据库竞争、原生进程树、双端 E2E 和权限负例的证据，独立 review 无未解决严重发现后才建议启用。

## 验收归属

每项 AC 由一处主验证负责；跨端 E2E 验证接线，不重复通过 DOM 枚举底层权限/状态矩阵。尚未执行的测试不能勾成通过。

| 主验证阶段 | AC | 主要证据 |
| --- | --- | --- |
| P1 数据/权限 | 01、05、13、17、45、46、60、68、71、72、73、79、80、81、82、89 | DB-backed handler/service、创建及字段编辑/角色/版本/来源选项；P6 验证双端接线 |
| P2 流转/责任 | 02、03、04、11、14、25、26、27、28、29、30、47、51、52、53、54、55、56、62、63、64、77、78 | 交接/版本竞争、直接加入/离线派单、手动改状态、持久查询与删除门禁 |
| P3 内容/实时 | 18、61、74、75、84、86、87、88 | 访问/撤权、评论版本/删除竞争、AI 无副作用及附件留存/GC；P6 双端接线 |
| P4 资源/执行协议 | 15、16、37、38、39、40、41、42、43、44、49、50 | grant/token/进程树/重放、切资源文件与身份边界 |
| P5 队列/终结 | 06、07、08、09、10、12、19、20、21、22、23、24、32、33、34、35、36、48、57、58、76 | 多 worker DB 竞争、取消等待项/幂等/当前角色、fake daemon、旧 writer 负例 |
| P6 双端体验 | 31、59、83、85 | Web/原生 Desktop 接续、转出/通知归档/重启后的持续入口；AC83 在 P2/P3/P5 事件全部接入后按完整事件清单验收 |
| P7 代管 | 65、66、69、70 | password/admin 范围、审计与双禁用原子接手；候选/账号恢复竞争及双端表单 |
| P8 原子新建集成 | 67 | P1 原子/幂等创建、P3 首次上传受控、P6 双端实际创建三项证据全部具备后验收 |

另需不依赖现有 AC 编号的工程证据：所有迁移索引并发建；schema 错误不放宽权限；unknown commit 不重复 fork；执行空间删除不抹来源历史；本地账本损坏不清屏障；低版本协议不能领取协作执行；无真实付费 AI 参与默认测试。

## 验证命令与环境

以下是未来实施时的入口，不表示本轮已执行。测试文件名为规划目标，落地后按实际文件筛选。

```bash
# SQL 变更后生成并审阅差异
make sqlc

# 按影响包做有界迭代
pnpm --filter @multica/core test collaboration
pnpm --filter @multica/views test collaboration
pnpm --filter @multica/web test account
pnpm --filter @multica/desktop test

# 实现完成后的统一检查，已包含 lint/typecheck/TS/Go race/Web E2E
make check

# 统一入口未覆盖的静态检查
pnpm knip
pnpm check:ui-exports
node apps/desktop/scripts/verify-renderer-file-access.mjs

# 需要单独复现 Web E2E 时，使用本任务拥有的生产模式环境
make up C=api,web ARGS="--web-mode production"
make status ARGS="--json"
make env-exec ARGS="-- pnpm exec playwright test e2e/account-workbench.spec.ts --workers=1 --retries=0"
```

Go handler/service 测试使用 `server/internal/testutil` fixtures 和调用断言，在本任务隔离数据库运行；必要时通过 `make test` 做阶段回归。锁测试用确定性同步，真实平台进程测试用 fake executable。默认测试不得查找或启动用户已安装的 AI CLI；真实 AI smoke 需独立明确授权且受 `agentintegration` 与环境变量双门禁，不是本计划默认验收项。

`make check` 默认 E2E 环境不覆盖 password/admin；P7 需单独具备该配置的隔离环境。原生 Electron 与 OS/provider supervisor 能力矩阵另存证据，不以 jsdom 或 Web E2E 代替。检查通过后不无理由重复全套。

## 当前规划验证

规划阶段只更新本任务资料，不运行迁移、服务或产品测试。完成文档后执行：

```bash
git diff --check -- .trellis/tasks/10-10-cross-workspace-collaboration
python3 .trellis/scripts/task.py validate .trellis/tasks/10-10-cross-workspace-collaboration
```

并核对 JSON/JSONL、R/AC 完整性、相对链接、验收归属、jsonl 引用文件各 32768 bytes、prd/design/implement 各 65536 bytes 与总自动上下文 196608 bytes 限额（`.trellis/config.yaml` 的 `context_injection.max_total_bytes`）；结果和访谈/审批状态记入 task.json 与 discussion.md，不把 planning 改成已实施。
