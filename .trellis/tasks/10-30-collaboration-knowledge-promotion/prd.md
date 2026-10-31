# 知识候选逐层沉淀

## Goal
让协作单结论成为带出处、范围和验证证据的候选，经人确认后进入项目或空间上下文。

## In scope
issue-scoped candidate、提议/确认权限、project/workspace revision、版本/纠错/过期/撤回和后续注入过滤。

## Confirmed authority and dependencies

- 用户已确认：发起人或当前处理人提出；目标项目负责人/管理员确认项目知识；目标 workspace owner/admin 确认空间知识；技能层不纳入本轮。依据：会话 `01a18a36-e482-7840-9900-3ff8174b4ce7` 用户轮次 14。确认操作必须由人执行。
- 依赖可靠交接子任务的来源、版本、授权与记录覆盖合同。当前仍为 planning，不因权限已确认就跳过数据流设计。
- 代码核对：Project Context 使用 project description/resources（`runtime_config_sections.go:417`）；workspace 使用 `workspace.context`（`:184`）。项目更新有 expected revision，workspace 当前直接写 context，不能假定两者都有可复用的候选版本/撤回服务。
- 确认不得把来源协作单的可见范围无条件扩散到目标所有运行；正文注入需同时校验候选状态、有效期、来源可见权与目标适用范围。撤回/过期必须影响下一次注入，不只修改候选 UI 状态。

## Out of scope
未经确认写入生效上下文、自动写入技能层、向量检索、跨 Server。

## Acceptance
未经确认不改变任何目标上下文；确认记录 candidate/source/version/authority；纠错、过期、撤回和来源权限变化可追溯并阻止后续注入；技能层只形成后续 ADR/研究。
