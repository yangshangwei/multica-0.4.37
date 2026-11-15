# 首轮协作修复交付闭环续作计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 完成现有首轮闭环任务的完整确定性验证、password 定向验收和原 E0–E6 实验证据，并按实际结果核销父子任务。

**Architecture:** 保留失败的 v9/v10 原始证据，在依赖和构件均属于同一工作区的候选上进行新一轮完整检查。主会话负责资源归属、执行和任务核销，Trellis 子代理负责独立复核及有明确边界的修复。原实验继续使用已登记的两条旧产品源码、固定 Provider 和剩余试次。

**Tech Stack:** Go、PostgreSQL、pnpm/Turborepo、Next.js production、Playwright、原生 Electron、私有 Python 验证启动器。

---

## 1. 核对最新证据与接管入口

- [x] 激活 `.trellis/tasks/10-30-collaboration-fix-closure`，保留共享工作区的其他任务改动。
- [x] 读取实际 v10 记录：候选 `118fc53ea8eb8ebc53b4f9c1131e3e7910eacce9` 的完整命令 exit2；浏览器39 expected、36 unexpected、364 skipped，含341未运行和1 interrupted。台账中的“v10仅准备”已过时。
- [x] 发现后续依赖隔离和 v11 准备记录：候选 `/Volumes/artisan/code/2026/mc-collaboration-final-owned-20261103`，HEAD `304f74dd3ba4ddea862c94917c5bb0770355a351`；新记录需以实际读取的最新 provenance 版本为准。
- [x] 保存候选、launcher/config/bindings、依赖真实路径、构件及运行状态快照；两条会话明确移交独占窗口，旧catalog与预检API/Web已确认归属并停止。
- [x] 独立复核 v10/v11执行与来源；v11私有50项通过、0初始阻塞，另补最终身份审计并独立16项通过。旧失败保留。

## 2. 准备并执行完整检查

Files: 候选 `.omx/private/final-check-v11/` 下的版本化启动器、绑定及新执行证据；本任务 `research/continue-20261103/`。

- [x] 使用独立复核的dd0f5903 launcher、6db6435e bindings、999decf0配置；current canonical462/7962、dev3f97及alltracked26e3/17182分开绑定。
- [x] 重新核对19846 renderer当前HTTP/文件哈希和PID/cwd、Redis16422确切容器/零阻塞客户端，以及PG100上限/33既有API idle连接；保存当前主会话资源证明。
- [x] 在实际自有依赖和新迁移DB完成production SSR/API/WS identity及原4项auth测试4pass0skip/retry；预检结果不代替整套。
- [ ] 由主会话通过审定启动器执行仓库原始 `make check`，每个 Go 包使用新迁移克隆，Web 为 production；保存日志、所有具名结果、原 Playwright JSON 和最终退出码。
- [ ] 完整 Make 结束后实际运行审定 `final_state_audit.py --output <新报告>`，核对原始整套 exit0、审计 status=pass 和完整具名 inventory 三者；私有16项回归或初始检查不替代这个最终门。
- [ ] 若出现新失败，先保留失败及确定根因；在所属层补有意义的回归并做最小修复，独立复核后使用新的冻结候选/运行编号完整重跑。不得扩大超时、增加跳过或拼接通过结果。

## 3. 补 password 定向验收

Files: 原 C3/runtime handoff E2E；候选私有 password 部署配置及本任务新证据。

- [ ] 完整默认检查结束后，在同一候选新建独立 password 环境，核对 registry、实际数据库、API/Web/renderer 归属和页面实际 API/WS 地址。
- [ ] 运行 C3 Web/native/offline 四项与 runtime handoff 三项，要求实际7pass、0skip/retry；默认部署的条件性跳过不能关闭此门。
- [ ] 保存原断言、构件来源、实际平台/认证模式及原始结果；具体失败保留并按根因修复。

## 4. 按原登记续作实验

Files: 原两个 frozen source worktree；本任务 `research/continue-20261103/original-series-run/` 的追加式接管登记与私有证据。

- [ ] 独立确认旧槽位、实际调用核销、正式 E0、源码和 successor 框架。当前只读快照为13次已消耗、剩余上限14次；真实启动前再次核对。
- [ ] 使用当前主会话身份建立新的接管登记和 hash-bound 执行放行；不复用旧会话身份。固定 Claude2.1.316、model `claude-opus-5-5`、场景/评分/超时及原源码。
- [ ] 确定性门和独占账号窗口具备后，串行运行 E3 after trial2/3 → E4 baseline1/2/3 → E5 baseline1/2/3 → E6 baseline1/2/3（每轮两次启动）。
- [ ] 每场景独立复核 prompt/HTTP/tool/outcome/lifecycle/proof 和语义标准；首个运行不完整或 E5 canary 立即停止后续调用，保留分类、真实次数和下一入口。旧无效槽位不补跑，RK1和知识系列不占用本任务预算。

## 5. 核销交付

Files: 本任务 task/implement/check manifests、验证报告、父任务验收映射；必要的项目 spec。

- [ ] 独立 check 子代理执行本任务全范围复核，逐项对应“问题 → 修复 → 原始证据 → 限制”。
- [ ] 更新父任务 AC0/AC3 关联项和本任务 metadata，记录所有条件skip、模型行为失败及未运行门。
- [ ] 仅提交本任务实际拥有且已验证的文件，使用 Lore 决策记录；公开推送/部署/发布不属于本次工作。
- [ ] 所有必需门完成后归档；仍有环境阻塞或未运行项则保留 in_progress 和具体恢复入口。

## 验证原则

完整命令的原始退出码、具名 inventory 与来源绑定共同证明确定性门。准备、fake、源码路径检查和定向回放分别记录，不能替代完整运行。计划随新证据更新，既有失败文件保持原样。
