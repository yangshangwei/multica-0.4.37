# 简化 AI 小队页并迁移模板到能力市场 — 技术设计

## 目标和边界

保持现有产品视觉系统与业务语义。AI 小队页沿用集合页结构，只去掉模板切换层并补充和智能体目录一致的说明/文档入口；能力市场增加一个 URL 驱动的 AI 小队模板类别。

不调整后端 API、模板响应、模板创建页或项目应用对话框。Web 与 Desktop 继续使用 `packages/views` 的共享实现。

## 页面与路由

### AI 小队目录

- `SquadsPage` 始终展示当前 workspace 的已保存小队列表或现有 welcome/loading/error 状态，移除页面级 `Tabs`、`activeView` 和 `selectView`。
- 集合页头使用现有 `CollectionPageHeader` 的 `description` 与 `learnMore` 属性，采用智能体目录已经验证的图标、标题、说明和文档链接层级。空目录仍由 `SquadWelcome` 提供唯一主创建动作，并增加进入模板市场的次级入口。
- 当路径参数 `view=templates` 时，通过共享 `NavigationAdapter` 执行 `replace`，把 pathname 改为 `paths.marketplace()`，把 `type` 设为 `squad-templates` 并移除 `view`。其余 query 参数和当前 hash 保留。只兼容这一精确旧值，其它未知参数不影响目录显示。

### 能力市场

- 扩展市场类别为 `mcp | skills | squad-templates`，并在 `marketplaceTab()` 路径 helper 中加入 `squad-templates`。
- `type` 继续是选择状态唯一来源；缺失或未知值仍落到 MCP。类别切换继续通过 `currentPath()` 拼接 URL，以保留其它 query 参数和 hash。
- 第三类使用现有 `Tabs` 结构，遵循已访问 panel 保持挂载、隐藏 panel 标记 `hidden` + `inert`、并把 `active=false` 传入非活动目录的约定。
- 新增轻量 `SquadTemplatesMarketPanel` 作为市场与小队领域组件间的适配层。它在 active 且 workspace ID 有效时通过 `squadListOptions(workspaceId)` 加载实例，并分别反馈加载和错误/重试状态；成功后把列表交给现有 `BuiltinSquadCatalog`。模板仍通过现有 `useSquadTemplates` 查询，增加其 `enabled` 接线，使非活动 panel 停止查询。
- `BuiltinSquadCatalog` 加入 `active` 属性（默认 true，保留既有直接使用测试/调用语义），非活动时不打开项目配置对话框，并清除临时选择，避免回到该类别时重新出现过期弹窗。

## 用户流程与兼容性

1. 用户打开「AI 小队」时直接进入目录；welcome 的「浏览模板」入口进入 `/marketplace?type=squad-templates`。
2. 用户打开「能力市场」时，缺省仍是 MCP；选择 Skills 或 AI 小队模板时更新 URL，前进/后退恢复选择。
3. 模板卡片的摘要、队伍数、已有实例链接和「应用到项目」动作沿用原实现。项目配置、运行时选择、权限、API 和导航副作用不改变。
4. 旧 `/squads?view=templates` 书签由共享页内导航重定向到新的市场类别。`replace` 避免在历史栈新增无意义页面；原 query/hash 中的其它上下文会保留。

没有持久化或 API schema 变化，无数据库迁移。可逆回退方式是恢复小队页原模板标签并移除市场第三类及旧链接重定向。

## 本地化与无障碍

- 英文与简体中文均新增市场类别标签、小队页 tagline/learn-more 与 welcome 的模板入口文案。
- 使用现有 `Tabs` 原语保留键盘导航和 tab/tabpanel 语义。
- 检查新类别的 panel 命名、选中状态及窄屏导航可达性；目录说明沿用公共集合页 header 的截断/断点策略。

## 验证策略

- 单测覆盖小队页不再展示旧 tabs、`view=templates` 的 path/query/hash 替换、市场三种类型的直接恢复/键盘切换/历史切换、已访问 panel 保持挂载且 inactive 关闭查询、工作空间切换隔离、模板实例匹配与项目对话框保持原始 template。
- 更新生产 Web journey，覆盖新市场入口、旧书签重定向、现有安全修复模板应用、两种语言和窄视口；既有 Desktop 路由共用实现，执行可用的 Desktop/市场回归检查。
- 执行相关 Views 与 Core 定向测试、类型检查及 lint。
