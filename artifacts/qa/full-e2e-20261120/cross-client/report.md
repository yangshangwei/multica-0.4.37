# Cross-client execution and installed CLI regression

Twelve canonical cases passed on their latest executions with zero retries or skips: one concurrent Web/Electron A+B execution, six Web/Electron independent-lane completion/cancellation controls, two creator-removal/reconnect cases, two offline result-notification cases, and one exact historical installed CLI authority boundary.

The two lifecycle cases first failed (native heading5s and runtime reconnect30s); both passed unchanged on a separately recorded run (25.1s/28.8s). Original failures remain in collaboration-lifecycle-controls.json, successful replay in lifecycle-replay.json. No timeout/selector/assertion edits were made.

Actual Linux ARM64 namespace runners execute the current compiled daemon and deterministic provider. Actual Electron renderer/preload and production Web use the fixed Core build. Historical CLI checks run the exact locally cached v0.6.7 image sha256:8d5ede4df2fa4b66d499803b95428b729fcb6ba753b3800e9b931b968ee16104 with network=none; a private stdio relay admits only the expected paths and synthetic credentials.

Owned API18578,Web13498,renderer5672 processes stopped, ports free, database client count0. Gate restored false/true/false with monotonic epoch4 (cleanup.json). Owned fixture containers/volumes are removed by the canonical fixtures; no real installed agent account is used. These results establish current controlled-runtime and old CLI compatibility, not commercial model quality or native installer upgrades.
