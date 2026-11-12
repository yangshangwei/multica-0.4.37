# Runtime handoff environment boundary

The runtime-handoff Web and Electron specs use `seedRuntimeHandoff`, which calls the password-only `seedOfflineHandoff` fixture. They now explicitly skip only when `E2E_PASSWORD_AUTH !== "1"`, matching the existing offline-handoff and retained-attachment lane pattern.

Configured password runs still execute all fixture environment validation and fail for missing or wrong API, database, renderer and session setup. The legacy run must report these intentional skips separately; the final password gate must execute both cases without skipping. No business assertion changed. Root owns the concurrent E2E environment spec update and final lane registration.
