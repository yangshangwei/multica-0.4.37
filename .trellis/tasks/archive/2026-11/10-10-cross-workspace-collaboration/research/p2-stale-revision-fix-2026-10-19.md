# CWC-QA-001 P2 修复（2026-10-19）

用户要求修复此前专项 QA 的 P2。本轮修复记录归入现有父任务，父任务保持进行中。

根因：HTTP 409 stale_revision 的合法最小正文省略 status，API 保留脱敏 ApiError，共享操作层因而未把已知拒绝收敛。修复仅在 executeCollaborationAction 的 protectCollaborationRead 回调内，await 并严格识别该确切拒绝；原有权限/访问版本检查完成后，复用对应草稿移除与查询失效流程。表单保留输入，显式采用当前版本后以新键重新提交；网络未知、畸形/冲突回执和其他错误码仍走原键恢复。

验证：Core/API 47 文件 680 项、Views 13 文件 46 项、生产 Web 7 项通过；Core/Views/Web/Desktop node+renderer 类型与受影响 lint/diff 通过；独立审查无实质发现。修复前两层失败证明与中途旧恢复夹具失败均保留；夹具修正不削弱原重挂载、字段/false 值、版本及单 PATCH 断言。

真实双账号回归记录：服务器409没有正文status；即时拒绝，原页保留输入，显式采用当前版本后新键200、同单done；严格两次生命周期POST、无恢复GET、无AI或页面错误。没有用刷新绕过复现。

证据与修复说明：`.omx/reports/cwc-p2-fix-20261019/report.md`、`summary.json`、`review.md`、`source-manifest.json`、`browser/lifecycle-conflict-evidence.json`。本地提交将记录在同目录 commits.json。

运行代码仅 operation.ts 一处；复用现有机制，无依赖、API/服务端、迁移、布局或翻译改动。原生 Electron、真实 provider、Linux runtime/安装包完整验收没有在本修复轮次重跑。父功能的其余验收仍保持原状态。
