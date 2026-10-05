# Multica 内网离线升级操作文档

本文档适用于已经拿到 Multica 服务端离线升级包、且服务器不能访问公网的场景。

如果本次只发布桌面安装包，使用[桌面端内网部署与升级操作手册](desktop-intranet-update-runbook.zh-CN.md)中的 `publish` 和 `verify`，无需执行这里的后端和数据库升级。一次交付同时包含服务端与桌面端时，分别按两份手册完成验收；先根据该版本兼容性要求安排升级顺序，再开放桌面更新通道。下载服务健康不代表业务服务或客户端安装已经升级成功。

升级包已经基于指定源码构建完成。内网服务器不需要源码、Node.js、pnpm 或 Go 工具链，只需要 Docker 和 Docker Compose。

升级包包含后端、前端和 PostgreSQL 镜像。后端启动时会自动执行数据库迁移。升级过程中不要执行 `docker compose down -v`，也不要删除 `pgdata` 或 `backend_uploads` 数据卷。

自动迁移由后端镜像的入口脚本执行：每次启动先运行 `migrate up`，已执行的迁移会跳过，只有迁移成功后才启动 API。迁移失败时容器退出，升级脚本不会报告成功。验收使用 `/healthz`，同时检查数据库连接和迁移版本；`/health` 只说明进程存活，不能作为升级成功的依据。

直接运行 `server` 二进制或 `go run ./cmd/server` 会绕过镜像入口，不会自动迁移。本地源码环境应使用 `make up C=api` 或 `make start`；手动部署二进制时，必须用同一份数据库配置先执行 `migrate up`，成功后再启动 `server`。仅升级桌面安装包不会更新服务端数据库。

升级包也包含累计变更说明和发布工具。桌面端与 Web 的“帮助 → 变更说明”从本部署读取这些记录，不需要访问官网。发布器会复用已导入的前端镜像运行，使用 `--pull never`，目标机无需另装 Node。

升级默认保留现有认证方式。需要在桌面端设备自动注册与用户名密码注册之间选择时，先阅读[第七节：认证方式配置与迁移](#authentication-mode)，再安排切换；仅安装新版桌面端不会改变服务端认证模式。

## 一、把升级包传入服务器

通过 U 盘或内网把 `multica-server-upgrade-*.tar.gz` 以及对应的 `.sha256` 文件传到服务器。升级包必须与服务器架构一致：

```bash
uname -m
```

`x86_64` 对应 `linux/amd64`，`aarch64` 或 `arm64` 对应 `linux/arm64`。

## 二、在服务器上执行升级

假设当前部署目录是 `/opt/multica`：

```bash
mkdir -p /opt/multica-upgrade
tar -xzf multica-server-upgrade-*.tar.gz -C /opt/multica-upgrade --strip-components=1

cd /opt/multica-upgrade
./offline-upgrade.sh \
  --deployment-dir /opt/multica \
  --yes
```

脚本会依次：

1. 检查升级包架构和镜像归档校验和；
2. 导入后端、前端和 PostgreSQL 镜像；
3. 将当前 PostgreSQL 导出到 `/opt/multica/backups/<时间>/database.sql`；
4. 备份当前 `.env`；
5. 校验变更说明，在日志目录中原子替换 `changelog.json`，并保存日志路径和选定的镜像配置；
6. 使用升级包中的 Compose 文件启动后端和前端；
7. 等待 `/healthz` 返回成功。

脚本更新 `/opt/multica/.env` 中的 `MULTICA_BACKEND_IMAGE`、`MULTICA_WEB_IMAGE`、`MULTICA_IMAGE_TAG`、`CHANGELOG_FILE` 和 `CHANGELOG_DIRECTORY`，保留其余设置、文件权限和 Docker 数据卷。镜像选择会持久保存，之后在部署目录直接执行 `docker compose -f docker-compose.selfhost.yml up -d --pull never` 仍会使用本次升级的版本，无需重新传入镜像环境变量。

默认日志目录为 `/opt/multica/changelog`；已配置目录会继续使用。Compose 挂载整个目录，容器内的日志路径为 `/app/data/changelog/changelog.json`。已有非空且不兼容的 `CHANGELOG_FILE` 会使升级停止，需先核对配置。

不加 `--yes` 时脚本会在执行前显示版本、镜像和备份目录并要求确认：

```bash
./offline-upgrade.sh --deployment-dir /opt/multica
```

如果服务器使用内网镜像名或需要强制指定标签，可以覆盖镜像参数：

```bash
./offline-upgrade.sh \
  --deployment-dir /opt/multica \
  --backend-image registry.intra.example.com/multica-backend \
  --web-image registry.intra.example.com/multica-web \
  --image-tag v0.4.40 \
  --yes
```

## 三、升级后检查

```bash
cd /opt/multica
docker compose -f docker-compose.selfhost.yml config --images
docker compose -f docker-compose.selfhost.yml ps
docker compose -f docker-compose.selfhost.yml logs --tail=200 backend
curl -fsS http://127.0.0.1:8080/health
curl -fsS http://127.0.0.1:8080/healthz
```

确认镜像列表中的后端和前端标签是本次升级版本。如果后端端口不是 `8080`，使用现有 `.env` 中配置的端口。

在客户端打开“帮助 → 变更说明”，核对本次版本、来源和具体更新。已经打开的页面每 60 秒检查新内容，也可以点“刷新”。首次安装这项功能需要正常升级客户端；之后单独发布新说明无需重新安装或重启客户端。

需要只更新说明而不升级镜像时，先确保新包中的镜像已导入，再运行：

```bash
bash /opt/multica-upgrade/install-changelog.sh --deployment-dir /opt/multica
```

记录必须先通过现有内网交付方式到达服务器。不要直接编辑正在使用的 JSON，不要将单个文件以 bind mount 或 Kubernetes `subPath` 挂载。文件无效或不可读时，页面会保留最近可用内容并提示尚未同步；重新发布有效文件即可恢复。

## 四、失败处理

配置写入阶段失败时，脚本不会开始重建服务；如果 `.env` 写入失败，发布器会尝试恢复此前的变更说明，并报告恢复失败的情况。原 `.env` 和数据库导出保留在备份目录。

开始重建容器后，如果启动或健康检查失败，脚本会以非零状态退出，保留已保存的新版本镜像选择和变更说明。容器、数据库迁移与配置写入不是一个原子事务，脚本不会自动回退容器或数据库。此时可能已有部分服务更新，恢复前应检查实际运行状态与迁移结果。

先查看日志和备份：

```bash
docker compose -f docker-compose.selfhost.yml logs --tail=300 backend
ls -lh /opt/multica/backups/
```

保留旧镜像和旧 Compose 文件，不要立即删除。可以把 `.env` 中的镜像配置改回旧版本，再执行：

```bash
docker compose -f docker-compose.selfhost.yml up -d --pull never backend frontend
```

如果新版本已经执行了数据库结构迁移，单纯回退镜像可能不够，需要使用备份的 `database.sql` 恢复数据库。恢复前应先停止后端并确认备份文件和目标数据库，避免覆盖错误的数据库。

## 五、重要注意事项

- 现有 `JWT_SECRET` 必须保持不变，否则已有登录会话会失效。
- `POSTGRES_PASSWORD` 必须保持与现有 `.env` 一致。
- 不要执行 `docker compose down -v`。
- 附件不在 PostgreSQL dump 中；重要附件还需要单独备份 `backend_uploads` 数据卷。
- 后端和前端应尽量从同一源码提交构建，避免 API 与页面版本不匹配。
- 内网完全无外网时，Agent 仍需要能够访问内网 LLM 网关，否则任务不会真正执行。

## 六、选择 Windows 桌面端架构

桌面端的首次部署和每次发包操作，参见[桌面端内网部署与升级操作手册](desktop-intranet-update-runbook.zh-CN.md)。其中包含固定 `updates.env`、离线镜像导出脚本、产物收集发布、自动 HTTP 校验和客户端配置；设计依据见[桌面端内网升级方案](desktop-intranet-update-plan.zh-CN.md)。

桌面端安装器单独提供，不在服务端升级归档中。按客户端架构选择文件：

| 客户端架构 | 安装器文件名 | 自动更新元数据 |
| --- | --- | --- |
| Windows x86，32 位 | `multica-desktop-<版本>-windows-ia32.exe` | `latest-ia32.yml` |
| Windows x64，64 位 | `multica-desktop-<版本>-windows-x64.exe` | `latest.yml` |
| Windows ARM64 | `multica-desktop-<版本>-windows-arm64.exe` | `latest-arm64.yml` |

ia32 安装器中的桌面程序和 CLI 均为 32 位；x64 安装器用于 x86-64。Linux 服务端的 `linux/amd64` 与 Windows 客户端架构独立选择。

内网更新目录应保留原始安装器文件名、对应的 `.exe.blockmap` 和各自的元数据文件。先放入安装器及 blockmap，再替换 `latest*.yml`，不要用 ia32 的元数据覆盖 x64 的 `latest.yml`。客户端继续使用 `%USERPROFILE%\.multica\desktop.json` 中配置的 `updateUrl`。

<a id="authentication-mode"></a>

## 七、认证方式配置与迁移

### 7.1 当前支持范围

两种方式由服务端统一选择，同一部署不能同时启用。桌面端启动时读取服务端配置，显示对应的登录方式；客户端没有自由切换两种注册方式的按钮。

| 方式 | 行为 | 平台管理后台 |
| --- | --- | --- |
| 设备自动注册、登录 | 桌面端首次生成随机设备身份，服务端据此创建账号；以后复用该身份 | 不支持 |
| 用户名和密码注册、登录 | 用户注册密码账号，之后输入用户名和密码登录 | 支持，但必须另行启用并初始化管理员 |

设备身份保存在 Electron 用户数据目录的 `device-identity.json`，不是硬件序列号，也不是每次启动重新生成。升级时保留该目录；删除、损坏该文件或更换用户数据目录可能产生新账号。设备模式允许能访问 API 的客户端自行创建身份，只适用于允许这种访问方式的内网部署。

### 7.2 保存到业务服务的配置文件

以下设置写入现有部署的 `/opt/multica/.env`。修改已有同名项，缺少时再补充，不要重复添加，也不要用升级包的 `.env.example` 覆盖现有文件。桌面下载服务的 `updates.env` 和客户端的 `desktop.json` 都不能设置业务认证模式。

保留设备自动注册方式，或为新的设备模式部署配置：

```dotenv
MULTICA_AUTH_MODE=legacy
MULTICA_DEVICE_AUTH_ENABLED=true
MULTICA_CLOUD_URL=
```

设备模式不使用密码限流。`ALLOW_SIGNUP=false` 不能关闭设备自动创建账号；关闭设备入口要设置 `MULTICA_DEVICE_AUTH_ENABLED=false`。如部署另行开启了 `MULTICA_MANAGED_INSTALLATIONS_ENABLED` 或 `MULTICA_PLATFORM_ADMIN_ENABLED`，设备模式下必须关闭；受管安装要求密码模式，平台后台也不支持设备模式。

选择用户名和密码注册、登录，单个 API 进程使用：

```dotenv
ALLOW_SIGNUP=true
MULTICA_AUTH_MODE=password
MULTICA_DEVICE_AUTH_ENABLED=false
MULTICA_CLOUD_URL=
MULTICA_PASSWORD_LIMITER_MODE=single
```

`ALLOW_SIGNUP=true` 允许密码账号自行注册；改为 `false` 会关闭新账号注册，已有密码账号仍可登录。密码注册不使用邮箱白名单。密码模式不能同时开启设备自动认证，也不会退回邮箱或设备登录。

多个 API 进程或副本必须统一使用 `MULTICA_PASSWORD_LIMITER_MODE=shared`，并设置连接同一个 Redis 的 `REDIS_URL`；Redis 不可用时密码请求会被拒绝。不要用 `single` 代替多副本的共享限流。

### 7.3 旧设备账号转为密码账号

需要保留旧账号的工作空间、任务和成员关系时，使用账号绑定流程。直接在密码页面重新注册会创建另一个账号，不会自动合并旧账号的数据。

1. 先完成服务端升级并保持原认证模式，准备兼容密码登录的客户端，确认数据库备份可用。备份现有 `.env`，保持 `JWT_SECRET` 和已有固定 `MULTICA_DEPLOYMENT_ID` 不变。
2. 切换前让需要迁移的用户保留有效旧登录会话，不要退出登录或清空客户端数据。记录旧账号 UUID，便于迁移失败时恢复访问。
3. 在密码模式配置中额外设置下面两个时间。占位值必须替换成实际的 UTC RFC3339 时间，以 `Z` 结尾；`CUTOFF` 是接纳旧会话签发时间的截止点，`DEADLINE` 是绑定完成期限，必须晚于 `CUTOFF`。

```dotenv
MULTICA_PASSWORD_MIGRATION_CUTOFF=<旧会话签发截止时间，UTC RFC3339>
MULTICA_PASSWORD_MIGRATION_DEADLINE=<完成绑定的截止时间，UTC RFC3339>
```

4. 按下一节重建后端，再让用户完整退出桌面程序并重新打开，保留登录数据。只有在 `CUTOFF` 之前签发、尚未过期且符合账号状态要求的旧会话，才能在 `DEADLINE` 之前进入账号设置页面。该会话只能完成账号设置，不能继续执行业务操作。
5. 用户为原账号设置用户名和密码后，核对账号 UUID、原工作空间和任务仍然一致，并重新登录需要更新凭据的客户端或运行时。绑定会撤销旧凭据。
6. 迁移结束后将两个迁移变量同时清空，再重建后端，关闭旧会话绑定入口。两项必须同时设置或同时为空。

旧会话已过期、已退出登录或错过迁移期限时，由部署维护人员核实账号 UUID 后，使用服务端 `server password-recover --user USER_UUID --username USERNAME --reason "Recover legacy account access"` 恢复原账号。沿用该部署的数据库和密码模式配置；`--username` 只用于尚无密码凭据的历史账号。临时密码从受保护的标准输入读取，不写入命令参数或环境变量，用户下次登录必须改密。

已建立密码凭据或启用平台管理的部署，不得把切回 `legacy` 加设备自动登录当作常规回退。它会改变原有认证约束，也不能自动把密码账号变回原设备身份。登录故障应优先通过账号恢复或修复密码配置处理。

### 7.4 应用配置并验收

以下沿用本手册的部署目录和升级包解压目录，使用与升级脚本相同的 Compose 文件。已有额外 Compose 覆盖文件的部署，应保留原来的完整文件列表。

```bash
docker compose --project-directory /opt/multica \
  --env-file /opt/multica/.env \
  -f /opt/multica-upgrade/docker-compose.selfhost.yml \
  up -d --pull never --no-deps --force-recreate backend

curl -fsS http://127.0.0.1:8080/healthz
curl -fsS http://127.0.0.1:8080/api/config
```

使用实际后端端口替换 `8080`。修改 `.env` 后只执行 `docker compose restart` 不会更新容器环境变量，必须重建后端。执行命令的 Shell 中如有同名环境变量，会优先于 Compose 的 `.env`；应清理过期的临时覆盖，并以实际 `/api/config` 返回为准。

| 验收字段 | 设备模式 | 开放注册的密码模式 |
| --- | --- | --- |
| `auth_mode` | `legacy` | `password` |
| `device_auth_available` | `true` | `false` 或不返回该字段 |
| `password_auth_available` | `false` | `true` |
| `password_signup_available` | `false` | `true` |

刷新 Web 登录页，完整退出并重新打开桌面端，检查登录入口与配置一致。迁移中的客户端不要清理旧会话。用测试账号验证目标模式的注册、登录和原数据访问；`/healthz` 成功只证明服务就绪，不能代替登录验收。

源码开发环境把配置写入启动时实际读取的 `.env` 或 `.env.worktree`，然后重新启动 API。认证方式由运行中的 API 决定，不需要仅为切换配置重新打桌面安装包；已安装客户端仍须具备对应功能。

### 7.5 每次打包升级必须核对

- [ ] 本次交付说明写明：保持原认证模式，还是计划从设备模式迁移到密码模式。
- [ ] 升级文档包含两组配置、限流选择、旧账号迁移、应用配置和验收步骤；示例与本次发布的实际代码一致。
- [ ] 使用 `scripts/build-offline-upgrade.sh` 或 `make offline-upgrade-bundle` 打包。脚本会把本文件复制为包内的 `README.md` 和 `操作文档.md`；解包确认两份均包含本节，而不是旧版说明。
- [ ] 仅发布桌面安装包时，随交付说明附上本节或对应服务端升级手册，并明确桌面升级不会自行切换认证模式。
- [ ] 记录切换前后 `/api/config`、实际登录验收和需要迁移的账号处理结果。迁移窗口使用本次部署确定的时间，不沿用过期示例。
