# Research: V0.8.8 配置与 Linux 离线交付

- Query: 核查最近实际交付、Linux 完整包、配置传递/保留、认证、CLI/daemon 顺序，审查发布计划。
- Scope: internal；只读源码/历史交付，不运行构建、数据库、Docker 服务，不读真实 `.env` 或私有凭据。
- Date: 2026-11-26
- Source: 初始 root main 基线由主会话记录为 `9ffa37281184ae605bcbc0bc7a09caf3df60d180`；研究期间主会话通知修复已合并，完整 main SHA 为 `c920c5e5b0354258ad0a37d500655a6856d5ed5b`。附带配置哈希是研究快照，最终发布冻结后必须重做比对。

## Findings

### 1. 基线有比 v0.6.6 更近的实际交付

`dist/release/0.6.7/` 是当前本地 release 根目录中语义版本最高的实际交付目录，包含 22 个文件，包括完整服务端升级包、Windows EXE/blockmap/latest.yml/download.json 和三个验证 JSON。注意此目录是 `0.6.7`，没有 `v` 前缀。

- `README-v0.6.7.zh-CN.md:3`：源码固定 `9caea35808afc8d05fb6135bfc74d4bfb0b45cf1`。
- `verification-v0.6.7.json`：2026-10-17 本地离线交付，`status=passed`，本任务未发布 GitHub Release、未推标签；这不证明远端至今没有更新的正式 Release。
- `server-verification.json`：从 v0.6.6 升级 password/device 两种模式，594 迁移账本记录，数字前缀上限 570，保留数据/会话/密钥/UUID/挂载/卷和备份恢复；运行于 Docker Desktop amd64，不是真实物理 Linux 主机。
- v0.6.6 源码 `4797e6d971296e56f3edc4f6cb6d3f218d9f8d85`；`.trellis/tasks/archive/2026-10/10-13-release-v0.6.6/verification.md` 可作为详细方法参考。

最近正式远端发布基线由主会话检查 GitHub Release 和 tag ancestry 后补齐。不能把 v0.6.7 本地 `Unreleased` feed 作为已发布累计记录。

### 2. 已完成的配置差异（直接比较 v0.6.7 实际包字节）

`config-baseline-inventory.json` 记录八个配置/脚本文件的旧/新 SHA-256；`config-baseline-diff.patch` 是逐行差异。

| 文件 | 研究快照对 v0.6.7 实际包 |
| --- | --- |
| `.env.example` | 修改：3 个新增实际赋值，1 个新增注释配置项，LLM 用途说明扩展 |
| `docker-compose.selfhost.yml` | 修改：向 backend 新增传入下列 4 项 |
| `docker-compose.resource-publishing.yml` | 字节相同 |
| `offline-upgrade.sh` | 字节相同 |
| `install-changelog.sh` | 字节相同 |
| `initialize-platform-admin.sh` | 字节相同 |
| `scripts/install-changelog.mjs` | 字节相同 |
| `scripts/admin-init-compose.mjs` | 字节相同 |

| 变量 | 模板/服务默认 | 真实传递与升级处理 |
| --- | --- | --- |
| `MULTICA_DESKTOP_DOWNLOAD_URL` | 模板空；空/非法隐藏入口 | Compose backend:196；`server/cmd/server/main.go:357` 检查非法值；服务端公开配置为 Web 提供 URL。必须是无凭据绝对 HTTP(S) URL，保留客户原地址，不能猜测下载主机 |
| `MULTICA_COLLABORATION_MEMORY_MAX_BYTES` | 注释示例 131072；默认 128 KiB | Compose backend:118；`main.go:184` 解析，657 验证；合法 32768–524288，非法拒绝启动。已有文本可读，后续写遵守新限额 |
| `MULTICA_KNOWLEDGE_CONTEXT_ENABLED` | 模板/Compose true；未设或空也开 | Compose backend:124；`router.go:432,436` 仅空或字面 true 开启，其他非空值关闭。保留原显式 false；不得混跑旧的不受保护知识读取 API |
| `MULTICA_COLLABORATION_DIRECTORY_ENABLED` | 模板空；有效默认 true | Compose backend:139；`router.go:467` 只有字面 false 关闭。默认展示全部可用账号姓名/头像；与开放注册同时启用会警告。应在升级手册明确行为变化 |

此基线没有删除实际赋值变量或调整既有模板赋值默认值。`MULTICA_LLM_*` 没新增变量，但已有服务端模型配置现在也可能承担项目知识提取；`.env.example:153` 说明来源/比较文本范围与策略门槛，不能遗漏其用途变化。

对更老版本另外保留累计差异：v0.6.0/v0.6.1 的三个平台 CLI 安装命令、旧通用命令开始穿透 Compose、`FF_ITERATIONS_I1` 不再生效、工作空间迭代默认行为。不要将“相对0.6.7无删除”误表述成所有版本都无废弃项。

### 3. 配置真实传递与身份保留

`docker-compose.selfhost.yml` 显式列出 backend environment 白名单；仅写 `.env` 不保证进容器。最终验收必须用隔离 fixture `.env` 和实际完整 `-f` 顺序执行 Compose config，并检查运行容器选定变量是否等于测试值，报告只记字段名、相等布尔值和非秘密公开设置，不输出密钥原文。

- `DATABASE_URL` 由 `POSTGRES_USER/PASSWORD/DB` 拼成 `postgres:5432`；若实际使用外部数据库，必须保留覆盖文件里真正的 DATABASE_URL 和网络关系。
- `JWT_SECRET` 是 runtime environment，不是 build arg。构建传入的 `build-time-placeholder-not-shipped` 不进镜像（`offline-bundle.sh:174`）；用干净 worktree 避免自动加载真实 `.env`。
- Postgres 使用 `pgdata`，附件使用 `backend_uploads`；Skill/MCP/changelog 为目录 bind；resource-publishing overlay 加 `managed_resources`。不删卷、不改 project name，不把示例目录套到真实部署。
- 密码模式要求 `MULTICA_AUTH_MODE=password`、设备认证 false、Cloud 空；限流 single 或 shared+REDIS_URL（`server/internal/auth/password_config.go:26`）。设备模式保留 legacy+设备认证，普通升级不迁账号。
- 保留固定 `MULTICA_DEPLOYMENT_ID`、认证迁移窗口（若实际启用）、JWT 和各集成加密密钥、数据库账户、URL/端口、原 `ALLOW_SIGNUP`、安装命令与所有操作人员 overlay。
- `.env.example` 的注释数据库池、Realtime/Channel Redis 与 `MULTICA_FEATURE_FLAGS_FILE` 等不是本次新增，也并非全在标准 Compose 白名单中。已使用的自定义部署要保留显式 overlay/mount，不应自动扩大标准容器配置。
- `MULTICA_COLLABORATION_RUNNER_CONFIG` 是 daemon 宿主机配置（`server/internal/daemon/config.go:196`），不是业务 backend `.env`；需要同源 CLI/daemon 和实际能力探测，不能把变量塞进 backend 当作运行时验收。

### 4. 完整包与镜像来源

`build-offline-upgrade.sh:64` 调 `offline-bundle.sh`，再将源 `docs/offline-upgrade.zh-CN.md` 分别复制成包内 `README.md` 和 `操作文档.md`，附操作脚本、外层 SHA-256。不能只交付内部镜像 tar。

`offline-bundle.sh:180–202`：

1. 通过 `docker-compose.selfhost.yml` + `docker-compose.selfhost.build.yml` 从当前源码构建 `multica-backend:<VERSION>`、`multica-web:<VERSION>`。
2. `docker pull --platform linux/amd64 pgvector/pgvector:pg17`，这是第三方数据库镜像，不是本项目源码构建。
3. `docker save --platform linux/amd64 backend web db | gzip`；必须使用支持 `save --platform` 的 Docker。
4. 放入主 Compose、resource-publishing overlay、纯模板 `.env.example`、changelog/发布器/管理员初始化工具、空 Skill/MCP 目录及操作文档。

`Dockerfile` 内 backend 同时包含 server、multica CLI、migrate、两个 backfill 工具和全目录 SQL。`Dockerfile.web` 的运行镜像仍有 Node，离线发布器借它执行，不要求目标机安装 Node。

标准包只包含三个镜像。密码 single 模式不需要 Redis；已启用 shared 限流、外部对象存储、代理网关等部署要保留其现有服务/镜像，不得说包自动包含这些可选依赖。若本次另交付下载服务，则同时交付其 Nginx 镜像，不能以业务包代替。

源镜像标签 `golang:1.26-alpine`、`alpine:3.21`、`node:22-alpine`、`pgvector/pgvector:pg17` 都可变。记录构建拉取时间、真正 digest、构件配置/层摘要和实际 Go 版本；最终运行及导出的 `multica-images.tar.gz` 都应验证 linux/amd64。Docker Desktop host 默认 image inspect 可能看 ARM 变体，需 `docker image inspect --platform linux/amd64`，并核对保存 config 的 architecture/os、rootfs、运行容器 ImageManifestDescriptor 与 `uname -m`。

### 5. 升级顺序与失败门槛

必须顺序：停止写入及所有连接同数据库的旧 API → 备份配置/数据库/附件/资源 → 完整服务端/Web升级与迁移 → `/healthz`、原认证登录/原身份数据验证 → 匹配 Desktop/daemon → 最后开放桌面更新源。

`docs/offline-upgrade.zh-CN.md:21,63` 明确服务端先行；新版桌面浏览器登录还要求 Web 返回一次性 `desktop_state`。旧 daemon 无新上下文能力；知识不匹配拒绝不能靠不断重试绕过。

多 API 尤其不能滚动混跑：迁移 745 删除旧 user 查询字段，旧读者会报错导致登录失败；迁移 744 会默认开放协作新建。AI execution 和 host execution gate 保留关闭，不等于所有协作能力都关闭。设备模式不支持平台管理后台，无法在界面关闭新建，应明确这一变化（手册:499–532）。

入口 `docker/entrypoint.sh:1` 先 migrate up，失败用其退出码退出；迁移和 API 共用启动预算，收到 SIGTERM/INT 时终止迁移，不会继续 API。`offline-upgrade.sh:233` 用 `up -d --pull never backend frontend`，244 起检查实际公布端口 `/healthz`；失败保留已选镜像设置和备份，不自动回退。它只等待约120秒，而 backend DB startup 默认预算3分钟，慢启动失败应如实报告/检查，不可改成健康成功。

其自动数据库备份不含附件和资源卷，部署多写入者须在操作前停止写入。恢复旧镜像不逆转新 schema；需要同一恢复点的数据库、配置和文件数据，先隔离恢复演练，不能向已迁移非空库直接灌 dump。

### 6. CLI / daemon 必需物料与 fork 限制

- Linux backend 的 `/app/multica` 与服务端同源，但用户运行机器仍需 standalone Linux CLI archive；Windows installer 有 `resources/app.asar.unpacked/resources/bin/multica.exe`，要检查真的存在、amd64、版本0.8.8和VCS完整源码。
- 桌面 `bundle-cli.mjs:115` 每次构建 CLI，显式 `MULTICA_DESKTOP_VERSION=0.8.8` 也传给 CLI；缺 Go 会警告并可能 fallback，因此发布检查必须拒绝缺失/旧 bundled CLI。
- `.goreleaser.yml` 同时规定 legacy `multica_<os>_<arch>` 和 versioned `multica-cli-<version>-<os>-<arch>`，附 `checksums.txt`；本次最低需要 linux/amd64 与 windows/amd64，其他系统不得冒充已验证。
- **当前 CLI `multica update` 仍请求 upstream multica-ai/multica**（`server/internal/cli/update.go:229,255`）；桌面缺 CLI 的 bootstrap (`cli-bootstrap.ts:19`) 与 electron-builder publish owner 也仍是 upstream。fork Release 上传不改变这些代码。
- 不指导内网用户靠 `multica update` 获取本次 fork。使用随包匹配 CLI/内部安装命令并按既有生命周期替换，保留配置/设备身份；self-host daemon 自动更新默认关闭（`server/internal/daemon/config.go:565`）。Windows/下载源继续显式设置本次 `updateUrl`。

### 7. 可复用命令与验证工具

构建（只能最终冻结后执行）:

```bash
COPYFILE_DISABLE=1 VERSION=v0.8.8 COMMIT=<full-final-SHA> \
  bash scripts/build-offline-upgrade.sh --platform linux/amd64 \
  --output /Volumes/artisan/code/2026/multica-0.4.37/dist/release/v0.8.8/build \
  --changelog <exact-release-changelog-artifact>/changelog.json
```

建议后续生成最终公开目录时只收主包/校验/文档，私有演练和构建日志另放受限任务目录。macOS 必须 COPYFILE_DISABLE=1 防 AppleDouble；检查外层无链接/设备/穿越路径/私有凭据。最终两份指南与冻结源逐字节一致。

相关现成检查：

- `node --test scripts/offline-changelog.test.mjs scripts/offline-build-changelog.test.mjs scripts/install-changelog.test.mjs scripts/admin-init-compose.test.mjs scripts/release-changelog.test.mjs`
- `bash scripts/selfhost-config.test.sh`、`bash scripts/entrypoint.test.sh`
- 可选真实发布器冒烟：`MULTICA_RUN_DOCKER_CHANGELOG_SMOKE=1 MULTICA_CHANGELOG_SMOKE_IMAGE=<loaded-web> node --test scripts/offline-changelog.test.mjs`
- `.omx/qa/release-0.6.7/server-acceptance/acceptance.py`：password/device旧→新、管理员初始改密、备份恢复、原数据/卷/配置保留。
- `.omx/qa/release-0.6.7/verify-images.py`：内部镜像内容地址 blob、config/rootfs/architecture。
- `.artifacts/release-v0.6.6/rehearsal/`、`linux-evidence/`：早一版的同类工具与失败保留方法。

**不能直接运行旧脚本**：硬编码历史版本、目录/端口/项目名和“594条账本完全不变”。需复制到本任务私有演练路径，选空闲端口和隔离 project/DB，改成精确预期新迁移集合/实际 schema 断言，保留旧失败并记录修正。

依赖工具/资源：Docker Engine+Compose plugin、Go 支持补丁版、Node22、仓库 pnpm、tar/gzip/curl/SHA工具；Linux amd64 在 ARM Mac 上 emulation，Go/Web 构建慢且耗内存。历史 Web build 需要仅对 build RUN 提高 V8 heap 4096，不能泄漏 NODE_OPTIONS 到 runtime。优先原配方，确需修改先记录新确定源码/recipe哈希。构建前检查系统盘、Docker VM盘与目标卷；不能清理别的会话资源。

### 8. 三份计划评审结论

**PRD：PASS。** 覆盖用户的8项交付及等待依赖/安全约束，不需再次请求创建或推进授权。需最终注明上一实际交付0.6.7与正式远端基线是两个不同事实。

**design.md：PASS WITH REQUIRED CORRECTION before build。** 冻结单一最终源码、隔离构建与逐门槛验收正确，但要明确 exact cumulative changelog 的唯一来源。当前 release.yml 在 tag push 后生成含当次时间的 feed；本地先 prepare 后构建、最后 CI 再 prepare 会产生不同字节。建议 final main CI通过后 push已冻结tag，等CI产生 release-changelog artifact，用该 JSON构建本地包，最后核对CI镜像/本地包/Release一致。CI会按现有 workflow先发布基础Release，再补齐本地附件；只有附件及摘要核对完成才宣布总交付。若要求最终前完全不公开，则需事先设计审核workflow draft门槛，不能临时绕过publisher校验。

**implement.md：PASS WITH REQUIRED ADDITIONS before execution。** 在现有有序检查中加：

1. 冻结commit当前main CI和漏洞扫描通过，不能仅依据前一修复分支验证。
2. 将上面 exact changelog / tag / CI / 本地包顺序写清，防前后feed不同。
3. fork GoReleaser/desktop jobs不跑，需要 standalone CLI交付；可利用 `.github/RELEASING.md:104` 的 `desktop-smoke.yml` 手动对最终ref在Windows runner真实静默安装并核对x64内置CLI/PE。此workflow不测试GUI/跨版本自动升级，仍如实留缺口。
4. 0.6.7 password/device真实包升级和配置/卷/身份保留；旧支持起点按SQL代理定义矩阵，不能只测空库。
5. 进入新版本前停所有旧API（迁移745），保持AI/Host执行默认关闭并记录默认新建行为。
6. 记录三镜像来源和可选外部依赖边界；失败/回滚/备份恢复与最终包同一SHA。
7. 包含源/构件内配置和SQL清单、真正运行容器值、全部公开文件最终摘要；不含私有演练env。

上述是技术计划需补齐事项，现有授权足够直接修订与推进，不需要用户另行许可。研究不等于发布批准；构建/运行验收门槛仍未执行。

## Files found

- `scripts/build-offline-upgrade.sh` — 完整升级归档与双指南复制。
- `scripts/offline-bundle.sh` — 同源镜像构建、目标架构保存与模板/工具分发。
- `scripts/offline-upgrade.sh` — overlays/备份/持久镜像选择/healthz/管理员初始化。
- `docker-compose.selfhost.yml`, `.build.yml`, `.resource-publishing.yml` — runtime env 与构建、持久存储契约。
- `Dockerfile`, `Dockerfile.web`, `docker/entrypoint.sh`, `.dockerignore` — 镜像二进制/SQL/启动/上下文边界。
- `.env.example`, `server/cmd/server/main.go`, `router.go`, `server/internal/auth/password_config.go` — 配置解析与验证。
- `.github/workflows/release.yml`, `.github/RELEASING.md`, `.goreleaser.yml` — fork发布/CLI/CI路径。
- `.omx/qa/release-0.6.7/` — 已验收演练工具，但必须隔离适配。

## External references

未访问外部网页或 GitHub；此轮为仓库内源码和本地交付调查。远端发布状态/CI结论由主会话确认。可变镜像tag不可当作固定版本来源。

## Related specs

- `.trellis/spec/server/offline-delivery.md`
- `.trellis/spec/guides/changelog-release-flow.md`
- `.trellis/spec/server/verification-environment.md`
- `.trellis/spec/server/collaboration-host-execution.md`
- `.trellis/spec/server/collaboration-runtime-cli.md`
- `CLAUDE.md` 的迁移、发布与认证手册要求。

## Caveats / Not Found

- 没有执行构建、容器、测试或发布；未验证最终v0.8.8产物。上述脚本命令是后续建议，不能写作已通过。
- 完整运行时配置代码对比须由实现/主会话对最终SHA与基线Git对象执行；本角色禁止Git操作，因此只对实际包可见配置及当前源码做了检查。
- 当前主手册仍有候选、旧0.6.6包名与只列最大763前缀；最终应按SQL完整清单更新，`/health`字段是commit而非版本，版本应查二进制/认证changelog。
- Namespace/模型、匿名GitHub外部失败和Darwin整包HOLD不由main合并自动解除；不得继承为本次平台全部通过。按本次支持范围与实际门槛逐项处理。
- 本角色只能写研究目录；新版源手册/配置交付实施应交给 `trellis-implement` 角色，避免越权改代码。
