# Implement: 技能库分类、图标与卡片视图

按层从内到外推进，每一步都可独立验证。TDD：先写失败测试再实现。

## Step 1 — core 层定义与解析器（`packages/core/skills/`）

- [ ] 新建 `presentation.ts`：分类枚举、图标白名单、标签上限、`readSkillPresentationMeta`、`normalizeSkillTags`、`writeSkillPresentationMeta`。
- [ ] 新建 `presentation.test.ts`（`// @vitest-environment node`）：缺字段 / 非对象 / 非法分类 / 非法图标 / 标签去重截断 / 写回保留 `origin`。
- [ ] `index.ts` 导出。
- [ ] `stores/view-store.ts`：`categories` 筛选维度、`viewMode` + `setViewMode`、新列 key、`category` 排序字段；补 `view-store.test.ts` 覆盖旧持久化 payload 合并。

验证：`pnpm --filter @multica/core test -- skills`、`pnpm typecheck`。

## Step 2 — Go 服务端校验与预设（`server/internal/`）

- [ ] `skill/presentation.go`：常量 + `ValidatePresentation` + `NormalizePresentation`；`presentation_test.go` 表驱动。
- [ ] `skill/presentation_parity_test.go`：读取 `packages/core/skills/presentation.ts` 字面量与 Go 常量对拍。
- [ ] `skill/frontmatter.go`：新增 `ParseSkillFrontmatterMeta` 返回结构体（含 `metadata.category/icon/tags`），旧函数改为其封装；补测试。
- [ ] `handler/skill.go`：Create / Update / `finishSkillImport` / overwrite 路径调用校验；导入路径种子填充；`skill_test.go` 用 `testutil.Call` 加 400 与落库用例。
- [ ] `service/builtin_role_skills.go`：`RoleSkillTemplate` 加 `Category` / `Icon`；8 个 `builtin_role_skills/*/SKILL.md` 补 `metadata`；`handler/agent_template.go` 投放写入 `presentation`；扩展 `agent_template_test.go` 断言 config。
- [ ] 若 `builtin_skills/multica-skill-importing/SKILL.md` 描述了 config 结构，同步更新它及 `references/*-source-map.md`。

验证：`cd server && go test ./internal/skill/... ./internal/service/... && go test ./internal/handler/ -run 'Skill|AgentTemplate'`（DB-backed，需按记忆中的模板库方式提供 DATABASE_URL）。

## Step 3 — views 层展示原语（`packages/views/skills/lib/`、`packages/ui/styles/`）

- [ ] `lib/skill-presentation-icon.ts`：`SKILL_ICON_COMPONENTS`、`SKILL_CATEGORY_TONE`、`resolveSkillIcon(meta)`；测试断言白名单每一项都有组件。
- [ ] 如 tokens 不足，在 `packages/ui/styles/tokens.css` 增加 `--skill-category-*` 变量（明暗两套）。
- [ ] `components/skill-presentation-icon.tsx`：接受 `meta` 与 `size`，渲染分类色底图标。
- [ ] i18n：四语 `skills.json` 新键；跑 `packages/views/locales/parity.test.ts`。

## Step 4 — 侧栏、chips、facets

- [ ] `hooks/use-skill-list-facets.ts`：从行派生分类计数、来源计数、标签集合；工具栏改用它。
- [ ] `components/skill-category-sidebar.tsx` 与 `skill-category-chips.tsx`；测试：计数、单选切换、清空、`data-active`。
- [ ] `skills-page.tsx` 接入：布局、`@container` 显隐、`categories` 参与过滤与 `countActiveFilterDimensions`。

## Step 5 — 卡片视图

- [ ] `components/skill-card.tsx`、`skill-card-grid.tsx`（按行虚拟化）。
- [ ] 工具栏视图切换按钮组（`ToggleGroup`，若 ui 缺则 `pnpm ui:add toggle-group`）。
- [ ] `skills-page.tsx` 按 `viewMode` 分支；空状态分类变体。
- [ ] 测试：`skill-card.test.tsx`（内容、勾选、kebab、链接语义）；`skills-page.test.tsx` 增加视图切换持久化、分类空状态用例。

## Step 6 — 列表视图与编辑入口

- [ ] 列表：名称列图标、分类列、标签列、轨道模板与宽度常量、列设置面板项。
- [ ] `components/skill-presentation-fields.tsx` + `hooks/use-skill-tag-suggestions.ts`；测试标签输入规则与图标 picker 的「跟随分类」。
- [ ] 新建对话框、详情页接入；批量「设置分类」。
- [ ] 测试：`create-skill-dialog.test.tsx`、`skill-detail-page.test.tsx`、`skill-list-actions.test.tsx` 各加对应用例。

## Step 7 — 全量验证与收尾

- [ ] `pnpm typecheck && pnpm lint && pnpm test`
- [ ] `make test`（注意记忆中的 sweeper 与 TestGitEnv 已知失败）
- [ ] 在 `make up` 环境里用浏览器实际截图卡片视图、列表视图、窄容器 chips、暗色模式，确认视觉。
- [ ] 更新 `.trellis/spec/views/frontend/` 中技能相关约定（presentation 的存储与解析入口）。
- [ ] 按 conventional commit 分次提交：`feat(core)`、`feat(server)`、`feat(views)`、`docs`。

## Step 8 — 标签改用工作区标签（2026-09-19 变更，覆盖 Step 1–6 中的 tags 部分）

服务端与前端可并行；契约见 design.md §1.6。

服务端：
- [ ] `skill/presentation.go`：删除 `TagLimit` / `TagMaxLength` / `NormalizeTags` / tags 校验与归一化；`PresentationFromFrontmatter` 不再写 tags；`frontmatter.go` 删除 `Tags` 与 `frontmatterTags`；对应测试与 parity 测试同步。
- [ ] `handler/skill.go`：`SkillSummaryResponse.Labels`，`ListSkills` 用 `ListLabelsForSkills` 批量填充；`CreateSkillRequest.LabelIDs` 校验（存在、同工作区、`resource_type == "skill"`）后在 `createSkillWithFilesInTx` 内挂接；`skill_presentation_test.go` 删除 tags 用例，新增列表内嵌 labels、非法 `label_ids` 400、合法 `label_ids` 落库三个用例（`testutil.Call`）。
- [ ] `builtin_skills/multica-skill-importing/SKILL.md` 与 `references/skill-importing-source-map.md`：去掉 tags 描述，补 `label_ids` 与列表 `labels`。

前端：
- [ ] `core/skills/presentation.ts`（+test）：删除 tags；`api/schemas.ts` `SkillSummarySchema.labels`；`types` `CreateSkillRequest.label_ids`；`api/schemas.test.ts` 补 labels 缺失/畸形回退用例。
- [ ] `core/skills/stores/view-store.ts`（+test）：`tags` 列 → `labels`；filters 增 `labels`。
- [ ] `views/labels/resource-label-picker.tsx`：草稿模式。
- [ ] `views/skills`：删除 `use-skill-tag-suggestions.ts(+test)`；`skill-presentation-fields.tsx` 去掉标签输入；新建对话框接草稿选择器并提交 `label_ids`；`skill-card.tsx` / `skills-page.tsx` 用 `LabelChip`；`use-skill-list-facets.ts` `labelOptions`；`skill-list-toolbar.tsx` 「标签」筛选子菜单；sidebar / chips / list-actions 的 `meta` 字面量去掉 `tags`。
- [ ] 四语 `skills.json`：删 `presentation.tags_*`，`table.tags*` → `table.labels*`；`parity.test.ts`。
- [ ] 测试：`skill-card.test.tsx`、`skills-page.test.tsx`（标签列、标签筛选、搜索命中标签名）、`create-skill-dialog.test.tsx`（提交 `label_ids`）、`skill-detail-page.test.tsx`、`skill-list-actions.test.tsx`、`use-skill-list-facets.test.ts`、`skill-presentation-fields.test.tsx`。

验证：`pnpm --filter @multica/core test`、`pnpm --filter @multica/views test -- skills labels`、`pnpm typecheck`、`cd server && go test ./internal/skill/... && go test ./internal/handler/ -run 'Skill'`（DB-backed，按记忆克隆模板库）。

## 回滚点

- Step 2 之前：纯前端，回滚即删文件。
- Step 2 之后：服务端只增加校验，不改表；回滚移除校验调用即可，已落库的 `presentation` 键对旧代码无害。
