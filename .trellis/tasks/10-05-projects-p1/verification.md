# P1 实施验收记录

状态：P1 范围实施与验收完成（2026-10-06，Asia/Shanghai）。用户于 2026-10-05 授权实施已评审方案。本文件记录实际实现证据，[planning-verification.md](planning-verification.md) 仅保留此前规划评审。合并与部署状态独立记录于末节。


2026-10-06 追加测试细化已完成：从 8 个复合流程扩为 **61 条独立测试，61/61 通过、137.9s、零重试/跳过**；本轮未改产品代码。[逐条用例](../10-05-projects-p1-verification/e2e-case-matrix.md)与[运行证据](../10-05-projects-p1-verification/e2e-expansion-verification.md)单列，原八项/全仓结果保留为历史。

## 工作树与基线

- 分支：codex/projects-p1；工作树：/Volumes/artisan/code/2026/multica-projects-p1；起点：0af59c5d5。原 main 工作树中的其他会话改动未复制或修改。
- make setup-worktree 已完成；锁定依赖由 pnpm 10.28.2 安装，隔离数据库 multica_multica_projects_p1_991 初始迁移至 535。
- 初始后端基线：经 scripts/go-test-with-agent-cli-guard.sh 运行 go -C server test ./internal/handler -run TestProject -count=1，exit 0，2.037s。
- 初始 core 基线：pnpm --filter @multica/core exec vitest run projects api/project-execution.test.ts --maxWorkers=2，5 文件 / 52 测试通过。
- 此基线不是 P1 验收；UI 准备阶段随后新增了预期失败的契约测试，见 UI/preparation.md。

## 执行所有权补充

父计划保持五子任务。foundation 内另设关联 writer 子lane：handler/issue.go、triage_actions.go、autopilot.go、资源/聊天/小队配置入口、service/issue.go，以及实际调用方 handler/lifecycle_handoff_transaction.go 的必要锁与重试；证据在 foundation/association-verification.md。foundation 主lane仍独占 project.go、workspace.go、所有迁移/SQL/生成文件、事务helper、router/main 与删除整合。其余 lane 不得写这些共享文件。

## 里程碑

| 门槛 | 状态 | 证据 |
| --- | --- | --- |
| FG 基础schema/锁/事务/DTO及局部测试 | 通过 | `c4250df70`、`60f3346e4`、`7ec7bba14`、`fe54efd4a`；[基础验证](../10-05-projects-p1-foundation/verification.md)及[关联写入验证](../10-05-projects-p1-foundation/association-verification.md)，17 入口双序、10 阶段删除回滚 |
| HG 健康API/同事务采集/测试 | 通过 | `8f27d663d`、`1bc589119`；[健康验证](../10-05-projects-p1-health/verification.md)、[C 性能](../10-05-projects-p1-verification/performance-c-report.md) |
| PG 进展API/outbox/测试 | 通过 | `bb1c20dcb`、`0bd01de1d`、`262c011cf`；[进展验证](../10-05-projects-p1-progress/verification.md)，真实 inbox wire 与执行证据授权 |
| UG Web/Desktop及移动兼容 | 通过 | 最终产品 `31534d844`；[UI 整改](../10-05-projects-p1-ui/review-fixes-2.md)、[预览修复](../10-05-projects-p1-ui/preview-whitespace-verification.md)、[移动兼容](../10-05-projects-p1-ui/mobile-verification.md)及[最终双端验证](../10-05-projects-p1-verification/browser-verification.md) |
| 最终整合 | 通过 | 静态检查、10,052 TS 测试、Go race/vet、独立 mobile 检查；新增 UI 修复 26 项定向回归通过，重新构建后 8/8 E2E 通过 |

## 逐项验收

逐条测试名、实质断言、实施提交和执行记录集中于[独立验收审计](../10-05-projects-p1-verification/acceptance-audit.md)，覆盖 27 AC、14 项需求和 18 项补充 FR；避免在本文件复制一份容易失同步的通过表。测试设计及原要求见 [test-spec.md](test-spec.md)。

独立[后端复审](../10-05-projects-p1-verification/backend-review-2.md)与[前端三审](../10-05-projects-p1-verification/frontend-review-3.md)均 APPROVE。BR-01—03、RR-01—03 已关闭。额外审计的 G1 旧 router 实际 404/400、G2 客户端规划日切换刷新、G3 闭合 100% 仍无人工验收，均已补充实际证明。

AC-22/23 的真实迭代部分待 I1；本次验证计划规定的已有任务/执行/项目边界，不虚构迭代实体或通过记录。

## 最终检查及可复现证据

完整检查命令（仓库 `make check` 对应脚本，E2E 范围为 P1 两份 spec）：

```sh
export PATH="/opt/homebrew/opt/libpq/bin:$PATH"
ENV_FILE=.omx/projects-p1-browser.env \
PLAYWRIGHT_JSON_OUTPUT_NAME=.omx/p1-complete-check-e2e.json \
bash scripts/check.sh e2e/projects-p1.spec.ts e2e/projects-p1-desktop.spec.ts \
  --output=.omx/p1-complete-check-results --reporter=list,json
```

首次完整流水线候选 HEAD `61ba556e4`，产品源码 `97e3f9124`；两者之间只有文档改动。环境 `check-20261005144412-26132` 使用独立 Go/API 数据库及 API 19072/Web 13992；归属与构建证据在 `~/.multica/dev/envs/check-20261005144412-26132/`。完整日志 `.omx/p1-complete-check.log`，其退出码 1 与各阶段结果已固化为 [full-check-summary.json](../10-05-projects-p1-verification/full-check-summary.json)。最终补验候选为 `31534d844`，不把此前失败流水线改写成一次全绿。

| 检查 | 实际结果 |
| --- | --- |
| lint/typecheck/UI exports | 15/15 Turbo 任务通过，强制执行；lint 0 errors，UI 3、views 27 条已有 warnings |
| TypeScript | 856 文件 / 10,052 测试通过：core 2635、docs 62、views 6116、desktop 957、web 282 |
| 脚本回归 | `test-go.test.sh`、`dev-env.test.sh`、`check.test.sh` 通过 |
| Go | `scripts/test-go.sh --race` 69 个有测试包通过，包含 handler、service、migrations 与受 guard 保护的 agent 测试；`go vet -p 2 ./...` 通过 |
| 生产构建 | 当前 API 健康检查通过；production Web build 成功；Desktop `electron-vite build` 与源码/产物哈希证明另由 browser 报告保存 |
| 本轮首个完整 E2E | 7 通过 / 1 失败，0 retry、0 skip；发布用例在添加 execution 证据时按钮禁用超时，保留失败记录，不将整条 `check.sh` 标绿 |
| 发布预览修复补验 | `31534d844`；尾空格 mention 的 raw getter/trimmed callback 忠实回归 RED→GREEN；2 文件 / 26 测试、views typecheck、修改文件 lint 通过 |
| 最终 Web/Desktop | `31534d844` 两端重建成功；同一全组 8/8 通过，49.7s、0 retry、0 skip。Web build `7nHz90Ry9eikLfmkC-5ER`；最终 JSON `.omx/p1-browser-final-e2e.json`，日志 `.omx/p1-browser-final.log` |
| 视觉与证据归档 | [visual-verdict](../10-05-projects-p1-verification/browser-evidence/visual-verdict.json) 为 pass、94/100；描述冲突按钮完整、中文 390px 保存可点击、Electron 680px 与风险提示可读。20 份浏览器证据 manifest/hash 校验通过 |
| 双页收敛 | 最终 30 次实际 due_date/assignee/admission 变更，各 10 次；两页 DOM 与版本收敛总体 P95 161ms、最大 281ms，低于 5 秒预算。此前 110ms/130ms 为首轮数据，另行保留 |
| Mobile 独立检查 | `c66ba6cfc` 的 typecheck/lint/test 通过，27 文件 / 185 测试及 iOS wrapper shell 断言；7 条已有 lint warnings。之后 mobile 与其共用 schema/metrics 无改动 |

Mobile 原始证据 `/tmp/p1-mobile-404-{typecheck,lint,test}.log`；根检查排除 mobile，以上为单独执行结果。TriageFilters 的异步 portal 选项断言从同步查询改为等待真实选项，仍验证原 member/agent 标签和 actor ID；定向与完整 views 测试已通过。

首次 E2E 暴露两个连续问题：提及候选的延迟 focus 使立即输入的 UUID 落入正文，测试改为等待候选插入并显式验证证据输入框焦点和值；随后真实预览 200 后仍退回编辑态。请求/响应证明 `getMarkdown()` 保留 mention 后尾空格，而 onUpdate/unmount 会 trim，导致草稿被误判为新编辑。`31534d844` 只统一进展 composer 的读取规则，保留真正编辑时旧预览必须失效的语义。最终测试仍验证实际 execution 证据、授权 transcript、丢响应幂等重试、更正历史、描述版本验收与另一成员 inbox 深链；没有删除或弱化这些断言。

最终复跑命令（沿用独立环境，不重跑未改变的后端及移动代码）：

```sh
bash scripts/dev-env.sh exec check-20261005144412-26132 -- \
  env PLAYWRIGHT_JSON_OUTPUT_NAME=.omx/p1-browser-final-e2e.json \
  pnpm exec playwright test e2e/projects-p1.spec.ts e2e/projects-p1-desktop.spec.ts \
  --workers=1 --retries=0 --output=.omx/p1-browser-final-results --reporter=list,json
```

## 迁移与性能

- [迁移报告](../10-05-projects-p1-verification/migration-verification.md)：完整 573 条历史 up、仅 P1 536—549 的 14 条 down；9 顶层测试含 252 个 guard/writer 子场景通过；实际 INVALID concurrent index 失败恢复 red→green、旧数据无损、工作空间清理原子性已验证。
- [原 A 性能失败](../10-05-projects-p1-verification/performance-report.md)保留为设计变更依据；[ADR-05](research/pagination-adr.md)经独立架构/批判评审批准后实施 C。
- [C 性能报告](../10-05-projects-p1-verification/performance-c-report.md)：500 项目、目标 10,000 正式任务；0/1/10 次变更每秒各不少于 600 秒。1,709/1,709 非空后续页严格前进，三档第二页成功率均 100%，0 HTTP/写入/一致性错误，最高静态 P95 558ms。完整风险 SQL/API 集合审计 74 页一致。
- 94 份性能脚本和原始证据已校验 JSON/JSONL，约 4.16MB；凭据特征扫描无 bearer token、数据库连接 URL 或私钥。私库配置和二进制保留于 ignored 目录。

## 交付与限制

- 工作保存在独立分支 `codex/projects-p1`；P1 未推送、未合入 `main`，未创建发布标签，未生产部署。T1 的已有合并/CI事实以其单独验收记录为准。
- 验收后已停止本任务的 API/Web 服务，保留专用库、registry 与证据。六份任务状态已完成，本会话 Trellis 活动指针已清除；测试提交为 `21f263796`，产品提交为 `31534d844`。
- AC-22/23 的真实迭代归属/历史分支留待 I1。P1 只证明当前任务、执行与项目边界，不将不存在的迭代实体列为通过。
- 未运行 iOS 模拟器、设备或 IPA 视觉验收；移动交付限于既定兼容范围。未执行真实 agent CLI/消耗真实账户配额，测试使用安全 fixture 和 guard。
- 未配置真实 Redis 测试连接，对应限流集成测试按既有条件跳过；不声称 Redis 集成已验收。用户目标找寻成功率、风险定位时长等产品指标未做用户实测。
- 保留已完成任务目录以维持 PRD、设计、API 与证据链接；最终状态以 task.json 与本文件为准。历史规划/中间失败记录不重写成最终通过记录。
