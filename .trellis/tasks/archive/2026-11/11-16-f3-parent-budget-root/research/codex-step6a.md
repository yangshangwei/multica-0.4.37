# Codex step 6a server usage

Code worktree: `/Volumes/artisan/code/2026/mc-f3-completion-20261120` only. No commits.
Original F3 worktree is untouched. Task design/plan/other reports are untouched.

## Implementation

- New `server/internal/service/issue_budget_usage.go`: `NewIssueBudgetUsageService`, `Report`, `Query`; one READ COMMITTED transaction with only the root row lock, replay before switch/ledger, one usage batch per attempt, frozen operation results, post-commit cache publication.
- New `issue_budget_usage_decode.go`: pure strict decoder and raw SHA-256. It walks the exact struct-tag schema before JSON unmarshal, rejecting unknown/case-variant/duplicate keys, missing/null/wrong types, invalid UTF-8/control characters, noncanonical decimal/UUID values, duplicate models/invocations, bad timestamps, bounds and unknown-outcome usage. Error paths use **zero-based** indexes.
- New `issue_budget_usage_normalize.go`: injected evidence table and price source. Production table is exactly `claude/1`, fresh proven, resumed unproven, reported-cost unproven. No provider calls. Four shortest decimal prices become exact integer rates. No floating-point arithmetic beyond `FormatFloat`.
- Authorized minimal extension of `issue_budget_ledger.go`: explicit attempt and optional daemon envelope/snapshot fields in the existing append input. Explicit attempt must equal a reservation's attempt when both exist. Existing callers preserve zero-value behavior. Daemon timestamps retain valid RFC3339 year-one values rather than treating zero time as absent.
- New `server/internal/handler/daemon_budget_usage.go`: `ReportBudgetAttemptUsage`, `GetBudgetAttemptUsage`; UUID → retained attempt → workspace → runtime → bounded body/decode → protocol → transaction. No task/issue lookup. Read errors remain 5xx. A private handler method accepts request time and the injected service for deterministic tests.
- New `server/pkg/db/queries/issue_budget_usage.sql`: unlocked attempt lookup and usage-batch count on the existing operation scope index prefix. Generated output `pkg/db/generated/issue_budget_usage.sql.go` produced by leader. No migration/index/dependency.
- New colocated tests: `issue_budget_usage_{decode,normalize,db,locks}_test.go` and `daemon_budget_usage_test.go`.
- Router wiring belongs to leader: POST `/api/daemon/budget/attempts/{attemptId}/usage` → `h.ReportBudgetAttemptUsage`; GET same plus `/{batchKey}` → `h.GetBudgetAttemptUsage`, inside DaemonAuth.

## Mapping and stored identities

| Source | Stored entry / row |
|---|---|
| Usage item | `usage/<attempt>/<invocation>/<model>`; delta, sequence 0; scope `<attempt>/<invocation>`; invocation column set |
| Empty returned/interrupted invocation | `gap/<attempt>/<invocation>`; coverage_gap; detail `no_usage:<invocation>` |
| Unknown invocation | same gap identity; detail `outcome_unknown:<invocation>` |
| Gap columns | invocation/scope/completeness NULL; attempt always retained |
| Reservation | attempt ID only when `ledger.HasReservation` is true; row attempt always present |
| Overlap | fresh/resumed corresponding evidence fact → none; otherwise unproven, quarantined scope_ambiguity |
| Supplied cost + reported evidence | reported USD ticks, no snapshot |
| Supplied cost without proof | estimated USD ticks, no snapshot; never re-estimated |
| Missing cost with valid rates/estimate | estimated ticks, price version/digest + exact snapshot bytes |
| No price/invalid rate/overflow | zero Cost (absent), no snapshot; never numeric zero-cost evidence |
| Envelope | daemon_report, adapter/version or adapter/unknown, normalizer, epoch, raw-payload digest, observed/reported/received timestamps, operation ID |
| Completeness | returned usage final; interrupted usage partial; no extra interrupted gap |

No correction authority, settlement or release proof is produced by the endpoint. Internal full-proof settlement is separately tested through the append helper. Token counts remain per entry.

Price-version canonical bytes: `budget-metrics-rates-v1\n` followed by compact JSON array `[input,output,cache_read,cache_write]`, each rate exactly `{"ticks":"…","per_tokens":"…"}`. `price_version = metrics:` + first 16 SHA-256 hex digits. Independent Python vector for prices 0.1,15,0.075,1.25 gives `metrics:efec33c3742ffc3e`. Model/provider labels do not affect this version. Snapshot digest uses the pure package's `budget-price-snapshot-v1\n` + snapshot JSON.

Operation intent uses design 11.4's five-key order/domain. Independent Python golden for workspace `11111111-1111-4111-8111-111111111111`, attempt `22222222-2222-4222-8222-222222222222`, payload digest 64 `a`s: `45030cf08af04f7ac9156836566909b2339e59b045c404d9192fb707755a2bb7`.

## Public and stored results

- First: `{"status":"recorded","replayed":false,"receipts":[{"entry_id":"usage/<attempt>/<invocation>/<model>","kind":"usage","disposition":"accepted","reason":"estimate_only"}]}`.
- Replay: same shape, original receipt fields, `replayed:true`. Empty recorded batch has `receipts:[]`.
- Off: `{"status":"not_recorded","reason":"observation_off"}`; no operation persisted.
- Query pending/absent: `{"status":"unknown","query_path":"/api/daemon/budget/attempts/<attempt>/usage/<batch-key>"}`. No restart permission inferred.
- Operation `result`: operation_id, operation_kind, budget_root_id, attempt_id, batch_key, payload_digest, ledger_revision (decimal string), full budget receipts, and invocations `{invocation_id,resumed,ended,evidence:{matched,adapter,normalizer_version,fresh_own_usage,resumed_own_usage,reported_cost}}`.
- Daemon response deliberately contains no amount, root ID, root total or ledger revision.

Transaction statements with a warm ledger: isolation declaration → LockIssueBudgetRoot → GetIssueBudgetOperationByKey → CountIssueBudgetUsageBatches → InsertIssueBudgetEntry (per entry) → UpdateIssueBudgetRootLedger → InsertIssueBudgetOperation → commit. Cold load additionally reads ListIssueBudgetEntryFormsAfter after the count. Empty batch writes only its operation. Query reads GetIssueBudgetOperationByKey only after handler authority reads, no transaction/locks/writes.

## Coverage

| Contract | Tests |
|---|---|
| Decoder/wire fixtures/bounds | DecodeFrozenFixtures, DecodeRejects, DecodeBoundaries, DecodeEveryRequiredFieldAndObjectKey, DecodeStringAndDecimalLimits |
| Production evidence & costs | Evidence, PriceSnapshot, NormalizeCostAndScope, ProductionEvidenceConsequences, NormalizeEnvelopeGapsAndFrozenPrice, LargestStoredEntry |
| B16 legacy/hourly independence | DaemonBudgetUsageHTTPReceiptsAndLegacyIndependence |
| B24 frozen price | NormalizeEnvelopeGapsAndFrozenPrice; DBEnvelopeGapsAndFrozenPrices |
| B31 strict/aggregate atomicity | DBAtomicInvalidAndProtocol; DaemonBudgetUsageAtomicRefusalVersusLegacy |
| B32 replay, conflicts, off/corrupt replay | DBReplayConflictsAndSwitch; DBOffAndUnavailableRemainRetryable; DaemonBudgetUsageHTTPStateAndErrorMapping |
| B33 pending/committed query | DBStatementsAndUnknownQuery; DBPendingQueryDoesNotLock |
| B42 late states, hold/revocation | DBLateClosedDeletedSuperseded; DaemonBudgetUsageLateHoldAndRevokedWorkspace |
| B49 gaps / unknown | DBEnvelopeGapsAndFrozenPrices; NormalizeEnvelopeGapsAndFrozenPrice |
| B53 protocol | DBAtomicInvalidAndProtocol; DaemonBudgetUsageAuthorizationAndValidationOrder |
| Daemon authority/errors | DaemonBudgetUsageRuntimeRemovalAndDatabaseErrors; DaemonBudgetUsageTaskCredentialRejectedByDaemonAuth |
| Same-batch race | DBConcurrentDuplicate |
| Usage/finalize and switch | DBConcurrentFinalize; DBConcurrentSwitch |
| Empty and missing reservation | DBEmptyAndUnreserved |
| Settlement storage, explicit attempt | DBInternalSettlementAndAttemptConsistency |
| Invalid first-delivery replay invariant | DBUnexpectedEntryReplayRollsBack |

Test names above use `TestIssueBudgetUsage` prefix unless `DaemonBudgetUsage` is given (`Test` prefix). No existing tests were weakened or edited.

## Verification and retained failures

Commands run with explicit DATABASE_URL. DB tests only through leader's identity-verifying runner `/tmp/f3-codex-20261120/run-db.py`, keys usage_service / usage_handler. It uses private JWT, agent CLI guard, package permits, and a serialized DB lock. No DB created/migrated/dropped by this agent.

| Run | Result |
|---|---|
| `/tmp/f3-codex-6a/decoder-red.log` | Expected compile failure before implementation |
| `pure-01.log`, `pure-02.log`, `pure-03.log` | New pure tests passed; unreachable explicit DB, no DB fixtures selected |
| `cx6a-svc-01.jsonl` | `f3_budget_cx_usage_service`, Go 1.27.0, exit 0, 83 pass / 0 fail / 0 skip |
| `cx6a-hdl-01.jsonl` | Handler fixture failure: runtime referenced by agent; 9 pass / 1 fail; fixed fixture unbind before runtime delete |
| `cx6a-svc-02.jsonl` | Concurrent sibling test unused-import compile failure; product execution did not start |
| `cx6a-hdl-02.jsonl` | `f3_budget_cx_usage_handler`, Go 1.27.0, exit 0, 10 pass / 0 fail / 0 skip |
| `cx6a-svc-03.jsonl` | `f3_budget_cx_usage_service`, Go 1.27.0, exit 0, 91 pass / 0 fail / 0 skip |
| `build.log`, `vet.log` | `go build ./...`; `go vet ./internal/service ./internal/handler`, exit 0 |
| `budget.log` | `go test ./internal/budget -count=1 -v`, 112 pass / 0 fail / 0 skip |
| `pure-04.log` | During incoming 5c integration, older tests used prior binding signature; coordinator updated callsites |
| `pure-05.log` | Go 1.27.0, 110 pass / 0 fail / 0 skip, including expanded strict matrix and immutable-ledger guard |
| `pure-go126.log` | Concurrent incoming 5c attribution test unused-import compile failure; no product execution |
| `pure-go126-02.log` | Go 1.26.9, 110 pass / 0 fail / 0 skip |
| `budget-go126.log` | Go 1.26.9, 112 pass / 0 fail / 0 skip |
| `max-entry.log` | Max-length HTML-escaped model, all four token dimensions and exact estimate at MaxInt64: cap test passes |
| `vet-all.log` | `go vet ./...`, exit 0 |
| `build-final.log` | Integrated `go build ./...`, exit 0 |

Logs `cx6a-*` live in `/tmp/f3-codex-20261120/`; other logs in `/tmp/f3-codex-6a/`. Expanded pure matrix is verified on both toolchains. Final two HTTP cases and integrated normal/race suites are delegated to the coordinator on fresh databases after final fixes; do not infer their results from earlier runs.

## Limits / handoff

- The daemon wire remains frozen; both fixture digests are pinned. Daemon delivery accepts 2xx regardless of body; its test examples use `result`, while design permits the server response shape above (`status`). No daemon code changed here.
- Aggregate overflow has no specific offending item in the pure library error; decoder errors have precise indexes, aggregate failure is batch-level and atomic.
- Full package/regression/migration suites and the actual cmd/server router negative are leader integration work (router now wired by leader). Handler test uses the frozen routes with real DaemonAuth and a valid issued mat_ credential.
- No provider execution, installed CLI, enforcement, UI, billing reconciliation, deployment or release performed.

Independent reciprocal 7a review found no confirmed 6a product defect. 6a author separately reviewed management and 5c creation/off code in `codex-crosscheck-7a.md` and `codex-crosscheck-5c.md`. Final integrated verification belongs to coordinator. Final server spec received an independent factual review and local corrections in completion-tree `.trellis/spec/server/issue-budgets.md`; source audit confirms `Evidence`/`Prices` overrides occur only in tests, both HTTP routes always construct production sources, and no network input configures them.


## Supplemental acceptance closure

New test-only file `server/internal/service/issue_budget_usage_acceptance_test.go`; no product edits.

| Row | Test and literal expected behavior |
|---|---|
| B07 | `TestIssueBudgetUsageAcceptanceRetryKeepsActualAndLimit`: real `CreateRetryTask` clone preserves retry lineage; original actual40 + retry25 =65, unchanged L500/policy revision1; R75/H60, original-key replay unchanged |
| B38 | `TestIssueBudgetUsageAcceptanceUsageAndCancellationBothOrders`: two root-lock queued writers, usage→hold and hold→usage; A30/R0/H70, one hold and one usage operation, duplicate report/reconciliation no writes |
| B10/B11/B57 | `TestIssueBudgetUsageAcceptanceMovesAndParentDeletionKeepOriginalRoot`: later reports after cross-root move, rootless-parent move, cleared parent, project move, and real DetachDirectChildIssues + parent delete; original-root A progresses10→30→60→100→150→210; R2 stays0; member binding stays byte-for-byte stable; root_conflict/root_deleted observations retained |
| B45 | `TestIssueBudgetUsageAcceptanceConcurrentReservationsPastLimit`: actual90, L100, two ordered concurrent reserve10 claims both succeed; A90/R20, exactly second has limit refusal with literal amount detail; execution-facing task state/priority/dispatch timestamp/token match the unbudgeted control |

`cx6a-supp-normal-01.jsonl`: fresh `f3_budget_cx_supp_normal`, exit0, 6 pass /0 fail /0 skip. `cx6a-supp-race-01.jsonl`: fresh `f3_budget_cx_supp_race`, exit0, 6 pass /0 fail /0 skip. `go vet ./internal/service` and gofmt passed after adding the supplements. Runner uses identity-checked private DATABASE_URL and installed-agent CLI guard. Both logs are in `/tmp/f3-codex-20261120/`. Independent acceptance auditor read all four test bodies and found coverage adequate.

STATUS: complete (usage implementation and supplemental acceptance normal/race verified; coordinator owns integrated archive)
