# Remote merge delivery

The user explicitly requested continued merge on 2026-11-23. Candidate81193dd29 and main dd87e0b33 were re-fetched and checked; all38 previously reviewed source hashes matched. Six local commits were pushed to origin/codex/cross-workspace-collaboration. PR #6 was created in the fork repository: https://github.com/yangshangwei/multica-0.4.37/pull/6 . Its body includes the reviewed image exemption text; the actual local image check with that same body exits0 and reports all121 images.

GitHub CLI defaults to the upstream repository in this checkout. Earlier unqualified PR/run lists therefore did not establish the fork's run history; all subsequent remote commands explicitly use --repo yangshangwei/multica-0.4.37. The fork has an earlier merged PR#4 and repeated workflow-parse failures. No current main merge has occurred yet.

GitHub rejected agent-critical-path.yml at job-level env lines50–52: runner.temp is not valid in that expression context. Regression first failed (3pass1fail) against original. The fix initializes paths from RUNNER_TEMP into GITHUB_ENV in a shell step before install/allocation/run. No provider, permission or output path semantics change. This is a merge-blocking CI fix within the approved task; child-agent transport unavailable, so parent executes bounded fix directly and records checks.

PR CI run45195437861 began. Image-budget passed; remaining remote checks are being monitored and failures distinguished from runner allocation/toolchain errors before merge.

Windows installer, execution-environment and collaboration-native jobs failed during checkout: retained evidence paths exceeded the default Windows path limit. There are no Windows-invalid filenames;139repository paths exceed260characters before the runner checkout prefix. Enable `core.longpaths` on the ephemeral Windows runner before checkout in those three jobs. A new contract fails on the original ordering and passes after the repair. No evidence files or platform tests were removed.

Local verification for both fixes:74Node contract tests pass;checkJs and scopedESLint pass using the existing shared-core dependency resolution. The initially generic lint invocation flagged the existing yaml resolution from core; packageDir root+core correctly models that existing contract without adding a dependency or source suppression.

The first PR vulnerability job failed while downloading redis from proxy.golang.org (`HTTP/2 INTERNAL_ERROR`), not a vulnerability result. Latest candidate rerun will supply a fresh result. First PR frontend-test, sqlc-check, image-budget and native Linux/macOS jobs passed before the CI repair push.

Windows native execution reached the real tests after checkout repair. It exposed three platform assumptions: the config fixture used POSIX-only absolute paths, the Codex resume fixture created the host root through ordinary MkdirAll before the production private-ACL initializer, and Git helpers inherited long temporary work paths while their deliberately isolated Git config excluded the checkout-level longpaths setting. Use native temp paths in the fixture, initialize Host.Probe before fixture descendants, and set core.longpaths explicitly in the helper command's frozen Git options. Keep all original provider/termination assertions. The focused original Git/Codex/config tests passed locally; native Windows replay is still required.

Agent smoke now builds and starts API/Web, but refuses its Web listener as unowned. Added only bounded numeric launcher/listener/group/parent diagnostics before the existing refusal; ownership checks and cleanup are not relaxed. Remote replay will establish the process shape before a fix.
