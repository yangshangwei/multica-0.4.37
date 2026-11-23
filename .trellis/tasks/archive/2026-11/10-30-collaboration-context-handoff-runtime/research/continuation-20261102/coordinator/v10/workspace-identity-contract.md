# Workspace identity contract for production verification

The v10 investigation proved an SSR auth-facade split: CoreProvider registered module 99423 while AdminSessionBoundary read module 10100, which had no registration and threw. The retained build trace shows CoreProvider resolving auth from final4 and the boundary resolving it from final3. Identical package bytes and a clean tracked source manifest did not prevent this module-identity failure. The exact upstream resolver mechanism remains a hypothesis until a fresh owned build is reproduced.

Before the next production build, record the effective physical roots of every internal workspace package consumed by the app and its shared packages. App and shared consumers must resolve the intended workspace sources in the candidate. Borrowing a donor checkout’s whole node_modules directory can retain its workspace links; any repair must avoid mutating the donor directories. External dependency caches may be shared when their locked installed bytes are demonstrated unchanged.

For each app and build layer, registration and consumers of a singleton must resolve the same module identity. Server and browser layers may legitimately have different module IDs; compare within one graph, not across layers or apps. For this incident, verify the auth registration/accessor graph and retained build manifest/trace after an owned clean build. Keep the missing-registration guard and existing assertions/timeouts.

The replay sequence is: preserved interrupted v10 record → exact integrated source/E2E patch candidate → owned internal workspace dependency layout → minimal production SSR browser reproduction with console/pageerror/trace evidence → independently reviewed source/configuration/artifact identity → single complete make check → same-candidate password7 → committed final RK1 inputs and separate serial paid windows. No raw cookies, passwords, credentials or private Provider prompts belong in the public diagnostic record.

This note is a task-owned spec draft for the shared verification owner to integrate. It does not activate a paid gate, modify source/dependencies, or substitute partial results for a completed full check.
