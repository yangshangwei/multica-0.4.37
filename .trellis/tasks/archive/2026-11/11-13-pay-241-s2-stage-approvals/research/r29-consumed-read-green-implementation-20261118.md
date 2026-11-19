# R29 consumed authorization GetByID GREEN implementation

Status: **PRODUCT_FROZEN_STATIC_READY_MAIN_EXECUTION_REQUIRED**. Only the two authorized private products and this new report pair were written. No GREEN tests, compilation, vet, Go AST/list/TestMain, DB/SQL, API/browser, environment/configuration, process/signal or Git action was performed by this agent.

The minimal repair lets the current same-human issuer select compatible shared fences through GetByID's existing authority chain. Actual grant creation, by-request recovery, revocation, consumption and finish preserve their defaults. The same borrowed transaction reaches the grant, receipt root, source child, collaboration/session and exact artifact observer; no new transaction or authorization is created.

## Genuine RED and corrected failed-row identities

Main's retained RED is **3 top-level / 13 named: 9 PASS, 4 FAIL / 10 leaves: 7 PASS, 3 FAIL**, with no skip/missing/pending/drift. Its result SHA256 is `09765a6246c9a194219f4b91076ef6bc30661735fa6d64496337673f268a90f8`; support SHA256 is `10fac670966442dc2699c5431efca5a45f7c61dfd0d93dadb84fcd4ffcdd7f95`. Both were reread and matched before the authorized GREEN edit.

| Failed leaf | First actual query | Actual failed row | SQLSTATE / second PID |
| --- | --- | --- | --- |
| `r4_first` | LockLifecycleIssue | source child `c0e93ca6-29f9-4c59-b6d0-69ee3343cd22` | `55P03` / 97046 |
| `authorization_first` | LockDeliveryEvidenceIssueRead | source child `ef023c7b-f8e3-4c96-af00-316c41435616` | `55P03` / 97071 |
| `same_grant_siblings` | LockDeliveryCompletionAuthorization | grant `01a1eaa3-c9cb-7301-a487-584453b39551` | `55P03` / 97097 |

The independent checker correction relayed by main is applied: both issue UUIDs above are **source children**. The baseline consumed receipt roots are respectively `01a1eaa3-ad62-7a41-846b-4cc1b293b3e9` and `01a1eaa3-b7f3-70b7-9bb6-b88f4e7b76f1`; neither equals its first failed issue UUID. Raw SQL hashes, actor, workspace arguments and PID are retained in JSON. Same-grant siblings fail at the actual grant row. The root also needs shared fencing for the newly reachable sibling chain; the first two RED errors did not themselves occur there.

Operation read orders, real Revoke write-fence orders and current actor/password/source controls remain retained PASS. R29 proves this deterministic fixture's reachable incompatibility. The physical holder of the original R28 runtime409 remains **UNPROVED**; the successful original completion was not replayed.

## Change boundary and preserved defaults

Private GREEN: `/Volumes/artisan/.multica-acceptance/pay241-r29-receipt-read-20261118-6dhi01s2/source-green-r01`.

| Product | Lines | RED SHA256 | GREEN SHA256 |
| --- | --- | --- | --- |
| `server/internal/service/delivery_completion_authorization.go` | 779 → 815 | `e3eb54ca4deffe5ad6f7199b139008cfd0bae4c7af3cdd0957d3c333e454c789` | `a8445340a055faa12fceaa04e70f62eb213e3aeac333b0bc5abff50520e4f0b0` |
| `server/internal/service/delivery_completion_collaboration.go` | 565 → 579 | `20fc764b20edbfae356db7785f82ac17c0d12a71d23f9a0dcf318624a71e0084` | `50c048f059392bed2f1b5c99cedba605c6fade3facbc28b963fee2f0c586183b` |

Both products remain mode0644. GetByID calls unchanged `completionReadSetup`, executes the actual `SELECT id FROM delivery_completion_authorization WHERE id=$1 FOR SHARE NOWAIT` on that Tx, then uses the existing generated `GetDeliveryCompletionAuthorization` getter. No query-name annotation disguises the grant SHARE as the old UPDATE. Its new private `readCompletionAuthority` path shares the receipt root with existing `LockDeliveryEvidenceIssueRead` and reaches `readCompletionIssuer` only for the unchanged current-human/original-issuer branch.

The original `completionReadAuthority`, `completionIssuerRead` and `completionSourceIssue` signatures remain, each passing **constant false** to one common body; their new private read entries pass true. The issuer body selects existing `readCompletionCollaborationAccess`, new `readCompletionSourceIssue` and existing `evaluateDeliveryEvidenceReadInTx`. The ordinary target still uses original `completionOrdinary`; current actor/source/project/policy/producer/spec/execution and exact retained execution identity checks remain in their original order.

The source validator is factored once. The original `exact` argument is propagated unchanged and is independent of `readProjection`; GetByID's source read still passes structural `exact=false`, while freeze/revalidate retain `exact=true` with default exclusive mode. The child read uses the same evidence UUID/workspace and existing shared query. Ordinary/type/child/parent/project/revision predicates are unchanged.

The consumer branch is exactly `completionConsumerAuthority(..., false)`; its `newAdmission` false is not used as a read-mode signal. R38 nomination/advisory logic and accepted R28 access/intent bodies are unchanged. By-request retains its original UPDATE getter and false authority; create-key replay and Revoke retain false issuer authority. Prepare/Finish and other products are protected by the complete source audit. Existing setup, stored request, minimal DTO/view and immutable receipt checks are untouched; no historical target re-admission, renewal or second consumption is added.

## Static verification performed

- Complete independent filesystem discovery: **5,052 same paths and modes; exactly 2 product deltas; 5,050 protected files unchanged; RED drift0**. GREEN canonical path/SHA256/mode manifest fingerprint: `a17a1d1fbfadfffaa6479c9ae52b7f34a4294ed738eed6f17cb93a6f00c9e8f7`. The full pre-edit manifest is in the hash-bound preparation receipt.
- Original **558-line test** remains `7e927d2721c00d98fead050c2129a223aac887b7cac8ef058a4789fa640a17d5`, mode0644. All existing tests, SQL/generated/migrations, DTOs, dependencies and remaining source bytes/modes are unchanged.
- Both recorded unified patches reconstruct their exact GREEN products from RED. JSON carries complete patches and hashes.
- **34 original function fragments are byte-identical**, including setup/view/receipt/stored request, Create/by-request/Revoke, original consumer/admin helpers and R28 intent/access. Three common default bodies reduce exactly to their original bodies after removing only the declared mode dispatch; signatures and constant-false wrappers match.
- Static `rg` caller inventory: `readCompletionAuthority` is referenced only by GetByID; `readCompletionIssuer` only by its true authority branch; `readCompletionSourceIssue` only by its true issuer dispatch. The common bodies are reached only by their false/true wrappers. No new Begin/Commit/Rollback, context flag or transaction marker exists. This is a textual/static check; main owns actual Go AST/type proof.
- Exact two-file `gofmt -w` and `gofmt -l` each exited0 with empty output. No compilation or database test is implied by formatting.

## Main-owned verification still required

Use newly recorded package-specific configurations and fresh owned databases, with full frozen source/configuration/command bindings. The command templates below were not executed by this agent; main must use its admitted working directory/environment and output paths.

| Owning package | Main normal/race inventory | Selector |
| --- | --- | --- |
| `./internal/service` | each 7 top / 76 named / 54 leaves: R29 13 plus original service63 | `^(TestDeliveryCompletionConsumedGetReadCoexistence|TestDeliveryCompletionConsumedGetWriterFences|TestDeliveryCompletionConsumedGetCurrentAuthority|TestDeliveryCompletionReceiptRejectsMismatchedRetainedFacts|TestDeliveryCompletionReceiptValidatesClosedVariants|TestStageCompletionAllM1ModesAndActualWriters|TestStageCompletionIndependentConsumerAuthorityAndMinimalReceipt)$` |
| `./internal/handler` | each 3 top / 6 named / 5 leaves, direct Get adapter controls | `^(TestDeliveryCompletionAuthorizationActualArtifactAtomicConsumeAndReplay|TestDeliveryCompletionCollaborationActualSourceAndOriginalRecovery|TestDeliveryCompletionR38AuthorizationHonorsCurrentConfiguration)$` |

For each selector, main's commands are `go test -json <package> -run '<selector>' -count=1` and `go test -race -json <package> -run '<selector>' -count=1`, preserving original deadlines/default suite budget and no retries. JSON contains their exact expanded templates. Main also owns real AST/caller verification, scoped compile/vet, package/DB identity and complete raw named/leaf/resource inventories, followed by independent GREEN acceptance.

No GREEN behavioral PASS is claimed here. ROOT integration/matched rebuilds, original four-scenario remainder, A01–A12/S2, parent24 and S3–S5 remain governed by their existing gates. All original runtime/RED/source/report history is preserved. Captured UTC: `2026-11-17T18:20:12.606417+00:00`.
