# 共享技能内容变更的持久失效

## 范围与实现计划

704 已在 `agent_skill` 绑定变更时推进智能体配置 epoch，但更新共享 `skill` 正文、版本/来源或 `skill_file` 内容不会推进 epoch，因此旧授权仍可准入，后台也没有停组工作。本切片复用既有 agent epoch 与 `collaboration_config_invalidation` 消费者，不增加执行模型、兼容路径或新依赖。

1. 先补真实 SQL/HTTP writer 回归，覆盖同技能多个启用绑定、禁用绑定和无关目标；检查提交即拒绝旧准入、持久 hold/撤授权/stop、原 proof 身份与重启后收敛，并验证更晚的有效授权不被旧意图撤销。
2. 新增前向迁移 709，为技能正文、执行 bundle 中实际使用的 name/description、非 presentation 配置（含版本/来源）、文件增删改/路径移动及删除接线。仅启用绑定影响执行；文件时间戳和完全相同的写入不失效。`config.presentation` 为展示字段。不新增表、索引或外键。
3. 技能写入持 skill 行锁；文件写入及绑定变更用同一 skill 行的 NOWAIT 锁交汇，再按 UUID 顺序推进受影响 agent epoch，避免并发添加绑定在技能写入快照外漏掉失效。所有资源失效意图与内容同事务；冲突回滚而不半更新。
4. `UpdateSkill` 与 `overwriteSkillWithFiles` 目前全删再插文件，会把不变的导入误判成配置变更。提取仅供这两个事务 writer 使用的文件集合替换 helper：保留同路径同内容记录，只删除消失的文件并 upsert 变化。保持原 HTTP 文件列表语义；SQL trigger 仍覆盖单文件 API、导入/刷新、Plugin writer 和直接数据库变更。
5. 覆盖实际升级/重放及拒绝回退。已有协作授权的智能体在升级时推进一次，避免把升级前无法追踪的技能内容当作可信当前配置；升级重放不重复推进。既有运行历史与 lane pause 保留。

## 预期文件与验证

- `server/migrations/709_collaboration_skill_content_invalidation.{up,down}.sql`：内容、文件、绑定竞争和升级接线；不修改 704。
- `server/internal/handler/skill.go`、`skill_create.go` 与局部文件替换 helper：避免无变化文件重建；记录已知发布版本。
- `server/internal/service/collaboration_skill_invalidation_test.go`、对应 handler/migration 测试：实际 PostgreSQL 行为与 HTTP 接线；沿用已有 fixture 和消费者，不复制控制框架。

先记录回归失败，再在主会话提供的隔离数据库执行定向 Go 测试、race、vet 和 diff 检查。父会话负责全任务状态、sqlc、规范与提交。本切片不宣称完成完整 Switch、小队 M08/M11 或双端验收。
