# v0.6.0 release acceptance

User-authorized scope: verify `codex/projects-p1` end to end, produce the Linux x86-64 offline server upgrade and Windows x64 desktop installer, deliver Chinese upgrade/configuration instructions, and push source and verified assets to `yangshangwei/multica-0.4.37` as v0.6.0.

## Acceptance

- Full `make check`: lint, typecheck, TS, Go race/vet, production Web E2E.
- Dedicated gated production E2E for password/admin and new projects/iterations, including desktop coverage; record real coverage and skips.
- Windows native installed lifecycle/business/update acceptance uses the same 0.6.0 bytes delivered.
- Linux amd64 runtime images and offline archive verified; rehearse upgrade with disposable data and preserve configuration/data.
- Chinese documentation covers actual configuration changes, feature gates, authentication preservation, backup, installation, verification and rollback.
- Immutable version/commit provenance, manifest and checksums accompany GitHub assets.
- Do not overwrite existing tags/assets or include unrelated initial dirty files.

## Initial evidence

- HEAD: 8ae60f71b; branch codex/projects-p1.
- Fork latest release v0.5.6; last full binary delivery v0.5.5; v0.6.0 absent.
- Root package versions are not release version sources; build scripts support version overrides and tagged provenance.
- Existing `make check` requires libpq bin on PATH and isolates databases/processes.
- Windows acceptance workflow hardcodes an old candidate version; deployment Compose omits new feature flag forwarding.
