# 原生 response-stage 前置条件续接

当前源码：`103e2bc329a9fb845d9f45c20b2a9bb3ebb54625`。本记录由 `native_stage_resume` 维护，主会话消息暂时返回 `agent thread limit reached` 时也可从这里恢复交接。

## 已准备、待独立审查

`native-response-01` 的真实失败发生在私有 probe 的 context 准入，尚未打开 renderer/Electron。原运行器的 `--probe-only` 有意把 `context.web` 和 `context.renderer` 写成 null；原 probe 却要求后者是 URL。运行器已经在同一执行目录的 `config.private.json` 和实际 child environment 中分配了 `COLLABORATION_ELECTRON_RENDERER_URL`，也把这个 origin 加入真实 API 的两个 allowlist。

修正只涉及 private `native-response-probe/probe.py` 与 `probe_selftest.py`：严格保持两个 context 字段为 null，renderer 从真实 child environment 读取，并与该 lane 的原始 `config.private.json` 一致校验。继续要求同一 API、数据库、deployment、精确 context/output label 和两个 origin allowlist。由 probe 自己启动的 renderer 仍逐字节核对 index、核对进程组和监听端口、finally 关闭。原 `probe.mjs` 的自然 HTTP200、response-stage 暂停、JS 未完成、无替换释放、body hash 相等断言完全未改。

旧程序、测试和已接受 review 保存在 private `native-response-probe/interface-v01/`。新的 6 项 RED 测试保留 2 个真实/合成有效 context 错误与 1 个错误放行；最终 8 项离线测试全部通过，未进行 SQL、HTTP 或 Electron 调用。

新审查模板为 private `native-response-probe/probe.py.interface02-review-template.json`，状态仍为 `REVIEW_REQUIRED`。程序 SHA256：`e71c153e13b86082f252574631593076c5329a4f3dc581c4d885a2bef0666521`。原执行 sidecar 未改，故不能运行修正程序。主会话需要审阅 QA `native-response-probe/interface-02.diff`、`interface-{plan,red,green}-02.json` 并签发匹配的独立 review。

审查和 runtime 独占时段成立后，使用全新标签执行：

```text
python3 PRIVATE/runtime/run_runtime.py run --mode password --native --probe-only --probe ABS/PRIVATE/native-response-probe/probe.py --label native-response-02
```

自然传输前置条件实际通过并经复核之后，才能授予原 Desktop recovery-extras 第六项的 hold-accepted 前置标记。离线通过不构成该授权。

## Stage 与 Linux 交接

- `stage-02` 已有独立 accepted review 和 inputs，但没有 `run-start`。它选中 `e2e/collaboration-stage-execution.spec.ts` 原 3 项，与当前后台队列的 `collaboration-stage-approvals.spec.ts` 原 1 项不同。runtime 负责人已预约当前 batch 完整结束后的独占服务时段。M1 原 2 项已通过，不重跑。
- Linux full Main 仍是单独的原 `e2e/collaboration-installed-main.spec.ts` 1 项；offline 包组装已从原 current06 frozen tree、缓存 Electron39.8.7 和当前 Linux CLI 开始，结果待核实。
- 发现旧 Linux adapter 的 Python `subprocess.Popen` patch 无法拦截 Playwright Node child 自己调用的 `spawn("docker", ...)`，因此准备的空 `/root` 和 provider 覆盖不能按该方案证明已到达容器。尚未运行这个 adapter。修正应使用 child PATH 上的任务私有 Docker wrapper，并用实际 Node spawn 离线传输验证；需要新的独立审查后才可运行 Main。

所有 QA 相对路径均位于 `artifacts/qa/main-post-merge-e2e-20261124-080504/`。所有 `PRIVATE/` 路径均位于 `.omx/private/qa-work/main-post-merge-e2e-20261124-080504/`。产品源码、canonical 测试、依赖、native-acceptance 共用 helpers 均未修改。

## 2026-11-25 06:57 UTC runtime slot returned

`stage-02` completed all 3 original cases with no failures/skips/retries. `native-response-02` completed its real native natural-response prerequisite: HTTP200, JavaScript unresolved while held, exact same body SHA256 before/held/after original release, no replacement and daemonStarts0. Renderer/API/process/protocol/source cleanup passed; cluster other clients0 and both runtime locks absent. Full receipts and hashes: `stage-acceptance/slot-handback-runtime-20261125-0657.json`. Parent must independently review `runtime/native-response-02/{probe-result,native-response-detail,probe.receipt,completion,cleanup}.json` before issuing any hold-accepted flag. No flag set here. Shared runtime slot belongs to parent/runtime again; Linux remains offline preparation. App ancestor messaging refused the delivery; this durable note and commentary carry the handoff.
