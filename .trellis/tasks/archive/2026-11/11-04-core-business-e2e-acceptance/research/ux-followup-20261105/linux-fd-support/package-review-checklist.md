# Fixed CLI package acceptance boundary

Read-only support checklist for the core owner's package variant; no build/database/browser operation is authorized by this file.

1. Preserve base1a32 package, app.asar, Electron executable and prior CLI bytes. Copy to a new path and replace only resources/app.asar.unpacked/bin/multica (confirm actual package-relative CLI path from the fixture). Record all regular files, symlink destinations and executable modes so equality is not inferred from two summary hashes alone.
2. Verify the replacement CLI hash equals the newly built LinuxAArch64 output and binds base1a32 plus exact reviewed FD patchd6f4ce. Keeping the historical version string is acceptable only when evidence explicitly identifies the patched source/binary variant; it is not unchanged1a32 acceptance.
3. Bind the original spec and diagnostic fixture bytes, actual API/Web/renderer source/ports, new package hash and fresh owned DB in a new run. Keep previous r7 failing evidence and preflight rejections intact.
4. Require the original named fullMain test to execute once without skip or automatic retry. Its6directedhumanwrites, exactresults/revisionrecipients, queue/Stop/restart/explicitResume and read/write authority assertions remain mandatory. Reaching close or a successful process exit alone is insufficient.
5. Read no-payload command timeline: restart.close must complete and replacementMain must open. Preserve primary assertion failures separately from status/close cleanup failures; neither can overwrite the other or be accepted as a pass.
6. Correlate application/daemon ownership and final cleanup (container/process/ports/DBconnections) with the run. The discovery-only fakeCLI rejects real execution; do not imply commercial-model acceptance or broaden the fix to unrelated platforms.
7. Final classification: fixedvariantPASS can close the defect and support narrowly integrating its product patch; original frozen1a32 failure remains historicalFAIL. P95 measured2155.025ms is already a user-accepted deviation and must not be rerun for this repair.
