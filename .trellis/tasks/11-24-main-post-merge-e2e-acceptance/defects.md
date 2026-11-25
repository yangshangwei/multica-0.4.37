# 验收失败与后续处理

候选：`103e2bc329a9fb845d9f45c20b2a9bb3ebb54625`。此清单区分实际缺陷、执行前置失败与尚未验证的后续行为。优先级表示验收阻塞的重要程度，不等于每项都已证明产品根因。产品和 canonical 测试在本任务中保持原样。

| ID | 优先级 | 已观察事实与分类 | 后续处理边界 | 证据 |
| --- | --- | --- | --- | --- |
| D01 | P1 | 机器任务凭据继承 owner/admin 权限，真实编辑/删除其他人类作者评论；产品越权已复现 | 由已有 `11-20-comment-admin-human-authority` 修复任务限定 human principal，随后回归正反权限矩阵 | `runtime/comment-authority-{legacy-02,password-01}/probe-result.json` |
| D02 | P1 | Go race 整包 wrong_consumer 在合法 M1 授权准备时遭遇 NOWAIT 表锁冲突；持锁者未知 | 捕获实际竞争持锁者，验证完整包；不得以窄测试通过覆盖原失败，也不能放宽授权守卫 | `research/backend-continuation-results.md` |
| D03 | P1 | 安全扫描 exit3，14 项可达发现；包括本机 Go 标准库、第三方依赖和平台限制 | 在受支持修复工具链及依赖升级范围单独处理并重新扫描；不是14项线上攻击复现 | `backend/continuation-result-01.json` |
| D04 | P1 | memory-runtime/order-export 在启动许可前结束，Provider 未启动；API慢请求、503及连接池压力可见，取消先后与根因未知 | 补充许可 transport/context 错误及单调时间线，再定位；不提高超时或连接池来替代诊断 | `research/host-visit-failure-review.md` |
| D05 | P1 | cross-client、lane-close、lifecycle、offline-notifications 共9个原始身份在 namespace daemon ready 前置被拒绝 | 捕获实际 NamespaceController.Probe 错误；不能用另一配置的 EPERM 推定当前根因；后续执行断言未验证 | `runtime/{cross-client,lane-close,lifecycle,offline-notifications}-01/failure-classification.json` |
| D06 | P1 | macOS/Linux packaged Main 均实际启动和登录，等待旧 Collaboration 标题失败，未进入 Stop/重启/Resume | 在独立测试维护范围修复导航定位，再完整验证同一生产 Main 场景 | `native-acceptance/installed-main-02/`、`linux-main-acceptance/linux-main-04/` |
| D07 | P2 | manual-detail 两端、PAY creation 两端和资源发布一项等待已改变的导航标题/入口 | 维护 canonical fixture 的当前导航契约；不能把中间通过片段拼接成完整成功 | `runtime-supplement/{manual-detail,pay-creation}-01/`、`runtime/resource-publishing-01/` |
| D08 | P2 | Snooze 跨工作区场景未打开正确的切换菜单 | 在测试维护范围修复入口定位，完整复验 snooze/mention/分配唤醒和未读链路 | `runtime-supplement/inbox-snooze-02/` |
| D09 | P2 | Desktop delayed protected-read 拒绝提示定位器匹配6个元素；后续 source-loss 断言未完成 | 修复定位器语义后完整重跑；当前不能据此判断访问控制全部正确或存在泄漏 | `runtime/recovery-extras-delayed-02/failure-classification.json` |
| D10 | P2 | 公共 GitHub skills 集成实际 HTTP403；无响应头证明限流原因 | 保留外部依赖失败，另行核对服务/访问条件；不借用个人 token 掩盖本次失败 | `backend-diagnostics/skills-public-01/` |
| D11 | P1 | 旧CLI修正复制权限后原场景在240005ms超时，9次relay启动未完成全部10次；另一次兜底卷删除保留清理FAIL | 定位原时间预算内的实际瓶颈及清理中断，不提高超时或拼接步骤冒充通过 | `installed-cli-acceptance/installed-cli-03/`，20容器10卷最终已确认不存在 |

QA 路径均相对于 `artifacts/qa/main-post-merge-e2e-20261124-080504/`；`research/` 路径相对于本任务目录。

已通过环境修正补齐的历史失败仍保留：Desktop 信息恢复独立实例避免36次注册挤入30次预算；业务场景以新标签补齐中断后缺少的退出与收尾；Web delayed-read 绑定实际通过的自然响应暂停准入。旧CLI复制权限经断网对照证明并已用字节相同0755副本执行，最终仍超时，不能标绿。移动端还有既有ESLint直接依赖声明债务，静态检查本轮无错误；详见 `research/frontend-results.md`，不混入上述新发现。

明确平台缺口：原生 iOS 缺 Xcode/模拟器；Windows、签名与更新器缺相应运行条件；Firefox/WebKit 未安装；Helm/PowerShell四个脚本入口缺运行条件；pg_bigm 扩展缺失。七个后端 Linux namespace 身份在本次默认容器安全策略下 BLOCKED，保持原策略，不宣称所有 Linux 均不可运行。真实商业模型与生产发布不在本地验收范围。
