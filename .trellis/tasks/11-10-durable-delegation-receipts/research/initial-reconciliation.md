# 初始状态核对
当前业务分支HEAD仍38d07b6ed，无后续本地提交；不回退、不fetch远程。原工作区372项未提交变更，路径/内容hash存于专属外部证据目录。新worktree从当前HEAD创建，上一观察worktree保持完成状态。

读取父任务prd/design/implement/progress/task、总推进文档、观察归档delivery/prd/design/implement及verification/独审JSON/MD、即时回执归档、三个spec。观察证据140 handler/34 router每轮含子测试、6文件冻结、原失败保留；未新增模型验收。即时回执仅create/edit响应/CLI警告与leader消费，没有持久化/幂等。

源码定位：comment.go CreateComment先保存后派发；enqueueSingleCommentTrigger丢弃已有task返回；merge/register SQL已RETURNING ID；AlreadyPending布尔不能成为真实任务证明。TaskService含mutex与外部overlay/广播/唤醒，不复制/不套全程长事务。选择短claim+comment事务与终态unknown策略。

Auth：loadIssueForUser依赖router WorkspaceAuth成员校验；durable路径原actor+originator绑定，source task留存但不作为同agent重启恢复键；不同originator不可读原私人回执。无权目标仅保存请求中的UUID及通用拒绝原因。

暂时离线runtime旧行为是queued等待；未绑定/不可用拒绝。已向用户提出具体语义选择，其余实现独立推进。在无新选择前保留既有行为，不将离线入队冒充离线拒绝覆盖。

本批专属PG容器multica-durable-receipts-20261110-pg，127.0.0.1:61771，镜像pgvector/pgvector:pg17在ARM64宿主上运行amd64镜像；只用于功能/race，不作性能声明。数据库每包独立。真实模型调用0，全部历史停止规则保持。
