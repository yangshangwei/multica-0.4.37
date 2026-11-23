# Research: S4 文件准备、provider 执行包与证明边界

- Query: 在不增加依赖、不搬运未提交目录/私人 session、不借普通 token/执行器的前提下，实现明确附件版本或固定 Git commit、新私有工作目录和可验证 provider/OS 支持。
- Scope: internal；当前 Runner/controller/protocol、runtime business/attachment 服务、`server/pkg/agent`、相关规范与测试；只写本文件。源码处于生产 build 冻结期，未写产品、配置、测试或规范。
- Date: 2026-10-21
- Active task: `.trellis/tasks/10-20-collaboration-gap-recovery`。

## Findings

### 1. 推荐交付顺序

按 **S4a 固定附件与准备账目 → S4b 明确 Linux 文件包及一个真实 provider 协议适配器 → S4c 固定 commit Git bundle → S4d 逐组合验收** 实施。每步能独立验证，仍以完整 G04/AC-I06 为最后验收；附件或 fake provider 单项通过不能代表整个 S4 完成。

第一个普通 provider 协议建议使用现有 Claude stream-json，而不是直接开放整个 `agent.New` 工厂。它已有明确的 argv、双向 stdin 控制请求、结构化终态和 usage 解析；用实际 Linux namespace 内的测试生成 executable 证明协议与隔离后，才声明这个配置组合支持。Codex app-server、ACP、Node/Python CLI 等仍各自需要对应测试，不能由 canonical registry 成员资格推导可执行能力。

Git 的首个可交付格式选**环境 owner 明确配置的本地 Git bundle 文件 + 完整 commit OID**。在被监督的准备子范围内验证对象并导出该 commit；无需读取原 working tree、原 `.git/config`/hooks、Git 账号、SSH agent 或联网 fetch。支持任意远程 URL/原目录自动发现不是本增量的前提。

### 2. 当前代码决定了哪些边界

| 源码 | 当前事实 / 实施含义 |
| --- | --- |
| `server/internal/daemon/collaborationrun/runner.go:74`–`:106` | Claim → 本地 Register → RequestPermit → Permit → Authorize → StopIntent → Burn → Controller.Start。business token 只有 permit 后取得；烧 nonce 前不得新增任何处理业务输入的 helper subprocess。 |
| `runner.go:49`–`:65`、`:107`–`:133` | pre-burn 恢复可证明 nonlaunch；burn 后启动/观测不确定为 unknown。OS exit、provider outcome 和 proof 分开。 |
| `controller.go:222`–`:282` | 当前只复制明确静态 agent/CLI executable，创建新 root；未准备附件、Git 或账号。 |
| `controller_linux.go:132`–`:137` | `/work`、`/tmp` 会重新挂空的 64 MiB tmpfs；预先写到宿主 root/work 的文件会被遮住，不能当成已完成物化。 |
| `controller.go:285`–`:317` | 仅允许本机架构 ELF，无 PT_INTERP；脚本/动态库需要新的显式文件包合同。 |
| `controller_linux.go:22`–`:28`、`:100`–`:102` | user/PID/mount namespace，PID 1 退出销毁后代范围。这个 kernel 事实仍是 proof 权威，PGID/stdio/`agent.Result.ProcessExited` 都不能替代。 |
| `server/internal/service/collaboration_squad.go:84`–`:105` | `lockRuntimeBusiness` 以 mcb token 检查当前 run、attempt、完整 admission 和 reservation；不得把 owner 转为人类调用者。 |
| `collaboration_runtime_capabilities.go:197`–`:242` | 首次 claim 冻结 prompt；重放读取同一 snapshot。准备 manifest 应在同一事务冻结，不能每次重试重新选择附件或 commit。 |
| `collaboration_runtime_capabilities.go:247`、`runner.go:81` | 服务端与 Runner 目前只支持 attempt 1。不能把普通 daemon 的 resume/model fallback 接进来。 |

当前 Probe 会启动**不接触业务输入、provider 账号或 Git 的固定内部 helper**验证 kernel；S4 不得将真正的文件处理/供应商启动塞进 Probe，借此绕过 Burn。Probe 的基础执行与业务准备必须保持分离。

### 3. 最小 versioned 合同

保留现有 runtime identity/proof 协议 2；增加独立协商的 `preparation_version:1` / `provider_bundle_version:1`。不要修改已经留存的 run identity、nonce 或 sealed proof 字节。以下名称为建议的新合同，尚未实现。

**本地明确执行包**（扩展 `CollaborationRunnerConfig`；本地路径和凭据来源不出机器）：

```text
bundle_version: 1
runtime_id: UUID
provider: canonical runtime ID
protocol_family: canonical family
adapter: {id, version}
files: [{source: absolute regular file, destination: absolute in-root path,
         sha256, mode: read-only | executable}]
entrypoint: in-root path
argv_prefix: adapter-approved fixed argv
account: {source: environment, env: [{name, source_env}],
          files: [{source: explicit private file, destination: provider-home file}]}
git_source?: {bundle_file: explicit private regular file,
              object_format: sha1 | sha256, commit_oid: full hex OID}
limits: {file_count, per_file_bytes, total_input_bytes, git_expanded_bytes,
         workspace_bytes, entry_count, preparation_timeout}
```

文件列表必须精确，不能解释 glob、递归复制任意宿主目录或运行 package installer。若文件较多，外层 64 KiB config 可指向一个有独立大小/条目上限的明确 bundle manifest 文件，而不是取消现有配置上限（`server/internal/daemon/collaboration.go:30`–`:45`）。

建议第一版限制为 32 个输入文件、单附件沿用 100 MiB、总输入/解包内容默认 256 MiB、工作 tmpfs 默认 512 MiB；按明确配置下调，扩大须同样有硬上限与实际资源证据。超限在准备准入时明确拒绝，不截掉附件、不默默回退到原目录。数值是建议的第一版能力参数，不是既有产品保证。

**向 Server 注册的非秘密能力摘要**：runtime/installation/binding 及代次、OS/arch、实际 namespace probe 结果、canonical provider/family、adapter version、bundle digest、准备格式、上述 limits。不含宿主路径、环境变量值、credential file 内容。推荐 `POST /api/collaboration/runtime/capabilities`，复用当前 managed-daemon 身份校验与资源事务；同摘要重复上报无副作用。

能力/执行包变化是执行配置变化：更新 `agent_runtime.metadata` 的明确 collaboration 子对象时，必须同时推进 `collaboration_config_epoch` 并走既有持久 invalidation。**只写 metadata 不够**：migration 704 的 runtime 比较字段没有 metadata（`:35`–`:45`）。原 grant 的 fingerprint 已包含 runtime epoch，因此不必制造第二种 grant。既有 runtime writer 不能静默覆盖此摘要或让任意 metadata 写入绕开代次。

**每 run 冻结准备 manifest**：

```text
version: 1
identity: existing CollaborationRunIdentity
resource: {agent_id, runtime_id, owner_user_id, execution_workspace_id,
           target_epoch, config_version, runtime_config_epoch}
provider: {provider, protocol_family, adapter_id, adapter_version, bundle_digest,
           account_source: runtime_configuration, resume: fresh}
attachments: [{id, version, name, content_type, size_bytes, destination}]
git?: {object_format, commit_oid, bundle_digest, destination: work root}
limits: admitted limits
manifest_digest: hash of canonical nonsecret manifest
```

附件集合冻结为本次 immutable prompt 中的附件 ID，以及在首次 claim 时已明确向本单交付的文件；去重后每个条目固定 metadata/version。不得在重试或续作里自动取“最新目录”替换此集合；后续 run 有自己的新快照。M11/派生 run 依其真实 prompt reader/group 身份另冻结，不继承队长的宽读取能力。

当前附件公开合同不暴露 `content_sha256`、storage key 或 encryption key；不要为校验下载而破坏它（`collaboration_attachments.go:33`–`:45`）。Server 在 `ReadAttachmentBytes` 校验真实内容；daemon 验精确 run/file/version/length，再计算仅本地保存的下载 checksum 防损坏。manifest digest 不等于附件内容 digest。

Claim request 可增加可选、严格验证的能力版本与 bundle digest；只有明确协商的新客户端接收新 manifest 字段。旧静态客户端不应在忽略文件的情况下领取需要 S4 的 run：先返回 `runtime_preparation_unsupported`，不发新许可；新客户端遇旧 Server 拒绝也不能降级普通任务执行。不要无条件给旧严格 decoder 添加响应字段。

### 4. 附件字节和准备事实的 API / schema

新增窄 business 路由，均以 `mcb_` 校验且绑定 path run ID：

| 路由 | 责任 |
| --- | --- |
| `GET /api/collaboration/runtime/runs/{runId}/preparation` | 当前 `lockRuntimeBusiness` 后返回已冻结 manifest；不创建新 snapshot、不换附件版本。 |
| `GET .../{runId}/attachments/{attachmentId}/content?version=N` | 此 file/version 必须在该 run manifest 中且仍为合法同单交付；不接收 storage key、任意 URL 或磁盘路径。 |
| `POST .../{runId}/preparation/observations` | 严格 typed、同 manifest 的 `prepared` 或准备失败事实；原 event ID + digest 幂等。只记录 stage/reason/commit/manifest identity，不是 provider outcome、permit、proof 或新授权。 |

字节 handler 可复用 `acquireCollaborationAttachmentBytes`、`ReadAttachmentBytes` 和 `serveCollaborationAttachmentBytes` 的缓冲/送首字节前重验模式（`handler/collaboration_attachments.go:295`–`:325`）。新增服务层 `RuntimeAttachment` 用 `lockRuntimeBusiness` + frozen manifest + 同单 row/version 校验，事务外取存储字节，送首字节前再次调用同一窄校验。配置 service 时要使用 `h.collaborationAttachmentService()` 提供 storage；普通 `NewCollaborationService` 本身不提供 storage。

**不能复用人类 GetAttachment 作为权限捷径。** 它通过 `collaborationNotificationViewer` 与人类 read basis（`service/collaboration_attachments.go:300`–`:321`）；也不能 mint 人类 native-download capability 给 provider。term token 对三个新路由均拒绝，已 settled/撤权 mcb 不能继续读字节或写准备记录。

元数据先在相同事务验证，存储 I/O 不持业务锁。文件 API 保持 no-store、同 Server、禁止外部 redirect、当前授权、容量上限；daemon 将数据写入自己的 private staging，使用 ID/version 确定路径，用户上传名只作展示，不能作为路径拼接。部分下载、不等长响应或状态不明不能标 ready。

**最小 DB 改动**：为 `collaboration_run` 加 nullable `preparation_snapshot jsonb` 与可选的固定 digest，首次 claim 和 prompt snapshot 同事务冻结。旧行 NULL 只表示旧路径没有该证据，不能推论没有文件需求。typed 准备/执行包来源事实可用已有 `collaboration_run_event`，无需新队列/operation 表；其 kind 目前开放（migration 571 `:112`–`:120`）。若查询仍按 run ID，用既有索引即可；任何后续新索引必须独立单语句 CONCURRENTLY、无 FK/cascade、登记 cleanup。迁移号由协调者分配。

`freezeResourceRecording` 当前只存配置的 provider/model/account source（`collaboration_records.go:26`–`:75`），不是实际账单身份。扩展其精确来源为 bundle/adapter/manifest 和实际固定 commit/附件版本；失败阶段另记，不把环境账号标签说成已验证的付费账户。

### 5. 准备时序、burn 和恢复

推荐 Runner 的具体顺序：

1. **无业务 Probe**：验证本地文件包结构/架构、所需 kernel containment；不查找用户 CLI、不调用 provider `--version`、不读账号或 Git 内容。
2. Claim 校验能力与配置，原事务冻结 prompt + preparation；本地 Register 保存 term identity。随后原 RequestPermit/Permit/Authorize，取得本 run 的 mcb。
3. **纯 Go pre-burn 准备**：下载授权附件、打开并校验明确 runtime/credential/bundle 文件、建立私有 staging；这里不得调用 `git`、tar CLI、shell、npm、ldd、credential helper 或其他 subprocess。文件包 ELF 检查使用现有 `debug/elf`；tar 解包使用 Go `archive/tar`。
4. 在开始前重验同 manifest 的 current business admission，并查 StopIntent。校验通过后才 Burn；pre-burn 的错误通过原 `abandon` 证明 nonlaunch，并保留真实准备错误，不能新造 run/nonce 重试。
5. Burn 后启动唯一受监督 namespace init。**准备发生在已记录的 Process 内**，可观察 Stop/取消；不要等长时间 Git 准备结束才返回 Process，导致启动超时后失去过程身份。先完成隔离，挂 tmpfs，再把只读 `/inputs` 中的附件用 Go copy 物化到 `/work`，因此不会被 tmpfs 遮住。
6. 若有 Git，在下节的独立准备子范围内完成；然后销毁该子范围及 Git 输入路径，创建 fresh provider home、只放账号白名单，最后再次用 mcb 重验 current admission/manifest，再启动 provider。
7. 原 Stop 观察覆盖整个外层 namespace，含 Git、provider、工具和后代。结束后先得到 kernel 范围退出事实，再 Reap/Seal/Proof；provider 结果与准备失败事实分别上报，business 拒绝不阻塞 term proof。

| 失败边界 | 可声明的事实 |
| --- | --- |
| Claim 未成功、未持本地 term identity | 不伪造 daemon proof；服务端原 never-permitted 路径负责没有 permit 的 run。 |
| Permit 响应不确定、pre-burn HTTP/Go 文件准备失败 | 没有业务 subprocess 执行，沿原相同 permit nonce 的有界重放及 pre-burn nonlaunch 收尾；不能换 key/nonce。 |
| Burn 后，controller Start/ready 结果不确定 | unknown，保留实际 slot/barrier；“只是准备 helper”不能作为例外。 |
| Git/helper 已启动、provider 尚未启动，后续准备失败 | 不能报 never_launched；只在整个已监督范围确切结束后交 exited proof，另记 preparation failure。 |
| 清理/范围退出不确定、crash 丢过程观测 | 保留 unknown；不能删 ledger/staging 后重新 fork。 |
| provider 输出失败/缺失、mcb 已撤销 | 业务记录可缺失；term proof 仍交原 run，不能把 exit 0 写成成功或自动恢复队列。 |

准备文件/manifest 本地元数据使用 installation/run 作用域、私有权限、原子写 + fsync，与 nonce ledger 分离且不被当成 run JSON。已 sealed/ACK 的 run 不再打开账号或重新准备。仅在权威已知该 scope 结束后清理自己的 staging；不按扫描跳过/文件不存在释放容量。

当前后续 attempt 尚未实现（service 明确拒绝 `seq != 1`）。首版必须声明 `fallback=false,resume=fresh`；不调用普通 daemon 的 fresh-session/model fallback。未来若支持第二个 provider attempt，须先扩展并验证原 ledger + permit + 独立 attempt nonce/前次完整退出/最终 seal；不能在同一旧许可里再次执行 provider。Git 工具子进程属于当前尝试的受监督准备，并不让它获得第二次 provider 启动权。

### 6. Linux 文件包与固定 Git 的可实施方式

**文件包**：用现有 `prepareRoot` 拷贝代码作为起点，支持明确的多个 regular files。初版可支持 native ELF + 明确 interpreter/DT_NEEDED 库闭包；更后面的 Node/Python/script 只能在解释器、脚本和所有依赖文件均明确列出并经过测试时准入。不能运行 `ldd` 推导依赖（可执行未知代码），也不能把宿主 `/usr`、`/lib`、home 或原项目整体挂入。

检查绝对 source、固定 destination、常规文件、私有可信目录/文件 owner、最终组件 symlink/替换、大小及 digest；用已打开文件复制和重验，不在检查后按同一路径重新读取可变内容。destination 不能覆盖 `/proc`、控制 FD、`.oldroot`、private ledger 或 Supervisor 可执行入口。动态 ELF 的 PT_INTERP 必须恰好映射到提供的 loader，所需库逐项验证；root 保持只读，仅工作/home/tmp 可写。

真实 provider 的 DNS/TLS、shell/tool 依赖也属于能力包：可明确提供非秘密 CA bundle、最小 resolver/hosts 配置和确需的 runtime device，不复制全 `/etc`；当前仅有 `/dev/null`，不能从 static fake 的成功推断真实网络依赖齐全。`/bin/sh`/工具缺失导致的能力不足应显式报告，不能转到宿主运行。

**Git 不是复用普通 execenv**：`server/internal/daemon/execenv/git.go:16`、`:35`、`:77` 分别用 ambient git 探测/fetch/建 worktree，存在 mutable branch/HEAD 和修改原 repository 的路径；不能从 S4 调用。检索未发现现成受控 commit archive helper，`server/go.mod` 没有 go-git；CLI update 的 tar helper（`server/internal/cli/update.go:481`–`:508`）只找一个 binary，缺少工作树级路径/总量规则，不能原样用于仓库解包。

**推荐首版 Git 实现**：

- 本地 owner 明确提供一个 regular `.bundle` 文件和完整 SHA-1/SHA-256 commit OID，随执行包摘要注册；不是从评论抽取任意 path/URL。bundle 与 commit 的改变推进 runtime 配置代次，需要原授权流程重新确认。不要读取原工作目录、index、hooks、SSH/credential helper 或原 `.git/config`。
- 把 bundle 以专用只读 FD 交给准备 helper，不让其在最终 provider root 中留下可读副本。准备 helper 在**外层 run PID 范围内的独立子 PID/mount namespace**运行；只含明确 Git executable、所需库、临时 bare repository 和 bundle，无 provider/平台 credential。
- 子 helper 先完成自身隔离/降权，再执行固定 argv 的 `git init --bare`、从该本地 bundle 导入对象、`git cat-file -t <full_oid>` 确认为 commit、`git archive --format=tar <full_oid>`。所有参数由结构化配置构造；禁 shell 拼接、short ref、HEAD、branch fallback、网络、submodule/LFS 下载与 git aliases/外部 config。使用新临时 repository、禁 system/global config 和 replace objects，不能执行仓库内脚本/hooks。
- 外层 init 用 Go 读取 tar 到工作 tmpfs：限制压缩/展开字节、entry 数和单文件；拒绝绝对路径、`..`、重复冲突名、NUL、device/FIFO、hardlink/symlink 等未支持类型。首版仅 regular file/directory；symlink/submodule/LFS 等需要额外支持的 commit 明确拒绝，不能交付一棵悄悄缺文件的树。
- 子 namespace init 完成并被回收时 kernel 销毁其全部后代，之后关闭 bundle FD、删除准备输入/临时 bare repo，再给 provider 凭据并运行。**仅等 git 直接子进程退出不够**：它的后代或遗留 FD 不能继续读额外 Git 对象。若内核不支持这个独立准备范围或清理无法证明，Git 格式保持 unsupported；不放宽外层 proof。
- 任意 Stop/crash 均由外层 PID 1/liveness 关闭整个子树；新子范围须有真实 Linux 测试，不是以一句“git 是可信工具”替代证明。固定 commit 与文件集合进入准备 observation，Git stderr 不当作业务结果。

这比在第一次收尾前启动另一个未记账宿主 Git process 更容易审查，也不要求 Docker 成为产品依赖。后续若支持本地 bare repository 或远程来源，仍须在同等隔离的准备范围中只导出明确 commit，并另有授权/配置与证据；不是本版的隐式 fallback。

### 7. Provider 适配、账号与记录

canonical provider/family 取 `agent.SupportedTypes`/`BuiltinRuntimeByID`（`agent.go:314`、`builtin_runtimes.go:103`），再与 collaboration 已验证 adapter allowlist 相交。别名如 omp→pi 只说明协议家族，不代表两个 binary 的文件/账号/隔离能力相同。

**推荐的复用边界是纯 argv/stream codec**：在 `server/pkg/agent` 内从现有 Claude 路径抽取或暴露小的 invocation/codec leaf，供原普通 backend 和 collaboration helper 共用；不返回可自行启动的普通任务执行器。已有可复用点：

- `claude.go:715` `buildClaudeArgs`，`:775`/`:786` stdin frame；`:450` control request/response 必须保持双向 stdin，不能写一帧后关闭。
- `handleAssistant` (`:367`)、`handleUser` (`:424`)、`claudeResultUsage` (`:625`) 和 `stream_json_result.go:89` 的终态校验。后者明确需要结构化 result，exit 0 不够。
- `agent.go:164` Message / `:177` TokenUsage 与 `protocol/collaboration_record.go:78`–`:108` 的受限记录结构。保持原已验证解析语义，不另写基于 stdout 关键词猜成功的逻辑。

helper 在已经进入 namespace、完成文件准备和最后 admission 重验后，用 controller 管理的唯一 Start 启动该 invocation；stdin/stdout/parser 只做协议 I/O。不能在宿主调用 `agent.New(...).Execute`，也不能默认将所有 ordinary adapters 放入 helper：它们含 PATH discovery、环境继承、MCP/session 操作和各自的进程行为。比如 `claude.go:42`–`:46` 有默认 command/LookPath，`:878` 从 os.Environ 合并；S4 必须提供明确 in-root executable 和干净环境，避免走默认分支。

首版配置只接受已经接线验证的 model、thinking、必要 instructions/运行简报和固定协议参数；resume 为空。遇无法安全表达的 `CustomArgs`、`McpConfig`、connected-app broker、hook/config、skill 依赖等，返回 `unsupported_provider_configuration`，不能悄悄忽略授权配置或复制普通 execenv 目录解决。共享业务上下文来自 RuntimeContext/冻结 prompt；agent instructions 可作为明确 prompt 部分交付，不覆盖 commit 中的用户文件。

**账号白名单**：只读本 runtime owner 明确配置的 provider API-key env 或 credential 文件快照。Claude 初始可用专门配置的 `ANTHROPIC_API_KEY`；其他名称/文件布局随对应 adapter 验证。env 为白名单重建：基础 LANG/HOME/TMPDIR/PATH、原五个 collaboration business variables、经 profile 允许的 provider 项。禁止整个 os.Environ、宿主 HOME、SSH_AUTH_SOCK、Git helpers、LD_PRELOAD/任意 loader、NODE_OPTIONS、普通 Multica PAT/JWT/task/MDT/term 等继承。

文件账号放 fresh private provider home，regular copy、0600；不链接宿主文件、不回写宿主、不复制 session/history/log/cache 或全量 config。普通 `execenv/codex_home.go:17`–`:30`、`:194`–`:229` 的 auth symlink/config/session 逻辑仅是识别文件来源的参考，不能直接复用。初版始终 fresh，不把前任务 `SessionID` 或其恢复路径送入新环境。token refresh 只影响该私有副本；支持共享刷新/换账号需另定源身份合同。

**输出/usage**：只上传受限的结构化 provider 结果与明确记录字段，经 `ValidateCollaborationProviderResult`；遵守现有 2 MiB envelope、256 items、64 KiB/item 等上限。raw stderr、credential/config、任意原生 JSON 不进事件或日志。缺少终态/输出被截断时标不完整，proof 仍独立收尾。

不要直接把 `agent.TokenUsage` 的默认 0 填满所有字段：该类型会混淆缺失与零，部分 cost 来源不同。codec 必须保存原字段 presence，并把明确观测值转 exact decimal strings；未报告字段保持 null，不估算 tokens/cost。需要精确 cost 时从原 JSON number/字符串解析，不能先经 float64 再假装精确。无法取得该字段时保留未知，不能为了“有 usage”编造数值。

Linux user namespace 内 euid 为 0；Claude 的现有 root 检查（`claude.go:882`–`:886`）会拒绝绕过权限模式。仅在真实 namespace/降权已建立后使用其明确 sandbox 环境标记，不能在宿主靠标记跳过安全条件；相关 argv/环境行为要有 fake-native 测试。

### 8. Windows/macOS 与测试证据

Windows/macOS 本版保持 `ErrUnsupported`，且声明准确原因。`controller_unsupported.go` 明确关闭非 Linux，服务端也限制 installation OS=linux（`collaboration_runtime.go:58`）。不得只改那个 if 或给普通 executor 加一个新字符串便宣称支持。

Windows 可参考 `server/pkg/agent/proc_windows.go:80`–`:100` 的 suspended-create→Job 绑定及 `:146` kill-on-close、ActiveProcesses 观测；它还不是 collaboration 的受限文件系统、凭据、重启 ledger、nonce/seal 和 proof 接口。macOS 普通进程组/后代轮询没有同等闭合证明；在这些缺口补齐前不执行。

建议测试均使用临时私有文件、测试生成 executable、httptest 和隔离 DB，不需要真实 provider 登录或额度：

| 主位置 | 必須证明 |
| --- | --- |
| protocol/config + collaborationrun 新 preparation/bundle tests | manifest/version/identity/digest 关联；声明过大/缺 dependency/架构不符/路径逃逸/symlink/credential 越界/未支持 provider 均在业务 fork 前拒绝。 |
| service/handler runtime attachment tests，复用 `collaboration_attachments_test.go` | mcb 本 run/manifest/version 可读；跨 run/issue、term、普通 task、人类替身、撤权/结清拒绝；slow object read 后重验；私密 storage/key/hash 不出 API。 |
| `runner_permit_test.go`、`runner_test.go`、`ledger_test.go` | 准备失败和失响应分在 pre-burn/burn 后；相同 nonce、无第二次 fork、原 proof 重放；crash/unknown 不释放容量。 |
| `controller_linux_test.go:176`、`:245` 扩展 | 附件实际出现在 tmpfs 工作目录；只读文件包、fresh home；工具 setsid/double-fork/早关 stdio 仍被 namespace 销毁；宿主 secret/term/账号外文件不可读。 |
| `runner_linux_test.go:24`、`:130` | 生产 Runner + HTTP + fake provider，附件准备/运行/撤权/Stop/crash/proof 完整连线；明确 provider 失败与 OS proof 互相独立。 |
| 新 Git preparation native tests | 临时 repo 的固定 commit bundle；工作树有未提交 sentinel、后来提交/branch 变动仍只得指定 commit；拒绝坏对象/缺 commit/过量/链接/子模块格式，helper 后代和对象 FD 不延续到 provider，Stop 覆盖准备阶段。 |
| `server/pkg/agent` 现有 stream-json tests + 新 codec tests | argv、stdin 控制请求、结构化终态、真零/字段缺失、精确 cost、bad/truncated JSON；没有发现或执行安装的 agent CLI。 |
| core/views record schema/页面 | 当前 run 的真实 provider/bundle/commit/file version、准备失败原因和未知用量；旧字段缺失不猜配置、不泄露本地账号路径。 |

验证命令草案（**未运行**，从 `server/` 执行；Linux 项必须在真实 Linux 环境，不能在 macOS skip 后记为通过）：

```bash
GOTOOLCHAIN=go1.26.6 bash ../scripts/go-test-with-agent-cli-guard.sh go test ./pkg/protocol ./pkg/agent ./internal/daemon/collaborationrun -run 'Collaboration|Preparation|ProviderBundle|Stream|Claude' -count=1
GOTOOLCHAIN=go1.26.6 bash ../scripts/go-test-with-agent-cli-guard.sh go test -race ./internal/service ./internal/handler -run 'CollaborationRuntime.*(Preparation|Attachment)|Collaboration.*Resource' -count=1
MULTICA_REQUIRE_COLLAB_NAMESPACE=1 CGO_ENABLED=0 GOTOOLCHAIN=go1.26.6 bash ../scripts/go-test-with-agent-cli-guard.sh go test ./internal/daemon/collaborationrun -run 'TestLinux' -count=1 -v
GOTOOLCHAIN=go1.26.6 go vet ./internal/daemon/collaborationrun ./internal/service ./internal/handler ./pkg/agent ./pkg/protocol
```

static 测试自身复制 executable 的模式见 `controller_linux_test.go:24`–`:46`；动态 ELF/interpreter 闭包须另有实际非 static fixture 测试，不能将上面的 CGO_ENABLED=0 当成动态加载证据。`MULTICA_REQUIRE_COLLAB_NAMESPACE=1` 会将缺少真实 kernel 能力变为失败；目标验收必须检查没有 skip。

**普通 provider 支持与付费 smoke 分开**：fake executable 发出对应 provider 真正的 wire 格式，native namespace/文件/账号白名单测试可验证适配实现；真实 binary 在实际环境中的完整依赖还需明确配置与验证。访问真实账号/消费额度的 smoke 始终需要另行授权，并保留 `agentintegration` + `MULTICA_RUN_REAL_AGENT_SMOKE=1` 双门禁（`server/pkg/agent/real_agent_smoke_integration_test.go:1`–`:15`）。本次没有这项授权，也没有执行。

## Files Found / Related Specs

- `server/internal/daemon/collaboration.go` / `config.go`：本地 runner config、安装作用域、controller 选择；扩展显式 bundle 配置。
- `server/internal/daemon/collaborationrun/{controller,controller_linux,controller_unsupported,runner,ledger,client}.go`：受控文件包、准备阶段、business bytes、进程观察与原 proof 边界。
- `server/pkg/protocol/collaboration.go` / `collaboration_record.go`：保持现有 identity/proof；增加独立 versioned preparation/capability types。
- `server/internal/service/collaboration_{runtime,runtime_capabilities,squad,attachments,records}.go`：当前 admission/claim snapshot、run-bound 文件、准备/来源事实；不得借 human attachment getter。
- `server/internal/handler/collaboration_runtime_business.go` / `collaboration_attachments.go` / `server/cmd/server/router.go`：窄 mcb 路由、真实 storage adapter、二次字节授权。
- `server/pkg/db/queries/collaboration_runtime.sql`、新加法 migration：run preparation snapshot；其余复用现有 attachment/run-event/epoch/outbox。
- `server/pkg/agent/{agent,builtin_runtimes,claude,stream_json_result,stream_scanner,proc_windows}.go`：canonical family、可复用 codec/usage、Windows Job 参考；不是普通任务执行 fallback。
- `server/internal/daemon/execenv/git.go` / `codex_home.go`：已有实现的复用限制；没有安全的原目录复制/commit-archive捷径。
- 规范：`.trellis/spec/server/collaboration-targets.md`（实际容量/nonce/proof/明确 Resume）、`collaboration-runtime-cli.md`（窄 CLI/显式 executable）、`collaboration-content.md`（附件版本/字节/隐私）、`collaboration-records.md`（明确输出和精确用量）、`collaboration-switch-and-skills.md`（配置变化及 owner consent）；父 `research/execution-contract.md:17`、`:88`、`:104`–`:110`、`:165`、`:178`。
- 修改运行字段/行为时同步 `server/internal/service/builtin_skills/multica-working-on-issues/references/collaboration-workflow.md` 及对应 source map，主会话更新规范。此研究没有修改这些文件。
- External references: none；结论依据当前仓库。未联网查供应商账号、未执行 Git 命令、未探测安装的 provider、未读取实际凭据。

## Caveats / Not Found

- 本研究没有产品、数据库、Linux/native、动态 ELF 或 provider 的新运行结果；当前静态 Linux 源码/历史测试不代表 S4 已支持一般 CLI。
- 可立即实施 schema、附件端点、纯 Go 准备、显式 bundle 与 fake/native codec 测试。没有必须重新讨论的产品选择；初版 unsupported 组合要保留在验收矩阵中，不能降低 G04/AC-I06 或声称全部 provider/OS 完成。
- 本研究尚未取得或核实的**实际运行输入**是：某个支持组合的明确 executable/loader/library/CA/tool 文件包、owner 选定的账号来源、Git bundle + 完整 commit OID，以及通过必要 kernel probe 的 Linux 验证环境证据。这不等于断言环境中没有这些实物；也不能用 factory 构造函数或宿主 PATH 猜出来。它们不阻塞用测试 fixture 开始实现，但未确认前不能宣称该真实组合端到端可用。
- Git 子范围与 provider codec 仍是必要的新实现，不是已存在 helper。若具体 Linux 环境不能封闭准备 helper 的后代，Git 准备必须保持 unsupported；Windows/macOS 同理。真实 paid smoke 没有授权，须与通用 adapter 验证分别报告。
