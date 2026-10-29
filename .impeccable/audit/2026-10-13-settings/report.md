# 设置页面技术审计

审计目录创建于 2026-10-13；2026-10-14 续查完成。对象是 Web/Desktop 共享设置及桌面注入的设置页，属于 Operate 工作界面。本轮完成审计和复现，没有修复应用源码。

## 结论

**实现完整性未通过。** 已确认读取失败被当作空数据、保存失败误报成功、重复提交、配置操作被忽略等问题。界面采用统一组件、语义颜色和角色字号；机械扫描未发现模式问题，但运行验证暴露了这些扫描器无法判断的行为缺陷。

当前可见设置范围合并为 **17 组问题：P0 0、P1 8、P2 9、P3 0**。其中 15 组至少有一条运行复现，F14、F15 为源码确认；同组扩展位置不代表逐个完成了运行验证。另有 1 条插件开关关闭时的静态附录，不计入当前界面评分。

| 维度 | 分数 | 主要依据 |
| --- | ---: | --- |
| 可访问性 | 2/4 | 8 个已验证的无名控件、11 项已绑定快捷键的键帽值被排除出 AX、更新结果缺少播报语义 |
| 性能 | 2/4 | 状态目录在读取失败后反复挂载订阅者并重新请求；未进行生产构建性能测试 |
| 主题 | 3/4 | 语义 token 基本统一；两类浅色状态文字误用了填充色 |
| 响应式 | 2/4 | 900px 窗口裁掉行菜单，长属性弹窗超出视口 |
| 实现完整性 | 1/4 | 多处重复的读取恢复、保存反馈及 pending 保护缺口 |
| **合计** | **10/20** | **需要集中整改** |

优先修复 F01 请求循环、F02 读取/权限误报、F03–F04 操作不可见以及 F16 响应解析失败。其余问题与建议验收方式见下文。

## 范围与证据口径

- 20 个可见页：个人资料、偏好设置、通知、快捷键、API Token、常规、成员、标签、分拣台、迭代、任务状态、属性、快捷操作、代码仓库、集成、MCP、守护进程、服务器连接、更新、行为。
- 保留上一轮 1440px 明暗主题的 40 张截图；本轮补齐 900×1000 原生窗口、200% 缩放下的全部 20 页，实际内容视口为 **450×500 CSS px**。另外检查 900px 宽窗口、长弹窗、可访问树、错误及保存状态。
- 运行环境：macOS 上的 Electron 39.8.7，当前工作区开发渲染器、真实 preload、路由与共享组件，独立临时 profile。通过 IPC 夹具显示 Windows/Linux 的“行为”页，**不等于在 Windows/Linux 上验收**。
- 网络写入由既有 harness 拦截；头像、邀请、令牌和 GitHub 保存探针使用内存 API 替身。没有真实发邀请、创建/吊销令牌、修改工作区、安装插件或启动智能体。
- Electron 的 CDP `route.fulfill` 在本机出现 status 0，已在 200/null 探针中确认。早期错误探针没有单独断言客户端收到的 HTTP 状态，只作为**读取失败分支**的证据，不能当作真实 HTTP 500 验收。F01 已另用浏览器原生 `Response(500)` 复核；F16 使用 `Response(200, null)`，经过真实 ApiClient 解析路径。两者均为受控响应夹具，未声称后端实际返回这些载荷。
- [verification.json](verification.json) 记录接受的结果文件、源码指纹、范围与清理状态。旧超时/定位错误保留为诊断记录，不计入通过或产品缺陷。

## P1：发布前应修复

### F01：状态目录读取失败形成重新请求循环

- 位置：`packages/views/settings/components/issue-statuses-tab.tsx:135`、`:189`、`:255`；`packages/views/issues/utils/status-label.ts:24`；`packages/core/issue-statuses/hooks.ts:13`。
- 维度：实现完整性、性能。
- 关键代码：

  ```tsx
  const { data: statuses = [], isLoading } = useQuery(issueStatusListOptions(wsId));
  {isLoading ? <Loading /> : groups.map(group => <CategorySection ... />)}
  const labelOf = useStatusLabel(wsId); // CategorySection subscribes again
  ```

- 证据与影响：失败后分组挂载，新的 Query observer 触发 `retryOnMount`；再次进入 loading 后分组卸载，如此循环。500 Response 的复核在约 5.09 秒记录 11 次请求。页面停留在加载/分组切换状态，无法稳定展示失败及重试入口。
- 标准：仓库集合恢复契约；重试不应由错误/加载子树挂载反复触发。
- 建议：在父层处理无缓存失败；保留稳定错误/重试节点，避免分组重新订阅触发循环。复用已取得的目录数据/分类文案，不通过全局禁止重试掩盖问题。
- 核实程度：**源码与运行确认**。[真实 500 Response 复核](probe-status-loop-response500.json)、[最初的 observer 轨迹](probe-status-loop-results.json)。

### F02：读取失败被显示为空目录或权限/配置不足

- 位置：`packages/views/settings/components/members-tab.tsx:379`、`:418`、`:861`；`workspace-tab.tsx:79`、`:146`；`labels-tab.tsx:79`；`properties-tab.tsx:125`；`quick-actions-tab.tsx:191`。
- 维度：实现完整性。
- 关键代码：

  ```tsx
  const { data: members = [] } = useQuery(memberListOptions(wsId));
  const { data: labels = [], isLoading } = useQuery(labelListOptions(wsId, resourceType));
  ```

- 证据与影响：无缓存读取失败后，分别出现“未找到成员”“暂无任务标签”“还没有属性”“还没有快捷操作”，均无页内 Retry。工作区页显示“只有管理员和所有者可以修改”，同时“离开工作空间”仍可用。不能把这些表现解读为权限确实被撤销或服务端允许非法退出。
- 同类源码位置：`github-tab.tsx:50`/`:198`、`vcs-tab.tsx:51`/`:271`，以及 Slack/Lark/钉钉/企微的首次失败配置分支；这些详情页没有逐个故障注入。
- 标准：`.trellis/spec/views/frontend/desktop-core-accessibility.md` 的 pending/error/empty 与授权缓存契约。
- 建议：以查询成功状态判定空目录、权限和服务配置；无数据失败显示命名 alert/Retry，有授权缓存时保留内容。参考通知设置的现成实现。
- 核实程度：**成员、常规、标签、属性、快捷操作已运行确认；其他列明位置为源码确认**。[读取失败证据](probe-query-recovery-results.json)、[成员错误截图](screenshots/resume-error-members.png)。

### F03：900px 窗口隐藏目录右侧操作

- 位置：`packages/views/settings/components/labels-tab.tsx:141`、`:168`；`properties-tab.tsx:185`、`:218`；`quick-actions-tab.tsx:268`、`:304`。
- 维度：响应式。
- 关键代码：

  ```tsx
  <div className="overflow-hidden rounded-lg border border-surface-border bg-card">
  // Viewport-based md grids keep fixed minimum column widths.
  ```

- 证据与影响：900px 窗口中，标签卡片宽 594px，内容宽 688px；行菜单位于 x=921–949，超出视口。快捷操作菜单也被裁掉。没有横向滚动条，鼠标用户看不到操作入口。**Tab 聚焦可使隐藏溢出的卡片滚动并露出菜单**，不能说键盘也完全不可达。属性同类网格宽 752px，当前空目录只验证了布局约束，未验证有数据行菜单。
- 标准：仓库溢出/窄窗口规则；缩放后操作应保持可用。
- 建议：按内容容器宽度切换堆叠布局，或提供明确且可操作的横向滚动；不能仅用 viewport 的 `md` 判断。
- 核实程度：**标签、快捷操作运行确认；属性网格约束确认**。[尺寸结果](resume-baseline.json)、[聚焦前后](probe-final-evidence-results.json)、[900px 截图](screenshots/resume-labels-light-900.png)。

### F04：长属性弹窗的标题及保存按钮超出窗口

- 位置：`packages/views/settings/components/properties-tab.tsx:454`、`:466`、`:575`；`packages/ui/components/ui/dialog.tsx:60`。
- 维度：响应式、可访问性。
- 关键代码：

  ```tsx
  <DialogContent className="sm:max-w-lg">
  <div className="grid grid-cols-[4.25rem_minmax(0,1fr)_10rem] gap-3">
  {draft.options.map(...)}
  ```

- 证据与影响：12 个本地选项时，900×700 窗口中的弹窗高 891px，顶部 -95.5px、底部 795.5px；450×500 CSS px 时高 931px，顶部 -215.5px。没有最大高度或正文滚动，聚焦保存按钮也不能使其完整进入视口。选项通过本地编辑构造，未保存数据。
- 标准：WCAG 1.4.4/1.4.10；仓库对话框溢出规则。
- 建议：限制弹窗最大高度，正文独立滚动，底部操作始终可达；窄屏首行改为纵向布局。复用仓库选择/MCP 配置弹窗模式。
- 核实程度：**源码与运行确认**。[几何与聚焦结果](probe-configuration-results.json)、[200% 截图](screenshots/configuration-properties-900x1000-200pct.png)。未验证 320px 设备，不把静态估计当作截图结论。

### F05：八个控件没有可访问名称

- 位置：`packages/views/settings/components/members-tab.tsx:824`、`:901`、`:922`；`properties-tab.tsx:543`；`issue-statuses-tab.tsx:649`；`apps/desktop/src/renderer/src/components/daemon-settings-tab.tsx:158`、`:169`；`desktop-behavior-settings-tab.tsx:96`。
- 维度：可访问性。
- 关键代码：

  ```tsx
  <SelectTrigger size="sm"><SelectValue ... /></SelectTrigger>
  <Switch checked={prefs.autoStart} onCheckedChange={...} />
  ```

- 证据与影响：AX 中成员页 3 个 combobox、守护进程 2 个 switch、行为 1 个 combobox、属性类型与状态类别各 1 个 combobox 的 name 均为空。所选“成员”“单选”等属于 value，不能代替字段用途名称。
- 标准：WCAG 1.3.1、4.1.2。
- 建议：关联现有可见标签，使用稳定 id/`htmlFor`/`aria-labelledby`；没有可见标签时补具体的本地化 `aria-label`。
- 核实程度：**八个控件均运行确认**。[成员及桌面 AX](probe-final-evidence-results.json)、[属性/状态 AX](probe-configuration-results.json)。VCS Webhook 字段（`vcs-tab.tsx:340`）及插件配置字段（`plugins-tab.tsx:231`）存在同类源码缺口，当前未构造这些配置。

### F06：快捷键页不向辅助技术暴露当前绑定

- 位置：`packages/views/settings/components/keyboard-shortcuts-tab.tsx:305`、`:314`；`packages/views/common/shortcut-keycaps.tsx:104`。
- 维度：可访问性。
- 关键代码：

  ```tsx
  aria-label={t(($) => $.shortcuts.record_aria, { action: label })}
  <ShortcutKeycaps shortcut={shortcut} decorative />
  ```

- 证据与影响：23 个录制按钮的名称均不含当前值，其中 **11 项已有绑定的键帽被排除出 AX**。例如界面显示 Ctrl+K，按钮名称只含“修改‘打开搜索’的快捷键”，键帽为 `aria-hidden=true` 且无关联描述。另 12 项“未设置”的文本仍在 AX 中，不能说这些未分配状态也完全不可读。已有绑定的用户无法通过该控件查询当前组合。
- 标准：WCAG 1.3.1、4.1.2。
- 建议：复用已有格式化函数，把当前组合或“未分配”加入名称/描述，并在修改后同步。
- 核实程度：**11 项默认绑定的缺口运行确认**；没有实际改写快捷键。[23 项 DOM/AX 对照](probe-configuration-results.json)。

### F07：浅色状态文字对比度不达标

- 位置：`packages/views/settings/components/quick-actions-tab.tsx:151`、`:326`；`integrations-tab.tsx:164`；`packages/ui/styles/tokens.css:228`、`:230`。
- 维度：主题、可访问性。
- 关键代码：

  ```tsx
  mismatched && "text-warning"
  stale && "text-warning"
  "bg-success/10 text-success"
  ```

- 证据与影响：原生渲染计算的浅色 warning 文字为 **2.25:1**，11px“已连接”为 **4.00:1**，均低于小字 4.5:1。对应深色分别为 6.56:1、5.05:1。使用的是内存中的过期快捷操作和已连接 GitHub 夹具，没有更改真实连接。
- 标准：WCAG 1.4.3；仓库状态填充色与文字色分离契约。
- 建议：使用已有 `warning-foreground` / `success-foreground`，验证实际背景及 hover 状态，不重新发明颜色。
- 核实程度：**明暗主题运行确认**。[警告文字测量](probe-configuration-results.json)、[连接徽标测量](probe-integrations-results.json)。

### F17：更新检查结果缺少状态播报

- 位置：`apps/desktop/src/renderer/src/components/updates-settings-tab.tsx:106`、`:112`、`:120`。
- 维度：可访问性。
- 关键代码：

  ```tsx
  {state.status === "up-to-date" && <p ...>{...up_to_date}</p>}
  {state.status === "available" && <p ...>{...downloading}</p>}
  {state.status === "error" && <p ...>{state.message}</p>}
  ```

- 证据与影响：点击“立即检查”后“已是最新版本。”显示在普通段落中，其祖先没有 `role=status`、`role=alert` 或 `aria-live`。用户停留在检查按钮时无法通过状态播报获知结果。
- 标准：WCAG 4.1.3。
- 建议：给检查结果设置稳定的 live region；成功/进行中用 status，失败按现有错误模式提示，避免移动用户焦点。
- 核实程度：**源码与 AX/DOM 运行确认**；未进行真人读屏听测。[更新状态证据](probe-final-evidence-results.json)。

## P2：下一轮应修复

### F08：缓存刷新失败移除正在编辑的控件

- 位置：`packages/views/settings/components/mcp-tab.tsx:286`；`packages/views/triage/triage-settings-tab.tsx:56`。
- 维度：实现完整性、可访问性。
- 关键代码：

  ```tsx
  serversQuery.isLoading ? <Loading /> :
    serversQuery.isError ? <Error /> : <ServerRows />
  ```

- 证据与影响：MCP 列表已有 5 条缓存，改名过程中刷新失败仍卸载输入，原 DOM 的 `isConnected=false`、焦点丢失。草稿由父层保存，**没有据此认定 MCP 草稿数据被清空**。分拣设置也优先用错误块替换现有编辑区。
- 标准：仓库授权缓存及编辑节点保留契约。
- 建议：无数据失败才替换主体；有缓存时保留列表/表单，在外部显示刷新失败和 Retry。
- 核实程度：**MCP 运行确认，分拣源码确认**。[缓存与节点证据](probe-query-recovery-results.json)。插件、Telegram、Composio、计费有同类分支，未逐页运行。

### F09：头像保存失败仍显示成功并关闭裁剪

- 位置：`packages/views/settings/components/account-tab.tsx:126`；`workspace-tab.tsx:350`；`packages/views/common/avatar-upload-control.tsx:186`。
- 维度：实现完整性。
- 关键代码：

  ```tsx
  if (!(await persistedByCaller(() => onUploaded(result.link)))) return;
  setDialogOpen(false); setPickedFile(null);
  toast.success(t(($) => $.avatar_upload.updated));
  ```

- 证据与影响：个人/工作区调用方 catch 后正常返回，被共享上传控件当作成功。个人头像探针同时产生失败 toast 和“头像已更新”，裁剪弹窗关闭；重试需要重新选图和裁剪。
- 标准：保存反馈必须与持久化结果一致。
- 建议：失败时保留已有错误提示并向调用方抛出，让现有 `persistedByCaller` 保留编辑流程。
- 核实程度：**个人头像运行确认；工作区 Logo 源码确认**。[复现结果](probe-personal-results.json)、[成功误报截图](screenshots/resume-personal-avatar-failure.png)。上传/保存均为内存替身。

### F10：清空名称会无提示地停止整组自动保存

- 位置：`packages/views/settings/components/account-tab.tsx:90`；`workspace-tab.tsx:216`；`use-auto-save.ts:114`。
- 维度：实现完整性。
- 关键代码：

  ```tsx
  enabled: !!user && !!profileName.trim() && !descriptionTooLong
  if (!enabled) setStatus("idle");
  ```

- 证据与影响：清空姓名后编辑个人介绍，等待超过 debounce，写请求为 0；没有必填错误、`aria-invalid` 或未保存状态。离开并返回页面后介绍草稿丢失。工作区名称采用同类整组 gate。
- 标准：字段校验应解释无法保存的原因，错误与输入关联。
- 建议：显示名称必填错误及未保存状态，保留草稿；避免让其他字段修改静默进入 idle。
- 核实程度：**个人资料运行确认；工作区源码确认**。[草稿与返回结果](probe-personal-results.json)。

### F11：提交中的其他入口未做重复提交保护

- 位置：`packages/views/settings/components/members-tab.tsx:434`、`:812`、`:1085`；`workspace-tab.tsx:534`；`tokens-tab.tsx:95`、`:253`。
- 维度：实现完整性。
- 关键代码：

  ```tsx
  if (e.key === "Enter" && inviteEmail.trim()) handleInviteMember();
  ```

- 证据与影响：邀请按钮已 disabled，但邮箱输入仍接收 Enter；第一笔请求 pending 时再按一次，记录到 **2 次 createMember**。成员/前缀确认和 token 吊销还存在相同的 pending 保护缺口；吊销 catch 后外层仍关闭确认，不能就地重试。
- 标准：仓库确认操作应等待服务器，所有提交入口共享 pending guard。
- 建议：在处理函数和所有提交控件统一防重；确认失败保留目标与弹窗，成功后关闭。
- 核实程度：**邀请重复 Enter 运行确认；其他列明入口源码确认**。[两次调用证据](probe-personal-results.json)。没有真正发送邀请或吊销 token。

### F12：令牌保存确认可以被其他关闭路径绕过

- 位置：`packages/views/settings/components/tokens-tab.tsx:124`、`:266`、`:330`。
- 维度：实现完整性。
- 关键代码：

  ```tsx
  onOpenChange={(v) => { if (!v) closeCreatedDialog(); }}
  <Button disabled={!storedConfirmed} onClick={closeCreatedDialog}>
  ```

- 证据与影响：“已安全保存”未勾选、完成按钮禁用时，Esc 和右上关闭按钮仍直接关闭，丢掉只显示一次的令牌。
- 标准：同一确认约束应覆盖所有退出路径。
- 建议：统一关闭处理；未确认时保持弹窗，或显式确认放弃未保存令牌。
- 核实程度：**两条关闭路径运行确认**；遮罩未单独验证。[虚拟令牌结果](probe-integrations-results.json)。

### F13：GitHub 保存过程中静默忽略另一项设置

- 位置：`packages/views/settings/components/github-tab.tsx:74`、`:158`、`:269`、`:287`、`:301`。
- 维度：实现完整性。
- 关键代码：

  ```tsx
  if (!workspace || savingKey) return;
  disabled={!canManage || savingKey === "github_enabled"}
  ```

- 证据与影响：第一项更新未返回时，第二项仍 enabled；点击后没有第二笔更新，最终该项仍是旧值，也没有反馈。
- 标准：控件可操作状态应与实际可接受的操作一致。
- 建议：统一禁用本组保存控件并显示进度，或使用现有队列保证后一次选择生效。
- 核实程度：**运行确认**。[仅一次保存调用](probe-integrations-github-results.json)。所有设置更新仅发生在内存夹具。

### F14：删除仓库行先清除界面，失败后无直接恢复入口

- 位置：`packages/views/settings/components/repositories-tab.tsx:376`、`:709`；`use-auto-save.ts:96`。
- 维度：实现完整性。
- 关键代码：

  ```tsx
  setRepositories(next);
  autoSave.saveNow(next);
  setPendingRemovalIndex(null);
  ```

- 影响：`saveNow` 不可等待，行及确认先消失；保存失败不回滚，也无 Retry。删除最后一行后，没有剩余输入可通过 blur 再触发保存。
- 标准：CLAUDE.md 要求删除/确认流程等服务器成功再清理。
- 建议：让删除使用可等待的保存流程；pending 防重，失败保留行及确认，允许重试。
- 核实程度：**源码确认，未执行删除**。修复验收应对“唯一仓库行、延迟失败、随后成功重试”写有意义的回归。

### F15：个人时区的乱序保存可能覆盖最后一次选择

- 位置：`packages/views/settings/components/preferences-tab.tsx:320`、`:351`。
- 维度：实现完整性。
- 关键代码：

  ```tsx
  const updated = await api.updateMe({ timezone: payload });
  setUser(updated);
  ```

- 影响：请求期间选择器没有 pending 锁或串行处理；连续选 Tokyo、Paris 时，旧响应若最后完成，可将界面回退至 Tokyo。
- 标准：同一偏好应保证最后一次用户选择具有明确结果。
- 建议：复用串行保存机制，或保存中锁定该字段并反馈状态。
- 核实程度：**源码确认，未模拟真实后端并发**。验收须控制两次响应的先后顺序；现有测试只覆盖单次成功/失败。

### F16：令牌响应未校验，畸形成功载荷使设置页报错

- 位置：`packages/core/api/client.ts:4240`、`:4244`、`:1375`；`packages/views/settings/components/tokens-tab.tsx:66`、`:190`、`:200`。API 文件有其他并行工作，行号以本轮源码快照为准。
- 维度：实现完整性、API 兼容性。
- 关键代码：

  ```tsx
  return this.fetch("/api/tokens");
  setTokens(list);
  tokens.length === 0 ? ... : tokens.map(...)
  ```

- 证据与影响：给真实 ApiClient 的 fetch 返回 `Response(200, null)`，随后出现 `Cannot read properties of null (reading 'length')`，设置内容被路由错误页替代；桌面外壳仍保留。没有发现当前后端实际返回 null 的证据。
- 标准：CLAUDE.md 要求 UI 使用的网络 JSON 经 zod + `parseWithFallback`。
- 建议：在 core API 边界校验列表和创建结果；畸形创建结果不能显示成功/丢失可恢复信息。补 malformed-response 回归。
- 核实程度：**真实 ApiClient 解析路径与渲染运行确认**。[200/null 复现与恢复](probe-token-schema-results.json)。

## 功能开关之外的静态附录

### E01 [P1]：插件卸载及发布包删除没有确认

- 位置：`packages/views/settings/components/plugins-tab.tsx:505`、`:608`；直接依赖 `packages/core/plugins/mutations.ts`。
- 证据：垃圾桶的 `onClick` 直接调用 `onDelete(pluginPackage.id)` 或 `uninstallMutation.mutateAsync(installation.id)`。
- 影响：误点立即发起删除/卸载，没有对象确认、影响说明或撤销入口。
- 建议：复用 AlertDialog，显示具体对象，pending 防重；失败保留确认，成功后清理。
- 核实程度：**源码确认，当前开关关闭、未运行**。不为审计开启插件、安装包或执行真实删除；不计入当前 17 组评分。

## 值得保留与无新增高置信度问题的范围

- `settings-layout.tsx` 的标题层级、控件宽度档位、语义 token 及保存 `role=status`；`settings-page.tsx` 的命名目录、移动 Sheet、选中/hover 区分、路由参数处理。页面切换已通过本轮 200% 取证。
- `notifications-tab.tsx`、`browser-notification-setting.tsx` 的加载/失败/空态处理及命名控件。上一轮通知恢复修复未重复报为新问题。
- `workspace-planning-timezone.tsx` 的草稿保留与不确定写入确认；`delete-workspace-dialog.tsx` 的精确名称确认、IME 保护和 pending 锁。
- `packages/views/iterations/iteration-settings-tab.tsx` 及 `iteration-recovery.tsx` 的权限、缓存恢复和操作结果区分；`runtime-config-settings-tab.tsx` 所复用编辑器的字段错误关联及操作按钮换行。

另外保留：状态排序已有键盘上下移和归档保护；成员行操作已包含成员姓名；集成目录返回时恢复卡片焦点；MCP 现有配置弹窗有滚动区、字段错误和命名操作；代码仓库选择器有分页及 HTTPS/SSH 去重。这些都不因本轮发现而整体推翻。

## 建议修复顺序与验收

| 顺序 | 范围 | 最小有意义的验收 |
| --- | --- | --- |
| 1 | F01、F02、F16，`$impeccable harden` | 真 QueryClient 的初次失败/热缓存失败/空响应；有限重试；成员权限未知时保持中性；token malformed JSON |
| 2 | F03、F04，`$impeccable adapt` | 900px 与 450 CSS px；鼠标、Tab；12+ 选项弹窗首尾控件及保存按钮 |
| 3 | F05、F06、F07、F17，`$impeccable harden` / `typeset` | 实际 Base UI 的 AX 名称、快捷键当前值、live region；明暗文字对比度 >=4.5 |
| 4 | F08–F15，`$impeccable harden` | 保留编辑节点；保存失败无成功 toast；空名称有错误；所有提交/退出路径；乱序响应 |
| 5 | `$impeccable polish` | 在现有设计体系内确认本轮改动；随后用 `$impeccable audit` 复核评分 |

## 证据限制和工具误报

- 原 `baseline-light-200pct.json` 的 20 项及合并文件中的同批 20 项全部超时，不能计为通过。本轮对应截图与 AX 已补齐，MCP 的嵌套 tabpanel 单独在最终证据中确认。
- Base UI 的隐藏原生 input，以及正确保留、`aria-hidden=true` 的聊天 DOM，会被单纯几何 `:visible`/裁切扫描命中；已排除，没有作为产品缺陷。
- 老 harness 的 updater 偏好字段名与真实 preload 不一致；续查仅纠正了**审计夹具**。同样未把模块 URL、隐藏 input 定位、原生窗口 resize 时序等探针错误算作产品问题。
- 读取失败证据不代表生产 outage；内存写入夹具不代表后端落库验收。没有进行 Windows/Linux、VoiceOver/NVDA 听测、物理触摸、320px 设备、全语言矩阵、生产构建性能、计费或真实插件生命周期验证。
- 本轮无应用源码变更，未运行全仓 lint/typecheck/unit/Go 测试。运行了检测器、实际 Electron 取证、响应/保存夹具及审计脚本语法校验；既有测试只作为源码审阅材料。
- 使用现有目录，不新建 Trellis 任务，不提交代码，不修改并行开发中的业务变更。
