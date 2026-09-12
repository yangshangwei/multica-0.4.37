# 自动化模板的数据流与业务证据

研究基线：`502e47d0b`，2026-09-12 开始；当前工作树已有其他任务的未提交修改。

## 根因与数据流

- `service/builtin_autopilot_templates.go` 的 `Prompt()` 无语言参数，读取 9 个 key 对应的唯一 `PROMPT.md`。原文全部英文。
- `handler/autopilot_template.go` 的 `autopilotTemplateToResponse()` 仅本地化 title/description/category_label；`CreateAutopilotFromTemplate()` 把正文原样写入 `autopilot.description`。客户端未知字段被忽略，而不是返回 400。
- `packages/views/autopilots/components/template-create-autopilot-page.tsx` 展示 `template.prompt`，创建仅提交 key、负责人、项目、时区、language 等允许字段。详情显示数据库 description。
- `service/autopilot.go` 的 `dispatchCreateIssue()` 经 `buildIssueDescription()` 写入任务，再走普通任务派发；`dispatchRunOnly()` 建立队列执行，`handler/daemon.go` 领取时填充 `AutopilotDescription`。
- `daemon/prompt.go` 的 `buildAutopilotPrompt()`、`execenv/runtime_config_sections.go` 的 `writeWorkflowAutopilot()`、`execenv/context.go` 的 `renderAutopilotContext()` 原样注入 description。外框无强制英语回复要求，不需修改执行框架。
- 生产代码不按英文句子解析模板。注册表测试的去重短语断言需要转成中文等价合同。

## 已有实例与不可变合同

实例在创建时复制模板。后续读写、派发都读取数据库，不再按 key 加载模板。切换 locale、发布新模板不覆盖已有实例。保持 9 个稳定 key、顺序、cron、mode、分类 slug、时区默认值、负责人校验、规则版本、订阅与原子创建。

四个汇总：release-readiness / daily-change-review / bug-triage / weekly-progress-report，每次预建任务并发表评论。其余五个巡检不预建任务，查重后补充已有任务或在实质问题未覆盖时新建，无发现保持静默。

`handler/issue.go` 的 `validPriorities` 与 CLI 校验接受 `urgent/high/medium/low/none`，不接受 `critical`。缺陷分级正文应明确严重程度到合法 priority 的映射，证据不足保持 none；`contributor` 可改优先级，`observer` 只评论建议，拒绝写入时如实说明未应用。

## 验证入口与限制

- `service/builtin_autopilot_templates_test.go`：顺序、周期、模式、嵌入正文、查重、日期标题与四语标签。
- `handler/autopilot_template_test.go`：真实落库、来源、时区、规则版本、权限、触发器失败回滚。`handler_test.go` 数据库不可用时会返回成功退出码，必须确认实际 RUN/PASS 且无 Skipping tests。
- `service/autopilot_test.go`：`TestBuildIssueDescription*`、`TestInterpolateTemplate*`。
- `daemon/daemon_test.go`：`TestBuildPromptAutopilotRunOnly`；`handler/daemon_test.go`：`TestClaimTask_AutopilotRunOnly_PopulatesWorkspaceAndProjectContext`。
- `handler/autopilot_subscriber_test.go`：派发复制订阅者、通知和空订阅行为。
- `packages/views/autopilots/components/template-create-autopilot-page.test.tsx`：预览、负责人门槛、字段边界、错误及项目默认值。core 模板 schema 测试覆盖响应漂移。
- 旧 `e2e/autopilot-template.spec.ts` mock 了目录、负责人和创建，不能证明中文落库。
- `e2e/fixtures.ts` 的 `seedProjectRuntime()` 创建无进程、无模型账号的隔离假运行时；`requestJSON()` 调真实接口；`deleteFeatureWorkspace()` 清理专用工作区。可用于真正创建、读取、手动触发和检查队列。
- Go 测试经 `scripts/go-test-with-agent-cli-guard.sh`，浏览器使用当前 checkout API/Web 端口。

## 相关文件

生产内容限于 9 个 PROMPT、注册表中文文案与任务标题；同步 `builtin_autopilot_templates.go`、`handler/autopilot_template.go`、core 类型和 client、共享预览中的过时注释。按 CLAUDE.md 同步 `builtin_skills/multica-autopilots/SKILL.md` 与其 source map 的语言合同。更新 `.trellis/spec/server/builtin-templates.md` 时保留进入任务前的其他变更。
