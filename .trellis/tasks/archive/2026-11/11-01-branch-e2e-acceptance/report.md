# 822c15785 分支验收报告

**结论：未通过完整验收。** 本轮执行结束，尚有 P0 验收失败、既有未关闭缺陷和安装/真实 Provider 条件阻塞。此报告不批准发布。

## 版本与边界

- 被测分支：`codex/cross-workspace-collaboration`；固定 HEAD：`822c15785449b6c1cef699f6fd4d8e1f5483bd5e`。开始时工作区干净。
- 已核对远端主干及 merge-base：`dd87e0b33c1d6a64d488f7ee9ecd0a65984a68ff`。覆盖整条分支的业务差异，非只检查最近文档提交。
- macOS 26.2 / arm64；Node 22.23.2、pnpm 10.28.2、Go 1.27.0；PostgreSQL pgvector/pg17。
- Web 使用 `next build` + `next start`，两认证环境均保存 API/Web 提交、PID、源码指纹、Build ID。API 使用测试配置 `APP_ENV=development`；不宣称后端生产部署配置验收。
- 两个 detached 工作树均固定同一 HEAD；生成的 next-env 类型路径变化已识别。源分支由另一统筹会话继续合流：本轮结果只属于上述旧版本，不代表其新候选。

## 最终用例统计

按完整标题和参数去重，原库存 437 项，加新增 ENV-1 回归 1 项，共 **438 项**。

| 最终验收状态 | 数量 | 说明 |
| --- | ---: | --- |
| PASS | 421 | 部分在明确修正测试前置或交互后复验；原失败全部保留 |
| FAIL | 15 | 原库存 14 项失败，加 ENV-1 产品回归失败 |
| BLOCKED | 2 | 当前源码的 Linux 完整安装 Main / 保留旧 CLI 测试构件不足；原 Playwright 状态为 SKIPPED |
| SKIPPED | 0 | 原有可准备的条件用例已补跑；额外平台排除另表说明 |
| 实际执行业务用例 | 436 | 不含两个安装前置阻塞 |

累计记录 **553 次逐例执行结果**（含人工定位/复验和前置失败），其中原始 `passed=449, failed=79, timedOut=5, interrupted=1, skipped=19`。这些不是 553 个独立业务用例；汇总验收状态为 PASS 449 / FAIL 52 / BLOCKED 33 / SKIPPED 19 次。另有 3 条测试收集错误及未进入用例的启动错误。所有测试自动重试为 0，未增加断言超时或删除失败断言。

原完整首轮：legacy 335 通过 / 15 失败 / 17 条件跳过；password 40 通过 / 25 失败。首轮浏览器缺失、未迁移 Go 库、错误模式、目录及窗口配置均单独保存，未作为产品缺陷或成功业务执行。

## 静态、单元及集成结果

| 检查 | 实际结果 | 证据 |
| --- | --- | --- |
| lint / TypeScript typecheck | PASS | `runs/static-first` |
| TS 单元测试 | 13,059 PASS / 1 FAIL，共 13,060；失败独立复现 | `runs/typescript-first`, `runs/typescript-remaining-first`, `runs/unit-admin-filter-reproduction` |
| Go build / vet | PASS | `runs/go-build-vet-first` |
| Go 包回归及 race | 82 包完成：73 含测试包通过，9 无测试包；无包失败 | `go-summary.json`, `go-tests.csv` |
| Go 唯一顶层测试 | 10,261 PASS / 58 SKIPPED | `go-skips.json` 逐项列出 pg_bigm、Linux 专属、账号等原因 |
| Go 子用例 | 10,359 PASS / 1 SKIPPED；不与顶层数混计 | `go-tests.csv` |
| Redis 集成 | 专属实例、专库补跑成功；合并时不重复累计 | `redis-run-summary.json`, `runs/redis-*` |
| 脚本及配置检查 | 常规脚本 / UI exports / native checker 完成；Helm 校验 BLOCKED，缺少 Helm CLI | `runs/repository-script-checks-first` |

Go 原入口中的前 60 个完整包结果保留；共享库中 service 的部分运行停止后保留，未算作完整通过。其余 22 包各用全新迁移专库完成。`make check` 的相关阶段分开执行，未声称完整单命令或全栈门禁为绿色。

## 核心业务覆盖

| 领域 | 已实际验证 | 未通过或局限 |
| --- | --- | --- |
| 身份、工作区、成员 | 密码/验证码/设备登录，退出、会话撤销、首次改密、绑定，邀请/角色/移除，工作区隔离 | 不外推生产认证部署或多节点升级 |
| 任务、项目、迭代、分诊 | 创建、分配、评论/附件、批量/看板、完成/删除、项目进度、范围及历史 | 初轮优先级/隐藏字段失败后独立复验通过，稳定性原因未定 |
| 跨账号协作 | 三职责用户、私有工作区、接收/转交/补充/验收、通知和多会话；工作台全断言补验通过 | 多个旧收件人定位器使其余场景未完整走完 |
| Agent / 小队 | Web + Electron 同一 Agent，两个本机 host profile 接力，两个独立 Linux namespace、合并输入、停止/恢复/撤权与回写 | 确定性测试专属 Provider；非商业账号、模型理解或费用证明 |
| handoff / memory / recall | 启动/最新版本边界专项、冲突合并、恢复版本、关闭后清除、分页/大评论/撤权读取、原生入口 | **ENV-1 失败**；runtime 其余审计项和真实模型理解仍开放 |
| 知识 | 8 个 Web/native 候选提案、准确版本审批、纠错派生、发布/撤回、来源清除与租约 | 不等于“UI批准→真实模型使用”全链路；知识审计和执行 ON 门未关闭 |
| 聊天、Skills/MCP、自动化、管理 | 消息/失败恢复、附件、市场/目录/原生消费、资源发布、管理员/开关、自动化、模板 | 部分受测分支/平台组合仍按下表阻塞 |
| 错误、幂等、实时和兼容 | 超时/丢响应/重复请求、旧字段/未知字段解析、双会话同步、重连、重启、焦点/窄屏 | 一次 Web 503 与数次 UI 首败后复验通过仍需稳定性核销 |

需求/变更→风险→测试文件和用例键：`scope.md`、`change-risk-test-mapping.csv`、`research/review.md`。1,726 条代码/测试/配置路径的关联是领域关联，不冒称逐行覆盖。逐例→结果→证据见 `coverage.csv` 和 `executions.csv`。

## 实际平台与 Provider 范围

| 维度 | 实际结论 |
| --- | --- |
| Chromium 145（Playwright 1.58.2） | 生产 Web 正式回归；各例状态见矩阵 |
| 已安装 Chrome 156 | 独立早期冒烟 13 + 核心旅程 5 通过；中断轮和后续失败保留；不混作默认 Chromium |
| macOS Electron 39.8.7 | 真 Electron/preload/IPC，多会话、深链、账号/工作区/资源/协作等已运行；共享 harness 与完整 Main 分列 |
| macOS arm64 `.app` 完整 Main | 1 项 PASS：真实密码表单、原生 IPC、Stop/重启/显式 Resume、退出证明；固定包/ASAR/CLI 哈希；临时签名、未公证 |
| Linux arm64 | 两个实际 Docker namespace daemon/Provider 进程及生命周期专项；不是原生 Linux 桌面/安装升级证明 |
| Linux 完整安装版、保留 CLI 组合 | BLOCKED：未提供本次固定构件/relay；不复用其他 worktree 或旧缓存产品能力 |
| Windows 原生/安装/升级 | BLOCKED：无本次可用原生 runner 或安装构件，未触发外部 CI |
| Mobile、Firefox/WebKit | 本轮未执行；根脚本排除 Mobile，Playwright 默认只有 Chromium，不能由共享代码推定通过 |
| 商业真实 Provider / 多账号额度 | BLOCKED：无本次账号、模型、次数、费用授权；**实际调用 0，未重启旧实验** |

本地 HTTP assist/recommendation 使用测试专属固定 JSON/SSE、固定输出、延时/失败协议；覆盖 UI→真实 API→数据库，不评价模型质量或真实供应商兼容性。

## 缺陷与验收阻断

1. **QA-PRODUCT-001 / ENV-1，S2 medium，P0 验收项 FAIL**：DB 启动 memory v0，后续真实 PUT 到 v1，live handoff 和生产 UI 显示 `Started from v1`。只证明投影/标签错误，未证明模型真实启动输入被改写。截图、DB/API事实与失败断言均保留。
2. **TS 测试门禁 FAIL**：管理筛选用例固定 2026-10-01、无结束时间，在本次日期超过 31 天；合法性校验拒绝后测试读取空导航调用。应固定测试时钟，不能降低产品校验。
3. 原库存仍有 14 项 FAIL：旧收件人交互、包含动态 `evaluated_at` 的整体快照比较、记录区域分页断言及一个超时；不能将其余流程推定通过，也不全部包装为产品漏洞。逐项见 `defects.md`。
4. 既有 runtime 9 项、知识 21 项在冻结版本的核销门未关闭；本轮新复现 ENV-1，其余按原审计/源码核对继承，未冒称全部重新复现。后续主干修复不属于本轮版本。
5. 4 项首败后独立复验通过（Web 503、优先级、隐藏字段、原生选择）仍保留不稳定性记录；`flaky=0` 只表示无自动重试识别，不表示无波动。

## 环境与清理

专属 API/Web：18910/13830、19010/13930；renderer 19830；独立 Redis；两个固定源码树、合成用户、多个独立 browser context、runtime/profile/workdir。配置均经过 live `/api/config`、SQL gate 或服务身份验证。

本任务 API/Web/renderer、两个 HTTP fixture、Redis 已停止，端口无监听；password gate 恢复原布尔值且 epoch 单调递增。保留失败库、专库、包、工作树和原始日志用于复现；未清理默认 `multica` 或其他工作树。检查 `environment/cleanup.json`、`runs/restore-owned-gates`。

没有修改业务代码、降低断言或做业务重构。新增的隔离回归、交互适配和观察 hook 均有原文件来源；产品修复仍由各自任务处理。本任务暂不归档为验收通过。
