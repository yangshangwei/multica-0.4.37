# 专项角色 workload gate 复核

## 取证范围

本次只统计仓库内已经归档的两个最近发布周期，不把当前未提交的计划当作工作量证据：

- `09-16-release-v0-4-46/verification.md`：91 条生产 Web/API/Electron Playwright 用例通过；另有 5 条近期回归、桌面更新/菜单、Windows ia32 预检、截图检查和 12 项离线升级持久性检查。
- `09-22-release-v0-4-47/e2e-results.md`：首轮 78 passed / 10 failed / 3 skipped，随后有多轮隔离复跑、冷编译和 SPA 导航故障定性；另有 Linux amd64 离线升级包、Windows ia32 安装包、校验和和 CI verify 证据。

这些记录证明发布周期确实反复消费浏览器、桌面和跨平台验证，但不能自动证明新的 listed role 已经必要。门槛还要求独立交付物和现有 QA 的排队/覆盖缺口。

## Gate 结果（本轮最终复核）

| 候选角色 | 两个周期的客观信号 | 独立交付物/缺口证据 | 结论 |
|---|---|---|---|
| `experience-validation-engineer` | 两个周期均有至少两条真实浏览器或桌面关键路径；v0.4.46 有截图审查，v0.4.47 有多轮 E2E 失败分类和跨端回归证据 | 发布验证记录包含可复核路径、截图/日志、失败分类和 QA 覆盖缺口；专项角色交付物将其结构化 | **达到 listed gate**；由 `qa-engineer` + Playwright/Chrome MCP + `product-analyst` 路由，并在 review/release 关键路径上独立交付 |
| `migration-reviewer` | 两个周期都有离线升级/安装交付物，并有客户端 view-store/API fallback、issue lifecycle migration、兼容修复和迁移 review 记录 | 每周期均有旧/新契约、兼容窗口、校验/校验和、重试或回滚限制的独立材料 | **达到 listed gate**；由 `architect` + `security-reviewer` + `release-engineer` 路由，迁移证据缺失时阻塞发布 |

## 重新评估条件

若未来连续两个周期出现独立交付物缺失，需要重新评估 listed role：

- 体验：每周期至少两条需要真实浏览器/可访问性/跨端证据的关键路径；交付物必须包含路径、截图/日志、可访问性结果和跨端差异结论。
- 迁移：每周期至少一项 schema、API 或客户端兼容迁移；交付物必须包含旧/新契约、兼容窗口、校验、回滚/重试条件和人工审批点。

若未来门槛失守，暂停新副本创建并将专项路由降级为现有角色组合；现有 workspace 副本不被删除或覆盖。当前版本已完成四语模板、权限、skill、squad 和正负向 roster 测试。

## 其他常用场景复核

当前已由既有角色/skill 覆盖的场景：需求澄清、架构兼容、实现与回归、代码审查、安全/供应链、依赖升级与不稳定测试、发布/回滚、同产物观察、事故止血/RCA/复盘、SLO/容量/灾备、产品结果、文档和 Agent/Skill/MCP 评测。

暂不新增角色或 MCP 的场景：隐私/合规、FinOps、客户支持反馈和自动生产控制。这些目前没有独立且稳定的交付物证据；隐私/供应链由安全与发布审查覆盖，成本由 Agent Evaluator/Progress Reporter 记录，生产控制继续保留人工审批和无凭据 MCP 边界。
