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
