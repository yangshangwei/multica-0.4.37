# 交付治理与灾备能力

## Goal

覆盖容易被单点角色遗漏的交付治理与恢复场景，并把它们路由到已有角色或经工作量证明的新角色。

## Requirements

- 增加或补齐五类可复用能力：契约兼容、威胁建模、供应链风险、产品结果验证、灾备/恢复验证。
- 契约兼容检查 API/OpenAPI、桌面旧版本和插件边界；威胁建模检查信任边界、滥用路径和缓解措施；供应链检查依赖、构建产物和来源；产品结果检查目标指标与实际结果；灾备检查 RPO/RTO、备份恢复和降级路径。
- 优先复用 `architect`、`security-reviewer`、`release-engineer`、`qa-engineer`、`product-analyst` 和 `reliability-engineer`，只有独立交付物和工作量成立时才新增角色。
- 每项能力必须声明触发条件、输入、输出、负责角色、验证命令/信号和人工审批边界。
- 不自动读取生产凭据、不自动执行线上迁移/发布/恢复、不把合规结论伪装成测试通过。

## Acceptance Criteria

- [ ] 五类能力各有一个模板化 skill 或明确复用的现有 skill，并有触发/产物/验证矩阵。
- [ ] 契约兼容覆盖旧客户端响应漂移和插件边界；安全与供应链覆盖高风险依赖/构建来源；结果验证覆盖指标缺失；灾备覆盖恢复演练证据不足。
- [ ] 至少一个发布门禁小队演练串起兼容、安全、供应链、结果和灾备检查，并报告跳过项与人工审批点。
- [ ] 服务器模板、workspace 复制保护、权限、本地化、文档和回归测试一致。

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
