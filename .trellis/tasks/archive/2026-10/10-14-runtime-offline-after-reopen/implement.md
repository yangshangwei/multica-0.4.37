# 执行计划

状态：2026-10-15 按用户要求归档。以下保留原规划，Windows 现场验收未完成；未执行 `task.py start`，未修改产品代码。

## 已完成的准备

- [x] 对齐用户现场 v0.6.5 与本地 tag，记录脚本的组合恢复动作。
- [x] 对比凭据、心跳和在线投影，区分已确认代码缺口与现场假设。
- [x] 确认 `296c377a4` 包含于 v0.6.6，相关 Desktop Main 实现与当前 HEAD 一致。
- [x] 跑通现有 Desktop 认证 44 项、daemon 续期/心跳 12 项回归；服务端研究补充 5 项独立测试。
- [x] 编写 PRD、设计、研究、验证记录和 JSONL 上下文，供审阅。

## 计划审阅后的顺序

1. **核对交付物。** 确认目标 Windows 安装包实际包含 v0.6.6 的 Desktop Main 修复。记录 Desktop、bundled CLI、运行中 daemon 的版本和 server/profile 对应关系；不能只看日志里的 CLI 版本。
2. **隔离复现旧版。** 用一次性账号/配置或已有隔离夹具构造“同用户桌面会话有效、缓存 daemon PAT 被拒”的场景，确认 v0.6.5 的恢复缺口。模拟撤销即可，不需等待一晚，也不撤销真实用户凭据。
3. **验收已有修复。** 对同一场景验证新版自动取得新 PAT、保持原账号/profile、替换旧 daemon 并恢复两个工作区运行时。验证正常重开不重复 mint，网络/5xx 不误清凭据，用户会话本身失效才进入登录。
4. **现场闭环。** 在用户授权的交付范围内，验证 Windows 重开和至少一次实际休眠/次日恢复；检查运行时及绑定智能体最终一致，无需原脚本。Qwen 仍低于 0.20.0 时应明确不可用，不能把它算作 Claude 回归失败。
5. **仅对未覆盖失败增量修复。** 若已有修复足够则不增加代码。若仍失败，先确定是 401、进程未换凭据、profile 不一致还是 API/UI 不一致；为可复现缺口补一条有意义的回归，再委派 `trellis-implement` 做最小改动。
6. **检查与收尾。** 有产品变更时委派 `trellis-check`，执行相关 lint/typecheck/测试及边界静态检查；本轮纯诊断不跑无关全仓验证。按实际交付更新风险和验收结果，满足标准后再完成任务。

## 验证命令

```bash
pnpm --filter @multica/desktop exec vitest run src/main/daemon-manager-auth.test.ts src/main/daemon-auth-probe.test.ts src/renderer/src/platform/daemon-reauth.test.ts
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test ./internal/daemon -run '^(TestClient_RenewToken_|TestTryRenewToken_|TestPreflightAuth_|TestWSHeartbeatFreshnessSuppressesHTTP$|TestManagedHeartbeatHTTPFallbackUsesScopedCredentials$)' -count=1 -v
```

如需 Desktop 代码变更，再执行 `pnpm --filter @multica/desktop typecheck` 和针对变更文件的 ESLint。DB handler 回归必须显式连接隔离测试库并确认测试实际运行；其 TestMain 在无 DB 时可能退出 0，不能只看退出码。

## 风险和回退点

- 保留既有账号、工作区、profile 和设备身份；不要清空整个配置、删除任务目录或回退到旧失效凭据。
- 401 才是确认拒绝；超时/5xx 不能推断 token 到期。
- 无故障前日志时，记录“恢复缺口已验证，首次失效原因未确定”，不宣称已复现原事故。
- 当前工作区有其他任务的未提交变更，实施和提交只处理本任务明确拥有的文件。
