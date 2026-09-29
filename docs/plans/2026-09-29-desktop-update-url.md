# Desktop update URL implementation plan

**Goal:** Persist `http://<api-host>:18080/desktop` so installed clients use the intranet update service after entering their server address.

**Decision:** The user explicitly selected the same hostname, HTTP, port 18080, and `/desktop`. Preserve custom update sources; an update source equal to the old server's default follows a server change. Use the existing URL validation and atomic file writer, without adding dependencies or changing UI.

**Architecture:** The runtime parser supplies the default. The packaged loader fills missing fields on disk while preserving unknown fields; development configuration remains untouched. The updater reads the current runtime configuration before each check, so first-time setup takes effect without restarting and an unconfigured client does not contact the packaged GitHub provider.

## Steps

1. Add failing regressions beside `runtime-config.ts`, `runtime-config-loader.ts`, `updater.ts`, and `configure-updates.mjs`: URL derivation, custom-source preservation, server changes, real file persistence/migration, and configuration becoming available after updater initialization.
2. Implement the default in `src/shared/runtime-config.ts`. Reuse the atomic writer in `src/main/runtime-config-loader.ts` for the missing-field migration, preserving unknown JSON fields and leaving explicit values untouched. Keep the explicit configuration CLI's required-URL validation.
3. Replace the updater's startup snapshot with a configuration getter in `src/main/updater.ts` and wire it to `runtimeConfigResult` in `src/main/index.ts`. Preserve architecture channels, single-flight checks, preferences, and existing download behavior.
4. Update desktop specifications and current English/Chinese installation and intranet runbooks. Do not rewrite historical verification evidence.
5. Run focused regressions, the desktop unit suite, desktop lint and typecheck, and `git diff --check`. Review only task-owned changes; preserve unrelated workspace edits.

**Verification boundary:** Unit tests exercise real temporary config files and mock Electron's updater. No installed-client upgrade, installer signing, or live intranet availability is claimed.
