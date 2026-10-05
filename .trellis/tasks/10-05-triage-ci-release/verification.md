# 分拣台 CI 与发布验证

更新：2026-10-06。修复已完成本地验证和独立审查；远端 CI、合并与发布待本次流程完成后更新。I1 工程方案另行交付，未实现产品功能。

## 已修复根因

| 失败 | 根因与处理 |
| --- | --- |
| Mobile Verify | InboxItemType 新增 triage 后完整标签映射遗漏；补齐标签 |
| 内置技能模板 | 创建智能体技能正文534行；原文提取至单层reference，入口469行 |
| 文档bundle漂移 | 中文快速上手新增MCP部署说明未重新生成；生成并验证确定性 |
| 分拣提醒两测试 | pgx截断/PostgreSQL文本转换四舍五入使纳秒deadline差1微秒；fixture统一微秒精度，保留精确数量断言 |
| 生命周期删除竞态 | 将全部admission错误误映射409；仅Blocked为409，保留底层缺行/数据库错误 |
| 长时间handler测试 | TestMain运行时心跳超过150秒；claim fixture模拟领取前心跳，不放宽生产门槛 |
| 离线发布验证 | 测试fixture未带新增MCP指南；复制真实源并验证三类归档必需文档/模板目录 |
| 管理审计 | 资源UUID修订不符合原整数version schema；仅该字段兼容UUID，隐私字段仍过滤 |

## 本地证据

- lint/typecheck：15个任务通过；UI导出静态扫描通过；E2E严格TypeScript检查通过。
- TS全套：9956测试、846文件通过，五个包；Mobile另123测试、21文件通过，7条已有lint warning。
- Go：66个常规包在完整check中通过；最终handler整包race通过（2396顶层、含子测试4184通过，51条件跳过，199.673s）；pkg/agent race整包通过（264.408s）；全仓go vet通过。
- 确定性复现覆盖原始3项失败与过期心跳2项；修复后窄race分别100次、20次通过。
- Web生产构建、桌面生产构建通过；真实Web/HTTP/原生Electron分拣端到端17/17通过，单worker零重试，34.1s。
- 发布changelog/离线包装测试53通过、0失败、4个显式opt-in真实容器smoke跳过。
- go tool govulncheck ./...：No vulnerabilities found。
- 独立只读代码审查：无新增P1/P2，未通过放宽行为断言掩盖问题。

首次非限流TS运行4个views测试超时；仓库规定的2worker完整复验全通过。完整check的Go编译曾捕获修复前心跳fixture，handler因此失败；其余包结果保留，随后最终源码独立完整handler、agent包、vet与API/Web/E2E均通过。本记录是可追溯的分阶段累计验证，不称首次单条make check全绿。

日志：`/tmp/triage-release-check.log`、`/tmp/triage-backend-handler-final.jsonl`、`/tmp/triage-release-agent-tests.log`、`/tmp/triage-release-vet.log`、`/tmp/triage-release-browser.log`、`/tmp/triage-release-scripts-reverify.log`、`/tmp/triage-release-vuln.log`。受控环境`check-20261005155531-55778`，API/Web18572/13492；运行身份与构建日志已保存在`.omx/reports/triage-ci-release/`。API/Web已停止，临时API数据库、Go数据库、profile和环境注册槽位已全部清理。

原始管理/注册E2E草稿来自前次234项回归，本次核对源码与严格类型，但未重新执行全部管理浏览器场景。真实模型调用、目标内网部署和桌面签名安装不在这些本地结果内。

## 发布跟踪

目标仓库：yangshangwei/multica-0.4.37。计划版本v0.5.6；按现有tag驱动工作流发布。该fork的工作流发布后端/Web镜像与累计changelog，upstream-only CLI/Homebrew/Desktop工作不会因此被宣称交付。业务生产环境部署与版本核验另需实际目标环境证据。

## PR 第一轮远端结果

[PR #2](https://github.com/yangshangwei/multica-0.4.37/pull/2) 的初始提交 fcc116c1b：Mobile Verify、backend-tests、Windows执行环境、SQL生成、漏洞扫描、前端构建、基础前端测试和视图第二组均通过。CI 37340463506 仅视图第一组的既有快速创建分页测试超时5000ms，因此整体不通过，未合并或发布。

该用例初始筛选逐字触发50行列表的五次渲染，并重复全树可访问名称扫描。最小测试修改使用真实粘贴设置初始查询、按现有aria-label定位pin，仍保留55项fixture、50→55→55→50、pin焦点与末尾单字符键入复位。未改变生产代码、timeout或retry。完整suite22/22和独立关键case5/5通过，ESLint通过；待远端再次核验。
