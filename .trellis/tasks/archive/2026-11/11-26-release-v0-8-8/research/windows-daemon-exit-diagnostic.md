# Windows daemon early-exit diagnostic

Run45686400357 reaches real boot/runtime/password login. Services remain owned/healthy. Public daemon projection shows expectedrunning/actualstopped, profilematches, installation server/user identity matches, but daemonserverMatches=false; profile config/owner/PID/errlog exist while structuredlog does not. This is evidence of a failed startup/readiness attempt, not a demonstrated wrong server configuration: when status is stopped, the runtime server field may be absent.

The controller already converts private errlog/Main stderr into a fixed20-code allowlist and8literal markers. Preserve this source contract: public projection intersects arrays with those literal lists and exposes no raw text/path/PID/hash. No source logging, timeout, retry or product change. The original private errlog is unavailable from the completed runner; new diagnostics require a new run.

Read-only startup audit: Desktop executes bundled CLI daemonstart with explicit profile/managed handoff and inherited alignedhome. Go background launcher writes daemon.err.log, reserves PID, and spawns foreground (Windows breakaway with access-denied fallback). Foreground checks process identity/handoff before logger initialization. Missingdaemon.log narrows the evidence but does not prove which early guard failed. No provable new product fix yet.

## Projection prepared for the next native run

The existing public phase report will retain main_error_codes and daemon.log/daemon.err.log error_codes/markers only after intersection with the controller's literal20codes/8markers. Nonarray, arbitrary string, object entries and duplicates are removed. No raw log, hash, byte count, path or PID is exposed.

Also project the existing cli/connection diagnostic using that same code allowlist, five fixed stages and HTTP100–599 integer ornull; profile config server/deployment/credential-presence and owner-user comparison are strictboolean/null. Never publish token/server/deployment/user values. Existing bounded exact-marker parser remains unchanged.

Narrow daemon projection tests5PASS/0FAIL/0SKIP, including injected secrets/unknown strings/wrong types/duplicates/malformed/oversize marker cases. node--check/diff-check pass. Only diagnostics module and existing config node-test changed; no product, controller, workflow, startup policy or timeouts changed. Parent review and new native run required; previous runner private logs cannot be recovered from sanitized artifacts. No commit/push.
