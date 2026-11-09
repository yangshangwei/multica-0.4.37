# Native clipboard focus readiness

The second final full run retained one failure in the native account/popout journey. Before Copy link, the readiness assertion observed `BrowserWindow.isFocused=false`, `webContents.isFocused=false`, while `document.hasFocus=true`. The screenshot showed the correct business page and Copy link control. Native foreground focus is separate from Chromium's document focus; the fixture activated the app/window once, then only observed for five seconds. Other Playwright/native application processes were concurrently present and were not stopped by this task.

Only `e2e/collaboration-account-desktop.spec.ts` changed. Within the original five-second readiness poll, the helper now activates the task-owned application, restores a minimized target or shows a hidden target if necessary, focuses the actual BrowserWindow and its webContents, then samples focus after rendering frames. It retains the exact `{native:true,contents:true,document:true}` assertion. The clipboard sentinel, single real Copy link click, native clipboard read and exact absolute URL assertion are unchanged. No clipboard fallback, product change or timeout increase was added. Visibility/minimized/focus samples are attached on success and failure.

Exact source hash: `5fb058ef8cf9af36febdd00c23b78dbbf573a78bbb2ce283b0921ccb95c23762` → `7119535b04fce5185a23b4bd602b7b1165ce5cbe9b5c1ad7e6431ccc7dc1ac1a`. The root and QA copies match. This is a test-only delta after final package compilation and needs an exact source-binding update, not a package rebuild.

The complete affected spec passed **11/11 tests, zero skips/retries/flaky results, 74.282 seconds**, exit 0:

```sh
QA_RUN_ENV=<current owned registry env> \
  bash /Users/artisan/.multica/qa/core-20261108/browser.sh \
  focus-final1 e2e/collaboration-account-desktop.spec.ts
```

Both native account and run-link copy attachments prove a visible, non-minimized window; all three focus checks true; and native clipboard transition from the task sentinel to the expected absolute URL after one click. Original failure remains in `baseline-final-2/`; the passing replay is `focus-final1/results.json` plus attachments. The decoded named inventory/focus evidence is `/Users/artisan/.multica/qa/core-20261108/native-focus-repair/results-summary.json`, and exact sync identities are in `sync-final.json`.

Exec session 85897 was observed terminal exit 0 before returning API ownership to root. The original full run remains failed; root's new full-suite run is still required for complete acceptance. The precise cause of the competing foreground activation is not asserted from a process list alone; the observed focus state and readiness correction are directly evidenced.
