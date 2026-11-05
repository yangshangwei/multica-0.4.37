继续推进 Multica 智能体与 AI小队研发体系优化，按 M1 → M2 → M3 的顺序交付。请实际推进开发与验证，不停留在建议清单。

工作目录：/Volumes/artisan/code/2026/multica-0.4.37
集成目标分支：codex/cross-workspace-collaboration
现有 Trellis 父任务：.trellis/tasks/11-05-agent-squad-rnd-improvements（in_progress）

先读取 AGENTS.md、CLAUDE.md、trellis-start/trellis-continue 对应流程，以及以下文件：
1. docs/plans/2026-11-05-agent-squad-rnd-improvements.md
2. .trellis/tasks/11-05-agent-squad-rnd-improvements/prd.md
3. .trellis/tasks/11-05-agent-squad-rnd-improvements/design.md
4. .trellis/tasks/11-05-agent-squad-rnd-improvements/implement.md
5. .trellis/tasks/11-05-agent-squad-rnd-improvements/progress.md 和 delivery.md
按涉及模块加载 .trellis/spec/；需要原始审计依据时再读 docs/audits/2026-11-05-agent-squad-rnd-audit.zh-CN.md。

已完成的 M0：
- 1cd9c1e60：结构化凭据、JSON、原生令牌脱敏，以及完整工具输出脱敏后再截断。
- 5975edae9：Release 数据库/Redis/迁移准备，完整测试入口拒绝空配置，三个 TestMain 对错误配置失败关闭。
- 028e93bb0：区分子任务交付 in_review 与人工/既有集成验收 done，补齐门禁队长六类审查并升 v3，同步 skill、双语与内嵌文档。
- M0 源分支已在 48aa99745 快进合入目标分支。先核对这些提交为当前 HEAD 祖先，使用最新 HEAD 续作，不切回旧提交。
- 三个 M0 子任务已归档到 .trellis/tasks/archive/2026-11/11-05-agent-log-redaction、11-05-agent-ci-test-integrity、11-05-squad-handoff-contract；各目录有 verification.md、independent-review.md 和原始 red/green 证据。

后续目标与顺序：
M1（优先）：任务/资源/动作 capability；受控高风险执行与一次性审批回执；绑定当前 commit/artifact 的项目交付证据门禁；leader 消费真实 trigger_outcomes；关键用户路径 PR smoke。先从报告员/评审员的最小资源动作权限矩阵与强制 API 检查入手，同时研究可独立推进的委派回执和CI路径，避免一次改造全部运行时。
M2：确定性日报周报快照（范围、时区、[start,end)、水位、完整性、来源清单）；跨子任务/重试的Token、时间、估算成本预算；基于能力、排队和预算的调度。
M3：固定环境下的容量/恢复/取消等非功能验证；expected/observed/verdict 分离的模型行为评测；供应链与制品证据；保留定制的模板升级、灰度回滚与知识失效治理。

推进方式：
- 沿用父任务；我授权在该父任务下创建所需 M1–M3 子任务，并按既有路线推进本地设计、实现、测试和原子提交。先把当前子任务的需求、设计、实现计划、验收和上下文清单做实，经过独立评审后实施；普通可逆步骤自主完成。
- 先检查 git status、最近提交、现有任务与源码，确认哪些能力已有或被其他会话推进。工作区有并行验收的未提交规范/日志/任务，不覆盖、不混入本任务提交。需要隔离时从最新集成分支创建自己的 worktree。
- 使用原生 trellis-implement / trellis-check 子代理，明确文件所有权；主会话负责跨层设计、集成、规范和交付。独立子任务并行，有依赖的变更顺序执行。
- 不重复建设已有身份/授权、lifecycle校验、MCP broker、指标和版本机制。新权限是发起人当前权限、工作空间授权与任务范围的交集，不能靠提示词冒充强制限制。
- 保留人工验收done与高风险审批边界；自动阶段推进、扩大权限或改变兼容/统计语义属于需要明确决策的事项，先提供具体方案和影响。普通实现细节自行决定。
- 每个子任务先建立有效回归，再最小实现、独立检查、更新spec、记录实际测试结果与未测项，按Lore协议原子提交。M1未通过其验收前，不把依赖它的M2/M3标为可交付；父任务保持未完成直到全部里程碑验收。

验证和权限限制：
- M0已有定向验证：脱敏 77 项及 race 77、daemon 37、provider 16、execenv 22、service 61、工作流 5、数据库前置条件 13，均有独立审查；这些不是完整系统或上线验收。仅因新改动、失败或集成风险复跑相关检查。
- 健康数据库fixture、seed故障注入、托管Go1.26 CI/Release、浏览器整体验收和真实模型行为尚未由M0证明。按当前子任务需要补齐，不能把未执行写成通过。
- 数据库测试使用自己分配的隔离数据库，并核实归属；go test -run仍会执行TestMain，编译用go test -c。测试状态放Git目录外，遵守共享资源窗口和连接预算。保留原始失败及已有明确停止条件，不重置其他任务真实实验额度。
- 不自动运行付费模型、读取生产凭据、操作线上数据、发布、推送远程或发送外部通知；需要这些动作时，先完成可审阅结果再按具体授权处理。
- 不引入新依赖，除非获得明确授权；遵守React Query/Zustand及包边界、旧桌面兼容、数据库无外键和单条CONCURRENTLY索引迁移规则。

开始时简短说明当前M1范围、首个可交付子任务及验收依据，然后持续推进；只在真正影响产品语义、权限、兼容或不可逆风险的分歧处提问。
