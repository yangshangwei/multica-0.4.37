# 设计

最小行为修复在 daemon 的 writeWorkflowIssue 文本：保留交付→in_review，明确子任务等待人工/既有集成验收；done/cancelled 才推进阶段。后端不变。该选择保证普通同 issue mention/成员评论唤醒继续工作，同时不把“输出完成”当作“验收接受”。

Review Gate leader 补齐已有六类岗位职责与条件，保留每轮一个成员和作者独立修复的协议。角色版本递增为3，小队正文行为已经支持六类故不升其版本。既有实例保留复制文本；本次交付不宣称它们已自动升级。

文件：internal/daemon/execenv/runtime_config_sections.go、runtime_config_test.go；service builtin working-on-issues / squads SKILL 与 source-map；review-gate-lead/INSTRUCTIONS.md、builtin_agent_templates_roster.go、builtin_squad_templates.go、builtin_agent_autonomy_test.go（或对应模板测试）；apps/docs/content/docs/squads.mdx、squads.zh.mdx；packages/views/locales/en/squads.json、zh-Hans/squads.json。

500 行 skill 限制通过替换现有句子而非添加长章节；source-map 容纳机制细节。主会话更新专用 spec，不覆盖其他任务正在修改的公共 index。

中文 docs 改动按现有 docs-bundle 合同同步 server/internal/docs/content/pages/squads.json；仅保留生成器产生的必要 squads/manifest 差异。
