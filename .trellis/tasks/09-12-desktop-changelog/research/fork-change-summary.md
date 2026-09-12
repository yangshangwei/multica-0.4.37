# Fork changes included in the initial reading experience

The fork is a snapshot repository with a reachable local base tag `v0.4.37` at `f1c059edee4ec39d464915e9c9a83cf6c0c37925`. Later upstream tags are not its ancestors. HEAD is moving because separate tasks are being committed concurrently; the generated preview must record the exact commit it describes.

## User-facing committed changes

| Group | Chinese summary for readers | Evidence |
|---|---|---|
| Added | 在帮助菜单中直接阅读内网文档，无需访问公网。 | 0c96d792b, 91ea4362a, 3a8f1050d |
| Added | 使用内置角色、skill 和小队模板，更快组建协作团队。 | ca3a79d18, f31d33fa2, 09dfbf350 |
| Added | 创建项目时采用默认小队；配置失败后可继续完成。 | ef494ddd3, af2977950 |
| Added | 从模板创建定时或事件触发的自动化，先预览再确认。 | 69711dc44, 301f26a16 |
| Added | 从 skill 模板开始编辑，再保存为工作区自己的版本。 | 0008e8665 |
| Added | 支持包含桌面安装包的离线部署与可重复的服务端升级包。 | 68ecd9f22, 3d28c619d |
| Changed | 内置智能体指令和 skill 描述提供中文内容，并保留工作区的个性化配置。 | a99cfe687, c9d6cae6b, c4ae6ac68, 5ceecd789 |
| Changed | 按运行时实际支持的能力刷新模型列表；MCP 连接替换更明确。 | c29c5af38, 8509e15f7, 8418279e4 |
| Changed | 智能体自主执行遵守工作区的权限和审批边界。 | d75e8642b, a7b503ee0, ed26fd842 |
| Fixed | 旧会话的延迟鉴权响应不再清除新会话。 | e6f34a1a7 |
| Fixed | Windows 终端与 PowerShell 输出正确处理中文编码。 | 8b57ed346, e6cabfdea |
| Fixed | 多次执行保留同一会话的工作分支，并等待前一次释放工作目录。 | 35e71ce4f, b25630a44 |
| Fixed | 自动化生成的任务标题带日期，并记录来源。 | 779dbda3a |
| Fixed | 项目小队配置失败时保持项目可恢复，避免重复创建。 | af2977950 |

These summaries are representative, attributed local **unreleased** work, not fabricated published releases. Generator output should include all eligible commit records by default; a public Release-note trailer can supply editorial Chinese wording going forward. The preview's hand-curated summaries must not be duplicated as fresh releases or silently added to historical upstream entries.

## Concurrent dirty work at intake

`initial-worktree-status.txt` records work already present for intranet messaging configuration and requirement-skill scope. It informs provenance only. Published generation consumes git objects, never the dirty working tree. A change becomes eligible only after it is committed and falls within the selected release range.
