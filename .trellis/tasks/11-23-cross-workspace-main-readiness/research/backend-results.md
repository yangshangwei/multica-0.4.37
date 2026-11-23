# Backend merge-readiness verification

Status: completed for the native macOS backend scope. All 86 default packages have a latest passing execution. Original failed/interrupted attempts remain retained; this is reconciled per-package evidence, not a claim that one unchanged wrapper invocation passed end to end.

## Result

| Check | Result |
| --- | --- |
| Go build and fresh PostgreSQL migrations | Passed, Go1.26.6; migration ledger through889 |
| `scripts/test-go.test.sh` | Passed |
| Default guarded race inventory | 86/86 packages latest pass; 25,564 named PASS /24 SKIP /0 FAIL |
| Top-level inventory | 11,741 PASS /23 SKIP /0 FAIL; all expected Test/Fuzz identities accounted for |
| Supplemental shared-password Redis race case | 1 PASS, recorded separately; original default skip remains visible |
| `go vet -p 2 ./...` | Passed with Go1.26.6; final test-only dbid delta separately vetted |
| `go tool govulncheck ./...`, Go1.26.11 | Exit0, zero reachable vulnerabilities /zero imported-package findings; seven uncalled module-level advisories |
| Final source/inventory audit | Passed; only approved dbid test changed after initial binding |
| Owned PG/Redis cleanup | Zero other clients before stop; exact containers stopped, both ports closed |

The router package passed 507 top /1,203 named tests with eight skips. Handler passed 3,140 top /6,170 named tests with seven skips. Service passed 1,888 top /4,613 named tests with no skips. Agent passed 1,232 top /1,997 named tests with three top /four named skips. Final dbid passed five top /35 named tests, including the original failed guard and its new11-case allowance matrix.

## Source and execution boundary

Initial candidate was `cb192014b2f27e9bb64112db52073cbb31ce8f04`; the parent advanced the private checkout through `73a28ed90fdcfd9eda1ac5d2970df5af2c486f94` while integrating unrelated frontend changes. Every production server and guard-script hash remained identical to the initial manifest. Four parent-owned generated docs JSON repairs were already included in that initial binding.

Only `server/pkg/dbid/writes_test.go` changed during verification, from SHA256 `1ce8f397b15abd5d9a8660e941a6fd414d9ce33f7f301239656b3019c305a0f6` to `71b2ed4d293ef1872a0df052821480b9d897887a7d20a9b0db5515fe19b74360`. The owning package received a fresh guarded race run and vet after the reviewer froze those bytes. Its final hashes matched before/after execution.

Tests/build/vet used Go1.26.6 darwin/arm64, GOMAXPROCS=2. Vulnerability scanning additionally used task-selected Go1.26.11, matching CI's latest1.26 patch policy. It does not imply tests were rerun under1.26.11.

PostgreSQL used the native Linux/arm64 immutable image `215ae4d47393a980e9ef7ff7760f7dad53ec3e84bceb9fbd45cef34e50b5ba0f`, with a1GiB memory limit and an isolated Redis128MiB container. Each top-level package received a fresh migrated database clone and private HOME; shared Go caches were explicitly pinned. No root `.env`, ordinary service database or installed real-agent CLI was used.

The repository wrapper and Agent CLI guard remained unchanged. A task-only Go launcher admitted the wrapper's fixed argument grammar, serialized packages, added `-json -count=1`, and allocated the per-package database/HOME. The failed non-agent wrapper branch prevented its normal agent tail, so that exact guarded agent branch was run separately. Replays retain the original package invocation and use a new database/environment.

## Findings and corrections

1. **Default cached image architecture.** The local `pgvector/pgvector:pg17` tag resolved to amd64. The initial owned allocation was stopped; its tests were interrupted during early command packages. That attempt is retained under `initial-amd64/` and contributes no accepted tests. Native acceptance used a new allocation.
2. **Secure artifact fixture roots.** The first native TMPDIR was below user-owned `/Volumes/artisan` mode0775. Product artifact guards correctly rejected its writable ancestor, causing80 router and10 artifact top-level failures. No product guard or shared directory permissions changed. Full router/artifact replays on new databases passed after selecting a verified canonical external temp chain. Subsequent tests pinned external HOME/TMPDIR/TMP/TEMP/GOTMPDIR, with actual child configuration independently read back. Original failures remain under `bad-temp-packages/`.
3. **Test-process umask.** The private launcher inherited umask077 into Go tests, causing one storage mode assertion to see0600 instead of0644. Package children now use022; credentials/logs remain0600 and owned directories0700. The entire storage package replay passed on a fresh database. Original failure remains under `umask077-packages/`.
4. **UUID AST guard contract.** `TestEveryV7ParamsLiteralSetsAnIDAndImportsDBID` incorrectly required a dbid import at two durable inbox delivery sites using persisted `row.ID`. The parent-assigned reviewer repaired only the AST test with a narrow provenance allowance and negative controls; product IDs were preserved. The original failure remains under `dbid-original-failure/`; final whole-package race/vet passed.
5. **Old toolchain vulnerability scan.** Go1.26.6 reported13 reachable standard-library advisories fixed in1.26.9–1.26.11. The1.26.11 scan passed with zero reachable vulnerabilities. Both outputs are retained; the minimum-toolchain failure is not mislabeled as a current-CI dependency defect.

## Coverage limits

Twenty-four default named skips remain explicit in `skip-inventory.json`: Linux-only native route/runtime cases, Windows-only/root behavior, dedicated stage/browser fixture modes, optional performance/public-network/pg_bigm scenarios and existing disabled regressions. One shared-password Redis opt-in skip received a separate successful run; the default inventory was not rewritten. Real-agent `agentintegration`, Windows host tests and external account execution were excluded. Native macOS race verification does not establish Linux/Windows release acceptance.

The declaration inventory and actual JSON events were reconciled per package. No missing top-level Test identity, unfinished/duplicate named terminal, malformed event or build failure remains in the accepted package set. The added dbid test inventory was refreshed after its repair. Earlier valid package passes retain their original environment descriptions rather than being relabeled as executions under later environment corrections.

## Evidence and cleanup

Raw/private evidence remains outside Git at `.omx/private/qa-work/main-readiness-20261123/backend/`:

- `source-initial.json`, `final-audit-cleanup.json`: source, allowed delta, inventory and resource identity.
- `identity.json`, `commands*.json`: runtime/toolchain and original command attempts.
- `packages/*/{receipt.json,test.jsonl,stderr.log}`: accepted latest package executions.
- `expected-tests.json`, `test-summary.json`, `skip-inventory.json`: complete machine inventory.
- `dbid-original-failure/`, `bad-temp-packages/`, `umask077-packages/`, `initial-amd64/`: immutable failed attempts.
- `dbid-final-command.json`, `logs/dbid-final-vet.log`: final guarded repair acceptance.
- `redis-auth-supplement-command.json`, `logs/redis-auth-supplement.jsonl`: supplemental Redis case.
- `vulnerability-current12611-command.json`, `logs/vulnerability-current12611.log`: current-patch scan.
- `run_backend.py`, `bin/go`, `summarize.py`: task-only runner/adapter/accounting scripts.

At20:38 CST, after verifying exact container IDs/owner labels and zero other PostgreSQL clients, the final owned PostgreSQL and Redis containers were stopped. Ports54733 and54738 are closed. Original setup containers are also stopped. Databases, container records and private logs were retained for inspection; nothing was force-disconnected, pruned or removed from another session.
