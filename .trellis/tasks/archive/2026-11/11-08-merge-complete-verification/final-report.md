# 合并后完整测试与近期业务影响验收报告

## 结论

候选产品树 `52162f19af7e4a1e2459ce3f08e4627ca14c3dfc` 已在独占 API/Web/Desktop 环境完成静态、包级、Go race/vet、生产构建和业务浏览器验收。当前 HEAD `ee037d4ee3c4e67598e0767c056a8244a7a1ccbf` 与候选在 `apps/`、`packages/`、`server/`、`e2e/`、`scripts/` 和构建配置上无差异；当前 HEAD 新增的是任务记录。

完整 `make check` **不是全绿**：Playwright 原始结果为 `441 tests: 327 passed, 112 skipped, 2 failed`，退出码 `1`。两个失败分别是 Collaboration stale lifecycle 的 `POST` response 等待超时和 Desktop Iterations I1 的 native assignment dialog 隐藏超时；它们在同一候选构建、同一独占环境中单独复跑均通过（`2 passed, 19.2s`）。准确结论为：**原始门禁有两个时序敏感失败，单独复跑未复现稳定业务回归**。

## 固定身份与证据

| 项目 | 值 |
| --- | --- |
| 当前 HEAD | `ee037d4ee3c4e67598e0767c056a8244a7a1ccbf` |
| 产品候选 | `52162f19af7e4a1e2459ce3f08e4627ca14c3dfc` |
| 候选 worktree | `/Volumes/artisan/code/2026/mc-merge-unmerged-worktrees-20261108`（clean） |
| 检查环境 | `check-20261108112551-74475` |
| API / Web / renderer | `18917` / `13837` / `6011` |
| API DB / Go DB | `multica_check_20261108112551_74475_api` / `multica_check_20261108112551_74475_go` |
| 完整日志 | [`make-check.log`](make-check.log)，SHA256 `10221e3f66435befa5c15ff43813576e3733e20cd8e04774cebabe86155c1d71` |
| 复跑日志 | [`targeted-rerun.log`](targeted-rerun.log)，SHA256 `b50c28a814a8a91bd91221c697a88ffa026e48880b9ca03cfc70c8e3d20cee37` |
| 原始失败摘录 | [`research/original-playwright-failures.md`](research/original-playwright-failures.md) |

## 五个业务域

### 1. Collaboration — PASS / SKIP / FAIL（未复现）

**PASS**：Collaboration 字段编辑、来源桥接、工作流动作、批量动作、富文本、Knowledge Web 链路和相关 TS/Go targeted 检查通过；stale lifecycle 场景单独复跑通过。

**SKIP**：真实 provider、部分 native/installed/deployed collaboration 场景遵循仓库既有条件 gate 跳过，不能当作通过。

**FAIL（原始证据，未复现）**：完整门禁在 `e2e/collaboration-workbench-web.spec.ts:183` 等待 lifecycle `POST` response 超时 15 秒，页面停在 Waiting for the server；同环境单独复跑通过。保留该原始失败计数和日志。

### 2. 权限 / Task capability — PASS / SKIP / FAIL

**PASS**：task capability 的正常、拒绝、snapshot/current runtime lookup，以及 auth/handler/service/cmd-server targeted race、Go 全量 race 和 `go vet` 通过。

**SKIP**：真实商业 provider 不在本轮授权范围。

**FAIL**：未发现稳定产品失败。

### 3. Daemon / Context / Knowledge — PASS / SKIP / FAIL

**PASS**：daemon 终态结果与 transcript drain、watchdog、Context fake-provider stop/continue、Knowledge continuation 和 Web 业务证据通过；相关 Go race 与 TypeScript 测试通过。

**SKIP**：真实 provider/付费模型、部分 native/deployed knowledge 场景跳过。

**FAIL**：未发现稳定产品失败。

### 4. Projects / 导航 — PASS / SKIP / FAIL

**PASS**：Projects lifecycle 刷新/撤销、全局作用域导航、请求替换后的恢复，以及完整门禁中的相关浏览器场景通过。

**SKIP**：平台专属和部署场景按既有 gate 跳过。

**FAIL**：未发现稳定产品失败。

### 5. UI / 构建 — PASS / SKIP / FAIL（未复现）

**PASS**：lint/typecheck `15/15`、UI wildcard exports、全部非 mobile TypeScript 测试、生产 Web 构建/启动、Desktop renderer 构建/启动通过；I1 场景单独复跑通过。

**SKIP**：mobile 明确不在完整门禁范围；密码、平台专属和部分 packaged/deployed native 场景按既有条件跳过。

**FAIL（原始证据，未复现）**：完整门禁在 `e2e/iterations-i1-desktop.spec.ts:33` 等待 native assignment dialog 隐藏 5 秒超时；同环境单独复跑通过。保留该原始失败计数和日志。

## 门禁与边界

- 通过：lint/typecheck、UI exports、非 mobile TS 全量、targeted Go race、Go 全量 race、`go vet`、API/迁移、生产 Web、Desktop renderer。
- 原始完整门禁：Playwright `327 PASS / 112 SKIP / 2 FAIL`，退出 `1`；不能写成完整门禁通过。
- 复跑：两个原始失败各自保留原始证据后，在同候选和独占环境单独执行，`2 PASS`，未复现稳定业务回归。
- 112 个 skip 的边界：mobile、真实 provider/付费模型、密码/平台专属 native、installed/deployed/平台专属场景等，具体触发条件由各 E2E gate 和日志保留。
- 根 worktree 的既有 dirty/untracked 文件和其他 worktree 未被重置、清理或纳入候选结果；本任务仅新增任务证据。

## Phase 3 记录

- Phase 3.2：两个原始失败均保留，完成同环境单独复跑分类；未修改产品代码或测试实现。
- Phase 3.3：复核相关 code-spec；现有 E2E 环境规范已经覆盖生产构建、独占环境、原始失败留存和编号复跑，本轮没有新增 executable contract，因此不改 `.trellis/spec/`。
- Phase 3.4：待提交的文件仅限本任务的规划/证据文档：`progress.md`、`final-report.md`、`research/original-playwright-failures.md` 及既有任务记录；原始日志文件继续留在任务目录并由 SHA256 标识，不提交产品代码或其他任务文件。
