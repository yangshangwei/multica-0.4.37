# Verification infrastructure findings

## Fixed: canonical project-update fixture ownership

- Changed `server/internal/handler/project_update_test.go` to read
  `testdata/project-update-canonical.json` rather than an active Trellis task.
- Added `server/internal/handler/testdata/project-update-canonical.json`, preserving
  all 1,578 source bytes. Its introducing commit is
  `bb1c20dcb16751a64931f2321a05b8773cec3aaa`; task archival occurred in
  `ff6bb36e3c5c9fd7cb8094f2c67652d09d7b2a33`.
- Recomputed canonical JSON using Python standard-library sorted keys, unescaped
  UTF-8 and compact separators; both the stored canonical string and SHA-256 match:
  `31ea518dfd894066d143943fd89e156363086d710a27faf29b04a20967274e08`.
- Exact file SHA-256 is
  `d2f06b1c566b9b1913b1d2751ea205bfc6bce1af3eb5ccb5cda24a0a860d2c41`.
  Original, archived and test-owned bytes all match.
- No other live consumer of this canonical fixture exists. The archived source
  remains historical evidence; no active task, symlink, fallback or replacement
  digest was created. All digest/normalization/URL assertions remain unchanged.
- Evidence: `workbench-check-canonical-provenance.json`,
  `workbench-check-canonical-before.log` (expected missing-path failure),
  `workbench-check-canonical-after.log` (pass).

## Not changed: existing Knip findings

The root-owned scoped run finished with exit 1. Analysis of
`knip-affected-dependencies.json` found **8 file reports, 4 binary reports, zero
dependencies, devDependencies, unlisted imports or unresolved imports**.

All eight reported files are byte-identical to HEAD
`bd168d18edb37829203cd97c980d3e36895cce64`:

- `apps/desktop/scripts/verify-renderer-file-access.mjs`
- `apps/desktop/scripts/windows-business.mjs`
- `apps/web/test/helpers.tsx`
- `packages/views/labels/index.ts`
- `packages/views/chat/components/quick-agent-bar.tsx`
- `packages/views/agents/components/inspector/concurrency-picker.tsx`
- `packages/views/agents/components/inspector/skill-attach.tsx`
- `packages/views/agents/components/inspector/visibility-picker.tsx`

The native file-access script is an explicit manual acceptance entrypoint.
`windows-business.mjs` is invoked from `accept-windows-installer.ps1`; that
reference is unchanged from HEAD. The other inspector/chat references are
unchanged as well. No task change removed a live importer.

Binary reports are `eslint` in Desktop/Web/core package scripts and system `go`
in `apps/desktop/scripts/bundle-cli.mjs`. All corresponding script and dependency
sections are unchanged; core's manifest changes only add collaboration/account
exports. No installed system binary was converted into an npm dependency.

The repository-wide `knip.log` retains its old release-snapshot import noise plus
unrelated docs/plugin findings (`mermaid` and `@multica/eslint-config`). Those
packages have no task diff. The release snapshots, baseline files/dependencies
and Knip configuration were left intact. No additional Knip process was started.
Detailed comparisons: `workbench-check-knip-baseline.json`.

## Verification

With `GOTOOLCHAIN=go1.26.6`, the repository agent-CLI guard and the isolated
`collaboration_switch` database, the unchanged
`TestProjectUpdateCanonicalFixtureAndNormalization` passes under `go test -race
-count=1 -p 1 -timeout=3m ./internal/handler -run
'^TestProjectUpdateCanonicalFixtureAndNormalization$' -v` (one test, no skips;
package time 2.496 s). Handler `go vet`, gofmt and whitespace checks pass.
No TypeScript or business runtime source changed; their prior verification and
production Web evidence remain applicable.
