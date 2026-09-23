# Design: 软件研发生命周期分类与多维标签

## 1. Decision

扩展现有 `SkillCategory`，不替换其存储模型：保留六个 key，新增 `design` 与 `quality`，通过 i18n 更新显示语义。工作区 skill 标签继续作为多选横向属性；本次增加批量挂接能力，但不增加标签分组字段。

分类顺序：

```text
research -> design -> engineering -> quality -> operations -> writing -> data -> other
```

这让前五项形成从规划到运行的主要交付链路，后三项承载贯穿多个阶段的支撑能力。分类回答“主要归属”，标签回答“还具有什么属性”。

## 2. Alternatives

### A. 保留 key，新增两类（采用）

- 优点：无数据库迁移；六类存量数据立即获得新显示名；旧客户端与新 backend 仍共享旧值；改动沿用现有单一真源和 parity 测试。
- 代价：内部 key 与新显示名不是逐字对应；旧客户端会把两个新 key 显示为 `other`。

### B. 全部替换为新的生命周期 key（拒绝）

- 优点：内部命名最纯粹。
- 拒绝原因：需要重写所有 JSONB；旧 Desktop 会回退或覆盖新值；新 backend 若拒绝旧 key 会造成写入兼容问题。

### C. 新增数据库 `lifecycle_stage` 与结构化标签维度（暂缓）

- 优点：模型最严格，可按维度分组标签。
- 暂缓原因：当前 `category + skill labels` 已覆盖单选主分类和多选属性；新增列、迁移、API 和设置页分组的成本没有被当前需求证明。

## 3. Contract Changes

### 3.1 Core and backend

- 在 `packages/core/skills/presentation.ts` 和 `server/internal/skill/presentation.go` 同步增加两个值及默认图标。
- 继续由 TS 常量作为前端单一真源，并由 Go parity 测试保证镜像一致（当前契约见 `packages/core/skills/presentation.ts:1-26`、`server/internal/skill/presentation.go:7-23`）。
- `readSkillPresentationMeta` 的容错和 `writeSkillPresentationMeta` 的规范化语义不变。
- API 形状不变，仍通过 `config.presentation.category` 读写；因此不新增 sqlc 查询和迁移。

### 3.2 Presentation

默认图标：

| key | icon |
| --- | --- |
| `research` | `list-checks` |
| `design` | `landmark` |
| `engineering` | `code` |
| `quality` | `clipboard-check` |
| `operations` | `rocket` |
| `writing` | `book-open-text` |
| `data` | `database` |
| `other` | `wrench` |

现有显式 icon override 保持不变。默认 icon 本身不存储，因此未覆盖图标的 skill 会在升级后直接采用更匹配的新默认。

`design` 和 `quality` 各增加语义 token 的 theme alias、light 值和 dark 值；映射继续集中在 `SKILL_CATEGORY_TONE`，不在组件中硬编码颜色（现有位置见 `packages/views/skills/lib/skill-presentation-icon.ts:121-147`）。

### 3.3 Built-in role skills

更新八个 `server/internal/service/builtin_role_skills/*/SKILL.md` 的 frontmatter，并同步 `TestRoleSkillTemplates_PresentationDefaults`（当前断言见 `server/internal/service/builtin_agent_templates_test.go:399-430`）。

| role skill | category |
| --- | --- |
| requirement clarification | `research` |
| architecture decision record | `design` |
| code review | `quality` |
| security review | `quality` |
| test report | `quality` |
| release check | `operations` |
| documentation change | `writing` |
| progress report | `writing` |

已物化副本不自动更新。管理员可用现有批量分类功能重新整理存量 skill。

## 4. Label Workflow

### 4.1 Data model

不改数据模型。标签仍是 workspace-scoped `issue_label` 行，`resource_type = 'skill'`，通过 `skill_to_label` 关联（`server/migrations/162_resource_labels.up.sql:5-22`）。标签名称中的 `/` 只是团队命名约定，任何业务逻辑都不得解析它。

### 4.2 Batch interaction

在现有批量工具栏中增加 `Manage labels` 菜单，复用 workspace skill labels：

- 只统计和修改 `canEdit` 的选中项；不可编辑项计入 skipped。
- 每个 label 显示三态：全部拥有、部分拥有、全部没有。
- 若全部拥有，点击执行从全部可编辑 skill 移除；否则点击执行为全部可编辑 skill 添加。
- 调用现有 attach/detach API，按 skill 顺序执行并聚合 `updated / failed / skipped`。沿用 `setSkillsCategory` 的容错和最终 toast 模式（`packages/views/skills/components/skill-list-actions.tsx:663-729`）。
- 完成后失效 workspace skills query；不做乐观更新，避免部分失败时回滚多个实体。

第一阶段不新增批量后端端点。若真实工作区出现明显延迟，再以独立任务增加原子或分块 batch API。

## 5. UI and i18n

- `useSkillCategoryLabels`、四种 `skills.json`、sidebar、chips、toolbar、presentation fields 和批量菜单都从八项枚举推导，不维护第二份排序。
- 宽栏现有 `w-52` 在中文应能容纳最长的四字分类名与计数；实现后必须用实际截图验证，不能仅凭单测判断（当前 sidebar 尺寸见 `packages/views/skills/components/skill-category-sidebar.tsx:50-56`）。
- 新增两个分类后，sidebar 可滚动，窄屏 chips 保持横向滚动；不压缩字体或用 viewport 字号。
- 标签推荐词表进入中英文 skills 文档，不塞进列表页面作为解释性文案。

## 6. Compatibility and Rollout

| 组合 | 行为 |
| --- | --- |
| 新 backend + 新 client | 支持八类 |
| 新 backend + 旧 client | 六个旧 key 正常；新 key 容错显示为 `other`；普通保存不覆盖 config |
| 旧 backend + 新 client | 旧 backend 会拒绝 `design`/`quality` 写入；客户端展示真实错误，不静默改类 |

发布不需要数据 migration。回滚代码后，已写入的 `design`/`quality` 会被旧 UI 当成 `other`，但原 JSONB 值仍在；再次升级即可恢复显示。不要添加把新值批量改成 `other` 的 down migration。

## 7. Test Shape

- Core：枚举顺序、默认图标白名单、两类 round-trip、旧值解析、非法值回退、view-store 旧持久化筛选兼容。
- Server：TS/Go parity、默认 icon 完整性、`design`/`quality` 校验与 frontmatter seed、role skill 分类矩阵。
- Views：sidebar/chips 八项顺序与计数、筛选/排序、新建/详情/批量分类、批量标签三态和部分失败、locale parity。
- Visual：Web/Desktop，宽/窄容器，亮/暗模式；检查文本、计数、横向滚动、选中态和分类色辨识。
