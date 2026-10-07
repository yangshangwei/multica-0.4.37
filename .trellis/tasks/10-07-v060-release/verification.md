# v0.6.0 verification ledger (in progress)

Repository: yangshangwei/multica-0.4.37. Branch: codex/projects-p1.
Local evidence root: `.artifacts/v0.6.0/` (ignored; private fixture credentials
must never be uploaded). Prior complete binary release: v0.5.5.

## Completed baseline

- `PATH=/opt/homebrew/opt/libpq/bin:$PATH make check`: exit 0.
- API/Web source HEAD at launch: `8ae60f71b`; production Web, one worker,
  zero retries, isolated API and Go databases.
- Static checks: 15 tasks passed; UI exports passed.
- TypeScript: core 2746, views 6197, docs 62, web 282, desktop 962;
  total **10249 tests passed**.
- Go race suites and `go vet -p 2 ./...`: passed; agent CLI guard active.
- Production Playwright: **247 passed, 54 skipped, zero failed** (301 total).
  The skips are not accepted as passes: planned supplemental lanes cover
  25 legacy/provider/Desktop/I1, 28 password/admin, one device-auth case.
- `go -C server tool govulncheck ./...`: no vulnerabilities found.
- Production Desktop build: passed.
- Release/changelog/Windows-candidate tests with Docker smoke enabled:
  **73 passed, zero skipped** before the final overlay image-identity guard.
- Selfhost config and shell syntax checks: passed.

Baseline log: `make-check.log`. Provenance retained at
`~/.multica/dev/envs/check-20261007152623-4598/`.
API/Web stopped and isolated Go database dropped by the completed check.

## In progress / still required

- Final upgrade image-identity guard and fresh targeted regression results.
- All 54 supplemental E2E identities, with fresh source and process provenance.
- Windows hosted native installation/business/upgrade/update acceptance of
  the exact 0.6.0 artifact; checksum-preserved publication.
- Linux amd64 build, archive checks and actual disposable upgrade rehearsal.
  Old v0.5.5 images are running in Docker amd64 emulation on this ARM Mac;
  health and password config passed, a synthetic user/workspace/task and an
  upload sentinel are retained, migration baseline is 511.
- Fork v0.6.0 tag/release, immutable changelog, checksum/download verification.

No completion claim is made by this intermediate ledger.
