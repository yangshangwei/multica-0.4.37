# 合并后完整测试与近期业务影响验收

## Goal

确认当前分支 `codex/cross-workspace-collaboration` 在近期合入其他分支/worktree
后仍可构建、可启动、可执行核心业务，并给出按业务范围划分的可复现测试证据。
验收结果必须区分“本次固定提交已验证”“条件跳过”和“失败/未知”，不能把已有历史
报告当成本次运行结果。

## Confirmed facts

- 当前 HEAD 为 `ee037d4ee`，相对 `origin/codex/cross-workspace-collaboration`
  多 11 个提交；近期代码变更集中在 2026-11-08。
- 当前分支的产品代码树与此前通过完整门禁的候选 `52162f19a` 相同；HEAD 与该候选
  之间新增内容仅为 `.trellis/tasks/archive/2026-11/11-08-merge-unmerged-worktrees/`
  验收记录。
- 相对 origin 的实际产品增量覆盖：
  - UI 输入边界、token 和表单可见性；
  - Collaboration 字段编辑、来源桥接、工作流动作、批量动作和富文本；
  - Task capability 的快照 runtime lookup、handler/service/auth 路径；
  - daemon 终态结果与 transcript drain 的生命周期；
  - Context experiment 停止/继续条件与 Knowledge continuation 证据；
  - Projects 生命周期刷新、全局作用域导航和完整 `make check` 的桌面 renderer 预检。
- 根 worktree 有其他任务留下的大量 dirty/untracked 文件，不能把根目录的结果直接
  当成干净合并候选的证明。仓库已有干净集成 worktree
  `/Volumes/artisan/code/2026/mc-merge-unmerged-worktrees-20261108`，其 HEAD 为
  `52162f19a`。
- 既有归档报告记录过一次 `make check` 成功，但本任务仍需重新执行至少受影响的
  targeted checks，并记录本轮固定提交和环境身份；历史证据只用于确定范围。

## Requirements

### R1. 合并完整性与测试基线

验证当前 HEAD 的产品路径与 `52162f19a` 字节/模式一致，确认未有未记录的产品差异；
所有测试在 clean checkout 或任务独占环境运行，不能读取其他 worktree 正在运行的
API/Web/数据库。

### R2. 静态与包级回归

运行 `git diff --check`、仓库 lint/typecheck、UI export 检查以及所有非 mobile 的
Vitest/Turbo 测试。对近期修改的 UI/views/web 测试文件单独保留 targeted 结果。

### R3. 后端和运行时回归

运行完整 Go race tests 与 `go vet`，并单独覆盖近期变化的 `auth`、`handler`、`service`、
`daemon`、`daemon/collaborationrun`、`cmd/server` 包。验证 task capability 的正常、
拒绝、快照/当前 runtime lookup 以及 daemon 终态/transcript drain 边界。

### R4. 业务链路与浏览器验收

在隔离数据库、API、生产 Web 和 Desktop renderer 环境中运行受影响的 Projects、导航、
Collaboration、Knowledge/Context、agent execution 等 Playwright 场景，随后运行完整
`make check` 作为跨层门禁。保留通过、失败、跳过的计数和失败用例名称。

### R5. 证据与风险报告

把命令、固定 SHA、环境端口/数据库、退出码、日志位置和结果摘要写入本任务；对失败项
定位到业务范围和可复现命令，对条件跳过说明触发条件及未覆盖的风险。测试不修改产品代码，
不清理其他分支/worktree，不推送或部署。

## Acceptance criteria

- [ ] 当前 HEAD 的产品代码与已验证候选关系已由路径级 diff 证明。
- [ ] 静态检查、TypeScript 全量测试、Go race/vet 和受影响 targeted tests 均有本轮结果。
- [ ] 生产 Web/Desktop renderer 启动、API/数据库迁移和受影响浏览器业务链路通过，或有明确
      的失败/条件跳过证据。
- [ ] 完整 `make check` 已执行；退出码、通过/失败/跳过统计与日志哈希已记录。
- [ ] 最终报告按 Collaboration、权限/Task capability、Daemon/Context/Knowledge、
      Projects/导航、UI/构建五类说明业务影响和遗留风险。

## Out of scope

- 不修改产品代码或测试实现来“迎合”本次门禁；发现缺陷时只记录并在用户明确要求修复后另开修复工作。
- 不测试 mobile（仓库完整门禁明确排除 `@multica/mobile`），不执行付费真实模型、生产部署、
  updater 发布或远端 push。
- 不把根 worktree 现有 docs、fixtures、截图和研究产物纳入本次合并候选，也不删除或清理
  任何 worktree/分支。

## Risks and deferred items

- 完整 `make check` 依赖本机 PostgreSQL、Node/Go、Playwright browser 和可用端口；环境问题
  必须与产品失败分开记录。
- Playwright 中密码、原生 Electron、真实 provider、平台专属和部署场景可能按既有契约跳过；
  跳过不等于通过。
- 归档候选已证明产品树相同，但若 clean worktree 不能直接跑当前 HEAD，将以候选固定 SHA
  的产品等价证明加上当前 HEAD 路径 diff 作为边界，并明确说明。

## Open questions

无。用户已要求完整测试并特别关注近期合入代码，验收范围由当前分支 diff、既有测试入口和
项目规范确定。
