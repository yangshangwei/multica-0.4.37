# E2E 结果记录 — v0.4.47

## Round 1（全量，14.9m）
- 命令：`pnpm exec playwright test`（对本机 human dev env：web `:13492` / api `:18572`，commit `3e07a860c`）
- 结果：**78 passed / 10 failed / 3 skipped**，EXIT=1
- 日志：`/tmp/v0447-e2e.log`

### 失败清单与初判
| # | spec | 现象 | 初判 |
|---|------|------|------|
| 1 | auth.spec.ts:22 未登录跳转 /login | 实际跳到 `/onboarding`（不是 /login） | ⚠️ 可能真行为变更 — 指向 HEAD「Enable intranet CLI onboarding」 |
| 5 | onboarding-smoke.spec.ts:17 跳过 runtime 后到 /projects | 卡在 `/onboarding` | ⚠️ 同上，onboarding 流相关 |
| 4 | localized-template-defaults.spec.ts:132 建 squad | URL 未跳到 /squads/{id}（squad 未建成/未导航） | 疑 load flake / 状态 |
| 3 | issues.spec.ts:68 日期筛选 | reload 后新建 issue 不可见 | 疑 timing/load flake |
| 6 | progress-reporting.spec.ts:344 | 中文「每日进展报告」标题未出现 | 疑 timing/load flake |
| 2 | changelog.spec.ts:56 | 60s 超时 | 疑 load flake（内网 changelog） |
| 7 | settings-preferences.spec.ts:55 | — | 待看详情 |
| 8 | settings-preferences.spec.ts:103 | — | 待看详情 |
| 9 | upstream-selected-fixes.spec.ts:131 中文/emoji 评论 | — | 待看详情 |
| 10 | workspace-defaults.spec.ts:47 | 120s 超时，page closed | 疑 load/超时 |

### 关键关注点（发布门禁）
失败 #1、#5 都卡在 `/onboarding`，而 HEAD 提交正是「Enable intranet CLI onboarding without GitHub access」。这可能是：
1. 内网 onboarding 特性合法改变了「未登录/未完成 setup → /onboarding」的重定向，导致这两个测试**过时需更新**；或
2. 当前 dev 部署处于「onboarding 未完成」的**脏状态**（首启 setup 未做），全局-scope 断言因此失败；或
3. 真回归。

无论哪种，都属**发布级信号**，未查清前**不推 tag**（符合用户「先测后推」的门禁意图）。

## Round 2（--last-failed 隔离复跑，6.0m）
- 结果：**3 passed / 7 failed**。恢复：issues 日期筛选、progress-reporting、upstream 中文/emoji 评论 → 判为负载/交织 flake。

## Round 3（--last-failed 预热复跑，5.4m）
- 结果：**1 passed / 6 failed**。仅 settings-preferences:103 恢复。
- 关键观察：预热后失败**没有减少**，且耗时**变长**（workspace-defaults 40s→2.1m、settings:55 15s→30s）。指向运行中的 **dev server 处于降级状态**（`pnpm dev:web` 已连续运行 ~14h、118% CPU），而非「冷编译预热就能好」。

## 三轮后稳定失败集合（6）
1. auth.spec.ts:22 未登录跳 /login — 实渲染「Workspace not available / Sign in as a different user」页；用例取 `getWorkspaces()[0]` 依赖共享库状态
2. changelog.spec.ts:56 — 1.0m 超时
3. onboarding-smoke.spec.ts:17 — 卡 /onboarding（快照曾见 "Compiling..."）
4. workspace-defaults.spec.ts:47 — 2.1m 超时，「built-in」标题未出现（快照近空白）
5. localized-template-defaults.spec.ts:132 — 建 squad 后未导航到 /squads/{id}
6. settings-preferences.spec.ts:55 — Settings 的 Profile tab 未找到

## 定性判断
- 环境证据强：dev-server 长跑降级 + 冷编译（"Compiling..." 快照）+ 耗时随复跑增长 → 多数失败疑为**环境产物**。
- 但预热未清除 6 项 → 不能纯归因冷编译；需一个**干净环境**的单次结果才能可信定性。
- CI 侧 `release.yml` 的 verify（Go test + govulncheck）与本地 Playwright 相互独立。

## Round 4（重启 web + 预热后复跑，4.1m）
- 动作：外科式 kill 仅 web 进程树（3291→30339），保留 API/daemon/desktop；重启 `pnpm dev:web`（fresh，Ready 398ms）；curl 预热 /login /onboarding /issues /squads /settings /projects。
- 预热实测冷编译耗时：/settings 22.8s、/squads 15s、/issues 8.4s、首个 / 17.4s → 坐实「懒编译超时」。
- 结果：**1 passed（onboarding-smoke）/ 5 failed**。
- 剩 5 项：auth:22、changelog:56、workspace-defaults:47、localized-template:132、settings-preferences:55。

## 关键定性证据
- HEAD `3e07a860c` 是 self-host/onboarding 配置改动，**未触碰** auth 重定向/中间件/settings/squads/changelog 相关产品代码；auth-initializer 仅 +1 行 `cliInstallCommand`。
- `origin/main == HEAD`：这 5 个失败**已存在于已发布的 main**，打 v0.4.47 tag **不改任何代码**。
- changelog:56 是**设计上的中文用例**（设 zh-Hans、断言「变更说明」），非 locale bleed；它导航到**未预热的 /changelog** 路由，60s 内冷编译超时。
- auth:22：未登录访问 workspace 渲染了「Welcome/Continue on web」营销页而非 /login；用例取 `getWorkspaces()[0]` 依赖共享库。是否为 intended 行为变更由子代理核。

## 子代理定性（只读 trace + git-blame）
- 基石事实：HEAD `3e07a860c` == origin/main；其代码改动仅 CLI-install 配置管线（config/schemas/config.go/auth-initializer +1 行 `cliInstallCommand`）+ cli-install-instructions + connect-remote-dialog。**5 项被测产品源码都在 HEAD 前 14–233 个提交定型,与本 tag 无关。**
- #1 auth:22 = **stale-test**：main 在非云/localhost 默认 mint device session（device-mint 特性 233 提交前,`.env` 默认开）→ 未登录访问 workspace 渲染 onboarding welcome 页（快照有 "Log out" 按钮=已认证 device 会话），不再跳 /login。高置信。
- #2/#3/#4/#5：子代理判为 dev-server 冷编译超时，但**自陈未用一次绿跑验证过**。

## Round 5（预热 /changelog /squads/[id] /skills /agents 后复跑 5 项）
- 预热实测：/changelog 32.0s、/agents 42.5s、/squads/[id] 3.9s、/skills 0.9s。
- 结果：**1 passed（workspace-defaults）/ 4 failed**。auth、changelog、localized-template、settings-preferences 仍失败。
- **修正**：我先前预测「预热后 #2/#4/#5 会转绿」**过度乐观、被推翻**。子代理的冷编译分类对 #3 成立,对 #2/#4/#5 不成立。
- 复看 spec：#4/#5 失败在**客户端 SPA 导航**（squad-detail push、settings tab 点击）后的断言,curl 的是**服务端**路由,编译不到这些客户端 chunk；#5（settings-preferences）用 **randomUUID 独立 workspace**,排除脏库。

## Round 6（客户端路径真实走过后复跑 #2/#4/#5）
- 结果：**localized-template:132 转绿**（客户端 squad-detail chunk 编译后 push 成功）→ 坐实 #4 = 客户端 chunk 冷编译竞态。
- settings-preferences:55 仍败,但 **web 日志实证**：该用例把 /settings 各 tab 全部走通并返回 200（?tab=workspace/issue-statuses/repositories/preferences 均 200），失败断言在多轮间**漂移**（line58 toHaveAttribute → line68 toHaveURL）→ 客户端 SPA 导航断言竞态,产品功能正常,非回归。
- changelog:56 = 唯一每轮都固定 60s 整超时者。其流程：新建 workspace → goto /issues（等 帮助 最多 45s）→ 点 帮助 → 点 变更说明 → 等 /changelog（实测冷编译 32s）≤30s，全测 60s 上限。两段冷编译叠加 > 60s 测试上限 → 稳定撞顶。快照每轮都是正确渲染的中文 /issues 壳,未到 /changelog。属 dev-server 测试上限超时,非回归。**诚实说明：此项我未能取得一次绿跑。**

## 最终统计（原 10 失败）
- 恢复过至少一次（负载/交织或冷编译竞态）：issues 日期、progress-reporting、upstream 中文评论、onboarding-smoke、workspace-defaults、localized-template、settings-preferences（日志证功能正常）= 8 项。
- auth:22 = **stale-test**（device-session 行为变更,233 提交前特性,与 tag 无关）。
- changelog:56 = dev-server 60s 测试上限超时（未取得绿跑,但被测源码 92 提交前定型、快照渲染正确、与 tag 无关）。

## 发布判断：GO（无发布级回归）
无一项是 v0.4.47 要打 tag 的代码引入的回归。origin/main == HEAD；HEAD diff 仅 CLI-install 配置管线,与全部失败路径无关；被测源码 14–233 提交前定型。逐个 flake = 测试卫生债 + dev-server 懒编译,另案处理。等用户确认后推 tag。

## 结论（发布判断，独立于逐个 flake 归因）
无论 #2/#4/#5 最终归为冷编译还是别的测试/环境因素:**它们都不是 v0.4.47 要打 tag 的代码引入的回归**——origin/main 已等于 HEAD,HEAD diff 与这些路径无关,被测源码久未变更。CI `release.yml` 的 Go verify + govulncheck 是独立的发布门禁。逐个 flake 属测试卫生债,另行处理,不阻断本次发布判断。

## 打包与发布（已完成）
- 发布 CI run 35691775394 全绿：verify(Go+govulncheck)、changelog、docker backend/web(amd64+arm64)构建与合并、publish-changelog 均 success；上游专属 desktop/goreleaser/helm 跳过。
- GitHub Release **v0.4.47 已发布**（draft=false, prerelease=false）：https://github.com/yangshangwei/multica-0.4.37/releases/tag/v0.4.47
- dist/release/v0.4.47/：
  - linux-amd64/multica-server-upgrade-v0.4.47-linux-amd64.tar.gz（290,499,266 B，sha256 061b678a…），MANIFEST 版本 v0.4.47 / linux/amd64 / 镜像 multica-backend|web:v0.4.47。
  - windows-ia32/multica-desktop-0.4.47-windows-ia32.exe（165,405,175 B，sha256 1ab99b5a…）+ .blockmap + latest-ia32.yml。三处 PE 均 0x14c(32 位)，CLI windows/386。
  - SHA256SUMS-v0.4.47.txt（9 文件全部 shasum -c OK）、README-v0.4.47.zh-CN.md、linux-upgrade-verification.json、verification/windows-ia32/。
