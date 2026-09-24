# 脱敏双闭环演练

## 研发交付闭环

同一源 issue 先提交 `bug-fix` RCA handoff，创建带 parent 的修复后续；随后以同一 approved/artifact digest 提交完成 observation window 和健康信号，最后提交 incident-learning，引用已有 prevention task。每次 handoff 均留下最新值、history 条目和审计评论；digest 不一致、窗口未完成、事实/推断/unknown 缺失时保持 `unknown`，不执行回滚。

## Agent 质量闭环

以脱敏的 Agent/Skill/MCP 版本和六类 case manifest 提交 `agent-evaluation`，安全 case 必须阻断越权请求，成本/延迟/工具漂移进入 hold 或新增评测建议。发布后仍只记录同一 artifact 的观察证据，不修改模型、工具绑定或生产配置。

## 治理门禁

同一 handoff API 对五类 capability 使用必需信号：兼容矩阵/插件边界/fallback、信任边界/滥用路径/缓解、锁文件/来源/SBOM/许可证、结果 baseline/窗口/观测/owner、备份/恢复演练/RPO-RTO/降级路径。全部 pass 才可 pass；缺失为 unknown，明确失败为 hold。

真实模型、生产 observability、发布、回滚、迁移、恢复和外部通知均未执行；这些边界由 skill、autonomy 和人工审批规则继续保护。
