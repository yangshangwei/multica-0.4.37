# #37 验收记录：普通状态编辑在待响应期间保持即时显示

本项已通过独立审查和浏览器验收。普通事项从 UUID 或可读编号进入，PUT 待响应期间均保持 In Progress；旧 Todo 读取不能覆盖它；503 后回到 Todo，一次显式重试及刷新后均为 In Progress。受控完成继续等待授权及真实回执。

## 实际修改

- `packages/core/issues/mutations.ts`：只在普通 `onMutate` 中保存当前工作区、当前 UUID 的已加载详情，精确取消其进行中读取；仅当取消清空详情时恢复已保存的行，再交给原缓存协调器更新、回滚和调和。
- `packages/core/issues/mutations.test.tsx`：真实 QueryClient、`useCanonicalIssue`、详情 observer 和 `useUpdateIssue` 回归。覆盖两种入口的旧读覆盖、无重叠对照、其他事项/工作区读取、受控完成等待/拒绝，以及 UUID 读取先开始、编号 initialData 后到的时序。
- `e2e/issue-optimistic-status.spec.ts`：四个真实界面流程，保留即时按钮、关闭的选项、独立服务器回读、503 提示、一次真实重试、刷新及恰好两笔 PUT 的断言。延迟场景扣住成功冷启动之后、写入之前所有自然发生的 UUID GET，不强制读取或断言内部请求数量。
- Core 的 `state-management.md` 与索引记录精确取消和 late initialData 保留契约。

两个原 E2E 文件保持原字节；未改权限、完成协议、后端、状态目录、超时、自动重试或原断言。最终产品 SHA256 为 `d53b3b2d7c75510ae06625088625d0048b51a974d9ea6a927c3d624ac8f0fde0`；新增 E2E 为 `6d9f4002d716cd34f4eb7d93b24de29c5b5a059eeac0630922aa7019cd05c120`。

## 根因及修前证据

1. **旧详情结果覆盖。** 两次 Core RED 均证明，同一个 QueryClient、同一个 `issues / workspace / detail / UUID` 键在 PUT 待响应时已为 In Progress；放行更早的 Todo GET 后，缓存、canonical 读取和 observer 同时退回 Todo。两种入口失败，四项对照通过。
2. **浏览器锁定真正的读取。** 原标题本轮首次运行通过，并未重现历史竞态。最初仅扣住第一个匹配的 GET，后来查明同一端点也供 reactions 查询使用，因此没有把 UUID 的通过误记为详情读取已被取消。最终门控覆盖全部自然写前 GET，在保留的原生产构建上原样运行两次，UUID 与编号均在新 E2E 第 208 行稳定失败：GET 完成、已绘制、PUT 仍待响应，真实按钮为 Todo。
3. **取消不能丢掉已加载初值。** 首次一行精确取消虽通过 Core/Views 检查，浏览器只有 3/4 通过；编号延迟场景在放行 GET 前的第 192 行失败。实际生产 QueryCache 订阅记录了完整顺序：UUID 查询以空数据开始 fetch，编号解析晚到后用 initialData 放入 Todo；取消恢复空的 pre-fetch 状态，协调器没有详情行可更新；canonical observer 再补回 Todo。期间没有新的 GET 返回。
4. **补齐缺失的时序。** 使用真实 canonical hook 的 late-initialData 回归，在一行修复上连续两次于 `mutations.test.tsx:295` 失败。保留取消前已加载详情后转绿。独立库模型也证明，`revert:false` 会引入 CancelledError、查询错误和 invalidated 状态，因此保留默认取消语义。

历史 #37 trace 的线索是 PUT 于 `14:34:26.179` 发出，随后客户端才处理同一端点的 GET `14:34:26.204`；本轮以明确的读写键、真实缓存事件和真实按钮补足因果证据。未重新操作历史 trace。

## 环境与结果

唯一运行号：`20261119-002358-fc3f5ec4`。证据根目录：`.artifacts/e2e-fix-20261119-002358-fc3f5ec4/`。

源码快照位于普通目录 `/Volumes/artisan/code/2026/multica-e2e-fix-20261119-002358-fc3f5ec4`，包含起始工作树的所需未提交内容。起始 HEAD 为 `72c207f72d67af979e6f6222a44d397036f4c79b`。使用全新数据库 `e2e_fix_37_38_20261119_002358_fc3f5ec4`、独立 HOME/上传目录、API `54150`、生产 Web `54151`。`current_database()`、监听 PID、`/health`、`/readyz`、live legacy auth 及实际 CSS 与构建字节均已校验。

原构建 ID：`hmsXneolrRgzgdBEAYmhy`；一行修复尝试：`WhRBO4garQzuzdn4pGCHx`；最终保留详情的构建：`UkGXK1ed0UWW3CgSnEhGw`。前两个构建及失败证据均保留。

| 阶段 | 证据相对根目录 | 结果 |
| --- | --- | --- |
| 原标题修前运行 | `37/red/original` | exit 0，1 passed；不是 RED |
| 旧读 Core RED ×2 | `37/red/unit/run-01`、`run-02` | 均 exit 1，UUID/编号两项命中原机制 |
| 完整自然 GET 浏览器 RED ×2 | `37/red/browser-cohort-01`、`browser-cohort-02` | 均 exit 1，两入口均在第 208 行失败 |
| 一行修复的浏览器尝试 | `37/green/targeted-browser` | exit 1，3 passed / 1 failed；未计为验收 |
| 实际 QueryCache 诊断 | `37/diagnostic/fixed-identifier-early-cache/actual-query-cache.json` | 记录空 fetch → late Todo → cancel 空 → re-seed Todo |
| late-initialData Core RED ×2 | `37/red/unit/late-initial-data-01`、`late-initial-data-02` | 均 exit 1，第 295 行预期失败 |
| 最终包级回归 | `37/green/unit/implementation-summary-preserved-detail.json` | 定向 7/7、Core 四文件 80/80、Views 三文件 108/108 |
| 独立最终审查 | `37/check/final-review.json` | lint/typecheck 通过，无剩余发现 |
| 最终新增浏览器整文件 | `37/green-preserved/targeted-browser` | exit 0，4 passed、0 skip/flaky/retry；4 个成功 trace |
| 原失败标题 | `37/green-preserved/original` | exit 0，1 passed |
| 原完整七用例文件 | `37/green-preserved/original-full-file` | exit 0，7 passed，包括相邻优先级回滚 |

所有 Playwright 命令使用 Chromium、`--workers=1 --retries=0 --reporter=line,json --trace=on` 和全新结果目录。原文件自身的 `test.use({trace:"retain-on-failure"})` 使成功的原用例没有保存 trace；文件未为此修改，原命令、报告、构建及输入指纹保留。新增回归的 RED/GREEN trace 均存在。

两包 lint、typecheck 与 `git diff --check` 退出 0。Views 留有 27 条既有、未改文件中的 lint 警告；生产 CSS 优化有两条既有 highlight 警告，无构建错误。

## 范围与后续

本项不代表原 52 个失败标题整体通过，也不销号 #51、#52 或两条 S3 收集错误。本项没有改平台导航接线，相关共享逻辑及授权回归已运行。#38 完成后，在匹配两项最终源码的生产 Web 构建 `C2Kji8hfWmojMSLxyjsnu` 上已串行运行两份原 E2E 与两份冻结新 Web 回归，`joint/green/web-final` 为 14/14，0 skip/flaky/retry。#37 两种入口的即时显示、旧读隔离、503 回滚、重试及刷新均再次通过。

#38 同批真实 Electron 迭代相关两整文件及新增 A→B 回归合计 6/6；它们证明该路由/历史边界，没有替代 #37 的状态专项浏览器与 Core 证据。未运行全部 E2E、打包后的 Desktop main/安装包或真实智能体验收。联合结果与输入指纹见同根目录的 `joint/verification-summary.json`，本批两子任务已收妥独立最终审查并统一归档。

没有操作 `qa-retained-1118`、`KEEP-1`、旧报告/trace、导出或备份；只创建和控制本次私有进程与数据库。
