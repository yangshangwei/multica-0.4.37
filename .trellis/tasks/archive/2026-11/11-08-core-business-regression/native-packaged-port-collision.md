# Native packaged Main: profile health-port collision

The final native packaged Main journey passed **1/1, 28.4 seconds, zero retries/skips**, using the unchanged current mac-4 package and production Main/preload/renderer. The failed startup was caused by local test-environment isolation, not an enrollment or provider implementation defect. No product code change was needed.

## Evidence and cause

Original `packaged/packaged-1/pw-10` reported managed registration `operation_failed`, with no daemon PID/log and no managed deployment in its profile config. Read-only inspection showed the installation identity already contained a saved installation ID and key version, and no unexpected provider invocation marker existed. A fresh private diagnostic replay captured only the exact packaged application's stdout/stderr into mode-0600 files. The actual cause was `Daemon credential sync was superseded` at the post-enrollment `assertCurrent` in `daemon-manager.ts`.

The desktop profile for `http://localhost:18953` is `desktop-localhost-18953`; its deterministic health port is **19602**. That port was already held by an unrelated daemon PID **48070**, reporting server URL `http://localhost:18917` and an older CLI candidate. A health poll correctly rejected the foreign server and invalidated the active profile while enrollment completed; the sync fence then correctly rejected the obsolete operation. Private HOME isolates files but does not isolate TCP health ports. The unrelated process was only observed and was never stopped or modified.

Original failures remain under `packaged/packaged-1/` and `packaged/native-diagnostic-1/`. Private diagnostic logs and the bounded foreign-listener record are under `packaged/native-diagnostic/`. The package/API setup log is now copied privately before each owned API switch so later restoration cannot overwrite its evidence.

## Environment correction

The private `native-diagnostic/alias-run.py` wrapper starts one owned TCP byte-forwarding proxy on **localhost:19080 → 127.0.0.1:18953** for the native test child. The derived native daemon health port becomes **19594**, verified free before entry. TestApiClient and the packaged app both use the alias; production Web continues to use the original origin against the exact same API process/database. Cookies remain host-scoped to localhost across ports. A live preflight compared complete health and public configuration responses, including PID/commit/deployment identity.

Credential-free tests verified HTTP method/path/body and WebSocket Upgrade bytes are unchanged. The wrapper records controller/proxy/recorder/wrapper hashes, actual proxy PID, alias/target and live API proof. A separate read-only record audit confirms the ready record's PID/origin/target match the live proof. An independent reviewer found no blocking transport or identity issue. No application source, IPC, login response, production assertion or package byte was replaced.

## Passing run and cleanup

```sh
python3 /Users/artisan/.multica/qa/core-20261108/packaged/native-diagnostic/alias-run.py \
  --execute --run-id native-alias-1 --ids pw-10
```

Evidence root: `/Users/artisan/.multica/qa/core-20261108/packaged/native-alias-1/`.

- `pw-10/results.json`, case inventory and installed-host attachment retain actual login, managed daemon, native Host controls, restart and explicit Resume evidence.
- `pw-10/native-api-alias-proof.json` and `native-api-alias-record-audit.json` bind the proxy and actual API.
- `final.json`: complete, original API restored, no cleanup errors.
- `pw-10/final-cleanup-audit.json`: alias port 19080 and task daemon port 19594 have no listeners; original LaunchServices default/handler set restored exactly; package cleanup has no errors.
- Exec session 80208 was observed terminal with exit 0 before returning API ownership to root.

The original complete failures are preserved. This is current-source macOS packaged Main business acceptance, not a publisher signature/notarization, installer/updater or commercial-model claim.
