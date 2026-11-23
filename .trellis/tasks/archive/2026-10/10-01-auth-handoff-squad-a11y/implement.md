# Implementation plan

1. Capture and preserve current working-tree changes; other work is concurrently integrating password registration.
2. Auth owner: root. Add an integration regression around the real CoreProvider, API client, initializer and store, pausing the device response. Defer the global unauthorized callback during initialization because the initializer owns its final outcome; keep normal session expiry unchanged. Avoid broad API/store/router changes.
3. A11y owner: bounded helper. Add exact-heading-name regression and hide only the redundant template avatar from the accessible tree at its composition site.
4. Run red/green targeted tests, related authentication tests, package checks and static analysis. Add a delayed real-response browser regression if feasible without weakening existing assertions.
5. Document the restored boundary and verification, without staging or reverting other work. Network sandbox currently denies loopback connections, so do not claim real-server E2E execution.
