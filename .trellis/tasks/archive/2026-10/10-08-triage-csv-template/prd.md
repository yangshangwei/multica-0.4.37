# 分拣台：CSV 导入模板与格式提示

## 背景

导入 CSV 对话框只写了"UTF-8、5 MiB、1,000 行"，没有说明表头和取值格式。
实际规则散落在 `server/internal/triagecsv/parse.go` 与 `triage_import.go`：

- 第一行必须是表头，表头非空且不重复；按中英文别名不区分大小写自动映射，上传后可在"映射列"中手动调整。
- 只有标题必填；优先级只接受小写 `urgent/high/medium/low/none`，填"高"会整行报错。
- 日期必须是 `YYYY-MM-DD`，截止日期不得早于开始日期。
- 标签可用 `;`、`,`、`|` 分隔，标签/项目/负责人只匹配本工作空间已有名称（成员用邮箱，智能体用名称），匹配不到会留空并警告，不会自动创建。
- 外部编号用于去重；状态、迭代、附件列不会被采用。

## 需求

1. 对话框提供"下载 CSV 模板"，模板表头与界面语言一致（中文界面用中文表头）。
2. 模板含两行示例：一行填满所有字段，一行只填标题，示例标题以"示例："开头，提醒用户替换。
3. 模板文件带 UTF-8 BOM、CRLF 换行，Excel/WPS 双击打开不乱码；用户另存为"CSV UTF-8"即可回传。
4. 在文件选择框上方放置显著的格式说明：必填项、优先级与日期格式、编码要求；全部字段说明可展开查看。
5. 已有预览后收起要点说明，只保留模板下载和字段说明入口，不挤占预览表格。
6. 不改后端解析规则、不改 API，旧服务器同样适用。

## 方案

- 单一模板来源：`packages/views/triage/triage-csv-template.json`（字段顺序、各语言表头、示例行、文件名）。
- 前端 `buildTriageCsvTemplate(locale)` 在浏览器内生成 CSV，Web 与 Desktop 共用；Desktop 已有 `will-download` 保存对话框。
- 后端测试直接读取同一 JSON，用 `triagecsv.Parse` 验证每个表头自动映射到预期字段、示例行零错误，且模板覆盖全部可映射字段。以后增删字段或别名时测试会提醒同步模板。
- 映射字段列表从对话框移到 `triage-ui.ts`，前端测试保证与模板字段一致。

## 验收标准

- [x] 中文与英文界面下载的模板，原样上传后预览零错误、所有列自动映射。（前端生成的真实字节经 `triagecsv.Parse` 验证：10 列全部映射、示例行零错误零警告）
- [x] 下载按钮在对话框首屏可见，键盘可达。（开发版 Desktop 中文界面实测截图）窄窗口未实测，头部使用 flex-wrap。
- [x] 中英文文案齐全，locale parity 测试通过。
- [x] `go test ./internal/triagecsv`、views 分拣台相关测试、views/web/desktop typecheck、eslint 通过。

## 追加：中文优先级别名（2026-10-08 用户确认）

- CSV 优先级接受界面上的中文标签：紧急、高、中、低、无优先级、未指定优先级，英文值不区分大小写；预览和入库都存规范值，失败导出保留原文。
- 只在 CSV 导入边界归一化，任务与分拣投递 API 仍只接受规范值。
- 模板示例继续写 `high`：新版客户端连旧服务器时模板仍能导入；旧服务器遇到中文标签会给出原有英文错误，不会创建任何任务。
- 验证：`TestParseNormalizesPriorityLabelsAndKeepsOriginalCells`、`TestTriageImportStoresCanonicalPriorityFromChineseLabel` 均先在旧解析器上失败、再通过；迁移到 566 的独立克隆库上 70 个 Triage handler 测试通过。

## 不做

- 不翻译服务端返回的英文行级错误。
