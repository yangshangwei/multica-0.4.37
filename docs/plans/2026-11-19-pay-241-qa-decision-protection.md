# PAY-241 TC-13：有效 QA 决定后的 Agent 覆盖验证

继续现有父任务的跨阶段验收，不改变已归档 S2/S3 的产品合同。当前 TC-13 为 PARTIAL：已证明人类专属签字入口和 QA 退回后运行报告不改写决定，但缺少有效 QA 决定形成后的精确 Agent 覆盖/撤销请求。

## 最小边界

先只补测试。复用现有 stage review/report、testutil、实际 Auth/TaskCapabilityGuard、原 workflow operation 和 QA 决定；不添加权限、恢复机制或依赖。真实模型调用不需要。若发现产品缺陷，先记录失败并独立定位，再明确最小产品修改范围。

实现 owner 仅写新的 `server/internal/service/collaboration_stage_qa_protection_test.go` 与 `server/cmd/server/collaboration_stage_qa_protection_test.go`。如 router 必须复用已有包内 helper，优先在新测试文件组合现有实际入口，不跨包复制整套夹具。

## 有序验证

1. 在当前已提交源码的独立副本建立人类 QA 有效决定。服务层覆盖 reason-only return 和具备真实冻结报告/依赖的 approval；使用既有合法路径而非插入假决定。
2. 决定后，分别使用 machine principal 和带 AgentActor 的主体提交有效格式的相反决定及撤销请求，要求确切拒绝。原决定内容/签字人/时间/依赖、当前 head、stage 状态、事件/原回执、报告及运行记录保持；恢复原人类回执仍相同。
3. 真实 router 用实际签发的 `mat_` 任务凭据：先经支持的正向读取证明凭据和冻结作用域有效，再对原 QA 决定发精确覆盖/撤销请求，记录真正拒绝层和代码。`mcb_` 只支持 runtime 路由，不能用普通 Auth 对它的认证失败冒充 Agent 权限验证。JWT加Agent头只作为独立拒绝对照。已确认最小分层为 service 两顶层分别覆盖 return/approval，实际生产 router 一顶层覆盖 QA return 纠正后的机器请求；不重复搭建完整运行夹具，也不声称新跑了 router approval 场景。
4. 主会话为 service/router normal/race 分配独立新库、私有状态、源码/命令/工具绑定。测试需包含 cleanup 并核对原 gate、schema、关联记录和连接；不调用用户安装的模型 CLI，不复用旧失败库。
5. 独立检查核对新测试确实在有效 QA 决定后发起请求、正向认证控制有效、拒绝无副作用。仅在定义场景证据完整时将父 TC-13 更新 PASS；原历史和 TC-04 模型分析质量仍独立。
6. 定向提交并推送测试/规范/进度；后续转 TC-02 三会话验收标准修改与历史可回看，TC-01先明确场景字段映射，不擅自把通用可选字段改为必填。

当前验证计划不是通过结论；实际命名分母、原失败及最终判定记录到父任务 research。

## 已执行结果

2026-11-19：定义的TC-13独立接受，父矩阵更新8PASS/9PARTIAL/7NOT_RUN。service2顶层10命名、生产router1顶层7命名在普通/竞态各完整通过，共34执行/17不同命名；gofmt、vet、编译通过。四新专库的schema/gate/连接收尾已核对，原service夹具creation标志清理行为和保留失效历史如实记录。只新增两份测试并补规范，产品代码未改。下一步按剩余内部映射推进TC-02历史同步。
