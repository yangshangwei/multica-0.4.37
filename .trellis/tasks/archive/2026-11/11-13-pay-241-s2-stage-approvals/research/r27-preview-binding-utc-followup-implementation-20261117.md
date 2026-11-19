# R27 preview expiry UTC follow-up

The new `source-green-r02` contains one additional product line: normalize the
computed completion-preview expiry to UTC microsecond precision after all
horizon minima and before checking that expiry is still in the future. Both
regressions remain byte-identical. The prior sources and failed normal result
are preserved; actual corrected normal/race execution remains main-owned.

## Actual failure and existing contract

The original-product admin RED reached
`verification/stage_submission_unbound`, with zero PASS/two FAIL named
identities. Its result SHA256 is
`0684d9b12408e829710712e3e25394e5fc51ad028f32906a24f534190959f2aa`.

The initial binding fix's actual normal result is
`verification/binding-green-r01/result.json`, SHA256
`994848fffaa0e8bf575c5845bf43116700a61777434afc9b573fde7c67e30415`.
It has four PASS/four FAIL named identities, no skips, source drift or pending
names. Both human and admin previews now return `authorization_available` with
empty blockers and the correct binding/observation. The remaining failures
come only from the returned expiry's location: equal instants are represented
as PostgreSQL `+0800 CST` rather than UTC. The original failure witnesses were
read without executing the tests again.

The task's `research/wire-contract.md:65` states: “Timestamps are RFC3339 UTC.”
The ordinary `delivery_completion_preview.go` already normalizes expiry with
`expiry.UTC().Truncate(time.Microsecond)` before its future-expiry check. The
follow-up uses that exact existing convention. The database timezone, process
environment and test assertions are not changed.

## Exact product boundary

Only
`source-green-r02/server/internal/service/collaboration_completion_preview.go`
changes. Immediately after the issuer/stage/evidence horizon minima, the code
adds:

```go
expiry = expiry.UTC().Truncate(time.Microsecond)
```

Normalization occurs before `!expiry.After(clock.Time)`, so rounding down to the
checked instant cannot produce a returned expired authorization preview. The
existing `boundSubmission` initializer stays in place. Against R01 this is one
additional line; against original admin RED the complete product delta is the
bound-ID initializer plus this normalization line. No helper, lock, authority,
operation, source, writer, SQL or dependency changes are included.

| Identity | SHA256 |
| --- | --- |
| Original admin RED preview | `07637fee75dc894a50dd32eaa2beee3254784f015f0096fdd8cd3b44410fd0fe` |
| Preserved R01 preview | `9272930e7d665f61eb7ad1c8bafbd412e02651aa77138c682678fa8765cdd3ba` |
| R02 preview | `5fc937a4627c9271be1698847221be1abc4391934072ae87d117b8dc3b6b91ba` |
| Preserved 224-line managed regression | `f1aa77c0749504ee1826942c1511e5ad751a2254c8385ac69c2b0514ebc865e7` |
| Preserved 174-line admin regression | `658bb12c83611a78cb6974bc054e82bbb872a3d9a8ee51bfbb5a195db64be925` |

The R02 source has the same 5,050 paths and modes as R01. All 5,049 other files
match in bytes, and the product has exactly the normalization delta. Real Go
AST parsing confirms the unchanged two top-level/six leaf/eight named inventory.
Initial RED, admin RED and R01 sources match their existing complete manifests.

## Frozen evidence and remaining verification

New private evidence uses only the `binding-green-r02-*` prefix under the R27
root's `evidence/` directory:

- `binding-green-r02-freeze-r01.json` and full source manifest bind all 5,050
  files, the unchanged tests, existing source manifests and actual failures.
- Both new AST results bind the exact original five managed leaves and one
  admin leaf. No skip or parallel call is added.
- Product patches separately prove the one-line R01 delta and the cumulative
  two-line delta from original admin RED.
- Formatting, one optional compile-only command and service vet use the decoded
  GREEN profile, SHA256
  `37eaac0bfbf54db8c89f231e22e24a4ff6f47b5def528bada350621db7d0821d`,
  with no inherited variables and no database function called by its loader.
- The compile command is
  `/opt/homebrew/bin/go test -c -o <R27root>/evidence/service-binding-green-r02.test ./internal/service`
  from `source-green-r02/server`; vet targets `./internal/service` in the same
  source and environment. Both finished with exit 0, no timeout and empty
  stdout/stderr. The unexecuted test binary SHA256 is
  `7fe749b3c8a18212a87c9828f2af5e2514f357cf3ca793f6f4e74f40a54928a3`.
- `binding-green-r02-implement-postchecks-r01.json` rechecks all four complete
  source manifests, all prior implementation report hashes, the failed R01
  normal/admin RED results and all static log/binary hashes. Every identity
  matches after compilation/vet.

This implementer executes no test binary, TestMain, database/SQL command, API,
browser, application-process management or Git operation. Prior reports and the
failed R01 normal database/result remain intact. The old race permit is not
executed here. Broader preview read-lock coexistence, full S2 and parent
acceptance remain separate; historical R24/R25 holders remain unproved.
