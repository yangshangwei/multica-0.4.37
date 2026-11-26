# Windows checkout 长路径修复

## 根因与边界

原生 Windows run 45659248507 在 Checkout candidate source 阶段退出128，日志显示 .trellis 历史证据路径 Filename too long，尚未进入编译或安装。主 CI 三个 Windows 路径已在 checkout 前运行 git config --global core.longpaths true，desktop-smoke 两个入口未配置；release.yml 的桌面 Windows matrix 入口也遗漏。

最小修改：在 desktop-smoke 的 desktop matrix 和 windows-acceptance，以及 release.yml 的 desktop matrix 中，按主 CI 模式加入 Windows-only 前置步骤。global 仅作用于临时 GitHub runner，不执行本机全局配置，不删历史证据、不改 checkout 深度或产品逻辑。扩展现有唯一长路径契约测试覆盖上述入口，验证配置先于checkout且非Windows不执行。

## 修改与回归

- `.github/workflows/desktop-smoke.yml`：desktop matrix 与 windows-acceptance 均在 checkout 前启用长路径，条件为 runner.os == 'Windows'。
- `.github/workflows/release.yml`：desktop matrix 采用相同前置步骤，不改 fork 发布依赖图或上游限定。
- `scripts/agent-critical-path-workflow.test.mjs`：扩展现有长路径顺序检查，统一覆盖主 CI 与两个桌面工作流的6个Windows入口，没有另建重复测试。

验证：

1. 先扩展测试、未修workflow时，`node --test --test-name-pattern='Windows CI enables long paths' scripts/agent-critical-path-workflow.test.mjs` 确实失败：desktop-smoke.yml: desktop，1 FAIL。
2. 修复后运行 `node --test scripts/agent-critical-path-workflow.test.mjs scripts/changelog-workflow.test.mjs scripts/windows-candidate-version.test.mjs apps/desktop/scripts/windows-collaboration-workflow.node-test.mjs`：17 PASS / 0 FAIL / 0 SKIP，含实际YAML解析和顺序检查。
3. 在apps/desktop运行 `pnpm exec vitest run scripts/windows-installer-smoke.test.mjs --maxWorkers=1`：11 PASS。
4. `git diff --check`：通过。

现有pnpm发出package.json中旧pnpm字段被忽略的警告，本次不改包管理配置；未执行安装或重型构建。测试只证明workflow结构与现有门槛，实际Windows checkout恢复和后续安装流程需要父会话在新冻结SHA重新dispatch验证。未提交、推送，未修改本机Git全局设置。
