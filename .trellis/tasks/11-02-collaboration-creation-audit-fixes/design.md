# 技术设计

## 改动边界

修复发生在共享 views 的创建与接收人交互边界。Core 继续拥有原请求、回执关联、草稿/附件退役与账号/source access epoch；UI 不重新实现这些机制。没有新 API、新 store、新依赖或平台路由。

预期产品文件：creation-form.tsx（R1/R4/R7/R8）、account-picker.tsx（R2/R5/R6）、field-inputs.tsx（R3/R6）、issues/components/pickers/property-picker.tsx（R2 可选关系与内容样式入口）、collaboration/labels.ts（R4 本地化映射）及 en/zh-Hans collaboration 资源。必要时用一个小型局部样式常量文件统一本次焦点覆盖；不添加行为抽象。相关 colocated tests 为回归边界。

共享 Dialog/Input/Textarea/Button/tokens 的源文件优先保持不动，使用调用方的语义 class 和 opt-in 内容 class 修复此弹窗。若实际测量迫使共享调整，先记录原因、受影响调用方和扩大检查清单，再由主会话确认符合本授权范围。

### 新的并行实现基线

计划评审期间，11-02-collaboration-source-picker 任务在相同文件新增父任务 popup、失败重试、权限否决时卸载，以及 PropertyPicker.searchMaxLength。以 research/pre-implementation/manifest.json、inherited.diff.gz 和逐文件快照为实现前基线，全部保留。SelectedCreationSource 的 isCollaborationDenied 分支、经授权的 Query fallback、optionsError/onRetryOptions 与 isPlaceholderData 判定不可移除。field-inputs 的父任务 popup、候选加载/错误/空反馈、触发器描述与 focus retry 不可退回旧 input+select。两 locale 已有的字段文案和新 source-picker tests 保留。

## R1：来源锁定与回执

上层 CollaborationCreationPanel 已订阅 Core delivery record。组合当前账号/API 下的进行中 creation/recovery mutation（沿用 useIsMutating）与内容完整的未确认 creation/upload record，派生来源锁定；select 同时 disabled，change 处理也拒绝锁定期间的更新。关联必须限于 creation context，避免普通 issue 或其他账号操作误锁。

记录在发送前保存，但成功时先于 mutation callback 退役，故不能只用记录锁定。保持 active ref 和 isCollaborationScopeCurrent；权限撤销仍允许现有私有输入清理，不为保留回执而绕过权限。input:null 的闲置恢复记录不永久禁用新来源，其 in-flight recovery 则需要锁定。

明确拒绝保留可编辑的 form/files；未知响应继续保留不可变请求与原 GET/key 恢复。主动关闭沿用现有草稿机制，此修复不扩展跨关闭成功通知的产品行为。

## R2：兼容的 ARIA 与键盘

PropertyPicker 仍是可含普通操作按钮的通用 Popover；仅本次 account picker opt-in。保留原生 textbox 角色与 Filter options 名称，为它设置指向实际 listbox 的 aria-controls 与 aria-activedescendant。WAI-ARIA 1.2 允许 textbox 控制拥有活动后代的 listbox；不声称实现 APG combobox 角色，不在普通 textbox 上添加 combobox 专属状态。

账号 picker 用每实例 useId + 规范化 account identity 建立 listbox/option ID。option 的 aria-selected 反映已提交选择，aria-disabled 反映可选性；方向高亮只更新活动后代，保持搜索 DOM 焦点。多选 listbox 标记 aria-multiselectable。

沿用现有 committed-candidate sync、navigationResetKey、typed search priority、append-only paging、IME/Safari-229 守卫及 Escape 层级。paint/sync 清空或更换活动 ID 时必须与 DOM 候选同步；关闭、移除、空结果不留悬空引用。footer/exact lookup 操作不进入列表方向导航。非 opt-in 调用方不自动获得 listbox/option 角色。

## R3/R6：局部焦点与触控

复用现有 faint-foreground 语义 token，使用不透明 focus-visible ring/outline/border；覆盖 Input/Textarea/Button 原 ring/50 的颜色和透明度。优先沿用 className/cn 的覆盖机制，必要时加入通用 contentClassName 入口，不能用业务专属逻辑污染共享 picker。

Portal 内容不继承弹窗祖先 class，必须显式覆盖接收人搜索、精确输入、按钮与焦点候选。按实际编译 CSS 与相邻背景采样，而非只检查 token。静态预测 light/dark 的 solid faint-foreground 分别约 3.87/4.22:1，仅用于方向选择。

粗指针 option/close/search/stage 使用 min-height/min-width 44px；图标不放大，行可随身份换行增高。DialogContent 在本调用方扩大 close 命中区，并为 header 预留空间。细指针保持产品密度。父任务搜索现已位于 portaled PropertyPicker，也需显式局部 focus/coarse 样式；field-inputs 用于 source 编辑，需覆盖相关回归。

## R4/R5：拒绝与身份

只对 CollaborationCreationRejectedError 的已确认 code 呈现明确拒绝。沿用现有 labels 的 selector/switch + default 模式，为 26 个已知 code 按账号/来源、附件、父任务/项目、额度/待审、字段/迭代分组提供具体原因和动作；未知 code 使用事实兜底，不把网络不确定性伪装成拒绝。issue_limit_reached 不再建议单纯重试，说明由管理员处理上限条件；保留原 draft-preservation 句。

当前 projection 仅有 user_id/name/avatar/external。列表、精确结果与单选 trigger 使用完整 user_id 辅助行，允许换行并进入可访问名称/描述；不新增用户名/邮箱请求，不用 UUID 截断前缀作为唯一身份。已有 Selected account: <name> 句作为独立元素保留，避免破坏另一个会话的 exact-text fixture。external/null 与 membersOnly 判断不变。

## R7：主体滚动、操作固定

复用 triage-create-dialog 的 normal-flow flex 模式：DialogContent max-height 90dvh、overflow-hidden，header shrink-0，主体 min-h-0/flex-1/overflow-y-auto/overscroll-contain，操作 shrink-0。内边距容纳 focus ring，footer 不悬浮遮挡内容。

保留原生 form submit/required 验证。通过可选 onCancel/受限布局入口保持 CreationPanel 的独立可用性，不能在独立 panel 中无条件调用 DialogClose。弹窗无来源、加载/错误和有正常 form 时都要能取消；成功仍关闭并通过 NavigationAdapter 打开原任务。避免额外 scroll observer 或把 submit 改成 click-only。

## R8：搜索防抖

SelectedCreationSource 保持立即显示 raw parentSearch，独立 effect 在 250ms 停顿后设置 trimmed query，值变更或卸载清除定时器；沿用账号搜索的已有模式。现有 scope/source key 重挂载隔离定时器。collaborationCreationOptions 的 key、AbortSignal、placeholderData、权限 mount/focus/poll 与 optionsPending 判定保持原样。

raw trimmed search 与 debounced query 不一致的 250ms 窗口也应视为 optionsPending，不能在新输入下选择旧候选。继承的晚到搜索回归要等待原查询实际开始后再切换，保留晚响应不污染的断言；不得删掉测试以适配防抖。

## 取舍与回退

| 选择 | 未采用方案 | 原因 |
|---|---|---|
| 锁来源，保留现有 owner | 在全局新建回执 owner 或删除 scope guard | 前者超范围，后者破坏账号隔离 |
| textbox 控制 opt-in listbox | 全部 picker 转 combobox/listbox | generic picker 有操作按钮，且已有 E2E textbox 选择器 |
| 局部 opaque faint-foreground | 全局 ring/token/Input 改造 | 全局 /50 仍可能不达标，影响面过大 |
| 完整已有账号 ID | 扩展 API 返回邮箱/用户名或截断 ID | 权限/隐私契约不允许扩展，截断可能重名 |
| 正常流 flex header/body/footer | 整弹窗滚动或叠加悬浮操作 | 前者丢操作，后者遮挡焦点 |

无迁移。按文件局部回退即可；不碰其他会话改动。八项逻辑/语义与 UI 变更都在同一任务内，P1 验证完成后再验收 P2。

### 首轮浏览器修正（仍属 R3/R4/R7）

实际 Chromium 显示，native picker 搜索的原 outline-none 仍压制仅设置宽度/颜色的局部规则，computed outline 为 none 0px。显式 focus-visible:outline-solid 与现有 opaque faint-foreground/width/offset 一起使用；同类 native option/summary 规则也保持明确绘制。将同一局部焦点样式传给本表单的标签 popup，入口按钮也用 opaque token；不扩展自定义属性编辑器公共 API 或全局 UI。

固定 Submit 使用户能从主体顶部提交，原来放在主体底部的拒绝/结果未知反馈可能不在视野中。在受限创建表单中保留一个反馈锚点，相关 mutation 落到错误或未确认恢复状态后仅将主体滚到该反馈（block:nearest），不移动键盘焦点、不自动恢复/重发、不改变 Core；普通 query/refetch 与编辑不能反复抢滚动。独立 panel 不改变原布局行为。浏览器验证提交后的双语 quota 原因及原请求恢复操作实际可见。

触控 41.8px 读数恰为 44px 的 95% 入场帧；修正等待过渡后再采样，暂态不算产品缺陷。此轮只集中修上述已确认问题，再一次确认，不开展额外视觉润色。

320×568 的最终 Tab 走查还确认描述框下缘被滚动主体裁切（390px 正常）。在 CreationContents 的受限主体增加按焦点事件的 nearest 滚动与环余量：仅当目标是主体实际 DOM 后代且控件不完全可见时处理，排除通过 React portal 冒泡的搜索输入；不移动焦点、不过度监听滚动、独立 panel 不处理。此修正属于 R7 的明确验收项，重验两个 320px 主题，不扩展视觉润色。

## 验证边界

Component tests 锁住行为；Core 既有矩阵锁住原请求与 epoch。独立构建原始组件、共享 CSS 和正式双语资源，使用模拟 API + 内存 StorageAdapter、全新 Playwright Chromium context；静态 HTTP listener 由本任务绑定 loopback 空闲端口，所有外部请求拒绝，记录自身 pid/port/build hash。

保留前后截图、DOM/AX、真实定位触摸、请求日志和明暗对比数值。此证据不等于真实提交、Electron Main、读屏朗读、物理触屏或 iOS Safari。现有服务只读核对，不运行 make up/gc/down/destroy 或现有服务重启。
