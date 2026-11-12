# 可靠上下文交接与召回：本轮进度（2026-11-01）

任务实际已处于 **in_progress / 实施与集成验证中**；用户表格中的“规划中”落后于任务记录。本轮恢复现有任务，未新建任务。完整产品验收尚未完成，任务未归档。

## 本轮完成

1. 接入统一 host 恢复流程：记录上传持续不可用或挂起时，原有 Stop、启动终止事实和原终止证明仍可完成；失败的记录与封口继续保存在 ledger 中。
2. 记录重放有两秒总预算，与 outcome、seal 共用有界上报窗口，并为原证明保留调用方剩余时间；已持久化 proof 优先确认。
3. 新 Runner 重新打开磁盘 ledger 后，只恢复原身份、原 attempt/nonce、原 token、原 payload/key。测试禁止重新 Claim、Permit、读取新交接输入或启动 Provider，已确认 proof 不重发。
4. 丢失封口 ACK 时先重放完全相同的旧请求；其确认后，待全部片段得到明确结果，再以相同原因递增 sequence 更新已知水位，避免耗尽封口预算。
5. 封口响应缺字段、retained 为 null、负数/超限/矛盾水位、completed 没有已知观察范围均拒收；有效零值、显式未知尾部、已知缺失尾部和较晚的原请求 ACK 保持可读。畸形 ACK 不清除待恢复封口或改变已确认 proof。
6. 修复执行面板两条旧测试的响应队列串用：独立 handoff 查询由其规范测试负责，记录分页测试继续验证懒加载、重试和撤权清除。
7. 本轮 7 个源码/测试文件与 3 个 spec 文件已按保存的旧文件哈希合回当前工作区；全部字节与通过检查的隔离快照一致，保存的 81 文件源码快照中，本轮范围外的先前改动均未变化。

## 验证证据

| 检查 | 本轮结果 |
| --- | --- |
| 全量 pnpm test | 12,949 tests passed，5/5 tasks successful |
| Go protocol / CLI / cmd/multica / daemon | 退出码 0；1,640 个通过事件（含子测试）；1 项既有 CLI-config 主动跳过 |
| 恢复 / checkpoint / ACK / 版本边界 race | 38 个通过事件，0 跳过 |
| 专用数据库 service context | 8 个通过事件，0 跳过 |
| 专用数据库 handler 权限/版本/封口 | 11 个通过事件，0 跳过 |
| 执行面板与 handoff 组件 | 11 tests passed |
| 最终 lint / typecheck / Go vet / Go build | 全部退出码 0 |
| 合回主工作区后 protocol / daemon 复跑 | 316 个通过事件，0 跳过；源码哈希保持一致 |

命令、原始日志、实际测试名、源文件哈希、运行目录、退出码和跳过项见同目录的 JSON/log；最终汇总为 `summary.json`。测试数据库为本轮专用数据库，不由 API 提供服务；凭据只留在私有 `/tmp` env 文件。原有默认数据库 bootstrap 的一次 handler 探索检查未计入专用数据库证据。

Go 跳过项为 `TestCLIConfig_UnknownFieldsArePreserved`：原代码主动记录 encoding/json 不保留未知配置字段的既有限制。本轮不计其通过，也未修改该测试。

Stop 用例运行了真实测试二进制子进程作为 fake Provider，验证在上传 fixture 释放前完成终止与 proof ACK；重启用例为新的 Runner/Client 打开真实磁盘 ledger。它们不代表真实模型理解，也不是杀死整个 daemon 后的安装版验收。

## 仍需收口

- 独立代理终审因 token-rate-limit / 模型路由不可用未完成。已保存初次权限边界复核、失败记录与主会话手工审阅，不能将独立终审计为通过。
- 任务全量 Stop/撤权/退出锁序矩阵、启动注入整合、生产 Web 与原生 Desktop 最终验收。
- 当前源版本的真实模型验收；既有 E 系列样本、失败记录和后续槽位保持原样，不以本轮 fake Provider 结果替代。

## 本轮改动文件

- `server/internal/daemon/collaborationrun/host_runner.go`
- `server/internal/daemon/collaborationrun/host_checkpoints.go`
- `server/internal/daemon/collaborationrun/host_checkpoint_ledger.go`
- `server/internal/daemon/collaborationrun/client_host.go`
- `server/internal/daemon/collaborationrun/host_recording_recovery_test.go`
- `server/internal/daemon/collaborationrun/client_recording_ack_test.go`
- `packages/views/collaboration/execution-panel.test.tsx`
- `.trellis/spec/server/collaboration-runtime-recovery.md`
- `.trellis/spec/server/index.md`
- `.trellis/spec/core/frontend/collaboration-workbench.md`

简化方式：复用原 host lifecycle/proof 和现有协议水位校验；收敛上报的总时间预算，不增加依赖、公共协议或新的执行路径。完整任务与其先前 WIP 仍在进行中，代码保持未提交，等待任务整体验收与来源收口。
