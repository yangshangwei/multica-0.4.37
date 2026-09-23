# 发布 v0.4.47：推送 main、E2E、内网离线升级包

## Goal

在当前 `main`（HEAD `3e07a860c`）上发布 fork 版本 **v0.4.47**：把代码推到远端、跑完整端到端测试，并构建内网离线交付物（Linux x86-64 服务端升级包 + Windows x86/ia32 桌面安装包），产物落到 `dist/release/v0.4.47/`。

## Context / 已确认决策

- 当前分支 `main`，HEAD `3e07a860c`，与 `origin/main` 完全同步（0/0）。`git describe` = `v0.4.46-29-g3e07a860c`，即 HEAD 未打标签。
- 版本号不写在任何文件里，全部来自 `git describe --tags --match 'v[0-9]*'`（见记忆 build-version-comes-from-git-describe）。因此要让产物稳定地打上 `v0.4.47`，必须先在 HEAD 上建注解 tag `v0.4.47`。
- **推送**：直接推 `main`（用户确认）。因 main 已同步，实际是空推/确认；真正新增的是 tag。
- **v0.4.47 tag**：推到 `origin`（用户确认），会触发 `release.yml`（fork tag 发布：verify + govulncheck + changelog + 后端/web 镜像清单 + publish-changelog；不含上游专属的 Desktop/GoReleaser/Homebrew 任务）。
- **打包路径**：内网离线（用户确认），本地构建，落到 `dist/release/v0.4.47/`，沿用 v0.4.46 目录约定。
- **Windows 架构**：ia32 / 386（32 位，用户确认），即 `windows-ia32.exe`（PE `0x14c`，CLI `arch: 386`）。不构建 x64。
- **Linux 架构**：linux/amd64（x86-64）。服务端不支持 linux ia32（RELEASING.md 明确）。

## Requirements

1. 在 HEAD `3e07a860c` 建注解 tag `v0.4.47`（fork tag，须在 main 祖先链上、编号高于既有 v0.4.46）。
2. 推送 `main` 与 tag `v0.4.47` 到 `origin`（触发 release.yml）。
3. 运行完整 Playwright E2E 套件，如实报告结果；已知脏 DB / 全套件 flake 问题需按记忆规避（单基名循环、清理/隔离 DB、redirect 后读 `$?`）。
4. 构建 Linux x86-64 服务端升级包：`scripts/build-offline-upgrade.sh`（platform=linux/amd64），产出 `multica-server-upgrade-v0.4.47-linux-amd64.tar.gz` + `.sha256`。
5. 构建 Windows ia32 桌面安装包：electron-builder `--win --ia32 --publish never`，产出 `multica-desktop-0.4.47-windows-ia32.exe` + `.blockmap` + `.sha256` + `latest-ia32.yml`。
6. 按 v0.4.46 约定组织 `dist/release/v0.4.47/`：`linux-amd64/`、`windows-ia32/`、`verification/`、`SHA256SUMS-v0.4.47.txt`、`README-v0.4.47.zh-CN.md`。

## Acceptance Criteria

- [ ] `git tag -l v0.4.47` 存在且为注解 tag，指向 `3e07a860c`；`git describe --tags` 在 HEAD 干净时输出 `v0.4.47`。
- [ ] `origin` 上存在 `main` 与 tag `v0.4.47`；`release.yml` 运行已触发（记录 run URL 与结果）。
- [ ] 完整 E2E 已运行，结果如实记录（通过/失败清单 + 退出码，不被管道吞掉）。
- [ ] `dist/release/v0.4.47/linux-amd64/` 内 tar.gz 内嵌 CLI 版本为 `v0.4.47`（非 `v0.4.46-N-g…`），sha256 校验通过。
- [ ] `dist/release/v0.4.47/windows-ia32/` 内安装包为 32 位：`Multica.exe` 与内置 CLI PE machine 均为 `0x14c`，CLI `arch: 386`；sha256 校验通过。
- [ ] `SHA256SUMS-v0.4.47.txt` 覆盖全部交付文件；README 含提交、日期、sha256、desktop.json 内网配置。

## Non-Goals

- 不构建 Windows x64、macOS、Linux 桌面 AppImage、arm64 服务端。
- 不修改产品代码（纯发布/打包；如 E2E 暴露需改代码的回归，回到 Plan 决策）。
- 不部署到任何真实内网服务器（只产出可交付文件）。

## Notes

- arm64 Mac 主机：linux/amd64 镜像构建走模拟，较慢；失败重试会复用已缓存的 `pnpm install` 层。
- 执行顺序（用户确认调整为「先测后推」）：完整 E2E → 推送 main+tag（触发 CI）→ 打包。E2E 作为推送前的硬门禁，避免把未验证代码发到远端。
