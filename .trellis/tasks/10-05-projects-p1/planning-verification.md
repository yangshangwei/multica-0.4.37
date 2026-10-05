# P1 规划交付核验

日期：2026-10-05。本记录核验用户要求的 Trellis 任务、技术设计和开发拆分；不表示产品已经实现或发布。

## 目标逐项核对

| 用户要求 | 当前权威证据 | 判定 |
| --- | --- | --- |
| 建立 Trellis 任务 | `task.json` 与 foundation/health/progress/ui/verification 五个子任务，父子链接互相对应；均为 planning，implementation_status=not_started | 已完成规划建档 |
| 补齐技术设计 | [design.md](design.md)、[api-contract.md](api-contract.md)：数据模型、版本/CAS、快照、权限、通知、删除并发、迁移/回退和兼容合同 | 已完成 |
| 补齐开发拆分 | [implement.md](implement.md) 与子任务文档/metadata：明确文件所有权、共享整合者、准备/启动/集成门槛及验证命令 | 已完成 |
| 目标模板 | 单一项目描述，模板预览与追加、并发编辑；PRJ-002、AC-02/03 | 已进入设计与测试计划 |
| 完成／取消统计 | 旧 done_count 保留闭合含义，新分项区分完成/取消；PRJ-004、AC-07/08/09 | 已进入设计与测试计划 |
| 健康概览 | 同快照全量集合、风险解释、下钻隔离个人筛选、变化刷新与不完整提示；PRJ-006/007、AC-12—17 | 已进入设计与测试计划 |
| 手动进展 | 人工发布、更正历史、幂等、成员通知及目标验收不可变证据；PRJ-003/008/009、AC-04—06/18—20/27 | 已进入设计与测试计划 |
| 延续 T1 正式准入口径 | PRD、设计与测试统一限定同空间真实项目关联，准入仅 not_required/accepted；pending/rejected/duplicate 与候选项目不计；AC-10/11 | 已明确并经代码映射核对 |

## 审查记录

- 第一轮 [Architect](research/architect-review.md) 与 [Critic](research/critic-review.md) 要求补齐发布隔离、通知锁序、DTO/证据版本、旧端能力检测、调度门槛和移动独立验证。
- [修订记录](research/planner-revision.md) 逐项对应 AR-01—04、CR-01/02，保留方案取舍与不实施 I1/P2/P3 的边界。
- 顺序复审：[Architect 第二轮](research/architect-review-2.md) APPROVE 后，[Critic 第二轮](research/critic-review-2.md) APPROVE。结论仅为方案可交付。

## 文档与任务检查

- 六个任务均运行 `python3 .trellis/scripts/task.py validate <task-dir>`：context 路径有效，无截断警告。
- 自定义只读核对覆盖六个 task.json、父子双向关系、五套阶段门槛、12 份 JSONL、全部本任务 Markdown 本地链接和 `git diff --check`。
- 验收规格逐项保留 14 个 PRJ 功能编号、27 个 P1 AC、18 个补充边界；原 PRD 与两份代码映射提供语义依据，未只以编号计数代替审查。
- 注入文档单文件低于 32 KiB；每套 manifests 加自动注入的 PRD/design/implement 均低于 128 KiB，避免验收与接口内容被截断。完整原需求分为两份 source excerpt，原 PRD 仍权威。
- 本轮仅创建/修改任务与规划文档。未运行产品单测、数据库迁移或浏览器测试；未将 T1 的既有结果作为 P1 通过证据。

## 交接限制

产品实现仍需按测试规格逐项验证。旧客户端真实改写描述而不带版本会收到 428；这是已记录的防丢写取舍。性能阈值仍需实测基线后固定。I1 不存在的真实迭代集成分支保持待 I1 补验，不虚构通过。下一开发起点是 foundation；FG 就绪后才启动其他实现子任务，foundation 继续承担下游共享文件整合。
