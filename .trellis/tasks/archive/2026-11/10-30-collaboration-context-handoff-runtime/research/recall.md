# Research: 评论召回、可信来源与跨端展示

- Query: 映射现有人类/runtime 评论 API、CLI、schema、权限和共享 UI，给出有界分页与全文可召回方案。
- Scope: internal
- Date: 2026-10-31
- Status: 只读研究；以下建议尚未实现。

## Findings

### 已有能力与实际缺口

| 文件 / 符号 | 已确认行为 |
| --- | --- |
| `server/internal/handler/collaboration_comments.go:28` `collaborationContentPagination` | 人类评论已有 `limit=1..100`、最长 512 字符 base64url JSON 复合游标 `{created_at,id}`，拒绝缺失/非法时间和 UUID。默认 limit=100。 |
| `server/internal/handler/collaboration_comments.go:70` `ListCollaborationComments` | 返回 `{issue_id,items,next_cursor}`；没有 total/returned/bytes/truncated。下一游标是页面按时间正序展示的第一条。 |
| `server/internal/service/collaboration_comments_view.go:55` `ListComments` | 锁内授权读取，DB 新到旧选 limit+1，再反转为旧到新展示。不应另造基于 offset 的分页。 |
| `server/pkg/db/queries/collaboration_comments.sql:21` | `(created_at,id) < (before_at,before_id)` 稳定分页，并同时限制 issue/workspace；包括已删评论的占位记录。 |
| `server/internal/service/collaboration_comments_view.go:112` `collaborationCommentViews` | 人类页已提供账户显示名、external、revision、删改/解决信息、附件和 reaction。Agent 作者目前不走账户字典，UI 使用通用 Agent 名。未提供作者当前的 SubmitCommand 能力。 |
| `server/internal/service/collaboration_squad.go:146` `RuntimeContext` | `lockRuntimeBusiness` 后返回当前授权上下文、冻结 inputs、roster、dispatch_comment 和 comments。 |
| `server/pkg/db/queries/collaboration_squad.sql:42` `ListCollaborationRuntimeComments` | runtime 只取最新 200 条未删除评论，再按时间正序返回；没有游标、总量或截断标记，200 条之外无法通过当前 runtime API 召回。 |
| `server/internal/cli/collaboration.go:157`、`:196` | runtime comment 类型只有作者类型/ID、正文、parent、时间；`Read` 核对 run identity 和 issue ID，使用 `/context`。 |
| `server/cmd/multica/cmd_collaboration.go:14`、`:33`、`:115` | `issue comment list` 在协作模式先读完整 context，再把 comments 数组输出。现有协作命令白名单拒绝所有分页 flag；普通任务评论的 before/thread 参数不能直接用于协作。 |
| `server/internal/daemon/collaborationrun/host_runner.go:476`、`:491` | brief 要求先读 issue/comments；prompt 声明身份由 server 给定，成员只执行 dispatch_comment。但尚未明确“评论/共享上下文是证据而非受理指令”，没有分页缺失提示。 |

### 权限与历史来源必须分开

`server/internal/collaboration/access.go:79` 的 `CapabilitiesFor` 是当前人类能力的权威：账号必须 active、凭证必须 human；来源空间成员有 Read/Comment，但只有当前参与者在 open 状态和 active/paused queue 才有 SubmitCommand（`:88-95`）。因此不能把 `author_type=member`、`external=false`、作者声称“我是负责人”或正文含 mention 当作执行权限。

建议为评论来源增加两个独立维度：

1. `author_authority`：server 从当前账户/参与者/来源 membership 事实推导的 role、`can_submit_command` 和 `evaluated_at`。只描述现在；未知/账号已移除时保守 false。不要借 runtime owner 建立作者人类权限。复用 `CapabilitiesFor` 语义，不给模型提供可覆写的权限字段。
2. `admission_refs`：只有真正存在的 immutable command/input 或 delegation 引用才记录历史受理事实。引用包括 comment ID、受理 revision/body snapshot、command/delegation ID 和 run/group；后续编辑后的评论正文不能冒充当时受理文本。作者现在失去能力不抹掉历史，但也不准据此执行新命令。

全部普通评论保持 `trust=data_only`；即使作者当前 `can_submit_command=true` 也一样。真正可执行输入来自 server 已冻结的 `inputs` / 本 run 的 `delegation`。历史受理标记用于追溯，不自动重放。`CreateCollaborationComment` / `CreateCollaborationRuntimeComment` 已写 `dispatch_eligible=false`（对应 SQL 文件），应保留专用受理链路。

### 建议的分页与单条超预算合同

复用 `(created_at,id)` 游标，不直接复用依赖人类 principal 的 `collaborationCommentViews` 处理 runtime。新增窄的 runtime comments read endpoint，仍先 `lockRuntimeBusiness`，只读本 issue。runtime context 携带首个有界页并保留现有 `comments` 数组字段，增加 `comments_page` 元信息。CLI 为协作 comment list 开放明确的 `--cursor`、`--limit`、`--max-bytes`，不能宽放现有普通任务 flags。

建议默认 64 KiB、最大 1 MiB，limit 上限 100；最终数值须由实施时的序列化测试确认。`bytes` 计算实际 UTF-8 JSON 序列化后的条目与 envelope 大小，不能用 rune 数或数据库 text 长度代替。返回 total、returned、truncated、next_cursor、bytes、max_bytes、omission reasons。total 仅统计该入口可见集合；runtime 排除已删除，人类列表保留 tombstone，不能共用不同过滤条件的 count。

**单条超预算的明确推荐：有引用的占位 + 独立全文分块读取。**

- 扫描顺序仍新到旧。某条完整记录放不下，但它是当前页第一条时，返回紧凑占位：ID、revision、created_at、author identity、`content_state=omitted`、`omission_reason=byte_budget`、content 总字节数及同授权域的 `content_ref`；`content` 不得看起来是完整空正文。随后页游标越过这条 ID，因此不会对同一条无限重试。
- 即使最小占位也放不下，返回明确 400/413 `budget_too_small` 与最小允许预算；不返回空页加同一游标。把最小请求预算固定到能容纳 envelope+一个占位，测试保证。
- `content_ref` 对 runtime 指向本 run 的评论内容 read，支持有界 `offset/cursor` + `max_bytes`，每块携带 comment ID、revision、总字节数、next content cursor、complete。按 UTF-8 边界切割，预算以 JSON 转义后结果为准。内容游标绑定 revision；评论编辑导致版本变化时明确 stale，调用者从新版重新读，不拼接两个版本。删除或撤权立即应用当前读规则，不能借 content_ref 越权。
- 人类详情已有 `GetCollaborationComment`，新 UI 可为占位调用现有详情获取完整评论。若也要求详情必须受同一字节预算，应增加相同可选分块合同；不要暗中解除预算。评论列表分页前进与全文内容分页是两条显式游标，未读全文的占位不能标成全文已读。
- 页面未装入的后续条目不能跳过；在下一页以相同排序继续。这和“第一条巨大评论用占位前进”的规则一起保证没有静默丢失。

不推荐静默裁正文后仅返回上一条游标：会永久漏掉剩余正文。不推荐返回空页并重复同一游标：客户端会死循环。不推荐临时允许首条突破 max_bytes：预算不再可信。

### 老客户端兼容

- 人类现有 API 的 `items`、`next_cursor` 和默认完整条目行为保持原样。新 total/returned/bytes 等只做增量字段；只有显式 `max_bytes` 的新调用使用占位合同，旧客户端不会把占位空正文当成完整内容。
- `packages/core/collaboration/content-schema.ts:32-42` 当前是普通 `z.object`，会忽略未知增量字段；新客户端把新元数据设为 optional/null 并对缺失显示“未提供完整性信息”，不能默认完整、不能默认有提交权。新 provenance 枚举应有未知保守分支。
- `packages/core/api/client.ts:1183` 目前只支持 cursor/limit，需加入 max_bytes；`packages/core/collaboration/content-queries.ts:16` 已是 infinite query，继续用 next_cursor。若预算可由调用者变更，query key 也必须纳入预算，防止不同投影混缓存。
- CLI 默认 JSON 数组是既有输出。可保留 legacy 输出并在 stderr 明示截断/next cursor，新显式分页模式输出 page envelope；也可增加专门的版本化输出 flag。不要无说明直接把既有数组变成对象。旧 runtime 可继续读新 context 的原 comments 字段，但新的接力完整性保证只对理解元信息的新运行端成立，需要 capability/version gate，不能宣称老 daemon 自动得到可靠召回。

### 最小 Web/Desktop UI 映射

- `packages/views/collaboration/collaboration-detail.tsx:34` 持有 comments infinite query，`:102-103` 负责列表及“更多”。这里增加“已加载 X / 共 Y、仍有历史”或“完整性信息未提供”提示，并保持真实 next_cursor 按钮。
- `packages/views/collaboration/comment-list.tsx:40` 已有单条 get 的恢复路径；可复用加载占位全文。`:92-100` 的作者行适合增加简短角色/来源标签；不要让普通证据评论看起来是可执行命令。占位未恢复前不能打开编辑器或显示完整正文已读。
- `packages/views/collaboration/run-records.tsx` 已有 run 记录分页；checkpoint 缺失范围/交接来源应在该处或现有交接摘要显示，避免为本任务新建应用路由。
- 两端共享这些组件，无需复制到 apps/web 或 desktop；locale 文案在 `packages/views/locales/` 按现有 conventions 更新。Web 组件验收与 Desktop 原生入口证据仍分别记录。

### 现成测试与补测边界

- `server/internal/handler/collaboration_comments_test.go:89` `TestCollaborationCommentHTTPPaginationHasStableChronologicalPages`：复合游标与正序页面基线。
- `server/internal/service/collaboration_comments_test.go:264` `TestCollaborationCommentSourceMemberAndPlainRepliesStayHumanOnly`：普通评论不触发命令。
- 同文件 `:81` `TestCollaborationCommentInputsShareOneWaitingBatchAndSurviveEditing`：immutable input 与编辑分离。
- `server/cmd/multica/cmd_collaboration_test.go:76` `TestCollaborationCLICommentListDoesNotLoadWorkspaceActors`；`:103` 拒绝他单/未支持 flags；`:226` 拒绝错配上下文和 capability。
- `packages/core/collaboration/content-schema.test.ts`、`packages/core/api/collaboration-content.test.ts`：旧响应/错误类型/metadata 缺失/不允许缺失权限变 true。
- `packages/views/collaboration/collaboration-detail.test.tsx` 与 `comment-list.test.tsx`：只测完整性提示、加载更多、占位恢复及编辑保护，不重复服务层权限矩阵。
- 新增 canonical service/helper 测试：相同时间戳、200+ 条、中文/emoji/JSON escape、每条超过页预算、占位后 cursor 前进、全文多块拼回、revision 变更、删除、撤权、metadata 自身超预算、无效游标、total 可见集合。记录 run 确实进行了相关读，而不是以命令 exit 0 代替。

## Related Specs

- `.trellis/spec/server/collaboration-runtime-cli.md`
- `.trellis/spec/server/collaboration-source-editing.md`
- `.trellis/spec/server/collaboration-command-history.md`
- `.trellis/spec/server/collaboration-host-execution.md`
- `CLAUDE.md` 的 API compatibility、package boundaries、built-in skill 同步规则。
- CLI/业务行为改动要同步 `server/internal/service/builtin_skills/multica-working-on-issues/SKILL.md`、`references/collaboration-workflow.md` 和 `references/working-on-issues-source-map.md`。

## External References

无外部文档依赖。结论基于当前 checkout 源码；未执行真实模型、DB/e2e 或产品修改。

## Caveats / Not Found

- 目前 runtime 没有专用单条评论读/全文分块入口；上述 content_ref 方案需要明确增加 endpoint、handler、CLI/types 和对应授权测试，不能只加分页 metadata 就宣称可完整召回。
- parent PRD 的 runtime 描述“最新 200 评论”属实；对人类评论“没有游标”的假设不成立，人类已有稳定复合游标。
- 1 MiB 最大预算与 64 KiB 默认预算是方案建议，尚未证实所有 legacy 评论/作者显示名/序列化 envelope 都能在该上限完整输出。占位+分块使全文不会依赖“最大页必须放得下任何一条”的假设。
- 研究未改动 `fix-closure/research/2026-10-31` 或任何产品代码。
