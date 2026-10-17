# Journal - artisan (Part 2)

> Continuation from `journal-1.md` (archived at ~2000 lines)
> Started: 2026-10-16

---



## Session 75: 独立功能分批合入本地 main
<!-- trellis-session: v=2 fp=5dc447dfccb9ff63 -->

**Date**: 2026-10-16
**Task**: 独立功能分批合入本地 main
**Branch**: `main`

### Summary

按用户批准拆分合入六组独立功能，保留协作开发分支；11377项TS、19项Web/Electron E2E、三端构建及下载/seed验证通过，测试资源已清理。

### Main Changes

- 保留 main 认证逻辑，只移入 PAT 改动；不引入协作代码、迁移和旧截图。

### Git Commits

| Hash | Message |
|------|---------|
| `52cdbb8bf` | test(views): keep page regressions trustworthy during feature landing |
| `4edaae527` | feat(desktop): let intranet users obtain installers without manual handoff |
| `bddc9ef24` | fix(autopilots): preserve creation context across navigation |
| `f7352d91e` | fix(settings): keep edits and access boundaries correct after failures |
| `79d2bc483` | docs: make installed usage guidance complete and navigable |
| `11865f7a0` | test(seed): make business acceptance reproducible on local databases |

### Testing

- [OK] 11377 TS；19 Web/Electron E2E；15 static tasks；24 download＋13 seed；Web/Desktop/docs builds；Go docs。

### Status

[OK] **Completed**

### Next Steps

- 本地 main 已完成合并；远端推送或发布需后续任务。


## Session 77: 能力市场整合：完成 Web/Desktop 验收与归档
<!-- trellis-session: v=2 fp=baf9e5d1484c181d -->

**Date**: 2026-10-17
**Task**: 能力市场整合：完成 Web/Desktop 验收与归档
**Branch**: `feat/capability-market`

### Summary

完成独立能力市场及资源管理分离；修复上下文焦点与测试夹具，11406项单测和25个唯一Web/Electron用例通过，已本地提交归档并清理测试服务。

### Main Changes

- 复用 Skill/MCP 原创建与显式分配流程，移除旧市场组合和 libraryView；保留原管理入口。

### Git Commits

| Hash | Message |
|------|---------|
| `581ba2ba5` | feat(marketplace): make capability discovery reachable from one workspace entry |
| `96943ea64` | test(marketplace): keep completed acceptance reproducible across both clients |

### Testing

- [OK] 945文件/11406单测；25唯一E2E/10spec；lint、typecheck、三端构建通过；Knip与13项基线一致。

### Status

[OK] **Completed**

### Next Steps

- 远端推送、main 合并和发布未执行。已保留其他任务原有未提交材料。
