# Research: 账号级协作工作台的 Web / Desktop 设计

> 2026-10-17：本文保留旧客户端研究证据；当前多目标 UI/候选/两级控制见[调整清单](../mention-redesign-2026-10-17.md)和 design D09/D10。仅人员选择器、单 AI 面板、@不触发等旧建议已被新稿替代，未表示产品已实现。

- Query: 如何在同一 Server 中，为不是来源工作空间成员的账号提供持久待办、持续参与入口、本单协作和唯一 AI 队列，并落实 Web / Desktop 的路由、权限、状态目录、文件、实时同步及验证边界。
- Scope: internal；依据本任务 PRD R01–R38、AC01–AC66 的客户端技术设计研究。新组件、字段和 API 均为建议契约，不代表已实现或已经批准的产品选择。
- Date: 2026-10-12。
- Active task: `.trellis/tasks/10-10-cross-workspace-collaboration`。
- 方法: 静态阅读当前工作区的 AGENTS.md、CLAUDE.md、Trellis 工作流、相关规范、PRD、既有研究与源码。没有执行测试、启动浏览器、运行真实 AI CLI、修改产品代码或操作 Git。
- 写入范围: 仅本文件。权限与运行队列的服务端实现由平行研究负责，最终命名和接口以 `design.md` 为准。首版是否转换存量普通任务正在由主会话确认，下文保留条件分支。

## Findings

### 1. 结论与边界

推荐新增账号级持久目的地 `/account`，用 `?view=current|participating&issue=<uuid>` 表达列表视角和选中的任务。复用现有 NavigationAdapter、Query、编辑器与基础展示组件，新增本单权限与控制的连接层。不要把接收人加入来源空间，不要伪造一个 Workspace 来满足旧组件，不要把通知或浏览历史当作业务待办。

Web 和 Desktop 使用同一工作台与协作详情。Desktop 需要增加账号会话分组及相应启动规则；`WindowOverlay` 的现有含义是一次性过渡流程，不能承载刷新后仍要回到的工作台。共享任务中的普通评论、旧执行重跑和状态修改也必须由服务端统一隔离旧触发路径，按钮调整不足以满足 R24、R35。

状态仍使用来源空间的七个既有 category。交接目的、当前处理责任、AI 队列门禁和运行端退出证据分别表达，不能把“交给测试”“补充完成”或“停止请求受理”写成整单完成。

### 2. 已有文件与可复用模式

以下行号是本轮静态读取的锚点；右栏区分现有行为和本次设计含义。

| 文件:行 | 已有行为 / 设计含义 |
| --- | --- |
| `packages/views/my-issues/components/my-issues-page.tsx:29`；`packages/core/issues/queries.ts:84` | “我的任务”调用 `IssueSurface`，其 my 查询键仍带 wsId；个人关系筛选不等于账号级共享权限。 |
| `packages/views/inbox/components/inbox-page.tsx:124`；`packages/core/inbox/queries.ts:5` | 收件箱正文、归档和未读按空间查询。`unreadSummary` 只是账号级各空间通知计数，不能代表谁正在负责哪张任务。 |
| `apps/web/app/[workspaceSlug]/layout.tsx:49`、`:104`；`packages/views/layout/use-dashboard-guard.ts:41` | 来源空间不在当前用户的 workspace list 中时不能挂载详情；DashboardGuard 本身要求解析出 Workspace。 |
| `packages/core/platform/auth-initializer.tsx:275`、`:296` | 身份验证成功会立即发布用户，workspace 预热另行进行；账号工作台可以沿用这项分离，无需重写认证。 |
| `packages/core/paths/paths.ts:122`；`apps/docs/content/docs/developers/conventions.zh.mdx:14` | 全局根路由使用单词或 `/{noun}/{verb}`；共享 UI 必须走集中路径构造与 NavigationAdapter。 |
| `server/internal/handler/reserved_slugs.json`；`apps/web/proxy.ts:14`、`:61` | `account` 已保留，`workbench` 未保留；根 `/inbox` 仍被旧 URL 兼容逻辑重定向至最近空间，不适合直接改作工作台。 |
| `packages/views/navigation/types.ts:1`；`apps/web/platform/navigation.tsx:78` | 已有 push、replace、back、hash、跨平台分享 URL 与可选新 tab 接口，无需共享页面直接引入框架路由。 |
| `apps/desktop/src/renderer/src/platform/navigation.tsx:49`、`:90`、`:187` | 过渡路由转浮层，跨空间路由调用 switchWorkspace，其余更新 tab store；Coordinator 才能写 MemoryRouter。 |
| `apps/desktop/src/renderer/src/stores/tab-store.ts:77`、`:131`、`:343`、`:717`、`:982` | tab 目前只属于 workspace；保留词路径被拒绝；resourceKey 只取 pathname；持久格式 v4。这些是新增账号目的地的真实改动面。 |
| `apps/desktop/src/renderer/src/components/tab-content.tsx:31`；`apps/desktop/src/renderer/src/platform/tab-coordinator.ts:169` | 同时只挂载一个 tab；memento 保存恢复视图，router 是 active session 的投影。应扩展原模型，不另建一套工作台 router。 |
| `apps/desktop/src/renderer/src/App.tsx:234`、`:305`、`:351`；`apps/desktop/src/renderer/src/components/desktop-layout.tsx:267` | 当前启动会为零空间用户开 new-workspace 浮层，自动选择第一个空间，workspace 查询失败还会接管整个窗口；账号视图需要独立通过这些门禁。 |
| `apps/desktop/src/renderer/src/components/workspace-route-layout.tsx:107`、`:163`；`apps/web/platform/admin-route.tsx:14` | Desktop 通过实例所有权避免旧布局清掉新布局的 workspace 镜像；Web 管理路由已有进入全局页面先 `setCurrentWorkspace(null, null)` 的例子。 |
| `packages/core/api/client.ts:625`、`:1062`、`:1119` | 默认 HTTP header 读取当前 slug。显式 wsId 请求要同时清掉 ambient slug；admin 请求已经有剥离空间 header 的边界。账号请求应明确无空间 header。 |
| `packages/views/issues/components/issue-detail.tsx:1162`、`:1384`、`:1837`、`:2142` | 整体 IssueDetail 会取成员、智能体、整个 issue 列表、父子任务、项目、标签和属性目录，并接入普通任务操作。不能原样挂到账号路由。 |
| `packages/views/issues/components/issue-detail-route.tsx:77`；`packages/core/paths/hooks.tsx:56` | 原详情解析 UUID/identifier 时仍要求 wsId；账号链接应使用全局唯一 issue UUID，不能只带可能在多个空间重复的编号。 |
| `packages/core/workspace/hooks.ts:65`、`:94`；`packages/views/common/actor-avatar.tsx:78` | 纯 `buildActorNameResolver` 可接显式摘要；视图 ActorAvatar 却会拉空间目录并默认链接成员/智能体详情。共享详情应使用摘要和 UI 层基础头像。 |
| `packages/views/editor/extensions/mention-suggestion.tsx:695`；`packages/views/editor/content-editor.tsx:181` | 提及查找读取当前 workspace singleton；`currentIssueId` 还会启用“从选择创建子任务”。共享编辑器必须显式约束这些能力。 |
| `packages/views/issues/components/comment-input.tsx:43`、`:143`；`packages/core/issues/mutations.ts:750` | 普通 CommentInput 有自动触发预览和 suppression 参数；现有 create-comment mutation 也依赖 useWorkspaceId。可借用提交、上传与草稿机制，不能借用执行语义。 |
| `packages/views/issues/components/execution-log-section.tsx:83`、`:344`、`:480` | 旧执行区直接 cancelTask / rerunIssue，按 task.status 分 active/past；无法表达已 cancelled 但运行端尚未确认退出的共享执行。 |
| `packages/views/common/task-transcript/transcript-button.tsx:62`、`:117` | transcript 可接外部 `items`，默认路径却会自行拉 `listTaskMessages(task.id)` 并存组件局部快照；共享模式应注入已鉴权数据并清理失效内容。 |
| `packages/core/realtime/provider.tsx:79`；`packages/core/realtime/use-realtime-sync.ts:762`；`packages/core/query-client.ts:3` | 当前 websocket 需要 wsSlug；许多失效操作读取 current wsId；Query 默认 staleTime 为 Infinity。进入无空间页面不会自动得到实时或新鲜权限。 |
| `packages/core/api/client.ts:4085`、`:4422`；`packages/views/editor/use-download-attachment.ts:114` | 上传、元数据、内容和下载仍使用旧资源入口；下载会复取元数据再交给浏览器或 Electron 原生能力。必须提供本单范围的授权契约。 |
| `packages/core/issues/stores/comment-draft-store.ts:241`；`packages/core/platform/workspace-storage.ts:94` | 评论草稿使用当前空间存储命名；没有当前 slug 时写入被丢弃。账号工作台不能直接照用，否则跨路由丢草稿。 |
| `packages/core/admin/use-admin-access.ts:17`；`packages/views/admin/users/user-action-form.tsx:39` | 已有按 server/actor 隔离的管理权限，以及原因、版本、幂等 key、结果未知和回执交互，可借鉴 R38 的窄代管操作。 |

其他需要跟随路由变更的现有文件：`packages/core/paths/tab-subject.ts`、`tab-presentation.ts`、`route-icons.ts`，`packages/views/layout/tab-presentation.tsx`，`apps/desktop/src/shared/renderer-route-context.ts`，Web / Desktop 通知 bridge、分享链接及编辑器内链接解析。只添加 page 会留下错误 tab 标题、错误资源识别和不工作的深链。

### 3. 推荐路由、导航与应用外壳

#### 3.1 最小路由契约（建议）

| URL | 含义 |
| --- | --- |
| `/account` | 个人工作台，默认“当前处理”。 |
| `/account?view=participating` | 我仍有协作身份的任务；包括已转出的任务及可读的终态历史。 |
| `/account?view=current&issue=<uuid>` | 在当前处理列表中打开一张任务；详情授权独立于其是否仍匹配列表。 |
| `/account?view=participating&issue=<uuid>#comment-<id>` | 可分享的同单/评论链接；hash 从 NavigationAdapter 读取。 |
| `/account?issue=<uuid>&attachment=<id>` | 需要全页附件预览时沿用单词根路由；校验 attachment 确实属于本单可见内容。也可先只用现有预览 Modal。 |

新增集中路径构造器和 query parser；UUID、view、attachment、hash 分别校验。只在选中任务真实改变时 push；筛选、排序和非法参数规范化用 replace，并保留有意义的其他参数。不要手拼后丢掉 `issue` 或评论锚点。

选择 `/account` 是本轮技术建议：该 slug 已保留，也没有现有 Web account page。`/account?issue=` 严格满足当前全局根路由规范，无需解释 `/account/issues/:id` 是否属于规范的允许子树，也无需新增 `workbench` 保留词。若最终选择 `/workbench`，必须检查既有 workspace slug 冲突并安排处置；不能先让路由抢占名字，再寄希望于新增保留字。

#### 3.2 Web

- 建议 `apps/web/app/account/page.tsx` 只接入共享工作台，`apps/web/platform/account-route.tsx` 管理全局路由镜像、认证返回地址和 document title。业务 guard 放共享 views；不复用要求 Workspace 的 DashboardGuard。
- 账号 route 在子查询开始前清空 workspace slug/id；旧来源布局不能在新路由已生效后重新写回镜像。借鉴已有 Web pathname 检查和 AdminRoute，不在组件 effect 中临时切成任务来源空间。
- 使用真实认证状态；账号设置、强制改密仍须先完成。不要把某个 workspace list 的 500 或空数组当作账号失效，也不能要求先创建/加入来源空间才能读共享任务。
- 对 `onboarded_at` 与零空间场景，应把账号查看/处理和创建空间的产品引导分开；来源工作空间创建权限仍由原流程管理。这是需纳入主设计的既有外壳适配，不是新增登录类型。
- 登录前复制的本单链接在认证后恢复，返回地址只接受当前部署的 `/account` 路径与已验证参数；不能通过任意 `next` 引入外部跳转。
- 来源空间成员从原 `/{slug}/issues/{id}` 打开共享任务时，接入同一协作详情和能力集。新增分享、通知和工作台链接使用账号 URL，不给外部协作者发送必经来源空间 guard 的链接。

#### 3.3 Desktop

推荐扩展现有 tab store，加入显式 `activeScope: account | workspace | none` 与一个账号 `TabGroup`，保留原 `byWorkspace` 分组。具体字段由实现定稿；关键是不使用伪造 slug，不把 `activeWorkspaceSlug === null` 继续等价为“没有任何可用目的地”。

1. route classifier 先识别允许的账号路径，再识别过渡路径与真实 workspace 路径；`sanitizeTabPath`、`navigateActiveSession` 和 `mergePersistedTabs` 同时遵守这个分类。
2. 账号页注册在 MemoryRouter 的 `:workspaceSlug` 兄弟位置，挂账号 guard/layout；新建空间、邀请和 onboarding 继续是 WindowOverlay。
3. 所有入口仍只写 tab store，由同一个 Coordinator 投影 router。适配 push、replace、新 tab、内容链接、通知、恢复持久状态及后退/前进；不从共享组件直接调用 router.navigate。
4. 首版可保持一个持久工作台 tab。当前 `resourceKeyForUrl` 只看 pathname，选中哪张任务作为这个 tab 的视图状态；**打开已存在工作台时仍要应用目标 `?issue=`**，不能照抄 switchWorkspace 在 resourceKey 命中后只激活旧 URL 的行为。选单进入该 tab 的虚拟历史，换视角/筛选保留清晰的返回行为。
5. 若后续要求一单一个 Desktop tab，应明示新的资源身份规则：仅账号任务的 issue UUID 参与 resourceKey，而不是让任意 search/hash 变成资源身份。这不是首版完成 R22 的前提。
6. memento 的滚动/选中项键要包含本次 `issueId` 或列表视角。现在许多恢复键只用 pathname，全部堆在 `/account` 会把 A 的滚动位置恢复到 B。页面挂载按稳定 issue 身份切换，不能按 revision 重挂载编辑器。
7. 账号 tab 的渲染、工具栏和工作台入口不能被 `slug && ...` 包住。账号壳使用无空间依赖的导航；共享 `AppSidebar`、GlobalShortcuts、SearchCommand、FloatingChat 只能在其各自有正确空间上下文时挂载。
8. 启动与 workspace 校验保留有效账号 tab；退出/删除/被踢出某个空间只修复该空间分组，不能删除账号分组或把仍有协作权限的本单当作 stale workspace 清掉。零空间、workspace 查询初次失败也不覆盖已有账号工作台。
9. 账号布局与 workspace 布局的镜像所有权要共同测试：A 空间 → account → B 空间、同空间 tab 互换、旧布局延迟卸载、StrictMode 双挂载、server/account 更换。
10. 持久格式应从 v4 明确升级，保留正常 workspace tab、pin、MRU、history 和 memento；新建账号组只保存布局/URL，不保存服务器任务或权限。退出登录与换 Server 要清除账号作用域的持久布局和未发送内容，不能让不同登录者读到旧任务标题。

工作台处于完整 Desktop 外壳中时复用现有拖动区。若某个受限账号页/错误页改为壳外全窗口，首个 flex 子项仍要放 DragStrip，顶部交互控件标记 no-drag。首版不需要新增一套独立 issue window；已有 pop-out 入口必须传递新的账号目标或收敛到主窗口，不能把缺少来源成员身份的账号送入旧 workspace-only issue window。

### 4. 工作台数据、查询键与 UI 连接层

#### 4.1 两个长期存在的列表

- **当前处理**：共享任务中 `current_handler_user_id` 是登录者，业务类别不属于 done/cancelled。`blocked` 的补充请求和 `in_review` 的待验收仍是需要处理的任务，不能因为不是 todo 而漏掉。
- **我参与的**：存在有效的本单协作身份，不以当前处理人或未读通知为条件。转出、归档通知、重启客户端以后仍可查；终态任务继续可读。退出/移除后不能再凭旧协作身份列入此视角。
- 来源空间成员的基础访问是另一条依据，不代表自动加入“我参与的”。其普通任务仍可由原空间入口读取。是否将所有普通任务也聚合到账号“当前处理”，是列表产品范围选择；不要通过 `involves_user_id`/评论订阅来暗中改变权限或自动转换普通任务。
- 所有计数来自相同服务端谓词。不要数当前加载页、通知条数、客户端已见过的 issue 或在若干 workspace list 上做拼接。
- 详情按 UUID 单独查询。转交成功导致任务离开 current 列表时，保留当前详情和转交结果，允许切到参与列表；不能复用 Inbox 的“找不到选中通知就改路由”逻辑。
- 无数据/加载、确实为空、有筛选但无匹配、无数据的读取失败、有缓存的刷新失败分别展示；刷新失败保留授权范围内的列表、输入和滚动。权威撤权不能作为普通刷新失败保留展示。

#### 4.2 建议模块

| 位置（拟新增/调整） | 责任 |
| --- | --- |
| `packages/core/types/collaboration.ts`、`api/collaboration-schemas.ts`、`api/client.ts` | 新 DTO、严格的关键身份/能力校验、统一传输与错误码。 |
| `packages/core/collaboration/queries.ts`、`mutations.ts`、`access.ts`、`realtime.ts` | 账号与本单缓存、命令回执、版本/权限代次、显式 server/actor/issue scope。 |
| `packages/core/collaboration/*draft*` 或对现有 draft 工具的有界扩展 | 使用 StorageAdapter 保存账号与本单草稿；复用上传协调器与安全清理模式。 |
| `packages/views/collaboration/` | WorkbenchPage、SharedIssueDetail、HandoffDialog、RecipientLookup、SharedAIQueue、RuntimeAuthorization/Rebind、LifecycleAction 等共享业务 UI。 |
| `packages/views/issues/` 与 `packages/views/editor/` 的局部提取/参数化 | 复用评论、只读内容、附件、transcript，向其传入已授权的数据与动作；禁止复制完整 IssueDetail。 |
| Web / Desktop platform、routes 和现有 tab store | 仅路由、认证返回地址、外壳、深链、存储和原生能力接线。 |

实际共享 hooks 接受显式 scope，禁止在新 account hooks 内调用 `useWorkspaceId()`。来源 workspaceId 是资源归属，不是“当前 workspace”；运行环境的 workspaceId 又是第三个独立值。它们必须在类型与字段命名中可区分。

#### 4.3 Query / draft 身份（建议）

```text
accountWorkbenchKeys.list(serverScope, viewerId, normalizedFilters)
accountWorkbenchKeys.counts(serverScope, viewerId)
sharedIssueKeys.detail(serverScope, viewerId, issueUUID)
sharedIssueKeys.timeline(serverScope, viewerId, issueUUID)
sharedIssueKeys.queue(serverScope, viewerId, issueUUID, sessionId)
sharedIssueKeys.runMessages(serverScope, viewerId, issueUUID, runId)
sharedIssueKeys.attachments(serverScope, viewerId, issueUUID)
```

这些是账号/单据作用域键；原有空间查询仍保留 wsId。不要把 narrow 共享摘要写成整个 `workspaceKeys.members(sourceWsId)` 或整空间状态目录，避免其他组件把不完整/不应可见的数据当成权威目录。

草稿键至少包含 Server、账号、issue UUID、comment/AI 模式与 thread parent。新建任务的草稿还需独立 draft ID。现有 workspace-aware storage 在无 slug 时会丢写，不能在账号页靠临时设置来源 slug 解决。评论切成 AI 模式时仍要显示该模式；不同任务的新 composer 默认普通评论，不能继承上一张任务的 AI 发送模式。

只持久化用户偏好、未发送草稿、上传恢复信息，以及需要恢复的不确定命令 key/原始载荷；不把共享任务、角色、队列或 grant 镜像到 Zustand。账号草稿不能粗暴登记为现有 cleanup registry 的 `workspaceScoped:false`：该类全局条目会在任意空间 leave/delete 时清理。应有明确的账号会话 cleanup，参考现有 admin control draft 规范。

### 5. API 最小契约（全部为建议，待主设计统一）

API 路径不是前端路由。以下用 `/api/collaboration` 表示新增的本单用途入口；不能只把旧 workspace middleware 从一组普通资源端点拿掉。

| 请求 | 输入 / 输出与必须保证的事实 |
| --- | --- |
| `GET /api/me/workbench?view=...&cursor=...` | 登录账号服务端确定；分页摘要、准确计数/has_more、稳定顺序、来源空间最小展示。不得以 URL user_id 代替当前认证主体。 |
| `POST /api/collaboration/recipients/resolve` | `kind: username|email|user_id` 与完整 value，另带当前 issue 或合法创建上下文；最多返回一个可确认账号或无匹配。只查找，不新增协作者。 |
| 新建 `POST /api/issues` 的显式 collaboration 请求分支 | 来源空间创建权限仍按原入口；创建唯一 issue、发起人、处理人、初始参与关系与共享会话须原子完成。响应提供同一 issue UUID。不可先创建普通 issue 后由客户端无保护地补第二次启用。 |
| `GET /api/collaboration/issues/:issueId` | 本单摘要/详情、来源状态展示、真实发起人和处理人、参与摘要、viewer 能力、共享 session 概况。非成员不借此得到整个来源空间。 |
| `GET .../timeline`、`.../runs`、`.../runs/:runId/messages` | 本单专属历史；执行通过明确的 source-issue/run 关联鉴权，不假设 engine task 的旧 issue_id 或运行空间就是来源空间。 |
| `POST .../handoffs` | `target_user_id`、purpose、说明、`expected_revision`；改变责任与必要协作关系的同一事务结果，返回交接记录 ID、最新责任和权限。 |
| `POST .../information-returns` | `information_request_id` 与 expected_revision；收件人来自该次请求的 return_to，客户端不能自由覆盖为另一个账号。 |
| `POST .../acceptance-submissions` | 当前处理人明确提交给原发起人；返回责任和交接记录，不完成整单。 |
| `POST .../collaborators`、`.../collaborators/:userId/remove`、`.../leave` | 明确身份管理；服务端同时处理 R15/R16/R31/R32 副作用，响应反映最新身份和队列状态。 |
| `POST .../comments` 或共享任务专用的原 comments 分支 | 永远是普通评论；reply、@AI、编辑或旧 suppress 参数均不能变成执行许可。返回经过 schema 校验的真实评论。 |
| `POST .../commands` | 显式 AI 指令，固定 body/附件快照、真实 author、幂等 key；返回 command ID、session ID、严格 seq 和当前等待原因。 |
| `POST .../session/stop`、`.../session/resume` | 请求操作版本/当前 run 引用与幂等 key；停止受理、停止确认和未确认是不同结果。拒绝的 resume 不登记未来恢复意愿。 |
| `POST .../runtime-authorizations`、`.../runtime-authorizations/:grantId/revoke`、`.../runtime-binding` | grant 由实际环境 owner 授予/撤回；绑定切换由发起人/当前处理人执行，服务端验证旧执行结束、paused、新 grant。 |
| `POST .../lifecycle` | complete/cancel/reopen 语义动作、来源 status key、expected_revision；权限和 R35 收尾同一业务操作。 |
| 管理域中的独立 lifecycle takeover 请求 | 限 R38 三个动作；真实管理员、平台管理范围、发起人 disabled、原因、版本/幂等与审计。不与普通 collaborator 能力合并。 |
| `GET .../operations/:requestId` | 本人/本单授权范围内的持久结果查询；请求 key、动作和目标必须匹配，避免网络超时后生成另一条指令或转交。 |

必要 DTO 内容，不要求后端逐字采用以下字段名：

```text
SharedIssueDetail
  issue: { id, source_workspace_id, identifier, title, status,
           status_category, status_name, revision, ...本单允许展示字段 }
  source_workspace: { id, slug, name }     # 摘要不是成员身份
  originator: AccountSummary               # 不可变的整单发起人
  current_handler: AccountSummary
  participants: ParticipantSummary[]      # 当前身份与历史 actor 分开
  current_information_request: { id, requester_user_id, ... } | null
  latest_handoff: { id, purpose, from, to, note, occurred_at } | null
  viewer: { access_basis[], capabilities, permission_epoch }
  status_presentation / action_status_options: 来源目录的本单投影
  session: { id, queue_state, control_version, active_run, resource_binding }
  contract_version

SharedQueue
  session_id, queue_state, control_version, issue_epoch, resource_epoch
  commands: { id, seq, author, state, submitted_at, reason, run_id? }[]
  active_run: { id, command_id, engine_task_id?, runtime_snapshot,
                proof_state, progress_state, stop_operation? } | null
```

session 的 active/paused/closed、未结 run 屏障及 command/run 状态建议与同目录 [共享队列研究](shared-queue-runtime-design.md) 对齐。`AgentTaskSchema` 中某些缺失字段会默认 cancelled，这种普通列表兼容行为不能用来证明共享执行已经退出。

传输与解析要求：

- 账号及本单接口明确剥离 `X-Workspace-ID`、`X-Workspace-Slug`；请求不能在异步回调中重新读取用户后来切换到的 workspace。原显式空间请求使用 `workspaceRequestInit` 时仍同时清 ambient slug。
- 使用 `parseWithFallback`；关键身份、session/command 关联、能力版本、序号不合法时返回 typed unavailable/error，而不是回退为空工作台、允许写入或已停止。可选显示字段单独降级，未知 enum 显示未知并停用相关动作。
- capabilities 只有显式 `true` 才可用，并结合可识别模式、有效目标、业务状态及版本；其作用是准确呈现 affordance，最终权限永远由服务器重新校验。
- `username` 在现有 UserSchema 可缺失，不能把 display name 当完整用户名，也不能从 MemberWithUser.id 误取账号身份：后者还有独立的 user_id（`schemas.ts:3816`）。查找归一化沿用服务端账号规则，前端只做必要 trim/格式反馈。
- 所有多实体动作 await 服务端；不要乐观移除 current 列表、移除成员、改 handler 或解除 paused。确定成功后采用返回 revision，再失效列表/计数等不确定投影。
- 网络结果未知保留同一幂等 key 与原载荷，先查回执或按协议重发相同请求。409/目标失效要求重新阅读现状并明确确认；不能拿旧输入配上新 revision 自动重做。
- 查找邮箱/用户名不写入全局目录缓存或长期最近搜索；日志不能输出完整私有请求/正文。需要记录时优先 request ID、结果码和稳定账号 ID。

### 6. 权限与界面矩阵

下表描述已确认的人类身份规则。AI 栏还受业务生命周期、按单资源授权、队列/旧执行状态门禁约束；并不意味着看到按钮就能跳过这些条件。

| 当前身份 | 查看 / 普通评论 | 发指令、停止、明确恢复 | 邀请 / 普通转交 | 移除他人 | 完成 / 取消 / 重开 | 主动退出 | 更换运行环境 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 发起人 | 是 | 是，按门禁 | 是 | 是；不能移除自己 | 是 | 否 | 是，按 R26 |
| 非发起人的当前处理人 | 是 | 是，按门禁 | 是 | 否 | 否 | 先转交后才可退出 | 是，按 R26 |
| 其他有效协作者 / 原处理人 | 是 | 是，按门禁 | 否，即使目标已经参与 | 否 | 否 | 是 | 否 |
| 来源空间普通成员，未加入本单 | 是 | 否 | 否 | 否 | 否 | 无本单身份可退出 | 否 |
| 已退出/被移除但仍是来源空间成员 | 是，通过成员依据 | 否 | 否 | 否 | 否 | 已无本单身份 | 否 |
| 两条访问依据都失效的账号 | 否 | 否 | 否 | 否 | 否 | 不适用 | 否 |
| 仅有来源空间 owner/admin 身份 | 按来源空间基础访问 | 不因此取得 | 不因此取得 | 不因此取得 | 不因此取得 | 不适用 | 不因此取得 |
| 符合 R38 的平台管理员 | 不自动取得协作内容读取权 | 不自动取得 | 不取得 | 不取得 | 仅明确代管这三项 | 不适用 | 不取得 |

补充规则：

- 运行环境 owner 的“授权/撤回本单使用”与上表身份正交；拥有运行环境并不自动成为当前处理人或整单管理者。批准的是本单现有及以后合法加入者，不逐人重复征求资源授权。
- `information-return` 只在当前处理人承接有效补充请求时显示；`acceptance-submission` 是当前处理人的动作。普通评论永远不会代替它们。
- 正在转交或移除时按钮可 pending，但角色和 current 待办只在服务端成功后改变。另一个客户端已经改了责任，旧弹窗必须明确显示冲突，不继续使用进入弹窗时的权限。
- 发起人不是当前处理人时仍保有其独立权限；当前处理人变更不能顺带改变 originator，也不能把旧 assignee agent 当作人的业务身份。
- 终态仍显示可读历史和真实未结执行；新 AI 指令与恢复必须禁用。若旧执行仍待收尾，停止/查看回执不应因整单已结束而消失。
- 能力缺失、查询失败、未知角色不是“默认允许”。UI 使用集中 Decision/原因码呈现权限解释，不让各组件分别从空间角色推断。

### 7. 人类交接与精确查找的边缘流程

| 情况 | 建议界面与请求行为 |
| --- | --- |
| 查找接收人 | 选择用户名/邮箱/账号 ID，输入完整值后显式查找。空输入、姓名片段不列目录；不挂现有成员模糊 picker 或全站 typeahead。 |
| 查找命中 | 展示姓名、精确匹配标识、稳定账号 ID 的可核对部分；确认之前不邀请、不转交。修改查询后清掉旧命中，延迟返回不能重新选回旧人。 |
| 查找账号后来被禁用 | 最终提交重新校验；保留交接说明，提示目标不可接收，重新查找。不能相信刚才 lookup 的成功。 |
| 转给既有协作者 | 同样校验当前操作者是发起人/当前处理人；不能因“已经加入”而放开 transfer。 |
| 请求补充信息 | 显示本次请求人、接收人、说明和“补充完成将交回谁”；持久 request ID/return_to，不从上一条评论作者或“最初产品”推算。 |
| 多次/嵌套交接 | 服务端明示当前有效补充请求；较早 return ID 已被消费或替代时拒绝。若采用补充请求链，每条父子关系也由服务器明确返回，客户端不猜。 |
| 交回目标已退出/被移除/禁用 | 阻止该定向交回并解释原因；保留补充内容。只能由当时有权限者重新选择正常转交，不自动重新邀请旧目标。 |
| 提交验收 | 确认摘要明确写原发起人是接收人；没有“测试通过就完成”的联动。原处理人仍能从参与列表进入。 |
| 发起人禁用导致验收目标不可用 | 展示账号不可接收与有限代管渠道；不擅自把管理员变为发起人或普通接收人。R38 只允许完成/取消/重开，不允许额外代转交。 |
| 发起人移除当前处理人 | 成功回执同时显示权限撤销、handler 回到发起人、该人未执行指令取消、需要时停止/挂起与资源撤回；任何部分失败都不能仅在 UI 做另一半。 |
| 主动退出 | 当前处理人显示先转交的原因；发起人不提供退出。非当前处理人可明确退出，展示本人指令/资源会发生的实际处置。 |
| 两个客户端同时处理 | expected_revision 和 request ID 决定结果；旧请求不能覆盖后来的 handler，也不能重新激活旧补充关系。 |

“说明”可以是交接事件内容/关联评论，但交接的语义记录独立存在；删除或编辑一条评论不能抹掉实际责任变更。

### 8. 普通评论、共享队列与真实停止状态

#### 8.1 Composer

- 新 composer 默认“发表评论”；“发给 AI”是可见的显式选择，发送按钮及辅助文字随模式变化。回复 AI、输入 `@AI` 或选择提及项本身不切换模式。
- 普通评论模式不挂自动触发 chips、自动触发 preview 或旧的执行确认 gate。旧客户端发来的 comment 也由服务端识别共享任务并禁止自动执行；不能只补 `suppress_agent_ids`，因为它不代表拒绝一切未知/后续触发器。
- 复用 `useComposerSubmit` 的防重、上传 gate、仅清理本次提交草稿等机制。上传中不可发送；失败保留正文、附件与显式模式；成功只清提交时的那份草稿，不能擦掉另一 tab 或重新打开后编辑的新内容。
- AI 入队前显示 pending；收到服务端回执后才能显示稳定 command seq、实际作者和“等待上一条结束”/“等待恢复”。不能把 accepted 或 queued 画成已送达正在运行的模型。
- 已入队指令使用不可变正文/附件快照。后来改评论不改指令。需要再做相同内容时，是当前有权限者明确提交的新 command；不会复活原 stopped/cancelled 记录。
- 现有 ContentEditor 可关闭新提及建议并保留已有节点；若首版提供参与者提及，注入本单参与摘要和受限查找，不使用 current workspace 的成员/项目/任务目录。`@all` 不扩大共享对象，mention 链接本身不授予访问。
- 从本单详情看到的父任务、子任务、项目、引用链接各自鉴权。共享本单不自动共享全部父子节点或项目。无独立权限时显示不可打开的引用或服务端给出的最小摘要，不主动请求整个资源目录。

#### 8.2 会话和执行状态展示

以平行队列研究的 queue_state、active_run 屏障、progress/proof 与 operation receipt 为权威。下面是 UI 结果矩阵，不新增 issue 状态类别。

| 权威事实 | 可见文字 / 动作 |
| --- | --- |
| active，无未结 run，队列为空 | AI 空闲；显式发给 AI 后等待服务端入队/领取结果。 |
| active，当前 run 进行中 | 显示当前指令、作者、执行环境；后续指令按 seq 排队。 |
| paused，新指令被允许接收 | 显示“已入队，等待恢复”；新指令不会把 paused 改 active。是否可接收由当前 can_enqueue 与资源准入结果确定。 |
| Stop 请求返回 accepted | “停止请求已受理，后续队列已挂起；等待运行端确认”。记录操作者，不能显示“执行已结束”。 |
| stop unconfirmed / 运行端失联 | 保留旧 run 与未确认原因；恢复禁用/服务端拒绝；没有“等待恢复请求稍后自动生效”开关。 |
| 旧 run 随后确认结束 | 显示实际结束事实；会话仍 paused。之前被拒绝的恢复不能重放为成功，用户需要新一次明确恢复。 |
| grant 已撤回 | 清楚区分“授权已失效”与“旧执行仍待确认”；不允许新执行或恢复。普通评论不受本项影响。 |
| 重新授权 / 手动切换成功 | 仍 paused；明确告知“可恢复”而不是“已恢复”。 |
| done/cancelled | 业务终态、所有剩余指令取消原因仍可查；旧 run 未确认时仍显示收尾状态；发指令和恢复禁用。 |
| 重开 | 新业务代次可接受合法新工作；保留旧停止屏障、旧取消记录与 paused，明确恢复后才可执行。 |
| 读取失败 / 未识别版本 | 不把缺失状态默认成空闲、已停止或可恢复；提供重试与当前不可确认的说明。 |

复用 transcript 的 buildTimeline、RunTimeline 和展示组件，但数据由本单授权的 run-message query 提供。历史实际执行者、指令提交者和环境 owner 是不同身份；不因供应商账号相同而把所有指令都显示成 owner 提交。

`ExecutionLogSection` 的默认 cancel/rerun、私人 ChatPage/ChatQueue 的会话选择，以及普通 issue 的 Run 按钮均不能作为共享控制实现。可以提取展示部分，控制入口只调用同一共享 session。停止与挂起不会停止别的 issue 或私人聊天。

### 9. 个人运行环境授权与手动重绑

建议用同一共享组件分成可独立恢复的三个步骤；不是三次自动执行。

1. **资源授权**：选择本账号可提供的环境，owner 明确确认“供本单现有及以后合法加入的协作者使用”。显示任务、环境、owner 和供应商账号沿用事实；不返回或复制供应商凭据，不授权其他 issue/私人 session。不是 owner 的发起人/处理人只能请求授权，不能代为勾选。
2. **准备与切换**：当前发起人/处理人先明确挂起/停止，等待旧 active_run 结束证据；目标 grant 有效后提交带旧 binding/control version 的切换。准备信息可记录 Git 仓库/commit 与明确交付附件。普通转交不会进入这一流程。
3. **明确恢复**：切换提交只更新 binding/resource epoch；queue/session/history 和剩余 seq 保留。仍须有权协作者再点击恢复，从下一条有效指令开始。

可提供资源的选择器使用 owner 自己的资源范围或本单已授权资源投影，不为所有协作者列出某个空间的机器、智能体、profiles 与环境变量。运行空间与 source workspace 分别显示，关联方案按 [共享队列研究](shared-queue-runtime-design.md) 的明确执行关联处理。

旧运行环境有未提交改动时只说明未自动交付；不会上传目录、搬运工作树、复制未提交文件或把“准备文件”当作执行命令。Git 权限不由共享 issue 自动获得。

owner 退出/被移除时 UI 要显示 grant 同时撤回，即使当前 command 作者是别人也请求停止；其他作者的等待指令保留。重新加入不复活 grant。owner 单独撤回资源也不删除协作者身份。

### 10. 已确认业务动作与来源状态目录的映射建议（O02）

**现有契约**：七个 category 为 backlog、todo、in_progress、in_review、done、blocked、cancelled；自定义 key 继承其 category 的全部行为。见 `server/internal/issuestatus/issuestatus.go:3`、`packages/core/types/issue.ts:5`、`packages/core/issues/status-category.ts:20`。不要新增 waiting_information、awaiting_acceptance 等平行生命周期 enum 来替代这一体系；它们可以是交接 purpose/展示文案。

以下是供主设计定稿的默认映射，不把它写成已经由用户确认的新需求：

| 业务动作 | 推荐业务状态规则 | AI 会话 |
| --- | --- | --- |
| 新建并派给账号 | 使用创建请求明确选择的来源状态；首版派单默认 todo 是合理建议。 | 尚未授权/执行的状态由 session 控制，不由 todo 推断。 |
| 普通转交 / 交付阶段结果给下一人 | 保留当前非终态 status key；用 handoff purpose 表达接续处理/测试，不自动完成。 | 完全保留原状态。 |
| 请求补充信息 | 默认映射来源 blocked；保存此次 request 的 previous_status 与 requester。 | 完全保留原状态。 |
| 补充完成并交回 | 优先恢复该请求保存的、仍有效且可选的非终态来源 key；不能恢复时明确采用来源 canonical in_progress，并在确认中展示。 | 完全保留原状态。 |
| 提交验收并转交 | 来源 in_review；已是该 category 的有效自定义 key 可保留。 | 完全保留原状态。 |
| 发起人完成 | 来源 done category 的有效目标 key；触发 R35 原子收尾。 | closed，取消所有未执行项，旧 run 请求停止。 |
| 发起人取消 | 来源 cancelled category 的有效目标 key；触发 R35 原子收尾。 | 同上。 |
| 发起人重开 / 合格管理员代为重开 | 目标必须是来源非终态 key；默认 todo 的选择由最终设计确认。 | 不复活旧指令，不清旧运行屏障，仍等待明确恢复。 |

状态解析/选择的具体约束：

- 列表使用每行 source workspace 的 status_category/status_name/必要颜色摘要；同名 key 在 A 为 blocked、在 B 为 done 时，不能用当前空间 B 的目录解释 A 的任务。
- `useIssueStatuses(sourceWsId)` 虽然接受显式 wsId，但当前 queryFn 仍调用依赖 ambient header 的 `api.listIssueStatuses(true)`（`issue-statuses/queries.ts:21`）。仅把参数改成 sourceWsId 不足以修复路由和权限。
- 非来源成员不请求整空间状态目录。详情提供本单当前状态展示与该操作者每个已批准动作允许的来源状态选项；服务端再次用实际来源目录校验。可复用纯 `buildIssueStatusCatalog` 的解释方法，但不伪造全目录缓存。
- 历史可显示 archived 状态，新的选择不能分配 archived key。补充请求期间原状态被归档、来源目录更新或目标 key 消失时，确认 UI 显示实际替代，不静默套用另一空间的同名 key。
- `statusCategoryOfKey` 的未知 key → todo 是显示兜底，不是执行或生命周期权限判定。未知 category 必须无法启动/恢复，UI 也不能把它当已知待办空闲。
- 终态判断覆盖所有 done/cancelled 自定义状态；普通状态 picker、表格批改、快捷键、CLI、自动化或 AI 写 issue status 均不能绕过发起人/R38 权限和 R35。
- 上述人类交接导致 blocked/in_review 等变化时，共享 issue 的服务端分支不得运行旧 status-change auto dispatch。它们不能违反 R11、R24，也不能由旧自动化/失败恢复随意覆盖交接责任与状态。

### 11. 实时、失效权限与附件

#### 11.1 账号与本单实时范围

当前 WSProvider 在 `!wsSlug` 时不连接，进入 account 必须补账号订阅。建议保留已有 workspace websocket，增加同认证会话的 account subscription/有限 account connection，在账号壳打开的当前 issue 上订阅本单事件。不要按所有来源 workspace 建连接，也不要扩大来源空间广播受众。

- 账号事件用于 current/participating 列表、准确计数和权限变化；本单事件用于已授权讨论、queue、run 和 receipt。
- 事件携带 issue/source 身份、revision 或 seq、permission/session/resource epoch、真实 actor；新 updater 按事件的显式 scope 更新 Query，不调用 `getCurrentWsId()` 猜目标。
- 复用单个调度入口和 revision-aware patch/invalidations。迟到 HTTP、重复 WS、自事件、不同客户端乱序都不能把 handler、队列和权限退回旧值。
- 连接认证仍用现有 cookie 或第一帧 token 方式，不将 bearer token 放到 URL。认证成功/reconnect 后读当前权限与 session snapshot，再合并增量；断线、刷新或重新打开 tab 从来不是 resume。
- 新 queries 显式设置合适的挂载/焦点重校验与断线恢复；不能依赖默认 Infinity 下自然过期。实时可失联，业务待办必须始终能通过持久查询恢复。
- 命中明确撤权时先推进本地 access epoch，取消受保护查询（注意 cancelQueries 的 revert 行为），清 Query、mutation payload、draft/上传、预览 iframe 与 transcript 局部快照，关闭不再有权的弹窗；迟到成功不能重新填回内容。
- 若仅失去 collaborator 身份但保留 source-member 基础访问，重新取得这种基础投影，保留合法阅读/普通评论，并撤销全部旧 AI/管理能力。不能误清用户的 workspace membership 或整个登录状态。
- 上述模式可借鉴 `.trellis/spec/core/frontend/iteration-operations.md` 中 permission epoch、`revert:false`、请求 scope 与未知操作恢复约束，但不要把 I1 的资源权限函数直接当本单权限。

#### 11.2 附件与记录范围

共享详情中的描述、评论、指令快照、执行结果附件均以本单归属和两条访问依据授权。图片/PDF 内嵌、HTML/text 预览、元数据、原始字节、下载、上传与最终绑定使用相同权限规则。任意 attachment_id 或 engine task ID 都不能跨过归属校验。

客户端可复用 AttachmentDownloadProvider、文件卡片、上传 gate、HTML sandbox 和 Electron 原生 download bridge；必须向其提供带明确 issue scope 的 API/loader，而不是继续从当前 workspace 注入授权。新建任务尚未存在时的附件需要私有草稿/明确协作用途上传，不能先产生不受控的公开 URL 再在创建后补权限。

`api.getAttachment` 注释当前签名 URL 有 30 分钟有效期（`client.ts:4426`）；Web 还会把签名能力交给原生导航，Desktop 会交给 native download。刷新元数据和清本地缓存不能撤销已签发 URL。服务端需明确选择可按身份/permission epoch 校验的本单文件代理或可撤销能力，并覆盖非代理的 CDN/S3 路径。已下载到本地的字节无法远程收回，不应把 R18/R61 描述为这类承诺；后续服务器访问必须能失效。

如果实现要求共享附件受即时撤权保护，不能拿旧的可公开或长效链接作为共享接口的兼容 fallback。未知/旧协议应给出不支持或不可读取状态，不能降级为裸公开下载。

安全展示延续 `sandbox="allow-scripts"`、不加 allow-same-origin 和现有 Renderer 文件访问防线；本任务的附件机制不能使不可信 HTML 获得本机目录读取能力。

协作者退出、移除、禁用后保留服务端正文、actor 历史与附件；清理的是其访问能力和客户端缓存，不是全体成员的记录。历史 actor 用快照和当前可披露状态展示，不依赖对方仍在来源成员列表中。

### 12. 新建、条件转换、旧客户端与生命周期例外

#### 12.1 新建路径必须可发现

- 在有创建权限的来源空间任务入口提供明确“新建协作任务”，工作台空状态也提供入口并让用户选择自己有权创建的来源空间。UI 说明会共享本单记录，接收人经精确查找后明确确认。
- 新建请求选择人的处理责任；agent/runtime 是单独的执行资源，不把 assignee_type=agent 的普通派发当跨账号 handoff。
- 唯一业务 issue 与协作记录从创建起一致；没完成创建的普通评论草稿、查找结果与临时上传都不能算已发出派单。
- 未选择转换前，既有普通任务继续原语义。不能为了让新入口显得完整而自动批量转换、把评论参与者一律加入、或静默复制成另一张共享 issue。

#### 12.2 存量转换是条件路径，尚未定为首版必需

若用户选择同时支持已有普通 issue 转换，需要在原详情提供明确的“开启跨账号协作”入口及服务端预检/原子启用，不能只有新建入口。转换必须保留 issue UUID、原有编号和本单历史，不能复制另一张来完成同单流转的要求。

转换预检至少包含：

1. 明确 human originator。现有 creator_type 可以是 member/agent/squad；不能自动把 agent UUID、环境 owner 或点击按钮的人改称原发起人。不是可确定人类发起人的旧单需要明确规则或首版拒绝转换。
2. 初始 handler/协作者以明确的人类账号为准，不自动将所有成员、订阅者、旧评论作者或曾被 @ 的智能体变成协作者。
3. 保留本单已有讨论和 issue-exclusive 执行记录，但不暴露引用的私人 chat session 整段聊天。历史执行关联与作者摘要由服务端裁剪返回。
4. 已运行、已派发、旧 pending 合并、自动重试、offline daemon 和旧 MAT/lease 必须先有明确迁移/fence 方案。仅 task.status 看起来终态不足以证明可以创建新的共享 session。
5. 所有既有附件的公开路径、旧签名能力及内容归属完成安全迁移/隔离；检查前后不能出现“已共享”成功 UI 但旧路径仍可旁路的窗口。
6. 预检结果变化时重新验证；启用失败不留下半张 shared issue。转换后的任何旧评论、rerun、assign/status-trigger 入口也立即受共享门禁约束。

若首版最终只支持新建，以上作为后续迁移工作，不伪装为当前必须实现的验收；普通详情不给一个看似已支持的跨账号 transfer。主会话的产品选择要同步 PRD/design/实施清单。

#### 12.3 兼容及上线顺序

- Server 先提供完整 per-issue read/write/download/realtime 与队列门禁，再启用 Web/Desktop 创建/加入入口；安装版 Desktop 不能依靠前端 flag 防旧写请求旁路。
- 通过 contract_version 与可解析的最小 DTO/能力声明辨认支持状态；未支持服务器上工作台显示明确不支持，不把 404 当空列表，更不能回退为给来源空间加成员。
- 现有 IssueSchema 可增加独立的协作标记/最小展示，必须处理未知 mode、缺失/损坏的关键权限数据。不能在 shared route 因字段缺失而回到普通可运行/可删除的 IssueDetail。
- 原 workspace 普通任务继续既有行为；共享任务的旧入口要么进入同一 service，要么明确拒绝。Mobile 不交付新工作台，但旧 Mobile/CLI/daemon 请求也不能绕过 shared 规则。
- 深链与通知生成器统一生成账号 URL。若需支持已经传播的 source-space 深链，解析必须在“不渲染整空间”的受限恢复路径中进行，只解析这一张获授权 shared issue 后跳到账号 URL。不要让外部协作者通过一个假的 WorkspaceProvider 渲染来源 dashboard。
- 回滚客户端功能时仍保留服务器对已存在共享任务的授权与执行防线；不能因隐藏菜单把实体重新解释为普通任务，或让旧 runtime 再领取共享指令。

#### 12.4 完成、取消、删除空间与有限代管

完成/取消成功可以立即显示业务终态，同时明确旧运行仍在等待停止确认。重开不能复活命令、自动恢复 grant、隐式恢复队列或修改历史作者。被禁用的 handler 等例外不由管理员代管自动修复，R38 不授予转交或更换资源的额外能力。

来源空间删除沿用 `useDeleteWorkspace` 先 await 后 cleanup 的模式（`workspace/mutations.ts:53`），服务器返回“仍有共享任务记录”时保留对话框、空间、tab、数据与草稿，给出任务记录仍需保留的原因；不能通过过滤掉 done/cancelled 后再判断可以删除。普通空间删除的原行为仍应回归测试。

本任务没有授权新增共享任务物理删除/迁移产品流程。新 shared UI 不提供能绕过历史保留和来源空间删除限制的无条件 DeleteIssue；既有 delete 入口需由服务端显式策略处理。不要把“归档通知/隐藏列表项”展示为删除共享任务。

R38 可以复用管理域能力探测、原因输入、版本和持久回执模式，新增**仅完成/取消/重开**的共享 dialog。Web 可以在现有管理任务入口使用；Desktop 在账号外壳提供同一窄操作入口/目标查询，不必为了三种例外复制整个平台管理后台。非协作者管理员只读完成代管所需的最小目标元数据，不因此打开本单讨论、AI 日志或资源目录。请求进行中发起人恢复、管理范围改变、原因缺失都要拒绝，审计显示真实管理员和不变的原发起人。

### 13. 实现后的验证设计与实际仓库命令

本节是执行阶段检查计划，本轮没有运行这些测试。

#### 13.1 测试落点

| 层 | 推荐测试位置与重点 |
| --- | --- |
| Core 纯规则 | `packages/core/collaboration/*.test.ts`：query/filter scope、权限/动作矩阵、来源状态映射、未知 enum、UUID/账号身份、queue 展示推导；无 DOM 文件用 node 环境。 |
| API 边界 | `packages/core/api/*collaboration*.test.ts`：所有新 endpoint 的 malformed 2xx、缺关键字段、null collections、未知模式、header 无 ambient ws、过期 auth/endpoint、错误码和幂等回执。 |
| Core hooks / 数据流 | `packages/core/collaboration/*.test.tsx`：真实 QueryClient、迟到响应、恢复回执、server/actor/issue epoch、防乐观假成功、撤权取消与 draft/上传清理、WS 乱序/重连。 |
| 共享界面 | `packages/views/collaboration/*.test.tsx`：明确评论/AI 模式、精确查找确认、交接与回交、条件按钮、真实停止 banner、读失败/重试、权限变化、键盘/焦点、长文和附件保留。通过 NavigationProvider 测试，不 mock next 或 react-router-dom。 |
| Web 接线 | `apps/web/platform/account-route.test.tsx`、account page/layout tests、`apps/web/proxy.test.ts`：认证返回地址、无空间路由、slug 清理、查询参数/hash、旧 root inbox 兼容不被破坏。 |
| Desktop 接线 | 扩展 `stores/tab-store.test.ts`、`platform/navigation.test.tsx`、`platform/tab-coordinator.test.ts`、workspace/desktop-layout gate suites、tab presentation/diagnostic suites：v4 导入、账号组恢复、单 router、选单 query 更新、MRU/pin、旧布局清理、零空间和失败恢复。 |
| 既有行为回归 | 现有 my-issues/inbox、ordinary CommentInput/IssueActions、source status/custom status、workspace 删除以及普通附件/私人 chat；新分支不能把所有普通任务都变成共享规则。 |
| 服务端 | 后端同伴的 authority/queue 计划为 canonical：事务、旧入口旁路、严格顺序、真实退出屏障、授权/参与撤销、附件与 WS 拒绝、R38 管理范围。前端 UI 成功不是这些保证的证据。 |
| E2E | 建议 `e2e/account-workbench.spec.ts`、`e2e/account-workbench-desktop.spec.ts`、对应 fixtures：同部署独立账号/独立空间；一个 Web 与一个真实 Electron 客户端共同处理同 UUID，另有来源普通成员和未授权账号。 |

每项纯规则在一个 canonical 测试文件拥有完整矩阵；DOM suite 只保留装配、可访问性、正常流程和已命名回归，避免把同一矩阵通过 DOM 再跑一遍。测试使用 `TestApiClient`（`e2e/fixtures.ts:48`）准备/清理数据；R37 会禁止普通删除含共享记录的空间，测试清理必须由专用隔离数据库/fixture 处理，不能松开产品删除限制来让 teardown 通过。

#### 13.2 验收分组（覆盖 AC01–AC66）

| 分组 | 验收条目 | 必须展示/验证的结果 |
| --- | --- | --- |
| A. 账号入口与读取边界 | AC01、AC02、AC05、AC18、AC45、AC46、AC59、AC60、AC61、AC64 | 不加入来源空间仍能处理本单；已读/归档/重启不丢任务；两条基础访问依据；精确 lookup 不改关系；撤权后直接链接、附件、实时一致拒绝，历史不删除。 |
| B. 人类责任流转 | AC03、AC04、AC11、AC13、AC14、AC25、AC26、AC29、AC30、AC51、AC52、AC53、AC54 | 同 UUID 转交与阶段目的、补充交回、验收归发起人、旧处理人留权限、移除当前人原子回交、过期请求不覆写；会话状态不变。 |
| C. 显式指令和共享队列 | AC06、AC07、AC08、AC09、AC10、AC12、AC23、AC24、AC32、AC33、AC34、AC35、AC36 | 普通评论/reply/@AI 不执行；共同指令按服务端 seq；停止受理与退出分开；无自动恢复/重试已停止项；跨端或旧入口无法另建并行队列。 |
| D. 个人运行资源 | AC15、AC16、AC37、AC38、AC39、AC40、AC41、AC42、AC43、AC44 | owner 按单授权、以后合法加入者继承范围、撤回即阻止新执行、手动切换全门禁与显式恢复、Git/附件交付、沿用环境供应商身份但记录真实作者。 |
| E. 退出和移除 | AC17、AC19、AC20、AC21、AC22、AC47、AC48、AC49、AC50 | 谁能移除/退出、未执行指令按作者取消、运行中请求停止并挂起、资源 owner 离开还撤 grant、重加不复活。 |
| F. 生命周期、来源空间与有限代管 | AC27、AC28、AC55、AC56、AC57、AC58、AC62、AC63、AC65、AC66 | 只有发起人或合格有限代管完成/取消/重开；自定义终态不可绕过；业务结束但旧运行仍诚实显示；空间删除全量拒绝且不先清理；代管原因/真实 actor/原发起人。 |
| G. Web / Desktop 共同使用 | AC31 | 真正通过两个客户端接续读取、提交、转交、队列控制；不依赖只操作 API 或手工塞缓存的假跨端通过。 |

建议至少有这些故障注入：Web 发起、Desktop 接手后归档通知并重启；读取/控制响应丢失；提交期间 handler 改变；stop 受理后 daemon 断线，拒绝 resume，再收到可信停止确认；原 A/B 空间同 key 不同 category；外空间 collaborator 被移除后 cached transcript/HTML 预览关闭；owner 离开而当前 command 来自另一人；新授权/重绑/重开不自动运行；源成员和 collaborator 双身份分别撤销；lookup 延迟结果；待办跨页及计数一致。

真实 provider 测试不是默认验证入口。用测试创建的 fake executable / fake daemon 协议及明确的结束证明控制竞态；真实 AI smoke 必须另有明确授权并遵守 `agentintegration` 和 `MULTICA_RUN_REAL_AGENT_SMOKE=1`，本计划不要求消耗个人额度。

#### 13.3 仓库现有命令

来源：根 `package.json:22`、各包 package.json scripts、`Makefile:260`、`:383`、`scripts/check.sh:117`。以下命令是实施时执行的真实入口；文件名中 collaboration/account-workbench 是拟新增测试的筛选词。

```bash
# 迭代时的有界检查
pnpm --filter @multica/core test collaboration
pnpm --filter @multica/views test collaboration
pnpm --filter @multica/web test account
pnpm --filter @multica/desktop test

# 完成实现后的跨包检查
pnpm lint
pnpm typecheck
pnpm test
pnpm knip
pnpm check:ui-exports

# Go 测试会准备目标 DB 并跑迁移，应使用任务隔离环境
make test

# 全量统一入口：静态、受限并发 TS/Go、独立生产模式 Web E2E
make check

# 需要单独复现浏览器场景时，使用任务拥有的生产 Web/API
make up C=api,web ARGS="--web-mode production"
make status ARGS="--json"
make env-exec ARGS="-- pnpm exec playwright test e2e/account-workbench.spec.ts --workers=1 --retries=0"
```

`make check` 的实际脚本使用 `turbo run lint typecheck --concurrency=1 --force`、`turbo run test --concurrency=1 --force -- --maxWorkers=2`、Go vet、隔离 Go race 测试及 Playwright；不会自动代替 `pnpm knip` 或本任务需要的全部原生验证。不要无理由重复完整 pipeline。

Playwright 不自动启动服务（`playwright.config.ts:19`），必须指向 `next build` + `next start`。默认 check 会将测试副本改成 legacy auth 并禁用管理相关开关，**不能据此宣称 R38 通过**；平台代管需要独立 password/admin-capable 的任务环境及合成账号。

原生 E2E 可参照 `e2e/fixtures/project-p1-desktop.ts:10`：隔离 Electron profile，明确测试输出目录，HTTP 与 WebSocket 均代理，断言没有真实 daemon 启动。新 fixtures 必须透传本功能需要的 Idempotency-Key、控制版本及必要响应头；现有该 fixture 的 HTTP header 白名单（`:30`）不能直接满足新控制协议。已有 fixtures 只为当前 `/ws` 代理升级，若账号 websocket 改路径也要相应覆盖。

Native 验证还需实际确认重启恢复、浏览器/原生链接、新窗口行为、Tab/Enter 操作、44px 触控目标、窄屏与 200% 缩放、焦点归还、reduced-motion、HTML 附件隔离。若改到原生文件预览/权限边界，执行已有 `node apps/desktop/scripts/verify-renderer-file-access.mjs`；不能用 DOM class 或 JS 注入的数据代替原生安全证据。

### 14. 建议加入实现/检查上下文的材料

本研究不读取或修改 implement.jsonl/check.jsonl；以下交由主会话登记。

- `CLAUDE.md`、`apps/docs/content/docs/developers/conventions.zh.mdx`：边界、状态归属、全局命名、中文术语、兼容与测试放置。
- `.trellis/spec/core/frontend/index.md`、`state-management.md`、`type-safety.md`、`api-client-auth-epoch.md`：Query/草稿隔离、空集合 schema、stale auth/endpoint、上传请求。
- `.trellis/spec/core/frontend/iteration-operations.md`：稳定请求、权限代次、终态仍可恢复不确定命令、导航身份和撤权清理；只复用模式。
- `.trellis/spec/views/frontend/index.md`、`component-guidelines.md`、`quality-guidelines.md`、`desktop-core-accessibility.md`：共享结构、真实 primitive、缓存恢复、不可交互隐藏内容、焦点和 native 验收。
- `.trellis/spec/web/frontend/index.md`、`e2e-run-environment.md`：生产模式浏览器验证、隔离配置和证据记录。
- `.trellis/spec/desktop/frontend/index.md`、`renderer-file-access.md`：原生页面/预览安全和实机验收。
- `.trellis/spec/guides/cross-layer-thinking-guide.md`、`code-reuse-thinking-guide.md`：来源身份 → API → Query → UI → runtime 各边界及复用核对。
- 由后端同伴配套选择 `.trellis/spec/server/security-boundaries.md`、`managed-installations.md`、`platform-observability.md`：具体权限、收尾及平台管理约束。
- 本文件、`research/status-and-access-contracts.md`、`research/runtime-identity-and-usage.md`、`research/shared-queue-runtime-design.md`，以及后端权限研究最终产物。

多数包 index 标为 To fill 的通用文件仍有模板内容，不能把这些占位文字当作真实项目规则。本轮以根 CLAUDE.md、已填的专门规范和源码为依据。

### 15. 外部参考与版本

本轮为仓库内部研究，没有联网查询或引入外部依赖。版本依据当前声明，不声称是机器上实际安装/锁定的精确版本：

| 组件 | 当前声明 | 本设计用到的能力 |
| --- | --- | --- |
| pnpm | `10.28.2`（根 package.json） | workspace package scripts。 |
| React / Zustand | `19.2.3` / `^5.0.0`（pnpm-workspace.yaml） | 现有外壳、稳定 selectors、客户端草稿与布局。 |
| TanStack Query | `^5.96.2`（catalog） | 显式 query keys、AbortSignal、mutation/回执与缓存权限。 |
| Zod / Vitest | `^4.1.5` / `^4.1.0`（catalog） | 现有 schema 与分层测试。 |
| Next / react-router-dom | `^16.2.5` / `^7.6.0`（Web/Desktop package.json） | Web App Router、Desktop 单 MemoryRouter + coordinator。 |
| Electron / Playwright | `^39.2.6` / `^1.58.2` | 原生窗口、下载与真实跨端 E2E。 |

实施若遇到陌生 API 应再查相应官方版本文档；本文件没有用未核实的外部 SDK 行为代替仓库现状。

## Caveats / Not Found

1. 首版新建-only 或同时转换存量任务仍待主会话得到产品选择。两条路径的附件/旧执行门槛不同，不能根据本研究自行把转换定为必需或排除。
2. O02 的动作→状态映射是建议；最终应选定补充交回原状态失效时的 fallback、重开默认状态等，并在 design.md 固定。不能把这里的推荐当作新增状态体系或已确认要求。
3. 普通任务是否并入账号工作台的全量“当前处理”列表尚未由 R01–R38 明确。明确其范围即可，不应为此自动改变普通任务身份。至少已有普通任务的新建/条件启用入口必须清楚可发现。
4. 未找到现成的账号级共享 issue 工作台、精确全站接收账号 lookup、本单 runtime grant 或符合此任务的 shared-session 控制 UI；现有 Inbox unread summary、workspace member picker、私人 ChatQueue 都不是这些能力。
5. 当前应用已存在旧实现的 header、actor、status、draft 和 websocket 依赖，给 IssueDetail 增加一个 wsId prop 无法覆盖。应以本单数据连接层和有界展示提取实现，避免假 membership provider 或整页复制。
6. 即时撤权的附件保障、旧来源链接恢复、可信退出证明、跨空间执行身份和旧入口隔离都要由服务端协议落实；客户端只能诚实展示，不能通过隐藏按钮证明安全。
7. 现有 root 命名规则、Desktop workspace-only tab 约束需要在此次已评审设计中明确增量扩展；本文件没有修改这些规则或运行状态。
8. 本轮只完成静态研究与文档；没有产品变更、功能通过、性能数据、运行环境或浏览器/原生验收结果可报告。
