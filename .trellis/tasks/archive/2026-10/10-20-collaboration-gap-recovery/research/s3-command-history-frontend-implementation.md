# S3b: 只读命令历史共享前端

状态：共享客户端切片实施与专项验证完成，交主会话独立检查及生产 Web/实际 Electron 验收。承接已验证的 `GET /api/collaboration/issues/{id}/commands`；本记录只覆盖 S3b 共享客户端切片，不作为完整 AC-I05 或父任务完成证据。

## 修改边界

现有执行面板只显示 waiting 输入和已有 run，未消费的取消输入在刷新后没有可见记录。权威数据来自受当前本单 ACL 保护的命令历史接口，不能从可编辑评论、当前 target 环境或运行结果推断。

- `packages/core/collaboration/command-history-schema.ts`：纯 API 解析、精确 decimal、原始输入与取消证据的关联及缺失语义。
- `packages/core/collaboration/command-history-queries.ts`：复用账户/对象访问 epoch 围栏与 TanStack Query；校验分页链后缓存，禁止回环和重复输入。
- 相邻 schema/query 测试及共享 fixture：canonical 解析、分页、访问恢复测试。
- `packages/views/collaboration/command-history.tsx` 及相邻测试：按需展开的只读历史，复用受限 RichContent、当前授权附件、原评论链接和语义样式。
- `packages/views/collaboration/execution-panel.tsx`：仅挂载历史，不从历史添加 Cancel/Stop 权限。

API client、barrel、locale、Web/Electron 环境与视觉验收由主会话整合。保留 S2 资源提供和所有原有脏改动，不修改后端、评论编辑器或资源表单；不新增依赖、不提交或归档。

## 验证记录

所有命令从仓库根目录执行；未启动 API/Web/daemon 或真实 provider。

1. Schema 先写 canonical 测试，再加基础 DTO 形状。初次行为 RED：37 项中的 21 项通过、16 项失败，具体为作者/事件/取消字段关联、未知 legacy 证据、页内重复/混合 session 和精确时间排序未受保护。补齐这些约束后 37/37 通过；随后增加同一评论面向不同 typed target 的正例和 UUID 大小写保留正例，最终 39/39。
2. Query 初次运行遇到测试文件多余右括号的语法错误，0 个实际测试，不能算行为 RED。修正后 24 项中 17 通过、7 失败：6 项为跨页回环/重复/冲突/session/排序/取消事件复用未被拒绝；1 项是测试错误要求传输 AbortSignal 保持同一对象，而 ApiClient 实际合成信号。改为真实进行中的 abort 传播测试，不改传输实现。补齐分页链检查后 API/query 最终 25/25。
3. UI 回归先作用于原执行面板，实际 1 项失败于缺少可访问的 `Command history` 按钮。初次挂载实现后该完整旅程通过；随后加入来源成员读取缺失 legacy/自动取消记录、下一页撤权移除历史的两个共享接线测试。与现有执行面板共 8 项通过。
4. 首轮 scoped ESLint、core typecheck 通过；views typecheck 报 TS6059，原因是新测试相对导入了 core 夹具，超出 views 的 `rootDir`。主会话从既有 `@multica/core/collaboration/test-fixtures` 子路径重新导出夹具，本切片改用该包导入。没有放宽 tsconfig、增加依赖或新增包导出路径。修正后 views typecheck、8 项视图测试及 scoped ESLint 全通过。

实际失败记录保留于仓库外：

- `/tmp/multica-s3b-history-query-red.json`：首次测试语法错误。
- `/tmp/multica-s3b-history-query-behavior-red.json`：24 项中的 7 项失败，含上述单独标注的 signal 测试假设错误。
- `/tmp/multica-s3b-history-view-red.json`：原执行面板缺少历史入口的 1 项行为失败。
- `/tmp/multica-s3b-history-view-first.json`：首次完整 UI 旅程通过。

最终通过命令与结果：

```bash
pnpm --filter @multica/core test command-history run-records-queries access --reporter=dot
# 11 files / 161 tests passed; 0 failed, 0 skipped.

pnpm --filter @multica/core test command-history --reporter=json --outputFile=/tmp/multica-s3b-history-core-verified.json
# Final fixture-package integration: 2 files / 64 tests passed.
# command-history-schema.test.ts: 39; command-history-queries.test.ts: 25.

pnpm --filter @multica/views test command-history execution-panel --reporter=json --outputFile=/tmp/multica-s3b-history-views-verified.json
# 2 files / 8 tests passed: 3 new history tests + 5 existing execution-panel tests.

pnpm --filter @multica/core typecheck
pnpm --filter @multica/views typecheck

pnpm --filter @multica/core exec eslint collaboration/command-history-schema.ts collaboration/command-history-schema.test.ts collaboration/command-history-queries.ts collaboration/command-history-queries.test.ts collaboration/command-history-test-fixtures.ts
pnpm --filter @multica/views exec eslint collaboration/command-history.tsx collaboration/command-history.test.tsx collaboration/execution-panel.tsx
```

以上最终命令全部 exit 0。JSON 结果已逐项核对：64 项 core、8 项 views 均无失败或 pending/skip。两包 typecheck 及所改文件 ESLint 均通过；scoped `git diff --check` 无输出。pnpm 输出已有根配置迁移提示（根 package.json 的 `pnpm` 字段），本切片没有修改包管理配置。

## 最终行为与复用

- `command-history-schema.ts` 是 API 可直接导入的纯叶子，不导入 request helper/客户端 barrel。`protocol_version: 1`、typed target、必需的 nullable evidence 和 signed-64-bit decimal string 都保留原域。已知 manual/automatic 与六字段全 null 的 legacy 对象有明确区别；缺事件时不能仅填一个 actor/time，不能把后台执行冒充为人类。
- 原输入正文、作者 UUID、附件 ID 和原评论事件来自历史 DTO，当前可编辑评论不提供替代正文。多目标命令可共享同一个真实 input event；每条精确取消仍有独立事件。精确时间/UUID 降序用于分页顺序检查，不用时间邻近推断 provenance，也不把 channel sequence 当成全单时间顺序。
- `hasValidCollaborationCommandHistoryItems` 同时供页解析和完整分页链检查复用。链内拒绝重复 ID、冲突原文、跨 session、反向时间、复用取消事件、游标回环；完整 refetch 可以替换旧状态和 Switch 后当前位置，不能对照旧快照阻止真实取消或移动。旧 run/group 历史继续由原执行投影负责。
- Query 完整使用 `collaborationKeys` 和 `protectCollaborationRead`。缺可选路由或畸形响应在同一围栏内重查当前本单权限；明确拒绝清除所有已加载页，暂时失败保留先前合法数据。Server/viewer/session 或对象访问 epoch 改变后的迟到成功/拒绝不能恢复数据或清掉新权限。
- 主会话集成的真实 ApiClient 已被本切片测试覆盖：UUID、1–100 limit、1024 字符 cursor、返回数量和当前 cursor 关联、请求本单 ID、no-store、无 ambient workspace header、畸形 JSON/证据、实际 abort 和 credential epoch。
- `CollaborationCommandHistory` 按需展开，使用同一共享实现服务 Web/Desktop。正文复用受限 `RichContent`，附件复用当前授权的 `CollaborationCommentAttachments`，原评论使用 `AppLink`/`accountPath`。事件/作者/目标/附件 UUID 和 raw reason 留在默认折叠的详情。Query 是唯一服务端状态；组件只保留展开状态。
- 历史不增加控制按钮。执行面板原本的当前 waiting/作者/creator/handler/写入上下文仍决定取消；历史没有 Cancel→Stop 的升级逻辑。来源只读成员可以打开历史但没有控制权。

## 文件与所有权

本切片新增 5 个 core 文件（schema、query、两套相邻测试、夹具）、2 个 views 文件（组件、相邻测试），仅对 `execution-panel.tsx` 增加 import 和挂载，另更新本证据文件。该面板原有 S2 的 `offer_resource`、`adopt_resource_offer`、`withdraw_resource_offer` 三个 action 名完整保留。

主会话独占整合 `api/client.ts`、`collaboration/index.ts`、中英文 locale 和既有测试夹具的 re-export；这些协调改动没有被本代理覆盖。`labels.ts`、S2 资源表单和评论编辑器未由本切片修改。复用已有读取围栏、Markdown、附件、导航和执行控制，没有新增状态仓库、兼容控制层或依赖，也没有提交/归档。

## 交接与限制

本切片 scoped 类型、测试、lint 无已知未解决错误。生产 Web/实际 Electron 的历史展开、长文本/窄屏/200% 缩放、换端/撤权及新页面截图 `visual-verdict` 仍由主会话验证。仅沿用 `.omx/reports/cwc-gap-recovery-20261021/visual-baseline.json` 的历史样式依据，不将旧截图视为本次 UI 通过证据，也未进行未验证的后续样式迭代。

未运行完整 `make check`、全部 97 项、安装包、provider/OS/S3/Redis 验收。后端权限/事务/claim 竞争矩阵使用其独立报告，不在 DOM 重跑或宣称本次重验。创建/执行开关和父任务完成状态均未改变。
