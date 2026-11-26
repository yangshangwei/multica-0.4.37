# Material workflow browser acceptance

Run from the repository root, with an **already allocated and migrated task database**. Do not source the checkout's development environment or point this runner at an existing API. The runner starts its own API, production Web server and loopback HTTPS GitLab fixture, verifies both API and Web proxy identity, and stops them after Playwright finishes. The Electron test uses the shared production renderer and preload in a fresh native test profile, without starting a real daemon.

```sh
node e2e/fixtures/material-run.mjs "$PRIVATE_UI_RECORD" "$EXPECTED_DATABASE" "$BUILT_WEB_APP" "$BUILT_DESKTOP_OUT"
```

- Use an owner-only allocation JSON (mode 0600, in a private directory). Required fields: `database`, `connection`, `ports.{api,web,gitlab}`, distinct `git_token` and `control_token`, and `env`. The database must be named `multica_gitmaterial_ui_*` and reached on loopback. `env.DATABASE_URL` must match `connection`; the runner checks `current_database()` before starting the server.
- The environment must point `PORT`, `FRONTEND_ORIGIN`, `REMOTE_API_URL` and `NEXT_PUBLIC_API_URL` at the allocated ports on 127.0.0.1. Enable local verification-code sign-in and workspace signup, set a fresh JWT secret, set `MULTICA_VCS_INTEGRATION_ENABLED=true` and provide a test-only base64 32-byte `MULTICA_VCS_SECRET_KEY`. Do not configure mail, external providers or Redis.
- Supply an unexpired test certificate trusted by `env.SSL_CERT_FILE`, valid for IP 127.0.0.1, and its matching `gitlab-key.pem` alongside the record. On macOS the runner adds a Go build overlay for this binary only, using that PEM as its fallback roots. No certificate checks are disabled; no system keychain change is needed.
- Build Web with `next build --webpack` in a private copy of the Web app with the allocated API URLs. Pass that app directory, not its `.next` directory. Build Desktop with `electron-vite build --outDir "$BUILT_DESKTOP_OUT"` from `apps/desktop`. The normal workspace dependencies, Go, Playwright Chromium and Electron must be installed.
- Results and screenshots are in a new private `material-run-*` directory beside the record. Review history intentionally prevents deleting these test workspaces. The task owner disposes of the dedicated database after reviewing the evidence.

This tests a human-entered plan candidate and one actual queue entry on an unclaimed fixture runtime. It does not claim real model generation, installed desktop packaging or live GitLab compatibility.
