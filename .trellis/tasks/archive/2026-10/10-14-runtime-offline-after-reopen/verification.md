# 诊断验证记录

日期：2026-10-14。验证对象为当前源码中的已有修复及相同的 daemon 心跳实现；未修改产品代码。现场 v0.6.5 和新版本 Windows 安装包未在本轮执行。

## 协调者执行

### Desktop 认证与恢复

```text
pnpm --filter @multica/desktop exec vitest run src/main/daemon-manager-auth.test.ts src/main/daemon-auth-probe.test.ts src/renderer/src/platform/daemon-reauth.test.ts
Test Files  3 passed (3)
Tests       44 passed (44)
exit_code   0
```

包含实际 Main/IPC 代码配合隔离文件、网络、进程夹具的测试：同用户失效 PAT 替换、凭据变化后停止/启动 daemon、网络失败保留配置、登出/切服务器后的过时操作保护、快速启动失败后的认证状态。

pnpm 同时报告仓库现有 `package.json` 中 `pnpm.onlyBuiltDependencies` / `pnpm.overrides` 被当前 pnpm 忽略。这是环境配置警告，测试均实际运行通过；本任务未变更依赖或包管理配置。

### Daemon 续期与心跳

```text
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test ./internal/daemon -run '^(TestClient_RenewToken_|TestTryRenewToken_|TestPreflightAuth_|TestWSHeartbeatFreshnessSuppressesHTTP$|TestManagedHeartbeatHTTPFallbackUsesScopedCredentials$)' -count=1 -v
12 named tests: PASS
ok github.com/multica-ai/multica/server/internal/daemon 0.705s
exit_code 0
```

覆盖续期接口、401 提示、正常未到续期窗口、启动续期次序、超时/临时失败、WS ACK 抑制 HTTP、过期 ACK/断线恢复 HTTP。执行带 agent CLI guard，没有调用本机已安装智能体。

### 版本证据

- `git merge-base --is-ancestor 296c377a4 v0.6.6`：exit 0。
- `git diff v0.6.6 HEAD -- apps/desktop/src/main/daemon-manager.ts apps/desktop/src/main/daemon-manager-auth.test.ts`：无差异。
- `git diff v0.6.5 HEAD` 的目标心跳、续期、清扫器路径：无差异。
- v0.6.5 缺少上述 Desktop 凭据恢复变更；v0.6.6 包含，见研究文档的源码说明。

## 研究代理执行

`research/server-liveness.md` 记录三个 WS heartbeat 测试和两个 auth TTL 测试通过，采用本机 loopback/纯函数夹具，不访问生产数据。

## 未验证

- 脚本执行前的 token 到期/撤销/版本失效原因，以及同一时刻的 API 与 UI 状态。
- 用户 Windows 真实重开、休眠恢复及升级后行为。
- DB-backed handler、真实 Claude/Qwen 调用；本轮不消耗账号额度。
- 现场健康接口：本机请求五秒超时，无响应，不能据此判定现场服务不可用。
- 无产品代码变更，因此未运行全仓 lint/typecheck/build/E2E；没有把这些未执行检查记作通过。
