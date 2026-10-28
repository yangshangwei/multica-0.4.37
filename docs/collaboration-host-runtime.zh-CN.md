# 协作普通本机执行：配置与验收

新协作执行复用 Linux、macOS、Windows 上的普通智能体 backend，沿用所选运行时的账号和工具权限。每个协作身份拥有独立会话、工作目录和执行临时目录；目录分离不构成操作系统沙箱。

此功能默认关闭。代码、交叉编译、测试 CLI、真实 Provider 和安装包的验收分别记录；开放范围以 [任务验收记录](../.trellis/tasks/10-24-cross-platform-collaboration-runtime/verification.md) 为准。

## 升级顺序

1. 先部署支持 runtime v3、执行视图 v6 和迁移 738 的服务端。保留旧 v2/namespace 收尾能力与历史记录。
2. 更新 Web/Desktop，使其读取时声明 `execution_view_version=6`，新 AI 操作保存同一版本声明。旧请求重放仍使用原载荷。
3. 更新 daemon，并为已验收的本机运行时配置 host 执行。Provider 的名称、实际版本、本地模式和托管配置指纹必须与注册记录一致。
4. 从 namespace 切换到普通权限，或托管权限配置变化，会使原授权失效。实际资源拥有者在每个受影响任务重新确认一次。等待输入保留，重新授权不自动恢复。
5. 验证相应平台组合后再开放 host 门禁。开启后，新执行组不能再使用 namespace 根执行；已领取的旧 Linux 执行组及其必要续作继续按原模式收尾。

`collaboration_execution_gate.host_execution_enabled` 默认是 `false`，与创建/执行门禁分别生效。发布操作必须同时维护既有门禁版本，不以删除旧 run、许可或 unknown 记录代替切换。

## 创建与执行门禁

超级管理员在管理后台 **设置 → 部署设置 → 协作** 中开放或关闭“新建协作任务”，并开放“AI 执行”。每次修改都要填写原因，并记入管理审计；平台观察员只能查看。

- 只开放新建、未开放 AI 执行时，成员可以建任务，但智能体不会运行。
- 开放 AI 执行会提升执行 epoch，且要求请求带上管理员看到的 epoch，期间若有人关闭过执行，请求会被拒绝并留下回执。开放不会改写任何队列：已暂停的队列需由参与者恢复，但仍处于活跃状态的队列会立即继续。若曾用 SQL 直接关闭执行，开放前先确认没有需要保持停止的活跃队列。
- 暂不支持在界面关闭 AI 执行，因为安全关闭需要先持久地停止所有进行中的执行（执行契约 E06）。
- 执行方式（`host_execution_enabled`）在此页只显示当前值，切换仍按上文的升级顺序执行。
- 协议升级迁移可能把这些开关重置为关闭，例如迁移 620。升级后请回到此页确认开关状态。

## Daemon 配置

沿用现有 `MULTICA_COLLABORATION_RUNNER_CONFIG` 配置入口。下面是结构示例，UUID、版本、账号来源标识和路径须取自该安装及运行时，不能直接复制占位值运行。

```json
{
  "runtime_id": "<registered-runtime-uuid>",
  "expected_runtime_config_epoch": "<current-epoch>",
  "ledger_directory": "<profile-home>/.multica/management/<deployment-uuid>/collaboration-runs/<installation-uuid>",
  "workspace_directory": "<dedicated-collaboration-directory>",
  "host": {
    "provider": "claude",
    "provider_version": "<verified-installed-version>",
    "provider_mode": "local",
    "account_source": "<stable-account-configuration-identity>",
    "limits": {
      "file_count": 32,
      "per_file_bytes": 104857600,
      "total_input_bytes": 268435456,
      "workspace_bytes": 536870912,
      "git_expanded_bytes": 268435456,
      "entry_count": 65536,
      "timeout_seconds": 600
    }
  }
}
```

host 执行从 daemon 已注册的普通运行时取得可执行文件和配置，不接受用协作输入选择另一个程序。`provider_version` 必须匹配已检测版本。`account_source` 表示稳定的账号配置来源，不填写 token、密码或账号文件内容；普通 token 刷新不构成新的授权事件。

每个安装的多个 profile 共用规定的账本目录。不要复制或清空账本来解决“执行状态未确认”；这会丢失防重复和会话占用证据。工作目录另设，不能混入旧 `sandbox_directory`、`bundle`、`executable` 或 `arguments` 配置。旧配置只用于切换前兼容与已有 Linux 执行的收尾，不能成为 host 失败后的 fallback。

当前生命周期适配器覆盖 Claude、Codex、OpenCode 的本地执行接口。远端/gateway 模式、未经验证的自定义 runtime，以及尚未映射的 Agent 配置明确拒绝。模型、指令、推理级别使用领取时冻结的配置；准备阶段拒绝不支持的选项，不能悄悄忽略或改用其他 Provider/账号。普通宿主配置中的工具和账号仍遵循已经同意的本机信任范围，不进行全机配置监测。

Git 输入可在 `host.git` 中声明本机 `executable`、`bundle_file`、`bundle_digest`、`size_bytes`、`object_format` 和完整 `commit_oid`。只导出声明的提交；私有未提交/未跟踪文件不迁移。每个实际 Git helper 都取得独立许可并完成真实等待。附件仍通过本单、本次执行的凭据和固定版本下载。

## 停止和恢复

| 事实 | 含义 |
| --- | --- |
| 业务成功/失败 | Provider 的明确结果；退出码或输出流关闭不代替业务结果 |
| `host_exit_observed` | 所有已登记的受管理启动都完成真实生命周期观察，且不再允许 fallback |
| `tracked_scope_complete` | 仅已记录范围内的清理得到确认 |
| `best_effort` | 已尝试清理，未作全后代保证 |
| `unverified` | 后台清理情况未验证 |
| `unknown` | 启动或等待仍有疑点；保持恢复和自动重跑屏障 |

停止先封闭所选范围的新派发，再请求取消已启动执行。断网时本机执行可能继续；没有新的服务端许可，不能启动重试或派生成员。重连仅同步原记录和回执，不自动恢复。

Windows 的原生可执行文件可使用已声明的直接进程降级，并如实记录清理范围。shell/batch wrapper 的真正 Provider 位于其子进程中，因此必须在恢复 wrapper 前绑定 Job，并确认受管理子执行结束；Job 绑定失败时结束尚未恢复的 wrapper，不以 wrapper 退出冒充 Provider 结束。

结果记录和执行结束分别处理。结果上传失败不能阻挡可靠的结束回执；缺失结果显示记录不完整。服务端已保存的同一结果可按原 token、事件和完整载荷确认 ACK，不能借此新增结果、追加用量或复活执行。

## 验证与回退

CI 的 `collaboration-host-native` 作业在 Linux、macOS、Windows 运行测试创建的 Provider 和本机进程/文件锁故障用例，并保留 JSONL 证据。关键用例跳过、没有匹配测试或测试失败都会阻止该作业通过。实际 OS/arch 保存在证据中，不能把交叉编译称为原生验收。

真实 Provider 测试可能访问已登录账号并消耗额度，按根目录 `CLAUDE.md` 的独立授权与 `agentintegration` 开关执行；默认测试不调用用户安装的 Agent CLI。

回退首先关闭新 host 准入，继续处理已有 host/v2 记录与回执。存在新模式数据时，不回退到不能读取这些状态的服务端，也不执行会删除这些记录的 down migration。
