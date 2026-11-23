# 新建协作任务页面收尾验收

本次完成现有底部工具栏版本的页面收尾。用户要求直接处理，因此未创建或
激活 Trellis 任务。验收范围是共享 Web／桌面端创建组件；其他协作阶段、
后端、文档站和翻译工作的已有改动不属于本次提交。

## 页面改动

- 紧凑接收人按钮继续复用 `PillButton`，恢复完整、可换行的账号 ID，保留
  同名账号辨识、外部账号标记和默认选择器行为。
- 创建正文恢复独立的接收人状态提示和可见的 AI 参与说明，复用现有中英文
  文案，保留 AI 选项的 `aria-describedby` 关联。
- 保留底部接收人、AI、初始字段工具栏，以及正文独立滚动、描述编辑器利用
  剩余高度、附件列表和错误反馈在正文内显示的结构。
- 将 AI 选项与初始字段按钮放进可收缩的单行组，预留稳定的 20rem 布局宽度，
  适配窄窗口，避免字段摘要变宽时增加工具栏行数、挤住正在编辑的字段。
- 新增独立接收人状态回归，覆盖受限弹窗和普通独立面板，验证选择账号和
  改回自己后的状态更新。

生产代码只改动
[`account-picker.tsx`](../../packages/views/collaboration/account-picker.tsx)、
[`creation-form.tsx`](../../packages/views/collaboration/creation-form.tsx) 和
[`creation-form.test.tsx`](../../packages/views/collaboration/creation-form.test.tsx)。
布局与验证要求同步到
[`collaboration-creation.md`](../../.trellis/spec/views/frontend/collaboration-creation.md)。
本次没有增加依赖、状态仓库、观察器或另一套表单／上传实现。

## 验证结果

| 检查 | 结果 |
| --- | --- |
| 协作、共享描述助手、语言键一致性测试 | 36 个文件、425 项测试通过 |
| 新增独立接收人状态回归 | 修复前按预期失败，修复后 2 项通过 |
| Views 类型检查 | 通过 |
| Views 完整 ESLint | 0 个错误，27 条既有警告；相关页面无新警告 |
| 页面 Impeccable 单次检测 | 信息修复后的扫描为 `[]`；最终占位调整另经 lint 和浏览器确认 |
| 页面及规范差异检查 | 通过 |
| 最终实际组件浏览器矩阵 | 13 个场景通过，0 个页面错误 |
| 桌面原生关闭按钮 | OS 鼠标点击能关闭，点击和拖动均不移动窗口 |
| 最终构建源码核对 | 475 个项目源码指纹一致，无不匹配项 |

类型检查、完整 ESLint 和测试均在最后一批页面改动之后运行。浏览器使用
同一份冻结生产构建。所有验证拥有的浏览器、Electron 应用与回环监听器
均已关闭。

## 实际渲染与交互

最终矩阵覆盖 722×660、1280×900、390×740、320×568，浅／深色主题；
320 像素宽度同时覆盖中英文和长接收人名称。实际文本范围核对了选择列表
和已选按钮内的完整账号 ID，正文滚动后可读的 AI 说明，以及独立状态提示。
窄屏按钮尺寸至少 44 像素，工具栏和提交区域保持可达，页面没有横向溢出。

初始字段通过键盘打开、修改优先级和关闭；当前字段在摘要更新后保持可见，
关闭时焦点留在工具栏按钮上。关窗重开后保留描述和接收人。长内容场景覆盖
32 个附件、长 AI 输出与补充问题、调整尺寸时的表单身份和生成保留、原始
文本撤销，以及创建拒绝后的草稿、文件和错误反馈。补充的 520×568 回归验证
首次填写开始日期时工具栏保持 126→126 像素，获焦控件底边为 284.4 像素，
完整位于正文底边 292.6 像素之内。

原生检查使用隔离的 Electron 窗口、真实共享组件和 48 像素原生拖动区域。
Quartz OS 鼠标事件在关闭按钮上产生 `pointerdown`、`mousedown`、`click`，
关闭弹窗且窗口坐标不变。拖动后在按钮外释放保持弹窗打开，窗口仍不移动。
另有 Chromium 点击作为补充；原生输入结论来自 OS 事件记录。

## 修复与确认记录

基线四组截图均显示已选账号 ID、AI 说明被隐藏，且没有独立接收人状态。
恢复信息后的第一轮有 8 个场景通过，4 个 320 像素场景发现字段摘要变宽后
工具栏多换一行，导致当前优先级控件被裁切。修正工具栏分组后，12 个场景
通过；独立复核进一步确认 520×568 的外层换行仍会裁切 32 像素的开始日期
输入框。该场景加入实际 CSS 回归，修复前按预期失败。补足稳定占位后，最终
13 个场景全部通过，视觉判定为 95／通过。后续调整均针对这些已确认的输入
可达性问题。

- [基线记录](../../artifacts/qa/collaboration-create-20261114/browser-baseline/results.json)
- [首轮记录与失败原因](../../artifacts/qa/collaboration-create-20261114/browser-round-1/results.json)
- [520 像素修复前回归](../../artifacts/qa/collaboration-create-20261114/browser-round-2/targeted/results.json)
- [最终浏览器记录](../../artifacts/qa/collaboration-create-20261114/browser-round-3/results.json)
- [桌面原生输入记录](../../artifacts/qa/collaboration-create-20261114/native-round-3/results.json)
- [最终构建与源码指纹](../../artifacts/qa/collaboration-create-20261114/preview/round-3-build.json)
- [检查汇总](../../artifacts/qa/collaboration-create-20261114/quality-checks.json)

代表截图：
[桌面浅色](../../artifacts/qa/collaboration-create-20261114/browser-round-3/toolbar-722-light-zh-Hans.png)、
[窄屏深色](../../artifacts/qa/collaboration-create-20261114/browser-round-3/toolbar-320-dark-zh-Hans.png)、
[窄屏英文](../../artifacts/qa/collaboration-create-20261114/browser-round-3/toolbar-320-light-en.png)、
[开始日期焦点](../../artifacts/qa/collaboration-create-20261114/browser-round-3/intermediate-date-focus.png)、
[长内容补充问题](../../artifacts/qa/collaboration-create-20261114/browser-round-3/long-content-dark.png)。

## 验证边界

预览使用实际共享组件、CSS 和 Core 请求／SSE 解析，传输和草稿存储由隔离
夹具模拟。它证明当前页面的渲染与集成行为；本次没有验收真实模型输出质量、
付费额度、生产服务器提交、安装版应用的完整环境、物理触屏／系统输入法或
屏幕阅读器朗读。原生检查只覆盖上述窗口和关闭按钮的输入边界。

构建记录保留当时工作区的全部已打包项目源码和输出指纹；既有 Core、阶段
审批与翻译改动没有并入本次页面提交。重放使用本项目已安装的依赖：

```sh
node artifacts/qa/collaboration-create-20261114/preview/build.mjs replay /tmp/multica-collaboration-create-replay
node artifacts/qa/collaboration-create-20261114/preview/verify.mjs replay
node artifacts/qa/collaboration-create-20261114/preview/native-verify.mjs replay
```

原生重放需要已授予的系统辅助功能权限；夹具不会自行请求该权限。临时构建
目录可以替换，构建脚本会按当前检出路径重新生成预览 CSS。

## 提交前恢复核对

恢复收尾时，三个页面源码文件、样式、文案和浏览器验证脚本仍与最终构建
一致。475 个记录文件中有 474 个指纹一致，唯一变化是其他任务的
`packages/core/collaboration/stage-test-fixtures.ts`；该文件不纳入本次提交。
上文 13 个浏览器场景和原生输入结论仍指向保留的冻结构建，没有改写为新一轮
运行结果。当前工作区另行复跑创建表单与接收人选择器的 40 项测试，以及三个
页面文件的 ESLint，均通过。

本轮两次全量 Views 类型检查未通过。第一次报错是未提交的阶段审批文件
`packages/core/collaboration/stage-actions.ts:36` 缺少类型导入；核对时并行
工作已补上导入。再次检查报错于同属阶段审批的未提交文件
`packages/views/collaboration/stage-forms.tsx:102`：调用需要两个参数，实际
传入一个。两次报错均不在本次页面改动中，这些文件也不纳入本次提交。
上文完整类型检查通过的记录属于先前验收时的工作区，不能据此宣称当前整个
工作区类型检查通过；当前页面测试、lint、源码一致性与保留的浏览器验收分别
成立。详见[提交前检查记录](../../artifacts/qa/collaboration-create-20261114/resume-checks.json)。
