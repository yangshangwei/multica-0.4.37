# 真实验收准备的源码索引（2026-11-20）

本记录用于 context 注入，避免把超过 64 KiB 的子任务一 design 整份截断。子任务一最终规则以它的正文和父任务最新决定为准；按需读取其 design 第 4、5、8.5、9.4、11、14 节，不用本摘要替代。

| 源 | 本任务使用的事实 |
|---|---|
| `server/internal/handler/squad_security_remediation_flow_test.go` | 四个现有脚本场景的真实平台路由、作者、来源任务、回执、活动、状态；脚本不证明模型行为 |
| `server/internal/handler/squad_security_remediation_routing_test.go` | reviewer failure system notice、受限评论不触发、精确小队提及及作者/revision 平台事实 |
| `server/internal/handler/squad_briefing.go` | 平台协议、花名册、小队正文注入；原文不投影/重写 |
| `server/internal/daemon/rk1k_driver_agentintegration.go` | agentintegration 下已有普通 daemon Claim/Run/Report 驱动；生产代码路径可复用，knowledge 专用夹具不是本场景 |
| `server/pkg/agent/codex.go` | 原生 app-server、session/runtime input 与 usage 读取；本任务不用 exec 桥冒充此路径 |
| `server/internal/daemon/execenv/codex_home.go` | shared config 复制、auth 链接和任务会话隔离机制；不同目录不构成 OS 边界 |
| `server/internal/daemon/execenv/codex_sandbox.go` | Linux 当前默认 danger-full-access，隔离依赖外部容器/用户边界 |
| `server/cmd/server/rk1k_codex_registration_test.go` | 精确 source/config/executable 绑定、预留与物理启动账本；不继承该任务额度 |
| `.trellis/spec/server/task-api-capabilities.md` | 真实验收必须冻结预期、看平台完整业务字段和实际身份、保留未知与不完整 |
| `.trellis/spec/server/collaboration-context-experiments.md` | 原始供应商证据私有、预算预留、失败不重置、假运行与真运行分开 |
| 历史 task `10-26-security-remediation-squad/pilot/runtime/` | 旧 wrapper 投影指令且环境隔离不足，不能直接恢复为 v2 验收 |

子任务一试点前修订应覆盖：更新分诊使用新评论；负责人匹配是角色说明子串；需要完整运行时绑定和每名智能体调用权限；G1/G2/临时审查点先等本次 required reviewers 再放行；每 reviewer 每 gate 最多重派一次；revision>1 拒绝，负责人重发新评论，直到 leader 明确核验通过前不做会改变 revision 的操作；测试工程师/诊断工程师停止边界保持；停止后的恢复不能绕过 incident、真实凭据或成员停止条件。

具体未决模型参数只从父任务当前用户决定确认：候选配置是 `api666-zzzcoding/gpt-6-astra`/`magpie`/`max`，尚不代表确定账号或预算。真实调用已授权，不重复问许可。
