# 核心功能与近期合入代码测试报告（执行中草稿）

**结论：测试已覆盖主要本地业务路径，完整验收尚未闭环。** 密码补测、Host/知识部署开关和完整 Main 等条件项仍在推进。本草稿不批准发布，也不将历史失败追认为通过。

统计快照：**2026-11-02 01:28:24 UTC（09:28:24 上海时间）**。后续原始结果以各 `evidence/<run>/run.json` 和 `results.json` 为准；收尾时须重新生成覆盖表并更新本报告的数字。

## 版本与测试边界

- 分支：`codex/cross-workspace-collaboration`；业务候选：`7af9ab9905c98c77bb091223ebe687c639475bb4`。
- 比较基线：本地 `main` / `origin/main` 的 merge-base `dd87e0b33c1d6a64d488f7ee9ecd0a65984a68ff`；本轮未 fetch，不称为最新远端主干。
- 整条分支 446 个提交；旧 `822c15785` 验收后 128 个提交。旧版本报告仅作风险索引，未计入本轮通过数。
- 当前工作副本增加两项测试修复：memory 历史展开交互与五个 handler 用例的在线运行时前置。业务实现、接口标准、生产心跳门槛和依赖未改。每次修复的原失败、补丁哈希、独立审阅及复验分开保存。
- Web 使用生产构建和 `next start`；API 使用开发测试配置及合成身份。单独核对数据库、实际 `/health`、认证模式、页面 API/WS 地址与构件哈希。此结果不外推生产部署、邮件送达或商业模型效果。
- 使用测试专库、专属 Redis、私有客户端数据目录及测试 Provider。Go 数据库包逐包分库。默认登记中的不匹配 API 未作为被测服务。
- 本机工具链为 Node 22.23.2、pnpm 10.28.2、Go 1.27.0；CI 的 Go 1.26 系列组合未由本机结果代替。

计划与执行入口：[测试计划](../test-plan.md)、[执行清单](../implement.md)、[进度](../progress.md)、[覆盖表](../coverage.csv)。

## 已执行结果

| 层次 | 实际结果 | 主要证据 |
| --- | --- | --- |
| 静态、类型与脚本 | 非 Mobile lint/typecheck/UI exports 及九段验证命令均 exit 0 | [汇总](static-ts-result.json)、`evidence/static-ts/` |
| 非 Mobile TS 单元 | 13,082 通过，1,063 个文件，五个测试包均成功 | `evidence/static-ts/03-typescript/` |
| Mobile | 196 通过，28 个文件；typecheck/lint 与三类 iOS wrapper 回归通过；lint 留有七条警告 | [Mobile 报告](mobile-result.md) |
| 数据种子集成 | 原常规入口的可选数据库用例另在新库补跑，13 项通过、无跳过（含子例） | `evidence/seed-integration-1/` |
| Go build / vet | 通过 | `evidence/go/00-build/`、`00-vet/` |
| Go 首轮 | 82 包全部执行；顶层 10,325 PASS / 23 FAIL / 35 SKIP；子例 10,487 PASS / 1 SKIP；九包无具名测试 | [原始汇总](go-result.json) |
| Go 后续整包复验 | 五个失败包均有新成功整包证据；另有 auth+Redis 整包 51 PASS / 0 SKIP。按每包最新完整证据派生：73 个含测试包成功、九个无具名测试包；顶层 10,362 PASS / 21 SKIP，子例 10,487 PASS / 1 SKIP | 下述复验记录；该派生不改原首轮结论 |
| 原样完整 E2E | 439 库存：349 PASS / 1 FAIL / 89 SKIP；exit 1，22.5 分钟 | `evidence/legacy-full-1/` |
| 后续条件 E2E | 截至快照，47 个已完成 Playwright run，共 571 次逐例执行：471 PASS / 11 FAIL / 89 原始未执行状态。按 439 个库存身份、每例最新有效结果只读对账：**426 PASS / 1 FAIL / 12 NOT_RUN** | 见 [覆盖审计](coverage-audit.md)；汇总表当时尚未更新到这一快照 |

Go 后续整包证据包括 `go-cmd-server-replay/full/`、`go-home-replay/{cli-full,daemon-full,execenv-full}/`、`handler-full-freshfixture-1/` 和 `auth-redis-full-1/`。没有把五项单例成功当作 handler 整包成功；handler 后续确实完成了整包 race：**2,863 PASS / 0 FAIL / 7 SKIP**。

原始完整 E2E 仍为失败。后续逐例结果覆盖了新增条件与经审阅的测试交互，不等于同一原始完整轮变绿，也不等于执行过一次全部通过的 `make check`。多次执行数量不能当作独立功能数量。

## 核心与近期高风险功能

已执行的业务路径包括登录与会话、工作空间与成员权限、任务及评论生命周期、失败回滚与幂等重试、项目/迭代/分诊、通知及多会话同步、资源/MCP 与能力市场、自动化、管理界面，以及实际 Electron 的账号导航、语言偏好、弹出窗口、重启、推荐与设置。

| 近期风险 | 当前证据与限制 |
| --- | --- |
| Transfer 收件人契约 | 工作台等正常转交及原键恢复路径在当前库存中执行；保留全部原断言。完整安装版路径仍需自己的实际执行结果 |
| memory 嵌套清理后 Leave | `password-19-collaboration-memory-runtime-2` 与 `password-20-collaboration-leave-web-2` 均 PASS；同一数据库、API PID 26546、配置哈希。memory 执行前与 Leave 进入前的执行门保持 enabled=true / host=false，epoch 5→7；未在两例间重置环境 |
| 协作上下文并发、恢复与历史清除 | 原样 full/single 两次失败后，仅补展开历史并加强可见历史中的 closed restore 检查；十项关联回归、最终两项 memory 回归通过 |
| Linux 双执行域与跨客户端 | `password-21-collaboration-cross-client-execution-2` 已 PASS；范围是实际 Linux namespace 与测试 Provider，不能据此声称 Linux 桌面安装/升级通过 |
| 知识审核、纠错、撤回与清除 | 八项 Web/Electron UI/API 旅程通过；实际部署开关 OFF/ON 与物理 Host 输入必须另看 dedicated probe，快照中尚未完成 |
| 初始 memory 归因与召回 | Go 对应回归已有执行；当前 E2E 的 ENV-1/原生 handoff 补测在密码批次后段，快照中仍未运行完 |
| 原生来源、可信 bundle 与时间边界 | 已运行构建、脚本、Go/TS 及实际 Web/Electron 路径；未放宽来源、过期、焦点、断言、超时或自动重试规则 |

具体需求和入口以测试计划的 F01–F14 为准。文件归属、框架通过与平台能力分别报告，不将共享代码通过推定为所有客户端功能通过。

## 本轮发现与处置

1. **测试进程环境污染。** Go 测试继承了协调者注入的 pool 和 task-path 覆盖，干扰默认值与 HOME 隔离断言。原 18 项失败保留；改用每包私有 HOME、显式缓存路径并移除这些测试进程覆盖后，四包整包复验通过。API 的连接预算仍单独保留。
2. **memory 测试未展开已收起的历史。** 截图显示关闭任务后的历史收起，原用例直接等待 Purge 控件。仅补正常展开操作并在历史可见时检查 Restore 仍不可用，所有原清除、权限、版本与窄屏断言保留。历史收起的具体触发原因尚未证实，不将它写成产品设计要求或声称已排除所有 UI 状态保留问题。审阅：[memory harness](memory-harness-review.md)。
3. **在线 fixture 前置不稳定。** 五个 handler payload/mismatch 用例使用一次创建的共享运行时。仅为这五个预期在线前置刷新心跳，保留生产 150 秒规则及独立 stale/offline 负例；新库整包 race 通过并已同步根分支。新 run 的刷新前年龄为 107.824 秒，低于门槛；旧失败行已被 TestMain 清理，因此不能仅由这一新日志重建旧失败的确切因果。审阅与边界：[handler 审阅](handler-heartbeat-review.md)、[集成记录](handler-heartbeat-integration.json)。
4. **条件补测前置漏配。** 首批 namespace/control 用例运行时执行门仍为默认关闭；memory-runtime 还选择了普通 host-provider。记录原门与 epoch，仅在专库准备 namespace 允许状态并选择 memory-provider 后，旧 CLI、memory→Leave、跨客户端及控制/交接补跑通过。产品默认值未改。
5. **合成账号触发真实限流。** 密码补测保留账户与注册限制；触发后暂停准备并等待窗口自然恢复，未重启 API 清空计数或提高阈值。快照唯一最新 FAIL 为 retained-attachment 第三例的 fixture 注册 429，业务断言尚未进入；前两例已通过，需在窗口恢复后原样复验。

完整问题记录见 [defects.md](../defects.md)。以上分类不将所有首败当作产品漏洞，也不删除环境或测试前置造成的失败。

## 尚未完成的门

- 快照中 12 个库存身份仍 NOT_RUN：Host runtime/squad/switch、完整 Main、部署知识开关、ENV-1/原生 handoff、选择边界、源视图与关闭后重开等。须以新结果更新，不能用准备好的二进制或旧报告代替。
- 知识 OFF 与 ON 是两个必验配置轴，同一用例标题的一次 PASS 不能同时关闭两个轴。
- 同源等价 Mac/Linux 包已经做来源和哈希预检。其构建来源仍声明为 `639bf6ef8`；即使三棵业务目录 Git 树一致，也必须实际新运行。完整 Main、目录包、安装器与升级分别计证据。
- Windows 原生/安装升级缺当前 runner；iOS 实际 UI 缺完整 Xcode/simctl；商业 Provider 缺本轮账号/模型/调用上限授权。保持 BLOCKED，商业实际调用数为 0。
- Go 的剩余条件跳过须逐项说明，包含平台分支、性能/外网 opt-in，以及 CLI 未知字段保存、Webhook DB-error 注入等已有测试空白。不能统一归为“缺平台”或记作通过。
- 开关恢复、任务进程退出与清理证据尚待收尾。未使用旧 private launcher，也未生成旧 clearance 或借用历史付费额度。

## 改动与交接

当前根分支只有两个测试源码改动：`e2e/collaboration-memory-web.spec.ts`、`server/internal/handler/daemon_test.go`；另有本任务规划、执行证据和报告材料。沿用现有 `TestApiClient`、`testutil`、Playwright/Vitest/Go 入口，没有业务重构、新依赖或降低验收标准。

收尾顺序：完成仍可执行的密码与 Host/知识/完整 Main 门 → 根据原始结果刷新 `coverage.csv` 和条件矩阵 → 更新问题与风险 → 记录开关恢复、进程退出 → 更新正式报告、进度及新会话引导语。最终引导语应明确已获创建和执行授权，先核对实际运行状态，再继续未关闭项；无需重复询问任务授权。

**正式报告发布前待替换：** 快照后的数值、retained-attachment 复验、Host OFF/ON 和完整 Main 结果、最终剩余平台条件、清理状态，以及当前源码/测试补丁指纹。
