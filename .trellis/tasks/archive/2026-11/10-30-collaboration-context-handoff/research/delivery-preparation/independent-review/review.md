# Independent Windows delivery preparation review

Verdict: **pass-with-notes** for tip `69bbc9cf0f74f3b0177d9c8bd4c812acaa9a3187`, including Windows harness product `9de37277d` and separately corrected Host cleanup `8d976a696`, compared with `822c15785`. Reviewer `/root/delivery_independent_review` did not implement these changes. Only this report directory was written; no database, migration, installer, GUI, Provider, installed Agent CLI or external CI command ran.

## Findings

- **WD-1, resolved blocker:** the original new Windows journey reached a Host close that sent POSIX `SIGCONT` before stdin EOF. The source-bound modeled probe reproduced failure before graceful cancellation/summary. The implementation owner committed `8d976a696`; Windows now skips that signal, retains EOF and the real zero-exit/claims/HTTP assertions, and refuses unsupported suspension helpers explicitly. Independent actual-function cleanup4/4 passed; reviewed file hashes match the committed fix. No native result is claimed.
- **WD-2, low and open:** the shared layout helper preserves literal percent escape sequences when assigning filesystem paths to `URL.pathname`. A path such as `Multica%20Candidate.app` therefore compares against a different decoded path. Default hosted Windows roots do not contain that component, so this does not block the selected runner preparation. See `finding-WD2.md` and the executable pure probe. Correctly encode filesystem paths and add literal-percent coverage when extending the helper.

## Independent verification

The final canonical Node suites passed **25/25,0 fail/skip**, including native-checker omission/skip gates and4 Host cleanup controls. Strict installed transport and whole-journey TypeScript checks passed; selected desktop script lint, workflow YAML parsing, syntax and diff whitespace checks passed. A pure Node probe confirms the nonblocking percent-path mismatch. Original author hashes at the product commit and the separate WD-1 fix hashes match their recorded manifests. The retained Windows amd64 PE hash/machine match author compilation metadata; no PE was executed or recompiled by this reviewer.

## Assessment

The new path reuses the actual packaged Main/preload/renderer and existing native Host fixture. Candidate upgrade/reinstall hooks do not impose the new contract on the old baseline. The workflow input defaults false and selects the Windows lifecycle job; single-test expected/non-skipped/non-retried results are checked before reporting pass. Services use source-bound fixture binaries and an explicit new PostgreSQL cluster/database, validate actual database and cluster directory before migration, keep knowledge false, and keep private profiles/setup/raw Playwright artifacts outside the upload list. Source and installed EXE/ASAR/CLI hashes are reported; cleanup targets owned process identities and preserves failures. These are reviewed code contracts, not executed Windows service assertions.

## Remaining gates

Native PowerShell parsing/runtime, installed Windows/Host GUI journey, upgrade/reinstall, Windows arm64, signed/offline trust, real Provider accounts/quota/AC43 and final merged-candidate checks remain open. The candidate guide still names migration ceiling760 on this branch and explicitly requires rechecking after audit repairs; update it to the actual final ceiling before packaging and compare both delivered guide copies byte-for-byte. This review does not authorize public push, external CI, deployment or release, and does not turn local contracts/cross-compilation into a native platform pass.

Exact command exits/counts, source/log hashes, preserved probe setup errors and the failed remote-doc lookup are in `review.json`. The original WD-1 failure is retained separately; the reviewer did not self-fix it.
