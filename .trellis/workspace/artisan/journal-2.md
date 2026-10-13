# Journal - artisan (Part 2)

> Continuation from `journal-1.md` (archived at ~2000 lines)
> Started: 2026-10-12

---



## Session 63: Desktop connection recovery and native acceptance
<!-- trellis-session: v=2 fp=ede83f1031e835d0 -->

**Date**: 2026-10-12
**Task**: Desktop connection recovery and native acceptance
**Branch**: `codex/cross-workspace-collaboration`

### Summary

Implemented bounded boot requests, live recovery metadata, shared workspace recovery and persistent server editing with authorized child-window handoff. Fixed language-sync reload losing unsaved input by deferring desktop locale refresh until Back. Verified 2030 related tests, affected lint/typechecks/build, independent review, six isolated macOS Electron groups and visual score 93. Archived the completed task; unrelated planning and admin archive changes remain separate.

### Git Commits

| Hash | Message |
|------|---------|
| `0c6afec4c` | fix(desktop): keep recovery usable while the server is unavailable |

### Status

[OK] **Completed**


## Session 65: 丰富当前后端与桌面业务测试数据
<!-- trellis-session: v=2 fp=2329d15c9880a9f5 -->

**Date**: 2026-10-12
**Task**: 丰富当前后端与桌面业务测试数据
**Branch**: `codex/cross-workspace-collaboration`

### Summary

当前 desktop-qa-1008-1743 新增44条任务及5项目、计划迭代、标签/评论/通知/聊天/合成执行。原有15任务与项目/迭代/评论/设置逐条保留。幂等重跑无新增，13安全测试、31真实API、9原生Electron页面、lint/typecheck通过。模拟运行时离线，验证临时数据库已清理。

### Main Changes

- 新增make seed与pnpm seed/test:seed入口，无新依赖

### Git Commits

| Hash | Message |
|------|---------|
| `ede5ae202` | test(seed): retain realistic scenarios for desktop business acceptance |

### Testing

- [OK] 13/13隔离schema安全测试；31 API；9 Electron页面；lint/typecheck

### Status

[OK] **Completed**


## Session 66: MCP agent category filters
<!-- trellis-session: v=2 fp=2cc1f4923c4f3dc0 -->

**Date**: 2026-10-12
**Task**: MCP agent category filters
**Branch**: `codex/cross-workspace-collaboration`

### Summary

Implemented and verified category filtering in the shared MCP agent assignment picker. Category chips use explicit saved categories and existing template provenance, preserve hidden selections across category/search projections, and keep All/explicit categories usable while metadata recovers.

### Main Changes

- Added represented category chips and full-directory counts to the shared MCP picker.
- Gated existing role/squad template catalog queries to eligible template-backed rows and preserved retry/cache behavior.
- Recorded bilingual copy, focused regressions, production browser evidence, and the MCP spec contract.

### Git Commits

| Hash | Message |
|------|---------|
| `6642de425` | Make MCP assignment choices easier to classify |

### Testing

- [OK] pnpm --filter @multica/views test: 546 files, 6694 tests passed.
- [OK] pnpm typecheck and pnpm lint passed with existing warnings only.
- [OK] Production component fixture passed five responsive locale/theme scenarios with zero browser errors.

### Status

[OK] **Completed**

### Next Steps

- Knip still reports the repository baseline unused files/dependencies; clean it in a separate task.


## Session 67: 内网静态下载页与首次安装指导
<!-- trellis-session: v=2 fp=f1692dc37447d725 -->

**Date**: 2026-10-13
**Task**: 内网静态下载页与首次安装指导
**Branch**: `codex/cross-workspace-collaboration`

### Summary

基于现有下载页交付 Nginx 静态版本，自动跟随 Windows x64 发布，支持业务地址复制和完整中文首次使用指导。

### Main Changes

- 静态深色首屏和浅色平台列表，其他平台待开放，纯内网运行。
- 发布流程从已校验安装包生成 download.json，配置保留并支持失败重试。

### Git Commits

| Hash | Message |
|------|---------|
| `35cc68ca2` | feat(desktop): let intranet users install without a manual handoff |

### Testing

- [OK] 166 项测试、Desktop 类型检查、lint 和内嵌脚本检查通过；有 1 条既有 lint 告警。
- [OK] 隔离 Nginx/Chromium 验证下载摘要、Range、新版刷新、HTTP 复制、错误重试、窄屏和重启持久化。

### Status

[OK] **Completed**

### Next Steps

- 部署现有内网 Nginx；配置业务 URL，使用正式 Windows 安装包完成原生验收。
