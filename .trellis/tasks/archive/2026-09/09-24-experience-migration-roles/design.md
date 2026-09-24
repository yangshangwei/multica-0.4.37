# 技术设计

先实现 workload evidence gate，再决定是否把两个角色设为 listed。体验验证复用 Playwright/Chrome MCP 和现有 QA，输出关键路径、可访问性、截图/日志、跨端差异；迁移审查复用 architect/security/release，输出旧新契约、兼容窗口、校验、回滚、重试和孤儿数据检查。两者均不直接操作生产。

若阈值未达，保留未列出的模板或 skill 仅供受控路由，并增加负向测试保证默认 picker 不显示；若阈值达到，按核心角色同样的模板、权限、本地化和 copy-preservation 规则上线。
