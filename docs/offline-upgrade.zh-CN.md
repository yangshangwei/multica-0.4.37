# Multica 内网离线升级操作文档

本文档适用于已经拿到 Multica 服务端离线升级包、且服务器不能访问公网的场景。

升级包已经基于指定源码构建完成。内网服务器不需要源码、Node.js、pnpm 或 Go 工具链，只需要 Docker 和 Docker Compose。

升级包包含后端、前端和 PostgreSQL 镜像。后端启动时会自动执行数据库迁移。升级过程中不要执行 `docker compose down -v`，也不要删除 `pgdata` 或 `backend_uploads` 数据卷。

升级包也包含累计变更说明和发布工具。桌面端与 Web 的“帮助 → 变更说明”从本部署读取这些记录，不需要访问官网。发布器会复用已导入的前端镜像运行，使用 `--pull never`，目标机无需另装 Node。

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

桌面端安装器单独提供，不在服务端升级归档中。按客户端架构选择文件：

| 客户端架构 | 安装器文件名 | 自动更新元数据 |
| --- | --- | --- |
| Windows x86，32 位 | `multica-desktop-<版本>-windows-ia32.exe` | `latest-ia32.yml` |
| Windows x64，64 位 | `multica-desktop-<版本>-windows-x64.exe` | `latest.yml` |
| Windows ARM64 | `multica-desktop-<版本>-windows-arm64.exe` | `latest-arm64.yml` |

ia32 安装器中的桌面程序和 CLI 均为 32 位；x64 安装器用于 x86-64。Linux 服务端的 `linux/amd64` 与 Windows 客户端架构独立选择。

内网更新目录应保留原始安装器文件名、对应的 `.exe.blockmap` 和各自的元数据文件。先放入安装器及 blockmap，再替换 `latest*.yml`，不要用 ia32 的元数据覆盖 x64 的 `latest.yml`。客户端继续使用 `%USERPROFILE%\.multica\desktop.json` 中配置的 `updateUrl`。
