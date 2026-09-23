# Visual Acceptance: 八分类生命周期与批量标签

Playwright spec: `e2e/skill-category-taxonomy.spec.ts`（新增，随本任务提交）
运行环境: `multica_0_4_37-492`，web `:13492`（`next dev`，工作树代码），api `:18572`
结果: **passed**，13 张截图 + `verification.json`

## 覆盖的表现形态

| 截图 | 视口 | 证明了什么 |
| --- | --- | --- |
| `wide-light-card-sidebar` | 1440×900 | 卡片视图（默认）侧栏八分类顺序、计数、八种独立色调 |
| `wide-light-list` | 1440×900 | 列表视图 + Labels 列，分类列逐行渲染 |
| `wide-light-category-filtered` | 1440×900 | 选中 design 后只剩一行，hover 下仍保持 `aria-current` |
| `wide-light-selection-bar` | 1440×900 | 两行选中，批量栏出现 |
| `wide-light-label-menu-tristate` | 1440×900 | 三态：全选 ✓ / 部分 − / 未选空白 |
| `wide-light-label-menu-after-add` | 1440×900 | 点部分标签后提升为全选 |
| `wide-dark-list` / `wide-dark-label-menu` | 1440×900 暗色 | 暗色 token 是独立值，非亮色复用 |
| `narrow-light-chips` / `-scrolled-end` | 390×844 | 侧栏让位给九枚 chips，横向滚动可达末项 |
| `narrow-light-selection-bar` / `-label-menu` | 390×844 | 窄屏批量栏与标签菜单 |
| `wide-light-sidebar-zh` | 1440×900 | 中文八分类顺序与显示名 |

机器可读断言（`verification.json`）：分类写入/读出 round-trip 一致、亮色八种色调互不相同、暗色八种互不相同且与亮色不等、chips 九枚且溢出滚动、三态验证、部分标签提升后两行标签集合一致。

## 发现

### 1. 英文分类名在侧栏与分类列被截断（本次改动引入）

1440px 宽视口下，侧栏 208px 固定宽，八项里六项截断：

- `Planning & requ…`、`Design & archit…`、`Development & …`、`Release & oper…`、`Collaboration & …`、`Data & automati…`
- 只有 `Testing & quality`、`General tools` 完整显示。

列表的「分类」列同样截断成 `Data & autom…`、`Design & arch…`、`Development …`。

旧六项显示名（`Research & analysis`、`Writing & communication`…）本来也长，但这次把八项全部换成了「A & B」双词组合，平均更长，截断面扩大。中文/日文/韩文不受影响——`wide-light-sidebar-zh` 里八项全部完整。

这是纯展示层问题，不影响数据，也不影响 vitest（jsdom 无布局）。可选处理方向：侧栏加宽、英文文案改短、或给截断项加 `title`。**未改动**——属于文案/布局取舍，该由你定。

### 2. 批量操作栏在窄视口溢出（本次改动加剧）

390px 视口实测（独立探针，已删除）：

```
barWidth 690px, left -150, right 540, viewport 390
按钮宽度: Clear 18 / Add to agent 124 / Set category 123 / Manage labels 134 / Update 89 / Delete 84
```

两侧各裁掉 150px：左边 `Add to agent` 被切，右边 `Update`/`Delete` 被切，`2 selected` 计数完全不可见。

容器本身（`absolute bottom-6 left-1/2 -translate-x-1/2`）不是这次改的，改动前约 552px，同样溢出 390px。所以**溢出是既有缺陷，本次新增的 `Manage labels`（134px）把它加宽了约 24%**。

spec 里这条写成 `expect.soft`：它只校验 `Manage labels` 自身边界，而那颗按钮恰好落在视口内，所以软断言通过了——真正的溢出是看截图才发现的。这条留作记录，**未改动**：修法（换行、图标化、收进 kebab 菜单）是设计决定。

### 3. 展示设置触发器没有可访问名（既有，本次未触碰）

`skill-list-toolbar.tsx` 的 Display popover 触发器只渲染当前排序字段（`Updated` + 箭头），字符串「Display」只存在于 `TooltipContent`。读屏用户听到的是「Updated」，与列头排序按钮同名——我写 spec 时两者撞名，只能按结构定位。

与本任务无关，顺手记录。

## spec 自身的两处注意

- `page.setDefaultTimeout(20_000)` 是必需的：Playwright 的 click 默认继承整个 test 预算，一个解析不到的选择器会静默吃掉 5 分钟而不报是哪一句。四次 `goto`/`reload` 因此显式给了 120–180s，冷编译 `/skills` 实测 137s。
- 主题用 `emulateMedia({ colorScheme })` 而非 `localStorage.theme`：`addInitScript` 会在每次 reload 重放，写死主题会把暗色切换悄悄改回亮色。
