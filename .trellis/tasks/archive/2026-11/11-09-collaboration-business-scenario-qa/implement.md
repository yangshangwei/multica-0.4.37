# 执行计划

状态：in_progress（2026-11-09校准）。隔离环境已构建，主线复现4/4通过；2026-11-10第3–5节执行及证据复核已完成，下一步按路径范围提交归档，保留已有原始结果，不重复创建任务。环境脚本：`research/environment/qa_env.py`（只写本任务私有目录、新数据库和证据目录）。

## 0. 前置（批准后）

- [x] `python3 ./.trellis/scripts/task.py start .trellis/tasks/11-09-collaboration-business-scenario-qa`
- [ ] 记录 HEAD、非 `.trellis` 的脏路径清单，以及 Docker 中 `multica-postgres-1` 的 StartedAt（用于判断测试期间数据库是否被他人重启）。

## 1. 环境（约 30–40 分钟，主要花在构建）

```sh
Q=.trellis/tasks/archive/2026-11/11-09-collaboration-business-scenario-qa/research/environment/qa_env.py
python3 $Q allocate      # 私有根、DB 名、端口复核
python3 $Q snapshot      # git archive HEAD → source/，零漂移审计，pnpm install，workspace 链接核对
python3 $Q build         # api/cli/host-daemon/三种 Provider/web/renderer，记录哈希
python3 $Q database      # 余量与重名检查 → CREATE → 身份核对 → migrate
python3 $Q start && python3 $Q verify
```

- 任一步失败：保留日志，修正后在新的私有根和新数据库上重来，不复用半成品。
- 脚本在第一次执行前先 `py_compile`；每个子命令执行后读它的 JSON 输出再进入下一步。

## 2. 复现批次（知识开关关闭）

```sh
python3 $Q run --provider closure QA_RUN_ID=r1-closure pnpm exec playwright test e2e/collaboration-multi-account-closure.spec.ts --config playwright.qa.config.ts
python3 $Q run --provider host    QA_RUN_ID=r2-host    pnpm exec playwright test e2e/collaboration-host-squad.spec.ts e2e/collaboration-host-switch.spec.ts --config playwright.qa.config.ts
python3 $Q run --provider memory  QA_RUN_ID=r3-memory  pnpm exec playwright test e2e/collaboration-memory-runtime.spec.ts --config playwright.qa.config.ts
```

先执行 `python3 $Q sync`，写入任务专用的 Playwright 配置（单 worker、0 重试、trace 关闭、JSON 报告）。

## 3. 编写新测试（与第 2 步并行）

- [x] `e2e/fixtures/collaboration-business-scenarios.ts`：helper 与证据记录器（见 design §4.2）。
- [x] `e2e/collaboration-business-scenarios.spec.ts`：S1、S2、S3。
- [x] 静态检查：
  - 聚焦 `tsc --noEmit --strict ...`（与 Codex 审核用的参数一致）；
  - 聚焦 eslint（`packages/eslint-config/base.js`）；
  - `pnpm exec playwright test --list` 确认收集到 3 个用例。
- [x] `python3 $Q sync` 同步到快照，记录哈希。

## 4. 业务批次

```sh
python3 $Q run --provider host QA_RUN_ID=b1-support-governance pnpm exec playwright test e2e/collaboration-business-scenarios.spec.ts --grep "S2 support ticket|S3 governance" --config playwright.qa.config.ts
# 只重启 API 并打开知识开关（脚本记录配置切换），verify 后再跑 S1
python3 $Q run --provider host QA_RUN_ID=b2-outsourcing QA_KNOWLEDGE_CONTEXT=on pnpm exec playwright test e2e/collaboration-business-scenarios.spec.ts --grep "S1 outsourcing" --config playwright.qa.config.ts
```

- 失败时：先保存截图、证据 JSON 和 API 日志片段，再判断原因：
  - 测试代码错误：修正后用新的 run ID 重跑（如 `b1-...-rerun1`），首轮结果保留；
  - 产品缺陷：记录最小复现，继续执行不依赖该缺陷的步骤（必要时拆出补充测试），不修改产品；
  - 环境问题（如数据库被他人重启、`57P03`）：核对 StartedAt，恢复后用新的 run ID 重跑。
- 新测试只在本任务数据库里运行，各批次串行执行，不与其他测试共享数据库闸门。

## 5. 收尾与报告

- [x] `python3 $Q audit`：快照仍与 HEAD 零漂移，本任务新增文件的哈希与同步记录一致。
- [x] `python3 $Q stop`，核实端口释放和闸门恢复。数据库、日志和私有根保留（README 写明位置与清理方法）。
- [x] 汇总 `results.json` 和中文 `report.md`：
  - 环境与源码身份、每个场景的叙述和拓扑；
  - 逐步的预期与实际、证据与截图链接；
  - 与 Codex 对照的覆盖矩阵；问题列表（产品、设施、环境三类分开）；
  - 未覆盖项和复现命令。
- [x] `python3 $Q scan`：原15项公开标识符误报保留；精确上下文独立复核后实际凭据形态0命中，见credential-scan-adjudication-20261110.json。
- [x] 独立复核：由另一个代理只读核对报告里的数字与证据文件。如果网关不稳定导致代理不可用，就由主会话逐项核对，并在报告里说明复核方式。
- [x] Phase 3：只提交本任务新增的测试文件、证据和任务目录，不夹带工作区里其他会话的修改。提交前再看一遍 `git status`、`git log`，确认没有同伴会话改过这些文件。

## 风险与回退点

- 网关不稳定（子代理已两次因 ECONNRESET 中断）：主会话直接执行；长构建放后台，完成时会收到通知。
- 共享 PostgreSQL 被其他会话重建：比较 StartedAt，换新的 run ID 重跑。
- 知识注入不作用于二期任务：如实记为发现；可以补充同一任务内的复用作为对照，但要单独标注。
- 运行时间：环境约 40 分钟，编写约 1.5 小时，执行与排查约 1–1.5 小时，报告约 0.5 小时。
