# Research: R33 original policy-wait control inventory

- Query: Exact original top/named/leaf sets and selectors for the M1-only wait verification, including the two added writer controls.
- Scope: internal; textual source inspection and file hashes only, no Go/test-list/AST execution.
- Date: 2026-11-18

## Findings

The final original set is **8top / 34named / 29leaf**. Combined with the planned new **2top / 6named / 4leaf**, the expected complete normal/race inventory is **10top / 40named / 33leaf**. These are source inventories, not executed passing results. A childless top is also one leaf; a parent with subtests counts once in named and top, but not leaf. Unnamed loop iterations do not add test names.

This inventory incorporates the coordinator's added `TestDeliveryPolicyConcurrentCAS` and `TestDeliveryPolicyWriterFirstRefuses`. The earlier six-control inventory remains true at6/30/26; the additions contribute2/4/3. The sealed prior plan JSON `64486bd4ff02d9a88eb46f507c52abc18104fc09d2ac41311cbb3e22212d7e8a` remains unchanged. This report supplies its updated execution selector, not a new product boundary.

### Frozen files found

Source root: `/Volumes/artisan/.multica-acceptance/pay241-r33-policy-wait-20261118-hc2gedj3/source-red-r01` (5,055 baseline files per coordinator; no full manifest recount).

| File | SHA256 | Source used |
| --- | --- | --- |
| `server/internal/handler/delivery_policy_test.go` | `9f6a3b45e67c146f7f8a0ac0005aa2f2a17608e995f56b48547cbdd93c68b12d` | Default/CAS94; authority120; strict142; concurrent CAS595; writer-first630. |
| `server/internal/handler/delivery_policy_schema_test.go` | `96676a7adb2377e541a46a02393d242de4e05b53e5e0852d12f2146df64fdaae` | Five real schema/index shapes at12–14. |
| `server/internal/handler/delivery_completion_activation_test.go` | `98ce60c923f145813333d7e8b723c5ac8cc3977cae87b74b1ef2fc6c018907b0` | Explicit activation15; five partial-schema cases74–76; excluded grant-expiry test117 inspected only to classify its coverage. |

### Complete original inventory

All subtest strings below are suffixes of the exact top named in that row. Their full names and the complete top/named/leaf arrays are also stored in the companion JSON.

| Exact top | Named | Leaf | Exact leaf suffixes, or the childless top itself |
| --- | ---: | ---: | --- |
| `TestDeliveryPolicyDefaultAppendAndCAS` |1|1| Childless top itself. |
| `TestDeliveryPolicyManagementAuthority` |4|3| `owner`, `admin`, `member`. |
| `TestDeliveryPolicyStrictRequests` |12|11| `0`, `1`, `2`, `3`, `4`, `5`, `6`, `7`, `8`, `9`, `10`. |
| `TestDeliveryPolicyRuntimeSchemaRefusesPartialAndWrongIndexes` |6|5| `missing`, `nonunique`, `reordered`, `partial`, `included`. |
| `TestDeliveryCompletionActivationExplicitCASAndCapabilities` |1|1| Childless top itself. |
| `TestDeliveryCompletionActivationRejectsPartialSchemaAllowsOff` |6|5| `authorization_index`, `project_change_index`, `github_trust`, `vcs_trust`, `github_trust_weakened`. |
| `TestDeliveryPolicyConcurrentCAS` |1|1| Childless top itself. |
| `TestDeliveryPolicyWriterFirstRefuses` |3|2| `project`, `member`. |
| **Total8 tops** |**34**|**29**| |

The strict-request loop is exactly11 bodies and uses `t.Run(fmt.Sprint(i))` (`delivery_policy_test.go:143`,155). Index0 is `{}`,1 is JSON null,2 unsupported skip mode,3 null reason,4 blank reason,5 duplicate reason,6 wrong stage,7 differently cased `Reason`,8 negative revision,9 wrong policy version,10 extra skip field. The four successful policy modes in the default test are an unnamed loop, not four subtests. The schema and activation switch branches likewise do not create additional nested test names beyond their five `t.Run` values.

Original-only selector (literal `-run` value):

```text
^(TestDeliveryPolicyDefaultAppendAndCAS|TestDeliveryPolicyManagementAuthority|TestDeliveryPolicyStrictRequests|TestDeliveryPolicyRuntimeSchemaRefusesPartialAndWrongIndexes|TestDeliveryCompletionActivationExplicitCASAndCapabilities|TestDeliveryCompletionActivationRejectsPartialSchemaAllowsOff|TestDeliveryPolicyConcurrentCAS|TestDeliveryPolicyWriterFirstRefuses)$
```

### Planned new inventory and combined selector

The latest agreed new file is `server/internal/handler/delivery_policy_project_wait_test.go`. Its source was not read or executed in this inventory task; verify its eventual frozen source against these six planned names before claiming combined completion:

- `TestDeliveryPolicyProjectWaitShortReader`.
- `TestDeliveryPolicyProjectWaitShortReader/short_reader_release`.
- `TestDeliveryPolicyProjectWaitBoundaries`.
- `TestDeliveryPolicyProjectWaitBoundaries/long_reader`.
- `TestDeliveryPolicyProjectWaitBoundaries/competing_writer`.
- `TestDeliveryPolicyProjectWaitBoundaries/member_denied`.

The new leaf set is the four slash-qualified names; its two parent names are not leaves. The combined named set is the disjoint union of those six and the34 original names. The complete40-name union and33-leaf union are in JSON.

Combined selector, identical for normal and race (literal `-run` value):

```text
^(TestDeliveryPolicyProjectWaitShortReader|TestDeliveryPolicyProjectWaitBoundaries|TestDeliveryPolicyDefaultAppendAndCAS|TestDeliveryPolicyManagementAuthority|TestDeliveryPolicyStrictRequests|TestDeliveryPolicyRuntimeSchemaRefusesPartialAndWrongIndexes|TestDeliveryCompletionActivationExplicitCASAndCapabilities|TestDeliveryCompletionActivationRejectsPartialSchemaAllowsOff|TestDeliveryPolicyConcurrentCAS|TestDeliveryPolicyWriterFirstRefuses)$
```

Selecting each anchored top includes all its descendants. Keep native results and compare each normal/race run separately against10top/40named/33leaf, with no missing, extra or skipped names. A failed fixture can prevent later subtests from being reached; that is an incomplete actual result, not a revised expected denominator. No test-list or actual test command was invoked here.

### Adequacy and limits of this focused set

- The new short-reader case must hold an actual GET's shared project fence, start actual PUT before release inside existing250ms and preserve real RED409/55P03 before the repair. Long-reader coverage proves retained busy/no-append behavior and records the actual budget without an exact249/250ms timing assertion.
- The added original concurrent-CAS control holds a real first PUT at its commit barrier until the second has returned409, then commits the winner and explicitly checks stale409/one append (`delivery_policy_test.go:595`). Under waiting SQL, this remains a long-writer busy control; it does not claim immediate refusal. The new competing-writer leaf separately proves short-writer release followed by exact stale-CAS409.
- Original writer-first `project` makes a real uncommitted project revision update; `member` makes a real uncommitted role update. Both expect409 and zero policy rows (`delivery_policy_test.go:630`). They preserve distinct project and current-member authority fences; they are included as requested, not discarded as duplicate new cases.
- Default policy/CAS and explicit activation cover virtual off, exact append/CAS, same-value revisions and off rollback with retained policy history. Authority covers current owner/admin positives and member403. Strict requests and both schema families preserve400/503 refusal, no invalid append and safe off behavior with incomplete enforcement dependencies.
- This inventory does not assert current JWT/password expiry during the new wait was dynamically tested. The existing handler's pre/final credential calls and credential helper are still hash-equal to the sealed baseline and must remain unchanged in GREEN. Previously accepted current-credential/password controls may be retained only with their original source/evidence binding. No new mirrored authority/expiry matrix is added.
- `TestDeliveryCompletionActivationExpiryAndOffRetention` is **excluded**: at117–148 it expires a completion authorization grant using the actual database clock and verifies retained observation/authorization history. It is not proof of JWT/password expiry during a policy wait. The selected two activation tops are the focused policy/schema controls; do not add the grant test solely because its name contains expiry.

### Product scope and static preservation

The chosen conditional GREEN remains three files: `handler/delivery_policy.go` changes one PUT call; `queries/delivery_policy.sql` adds explicit `LockDeliveryPolicyProjectWait` with identical project/workspace predicates and exclusive `FOR UPDATE`; `generated/delivery_policy.sql.go` contains canonical generated output. The250ms setting remains in `PrepareArtifactTransaction` (`artifact_authority.go:72`) and applies per lock acquisition. Actual current authority, CAS, schema, final credential recheck and rollback remain.

Current frozen source hashes still match the sealed R33 baseline:

- `server/internal/handler/delivery_policy.go`: `97bde61992f28bb278c5ec27ae71b361ed6a371ad27cc49649dc28bebbb366f8`.
- `server/internal/service/artifact_authority.go`: `61e457c83e94f46e8ace6dd58d772fa20c9334dc217421f874b4037f9af7aab8`.
- `server/internal/service/collaboration_stage_policy.go`: `a892c72b67010b78a7d1383406ba11412efc2a9ecc3acd1e3df36295265cf9d6`.
- `server/pkg/db/queries/delivery_policy.sql`: `3cdd43c157d89fa152d53cfbc887036d08a00bf30a76736da6852f56a9d1a6c8`.

For GREEN, verify the original `LockDeliveryPolicyProject` SQL/generated adapter and stage-policy caller remain byte-bound to their originals; the original query continues `FOR UPDATE NOWAIT`. Adding the separate wait query must not silently switch the stage caller. With that preservation, this increment introduces no stage-service/routing behavior change requiring a service, whole-handler99 or router54 reexecution. An accidental changed caller, generation drift or new failing dependency needs its own affected-scope review; there is no blanket rerun exemption or gate.

### Related specs and references

- `.trellis/spec/server/verification-environment.md` and `database-test-isolation.md`: frozen sources, independent package normal/race profiles and complete native inventories.
- `.trellis/spec/server/delivery-policy-evaluations.md` and `prd.md` A08/A09: current authority, exact policy CAS, retained off/observe/enforce boundaries and no retry.
- `research/r33-policy-project-wait-plan-20261118.md/json`: unchanged conditional3-file repair and new4-leaf test boundary; this inventory adds the two original writer controls.
- Primary dependency facts remain local Go1.26.6/pgx5.9.2/Chi5.3.0; no external documentation lookup or dependency is needed to enumerate these tests.

## Caveats / Not Found

- Static names/counts are intended complete source sets, not PASS claims. Neither original nor new tests, Go/test-list, AST, database, API, browser, process control or agents were run.
- Only these two new research outputs are written and sealed0444. The prior sealed plan and all peer source edits are preserved.
- Native R32 project-lock55P03 → HTTP409 busy is proven for that request; physical holder remains unknown. Original completed writes stay preserved, fullS2 and R38 acceptance remain open. Profile allocation and5,055-file manifest count are coordinator-supplied preparation, not executed evidence.
