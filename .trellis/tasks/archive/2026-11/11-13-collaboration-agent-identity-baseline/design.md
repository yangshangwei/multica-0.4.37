# S1 技术设计

状态：已于 2026-11-13 实现并完成验证；实际结果见 report.md。

## 1. API合同

为 `CollaborationCommentView` 增加可选的 `author_agent` 最小摘要：

```json
{
  "author_type": "agent",
  "author": null,
  "author_agent": { "agent_id": "<UUID>", "name": "支付研发 Agent" }
}
```

- `author`仍只表示人员；member/未知类型的 `author_agent` 为null或省略。旧客户端忽略新增字段。
- 只有 `author_type=agent` 且数据库记录的 `author_id` 有效时构建摘要。ID来自已授权任务的评论，不接受请求参数、评论文本或显示名反推。
- 名称采用当前Agent名称，与已有协作上下文作者语义一致。查询不到实体时保留可信ID、名称为空，客户端显示通用智能体；不补造历史名称。
- 不返回run、资源所有者、工作空间、头像URL或运行时详情。现有运行记录已承担精确执行归因，不为每条评论再查询或请求这些内容。

## 2. 服务端读取路径

在 `server/internal/service/collaboration_comments_view.go` 的 `collaborationCommentViews` 收集本页Agent ID并去重，一次调用现有 `ListCollaborationMemoryAgentNames` 查询最小ID/name。该sqlc查询虽由memory命名，但语义是纯ID批量查名，可直接复用；不为改名字牵动其他模块。

读取前继续经过 `readCollaborationContent` 的授权repeatable-read事务。`ListComments`、`GetComment`与`RecallComments`共用投影，保持同页身份一致。没有Agent行时不发查名SQL。无权调用在读取评论前拒绝；只允许已有任务读权限获得这些评论作者的最小信息。

资源grant撤回、资源停用、Agent归档不抹掉历史作者；查询当前存在Agent的名称。Agent硬删除后缺名称不会造成整个列表失败。读者撤销任务权限则拒绝全部评论数据。查名的真实数据库错误应返回错误，不能伪装成已删除作者。

新增字段参与现有 `budgetCollaborationComments` 实际JSON字节预算；不手算附加常量。测试验证转义名/长名、新字段导致的分页、placeholder补读及cursor推进，保证不超预算、不丢评论、不重复。

不修改runtime写协议、回执、角色、运行准入、历史事件类型或数据库表。

## 3. Core解析与界面

`packages/core/collaboration/content-schema.ts` 增加展示字段：合法UUID和string名称；字段缺失/null标准化为null。仅新增的展示摘要畸形时丢弃该摘要、保留合法评论；原ID、revision、content-state、访问及分页校验维持严格。非agent评论上的摘要不得用于显示或权限判断。

| 数据 | 列表 | 回复提示 |
| --- | --- | --- |
| agent、有非空名称 | 名称 + 独立“智能体”标识 | 同一名称及智能体标识 |
| agent、缺字段/畸形/空名 | 通用“智能体”，不重复显示标签 | 通用“智能体” |
| member | 现有人员名称/头像/外部标识 | 现有人员名称 |
| 其他类型 | 现有未知作者回退，不信任Agent摘要 | 同样回退 |

`comment-list.tsx` 与 `comment-composer.tsx` 复用一个小的展示选择函数，避免两处回退分叉。使用现有semantic样式、换行和本地化词条；必要新词条仅补en/zh-Hans，不增加组件库。Agent标签通过文字识别，不仅靠颜色。现有人类编辑权限计算不读取新增字段。

`comment-history.tsx`的人类编辑事件不变。当前Agent写入的是 `runtime_comment_created`，并不在 `ListCollaborationCommentHistory` 的人类事件查询中；首包不把Agent投影成历史人类actor。实际归因继续核对已有run记录。

## 4. 回归基线与证据

扩展 `e2e/collaboration-multi-account-closure.spec.ts` 原canonical场景：保持三账号/三Agent/五run（1/2/2）、普通评论不派发、原key重放、授权撤回与显式Resume、信息往返、首次交付退回、第二次发起人精确接受。新增每个Agent结果的API摘要与对应run Agent ID一致、三会话评论可见、回复提示一致的断言。

Core/服务端已有测试分别负责原操作恢复、冲突、撤权和幂等矩阵；离线通知及撤权使用已有专用E2E。结果报告分别列出其场景，不能拼成一个实际上未执行的全链路故障故事。

在canonical闭环产生的新任务上，通过现有 `collaboration-native` Electron renderer harness做定向作者/回复提示检查；不再执行一遍完整Provider流程。使用测试自有profile、登录与deep link，验证完只清理自有进程/profile。三会话Web闭环与Electron定向证据分别标明。

每次验证记录HEAD、实际源码差异/哈希、构建、API进程来源、测试/Provider二进制来源与环境ID；结束再核对相关源码未漂移。历史PAY-1报告只作来源参考。本包基线详见 `baseline-cases.csv`。

## 5. 环境与失败处理

DB单测每包新建独立数据库；浏览器API使用另一数据库。按现有规范核对数据库身份、连接预算和配置传播，不能借共享fixture库或先运行TestMain探测环境。明确自建daemon/provider/CLI，不从PATH查找真实Agent账号。

production Web与API必须由本次记录的同一源码状态构建。若相关文件在共享工作区继续变化，保留失败/中断证据，重新绑定新版本后运行，不复用旧通过。

运行与清理失败均导致该次失败；保存原始失败，保留准确命中测试数。未配置而skip不算通过。默认不开启真实Provider或生产发布；本任务自有测试开关仅在受控fixture内使用，并恢复原值。

## 6. 取舍与回滚

- 采用加法最小摘要；不改人类author结构，不新增统一身份大模型。
- 采用当前名称+稳定ID；不新增写入时名称快照/迁移。
- 采用服务端批量投影；不让前端逐条抓Agent详情，不要求外部参与者取得资源工作空间权限。
- 采用现有三Agent闭环；不为首包扩展第四Agent、支付业务或职业RBAC。
- 代码可按服务端投影/Core解析/共享展示整体回滚，不改存量数据；旧客户端/旧服务端均有明确退化行为。
