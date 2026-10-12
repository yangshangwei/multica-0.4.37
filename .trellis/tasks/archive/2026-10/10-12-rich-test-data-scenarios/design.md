# 技术设计

## 边界

新增 `scripts/seed-rich-test-data.mjs`，只连接现有 PostgreSQL 并在一个事务内写入 fixture 数据。脚本不调用产品 API，不启动 agent，不修改 schema；这适合当前已经运行的后端和桌面端，也避免依赖当前登录会话。

## 数据模型

- 通过 workspace slug 解析目标工作区，并选择该工作区 owner 作为 fixture creator/recipient。
- 用固定 `fixture_key` 写入可支持的 `metadata`、`description` 或 `details` 字段；实体标题统一使用 `桌面场景` 前缀。实体 ID 按 fixture 版本、workspace UUID 和场景 key 确定生成；清理按确定 ID 与 marker 校验反向删除，标题前缀不授予删除权。
- 先插入多个项目生命周期样例、计划迭代、标签和一个可见模拟智能体，再插入 35–50 条任务，覆盖状态、优先级、层级、长文本、期限、成员/未指派/智能体、项目和计划迭代组合。
- 现有进行中迭代不参与本 fixture 写入；历史覆盖复用已有冻结历史。只有能生成符合规范的完整 snapshot/events 时才新增完成迭代，不伪造历史。
- 评论、订阅、收件箱、聊天数据与任务/用户/智能体绑定；执行队列覆盖 `completed`、`failed`、`cancelled`、`queued`，绑定专属离线模拟运行时。所有执行耗尽自动重试预算，普通产品 sweeper 仍可在重连宽限期后更新排队样例。
- 重跑保留已有数据的编辑、已读状态和删除；首轮写入原子完成，因此部分缺失视为用户删除，不在重跑中恢复。

## 安全与回滚

脚本要求显式本地 `DATABASE_URL`，解析 workspace 后在事务中执行；`--dry-run` 使用真正只读事务，不占用任务编号。`--reset` 显式清理；先校验确定 ID 与 marker，以及所有外部软引用，再按依赖顺序删除。清理身份集合不受当前分拣/迭代启停影响。任何 SQL 错误都回滚，包含已预留编号和已插入父记录。缺少 workspace/owner、marker 被移除或模拟运行时上线时拒绝操作。

## 验证

脚本结束输出各表 fixture 数量和关键 slug/ID。重复运行后数量必须不变；reset 后原有手测任务仍存在。桌面端刷新当前工作区后检查 Issues、Projects、Iterations、Inbox、Chat、Agents 页面。
