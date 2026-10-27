# 最近提交完整端到端验证（2026-10-27）

最终结果：仓库现有 **408/408 条 E2E** 全部实际通过，另补充的 **3/3 条场景** 通过。TypeScript **12,804 条测试**、Go race 测试与 `go vet`、lint 和类型检查均通过。发现并修复了两个 Electron 测试替身缺少 `app.getPath` 的问题，原先失败的 18 条桌面单测全部恢复；没有修改产品逻辑或降低断言。

## 范围与证据

- 应用源码固定为 `a8e374bb99c5f3c407f76c2d607e623ce15d6552`，覆盖该分支相对 `e3d5a93d732a6fc5496a0c255a85acef8f8a4dbd` 的最近改动，重点是协作上下文、普通宿主机执行、跨空间流转和打包验收。
- 执行期间主工作区新增 `87939bbe0`；其改动仅为 Trellis 文档、说明与证据，应用源码没有变化。Web 和桌面产物使用固定提交的独立工作树。
- [机器可读汇总](../../.trellis/tasks/archive/2026-10/10-27-recent-commits-e2e/evidence/summary.json)、[全部用例结果](../../.trellis/tasks/archive/2026-10/10-27-recent-commits-e2e/evidence/e2e-cases.csv)、[运行时与安装包结果](../../.trellis/tasks/archive/2026-10/10-27-recent-commits-e2e/evidence/runtime-summary.json)。
- 原始日志、Playwright JSON、截图、失败追踪及重跑脚本保留于 `.omx/reports/recent-commits-e2e-20261027/`，该目录不入 Git。

## 最终验证结果

| 检查 | 结果 | 证据 |
| --- | --- | --- |
| 全项目 lint / typecheck / UI exports | 通过；有既存的非阻断 lint 警告 | `baseline-check.log` |
| Core / Docs / Views / Web 单测 | 4,173 / 62 / 7,046 / 285 通过 | `baseline-check.log` |
| Desktop 单测 | 1,238 通过，0 失败 | `desktop-final.json` |
| Desktop 修复后 lint / typecheck | 通过 | `desktop-final-lint.log`、`desktop-final-typecheck.log` |
| Go race / go vet | 73 个有测试的包通过，含 Go 测试缓存命中 | `backend-check.log` |
| 验证与环境脚本回归 | `test-go.test.sh`、`dev-env.test.sh`、`check.test.sh` 通过 | `backend-check.log` |
| 现有 Playwright E2E | 133 个文件、408 个唯一用例，最终全部通过 | `coverage-progress.json` |
| 补充 E2E | 3 个唯一用例通过 | `additional-memory.json`、`committed-memory-retry.json`、`admin-memory-run3.json` |

每次 Playwright 运行均为 1 worker、0 自动重试。最初的全量收集有 75 条环境条件跳过，随后在相应环境中全部执行，最终没有未执行的现有用例。表中的结果按唯一用例去重，配置修正后的专项重跑不重复计数。初始 `scripts/check.sh` 在桌面单测处退出；修复后重跑整个桌面包，并独立完成后续所有检查阶段，没有把初次退出记录改写为成功。

## 业务覆盖

- 登录、密码注册与迁移、设备登录、首次管理员初始化、权限撤销与会话恢复。
- 工作区、任务、项目、迭代、视图、评论、附件、Inbox、自动化和智能体任务闭环。
- Web 与原生 Electron 的协作创建、资源共享、跨空间转交、退出、撤销、关闭/重开、来源边界和执行记录。
- 协作上下文的并发冲突、草稿保留、历史恢复、终态清除、实时刷新和双运行时接力；运行时通过实际 run-local CLI 读写上下文，检查版本与作者归属。
- 管理后台账户/权限、执行和安装元数据、监控、资源发布与撤回；发布结果进入实际 Web 和原生资源市场。
- 假模型 Provider 驱动的真实 API/数据库起草流程、候选推荐，以及无需重启的更新日志发布。
- macOS 原生宿主机运行和真实 `.app` Main/IPC 的停止、暂停后重启与显式恢复；Linux 容器中的隔离执行、生命周期/关闭/通知、已发布 CLI v0.6.7 兼容性及当前 Linux 目录包 Main/IPC。

## 发现与修复

`b24f5b92e` 将 daemon 管理器的 home 来源改为 `app.getPath("home")`，两个基于 VM 的测试仍只模拟 `os.homedir()`。调用在凭据探测之前抛出 `TypeError: electron_1.app.getPath is not a function`，并连带使健康检查和重启断言失败。

- 修改 `apps/desktop/src/main/daemon-manager-auth.test.ts` 和 `apps/desktop/src/main/daemon-manager-managed-recovery.test.ts`，各补一处 `getPath: () => root`，复用原有临时目录。
- 原样复现：61 条中 43 通过、18 失败；修复后整个桌面包 1,238 条通过。
- 保留全部原有断言和生产代码的 Electron home 边界，没有引入兼容回退、依赖或重构。

测试环境问题已分别处理：密码注册的同 IP 30 次/5 分钟预算通过独立批次执行解决；CLI 路径使用指定工作树的 `server/bin`；原生 MCP/密码绑定用例补齐其本工作树要求的、经哈希验证的 renderer/preload。原始失败结果仍保留。

## 补充的三个场景

1. 原生 Desktop 编辑上下文，Web 实时读取并写回，Desktop 恢复历史后两端一致，历史版本保留且账户请求不携带工作区头。
2. 先确认服务端提交 v1，再模拟丢失响应；刷新后以原始幂等键重试，只保留 v1，不重复生成版本。原有测试只在转发前制造失败，因此此次单独覆盖提交后的未知结果。
3. 发起人未禁用时阻止管理操作；禁用后管理员清除历史 v1，保留当前 v2，管理页面始终不展示正文，操作有真实管理员署名与原因。

补充用例的源码和配置保存在两个隔离工作树的 `.omx/qa/` 中。编写时修正了网络完成时序和“发起人已禁用”的测试前提；这些早期脚本失败没有计为产品缺陷。

## 环境与清理

| 环境 | API / Web | Web 构建 |
| --- | --- | --- |
| Legacy 与设备/首次初始化验收 | 18266 / 13186 | `B9PfMvpybsHoXzeA2hz47`，production |
| Password / managed / admin / runtime | 18273 / 13193 | `00e_i90HREe_jQfYb3kEM`，production |

两个环境均使用隔离 PostgreSQL 数据库和合成账户。所有本次 API/Web、renderer、假 Provider 已关闭；相关端口已确认关闭。协作创建/执行/宿主机执行三个门禁均恢复为 false；Go 和首次管理员初始化的临时数据库已删除，其余隔离测试数据与证据保留供复查。本次没有停止其他窗口的服务或归档其他任务。

这次验证不代表商业 Provider 账号/配额、两台物理机器、原生 Windows、安装器升级、正式签名或公证通过。宿主机 Provider 为明确构建的测试替身；Linux 目录包运行与包管理器安装是不同事实。
