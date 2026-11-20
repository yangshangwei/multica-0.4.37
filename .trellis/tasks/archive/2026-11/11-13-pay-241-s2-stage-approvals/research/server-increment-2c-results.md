# S2 server increment 2c results

Status: **implemented; isolated-source focused verification passed; independent review pending**. Project policy activation remains false until every2d writer is wired and verified. Full A01–A12, production Web/matching Desktop and real external integrations remain open.

## Delivered behavior

- Added three append-only ledgers for execution intent, execution observations and rollback metrics. Migrations829–843 use13 separate concurrent indexes, registered failed-build cleanup, explicit audit retention and repeated history refusal at both843 and829 down boundaries. No foreign keys or cascading actions were added.
- `StageExecutionSources` is a purpose-specific trusted server dependency forwarded through `Handler.stageService()`. Production construction supplies none. There is no public producer, simulation switch, verifier registration, scheduler or automatic retry.
- A7 release binds the server-observed artifact, exact QA report/approval chain and reviewed target digest. Rollback is signed in verification and binds the exact successful deployment, its observed immediate predecessor, current registered anomaly and selected release approval. Readable optional return references and reason-only returns remain available independently of passing evidence or a configured execution adapter.
- Internal Prepare validates the actual registered integration consumer separately from original human issuers. It reuses the workflow lock order, current policy/roles/scope/code/CI/build and original signer checks. No approver context is installed as the caller. Each artifact-bound original issuer retains their own current M1 source-read authority; a different manager consumer cannot replace a demoted original issuer.
- The reviewed configuration digest excludes evolving conditional/current-artifact state. The private approval still freezes that state. Prepare re-resolves and compares it, including after slow checks, so an outside deployment between signature and admission refuses. A9 accepts only the selected approval's unique intent and its own authenticated successful after-state; a newer approval cannot endorse an older deployment.
- A stable physical-target key excludes workspace/root, local integration binding, display names, trust/config revisions and conditional versions. One unresolved intent blocks the same physical domain across local bindings and workspaces. One intent per approval and original consumer request is enforced; exact history lookup never recreates entry.
- The service owns one synchronous atomic entry guard after **definite commit**. No permit escapes. Its finite deadline is the minimum of30seconds and the earlier approval/issuer/CI/artifact/metric/consumer deadlines. The monotonic anchor is taken **before** the final database-clock query, so query/return/commit waits are conservatively counted. Pre-entry expiry/cancellation produces zero calls and may retain internal `not_started`; ambiguous commit produces zero calls and no success claim. Post-entry panic/transport uncertainty remains unknown. Finalization uses the separate bounded5second context.
- Authenticated observations retain exact native identity, original external key, target and before/after artifacts. Terminal success must observe the intended artifact. Native operation IDs are bounded strings, not UUIDs. Equivalent captures recover history; changed native payloads/terminal contradictions refuse. Pending, unknown and not-started are never execution facts. A8 refuses revocation once an intent consumed the approval. Late facts do not renew approvals, call again or advance the stage.
- Metrics use native ordering rather than arrival order. Equal native version plus different canonical content persists both captures and makes the stream ambiguous. Both existing and new rollback approvals are blocked; an older arrival cannot resolve it. A strictly newer registered observation and a fresh explicit approval are required. Stream locks protect consumption against a concurrent capture; deadlines and current query/rule identity remain mandatory.
- R4 initializes all three new arrays, preserves the anonymous withheld-source group and exposes bounded targets, operations, facts and metric states. Fact ordering follows received observations, including late facts. Builtin guidance/source maps distinguish human approval, uncertain operation and authenticated execution.

## Test-only router control

`cmd/server/collaboration_stage_execution_{source,control,control_boundary}_test.go` implements the frozen private JSONL control surface. It constructs the actual router/auth and required account/config/realtime/knowledge/runtime maintenance workers. No human approval or execution fact is seeded by this control path. Controlled source identities use separate real database integration/trust rows and private authenticated captures before calling production methods.

The harness honors the existing `PORT` only in its explicit test-harness mode and binds127.0.0.1. It emits the documented ready/reply prefixes and safe invocation counts. Root compiles/runs it as a persistent test binary with an explicit coordinator-owned lifecycle; this does not change any E2E, Host, entry or finalization timeout. The harness binary was compiled here, **not launched long-lived**. Actual Web/Desktop/API/CLI/owned-Git scenarios wait for2d and root execution.

Four router tests exercised the real listener/auth adapters, private control validation, production's nil-source default, immutable native captures, and actual artifact create → human approve → package routes. The produced tar entry/bytes/hash/EOF were checked with the existing artifact helper. These are bounded router/packager proofs; the complete public A7→internal execution→UI journey has not run in this increment.

## Final verification on S2-only source

All counts include named subtests; selectors overlap and must not be added as business acceptance cases.

| Command scope | Named pass / fail / skip | Exit |
| --- | ---: | ---: |
| service `^TestStage` | 145 / 0 / 0 | 0 |
| service execution/ACL/QA/expiry `-race` | 44 / 0 / 0 | 0 |
| handler `^TestStage` | 49 / 0 / 0 | 0 |
| handler `^TestStage -race` | 49 / 0 / 0 | 0 |
| router `^TestStageExecutionRouter` | 4 / 0 / 0 | 0 |
| router same selector `-race` | 4 / 0 / 0 | 0 |
| affected service/handler/router `go vet` | static | 0 |
| full backend `go build ./...` | static | 0 |
| dedicated router/harness test-binary compile | compile only | 0 |
| frozen overlay `gofmt -l`; whitespace audit | empty / zero violations | 0 |

Tests include real concurrent Prepare calls with one entry; cross-workspace/rebinding uncertainty; lost intent commit; post-commit cancellation; commit delay beyond the original approval deadline; late authenticated success after expiry; changed terminal/native identity; exact selected-fact progression; opposite metric arrival orders and older delayed observations; original manager demotion; source-denied arrays; old five-nullable-field code-review/QA request bytes; and the prior T4/QA artifact ACL regressions.

The six permitted service/handler/router normal/race processes ended and their profiles were released. Each actual test command used root's identity-checking `run.mjs` and its exact package profile. This lane created, migrated, dropped or switched no database, and did not launch the long-lived harness or use root's migration database.

Root's independent actual migration evidence is in `server-increment-2c-migration-results.md/json`: all eight allocated databases have the831-entry ledger through843, exact ready indexes/triggers and zero new FKs; empty down/up succeeded; a clearly marked retained synthetic intent separately refused843 and direct829 downgrade without losing ledger/index/history. The global cleanup inventory still has only the eight pre-existing F2 omissions; the S2 inventory passed.

## Source ownership and freeze

Final source:

`/Users/artisan/.multica/acceptance/pay241-20261114-resume/s2-server-inc2c/isolated-r01-source`

It contains **4,938 files**, reconstructed from the independently checked **4,894-file ACL base** plus a **63-file S2 overlay**. `isolated-r01-manifest.json` binds the overlay; `full-source-manifest.json` binds every final file. The JSON companion records hashes, commands, raw logs/exits and ownership checks.

A concurrent existing knowledge-automation task added workspace changes, including844–859 migrations and shared generated/configuration files, while this increment was being verified. The initial mixed `final-candidate-1` capture and its tests remain intact. Its service run returned Go exit0 but failed the final source-identity check; it is **not** the final S2 source or an S2 staging list.

The final reconstruction used the certified ACL base, S2-owned files and the earlier S2-only DDL capture for shared migration/retention/sqlc configuration. Only the S2 dependency-injection hunks were applied to the shared service/handler structs. sqlc was then generated **inside the private reconstruction**, with no knowledge-automation or844+ source. Peer workspace changes were neither reverted nor copied into S2 acceptance.

All exclusive workspace files match their frozen S2 bytes. The workspace retention manifest, generated models and sqlc configuration have additional peer changes; `workspace-ownership-audit.json` records those three explicit shared differences. Root must merge/stage the S2 hunks with their recorded provenance, rather than replacing the shared files. No Git writes were performed by this lane.

## Preserved failures and limits

- `target-red-1` records the initial missing target/physical-domain implementation.
- `core-build-1` records two compile corrections: the ordinary Issue model has no collaboration boolean, and generated readiness is a nullable pgtype.Bool. Neither was hidden or relabeled as an executed scenario.
- `issuer-artifact-red-1` records a real authorization failure: a demoted original code reviewer was already denied R4, while another manager consumer still invoked execution once. The correction reuses the original M1 source reader for each retained signer; `issuer-artifact-green-1` and the final normal/race selection pass.
- Initial DDL capture is retained. Root identified the missing repeated829 table-drop history guard; `ddl-ready-r02.json` adds that exact correction without overwriting the first capture.
- Mixed-source capture/checks are retained separately from final isolated-source evidence. No844+ migration, peer implementation or unrelated cleanup assertion was absorbed into this increment.
- No real model account, payment, remote CI, deployment or rollback provider was called. Service observations and metric rules are explicitly controlled fixtures. Production has no execution adapter; S4 provider semantics and business sign-off remain NOT_RUN.
- Independent2c review is pending. All2d submission/completion/project-exit writers, final guards, activity projection, collaboration M1 preparation consumers and legitimate public policy activation remain the next server increment. The mechanical2d client/read/activity contract is frozen separately; it is not implemented by this result. A01–A12 and the parent matrix remain open.
