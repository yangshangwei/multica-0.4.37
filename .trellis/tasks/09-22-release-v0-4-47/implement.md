# Implement — 发布 v0.4.47

执行顺序（用户确认「先测后推」）：E2E → 推送 → 打包。每阶段末尾为验证/检查点。

## 0. 预检
- [ ] `git status` 干净、`git rev-parse HEAD` == `3e07a860c`、当前分支 `main`。
- [ ] `git describe --tags --match 'v[0-9]*'` == `v0.4.46-29-g3e07a860c`（确认 HEAD 未打标签）。
- [ ] `git rev-list --left-right --count origin/main...main` == `0	0`。
- [ ] `git tag -l v0.4.47` 为空（未占用）。

## 1. 完整 E2E（发布门禁）
- [ ] 起环境：`make up C=api,web` 或 `make dev`（后台）；`make status` 确认 pid/commit 属本 checkout。
- [ ] 确认 DB 无脏残留（必要时隔离/新 DB，避免全局-scope spec 因残留 workspace 失败）。
- [ ] 运行：`pnpm exec playwright test > /tmp/v0447-e2e.log 2>&1; echo "EXIT=$?"`（redirect 保留退出码，勿管道 tail）。
- [ ] 失败项：单独复跑该 spec 文件判定是否 flake（mcp-tab / daemon-codex 等已知 flake）。
- [ ] 记录通过/失败清单与最终退出码到 task 笔记。
- **检查点（硬门禁）**：仅当无发布级回归（剩余失败均为已知 flake 且复跑通过）才进入第 2 步推送。否则停下与用户确认。

## 2. 打 tag 并推送（E2E 通过后）
- [ ] `git tag -a v0.4.47 -m "Release v0.4.47" 3e07a860c`
- [ ] 验证：`git describe --tags` 输出 `v0.4.47`（HEAD 干净）。
- [ ] `git push origin main`（预期 up-to-date）。
- [ ] `git push origin v0.4.47`（触发 release.yml）。
- [ ] 记录：`gh run list --workflow release.yml -L 3` / run URL；`gh release view v0.4.47`（可能先是 draft）。

## 3. Linux x86-64 服务端升级包
- [ ] 后台构建：`bash scripts/build-offline-upgrade.sh --output dist/release/v0.4.47/linux-amd64`（linux/amd64 默认，QEMU 模拟慢）。
- [ ] 验证 tar 内嵌 CLI 版本/架构（解包后 `... version --output json` 应报 `v0.4.47` / `arch: amd64`）。
- [ ] 生成 `.sha256`；写 `verification/linux-amd64` 与 `linux-upgrade-verification.json`。

## 4. Windows ia32 桌面安装包
- [ ] `CSC_IDENTITY_AUTO_DISCOVERY=false pnpm --filter @multica/desktop package -- --win --ia32 --publish never`
- [ ] 从 `apps/desktop/dist/` 拷 `multica-desktop-0.4.47-windows-ia32.exe`(+`.blockmap`) 与 `latest-ia32.yml` 到 `dist/release/v0.4.47/windows-ia32/`（再次 package 前拷走，package 会清空 dist）。
- [ ] 架构核验：`Multica.exe` 与内置 `multica.exe` PE machine == `0x14c`；CLI `arch: 386`。写入 `verification/windows-ia32`。
- [ ] 生成 `.sha256`。

## 5. 清单与文档
- [ ] 汇总 `SHA256SUMS-v0.4.47.txt`（覆盖全部交付文件）。
- [ ] 写 `README-v0.4.47.zh-CN.md`（参考 v0.4.46）：提交哈希、构建日期、sha256、desktop.json `updateUrl` 内网配置。

## 6. 最终核对（对齐 prd 验收）
- [ ] tag/推送/CI run 均已记录。
- [ ] E2E 结果如实记录。
- [ ] linux-amd64 tar 版本正确、sha256 通过。
- [ ] windows-ia32 为 32 位（PE `0x14c` / `arch: 386`）、sha256 通过。
- [ ] 目录结构对齐 v0.4.46 约定。
- [ ] 核 CI：`gh run view <id>` 结果；Release 是否 publish（fork tag 只发 changelog Release）。

## 验证命令速查
```bash
git describe --tags --match 'v[0-9]*'
pnpm exec playwright test > /tmp/v0447-e2e.log 2>&1; echo "EXIT=$?"
bash scripts/build-offline-upgrade.sh --output dist/release/v0.4.47/linux-amd64
CSC_IDENTITY_AUTO_DISCOVERY=false pnpm --filter @multica/desktop package -- --win --ia32 --publish never
gh run list --workflow release.yml -L 3
```

## 回滚
- tag 未推：`git tag -d v0.4.47`。
- tag 已推：`git push origin :refs/tags/v0.4.47` + 删 GitHub draft/Release。
- 产物：删 `dist/release/v0.4.47/` 重建（镜像层缓存，重试只重编译）。
