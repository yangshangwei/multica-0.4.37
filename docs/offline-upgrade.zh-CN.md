# Multica 内网离线升级操作文档

本文档适用于已经拿到 Multica 服务端离线升级包、且服务器不能访问公网的场景。

升级包已经基于指定源码构建完成。内网服务器不需要源码、Node.js、pnpm 或 Go 工具链，只需要 Docker 和 Docker Compose。

升级包包含后端、前端和 PostgreSQL 镜像。后端启动时会自动执行数据库迁移。升级过程中不要执行 `docker compose down -v`，也不要删除 `pgdata` 或 `backend_uploads` 数据卷。

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
5. 使用升级包中的 Compose 文件启动后端和前端；
6. 等待 `/healthz` 返回成功。

脚本不会覆盖 `/opt/multica/.env`，不会删除 Docker 数据卷。

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
docker compose -f docker-compose.selfhost.yml ps
docker compose -f docker-compose.selfhost.yml logs --tail=200 backend
curl -fsS http://127.0.0.1:8080/health
curl -fsS http://127.0.0.1:8080/healthz
```

如果后端端口不是 `8080`，使用现有 `.env` 中配置的端口。

## 四、失败处理

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
