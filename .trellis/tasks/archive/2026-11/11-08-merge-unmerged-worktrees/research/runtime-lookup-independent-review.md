# Independent review: capability runtime lookup repair

Reviewed at: 2026-11-08T11:13:50.571789+08:00

**Verdict: APPROVE for the exact eight files and hashes below.** No remaining concrete finding. This approval covers the narrow repair and its consistency with the previously approved integration; it does not replace the new fixed-commit complete gate or root preservation checks.

## Findings (fixed)

None. This was a read-only code review. No product or test bytes were changed by the reviewer.

## Findings (not fixed)

None in the reviewed repair.

## Behavior and dependency trace

- `CurrentTaskCapability` still constructs the same capability and performs preliminary profile/identity/task-status checks before any runtime query. Its only runtime-read change is invoking the supplied bound getter with the original context and task runtime ID. All later owner/workspace/agent binding, issue/project, membership, disabled-user, invocation-target and strict capability-decoding checks are unchanged.
- All three production callers were enumerated. Claim response construction passes `h.runtimeLookup(daemon_api).Get`, request validation passes `h.runtimeLookup(task).Get`, and claim finalization passes `s.runtimeLookup(q).Get`. There are no omitted callers or new direct runtime query exceptions.
- Handler runtimeLookup binds `Queries: h.Queries`, so a copied handler with transaction queries continues using that transaction. `taskCapabilityRead` builds the copied handler from its repeatable-read transaction; the getter cannot silently fall back to the pool.
- Finalization changed from a free helper to a TaskService method solely to access the existing metrics collector. `FinalizeTaskClaim` still supplies `qtx` after task/principal/invocation locks; `s.runtimeLookup(q)` uses this exact query object, not `s.Queries`. Token persistence remains in the same transaction after the same scope comparison.
- Reused RuntimeLookup returns the original row/error and counts `ok`, `not_found` or `error` through the existing closed-label implementation. Nil metrics remain safe. Missing rows and backend failures still reach the existing 403/503/refused-claim handling unchanged. No cache, retry, extra DB read, fallback identity or permission change was introduced.
- Passing a bound function avoids the existing service-to-auth import cycle without moving modules, adding a compatibility package or duplicating instrumentation. All actual call sites supply a nonnil getter; no speculative nil-getter fallback is needed for this internal contract.
- The existing metrics scanner and runtime wrapper are unchanged. The base/candidate object IDs in `baseline-blobs.json` confirm the original direct auth query and scanner predate this integration; the original failure is retained rather than waived.

## Regression adequacy

- The new auth test passes nil queries intentionally: original runtime errors must return before any subsequent authority query. It checks exact context/runtime identity, one call and original error identity for both missing-row and database failure.
- Finalization regression alters the frozen issue project, asserts capability refusal and verifies all three actual task-attributed runtime reads. Its final red control produced two reads on the original production implementation; the new expectation is sensitive to the missing capability instrumentation. The two earlier exploratory oracle attempts remain labelled in the diagnosis record.
- Snapshot regression first establishes a repeatable-read token snapshot, changes runtime ownership through another connection, then requires snapshot authorization to pass while current authorization fails. Both runtime reads must count. This distinguishes connection correctness from a counter-only fix. Fixtures restore runtime ownership and metrics using existing cleanup helpers; these tests are not parallel.
- Existing runtime wrapper tests canonically cover result classification, nil metrics and transaction-connection/collector binding. Existing capability suites cover both claim endpoints, resource limits, token lifecycle, current authority, snapshot/write recheck, inert comments and tool refusal; no duplicate broad permission matrix was added.
- The synchronized task capability spec accurately records the new signature, query/snapshot binding, source labels, unchanged error propagation and regression pointers. It does not claim provider/native/deployment acceptance.

## Verification

Reviewer independently ran `git diff --check` and `gofmt -l` for all seven changed Go files: both passed with no output. No DB tests were started by this reviewer while the implementer owned the focused run.

Read the implementer's raw focused logs and `results.json`. Independent log terminal counts:

```json
{
  "behavior-after": {
    "pass": 40,
    "skip": 0,
    "fail": 0,
    "race": 0
  },
  "auth-metrics-service-after": {
    "pass": 20,
    "skip": 0,
    "fail": 0,
    "race": 0
  },
  "focused-race": {
    "pass": 61,
    "skip": 0,
    "fail": 0,
    "race": 0
  }
}
```

Normal focused runs: 60 named tests/subtests passed across the two logs, zero fail/skip. Race focused run: 61 named tests/subtests passed, zero fail/skip and no DATA RACE marker. `results.json` records exit0 for these commands and for go vet over auth/metrics/service/handler; vet log identifies the owned router database and repository CLI guard. These are reviewed execution results, not a separately rerun suite. No real paid Agent execution occurred in this review.

The complete `make check` on a newly committed clean candidate is still required. Original full-check-01 remains exit2. Earlier 49-file integration review and its Knowledge alias repair approval remain unchanged; this review does not repeat or supersede the completed source audit.

## Exact reviewed scope

```json
{
  ".trellis/spec/server/task-api-capabilities.md": "7daa1ea5610b6d3917b8e4816d2f89d09f4dae97b166bdd8e0f273ef5b9f4a07",
  "server/internal/auth/task_capability.go": "7cd8a09b08224ee20584f97b0cacc7773e0649b22d8547c924d9b7749579f8c5",
  "server/internal/auth/task_capability_test.go": "b84335d3dee23c4085f6a59f97762ee246709157173a80fcad5da41e518f0344",
  "server/internal/handler/daemon.go": "df46018acf86d79f03a9a4c93c93ed1ee0fa9309b139cb8064efdacdf9faf008",
  "server/internal/handler/task_capability.go": "18a1e75f5a6150831137c6f9302733e9ec2fbc9c913fa2b5df9b81a39beaec98",
  "server/internal/handler/task_capability_runtime_lookup_test.go": "8aaf7e6e0812e19a2f1141d465eabe9d21782119c46cf48038bd6c90a8b32ae3",
  "server/internal/service/task.go": "88ff121c5cc48f216df43ef56e59393dd22435b50874028a5ac39c987039ff71",
  "server/internal/service/task_capability.go": "6aff9cd37776e74a6e46e26d69e2c59b99b813df70d099cc6027b16679159b23"
}
```
