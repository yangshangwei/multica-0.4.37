# Current plan coverage

Captured: 2026-11-25T08:59:03.106908+00:00. Source remains `103e2bc329a9fb845d9f45c20b2a9bb3ebb54625`.

Statuses describe the mapped evidence and known gaps, not exhaustive behavior beyond those tests. Domain counts overlap.

| ID | Domain | Status | Named case dispositions | Remaining boundary |
| --- | --- | --- | --- | --- |
| T01 | 启动、配置、迁移 | PASS | See independent non-browser receipts | Fresh per-package PostgreSQL integration, actual runtime health/provenance and script fixtures; no production deployment |
| T02 | 认证与账号 | PASS | PASS:34 | Real auth/browser journeys plus synthetic native Server-switch and session fixtures; injected login helpers alone do not prove login |
| T03 | 工作区、组织、成员 | PASS | PASS:20 | Actual browser/API workspace flows and backend authorization matrix |
| T04 | 任务生命周期 | PASS | PASS:51 | Real CRUD/API/DB; core Agent journeys use SimulatedDaemon; M1 and deterministic assist need their own permits |
| T05 | 项目与小队默认 | PASS | PASS:67 | Current canonical Web/Electron-renderer project lifecycle, goals, health, risk and progress cases |
| T06 | 迭代 | PASS | PASS:23 | Real Web/native-renderer flows, database timezone round trips and handler P95; mobile is headless data only |
| T07 | 筛选、搜索、分页、导航 | PASS | PASS:26 | Canonical browser/renderer navigation and pagination; mocked large actor lists are not server load evidence |
| T08 | 评论、附件、引用 | FAIL | PASS:22 | Machine owner/admin task credentials changed other human authors' comments through real HTTP; separate generic-upload observations remain policy questions |
| T09 | Inbox、Triage | FAIL | FAIL:3, PASS:36 | Browser and actual database snooze/triage, explicit native renderer; first snooze attempt refused incomplete allocation before business setup |
| T10 | Agent、角色、小队 | PASS | PASS:99 | Real API/UI controls; recommendation and assist providers are deterministic local fixtures, not commercial model quality |
| T11 | 协作创建与信息 | FAIL | FAIL:5, PASS:60 | Actual API/production browser and fixture Main, exact original-key/history/permission recovery |
| T12 | 协作审批与交付 | FAIL | FAIL:2, PASS:13 | Original full Go race wrong_consumer scenario failed during legal M1 setup on a NOWAIT table-lock conflict; narrower diagnostic pass does not close it |
| T13 | 协作执行与控制 | FAIL | FAIL:12, PASS:24 | Seven original Linux Go namespace identities blocked under tested default container policy; two original process identities passed, root-only subcase passed separately |
| T14 | 知识全生命周期 | FAIL | FAIL:1, PASS:16 | Human knowledge API/browser and service integration; automatic knowledge and deployed on/off Host input need dedicated runs |
| T15 | 全局权限与隔离 | FAIL | FAIL:2, PASS:78 | Comment machine-principal authority defect reproduced; govulncheck also reports 14 reachable findings with toolchain/platform limitations |
| T16 | 实时与一致性 | FAIL | FAIL:3, PASS:61 | Actual clients/WS where exercised; core daemon is simulated and live recovery/host paths remain separate |
| T17 | 设置与资源 | FAIL | PASS:35 | Fresh public GitHub skill-import integration returned HTTP403; no headers establish rate-limiting as the cause |
| T18 | 管理、安装与观测 | FAIL | FAIL:1, PASS:12 | Dedicated real password admin/enrollment/runtime lanes; unit and Compose contract results cannot replace their UI actions |
| T19 | 桌面原生与持久化 | FAIL | FAIL:2, PASS:49 | Renderer/preload/fixture Main and actual packaged production Main reported separately; ad-hoc signing is not distribution/notarization |
| T20 | 移动端 | BLOCKED | See independent non-browser receipts | Native iOS/RN login, keyboard, foreground/background and network recovery |
| T21 | CLI、daemon、安装/升级 | FAIL | FAIL:5, PASS:13 | Native Windows installer/Jobs/ACL and signed OS updater/restart; Seven original Linux Go namespace identities blocked under tested default container policy; two original process identities passed, root-only subcase passed separately |
| T22 | 多语言、主题、可访问性 | FAIL | FAIL:1, PASS:54 | Only named viewport/theme/keyboard/layout assertions and saved captures; no blanket claim of every UI surface or native mobile accessibility |
| T23 | 文档、插件、自托管 | BLOCKED | PASS:14 | Four declared platform script entry points: one Helm, three PowerShell |
| T24 | 性能、规模、清理 | PASS | PASS:3 | Performance thresholds and final current-resource closure verified: PG/Redis stopped, retained data,182ports closed,0owned processes/locks/extra containers; CLI03 historical fallback cleanupFAIL retained separately and not rewritten. |
| T25 | Chat 会话与消息 | PASS | PASS:10 | 10 current browser cases plus canonical headless/backend tests; injected failure responses remain labeled fixture recovery, not arbitrary network/device proof |

| Lane | Status | Scope |
| --- | --- | --- |
| L0 | FAIL | Lint/typecheck/TS and normal Go passed; original race and govulncheck failed; Linux normal supplement closes2 original process skips +1 root preflight subcase;7 namespace identities BLOCKED in tested default policy; remaining platform/script gaps retained |
| L1 | FAIL | Original538 complete:400PASS138SKIP; assist/recommendation/manual detail supplements tracked separately; fixture Main only |
| L2 | FAIL | Independent compatible password suites, real auth flows and separately declared native fixtures |
| L3 | FAIL | Fresh synthetic admins, actual enrollment/resource/observability, no inherited production account |
| L4 | PASS | One exact live device login case; independent device API |
| L5 | FAIL | Namespace-only gate, explicit fixture binaries; not real commercial-model/host proof |
| L6 | FAIL | Real controlled local daemon/provider/CLI; deterministic fixture provider, distinct knowledge-on/off variants |
| L7 | FAIL | Full packaged journey result requires original named case and exact protocol/process restore; package assembly alone is preparation, failed boot/control attempts remain failed |
| L8 | BLOCKED | Only Chromium installed/configured in inspected cache; no added browser dependencies |
| L9 | BLOCKED | Independent static and196headless tests PASS; native Xcode/simulator/driver absent |
| L10 | PASS | Fresh100-account manifest/default API pool and sequential prepare→propagation→writeback; require complete10000 propagation and100×2 writeback samples plus original thresholds and cleanup |
