# Local 639bf6ef8 artifact independent review

Verdict: **pass-with-notes**, no blocking artifact-preparation finding. This review binds candidate `639bf6ef85ad894553b244873c2ed8d9f9dd29b2`, product source ID `9f72fc214b8e81f6fc3c1969be5e3304b7f158f60a6f7305f1531d55574d80ee` and the actual artifacts recorded below. It does not mark final global acceptance complete.

The reviewer independently rehashed all 7,960 product files, reproduced the source ID, checked the tracked source trees and input hashes, and verified all 18 evidence-index records. The detached HEAD remains exact and tracked source is unchanged. Raw dependency/package/collector logs match their records, including the retained Mac schema failure, flattened-framework signing failure and initial collector Node-path failure. No historical failure was replaced by the later success.

## Actual artifact checks

| Artifact | Independent evidence |
| --- | --- |
| macOS arm64 APP | Complete 553-entry file/mode/symlink inventory and tree hash match; Info.plist matches version/identifier; actual bundled CLI hash and Mach-O arm64 header match the retained version evidence |
| Mac ZIP / DMG | Actual sizes, SHA-256 and SHA-512 match; every ZIP entry was read with CRC checking and its content or symlink target compared to the final APP; all paths safe; retained DMG verification log/hash matches exit 0 |
| Mac signature | Original deep/strict codesign verification and signature detail logs match their hashes; ad-hoc signature only |
| Canonical local collection | All five actual destination files match the source bytes; metadata SHA-512 references and sizes independently match; collect command and raw log confirm no publication |
| Linux arm64 directory | Complete 85-entry inventory/tree hash matches; actual Electron and bundled CLI ELF headers are aarch64; CLI build-info log/hash matches candidate/version; no Linux executable was run |

The final Mac CLI, ASAR, tree and archives still match after Linux packaging. The platform-specific CLI bytes differ as expected; runtime main/preload/renderer and LICENSE/NOTICE hashes match across platforms.

Both actual ASAR headers were parsed directly, with 52,484 entries each matching their retained full inventories. Runtime roots are only `node_modules`, `out`, `package.json` and `resources`; actual runtime byte hashes and package version match. No project `.env`, private agent/task/session directory or private tool result was found. The dependency file `node_modules/recharts/AGENTS.md` is ordinary vendored documentation, not project or private session material.

Cached Electron 39.8.7 archive hashes match. The corrected Mac input retains real framework symlinks from the canonical cached ZIP; this is a packaging-input repair and does not change production code, signing policy or network policy. The original installed flattened distribution may still serve a limited fixture use; it is not substituted for the canonical packaged framework layout.

All recorded package/build/collect commands use the exact deny-network sandbox profile. Its hash and retained local-execution/loopback-bind-denial preflight logs match. The package launcher strips account/signing credentials and uses offline caches, frozen dependencies and publish-never. No new dependency/tool, account, Provider, service or database work was used by the reviewer.

## Remaining boundaries

PK-1: Artifact preparation is accepted in this scope. Final full make-check and all required final-candidate gates remain separate; this report supplies no global pass.

PK-2: Mac signing is ad-hoc, with no Developer ID or notarization claim. Installation, upgrade, real GUI and account journeys remain to be performed against the pinned artifacts.

PK-3: Linux directory packaging and static build metadata do not prove native execution, `.deb`/RPM installation or Windows delivery. Missing packaging tools/base images keep those and the offline server image bundle open; no download authorization is inferred.

The reviewer used only read-only source/git/JSON/archive/hash inspection and wrote this directory. No build, test suite, GUI, DB, service, container, Agent/product CLI execution, Provider, network or git mutation was started.
