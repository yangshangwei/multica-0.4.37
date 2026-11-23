# Creation refusal: independent backend review

The creation-refusal delta is ready for integration. No production-code defect
was found, and this review made no production, SQL, migration or generated-code
changes. Earlier content, resource and realtime reviews were not repeated.
The parent retains final TypeScript, browser/native and full-product acceptance.

## Reviewed behavior

- Read the complete saved Trellis hook, task contracts, root rules and full
  `server/security-boundaries.md` spec before reviewing the delta.
- Traced `Create`, `CreateInTx`, the borrowed issue creator, attachment binding,
  quota/recipient checks, operation SQL, result decoding and authenticated HTTP
  routing. Current actor validation precedes completed-key replay. New requests
  reserve an operation only after source authority; sorted user, source-member,
  source-workspace and gate locks remain outside the creation savepoint.
- A recognized business refusal rolls back issue/counter/account-fence,
  participation/session, attachment and event/notification effects. Only the
  owning transaction commits its original result. A borrowed caller can roll
  back the staged refusal. Existing required-write wrappers still observe the
  inner work because the savepoint retains the caller's `pgx.Tx` wrapper.
- The classifier accepts the specified typed business errors. Actual query,
  savepoint, required-result-write and uncertain-commit failures remain unknown;
  the parent lookup maps real absence separately from database failure.
- Recipient/gate restoration and corrected draft consumption do not reinterpret
  an old key. Changed payloads conflict. The own-result reader revalidates the
  current human/password actor and returns exactly five correlated fields,
  without issue/recipient/file content, hashes or credentials. Source departure
  does not remove access to that minimal result. Successful issue POST/GET
  recovery retains current object authorization and its existing response shape.
- Malformed, missing, foreign, unfinished and corrupted result requests do not
  become rejection proof. A real commit with a lost acknowledgement is resolved
  only by reading the original operation. Machine/restricted principals cannot
  obtain the result or reserve a creation.

## Findings fixed

- `server/internal/service/collaboration_creation_result_test.go`: completed-key
  recovery had no direct password-version/restriction regression. Added both
  created and rejected outcomes across an actual credential version change,
  missing/mismatched password context, forced change, and a fresh valid session.
  Added the real workspace-deletion lock to the existing deterministic refusal
  commit waiter test. These are coverage additions; production behavior passed.
- `.trellis/spec/server/collaboration-content.md`: documented the new result
  route, five-field/closed-code contract, savepoint/commit authority, unknown
  outcomes and explicit fresh-key resubmission. Added the required regression
  touch points and clarified that the creation gate does not close recovery.
- `research/creation-rejection-2026-10-18.md`: replaced the obsolete requirement
  to edit input before resubmission. The confirmed contract permits unchanged
  input with a fresh key after a correlated settled refusal and an explicit user
  resubmission; reading the result never submits automatically.

## Findings not fixed

No unresolved backend behavior findings. The parent owns the continuation
artifact; its `continuation-product-2026-10-18.md` refusal paragraph still said
`明确修改后才建立新意图` at review handoff. The parent was notified to align that
sentence with the explicit-resubmission contract. This is documentation drift,
not a required production-source change.

## Verification

All database tests used only
`postgres://postgres@127.0.0.1:51974/cwc_product_integration_nebz79rl?sslmode=disable`
and the guarded Go runner with `-race -p 2 -parallel 2 -count=1 -v`. No tests
skipped. No service was restarted and no user-installed agent CLI was used.

| Evidence log | Result |
| --- | --- |
| `creation-rejection-review-service.log` | 41 top-level creation/result, recipient, draft and ordinary-parent tests; pass, 28.451 s |
| `creation-rejection-review-handler.log` | 3 HTTP result privacy/auth/unknown-commit tests; pass, 2.534 s |
| `creation-rejection-review-routes.log` | Real authenticated route and unchanged successful recovery; pass, 2.458 s |
| `creation-rejection-review-boundaries.log` | Added current-password regression and 5 deterministic lock contenders; pass, 3.870 s |
| `creation-rejection-review-attachment-creation.log` | Existing required-write attachment rollback; pass, 2.143 s |
| `creation-rejection-review-vet.log` | Guarded `go vet -p 2 ./internal/service ./internal/handler ./cmd/server`; pass |
| `creation-rejection-review-static.log` | `gofmt -l`, scoped `git diff --check`, and whitespace checks including untracked files; pass |

Evidence directory:
`/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-cwc-product-20261018-nebz79rl`.
`creation-rejection-review-summary.json` records exact test commands, counts,
durations and reviewed file SHA-256 digests. Go package type checking is covered
by compilation of all three affected packages during the race tests. TypeScript
checks and final browser/native builds remain assigned to the parent/client
reviewer. The exclusive integration database was released after verification.

This evidence covers the backend correction only. It does not complete the
parent task or its full 97-item product acceptance, and does not authorize
creation/execution gate enablement or a production release.
