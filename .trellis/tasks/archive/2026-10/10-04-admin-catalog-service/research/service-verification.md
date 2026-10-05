# Service implementation verification

Owned changes: `server/internal/service/resource_{types,validation,store,catalog,lock_unix,lock_windows,lock_other}.go`; validation/store/catalog/process tests; narrow `task.go`, `skill_template_dir.go`, `skill_template_dir_test.go`, `mcp_catalog.go` integration. Parent-approved scope extension: private `mcp_template_dir.go` scan snapshot carries parsed recipes, all original directory names, and consumed byte budget.

## Implemented invariants

- Strict raw SKILL.md, ZIP/.skill and MCP parsing, no partial acceptance. Portable paths, case/prefix aliases, symlinks/special files, duplicate manifests, malformed UTF-8/NUL and declared limits are rejected.
- Immutable revision bundles under a confined root; mutation UUID separate from canonical content digest. One synced, atomic index replacement commits both resource row and independent durable receipt.
- Organization binding, actor-scoped lookup, payload-bound idempotency, revision CAS, replay reauthorization, bounded cross-process lock wait and explicit unknown outcome after commit.
- Consumer reads preserve captured immutable index snapshots. Manual collisions and MCP byte/count bounds check both the exact retained manual scan and live reservations; invalid manual entries still reserve identities.
- Unset publishing keeps existing catalogs. Configured corruption and consumer failures propagate explicit errors. Managed MCP recipes remain absent from public list previews.

## Evidence

- `go test -race ./internal/service -run 'TestResource|TestSkillTemplates|TestMcpCatalog|TestMcpTemplateDir' -count=1` passed (7.553 s).
- `go vet ./internal/service` passed.
- `GOOS=windows GOARCH=amd64 go test -c ./internal/service -o /tmp/multica-resource-service-windows.test.exe` passed.
- Additional combined MCP snapshot entry/byte boundary matrix passed after final race run; production code unchanged.
- Tests include concurrent writers, concurrent readers during atomic index replacement, two-process lock contention, process exit immediately after filesystem commit, original receipt recovery after later mutations, audit-finalize failure, rejected replay authorization, scope mismatch, corrupt/ambiguous index, revision tampering, symlinks, invalid manual collisions, snapshot disappearance races, archive per-file/supporting/count/directory-entry limits, and MCP consumer List/Resolve.

## Remaining limits

Windows compilation is verified, but Windows directory-sync behavior is not runtime-tested. Writes fail closed if filesystem sync is unsupported. Production support requires a shared filesystem with working OS locks, atomic replacement, file sync and directory sync. Hardware power-loss and remote/network filesystem durability are not simulated. Root must be provisioned as a real existing directory; no content garbage collection or silent receipt eviction is introduced.

No commits or external publication performed by this lane.
