# 业务影响范围与合并来源

本文件记录代码影响；最终验收结果、跳过项和限制见 verification.md。

| 官方提交 | 本地提交 | 影响范围 | 预期行为变化 | 覆盖重点 |
|---|---|---|---|---|
| [51803b3b3](https://github.com/multica-ai/multica/commit/51803b3b3) | `0a987e2c8` | 任务查询、daemon 心跳与任务记录读取 | 数据库暂时失败由 404 改为 500，避免误终止运行；真正不存在及跨工作区访问仍为 404。 | 真实数据库故障、恢复、后续关联成功、Quick Create 参数、用户消息读取和权限隔离。 |
| [6194a8b3a](https://github.com/multica-ai/multica/commit/6194a8b3a) | `181b56fcc` | Codex 环境复用 | 旧 home 准备失败时放弃复用并走新环境准备。该失败路径会失去旧会话／旧隔离目录的连续性；成功复用规则保留。 | 正常复用；损坏 home→新目录／home→thread/start；持续准备失败时不启动假 agent。 |
| [485278407](https://github.com/multica-ai/multica/commit/485278407) | `e6999e17a` | 工具输出上报预览 | 8 KiB 预览按 UTF-8 安全边界截断，清理非法编码和 NUL；agent 使用的完整输出不受预览限制。 | 空值、边界长度、2～4 字节字符、中文／emoji、非法编码与真实上报链。 |
| [9d3613653](https://github.com/multica-ai/multica/commit/9d3613653) | `42d38d013` | 任务失败分类和恢复提示 | 并发请求上限归为容量问题，不再误提示重新登录或缩短上下文；不会新增自动重试。 | 并发限制优先级、普通认证／上下文错误保留、旧 daemon reason 归一化与重试策略。 |
| [71ee15422](https://github.com/multica-ai/multica/commit/71ee15422) | `97275b3bf` | Web／Desktop 共用编辑器 | Tab、Shift-Tab、Enter 只作用于当前列表层级，避免误改外层 checkbox。 | 三类列表和混合嵌套、范围选择、撤销、Markdown 往返、文本／表格；浏览器保存后重新打开。 |
| [440ef8aa1](https://github.com/multica-ai/multica/commit/440ef8aa1) | `f34feb83b` | Agent 创建任务的前端预检查 | 看不到私人 runtime 不再等同缺版本。后端仍执行可见性、可用性及最低版本检查。 | 不可见 runtime；可见旧版；真实非管理员隐藏 runtime 的 202 接受与 422 拒绝且无任务写入。 |
| [b566b5fd4](https://github.com/multica-ai/multica/commit/b566b5fd4) | `0495e73f8` | Inbox 活跃／归档列表 | 关联 Issue 的 new_comment 通知 body 变为最多 200 个 Unicode 字符（含省略号）。存储、详情定位及已读／归档响应保留全文。 | 长度／Unicode 边界、无 Issue 和其他类型不截断、comment_id、数据库全文、浏览器从摘要跳转全文。 |
| [ca47495fc](https://github.com/multica-ai/multica/commit/ca47495fc) | `ab289ec84` | Codex 旧 compaction 错误诊断 | 补充配置排查信息，不改变错误分类、重试和会话恢复决定。 | 匹配旧路由 404；新路由／其他 provider 不误判；配置参数来源及分类保持。 |
| [a419cf20e](https://github.com/multica-ai/multica/commit/a419cf20e) | `1e64edc9c` | Inbox 响应式导航 | 双栏详情不重复生成侧栏按钮，紧凑布局保留返回列表。 | 组件 1024／1440 和 390／851 宽度；1100 浏览器唯一按钮可用；窄屏返回与侧栏可用。 |

## 全量回归发现的额外修复

`efb3c251c` 单独修复基线已有的 Git 快照遗漏。复制索引时刷新 mtime，可能把同尺寸快速修改误判为缓存有效，导致快照仍记录旧内容。新实现从同一文件描述符获取内容与 mtime，保留私有副本时间；种子索引不可用时检查清理并从 HEAD 重建。用户实际索引、refs、工作文件和原 Finalize 冲突断言均保留。

这是本次九项之外唯一额外的生产修复，范围仅 `server/internal/daemon/execenv/local_worktree.go`。它改变私有快照创建与错误回退，不涉及数据库、界面、权限或配置协议。确定性测试先在旧实现失败，修复后通过；原冲突保护又通过 20 次重复与整个 execenv race 套件。

`d143ffdd2` 只修正原有 TestGitEnv 的环境假设：空配置用例显式设为 0，已有配置用例继续验证键和值不丢失。没有改变生产 Git 配置。

## 保留的本地能力

中文 Agent／小队／skill／自动化模板、人工审批与自治等级、项目默认小队、内网消息集成开关、自部署认证、离线文档／更新日志、原目录锁和冲突保护继续沿用。九个上游补丁逐项稳定 patch-id 等价；没有混入上游线程队列、导航重组、数据库迁移或依赖升级。全量 E2E 包含这些本地功能的既有用例。

## 测试与交付边界

运行环境是本机 macOS。Go 默认套件使用防护脚本阻止调用已登录的真实 agent CLI；新增 Codex 流程使用测试创建的假 app-server。没有声称验证真实模型服务或 Windows/Linux 原生运行。浏览器完整套件使用 Chromium，并实际执行独立 Electron 窗口的更新日志测试。

Inbox 列表 body 的有意截断是外部脚本需要知道的响应语义；将列表 body 当完整评论的自定义客户端应按 issue_id/comment_id 读取完整内容。Codex home 准备失败时转新环境也是明确的恢复策略变化。通过测试意味着未在覆盖场景中发现回归，不是对所有外部环境的绝对保证。
