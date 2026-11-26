# main 合并后验收失败定位与修复

用户于 2026-11-25 授权创建、拆分、内部审查后实施及回归；禁止提交、推送、部署和发布。
基线 a7bf6ba70fcc3af8171d6a406e0fae6a1b0c29e1；原被测候选 103e2bc329a9fb845d9f45c20b2a9bb3ebb54625。

## Goal
按根因关闭 main 验收失败，交付真实可复核的新候选验证与剩余风险。

## Requirements
- User explicitly approved restoring a separate return-to-previous-collaboration control on 2026-11-25; preserve the Collaboration disclosure and original manual-detail selection/scroll/tab restoration assertions on Web/Desktop. This extends client ownership to minimal shared sidebar and platform wiring, with new builds and complete original journeys.
- 保留默认 400 PASS/138 SKIP、补测117 PASS/21 FAIL以及历史114 PASS/24 FAIL及全部旧证据。
- 评论凭据 principal 权限正确，复用11-20-comment-admin-human-authority，generic upload 单独定约。
- 识别 NOWAIT 实际竞争者，整包 race 回归；不放宽守卫。
- 对支持工具链/依赖和发布平台重新安全扫描。
- Host permit、Namespace probe 先捕获区分假设证据，禁止仅扩大资源或时限。
- 修复确已过时的 fixture 后完整执行 Web/Desktop/真实打包Main/旧CLI旅程。
- 所有执行使用任务专属 DB、账号、端口、HOME/profile与新的source/tool/build/resource绑定。
- 不用真实付费模型、个人 Agent 账号、生产数据；不降低宿主机或Docker安全策略。

## Acceptance
- [ ] 每组有原始失败→假设→区分验证→最小修复→回归证据。
- [ ] 变更对应lint/typecheck/unit/integration/race与完整相关E2E有新退出码。
- [ ] 独立复核修复、未闭合问题、平台阻塞和自有资源清理。
- [ ] 明确发布建议；不把子步骤通过或环境阻塞算整例PASS。
