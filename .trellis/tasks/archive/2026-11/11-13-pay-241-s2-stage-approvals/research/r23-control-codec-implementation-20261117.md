# R23 opaque conditional-version reply codec

Status: **private RED/GREEN, strict type-check and scoped lint passed; independent acceptance and ROOT integration pending.** The implementation lane changed only the private copies of `collaboration-stage-control.ts` and its owning `.node-test.mjs`. ROOT, Go, API, database, provider and original runtime request were not changed or replayed.

## Behavior and repair

R22 emitted a correlated successful rollback reply for request `61b4733a-f70e-424c-8f4b-c337ea9c3f52`. Its invocation snapshot contains `conditional_version = "1/after/01a1e575-3092-7cb9-bfa2-8a28d87745ff"`. The client applied its positive-decimal protocol-version validator to that opaque provider state and converted the valid reply into a generic unconfirmed result.

The server's existing `stageBoundedText(ConditionalVersion, 256)` contract permits nonempty, bounded valid UTF-8 text without NUL. The private GREEN changes exactly one reply validation call:

```typescript
text(last.external_request_key); text(last.conditional_version, 256);
```

The decimal `version` function, candidate/native/request initial-configuration validators, closed envelope, digests, UUIDs, strict duplicate-key parsing, 64 KiB frame limit, 20-second timer, socket permissions and no-replay behavior remain byte-identical. No dependency or second parser was added.

## Actual RED/GREEN and verification

| Check | Actual outcome | Result SHA256 |
| --- | --- | --- |
| Original codec with new regressions | 16 tests: 14 PASS / 2 FAIL, zero skip/cancel; source stable | `76205e5cf02fbf9db85c7833f363476d070d2439e7d15026773878752af9bf72` |
| Initial private GREEN | 16 PASS, zero fail/skip/cancel; source stable | `03ae6f78a6623edbfe3db977b9ee03c06ef1ac62079cac44f662defabf73f3c0` |
| Final GREEN with actual package context | Same 16 PASS, zero fail/skip/cancel; source stable | `d3adcd8b7e2db5d95f2afd3cfbc8beb316ebafb4d7cff392da1e9b20ef13c829` |
| Final strict codec type-check | exit 0, empty stdout/stderr; source stable | `d46651da0a50354190037db761a6fb2cd7f217bdfb1a0c2cd8d8e3b797dd21a5` |
| Final actual two-file lint | exit 0; both exact files covered; zero errors/warnings | `a2e55bf8449625ecb6f4d3f1c517e55ef1ffbb6815b95fac50e50c419fab3af4` |

The two original-code failures are exactly the retained successful-reply parser case (`Invalid stage control version`) and its one-send fake-socket confirmation case (generic unconfirmed error). All other controls pass on RED. New coverage preserves the exact opaque reply data, accepted 256-byte UTF-8 boundary, empty/null/missing/non-string/NUL/lone-surrogate/over-limit refusals, and strict request decimal variants. Existing malformed-envelope, duplicate, identity, digest/count, disconnection/no-replay, private-socket, frame/UTF-8 and completion-fault controls remain.

The original reply is parsed **offline as data**. Fake sockets use a fresh synthetic request identity and verify exactly one sent frame. No real request `61b…`, API, database, Go binary, provider or runtime harness was invoked.

Commands used pinned Node `v22.22.0`, installed TypeScript `5.9.3` and ESLint `9.39.4`, with explicit owned HOME/TMP directories and no ambient environment. Exact argv/cwd/environment/source hashes are retained in each run's `command.json`. Node tests use `--test --test-concurrency=1 --test-reporter=tap`. Type-check uses `--noEmit --strict --skipLibCheck --target ES2022 --module NodeNext --moduleResolution NodeNext --types node`. Scoped lint uses existing repo base rules, `--no-ignore`, an explicit private config and JSON output for the two exact files.

## Retained lint environment failure

Initial strict type-check passed. Initial lint reached the source but the existing `import-x/no-extraneous-dependencies` classifier failed with a null package path because the two-file private copy lacked an ancestor `package.json`. Its exit-2 result (`f69d5131…`), config and raw logs remain unchanged.

The coordinator supplied read-only links to the actual existing ROOT `package.json` (receipt `b337a707…`); installed dependencies and the adjacent immutable completion helper already had explicit support links (`2fef397d…`). No target was modified or package installed. New Node/type/lint r02 runs bind that actual package context. The lint rule was not disabled, and an ignored-file exit 0 was not counted as verification.

## Source and integration handoff

Private root: `/Volumes/artisan/.multica-acceptance/pay241-20261115-continuation/resume-r05/r23-control-opaque-version-r01`.

- `codec-red-r01` and its original results remain retained.
- `codec-green-r01/e2e/fixtures/collaboration-stage-control.ts`: SHA256 `a6e433acdb92f943cbe0c1288908ffcfe5ab667b2a0771106c7a090979e51c96`.
- The unchanged RED/GREEN test: SHA256 `479bc8b47e5d533a6cb1db728b857529c7a48b09ecf3b77b1fc03cf46334f3dc`.
- Exact one-field codec diff: `evidence/green-one-field-diff-r01.patch`, SHA256 `9e71916b9454d9c6d0880b82a5f06c00d2937fdaa540bc08173d6fb2487a3a22`.
- Guarded two-file ROOT integration patch: `evidence/ROOT-two-file-integration-r01.patch`, SHA256 `ef9728ee25cf87ab27983371c3cdc296157bf974388727b3bf6bde38fe043a60`.
- Full bounded handoff: `evidence/implementation-handoff-r01.json`, SHA256 `35a205a51b2f3f04ed42be98a987e171b94985a2f0170d688c0bdd46941f2f17`.

ROOT's selected two files still match their original `cd3d1e26…` / `af4b5aa1…` bytes and 0644 modes. The leader must recheck these guards immediately before integration and preserve peer edits.

The actual reply line is retained with SHA256 `b00bc0a507bf3c9d8652964f8880da4d6e2bdf826c247b13fc25ad82719c71c6`. The inspected first-24-line log prefix is `30e41954…`; later legitimate runtime log appends are separate observations. No original sent request byte frame was found or replayed.

Independent check, ROOT integration, fresh matched test-source/runtime binding and the original four-case/A01–A12 acceptance remain main-session gates. R22's 1 PASS / 1 FAIL / 2 NOT_RUN, its successful durable rollback and all earlier failures remain preserved. The original R20 lock holder is still unproved; this codec repair does not close S2, real S4 integration or the parent task.
