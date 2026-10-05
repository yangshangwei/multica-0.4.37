# P1 Web / Electron 验收

状态：真实 Web 6/6、Electron 1/1 中间功能链路通过；三项 UI 红灯的修复已纳入候选。原 A 方案频繁重置促成 ADR-05 C，C 已经独立评审批准且实现冻结，风险断言已同步。等待父任务最终审查后的统一重建复测，未运行项目不计通过。

## 环境与方法

- 工作树：`/Volumes/artisan/code/2026/multica-projects-p1`，分支 `codex/projects-p1`。
- API：父代理管理的本机 `http://localhost:19071`，专库配置 `.omx/projects-p1-browser.env`。所有真实执行命令先导出该文件；不使用 Go 测试库。
- Web：必须 `next build` + `next start`；收到父代理 build / launch provenance 后执行。
- Electron：既有 `e2e/fixtures/changelog-electron.cjs` 加真实 preload、生产 renderer、router；独立临时 profile。固定只访问 localhost，无 daemon、真实 agent CLI 或外发。
- 数据：`TestApiClient` 经真实认证和 API 建工作空间；大集合、成员和 synthetic running task 使用参数化 SQL。每场景只删除自己建立的工作空间。
- 故障：描述 CAS 暂停一个真实 PUT，让另一个真实成员先保存；进展首次 POST 经 `route.fetch()` 真正提交，再 abort 原响应，重试同一 payload；概览 503 与 capability 回退是标记明确的浏览器单端故障注入。

## 测试映射

| 测试身份（`e2e/projects-p1.spec.ts`） | 主要覆盖 | 执行状态 |
| --- | --- | --- |
| `P1 goal template preserves content and a real member conflict keeps both descriptions reviewable` | AC-02/03/25：章节追加、不重复、原文保留、真实两人 CAS、显式保存复核稿 | 09:13 UTC 整组通过；待修复后最终复测 |
| `P1 formal totals split six completed and two cancelled; candidate acceptance adds one without execution` | AC-01/07/10/11：6/2/2、旧 done_count=8、五视图、候选排除、普通接受加一零执行 | 09:13 UTC 整组通过；待修复后最终复测 |
| `P1 risk drilldown uses ADR-05 live cursors, sticky disclosure and fresh restart while preserving personal filters` | AC-16、FR-15、ADR-05：156条fixture/初始155风险、子任务/个人筛选隔离；跨版本继续、sticky提示、空后缀151总数、失败保留/成功从头刷新 | 原 A 于09:13 UTC通过；新 C 待最终实跑 |
| `P1 publication retries a committed lost response once, preserves corrections, and versions acceptance` | AC-05/06/18/19/20/27：真实 mention 选择、预览无写、提交丢响应、同意图重放、一次站内通知、零agent执行、修订历史、旧版和当前版验收 | 09:13 UTC 整组通过；待修复后最终复测 |
| `P1 completion warning allows closure without changing issues; member deletion is refused and owner deletion preserves work` | AC-21/22/23/24：完成警告、项目多状态不改issue/running执行、成员拒删、owner删除保留issue/执行 | 09:13 UTC 整组通过；待修复后最终复测 |
| `P1 capability and overview failures recover; Chinese compact overview supports keyboard without overflow` | FR-09/13/14：真实读取上的503重试、模拟cap禁写不丢description、中文390px键盘/溢出 | 09:13 UTC 整组通过；待修复后最终复测 |
| `P1 real Electron preload and shared router publish, drill down, and retain Chinese compact history without a daemon`（desktop spec） | PRJ-014：原生router、风险、发布/历史、中文680px、daemonStarts=0 | 2026-10-05 09:25 UTC 通过，待最终构建复测 |

说明：AC-22/23 的未来迭代分支仍由 I1 接续。身份／证据授权全矩阵、迁移和性能由对应 canonical 后端测试及父任务报告提供；本表不能据此声称完整 27 项均已通过。

## 已执行检查

- `pnpm exec playwright test e2e/projects-p1.spec.ts e2e/projects-p1-desktop.spec.ts --list`：7 条测试发现成功；仅收集，非验收通过。
- `pnpm exec tsc --noEmit --strict --skipLibCheck --target ES2022 --module ESNext --moduleResolution bundler --esModuleInterop e2e/projects-p1.spec.ts e2e/projects-p1-desktop.spec.ts e2e/fixtures/project-p1.ts`：通过。

## 真实执行记录

### 已执行环境

生产 Web / API provenance 来自 `/Users/artisan/.multica/dev/envs/projects-p1/{web,api}.running.json`：

- Web commit `bb1c20dcb`，production，`next start --port 13991`，build ID `zDVE9tJGqlMUNz9aCFdxy`，listener PID 7686。
- API commit `bb1c20dcb`，`go run -ldflags '-X main.commit=bb1c20dcb' ./cmd/server`，listener PID 7162，`/health`返回相同 PID 与 commit。
- 两服务 source fingerprint `5dd65e9bbe9887d792e93ef488cde8c4299657594b6b6177cbce22650152f197`。
- Electron 采用 `1a6e0eff4` 源码，`pnpm --filter @multica/desktop exec electron-vite build` 成功，日志 `.omx/projects-p1-desktop-build.log`。尚未打包安装文件；测试实际启动 Electron 可执行文件和既有隔离 native fixture。

所有运行先执行：

```sh
set -a
source .omx/projects-p1-browser.env
set +a
PLAYWRIGHT_BASE_URL=http://localhost:13991 pnpm exec playwright test e2e/projects-p1.spec.ts --workers=1 --retries=0 --reporter=list,json --output=.omx/projects-p1-web-final
```

| 日志 | 真实执行结果 | 解释 |
| --- | --- | --- |
| `.omx/projects-p1-browser-run.log` | 2通过 / 4失败，325.3秒 | 初始脚本问题：描述误选折叠chat编辑器、inline验收文本exact定位、WS先重算、mention候选含头像缩写；均保留原始失败 |
| `.omx/projects-p1-browser-run2.log` | 2通过 / 2失败，31.5秒 | 模板/CAS、完整发布链通过；五视图路由reload抢跑、风险第二页Query缓存导致预期无新请求 |
| `.omx/projects-p1-browser-run3.log` | 2通过 / 0失败，13.3秒 | 显式等待路由；分页在新页面请求cursor后才改变DB输入，验证version刷新/重置 |
| `.omx/projects-p1-web-final.log` | **6通过 / 0失败 / 0跳过 / 0flaky，33.0秒** | 2026-10-05 09:13:32 UTC；当时完整功能链路整组通过 |
| `.omx/projects-p1-visual-red.log` | 1通过 / 1失败 | 新增描述动作完全可见断言；旧build的保存按钮只有77.48%可见；中文390px键盘编辑/预览通过 |
| `.omx/projects-p1-desktop-run1.log` | **1通过 / 0失败 / 0跳过 / 0flaky，7.3秒** | 2026-10-05 09:25:49 UTC；真实Electron启动，pageerror空、daemonStarts=0 |
| `.omx/projects-p1-footer-red.log` | 1通过 | 仅中心hit-test不足以捕捉浮钮部分遮挡，不能用于整按钮可访问结论 |
| `.omx/projects-p1-footer-red2.log` | 1失败 | 9点hit-test覆盖按钮25/50/75%位置，确认规划时区保存按钮被固定chat浮钮部分遮挡 |
| `.omx/projects-p1-risk-date-red.log` | 1失败 | 风险scope内未展示真实统计的reference_date；对应design §5的基准日展示要求 |

### 截图反馈与待复测

1. 描述冲突默认320px右栏动作被裁切。UI owner修复 `1a6e0eff4`，新增两按钮 `toBeInViewport({ratio:1})` 回归。旧截图：`.omx/projects-p1-web-final/projects-p1-P1-goal-templa-8ecad-oth-descriptions-reviewable-chromium/description-conflict.png`。
2. 390px概览滚动底部规划时区保存按钮被固定chat浮钮覆盖。已交 UI owner；`.omx/projects-p1-footer-red2/.../failure.png` 和原始日志记录红灯。
3. 风险页缺统计基准日／时区。已交 UI owner；新增对真实overview的reference_date/timezone文本断言，红灯日志 `.omx/projects-p1-risk-date-red.log`。

功能截图覆盖六二二统计、原五视图、风险版本重置、修订／验收、完成提醒、中文窄屏预览；最终可访问性结论必须绑定修复后重建，不能沿用初轮功能通过。

## 双客户端收敛性能（独立于业务验收计数）

父任务追加的 `P1 convergence performance: two real pages observe 30 due-date, assignee and admission changes within five seconds` 已创建并通过严格 TS / Playwright 收集检查，**尚未实跑**。

- 两个真实 Page 同时打开同一项目概览；独立工作空间、一个 member 指派任务、十个预先建立的 pending 候选。
- 截止日期、member/null 指派、普通 T1 接受各十次真实 HTTP 修改。每次起点在提交前，终点为各页真实 DOM 数字变化且观测到同一新 overview version；样本取较慢 Page 的耗时。
- `convergence-samples.json` 保存每次起止时间、变更类别、before/after counts 与版本、两个 Page 独立耗时、较慢值；按三类及总体分别给出 nearest-rank P50/P95/max。
- 每类及总体 P95 必须不超过 5000ms，页面异常为空，所有关联任务执行计数为零。每类十样本的 nearest-rank P95 等于该类最大值。
- 最终执行会使用 UI FR 批次后的 production 构建。健康服务纯 API/SQL/30分钟活跃分页负载由 health owner 独立执行，不冒充两客户端 DOM 收敛。

## 当前候选边界

父任务通知：原 A 分页在1次/秒变更下，第二页成功率42.8%、P95等待7秒、最长连续32次回首页，需先完成 ADR 复审。候选 C 计划让携带last_id的跨版本游标继续，当前页仍来自当前RR，不累计旧页冒充当前快照，并允许显式从头刷新。父任务随后明确 ADR-05 C 已获架构和 Critic 批准；本测试已按新契约改写，**尚未实际验收新候选**。原 A 测试全文保留在 `.omx/projects-p1-risk-a-test.txt`，旧运行与性能饥饿结果不改写。

`4336abc99` Desktop产物仅为中间候选，入口校验值在 `.omx/projects-p1-desktop-final-provenance.json`；文件名中的final不构成最终候选证明。

最终发布链还新增待跑断言：作者从已发布revision点击synthetic completed execution，授权GET返回正确task并打开真实TranscriptDialog；独立成员会话从实际inbox通知点击Open project update到对应进展，不能打开作者私有执行（UI无按钮、真实GET403）。它们不计入上述中间通过结果。

验收审计后的补强也尚待最终运行：同一两人描述场景再制造一次真实409，选择Use server version后旧本地文本不得覆盖新值；正式统计场景补混合终态100%和全取消100%的实际页面，两处均要求两个“无验收记录”摘要且项目仍planned。

health owner 已交付 C 的独立三档30分钟性能复测：1709/1709非空后续页严格前进，各档第二页成功30/30、31/31、31/31，错误／回退为零。该结论属于 `performance-c-report.md` 的服务端证据；本文件的浏览器及双Page收敛仍须最终统一构建后实跑，不能相互替代。

## 最终验证入口（等待结果）

父任务最新决定用完整 `scripts/check.sh` 执行两份P1 spec，总计7条业务验收+1条30样本性能测试。该脚本生成独立Go/API数据库、执行全仓检查并启动production Web；最终运行使用check导出的配置，取代早期手工`browser.env`配置。最终API/Web端口、commit、build ID和结果必须从check实际日志与provenance填写。

Desktop已在最终产品源码 `97e3f9124`（随后HEAD `61ba556e4` 仅诊断文档变化）下构建成功，日志 `.omx/projects-p1-desktop-build-97e3.log`，renderer入口`index-CTWCZZ1D.js`。不另启动API/Web，不在构建期间提交测试／报告。
