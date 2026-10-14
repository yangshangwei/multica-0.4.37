# I1 最终本地验收

> **本地提交已完成（2026-10-07）：** 下文“未提交工作区／本轮不提交”描述的是验收时的源码状态，保留为历史事实。已验收源码现以五个本地提交落在 `codex/projects-p1`，对应关系见[本地提交与已验证源码的对应关系](#本地提交与已验证源码的对应关系)。仍未推送、合并或部署，VG正式远端CI待办不变。

日期：2026-10-07。分支 `codex/projects-p1`，HEAD `115b4cd28277941a8d9ad8fda5acecf99b999152`。CG、客户端与此前修复均保留在未提交工作区。本轮没有提交、推送、合并、部署或启用发布开关。

## 结论与阶段边界

- FG/LG/HG/CG 的原通过结论保留。
- [29条验收矩阵](acceptance-matrix.md)及[原始PRD界面审查](ui-final-audit.md)已逐项核对；没有用全绿日志替代产品要求核验。
- **UG、FCG 已通过并关单**。最终视觉94/pass；12张最终截图及SHA保留于 `browser-evidence/final-screenshot-manifest.json`，完整要求与范围见 [UI审查](ui-final-audit.md)。
- VG 的全部本地项已验证；原执行表要求的**远端正式CI未运行**，因此 VG 不能标无条件 `passed`，父任务与 verification 子任务仍保留 `in_progress`。这是明确的本地／发布边界，不是回到CG或重新开发客户端。

## 本轮修正及验证增量

| 文件范围 | 实际改动 | 验证 |
| --- | --- | --- |
| `packages/views/iterations/iteration-page.tsx`、`iteration-operation.tsx`、`iteration-events-view.tsx`、EN/ZH projects locale | 手动模式说明、终态选择可读标识、冻结事件任务身份、保存时区预览时间 | 相关views100 tests、typecheck、scoped lint；四项真实I1 E2E |
| `iteration-operation.test.tsx`、`iteration-details.test.tsx`、`e2e/fixtures/iterations-i1.ts` | 回归实际遗漏，加强既有场景并保存预览／事件截图 | 3个RED行为遗漏后GREEN；不放宽旧业务断言 |
| `apps/mobile/.../(tabs)/inbox.tsx`、`lib/inbox-display.ts`及既有测试 | 有效当前空间迭代通知提示使用Web/Desktop；拒绝无效目标和跨空间回落导航 | 196 tests+iOS脚本、typecheck、lint0错误；原生接线审查，未做真机视觉 |
| `server/cmd/migrate/migrate_iteration_recovery_test.go` | 真实550–566部分／无效索引恢复、负控制、幂等、空／有数据回退 | 新建独占DB，race2顶层/4子项通过；库已确认所有权并删除 |
| `server/internal/handler/iteration_history_test.go` | 复用A–J fixture：真实父子各一次；F移出再由普通HTTP完成不改旧期 | History15顶层/21 pass事件、race0skip；go vet/gofmt通过 |
| 现有server／web验证规范 | 记录真实迁移／历史断言及多测试环境的构建代理核对方法 | 文档复核、最终diff-check |

本轮共13个运行／测试文件新增或修改。复用了既有统计fixture、格式化函数、通知guard模式和四个E2E场景，未增加依赖、业务层或重复统计实现。其余未提交文件是接手时已有成果，未回滚或自动纳入提交。

## 完整基线与最终源码的对应关系

[源码证据](source-provenance.json)同时记录两轮边界：

1. **完整基线**：原会话10:38启动 `pnpm --filter @multica/desktop build` + `make check`。与09:46的6,237文件快照相比仅5项变更，均早于该次启动；接手两次SHA核对稳定。15/15静态任务、10,215 TS tests、全Go race/vet、生产Web/Desktop、249 E2E通过／49跳过。
2. **本轮增量**：最终13项源／测试改动由上表覆盖；新生产构建与四项I1 E2E运行后，6,696运行／测试文件的SHA与冻结输入完全一致。87个sqlc生成文件也与已有无漂移验证快照一致；本轮无SQL修改，不重复生成。

完整基线不是最终小修后的又一次全套运行；相关回归有新证据，未受影响的检查直接复用。计数有重叠，不合计为独立场景数。Go若干未变包使用有效缓存，未宣称零缓存。

## 最终运行、环境及保留的失败

- 最终E2E：`.omx/logs/i1-verification/final-e2e-corrected.log`，**4 passed，0 skipped，0 retries，24.2s**；真实Electron、Web闭环、61任务历史过滤、交接后禁用。
- 环境：实际注册的 `check-20261006151843-35615`；独占测试库 `multica_check_20261006151843_35615_api`，API18574/Web13494。使用生产构建与agent CLI guard，未运行daemon或真实agent账户。
- [源码证据](source-provenance.json)保存实际listener PID、build ID、源与配置指纹，以及最终runtime记录；已确认所有生产rewrites指向18574。
- 本轮启动的API/Web已停止，E2E fixture已清理；原测试库与profile保留供检查。新的迁移演练库已删除，不触及其他数据库。
- 首轮Web代理误指另一已停测试环境：Electron1过／Web3失败，保留 `final-e2e-environment-mismatch.log` 和截图。第二次构建正确拒绝期间的tracked规范文档变更；冻结后第三次稳定构建通过。两次恢复均未改产品源码或放宽测试。

## 迁移、兼容、性能和限制

- [迁移恢复证据](migration-recovery-evidence.md)：真实553失败留INVALID、无hook负控制42P07、重试后16索引valid/ready/live和17ledger，重跑catalog/时间戳不变；有数据down拒绝P0001、部分空down保留共享时区。原有每个down步骤、P1联合及并发writer保护由完整race覆盖。
- [移动兼容](mobile-final-compatibility.md)：旧格式请求保留字段、未知响应不伪造0、默认禁止无确认写；移动端完整编辑后置。未安装旧发行Desktop/Mobile/CLI二进制，未做移动真机／模拟器视觉与完整读屏验收。
- [性能](performance-evidence.json)：1,000项detail/preview/closure warm P95为57.992/324.154/1,099.838ms；51计划禁用459.041/957.511ms。原冻结阈值不变；普通四写者关联任务426.3/s、P95 12.10ms通过原门槛。不是生产SLA或冷盘／并发标签编辑负载证明。
- 原 `/tmp` CG和普通写入原日志已过期。CG由保留的当前完整race覆盖；普通写JSON内12组样本及阈值可复核。未伪称重新hash不存在的文件。
- [49个E2E跳过项](skipped-tests.md)均有测试身份及fixture条件，I1无跳过；密码模式、管理后台、provider、其他独立Desktop及编译CLI等跳过项仍未执行。
- 既有lint、pnpm、CSS优化及test-mock警告保留；全部相关检查零错误不等于没有警告。
- `iterations_i1` 默认仍为false且未暴露到通用frontend flags；仅本地测试provider/env开启。数据已产生后不能退回不记录I1事实的旧服务端。
- 原计划的远端CI／发布验证仍待单独授权的发布阶段；本轮不提交、不推送、不合并、不部署，不归档或宣告整个发布目标完成。

## 本地提交与已验证源码的对应关系

2026-10-07按[本地提交handoff](commit-handoff.md)完成。基线 `115b4cd28`；上一Codex会话完成只读范围审计后因402配额中断、未暂存；本会话复核其审计并执行提交。结构化记录见父任务与本任务 `meta.local_commits`、[source-provenance.json](source-provenance.json) `local_commit_mapping`。

| 提交 | 意图 | 文件 | 复用的验证 |
| --- | --- | --- | --- |
| `823a61774` test(handler) | 位置/快速创建测试自建runtime与授权agent，不再借用共享agent | 2 | [FCG修复](fcg-backend-fixture-regressions.md)记录的窄race 2.431s与handler全量race 199.120s（其 `/tmp` 原始日志已过期）；完整基线handler PASS 202.745s |
| `7cf0a1e6b` test(projects) | 风险分页fixture按游标ID顺序分配父子ID | 1 | `pagination-regression.log` 仅保留单次PASS 2.926s；race `-count=20` exit 0 只见于原开发会话 `01a1119c` 的命令记录；完整基线 |
| `2055730f3` feat(iterations) | 关闭事务、outbox与撤权、历史priority/labels与标签写fence、迁移恢复测试；含DBTX `SendBatch` 的12个测试fake | 62 | 完整基线Go race/build/vet；最终history 15/21与迁移恢复2/4 race |
| `59b20ed57` test(e2e) | P1两个spec显式进入Issues分区，断言不变 | 2 | 完整基线E2E 249通过／49跳过 |
| `e516b15c2` feat(iterations) | 共享core/views、Web/Desktop接线、Mobile通知兼容、I1 E2E与Electron WS代理 | 106 | 完整基线TS/静态/E2E；[UI审查](ui-final-audit.md)views100；Mobile196（[移动兼容](mobile-final-compatibility.md)记录；保留的原始 `mobile-final.log` 是新增8项前的188）；最终生产Web/Electron 4通过 |

源码对应：`e516b15c2` 的树与 `.omx/logs/i1-verification/final-check-inputs.json` 6,696个文件中6,694个逐字节一致。另两项均非代码漂移：`apps/web/next-env.d.ts` 是排除提交的生产构建路由路径切换；`scripts/uninstall-desktop-windows.bat` 仅因 `eol=crlf` 检出转换与清单不同，其blob自基线未变。

本会话新增检查：两个仅含fixture的中间状态用 `go -overlay` 还原其余服务端改动后 `go vet` 退出码0（vet成功时日志为空；隐藏 `iteration_closure.go` 的负控制按预期失败）；已提交服务端树 `go vet ./...` 与56个Go文件 `gofmt -l` 通过；每批 `git diff --cached --check` 通过；13个复用日志SHA-256与记录一致。未重跑TS/Go/E2E套件，因为提交内容与已验收源码一致。除本节、任务记录和续接横幅外，对既有证据文件的唯一内容改动是删除 `handoff-2026-10-06-cg.md` 末尾多余空行，因此 `commit-handoff-snapshot.json` 中该文件的SHA `94ed90ac…` 是删除前的值。

保留未暂存：`.agents/skills/`（46）、`.codex/`（8）、`.trellis/.template-hashes.json`、`apps/web/next-env.d.ts`、`browser-evidence/history-metadata-final.png`（无引用或清单身份）。文档提交为 `d51810dcc`（原开发会话提交）。未推送、未建PR、未合并、未部署；父任务与verification仍in_progress，VG仍 `local_passed_remote_ci_pending`。

已提交说明的更正（不改写提交）：`2055730f3` 正文称SendBatch适配文件没有其他改动，但 `server/internal/service/iteration_lifecycle_test.go` 另加故障注入 `lifecycleFailBatch`，并把清理表扩到 `iteration_snapshot`/`iteration_notification`。`7cf0a1e6b` 的 `-count=20` 和 `e516b15c2` 的 Mobile 196 出处以上表为准。

## 文档与任务状态提交收尾

本次仅提交已核对的 I1 验收文档、任务状态、选定截图及相关 E2E 环境规范，共68个文件，位于五个代码提交之后。应用、服务端、测试源码均不在本次提交中。文档提交的自身 SHA 由 Git 历史提供，不在生成提交前预填“已提交”标识。

当前发布状态保持 `local_passed_remote_ci_pending`，父任务与 verification 不归档；已完成的实施子任务保留完成状态，并记录对应代码提交。旧文档中的未提交源码、旧 HEAD 和当轮测试结果均作为有日期的历史证据保留。

追加 E2E 会话 `01a11513-83b5-7de3-8adb-d81e252aec33` 正在独立复核，并报告 P1-L14 撤权请求循环问题；本次文档提交不替该会话签收、不改写之前测试运行的事实，也不代表所有后续测试已通过。最新修复和测试结果由该会话记录。

### P1-L14 撤权请求循环修复（`42a98ff05`，2026-10-07）

缺陷由上述 E2E 会话在 `e516b15c2` 生产 Web 上复现（3/3 超时；单次 trace 中 members/agents/squads/pins 各约5,460次404），诊断见该会话工作树 `/Volumes/artisan/code/2026/multica-e2e-20261007/.trellis/tasks/archive/2026-10/10-07-head-e516b15c2-e2e/research/p1-l14-diagnosis.md`（不在本仓库）。根因是 P1 既有代码：`clearProtectedProjectContent` 不幂等，拒绝后的每个本地错误都会再次移除整个工作区查询，仍挂载的观察者随即重建并重新请求。projects 访问代码自 `115b4cd28` 未变，不属 I1 回归。修复：同一已拒绝范围只清理一次，`resetProjectAccessSession` 仍清除拒绝状态。

验证：新增的 core（`access-lifecycle.test.ts`）与 views（`use-project-access-guard.test.tsx`）回归在去掉提前返回后失败（members 缓存被删；179次 members 请求），恢复后通过；core 2741、views 6177 项 vitest，core/views typecheck 与 eslint 通过。临时隔离 API（`218109daa+l14fix`，独立数据库，legacy，`FF_ITERATIONS_I1=true`）加生产 Web（`next build --webpack` + `next start`）：P1-L14 3.7s 通过，trace 共117条记录，四个工作区端点各一次404后停止；7个 projects-p1 spec 与 `workspace-access-branches` 共71项通过。环境已停止、数据库已删除，原始 trace/日志在 `/tmp/l14-e2e`，会过期。未运行：完整 E2E、Desktop Electron、经实时 WebSocket 送达的撤权、远端 CI。

`foundation/task.json` 的 `commit` 由 `e516b15c2` 改为 `2055730f3`：foundation 的共享SQL/router/sqlc整合位于 `2055730f3`，`e516b15c2` 不含 `server/` 文件。

本次核验包括文档 JSON 可解析、引用证据哈希、12张最终截图哈希、阶段状态一致性、文档提交白名单及 staged diff 检查。复用已保存的产品验证，不为纯文档提交重复运行产品测试。不推送、合并、部署或启用开关。
