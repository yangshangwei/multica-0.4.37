# Independent initialization kernel implementation

Date: 2026-11-27. Scope: `server/internal/materialrepo/**` only; no product wiring, credentials, remote writes, DB changes or commits.

## Delivered API

- `BuildCatalog(Binding) (Catalog,error)`: exactly20 deterministic base files, selected root, stable system ID substitution, per-file and catalog SHA-256. No timestamps, random IDs, fake product versions or empty directories.
- `ParseManifest(string) (Manifest,error)`: strict one-document YAML, known format/bundle/layout, stable ID and exact scalar types. Equivalent formatting/key order can be preserved. Duplicate fields, aliases/anchors, custom tags, unknown keys/versions, deep/oversized/NUL/invalid UTF-8 refuse.
- `Inspect(Binding,Snapshot) (Inspection,error)`: requires complete fixed-commit tree; verifies explicit ancestors, file modes, Unicode case-fold collisions, full managed bodies and hashes. Existing README and unmanaged data preserved. Known managed content accepted; custom AGENTS/templates/schemas conflict with no partial executable candidate.
- `ValidatePublication(Candidate,expectedHead,Publication) error`: frozen base, exact head, complete create-only allowed diff, and exact resulting tree. No approval/repository authority is inferred.
- `VerifyMerged(Candidate,actualMergeCommit,Snapshot) (Proof,error)`: verifies actual commit and every new or retained file/mode/hash/body against the frozen tree. Requires independently authenticated caller observation.
- `VerifyExisting(Binding,Snapshot) (Proof,error)`: no-op compatible repository proof without a manufactured PR or approval.

`Candidate` embeds a deep-copied canonical original snapshot. Verification recomputes the expected catalog/candidate instead of trusting arbitrary supplied file lists. Candidate digest must still be pinned by the owning authenticated operation: a pure hash is not a signature or proof that an input came from Git.

## File ownership

Five production Go files: `types.go`, `catalog.go`, `manifest.go`, `inspect.go`, `verify.go`.
Three test files: `catalog_test.go`, `manifest_test.go`, `inspect_test.go`.
`assets/` contains3 entry files,6 schemas and11 templates. Package README documents API/trust boundaries/limits. This report is the sole task artifact edited by this implementation lane.

## Verification actually run

- First red test: `go -C server test ./internal/materialrepo -count=1` failed at undefined catalog API before implementation.
- Strict manifest tests caught yaml.v3 accepting numeric IDs and custom tags; explicit node-type checks were added and these tests passed.
- `go -C server test ./internal/materialrepo -count=1 -cover`: passed,90.6% statement coverage.
- `go -C server test ./internal/materialrepo -race -count=1`: passed.
- `go -C server vet ./internal/materialrepo`: passed.
- Final normal suite after import formatting: passed.
- `git diff --check`: passed (untracked new files additionally gofmt formatted).

Meaningful assertions cover20-file empty-root output,19-file README-preserving plan, no-op reinspection, nested-root setup, retained uat bytes, deep-copy invariance, opaque unmanaged file digest retention, strict YAML, local schema refs, complete tree limits, symlink/submodule/executable/collision/path refusal, forged candidate bytes, incomplete/foreign/renamed/overwriting/duplicate diff, changed/missing retained data, missing required body and wrong commit.

## Remaining integration and limits

- No runtime queue/API/SQL/UI/human-review integration exists in this package; task must remain incomplete pending the dependency runtime contract.
- Existing custom AGENTS/templates/schema adoption and supported prior bundle upgrades are not implemented. They preserve content and report conflict.
- Only modes100644/040000 accepted; unsupported modes anywhere in the complete repository tree refuse, even if unmanaged. This is a conservative initial compatibility limit.
- Full exact-tree verification also refuses unrelated concurrent target additions/changes. Reinspection is required; no silent scope enlargement.
- Complete tree has at most10000 entries and32MiB supplied data; each supplied body at most1MiB. Managed bodies must be UTF-8; unmanaged bodies may be omitted with independently calculated SHA-256.
- Six JSON Schema files document formats and local references. No schema engine was added; this package does not claim validation of future release/issue business instances.
- No external APIs/accounts, real repository initialization, product task state or production tests were exercised.

## Independent review correction

Reviewer identified that a valid1024-byte root could yield invalid joined file paths. Added a red boundary regression proving the longest catalog suffix crosses the limit, then validated each joined path in BuildCatalog. The exact1024-byte file path passes; one byte over refuses before returning a catalog. Final `go test ./internal/materialrepo -race -count=1` and `go vet ./internal/materialrepo` pass after correction.

## Final review corrections

- Projected tree capacity: added red regressions for an original snapshot that fits the limit but cannot fit the20 additions and directories. `Inspect` now validates the projected tree before returning a candidate; `verifyTree` shares the same `projectSnapshot` helper. Exact MaxEntries/MaxSnapshotBytes applied states verify, one-entry/one-byte overflow refuses before publication.
- Schema path patterns: a shared path corpus is exercised by Go `validPath` and an explicit dependency-free Node test using real ECMAScript `RegExp`. The initial Node regression caught `.GIT/config` acceptance. Managed reference/source schemas now reject case variants of `.git`, leading/trailing segment Unicode whitespace and C0/C1 controls consistently with the kernel. Internal spaces and Chinese paths remain valid. JSON Schema character-length versus Go byte-length limits remain enforced independently by the host.
- YAML template comments are now English; Markdown authoring text and placeholder explanations remain Chinese.
- Final checks: `node --test server/internal/materialrepo/schema-path.test.mjs` passed3 tests; full Go package race tests and `go vet` passed after these changes. No new dependencies or system-tool execution from Go tests were introduced.
