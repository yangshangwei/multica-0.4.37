# Research: 业务协作

- 日期：2026-10-14
- 方式：原子代理中断后由主会话接手，只读核查当前实现。
- 完整结论和精确证据见 `../audit-report.md` 以及 `findings.json`。

## 确认问题

- D08（P2）：成员说明遗漏分享邀请，并把邮箱邀请的限制泛化
- D09（P2）：@all 不能覆盖同条评论里的显式智能体提及
- D10（P2）：编辑触发评论会取消旧内容对应的未完成执行
- D11（P2）：小队页声称显式转交其他智能体时仍自动唤醒队长

## 覆盖与源码入口

- `workspaces`：创建与设置权限、离开/移除的运行时和任务清理、数据范围。源码：`server/internal/handler/workspace.go`、`server/internal/handler/workspace_revoke.go`。
- `members-roles`：角色管理、最后一名 owner 限制、邮箱邀请、分享邀请入口。源码：`server/internal/handler/workspace.go`、`server/internal/handler/share_link.go`、`packages/views/settings/components/members-tab.tsx`。
- `issues`：状态分类与触发、权限、task 生命周期、项目/迭代关系、删除。源码：`server/internal/service/issue_trigger.go`、`server/internal/handler/issue.go`、`server/internal/issuestatus/issuestatus.go`。
- `triage`：默认开关、人类审核、正式接纳门禁、稍后处理、CSV、接纳与执行分离。源码：`server/internal/handler/triage.go`、`server/internal/handler/triage_actions.go`、`server/internal/handler/triage_import.go`。
- `projects`：状态、正式任务统计、健康指标、描述修订、验收、删除与小队关联。源码：`server/internal/handler/project.go`、`server/internal/projecthealth/counts.go`、`server/internal/projecthealth/snapshot.go`、`server/internal/handler/project_update.go`。
- `project-resources`：Git 与本地目录、Desktop 文件选择、in_place/worktree、能力门禁与回放限额。源码：`server/internal/handler/project_resource.go`、`packages/views/projects/components/project-resources-section.tsx`、`server/internal/daemon/execenv/local_worktree.go`。
- `iterations`：手动生命周期、权限、任务加入限制、统计、关闭快照和操作限额。源码：`server/internal/handler/iteration_management.go`、`server/internal/service/iteration_lifecycle_preview.go`、`server/internal/iteration/stats.go`、`server/internal/iteration/history_snapshot.go`。
- `squads`：模板、小队成员/leader 权限、显式路由与兜底、归档转交。源码：`server/internal/handler/squad.go`、`server/internal/handler/squad_template.go`、`server/internal/handler/comment.go`。
- `comments`：编辑/删除权限、触发重算、旧执行取消、@all 与显式提及。源码：`server/internal/handler/comment.go`、`server/pkg/db/queries/agent.sql`。
- `mentioning-agents`：显式和隐式路由、5 分钟延迟兜底、合并、/note、@all 和 Access。源码：`server/internal/handler/comment.go`、`server/internal/handler/comment_trigger_preview_test.go`。
- `chat`：私聊所有者读取门禁、发送时重新校验 Access、离线排队、归档和删除。源码：`server/internal/handler/chat.go`、`server/internal/service/task.go`。
- `inbox`：通知类别和偏好、已读/未读、归档、站内通知与执行的区别。源码：`server/internal/handler/inbox.go`、`server/internal/handler/notification_preference.go`、`server/cmd/server/notification_listeners.go`。

## 局限

未执行真实部署、真实账号操作、数据库集成测试或图片视觉对照。没有修改本任务之外的文件。
