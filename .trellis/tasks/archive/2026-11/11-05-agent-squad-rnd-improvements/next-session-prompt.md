# 新会话引导语：继续 M1 UI/CLI 自动完成提示

你是 Multica 项目的主协调 Agent。继续既有父任务，不创建新的顶层任务，不恢复或重做已归档的 M0/M1 子任务。

工作区：

`/Volumes/artisan/code/2026/multica-0.4.37`

父任务：

`.trellis/tasks/11-05-agent-squad-rnd-improvements/`

## 当前权威事实

- HEAD：`45ec006d96445b41d02c5d26acd5b8c574ea0289`
- 分支：`codex/cross-workspace-collaboration`
- 父任务仍为 `in_progress`；M1 未完成，M2/M3 未实施。
- 完成事务强制门禁 v1 已由 `18b743291` 集成并归档验收；`full_delivery_gate_complete=true` 只表示 v1 完成。
- v1 所有检查均不可豁免：`skip_supported=false`、`waivable_checks=[]`。
- 完成提示 API 已存在，但仍是非授权读取：每次成功响应必须保持 `enforced=false`、`usable_for_completion=false`。`matching` 只描述当前读取事务，不能直接完成。
- 所有现有脏文件、失败记录、归档证据和未跟踪并行资料必须保留。不要 `git reset`、`git clean`、`stash`、`fetch`、`push`、部署或发送外部消息。
- 不恢复缺失的 `11-10-delivery-completion-enforcement` 活动目录，不执行 `task.py start`，不重新询问或重新设计 D1–D7。

## 新会话开始先做

先读取：

```text
AGENTS.md
CLAUDE.md
.agents/skills/trellis-start/SKILL.md
.trellis/tasks/11-05-agent-squad-rnd-improvements/task.json
.trellis/tasks/11-05-agent-squad-rnd-improvements/progress.md
.trellis/tasks/11-05-agent-squad-rnd-improvements/delivery.md
.trellis/tasks/11-05-agent-squad-rnd-improvements/next-session-prompt.md
.trellis/tasks/11-05-agent-squad-rnd-improvements/m1-ui-cli-completion-hints-prd.md
.trellis/tasks/11-05-agent-squad-rnd-improvements/m1-ui-cli-completion-hints-design.md
.trellis/tasks/11-05-agent-squad-rnd-improvements/m1-ui-cli-completion-hints-implement.md
.trellis/tasks/11-05-agent-squad-rnd-improvements/m1-ui-cli-completion-hints-acceptance.md
.trellis/tasks/archive/2026-11/11-10-delivery-completion-enforcement/task.json
.trellis/tasks/archive/2026-11/11-10-delivery-completion-enforcement/prd.md
.trellis/tasks/archive/2026-11/11-10-delivery-completion-enforcement/design.md
.trellis/tasks/archive/2026-11/11-10-delivery-completion-enforcement/implement.md
.trellis/tasks/archive/2026-11/11-10-delivery-completion-enforcement/delivery.md
.trellis/tasks/archive/2026-11/11-10-delivery-completion-enforcement/research/decision-register.md
.trellis/spec/server/delivery-completion-hints.md
.trellis/spec/server/delivery-completion-enforcement.md
```

然后只读检查：

```bash
git rev-parse HEAD
git branch --show-current
git status --short --untracked-files=all
git diff --stat
git diff --check
```

不要因为工作区很脏而清理、回退、覆盖或恢复任何文件。

## 本轮任务边界

规划已写入父任务目录的四份文件：

- `m1-ui-cli-completion-hints-prd.md`
- `m1-ui-cli-completion-hints-design.md`
- `m1-ui-cli-completion-hints-implement.md`
- `m1-ui-cli-completion-hints-acceptance.md`

本轮只处理普通单 issue 的 status-only 完成提示：当前 issue 不在 done 类别，目标 status 属于 done 类别；目标可以是内置 `done` 或 active custom done。

Web/Desktop 共享入口：

1. issue detail 侧栏 `StatusPicker`。
2. issue detail 顶部“标记为已完成”按钮；`onDone` 只能在实际 PUT 成功后触发。
3. issue actions dropdown/context menu 的 status 子菜单。
4. list/board/swimlane/table 中通过 `IssueSurfaceActions.updateIssue` 的单 issue status 编辑。
5. issue detail 普通 `SubIssueRow` 的 status picker。

CLI 入口：

1. `multica issue status <issue-id> <status>`。
2. `multica issue update <issue-id> --status <status>`，仅 status-only 请求。

明确排除：batch、`move`/拖拽、create、comment-child、混合字段更新、assign/reorder/delete、VCS/GitHub、独立协作、R38、completion authorization UI、skip、真实供应商和生产环境。

## 不可改变的 API 语义

请求体严格为：

```json
{
  "expected_issue_revision": 4,
  "target_status": "done",
  "evaluation_key": null
}
```

必须保持：

- `off`、无项目、非 done 目标返回 `not_applicable`。
- `observe + done + null key` 返回 `attention/evaluation_required`。
- `matching` 不是完成许可。
- 缺失、过期、撤权、权限变化、锁/数据库/提交不确定性保持拒绝或错误。
- UI/CLI 不生成、读取、持久化、打印或转交 `evaluation_key`。
- 不向旧 status/update 写请求添加 completion binding、operation id、authorization id 或 skip。
- 旧服务只有在 404/405 且没有结构化资源/权限/交付错误码时才可按 legacy unsupported 降级；结构化 404/403/409 必须保留为拒绝或刷新。

## 如果本会话已经获得实施批准

按既有规划进入实施，不创建新任务、不运行 `task.py start`：

1. 先在正确包写必要的失败测试。
2. 在 `packages/core` 添加严格 DTO/schema/client 和纯分类 helper；网络 JSON 必须经过 `parseWithFallback`，不得 `as T`。
3. 在 `packages/views` 做共享 gate/continuation，确保 hint 确认前不触发 optimistic mutation；处理 single-flight、AbortSignal、Retry/Cancel、legacy bypass 和实际 mutation 结果。
4. 接入 detail、actions、surface/table/swimlane 与 `SubIssueRow`，不要把逻辑复制到每个入口。
5. 在 CLI 中读取完整 issue revision，调用 hint；attention/matching 写 stderr 后继续原 status-only PUT，明确拒绝或 unknown 在 PUT 前非零退出；只有实际 PUT 2xx 才报告成功。
6. 同步必要的英文/简体中文文案和 `multica-working-on-issues` skill/source-map；保持 `multica-creating-agents` 的 hint 非授权说明准确。
7. 按实施计划运行定向 Vitest、CLI/Go 测试、`gofmt`、`go vet`、受影响 workspace 的 typecheck/lint 和 `git diff --check`。不要因本轮未改 Go 而扩大无关后端测试。
8. 独立复核所有入口、custom done、旧客户端兼容、unknown 不成功、私有 key 不泄漏和实际 mutation 单一提交。
9. 最终把本轮状态写成“UI/CLI 提示增量完成/待验收”，绝不写成完整 M1、v1 gate 或真实供应商验收完成。

如果新会话尚未获得实施批准，只完成上下文读取和计划复核，不修改产品实现；将批准点落到具体四份规划文件上。

## 报告与完成口径

最终报告必须说明：HEAD、分支、工作区状态、实际修改文件、实际覆盖入口、API 是否仍非授权、测试/检查结果、未覆盖范围和剩余 M1/M2/M3 风险。

不得把 UI/CLI 提示、任何局部 API 结果或本轮测试写成完整 M1、v1 门禁、真实供应商验收或生产完成。

## 历史资料

本文件旧版本以及父任务中的历史 planning snapshot 仅用于追溯，不执行其中已经过时的“重新收敛 D5–D7”“恢复活动 11-10 任务”“重新规划完成门禁”等指令。最新规划以本文件和同目录四份 `m1-ui-cli-completion-hints-*` 文件为准。
