# 本机桌面更新下载服务

本服务实现[桌面端内网升级方案](desktop-intranet-update-plan.zh-CN.md)的静态文件存储与发布部分。Nginx 使用独立 Compose 项目，默认地址为 `http://127.0.0.1:18080/desktop`。桌面更新复用现有 `electron-updater`，无需修改业务 API。

## 启动与停止

需要本机 Docker Engine / Docker Desktop 和 Compose 插件。收集、发布产物还需要安装好仓库依赖；客户端配置脚本需要 Node.js 22.18 或更新版本。

首次在联网开发机准备镜像（固定 digest 与 Compose 一致）：

```bash
docker pull nginx:stable-alpine@sha256:985220252f3863977e468f611ef118ebd01421289dd86ee1ae99cb068c3bce2b
make desktop-updates-up
make desktop-updates-status
curl -fsS http://127.0.0.1:18080/health
```

健康接口返回 `ok`。启动命令使用 `--pull never`，不会在启动时访问公网。Nginx 镜像运行时不安装软件。

```bash
bash scripts/desktop-updates.sh logs
make desktop-updates-down
make desktop-updates-up
```

停止或重建容器不会删除宿主机文件。Docker 正在运行时，服务按 `unless-stopped` 策略恢复；Mac 休眠、Docker 退出或主动停止容器时，下载服务不可用。

## 存储与发布

默认目录位于当前仓库，已由现有 `data/` 规则排除在 Git 外：

```text
data/desktop-updates/
├── public/       # Nginx 只读挂载；安装包、ZIP、blockmap、latest*.yml
├── staging/      # 发布过程的临时文件，不对客户端提供
├── backups/      # 发布工具保留的历史元数据，不对客户端提供
└── .publish.lock # 发布期间的互斥锁
```

可以用 `DESKTOP_UPDATES_STORAGE=/绝对路径` 指定其他磁盘。启动和发布必须使用同一个目录。不要把已有的 `apps/desktop/dist` 直接挂载到 Nginx：后续桌面打包会清空它。

正式发布需要显式稳定版本号，例如：

```bash
MULTICA_DESKTOP_VERSION=0.5.1 \
  pnpm --filter @multica/desktop package -- --win --x64 --publish never
make desktop-updates-publish SOURCE=apps/desktop/dist
```

如果产物已经由管理员转入本机，只需要执行第二步，并将 `SOURCE` 改为对应目录。多架构构建应逐次收集归档，避免下一次打包清空之前的产物。

发布工具读取打包生成的 `latest*.yml`，核对引用文件的路径、大小和 SHA-512。它会拒绝公网 URL、目录跳转、缺失或损坏的文件、冲突的同名文件；先将产物放入在线目录，最后原子替换各通道元数据。带版本的文件不允许替换为其他内容，旧安装包会保留。

发布命令默认拒绝预发布版本。需要验证本机已有的 `-dirty` 等测试构建时，显式使用：

```bash
make desktop-updates-publish SOURCE=apps/desktop/dist ALLOW_PRERELEASE=1
```

这个开关只用于开发验证，不将预发布版本改写成稳定版，也不保证稳定版客户端会自动发现它。

只整理离线交付文件、暂不发布时：

```bash
node apps/desktop/scripts/update-artifacts.mjs collect \
  --source apps/desktop/dist --destination dist/desktop-release
```

离线组合打包 `scripts/offline-installer.sh` 也复用此收集逻辑，包含安装包、更新 YAML、ZIP 和生成的 blockmap。Windows 32 位可使用 `--desktop-target win-ia32`。显式指定稳定的 `VERSION`；测试构建需加 `--allow-prerelease`。

## 下载验证

根据实际发布的平台选择 YAML，Windows ia32 使用 `latest-ia32.yml`，Windows x64 使用 `latest.yml`。

```bash
curl -fsS http://127.0.0.1:18080/desktop/latest-ia32.yml
curl -sS -D - -o /dev/null -H 'Range: bytes=0-1023' \
  http://127.0.0.1:18080/desktop/实际安装包文件名.exe
```

Range 请求应返回 `206`，响应头包含 `Content-Range` 和 `Cache-Control: no-cache`。所有发布文件都需要可直接下载；目录列表被关闭，所以打开 `/desktop/` 返回 `403/404` 是正常行为。写请求、隐藏文件以及下载目录之外的路径不开放。

## 接入桌面客户端

先在正式安装的 Desktop 中配置业务服务器，然后完整退出应用。执行以下命令会保留已有 JSON 字段，仅合并 `updateUrl`；原始文件备份到同目录的 `desktop.json.<唯一标识>.bak`：

```bash
bash scripts/desktop-updates.sh configure
# 或显式指定另一份客户端配置
bash scripts/desktop-updates.sh configure --config /绝对路径/desktop.json
```

默认配置文件为当前用户的 `~/.multica/desktop.json`。文件缺失或无效时，脚本会拒绝修改。之后重新打开 Desktop，在更新设置中检查更新。

`pnpm dev:desktop` 使用开发环境业务配置，且更新器仅在打包应用中启用；不能用开发窗口代替真实安装升级验收。本机服务已经提供下载，也不意味着当前 Mac 能安装服务中的 Windows 包。

## 让其他内网终端访问

默认仅本机可连接。需要局域网访问时，用下面的命令重新配置端口监听，并在防火墙中允许该端口：

```bash
DESKTOP_UPDATES_BIND=0.0.0.0 make desktop-updates-up
```

也可将 `DESKTOP_UPDATES_BIND` 设为本机指定的内网地址。每次重新执行启动命令时都要使用相同的环境变量；不设置会恢复默认的本机监听。其他电脑的更新源必须填写下载服务器的内网 IP 或域名，不能填写 `127.0.0.1`：

```bash
DESKTOP_UPDATES_URL=http://下载服务器内网IP:18080/desktop \
  bash scripts/desktop-updates.sh configure --config /客户端配置副本/desktop.json
```

端口可用 `DESKTOP_UPDATES_PORT` 修改；客户端可见地址通过 `DESKTOP_UPDATES_URL` 设置。长期内网部署应采用固定域名和受信任的 HTTPS 证书，TLS 可由已有内网反向代理终结。

完全隔离环境先在联网机器保存镜像，再转入内网：

```bash
docker tag nginx:stable-alpine@sha256:985220252f3863977e468f611ef118ebd01421289dd86ee1ae99cb068c3bce2b \
  multica-desktop-nginx:985220252f38
docker save multica-desktop-nginx:985220252f38 -o nginx-desktop-updates.tar
# 在内网服务器执行
docker load -i nginx-desktop-updates.tar
DESKTOP_UPDATES_IMAGE=multica-desktop-nginx:985220252f38 make desktop-updates-up
```

离线 `docker save/load` 不保证保留 RepoDigest，所以这里显式使用已导入的本地标签。镜像归档应来自已核验的固定 digest，同时复制 Compose、Nginx 配置和脚本。发布操作可在具有仓库依赖的内网构建机执行，再转移经验证的存储目录；Nginx 服务器本身不需要 Node.js。

## 故障与恢复

- 启动提示缺少镜像：在联网机器准备固定镜像或导入离线归档，启动命令不会自行下载。
- 端口占用：换用 `DESKTOP_UPDATES_PORT` 并同步客户端地址。
- 元数据校验失败：修正源产物，重新发布；不要手工改 YAML 中的校验值。
- 发布中断：旧元数据仍可引用已完整发布的文件；多个通道分别切换，不保证同时切换。确认没有发布进程后再处理遗留锁和临时目录，随后重试。
- 需要撤回坏版本：先备份当前 YAML，从 `backups/` 选出对应旧 YAML，在 `public/` 同一文件系统中写临时文件后重命名替换。保留所有被引用的旧产物，并验证下载。已经安装或下载的客户端不会因此自动回滚，使用更高版本修复版或受控覆盖安装。
- 需要恢复客户端更新源：退出 Desktop，用保存的 `.bak` 覆盖配置文件后重新启动。

实际端到端验收仍按[方案中的验收标准](desktop-intranet-update-plan.zh-CN.md#11-验收标准)执行，包括签名、安装权限、断开公网、重启后版本与用户状态。HTTP 下载测试不能代替这些验证。
