# 桌面端内网升级手册复验记录

验证日期：2026-09-24。对应[操作手册](desktop-intranet-update-runbook.zh-CN.md)。

第 1 至 10 节保留首次复验记录；后续新增运维脚本的验证结果见第 11 节。后续的脚本检查补充了原有证据，真实客户端安装仍需单独验收。

上次保存手册时，实际执行的是文档本地链接检查、Shell 语法检查和 Compose 配置解析。没有执行手册中的镜像拉取、部署、发布或客户端升级命令。本次重新执行静态检查，并增加了真实文件下载、临时 Nginx 部署、发布失败保护、客户端配置工具和容器重启验证。

结论：本次静态检查、14 个本机验证检查项、121 项桌面测试及 12 项离线脚本测试通过。离线脚本套件另有 3 项 opt-in 测试未启用。真实内网终端的安装升级仍未验收。

## 1. 实际测试环境

| 项目 | 本次实际值 |
| --- | --- |
| 执行环境 | 本机 macOS，Docker 中的 Nginx 为 `linux/arm64` |
| Node.js | `v22.23.2` |
| pnpm | `10.28.2` |
| Docker Compose | `v5.0.2` |
| 现有下载服务 | `http://127.0.0.1:18080/desktop` |
| 临时下载服务 | `http://127.0.0.1:58900/desktop`，验证后已删除 |
| 实际安装包 | `multica-desktop-0.4.48-dirty-windows-ia32.exe` |
| 安装包字节数 | `165407520` |
| 最终本机集成复验 | 北京时间 2026-09-24，精确时间见机器可读记录 |

临时服务使用手册中的同一份 Compose 和 Nginx 配置。为避免影响现有服务，使用独立项目名、临时存储、仅本机监听和单独的固定端口。手册中的 `10.10.10.20`、`/srv/multica-updates` 和 `0.5.1` 是示例值，本次没有在这些目标上部署。

## 2. 文档静态检查

检查操作手册、本机操作说明和设计方案三个 Markdown 文件：

1. 提取 Markdown 本地链接，按文档所在目录解析路径，检查目标文件存在：28 处通过。
2. 提取操作手册中的 12 个 `bash` 代码块，逐段传给 `bash -n`：全部通过。
3. 检查代码块围栏成对，并运行 `git diff --check`；新文件额外使用 `git diff --no-index --check /dev/null <文件>`：无空白错误。

这一步不检查外部网站和 Markdown 标题锚点，也不执行代码块。`bash -n` 只证明 Shell 语法可解析，不能证明镜像存在、网络可达或安装成功。

## 3. Compose 配置和运行中的 Nginx

把手册中的 `dotenv` 代码块原样写入临时 `updates.env`，使用复制后的部署配置解析：

```bash
docker compose --env-file updates.env \
  -f docker-compose.desktop-updates.yml config --format json
```

实际逐项断言：

- 镜像为 `multica-desktop-nginx:985220252f38`。
- 主机监听为 `0.0.0.0:18080`，映射到容器 `8080`。
- 存储源为 `/srv/multica-updates/public`，且是只读挂载。

然后检查实际运行服务：

```bash
docker compose -f docker-compose.desktop-updates.yml \
  exec -T desktop-updates nginx -t

docker inspect multica-desktop-updates-desktop-updates-1
```

结果：Nginx 配置语法检查通过，容器为 `healthy`，根文件系统和安装包挂载均只读。实际绑定为 `127.0.0.1:18080`，并没有因此开放局域网访问。

## 4. 实际 HTTP 下载和校验

从现有服务读取 `latest-ia32.yml`，用项目安装的 `electron-updater` 元数据解析器获取文件名、大小和 SHA-512。再用真实 HTTP 请求验证以下项目：

| 请求或检查 | 预期 | 实际 |
| --- | --- | --- |
| `GET /health` | `200`，正文 `ok` | 通过 |
| `GET /desktop/latest-ia32.yml` | `200`，与磁盘文件逐字节一致 | 通过 |
| 元数据缓存头 | `Cache-Control: no-cache` | 通过 |
| 安装包 HEAD | `200`，长度与 YAML 相同 | `165407520` 字节 |
| `Range: bytes=0-1023` | `206`，正确的 Content-Range 和前 1024 字节 | 通过 |
| 安装包完整 GET | 实际字节数和 SHA-512 都与 YAML 一致 | 通过 |
| blockmap GET | 与磁盘文件逐字节一致 | 通过 |
| 目录列表、缺失文件、隐藏路径、备份和暂存路径 | 不可读取 | 均为 `404` |

完整下载计算得到的 Base64 SHA-512：

```text
gy6kj7te2nV68aCXU+VGunibV3lSW9PXWRQGki20NhUaD5w60X9agOGs99pLUmKccxt7RO9dlXaxOG95pG9JLA==
```

可以在当前机器复查：

```bash
curl -fsS http://127.0.0.1:18080/desktop/latest-ia32.yml

curl -fsSI \
  http://127.0.0.1:18080/desktop/multica-desktop-0.4.48-dirty-windows-ia32.exe

curl -fsS -D - -o /dev/null -H 'Range: bytes=0-1023' \
  http://127.0.0.1:18080/desktop/multica-desktop-0.4.48-dirty-windows-ia32.exe

curl -fS -o /tmp/multica-desktop-0.4.48-dirty-windows-ia32.exe \
  http://127.0.0.1:18080/desktop/multica-desktop-0.4.48-dirty-windows-ia32.exe

openssl dgst -sha512 -binary /tmp/multica-desktop-0.4.48-dirty-windows-ia32.exe \
  | openssl base64 -A
```

本次自动验证按块读取整个文件计算 SHA-512，没有用 HEAD 成功代替完整下载验证。

## 5. 镜像归档、产物发布和失败保护

这部分在临时目录及独立 Compose 项目中执行：

1. 给现有固定 digest 的镜像增加唯一的临时标签。
2. 执行 `docker save`，生成 `26896896` 字节的镜像归档。
3. 删除这个临时标签，再执行 `docker load` 恢复标签，确认镜像架构是 `linux/arm64`。
4. 对现有真实安装包执行收集。默认稳定版本策略拒绝 `0.4.48-dirty`，符合预期。
5. 显式使用 `--allow-prerelease` 后，完成文件收集并发布到临时存储。
6. 构造无效发布源：只把 YAML 中的文件大小从 `165407520` 改成 `165407519`，安装包不变。
7. 再次发布，实际返回 `Update size mismatch`；逐一比较已发布元数据和文件的指纹，确认原发布内容完全未变。
8. 使用临时标签和 `--pull never` 启动独立 Nginx，重复第 4 节的完整下载验证。
9. 在临时公开目录放置一个真实的隐藏测试文件，确认 HTTP 仍返回 `404`；发送 PUT 到专用测试路径，返回 `403`，未生成文件。

收集和发布实际调用的命令形式：

```bash
node apps/desktop/scripts/update-artifacts.mjs collect \
  --source data/desktop-updates/public \
  --destination /临时目录/release \
  --allow-prerelease

node apps/desktop/scripts/update-artifacts.mjs publish \
  --source /临时目录/release \
  --destination /临时目录/storage \
  --allow-prerelease
```

其中临时路径每次生成不同的值。本次验证只移除了临时镜像标签，已有服务所用的镜像层仍存在。因此证明的是本机镜像归档导出、导入和启动流程，不能代替在另一台干净内网服务器上的镜像导入验收。固定 digest 的远端镜像索引另外确认包含 `linux/amd64` 和 `linux/arm64`，但本次没有实际运行 x64 镜像。

## 6. 客户端 URL 配置工具

创建一份临时 `desktop.json`，包含业务 API、Web、WebSocket 地址，以及一个自定义字段，然后执行真实配置工具：

```bash
node apps/desktop/scripts/configure-updates.mjs \
  --url http://127.0.0.1:58900/desktop \
  --config /临时目录/desktop.json
```

逐项比较结果：

- 增加了预期的 `updateUrl`。
- 原业务地址和自定义字段保持不变。
- 自动生成的备份与原始配置一致。
- 对同一 URL 再执行一次，返回 `changed: false`。

实际用户的 `~/.multica/desktop.json` 未被读取或修改。这一步验证配置文件工具，不证明已经安装的 Electron 客户端完成了版本升级。

## 7. 重启验证，以及验证脚本自身的修正

最初的临时测试把主机端口设为 `0`，让 Docker 自动分配。重启后端口变化，而检查脚本还访问旧端口，导致一次健康检查超时。

单独复现结果：重启前地址是 `127.0.0.1:55002`，重启后是 `127.0.0.1:55003`，新地址的 `/health` 返回 `ok`。这是验证环境的随机端口问题。

随后把临时环境改为与手册相同的固定端口方式，重新执行完整集成复验：

1. 使用独立固定端口 `58900` 启动。
2. 执行 `docker compose ... restart desktop-updates`。
3. 重新读取实际端口映射，断言仍为 `127.0.0.1:58900`。
4. 再次读取元数据、执行 HEAD、Range、完整安装包下载和 blockmap 比较。
5. 全部通过，安装包仍为 `165407520` 字节，SHA-512 与重启前一致。

另一个验证脚本修正是正确处理 `git diff --no-index` 的差异退出码：新文件对比 `/dev/null` 可以返回 `1`，不能把它直接当成空白检查失败。最终同时检查允许的退出码以及是否存在错误输出。

结束后删除临时容器、网络、标签和文件，确认原有 `18080` 服务仍健康，容器启动时间未变。没有重启原服务。

## 8. 重新执行的自动化测试

桌面更新器、偏好、运行配置、更新界面、打包、配置工具和产物发布测试：

```bash
pnpm -C apps/desktop exec vitest run \
  src/main/updater.test.ts \
  src/main/updater-preferences.test.ts \
  src/shared/runtime-config.test.ts \
  src/main/runtime-config-loader.test.ts \
  src/renderer/src/components/updates-settings-tab.test.tsx \
  src/renderer/src/components/update-notification.test.tsx \
  scripts/package.test.mjs \
  scripts/configure-updates.test.mjs \
  scripts/update-artifacts.test.mjs
```

结果：9 个测试文件、121 项测试通过，0 失败。

离线交付脚本测试：

```bash
node --test scripts/offline-changelog.test.mjs
```

结果：12 项通过，0 失败，3 项 opt-in Docker 测试跳过。这里的跳过项不能记为通过；第 5 至 7 节的临时 Nginx 实测是另外执行的验证。

本次没有产品代码修改，没有重新运行全仓库 lint、typecheck、Go 或 E2E。pnpm 输出已有的根 `package.json` 中 `pnpm` 配置字段不再读取的警告，不影响本次测试退出码。

## 9. 尚未验证的内容

- 另一台真实内网服务器的部署、终端到服务器的路由、防火墙和 DNS。
- 实际受信任的 HTTPS 证书链。
- 全程阻断公网情况下的真实客户端更新。
- 稳定版本 A 到 B 的签名安装、Windows 权限提升、macOS 公证及 Linux 各包格式安装。
- 重启后登录状态、业务配置、用户数据和正在运行的任务。

下一步仍应按操作手册，选择一台真实安装了稳定旧版的终端完成 A → B 升级验收。

## 10. 原始证据

- [机器可读复验摘要](../.trellis/tasks/archive/2026-09/09-24-desktop-intranet-updates/reverification-2026-09-24.json)：本次检查项、实际值、时间和范围限制。
- 本机完整命令输出：`.omx/reports/desktop-updates-recheck-20260924/results.json`。
- 本机复验脚本：`.omx/reports/desktop-updates-recheck-20260924/verify.py`。
- 首次重启检查失败及端口变化证据：同目录的 `results-before-port-fix.json` 和 `restart-port-probe.json`。

`.omx/reports/` 是本机运行产物，不随文档提交保存；上述 Markdown 和机器可读摘要保留可审阅的结论。复验脚本只用于此次现有 ia32 测试包，未来发布新版本时应使用操作手册中的实际版本和架构参数。

## 11. 新增运维脚本后的验证

2026-09-24 北京时间 23:27:00 至 23:27:39，完成了新命令的实际 Docker 演练。持久保存的结果见[运维脚本验证摘要](../.trellis/tasks/archive/2026-09/09-24-desktop-intranet-updates/operations-verification.json)。

本次增加了 `updates.env` / `--env-file`、`export-image`、`collect` 和 `verify`。`publish`、`configure` 继续调用现有实现；HTTP 校验与发布工具复用同一套元数据引用校验。Shell 行为测试已加入 CI。

### 11.1 自动化测试与静态检查

先用新行为测试确认功能缺失，再实现并重新验证。Shell 测试第一次有 10 项失败，HTTP 校验器的正常下载用例第一次明确返回未实现错误；实现完成后：

| 检查 | 结果 |
| --- | --- |
| 桌面测试，包含新增真实 HTTP 测试服务器用例 | 10 个文件，162 项通过 |
| Shell 管理脚本测试 | 13 项通过 |
| 离线脚本回归 | 12 项通过，3 项原有 opt-in 测试跳过 |
| Desktop node/web typecheck | 通过 |
| Desktop lint | 0 错误，1 条原有 `tab-content.tsx:54` Hook 警告 |
| Shell 和 Node 语法 | 通过 |
| 文档与配置 | 本地链接、Bash 示例语法、围栏完整性、任务 JSON/JSONL、CI YAML 及 Compose 示例展开均通过 |

新增 HTTP 用例覆盖错误版本、文件损坏、错误大小、缺失文件、元数据格式、危险文件名、HTTP 重定向、Range 状态和内容错误、缓存头、内容编码、下载上限及超时。Shell 用例覆盖带空格路径、固定配置、不执行配置文本、失败退出、旧命令兼容及镜像归档不覆盖。

在第 8 节桌面测试命令末尾增加 `scripts/verify-updates.test.mjs` 即可复跑。Shell 和离线脚本可一起运行：

```bash
node --test scripts/desktop-updates.test.mjs scripts/offline-changelog.test.mjs
pnpm -C apps/desktop lint
pnpm -C apps/desktop typecheck
bash -n scripts/desktop-updates.sh
node --check apps/desktop/scripts/verify-updates.mjs
```

### 11.2 真正执行的新命令

在独立 Compose 项目、带空格的临时存储目录和固定本机端口中执行，共 10 个检查项通过：

1. 使用新的 `verify` 校验现有 `18080` 下载源。完整读取 `165407520` 字节安装器，SHA-512 与元数据一致。
2. 执行 `export-image <临时归档> linux/arm64`，再执行 `docker load`。导出归档为 `26886656` 字节，导入镜像平台为 `linux/arm64`。导出、检查和保存都明确选择平台。
3. 仅复制管理脚本、Compose、Nginx 配置和 env 模板，以保存的 `updates.env` 执行 `start`、`status`。此时临时目录没有 Node 工具，服务仍启动健康，存储中的空格正确处理。
4. 在真实仓库中，通过 `--env-file <临时配置>` 执行 `collect`、`publish`、`verify`，确认它们使用与临时 Nginx 相同的存储及 URL。测试包显式追加 `--allow-prerelease`。
5. 把预期版本设为不存在的 `0.5.1`，`verify` 返回非零退出码和明确的版本不匹配错误。
6. 对临时客户端配置执行 `configure`，确认读取保存的更新 URL、保留业务和自定义字段、备份一致。
7. 重启临时 Nginx，再次执行 `verify`，地址不变，安装包结果一致。
8. 故意使用另一份存储配置执行 `status`，确认输出仍展示实际运行容器的存储挂载。
9. 执行 `stop`，确认临时发布文件仍存在；随后只清理测试目录。
10. 检查原有 `18080` 容器，仍健康且启动时间未变。

临时服务在验证后已删除。此次演练的发布和客户端配置没有作用于实际用户环境。blockmap 结果明确为 `checksumVerified: false`：校验了传输自洽性，但 YAML 没有提供它的预期源摘要。

演练初次尝试曾用符号链接连接临时目录与仓库脚本，触发现有 Node CLI 的入口路径判断而没有执行命令；因此没有将退出码 0 当成成功。改为从真实仓库执行 Node 操作、显式传服务器 env 文件后，重新跑完全部检查。最终验证依据是 JSON 内容和实际发布文件，不只看命令退出码。

### 11.3 保留的边界

本次 Docker 实测使用本机 ARM64 Linux 容器和 Windows ia32 测试安装包；x64 镜像导出覆盖了命令回归测试，未进行实际 x64 服务器部署。镜像导出阶段按设计访问镜像仓库；没有进行物理断网验收。签名、真实 A → B 安装和用户状态仍按第 9 节验收。

本机完整输出及演练脚本位于 `.omx/reports/desktop-updates-operations/`，机器可读摘要已保存到任务目录。前一次符号链接测试环境失败记录为该目录的 `results-before-fixture-fix.json`。
