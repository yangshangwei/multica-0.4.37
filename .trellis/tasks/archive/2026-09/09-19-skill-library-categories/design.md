# Design: 技能库分类、图标与卡片视图

## 1. 数据模型

### 1.1 存储位置

`skills.config` JSONB 新增键 `presentation`，与现有 `origin`、`template_source` 平级：

```json
{
  "presentation": {
    "category": "engineering",
    "icon": "code"
  }
}
```

- 不新增数据库列、不新增迁移、不改 sqlc。
- 两个字段全部可选。缺失、类型错误或非法值在前端一律回退（分类 → `other`，图标 → 分类默认）。标签不在 config 里，见 §1.6。
- 服务端校验只在写入路径拒绝**非法值**，不补全缺省值，保持 config 是「用户写入什么就存什么」。

### 1.2 分类与图标定义（单一真源）

新文件 `packages/core/skills/presentation.ts`（headless，无 React、无 lucide 依赖）：

```ts
export const SKILL_CATEGORIES = ["research", "writing", "engineering", "operations", "data", "other"] as const;
export type SkillCategory = (typeof SKILL_CATEGORIES)[number];
export const SKILL_CATEGORY_DEFAULT_ICON: Record<SkillCategory, SkillIconName>; // microscope / pen-line / code / rocket / database / book-open-text
export const SKILL_ICON_NAMES = [...] as const;        // ~40 个 kebab-case Lucide 名称
export type SkillIconName = (typeof SKILL_ICON_NAMES)[number];
export interface SkillPresentationMeta { category: SkillCategory; icon: SkillIconName | null }
export function readSkillPresentationMeta(config: Record<string, unknown> | undefined): SkillPresentationMeta; // 容错解析
export function writeSkillPresentationMeta(config, meta): Record<string, unknown>; // 合并写回，其他键不动
```

- `icon` 存 kebab-case（`pen-line`），与 Lucide 官方名称一致，服务端和前端共用同一份字符串集。
- Go 侧镜像常量：`server/internal/skill/presentation.go` 里 `Categories`、`IconNames`，并加一个测试读取 `packages/core/skills/presentation.ts` 的字面量做对拍（模式同 `reserved-slugs`：TS 与 Go 列表必须一致）。

图标组件映射放在 views 层 `packages/views/skills/lib/skill-presentation-icon.ts`：`SKILL_ICON_COMPONENTS: Record<SkillIconName, LucideIcon>`，显式命名导入，避免全量引入 lucide。分类语义色 `SKILL_CATEGORY_TONE: Record<SkillCategory, string>` 用 tokens.css 现有语义类（如 `bg-info/10 text-info`）；若 tokens 缺少足够的语义色，在 `packages/ui/styles/tokens.css` 补 `--skill-category-*` 变量而不是写 Tailwind 调色板。

现有 `SkillIcon`（技能实体图标）不动，`other` 分类默认图标复用它。

### 1.3 服务端校验

`server/internal/skill/presentation.go` 新增 `ValidatePresentation(config map[string]any) error`：

- `presentation` 缺失或 `nil` → 通过。
- 非对象 → 400 `config.presentation must be an object`。
- `category` 非字符串或不在枚举 → 400。
- `icon` 非字符串或不在白名单 → 400。

调用点：`skill.go` 的 Create、Update（`req.Config != nil` 分支）、`finishSkillImport`、`resolveImportSkillConflict` 的 overwrite 路径、模板投放 `template-draft` 走的 Create 路径（自然覆盖）。

### 1.4 内置角色模板预设

`builtin_role_skills.go` 的 `RoleSkillTemplate` 增加 `Category`、`Icon` 字段，来源于各 `SKILL.md` frontmatter 的 `metadata.category` / `metadata.icon`（写进 8 个 SKILL.md）。`ParseSkillFrontmatter` 扩展为返回结构体 `Frontmatter{Name, Description, Category, Icon}`（保留旧签名的两个返回值封装函数以减少改动面）。`agent_template.go` 投放时把 `presentation` 写入 config。

预设：

| 模板 | category | icon |
|---|---|---|
| multica-requirement-clarification | research | message-circle-question |
| multica-architecture-decision-record | engineering | landmark |
| multica-code-review | engineering | git-pull-request |
| multica-security-review | engineering | shield-check |
| multica-test-report | engineering | flask-conical |
| multica-release-check | operations | rocket |
| multica-documentation-change | writing | book-open |
| multica-progress-report | writing | chart-no-axes-column |

### 1.5 导入种子

`finishSkillImport` 中，若 `imported` 的 SKILL.md frontmatter 解析出合法的 `metadata.category/icon`，且请求 config 未带 `presentation`，则写入。非法值静默忽略（导入不应因作者写错元数据而失败）。

### 1.6 标签：复用工作区标签（2026-09-19 变更）

自由标签与现有「标签」系统冲突，改为复用 `issue_label`（`resource_type = 'skill'`）+ `skill_to_label`。已有基础设施：`GET/POST/DELETE /api/skills/{id}/labels`、`ListLabelsForSkills` 批量查询（此前无人调用）、`labelListOptions(wsId, "skill")`、`ResourceLabelPicker`、设置页标签页签的 skill 作用域、实时 `label:*` 事件已失效 `workspaceKeys.skills`。

**服务端**

- `SkillSummaryResponse` 增加 `Labels []LabelResponse \`json:"labels"\``；`ListSkills` 用 `ListLabelsForSkills(skill_ids, workspace_id)` 一次取回并按 `skill_id` 分组填充。空数组而非 `null`。
- `CreateSkillRequest` 增加 `LabelIDs []string \`json:"label_ids,omitempty"\``；`parseUUIDSliceOrBadRequest` 解析；每个 id 用 `GetLabel(id, workspace)` 校验存在且 `ResourceType == "skill"`，否则 400 `label_ids: label not found in this workspace`；`skillCreateInput` 增加 `LabelIDs []pgtype.UUID`，`createSkillWithFilesInTx` 在同一事务内逐个 `AttachLabelToSkill`。模板投放与导入路径不传 label。
- `presentation.go` / `frontmatter.go` 删除 tags 相关常量、校验、归一化与种子；parity 测试不再读取 `SKILL_TAG_*`。

**core**

- `SkillSummarySchema` 增加 `labels: z.array(LabelSchema).optional().default([])`（旧服务端缺字段时回退空数组）；`CreateSkillRequest.label_ids?: string[]`。
- `presentation.ts` 删除 tags；`SkillPresentationMeta = { category, icon }`。
- `view-store`：列 key `tags` → `labels`；`SkillListFilters` 增加 `labels: string[]`（标签 id），`countActiveFilterDimensions` 计入。

**views**

- `ResourceLabelPicker` 增加草稿模式（`selectedIds` + `onSelectedIdsChange`，此时不请求资源标签、不发 attach/detach），对齐 issue `LabelPicker` 已有的 draft-mode 约定；`resourceId` 改为可选。
- `SkillPresentationFields` 删除标签输入与 `tagSuggestions`；删除 `use-skill-tag-suggestions.ts`。
- 新建对话框：草稿模式选择器，提交 `label_ids`。详情页：沿用已有 `ResourceLabelPicker` 行。
- 卡片：`row.skill.labels` 用 `LabelChip` 最多 3 个 + `+N`；列表「标签」列同样；搜索匹配 `label.name`。
- `useSkillListFacets` 的 `tagCounts` → `labelOptions: Map<labelId, { label, count }>`；工具栏筛选新增「标签」子菜单（色点 + 名称 + 计数），按 `filters.labels` 任一命中过滤。
- i18n：删除 `presentation.tags_*`；`table.tags` → `table.labels`、`table.tags_more` → `table.labels_more`；筛选子菜单标题复用 `table.labels`。

## 2. 视图状态（`packages/core/skills/stores/view-store.ts`）

- `SkillListFilters` 增加 `categories: SkillCategory[]`（`EMPTY_SKILL_FILTERS` 同步）。现有 `merge` 深合并已能给旧持久化补默认值。
- 新增 `viewMode: "card" | "list"`（默认 `card`）与 `setViewMode`；纳入 `partialize`。
- 新增 `SkillColumnKey`：`category`、`labels`；`COLUMN_WIDTHS` 加 `category: 120`、`labels: 176`；`DEFAULT_HIDDEN_COLUMNS` 加 `labels`。`SkillListFilters` 增加 `labels`。
- `SkillSortField` 增加 `category`（按分类枚举顺序，其次名称）。

## 3. 页面结构（`packages/views/skills/`）

```
skills-page.tsx
├─ CollectionPageHeader（不变）
├─ SkillListToolbar（+ 视图切换按钮组 + 分类 chips 仅在窄容器渲染）
└─ div.flex
   ├─ SkillCategorySidebar（新，≥@2xl 显示，w-56）
   └─ 主区域
      ├─ viewMode === "list" → 现有 ListGrid（+ 图标 / 分类列 / 标签列）
      └─ viewMode === "card" → SkillCardGrid（新，虚拟化）
```

### 3.1 `SkillCategorySidebar`

- 输入：`allRows`（未筛选行）、`filters`、`onToggleFilter`。
- 分类项：全部（清空 `categories`）+ 6 项，各显示图标、名称、计数；计数从 `allRows` 派生，不受当前筛选影响（与现有工具栏计数逻辑一致）。
- 选中态：单选语义（点击一个分类 = 设 `categories=[key]`；再次点击 = 清空），用 `data-active` 且在 hover 之外的维度表达（字重 + 文本色）。
- 来源二级组：复用工具栏 `originCounts` 的派生逻辑，抽成 `useSkillListFacets(rows)` hook 供工具栏与侧栏共用，避免两份计数。
- 窄容器：同一份数据渲染为 `SkillCategoryChips`（横向 `overflow-x-auto`，`snap-x`），放在工具栏下方。用 `@container` 类切换显隐，不用 JS 测宽。

### 3.2 `SkillCardGrid` / `SkillCard`

- 网格用 CSS grid `repeat(auto-fill, minmax(240px, 1fr))`；虚拟化按**行**做：先用 `ResizeObserver` 取容器宽度算每行列数，再把 rows 分块喂给 `useVirtualizer`，每块渲染一行卡片。固定卡高 `CARD_HEIGHT = 172`，与列表的固定行高契约一致。
- 卡片内容自上而下：`[图标 40px 分类色底 | 名称 + 锁定图标 | 右上 checkbox + kebab]`、描述 `line-clamp-2`、标签 chips（`LabelChip`，最多 3 个，其余 `+N`）、底部智能体头像叠放（`ActorAvatar` 复用，最多 3 个 + 计数）与来源小图标。
- 整卡是链接：复用 `useRowLink` / `rowLinkInteractiveProps`，与列表行同一套点击/修饰键语义。
- Checkbox 常驻但低对比，hover / 已选时提升；kebab 复用 `SkillRowActions`，传同一个 `SkillActionsContext`。

### 3.3 列表视图改动

- 名称单元格前渲染 `SkillPresentationIcon`（size-6，分类色底，圆角）。
- 新增「分类」列（图标 + 名称）和「标签」列（chips，超出截断）。
- `GRID_COLS` 模板加两条 CSS var 轨道，`FIXED_TRACKS_WIDTH` 的 gap 数相应 +2。

### 3.4 编辑入口

- 新组件 `SkillPresentationFields`（分类 Select、图标 Picker）供新建对话框和详情页共用。图标 Picker 是一个 Popover 网格，仅列出白名单图标；顶部有「跟随分类」选项（写 `icon: null`）。标签走 `ResourceLabelPicker`（新建对话框用草稿模式）。
- 详情页：头部大图标（size-12）；编辑态显示 `SkillPresentationFields`，保存走现有 `api.updateSkill` 的 `config` 合并（用 `writeSkillPresentationMeta`，保留 `origin` 等键）。
- 批量工具栏：「设置分类」下拉，逐个 `updateSkill` 合并写 `presentation.category`，用现有批量更新的并发/失败提示模式。
- 分类空状态：主区域无行且 `filters.categories.length === 1` 时，渲染 `CollectionPageState` 变体，「新建」按钮打开对话框并预填该分类。

## 4. i18n

`packages/views/locales/{zh-Hans,en,ja,ko}/skills.json` 新增：

- `categories.{key}.name`（6 项）、`categories.all`、`categories.empty_title`、`categories.empty_hint`。
- `toolbar.view_card` / `toolbar.view_list` / `toolbar.section_categories` / `toolbar.section_sources`。
- `table.category` / `table.labels` / `table.labels_more`。
- `presentation.category_label` / `presentation.icon_label` / `presentation.icon_follow_category`。
- `actions.set_category`。

中文措辞遵循 `conventions.zh.mdx`。

## 5. 兼容性

- 旧客户端读到带 `presentation` 的 config 时忽略未知键（`SkillSchema` 是 `loose()`，config 是 `record`），不受影响。
- 新客户端对旧服务端：服务端不校验 `presentation` 但会原样存储，功能可用只是缺少后端保护，可接受。
- 导出/导入 zip 与 CLI 不需改动：config 整体透传。
- Mobile 不改；它对 config 的读取不涉及 presentation。

## 6. 权衡记录

- **JSONB 而非列**：避免迁移与全链路类型改动；按分类筛选在前端做，列表接口本来就全量返回 summary。若未来要服务端分页 + 分类筛选，再迁列，届时 `readSkillPresentationMeta` 是唯一改动点。
- **图标白名单而非任意 Lucide 名**：控制 bundle 与视觉一致性，也让 Go 侧能校验。
- **分类色不可覆盖**：保证同分类卡片有统一色调，是「可分类查看」的视觉基础。
- **按行虚拟化卡片网格**：`useVirtualizer` 只处理一维；按行分块是最小改动，且列数变化时只需重算分块。
- **不自动归类存量技能**：避免错误分类带来的信任问题，用批量「设置分类」降低手工成本。
