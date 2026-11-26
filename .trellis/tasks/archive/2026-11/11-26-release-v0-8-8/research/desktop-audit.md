# Research: Windows x64、下载服务与 GitHub 发布

- Query: 确定 V0.8.8 的真实 Windows 构建和验收路径、下载服务变化、发布工作流边界，审查发布计划。
- Scope: mixed（本地源码/交付记录，GitHub REST 只读查询）
- Date: 2026-11-26（Asia/Shanghai）
- 研究时父会话确认最终修复已合并到 main `c920c5e5b0354258ad0a37d500655a6856d5ed5b`。本报告不是冻结后的构建或测试证据。

## Findings

### 1. 可靠基线是 v0.6.7

`gh release list --repo yangshangwei/multica-0.4.37 --limit 30` 返回最新正式版 v0.6.7，2026-10-17T06:57:46Z 发布；没有更高已发布版本。`gh release view` 与 annotated tag API 两条证据一致：

- Release: https://github.com/yangshangwei/multica-0.4.37/releases/tag/v0.6.7
- 源码提交：`9caea35808afc8d05fb6135bfc74d4bfb0b45cf1`
- annotated tag object：`c6b739ca5c99425ffe93602ee3287ca38e7d8895`
- 最近实际本地交付目录：`dist/release/0.6.7/`（注意无前导 v）。存在 Windows EXE、blockmap、latest.yml、download.json、Linux 升级包和详细验证记录。
- 线上 v0.6.7 只有 `changelog.json`、`changelog.md`、`changelog-metadata.json` 三个资产，**没有 Windows 安装包或 Linux 离线包**。
- v0.8.8 远端 tag REST 查询返回 404，正式推送前仍须再次检查。

旧版 `windows-verification.json` 证明 macOS ARM64 交叉构建、EXE 内 payload 与未打包目录对比、ASAR 完整性、版本/架构/CLI build info 和内网 HTTP 验证；明确原生 Windows 安装、GUI、自动更新和用户状态保留均未测。旧证据不能沿用成 V0.8.8 通过。

### 2. 推荐 Windows 构建与原生验收路径

已存在可用的 GitHub Actions 工作流 `.github/workflows/desktop-smoke.yml`，REST 查询 state=active，workflow id `347574272`。不必先把 macOS 交叉构建限制认定为无法做 Windows 验收。

冻结并推送**不可再移动的候选分支/引用**后，由主会话授权范围内执行：

```bash
gh workflow run desktop-smoke.yml --repo yangshangwei/multica-0.4.37   --ref <指向冻结SHA的已推送引用>   -f windows_acceptance=true   -f windows_business_acceptance=true   -f windows_collaboration_acceptance=true   -f candidate_version=0.8.8
```

三个开关均为已有能力；collaboration 选项增加正式安装后的 Host 业务旅程并将 job 时限提高到75分钟。不要求实际付费 Provider，而用受限合成 Provider 与本次真实 backend/daemon。结果必须检查 run headSha、每个步骤及 JSON 报告，不凭 workflow dispatch 成功或 job 状态片面判定。

- `.github/workflows/desktop-smoke.yml:120` 只选择 Windows x64 job。
- `:124` 对 installer 与内置 CLI 使用相同 `MULTICA_DESKTOP_VERSION`。
- `:216` 真正调用 `package.mjs --win --x64 --publish never`。
- `:219` 业务验收模式使用已验证功能基线 `0.5.2-rc.11.1`，run `36996436086`，EXE SHA256 `f0b5a5eeded51fa7d1336bd2b46975a0c9ddc6d2292cebba457fd0b6e090fd47`。
- 2026-11-26 只读 API 验证该 run 的 `windows-x64-accepted-candidate`（asset id `11222590689`）及 `windows-x64-acceptance-evidence` 均未过期，到期时间2026-12-31T10:36:55Z。
- `:258` 原生安装旧版、升级、卸载、重装并验证身份/版本/架构；业务开关增加真实服务登录和状态保留。
- `:273` 原生安装客户端的自动更新探测，loopback feed。
- `:289` 即使失败也上传已净化的指定报告；`:310` 只在成功后上传 `windows-x64-accepted-candidate`，包含 EXE/blockmap/latest*.yml。下载这些精确字节后 collect，**不得另行重建再冒充同一验收产物**。

局限必须保留：该业务模式实际测试 0.5.2-rc.11.1→0.8.8，不是0.6.7→0.8.8；previous_tag 在业务模式无效。无业务模式的 `previous_tag=v0.6.7` 会因该Release缺EXE与sha256资产在准备阶段失败。签名默认禁用且没有签名 secret 自动注入；`require_signed=true` 不会凭空生成签名，因此不能声称 Authenticode/SmartScreen 或真实内网终端信任验收。

如果CI不可用，已有本地备用命令：

```bash
MULTICA_DESKTOP_VERSION=0.8.8 CSC_IDENTITY_AUTO_DISCOVERY=false   pnpm --filter @multica/desktop package -- --win --x64 --publish never
```

在 macOS 执行仅证明交叉打包，不能替代原生 Windows 安装；必须保留对应未测限制。当前本机 Node v22.23.2 / pnpm10.28.2 / Go1.27.0，而 CI Go 选择1.26.x最新补丁，应记录实际工具链。冻结干净 worktree构建；`package.mjs:375` 会清空该worktree的apps/desktop/dist，不可对已有交付目录直接运行。

### 3. Windows构件完整性与更新源验证

`apps/desktop/scripts/package.mjs:96` 验证显式SemVer；`:327` 传入 `extraMetadata.version`，Windows x64使用latest.yml。`bundle-cli.mjs:148` 把源码交叉构建为windows/amd64并写入resources/bin。Go缺失会降级跳过CLI构建（`:178`），所以正式交付必须主动验证内置CLI存在及其构建信息，不接受此降级。

`apps/desktop/electron-builder.yml:5` 用out/resources/package.json白名单；`:154` NSIS产物格式为`multica-desktop-${version}-windows-${arch}.exe`；publish仍指向upstream，因此**所有本次构建使用 --publish never，显式上传fork**。NSIS安装器loader自身通常是32位，不可用其PE机型错误判定成32位应用；真正Multica.exe和内置multica.exe应是0x8664 / GOARCH=amd64。

完整收集：

```bash
node apps/desktop/scripts/update-artifacts.mjs collect   --source <Windows原始产物目录>   --destination <本次Windows交付目录>
```

collector (`:96`、`:124`) 校验元数据每个引用、大小、SHA512，并由已验证Windows x64 latest.yml重新生成download.json，不信任输入JSON。要求保留EXE、EXE.blockmap、latest.yml和生成download.json；其他真实构建输出按用途归档，不把win-unpacked/debug文件当更新源必需文件。

隔离HTTP验证（不得使用或修改真实更新源）：

```bash
node apps/desktop/scripts/update-artifacts.mjs publish   --source <已收集产物> --destination <独立持久存储根>
# 使用 docker-compose.desktop-updates.yml、独立项目名、127.0.0.1监听、空闲端口，配置存储根
bash scripts/desktop-updates.sh --env-file <独立updates.env> start
node apps/desktop/scripts/verify-updates.mjs   --url http://127.0.0.1:<端口>/desktop   --metadata latest.yml --expected-version 0.8.8
```

`:54` HEAD200+Content-Length；`:60` Range206+Content-Range；`:69`完整下载字节与range一致；`:70`SHA512匹配；元数据须no-cache且无重定向。`:85` blockmap验证传输，但latest.yml不包含blockmap摘要时`checksumVerified:false`是正确输出，不得改成true。额外比较构建原始blockmap与交付副本的SHA256，并列入SHA256SUMS。

**verifier不读取download.json**。需另行GET并校验schemaVersion=1/platform=windows/arch=x64/version=0.8.8/file与latest.yml引用同一EXE/size一致，下载页HEAD门槛、业务地址及重启后持久化。保留管理员download-config.json字节；不要因publish成功声称下载页交互或安装验收通过。

旧版可复用只读验证实现：`.omx/qa/release-0.6.7/windows/verify-artifacts.mjs`（需实现代理复制到本任务工作范围并参数化SHA/版本/目录后使用），覆盖PE资源、ASAR每文件/块hash、包版本、Go buildinfo、NSIS提取payload比对、压缩blockmap结构。不要直接运行硬编码旧版脚本。

特别历史坑：`.omx/qa/release-0.6.7/windows/rejected-build.json`记录首次app.asar包含workspace依赖里的`.turbo`日志，被拒绝并用干净worktree重建。app顶层白名单不保证workspace依赖内部没有私有文件。避免在正式打包工作树先运行会生成turbo日志的测试；检查ASAR内 .env、.git/.omx/.trellis/.codex/.turbo、身份及初始管理员密码等禁止内容。

### 4. 下载中心相对v0.6.7的差异

通过GitHub contents API读取确定SHA基线，每个文件进行完整字节SHA256比较（不是比较时间或版本号）。10项里8项相同；download.html仅两处注释修改，**运行逻辑无变化**；手册变化。无需宣称Nginx或下载服务功能升级，可附同源页面/手册部署小包以确保交付完整。配置模板和真实配置必须区分，模板不得覆盖已有配置。

| 文件 | v0.6.7 SHA256 | 当前 SHA256 | 结论 |
|---|---|---|---|
| `docker-compose.desktop-updates.yml` | `3c81c295b1662aead49b37b434c2567b4730f40e18b456682081d90aa3823fd7` | `3c81c295b1662aead49b37b434c2567b4730f40e18b456682081d90aa3823fd7` | 相同 |
| `scripts/desktop-updates.sh` | `d4d9ed58d8c3e4c2c232a60ccb88a8561645f3c3f25ae58a309e3ca6c08020f7` | `d4d9ed58d8c3e4c2c232a60ccb88a8561645f3c3f25ae58a309e3ca6c08020f7` | 相同 |
| `deploy/desktop-updates/nginx.conf` | `608c25f6dde982582ca6d87234923776f0f82c1c1dafd87a4661f261d10d124b` | `608c25f6dde982582ca6d87234923776f0f82c1c1dafd87a4661f261d10d124b` | 相同 |
| `deploy/desktop-updates/download.html` | `13299297cbb73f405ed94e6f33ce75f2820372d6010d4ea455e10c9e8e32461b` | `6980828eee56b23f0e04804f4087ecc6f9d753c5d9fe76808bd08ab4d4a2a495` | 不同（见正文） |
| `deploy/desktop-updates/download-config.example.json` | `e94d00068eb86fb666a95f2b17fe1a8232430bd9ac9f214d66ad2d967cdfa001` | `e94d00068eb86fb666a95f2b17fe1a8232430bd9ac9f214d66ad2d967cdfa001` | 相同 |
| `deploy/desktop-updates/updates.env.example` | `6d05adaf73effce225f443ce546e470b48b70962e7cad36e7b099c35a063a910` | `6d05adaf73effce225f443ce546e470b48b70962e7cad36e7b099c35a063a910` | 相同 |
| `apps/desktop/scripts/update-artifacts.mjs` | `3829bfeafe125838eae6cf5aa80590a8b46196cc2d9b3f025e1679e1ab45c61f` | `3829bfeafe125838eae6cf5aa80590a8b46196cc2d9b3f025e1679e1ab45c61f` | 相同 |
| `apps/desktop/scripts/verify-updates.mjs` | `f1ab7c5ab5c44b2aa86a9aa86a32bc8cb8b57527b525a1082bb34bb291787483` | `f1ab7c5ab5c44b2aa86a9aa86a32bc8cb8b57527b525a1082bb34bb291787483` | 相同 |
| `apps/desktop/scripts/configure-updates.mjs` | `1123fb0a291f102f4d34aed5091072f87f24fd91abdb96c5ce14ae64e9bf078c` | `1123fb0a291f102f4d34aed5091072f87f24fd91abdb96c5ce14ae64e9bf078c` | 相同 |
| `docs/desktop-intranet-update-runbook.zh-CN.md` | `79cf450f16c9c297da618b99191884e1eeb2e3a1244ecdf00ad8d54ac6c638e3` | `1097a62e8449b802d70de45791aa6d6658115c06f63e39b68242919fb898adff` | 不同（见正文） |

最终候选若修改这些路径，应重新对比并更新结论。部署包建议携带当前`docker-compose.desktop-updates.yml`、`scripts/desktop-updates.sh`、`deploy/desktop-updates/{nginx.conf,download.html,download-config.example.json,updates.env.example}`及本次手册。已有站点更新页面即可；start原子替换download.html，并且仅在download-config.json不存在时初始化（`desktop-updates.sh:137`）。不拷贝真实updates.env，不传递本机业务地址。

完全离线首次部署若需Nginx镜像，可用既有命令：

```bash
bash scripts/desktop-updates.sh export-image <新路径>/desktop-nginx-linux-amd64.tar linux/amd64
```

脚本`:83`提取Compose中固定digest，拉取并核验平台，以架构专有tag保存；默认镜像digest `sha256:985220252f3863977e468f611ef118ebd01421289dd86ee1ae99cb068c3bce2b`。已有v0.6.7同digest实例无需仅为注释重新换镜像。Node helpers需要仓库依赖，只有Docker/Nginx的内网服务器使用手册4.3中的顺序文件发布，不要求现场安装pnpm。

### 5. Fork GitHub Release真实边界与计划修正

`.github/workflows/release.yml:8`只由tag push触发；contents/packages写权限；同repo串行release并发组。`:21`verify做数据库迁移、Go race与govulncheck。`:163`准备唯一changelog artifact。`:194`GoReleaser仅multica-ai owner执行；`:559`desktop依赖该job，fork上同样跳过。因此tag流水线**不会替fork生成Windows或CLI附件**。

fork可执行amd64/arm64 backend+web GHCR构建和`:472`publish-changelog；后者在verify与两种镜像manifest成功后创建draft、上传三个changelog文件、转正式发布。不能等同于已完成本用户完整附件交付。需主会话补充EXE/blockmap/latest.yml/download.json/Linux离线升级包/文档/校验单/CLI档案，并对线上摘要核对。

`.goreleaser.yml:5`CLI构建windows/linux/darwin；`:35`同时提供legacy `multica_<os>_<arch>`与versioned `multica-cli-0.8.8-<os>-<arch>`档案，Windows zip、其他tar.gz，带LICENSE/NOTICE；checksums.txt供自更新校验。fork不能盲目用goreleaser默认publish，因为brews写upstream tap。可本地构建后显式fork上传，至少交付需求中的linux/amd64和windows/amd64，保留兼容名称与checksum。

现有独立CLI `server/internal/cli/update.go:229,255`和desktop缺CLI时bootstrap `apps/desktop/src/main/cli-bootstrap.ts:20`仍硬编码upstream。此次内网升级应使用所交付同源CLI手工受控替换并重启daemon，Desktop正常优先使用bundled CLI（daemon-manager.ts:686）；不可宣称fork上传以后`multica update`会自动读取fork。待在升级手册明确关闭/避免upstream自动升级路径。

**计划的实质顺序问题**：`scripts/release-changelog.mjs:132` prepare要求远端tag已存在且与本地相同；`:139`首次生成使用当前时间，两个独立prepare不是相同字节。因此“标签推送前制作最终含正式changelog离线包，之后CI再生成”与现有合同冲突。推荐顺序：

1. 冻结SHA并完成候选源码/升级/Windows门槛；允许候选用明确未发布feed预验，不冒称正式交付。
2. 推送不可变tag，监控verify/changelog。取**CI唯一release-changelog工件**与该SHA的GHCR镜像，把正式离线包绑定同一份feed，再完成最终构件验收。
3. 验证CI publish结果并显式上传完整附件；保持已发布元数据不可变、冲突失败关闭。

注意现有CI会在其自身门槛通过后立即公开changelog Release，早于本地离线包最终组装。若要求所有离线验收结束前绝不公开Release，需要实施明确的发布门控改动并测试；不要临时停用workflow、改远端tag或伪造success参数。该取舍由主会话结合用户授权决定并记录。

旧版本地 `dist/release/0.6.7/changelog.json` 是73383字节未发布候选，线上正式feed73391字节、SHA256 `f9611133a1bbe8584eec1c716cbc7f0b23f7e93d73d7284bb781851314f342fd`。必须使用线上正式累计历史作为准备基线，不能直接拿旧本地preview冒充published历史。

## Plan review

研究建议：在执行前把上述tag/changelog顺序、独立CLI交付和Windows原生CI路径写入implement.md；下载服务结论写“功能无变化，仅注释/文档变化”并附逐文件摘要。其余计划的隔离存储、保留身份/配置、服务端先行、无凭据、同SHA和最终线上摘要门槛符合需求。未执行测试或构建，本报告不是批准最终发布的验证证据。

## Files Found / Related Specs

- `CLAUDE.md`：架构、构件交付前认证/迁移手册审阅要求。
- `.trellis/spec/desktop/frontend/intranet-updates.md`：元数据、存储、来源、升级与真实安装验收合同。
- `.trellis/spec/guides/changelog-release-flow.md`：单次生成、不可变来源、累计历史、发布事务边界。
- `.github/RELEASING.md`：fork标记、CI边界、Windows候选与签名限制。
- `docs/desktop-intranet-update-runbook.zh-CN.md`：内网发布顺序、HTTP检查、配置保留；当前仍有历史示例版本。
- `apps/desktop/scripts/WINDOWS_ACCEPTANCE.md`：原生Windows隔离和业务/自动更新/Host证据边界。
- `dist/release/0.6.7/{windows-verification.json,verification-v0.6.7.json}`：最近交付证明与限制。
- `.omx/qa/release-0.6.7/windows/verify-artifacts.mjs`：历史构件完整性审计参考；硬编码旧版，不可直接当当前验证。

## External References / Versions

- GitHub Release/API for this fork (above); workflow active status and baseline artifact expiry checked live with authenticated `gh` read-only commands.
- Electron package source pins: `apps/desktop/package.json` Electron ^39.2.6, electron-builder ^26.0.12, electron-updater ^6.8.3; lockfile及实际最终构建安装版本应入证据。
- No generic web guidance needed; implementation contracts and primary repository API were inspected directly.

## Caveats / Not Found

- No build, test, workflow dispatch, server start, installer execution, tag change, git command or external write was performed by this researcher.
- Source snapshots/hash comparison must be repeated on frozen candidate if relevant files change.
- Native signed trust remains unestablished; available Windows CI provides unsigned native installation/business/update capability, not customer's intranet machine validation.
- Existing raw QA directories may contain private fixture state; inspected only named verification/config-free files. Never package/upload entire historical QA directory.
- First combined command referenced wrong `.goreleaser.yaml`; corrected discovery found `.goreleaser.yml`. A zsh glob lookup failed on absent extension and was replaced with `rg --files`; no build was attempted.
