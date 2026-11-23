# Installed Main followup independent review

**PASS, zero blocking findings.** This is a read-only static review, not a live execution result. No test, build, browser, database or service command was run during the externally owned model window.

## Exact changes

- Diagnostic patch SHA `4681777163391fc340de705fc371d6ed4361636f9d8175008882a408b3b16a35`, controller before `aa0f1ff6375dc9ca462a1361e5c3324fde58835c1d75b952c39e33eda2b2e749`, after `8bd7f89c3381cc99f6e51877b017ad6d8b1bdb8882cdeeaade99e1575721a363`.
- Outer restart patch SHA `715a8b3515c69d900e82669d247341161a1130f5156ea5b89b551e8cbe0a4df0`, spec after `e439fa251286bd879c1eda278ce1ae264ae8dd3744fba9005e196f069488c97f`.

Both patches match the exact frozen-to-root file differences. The diagnostic hash supersedes the older hash in the original dispatch. No product file is part of either patch.

## Behavior and evidence checks

The outer host restart expects the authenticated managed daemon to be running. The controller already verifies that running daemon against the task-private profile, PID file, server URL, deployment and authenticated user. Initial unauthenticated launch and metadata still require stopped, and autoStart=false remains asserted. Queue paused state, execution equality across restart, two existing provider visits and explicit Resume leading to the third visit are unchanged.

The Linux diagnostic code does not relax the running-state expectation. It catches an original assertion failure, captures a bounded read-only projection, and rethrows the failure. Profile files are restricted by realpath to the expected private home and regular-file tails are at most64KiB. Raw log text, credential values and metadata proofs are excluded from the returned projection. Logs contribute hashes and fixed marker/code names; config contributes presence/match booleans; daemon metadata contributes allowlisted state/stage/code fields. Main stderr is truncated before decoding and retains only the last20 recognized codes. Directory/CLI mode, owner and writability observations are read-only. The token presence key matches the frozen CLI Config JSON contract.

No blocking issues were found. No fixes were made by this reviewer. Lint, type-check and tests were not run because the main session requested static review during a shared execution window. Root must apply the two exact patches, refresh runtime source/provenance bindings and execute the original macOS/Linux cases afterward. The diagnostics only improve failure evidence; they do not establish that the Linux daemon startup problem is resolved.
