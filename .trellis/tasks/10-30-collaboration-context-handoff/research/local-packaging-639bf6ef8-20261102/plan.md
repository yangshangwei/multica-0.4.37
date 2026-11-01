# Local candidate artifact preparation

Candidate639bf6ef85ad894553b244873c2ed8d9f9dd29b2, detached clean worktree owned only by this lane. No product source or tracked dependency changes. Produce local macOSarm64 app/ZIP/DMG and Linuxarm64 directory artifacts using the repository package wrapper and already cached Electron39.8.7; do not publish, notarize, download, install tools, touch databases or run Agent CLIs.

Every subprocess uses a sanitized child environment and a macOS sandbox profile that denies network operations. First prove local execution works and a loopback bind is denied. Install only frozen/offline lockfile dependencies with scripts ignored; copy the matching cached Mac Electron distribution and use local MDX postinstalls if required. Extract the already cached Linux Electron archive into a fresh task-owned directory.

Use one platform at a time, explicit fresh output paths outside apps/desktop/dist so the wrapper cleanup cannot delete an earlier package. Require publishnever, version0.6.8-rc.20261102, disabled identity autodiscovery and notarization; remove Apple/CSC/account secrets. Supply explicit electronDist for each platform. Preserve any failed output and raw logs. Do not retry a download/cache failure through network.

Record source/lock/script/cache/binary/ASAR/archive hashes, commands/exits and actual signature/version/architecture. Check packaged runtime allowlist and absence of private environment/tool/task content. Mac own bundledCLI --version is allowed; do not run Linux binaries or containers. Linux .deb/rpm, native Linux/Windows installation and actual installed app/account journeys remain NOTRUN because this task covers local artifact preparation only. FPM/dpkg-deb/rpmbuild absence is an explicit retained tool gap, no installation requested.

Rootcoordination: readroot-coordination.md beforeanynetworkfallback; localadhoctimestamp=noneCLIoverrideperinstalledschemaisauthorized,noproductchanges/networkallow.
