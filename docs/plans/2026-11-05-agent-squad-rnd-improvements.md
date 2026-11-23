# 优化设计与取舍

## 方案比较
1. 逐项修复 + 复用现有能力（采用）：先关闭可重现的 P1，再把 lifecycle 校验、任务身份、指标和版本信息接成稳定交付能力；回滚范围清楚。
2. 重建通用工作流/权限平台：一次解决模型表达，但改动跨 API/DB/客户端，无法迅速降低已知风险，暂不采用。
3. 仅补提示词：交付快，但不能解决日志泄漏、测试假绿与执行权限，不能作为主方案。

## 首批架构
- 日志边界：provider event → 完整数据脱敏 → 有界预览 → RPC → 服务端再次脱敏 → DB/WS。复用 pkg/redact；字段语义与原生令牌规则在共同入口实现。保留长度/深度限制与无修改输入合同。
- 验证边界：CI 临时 PG17/pgvector → readiness → migrations → 必需 DB 测试；初始化失败终止。先关已确认的 TestMain 假绿，后续阶段再统一全仓 expected-test inventory。
- 小队边界：交付 in_review 与验收 done 分开；本批修正说明与回归，不改变持久状态或后台唤醒语义。Review Gate 六类检查由实际花名册驱动比对，不引入新角色。

## 后续目标设计
| 里程碑 | 审计映射 | 关键设计 | 前置依赖 | 验收与回滚 |
| --- | --- | --- | --- | --- |
| M0 当前 P1 | 1–4、8 的指令漂移 | 三个窄子任务，无 DB/API schema 变更 | 本轮方案 | 各子任务独立 red/green 与代码审查；按提交回滚 |
| M1 权限、门禁与有效委派 | 5、6、7、8 | task capability 是现有 principal/workspace 权限的交集；高风险动作授权绑定 SHA/环境/参数/期限；交付证据绑定 commit/artifact；leader 消费 trigger_outcomes；PR smoke | M0；资源动作矩阵与项目验收政策 | 报告员越权拒绝、旧 SHA 证据失效、重复委派唯一回执、smoke 实际执行；按项目关闭新增政策，不撤销旧授权边界 |
| M2 报告、预算与调度 | 9、10 | 只读聚合快照冻结 scope/timezone/window/watermark/coverage；模型只解释；父任务账本归集子任务和重试，原子预留/结算，未知 usage 不当 0；readiness 加队列与能力 | M1 身份/证据关联；定义统计与计费口径 | 分页/重开/取消/补跑确定性，重复周期幂等；并发预算不超发；按项目灰度，保留账本，退回原报告入口 |
| M3 持续质量与治理 | 11–14 | 固定负载 SLO/恢复演练；eval 分 expected/observed/verdict，版本/样本/成本/延迟可追溯；供应链证据随 artifact；模板差异与三方合并、知识有效期 | M1/M2 提供身份、事实与预算 | 故障场景有失败样本，模型漂移可检测，制品可核对，定制实例不覆盖；政策观察后阻断，版本化条件回滚 |

## 后续 API/数据边界（设计方向，尚未实现）
- 交付证据：按 issue + source revision + gate kind 记录 verdict、producer、artifact refs、observed_at、expires_at；授权和原子条件更新遵守既有 Tx/UUID 规则。
- 权限：每次执行绑定 resource/action 范围，不能超过人类发起者当前权限；撤权重新验证，现有机器身份不能授予更大范围。
- 报告快照：保留来源编号清单与完整性，不只存模型正文；[start,end) 和自然日/滚动窗口显式选择。
- 预算：整数金额/Token 计量、估算/实报/未知分离；重试不能创建新总预算，取消与超时释放预留需幂等。
- 新 schema 必须独立规划迁移，不加外键；concurrent index 每条独立迁移。老桌面兼容 additive response + schema parsing。

## 兼容与安全
首批不升级既有实例正文、不新增运行时权限、不改变人工 done 责任。代码评审和安全评审可在后续版本针对冻结产物并行，但本批保留每轮单成员协议，避免把吞吐优化混进状态修复。动态敏感值注册、跨流分片脱敏和通用恶意编码识别需各自预算/撤销/生命周期设计，不由首批前缀正则承担。

## 风险
- 脱敏过度会损害日志；明确敏感字段而非 token 子串匹配，保留 token_count 等对照。
- Fail-closed 暴露以前被忽略的环境问题是预期，必须提供可操作错误；不要以 skip 修绿。
- 原 worktree 有并行验收，代码和测试状态隔离；不重跑其他任务已获接受的性能例外。
- GitHub runner 完整执行需要后续 CI，不能用本地 YAML/脚本测试冒充发布完成。

## 后续工作包的可执行设计边界

| 能力 | 最小合同 | 关键负例 | 灰度/回退 |
| --- | --- | --- | --- |
| 角色 capability | Reporter: scope 内 read + 当前报告 comment/review；Reviewer: read/comment；Implementer: 已分配资源写；Coordinator: 指定父任务下调度。能力始终与发起人当前权限取交集 | 改 URL/UUID、复制 token、旧 turn、撤权、换 workspace、通过成员代做 | 先在新创建角色启用，旧实例显式迁移；禁用新策略不能放宽既有工作空间权限 |
| 受控高风险执行 | approval 绑定 actor/resource/action/commit/environment/argv digest/expires_at；原子一次性领取；独立执行 receipt | 同许可并发使用、参数变化、过期、执行结果不明重试、部分失败 | 先一个发布适配器；未知结果保持待核对，不自动重跑副作用 |
| 证据门禁 | 项目政策列出适用检查；证据绑定提交/制品与生产者，pass/hold/unknown 分离；skip 有负责人和理由 | 旧提交绿灯、伪造 reviewer 身份、已撤权结果、缺失/过期证据、并发更新 | 观察→提示→阻断逐级启用，记录政策版本；禁止把 rollback policy 当作批准危险操作 |
| 有效委派 | leader 消费已有 trigger_outcomes；记录目标 UUID、触发评论、实际排队/合并/延后/拒绝结果 | 正文有 mention 但解析失败、私有成员拒绝、离线、重复派发、成员替换 | 优先补消费和展示；只有证明分离请求不能满足原子性时再设计新接口 |
| 报告快照 | scope/timezone/[start,end)/as_of/coverage 与来源 task/activity/pr/deployment refs；确定性计数 | 多页、迟到状态、关闭再打开、取消、跨项目移入移出、缺权、报表递归计数 | 先与现有报告双向比对但不双写业务状态；明确哪种视图是权威 |
| 总预算 | parent/project/squad 的 reserved/observed/unknown，子执行/重试共享同一预算根；精度与价格版本记录 | 并发抢额、重试逃逸、取消未释放、usage迟到/重复、汇率/价格变更 | 先 observe 后 enforce；保留账本与未知成本，不把估算当账单 |
| NFR与行为评测 | 固定环境、负载、版本、样本、阈值；expected/observed/verdict分开；安全拒绝和执行失败分别解释 | 超时、工具空结果、重放、注入、上下文丢失、断网恢复、取消竞态、备份恢复失败 | 缺少基线时标unknown；真实模型须受预算与数据授权约束 |
| 模板/知识治理 | before/base/candidate hash、保留定制的合并、实例版本、证据引用、适用版本、失效条件 | 并发自定义编辑、同名不同模板、旧权限、不兼容工具版本、撤回知识仍召回 | 少量实例灰度，CAS条件写与条件回滚；不自动覆盖用户指令 |

建设顺序不是要求每项任务经过全部岗位。以风险和可验证交付为路由条件：文案跳过安全/迁移；schema或已安装客户端契约变更必须经过兼容性；Agent/Skill/MCP变更必须经过行为评测。安全与上线动作在获得对应权限之前只能提供计划。

## 后续交付链路（M1–M3 目标设计）

```mermaid
flowchart LR
  P[项目验收与权限政策] --> I[任务与阶段]
  I --> T[带资源权限与预算的执行]
  T --> R[隔离运行时与工具]
  R --> E[绑定提交和制品的证据]
  E --> G[检查齐全性与结论]
  G --> H[人工验收或获批发布]
  H --> O[发布后观察]
  I --> S[确定性事实快照]
  E --> S
  O --> S
  S --> W[日报周报与改进指标]
```

首批 M0 只修复链路中已确认的日志、测试前置条件和交接说明；图中完整权限、预算、快照与门禁仍属于后续交付。

# 开发计划

## 本轮执行顺序
1. 在独立 worktree 固定基线与实际文件范围，记录用户已批准首批。
2. 完成三个子任务 PRD/design/implement 和非空 implement/check manifests。
3. 三个 Trellis implement 子代理按文件所有权并行：redaction、CI/TestMain、squad 文本与契约。使用失败回归锁定缺陷；禁止启动真实模型或借用其他任务数据库。
4. 各子任务运行最小有意义验证，记录原始失败和修复通过，说明环境。共享包编译限制并发，测试状态在 Git 目录外。
5. 独立 trellis-check 复核全部改动、跨层合同、负例和范围；发现问题返回对应 lane 修复。
6. 主会话更新专用 spec/进度，检查 diff、格式和本批适用测试；以 Lore trailer 原子提交到独立分支。
7. 汇总计划与实际交付、剩余里程碑；本轮不执行线上发布。

## 所有权与检查
- Redaction lane：pkg/redact、必要 daemon 预览调用与窄测试。
- CI lane：release.yml、必要 ci 触发/测试入口、脚本、handler/router（及经研究确认的 testutil）数据库 TestMain。
- Squad lane：execenv workflow 指令与测试、角色/skill/squad 摘要、对应双语产品文档与 source map。
- 主会话：本任务树、设计计划、专用 spec、审查汇总与 git；不覆盖其他任务 spec/journal 工作。
- 各 lane 遇到重叠文件先报告。纯 Go/脚本文本改动无需无关前端构建；若涉及 i18n，同步校验双语 key 与类型。

## 后续执行包
| 执行包 | 优先级 | 交付与负责人职责 | 前置/依赖 |
| --- | --- | --- | --- |
| P1-A capability/受控执行 | P1 | 后端权限与安全审查负责资源动作矩阵、撤权、批准执行回执；发布工程负责执行适配 | M0；先审 API 兼容与高风险动作合同 |
| P1-B DoD/证据门禁 | P1 | 后端/前端负责项目政策、状态 UI、artifact 一致性，QA 验证拒绝路径 | P1-A 身份；复用 lifecycle 校验 |
| P1-C 关键路径 CI/有效委派 | P1 | CI owner + QA 跑固定 fake-provider smoke，后端消费真实派发回执 | M0-CI/M0-squad；不依赖真实付费模型 |
| P2-A 报告快照 | P2 | 后端聚合、报告员解释、产品确认自然周期/滚动周期，QA 核对历史口径 | P1-B 交付语义；跨区范围与水位合同 |
| P2-B 总预算/调度 | P2 | 后端账本与运行时准入，可靠性核对并发、取消、未知 usage | P1-A；费用/时间/Token 的政策确定 |
| P2-C NFR/eval/供应链/升级 | P2 | 可靠性、Agent evaluator、安全、迁移共同验收；技术文档记录恢复 | P1-C 自动化证据；P2-B 提供费用上限 |

每个后续包启动前建立独立、可测试的 PRD 与技术设计；其范围不因本轮三个子任务完成而自动标记完成。预计投入按复杂度分为小/中/大，时间排期应在依赖与真实执行基线具备后再承诺；不虚构工期或性能收益。
