# “新建协作任务”菜单审计

日期：2026-11-02。使用 impeccable audit，界面模式为 Operate。
范围：Web / Desktop 共用的创建弹窗、接收人选择器、初始字段与附件入口。
源码未修改；未创建 Trellis 任务。

实现一致性通过：使用现有语义颜色、字号、Base UI 弹窗和业务文案。机械检测器对 creation-form、account-picker、attachment-picker、field-inputs 返回 0 项。行为完整性仍存在下述缺陷，机械检测不覆盖这些状态与读屏问题。

| 维度 | 分数 | 依据 |
|---|---:|---|
| 可访问性 | 2/4 | 选项及键盘高亮未暴露给读屏；浅色焦点环对比不足 |
| 性能 | 3/4 | 父任务搜索逐字请求完整选项 |
| 主题 | 4/4 | 使用语义 token；明暗主题正文对比均通过 AA |
| 响应式 | 3/4 | 无横向溢出、合成触摸可打开选择器；部分触控目标和长表单操作区需改善 |
| 实现完整性 | 2/4 | 成功反馈可丢失；拒绝原因与身份信息不足 |
| **合计** | **14/20** | **良好，优先修补创建反馈与可访问性** |

共 8 项：P0 0 项、P1 3 项、P2 5 项、P3 0 项。

## P1：优先修复

### 创建中切换工作空间，成功结果失去反馈

位置：[来源选择器](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/collaboration/creation-form.tsx:65)、[创建回调](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/collaboration/creation-form.tsx:96)。

证据：延迟创建响应时，来源选择器仍启用。切换到另一个空间，再返回模拟成功结果，弹窗仍打开，新空间标题为空，导航调用为 0，原创建恢复记录已清除。

影响：用户无法从当前弹窗确认先前任务已创建，可能重复操作或遗漏新任务。

建议：请求待确认时锁定来源；若产品允许切换，由不会随来源卸载的层维护回执。保留现有 active / scope 防护，不要删除账号与 API scope 检查来解决反馈问题。
类别：实现完整性。命令：`$impeccable harden`。

### 接收人列表缺少完整读屏语义

位置：[列表容器](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/collaboration/account-picker.tsx:140)、[键盘高亮](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/issues/components/pickers/property-picker.tsx:211)、[选项组件](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/issues/components/pickers/property-picker.tsx:327)。

证据：容器有 `role=listbox`，其中 `role=option` 数量为 0；选项没有 `aria-selected`。按方向键只改变背景类，焦点仍在搜索框，搜索框没有 `aria-activedescendant` 或 combobox 角色。

影响：读屏无法获知当前浏览的候选项与已选状态。

建议：建立完整 combobox/listbox 语义，关联搜索框与列表，为候选项提供稳定 ID、选中状态和活动项关系，保留 IME 处理与 Escape 层级。
类别：可访问性。标准：WCAG 4.1.2 / WAI-ARIA listbox、combobox 模式。命令：`$impeccable harden`。

### 浅色焦点环对比不足

位置：[焦点色 token](/Volumes/artisan/code/2026/multica-0.4.37/packages/ui/styles/tokens.css:206)、[来源选择器样式](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/collaboration/creation-form.tsx:65)。

证据：浅色 ring 约为 #9f9fa9，对白色弹窗的对比为 2.62:1，低于 3:1。文本框还使用 ring/50，应一并检查实际合成后的颜色。

影响：键盘用户难以识别当前操作位置。

建议：提高焦点环与相邻背景的对比；调整共享 token 时检查其他共享页面的焦点状态。
类别：可访问性。标准：WCAG 1.4.11 非文本对比。命令：`$impeccable colorize`。

## P2：下一轮改进

### 拒绝原因被统一提示覆盖

位置：[错误呈现](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/collaboration/creation-form.tsx:174)、[保留错误码的类型](/Volumes/artisan/code/2026/multica-0.4.37/packages/core/collaboration/creation.ts:14)。

证据：模拟 `issue_limit_reached` 后，只提示“任务未创建，草稿和文件已保留。你可以再次提交。”按钮重新启用，没有显示数量已达上限。

影响：用户会重复提交，不知道需要处理什么条件。

建议：将已有 rejection code 映射为具体原因与后续操作，继续保留草稿和恢复机制。
类别：实现完整性。命令：`$impeccable clarify`。

### 同名接收人无法区分

位置：[接收人行](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/collaboration/account-picker.tsx:121)。

证据：两个不同 user_id、相同姓名、同为内部账号的样例，显示相同头像首字与姓名，没有可见身份补充信息。

影响：共享对象可能选错。证明的是辨识缺陷，没有执行或观察真实误共享。

建议：增加可辨别的用户名或账号 ID 辅助行，并在确认时保留身份信息；不要展示当前权限不允许读取的信息。
类别：实现完整性。命令：`$impeccable clarify`。

### 部分触控目标小于 44px

位置：[接收人选项](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/issues/components/pickers/property-picker.tsx:334)、[关闭按钮](/Volumes/artisan/code/2026/multica-0.4.37/packages/ui/components/ui/dialog.tsx:70)、[父任务搜索与阶段输入](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/collaboration/field-inputs.tsx:107)。

证据：390×740，pointer:coarse=true、1 个触点。接收人四行均高 32px，关闭按钮 28×28px，父任务搜索和阶段输入高 32px。合成触摸能打开选择器。

影响：误触概率较高，尤其是相邻接收人。

建议：粗指针下增加至少 44px 命中区域，视觉图标可保持原尺寸。
类别：响应式。标准：Impeccable 44px 建议 / WCAG 2.5.5 AAA；44px 不是 WCAG AA 的 24px 最小目标要求。命令：`$impeccable adapt`。

### 长表单将提交与关闭操作滚出视野

位置：[整个弹窗滚动](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/collaboration/creation-form.tsx:29)、[底部操作](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/collaboration/creation-form.tsx:181)。

证据：390×740，弹窗高 666px、默认内容高 840px，提交按钮不在初始视野。展开后内容高 1688px，滚到底部时关闭按钮 top=-977px，已不可见。

影响：填写、提交与退出需要额外滚动；查看底部内容时不能直接点击关闭。

建议：表单主体滚动，保留可见操作区，在底部提供取消/关闭；避免固定区域遮挡键盘焦点。
类别：响应式。命令：`$impeccable layout`。

### 父任务搜索逐字请求完整选项

位置：[输入](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/collaboration/field-inputs.tsx:107)、[查询状态](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/collaboration/creation-form.tsx:79)、[查询 key](/Volumes/artisan/code/2026/multica-0.4.37/packages/core/collaboration/creation.ts:18)。

证据：逐字输入“协作任务改进”，产生“协”“协作”“协作任”“协作任务”“协作任务改”“协作任务改进”共 6 次请求调用。

影响：慢网下增加不必要请求、完整选项目录解析与刷新。

建议：复用接收人搜索的 250ms 防抖模式，让输入立即更新、查询在输入停顿后执行；保留权限刷新与取消机制。
类别：性能。命令：`$impeccable optimize`。

## 共性问题与优点

问题集中在两处：共享 PropertyPicker 的语义与命中区域；创建表单的异步反馈、错误原因和滚动操作区。应在对应共享边界修复，避免 Web / Desktop 各写一套。

- 明暗主题稳定后的辅助文字对比为 5.91:1 / 6.34:1，超过 AA 普通文本的 4.5:1。
- 主表单使用标签、原生输入与 required；Escape 先关闭选择器，保留创建弹窗。
- 初始字段渐进展开，主要输入与粗指针提交按钮达到 44px。
- 320×568 下弹窗宽 288px，无横向溢出。
- 拒绝后标题草稿保留；上传与创建结果恢复机制应继续保留。

## 验证与边界

已执行：
`pnpm --filter @multica/views exec vitest run collaboration/creation-form.test.tsx collaboration/account-picker.test.tsx collaboration/field-editing.test.tsx`
结果：3 个文件、15 个测试全部通过。现有用例未覆盖本次新增发现。

浏览器使用原始组件、共享 CSS 与中文资源，在隔离 Chromium 中模拟两个工作空间、同名账号、额度拒绝和延迟成功响应；草稿写入内存。没有真实协作任务提交，不是 Electron Main 的原生桌面验收。触控证据来自合成 touchStart/touchEnd，未验证物理触屏或 iOS Safari。ARIA 来自 DOM / AX 检查，未进行真实读屏朗读验收。

主题与尺寸切换有 100ms CSS 过渡。最终读数与截图等待过渡结束后采样，瞬时混色、初始过高等暂态未计为缺陷。

[结构化证据](/tmp/impeccable-collaboration-audit-20261102/results-confirmed.json)。
[浅色弹窗](/tmp/impeccable-collaboration-audit-20261102/desktop-light.png)；
[深色弹窗](/tmp/impeccable-collaboration-audit-20261102/desktop-dark.png)；
[接收人](/tmp/impeccable-collaboration-audit-20261102/desktop-recipient.png)；
[窄屏](/tmp/impeccable-collaboration-audit-20261102/narrow-default.png)；
[展开后的底部](/tmp/impeccable-collaboration-audit-20261102/narrow-expanded-bottom.png)；
[切换来源后的成功响应](/tmp/impeccable-collaboration-audit-20261102/created-after-source-switch.png)。

建议顺序：
1. `$impeccable harden`：创建期间来源锁定、回执与接收人 ARIA。
2. `$impeccable colorize`：焦点环对比。
3. `$impeccable clarify`：拒绝原因与接收人身份。
4. `$impeccable adapt`、`$impeccable layout`：触控目标与长表单操作区。
5. `$impeccable optimize`：搜索防抖。
6. `$impeccable polish`：修复后的统一校验。

这些建议可以分别或一起执行；修复后再运行 `$impeccable audit` 验证。

环境附记：启动预览时误用了会自动清理环境的仓库命令，它已清理部分过期测试环境并重启本地 API。启动命令已中止；审计的隔离浏览器已关闭，源码未改。
