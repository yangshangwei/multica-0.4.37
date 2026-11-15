# Current-candidate runtime preparation

All seven fixtures compiled successfully from candidate `1a32ca36c2c4f4455a01bdd39d0e0ab4db20cccc`; tracked server/fixture source hashes match before and after. Commands ran sequentially with GOMAXPROCS=2, CGO_ENABLED=0, isolated HOME/TMP and explicit Go caches. No test binary, Provider, database, container or service was started. This proves preparation only.

| Artifact | Target | Consumer |
|---|---|---|
| native-daemon | darwin/arm64 | Host runtime/squad/switch/memory/knowledge fixtures |
| host-provider | darwin/arm64 | Host and deployed-knowledge physical-input observations |
| memory-provider | darwin/arm64 | Dedicated memory_input observer |
| password-server | darwin/arm64 | Password recovery / platform-admin initialization |
| linux-arm64-daemon | linux/arm64, static | Two contained namespace pollers |
| linux-arm64-cross-provider | linux/arm64, static | Cross-client observed contained execution |
| linux-amd64-cli-relay | linux/amd64, static | Pinned v0.6.7 installed CLI compatibility |

Absolute binary paths, SHA-256, sizes, architecture, compile commands, exits and per-lane environment mappings are in `runtime-preparation.json`. Per-command raw logs and build metadata are in `evidence/runtime-preparation/`; binary files are in the private `runtime/bin/` directory.

## Existing candidate CLI identity

The built CLI and bundled CLI have different raw hashes because `apps/desktop/scripts/bundle-cli.mjs` ad-hoc signs the copied macOS binary. Removing both signatures alone does not produce equal files, so that observation is retained in `cli-identity.json` rather than asserted equal. Replaying the exact `codesign -s - --force` step on a private copy of the built CLI produced the bundled CLI hash byte-for-byte (`cli-signing-replay.json`), and Go build metadata is equal apart from the inspected path. Original CLI files were never modified. Use the actual bundled CLI path and pin its raw hash for Host acceptance.

## Runtime prerequisites owned by root

- Host lanes require password auth, enabled managed installations, a stable deployment UUID shared with the fixture, current production Web and actual Electron renderer/preload. Fixture gates must be opened/restored in the selected task DB. Supply all Host binary paths together; partial setup is a setup failure.
- Memory lane must select memory-provider; it has a different observation contract despite sharing fixture-kind marker.
- Knowledge lane must run distinct false/true API deployments, with matching COLLABORATION_KNOWLEDGE_EXPECTED. Approval UI alone does not prove physical input injection.
- Cross-client uses local redis:7-alpine arm64 image; both ELF binaries are arm64. The root must establish an actual numeric Docker-host callback address and allowed API reachability before execution. No prior-session callback address is assumed.
- Installed legacy CLI uses the fixture-pinned amd64 image and the amd64 relay; this is a compatibility case, not current CLI installation acceptance.
- Password recovery/admin-init actions need the compiled server but must receive only root-selected task configuration. A separate fresh admin-init DB is required by its suite.

## Detail-journey proposal — not applied

`runtime/detail-proposal/current-task-allocation.patch` changes only the old task/allocation selection inside `e2e/fixtures/collaboration-detail-journey.ts`. New explicit keys: COLLABORATION_DETAIL_TASK and COLLABORATION_DETAIL_ALLOCATION_FILE. The root-supplied provenance pins allocation_file and allocation_sha256. The allocation must contain task, source_root, api_origin, web_origin, renderer_origin and database. Existing source hashes, production Build ID, artifact hashes, actual listener PIDs, API config, rendered API/WS addresses, manual-only DB gates and business assertions remain unchanged.

The patch passes TypeScript syntax transpilation and git apply --check on the frozen checkout. It has not been applied or runtime-tested. Required negative cases are captured in the private proposal.json: missing/malformed task or allocation, relative allocation path, wrong task/source, changed file/hash, and original downstream identity failures. Apply only after original baseline evidence, then review and run these contract cases plus the two real browser/native journeys.
