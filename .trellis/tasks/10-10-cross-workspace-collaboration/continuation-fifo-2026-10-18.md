# hold 之后的通道 FIFO、Switch 与后续接线

## 依据与修改边界

用户要求继续现有 `in_progress` 任务。起点为 `6356cd9cf`，工作区干净；`a389ef859` 是上一轮实现，之后两次提交只记录验证和续接。恢复现有任务绑定，不创建新任务，不重开已经确认的 M01–M11 产品选择。实现以当前 `research/execution-contract.md`、`research/data-contract.md`、PRD、设计和实施计划为准。

最小行为差距是 waiting 输入在入队时按目标预合批，顺序仍归目标所有，导致 held S 阻塞同通道 A，也使第二支小队之后的输入提前合入旧批。修复发生在通道序号、入队、promote 和相应持久约束，不能只在调度列表隐藏失效批次。

1. 通道分配单调 seq；promote 从逐条 waiting 输入中按当前资格选批，跳过 hold/无效/已并入输入，遇第二支合格小队才截断。保留作者、目标、内容、输入事件及版本身份；失效 hold 与拒绝结果按同一事务落盘。
2. 只对未开始输入重组或迁移通道。Switch 保留 typed target 和所有历史 group/run/proof，等待输入按相对顺序加入新通道尾部，写 switch_hold，旧通道控制状态不变。仅有可信 never-launched 证据的输入可按 E03 回等待。
3. 对外部 agent/squad 配置写入，复用已实现的失效和 stop/proof 逻辑形成持久收敛，不另建控制框架。完整冻结 target/config/grant 依赖是小队和 M11 的前置条件。
4. 在前置协议与验证完成后继续 M08/M11 与客户端消费；每一段分别留下证据，不能把存储、合成组或单智能体验证当作真实小队和完整双端验收。

预期触点为 `server/internal/service/collaboration_{batches,runtime,authority,groups,invalidation,execution_view}.go`、直接资源配置入口、对应 SQL/前向迁移/测试以及 `packages/core/collaboration/` 边界。通道/批次调整属于必要的局部结构修正，先补行为回归；优先删除入队预合批假设，复用当前事务、授权、资源预约与证明 helper，不增加依赖。创建与客户端执行能力仍按完整验收门禁控制。

## 执行与验证

- Trellis 实施代理分别拥有 FIFO/Switch 和资源配置失效代码；共享文件、SQL 和迁移先协调，主会话串行生成 sqlc。代理不得覆盖其他人的修改或递归派生。
- 主会话负责契约/任务资料、迁移编号与生成、隔离数据库和 Linux 环境、跨层集成、规范、提交及最终证据。
- 先运行真实 PostgreSQL 回归：held/invalid S 跳过、第二支合格小队截断、跨事务 FIFO、并发 promote、取消与迁移、旧 lane 历史、Switch 原键重放与证明屏障、never-launched 回队及失效配置收敛。
- 生产行为稳定后由独立 Trellis 检查代理复核，运行受影响 Go race/vet、core 边界测试、lint/typecheck、迁移升级/索引及 sqlc 一致性；改变运行链路时使用测试创建的可执行文件做实际 Linux 验证，不调用用户 AI CLI。
- 完整任务只有全部 P1–P8/97 项 AC 有对应证据后才完成。每段进度据实更新，不用上一轮日志替代新行为验证。

## 进度

- 已恢复现有任务并核对 hold 续接记录。
- 已实现通道统一序号、promote 时逐条筛选、第二支合格小队截断、held/失效输入跳过，以及暂存批次容量边界。
- 已实现不可变组输入记录及有完整 never-launched 证明时回队；配置切换的待授权阶段可安全迁移 waiting，历史与旧通道不变。
- 已接线真实配置写入的持久失效、独立后台收敛及 v4 服务端/core 读取；创建和客户端执行开关保持关闭。
- 独立复核修复 F01–F03：矛盾已结清记录、组结算后迟到 proof、读取中的不完整 nonlaunch 证明；均保留先失败后通过的回归。
- Linux 6 个路由用例及 4 个监督器用例已通过。公开 @S 入口仍明确拒绝 `squad_execution_unavailable`；held S 的 FIFO 用例使用真实授权后的内部输入，只有 A 经真实运行端执行。
- 扩大回归补齐了旧通道路由夹具、测试授权/任务清理、按自身资源等待的收敛夹具。内置说明压缩到 494 行，完整协作说明放在一级 reference。
- 最后一次固定源码 Go race 已在专用干净数据库通过；运行期间源文件无变化，结果见下表。


## 最终验证与余项

| 验证 | 结果 |
| --- | --- |
| Go 1.26.6 race | 1,191 顶层、1,211 子测试通过，0 失败，覆盖 service/handler/router/migrations/migrate |
| Linux ARM64 | 6 个路由用例、4 个监督器用例通过；12 子测试，0 跳过 |
| core | 265 文件、3,532 测试通过 |
| 静态检查 | typecheck 9/9、lint 6/6、5 个 Go 包 vet/gofmt 通过；保留 31 个既有 lint 警告 |
| 数据与生成 | 空库升级至 708、升级重放/保护校验及索引恢复通过；0 invalid index、0 孤立会话/授权；104 个 sqlc 文件再生成无差异 |
| 独立复核 | F01–F03 证明缺口已修复，F04 注释同步；本切片无未解决功能发现 |

Go 中的 4 个 Linux 跳过项已经实际补跑。唯一未运行的是默认关闭的
`TestFetchFromSkillsSh_AnthropicPptxIntegration` 联网用例，未把它计为通过。
新索引均使用独立并发迁移，无新增依赖。新执行身份模型拒绝回退到旧模型；
历史取消输入无法确定是否消费、或旧组缺唯一根运行时，升级明确拒绝而不猜测。

完整文件与哈希见[源文件记录](verification-source-manifest-fifo-2026-10-18.json)，
命令、日志和数量见[验证汇总](verification-summary-fifo-2026-10-18.json)，
独立结论见[复核报告](research/verification-fifo-review-2026-10-18.md)。

任务继续 `in_progress`。下一步完成冻结资源集合与授权就绪后的真正 Switch，
再推进 M08/M11 和容量 1 小队闭环；共享技能正文变更失效、附件/Git、通用供应商、
legacy enrollment、非 Linux 能力及完整 Web/Desktop/P1–P8 仍需独立验证。
本轮没有开放创建或客户端执行能力，未宣称全部 97 项 AC 已验收。

专用 PostgreSQL、Redis、两个 Linux 测试容器及网络已清理；日志和二进制哈希保留在验证汇总所列证据目录，原应用环境未改动。

本轮实现与规范提交：`33f584ae1`。源清单中的 85 个文件与该提交逐一核对一致。
