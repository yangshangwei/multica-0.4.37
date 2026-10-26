# 隔离运行时恢复记录

此目录保留阶段一实际使用的 `Dockerfile` 和 `opencode-pilot` 包装器。默认操作到准备镜像为止；容器启动、凭据配置和 task 触发由主会话在模型可用后处理。这里不包含启动脚本、凭据或试点通过结论。

本轮模型受阻后，容器已停止，容器内开发认证配置已移除。继续使用保留容器时应先确认其状态并通过原本地认证来源恢复配置；不要重复创建同名容器或假定保留容器仍持有凭据。最初构建的镜像中的包装器曾经缺少临时目录所有权处理，运行时随后通过 `docker cp` 修正；本目录保存的是修正版，重新创建容器前应按下方命令重建镜像。

## 已执行的环境与身份

- API：本机开发服务 `http://host.docker.internal:18572`，不是生产服务器。
- 源码 commit：`61ff581d2`（完整 ID `61ff581d247dbae6b00f148fbc660b9762385aac`）。
- 二进制：`GOOS=linux GOARCH=arm64 CGO_ENABLED=0`，`main.version=v0.6.7-security-pilot`、`main.commit=61ff581d2`。版本字符串仅为本地试点标签，不代表产品发布。
- 镜像：`multica-security-pilot:20261027`，基于 `node:22-bookworm`，安装 `opencode-ai@1.18.21`。
- 容器：`multica-security-pilot-20261027`；未挂载 host HOME、Docker socket 或任何卷。
- 原 daemon ID：`019fef71-c14f-757e-9ad2-a9750d306456`。
- 北辰工作空间原 OpenCode runtime：`39443bc2-1bf7-4515-8bdd-4684a41d64c9`。恢复该身份没有重绑已有角色。

恢复身份不等于完成试点。daemon 会注册其账号具有成员关系的所有工作空间，profile 的 `workspace_id` 不限制这个范围。主会话实测注册了四个工作空间 namespace 的 runtime，其中三个已存在、一个为猎户座新记录；上线前必须核对旧角色的排队任务，不能认为只会消费本试点工作空间。

## 仅准备镜像

从仓库内执行。输出二进制放入独立临时构建目录，不写回源码目录：

```bash
pilot_source="$(git rev-parse --show-toplevel)"
pilot_context="$(mktemp -d "${TMPDIR:-/tmp}/multica-security-image.XXXXXX")"
pilot_commit="$(git -C "$pilot_source" rev-parse --short=9 HEAD)"
cp "$pilot_source/.trellis/tasks/10-26-security-remediation-squad/pilot/runtime/Dockerfile" "$pilot_context/Dockerfile"
cp "$pilot_source/.trellis/tasks/10-26-security-remediation-squad/pilot/runtime/opencode-pilot" "$pilot_context/opencode-pilot"
(
  cd "$pilot_source/server"
  GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build \
    -ldflags "-X main.version=v0.6.7-security-pilot -X main.commit=$pilot_commit" \
    -o "$pilot_context/multica" ./cmd/multica
)
docker build --platform linux/arm64 \
  --tag multica-security-pilot:20261027 "$pilot_context"
```

记录当次真实 commit 和工作副本差异；后续源码变化不能继续宣称使用上面的旧 commit。基础镜像标签与 npm 来源仍是构建输入，本文件不承诺逐字节可复现或供应链已闭合。

## 容器与私有 profile

以下是已使用的容器形状，供主会话恢复现场；准备镜像时不运行：

```bash
docker run --detach --init \
  --name multica-security-pilot-20261027 \
  --pids-limit 256 --memory 2g --cpus 2 \
  --security-opt no-new-privileges \
  multica-security-pilot:20261027 sleep infinity
```

daemon 的开发 CLI profile 位于容器的 `/root/.multica/profiles/security-recovery/`。目录权限为 `0700`，配置文件为 `0600`；角色进程使用 UID 1000 的 `node`，不能读取这个 root 私有目录。主会话已验证此拒绝边界；本次文档保存没有重跑凭据测试。

配置只保留开发服务地址、北辰 `workspace_id`、已认证本地开发 profile 的 token、`workspaces_root=/work/tasks`、单并发和禁用自动更新/重载的设置。实际传入通过一个权限为 `0600` 的临时文件与 `docker cp` 完成，随后立即移除本机临时文件。仓库、构建上下文、命令参数和日志中均不得存放 token。不要自动读取或复制个人模型凭据；模型配置尚未获授权复制。

该恢复 profile 不含 `management_deployment_id`：原 daemon 属于旧的 legacy 身份，目的是恢复同一历史绑定，不是伪造托管安装或绕过 managed namespace。当前托管桌面 profile 保持原状；禁止把这个做法用于改变托管安装的身份。

## 模型配置完成后由主会话启动

下面仅记录启动参数。`PILOT_APPROVED_MODEL` 必须由主会话在确认 provider、模型、认证来源与费用边界后设置，不能用匿名轮换或重试绕过限额：

```bash
: "${PILOT_APPROVED_MODEL:?由主会话确认模型配置后再启动}"
docker exec --detach \
  --env MULTICA_OPENCODE_PATH=/usr/local/bin/opencode-pilot \
  --env MULTICA_OPENCODE_MODEL="$PILOT_APPROVED_MODEL" \
  --env MULTICA_GC_ENABLED=false \
  multica-security-pilot-20261027 \
  multica --profile security-recovery daemon start --foreground \
    --daemon-id 019fef71-c14f-757e-9ad2-a9750d306456 \
    --device-name security-pilot-isolated \
    --workspaces-root /work/tasks --max-concurrent-tasks 1 \
    --agent-timeout 3m --no-auto-update --no-auto-reload
```

任务本身没有指定模型时，daemon 使用 `MULTICA_OPENCODE_MODEL` 的 fallback；明确的 task/agent 模型设置优先。实际尝试 `opencode/big-pickle` 遇到匿名免费额度限制，`opencode/hy3-free` 遇到传输错误。最后一次模型运行自动重试后超时，没有成功的角色评论；主会话观察到没有待执行 task。这些失败不能登记为有效路由、独立 QA 或阶段一通过。

## 工作目录和权限修复

主会话将产品内项目资源绑定到 `/work/lab`，模式为 `in_place`，owner 是上述原 daemon。这里只复制 `../fixtures/` 的合成材料到容器实验室，不挂载整个产品仓库。保留仓库内原始失败基线；实现补丁只写入隔离副本。

成员的实验室目录和 leader 的工作目录不同。leader 应使用项目资源指向的绝对路径读取材料，不能因自己的 cwd 没有实验室文件而扩大权限或修改资源绑定。

最初 daemon 以 root 创建了模式为 `0700` 的 task 临时目录，切换为 `node` 的 OpenCode 无法写入，产生 `EACCES`。包装器在降权前仅对匹配的 task cwd（`/work/*`）和 `TMPDIR`（`/tmp/multica-task-*`）调整为 `node:node`，随后通过 `runuser -u node` 启动真实 OpenCode。它不会更改 `/root/.multica` 的权限，也不把 daemon 的私有 profile 移交给角色。

## 验证范围与剩余门槛

- 本目录只做 shell 语法、来源字节一致性与内容边界检查，不额外触发 provider 调用。
- Docker 仍使用默认 bridge，出站访问未受限制。它提供独立文件系统，但尚未满足完整的网络最小权限门槛；必须配置并验证所需的 API/模型目的地范围后再做试点验收。
- 未挂载 host HOME/socket、root profile 不可由角色读取，不等于角色间存在独立操作系统沙箱；同一容器中的角色都以 `node` 运行。
- 三类案例和生产升级演练仍需真实评论、activity、可追溯执行结果与独立验证。模型恢复与网络约束未闭合前，阶段二继续等待。
