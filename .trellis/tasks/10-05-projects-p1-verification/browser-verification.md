# P1 Web / Electron 最终验收

当前细化结果：2026-10-06 已按用户要求扩为 **61 条独立用例，全部通过，137.9 秒，零重试/跳过**。其中 25 条拆分/保留原覆盖、36 条新增/强化。参见 [逐条中文用例矩阵](e2e-case-matrix.md) 和 [本轮运行与修正记录](e2e-expansion-verification.md)。以下八项记录保留为首次实施验收历史，不是当前用例总数。

## 前轮八项整体验收（历史）

状态：**通过**。2026-10-06（Asia/Shanghai），产品候选 `31534d844` 重新构建后，**8/8 测试通过，49.7 秒，0 重试、0 跳过、0 flaky**。其中 7 项业务流程（Web 6、Electron 1）与 1 项双页收敛测量。最终结果见 [安全结果记录](browser-evidence/final-results.json)，此前记录原样保留于 [历史报告](browser-verification-history.md)。

## 环境与源码归属

- 工作树 `/Volumes/artisan/code/2026/multica-projects-p1`，分支 `codex/projects-p1`；完整 [构建与进程证明](browser-evidence/provenance.json)。
- 专用 registry `check-20261005144412-26132`，API `http://localhost:19072`、Web `http://localhost:13992`，与 Go 测试数据库隔离。
- API commit `31534d844`，listener PID 91721；生产 Web `next build` / `next start`，listener PID 92966，build ID `7nHz90Ry9eikLfmkC-5ER`。
- Desktop 同一产品提交执行 `electron-vite build`，exit 0；真实 Electron main/preload/renderer，入口 `index-DNeAcGUl.js`，产物 SHA256 已记录。没有打包安装器或调用真实 agent/daemon。
- 测试经真实认证/API 建立隔离工作空间；synthetic execution 仅插入测试行，未执行进程。每例清理自己的工作空间；结束后父代理停止本 registry 的 API/Web，保留数据库和证据。

## 最终测试映射

| 流程 | 实质断言 | 结果 |
| --- | --- | --- |
| 原生 Electron | 真实 preload/router、发布/历史、风险下钻、中文 680px；daemonStarts=0 | 通过，5.3s |
| 目标模板与描述冲突 | 章节追加/不重复/不覆原文、真实两成员 CAS；冲突按钮完整可见，采用服务端版本后旧草稿不回填 | 通过，8.6s |
| 双页收敛 | 30 次真实 due_date/assignee/admission 修改，两 Page 显示同一新版本与正确 DOM 数字 | 通过，5.7s |
| 正式范围和统计 | 6 完成/2 取消/2 未结束，旧 done_count=8；五视图与候选排除；普通接受加一且零执行；闭合 100%/全取消仍无验收 | 通过，11.5s |
| C 风险分页 | 个人过滤隔离、跨版本游标前进、sticky 变更提示、positive-total 空后缀、失败保留提示、从头刷新包含低 ID 重入任务 | 通过，2.1s |
| 发布、更正与验收 | 真实 member/agent mention，执行证据/TranscriptDialog，预览无写；提交丢响应后幂等重试；不可变修订，旧/新描述版本验收；另一成员 inbox 深链与私有证据拒绝 | 通过，8.1s |
| 完成/删除 | 完成警告允许继续，项目状态不改任务和 synthetic running execution；member 拒删，owner 删除保留工作与历史 | 通过，2.1s |
| 能力、恢复与窄屏 | 概览 503 恢复、标明模拟的旧能力回退、中文 390px 键盘流程、无横向溢出、末尾保存按钮九点 hit-test | 通过，4.7s |

AC-22/23 只涵盖 P1 当前任务/执行边界；真实迭代归属与历史属于 I1 后续验收。能力故障注入不冒充真实旧 router 证明，后者见 [legacy-capability-verification.md](legacy-capability-verification.md)。

## 双页收敛的实际结果

原始 [最终 30 样本](browser-evidence/convergence-final-30.json) 从 HTTP 提交前开始计时，直至两个真实页面显示更新后数字与同一版本，取较慢页面耗时。每类 10 次，nearest-rank P95。

| 类别 | 样本 | P50 ms | P95 ms | 最大 ms |
| --- | ---: | ---: | ---: | ---: |
| 全部 | 30 | 97 | 161 | 281 |
| 截止日期 | 10 | 97 | 281 | 281 |
| 指派 | 10 | 86 | 139 | 139 |
| 正式准入 | 10 | 109 | 135 | 135 |

全部满足 5 秒预算；page_errors 为空，执行计数保持为零。首轮 110ms/130ms 数据另保存在 [首轮样本](browser-evidence/convergence-first-check-30.json)，不混入本轮结果。服务端三档各十分钟的活跃分页验证见 [C 性能报告](performance-c-report.md)，与本项 DOM 收敛测量分别计量。

## 发现、修复与失败保留

首次完整 `check.sh` 在 `61ba556e4` 通过 15 静态任务、10,052 TS 测试、69 Go race 包、go vet 和生产构建，但 E2E 7/8，故整命令 **exit 1**；[首次结果](browser-evidence/first-check-results.json)与[流水线摘要](full-check-summary.json)保留，不重写为一次全绿。

1. 点击 mention 候选后，Tiptap 延迟恢复焦点，立即填证据导致 UUID 落入正文，Add evidence 正确禁用。保留 [焦点失败截图](browser-evidence/focus-race-before.png)。测试保持原操作顺序，等待候选插入完成，再验证证据输入框 focus 和实际 value。
2. 随后真实预览返回 200、正确 recipient 和 evidence，却退回编辑态。[请求/响应](browser-evidence/preview-whitespace-red.json)与[失败截图](browser-evidence/preview-reverted-before.png)证明：imperative getMarkdown 保留 mention 尾部空格，onUpdate/unmount trim 后被当成新编辑。
3. `31534d844` 仅统一进展 composer 的正文读取规则；[忠实 RED→GREEN 与 26 项回归](../10-05-projects-p1-ui/preview-whitespace-verification.md)、types/lint 通过，真正编辑仍撤销旧预览。随后双端重新构建并执行本次全 8 项，无重试通过。

此前描述按钮裁切、390px 保存按钮被浮动聊天遮挡、风险页缺基准日的问题，历史红灯与最终断言均保留；没有删除失败场景来获得通过。

## 视觉核验与交付证据

[visual-verdict](browser-evidence/visual-verdict.json)：**pass，94/100**。最终截图展示完整描述冲突按钮、390px 可点击保存动作、680px Electron 布局、风险基准日及持续变更提示。关键截图：[描述冲突](browser-evidence/description-conflict-after.png)、[中文窄屏](browser-evidence/chinese-390-after.png)、[Electron](browser-evidence/electron-680-after.png)、[执行证据](browser-evidence/execution-evidence.png)、[进展验收](browser-evidence/progress-acceptance.png)、[C 连续分页](browser-evidence/risk-live-continuation.png)。

20 份安全证据的路径、字节数、SHA256 见 [manifest](browser-evidence/manifest.json)，父代理核验全部一致；JSON 与凭据特征检查通过。私有数据库连接、认证 token、完整服务配置不进入此目录。原始本机测试输出留在 ignored `.omx`。

## 复现命令

```sh
bash scripts/dev-env.sh exec check-20261005144412-26132 -- \
  env PLAYWRIGHT_JSON_OUTPUT_NAME=.omx/p1-browser-final-e2e.json \
  pnpm exec playwright test e2e/projects-p1.spec.ts e2e/projects-p1-desktop.spec.ts \
  --workers=1 --retries=0 --output=.omx/p1-browser-final-results --reporter=list,json
```

命令退出码 0；原始日志 `.omx/p1-browser-final.log`。移动兼容、迁移与外部依赖限制见父 [实施验收记录](../10-05-projects-p1/verification.md)。本报告不代表生产部署、真实 agent 账户运行、iOS 设备/模拟器/IPA 验收或 I1 集成完成。
