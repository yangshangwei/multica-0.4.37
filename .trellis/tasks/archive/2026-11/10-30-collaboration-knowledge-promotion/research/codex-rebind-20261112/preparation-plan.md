# 当前 Codex 绑定迁移准备（沿用知识子任务）

本增量将现有 0.167.0 原生二进制与本机当前已配置 Provider 作为迁移候选，保持原生 `multica-agent-sdk` / `Multica Agent SDK` 身份。旧 api666 账户/配置及两次403属于历史，不与新配置拼接。迁移先关闭真实入口。

行为缺口位于 tagged RK1K 登记与当前续作→retry02 身份相等检查；不得改 retry02→original 的不可变历史校验。预期最小写域为 `rk1k_codex_registration_test.go`、`rk1k_codex_continuation_test.go` 及新增迁移合同/控制测试。先证明缺失/错误迁移拒绝，验证精确正例、历史/版本/配置/源码/预算/输出漂移和停止规则。

在 detached 隔离工作树保留当前 product roots 的全部字节和未跟踪文件；不提交、不修改源工作区其他改动。迁移必须绑定历史注册/账本、新native/wrapper/config/source/test binary/协议与独审。旧full/retry模式不扩大。

顺序：离线schema比较和受限迁移→新专库fake普通/竞态与构件复核→独立审查→源码/配置/数据库绑定登记。网关须有允许真实Multica原生集成的有效依据，独占窗须有当前所有权；本机CLI可运行、doctor HTTP401、编排会话可对话都不证明网关准入。缺少依据时保持全部真实permit为false，不启动未登记探测。
