# 新 runtime 的真实验收登记要求

状态：未运行。本页是执行准备清单，不是通过证据或已分配的付费实验槽位。原系列的下一入口仍为 E2a trial 3、剩余 22 次；本页不能消费或重新命名这些槽位。

## 版本与入口

- 产品基线为 `0e1ee8e1e`，确定性与生产 Web/原生 macOS Electron 证据为 `2b861347e`。补充测试提交单独登记；任何生产代码变化都需重新记录产品指纹及受影响验证。
- 新运行使用独立工作区、输出目录、run/session identity 和登记文件。保存 HEAD、完整 dirty/overlay manifest、框架 SHA-256、CLI binary/version、请求模型和实际观察模型、thinking/tier、平台/安装方式及来源账户的非敏感标识。
- 正式账号 smoke 入口为 `TestHostProviderAccountSmoke`（`server/internal/daemon/collaborationrun/host_agentintegration_test.go`）。它证明当前 adapter/账户的受管理执行；其 HTTP 服务为测试所有，不能代替真实 service 授权、checkpoint 摄取或原生故障矩阵。
- 新路径实验必须证明协商了 handoff/recording 能力、实际调用相应读取/写入/绑定接口，并保留原始请求、响应、实际 prompt/stdin、attempt/nonce、Started/termination/proof。只是在新 HEAD 运行旧 fixture，不足以声称覆盖新 runtime。

## 执行顺序与判定

1. 在所有编码/审查停止后的独立窗口，先确认原上游阻塞已有恢复依据。当前最新事实是 E2a trial 2 返回 502，不能视作恢复；本轮不追加健康调用。
2. 执行前写明本系列自己的最大调用数、模型、固定场景、轮次、顺序与停止规则，并用 fake executable 验证框架确实覆盖新路径。当前不另增预算。
3. 新版本正式 E0 验证 CLI context-read、实际 launch/outcome/termination/proof；保留自己的编号与目录，不覆盖旧正式 E0。
4. 交接行为验证须覆盖 checkpoint 已保存前缀和缺失提示、前序/成员结果及读者/唤醒来源、最新/最旧评论召回；正确答案必须来自所记录的实际输入和工具结果。
5. E5 的非指令评论与两个自控账号、E6 的隔离接力按原语义进行版本化复验。运行前冻结新框架映射；不得声称原 E5/E6 已运行或用新记录填旧槽位。
6. 首个 runtime/environment/harness 不完整结果后停止所有后续轮次/场景；保存原 verdict，单独做语义判定。无替换样本、额外探测、模型切换、超时扩展；E5 canary 和 E4/E6 语义审查保留原要求。

每项结果必须分别报告有效样本、无效尝试、已消耗槽位、未运行项、实际生效模型是否观察到。seeded UI、fake Provider、账户 smoke、service 权限测试与模型理解各有范围，不互相替代。Windows、安装包、多账户额度溯源仍见父任务当前验收矩阵。
