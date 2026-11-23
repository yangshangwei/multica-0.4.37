# Codex 0.166 真实验收：首轮遇网关限制后停止

## 结论

本轮确实启动了用户授权的本机 Codex。首项 E0b 未获得有效模型结果：网关返回
HTTP 403，内容为 `This account only allows Codex official clients`。测试按预定规则
在首个不完整结果停止，没有进入 K1、K2 或 K3。

累计实际物理启动 **1/15**，剩余 **14** 未使用；执行许可已关闭，台账为 stopped。
这不是知识模型行为通过，也不构成整任务完成。进程退出码 0 不能替代成功的模型终态。
每次 Agent 物理启动可能产生多次底层请求；本轮日志中存在多个拒绝事件，不能把
1 次物理启动解释为 1 次 HTTP 请求或已知计费金额。

## 本轮准备与源码

本机 Codex 在等待窗口期间从 0.165.0 更新为 0.166.0，原程序哈希失效且登记过期。
四个 tagged harness 文件修正了精确版本准入与 wrapper 版本报告，并新增真实模式
缺失版本绑定时拒绝的回归。对应提交 `079d6cc28`，本地集成为 `c1eb952fa`。
没有修改模型、账户服务或产品行为，也没有新增依赖。

本机安装版协议的线程/恢复/非持久化和轮次接口保持兼容；新增字段和错误枚举单独
记录。新普通与竞态 fake 套件各 **53 个具名结果通过**，0 失败/跳过/非法 JSON，
各 18 个样本、15 measured +3 fake seed +0 paid、54 个构件哈希通过独立复核。
编译绑定覆盖 4501 个服务端源码、依赖和嵌入资源。源码在执行前后保持一致。

实际使用 `Codex 0.166.0`、用户配置的 `gpt-6-astra`，独立测试配置保留原自定义
Responses 服务账户设置。原始配置与密钥未修改，密钥未进入此报告或版本库。
程序选用同一安装包内的原生二进制；未伪装为官方 CLI 客户端身份。

## 实际执行与停止

| 事件 | 实际记录（UTC） |
| --- | --- |
| 执行器启动 | 2026-11-05 05:40:18.687489 |
| 首个物理进程启动 | 2026-11-05 05:40:21.450253 |
| 测试结束 | 2026-11-05 05:40:44.771846，exit 1 |
| 样本结果 | E0b incomplete / unclassified；无成功模型结果 |
| 后继场景 | K1/K2/K3 均未启动，完整汇总未生成 |
| 台账 | 1 个预留、1 个实际启动；process_complete=false、complete=false、stopped=true |
| 释放 | 自有测试/Provider 进程 0，专库其他连接 0；已向 core/runtime/UX 明确释放 |

包装器已保存输入/输出/退出证据，`evidence_state=complete` 只表示该层捕获完整，
`result_seen=false` 和台账的不完整状态仍保留。成功封存日志不代表执行场景通过。
PostgreSQL 恢复后为本轮重新分配并迁移专库，未复用旧失败库。原普通测试失败、
编译漂移拒绝以及本次真实失败均保留。

## 外部阻塞与下一步

当前网关拒绝 Multica 的 Codex app-server 集成。两次实际尝试均收到相同限制，
目前没有新的放行证据。需要网关运营者允许真实客户端
`multica-agent-sdk` / `Multica Agent SDK`，或由用户选择允许此集成的账户/端点。
只要求调整合法访问策略，不更改客户端标识、originator/User-Agent 或冒充官方客户端。
单独 `codex exec` 可以运行，也不能证明本生产 app-server 链路被允许。

外部限制解除后，保留已消耗的两项及停止记录，使用新编号、专库、当前程序/配置
绑定和独立窗口登记剩余 13 次内的后续安排；不能直接重置原台账或自动续跑。
如果恢复后的完整计划需要额外调用，必须先调整明确的调用上限，不能挪用 runtime25。

## 证据边界

执行前审查报告在签署后被审查者追加了一处窗口描述，导致当前报告哈希与原签名不同。
这属于取证记录维护错误，不能把当前文件宣称为签署时的原件。独立复核已将重建的签署
原文另存，其哈希与签名一致；原签名、修改后报告和实际失败记录均不覆盖。
源码/程序/配置绑定和实际一次启动、403 拒绝不因这个文档变动而改判成功。

共享主分支后来还有其他任务修改的 `rk1_entry_test.go` 和 `rk1_fake_test.go`。
本轮结果绑定知识隔离工作区的已审源码，不宣称已测试整个后续共享分支或新 UX。

- [0.166 最终准入](research/codex-real-20261104/version0166-admission-final-review.md)
- [模拟验收结果](research/codex-real-20261104/version0166-fake-results.json)
- [原始真实结果指针](research/codex-real-20261104/real-v166-01-result-pointer.json)
- [资源释放](research/codex-real-20261104/version0166-window-release.json)
- [执行许可关闭](research/codex-real-20261104/version0166-permit-closed.json)
- [本地集成范围](research/codex-real-20261104/version0166-main-integration.json)

- [真实结果独立复核](research/codex-real-20261104/version0166-real-result-review.md)
- [原签署报告重建快照](research/codex-real-20261104/version0166-admission-signed-snapshot.md)
