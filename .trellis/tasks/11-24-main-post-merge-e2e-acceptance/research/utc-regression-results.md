# UTC targeted regression results

Status: **PASS** for the selected UTC regression scope. Candidate: `103e2bc329a9fb845d9f45c20b2a9bb3ebb54625`.

The actual run selected the current source's 41 top-level tests, allocated a fresh migrated database for each of three packages, and executed the unchanged Go tests with `TZ=UTC`. This is separate evidence from the full-module Asia/Shanghai normal/race runs; it is not a relabeling or replacement for those runs.

## Executed selection and results

| Package | Top-level PASS | Named PASS | Leaf PASS | Exit | Client connections after |
| --- | ---: | ---: | ---: | ---: | ---: |
| `internal/service` | 20 | 56 | 45 | 0 | 0 |
| `internal/handler` | 11 | 15 | 13 | 0 | 0 |
| `internal/inboxstate` | 10 | 22 | 18 | 0 | 0 |
| **Total** | **41** | **93** | **76** | **0** | **0** |

There were **zero failures, skips, missing top-level tests, unexpected top-level tests, duplicate terminals or unfinished named tests**. The retained mixed-event parser reported complete inventories for all three packages. No test assertion, timeout or source was changed.

- Service selection covers the stage internal-observation binding, current and retained completion previews, runtime memory/replay/authority, runtime comment identity and full-text recall. Its exact 20 names were selected from the already captured actual-platform AST inventory.
- Handler selection contains all 11 `Test` functions in `server/internal/handler/inbox_snooze_test.go`.
- Inbox-state selection contains all 10 `Test` functions in `server/internal/inboxstate/state_test.go`.
- The selected files contain no Redis integration dependency; the UTC run did not borrow or flush normal/race Redis logical databases.

## Commands and runtime identity

Each package used the same command form, with its exact frozen name regex and relative package:

```text
bash scripts/go-test-with-agent-cli-guard.sh -- <pinned-Go1.27> -C <current-source>/server test -json -count=1 -timeout=40m -p 1 -parallel 2 -run <exact-frozen-regex> <package>
```

Actual per-package argv, full selected names, start/end timestamps, source audits, environment digests and JSON inventories are in `qa/utc/*-receipt.json` and `qa/utc/inputs.json`. The CLI guard stayed active. No authenticated Agent CLI or model account was accessed.

The child environments explicitly selected UTC, Node22, the actual Go1.27 binary, the pinned Git tool, private Git-external HOME/TMP/GOTMPDIR, the existing cache/module paths and a fresh nondefault JWT key before each first test process. API-only pool/publisher, deployment credentials and task-root overrides were absent.

The source-owned PostgreSQL/Redis allocation was read and verified against the independently cleared backend resource plan. Before any database write, the exact container name, owner label, image, process and loopback endpoint matched; the template database identity and zero template clients were checked alongside connection headroom. Only three new UTC database clones were created:

- `qa_924fea80cfce_utc_cc8c4662_service`
- `qa_924fea80cfce_utc_cc8c4662_handler`
- `qa_924fea80cfce_utc_cc8c4662_inboxstate`

The backend template and normal/race databases were not modified or dropped. The owned UTC databases are retained for inspection and the coordinator's final resource cleanup.

## Source and evidence integrity

The current backend's reviewed source auditor checked the original **32,724 tracked paths/bytes/modes**, the exact six supplemental non-executable support bindings, current tools and backend clearance before execution and after each package. All checks passed; there were no generated-source exceptions or product modifications. The UTC launcher hash matched its frozen pre-execution hash at the end.

Evidence root: `artifacts/qa/main-post-merge-e2e-20261124-080504/utc/`.

| Raw stream | SHA-256 |
| --- | --- |
| `service-test.jsonl` | `87feedc835a8998cda55a09103f8a6412b4ce3e966f9b2e25286c2727a2275a9` |
| `handler-test.jsonl` | `15fb474d48f8ef2e75feb14c2f8278d72ee9236c755e73ff43c65b7e200d90f7` |
| `inboxstate-test.jsonl` | `b003265ca812c886b602c5f375fb8f684b6c9ece9ca145c1b7bcc88f53aa76cf` |

The complete machine-readable result is `qa/utc/result.json`; raw stderr and its hash are retained in each package receipt. Actual test execution ran from `2026-11-24T01:56:55Z` through `01:59:03Z`.

## Findings and limits

- No product or test defects were found within this selected UTC scope.
- Lint/TypeCheck: no product TypeScript or Go source changed. The task-private Python launcher passed `py_compile`; no Python static type checker/linter was installed or claimed.
- UTC covers this named 41-top regression selection, including the recent UTC JSON/time precision and retained-history concerns. It does not establish all-module UTC, race-under-UTC, browser/native, installed-client or commercial-model acceptance.
- No shared service was stopped. All three package databases had zero client connections after their test processes ended.
