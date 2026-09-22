# Design — 发布 v0.4.47

## 版本戳来源（关键约束）

所有产物版本戳来自 `git describe --tags --match 'v[0-9]*'`，只走 HEAD 祖先链：
- `Makefile VERSION` → server + CLI 的 `-X main.version`。
- `apps/desktop/scripts/bundle-cli.mjs` → 打进 Desktop 的 CLI（无 env override）。
- `apps/desktop/scripts/package.mjs deriveVersion()` → electron `extraMetadata.version`，可被 `MULTICA_DESKTOP_VERSION` 覆盖。
- `scripts/build-offline-upgrade.sh` `VERSION=` 默认取 `git describe`。

结论：**必须先在 HEAD `3e07a860c` 打注解 tag `v0.4.47`，且构建时工作树干净**，`git describe` 才会输出 `v0.4.47`，无需任何 `VERSION=` / `MULTICA_DESKTOP_VERSION=` 覆盖（见记忆 build-version-comes-from-git-describe / intranet-upgrade-packaging）。工作树脏会得到 `-dirty` 戳，`release.yml` 会拒收含 `-dirty` 的 tag。

## 交付目录（沿用 v0.4.46 约定）

```
dist/release/v0.4.47/
  linux-amd64/
    multica-server-upgrade-v0.4.47-linux-amd64/            # 解包目录
    multica-server-upgrade-v0.4.47-linux-amd64.tar.gz
    multica-server-upgrade-v0.4.47-linux-amd64.tar.gz.sha256
  windows-ia32/
    multica-desktop-0.4.47-windows-ia32.exe
    multica-desktop-0.4.47-windows-ia32.exe.blockmap
    multica-desktop-0.4.47-windows-ia32.exe.sha256
    latest-ia32.yml
  verification/
    linux-amd64            # 版本/架构核验记录
    windows-ia32
  linux-upgrade-verification.json
  SHA256SUMS-v0.4.47.txt
  README-v0.4.47.zh-CN.md
```

## 构建单元

### A. Tag 与推送
- `git tag -a v0.4.47 -m "..." 3e07a860c`（注解 tag）。
- `git push origin main`（已同步，确认性）。
- `git push origin v0.4.47` → 触发 `release.yml`。fork tag 发布链：verify（Go test + govulncheck 失败即停 + changelog fixture）→ changelog 生成 → 后端/web 镜像清单 → publish-changelog（在本 repo 建 Release）。不跑上游专属 Desktop/GoReleaser/Homebrew。

### B. Linux x86-64 服务端升级包
- `scripts/build-offline-upgrade.sh --output dist/release/v0.4.47/linux-amd64`（默认 platform=linux/amd64）。
- 产物：`multica-server-upgrade-v0.4.47-linux-amd64.tar.gz`（3 个运行镜像 + compose + 运维升级脚本 + docs）。
- arm64 Mac 上镜像构建走 QEMU 模拟，慢；后台跑。校验 tar 内嵌 CLI `version --output json` 报 `v0.4.47` / `arch: amd64`。

### C. Windows ia32 桌面安装包
- `CSC_IDENTITY_AUTO_DISCOVERY=false pnpm --filter @multica/desktop package -- --win --ia32 --publish never`。
- 干净 tag 后无需 `MULTICA_DESKTOP_VERSION`；`git describe` 已是 v0.4.47。
- 产物在 `apps/desktop/dist/`：`multica-desktop-0.4.47-windows-ia32.exe`(+`.blockmap`)、`latest-ia32.yml`。拷入 `dist/release/v0.4.47/windows-ia32/`。
- 关键校验：`Multica.exe` 与内置 `multica.exe` PE machine 必须为 `0x14c`（32 位），CLI `arch: 386`。NSIS loader 本身不代表应用架构。
- 注意：再次运行 `package.mjs` 会清空 `apps/desktop/dist`，出包后先拷走再做别的。

### D. 校验与清单
- 每个交付文件生成 `.sha256`；汇总 `SHA256SUMS-v0.4.47.txt`。
- `README-v0.4.47.zh-CN.md`：提交哈希、构建日期、各 sha256、desktop.json 内网 `updateUrl` 配置指引（参考 v0.4.46 README）。
- `verification/`：记录 linux/windows 版本+架构核验输出。

## 完整 E2E

- 运行器：Playwright（`e2e/*.spec.ts`）。需要跑起 dev 环境（`make up` 分配端口/DB）。
- 已知陷阱（记忆）：
  - 脏共享 DB 会让 auth/global-scope spec 因残留 workspace 失败 → 先确认/清理隔离 DB。
  - 全套件下 mcp-tab / daemon-codex 等有 5s/1s 预算 flake → 单独复跑该文件再判定。
  - stale import 会让裸跑在模块加载期整体挂掉 → 必要时按基名循环。
  - `... | tail` 会吞退出码 → redirect 到文件后 `echo $?`。
- 目标：如实产出通过/失败清单，不美化。E2E 属本地质量门，与 CI 的 Go verify 相互独立。

## 顺序与风险

执行顺序（用户确认「先测后推」）：完整 E2E → 推送 main+tag → 打包。
- E2E 是推送前的**硬门禁**：仅当无发布级回归（剩余失败为已知 flake 且复跑通过）才推 tag，避免把未验证代码发到远端触发 CI 发布。
- 打包放在推送之后、基于已打 tag 的干净树，保证版本戳为 `v0.4.47` 且交付物与远端 tag 对应同一提交。
- 若 E2E 暴露发布级回归：停在门禁前，不推 tag，回到 Plan 决策（无需回滚远端）。

## 回滚点

- Tag 未推前：`git tag -d v0.4.47` 即可。
- Tag 已推：`git push origin :refs/tags/v0.4.47` 删远端 tag + 在 GitHub 删除对应 draft/Release。
- 打包产物：删除 `dist/release/v0.4.47/` 重来；镜像层已缓存，重试只重跑编译。
