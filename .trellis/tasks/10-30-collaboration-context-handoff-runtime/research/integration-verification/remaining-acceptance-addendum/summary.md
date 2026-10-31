# Remaining deterministic acceptance matrices

Verified on 2026-11-01 against source HEAD `0bbc6a77a1a727b9da0de94f6f3356ecc8e8cbce` plus exactly the three new test files listed in `source-sha256.txt`. No production code changed. The main session independently reviewed all three files; its requested subtest-helper correction is included in the final handler evidence.

## Gaps closed

| Parent criterion | Added evidence | Observable result |
| --- | --- | --- |
| AC1: different squad attachment budgets | `TestCollaborationSquadPreparationBudgetMatrix` | A leader and two actual registered member runtimes have different limits. Each of file count, per-file bytes, total-input bytes, and workspace bytes after Git reservation has a successful optional-history case and a mandatory-reference refusal. Whole-roster admission rejects the mandatory overflow before any run/group/reservation; the original input remains waiting. Successful member claims retain the required reference, their own limits and exact optional selection; preparation and attachment authorization match each member's frozen manifest, including replay. |
| AC5: conflict followed by explicit refresh/merge | `TestCollaborationMemoryConflictRefreshMergeRetainsBothFacts` | Two human participants observe version 1 and submit concurrent distinct facts. Exactly one version 2 commits; the other intent receives a sealed stale refusal. The loser rereads version 2, cannot repurpose its refused UUID, and submits the merged replacement under a new UUID. Version 3 retains both facts once; versions 1 and 2 remain byte-for-byte unchanged as database rows. Authorized current/history reads preserve each version's real author, content, timestamp and version; original refusal and successful merge replay without rewriting history. |
| AC7 / runtime AC7: private runtime credential classes | `TestCollaborationRuntimeCredentialMatrix` | Eight new endpoints each reject an issued ordinary task token, plugin installation token, plugin callback token, live Feishu/Lark binding token, and the opposite-purpose run capability: 40 named negatives. Each refusal is no-store, emits no private sentinel/token and leaves the scoped runtime/content state unchanged. Seven matching business-capability controls succeed, including real private source/comment reads and checkpoint ingestion. After grant revocation the original termination credential reads Stop metadata and seals the retained prefix, rejects body/messages/content fields, and does not release the unproven reservation. |

The credential test mounts the **actual handlers on local Chi routes matching the production method/path boundary**. It does not instantiate the entire `cmd/server` router. Production `router.go` registers these capability-authenticated routes outside ordinary `Auth`; the test also supplies an ambient authenticated account context to prove it cannot replace the bearer capability. IM has no first-class collaboration run credential: this matrix uses a live locally issued Feishu/Lark binding token, with no commercial IM service access.

The host fixture settles its original legacy run, explicitly pauses the target, stages/consents/commits the production configuration Switch, then submits, claims and permits a new host run. It does not relabel a historical run or start a provider. The synthetic prior proof is a service prerequisite, not evidence of a real process.

## Verification

Exclusive database: `multica_runtime_final2_858f011e`, PostgreSQL 17.11. Host: macOS 26.2 arm64; toolchain: Go 1.27.0 darwin/arm64. Credentials were loaded from the existing private environment file and are absent from the retained evidence.

| Check | Exact command / record | Exit | Results |
| --- | --- | --- | --- |
| Focused service race suite | `go -C server test ./internal/service -run '^TestCollaboration(SquadPreparationBudgetMatrix\|MemoryConflictRefreshMergeRetainsBothFacts)$' -count=1 -timeout 10m -race -json` | 0 | 14 test pass events, 9 leaf scenarios, no failures/skips |
| Final handler suite after review | `go -C server test ./internal/handler -run '^TestCollaborationRuntimeCredentialMatrix$' -count=1 -timeout 10m -json` | 0 | 57 test pass events, 48 leaf scenarios, no failures/skips |
| Relevant static analysis, including review follow-up | `go -C server vet ./internal/service ./internal/handler` | 0 | No diagnostics |
| Formatting, including review follow-up | `gofmt -l` on all three owned test files | 0 | No output |
| Working diff whitespace | `git diff --check` | 0 | No output |

The service and handler database suites ran sequentially. `commands.json` and `review-commands.json` retain executable argument arrays, UTC start/end timestamps, exit codes and complete test inventories. Their nonempty/no-skip gates require every run event to have a passing result. `test-inventory.txt` contains the 71 accepted named pass events (57 leaf scenarios); counts include the matrix container tests separately from leaves.

Authoritative logs: `service-final.jsonl` / `.log`, `review-handler-final.jsonl` / `.log`, `review-vet.log` / `.stderr`, and `review-gofmt.log`. The earlier successful handler gate remains in `handler-final.*`. `source-before.json` / `source-after.json` and the `review-source-*` manifests hash the complete server Go/SQL/module input set and prove unchanged source during both gates. The final review modified only the handler helper's `testing.T` attribution; the service files still match their race-run hashes. `source-sha256.txt` also hashes the final capture script and boundary plan.

## Retained failed attempts and review

- `handler-initial.*`: compile failure because the first fixture referenced a service test-only helper and an unavailable test token variable. No behavioral result was claimed. The fixture now uses the production staged Switch API and a deliberately unrelated cookie.
- `handler-round2.*`: fixture correctly refused an unpaused target with `target_pause_required`. Added the explicit scoped Stop before configuration Switch; no production assertion was relaxed.
- `handler-round3.*`: all handler cases passed before the final helper attribution corrections.
- The main session's independent review found no behavioral issue and requested that the DB snapshot helper receive the current subtest's `*testing.T`. The final `review-handler-final.*` and `review-vet.*` results follow that correction.

## Scope and remaining limits

Changed only the three assigned new Go test files and this evidence directory. Existing registration, namespace, upload, version, and handler fixtures were reused; no production abstraction, dependency, shared fixture, or migration was added. No commits were made by this agent.

These are service/database and handler-boundary acceptance results. They do not establish model understanding, real provider execution, complete production-router integration, commercial IM delivery, or native Web/Desktop behavior. The earlier full Go/TypeScript/browser gates were not rerun; remaining real-model and parent-task acceptance stays with the main session's ledger. No product defect was discovered in these matrices.
