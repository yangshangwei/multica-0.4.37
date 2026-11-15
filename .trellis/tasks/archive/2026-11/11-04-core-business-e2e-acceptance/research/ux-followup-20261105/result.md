# UX 增量业务验收结果

候选 `49e991156`（包含界面修复 `5a7440952`）在独立副本中完成四个测试文件、五个实际用例，全部通过。自动重试为0；未运行真实AI模型或Provider。

| 实际用例 | 结果 | 关键证据 |
|---|---|---|
| 接收人搜索→选择另一账号→创建 | PASS | 等待真实搜索响应时Enter不误选/不创建；按确切UUID选人；显式单次创建；双方API读取同一处理人 |
| 丢失上传/创建响应后的文件交付与撤权 | PASS | 原请求恢复、私有文件字节、账户WebSocket撤销已打开预览 |
| 创建拒绝后显式重试 | PASS | 文件与输入保留，旧拒绝请求不复活 |
| 真实管理员Web处置 | PASS | 真实密码登录、管理员身份、禁用发起人后的继任和原回执恢复 |
| Electron管理员处置 | PASS | 真实Electron进程、生产渲染/预加载与IPC、账户壳隔离、原回执恢复 |

## 执行身份与修正

使用 `/Volumes/artisan/code/2026/mc-ux-e2e-49e991156-20261105`，Web正式构建与Electron构建均通过；752条迁移的全新模板给每个测试文件分配独立数据库。API/WS地址、实际监听进程与源码目录已核验，源码/构建产物在测试前后完全匹配。

附件交付测试需要一处夹具适配：评论附件和整单附件都有具名区域，整单定位增加自身标题约束。此修正不变更产品或原文件/撤权断言，并已实际通过后集成回主工作区。

首轮ux01接收人1/1通过后，在delivery测试启动前被TCP TIME_WAIT端口探测拒绝。无残留监听或数据库连接；仅给探测socket加入SO_REUSEADDR，并验证真实监听仍被拒绝。原中断完整保留，ux02在新数据库上只执行其余四例，不重跑已通过接收人。不能把两次运行描述为首次完整一轮通过。

## 清理与范围

API/Web/renderer全部停止，浏览器/Electron残留0；本轮6个数据库（模板+5个克隆，含未开测的delivery库）连接全部0，数据保留用于审阅。测试窗口已明确交给runtimeH1。

这是当前UX候选的增量业务证据，不替换另一会话冻结1a32的440PASS/1LinuxFAIL和性能FAIL。Electron使用生产代码的测试主进程，不是安装包完整Main；真实模型、安装交付、Windows/iOS、物理触控和屏幕阅读器仍属独立验收。

## 文件

- `evidence/acceptance-summary.json`：5个精确测试身份、结果、数据库和时长。
- `evidence/ux01/summary.json`：首轮PASS及setup中断。
- `evidence/ux02/summary.json`：其余4例PASS和完整审计。
- `evidence/final-resource-audit.json`：连接/监听清理。
- `evidence/screenshots.json`、`evidence/screenshots/`：实际截图与哈希。
- `live-preflight-review.md`：独立执行前审阅。
- `final-acceptance-review.md`：独立结果审阅。

原始框架JSON、包含测试凭据的配置/日志仅保存在权限受限的私有目录，不提交。

独立最终复核：PASS。复核读取原始Playwright结果、真实管理员回执/SQL证据、20张截图与6项原生几何记录，确认精确用例数、候选身份和资源清理，无遗留本轮阻断项。
