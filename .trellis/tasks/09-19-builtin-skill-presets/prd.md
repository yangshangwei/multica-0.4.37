# 内网可投放的 Skill 模板库

## Goal

让工作区 owner/admin 在「新建 skill → 从模板中修改」时，除了平台自带的内置 role skill，还能看到本部署（内网）自己投放的一批 skill 模板；选中后内容填入现有编辑器，保存成普通工作区 skill，再照常分配给 agent。

内网侧添加新模板的成本 = 把一个 `<name>/SKILL.md` 文件夹拷进服务器上一个约定目录，无需改代码、无需重新出离线包、无需数据库迁移。

用户价值：Multica 部署在内网、无外网，社区流行的 skill 无法通过 URL 导入；「从运行时复制」「从本地导入」都是单工作区、临时、要人肉搞一份进来。本任务提供「开箱即有一批、所有工作区都能选」+「内网自己随时扩充」两条能力，且内容主权留在部署方手里，与发行版解耦。

## Confirmed Facts（已勘察，勿再确认）

### 现有「从模板中修改」链路已打通（本任务复用，不改前端）

- 面板：`packages/views/skills/components/template-skill-create-panel.tsx`，读 `skillTemplateListOptions` → `GET .../skills/templates`。
- 后端：`server/internal/handler/skill_template.go` 的 `ListSkillTemplates` → `service.RoleSkillTemplates()`（`builtin_role_skills.go:106`），目前目录只有 8 条编译进二进制的 role skill。
- 选模板 → 编辑 → 创建 走 `use-template-skill-session.ts` + `POST /api/skills`，产出普通工作区 skill：可编辑、可删、可分配、需单独分配给 agent。
- 前端展示层已优雅兜底：`getBuiltinRoleSkillPresentation`（`packages/views/skills/lib/skill-presentation.ts:28-48`）对不在硬编码 `BUILTIN_ROLE_SKILL_NAMES` 名单里的 name 返回 null，面板 fall back 到条目自带 `description`（`template-skill-create-panel.tsx:44-48`）。因此挂载进来的模板不写四语言文案也能正常显示与搜索。
- 响应经 zod 解析：`SkillTemplateListResponseSchema`（`packages/core/api/client.ts` 附近，现有 `listSkillTemplates`）。

### 内置目录 embed 模式（本任务对齐）

- `server/internal/service/builtin_role_skills.go`：`//go:embed builtin_role_skills`，目录格式 `builtin_role_skills/<name>/SKILL.md` + 附属文件（`AgentSkillFileData{Path, Content}`）。
- frontmatter 解析：`server/internal/skill.ParseSkillFrontmatter`（读 name/description）。
- role skill 刻意 **不** 进 `BuiltinSkills()`——那批发给每个 agent 每次任务，会拉长每条 prompt（`builtin_role_skills.go:11-18` 明确警告）。本任务同样只做「待复制的模板」，不做运行时内置。

### 挂载目录先例（本任务对齐 B 方案）

- `MULTICA_PLUGIN_DIR` → `PluginService.LocalDir`（`server/internal/service/plugin.go:38,72`），从环境变量读一个服务器目录名。
- `CHANGELOG_DIRECTORY:/app/data/changelog:ro`（`docker-compose.selfhost.yml:64`）——只读挂载一个宿主目录进容器的现成写法。
- `Handler` 持有 `TaskService`（`server/internal/handler/handler.go:235`），是注入配置的落点。

### 工程约束

- 网络 JSON 必须过 `parseWithFallback` + zod（`packages/core/api/schema.ts`）。
- Skill 名称保留字与校验：`server/internal/skill/reserved.go`、frontmatter 契约。
- 内嵌/挂载静态内容，无数据库迁移。

## Requirements

- R1 后端内置 skill 模板目录来源从「仅 embed」扩展为「embed + 扫描 `MULTICA_SKILL_TEMPLATE_DIR`」；未配置或目录为空时行为与现状完全一致。
- R2 挂载目录格式 `<name>/SKILL.md` + 可选附属文件，与 `builtin_role_skills/<name>/` 同构；附属文件随模板一起进创建流程。
- R3 「从模板中修改」面板无需改动即可列出挂载模板；选中→编辑→保存产出普通工作区 skill（可编辑/可删/可分配/需单独分配）。
- R4 畸形条目（缺 SKILL.md、frontmatter 无 name / 非法名 / 目录逃逸）被跳过并记日志，不让整个模板列表崩溃或报错。
- R5 名字与 embed role skill 冲突时行为确定：embed 赢，挂载侧同名条目以告警跳过（内网文件不能覆盖平台内容）。
- R6 `ListSkillTemplates` 响应仍经现有 zod schema 解析；新增字段（若有）向后兼容。
- R7 部署文档（SELF_HOSTING / offline-bundle 说明）补一段「如何投放 skill 模板」。

## Acceptance Criteria

- [ ] AC1 服务器 `$MULTICA_SKILL_TEMPLATE_DIR/<name>/SKILL.md` 存在时，「从模板中修改」目录里能看到并选中该模板。
- [ ] AC2 选中后 SKILL.md 内容填入编辑器、可改，保存成功，产出的条目是普通工作区 skill（可编辑、可删、未被分配给任何 agent，可在 agent 页分配）。
- [ ] AC3 未设置 `MULTICA_SKILL_TEMPLATE_DIR`，或目录为空/不存在时，模板列表与现状一致（仍显示 embed role skill），无错误。
- [ ] AC4 挂载目录含畸形条目（缺 SKILL.md / frontmatter 无 name / 名称非法 / 含 `../` 逃逸）时，该条被跳过、其余正常列出，接口不 5xx。
- [ ] AC5 含 `references/*` 附属文件的模板，选中后附属文件随模板进入创建流程并落库。
- [ ] AC6 挂载条目名与 embed role skill（如 `multica-code-review`）冲突时，返回的是 embed 版本，挂载同名条目被跳过。
- [ ] AC7 `ListSkillTemplates` 响应经 zod 解析；构造一个畸形响应的前端测试证明添加流程不崩。

## Out of Scope

- 不改前端「从模板中修改」面板与创建会话逻辑（现有 UI/hook 直接复用）。
- 不做「零创建、直接勾给 agent」的运行时内置 skill——确认走三步：选模板 → 创建 → 分配（与 `builtin-mcp-presets` 结论一致）。
- 不做模板版本升级 / diff（保存即用户自己的副本）。
- 不碰 URL 导入 / 运行时复制 / 本地上传三条现有入口。
- 不为挂载模板做多语言文案后台（展示层已兜底 raw description）。

## Key Decisions

- D1 内容通道选 **B：服务器挂载目录**（而非编译进二进制），使内网可自主扩充、与发行版解耦。
- D2 分配语义走 **三步（模板→创建→分配）**，复用现有全部管线，零架构风险。
- D3 扫描时机：**每次列表请求扫目录**（目录小、丢文件即生效、无需重启）；可后续加轻缓存。若实现中发现 IO/校验成本过高，回退为启动扫一次 + 显式刷新。
- D4 名字冲突：**embed 赢**，挂载同名跳过并告警。
