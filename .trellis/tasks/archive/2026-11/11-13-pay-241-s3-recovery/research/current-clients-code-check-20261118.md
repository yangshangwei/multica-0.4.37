# S3 current-client harness code check — 2026-11-18

**ACCEPTED_BOUNDED_CLIENT_HARNESS_CODE_R05; open code blockers 0.** The final two-file R05 harness closes the concrete review findings and passes an independent strict targeted TypeScript and ESLint check. This is a code/static gate for a fresh complete run. Original field R01 remains **NOT_ACCEPTED**, and the new field/batch runtime evidence is not supplied or accepted by this report.

Active task: `.trellis/tasks/11-13-pay-241-s3-recovery`; reviewer `/root/s3_clients_check`. Main owns environment/SQL/build/browser execution and integration. This reviewer wrote only the assigned code-check pair and private `independent-check/client-code-r01` / `client-code-r02` supports. The exclusive implementer made all harness fixes. No product/shared fixture, prior failure, task matrix, Git index or peer F3 document was edited by this reviewer.

## Findings (fixed)

All locations below refer to `e2e/fixtures/collaboration-recovery.ts`; final hashes are recorded below and in the JSON, because line positions moved during repair.

| ID | Initial defect / proof gap | Final R05 correction |
| --- | --- | --- |
| S3C-C01 | Web context/native profile/app and CDP were acquired before the cleanup guard. Authentication/CDP failure could leave a resource unregistered with `withSessions`. | All acquisition and setup are inside one guard; failed acquisition records the original exception, partial lifecycle and cleanup outcome, and closes the created app/context/profile. |
| S3C-C02 | Sequential cleanup could stop after the first failed close. Repeated held-route `unroute` in `finally` could replace the actual primary failure, as the retained Desktop R01 reporter demonstrates. | Every session closes through `Promise.allSettled`; held-route disposal is idempotent and records its cleanup failures. The original body error and active fresh-intent phase are captured before owned teardown. Page/context close, native PID/exit and cleanup timestamps are retained. After `finally`, the original failure has priority; an otherwise successful body still fails for recorded cleanup failure. |
| S3C-C03 | A click failure could leave its previously created response-wait promise unjoined. | Recovery and ordinary picker gestures use the real click/response pair with `Promise.all` and settle both on failure. Existing action and case deadlines remain unchanged. |
| S3C-C04 | Both GETs were checked for stable self-equality after fresh intent, but the losing original GET was not directly matched to its actual POST refusal. | Both original priority/transfer GETs must equal the corresponding actual original POST's canonical `CollaborationOperationSchema` result before later immutability assertions. Forbidden403 and stale409 remain distinct. |
| S3C-C05 | Observer write audit watched only the selected issue and ordinary PUT, excluding draft uploads/creation. Asynchronous header metadata could be appended after a zero-write assertion. | Safe method/path/request-key/auth-presence metadata covers collaboration business routes, uploads/creation, collaboration notifications and ordinary PUTs. Pending metadata is drained before recovery baselines and final zero-write assertions. Raw auth, multipart bytes and frame payloads are not retained. |
| S3C-C06 | Raw canonical timestamp order used `Date.getTime()` milliseconds, losing the persisted PostgreSQL microsecond tuple while claiming exact `(created_at,id)` ordering. | Authoritative page comparisons parse an exact microsecond instant with bigint; UI grouping retains its separately named existing millisecond comparator. Raw global tuple and DOM thread ordering are reported independently. |
| S3C-C07 | The first cleanup-verdict fix threw inside `finally`; retained R04 ESLint correctly rejected `no-unsafe-finally`. | R05 stores body outcome and cleanup verdict through `finally`, then rethrows the primary failure or reports successful-body cleanup failure afterward. The rule was not suppressed. |

The original R01 type error, root-only dependency lint failure, actual Desktop runtime failure and R04 unsafe-finally failure remain separate raw evidence. Fixing the harness does not retroactively pass any original run.

## Behavior and scope reviewed

- Both collaboration UI requests are captured with their real original keys and same revision before either actual API forward. `route.fetch({maxRetries:0})` forwards the unchanged request; the winner's committed response is deliberately aborted, not replaced with a fabricated authoritative body. Both orderings retain winning fields, exact 403/409 refusal, content-free own GET, UI original-result recovery, deliberate new authorized UI intent and earlier result immutability. Old POSTs stay denied after authority changes; a wrong actor's own-result lookup remains404.
- Ordinary numeric CAS is explicitly supplied by authenticated platform API requests. One actual winner and the exact numeric `revision_conflict` are followed by a fresh numeric request and two unmodified ordinary picker requests that omit the optional revision. The parent business/activity/execution projection remains unchanged. The harness does not insert CAS into a picker request or claim all installed/ordinary clients enforce it.
- The batch authoring separately counts 55 unique original comments, one baseline and one reply: **57 persistent IDs**, with one edit and one tombstone represented through their existing identities. The observer's HTTP context goes offline and its actual native WebSocket closes while the real reconnect handlers remain attached. A replacement connection is required: Cookie Web uses successful101 plus authorized exact-issue subscription/current principal, while token Electron requires auth→auth_ack→issue subscribe→subscribe_ack.
- Both final read passes walk the real Load more/refetch page chain until a null cursor, compare raw unique IDs and complete bodies/tombstones, exact microsecond ordering, versions/parent identity, encoded byte bounds and the separately ordered DOM. A second actual disconnect/reconnect has no new intent. The zero observer mutation assertion covers the widened route metadata after drains. Runtime evidence still must prove those paths reached their terminal assertions on each platform.
- The six planned top tests contain four field tops with eight ordering subcases and two batch tops with two batch subcases. UI compatibility, API CAS, 55 original-comment and 57 persisted-ID counts are not additions to the parent 24 business cases. Test listing is inventory with zero bodies; it is not an executed batch test.

## Retained actual R01 failure

The exact first-field source is held separately under `client-code-r01`, retaining original fixture SHA256 `a4adc002b2080f2e046e06662a8efbfad5bbade225c52c5a5593490638e3fa01`. Its actual reporter has **4 planned tops: 2 PASS / 1 FAIL / 1 NOT_RUN**, with **8 ordering subcases: 4 PASS / 1 FAIL / 3 NOT_RUN**, retries0. The final top was not attempted after max-failures1; a reporter skipped status with zero results is not a business skip or pass.

Desktop priority-first reached the actual200 response-loss, original transfer409, both own GETs, UI recovery and same-key200/changed-priority409. Its last retained checkpoint precedes fresh intent. The reported failure is `page.unroute: Target page, context or browser has been closed` from outer cleanup. The 11.154-second result is not the 360-second case timeout. The saved screenshot shows the first priority client's readable confirmed action; it does not identify why the other page closed. The original primary error/native cause was not separately recorded, so no product crash or native cause is attributed. R05's diagnostic improvement enables a new run; it cannot reconstruct the missing old exception.

Main separately retained and corrected its creation-gate restoration assumption using the actual original allocation (creation true, execution/host false). This is coordinator restoration evidence, not a product failure or a new testcase. Preserve its first rejection and corrected receipt; final resources for the next run remain main's responsibility.

## Verification

Final physical ROOT files, mode0644:

| File | SHA256 |
| --- | --- |
| `e2e/collaboration-recovery.spec.ts` | `10cac566342deb24c68e90fd15cdcfd1c8a6aec50c9572116dfbb683c77b67ca` |
| `e2e/fixtures/collaboration-recovery.ts` | `0188d92a565ae9c63e7d08b30b1cd87cfe0444b8701a6cf0211fab10dcc32614` |

Independent commands ran sequentially from ROOT with only explicit PATH, trusted task HOME and LANG. They used the already installed Node/TypeScript/ESLint and the author's frozen R05 configurations; complete argv, config/tool/source hashes and raw outputs are in `independent-check/client-code-r02`:

```text
node <installed-typescript>/bin/tsc --project <R05>/tsconfig-e2e.json --pretty false --listFiles
node <installed-eslint>/bin/eslint.js --config <R05>/eslint.config.mjs e2e/collaboration-recovery.spec.ts e2e/fixtures/collaboration-recovery.ts --no-ignore
```

- **TypeCheck: PASS.** Exit0, strict two E2E roots plus their resolved graph, noEmit/nonincremental; no diagnostics. The file-list stdout is retained separately from test inventory.
- **Lint: PASS for the explicit two-file scope.** Exit0, empty stdout/stderr; existing Core React ESLint rules, no ignored-file substitute and no disabled unsafe-finally rule. Declaration lookup deliberately spans root/Core/Views: Core already declares zod, Views declares `@multica/core`, and existing E2E imports use these canonical packages. No new installation/dependency/package mutation is introduced. This is a declared E2E-consumer lint scope, not a claim that root directly declares both dependencies or that whole ROOT lint passed.
- **Static source identity: PASS.** Both checks have exact matching before/after bytes and modes. Original R01/R02/R03/R04 static raw receipts were read and preserved; fresh independent R05 checks did not execute them again.
- **Runtime tests: original R01 NOT_ACCEPTED; new field/batch NOT_RUN for this code gate.** This reviewer executed zero testcase bodies, DB/API/browser/Provider calls, builds or process controls. The saved main live-identity report establishes a prerequisite only; it does not make the incomplete R01 or future run green.
- **Full S3 / TC-16 / parent: NOT_ACCEPTED by this report.** Main can freeze this exact harness and run all four field cases on a fresh recorded database/configuration. Batch, notification/upload/revocation/blocking and final S3 remain their own required evidence.

## Findings (not fixed)

No open mechanical or scoped-code finding remains in R05. Runtime failure cause attribution and a fresh complete current candidate run remain required. Effective source/dependency/build/live identity, original gate/connection/process restoration and full persistent no-duplicate effects must be bound to that actual run; frontend projection alone does not certify the entire database.

The existing password/native fixture and two new files reuse the current application protocol and shared parsers. The native harness scope remains renderer/preload and real Electron with synthetic password accounts, without installed Main or real-model guarantees. Earlier S2/other-task/F3 WIP and all failures remain outside this code commit's ownership.

JSON support records the exact initial/final source states, every finding disposition, original run denominator, original/static receipts and fresh independent checks. Main owns runtime, matrix/spec integration and commits. This bounded code-review slice is complete.
