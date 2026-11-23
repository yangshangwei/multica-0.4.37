# S1 源码与测试依据

2026-11-13只读研究；未改产品代码、未执行产品测试。两个已有explorer续作均因stream断开无输出，主会话已直接完成下列核查，不将其失败作为产品故障。

## 可信写入与读取

- `server/internal/service/collaboration_squad.go:251` 用 `r.run.AgentID` 创建运行时评论；`:312` 留下runtime_comment_created事实；真实run关联在 `collaboration_run_event` 的comment记录内。
- `server/pkg/db/queries/collaboration_squad.sql:32` 固定 `author_type='agent'`；不从正文解析作者。`server/pkg/db/generated/models.go:1279` 的Comment有AuthorType/AuthorID，无作者名称快照。
- `server/internal/service/collaboration_comments.go:105` 的 `readCollaborationContent` 在repeatable-read事务内ResolveIssueAccess。
- `server/internal/service/collaboration_comments_view.go:33` 定义DTO，`:59` ListComments，`:87` GetComment，`:122` 共用collaborationCommentViews。当前仅member投影Author。
- `server/internal/service/collaboration_comments_recall.go:204` 的RecallComments使用同一投影，之后按JSON编码做真实预算；新增字段不能绕过预算。
- `server/pkg/db/queries/collaboration_memory.sql:73` 已有批量ID/name查询；`server/internal/service/collaboration_memory_view.go:222`已采用从可信作者引用取当前名称、缺失时空名的模式。可复用查询，不需要新SQL或迁移。
- `server/internal/handler/collaboration_comments.go:22`清理ambient工作空间头并设置no-store；`:74`分流普通/预算分页，`:111`单条读取。以上权限与防缓存边界保留。

## 历史与执行记录的差别

- `server/pkg/db/queries/collaboration_comments.sql:52`的CommentHistory只查询comment_created/edited/deleted；`server/internal/service/collaboration_comments_view.go:210`保留人类actor摘要。
- runtime_comment_created不是上述人类历史DTO。本首包不添加事件或把Agent当人类actor；canonical已有run records精确匹配comment_id和agent_id。
- `.trellis/spec/web/frontend/collaboration-closure-qa.md:15`的人类author合同仍保留，但“无公开具体Agent摘要”的旧说明需要随实现更新；原author=null断言不删除。

## 客户端

- `packages/core/collaboration/content-schema.ts:36`定义Comment及分页，`:177`导出类型；新增展示摘要应可缺失、可丢弃畸形值，保留其余严格约束。
- `packages/core/collaboration/content-queries.ts:18`默认limit50/maxBytes65536，详情与列表均调用protectCollaborationRead；无需新增Agent详情查询。
- `packages/views/collaboration/comment-list.tsx:92`的人类编辑判断独立于显示；`:96`与`:110`目前通用Agent回退；`:111`已有长文本换行。
- `packages/views/collaboration/comment-composer.tsx:116`回复目标使用同样的通用回退；这是必须同步的第二入口。
- `packages/views/collaboration/comment-history.tsx:25`显示人类history actor，不增加来源不明的Agent猜测。
- 可复用本地化 `execution.agent / detail.unknownAuthor / commentActions.replyingTo`；人员外部标签保持原逻辑。

## canonical回归

- `e2e/collaboration-multi-account-closure.spec.ts:22`三账号/三原生测试资源；`:164`原key重放不重复命令；`:180`准确run/Agent身份；`:193`目前author=null；`:199`关联run records；`:219`撤回/再授权/显式Resume；`:235`信息往返和首轮失败；`:254`第二轮人类验收；`:307`起finally检查终结和清理。
- `server/internal/handler/collaboration_comments_test.go:30`私有投影和人类控制，`:71`外部comment拒绝，`:89`分页；`server/internal/service/collaboration_comments_test.go:143`历史/删除，`:189`竞争修改与未知提交，`:264`普通回复不派发。
- `server/internal/service/collaboration_comments_recall_test.go:23`JSON预算，`:69`分块，`:114`当前runtime authority；新增公开Agent投影应在正确层测试，不把runtime机器reader扩大到公开人类DTO。
- `packages/core/collaboration/content-schema.test.ts`、`comment-actions.test.ts`、`comment.test.ts`、`command.test.ts`分别负责解析/编辑权限/原请求/派发恢复。
- `e2e/collaboration-offline-result-notifications.spec.ts:14`已有Web/Electron独立离线通知与撤权；`collaboration-live-revoke-web.spec.ts:14`是运行中撤权；`collaboration-handoff-chain-web.spec.ts:12`是责任流转。这些是独立场景，不编造为canonical一次执行的覆盖。
- `e2e/fixtures/collaboration-native.ts`提供Electron renderer harness；精确声明不是安装版完整Main验证。

## 环境

`e2e/fixtures/collaboration-host-runtime.ts:83`要求独立DB、password API、deployment ID、production Web和renderer origin、显式测试daemon/provider/CLI绝对路径。closure使用collaboration-multi-account-provider，不与host/squad/cross-client provider协议混用。

DB隔离与配置传播按 `.trellis/spec/server/database-test-isolation.md`、`verification-environment.md`；production Web按 `.trellis/spec/web/frontend/e2e-run-environment.md`。测试运行前后记录相关源码/二进制身份，旧环境通过不冒充当前通过。
