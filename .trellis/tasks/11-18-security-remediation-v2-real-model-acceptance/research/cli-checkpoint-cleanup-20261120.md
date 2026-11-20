# CLI直连阶段检查点与清理

真实官方CLI共7次启动、70条模型POST，P1/O1已满足冻结要求；C1合法可选分析分支和装置留存缺陷导致未完成，D1未真实执行。内容筛查中断的续接实现未改提示词重试、未换代理重做；不以不限预算授权绕过该限制。

9个任务数据库全部先保存权限0600的pg_dump并记录sha256。C1待执行product-analyst任务另保存结构化task/role/status快照；未将队长失败改为完成，也未清除分析师后伪称已执行。随后在0连接情况下按精确清单删除任务库、独占PostgreSQL/relay及internal网络，认证副本删除。恢复需恢复私有dump到新库并重新核验配置，不靠重置历史。

私有证据根 `/Volumes/artisan/code/2026/secrem-v2-cli-evidence-20261120/` 权限0700。provider原始请求/响应、会话/RPC、原生JSONL、private注册、历史src/binary与数据库dump不进仓库；仓库只放脱敏摘要/hash/控制日志。原SDK失败证据仍在上一轮私有目录，单独计数。

最终任务内Python控制50/50通过；直接backend11/11通过；真实P1/O1与脚本C1/D1的具体检查点见逐案记录。没有修改生产源码或模板正文，不应声称当前app-server后端通过验收。

历史首个CLI失败样本的未跟踪directbackend原始源码字节未及时保留，只有hash与旧binary/输入/事件；该限制明确记录，不事后重建冒充原文。后续P1/O1/C1准入源均保存实际字节副本。

最终检查点已实际合入15e9e8d52；175个父子任务文件逐字节一致、任务worktree干净后移除mc-secrem-v2-cli-20261120，保留codex/security-remediation-v2-cli-20261120分支及私有final-task-checkpoint源副本。合入过程中另一个会话更新了一个artifacts/qa文件，本任务未触碰或恢复它，回执明确此并发限制。所有实际模型运行来源仍是624eb71b9及当时绑定装置，不以最新基线冒充运行来源。
