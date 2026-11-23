# Linux ARM64 DEB package-manager lifecycle

The frozen candidate DEB was installed, removed, reinstalled and purged once inside a fresh task-owned Linux arm64 container using cached image `sha256:0d433ecf8ede66777494051568fd760468be288fd99d36d80bc68d022769471c`. The package was mounted read-only, networking was disabled, and no application GUI, daemon, Provider, database or package download ran.

All **95 assertions passed**. The package reported `multica` / `0.6.7~476~g1a32ca36c` / `arm64`; installed executable, ASAR and bundled CLI hashes matched the frozen package pins; desktop entry, launcher, alternatives, icons and ownership were checked; a separately created `/usr/bin/multica` sentinel remained unclaimed and intact across remove/reinstall/purge; four synthetic user state files remained unchanged. Final purge removed package payload, launcher, desktop entry and icons. The exact container was removed and no owned container remains.

This closes package-manager lifecycle evidence only. It does not prove native Linux GUI startup, full packaged Main collaboration execution, installer upgrade from an older release, signatures, or real Provider/model behavior.
