# A2 independent service review — 2026-11-01

**Verdict: no confirmed P1/P2 finding in the bounded A2 scope.** This is an independent read-only review of the current human service, projections, source availability, credentials, resolver/lock seam and generated serialization boundary. It is not a product enablement or full integration verdict.

The review covered A2's 27 exclusive files and three shared-file changes, the approved PRD/design/implementation contracts, CLAUDE.md, server security/source conventions and the F1–F4 lock contract. Current source hashes, retained evidence hashes and the exact review bounds are recorded in `service-review-2026-11-01.json`.

## Code-backed conclusions

| Boundary | Result of independent challenge |
| --- | --- |
| Human authority | `lockKnowledgeWrite` rechecks current source Read and creator/current-handler identity after source locks. Completed issues and source-member creators without participant rows remain eligible. Source workspace/current-project proposals permit lawful external handlers; unrelated destinations require normal membership visibility. Human confirmation remains source Read plus target membership and owner/admin or human project lead. |
| Replay and recovery | Writes share the existing source/actor/request operation namespace and bind kind plus canonical payload. Original authorized refusals remain sealed. Unknown commit errors leave the original identity recoverable rather than automatically replaying. Recovery resolves source internally from candidate ID and returns only the authenticated original actor's matching action/key/candidate receipt. |
| Version and publication | Approval validates exact state revision, immutable version and content hash. The source/lineage/counter fences plus SQL CAS prevent an older pending correction from replacing a newer approved version. Approval/supersede/withdraw update the independent target revision and one operation-correlated hint atomically; pending changes do not signal the target. Complete target budget accounting runs before source visibility filtering. |
| Locks and deletion | Workspace/project existence uses KEY SHARE NOWAIT; ordinary owner/admin approval avoids an unnecessary project SHARE lock. Human lead authorization retains SHARE. Missing counter initialization refuses published history and uses the documented 25 ms conflict bound. Source-authorized withdrawal can continue after target deletion without recreating its target row; target-only withdrawal exposes no source material. |
| Material privacy | `KnowledgeSourceAvailability` validates every required source's original identity/version/hash, including memory purge, comment tombstones and exact run-record identities. The discriminated marshaler omits body, summary, hashes, refs, applicability and quoted decision reason when unavailable. Restricted rows expose only candidate control identity/state/revision/withdrawal capability. History checks each version; ordinary withdrawal/expiry retains otherwise lawful human history. |
| PAT freshness | Auth records the original verified PAT hash on cache-hit, legacy DB and password paths with `json:"-"`. Knowledge reads/recovery recheck that token's live owner/revocation/expiry; writes lock it and recheck freshness. Read leases clamp to its expiry. The shared fresh-PAT helper is called by `ResolveIssueAccess`, covering existing memory/comment/run readers. |
| Runtime seam | Resolver inputs are server-only facts with distinct originator/runtime owner and no Accountable fallback. Private Chat uses the real creator and existing private-agent read semantics; same-workspace proof is required for workspace-visible output. Resolver/revalidator borrow the transaction and open no hidden transaction. Exact invalid history is refused, never substituted. |
| Discovery versus disclosure | Fresh metadata discovery may omit missing sources. `ValidateKnowledgeResolutionLocks` requires every returned candidate/manifest entry to belong to the acquired candidate set and match the returned contribution identity; a source becoming available after discovery cannot silently disclose an unlocked body. Candidate content and source refs are immutable. C remains responsible for the documented parent/user/member/consumer/credential and late runtime/task fences. |
| Paging and serialization | Source paging retains every version and the first-page lease; detail explicitly marks truncation. Every read snapshot reauthorizes. Target SQL filters hidden rows before LIMIT and omits restricted-source expiry from the page lease. The wildcard sqlc overrides cover base and CTE-derived task metadata; current generated declarations remain `json:"-"`. |

## Derivation interpretation

The reviewed contract makes `derived_from_candidate_id` project-to-workspace provenance for a new, independently reviewed lineage. Creation requires the origin to be approved, from the same source and currently material-readable; SQL also enforces project → workspace. The new proposal's `sources` remain its required immutable dependencies, and protocol validation requires every evidence anchor in that array. I found no bypass through the provenance field under these semantics. I did not impose an undocumented automatic state cascade or transitive inherited dependency set; withdrawal/supersede/expiry of an origin is not equivalent to purging a required source.

## Retained evidence audit

I parsed the retained JSONL and reconstructed its pass inventory rather than accepting the handoff totals alone:

| Retained run | Top-level tests | Subtests | Missing inventory entries / skips / failures |
| --- | ---: | ---: | --- |
| A2 owned | 44 | 40 | 0 / 0 / 0 |
| A2 service with race detector | 32 | 18 | 0 / 0 / 0 |
| Related source service/handler/router | 59 | 102 | 0 / 0 / 0 |

The inventories match the retained outputs exactly. The six actual HTTP fixture files and Core schema success log were inspected. The retained `go vet ./...`, `go build ./...` and sqlc 1.31.1 evidence report exit 0; sqlc records 126 generated files with zero drift. Historical red evidence for the CTE tag leak and owner/project lock contention is retained, followed by passing final runs.

Both shared generated A2 snapshot hashes still match. The router changed during review only by C4’s three runtime knowledge status/prepare/authorize registrations; removing those lines reproduces the retained router hash exactly, and all nine A2 human/target routes remain unchanged. Of 27 exclusive files, 26 match directly; `collaboration.go` matches exactly after removing only C4's later default-closed `KnowledgeContextEnabled` comment and field. Current C2 protocol/query/generated dependency changes are tracked separately, not treated as A2 drift. The manifest alias field order and current generated metadata tags were inspected.

## Scope and remaining verification

This review changed only `research/service-review-2026-11-01.md` and `.json`; no product simplification or edit was made. No verification runner, tests, database operation, sqlc generation, Provider or commit was executed. The test/static results above are audited retained evidence, not fresh runs by this reviewer.

Final C2/C4 integration, mandatory late authority fences, F3/B2 review ownership, G2 enablement, UI journeys and real-model/platform acceptance remain with their existing owners. No absent downstream gate is reported as an A2 defect. Final integrated verification must account for C2/C4 changes made after A2's static evidence snapshot.
