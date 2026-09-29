# 第一期技术与交互设计

## 决策摘要

采用“选择器内的常用／最近快捷视图 + 完整目录 + 全目录搜索”。复用已有工作区查询、描述预览、拼音匹配、PropertyPicker 和偏好存储；只扩展快速创建的状态与局部 UI。不新增服务端、数据库或第三方依赖。

本方案保留“创建者”的现有业务含义与默认预选链，第一期不重命名为默认助手，也不改变任务负责人或小队路由。

## 交互结构

```text
创建者  小阿孚 ▾
┌────────────────────────────────────────┐
│ 搜索名称或职责…                        │
│ [全部]  [智能体]  [AI小队]              │
│                                        │
│ 我的常用          查看全部常用（4）     │
│ ○ 小阿孚                    智能体  ◆  │
│   来自该智能体已保存的职责描述          │
│ ○ 缺陷修复小队              AI小队  ◆  │
│   来自该小队已保存的职责描述            │
│ … 最多 3 项                            │
│                                        │
│ 最近使用                               │
│ ○ 诊断工程师                智能体  ◇  │
│   来自该智能体已保存的职责描述          │
│ … 最多 5 项，排除全部常用               │
│                                        │
│ 浏览全部（N）                          │
└────────────────────────────────────────┘
```

示意名称不构成能力声明；具体内容取保存数据。◆／◇表示独立“固定为常用／取消固定”按钮，实施时使用现有图标及语义色。标题、搜索、类型栏固定；正文滚动；完整目录及全部常用提供返回快捷视图入口（无有效快捷项时无需展示该入口）。

桌面弹层初始设计宽度约 384 CSS px，最大宽度不超过视口减 24px；高度遵守现有弹层可用高度规则。采用项目字号和颜色 token，不新增全局样式。Web 窄屏与桌面共用该组件，移动 App 不在范围内。

## 状态模型与转换

| 状态 | 内容 | 转换 |
| --- | --- | --- |
| home | 前 3 个常用 + 前 5 个未固定最近 | 浏览全部→all；查看全部常用→favorites；输入→search |
| all | 全部可用对象，稳定排序 | 返回→home；输入→search |
| favorites | 所有可用常用，固定顺序 | 返回→home；输入→search |
| search | 全量可用目录与类型筛选的匹配结果 | 清空→原浏览视图 |

search 是非空 query 派生状态，不另存一份。view、query、typeFilter、分页显示数量属于组件临时状态，不持久化。typeFilter 在本次打开内保留，关闭时全部复位。home 在没有有效偏好时派生显示 all，不用 effect 改写偏好。

类型过滤后的快捷组为空，仍显示类型栏、“此类型暂无常用或最近使用”及浏览全部入口；不会自动替用户清除类型筛选。favorites 中取消最后一项时保留空态与返回／浏览全部动作，不突然关闭。

## 模块边界与文件规划

| 文件 | 责任 |
| --- | --- |
| `packages/core/issues/stores/quick-create-store.ts` | 扩展既有偏好，增加 actor ref 类型、固定／最近操作、持久化校验及清理 |
| `packages/core/issues/stores/quick-create-store.test.ts` | 偏好转换、去重、上限、旧数据、工作区／账号隔离的唯一主测试层 |
| `packages/views/modals/quick-create-actor-picker-model.ts`（新增） | 纯数据计算：身份键、可用目录投影、搜索分级、稳定排序、快捷分组及分页 |
| `packages/views/modals/quick-create-actor-picker-model.test.ts`（新增） | 搜索与分组完整边界矩阵；node 环境 |
| `packages/views/modals/quick-create-actor-picker.tsx`（新增） | 本次入口专用组件，状态转换、两行候选、类型栏、固定与焦点管理 |
| `packages/views/modals/quick-create-actor-picker.test.tsx`（新增） | 使用真实 PropertyPicker 的键盘、焦点、筛选、固定与状态测试 |
| `packages/views/modals/quick-create-issue.tsx` | 移出原 ActorPicker；传入目录／加载状态；在原成功提交位置记录历史，保留创建流程 |
| `packages/views/modals/quick-create-issue.test.tsx` | 提交与历史记录连接、异步作用域守卫、原业务回归 |
| `packages/views/issues/components/pickers/property-picker.tsx` | 可选且向后兼容的 navigationResetKey；不改变其他调用方默认行为 |
| `packages/views/issues/components/pickers/property-picker.test.tsx`（新增） | 新增可选参数的聚焦回归，保留无参数旧行为 |
| `packages/views/locales/{en,zh-Hans}/modals.json` | 新增本入口文案及无障碍标签，两种语言同步 |
| `e2e/quick-create-actor-picker.spec.ts`（新增） | 大目录、跨工作区、键盘及弹层真实集成 |
| `.trellis/spec/views/frontend/quick-create-actor-picker.md`（实施阶段新增） | 记录最终合同，更新同目录 index.md |

搜索在 views 内复用已有 pinyin-pro 依赖，不把该依赖加入 core。Zustand 仍只在 core。数据查询沿用 React Query，不能复制 Agent／Squad 服务端对象到偏好状态。新文件用于收束本入口逻辑，不建立通用“万能对象选择器”。

## 数据来源与可用性

沿用 `agentListOptions(wsId)`、`squadListOptions(wsId)`、成员及运行时查询；继承 `!archived_at`、`isAgentRuntimeBound`、`canAssignAgent`，小队继承当前可选队长的访问条件。离线不新增排除规则，CLI 版本门槛仍由原表单执行。

先建立可选目录，再解析常用／最近引用，最后做类型过滤及搜索。所有显示名称、描述、头像和计数均来自当前已授权目录。新增模型对 name／description 做字符串校验，缺失或非法 description 视为空，不将当前尚未 schema 化的 agent 响应假设为绝对可靠；不借此扩大到整个 Agent API 重构。未知、已删除或已归档引用只在渲染时过滤；加载失败、短暂缺失不清理持久偏好，恢复后可重新出现。20 条最近保留上限负责历史淘汰。

Actor 身份键固定为 `${type}:${id}`，小队和队长是不同对象，不能因执行时最终都路由到同一个智能体而合并。小队使用自己的描述；不加载成员详情，不产生每个对象一个请求。

冷启动需要的查询尚未返回时显示加载；无缓存失败时显示失败和重试。已有缓存且后台刷新失败时保留缓存可用内容并标明刷新失败。智能体／成员查询缺失时不能提前宣称目录为空；小队查询失败可保留已确认的智能体，但需要显式小队加载失败提示，避免把部分结果当成完整目录。

## 偏好合同

```ts
type QuickCreateActorRef = { type: 'agent' | 'squad'; id: string };
// 扩展既有 store 数据；名称、描述、头像、query、typeFilter 不入库。
favoriteActors: QuickCreateActorRef[]; // 固定先后顺序，快捷只显示前 3 个
recentActors: QuickCreateActorRef[];   // 最新在前，去重，最多 20 个
```

继续使用当前 `multica_quick_create` 工作区存储。沿用并保留 lastActorType、lastActorId、keepOpen 的含义。新增 `toggleFavoriteActor(ref)` 和 `recordSuccessfulActor(ref)`；后者在一次状态更新中写入 lastActor 与最近数组。仅在普通 `api.quickCreateIssue` 与评论上下文 `api.createCommentSubIssue` 分支共用的现有成功返回位置调用，不等待后台最终完成，不因单纯 onPick 调用。

- 使用明确字段的 partialize，不持久化 action、会话代次和临时状态。
- 从固定默认值和白名单字段构建 rehydrate 结果，不能用上一工作区的 data 字段作为缺失字段默认值。验证 type 和非空 id，去重并限制最近长度。
- 旧数据缺少 recentActors 时，可用有效 lastActor 作为一条最近；显式存在的空 recentActors 不回填。缺失 favoriteActors 默认空，保留有效 keepOpen。
- 缺失记录、非法字段、无效 JSON 都不得崩溃或沿用其他工作区数据；坏 JSON 的错误路径也要恢复为默认偏好并完成可恢复的 hydration 状态，不能只校验已解析 JSON。不要在读取目标工作区之前用普通持久化 setState 清空数据，否则会覆盖目标工作区原有记录；优先在本 store 的存储读取边界规范化错误，然后交给 merge。
- 扩展已有 registerDraftCleanup 的 resetInMemory，将两数组一起清空；使用该 store 内不持久化的会话代次区分登出／重登录（包括相同用户重新登录）。
- 发请求前捕获 userId、workspace UUID、会话代次。await 返回后记录偏好前，与 useAuthStore.getState()、getCurrentWsId() 和当前代次及已认证状态比较。任一变化则跳过本次偏好写入，不影响请求本身已经被接受的事实。
- hydration 与工作区切换期间禁用偏好写入，完成后才显示本工作区常用。就绪条件必须包含本次 hydration 完成时记录的作用域（工作区 UUID、用户、会话代次）与当前作用域一致，不能只读 persist.hasHydrated()：镜像切换同步发生，但 rehydrate 在下一微任务才开始，旧 true 会短暂残留。每轮读取捕获作用域及读取代次，过期完成不能发布为当前就绪；组件渲染和固定／历史写入入口均校验该条件。测试切换后、微任务执行前的立即读写，以及空目标、错误路径、过期读取完成。不要修改通用 ApiClient 或通用 storage 的行为来解决本入口问题。

偏好是本地、按工作区保存，并通过现有账号退出清理隔离；不是服务器个人资料。Web／桌面独立保存，退出登录可能清除本地常用，这一点与现有清理机制一致。

## 搜索、摘要与渲染

复用 `descriptionPreview` 生成展示摘要（已有去常见 Markdown、媒体和 300 字符安全截断）。列表再以单行省略限制高度；不改变原始 description。空预览采用本入口“暂无职责描述”。

搜索使用完整保存描述，不能对 descriptionPreview 的截断结果搜索；以大小写不敏感子串匹配为第一期合同，保留现有名称拼音匹配。匹配优先级：名称精确 > 名称包含 > 名称拼音 > 仅描述包含。每个对象只归入最高匹配级。同级按当前 locale 的名称比较，之后按 type、id 比较，确保稳定。空查询的完整目录采用同一名称比较器。

描述文本作为字符串，不渲染富文本／可点击链接，不读取系统指令；描述含特殊字符不会生成 HTML。第一期不承诺跨 Markdown 标记的模糊语义匹配或自然语言同义词识别。

已加载目录建立一次轻量投影，query 改变时只过滤／排序，不重新请求服务端。完整目录、全部常用和搜索结果首批渲染 50 项，使用明确的“显示更多（剩余 N）”追加 50 项；搜索始终遍历全量目录。query、typeFilter 或 view 变化重置显示上限，固定操作不重置搜索分页。无需在第一期引入虚拟列表或新依赖。

## 键盘与焦点

保留 PropertyPicker 输入框的上下箭头、Enter 选中、Esc 关闭、输入法保护。类型、固定、返回及显示更多为真实 button，具备可见焦点和清晰 aria-label，固定按钮使用 aria-pressed。

每行使用非交互容器，内含现有 PickerItem 主按钮和它的兄弟固定按钮。固定按钮不能嵌套进 PickerItem，也不标记 data-picker-item；选择器方向键只遍历主候选。补充动作通过 Tab／Shift+Tab 访问，Space／Enter 执行该按钮自身动作。

为 PropertyPicker 增加可选 navigationResetKey：仅传入者在 view、typeFilter、固定重排或异步可用目录发生外部变化时，清除旧数字索引高亮；同时使该轮 Enter 不走“唯一结果自动选择”回退，直到用户重新输入或按方向键。下一次 ArrowDown 进入第一项。明确优先级：用户输入查询词引起的结果变化继续执行原有“首个真实匹配高亮 → Enter 选择”，不能被外部重排规则覆盖。不得以 children 对象引用变化触发重置。输入法保护保持优先。无此 prop 的调用方行为不变；分页追加且原候选顺序未变时保留高亮。测试外部变化与输入发生在同一轮渲染的情况，并在 Enter 时防御尚未同步的旧候选签名。

固定后如果对象仍在当前视图，保留同对象固定按钮焦点；对象移除时移到原位置的下一条主按钮，若没有下一条则移到上一条，最后一条也消失则回搜索框。由本组件持有行容器引用完成；仅在确需输入框 ref 时扩展 PropertyPicker 可选 searchInputRef，并补充真实组件测试。主按钮内保留可读名称、类型和职责摘要，使用原生按钮文本形成可访问名称，已选项补充屏幕阅读器可读的“已选择”文本；不假设现有 PickerItem 透传 aria-* 属性，也不额外引入 option/listbox 语义。选中勾选与文字强调独立于 hover 背景。

## 备选方案与取舍

| 方案 | 判断 |
| --- | --- |
| 扩展侧边栏收藏 | 现有类型只支持 issue/project/view，需后端合同和导航产品变化，超出本期 |
| 复用聊天置顶 | 仅支持智能体、上限 5，不支持小队，且会影响聊天列表，拒绝耦合 |
| 新建服务端常用系统 | 有跨设备收益，但本期不需要引入 API、迁移和权限成本，后续独立设计 |
| 仅加搜索或放大下拉框 | 无法解决多位常用切换、名字相似和职责不清，收益不足 |
| 按模板推断能力／AI推荐 | 模板只是来源，用户可改职责；推荐属于第二期 |
| 全量 DOM 或立即虚拟化 | 前者大目录开销高，后者改变现有键盘模型；先采用全量匹配、分批呈现 |

## 兼容、发布与回退

无需 API 或数据库迁移。旧 store 字段向后兼容；回退 UI 代码后旧客户端仍可读取原有字段。回退版本可能在下一次写入时丢弃新增本地偏好，可接受且不影响任务数据；不承诺跨版本保留新收藏。

实施分成偏好／数据模型、交互、集成验证三个连续检查点，以一个 Trellis 任务跟踪，避免把强耦合改动拆成各自“完成”的任务。该工作区已有并行未提交修改，尤其 quick-create-issue.tsx、其测试和 modals locale；实施前按最新内容重读，提交只含本任务改动。

## 主要风险与验证归属

- 工作区空存储及坏 JSON 沿用旧状态：core store 测试；新默认合并和错误路径均需覆盖。
- 迟到提交污染工作区／账号：表单集成测试；使用明确 origin 身份和会话代次。
- 固定重排后 Enter 跳选、焦点丢失：真实 PropertyPicker 测试＋浏览器键盘走查。
- 摘要截断造成搜索漏项：模型测试固定词放在第 300 字符之后。
- 其他选择器回归：navigationResetKey 必须可选，运行 property-picker 和现有创建相关套件。
- 性能及布局：550 对象、桌面宽屏／窄屏与中文长名称验证，记录实测而非推断。
