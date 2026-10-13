# 本机桌面更新下载服务

本服务提供首次安装下载页及[桌面端内网升级方案](desktop-intranet-update-plan.zh-CN.md)的文件存储与发布。Nginx 使用独立 Compose 项目，默认下载页为 `http://127.0.0.1:18080/desktop/`，客户端更新目录仍为 `http://127.0.0.1:18080/desktop`。静态页只依赖同源文件；桌面更新复用现有 `electron-updater`，无需修改业务 API。

部署到内网服务器并持续发布新版本时，参见[桌面端内网部署与升级操作手册](desktop-intranet-update-runbook.zh-CN.md)，其中包含镜像离线导入、持久化配置、客户端 URL 和日常升级检查清单。

## 启动与停止

需要本机 Docker Engine / Docker Desktop 和 Compose 插件。收集、发布产物还需要安装好仓库依赖；客户端配置脚本需要 Node.js 22.18 或更新版本。

以下未指定配置文件的示例使用本机默认值。持续运维时，先从 [updates.env.example](../deploy/desktop-updates/updates.env.example) 创建仓库根目录的 `updates.env`，填写实际存储、监听、镜像和客户端 URL。管理脚本会自动读取它，也支持 `bash scripts/desktop-updates.sh --env-file /绝对路径/updates.env <命令>`。根目录 `updates.env` 已被 Git 忽略。

配置按 Compose dotenv 规则解析，不作为 Shell 执行；进程中已有的同名环境变量优先。自定义配置下执行 `configure` 或 `verify` 时需要明确填写 `DESKTOP_UPDATES_URL`，避免把内网客户端错误地指向其自身的 `127.0.0.1`。

首次在联网开发机准备镜像（固定 digest 与 Compose 一致）：

```bash
docker pull nginx:stable-alpine@sha256:985220252f3863977e468f611ef118ebd01421289dd86ee1ae99cb068c3bce2b
make desktop-updates-up
make desktop-updates-status
curl -fsS http://127.0.0.1:18080/health
```

健康接口返回 `ok`。启动命令使用 `--pull never`，不会在启动时访问公网。Nginx 镜像运行时不安装软件。

启动脚本原子安装 `deploy/desktop-updates/download.html`，并只在缺失时从模板创建 `public/download-config.json`。首次使用请编辑持久化配置，填入实际业务 HTTP(S) 地址：

```json
{
  "schemaVersion": 1,
  "serverUrl": "http://业务服务器:8080"
}
```

默认配置中的地址为空；示例必须替换为实际值，且不能包含凭据。`serverUrl` 是客户端首次连接的业务入口，与下载服务器、`DESKTOP_UPDATES_URL` 和客户端 `updateUrl` 无关。重复启动、页面升级和版本发布都保留管理员配置。页面首期只开放 Windows x64 当前版本，其他四个平台/架构显示“待开放”，并提供从安装到进入工作台的中文指导；对应业务环境应已启用密码认证和自助注册。

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
├── public/       # Nginx 只读挂载
│   ├── download.html        # 启动脚本更新的页面
│   ├── download-config.json # 管理员维护的业务地址
│   ├── download.json        # Windows x64 发布生成的版本信息
│   └── ...                  # 安装包、ZIP、blockmap、latest*.yml
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

发布工具读取打包生成的 `latest*.yml`，核对引用文件的路径、大小和 SHA-512。它会拒绝公网 URL、目录跳转、缺失或损坏的文件、冲突的同名文件；先将完整产物放入在线目录，再原子替换各通道元数据，Windows x64 最后替换 `download.json`。下载版本信息从已验证的 `latest.yml` 生成，不使用源目录中的手写 JSON；其他平台发布不覆盖它。带版本的文件不允许替换为其他内容，旧安装包会保留。刷新页面即可看到新版本，不需编辑 HTML 或版本号。

发布命令默认拒绝预发布版本。需要验证本机已有的 `-dirty` 等测试构建时，显式使用：

```bash
make desktop-updates-publish SOURCE=apps/desktop/dist ALLOW_PRERELEASE=1
```

这个开关只用于开发验证，不将预发布版本改写成稳定版，也不保证稳定版客户端会自动发现它。

只整理离线交付文件、暂不发布时：

```bash
bash scripts/desktop-updates.sh collect apps/desktop/dist dist/desktop-release
```

离线组合打包 `scripts/offline-installer.sh` 也复用此收集逻辑，包含安装包、更新 YAML、ZIP 和生成的 blockmap，Windows x64 还包含下载页用的 `download.json`。Windows 32 位可使用 `--desktop-target win-ia32`，但不会打开下载页入口。显式指定稳定的 `VERSION`；测试构建需加 `--allow-prerelease`。

已有 Nginx 文件服务首次增加页面时，不能只复制 HTML：先使用新版 `collect` 收集当前完整 Windows x64 发布，再用 `publish` 发布生成的目录。已有同版本、同字节安装包会保留。例如：

```bash
bash scripts/desktop-updates.sh collect \
  /已保存的当前Windows完整产物 dist/current-windows-with-page
bash scripts/desktop-updates.sh publish dist/current-windows-with-page
```

若当前只有 ia32 等其他架构，Windows x64 下载仍不可用，不要伪造投影。更新 Nginx 路由后按[手册第 2.5 节](desktop-intranet-update-runbook.zh-CN.md#25-为已有文件服务接入下载页)完成配置检查及 reload；普通 `start` 复用容器时不会自动让 Nginx 重新读取配置。后续发包无需重启服务。

## 下载验证

根据实际发布的平台选择 YAML，Windows ia32 使用 `latest-ia32.yml`，Windows x64 使用 `latest.yml`。

自动验证会实际下载元数据引用的完整文件，检查预期版本、大小、SHA-512、HEAD 和 Range，并输出 JSON 证据。以当前本机测试包为例：

```bash
bash scripts/desktop-updates.sh verify latest-ia32.yml \
  --expected-version 0.4.48-dirty
```

正式发包时将预期版本改为本次稳定版本，按每个实际发布通道分别检查。失败返回非零退出码；下载检查不证明安装或用户状态保留。当前测试包不会因为校验成功而成为正式稳定发布。

手动排查时：

```bash
curl -fsS http://127.0.0.1:18080/desktop/latest-ia32.yml
curl -sS -D - -o /dev/null -H 'Range: bytes=0-1023' \
  http://127.0.0.1:18080/desktop/实际安装包文件名.exe
```

Range 请求应返回 `206`，响应头包含 `Content-Range` 和 `Cache-Control: no-cache`。`/desktop/` 应返回静态页面，`/desktop` 规范化到带斜杠入口，目录列表仍关闭。检查 `/desktop/download.json` 的版本、文件名、大小和页面下载链接对应当前 Windows x64 发布；缺失或无效时页面保留教程并提示暂不可下载。写请求、隐藏文件以及下载目录之外的路径不开放。

## 接入桌面客户端

包含自动配置功能的 Desktop 会在保存业务服务器地址时写入 `http://业务服务器主机:18080/desktop`，旧配置缺少该字段时会在启动时补齐。

使用其他下载主机、端口或协议，或者为 v0.5.2 配置首次升级时，先在正式安装的 Desktop 中配置业务服务器，再完整退出应用。执行以下命令会保留已有 JSON 字段，仅合并 `updateUrl`；原始文件备份到同目录的 `desktop.json.<唯一标识>.bak`：

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

也可将 `DESKTOP_UPDATES_BIND` 设为本机指定的内网地址。建议将这些值保存到 `updates.env`；若仅通过临时环境变量设置且没有配置文件，下次未设置时仍会回到默认的本机监听。其他电脑的更新源必须填写下载服务器的内网 IP 或域名，不能填写 `127.0.0.1`：

```bash
DESKTOP_UPDATES_URL=http://下载服务器内网IP:18080/desktop \
  bash scripts/desktop-updates.sh configure --config /客户端配置副本/desktop.json
```

端口可用 `DESKTOP_UPDATES_PORT` 修改；客户端可见地址通过 `DESKTOP_UPDATES_URL` 设置。长期内网部署应采用固定域名和受信任的 HTTPS 证书，TLS 可由已有内网反向代理终结。

完全隔离环境先在联网机器准备镜像，再转入内网。以下示例针对 Linux x64 下载服务器；ARM64 服务器将平台改为 `linux/arm64` 并使用脚本输出的对应标签：

```bash
bash scripts/desktop-updates.sh export-image nginx-desktop-updates.tar linux/amd64
```

在内网服务器执行：

```bash
docker load -i nginx-desktop-updates.tar
DESKTOP_UPDATES_IMAGE=multica-desktop-nginx:985220252f38-amd64 make desktop-updates-up
```

离线 `docker save/load` 不保证保留 RepoDigest，所以这里显式使用已导入的本地标签。长期运维将该标签写入 `updates.env`，以脚本实际输出为准。导出命令从 Compose 读取固定 digest，拒绝覆盖已有归档；同时转入 Compose、Nginx 配置、管理脚本、env 模板、`download.html` 和 `download-config.example.json` 即可启停服务，完整归档命令见[手册第 2.1 节](desktop-intranet-update-runbook.zh-CN.md#21-准备配置文件)。发布操作可在具有仓库依赖的内网运维机执行；Nginx 服务器本身不需要 Node.js，Node 发布和校验工具仍需要仓库及依赖。手工转入已校验发布时先放完整安装包，再替换 YAML，Windows x64 最后替换生成的 `download.json`，备份两份旧指针且不要覆盖业务地址配置，具体命令见[手册第 4.3 节](desktop-intranet-update-runbook.zh-CN.md#43-下载服务器只部署-nginx-时)。

## 故障与恢复

- 启动提示缺少镜像：在联网机器准备固定镜像或导入离线归档，启动命令不会自行下载。
- 端口占用：换用 `DESKTOP_UPDATES_PORT` 并同步客户端地址。
- 元数据校验失败：修正源产物，重新发布；不要手工改 YAML 中的校验值。
- 页面提示地址未配置：编辑持久化 `download-config.json` 的 `serverUrl`，不要填下载/更新目录。
- 页面提示下载不可用：检查投影、安装包 HEAD 与字节长度，首次接入需重新收集/发布当前完整 Windows x64 产物；复制 HTML 不会自动补齐 JSON。
- 发布中断：YAML 和 JSON 分别切换，不保证同时切换；最后 JSON 写入失败时发布命令报错，旧 JSON 仍可指向保留的完整旧包。确认没有发布进程后再处理遗留锁和临时目录，随后用同一发布源重试。
- 需要撤回坏版本：先备份当前 YAML 和 `download.json`，从 `backups/` 的同一备份点选出对应旧文件，在 `public/` 同一文件系统中写临时文件后分别重命名替换；首次接入前缺少 JSON 的备份点恢复为未就绪状态。保留所有被引用的旧产物，不覆盖 `download-config.json`，并验证下载通道和页面。已经安装或下载的客户端不会因此自动回滚，使用更高版本修复版或受控覆盖安装。
- 需要恢复客户端更新源：退出 Desktop，用保存的 `.bak` 覆盖配置文件后重新启动。

实际端到端验收仍按[方案中的验收标准](desktop-intranet-update-plan.zh-CN.md#11-验收标准)执行，包括签名、安装权限、断开公网、重启后版本与用户状态。HTTP 下载测试不能代替这些验证。
