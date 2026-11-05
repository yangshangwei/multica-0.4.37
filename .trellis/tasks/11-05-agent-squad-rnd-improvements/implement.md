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
