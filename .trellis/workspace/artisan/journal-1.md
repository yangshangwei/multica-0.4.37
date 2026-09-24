# Journal - artisan (Part 1)

> AI development session journal
> Started: 2026-09-01

---



## Session 1: Prepare agent autonomy changes for main

**Date**: 2026-09-06
**Task**: Prepare agent autonomy changes for main
**Branch**: `fix/agent-autonomy-gates`

### Summary

Repaired the seven merge-review findings with regression-first changes, preserved ungraded-agent compatibility, and obtained independent approval after final local verification.

### Main Changes

- Reuse transaction connections for squad ACL checks and merge batch update field presence deterministically.
- Validate the final staged snapshot, preserve existing marker examples, and retain work on conflicts or inspection failure.
- Use complete API teardown, real Coordinator ceiling coverage, and accurate autonomy guidance.

### Git Commits

| Hash | Message |
|------|---------|
| `665a2c05f` | (see git log) |
| `2560b3b1d` | (see git log) |
| `6e4256ae3` | (see git log) |
| `066665951` | (see git log) |
| `4859dadf5` | (see git log) |

### Testing

- [OK] 64 Go race packages passed; four affected packages passed again after the final EOF correction.
- [OK] 25 API cases passed on the rebuilt isolated backend with zero orphan rows; TypeScript, lint, Go build/vet and independent reviews passed.

### Status

[OK] **Completed**

### Next Steps

- The branch is ready to merge into local main; no merge or remote push has been performed.

---

## 2026-09-06: 关闭审计遗留的三项边界问题（09-06-audit-open-findings）

### What was done

三条独立修复线，各自 RED→GREEN 后单独提交（分支 `fix/audit-open-findings`）：

- `a61c55d73 fix(server): refuse to mint human credentials for machine actors`
  — `/api/cli-token` 与 `/api/tokens` 全组加 `RequireHumanActor` 路由门禁，
  `IssueCliToken`/`CreatePersonalAccessToken` 加 403 backstop；SKILL.md 与
  source-map 同步。
- `dfd2cb06b fix(autopilot): make agent-created triggers act as the authorizing human`
  — `requireAutomationPrincipal` 沿委托链解析任务 originator 作为触发器
  `created_by`；无人类发起人 403。
- `e6f34a1a7 fix(core): ignore a 401 that answers a request from a superseded session`
  — `ApiClient.authEpoch`：三条请求路径捕获 epoch，过期 401 无副作用。

### Verification

- 全量 Go（cmd/server + internal/handler + internal/service，fix_open_findings
  DB 克隆）：4528 个测试 0 失败，-v 确认新测试名在运行。
- core：typecheck / lint（仅 2 个既有 warning）/ vitest 1748 全过。
- 审计 7 探针复验：正向探针（automation principal、stale session、execenv
  continuity）全部通过；反转探针（credentials laundering、autonomy routes、
  template access reuse）全部按预期失败（漏洞已关闭）。
- gofmt / go vet 干净；临时数据库 `fix_open_findings` 已 DROP，探针副本已清除。

### Learnings

- 沉淀到 `.trellis/spec/server/security-boundaries.md`（两条 server 契约）与
  `.trellis/spec/core/frontend/api-client-auth-epoch.md`（authEpoch 模式）。
- `pnpm --filter @multica/core exec ...` 在嵌套于主检出的 worktree 里会解析到
  主检出的包——要用 worktree 本地 `./node_modules/.bin/` 直接调用。


## Session 2: 归档内网设备免登录

**Date**: 2026-09-09
**Task**: 归档内网设备免登录
**Branch**: `feat/docs-in-app`

### Summary

用户确认真实桌面端、Web、身份持久性、双设备协作、看板及 Helm 验收均已测试通过；更新验收记录并归档 09-01-intranet-deviceless-auth。

### Git Commits

| Hash | Message |
|------|---------|
| `fb7b6bd64` | (see git log) |
| `f98bc0282` | (see git log) |
| `af0e9018f` | (see git log) |
| `6abc07db2` | (see git log) |
| `a82ad43ed` | (see git log) |
| `404860c19` | (see git log) |
| `1052e3586` | (see git log) |
| `670608882` | (see git log) |
| `7d10feef0` | (see git log) |

### Status

[OK] **Completed**


## Session 3: 归档内置研发智能体第一阶段

**Date**: 2026-09-09
**Task**: 归档内置研发智能体第一阶段
**Branch**: `feat/docs-in-app`

### Summary

确认 09-05-builtin-dev-agents 的实现与后续自治门禁加固均已合入 main，验收项全部完成，归档任务。

### Main Changes

- 归档 09-05-builtin-dev-agents：8 个角色模板、2 个 Squad 模板、四级自治与人工审批边界
- 核对 feat/builtin-dev-agents 相对 main 落后 16、领先 0，功能与加固提交均已是 main 祖先

### Git Commits

| Hash | Message |
|------|---------|
| `f495f2716` | (see git log) |
| `ca3a79d18` | (see git log) |
| `d75e8642b` | (see git log) |
| `a7b503ee0` | (see git log) |
| `f31d33fa2` | (see git log) |
| `652cd60b9` | (see git log) |
| `68be48e1d` | (see git log) |
| `caa8b88c1` | (see git log) |
| `661929c7b` | (see git log) |
| `ed26fd842` | (see git log) |
| `886be2898` | (see git log) |
| `665a2c05f` | (see git log) |
| `2560b3b1d` | (see git log) |

### Testing

- [OK] 已有证据：64 包 Go race 通过、pnpm test 417 文件 4996 项、模板创建 E2E 连续两次通过

### Status

[OK] **Completed**

### Next Steps

- 后续范围：自治等级设置页控件、审批实时事件、模板灰度与升级差异、per-role runtime


## Session 4: 应用内文档 P0 完成归档

**Date**: 2026-09-10
**Task**: 应用内文档 P0 完成归档
**Branch**: `main`

### Summary

确认应用内文档 P0 的 L1-L5 六个提交全部落在 main，任务标记 completed，补写 implement.md，推送 origin/main。

### Main Changes

- L1 内容管道：jsx-to-directive + parse-page 生成器，bundle 提交进仓库，CI 校验新鲜度
- L2 后端：go:embed 三端点（manifest/page/assets），manifest 白名单防越权，CLI 二进制不增重
- L3 core：zod+parseWithFallback 解析、TanStack Query、docsHref() 唯一寻址、锚点 parity 测试
- L4 渲染：packages/views/docs/ 自有组件树，图片 src 重写，内链走 navigation，UI 文案四语
- L5 入口：9 处硬编码 multica.ai/docs 收敛到 docsHref()，help-launcher 重指，智能体 WebFetch/issue URL 改本部署

### Git Commits

| Hash | Message |
|------|---------|
| `61c73d738` | (see git log) |
| `0ef6b1dfa` | (see git log) |
| `0c96d792b` | (see git log) |
| `af68f1075` | (see git log) |
| `91ea4362a` | (see git log) |
| `3a8f1050d` | (see git log) |

### Testing

- [OK] 各层测试随提交落地（生成器矩阵、Go 端点矩阵、core malformed-response、views 渲染 216 行）
- [OK] 收尾会话未重跑全量 pipeline，以各层提交时验证记录为准

### Status

[OK] **Completed**


## Session 5: 内网部署文档裁剪与验收

**Date**: 2026-09-10
**Task**: 内网部署文档裁剪与验收
**Branch**: `main`

### Summary

为内网部署裁剪内嵌中文文档：删除 9 个公网依赖页面（云快速上手、四个 IM bot、channels、GitHub 云集成、community-maintained、mobile-app），改写 4 个获取通道依赖公网的页面（self-host-quickstart、install-agent-runtime、tutorial、desktop-app），局部裁剪 7 页并清理产品侧深链（DOCS_SLUGS、设置页 bot Tab、help-launcher 公网菜单项、引导模板 curl 命令）与 6 个死 locale key；验收时发现并修复 CodeBlock 缺 use client 导致 web 文档页 500 的既有 bug，并将 project-resources/projects/issues 三页的 GitHub 默认措辞中性化。产物：bundle 36 页、pnpm test 5022 用例全绿、Go docs/handler 测试通过。新 spec：.trellis/spec/docs/frontend/docs-bundle.md。

### Git Commits

| Hash | Message |
|------|---------|
| `b59aa6d61` | (see git log) |
| `ecae52ac7` | (see git log) |
| `e3ca9cba6` | (see git log) |
| `e70228ba9` | (see git log) |

### Status

[OK] **Completed**


## Session 6: Autopilot 模板 review 后续修复

**Date**: 2026-09-11
**Task**: Autopilot 模板 review 后续修复
**Branch**: `main`

### Summary

完成 autopilot 模板功能的 review 后续修复并归档任务:四个 create_issue 模板补 {{date}} issue_title_template(核心:预建 issue 标题回落 autopilot.title 导致每期同名);AutopilotResponse 暴露 template_key/template_version 溯源;中文文案对齐术语表(仓库健康/每小时队列巡检);无效 ?template= deep-link 显示 not_found 提示;删除 AutopilotDialog 死代码 initial/initialSchedule;SKILL.md/source-map 补模板端点;新增 .trellis/spec/server/builtin-templates.md 契约沉淀。trellis-check 全量复跑:go build/vet、DB-backed handler 测试、typecheck、vitest、locale parity、lint 全绿。

### Git Commits

| Hash | Message |
|------|---------|
| `779dbda3a` | (see git log) |
| `2ecf6c5a7` | (see git log) |
| `05dc0edf4` | (see git log) |

### Status

[OK] **Completed**


## Session 7: Built-in role skill Chinese presentation

**Date**: 2026-09-12
**Task**: Built-in role skill Chinese presentation
**Branch**: `feat/agent-skills-localization`

### Summary

Localized seven built-in role skill titles and purposes across shared UI with bilingual search, stable identifiers, preserved user content, and verified single-locale behavior.

### Main Changes

- Added a shared provenance-aware presentation resolver and integrated lists, details, agent views, selectors, and tab titles.
- Kept slash suggestions responsive offline and ranked English and Chinese names consistently.

### Git Commits

| Hash | Message |
|------|---------|
| `a99cfe687` | (see git log) |

### Testing

- [OK] 5081 views tests passed; full repository typecheck passed; views lint had zero errors.
- [OK] 13 actual-component browser checks passed with a 94/100 visual verdict; pre-existing lint, Knip, and narrow-grid findings are documented.

### Status

[OK] **Completed**

### Next Steps

- Implementation complete; task archived with verification evidence.


## Session 8: Legacy role skill description localization

**Date**: 2026-09-12
**Task**: Legacy role skill description localization
**Branch**: `feat/agent-skills-localization`

### Summary

Fixed the two historical release and ADR defaults that remained English in existing workspace copies.

### Main Changes

- Matched exact version 1 descriptions to accurate Chinese copy while retaining custom edits and stored instructions.

### Git Commits

| Hash | Message |
|------|---------|
| `548788037` | (see git log) |

### Testing

- [OK] 5086 views tests passed; typecheck and lint passed with unchanged baseline warnings.
- [OK] 13 legacy and 13 latest browser checks passed; final visual verdict 96/100.

### Status

[OK] **Completed**

### Next Steps

- Correction complete and archived.


## Session 9: Chinese role skill bodies with explicit workspace updates

**Date**: 2026-09-12
**Task**: Chinese role skill bodies with explicit workspace updates
**Branch**: `feat/agent-skills-localization`

### Summary

Localized seven source skill bodies and updated all seven matching 海卫三 workspace copies; preserved frontmatter, historical v1 behavior, metadata and 16 bindings.

### Main Changes

- Preserved distinct v1 release/ADR semantics while translating current v2 source templates.
- Replaced the English-heading materialization assertion with exact source equality; recorded update and rollback evidence.

### Git Commits

| Hash | Message |
|------|---------|
| `c9d6cae6b` | (see git log) |
| `311bbf434` | (see git log) |

### Testing

- [OK] 55 Go top-level tests and 21 subtests; 66 frontend tests; go vet; cached typecheck and lint; independent review of 9 bodies and 8 scenarios.
- [OK] Read back all 7 workspace updates and verified metadata, files, labels and 16 bindings.

### Status

[OK] **Completed**


## Session 10: Template-based skill creation with protected drafts

**Date**: 2026-09-12
**Task**: Template-based skill creation with protected drafts
**Branch**: `feat/agent-skills-localization`

### Summary

Implemented the fifth skill creation method with seven built-in templates, local editing, independent copies, metadata preservation and bounded uncertain-result recovery.

### Main Changes

- Added read-only catalog; reused ordinary creation and shared cache/navigation; preserved template identity and YAML types.
- Kept unresolved submissions independent from edit mode and protected newer edits when opening recovered results; fixed narrow keyboard focus.

### Git Commits

| Hash | Message |
|------|---------|
| `0008e8665` | (see git log) |
| `078c73ca4` | (see git log) |

### Testing

- [OK] Core 1855 and views 5114 tests passed; final Go 30 test events without skips; earlier 223 backend regressions with documented optional skips.
- [OK] Real Web creation/navigation, Chinese desktop/narrow screenshots and keyboard assertions passed; visual verdict 96; typecheck/lint/go vet passed.

### Status

[OK] **Completed**


## Session 11: Ground skill output revisions and preserve deferred Codex acceptance

**Date**: 2026-09-12
**Task**: Ground skill output revisions and preserve deferred Codex acceptance
**Branch**: `feat/agent-skills-localization`

### Summary

Completed source-backed ADR guidance, CSV reference guidance and final-file documentation reporting; synchronized workspace 毕宿五. Local checks passed. User deferred real regression to a later Codex run.

### Main Changes

- Revised three skill bodies, versioned behavior changes and bundled the CSV reference without changing discovery metadata.
- Added a reusable local evaluation harness with complete skill snapshots, literal search evidence and opt-in account access.

### Git Commits

| Hash | Message |
|------|---------|
| `3a08c3b19` | (see git log) |
| `391d16b43` | (see git log) |

### Testing

- [OK] Go template/catalog checks, go vet, views typecheck and 61 focused frontend tests passed.
- [OK] Offline harness: macOS 21 passed plus 1 filesystem skip; isolated Linux 22 passed with no skips.

### Status

[OK] **Completed**

### Next Steps

- On user resumption, add a native Codex adapter and compare baseline/revised skill outputs under the same model and tools with independent semantic review.
- Parent full validation remains in review for prior E2E failures and uncollectable cases.


## Session 12: 工作区内置能力与项目开箱即用

**Date**: 2026-09-12
**Task**: 工作区内置能力与项目开箱即用
**Branch**: `feat/workspace-defaults`

### Summary

完成项目自动配队、内置目录、默认任务分配和自动化入口；需求、设计、拆解及验收已归档。

### Main Changes

- 新增可重试、权限受控的项目小队配置，复用已有角色和 skill。
- 连接项目创建、内置目录、普通任务默认分配及自动化明确启用流程。

### Git Commits

| Hash | Message |
|------|---------|
| `af2977950` | (see git log) |
| `ef494ddd3` | (see git log) |

### Testing

- [OK] 8015 项 TS 测试覆盖、65 个 Go 包、相关 race/vet、类型检查和 lint 通过。
- [OK] 3 个 Playwright 场景及桌面/窄屏视觉检查通过；未运行真实模型。

### Status

[OK] **Completed**

### Next Steps

- 功能实施无待办；独立分支保留供审阅。


## Session 13: 内置自动化模板中文化与验收

**Date**: 2026-09-13
**Task**: 内置自动化模板中文化与验收
**Branch**: `main`

### Summary

9 个内置自动化模板已按业务编写为中文，中文日期标题、缺陷优先级映射及仅运行说明同步完成。真实创建、编辑、派发、领取和中文浏览器验收通过，Trellis 任务已归档。

### Main Changes

- 9 份执行正文、中文名称和摘要；4 个中文日期任务标题；缺陷分级使用合法优先级并明确权限边界。
- 复用既有创建与派发链路，补充真实数据库和浏览器回归，修复测试订阅及规则版本清理。

### Git Commits

| Hash | Message |
|------|---------|
| `01353260b` | (see git log) |
| `b2cd5408b` | (see git log) |

### Testing

- [OK] 8040 项 TypeScript 测试、最终 486 项自动化专项、66 个 Go 包 race、lint/typecheck/go vet 通过。
- [OK] 两条 Chromium 流程通过；1440/390 预览和生成任务页视觉验收 97/pass；临时工作区和数据库已清理。

### Status

[OK] **Completed**


## Session 14: Desktop changelog and automatic release history

**Date**: 2026-09-13
**Task**: Desktop changelog and automatic release history
**Branch**: `main`

### Summary

Implemented and archived the desktop Help reader, deployment feed, cumulative commit-driven publication, and offline installation.

### Main Changes

- Preserved unrelated working-tree changes through partial staging; staged-only backend tests passed.

### Git Commits

| Hash | Message |
|------|---------|
| `b0bec41a6` | (see git log) |
| `ca6d82dcb` | (see git log) |

### Testing

- [OK] 8109 TS tests, full Go suites/vet, 43 release cases, 3 browser/Electron cases, desktop/web builds and visual review 94/100 passed.

### Status

[OK] **Completed**

### Next Steps

- Production releases use .github/RELEASING.md; unrelated legacy plugin E2E collection still references removed buildSurfaceDocument.


## Session 15: Fork release tag line and desktop updateUrl

**Date**: 2026-09-14
**Task**: Fork release tag line and desktop updateUrl
**Branch**: `main`

### Summary

Diagnosed why every build stamped v0.4.37-N: imported upstream tags v0.4.38-v0.4.43 sat outside main's ancestry. Preserved them as upstream-v* refs, deleted the bare copies, set remote.upstream.tagOpt=--no-tags, and tagged 9a6816401 as v0.4.44 (not pushed). Added optional updateUrl to desktop.json so Desktop takes electron-updater generic-provider updates from an operator-controlled static directory; login-page save now preserves it. Documented the field in four doc locales and the fork tag rule plus intranet update layout in RELEASING.md. Desktop vitest 598 pass, typecheck and lint pass, independent review found zero defects.

### Git Commits

| Hash | Message |
|------|---------|
| `9a6816401` | (see git log) |

### Status

[OK] **Completed**


## Session 16: 进展报告员、日报模板与 v0.4.45 内网发布

**Date**: 2026-09-15
**Task**: 进展报告员、日报模板与 v0.4.45 内网发布
**Branch**: `feat/progress-reporter-daily`

### Summary

新增一名内置报告员与相邻日报模板；85项完整E2E及源代码检查通过，v0.4.45已发布，Linux升级与Windows原生安装及11项上传资产摘要均已验证。

### Main Changes

- 复用既有角色、skill和自动化调度；业务任务只读，本期报告提交审核后收尾。
- 修正内网镜像版本注入和持久化，保留原配置与数据。

### Git Commits

| Hash | Message |
|------|---------|
| `e2d3f8891` | (see git log) |
| `19eca1ebd` | (see git log) |
| `ff41da457` | (see git log) |
| `88522b444` | (see git log) |

### Testing

- [OK] 85/85全套Web和Electron E2E；8139项TypeScript测试；Go race、vet和漏洞扫描通过。
- [OK] Windows原生x64安装及CLI版本通过；从v0.4.42真实升级至v0.4.45并重建后数据、JWT和版本均保留。
- [OK] 11个交付资产的GitHub SHA256与本地成品一致；模型内容以两份Codex离线样例验证，Claude429未计通过。

### Status

[OK] **Completed**


## Session 17: 引导侧栏同步「进入项目」终点项

**Date**: 2026-09-15
**Task**: 引导侧栏同步「进入项目」终点项
**Branch**: `main`

### Summary

方案 A 落地：onboarding 侧栏（StepSidebar）在三个持久化步骤后新增「进入项目」展示型终点行（PROJECT_EXIT，永不点亮/不可点击），移动端进度条分段同步为 4；step_nav.project 文案补齐 en/zh-Hans/ja/ko 四语言；ONBOARDING_STEP_ORDER 与流程导航逻辑不动，step-order.ts 注释补充设计决策。新增 4 个 step-shell 测试；onboarding+locales 271 测试、pnpm typecheck、eslint 全部通过。桌面端 dev 已重启供人工验证。

### Git Commits

| Hash | Message |
|------|---------|
| `3a9755183` | (see git log) |

### Status

[OK] **Completed**


## Session 18: E2E 遗留失败分类与生产 web 运行规则

**Date**: 2026-09-16
**Task**: E2E 遗留失败分类与生产 web 运行规则
**Branch**: `main`

### Summary

把 09-12 全量验证遗留的 10 条 E2E 失败与 2 条收集阻塞分类完毕：产品 bug 为 0，10 项是陈旧 harness、2 项是 next dev 冷编译超时；规则写入 spec，任务归档。

### Main Changes

- 逐条判据落到 e2e-failure-classification.md：10 项陈旧 harness 已由 5 个 test-only 提交修好（bfaabf6f7、99b266707、4d2804fef、eb8baaf3f、0b3ae5e80），每个只碰 1 个 spec 文件、零产品代码。
- comments.spec.ts:25 与 navigation.spec.ts:12 归为运行环境：那次跑的是 next dev --webpack，冷编译实测 30.4–35.8s，挤爆 15/30/60s 三档预算；两个 spec 至今一行未改。
- 新增 .trellis/spec/web/frontend/e2e-run-environment.md，并在 web 索引与 guides 索引挂入口；含误判特征与「不得据此改定位器」的禁令。
- 任务 09-12-full-skill-validation 归档，notes 与 4 个 meta 键记录分类结论、分类文档、绿色证据与绿色提交。

### Git Commits

| Hash | Message |
|------|---------|
| `8ff63fb51` | (see git log) |
| `76ce84a39` | (see git log) |

### Testing

- [OK] 本次未重跑 E2E。全绿证据取自既有报告 .omx/reports/main-upstream-merge-20260913/e2e-summary.json：提交 23d771ca7，82 expected / 0 unexpected / 0 skipped / 0 flaky，运行于 next build + next start，10 条原失败与 2 条原阻塞全部 expected。
- [OK] 该证据不在当前 HEAD（ee759228e）上；日志记的 2026-09-15 85/85 未找到报告目录，只算旁证。

### Status

[OK] **Completed**

### Next Steps

- 在 HEAD 上重跑全套 E2E，消除证据与 HEAD 的差距。
- 交给 Codex 的 grounding 行为回归仍未执行，是原任务唯一未闭合的验收项。


## Session 19: 侧边栏分组与中文命名落地，设置页四分组与守护进程中文化收尾

**Date**: 2026-09-16
**Task**: 侧边栏分组与中文命名落地，设置页四分组与守护进程中文化收尾
**Branch**: `main`

### Summary

侧边栏按参考稿重组为「工作 / AI 团队」两组，统计与设置下沉到 footer（滚动区外）；帮助按钮左对齐，桌面版本号折入帮助菜单，desktopAppVersion() 判空防止版本加载失败时悬挂空分隔线。中文导航命名迭代后落定：小队→AI小队（全局 106 处，含 e2e 定位器、⌘K 关键词、术语表新增 Squad→AI小队）、Skills→技能库（skill 正文按术语表保留小写英文）。settings-nav-groups（设置页四分组 + 冗余入口收敛）由并行会话 4b 实现并 check 9/9 通过，本会话独立复验后归档；desktop-daemon-chinese（守护进程设置全面 i18n 化，行为零变化）原会话随重启丢失，本会话逐行核查后接力提交。过程事故一次：提交时误收并行会话暂存的删除，已当场 pathspec 重做修正，此后审查与提交不再同链。

### Git Commits

| Hash | Message |
|------|---------|
| `3b035dae4` | (see git log) |
| `bacf2e3e3` | (see git log) |
| `28aced756` | (see git log) |
| `fb35f9ab0` | (see git log) |
| `fdb65ecba` | (see git log) |
| `421cd5bad` | (see git log) |
| `756030e2a` | (see git log) |

### Status

[OK] **Completed**


## Session 20: V0.4.46 发布收尾：升级 smoke 修复并全量交付

**Date**: 2026-09-16
**Task**: V0.4.46 发布收尾：升级 smoke 修复并全量交付
**Branch**: `main`

### Summary

接续上会话中断的发布流程：确认提交推送与 tag/Release 已完成（v0.4.46 -> ee2fe8a3f，desktop-smoke 在 main 与 tag 上均绿）。修复升级 smoke 的三个 harness 缺陷后，真实 V0.4.45->V0.4.46 离线升级 12 项检查全过（JWT/任务/SQL/附件/数据卷/配置持久化，Rosetta amd64）；三个缺陷均为环境适配：containerd 存储 inspect --platform 远端解析幽灵镜像、docker load 不重指 tag（需手动切 amd64 pg tag，cleanup 自动恢复）、Python 3.14 Request.method 条件赋值。上传 17 个交付资产到 release（三个大文件逐个传），重新下载核对 16 条 SHA256SUMS 全部一致；产出 README-v0.4.46.zh-CN.md 与验证 JSON。PRD 八条验收逐条核对通过，任务归档。

### Git Commits

| Hash | Message |
|------|---------|
| `a0eddb811` | (see git log) |
| `7138920d0` | (see git log) |
| `12db2f808` | (see git log) |
| `b6be6ba1e` | (see git log) |
| `2dfd0c43c` | (see git log) |
| `5ec932f12` | (see git log) |
| `43f6fc10e` | (see git log) |
| `ee2fe8a3f` | (see git log) |

### Status

[OK] **Completed**


## Session 21: 仓库文案跟随实际连接的 Git 托管方，GitLab 个人授权方案暂缓落盘

**Date**: 2026-09-18
**Task**: 仓库文案跟随实际连接的 Git 托管方，GitLab 个人授权方案暂缓落盘
**Branch**: `main`

### Summary

自建 GitLab 部署里「设置 → 仓库」的「连接 GitHub」指向一个装不上的 GitHub App，本会话让仓库相关文案跟随工作区实际连接的托管方。三层落地：api 层把 GET /api/workspaces/:id/vcs/connections 从裸 fetch 改为 parseWithFallback + ListVCSConnectionsResponseSchema，畸形响应降级为空列表，未知 provider 仍保留该连接交由消费方兜底；core/vcs 新增 resolveRepoProvider / useRepoProvider，恰好连接一个品牌时采用其名称与实例派生的示例克隆 URL，未连接或多品牌混连退回中性「Git」，同品牌跨实例则保留品牌名但不拿单一 host 充当示例；views 层的仓库连接按钮、创建项目弹窗与项目资源区的文案和示例 URL 统一跟随 provider，没有 GitHub App 时按钮显示「连接 {{provider}}」并跳转「设置 → 集成」，GitHub 之外改用中性图标，四语言 key 同步补齐。另落盘 2026-09-13 讨论的私有 GitLab 个人授权与 MR 归属需求/设计两份文档，文档首行即标注「暂缓，未实施」，第 15 章写明恢复前需用户再次确认，不因文档存在或方案审查通过而自动启动实施。c313866bb 是 Session 19 收尾后补的零散提交（帮助菜单中文四字统一），此前未入账，在此一并补记。本会话为事后补记，下列测试结论引自各提交的验证记录，未在补记时复跑。

### Git Commits

| Hash | Message |
|------|---------|
| `c313866bb` | (see git log) |
| `34b382e77` | (see git log) |
| `f5ddbee12` | (see git log) |
| `7eb2f3155` | (see git log) |
| `d9f11b53a` | (see git log) |

### Testing

- [OK] repo-provider.test.ts 覆盖空输入、畸形响应、单品牌、跨实例与未知 provider 九种情形
- [OK] schema.test.ts 新增两条 malformed-response 用例
- [OK] repositories-tab.test.tsx 与 create-project.test.tsx 共 31 用例，含「无 GitHub App 时按钮显示 Connect GitLab 且跳转 integrations」
- [OK] 语言包 parity 184 用例、pnpm typecheck 9/9

### Status

[OK] **Completed**

### Next Steps

- 私有 GitLab 个人授权若要推进，先完成设计文档第 15 章的 P0 决策（GitLab 版本与 scope、A/B 部署路线、旧 owner/成员映射、授权有效期与对账时限），不直接进入实施


## Session 22: 内网可投放的 Skill 模板库

**Date**: 2026-09-19
**Task**: 内网可投放的 Skill 模板库
**Branch**: `feat/builtin-skill-presets`

### Summary

新增 embed + MULTICA_SKILL_TEMPLATE_DIR 挂载目录合并的 skill 模板来源（embed 赢冲突、拒符号链接/逃逸、大小上限、单条失败仅跳过），handler 改调 TaskService.SkillTemplates()，前端零改动即可列出；「从模板中修改」面板按 presentation.isBuiltin 分「平台内置/本部署提供」两组、带来源 ⓘ 与空状态安利；docker-compose 只读挂载 + offline-bundle 空目录 + SELF_HOSTING/self-host-quickstart(en+zh) 文档；spec 记录挂载目录契约与分组约定。docker 部署时宿主机 ./skill-templates ↔ 容器 /app/data/skill-templates。

### Git Commits

| Hash | Message |
|------|---------|
| `8af415532` | (see git log) |
| `0793c05a1` | (see git log) |
| `9fa79f149` | (see git log) |

### Status

[OK] **Completed**


## Session 23: 内置 MCP Server 预设模板目录

**Date**: 2026-09-19
**Task**: 内置 MCP Server 预设模板目录
**Branch**: `feat/builtin-skill-presets`

### Summary

设置→MCP 页新增服务端内嵌的内置模板目录：行内「添加」把配置预填进现有添加弹窗，保存后即普通只写工作区 MCP Server（仍需单独分配给智能体）。服务端 McpServerTemplates() 名册首批三条免密钥 stdio（chrome-devtools/playwright/sequential-thinking，context7 按用户要求移除），只读接口 GET /mcp-servers/templates 挂成员可见分组；名册测试守 key 格式/唯一/config 有效/四语齐备/无密钥五条。core 侧 zod schema（全字段 default+loose）+ 客户端 parseWithFallback + malformed-response 测试。views 复用 builtin-template-catalog（新增可选 className/defaultOpen，既有四处零变化），mcp-server-dialog 增可选 preset 预填仍是「添加」态，重名行禁用显示「已添加」。四语 i18n + 文档 source-map。trellis-check 全绿：typecheck/lint/core/views(隔离)/go service/go handler(mcp) 通过，AC1–AC6 全覆盖；mcp_template handler 用例只读不碰 DB 是真跑过。收口两处非本次问题：pnpm test 整包下 mcp-tab/skill-panel 3 个 flake 隔离复跑全绿；go handler 一批 project/squad DB 用例因本机开发库缺 execution_squad 列而假红（需克隆已迁移模板库跑）。未纳入提交的无关项：skills/template-skill-create-panel 的分组→tab 改造与 .dev-skill-templates/（属 skill 模板线的另一路工作）。

### Git Commits

| Hash | Message |
|------|---------|
| `34bdbd3d5` | (see git log) |

### Status

[OK] **Completed**


## Session 24: 技能库分类、图标与卡片视图（标签复用工作区标签）

**Date**: 2026-09-20
**Task**: 技能库分类、图标与卡片视图（标签复用工作区标签）
**Branch**: `main`

### Summary

技能库新增固定 6 分类 + Lucide 图标白名单，存于 skill.config.presentation（category / icon），Go 与 TS 常量对拍；8 个内置角色 skill 预设分类图标，导入时从 frontmatter metadata 种子填充。页面改为左侧分类栏（窄容器折叠横向 chips）+ 卡片 / 列表可切换（按行虚拟化，视图模式按工作区持久化），列表新增分类 / 标签列，工具栏新增分类与标签筛选，新建与详情页可编辑分类 / 图标，批量「设置分类」，分类空态预填新建。中途用户决定放弃自由文本 tags、改用现有工作区标签：删除 tags 校验与存储，GET /api/skills 用 ListLabelsForSkills 批量内嵌 labels，POST /api/skills 支持 label_ids 并在创建事务内挂接，ResourceLabelPicker 增加草稿模式，卡片 / 列表用 LabelChip 展示。trellis-check 全绿（typecheck / lint / core / views / go skill+service / DB-backed handler 克隆库），自修 5 处（卡片 kebab hover 组名、卡片无匹配空态、分类空态误判、视图切换 active 态同 hover、批量设置分类无权限文案）。浏览器目视验证：暗 / 亮色卡片与列表、标签筛选菜单、600px 容器 chips、卡片 hover 操作菜单均正常；pnpm test 整包 mcp-tab / skill-template flake 单跑通过。

### Git Commits

| Hash | Message |
|------|---------|
| `d017c292f` | (see git log) |
| `6f5817444` | (see git log) |
| `3c766dd12` | (see git log) |
| `a9d034dc6` | (see git log) |

### Status

[OK] **Completed**


## Session 25: skill-lifecycle-taxonomy：审计、视觉验收与收尾

**Date**: 2026-09-23
**Task**: skill-lifecycle-taxonomy：审计、视觉验收与收尾
**Branch**: `main`

### Summary

完成八分类生命周期任务 Step 7 与 Phase 3：独立 trellis-check 审计抓出并修复 zh-Hans/ja/ko 死 _one 复数键（parity 3 例失败，修复后 5365 全绿，主会话独立复跑确认）；修复侧栏过期 six-categories 注释；Playwright 宽/窄/暗色+中英文视觉验收 13 张截图，两项展示层发现（英文分类名截断、390px 批量栏溢出）留档未修待设计决策；skill-presentation spec 补上线顺序与批量标签契约；feat/docs/chore 三笔提交入库。

### Git Commits

| Hash | Message |
|------|---------|
| `28c91e13d` | (see git log) |
| `a54668d2f` | (see git log) |
| `864e95f59` | (see git log) |

### Status

[OK] **Completed**


## Session 26: 新增体验验证与迁移审查专项角色

**Date**: 2026-09-24
**Task**: 新增体验验证与迁移审查专项角色
**Branch**: `main`

### Summary

两周期 workload gate 达标后新增 experience-validation-engineer(Contributor)与 migration-reviewer(Observer)两个 listed 角色及其 role skill,接入 review-gate/release/maintenance 小队,更新 roster(14 角色/15 skill)、creating-agents 技能与源图、四语文档与 locale,并补齐 roster/autonomy/证据契约/小队路由测试。独立 trellis-check 通过 4 条验收标准;service 与模板测试绿,execution_squad 迁移缺失与 sweeper race 属无关环境失败。

### Git Commits

| Hash | Message |
|------|---------|
| `c21d03a43` | (see git log) |

### Status

[OK] **Completed**
