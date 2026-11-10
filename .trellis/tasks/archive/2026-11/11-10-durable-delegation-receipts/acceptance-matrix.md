# 验收矩阵（待实际测试回填）

| 合同 | 实际证据要求 |
|---|---|
| 重复/并发 | 同键同payload一条评论、一组原始结果；不同payload409且无动作；并发唯一定提交者 |
| queued | task ID等于真实入队返回，agent/issue/trigger_comment一一对应 |
| coalesced | 原queued task原子更新，ID不变、coalesced_comment_ids包含实际评论 |
| deferred | 明确持久登记在实际任务上，不靠active布尔猜测；完成/后继执行不等于初始受理完成 |
| blocked | private/unbound/unusable/current revoked，目标UUID/通用reason留存，无泄露和新task |
| offline | 用户已确认暂时离线queued等待；实际离线观察持久化，切换在线后读取仍保留原状态；未绑定/不可用拒绝 |
| 成员替换 | 原target+executing agent保持，replay不依据新roster重新派发 |
| 重启恢复 | 新handler/router与同agent新合法task取回原回执；source_task保留旧身份 |
| 权限 | 当前membership、agent/originator、task capability；他人key、跨issue、跨workspace、伪造header拒绝 |
| unknown | claim已存、派发/收尾不确定或失败留unknown；POST/GET无再次派发 |
| 兼容 | 无键create/edit旧行为，JSON增量，CLI成功写入不因blocked改失败退出，无自动重试 |
| 输入 | canonical UUID key、payload绑定、附件明确拒绝且零写入 |
| 迁移 | noFK；每索引独立CONCURRENTLY；schema/sqlc同步；非空rollback留证 |

普通/race按各轮真实具名terminal（含子测试）分别记录；不得跨轮累加覆盖。所有失败原样保留。静态/build/vet不替代实际任务与认证验证。不调用真实模型，不改旧实验判据。
