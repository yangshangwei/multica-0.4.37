# 双闭环验收收口与未完成证据审计

## Goal

收口 RCA 三路由浏览器创建与授权边界证据，审计双闭环父任务和历史子任务验收状态，补齐可授权的最终检查。

## Requirements

- 复核 `bug-fix`、`maintenance`、`incident` 的模板 roster、squad instructions、runtime handoff 和权限边界；未知原因必须进入诊断，事故必须先缓解。
- 运行真实 API 浏览器 E2E，验证 `diagnostician` 创建、默认 `multica-debugging` skill、Contributor autonomy、并发上限、本地化和 workspace 自定义副本不被覆盖。
- 对真实模型 RCA 三类样例（可复现、间歇性/暂不可复现、证据不足）检查授权条件；没有显式授权时只记录精确 blocker 和可重复命令，不运行模型。
- 审计父任务和全部历史子任务的状态、验收勾选和证据文件，不能通过修改状态掩盖缺口。
- 不执行生产发布、回滚、迁移、恢复、通知，不读取生产凭据或客户原始数据，不新增 MCP、数据库表或控制器。

## Acceptance Criteria

- [x] RCA 三路由的可授权 Go/模板检查和真实 API 浏览器 E2E 有实际结果；失败、跳过和通过分开记录。
- [x] 浏览器证据包含模板字段、skill、权限、并发和 workspace copy preservation。
- [x] 真实模型检查有模型/工具/环境/授权状态记录；未获授权时明确为 `blocked-by-authorisation`，不标记为通过。
- [x] 父任务及历史子任务状态矩阵已记录，父任务只有在所有必要证据具备时才允许结束。
- [x] 相关 Go、TypeScript、Trellis validation 和 diff 检查通过，或将精确 blocker 写入 evidence。

## Requirements

- TBD

## Acceptance Criteria

- [ ] TBD

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
