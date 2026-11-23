# 本轮运行与双端基础接线验证

任务继续保持 **in_progress**。本轮验证的是已有协作单的运行协议、静态 Linux 执行链路和双端基础工作台；公开创建、私有附件及全部 97 项产品验收没有被标记完成。

## 本轮改动

- 服务端完成原子 Switch、逐输入冻结授权、共享技能正文/文件/版本失效、M08 派发及 M11 输入接续。修复后续读取者已运行却重复回队、非启动续作迟迟不能取消，以及事务开始时间误判最后队长结果的问题；724–726 提供对应数据库约束和确定顺序。
- 现有 CLI 在完整协作身份下使用窄业务接口和原请求 UUID；显式静态 CLI 进入 Linux 隔离环境，真实 daemon 以容量 1 完成队长→成员→续作并分别提交证明。
- core 将取消 schema 与请求辅助逻辑分开，修复 ApiError 初始化循环；取消/Switch 恢复在新权限拒绝时清理私有缓存，失权后的本人回执严格只含七个字段。
- 共享工作台提供当前处理/我参与的列表、详情、评论/操作历史、交接与控制及原请求恢复；Web 独立账号入口和 Desktop 持久账号标签、独立窗口、历史/滚动、登录/工作空间失败恢复已接线。
- 扩展回归修复了已被当前锁协议淘汰的测试时序、全局配置队列干扰和归档后的测试数据路径。删除检查明确记录 32 张协作表的保留语义，并验证终态/非终态的来源历史不被删除。

本轮复用了已有 CLI、窗口协议、数据库锁与 Query/草稿机制；没有新增项目依赖。完整改动路径和校验值见 [源清单](verification-source-manifest-completion-2026-10-18.json)。

## 验证证据

| 检查 | 结果 |
| --- | --- |
| TypeScript 全量 | 5 包、11,993 项通过：core 3,602；views 6,897；docs 62；Desktop 1,143；Web 289 |
| lint / typecheck | 全仓 15 项通过；修复后的 core/views/Web、Desktop、Go 改动包另作定向检查 |
| Go 受影响包 race | 最新完整结果为 6,168 顶层测试 + 6,402 子测试通过；12 包通过，protocol 包无测试 |
| Linux 真实运行 | 5 个路由场景及 1 个实际 CLI/daemon 容量 1 闭环通过；逐 run 独立许可/退出证明 |
| Web 生产构建 | 真实跨账号流程 1/1 通过，7.0s，无重试；包含未知取消结果、原键 GET、提及/评论、交接/历史、另一个账号和撤权 |
| macOS Electron | 最终六场景 6/6 通过，20.2s；无空间/500/登录等待/双层守卫、独立窗口、历史、滚动、账号/Server 切换 |
| 视觉 / 文件边界 | 宽屏、390px、原生 200% 放大通过；主会话视觉结论 94/pass；原生文件访问检查无失败 |
| 数据库 / 生成 | 测试库升级至 726；并发索引恢复登记/迁移回归通过；108 个 sqlc 文件重生成零差异 |
| 独立检查 | 服务端/CLI、共享 core/页面、Desktop 复核中确认的问题已修复，当前切片无未解决缺陷 |

Go 初次广泛执行的两个失败包，已在修复后完整复跑通过；其余成功包复用同轮已完成结果。81 个默认跳过项保留在 [机器可读摘要](verification-summary-completion-2026-10-18.json)，包括未配置的 Redis、性能/外部选择性检查、平台相关检查以及既有 CLI 配置限制。Linux 协作场景另在真实 Linux 执行，不能把所有跳过项都计为通过。

Knip 全仓扫描包含旧 `.artifacts/release-*` 源码快照噪声；受影响工作区扫描没有 dependency/unlisted/unresolved 项，保留的 8 个文件和 4 个二进制报告均与 HEAD 一致。没有删除旧快照、放宽配置或添加依赖来使输出变绿。详见 [夹具与静态分析](research/verification-fixture-knip-2026-10-18.md)。

## 环境与复现

全部使用测试自建账号/工作空间/记录和静态可执行文件，未调用用户安装的 AI CLI 或付费账号。使用独立 PostgreSQL 测试库；应用原数据库未执行本轮迁移。

证据目录：`/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-cwc-resume-20261018-23vtl_pb`。主要日志：

- `typescript-static.log`、`typescript-tests-final.log`
- `go-full-affected-race.jsonl`、`go-primary-final-race.jsonl`
- `linux-routes-726-final.log`、`linux-cli-daemon-combined.log`；当前源码追加完整复跑见 `linux-routes-all-final.log`（6/6）、`linux-daemon-final.log`（5/5）和 `linux-runner-final.log`（18 通过；相反平台负例在宿主验证）
- `desktop-native-reviewed.log`、`workbench-check-web-run2.log`
- `api.running.json`、`web.running.json`、`web-source-drift-final.json`
- `sqlc-consistency-final.json`、`visual-verdict-final.json`

已保留失败运行：ApiError 循环、旧原生预期、无效浏览器样本、失去静态服务器的 Desktop 重跑、Go 测试隔离/锁时序。最终通过结果没有改写这些失败证据。

## 交付边界与下一步

[未完产品清单](research/remaining-product-work-2026-10-18.md)明确列出公开新建/受控附件、目标注册/授权/Switch 写表单、参与人和其余字段/评论操作、来源页面、账号实时、完整 transcript/用量、通用 provider/Git 与 P7/P8 验收。创建及新的客户端执行仍保持发布门禁。

725 对无法用唯一祖先链恢复顺序的旧活动小队明确拒绝升级，既有终态历史不重算；不得伪造轮次或清理未确认屏障来绕过。正常发布前仍须完成任务剩余验收与对应部署准备。

## 本地实现提交

| 提交 | 范围 |
| --- | --- |
| `b434d191d` | 归档后的稳定测试数据 |
| `0155ee735` | 服务端、迁移与 CLI/daemon 运行协议 |
| `6063a2682` | core schema、账号数据及原请求恢复 |
| `374049cec` | 共享工作台与 Web/Desktop 入口 |

本轮测试 API、Web 和静态 Desktop 服务已停止；测试数据库/容器及证据保留用于后续任务，状态见证据目录的 `cleanup.json`。原应用环境未被停止或迁移。
