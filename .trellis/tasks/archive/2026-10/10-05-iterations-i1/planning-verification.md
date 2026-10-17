# I1 工程规划核验

日期：2026-10-05。范围仅本任务工程方案及六个子任务，全部保持 planning；产品实现 not_started。

## 交付

prd.md、design.md、api-contract.md、implement.md、test-spec.md、task.json、implement/check.jsonl；research 下两份原文摘录和 Architect/Critic 评审。六子任务分别 foundation、lifecycle、history、closure、clients、verification，均有独立 task/prd/design/implement/context。总入口 docs/plans/2026-10-05-iterations-i1-implementation.md。

## 已完成验证

- 独立 Architect 顺序先审：修复 today/handoff 跨午夜派生日期与锁后业务时刻采样后 APPROVE。
- 独立 Critic 后审：修复新增唯一任务数、FG/FCG与幂等底座所有权后 APPROVE，无规划阻塞。
- 对父任务和六子任务分别运行 `python3 .trellis/scripts/task.py validate <task>`：7/7 exit 0，无context截断警告。
- JSON/JSONL 可解析，全部 task.status=planning，implementation_status=not_started；双向父子链接及共享合同路径存在。
- 本范围 Markdown 相对链接与 context 文件存在性检查通过；context单文件小于32KB。原始PRD超过上限，已按产品/验收拆分原文摘录，未修改原始需求文件。
- ITR-AC-01—29 全覆盖到 canonical 测试层，I2/I3不纳入；额外覆盖29项以外的 handoff/迁移/全部 writer/幂等/旧端/午夜边界。
- 本范围新增文档空白检查，无行尾空格；无产品代码修改、无新依赖、无提交或推送。

## 尚未执行

未运行Go/TypeScript/端到端/性能/迁移产品测试，没有产品通过或发布声明。FG/LG/HG/CG/UG/VG 都待实施证据。P1 尚未实现的联合删除测试待其落地，现有项目删除保护必须在I1自身实现中先测。首批试点空间、实测性能阈值和部署上限待预发布记录，缺少试点授权时只关闭开关交付。

## 风险与简化

复用现有 Issue、Project、T1准入与P1统一规划时区，避免并行专项实体/第二套时区和事件溯源重构。单空间写fence为一致性付出吞吐成本，全部状态和执行启动writer覆盖是FG硬门槛；实际测试前不能声称风险已消除。历史数据产生后禁止回退不支持事件捕获的旧服务端，优先前向修复。
