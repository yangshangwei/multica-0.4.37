# S2 private control bridge — independent check

- Reviewer: `/root/s2_check_resume`, 2026-11-14.
- Verdict: **PASS for the frozen private transport tool after local corrections.** The coordinator may use the exact checked bytes for its separately bound test harness. No product API, database, deployment, model or actual Go harness was executed by this review.
- Scope: only the private `stage-control-bridge/bridge.mjs`, its pure Node tests, this report and private evidence. No server/client/E2E/spec/dependency/Git changes.

## Findings (fixed)

### SCB-01 — rejected input could still invoke a later frame in the same chunk

The original parser destroyed the socket after invalid JSON but continued consuming its buffered lines. A following valid `execution.prepare` reached the fake child. Invalid UTF-8 was also silently replaced before forwarding.

The parser now stops permanently after rejection, uses fatal UTF-8 decoding, and sends accepted JSON text once without reserializing it. The harness still owns its closed action/duplicate-key decoder. The two real counterexamples fail against the retained original bridge and pass against the corrected source.

### SCB-02 — partial setup and asynchronous startup failures leaked resources

Opening the second log after the first succeeded leaked the first descriptor on failure. A duplicate ready frame during pending `listen`, or a ready-file publication error, could escape as an uncaught exception.

Partial setup now closes already opened descriptors. Ready is marked seen before binding; publication failures enter the same bounded cleanup path. Startup rejects only after the directly owned child and listener have closed. Existing log/ready entries remain intact. Tests cover the actual second-open failure, same-chunk duplicate ready, executable startup failure, and socket/ready paths occupied during startup.

### SCB-03 — startup cancellation and shutdown could leave an owned child running

The original bridge had no ready deadline or cancellation handle before readiness. Its CLI registered signals only after ready. A child ignoring shutdown and SIGTERM could keep `stop()` pending forever. A further counterexample showed that one-shot CLI signal listeners let a second SIGTERM kill the bridge before it reaped its child.

The tool now has a30-second ready deadline, a5-second graceful root shutdown deadline and a1-second TERM-to-KILL interval. These are private transport lifecycle limits; product/Playwright/Provider deadlines were not changed. Tests can select smaller positive values within those maxima. Module callers may provide an AbortSignal; the CLI installs idempotent signal handling before startup. Only the held, directly spawned child handle receives signals. Its pipe-drain wait is also bounded; no descendant receives a signal.

Root shutdown closes socket admission immediately, and cancellation/exit destroys pending client transports without manufacturing a result or retry. Tests prove startup cancellation, repeated CLI signals, a child ignoring SIGTERM, and immediate shutdown admission closure.

### SCB-04 — Node Unix listener cleanup could delete a replacement path

An actual macOS test replaced the published socket pathname with a regular file. The original `server.close()` deleted that replacement through libuv, despite the bridge's separate inode guard.

The corrected tool binds in a new, exclusively owned0700 directory inside the canonical private parent, chmods the socket0600, and publishes it with the public Node `fs.linkSync` API. Hard-link creation refuses an occupied pathname atomically. Node automatically unlinks only the private binding path; public-path cleanup compares the original device/inode before removal. The binding directory is removed only if its device/inode still match.

The macOS probe established actual connection through the hard link, equal device/inode identity, private-path removal by `server.close`, and retention of the published link. The regression verifies that a replacement at the public path survives cleanup. Existing files, dangling symlinks and entries created during startup are refused and retained. Both the public and private bind path are checked against the macOS103-byte AF_UNIX limit before spawning the child. The private binding directory is exclusive to this bridge; other actors must not write inside it.

### SCB-05 — transport accumulation and broken reply correlation were unbounded

The tool now caps active sockets and pending requests at64, input frames at64KiB, socket reply buffering at128KiB, and each raw output log at64MiB. Idle sockets close after30seconds. Reaching a transport bound closes/refuses transport; it does not classify an execution as not-started or authorize a retry. Disconnected in-flight IDs remain reserved until their original reply or child exit, preventing another send from borrowing the pending correlation.

Reply IDs must match a pending request; an unsolicited reply stops the bridge. Out-of-order replies remain attached to their exact pending sockets. Completed IDs can be reused for the original historical receipt. Simultaneous duplicate IDs are refused without a second forward. The strict business result and actual original-operation semantics remain with the real harness/service and the E2E consumer.

## Findings (not fixed)

No open blocking finding remains in this private tool. Its authority boundary is unchanged: the coordinator selects and verifies the executable, database, environment and source bindings before launch. This bridge is not a database identity checker, business-result validator, execution retry queue or public capability. It assumes the private harness emits one correlated reply per accepted input; the transport contract does not include a second independent wire correlation ID.

Actual Go harness startup, router/auth/background behavior, real control actions, E2E consumers, Web/Desktop and external integrations remain **NOT_RUN in this review**. Test children were explicit Node fixtures. Their ready URLs were declared loopback fixture values, not HTTP servers launched by the checker.

## Verification

| Check | Result |
| --- | --- |
| Retained original tool/test/log/exit files | Four original files preserved with hashes |
| First regression run | Original8 tests passed;8 added tests failed; original log/exit retained |
| Exact original-source counterexample replay |8 selected failures reproduced against a frozen original bridge with hash-bound test source |
| Intermediate corrected run |15/16 passed; occupied-path validation ordering regression preserved and corrected |
| Repeated CLI signal counterexample |1 selected failure reproduced against the intermediate frozen source; corrected to persistent idempotent listeners |
| Final frozen pure suite |**26 PASS,0 FAIL,0 SKIP,0 CANCELLED** |
| `node --check` | Both files pass |
| ESLint core rules | `no-unused-vars`, `no-unreachable`, `valid-typeof`: pass, empty output |
| macOS Unix hard-link probe | Real socket connection and cleanup identity assertions pass |
| Final source/resource audit | Working and frozen hashes match;0 remaining owned binding directories or fixture Node processes |

TypeScript type-checking does not apply to these two JavaScript files. Tests ran with an explicit PATH and separate private HOME/TMP, without inherited database/account environment. Original failures and all command logs are retained; counts describe this tool's selected tests and are not S2 acceptance cases.

Final source, exact commands, hashes and raw evidence are bound by:

`/Users/artisan/.multica/acceptance/pay241-20261114-resume/stage-control-bridge/review/final-manifest.json`

The original8-test run, first red run, original-source replay, intermediate failure, repeated-signal failure and final26-test run remain separate records. Startup errors never imply a successful or definitely unstarted external execution. Root must still verify its actual harness/API/source/database and record that separate run before claiming integration acceptance.
