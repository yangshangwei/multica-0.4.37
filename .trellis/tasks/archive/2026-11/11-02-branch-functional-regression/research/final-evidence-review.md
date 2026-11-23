# 最终独立证据审计

结论：**PASS WITH NOTES**。本轮本地功能测试的主要结果、逐例对账、测试补丁与清理记录相互一致。没有发现阻断本地结果交付的技术证据缺口；交付前修正下文 N1 的文字计数。此结论不批准发布，不关闭 Windows/iOS/安装升级/真实商业 Provider 等独立验收门，也不批准旧私有 launcher。

审计快照：2026-11-02T01:55 UTC。产品候选 `7af9ab9905c98c77bb091223ebe687c639475bb4`。审计仅读取仓库、公开证据和现有 Git 差异；没有运行测试、连接数据库、操作 API/服务/容器/进程、读取私有凭据或派生代理。仅写本审计的 Markdown 与 JSON。

## 独立重算结果

| 口径 | 从原始证据重算 |
| --- | --- |
| E2E 库存 | 439 个唯一身份，148 个 spec |
| 完成的 E2E run | 60 |
| 逐例框架记录 | 588：486 PASS、13 FAIL、89 SKIP/NOT_RUN，含补跑 |
| 各库存身份的最新有效结果 | 439 PASS；0 FAIL；0 NOT_RUN |
| 与 `coverage.csv` 的结果/run 回链差异 | 0；身份未匹配或不唯一 0 |
| 原样完整 E2E | 349 PASS、1 FAIL、89 SKIP，exit 1，仍保留失败 |
| Go 最新完整包视图 | 82 包均有成功完整包证据；73 包有具名测试、9 包无具名测试 |
| Go 加明确条件补跑 | 顶层 10,365 PASS、18 SKIP；子例 10,491 PASS、0 SKIP/FAIL |
| 独立标签知识专项 | ordinary 14 顶层/13 子例、Host 9 顶层/8 子例，各在 normal/race 通过；共 46 次顶层与 42 次子例执行，无失败/跳过 |

审计从每份已完成 Playwright 原始 `results.json` 重新匹配库存并按 run 时间选择最终状态，没有执行正式 reconciler。每个实际 Playwright 命令均记录单 worker、零 retries；结果的 `retry` 值均为 0。Go 则读取每包最新完整 run 的原始 JSON 事件，再单独应用 `cmd-redis-1` 和 `linux-agent-kernel-2` 的条件用例，未把专项成功替换成虚构的整包运行。

非 Mobile TS 原始输出五包分别为 4,340、62、7,118、1,272、290，共 13,082 PASS；Mobile 原始 Vitest JSON 为 196 PASS、0 FAIL/PENDING。种子专库集成原始 TAP 为 13 PASS、0 SKIP。原 9 段静态/脚本命令均 exit 0。最后新增的 Node 控制器回归 11 项、Web typecheck、views lint 与 diff-check均通过。native checker 首次漏参数 exit 2 被如实保留，后续正确参数验证 11 个 macOS 必需用例通过，未将错误入口追认为通过。

## 产品、补丁与平台边界

- 当前 Git 差异局限于 4 个已跟踪测试文件、1 个新增测试 helper、2 份验证规范及任务材料；未发现业务源码或依赖变更。根目录 memory 文件、3 个完整 Main picker 文件的内容哈希均与集成记录相符；handler 测试补丁哈希与其成功整包运行一致。
- memory 修改增加展开历史后再检查 Restore 不可用；handler 修改仅为五个预期在线 fixture 建立当前心跳；完整 Main 使用当前精确 UUID picker 并保留选中状态及业务回执断言。对照差异，没有降低生产权限/时钟门槛，未删除业务断言或增加超时/重试。三个独立审阅及对应新运行均存在。
- 原样完整 E2E 与 Go 首轮失败没有被覆盖。原 handler 失败时的 runtime 行已被清理，新 run 的 107.824 秒诊断不能重建原行；memory 历史收起的具体触发也未直接建立。报告保留了这两个因果限制，没有把测试前置修复成功写成已证实的产品根因。
- 部署知识 OFF/ON 的原始附件分别观察到 section/contributions 关闭/开启，授权事件为 0/1，run 均 settled、`host_exit_observed`、success。它们是独立部署下的物理输入证明，与通过程序设置 gate 的标签专项分开陈述。
- Mac 原始附件标识真实 `darwin arm64 packaged production Main/preload/renderer`，`nativeIPCStubs=false`、`authInjection=false`；Linux 原始结果为目录包在容器中的完整 Main。报告没有将这两种运行称为包管理器安装/升级或原生 Linux 桌面验收。构件来源、产品树等价和实际哈希保留，当前结果来自新 run。
- Windows、iOS 实际 UI、Linux 安装升级/原生桌面、签名公证与商业账号/模型质量仍明确未完成。Go 剩余 18 条顶层跳过没有计 PASS。报告不把 fake Provider 或旧结果外推为这些门通过。

## 清理与敏感信息

`evidence/cleanup/run.json` 记录 9 个 browser 专库的门布尔值和非递减 epoch；全部 `enabled=false`、`host_execution_enabled=false`、creation 恢复。password 专库显式准备的 namespace 门另有前后恢复记录。自有 API/Web、renderer、两个 HTTP fixture 与带任务 label/ID 的独立 Redis 都有停止记录；四个分配端口均无监听，自有 Host/packaged 进程匹配为空。

清理记录只表明本任务资源的操作结果，不是全机器无进程证明。报告准确说明数据库/构件/日志保留以及 browser registry 24 小时 TTL 的回收限制。审计未重新操作这些资源。

对 7 份用户交接文档作了有界 JWT、API-key、Bearer 长值和敏感 env 赋值模式检查，未发现匹配值，没有输出任何凭据。该检查不等于对所有二进制 trace/截图作了完整秘密扫描；原始证据应继续按本地测试材料处理。

## 收尾说明

### N1：注册限流发生在三份 run，文字目前写成两次

原始结果有三个受真实注册 429 影响的 run：

- `password-22-collaboration-control-journey-web-1`：1 条 quota 错误；
- `password-26-collaboration-lane-close-controls-1`：2 条 quota 错误；
- `password-31-collaboration-retained-attachment-1`：1 条 quota 错误。

`report.md` 和 `defects.md` 的“两次触发”应改为“三份 run 受到注册限流影响”，或省略次数，避免把故障回合、失败用例数和 limiter 窗口混算。后续各身份通过与原失败保留均已核实，这个文字问题不改变 439 个最终状态。

### N2：交接进度已同步，发布边界继续保留

审计首次读取到的旧 `session-prompt.md` 仍描述 433 PASS、Linux Main/Host 尚在运行；主会话随后已在 `01:53:18 UTC` 同步。复读确认新引导语与进度均说明本轮结束、439 PASS、标签专项完成和自有环境已清理，并把下一步限定到剩余平台/真实账号条件。当前没有这项旧交接状态问题。

完成 N1 文字修正、将本审计引用写实后，可以交付测试计划、本地结果与新会话引导语。任务保留 in_progress 用于尚未完成的验收轴是诚实的状态，不应改成完整发布验收通过。
