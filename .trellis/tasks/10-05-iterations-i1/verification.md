# I1 实施进度与阻断记录

> 最新状态：FG 已通过，当前证据以 [FG 验收记录](../10-05-iterations-i1-foundation/fg-verification.md) 为准。发布保持关闭；历史记录中的未通过/网络限制不是当前阻断。下一步 LG/HG；完整 I1 仍待后续门槛。

> 2026-10-06 本轮更新：下方离线记录是历史材料，旧网络/Git 限制已解除。当前实际实施与检查见 foundation 的 `s2-batch-writers-verification.md`、`s2-execution-start-verification.md`、`s2-creation-writers-verification.md`、`s2-delete-writers-verification.md`、`s2-boundaries-verification.md`。W13/W14 与 W16 分别已提交 `ef0b989e9`、`39c3200ae`。其余 S2 接入及边界正在整合，FG 仍未通过；继续位置以父任务 handoff §10 为准。

最新实施见 [foundation S0/S1 验证记录](../10-05-iterations-i1-foundation/s0-s1-verification.md)。当前推进真实 W01 事务内 recorder；以下离线记录保留为历史证据，不能替代本轮验收。

W01 已在 `837e1abaf` 验证提交；继续实施的 W02/W15 和剩余 FG 证据见 [S2 验证记录](../10-05-iterations-i1-foundation/s2-verification.md)。

> 历史记录：以下为转移前的离线实施证据，保留原迁移编号和当时环境限制。当前分支、550–566 迁移及联合验证见[分支整合记录](branch-integration.md)。

日期：2026-10-06。用户已授权实施，父任务与foundation为in_progress。**I1未完成，FG未通过**；本记录只证明基础层离线准备。未提交、推送、迁移数据库、发布或启用迭代。

## 已完成的工作副本

- 536–552共17组迁移：默认关闭的I1表、共享规划时区、Issue归属和累计结转字段；16个独立CONCURRENTLY索引及清理登记，无新FK/内联唯一索引。
- `server/internal/iteration/`：类型合同、固定承诺/范围统计、日末图表、日历时区、严格草稿规范化、身份/意图哈希、事务重试、锁后时钟和持久操作读写基础。尚未接入现有业务路径。
- `server/pkg/db/queries/iteration.sql`及统一sqlc生成；Issue/Workspace显式投影补新字段，继续复用现有类型。
- [writer inventory](research/writer-inventory.md)：W01–W17覆盖HTTP、Plugin、GitHub、失败重置、T1、创建、项目/任务删除、Squad转移、workspace删除和真实执行开始；已修订KEY SHARE与T1锁序说明，尚未实施writer收口。

## 实际检查

所有命令使用已有本地工具/依赖、GOCACHE=/private/tmp/multica-i1-go-cache、GOPROXY=off；没有安装新依赖。

| 检查 | 结果与限制 |
| --- | --- |
| go test -race ./internal/iteration -count=1 -json | 69项pass（含子用例），0失败；`/tmp/i1-final-unit.jsonl` |
| go build -p 2 ./... | 退出0；`/tmp/i1-final-build.log` |
| go vet -p 2 ./... | 退出0；`/tmp/i1-final-vet.log` |
| 迁移静态与索引清理登记测试 | 3项通过；`/tmp/i1-final-schema.log`；不是执行真实迁移 |
| 完整handler测试编译 | go test -c通过；`/tmp/i1-handler-compile.log`；没有运行数据库测试 |
| sqlc v1.31.1生成 | 离线构建工具，两次生成SHA256无漂移，见foundation记录 |
| 最终canonical模糊测试 | 13,703次执行通过；`/tmp/i1-final-fuzz.log` |
| 独立只读代码审查 | 精度漏洞修复后无剩余实际代码阻断；不替代DB验收 |

最终审查发现并修复：UUID同值身份字段被map合并；ListWorkspaces投影缺列导致生成行类型漂移；原始JSON小数revision经浮点舍入被当整数接受。最后一项使用16个原始JSON组合先红后绿，四类修订号都在Float64之前检查，保留普通统计比例的浮点支持。基础DTO直接嵌入纯统计结构，避免重复字段及两套口径。

## 不能继续通过FG的原因

会话的managed sandbox禁止本机TCP；连接127.0.0.1:5432和6379均返回PermissionError [Errno 1] Operation not permitted。`.git`亦只读。未请求或绕过提权，未通过其他协议访问被拒绝服务。

真实数据库的迁移、单active竞争、旧字段保留、回退、幂等、撤权、响应丢失、锁序与吞吐未验证。DB fixtures已编写且可编译，skip不计为通过。生命周期、事件采集接线、结束/结转、outbox、Web/Desktop和29项端到端验收尚未完成；不得将离线helper通过称为I1可用。

## 恢复实施

在允许本机PostgreSQL/Redis连接的执行环境继续，不需要重新确认产品范围：

1. 按[foundation验证记录](../10-05-iterations-i1-foundation/verification.md)运行独立fixture数据库迁移/回退/两连接测试，补持久操作真实DB场景。
2. 按writer inventory逐条统一前置锁与同事务事件/版本更新，先回归T1及普通任务，再测普通写吞吐。满足FG后才启动生命周期和历史业务。
3. 依次完成LG/HG、closure CG、客户端UG、完整VG；按已实现的 P1 重新核对并补跑联合边界。
4. 有Git写权限后在独立开发分支提交并执行远端CI；本轮没有改动任何Git元数据。
