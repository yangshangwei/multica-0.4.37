# 原始 Playwright 失败证据

本文件是完整 `make check` 原始日志的逐行摘录，保留原始失败名称、断言、测试结果和退出码。
完整日志仍保留在 [`make-check.log`](../make-check.log)，SHA256 `10221e3f66435befa5c15ff43813576e3733e20cd8e04774cebabe86155c1d71`。

- 候选：`52162f19af7e4a1e2459ce3f08e4627ca14c3dfc`
- 检查环境：`check-20261108112551-74475`
- 原始结果：`327 passed`、`112 skipped`、`2 failed`，`exit 1`
- 独立复跑：[`targeted-rerun.log`](../targeted-rerun.log)，SHA256 `b50c28a814a8a91bd91221c697a88ffa026e48880b9ca03cfc70c8e3d20cee37`，同候选/独占环境，`2 passed (19.2s)`

> `make check` 完成清理后，候选 worktree 中原始 Playwright 的 `test-results/**/error-context.md` 不再可读；以下内容直接从原始门禁日志摘录，日志本身是不可变失败证据。截图附件路径也按原始输出保留。

## 原始输出摘录

```text
  1) [chromium] › e2e/collaboration-workbench-web.spec.ts:183:1 › Web stale lifecycle conflict immediately reports refusal and preserves input for explicit retry

    TimeoutError: page.waitForResponse: Timeout 15000ms exceeded while waiting for event "response"

      271 |     }, { times: 1 });
      272 |     const originalRequest = page.waitForRequest((request) => new URL(request.url()).pathname === lifecycle && request.method() === "POST");
    > 273 |     const originalResponse = page.waitForResponse((response) => new URL(response.url()).pathname === lifecycle && response.request().method() === "POST");
          |                                   ^
      274 |     await form.getByRole("button", { name: "Accept and complete", exact: true }).click();
      275 |     const original = await originalRequest;
      276 |     const originalKey = original.headers()["idempotency-key"];
        at /Volumes/artisan/code/2026/mc-merge-unmerged-worktrees-20261108/e2e/collaboration-workbench-web.spec.ts:273:35

    Error: End of central directory record signature not found. Either not a zip file, or file is truncated.

    attachment #1: screenshot (image/png) ──────────────────────────────────────────────────────────
    test-results/collaboration-workbench-we-66297-es-input-for-explicit-retry-chromium/test-failed-1.png
    ────────────────────────────────────────────────────────────────────────────────────────────────

    attachment #2: screenshot (image/png) ──────────────────────────────────────────────────────────
    test-results/collaboration-workbench-we-66297-es-input-for-explicit-retry-chromium/test-failed-2.png
    ────────────────────────────────────────────────────────────────────────────────────────────────

    Error Context: test-results/collaboration-workbench-we-66297-es-input-for-explicit-retry-chromium/error-context.md

  2) [chromium] › e2e/iterations-i1-desktop.spec.ts:33:1 › I1 Electron atomic handoff preserves existing next-period commitments

    Error: [2mexpect([22m[31mlocator[39m[2m).[22mtoBeHidden[2m([22m[2m)[22m failed

    Locator:  getByRole('dialog')
    Expected: hidden
    Received: visible
    Timeout:  5000ms

    Call log:
    [2m  - Expect "toBeHidden" with timeout 5000ms[22m
    [2m  - waiting for getByRole('dialog')[22m
    [2m    9 × locator resolved to <div id="_r_92_" data-open="" tabindex="-1" role="dialog" data-base-ui-focusable="" data-slot="dialog-content" aria-labelledby="base-ui-_r_9g_" aria-describedby="base-ui-_r_9h_" class="fixed top-1/2 left-1/2 z-50 grid w-full max-w-[calc(100%-2rem)] -translate-x-1/2 -translate-y-1/2 gap-4 rounded-xl bg-surface-raised p-4 text-body text-popover-foreground shadow-[var(--floating-shadow)] ring-1 ring-surface-border duration-100 outline-none data-open:animate-in data-open:fade-in-0 data-open:zoom-in-95 …>…</div>[22m
    [2m      - unexpected value "visible"[22m


       at fixtures/iterations-i1.ts:82

      80 |   await expect(assignment.getByLabel("Reason", { exact: true })).toHaveCount(0);
      81 |   await assignment.getByRole("button", { name: "Add 2 tasks", exact: true }).click();
    > 82 |   await expect(assignment).toBeHidden();
         |                            ^
      83 |   await expect.poll(async () => (await api.requestJSON<Detail>(`${base}/iterations/${first.id}`)).statistics.original).toBe(0);
      84 |
      85 |   await page.getByRole("button", { name: "Start iteration", exact: true }).focus();
        at iterationClosureFlow (/Volumes/artisan/code/2026/mc-merge-unmerged-worktrees-20261108/e2e/fixtures/iterations-i1.ts:82:28)
        at /Volumes/artisan/code/2026/mc-merge-unmerged-worktrees-20261108/e2e/iterations-i1-desktop.spec.ts:36:3

    attachment #2: failure-state (text/plain) ──────────────────────────────────────────────────────
    http://127.0.0.1:58975/
    Workspaces
    Collaboration
    Toggle left sidebar
    P
    P1 browser acceptance
    Search...
    K
    New Issue
    C
    Inbox
    My Issues
    Chat
    Work
    Workspace issues
    Projects
    Iterations
    Triage
    0
    AI Team
    Agents
    Squads
    Autopilot
    Resources & connections
    Skills
    MCP
    Capability market
    Runtimes
    Analytics
    Setting...
    ────────────────────────────────────────────────────────────────────────────────────────────────

    attachment #3: failure (image/png) ─────────────────────────────────────────────────────────────
    test-results/iterations-i1-desktop-I1-E-dcf89-ing-next-period-commitments-chromium/attachments/failure-093551356f6a372191e068f468658fa065dcb158.png
    ────────────────────────────────────────────────────────────────────────────────────────────────

    Error Context: test-results/iterations-i1-desktop-I1-E-dcf89-ing-next-period-commitments-chromium/error-context.md

  2 failed
    [chromium] › e2e/collaboration-workbench-web.spec.ts:183:1 › Web stale lifecycle conflict immediately reports refusal and preserves input for explicit retry
    [chromium] › e2e/iterations-i1-desktop.spec.ts:33:1 › I1 Electron atomic handoff preserves existing next-period commitments
  112 skipped
  327 passed (18.5m)
    ✓ desktop-renderer stopped (pid 21222)
    ✓ web stopped (pid 21678)
    ✓ api stopped (pid 20948)
    Dropping isolated Go database multica_check_20261108112551_74475_go
✗ Checks FAILED (exit 1).
make: *** [check] Error 1
```

## 分类

- Collaboration stale lifecycle：原始门禁等待 lifecycle `POST` response 15 秒超时；同一测试在同一候选和独占环境单独复跑通过。
- Desktop Iterations I1：原始门禁等待 native assignment dialog 隐藏 5 秒超时；同一测试在同一候选和独占环境单独复跑通过。
- 结论：这是“原始完整门禁有两个时序敏感失败；单独复跑未复现稳定业务回归”，仍须保留原始 `FAIL` 计数，不能把完整门禁写成全绿。
