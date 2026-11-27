# 运行时物料代码交付验证 — 2026-11-27

当前交付是所选运行时执行路径与确定性验证。真实模型、真实 GitHub/GitLab 远端写入、完整全仓 Go 验收及安装包验收仍是独立后续项，不因本报告通过而自动完成。

## 实现与简化

- 项目只选择 `execution_runtime_id`；删除物料连接选择、独立方案 Agent 选择及服务端物料 provider 接线。其他 GitHub App/VCS 功能保留。
- 复用任务队列和物料内部载体；任务与不可变操作一起入库。发布、审核记录同步和批准后合并都经选定运行时，daemon 只读核验。
- 回执绑定任务、运行时、领取代次、描述摘要和主机独立凭据；持久回执原样重放。正式基线来自完整差异、模式、固定提交字节和实际合并事实。
- 方案启动前也在运行端读取基线。自动关闭、旧待办保留、同 issue 人工终态重试、历史保留和运行时配置变更拒绝均保留。
- 恢复区分未发生、已发生和不确定；审核记录恢复后按原阶段继续。实际 `merged_at` 与原授权时间比较，延迟观察不会擅自延长授权。
- 无新增依赖或通用工作流引擎。

## 验证结果

| 层级 | 证据 |
| --- | --- |
| 独立全范围检查 | `independent-check.md`；四项发现均修复，无遗留阻塞项 |
| 最终服务竞态回归 | 新隔离数据库，50 项顶层测试通过，含普通队列领取、并发容量、凭据完成和启动 |
| 最终运行时完整链 | GitHub/GitLab 均通过竞态测试：实际队列→测试构建的原生执行器→本地远端效果→daemon 独立读取→服务器接收 |
| handler/router/迁移 | 84 项 handler、6 项 router 竞态、1 项实际迁移保护测试通过，详见 `backend-implementation.md` |
| 完整默认包测试 | daemon 1010、CLI 538、material 2、materialgit 12、protocol 61 项顶层通过；3 项平台/子进程辅助测试明确跳过 |
| 共享前端 | core 15、共享物料/选择器 35 项通过；新增 no_effect 文案与人工恢复回归 |
| 静态与构建 | 强制全仓 lint/typecheck 17 个任务通过；独立复核 lint/typecheck、Go 全项目 build、相关包 vet、sqlc、gofmt、diff-check 通过；Web/Electron 生产构建通过 |
| 生产界面最终源复验 | GitHub、GitLab × Web、Desktop 共 4/4 通过，每 provider 约 2 分钟；源码稳定且运行后与当前代码一致，详见 `frontend-final-browser.json` |

默认包测试证据：`/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-material-default-final-qo5jbgqq`。独立最终服务及完整链证据见检查报告中的私有目录。凭据只保存在仓库之外的私有分配记录。最初无输出且未运行的后台启动尝试不计入测试结果。

`implementation-scope.json` 列出本任务代码/规范路径；`source-manifest.json` 记录文件摘要。父任务文档与无关任务、设计、发布和历史 QA 产物未纳入提交范围。

生产界面最终证据保存在 `/private/tmp/multica-material-runtime-ui-Y7BDfo`：GitLab 为 `gitlab-final3/material-run-9UmYiA`，GitHub 为 `github-final/material-run-npcJe1`。测试 API、Web、daemon、Electron、本地 provider fixture 与专用数据库容器已停止；数据库和报告保留。视觉复核通过，设置截图包含显式运行时选择。

## 保留限制

- 此次没有调用真实模型，也没有向 `yangshangwei/test--multica` 或其他远端仓库写入。
- GitHub Enterprise、带 URL 前缀的 GitLab 实例明确不支持当前只读 CLI 传输；普通 GitHub/GitLab 的实际安装版本、凭据及原生策略兼容性待真实验收。
- daemon/主机是信任边界；Multica 阶段授权不构成任意宿主 shell 的强制隔离。自定义执行包装器的私有认证环境必须与独立观察器可见环境一致。
- 保留原始失败、修正后重跑、源码变化作废的浏览器运行；不将其改写为成功。现有 lint 警告不等于新增阻塞错误。
- 任务继续保持 `in_progress`，不归档或把真实验收条件勾选为完成。
