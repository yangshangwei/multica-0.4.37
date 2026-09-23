# 技术设计

本任务只扩展现有 built-in role/skill/squad/autopilot/MCP 模板体系，不增加新的 agent 类型、数据库表或生产控制面。skill 继续在模板创建时复制成 workspace-owned 普通 skill，后续版本不得覆盖用户编辑；squad 只负责把阶段和交付物路由到现有角色。

## 路由

- `incident`：release engineer 先在人工审批下止血，QA 确认用户侧恢复，diagnostician 在恢复后承接独立 RCA，incident-learning skill 产出复盘和预防任务；incident 主任务不等待学习完成。
- `release`：QA 针对不可变 artifact 验证，technical writer 完成发布文档，release engineer 准备发布/回滚；审批后由 rollout/canary skill 和 reliability engineer 记录观察窗口及同产物健康结论。
- `review-gate` / `release`：Agent 变更由 agent-evaluator 生成版本化评测和门禁建议，security/code/QA/release 仍保留各自独立结论。
- Reliability Review、Migration Review、Experience Validation 先作为可组合路由和 skill 证据表，不新增默认 squad；达到 workload gate 后另行进入 listed roster。

## 证据契约

所有新 skill 采用事实、推断、未知、负责人、验收信号、停止条件表。发布验证绑定 artifact digest、baseline、window、signals、decision、rollback outcome；事故学习关联原事故和既有未关闭任务，禁止重复无主任务。生产指标读取、生产操作、真实客户数据和付费模型均要求人工批准或显式授权。

## MCP 取舍

保留 Chrome DevTools、Playwright、Sequential Thinking 三个无凭据模板。Git/CI、Observability、Database MCP 只在未来满足官方来源版本固定、凭据注入不进模板、只读最小权限、脱敏和审计后再加入 catalog。
