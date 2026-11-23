# 单智能体运行端续作（2026-10-17）

## 变更边界与实施计划

当前普通 daemon 进程组停止不能证明协作运行的全部后代已经退出，也没有跨进程的 nonce/seal 账本。本次在 `server/internal/daemon/collaborationrun/` 实现独立的持久运行监督器和窄 HTTP 客户端；`server/pkg/protocol/collaboration.go` 固定与服务端共享的 DTO；CLI 入口仅增加可信 namespace init 的内部 re-exec 接线。

1. 与授权线固定 run/attempt 身份、claim/permit、停止读取、结果与证明 DTO。
2. 先测试 nonce 重放、另一进程、持久化失败和 seal，再实现私有目录、文件锁、原子写及文件/父目录 fsync。任何 burn/fork 结果不明均保留 unknown。
3. Linux 控制器在子程序执行前建立 user/PID/mount namespace，切入明确提供的私有根目录并重新挂载 proc，清除 capabilities、设置 no_new_privs。namespace init 的内核终结是封闭范围的证明来源；PGID、PID 轮询及 stdio 关闭不作证明。
4. 假可执行文件验证 setsid、双 fork、提前关闭 stdio、停止与重启，不访问供应商 CLI 或账号。macOS/Windows 明确拒绝运行能力；Linux 证据注明 Docker 内真实 Linux 内核，不冒充 macOS 原生测试。
5. 接线单智能体 claim→permit→启动→停止/结果→seal→proof；凭据只分配给其用途，终结 token 不进入子程序环境、参数、根目录或 stdin。

仅处理运行端与协议，不修改 service、handler、SQL、迁移或前端，不接入小队派发、自动 fallback、生产启用或新依赖。不会重构已有普通 provider 路径；单智能体完整 DB/API 闭环由主会话集成验证。

## 已实现的接线

- `protocol/collaboration.go` 固定 protocol 2 的 claim/list/permit/outcome/stop-intent/proof DTO。每条输入保留 command、作者、participant/account/lifecycle epoch、正文和附件 ID；run 身份包含 deployment/installation/issue/session/target/channel/batch/group/run、fence、claim generation、launch nonce。attempt nonce 独立。
- `collaborationrun.Client` 只用明确的 managed MDT 进行 list/claim/permit；outcome 使用业务 token，Stop/proof 使用预发终结 token。拒绝 redirect、重复 JSON 字段、缺失停止谓词及任一输入作者的无效代次。错误不包含私有响应。
- `collaborationrun.Runner` 依次执行 capability probe、claim、持久 permit 请求、许可、Stop 重验、持久 nonce burn、namespace 启动、真实等待、reap、显式业务结果、seal、持久 proof、HTTP 确认。退出码 0 不生成成功结果；业务 401 不阻塞最小 proof。丢失 proof ACK 只重发同一 receipt。重启遇到不确定 permit/burn/fork/身份写入阶段拒绝重新 claim/fork。
- Linux helper 在执行用户程序之前进入 user/PID/mount/IPC/UTS namespace，挂载新 proc，pivot root 并卸载旧根，使用只读程序根和独立 tmpfs 工作目录，清空能力集、锁定 root capability 规则、设置 no_new_privs 和 non-dumpable。子程序仅有业务 token；配置管道关闭，父进程 liveness FD 标记 close-on-exec。namespace init 的内核销毁是证明来源。
- daemon 通过现有机器 semaphore 在 claim 前取得槽位；unknown 保留槽位和自动升级屏障，重启扫描账本仍计占用。已 sealed proof 可在无 MDT 的窄客户端重试。普通 poller 的默认路径保持原样。

明确配置入口为 `MULTICA_COLLABORATION_RUNNER_CONFIG` JSON，字段是 `runtime_id`、`executable`、`arguments`、`ledger_directory`、`sandbox_directory`。目前只支持一个指定 runtime。`executable` 必须是本机架构的静态 Linux ELF，脚本、动态 loader 和 PATH 查找均拒绝；仍须实际 namespace probe 通过，不能仅凭 GOOS 宣告能力。managed daemon 的 `ledger_directory` 必须等于 `<OS home>/.multica/management/<deployment_id>/collaboration-runs/<installation_id>`，避免两个 profile 选不同账本绕过 nonce 墓碑；私有父目录由既有 installation 身份拥有。默认不启用。供应商普通 CLI、私人账号目录和自动 fallback 未接入。

## 运行证据

证据根目录：`/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-collab-content-20261017-zcB63m`，不提交其中的环境或凭据文件。

| 检查 | 实际结果 |
| --- | --- |
| `GOTOOLCHAIN=go1.26.6 go test -race ./internal/daemon/collaborationrun ./pkg/protocol -count=1`（server 内） | 通过；运行于 macOS arm64，包含本机不支持 namespace 的明确拒绝、私有账本/跨进程排他、重放/未知阶段、最小 HTTP 协议与 proof 丢 ACK 恢复 |
| `GOTOOLCHAIN=go1.26.6 bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race ./internal/daemon ./cmd/multica` | 通过；daemon 66.824s，CLI 2.564s；无供应商 CLI guard 命中；日志 `runtime-daemon-race.log` |
| `GOTOOLCHAIN=go1.26.6 go vet ./internal/daemon/collaborationrun ./internal/daemon ./cmd/multica`（server 内） | 通过 |
| `GOTOOLCHAIN=go1.26.6 GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go test -c ... ./internal/daemon/collaborationrun` | 交叉编译通过；未宣称 Windows 原生验证或运行能力 |
| Linux arm64 全部运行端测试 | 通过；唯一 skip 是仅适用于非 Linux 的平台拒绝测试，全部 Linux 正向 containment 用例实际运行；日志 `runtime-linux-final.log` |

Linux 正向验证使用 Docker 提供的真实内核 `6.12.68-linuxkit #1 SMP Mon Feb 2 10:12:50 UTC 2026 aarch64`，镜像 `node:22-bookworm`，image ID `sha256:8a34c4ab3ea2c5cd194f07e317b2a8f09461d3c8b05c4e34c8ccd56d56024c4d`。创建参数为 `--network none --privileged -e MULTICA_REQUIRE_COLLAB_NAMESPACE=1`；没有挂载宿主源码、账本或账号目录。该环境验证 Linux 内核，不能称为 macOS 原生进程证明。

复现步骤（从 server 目录构建；container ID 使用本次新建容器的真实返回值）：

```bash
GOTOOLCHAIN=go1.26.6 GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go test -c -o <evidence>/collaboration-runtime-linux.test ./internal/daemon/collaborationrun
docker create --network none --privileged -e MULTICA_REQUIRE_COLLAB_NAMESPACE=1 node:22-bookworm /runtime.test -test.v -test.timeout=60s
docker cp <evidence>/collaboration-runtime-linux.test <container_id>:/runtime.test
docker start -a <container_id>
docker rm <container_id>
```

最终 Linux binary SHA-256：`820e8fba4e63e7ed62687151c3323be0978f4a4738edf52e226acff74575aee2`。最终 Linux 日志 SHA-256：`a80171543e7ed916f05fa58d41675c038a29e0303c2083a463d75d243afc92f0`。

正向用例覆盖 setsid、double fork、stdio 提前关闭、显式 Stop 和根进程自然退出、降权/宿主文件与 FD 隔离；还实际终止监督进程，覆盖 fork 后及身份持久化后崩溃，证明后代已结束的同时确认账本仍保持 unknown、不 refork。HTTP 配套用例用真实 namespace 执行两条不可变输入，覆盖显式 outcome、退出码无 outcome、许可后 Stop 在 fork 前胜出、业务 token 被拒后的终结 proof。

## 集成责任与剩余范围

本代理未修改 SQL、迁移、服务端授权、评论或前端，未提交。运行端 HTTP 用例使用测试服务器；真实 PostgreSQL/API 的评论→批次→claim/permit→Stop→proof→Resume→下一批由主会话整合服务端后验证，不能以本处 HTTP 用例替代。未验证 Linux 原生 race（上述 race 在 macOS），未运行原生 Windows；macOS/Windows 运行能力均明确不可用。Git/附件文件准备、一般供应商适配、enrolled legacy 启动、小队、完整 Web/Desktop 和启用继续留在主任务后续范围。
