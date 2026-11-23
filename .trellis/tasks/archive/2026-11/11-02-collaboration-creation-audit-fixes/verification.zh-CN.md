# 新建协作任务弹窗修复验收

2026-11-03。保留原产品风格、草稿/文件、账号/API/source防护、IME、原请求恢复。按授权完成Trellis计划/独立评审，使用native trellis-implement / trellis-check。

| 项目 | 状态 | 实际结果 |
|---|---|---|
| R1/P1 | 已修复 | 创建/原键恢复锁来源；延迟成功完成一次，已确认拒绝后解锁。 |
| R2/P1 | 已修复 | 原生textbox关联listbox，稳定option/selected/disabled/active ID；键盘、IME/229、Escape、分页回归通过。 |
| R3/P1 | 已修复 | 实际绘制的局部焦点最低3.86:1浅色/4.20:1深色；包含原生popup搜索/标签/summary及入口。 |
| R4/P2 | 已修复 | 26个已知拒绝码有具体动作；双语额度原因在顶部提交后实际可见，文件/草稿保留。 |
| R5/P2 | 已修复 | 同名同UUID前缀账号显示完整ID，列表/精确/选中/可访问名称一致；窄屏全文范围无裁切。 |
| R6/P2 | 已修复 | 390和320两主题：关闭/接收人行/父搜索/阶段真实命中区≥44px，实测坐标合成触摸通过。 |
| R7/P2 | 已修复 | 主体滚动、header/source/actions保留；初始/到底提交取消关闭可见；24步Tab焦点完整可见。 |
| R8/P2 | 已修复 | 250ms防抖：连续6字立即显示且仅最终1次查询；等待窗口不选旧项，换来源清理。 |

## 修改文件

- packages/views/collaboration/creation-form.tsx
- packages/views/collaboration/account-picker.tsx
- packages/views/collaboration/field-inputs.tsx
- packages/views/collaboration/form-styles.ts
- packages/views/collaboration/labels.ts
- packages/views/issues/components/pickers/property-picker.tsx
- packages/views/locales/en/collaboration.json
- packages/views/locales/zh-Hans/collaboration.json
- packages/views/collaboration/creation-form.test.tsx
- packages/views/collaboration/account-picker.test.tsx
- packages/views/issues/components/pickers/property-picker.test.tsx

另有本任务计划/研究/证据，以及views规范collaboration-creation.md和index入口。全局Dialog/Input/Textarea/Button/tokens源文件未改。并行来源选择器改动已保留，其会话提交4ba3a868a成为本任务当前基线；未暂存无关工作。

## 检查证据

- 独立最终目标：7文件119测试通过；实现最后R7相关3文件35测试通过。详见research/check-report.md。
- 共享调用方兼容：6文件56测试通过；与目标account-picker重叠，不累计为独立总数。
- Core原请求/权限/API/退役：5文件43测试通过，Core未修改。
- Views/UI/Web/Desktop typecheck通过；完整Views lint0错误、27条范围外既有警告；改动文件lint无警告。
- UI通配导出静态检查、scoped diff whitespace、Trellis manifests校验通过。
- impeccable机械检测一次：research/impeccable-detect.json为[]；最终visual-verdict96/pass。

## 浏览器范围与复现

- Playwright Chromium 145.0.7632.6；原始共享组件的production构建、正式CSS/双语资源、模拟API、内存StorageAdapter。
- 16个场景：desktop明暗、390×740/320×568粗指针明暗、延迟创建、双语desktop/320额度、6字父搜索、desktop/320未知结果原键恢复、精确身份、generic picker/dialog。
- 保存源码SHA与build PID、DOM/AX、焦点rgba/实际绘制、DOMRect/文本range、截图、查询/导航日志。只重验失败/变更相关场景；portal BODY裁切和100ms/95%入场/退出暂态采样修正有记录。
- 主证据research/browser/results-confirmed.json；最终图片和AX在research/browser/。源hash核对匹配当前修复。
- 复现：从项目根运行本任务research/preview/build.mjs current，再运行verify.mjs current；依赖均已安装，script使用task-owned loopback ephemeral listener并finally关闭自身Chromium/context/listener。
- 未运行make up/gc/down/destroy/check/test，没有清理或重启现有API/Web/Electron服务，没有真实账号或真实创建/分享请求。

## 剩余边界

- 模拟API/NavigationAdapter证明组件调用与数据行为，未证明真实后端提交、真实Next导航或Electron Main/native安装版验收。
- DOM/AX与合成触摸不等于真实读屏朗读、物理触屏或iOS Safari；实际系统IME未验收，composition/229保护由回归证明。
- 浏览器目录是受控基础字段目录；自定义属性编辑器popup与真实权限/额度数据未逐类型全覆盖。
- 主动关闭后后台完成的全局通知行为未扩展；scope/卸载guard与Core退役语义保留。
- 范围外27条lint警告与其它会话的未提交/后续改动保留；完整机器服务或其它Trellis任务不在此轮验收。
