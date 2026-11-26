# V0.8.8 Windows、下载服务与 CLI 验收准备

状态：准备完成，尚未执行 V0.8.8 构建、workflow dispatch、安装器或下载服务验收。须由主会话提供最终冻结 SHA 和 CI run 后执行。本文不是通过证据。

## 边界与复用

- 产品源码零改动。验收工具只写 `/Volumes/artisan/code/2026/multica-0.4.37/.artifacts/release-v0.8.8/desktop-acceptance/`，正式公开目录由主会话集成。
- 用原生 `desktop-smoke.yml`、真实 `update-artifacts.mjs` 和 `verify-updates.mjs`。补充脚本只做证据绑定、下载 JSON 对照、持久配置/历史产物保留和隔离生命周期，不重写产品发布器。
- 现有最近正式基线是 v0.6.7 / `9caea35808afc8d05fb6135bfc74d4bfb0b45cf1`，实际产物为 `dist/release/0.6.7/`。线上该 Release 只有 changelog，不能假设有可下载的旧安装器。
- 主会话负责标签、GitHub Release 和 CLI 构建；本子任务不上传资产、不修改更新源、不运行个人 CLI/模型、不触碰其他 Docker 项目。

## Windows 原生候选

冻结、提交、推送候选引用后，由主会话执行已有流程：

```bash
gh workflow run desktop-smoke.yml --repo yangshangwei/multica-0.4.37 \
  --ref codex/release-v0-8-8 \
  -f windows_acceptance=true -f windows_business_acceptance=true \
  -f windows_collaboration_acceptance=true -f candidate_version=0.8.8
```

确认 run 的 `headSha` 等于冻结 SHA，再跟进至结束。业务验收使用固定哈希的 `0.5.2-rc.11.1`，不能把它报告成 v0.6.7→0.8.8。签名默认关闭；原生安装、登录、保留身份、自动更新、升级/重装后 Host 旅程如通过可逐项报告，但不延伸到 SmartScreen、离线证书、客户内网或付费模型。

将 run JSON、两个原始 artifact 下载到一个新的 run 专属目录；不得使用 `--clobber` 或重建安装包替代已验收字节：

```bash
gh run view RUN_ID --repo yangshangwei/multica-0.4.37 \
  --json databaseId,workflowName,headSha,status,conclusion,jobs,url > run.json
gh run download RUN_ID --repo yangshangwei/multica-0.4.37 \
  --name windows-x64-accepted-candidate --dir candidate
gh run download RUN_ID --repo yangshangwei/multica-0.4.37 \
  --name windows-x64-acceptance-evidence --dir evidence
```

准备好的 `verify-native-evidence.py` 要求 workflow/head SHA、安装生命周期/业务/自动更新/两次协作报告通过，核对交付 EXE SHA256 与原生安装报告相同、bundled CLI 版本/架构/commit、自动更新保留状态及截图摘要。运行参数：

```bash
python3 verify-native-evidence.py --run-json RUN_DIR/run.json \
  --evidence RUN_DIR/evidence --candidate RUN_DIR/candidate \
  --source-sha FULL_SHA --version 0.8.8 --output RUN_DIR/native-binding.json
```

该脚本无法代替 ASAR 内部检查。还需提取该 EXE 的真实 payload：核对 `Multica.exe` 和 `resources/app.asar.unpacked/resources/bin/multica.exe` 是 PE x64 (`0x8664`)，不能按 NSIS loader 的32位架构判断应用；`go version -m` 须含 Windows/amd64、0.8.8、完整源码 SHA、`vcs.modified=false`。遍历 ASAR integrity/blocks、package.json 版本与启动入口，排除 `.env*`、`.turbo`、身份文件和私有工具状态；许可证须匹配源码。参考 `.omx/qa/release-0.6.7/windows/verify-artifacts.mjs`，但不要直接运行其旧版硬编码或假设 CI 上传 win-unpacked（当前 CI 只上传更新构件）。Native报告里的 ASAR/CLI SHA 可用于绑定从最终EXE提取的payload。

## 本地下载服务演练

用已加载且确认来自 Compose 固定 digest 的 Linux amd64 Nginx。既有镜像无需升级；若本机缺镜像，由主会话按原 `export-image` 准备，镜像来源：

`nginx:stable-alpine@sha256:985220252f3863977e468f611ef118ebd01421289dd86ee1ae99cb068c3bce2b`。

准备好的 `accept-download-service.py`：

1. 要求工作区 HEAD 与最终 SHA 相同、相关源文件无修改；新建独立随机 Compose project、loopback固定空闲端口及任务目录。
2. 用真实 collector 校验 v0.6.7 与 0.8.8，再分别发布到持久存储。先写入带未知字段的管理员测试配置，验证 publish/start 不覆盖它。
3. 执行原 `verify-updates.mjs`：latest.yml、HEAD200/Range206、完整SHA512和大小。
4. 额外 GET download.json 对照 collector 生成的版本/文件/大小；latest.yml与collector原字节相同；完整下载blockmap对照构建副本SHA256。保留原 verifier 的 `checksumVerified:false`（其元数据没有blockmap摘要），另记源字节校验结果。
5. 校验旧安装器/blockmap摘要、业务配置、静态页面，实际重启并再次验证；finally只down本任务独立项目，不删存储或日志。

```bash
python3 accept-download-service.py \
  --repo /Users/artisan/.codex/worktrees/release-v0-8-8/multica-0.4.37 \
  --source-sha FULL_SHA --candidate RUN_DIR/candidate \
  --baseline /Volumes/artisan/code/2026/multica-0.4.37/dist/release/0.6.7 \
  --image 'nginx:stable-alpine@sha256:985220252f3863977e468f611ef118ebd01421289dd86ee1ae99cb068c3bce2b'
```

结果在新的 `http-*/verification.json`，失败也保存。不允许把脚本存在/语法检查报告为实际HTTP通过。另用真实页面确认下载按钮、业务地址、复制失败反馈和FAQ键盘操作；已有 `apps/desktop/scripts/download-page.test.mjs` 是边界回归，不能代替实际页面验收。

## 下载服务升级结论和材料

基线10文件完整SHA256见 `desktop-audit.md`。至审计时：Compose、Nginx配置、生命周期、发布/校验/客户端配置工具、环境模板均字节相同；HTML仅两处注释改变，手册变化，运行功能无升级。冻结后重核，禁止将注释差异包装成Nginx功能升级。

如提供同源部署材料包，最小内容：

- `docker-compose.desktop-updates.yml`
- `scripts/desktop-updates.sh`
- `deploy/desktop-updates/{nginx.conf,download.html,download-config.example.json,updates.env.example}`
- 本次 `docs/desktop-intranet-update-runbook.zh-CN.md`、`docs/offline-upgrade.zh-CN.md`及发布专属README/摘要清单。

已有站点仅需同步页面/手册。真实 `updates.env`、`public/download-config.json`、历史安装器和元数据备份均保留，例子只作参考。最小包的start/stop不依赖Node；collect/publish/verify需要完整仓库及依赖，纯Docker的内网站点按手册4.3先安装器/再YAML/最后download.json操作。若提供首次离线部署包，则另附经核验linux/amd64 Nginx镜像，不能含有脚本却缺所声称的镜像。

## CLI / daemon 同源补充

- Linux业务镜像 `/app/multica` 和Windows安装器 bundled CLI都须在构件内核验。独立运行机还需 `multica-cli-0.8.8-linux-amd64.tar.gz` 与 `multica-cli-0.8.8-windows-amd64.zip`，含许可证/NOTICE及摘要；按 `.goreleaser.yml` 可附兼容文件名副本，但不可向upstream tap发布。
- 优先从本次同源构件提取已验证二进制作为受控CLI物料，避免额外不同工具链构建。版本/完整commit/OS/arch、摘要和来源须写进记录。Linux容器内执行`multica version --output json`；Windows原生run已执行bundled CLI的版本命令。
- `multica update`、缺失CLI的公网bootstrap仍指向`multica-ai/multica`。不得指导内网用户靠该命令获取fork 0.8.8；服务端/Web先完成迁移、healthz和登录，再安排daemon停机受控替换匹配CLI、保留原身份和配置，然后才开放桌面更新源。

## 准备阶段验证

仅检查两个Python工具的语法和help入口；未联网下载候选、未启动容器、未运行Windows安装器。后续所有真实通过结论以对应run和本地验收JSON为准。
