# 技术设计：内网可投放的 Skill 模板库

## 架构总览

```
宿主机                                     容器 (backend)
$MULTICA_SKILL_TEMPLATE_DIR/               TaskService.SkillTemplateDir
  my-debug-helper/                           │
    SKILL.md              ── :ro 挂载 ──▶     │ 每次请求扫描
    references/foo.md                         ▼
  team-code-style/                     SkillTemplates()
    SKILL.md              =  embed(8 条 role skill)  +  scan(挂载目录 N 条)
                                              │  名字冲突 → embed 赢
                                              ▼
                              GET .../skills/templates  (现有 handler)
                                              │  zod: SkillTemplateListResponseSchema
                                              ▼
                        template-skill-create-panel.tsx  (不改)
                              选 → 编辑 → POST /api/skills  (不改)
                                              ▼
                              普通工作区 skill：可编辑/可删/可分配
```

核心判断：**前端与创建链路完全复用，改动集中在后端「模板目录来源」这一个点。**

## 边界与契约

### 后端配置注入

- `TaskService` 增加字段 `SkillTemplateDir string`（对齐 `PluginService.LocalDir`）。
- 在构造 `TaskService` 的地方（与 `PluginService` 读 `MULTICA_PLUGIN_DIR` 同一处初始化代码，`server/internal/service/plugin.go:72` 附近的 server 装配）从 `os.Getenv("MULTICA_SKILL_TEMPLATE_DIR")` 读取并 `TrimSpace`。
- 空字符串 = 未启用，`SkillTemplates()` 仅返回 embed（保证 AC3）。

### 目录来源合并函数

新增/改造一个方法（方案：在 `builtin_role_skills.go` 旁新增 `skill_template_dir.go`，把包级 `RoleSkillTemplates()` 保留为 embed-only，再由 `TaskService` 提供 `SkillTemplates()` 合并）：

```
func (s *TaskService) SkillTemplates() []RoleSkillTemplate {
    embed := RoleSkillTemplates()            // 现有，embed-only，稳定顺序
    names := set(embed.name)                 // 用于冲突检测
    mounted := scanSkillTemplateDir(s.SkillTemplateDir, names)  // 跳过同名 + 畸形
    return append(embed, mounted...)         // embed 在前
}
```

`ListSkillTemplates` handler 从 `service.RoleSkillTemplates()` 改调 `h.TaskService.SkillTemplates()`。响应结构 `SkillTemplateResponse` 不变（name/version/description/content/files）——所以前端与 zod schema 无需改。挂载条目 `Version` 取 0（或约定值），表示「非平台发行版内容」。

### 挂载目录扫描规则（scanSkillTemplateDir）

对 `$DIR` 下每个一级子目录 `<name>`：

1. **名称校验**：`<name>` 必须过现有 skill 名规则（`^[A-Za-z0-9_-]+$` 等，复用 `server/internal/skill` 校验 / 保留字），否则跳过 + `slog.Warn`。
2. **路径安全**：只接受一级目录名，拒绝含 `/`、`\`、`.` 前缀（对齐 `plugin_package.go:86` 的约定），防目录逃逸。
3. **读 `<name>/SKILL.md`**：不存在 → 跳过 + warn。
4. **frontmatter**：`ParseSkillFrontmatter` 取 description；frontmatter 里的 name 与目录名不一致时以**目录名**为准（对齐 plugin skill：manifest key 权威，`plugin_skill.go` 同款理由）。
5. **冲突**：`<name>` 已在 embed 集合里 → 跳过 + warn（D4，embed 赢）。
6. **附属文件**：递归收集 `<name>/` 下除 SKILL.md 外的文件为 `AgentSkillFileData{Path: 相对路径, Content}`，套用现有 per-file / 总大小 / 文件数上限（复用 import 的 `maxImportFileSize` 等常量或等价约束）。
7. 稳定排序（按 name），使列表与测试不依赖目录遍历顺序。

任何单条目失败都是「跳过该条」而非「整个列表失败」（AC4）。

### 前端

无代码改动。验证点：
- `getBuiltinRoleSkillPresentation` 对挂载 name 返回 null → 面板用 raw `description` 渲染 + 搜索（已存在行为，加一个测试锁定）。
- `SkillTemplateListResponseSchema` 已能解析挂载条目（同结构）；补一个 malformed-response 测试（AC7）。

## 数据流

1. 管理员把 `<name>/SKILL.md` 放进宿主机 `$MULTICA_SKILL_TEMPLATE_DIR`。
2. compose 只读挂载该目录进 backend 容器。
3. 用户打开「新建 skill → 从模板中修改」→ `GET .../skills/templates`。
4. `SkillTemplates()` = embed + 实时扫挂载目录，合并去冲突。
5. 面板渲染；选中 → 内容进编辑器 → `POST /api/skills` 创建工作区 skill。
6. 用户到 agent MCP/skill 页分配。

## 兼容性

- 未配置环境变量：`SkillTemplateDir == ""` → 纯 embed，与现状字节级一致（AC3）。
- 响应结构不变：已安装的旧前端也能解析，无 API drift。
- 无数据库迁移；embed role skill 行为不变。

## 权衡

- **每次请求扫目录 vs 启动扫一次**：选前者（丢文件即生效、运维直觉最简）。目录规模是「一个部署投放的模板数」，通常几十条以内，一次 `ReadDir` + 读文件成本可接受。若压测发现问题，回退启动扫 + 显式刷新端点（不改契约）。
- **embed 赢 vs 挂载赢**：选 embed 赢，保证平台自带内容不被内网文件意外顶替；内网若想「改平台模板」，正途是投放一个不同 name 的模板。

## 运维/回滚

- 回滚 = 删代码改动 + 移除 compose 挂载行；无数据残留、无迁移回退。
- 文档在 SELF_HOSTING / offline-bundle 说明补「投放 skill 模板」小节。

## 风险

- R-1 目录扫描的路径安全（逃逸/软链）——复用 plugin 的一级目录名约束 + 显式拒绝 `.` 前缀与分隔符。
- R-2 大文件/超多条目拖慢列表——套用 import 侧现有大小/数量上限，超限条目跳过并告警。
- R-3 挂载条目缺多语言文案导致搜索命中差——可接受：raw description 已进 searchText；后续可选增强。
