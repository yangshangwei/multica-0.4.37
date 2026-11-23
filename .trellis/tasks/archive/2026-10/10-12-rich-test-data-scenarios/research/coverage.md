# 测试用例与数据场景映射

目标是让已运行的本地后端和 Desktop 有可直接浏览、继续操作的业务数据。既有 E2E 夹具按用例新建后清理工作区，因此不能直接作为长期手测数据执行。

| 数据场景 | 依据 | 验收入口 |
| --- | --- | --- |
| 7 种任务状态、5 种优先级、成员/智能体/未指派、长标题、父子任务 | `e2e/issues.spec.ts`、`e2e/issue-table.spec.ts`、`e2e/task-lifecycle-qa.spec.ts` | 任务看板/列表/详情、我的任务 |
| 项目 planned/in_progress/paused/completed/cancelled、逾期、未指派、待验收 | `e2e/projects-p1-health.spec.ts`、`e2e/projects-p1-lifecycle.spec.ts`、`e2e/projects-p1-desktop.spec.ts` | 项目概览/风险下钻 |
| 计划迭代与既有冻结历史 | `e2e/iterations-i1-desktop.spec.ts`、`e2e/iterations-i1-history.spec.ts` | 迭代/任务范围/历史 |
| 待受理、暂缓、拒绝、重复请求 | `e2e/triage.spec.ts`、`e2e/triage-branches.spec.ts`、`e2e/triage-desktop.spec.ts` | 分拣台各子视图/审核历史 |
| 评论、回复、订阅、提及、未读/已读/归档通知 | `e2e/comments.spec.ts`、`e2e/core-business-journeys.spec.ts` | 任务活动、收件箱 |
| 聊天消息、Markdown、快捷建议 | `e2e/chat-recovery-branches.spec.ts`、`e2e/quick-actions.spec.ts` | 聊天会话 |
| 完成/失败/取消/排队执行记录 | `e2e/core-business-journeys.spec.ts`、`e2e/platform-admin-executions.spec.ts` | 智能体执行记录/任务执行日志 |

## 环境证据与数据保护

- `/health`：pid `13920`、commit `5804f3936`，对应当前 checkout 后端；registry 显示 mismatch 是后续纯任务记录 commit 引起。
- `/api/config`：password auth、平台管理和受管终端开启。
- Owner 的当前本地 Desktop profile token 经内存读取验证；所有接口检查不打印 token，不生成密码或登录凭据。
- 原数据 baseline 已保存在 `.artifacts/rich-test-data-20261012/baseline.json`。目标工作区 15 tasks、2 projects、6 iterations、14 agents、6 runtimes、4 comments、14 inbox、1 chat session、3 squads。
- 新数据独立 identity/marker；既有 active iteration、settings 和运行时不写入。reset 验证应在隔离测试数据库进行，正式填充后的手测数据保留。
- 原生验收用独立临时 Electron profile，加载正在运行的 `localhost:5666` renderer 和当前 preload；只读取真实 API。Native services 使用现有 E2E harness 隔离，daemonStarts 必须为 0。
- 验收 harness 原有 sandbox/webSecurity 与实际 Desktop 配置不同，首轮连接因 CORS 失败；仅在 ignored artifact harness 对齐 `webSecurity: false` 后连接正常。没有修改后端 CORS 或 Desktop 产品代码。
