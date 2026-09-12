# 源码调研记录

这是实施前的调研快照；最终源码和实例转换结果见 `docs/plans/2026-09-12-builtin-agent-instructions-zh.md`。

日期：2026-09-12。范围：当前 checkout 中用户可配置的内置智能体、角色模板与关联小队指令；未查询运行数据库。

- 清单为 Mika 1 个、工作角色 8 个、小队负责人 8 个、小队公共指令 8 份、角色 skill 7 份。前四类指令全部英文，角色 skill 正文全部含简体中文。
- 角色注册表：`server/internal/service/builtin_agent_templates_roster.go:14`。完整 key、中文角色名、行为版本、权限和文件位置已记入 `docs/plans/2026-09-12-builtin-agent-instructions-zh.md`。
- 原因：模板 API 的 language 参数只改变标题/描述，`Instructions()` 不按语言选择正文。证据：`server/internal/service/builtin_agent_templates.go:123`、`server/internal/handler/agent_template.go:69`。
- 新建模板复制正文到实例，不会持续同步。小队配置会复用已有角色、保留其定制。证据：`server/internal/handler/agent_template.go:187`、`squad_template.go:458`。
- 前端编辑的是原始 `agent.instructions`，实际更新接口为 PUT；普通更新 SQL 未提供正文比较交换。证据：`packages/views/agents/components/tabs/instructions-tab.tsx:39`、`packages/core/api/client.ts:1794`、`server/pkg/db/queries/agent.sql:139`。
- Mika 系统段读取服务器内嵌文件，产品系统段不写入实例；可编辑正文是团队补充。证据：`server/internal/service/builtin_agents.go:50`、`:73`。
- 执行时原样写入角色正文，然后还会加入自主权限规则、小队简介、运行时规则和 skills。证据：`server/internal/handler/daemon.go:2167`、`:2198`、`:2346`，`server/internal/daemon/execenv/runtime_config_sections.go:109`。
- 中文界面自身尚有英文标签和不适合中文团队的 placeholder。证据：`packages/views/locales/zh-Hans/agents.json:543`、`:552`。
- 测试固定了英文标题，需要保持语义契约后更新断言；前端“指令原样展示”的测试允许后端返回中文，禁止展示层偷偷改写内容。证据：`server/internal/service/builtin_agent_templates_test.go:101`、`packages/views/agents/create/agent-configuration-panel.test.tsx:28`。
- 现有角色 skill 中文正文及保留历史定制的规则：`.trellis/spec/server/builtin-templates.md:18`、`.trellis/spec/views/frontend/builtin-skill-localization.md:10`。

只读文件计数脚本已核验目录数量及正文汉字分布。`make status` 显示 API 健康响应归属不匹配、Web 停止；不把该环境当作 UI 或实例验证证据。未执行真实智能体、运行产品测试或修改工作区数据。
