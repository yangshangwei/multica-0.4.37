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


## Session 78: 修复注册、窄屏推荐和收件箱三类回归
<!-- trellis-session: v=2 fp=989041d9b570b251 -->

**Date**: 2026-10-17
**Task**: 修复注册、窄屏推荐和收件箱三类回归
**Branch**: `fix/qa-three-regressions`

### Summary

原六项失败已复现并全部修复；相关28个唯一E2E、6907项Views单测和15项静态检查通过，独立视觉97/pass，隔离环境已清理。

### Main Changes

- 注册预留校验提示空间，保留首次点击和键盘提交；375px推荐区限高并保留键盘滚动；正式收件箱测试改用语义按钮定位。

### Git Commits

| Hash | Message |
|------|---------|
| `44e67538447d3fe45f1838d1e39bf62a9977c99b` | fix(auth): keep the first registration submit click reliable |
| `1a47a28d56bdb9e49445c44bf9373e881df402b9` | fix(issues): leave room to edit tasks beside compact recommendations |
| `52a451bad73d4f52fcd989db86452aab52fb1c78` | test(inbox): keep notification acceptance aligned with native controls |

### Testing

- [OK] 原失败6/6通过；相关28项唯一E2E通过（含Electron），零自动重试/跳过。
- [OK] Views 563文件6907项通过；lint/typecheck15项通过，31条既有warning；独立审阅通过。

### Status

[OK] **Completed**

### Next Steps

- 本地fix/qa-three-regressions已提交归档；未推送，未重跑全量328项或Go测试。
