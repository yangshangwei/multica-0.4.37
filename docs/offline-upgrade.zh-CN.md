# Multica 内网离线升级操作文档

本文档适用于已经拿到 Multica 服务端离线升级包、且服务器不能访问公网的场景。

如果本次只发布桌面安装包，使用桌面下载服务单独交付的《桌面端内网部署与升级操作手册》（`desktop-intranet-update-runbook.zh-CN.md`）中的 `publish` 和 `verify`，无需执行这里的后端和数据库升级。一次交付同时包含服务端与桌面端时，分别按两份手册完成验收；先根据该版本兼容性要求安排升级顺序，再开放桌面更新通道。下载服务健康不代表业务服务或客户端安装已经升级成功。

升级包已经基于指定源码构建完成。内网服务器不需要源码、Node.js、pnpm 或 Go 工具链，只需要 Docker 和 Docker Compose。

升级包包含后端、前端和 PostgreSQL 镜像。后端启动时会自动执行数据库迁移。升级过程中不要执行 `docker compose down -v`，也不要删除 `pgdata` 或 `backend_uploads` 数据卷。

自动迁移由后端镜像的入口脚本执行：每次启动先运行 `migrate up`，已执行的迁移会跳过，只有迁移成功后才启动 API。迁移失败时容器退出，升级脚本不会报告成功。验收使用 `/healthz`，同时检查数据库连接和迁移版本；`/health` 只说明进程存活，不能作为升级成功的依据。

直接运行 `server` 二进制或 `go run ./cmd/server` 会绕过镜像入口，不会自动迁移。本地源码环境应使用 `make up C=api` 或 `make start`；手动部署二进制时，必须用同一份数据库配置先执行 `migrate up`，成功后再启动 `server`。仅升级桌面安装包不会更新服务端数据库。

升级包也包含累计变更说明和发布工具。桌面端与 Web 的“帮助 → 变更说明”从本部署读取这些记录，不需要访问官网。发布器会复用已导入的前端镜像运行，使用 `--pull never`，目标机无需另装 Node。

升级默认保留现有认证方式。需要在桌面端设备自动注册与用户名密码注册之间选择时，先阅读[第七节：认证方式配置与迁移](#authentication-mode)，再安排切换；仅安装新版桌面端不会改变服务端认证模式。

## v0.6.0 升级范围

本次交付为 Linux x86-64（`linux/amd64`）服务端升级包和 Windows x64 桌面安装器。fork 最近的完整二进制交付是 v0.5.5；v0.5.6 的 GitHub Release 只有变更说明，不能据此认定内网已经运行 v0.5.6。以现有容器镜像、`/health` 的版本和迁移记录确定起点。

| 当前服务端 | 本次补齐的数据库迁移 | 需要核对的配置 |
| --- | --- | --- |
| v0.5.5 | 512—566：MCP 模板来源、分拣、项目 P1、迭代 I1 | MCP 目录、可选资源发布，以及下表的配置传入 |
| v0.5.6 | 536—566：项目 P1、迭代 I1 | 项目/迭代开关，以及平台管理配置传入 |

保持已有认证方式，不需要为了升级切换成密码登录。合并现有 `.env` 中的同名项，**不要用 `.env.example` 覆盖它，也不要复制示例密码或重新生成密钥**。本次主 Compose 补齐了以下变量的传入；原文件里已有值现在会真正进入后端，升级前应确认它们仍符合部署意图。

| 配置 | 未设置时 | 操作要求 |
| --- | --- | --- |
| `FF_PROJECTS_P1` | `true` | 项目概览/进展默认可用；设 `false` 停止新进展写入并撤下入口，保留历史和描述版本校验 |
| 工作空间迭代 | 默认关闭 | 所有者或管理员在“设置 → 工作空间 → 迭代”中启用，无需环境变量 |
| `MULTICA_PLATFORM_ADMIN_ENABLED` | `false` | 仅密码模式下按需启用；升级脚本按[7.5 节](#platform-administration)自动初始化首次管理员，已有平台账号和权限保留 |
| `MULTICA_MANAGED_INSTALLATIONS_ENABLED` | `false` | 仅按需启用；要求密码模式和有效、固定的 `MULTICA_DEPLOYMENT_ID` UUID |
| `MULTICA_DEPLOYMENT_ID` | 空 | 已有值必须保留，不能每次升级生成新的 UUID |
| `MULTICA_ADMIN_*_RETENTION_DAYS` | 空 | 三项已声明的管理保留期现在可传入；只展示策略，不启用自动清理 |

`JWT_SECRET`、数据库账号密码、各集成加密密钥、公开 URL、端口及客户端身份文件均须保留。Web 和 Windows Desktop 应配套更新：新版 Desktop 的浏览器登录需要新版 Web 返回一次性 `desktop_state`。先完成服务端/Web 升级与登录验收，再发布桌面更新。

## 一、把升级包传入服务器

通过 U 盘或内网把 `multica-server-upgrade-*.tar.gz` 以及对应的 `.sha256` 文件传到服务器。升级包必须与服务器架构一致：

```bash
uname -m
```

`x86_64` 对应 `linux/amd64`，`aarch64` 或 `arm64` 对应 `linux/arm64`。

## 二、在服务器上执行升级

假设当前部署目录是 `/opt/multica`：

```bash
sha256sum -c multica-server-upgrade-v0.6.0-linux-amd64.tar.gz.sha256
mkdir -p /opt/multica-upgrade
tar -xzf multica-server-upgrade-*.tar.gz -C /opt/multica-upgrade --strip-components=1

cd /opt/multica-upgrade
./offline-upgrade.sh \
  --deployment-dir /opt/multica \
  --yes
```

脚本会用包内版本替换 `/opt/multica/docker-compose.selfhost.yml`，并备份原文件。**如果以前直接修改过主 Compose，先把自定义端口、挂载、环境变量等整理到持久化的覆盖文件中，再执行升级。** 不能依赖旧主文件里的定制内容自动合并。

已有覆盖文件时，每次升级都按原顺序重复传入 `--compose-file`。相对路径以部署目录为基准；文件应保存在 `/opt/multica` 等持久目录，不要指向之后会删除的升级包目录。例如已经启用资源发布并另有本机配置：

```bash
./offline-upgrade.sh --deployment-dir /opt/multica \
  --compose-file docker-compose.resource-publishing.yml \
  --compose-file docker-compose.local.yml \
  --yes
```

只传实际使用的文件，不要为普通升级启用资源发布。脚本在备份、变更说明解析和服务启动中沿用相同顺序；缺失文件会使升级停止。未列出的覆盖文件不会自动读取。

覆盖文件不能把后端或前端 `image` 固定为旧版本。脚本会用本次选定镜像和全部覆盖文件解析 Compose，并在导入镜像后、写入备份或部署配置前校验两个服务的实际镜像。不匹配时停止，提示移除或调整覆盖文件中的 `image`；此时仅镜像已导入，原配置和运行服务未改变。可移除该覆盖项，或分别使用 `${MULTICA_BACKEND_IMAGE}:${MULTICA_IMAGE_TAG}`、`${MULTICA_WEB_IMAGE}:${MULTICA_IMAGE_TAG}` 跟随本次目标版本，再重新升级。

脚本会依次：

1. 检查升级包架构和镜像归档校验和；
2. 导入后端、前端和 PostgreSQL 镜像，校验合并覆盖文件后的实际服务镜像与本次选定版本一致；
3. 将当前 PostgreSQL 导出到 `/opt/multica/backups/<时间>/database.sql`；
4. 备份当前 `.env`、旧主 Compose、已有命令记录和本次传入的覆盖文件；
5. 校验变更说明，在日志目录中原子替换 `changelog.json`，并保存日志路径和选定的镜像配置；
6. 保存新版主 Compose 和完整命令记录，再启动后端和前端；
7. 等待 `/healthz` 返回成功；密码模式且后台已开启时，执行一次性管理员初始化，再报告升级完成。

脚本更新 `/opt/multica/.env` 中的 `MULTICA_BACKEND_IMAGE`、`MULTICA_WEB_IMAGE`、`MULTICA_IMAGE_TAG`、`CHANGELOG_FILE` 和 `CHANGELOG_DIRECTORY`，保留其余设置、文件权限和 Docker 数据卷。镜像选择会持久保存，之后在部署目录直接执行 `docker compose -f docker-compose.selfhost.yml up -d --pull never` 仍会使用本次升级的版本，无需重新传入镜像环境变量。

有覆盖文件时，后续 `config`、`logs`、`up` 和下文的重建命令都必须追加原顺序的每个 `-f`。`/opt/multica/compose-command.txt` 保存本次完整、供 Bash 使用的命令前缀，可核对后复制并追加子命令；它不自动执行，也不会让下一次升级自动选择覆盖文件。下文无覆盖文件的示例不应直接用于有覆盖文件的部署。

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

如果使用了覆盖文件，此命令也要逐项追加 `--compose-file`，尤其是覆盖了变更说明挂载目录的部署。

记录必须先通过现有内网交付方式到达服务器。不要直接编辑正在使用的 JSON，不要将单个文件以 bind mount 或 Kubernetes `subPath` 挂载。文件无效或不可读时，页面会保留最近可用内容并提示尚未同步；重新发布有效文件即可恢复。

## 四、失败处理

配置写入阶段失败时，脚本不会开始重建服务；如果 `.env` 写入失败，发布器会尝试恢复此前的变更说明，并报告恢复失败的情况。只有发布器成功后才替换主 Compose；文件替换失败时仍可能已有部分配置更新，应核对实际文件。原 `.env`、主 Compose 和数据库导出保留在备份目录。覆盖文件依传入顺序保存为 `compose-overlay-1.yml`、`compose-overlay-2.yml` 等，恢复时核对原路径和顺序。

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

v0.6.0 的 P1/I1 down migration 会拒绝删除已使用的数据：P1 的历史、进展、请求、通知、非默认版本或规划时区，以及 I1 的启用记录、任务归属或任何迭代历史，都可能阻止降级。关闭功能开关不会清空这些数据。生产环境优先向前修复；必须回退时先停止所有写入方，制定数据库及文件备份的一致恢复方案。不要手工删表、删迁移记录或删数据卷来绕过保护。并发索引迁移不能包在总事务中，失败后由同版本迁移工具重试，不要仅凭迁移记录或 `IF NOT EXISTS` 判断索引有效。

## 五、重要注意事项

- 现有 `JWT_SECRET` 必须保持不变，否则已有登录会话会失效。
- `POSTGRES_PASSWORD` 必须保持与现有 `.env` 一致。
- 不要执行 `docker compose down -v`。
- 附件不在 PostgreSQL dump 中；重要附件还需要单独备份 `backend_uploads` 数据卷。
- 手工投放的 Skill/MCP 目录、使用中的覆盖文件也需备份；启用资源发布时同时备份整个 `managed_resources` 卷，不能只备份其中的索引或部分修订。
- 后端和前端应尽量从同一源码提交构建，避免 API 与页面版本不匹配。
- 内网完全无外网时，Agent 仍需要能够访问内网 LLM 网关，否则任务不会真正执行。

## 六、选择 Windows 桌面端架构

桌面端的首次部署和每次发包操作，按桌面下载服务单独交付的《桌面端内网部署与升级操作手册》执行。其中包含固定 `updates.env`、离线镜像导出脚本、产物收集发布、自动 HTTP 校验和客户端配置。服务端升级包本身不包含桌面下载服务。

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
  -f /opt/multica/docker-compose.selfhost.yml \
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

<a id="platform-administration"></a>

### 7.5 旧环境首次开启平台管理后台

**升级包不会自动开启后台或改变认证模式。** 当业务部署已经使用密码模式并开启后台时，新版升级脚本会在迁移和健康检查成功后自动初始化专用管理员。没有既有平台角色或管理员初始化历史时，创建用户名 `platform_admin`，生成本部署独立的随机初始密码，保存到业务部署目录的 `admin-initial-password.txt`。没有所有部署通用的默认密码。

已有管理员、观察者或保留的成功初始化记录时，脚本保留现状，不重置密码、不重复创建。后台入口位于 Web 的 `/admin`；新账号必须先完成强制改密，才能执行管理操作。

如果仍使用设备自动登录，先按 7.3 节安排原账号迁移，再切换密码模式；不要为了开启后台直接跳过迁移。已使用密码登录的环境保留原有认证设置，补齐缺少的管理配置即可。

#### 7.5.1 补齐服务端 ENV 配置

先备份业务部署的 `/opt/multica/.env`，修改已有同名项，缺少时再增加。以下为单个 API 进程的完整配置：

```dotenv
MULTICA_AUTH_MODE=password
MULTICA_DEVICE_AUTH_ENABLED=false
MULTICA_CLOUD_URL=
MULTICA_PASSWORD_LIMITER_MODE=single
MULTICA_PLATFORM_ADMIN_ENABLED=true
MULTICA_MANAGED_INSTALLATIONS_ENABLED=true
MULTICA_DEPLOYMENT_ID=<本部署固定的UUID>
```

多个 API 副本继续使用 `shared` 限流和同一个 `REDIS_URL`，不要照抄 `single`。保留现有 `JWT_SECRET`、数据库密码、集成密钥及账号迁移窗口设置。

| 配置 | 用途 |
| --- | --- |
| `MULTICA_PLATFORM_ADMIN_ENABLED` | 开启平台管理后台；不会给任何账号自动授权 |
| `MULTICA_MANAGED_INSTALLATIONS_ENABLED` | 开启受管终端注册与上报能力；需要兼容的桌面端和守护进程 |
| `MULTICA_DEPLOYMENT_ID` | 标识这一套部署，总览和安装查询也需要它；只开启后台开关仍可能返回 `installation_unavailable` |

**已有部署 UUID 必须沿用。** 仅当该部署从未配置过 UUID 时，在目标 Linux 服务器执行一次：

```bash
cat /proc/sys/kernel/random/uuid
```

把输出填写到 `MULTICA_DEPLOYMENT_ID`，替换上面的占位值，并随部署配置一起备份。之后升级、重启或新增副本都使用同一个值。不要复制开发机或其他部署的 UUID；也不要把开发机的管理员密码、账号 UUID 写入交付文档。

`.env` 中不需要填写管理员用户名或密码。初始化通过服务器上的部署维护命令完成，`ALLOW_SIGNUP=false` 时同样可用，无需临时开放普通注册。账号、密码哈希、平台角色和永久初始化记录保存在数据库中，初始明文密码仅写入宿主机的受限文件。

#### 7.5.2 重建后端并确认配置生效

以下命令在 Bash 中执行。若部署使用 Compose 覆盖文件，将原来的每个 `-f /实际路径/覆盖文件.yml` 按原顺序加入数组，保持与 `compose-command.txt` 一致；后面的重建、查询和初始化都使用同一数组。

```bash
cd /opt/multica
compose=(docker compose
  --project-directory /opt/multica
  --env-file /opt/multica/.env
  -f /opt/multica/docker-compose.selfhost.yml
)

"${compose[@]}" up -d --pull never --no-deps --force-recreate backend

curl -fsS http://127.0.0.1:8080/healthz
curl -fsS http://127.0.0.1:8080/api/config
```

使用实际后端端口替换 `8080`。等待 `/healthz` 成功后再进行账号初始化；它尚未就绪时查看后端日志，确认迁移是否成功。只执行 `restart` 不会读取新的容器环境变量；Shell 中残留的同名 `export` 也可能覆盖 `.env`，应按 7.4 节核对。

`/api/config` 应返回：

| 字段 | 预期 |
| --- | --- |
| `auth_mode` | `password` |
| `password_auth_available` | `true` |
| `platform_admin_available` | `true` |
| `managed_installation_supported` | `true` |
| `deployment_id` | 与该部署 `.env` 中保存的 UUID 一致 |

#### 7.5.3 创建专用账号并初始化超级管理员

如果执行升级时上述配置已经生效，`offline-upgrade.sh` 会自动完成这一步。若升级后才开启后台，或初始化阶段失败后需要重试，在服务器使用同一个升级包执行：

```bash
bash /opt/multica-upgrade/initialize-platform-admin.sh \
  --deployment-dir /opt/multica
```

已有 Compose 覆盖文件时，与升级命令一样，按原顺序追加每个 `--compose-file`。脚本使用部署目录里已安装的主 Compose 和实际镜像，不重新构建或从公网拉取，也不需要宿主机安装 Node.js 或 Go。

查看非秘密回执：`initialized` 表示首次账号已创建且密码文件可用；`already_initialized` 表示永久记录已存在；`preserved_existing_admin` 表示保留原有平台管理配置。仅在回执确认初始凭据文件可用时，按提示读取该文件；账号已改密后，旧文件中的初始密码不再有效。

在运行升级脚本的同一操作系统账号下读取初始密码：

```bash
cat /opt/multica/admin-initial-password.txt
```

文件为 JSON，登录使用其中的 `username` 和 `initial_password`。文件权限为 `0600`，其他系统账号不能直接读取；不要将内容粘贴到普通日志、工单、仓库或升级包中。路径跟随 `--deployment-dir`，不是升级包解压目录。

初始化先同步密码文件，再提交账号、角色和记录。重复执行会核对原有意图，不生成另一密码；普通 API 重启也不会创建账号。初始化失败时，服务可能已升级，但脚本不会报告整次升级成功。保留回执和原文件，修复原因后重试上述维护脚本，不必重复导入镜像。

#### 7.5.4 首次改密与登录验收

使用文件中的初始密码登录 **实际 Web 地址的 `/login?next=%2Fadmin`**，按提示设置与初始密码不同的新密码。完成改密前不能访问后台或业务数据；完成后进入 `/admin`，初始密码和此前受限会话均失效。

不要填写 API 端口；从其他电脑访问时使用服务器的实际 IP 或域名，`127.0.0.1` 指向访问者自己。没有生成新账号的既有部署继续使用原来的管理员登录。

- 在浏览器开发者工具的网络面板中确认已登录请求 `/api/admin/me` 返回 `200`，`role` 为 `super_admin`。
- 确认总览和账号列表正常加载，对应 `/api/admin/overview`、`/api/admin/users` 返回 `200`。没有受管终端时数量可以为零，但不应出现数据加载失败。
- 验证初始密码已不能再次登录；妥善保存新密码。完成改密后可删除失效的初始密码文件，再次升级不会重新创建它。
- 保存本次配置变更、非秘密初始化回执和验收结果。后续升级保留数据库、`JWT_SECRET`、固定部署 UUID 和永久初始化记录；停用或撤销管理员后也不会自动重建。

| 现象 | 处理 |
| --- | --- |
| `platform_admin_available=false` 或跳过初始化 | 核对密码模式、后台开关、Compose 变量传入及实际容器是否已重建 |
| 已登录，但总览返回 `installation_unavailable` | 检查固定 `MULTICA_DEPLOYMENT_ID` 是否有效并已进入后端容器；不要重新生成已有 UUID |
| 保留已有管理员，未生成密码文件 | 使用既有管理员；观察者或历史管理员也不会被自动替换 |
| `platform_admin` 已被普通账号占用 | 不会自动提权或改密。核实账号后按 7.5.5 节显式授权，或由已有超级管理员处理 |
| 密码文件权限、内容、部署标识或意图不匹配 | 保留原文件，核对实际部署和文件归属；不要覆盖文件来强行重试 |
| `credentials_unavailable` | 已创建的待改密账号没有匹配的本地密码文件；先核对原初始化主机。确已丢失时，对回执中的账号 UUID 执行现有密码恢复流程，不会自动重置；最后一位管理员的恢复仍需 `--break-glass` |
| 首次改密被拒绝 | 新密码必须与初始密码不同；保留当前页面并重新提交符合要求的新密码 |
| 升级提示管理员初始化未完成 | 先核对服务健康和回执，修复后单独重试初始化脚本；不要删除初始化记录或重新建账号 |

#### 7.5.5 保留人工授权方式

需要明确选择已有账号作为管理员时，仍可使用原来的人工 `bootstrap`。先通过 Web 建立密码账号并完成必要改密；该命令不创建用户、不设置密码，仅在没有有效超级管理员时向指定账号授权。已有超级管理员时优先在后台授予权限。

在服务器查询刚注册账号的 UUID。下面使用随部署运行的 `postgres` 容器，只读取账号标识，不读取密码；将 SQL 中的 `platform_admin` 换成实际注册的用户名：

```bash
"${compose[@]}" exec -T postgres sh -c \
  'exec psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1' <<'SQL'
BEGIN READ ONLY;
SELECT u.id, c.username, u.name
FROM "user" AS u
JOIN user_password_credential AS c ON c.user_id = u.id
WHERE c.username = 'platform_admin';
COMMIT;
SQL
```

核对结果恰好对应目标账号，再把 `USER_UUID` 替换成查询到的完整 UUID。若后端使用外部数据库，应在后端实际连接的数据库中执行上述只读查询，不要查询另一套本地数据库。

```bash
"${compose[@]}" exec -T backend /app/server platform-admin bootstrap \
  --user USER_UUID \
  --reason "Initialize deployment administration"
```

人工授权会撤销目标账号的旧凭据，之后重新输入其密码登录。首个注册用户、工作空间 owner/admin 都不会自动获得平台权限。人工授权也会保留永久初始化记录，后续自动升级不会再生成一个额外管理员。

### 7.6 每次打包升级必须核对

- [ ] 本次交付说明写明：保持原认证模式，还是计划从设备模式迁移到密码模式。
- [ ] 升级文档包含两组配置、限流选择、旧账号迁移、应用配置和验收步骤；首次开启后台时，按 7.5 节验证自动初始化、私有密码文件、首次改密和后台访问。
- [ ] 使用 `scripts/build-offline-upgrade.sh` 或 `make offline-upgrade-bundle` 打包。脚本会把本文件复制为包内的 `README.md` 和 `操作文档.md`；解包确认两份均与源文档一致，包含认证迁移及“旧环境首次开启平台管理后台”完整步骤。
- [ ] 管理配置使用目标部署已有或首次生成的固定 UUID；示例中没有开发机的真实 UUID、账号凭据或数据库密码。普通升级仍保持原认证方式和后台开关。
- [ ] 升级包包含独立初始化脚本及其辅助文件，不包含 `admin-initial-password.txt` 或 `.admin-initial-password.*` 暂存文件；验证重复升级保留既有凭据。
- [ ] 仅发布桌面安装包时，随交付说明附上本节或对应服务端升级手册，并明确桌面升级不会自行切换认证模式。
- [ ] 记录切换前后 `/api/config`、实际登录验收和需要迁移的账号处理结果。迁移窗口使用本次部署确定的时间，不沿用过期示例。

## 八、可选功能配置

### 8.1 项目与迭代

P1 的项目概览和进展默认可用。迭代由每个工作空间独立管理，不再需要设置服务器环境变量或为启停功能重启服务。

1. 完成版本升级后，由工作空间所有者或管理员在 Web 或桌面端进入 **设置 → 工作空间 → 迭代**。普通成员可以查看，不能修改启停和规划时区。
2. 核对页面显示的规划时区；需要修改时，通过链接进入 **设置 → 工作空间 → 常规** 保存，再回到迭代设置打开 **启用迭代**。启用只对当前工作空间生效，不创建周期，也不启动任务执行。
3. 返回 **迭代** 页面，手动创建周期、安排任务并开始；用完整的结束预览验证任务去向和历史快照。
4. 需要关闭时，在同一设置页关闭开关，核对进行中迭代、计划迭代和任务的影响范围，填写原因后确认。进行中迭代结束并保留快照，计划迭代取消，任务当前迭代归属清除；任务状态、项目、负责人和正在运行的执行保持不变。再次启用不会恢复旧安排。
5. `GET /api/workspaces/{id}/iteration-capabilities` 中 `supported=true` 表示该服务实现迭代能力，`enabled` 表示当前工作空间的真实启用状态。接口仍需成员权限；健康检查不能替代实际操作验收。

**升级保留工作空间原有启用状态。** 新空间默认关闭；过去已启用但被部署总开关隐藏的空间，升级后会恢复可用，已有通知可能继续投递。旧 `.env` 或操作人员的 Compose 覆盖文件里残留的 `FF_ITERATIONS_I1` 不再控制迭代，可以自行移除；升级工具仍保留这些操作人员文件。旧客户端可继续使用原有接口，新客户端连接不支持迭代的旧后端时会明确提示升级。

关闭后仍可从设置中的 **查看历史** 或已有链接访问历史。若关闭请求因网络中断而结果不明，使用页面的恢复入口确认原操作，不重复创建关闭请求。

`FF_PROJECTS_P1=false` 仍只停止新项目进展入口/写入，已有进展历史和描述并发版本校验保留；旧 CLI 的无版本描述写入可能返回 428，须升级 CLI 并使用实际读到的版本号。

### 8.2 MCP 模板与资源发布

从 v0.5.5 升级后，主 Compose 默认把宿主 `${MCP_TEMPLATE_DIRECTORY:-./mcp-templates}` 只读挂到 `/app/data/mcp-templates`。不使用部署模板时可以保持空目录。自定义宿主目录用 `MCP_TEMPLATE_DIRECTORY`；容器内的 `MULTICA_MCP_TEMPLATE_DIR` 保持默认路径。`MULTICA_MCP_TEMPLATE_ALLOW_HTTP=false` 默认拒绝明文 HTTP，仅在部署确实需要时改成 `true` 并重建后端。模板本身不要保存真实凭据。详见同包的 [MCP 目录发布规范](mcp-catalog-publishing.md) 和 [内网 MCP 准备说明](mcp-intranet-setup.md)。

后台资源发布保持关闭，包内 `docker-compose.resource-publishing.yml` 仅供选择启用。已有密码模式且已初始化超级管理员、确实要启用时，将此文件保存到部署目录（已有文件先备份并核对，勿覆盖定制），然后在所有 Compose 命令中追加 `-f /opt/multica/docker-compose.resource-publishing.yml`，在以后升级时追加对应 `--compose-file`。它设置 `MULTICA_RESOURCE_PUBLISH_DIR=/app/data/resources` 并挂载专用 `managed_resources` 卷；只在 `.env` 设置目录不会创建挂载。详见同包的 [后台资源发布说明](admin-resource-publishing.zh-CN.md)。

保留现有认证时，不要为了启用资源发布直接切换旧设备账号；需要切换的部署必须先完成第七节账号迁移。首次启用平台管理按 [7.5 节](#platform-administration)完成配置、账号授权和登录验收。部署管理开关和资源发布卷都不会在普通升级中自动打开。
