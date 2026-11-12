# 后续能力交接

本任务来源于用户批准的阶段结项，保留原路线，不表示全部路线已完成。

- [原 PRD](../archive/2026-11/11-05-agent-squad-rnd-improvements/prd.md)
- [原设计](../archive/2026-11/11-05-agent-squad-rnd-improvements/design.md)
- [归档评估](../archive/2026-11/11-05-agent-squad-rnd-improvements/research/archive-readiness-2026-11-12.md)
- [已交付阶段清单](../archive/2026-11/11-05-agent-squad-rnd-improvements/stage-closure-2026-11-12.md)

旧目录内的路径、源码哈希、审查与调用结果均属于其记录时间的历史事实。新实现必须使用当前源码身份；新验收结果不能改写旧失败。

## 2026-11-12 本轮规划交接

已按用户指定读取当前任务、阶段结项、完成门禁/提示 spec 和独立真实运行验收 PRD，并核对当前源码。开始时仍为 planning，design.md/implement.md 缺失，implement.jsonl/check.jsonl 都为 0 字节；无 active task pointer。本轮没有将 task.py start 当作选中任务操作。

当前首包是 M1-S1：普通单事项查看并选择当前证据 → 明确授权 → 提交完成 → 查询原操作和当前任务。详情见：

- [需求与验收](requirements.md)：16 条首包验收、其他入口的现行合同/待设计/后续顺序、M2 和五个 M3 独立包。
- [设计提案](design.md)：两个无业务写的准备接口、复用既有授权和 writer、两步明确 UI、CLI 五个子命令、原操作恢复。
- [实现计划](implement.md)：门禁、文件 ownership、顺序、验收主层、测试命令及回退。
- [后端核对](research/m1-backend-current-contract.md)、[客户端核对](research/m1-clients-current-contract.md)、[独立规划复核](research/m1-planning-review.md)。
- [续接状态](research/resume-check-2026-11-12.json)、[文档验证](research/m1-planning-validation.json)。两份 jsonl 已填真实 spec/research，上下文只覆盖首包和需保留的既有合同。

不要在实现时重新发明后端授权。已确认的主要缺口是任务范围证据发现、当前规范绑定/期限预览、共享 UI/CLI 的授权消费与未知恢复。准确字段是创建 header `Idempotency-Key`、body `operation_id`，普通 PUT 的 `completion`，创建回查 query `workspace_id`。原 GET `issued/not_checked` 不是当前可用证明；不能因查询404或未 consumed 自动重复写。

原操作创建未知查回原 issued 且未派发完成，可确认已签发并等待首次明确提交；完成/撤销未知查到 issued 则仍未知。已 consumed 后任务重开，保留历史收据并另读当前状态。候选列表必须遵守原 artifact authority，不从审批可见性推导制品可见性，分页游标也不能泄漏隐藏记录。

用户已明确批准 M1-S1 规划并启动现有任务。当前状态、文件 ownership、实现顺序与验收门槛以 implement.md 和各 lane 的 research 记录为准；所有操作继续保留其他工作区改动。后续入口和 F2–F5 需要各自设计，不能从本批准推导执行权。若实现中需实质改变已批准 wire、权限或范围，回到规划审阅。

真实 Provider、知识注入、发布、Hosted CI、部署和外部消息仍未授权；原授权、分系列预算、历史失败与停止条件继续由独立真实运行任务保留。账单对账和 Windows 范围不恢复。

## M1-S1 实现进展（2026-11-13）

普通单事项首包的后端准备 API、Core 原操作协调器、Web/Desktop 共享流程和 `issue completion` CLI 已实现。定向后端 handler/router normal/race 均零失败/跳过；Core 全量 4,546 项通过；M1 与受影响 Views/surface 定向 84 项通过；CLI 全包、vet/build 与内置 skill 合同测试通过。最终独立检查追加了状态/applicability、receipt 精确绑定和过期授权拒绝回归，详见 [最终检查](research/m1-final-check.md)。

实现和测试合同见 [implement.md](implement.md)；lane 证据分别见 [backend](research/m1-backend-resume.md)、[clients](research/m1-clients-resume.md)、[CLI](research/m1-cli-resume.md)。

仍待完成 enforcing Web/Desktop renderer 的人工流程/键盘/窄屏视觉验证。现运行 Desktop 的 Test 项目为 off/observe，未能打开授权弹窗；期间 POLA-1 曾由 `todo` 变 `done` 后恢复为 `todo`，两条历史事件仍在本地数据库。不要在此事项上继续测试，也不要为触发弹窗直接调整共享项目策略。Views 全包在修复前有 78 failures，其中回归及 timeout suites 后续定向通过；全包未在修复后重跑。
