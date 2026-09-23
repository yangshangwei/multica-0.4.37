# Implement: 软件研发生命周期分类与多维标签

按依赖顺序执行。每一步先补会失败的回归测试，再改实现。

## Step 1 - 锁定分类契约  ✅

- [x] 在 `packages/core/skills/presentation.test.ts` 先断言八项顺序、八个默认图标及 `design`/`quality` round-trip。
- [x] 更新 `packages/core/skills/presentation.ts`：保留旧 key，插入 `design`/`quality`，调整默认图标。
- [x] 更新 `server/internal/skill/presentation_test.go` 与 parity 预期，再更新 `presentation.go` 镜像。
- [x] 覆盖非法值继续回退/拒绝、旧六值继续有效。（`TestValidatePresentation` / `TestNormalizePresentation` 更新为 wrench 默认）

验证：`pnpm --filter @multica/core test -- skills/presentation`；`cd server && go test ./internal/skill/...`。

## Step 2 - 展示映射与多语言  ✅

- [x] 在 `packages/ui/styles/tokens.css` 为 `design`/`quality` 增加 theme alias、light/dark token。
- [x] 在 `packages/views/skills/lib/skill-presentation-icon.ts` 增加 tone，保持所有映射对枚举穷尽。
- [x] 更新 `use-skill-category-labels.ts` 和 zh-Hans/en/ja/ko 的 `skills.json`（八项新显示名）。
- [x] 更新 presentation/sidebar/chips 测试，固定八项顺序、label、tone 和 icon。

验证：`pnpm --filter @multica/views test -- skills`；locale parity 测试；`pnpm typecheck`。

## Step 3 - 内置 role skill 默认归类  ✅

- [x] 先更新 `TestRoleSkillTemplates_PresentationDefaults` 为新矩阵。
- [x] 修改四个 SKILL.md frontmatter：ADR→design，code-review/security-review/test-report→quality（其余四项分类不变）。
- [x] 确认 materialization 仍只影响新副本（未新增覆盖已物化 skill 的路径）。

验证：`cd server && go test ./internal/service/... -run 'RoleSkill.*Presentation'`，以及相关 template handler 测试。

## Step 4 - 分类全链路 UI  ✅

- [x] 更新 sidebar、chips、facets、详情/新建字段、批量分类的测试数据与断言（新英文显示名 + 八项/九 chips）。
- [x] 验证所有 surface（sidebar/chips/toolbar/presentation-fields/list-actions/facets/skills-page）都直接迭代 `SKILL_CATEGORIES`，无手写六项列表。
- [x] 旧 persisted filter 回归：view-store `merge` 深合并 filters；facets 由 `SKILL_CATEGORIES` 建桶、行分类经 `readSkillPresentationMeta` 容错，未知值不触发 `.length`/计数/排序异常。
- [x] 分类空状态预填分类的创建路径由 `skills-page.test.tsx` 覆盖。

验证：`pnpm --filter @multica/core test -- skills/stores`；`pnpm --filter @multica/views test -- skills`。

## Step 5 - 批量管理标签  ✅

- [x] 为 `skill-list-actions.test.tsx` 增加全选(all)、部分(some)、未选(none)、无权限跳过和部分 API 失败用例（9 个新测试）。
- [x] 增加纯计算 helper `deriveSkillLabelState`（仅统计可编辑行的三态）与 `setSkillsLabel`；`ManageLabelsMenu` 复用 Popover + 色点 + 权限语义。
- [x] 复用 `api.attachLabelToResource`/`detachLabelFromResource` 顺序执行，聚合 updated/failed/skipped；结束仅 `workspaceKeys.skills` invalidation，无乐观更新，无新 batch endpoint。
- [x] 未改动详情 label picker 与创建 `label_ids` 路径。

验证：`pnpm --filter @multica/views test -- skill-list-actions labels`；`pnpm typecheck`。

## Step 6 - 文档与行为契约  ✅

- [x] 更新 `apps/docs/content/docs/skills.mdx` 与 `skills.zh.mdx`：新增「Organizing skills / 组织 skill」章节，含八主分类表、分类/标签/来源边界、推荐标签词表、标签不授予权限的 Callout。
- [x] importing SKILL.md / source map 仅泛指 "a fixed category set"，未枚举分类值，无需同步。
- [x] 四语显示名按产品文案规范措辞。

## Step 7 - 全量验证和视觉验收

- [x] `pnpm typecheck` — 9/9 packages 通过。
- [x] `pnpm lint`（core/ui/views）— 0 errors（仅无关既有 warning）。
- [x] 定向 `pnpm test`：core `skills/presentation`（全绿）、views skills 全套 —— 初测报告"i18n guard 绿"有误：zh-Hans/ja/ko 混入死 `_one` 复数键，parity 实际 3 例失败；审计通道删除死键后复核 443 files / 5365 tests 全绿。
- [x] Go：`go test ./internal/skill/...`、`./internal/service/...`（Presentation/RoleSkill/AgentRoleTemplates）通过。
- [x] 独立 `trellis-check` 审计 — 结论 pass-with-notes，报告见 `check-report.md`。
  - 修复 1（审计通道）：删除 zh-Hans/ja/ko 的 `manage_labels_*_toast_one` 死键（违反 conventions「只填 `_other`」，parity 曾 3 例失败）；修复后主会话独立复跑全量确认。
  - 修复 2（主会话）：`skill-category-sidebar.tsx` 过期的 "six categories" 注释改为引用 `SKILL_CATEGORIES`；审计通道仅复核未编辑。
  - 3 条非阻塞备注：读屏下 tri-state 部分/未选均为 `aria-pressed="false"`（与既有 `AgentPickerRow` 同形，建议后续加视觉隐藏状态文本）、e2e spec 硬编码八分类文案与顺序、`labelKeys.byResource` 批量操作后至下次 refetch 前可能过期。
- [x] Playwright 宽/窄/暗色 + 中英文截图验收 — `e2e/skill-category-taxonomy.spec.ts` 通过，13 张截图；结论见 `visual-acceptance.md`。
  - 前置修复：跑着的 api 二进制早于 `presentation.go`，`ValidatePresentation` 以 400 拒绝 `design`/`quality`；重启 api 后通过。上线顺序必须后端先行。
  - 两项未修的展示层发现：英文分类名在侧栏/分类列截断（本次加剧）、批量栏在 390px 溢出（既有，本次加宽 134px）。
- [x] `python3 ./.trellis/scripts/task.py validate 09-22-skill-lifecycle-taxonomy` — implement.jsonl(5) / check.jsonl(4) 全部通过。

## Rollback

- 代码回滚不需要数据库回滚。
- 回滚后旧 client 会把已写入的 `design`/`quality` 显示为 `other`，但 JSONB 原值不丢失；重新部署新版本可恢复。
- 不执行批量数据重写，也不删除用户已有标签。
