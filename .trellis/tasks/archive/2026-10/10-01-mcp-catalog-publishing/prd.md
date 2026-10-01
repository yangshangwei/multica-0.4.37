# MCP 市场服务器目录发布

## Approval and goal

2026-10-02 用户选择“扩充官方精选目录，补齐上架、校验与发布流程”。
沿用服务端内嵌目录，让维护者能核验、上架、修订和撤回配方，让用户通过现有
市场发现并配置可用的公开 MCP 服务。

## Requirements

- 保留现有 Chrome DevTools、Playwright、Sequential Thinking 三个条目及版本。
- 新增 Microsoft Learn (`microsoft-learn`) 与 DeepWiki (`deepwiki`)，均使用官方
  免鉴权 Streamable HTTP endpoint，配方版本为 `1`；归入“文档与知识”。
- 提供中英文名称、描述、官方文档、网络/运行时要求和公开内容边界；DeepWiki
  明确仅面向已索引公共仓库，Microsoft Learn 不包含个人资料或培训内容。
- 使用现有可信 key/version 创建、只写配置、明确分配和来源追踪；未知或过期
  配方拒绝新建，不自动升级或删除已保存实例。
- 建立可重复的目录发布检查入口；校验 key 唯一合法、正整数配方版本、完整的
  双语元数据、HTTPS 文档/远程地址、合法配置形状、无凭据和无待填占位参数。
- 发布指南说明官方来源核验、候选准入、验证、后端发布、版本递增、撤回及回滚。
  新条目与修订仍通过代码审阅及既有发布流程交付。
- 保留共享 Web/Desktop 视觉、交互与安装版客户端兼容性，无新依赖、表或接口。

## Acceptance criteria

- [x] 五个模板均可被目录 API 发现，包含有效配置、双语元数据和真实运行条件。
- [x] 新“文档与知识”分类可筛选和搜索；新模板在市场及重命名后的共享列表图标一致。
- [x] 新 HTTP 模板能保存、保留来源、显式分配，保存本身不创建智能体绑定。
- [x] 目录检查在合法条目上通过，并在发布契约破坏时失败；默认检查不访问服务商。
- [x] 维护者文档给出可执行发布检查、版本修订及撤回步骤；更新过时的内置 skill 来源说明。
- [x] Go/TS 回归、lint、typecheck、静态检查通过；Web/Desktop 验证保存和分配新配方。

## Scope limits

用户公共投稿、外部 Registry 同步、OAuth/凭据输入、必填自定义参数、自动更新、
连通性按钮、认证徽章和移动端 UI 均不在本轮。Context7 不恢复。配方版本不是
上游包版本；保存和分配成功不代表已启动或连接。任务不包含远程推送或生产部署。

## Evidence

- [现状与历史决定](research/current-state.md)
- [官方来源与匿名协议核验](research/official-sources.md)
- [设计](design.md) / [实施计划](implement.md)
