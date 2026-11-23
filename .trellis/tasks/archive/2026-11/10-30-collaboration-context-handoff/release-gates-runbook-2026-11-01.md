# 父任务剩余发布门：可执行步骤与证据要求

> 2026-11-02 当前执行补记：集成快照4085e5b38，产品候选639bf6ef8；审计/框架/交付及memory/Transfer测试修复已独立复核合流。最新完整check实际exit2，不能继承或拼接定向通过结果。当前同源Mac app/ZIP/DMG已本地构建复核；Linuxarm64 Host230、namespace7、目录包完整Main1已实际通过有界范围，仍缺deb/RPM、升级、Windows原生和真实账号。RKI6静止环境fake normal/race各62pass，结果独立复核待429恢复；runtime25消耗0，原冻结另余14/E3aftertrial2，知识K1–K3/E0b/安装账号/AC43另需预算。原生焦点原断言已重验1pass；v5适配器独立复核暂受429阻断。下一完整check需有效GUI、独立复核、APImax10/min1及无额外API启动。只允许本地准备，不运行外部CI/推送/部署/发布/真实Provider。

以下为2294654a2历史调查和执行清单，其中旧版本/未实现/760上限/19次/22次等不作为当前入口。当前迁移上限763、知识默认false，机器结果以[父矩阵](acceptance-2026-11-01.json)为准。

历史调查状态：**当时仅准备清单，未执行**。本页核对基线 `2294654a2`，只做了只读调查：没有推送、触发 CI、构建安装包、运行 DB 套件或调用真实 Provider。它不关闭任何验收门，父矩阵中的 Windows、安装/升级、多账号额度及发布状态保持不变。

## 本次核对得到的事实

| 事实 | 依据 |
| --- | --- |
| 当前 `ci.yml` 已有三系统 `collaboration-host-native` 作业（`.github/workflows/ci.yml:613-659`），另有 `windows-execenv`；只在 push/PR 到 `main` 时触发 | `ci.yml:3-7` |
| 远端 `origin/main` 停在 `dd87e0b33`，没有 `collaboration-host-native`；`origin/codex/cross-workspace-collaboration` 停在 `c58f79874`，比本地 HEAD 少 72 个提交 | `git rev-list --left-right --count` |
| 10-27 准备的验证快照 `92bb6d86`（`codex/verify-collaboration-host-runtime-20261026`）不是 HEAD 的祖先，落后 192 个提交，不能代表当前源码 | `git merge-base --is-ancestor` |
| 仓库 `yangshangwei/multica-0.4.37` 为 **PUBLIC**；任何推送都会公开对应源码 | `gh repo view` |
| 快照到 HEAD 之间，Windows 专用文件（`ledger_windows*.go`、`launch_lifecycle_windows_test.go`、`launch_instance_windows.go`、`proc_windows.go`）没有变化 | `git diff --stat 92bb6d86 HEAD` |
| `collaborationrun` 的 `TestHost*` 从快照的 26 个增至 79 个，新增 53 个覆盖 handoff/checkpoint/recording/knowledge/memory；四个原生包合计 107 个 | 测试函数名对比 |
| 新增 checkpoint/recording 状态写入同一 ledger 记录，经 `protectNewPrivateFile`/`replacePrivateFile`（`collaborationrun/ledger.go:436,451`）落盘；Windows 实现为 ACL 私有文件与替换（`ledger_windows.go:101-187`）。这些重启恢复路径从未在原生 Windows 执行 | 源码 |
| 新 Host 测试的 fake Provider 是测试二进制自身（`host_runner_test.go:134` 的 `os.Executable()` + `TestMain` 分派），不依赖 `/bin/sh`，可在 Windows 原生运行 | 源码 |
| 原生检查器只强制 11 个共享 + 5 个 Windows 用例（`scripts/check-collaboration-native-tests.mjs:9-30`）；新增 53 个用例会随 `-run '^(TestHost|TestCollaborationHost)'` 运行，失败会使作业失败，但不在必需清单中 | 脚本 |
| GitHub Release `v0.6.7`（Latest）只有 3 个 changelog 资产，没有 Windows 安装器；`v0.6.6`、`v0.6.1`、`v0.5.1` 有 `windows-x64.exe` 及 `.sha256` | `gh release view` |
| 本地 `dist/release/0.6.7/` 的 `multica-desktop-0.6.7-windows-x64.exe`（`80570e09…1749859`）与 `multica-server-upgrade-v0.6.7-linux-amd64.tar.gz`（`cb963621…e4079d64`）与各自 `.sha256` 一致；源码 `9caea3580`，本地离线交付 | `shasum -a 256` |
| `desktop-smoke.yml` 的 Windows 业务验收基线固定为 run `36996436086` 的 `0.5.2-rc.11.1`，构件有效期至 2026-12-31 | `desktop-smoke.yml:206-218`，`gh api` |
| Windows 业务验收脚本（`apps/desktop/scripts/windows-business*`）没有协作内容；安装版协作 host 旅程在 Windows 上无现成 harness | `grep` |
| `docs/offline-upgrade.zh-CN.md` 仍写 “V0.6.6 升级范围”、迁移上限 570，未提 `MULTICA_KNOWLEDGE_CONTEXT_ENABLED`；HEAD 迁移上限 760，自 v0.6.7（570）新增 154 个编号 | 文档与 `server/migrations/` |
| 服务端只把账号来源记为 `account_source=runtime_configuration`（`collaborationrun/host_config.go:98`），不保存供应商账号身份；账号与额度事实只能在各执行机和供应商侧取得 | 源码 |

## G1 Windows x64 原生 host 与 ledger

目标：在原生 Windows 上执行 Host lifecycle、ACL/Job 以及新增 checkpoint/recording/knowledge 的落盘与重启恢复。交叉编译只能证明能构建。

本地前置（可逆，等真实模型窗口结束后再跑，避免与付费实验争用资源）：

1. 在独立 detached worktree 固定候选提交，记录 HEAD、`git status --porcelain` 为空、`go version`。
2. 交叉编译：`GOOS=windows GOARCH=amd64 go -C server vet ./pkg/agent ./internal/daemon/collaborationrun ./internal/daemon ./internal/cli`；对同四个包 `go test -c -o /dev/null`。不要用 `go test -run '^$'` 代替编译检查。
3. 在 macOS 上用同一过滤器跑一次，记录总时长，判断 Windows 作业的 `-timeout=5m` 是否够用。现有证据只有子集时长（joint race 22.1s、tagged 14.5s），没有完整过滤器的数字。
4. 可选：把 `TestHostHandoffCheckpointRestartRecoversWithoutExecution`、`TestHostRecordingSealReplaysBeforeAdvancingAcknowledgement`、`TestHostCheckpointLedgerFitsMaximumEscapedRecordingAndClaim`、`TestHostRecordingRecoveredUnflushedTailRemainsUnknown` 加入检查器的 Windows 必需清单，并补 `check-collaboration-native-tests.test.mjs` 回归。这些用例没有 OS skip，加入后能把“未运行”变成显式失败；属于 CI 门禁变更，需单独提交。

执行（对外可见，**需用户授权**）：

- 方案 A：推送当前分支并向 fork 的 `main` 开 PR。`ci.yml` 的 pull_request 运行使用 PR 合并引用中的工作流定义，会执行当前的 `collaboration-host-native` 与 `windows-execenv`，不需要另建工作流文件。
- 方案 B：基于候选提交重建仅验证用的快照分支（复用 `92bb6d86` 的工作流文件形态）并推送。只在不希望开 PR 时使用。

判定：Windows 作业结论 `success`；检查器输出 `Windows: N required native collaboration cases passed.`；`tests.jsonl` 无 `fail`，必需用例及其子用例无 `skip`；`runtime.txt` 为 `windows amd64`。超时或 runner 故障记为环境无效，保留原记录；不改超时后挑选成功轮次。

证据：run URL/ID、head SHA、runner 镜像版本、`runtime.txt`、`tests.jsonl` 的 SHA-256、过滤器内 pass/fail/skip 计数、所用必需清单的提交。

不覆盖：Windows arm64（托管 runner 只有一种架构）、安装版、真实 Provider 账号。

## G2 Windows x64 安装、升级与自动更新

现有入口：`desktop-smoke.yml` 的 `workflow_dispatch`，说明见 `apps/desktop/scripts/WINDOWS_ACCEPTANCE.md`。`windows_acceptance=true` 跑 16 个 managed identity/transport 原生用例、打包、旧版→候选升级、卸载/重装；再加 `windows_business_acceptance=true` 跑登录、保留状态的升级和 updater 探针。

运行前必须处理的参数问题：

- `previous_tag` 默认 `v0.5.1`。若改用 `v0.6.7`，下载步骤会失败，因为该 Release 没有安装器。可选 `v0.6.6`（已有 exe 与 `.sha256`），或先把本地已校验的 0.6.7 安装器及 `.sha256` 上传到 `v0.6.7` Release（对外操作，需授权）。
- `candidate_version` 默认 `0.5.3-rc.<run>.<attempt>`，低于 0.6.x 基线，而工作流要求 previous 版本更低。须显式传入更高的版本，例如 `0.6.8-rc.1`。
- `windows_business_acceptance=true` 时忽略 `previous_tag`，固定使用 `0.5.2-rc.11.1` 基线。验证“v0.6.6/0.6.7→候选”和“业务状态保留”需要两次 dispatch：一次 `business=false` + 所选 `previous_tag`，一次 `business=true`。
- 未提供签名密钥时只记录未签名状态；`require_signed=true` 必然失败。

示例（需授权，且分支已推送）：

```bash
gh workflow run desktop-smoke.yml --repo yangshangwei/multica-0.4.37 \
  --ref codex/cross-workspace-collaboration \
  -f windows_acceptance=true -f windows_business_acceptance=false \
  -f candidate_version=0.6.8-rc.1 -f previous_tag=v0.6.6
```

判定与证据：lifecycle/update/business 三份 JSON 报告、run ID、head SHA、候选版本、安装器 SHA-256、签名状态。

缺口：现有 Windows 业务验收没有协作内容。Windows 安装版控制协作 host（对应 Linux/macOS 的 `e2e/collaboration-host-installed.spec.ts`）需要新 harness，属于新增实施工作，本页不把它算作可直接执行的步骤。

## G3 服务端离线升级与新旧版本混用

现状：知识迁移 747–760 的 fresh/upgrade/reapply/down 合同在测试库中通过；746 演练属于原修复版本。还没有用离线升级包把真实形态的 v0.6.7 部署升级到当前候选。

步骤：

1. **先改文档**：`docs/offline-upgrade.zh-CN.md` 的升级范围表改为当前候选版本，写明从 v0.6.7/v0.6.6 起的迁移区间（上限 760）；新增 `MULTICA_KNOWLEDGE_CONTEXT_ENABLED`（默认 `false`，只有字面值 `true` 才开启，`docker-compose.selfhost.yml:124`、`.env.example:672` 已透传）。按 7.6 节清单写明认证方式保持不变、“新建协作任务”开放而“AI 执行”关闭。
2. 在 detached worktree 按 `scripts/build-offline-upgrade.sh` 打包；解包确认 `README.md`、`操作文档.md` 与源文档逐字节一致；记录镜像 digest 与 CLI 版本戳。
3. 演练：用已校验的 v0.6.7 升级包部署一个带代表性数据的环境（设备模式、密码模式各一套），再用候选包升级。检查 `schema_migrations` 到 760 且无失败行、`/health`、`/healthz`、`/api/config`、原认证方式登录、`collaboration_execution_gate`（`creation_enabled=true`，`enabled`/`host_execution_enabled` 保持原值）、知识开关为 false。然后重复执行一次升级，确认幂等。临时目录放在 `/Volumes/artisan/code/` 下，Docker Desktop 不共享 `/var/folders`。
4. 新旧版本混用：
   - 新服务端 + v0.6.7 Desktop/CLI：按 runtime 设计（`10-30-collaboration-context-handoff-runtime/design.md:112`），未知版本或错身份阻止新能力启动，旧客户端仍可读旧契约。实测已有功能可用、新能力不被宣告。
   - 新 Desktop + 旧服务端：缺失字段按 unsupported 显示，不伪造空包（parseWithFallback 合同）。
   - 已有确定性用例：`TestHostKnowledgeOldServerStrictClaimBodyRemainsCompatible`、`TestHostHandoffLaunchUnsupportedClaimKeepsLegacyPath`、`TestHostKnowledgeOlderCLIOptionRefusalNeverDowngradesOrReplays`。2026-10-26 的 v0.6.7 CLI 边界结果绑定旧源码 `1bfd169dc`，需要在候选源码上重验。
5. 回退边界：知识设计 §8.3 要求先停新 writer，保留 drainer，不能回退到会恢复 managed session 的旧 claim reader；存在候选、delivery metadata 或待处理 scan/hint 时 down migration 拒绝。是否承诺“候选→v0.6.7”回退需用户决定；不承诺时在交付说明中写明只支持前滚。

矩阵中的“混合 writer 滚动升级”：compose 部署是单 API 实例重启，不会有两个 API 版本同时写库。实际存在的混用窗口是新服务端配旧 Desktop/daemon，以及回退时旧服务端读新数据。这一解释需在执行前确认。

证据：两份包的 SHA-256、镜像 digest、升级前后 `schema_migrations` 快照、健康与配置响应、开关查询结果、登录验收、重复升级日志、混用场景记录。

## G4 安装版真实 Provider 账号旅程

现状：Claude Code 2.1.310 在 macOS arm64 的 `TestHostProviderAccountSmoke` 通过，属于旧源码；macOS `.app`（旧源 `61ff581d`）、Debian `.deb` 与目录包的完整 Main 控制只连测试自有 fixture。安装版连真实账号的旅程没有运行过。

步骤：

1. 用候选源码重建安装包（macOS `.app`/DMG、Linux `.deb`，Windows 由 G2 产出），记录 ASAR、Main、内置 CLI 的哈希。
2. 先用 fixture 跑 `e2e/collaboration-host-installed.spec.ts`，确认候选包的控制旅程仍通过，不调用 Provider。
3. 再在独立窗口用一个真实已登录账号跑一次。它和原实验系列、新 runtime/知识系列都分开：单独登记调用上限、模型、停止规则与结果目录，不占用 E2a trial 3 的 22 次。

证据：包哈希、CLI 版本与路径、请求模型和实际观察模型、run/attempt/termination/proof、ledger 终态、无凭据输出的扫描结果。

## G5 多账号与额度溯源（AC43，R13/R28）

要求（`10-10-cross-workspace-collaboration/acceptance.md:53`）：多个目标或小队成员使用不同环境时，各自沿用该环境的供应商账号和额度；记录真实发起者、实际执行者和授权来源；同环境多人提交与普通转交不切换供应商账号。

前置：至少两个执行环境，分别登录**不同**供应商账号；至少三个 Multica 账号（发起人、另一协作者、资源拥有者）；每个账号的额度和调用上限经用户明确授权。

| 场景 | 预期 |
| --- | --- |
| 两个目标分别在环境 A、B | 各 run 的 runtime/installation 指向各自环境；A 的供应商用量只含目标 1，B 只含目标 2 |
| 小队 leader 在 A、member 在 B | 每个 run 记在实际执行的环境；member run 不计入 A |
| 同一目标（A）收到两位用户的指令 | 都用 A 的账号；input author 分别是实际提交人 |
| 当前处理人由 X 转交给 Y | 后续 run 仍用原授权环境 A 的账号；授权来源不变 |

服务端只知道 `account_source=runtime_configuration`，所以账号身份在执行机上取证：记录 CLI 的账号状态输出，只保留脱敏指纹（哈希或部分组织/账号 ID）。额度以供应商控制台或用量导出为准。run 自报 usage 只作交叉核对，reported zero tokens 也不能证明零费用。

每个 run 记录：run_id、runtime_id、installation_id、执行机、grant_id/epoch、发起人、各 input author、授权来源、CLI 版本、账号指纹、自报 usage、时间窗。每个账号记录前后两次控制台用量快照；控制台有延迟，按时间窗对账，不要求数值逐一相等。

## 需要用户决定的事项

1. 是否推送当前分支并向 fork `main` 开 PR（或推送验证快照分支）来跑原生 Windows CI。仓库公开，推送即公开源码。
2. Windows 升级基线用已发布的 `v0.6.6` 安装器，还是把本地已校验的 0.6.7 安装器上传到 `v0.6.7` Release。
3. 本次交付是否明确不含 Windows arm64（内网只用 x64）。
4. AC43 用哪两个环境、哪两个供应商账号，额度与调用上限是多少；同一台机器上两个 OS 用户分别登录不同账号，算不算“不同环境”。
5. G4 安装版真实账号旅程的调用预算，与原系列和新 runtime/知识系列分开分配。
6. 是否承诺候选版本回退到 v0.6.7。

## 不需要授权、可以先做的部分

- G1 第 1–3 步（交叉编译和本地计时），以及第 4 步检查器改动的草稿。
- G3 第 1–3 步：文档更新、在 detached worktree 打包、本地 Docker 演练。
- G4 第 1–2 步：重建安装包并跑 fixture 安装版旅程。

这些步骤都要等当前真实模型窗口结束后再做，并且使用任务专用数据库、端口和临时目录。
