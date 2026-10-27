# 当前 AI 小队与安全修复路径盘点

日期：2026-10-26

## 已确认事实

- `server/internal/service/builtin_squad_templates.go` 当前注册 8 个内置小队：feature-delivery、bug-fix、review-gate、discovery、docs、maintenance、release、incident。
- `maintenance` 的成员为 security-reviewer、diagnostician、implementer、qa-engineer、migration-reviewer；其指令要求 CVE/安全公告先做适用性审查，一次只处理一个依赖/公告，并由实现和 QA 分离完成。
- `review-gate` 的安全审查员只审查已完成 diff，不修复；`incident` 负责正在发生的生产影响，先缓解/回滚，生产动作由 release-engineer 在人工审批后执行。
- `security-reviewer` 为 Observer，职责覆盖授权、输入处理、凭据、数据暴露、依赖和资源限制；配套 `multica-security-review` skill 增加威胁建模、跨租户信任边界、供应链/SBOM 和 `hold`/`unknown` 规则。
- `apps/docs/content/docs/agents.mdx` 与 `server/internal/service/builtin_agent_autonomy.go` 定义 Observer/Contributor/Coordinator/Operator 四级边界；高风险动作包括 production release、live database migration、secret access、external notification、destructive operation，均须逐项人工审批。
- `apps/docs/content/docs/security-model.mdx` 明确 daemon 默认继承操作系统用户权限，Multica 不提供通用文件系统沙箱；建议使用专用用户、容器或虚拟机。
- `apps/docs/content/docs/squads.mdx` 明确 leader 通过精确 @mention 路由，成员角色说明不授予权限，模板创建默认保留私有 Access。
- `packages/core/api/schemas.ts` 的 `SquadTemplateSchema` 使用字符串 key 和 loose schema；新增模板条目可向后兼容地加入列表，前端目录对未知 key 使用后端 description 回退。

## 设计结论

- 阶段一复用现有 maintenance-lead 和工作角色，在非生产运行时创建“安全修复小队（试运行）”，通过三个合成 issue 验证依赖、代码和其他风险三类路由。
- 阶段二新增 `security-remediation` 内置 squad 与未列出的 `security-remediation-lead` Coordinator；复用现有角色和 role skills，不新增数据库、审批风险类别、扫描器或生产执行通道。
- 生产可利用/凭据或客户数据暴露事项转 incident/release/人工路径；安全修复小队只准备非生产证据和方案。
