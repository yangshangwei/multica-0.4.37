# Research: 介绍、教程和开发说明

- 日期：2026-10-14
- 范围：当前工作区；静态代码对照，无业务文件修改。
- 负责页面：`index`、`concepts`、`how-multica-works`、`tutorial`、`developers/architecture`、`developers/contributing`、`developers/conventions`。

## 确认的不一致

### O1：开发规范错误描述 API 数据命名和 Query key 调用（P2）

- 文档：`apps/docs/content/docs/developers/conventions.zh.mdx:69` 声称 API client 在边界把 snake_case 转成 camelCase，TS 内部一律 camelCase；`:72` 给出 `issueKeys.detail(id)`。
- 实现：`packages/core/api/client.ts:1875` 直接用 `IssueSchema` 解析并返回；`packages/core/api/schemas.ts:1274` 保留 `workspace_id`、`assignee_type` 等键；`packages/core/types/issue.ts:161` 的业务类型也使用这些键，没有所述全局转换。`packages/core/issues/queries.ts:118` 的实际签名是 `detail(wsId, id)`。
- 影响：维护者按该说明读取 camelCase 业务字段会取到 undefined；复制 Query key 示例会缺少参数，忽略工作空间维度。
- 建议：区分本地变量 camelCase 与 API/domain 数据保留 snake_case 的实际契约，把示例改成 `issueKeys.detail(wsId, id)`。

### O2：开发规范要求角色和状态的 UI 标签保留英文，但界面已经本地化（P2）

- 文档：`apps/docs/content/docs/developers/conventions.zh.mdx:213`–`:223` 写明角色名和状态名“不翻”，并要求 UI 展示保持英文。
- 实现：`packages/views/issues/utils/status-label.ts:22`–`:35` 对内置状态调用 i18n；`packages/views/locales/zh-Hans/issues.json:86` 等实际展示“待规划”“进行中”“审核中”；`packages/views/locales/zh-Hans/members.json:3`–`:5` 是“所有者”“管理员”“成员”，成员列表通过 `useRoleLabels` 展示这些标签。
- 影响：维护者遵循自称唯一权威来源的文档，可能把现有中文状态和角色标签改回英文。`issues.zh.mdx:91` 已正确解释内置状态本地化，与规范页自身也矛盾。
- 建议：API、CLI 参数和内部标识符保持英文 key，用户可见的内置状态和角色标签使用本地化文案，自定义状态使用用户设置的名称。

### O3：完整教程使用了旧侧边栏分组和旧目的地名称（P3）

- 文档：`apps/docs/content/docs/tutorial.zh.mdx:43` 指向“侧边栏底部的 配置 → 运行时”；`:269` 称入口为“Skills”。
- 实现：`packages/views/layout/app-sidebar.tsx:175` 把 skills、MCP、runtimes 放入 `resourceNav`，`:931` 渲染该组；`packages/views/locales/zh-Hans/layout.json:61` 的组名是“资源与连接”。当前中文技能目的地使用“技能库”。
- 影响：第一次按教程操作时找不到所述“配置”分组，读者需要自行猜测新入口。
- 建议：更新文字入口为当前侧边栏的“资源与连接 → 运行时”和“技能库”。旧截图的视觉内容尚未逐张验证，需要在后续文档修订时核对。

## 代码偏离规范的已有例外（不计作应放宽规范的文档错误）

- `developers/architecture.zh.mdx:68` 要求会导航的创建、删除、离开操作等待服务端确认；这与根 `CLAUDE.md` 的总体约束一致。
- `packages/views/settings/components/workspace-tab.tsx:294` 在 `leaveWorkspace.mutateAsync` 之前导航；`CLAUDE.md` 已明确将其列为规避实时事件竞争的已知债务。应补充例外提示或后续修复代码，不能把这一处存量偏差推广为新的设计规范。
- `Makefile:280` 的数据库重置使用 Compose 容器中的 psql；本轮仅核查文档所说的本地目标限制与命令存在性，未执行破坏性命令。原生 PostgreSQL 与容器同时存在时的具体目标证明不在本轮运行验证范围。

## 逐页覆盖

| 页面 | 已核查重点 | 结果 |
| --- | --- | --- |
| index | 任务、协作、自动化和 skill 入口；文档链接 | 概览未发现额外确定性错误；“全部上下文”不能用于推断私聊公开 |
| concepts | Issue / task、负责人三种类型、执行和自动化的关系 | 与类型、任务/自动化入口一致；具体权限及异常由专题页核查 |
| how-multica-works | 服务端与 daemon 分工、队列、本地执行、task 与 issue 状态分离 | 与服务、daemon 和任务状态流程一致；触发边界以专题审计为准 |
| tutorial | 创建入口与默认智能体模式、运行时、仓库、浮动聊天、skills、小队、自动化 | 确认 O3；`create-mode-store.ts:28` 的默认模式确为 agent；外部 shadcn 模板命令和示例 AI 输出未实跑 |
| developers/architecture | 包分层、API/WS、任务链路、数据状态、工作空间边界 | 总体一致；记录离开工作空间的现有代码例外 |
| developers/contributing | Node/pnpm/Go 版本、Make 命令、worktree 初始化、检查流程、数据库约束 | 主要命令与当前脚本一致；`init-worktree-env.sh` 仍按路径生成传统 worktree 环境，不与新增 `make up` 的登记分配机制混为一谈 |
| developers/conventions | API 数据形状、Query key、状态和角色标签、包边界、路由和迁移规范 | 确认 O1、O2；其余规范作为编码约束，不把所有历史代码偏差当作文档错误 |

## 验证和局限

- 文档包在临时目录重新生成并比较，39 页、58 个资源、共 98 个文件与当前提交目录字节一致。
- 检查 237 个内部链接、35 个锚点和 57 次图片引用，无缺页、断锚或缺图。
- `pnpm --filter @multica/docs test`：9 个文件、62 个测试通过。
- `go test ./internal/docs/...`：通过。
- 未启动服务器或浏览器，未验证真实账号、第三方 AI、模板注册表、截图内容和发布中的二进制。
