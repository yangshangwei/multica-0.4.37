# 体验验证与迁移审查专项角色

## Goal

在有真实工作量证明时补充体验验收和迁移审查两类专项判断，避免把产品体验和数据迁移风险隐含在 QA 或发布提示词中。

## Requirements

- 先从现有 issue、发布和迁移工作量建立触发阈值；未达到阈值时只提供 skill/小队路由，不增加可见 role。
- 达到阈值后新增 `experience-validation-engineer`，交付关键用户路径、可访问性、跨端一致性和真实浏览器证据；不取代 QA 的功能回归。
- 达到阈值后新增 `migration-reviewer`，交付迁移前后数据契约、兼容窗口、回填/回滚/重试与孤儿数据检查；不执行线上迁移。
- 两个角色都要有明确输入、输出、停止条件、权限和与 release/security/reliability 的交接。
- 迁移检查必须遵守仓库规则：无外键/级联，索引使用 `CONCURRENTLY` 且每个并发索引单独迁移文件。

## Acceptance Criteria

- [ ] 统计报告说明新增角色的触发阈值、近期开工量和独立交付物；未达阈值时测试确认不出现在默认 roster。
- [ ] 体验角色样例包含浏览器路径、可访问性检查、失败截图/日志和跨端差异结论。
- [ ] 迁移角色样例包含旧/新契约、兼容期、校验查询、回滚条件和人工审批点。
- [ ] 两个角色的模板、skill、本地化、权限和文档在模板测试中保持一致。

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
