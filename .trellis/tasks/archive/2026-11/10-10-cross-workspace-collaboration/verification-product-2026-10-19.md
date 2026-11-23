# 账号创建、私有附件、资源表单与实时验证

本轮从 `4dd643126` 继续既有任务，完成下面列出的产品增量和独立检查。父任务仍为 **in_progress**；[剩余产品范围](research/remaining-product-work-2026-10-18.md)和默认发布门禁保留，未部署或开放生产功能。

## 交付与改动

- **新建和私有文件**：公开人类创建入口、明确来源/接收人、私有上传、整组草稿一次消费、同单交付、预览/下载和可撤权原生短能力。评论显示引用与已交付文件/指令快照分离。外人附件 ID 与不存在 ID 均不泄漏存在性。
- **新建拒绝恢复**：复用操作表和创建事务，在保存点中撤回被拒创建的单据、计数、参与和文件副作用；旧 key 的拒绝保持不变。只有匹配 actor/source/request 的 HTTP 200 原结果能解锁表单，用户可保留同样输入和文件明确重提，GET/重载不自动发送。
- **双端操作**：共享新建/附件、目标登记、本人逐资源授权/撤回、Switch 提案/同意/提交、参与人管理（含禁用账号）接线。禁用处理人被移除后，处理责任正确交回发起人。
- **实时**：账号/单据订阅、最终发送时授权、持久失效 outbox、运行状态事实与 Query 失效；普通变化保留有效草稿，失权先隔离迟到响应再清理内容/预览。账号连接不加入来源 workspace room。
- **恢复与规范**：上传取消会中止传输并保留不确定原请求；已确认评论在 core 中退役其上传组，卸载/重挂不能复活。补齐旧文件入口、未知 WS 类型日志、迁移保留及回滚竞争回归。

主要入口为 [创建服务](../../../server/internal/service/collaboration.go)、[私有附件](../../../server/internal/service/collaboration_attachments.go)、[实时投递](../../../server/internal/service/collaboration_realtime.go)、[core 恢复](../../../packages/core/collaboration/creation.ts) 和[共享工作台](../../../packages/views/collaboration/account-workbench-page.tsx)。161 个产品、测试和规范文件及 SHA256 见[源清单](verification-source-manifest-product-2026-10-19.json)。

本轮复用了现有 Storage/secretbox、用户锁、操作表、事务保存点、事件表、React Query、StorageAdapter 和 native IPC/窗口机制。草稿消费标记使用保留的交付记录，没有新增表或项目依赖；新增索引单独 CONCURRENTLY 建立并登记恢复钩子。

## 最终证据

| 检查 | 结果 |
| --- | --- |
| TypeScript 全量 | 12,073 项通过：core 3,672、views 6,907、docs 62、Web 289、Desktop 1,143；最后源码冻结后完整 core/views 又跑过 |
| lint/typecheck | 全仓 15/15 任务通过；31 个 warning 均在本轮未改文件，无新增范围 warning |
| Go | 受影响包和后续修正回归通过；合并保留 JSON 日志为 6,813 个顶层 + 6,373 个子测试通过，另有独立定向证据，不重复叠加统计 |
| Linux | 当前源码静态 CLI/daemon/路由六项通过，0 跳过；含容量 1 小队、分范围恢复及撤权后终结证明 |
| Web | 生产构建四项通过，24.98 秒，0 跳过/失败/重试；包含实时 WS 撤权和持久拒绝后的原样显式重提 |
| macOS Electron | 八项通过，28.16 秒，0 跳过/失败/重试；含资源/Switch、原生下载、重启、独立窗口、账号守卫和 200% 缩放 |
| 生成/迁移 | 112 个 sqlc 文件重新生成零差异；727/728 的并发索引、保留数据及未提交写入的回滚拒绝有实际 SQL 验证 |
| 文件/视觉 | 16 项实际 Electron 本地文件边界检查通过；UI exports 通过；最终宽屏/390px/原生 200% 截图视觉 93/pass |
| 静态分析 | Knip 与上轮相同的 8 个文件/4 个 binary 提示；所列文件均与基线相同，0 新增 dependency/unlisted/unresolved 问题 |
| 独立检查 | 后台、共享客户端、新建拒绝协议三次检查收口，无未解决的本轮行为发现 |

Go 默认条件跳过保留 102 个日志条目，包含未提供的扩展/外部配置、平台和显式 opt-in 场景。六个 Linux 场景已以真实 Linux 执行替代宿主跳过；其余不计为通过。逐条名称与原因在[机器摘要](verification-summary-product-2026-10-19.json)。`pg_bigm` 未安装，因此真实 opclass 验证仍跳过。

## 环境与复现

证据目录：`/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-cwc-product-20261018-nebz79rl`。

- `environment.json`：本轮独立数据库；全部迁移至 728。原应用数据库未改变。
- `api.running.json`、`web.running.json`、`desktop-renderer.running.json` 及三份 `*-product-build.json`：命令、PID、生产模式/构建 ID 和源码哈希。
- `web-product-final.json`、`desktop-product-final.json`、各自 `-input.json`：最终运行时源码与构建一致，漂移列表为空。
- `typescript-tests-final.log`、`typescript-core-views-current.log`、`typescript-static-final.log`：全量及最终受影响包验证。
- `go-affected-race.jsonl`、`go-agent-canonical-race.jsonl`、`go-collaboration-final-race.jsonl`、`builtin-template-final.jsonl`、`go-router-final-race.jsonl`：宽回归和修正证据。
- `linux-routes-final.log`、`migration-tests-race.jsonl`、`sqlc-consistency.json`、`desktop-file-access.log`、`knip-baseline-comparison.json`、`visual-verdict-final.json`：专项结果。

TypeScript 使用 `scripts/check.sh` 的单包/双 worker 入口。Go 使用 `scripts/go-test-with-agent-cli-guard.sh` 并遵守 `scripts/test-go.sh` 的 `-parallel 2`；实际 agent 测试只用自建可执行文件。Linux 使用原任务专用容器和本轮独立 DB，运行当前源码构建的静态程序。无用户安装的 AI CLI、真实付费模型或生产资料参与验证。

初次构建和失败日志均保留：浏览器缓存路径/原生 CORS 缺失、共享账号围栏夹具重复插入、非标准并发下的两个 provider 计时失败、技能正文超预算和资源用例过早读取。环境修正、条件式夹具初始化、标准并发全包复跑、文档分层和等待真实提交分别关闭这些问题，没有删除业务断言或放宽生产保护。

## 独立检查与边界

- [后台/稳定 core 检查](research/verification-product-2026-10-18.md)
- [共享客户端检查](research/verification-product-client-2026-10-18.md)
- [新建拒绝协议检查](research/verification-creation-rejection-2026-10-18.md)
- [创建拒绝实现与锁验证](research/creation-rejection-2026-10-18.md)

本轮尚不覆盖来源页面入口/完整字段与评论编辑、完整运行正文/用量及所有 R52 通知、运行端 Git/文件准备、通用 provider、有限管理员代管、真实 S3/分布式 Redis 故障，以及其他 OS 的原生验收。未消费草稿/未知提交加密孤儿的自动回收仍需后续实现。最终客户端证据采用生产 renderer、真实 HTTP/WS 和已有 native-service 测试程序，不等同完整安装包/Main 启动验收。

所有后续工作按[当前未完清单](research/remaining-product-work-2026-10-18.md)继续。父任务不归档。

## 本地实现提交

| 提交 | 范围 |
| --- | --- |
| `2f39f00be8` | 服务端、迁移、私有文件、原请求与实时契约 |
| `e5876258b4` | core schema、草稿、取消/恢复和账号同步 |
| `9a8b7f4833` | 共享 Web/Desktop 界面、双语文案及真实客户端验收 |

本轮 API、Web 和静态 Desktop 测试服务已停止；专用数据库、加密测试文件、构建产物和日志保留，见证据目录 `cleanup.json`。原应用环境未停止或迁移。
