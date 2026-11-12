# 协作内容续作：评论、提及与账号通知

实施范围：P3 新评论、typed mention 预览与逐目标回执，R43 评论编辑/墓碑，R55 正文与评论编辑新增人员通知，以及账号通知读取/已读/归档。后续主会话增加运行端 HTTP 适配器、control 接线与 M09 真实路由/原生监督集成测试。运行端与控制服务由对应 lane 实施；附件上传、实时 transport 和完整双端 UI 不在本 lane。

当前缺口：旧评论 API 对协作单拒绝，队列只能接受单目标内部调用，通知已有持久表但没有账号读取入口。专用评论服务必须在一个事务中保存一条评论、一个来源事件、每目标 command 或拒绝回执和按接收人去重的通知；不能循环调用会自行提交的 SubmitCommand。

修改边界：新增 collaboration_comments/mentions/notifications 服务、专用 handler 与 query 文件；正文编辑仅接新增有效人员差集；通知新增来源定位字段。复用现有 comment、operation、event、notification 表，不建另一套评论存储，不扩展普通执行或机器权限。aggregate 锁遵守已有 sorted users → source workspace → issue → session → sorted targets → grants 顺序，后置资源锁由 authority lane 的借用事务准入函数处理。

验证计划：隔离 PostgreSQL fixture 验证混合提及中 A 被拒、B 入队、H 只收一次通知；重复 key/并发/中途数据库失败；来源普通成员仅评论、权限撤回与目标配置变化重验；编辑只通知新增人员且旧 command 快照不变；通知当前 ACL、离线与归档不改待办。默认不启动 AI CLI。

普通创建开关仍关闭，运行端正向能力须以 M09 原生验证结果为依据；`shared_runtime_unavailable` 不能呈现为已启动。完整验收记录在实际验证后追加。

## 最新实施依据与接线状态

用户已确认讨论 §64：M04–M10 取代此前冲突语义。内容层不自行分配执行组；通过 authority lane 的 `AppendCollaborationInputInTx` 原子追加不可变输入，取得实际根智能体通道及等待批次。M09 首先支持直接智能体，小队提交显式不可用，M08 建议未批准。

已落盘：

- `collaboration_comments.go`：来源成员或参与者的内容事务、当前人类身份及共同聚合锁、原始 payload 幂等与未知提交不重试。
- `collaboration_comments_edit.go` / `collaboration_comments_view.go`：作者编辑、墓碑、稳定回复定位、前后内容历史、精确十进制版本、分页与单评论定位读取。
- `collaboration_mentions.go`：复用 mention Markdown 解析器，UUID 归一化去重，人员与 typed AI 分开；目录使用 authority 的已授权资源检查；预览为只读一致快照。
- `collaboration_notifications.go`：原事务固定接收人、正文/评论编辑仅新增有效提及差集；账号通知分页、未读数及幂等 read/archive；撤权/移除通知不返回标题或评论/正文锚点。
- 相应 HTTP handlers/router 已接评论读取/历史/编辑/删除、提及目录/预览和通知；同时注册 authority lane 的四条目标/授权 POST。
- 651 增加通知 `source_kind`/`source_id`；652 单文件并发建立按接收人/创建时间/UUID 的翻页索引。使用已有评论、事件、operation、notification 表，无新依赖。

已执行的针对性验证（尚非整项 AC 通过）：

- `TestCollaborationMentionParser*`：观察到预期 RED 后通过。
- `TestCollaborationBodyEditNotifiesOnlyAddedHumanMentions`：先验证旧实现不发新增通知的 RED，接入后与 `TestCollaborationNotificationsLoseContentAndAnchorsWithAccess` 一起通过。
- `TestCollaborationCommentAuthorEdit*`、`TestCollaborationCommentEditsSerialize*`、`TestCollaborationCommentEditFailure*`：隔离数据库通过，含作者限制、墓碑/回复、并发版本、未知 commit、每个必要写入失败原子回滚。
- `TestCollaborationCommentPreview*`：隔离数据库通过，含混合有效/失效目标、人员接收人、自身排除、取消本次目标、零预览写入、来源成员不获执行权。
- HTTP 用例首次运行发现测试 URL helper 覆盖了 issue 参数，改用既有 `withURLParams` 后通过，后续 race 结果见续记。跨 lane M10 生成模型变化造成过暂时编译失败，未据此修改其他 lane 源码。

## 2026-10-18 验证续记

- 新评论已经通过 `AppendCollaborationInputInTx` 入队，comment、input_event、逐目标回执、批次输入及通知同事务；资源锁在 batch/command 写入前取得。`channel_id`、`batch_id` 留在回执；一条评论不自行创建执行组。
- 评论/预览协议固定为独立 `CollaborationContentProtocolVersion=2`，不跟随 execution read v3。`/control` 原样返回服务状态及各目标结果，允许 `partially_resumed` 200。
- 账号/来源普通成员、纯回复、未知 @AI 及未出现在文字中的 target_versions 都不会获得额外执行权；已加入的新成员不会在编辑旧 @all 时被重新通知。
- 主会话授权窄改 core `content-schema.ts` / 对应测试：仅 `unchanged`、HTTP 200、空 targets 接受空/缺失 event identity；真实创建/编辑/删除仍要求 UUID 与精确事件序号。观察到预期 RED 后，内容 schema/API 8 项测试通过；两文件 ESLint 通过。
- runtime list/claim/permit 在独立 mdt_-only + DaemonAuth 路由组，服务与 handler 都要求真实 managed source；mcb_ outcome 和 mct_ stop-intent/proof 独立于普通账号认证。严格 JSON/大小/用途/路径身份校验，错误也 no-store，不回退 Cookie/PAT/JWT 或输出 token。
- Go 1.26.6 下 service 内容用例通过 race（5.554s），handler 内容/运行端负例通过 race（2.406s），实际 router managed/narrow 用途测试通过 race（4.388s）；相应三包 `go vet` 通过。后加来源成员/纯回复与编辑提及差集两项 race 通过（4.096s）。日志：证据目录的 `service-content-race.log`、`handler-router-content-race.log`。
- 真实 managed challenge/enroll/bind + 目标登记/授权已在默认 router 测试经过真实认证；授权创建的 HTTP 状态为 201。测试不使用真实 AI CLI 或付费账号。
- Linux 原生测试源码已完成：`TestCollaborationSingleAgentRuntimeRoutesLinux`、`TestCollaborationRuntimeProofSurvivesOwnerDisableLinux`。第一个通过实际 `collaborationrun.Runner` 验证 comment→batch→promote/claim/permit→真实静态 fixture 进程→Stop→sealed proof→明确 Resume→含两个作者输入的下一批次及服务端 never_launched 结算；第二个在真实 sealed proof 投递前禁用 owner，要求普通 MDT 已拒绝而旧 mct_ 仍收尾。测试只延迟真实 proof 的网络投递，不编造进程证明。

Linux 正向执行由主会话在其隔离容器验证；本机 macOS 的跳过不算通过。命令要求 `CGO_ENABLED=0 MULTICA_RUN_COLLAB_RUNTIME_E2E=1`，配合 guard wrapper 和任务自有数据库。当前需等待该结果并据实际失败修正。原生 Windows、真实付费 provider、附件受控上传/下载、R57 解决/回应写入口、实时 outbox/权限推送与完整 Web/Desktop 页面仍未由本 lane 验收。

隔离数据库凭据文件（仅 source，不打印）：`/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-collab-content-20261017-zcB63m/content.env`。使用 Go 1.26.6 与 `scripts/go-test-with-agent-cli-guard.sh`；代码生成由主会话统一执行。
