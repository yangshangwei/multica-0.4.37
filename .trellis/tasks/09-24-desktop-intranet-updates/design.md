# Design

Use a separate Docker Compose project with official Nginx stable Alpine. Port 18080 defaults to loopback and can bind a LAN address explicitly. Mount only ./data/desktop-updates/public read-only; private publication staging/backups remain outside the HTTP root. Enable GET/HEAD, Range, no-cache and a /health endpoint; deny directory listings and dotfiles. Pin the pulled image digest after verification.

A desktop-side Node CLI reuses the installed electron-updater YAML parser (no new dependency). It collects generated latest*.yml and versioned installers/blockmaps from flat or platform/architecture subdirectories, rejects unsafe references and collisions, validates sizes/digests, stages copies privately, publishes immutable artifacts first and atomically renames each metadata file last. Stable versions are default; explicit --allow-prerelease supports existing local smoke-test builds. A local publication lock prevents concurrent writers. Offline installer uses the same collector.

A standard-library JSON configuration helper merges only updateUrl into an existing desktop configuration, backs it up and writes atomically. It requires an existing valid business configuration rather than inventing one. Development mode uses environment overrides, so document packaged-client behavior separately.

Keep independent code lanes: artifact publisher/offline collection; updater regression/docs; Nginx/configuration/integration. No automatic application restart or installation. Failed publication may leave unreferenced immutable files, but never incomplete referenced files. Metadata backups permit withdrawal for clients that have not downloaded; no claim of client downgrade.
