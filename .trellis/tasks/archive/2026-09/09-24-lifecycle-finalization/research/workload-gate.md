# 专项角色 workload gate 复核

## 判定口径

本轮复核使用最近两个已归档发布周期的记录。角色进入 listed roster 需要连续两个周期同时满足：

- 体验验证：每周期至少两条真实浏览器/桌面关键路径，并出现截图、失败分类或可访问性/跨端证据；角色交付物必须独立记录路径、环境、证据和结果风险。
- 迁移审查：每周期至少一项 schema、API 或客户端兼容迁移，并有独立的旧/新契约、兼容窗口、校验以及回滚/重试限制证据。

## 来源与计数

| 周期 | 体验证据 | 迁移证据 |
|---|---|---|
| v0.4.46 | `09-16-release-v0-4-46/verification.md`：91 条 Web/API/Electron Playwright 通过；桌面更新/菜单、Windows ia32 预检、截图审查；12 项离线升级持久性检查 | `cacf7b92d` 客户端 view-store v1 migration 与 API schema fallback；同周期包含 issue lifecycle migration、编号调整和兼容修复记录 |
| v0.4.47 | `09-22-release-v0-4-47/e2e-results.md`：首轮 78 passed / 10 failed / 3 skipped，随后隔离复跑、失败分类、冷编译和 SPA 导航回归；Linux amd64 与 Windows ia32 安装验证 | Linux amd64 升级包、Windows ia32 包校验与离线升级记录；release-period 记录包含 migration review 和兼容窗口检查 |

## 结论

两个周期都超过体验路径的最低数量，并且已有截图/失败分类/跨端差异证据；两个周期都有独立的客户端/API/schema 迁移和兼容、校验或回滚材料。因此本轮 workload gate 达标，新增 `experience-validation-engineer` 与 `migration-reviewer` listed roles。

这不是生产质量或真实模型通过声明：发布记录仍是脱敏仓库证据，未读取生产指标、凭据或客户数据；角色只消费已批准的测试/迁移材料，生产动作继续由 Release Engineer 在人工审批后执行。
