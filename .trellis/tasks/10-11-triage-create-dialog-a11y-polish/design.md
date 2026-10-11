# 技术设计：待分拣任务创建弹窗优化

## 边界

改动集中在 views 的 triage 创建表单、其使用的字段编辑器/选择器触发器，以及 UI Dialog 的可配置关闭标签。服务端合同不变；创建请求继续发送现有 `CreateTriageItemInput`。

## 组件方案

1. `TriageField` 增加可复用的 label/control 关联能力，普通 input 使用 `htmlFor`，编辑器通过 `aria-labelledby` 关联可见标签；picker 通过 `triggerAriaLabel` 使用相同的本地化字段名称，并单独提供选中值的辅助描述。
2. `TriageFieldsEditor` 增加候选模式，仅在创建弹窗中切换到已有的 `candidate_project` / `candidate_assignee` 本地化文案；接受弹窗保持现有正式字段文案。
3. `PropertyPicker` 与四个 picker wrapper 接受可选的触发器 accessible name；创建表单通过字段 label 传入。创建表单的字段容器提供边界、焦点和触控高度样式，避免修改所有调用方的全局 picker 外观。
4. `TitleEditor` 补充 `aria-describedby` / `aria-invalid` 转发，`ContentEditor` 补充 `aria-invalid` 转发；创建表单为标题和描述提供持久标签、必填状态和字段错误描述。
5. 创建表单在提交前执行纯客户端校验；对 `ApiError` 的已知 4xx 文案做有限模式映射，其他错误使用 `clientErrorMessage` 或本地化通用错误，保留用户输入。
6. 创建弹窗改为 `flex` 容器，中间表单区滚动，`DialogFooter` 保持可见；仅该弹窗通过现有 `pointer-coarse` 选择器放大关闭按钮。共享 Dialog 增加可选 `closeLabel`，默认行为不变。

## 数据流与兼容性

- `fields.project_id` / `fields.assignee_*` 仍映射到 `candidate_*` payload 字段。
- 校验失败只更新本地错误状态，不触发 mutation；上传中和提交中仍保留原有门禁。
- 候选模式是展示层 prop，不改变接受动作或历史记录中的字段 key。
- `DialogContent.closeLabel` 为可选 prop，现有调用方无需修改；创建弹窗传入 triage locale 的 `close` 文案。

## 回滚

所有改动为前端组件和 locale 的可逆修改。若 picker 视觉范围过宽，优先回滚创建表单的 scoped class；若字段错误映射不兼容旧服务端，保留客户端校验并回退未知错误到全局 alert。

## 验收补充

- 创建字段编辑器使用 `idPrefix="triage-create"`；接受流程保留默认 `triage`，避免并存表单的标签和错误提示串联，同时保留原有接受焦点选择器。
- 显式命名的负责人选择器使用纯姓名作为辅助描述，避免头像首字母重复朗读；其他选择器和默认触发器行为保持兼容。
- 错误提交聚焦第一个非法字段；上传期间拦截按钮和快捷键，已删除附件不进入请求，未修改请求重试保留标识。
