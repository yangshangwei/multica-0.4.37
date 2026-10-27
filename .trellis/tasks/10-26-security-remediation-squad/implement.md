# 执行计划：安全漏洞修复小队两阶段落地

## 阶段 0：规划完成后再启动

- [x] 向用户呈现 `prd.md`、`design.md` 和本清单，获得对最终规划摘要的明确批准。
- [x] 运行 `python3 ./.trellis/scripts/task.py start .trellis/tasks/10-26-security-remediation-squad`，确认任务进入 `in_progress`。
- [x] 不在 `task.py start` 前修改产品代码、模板注册表或运行生产操作。

2026-10-27 续接沿用已获批准的阶段一和“恢复旧 runtime”选择，不重新建任务。重试后状态为 `phase1_real_results_routing_gate_open`：模型可执行、四个父案例与八个独立成员子任务均已交到 `in_review`，另有一次真实 leader → 成员委派已证明，但成员模型额度耗尽后取消、无结果，且派发误记 `no_action`；完整返回/收口、职责分离、报告准确性和网络门槛未闭合，阶段二未启动。证据见 `pilot/trial-results.md`。

## 阶段一：当前工作区试点

### A1. 准备安全边界

- [x] 选择非生产仓库/分支和隔离工作目录。
- [ ] 使用专用 Unix 用户、容器或虚拟机运行 daemon；确认 HOME、代码挂载和网络范围符合最小权限。
- [x] 不注入生产数据库密码、长期云凭据、个人 SSH key 或可写生产 MCP；需要的依赖公告/镜像信息使用只读来源。
- [x] 记录运行时、代码 ref、模型/skill 版本和可复现的试点时间窗口。

容器文件系统、provider UID 1000 和 root 私有 daemon profile 的拒绝边界已验证；默认 bridge 的出站访问未受限制，因此网络最小权限条目保持未完成。

### A2. 组装试点小队

- [x] 复用现有角色创建或选择：maintenance-lead、product-analyst、security-reviewer、diagnostician、implementer、qa-engineer、migration-reviewer、code-reviewer。
- [x] 核对自治等级：leader=Coordinator，security/product/migration/code review=Observer，diagnostician/implementer/QA=Contributor；发布工程师不加入。
- [x] 将 Access 显式设为试点成员可运行的最小范围；记录任何 widened Access 的理由。
- [x] 创建“安全修复小队（试运行）”，把阶段一路由和统一交接字段写入 squad instructions。
- [x] 记录成员重用情况；不要覆盖已有 workspace-owned 指令或 skill。

8 个角色的 Access 均保留 `private`，没有扩大或重绑。角色来源、指令摘要及 6 个去重 skill 的内容摘要已记录在 `pilot/evidence/`。A3 的四个案例均有成功真实任务，独立成员子任务 `POLA-7`～`POLA-14` 已完成；两次修复仍由 leader 直接实现，成员审查由操作者补派，A3/A4 的完整路由验收仍未通过。试点小队指令随后通过成对内容/时间前置条件更新为 `pilot/runtime/pilot-instructions-v2.md`，共享角色和 roster 不变。

### A3. 演练三个安全事项

- [ ] 组件漏洞：使用一个脱敏/合成依赖公告，覆盖受影响性判断、单项升级、锁文件/来源、回归验证和安全复审。
- [ ] 代码漏洞：使用一个本地可复现的权限/输入/数据边界案例，覆盖安全审查、必要时诊断、最小修复、负面测试和独立 QA。
- [ ] 其他风险：使用配置/供应链/Agent 或运行边界案例，覆盖 `hold`/`unknown`、证据缺口 owner 和条件性专家交接。
- [x] 至少让一个案例走过 migration-reviewer 分支；若没有实际契约变化，记录跳过理由。
- [ ] 至少让一个未知根因案例走过 diagnostician 分支；保留无修复时失败的最小用例。
- [x] 至少演练一次“当前生产可利用、凭据暴露或客户数据暴露”的输入：leader 先请求人工，再转 incident/release；确认没有生产命令被执行。

本轮执行证据：`POLA-4` 依赖补丁与 `POLA-5` 授权补丁经隔离实验室和可移植回放均六项通过；`POLA-6` 与真实安全审查子任务保持供应链 `hold/unknown`；`POLA-3` 交接草案及三个枚举纠正已留证。迁移分支由操作者手动派发 `POLA-7` 验证，不等于 leader 自主路由；`diagnostician`、`implementer` 本人的交接尚未执行。组件/代码完整路由条目继续保持未完成。

### A4. 试点验收

- [ ] 从 issue 评论和 activity 记录中回读每个案例的 `type`、`severity`、`evidence_state`、`production_impact`、`route`、`blocked_actions` 和 `next_owner`。
- [x] 由未修改代码的成员完成 QA/安全复审；确认评论没有 secret、客户原始数据或可用 exploit。
- [x] 汇总误分流、重复交接、缺失证据、角色指令冲突和需要产品支持的决策。
- [ ] 只有三类案例均满足闭环且所有 blocking gap 有 owner 时，才创建阶段二实现子任务。

独立身份已通过子任务 task/评论核对，但部分报告仍有错误摘要、测试计数和权限推断；独立审查完成不表示报告内容全数可采信。操作者需回读原始命令更正这些误述，完整分诊字段和真实自主交接仍待验收。最后使用显式小队 mention 触发真正 leader 并修正成员 Markdown 链接后，已生成精确 `delegation` 的安全审查员任务；成员因匿名额度耗尽于 12:09:35 取消、无结果，leader 派发时误记 `no_action`。实际新回合 briefing 投影已保存，但旧回合注入情况未知；一次派发不完成成员返回或全链验收。最终容器已停止、四个 namespace 无排队/活动任务、临时认证副本清理完毕，见 `pilot/evidence/routing-final-cleanup-20261027.json`。

## 阶段二：固化内置能力

### B1. 新增内置 leader 和 squad 模板

- [ ] 在 `server/internal/service/builtin_agent_templates_roster.go` 注册未列出的 `security-remediation-lead`，自治等级 Coordinator、并发上限 2、版本 1。
- [ ] 新增 `server/internal/service/builtin_agent_templates/security-remediation-lead/INSTRUCTIONS.md`，只包含试点已验证的安全分流、停止条件、交接和父任务状态规则。
- [ ] 在 `server/internal/service/builtin_squad_templates.go` 增加 `security-remediation`，版本 1、双语标题/描述、leader 和成员说明。
- [ ] 新增 `server/internal/service/builtin_squad_templates/security-remediation/INSTRUCTIONS.md`，保持与 leader/成员指令一致；禁止新增未在试点中证明的职责。
- [ ] 不新增数据库表、迁移、审批风险类别或新的执行模型。

### B2. 服务器测试与边界回归

- [ ] 更新角色/小队注册表测试：leader 一对一、template key/version、语言、角色文件存在、最大自治等级为 Coordinator、roster 不含 Operator。
- [ ] 更新 handler 模板列表/创建测试：新模板出现在列表，能够完整创建/复用 roster，默认 Access 仍私有，Coordinator 可以创建而不能越过自治上限。
- [ ] 为安全模板指令写最小文本契约测试：三类路由、incident/release 交接、禁止 exploit/secret/production 动作、`unknown`/`hold` 规则存在。
- [ ] 不添加真实攻击载荷；安全回归使用权限拒绝、租户边界、空/超限输入、锁文件/版本解析等安全 fixture。

### B3. 前端、API 和文档

- [ ] 在 `packages/views/squads/components/builtin-squad-catalog.tsx` 的摘要映射中加入 `security-remediation`；保留后端 description 回退。
- [ ] 在 `packages/views` 的 squads 中英文/中文 locale 添加模板摘要，避免新模板显示为未翻译的后端文本。
- [ ] 更新 `apps/docs/content/docs/squads.mdx`、`squads.zh.mdx`、角色模板和安全模型相关说明，写清试点结论、权限和事故交接。
- [ ] 确认 `SquadTemplateSchema` 的字符串 key/loose array contract 无需改变；若新增字段才按 API 兼容规则添加 schema 与 malformed-response 测试。
- [ ] 如仓库文档生成流程要求，同步 `server/internal/docs/content/pages/` 的生成副本，不手工修改生成产物以外的内容。

### B4. 完整验证与交付

- [ ] 运行服务器窄测：`(cd server && go test ./internal/service ./internal/handler -run 'Test(AgentRoleTemplates|SquadTemplates|ListSquadTemplates|CreateSquadFromTemplate)' -count=1)`。
- [ ] 运行前端相关测试：`pnpm test`（或按 Turborepo filter 运行 `packages/core`、`packages/views` 的模板/小队测试）和 `pnpm typecheck`。
- [ ] 运行相关 E2E：构建后的应用上执行 `e2e/squads-audit.spec.ts`、`e2e/project-squad-workspace.spec.ts` 以及新增的模板创建/权限回归；不要用 `next dev` 冷编译结果替代验证。
- [ ] 运行 `make check`，记录通过数、失败项和未运行范围。
- [ ] 用 `trellis-check` 完成 spec、lint、typecheck、测试、跨层数据流和权限边界复核。
- [ ] 先在 `in_review` 交付，不由小队或其 leader 标记 `done`；准备 Lore commit trailers 后再提交。

## 风险和回滚点

- 试点误把可利用生产问题当例行维护：立即停止，保留证据，交 incident/release 和人工；不得继续本地修复以替代止血。
- 试点暴露需要凭据或客户原始数据：标记 `unknown`，清理输出，向人工请求脱敏材料；不扩大 Access 或环境权限。
- 阶段二模板/文案冲突：不发布该模板，先修改 leader briefing 和测试；不得靠放宽自治等级解决路由问题。
- 阶段二发布后发现路由缺陷：优先前向修复并递增模板版本。尚未有人创建时可回退注册表；已有 squad 继续作为普通 squad 运行，不能删除成员或依赖记录。
- 若 API/旧客户端兼容证据不足，保持 `hold`，补充 schema/旧客户端矩阵后再发布。
