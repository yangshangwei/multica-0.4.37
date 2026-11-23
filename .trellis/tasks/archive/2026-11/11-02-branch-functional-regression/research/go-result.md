# Go first-run verification result

**Result: FAILED.** All 82 assigned packages finished. Full Go build and vet passed; five package commands failed. The Python orchestrator exited 0 because orchestration completed; this is not an all-tests-pass exit code. Original failures are preserved.

## Candidate and execution

- Candidate: `7af9ab9905c98c77bb091223ebe687c639475bb4`.
- Source checkout: `/Volumes/artisan/code/2026/mc-functional-7af9ab990-20261102`.
- Toolchain: `go version go1.27.0 darwin/arm64`; this is not a separate CI Go 1.26 execution.
- Started: `2026-11-02T00:09:13.310930+00:00`; finished: `2026-11-02T01:01:57.181349+00:00`.
- Commands: `go -C server build ./...`, `go -C server vet -p 2 ./...`, and per package `bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race -count=1 -timeout=40m -p 1 -parallel 2 -json <exact-package>`.
- Every DB-backed package first verified `current_database()` against its assigned permit. The child allocated, migrated, dropped and switched **zero** databases and did not start services.
- No Agent CLI guard violation was recorded. Commercial Provider calls: 0; no `agentintegration` build tag or live-account authorization was used.

## Actual counts

| Category | Pass / exit 0 | Fail / nonzero | Skip / no named tests |
| --- | ---: | ---: | ---: |
| Package commands | 77 | 5 | — |
| Packages with named tests | 68 | 5 | — |
| Packages without named tests | — | — | 9 |
| Top-level test identities | 10,325 | 23 | 35 |
| Nested subtest identities | 10,487 | 0 | 1 |

Top-level and nested counts are hierarchical and must not be added as independent product behaviours. The 77 zero-exit package commands include nine packages without named tests. `internal/service` completed 1,420 top-level and 1,920 nested passes with no failures/skips. `pkg/agent` completed 1,221 top-level passes, three top-level skips, 730 nested passes and one nested skip.

## Original failures and correction boundary

| Original package | Top-level passes | Top-level failures | Top-level skips | Classification |
| --- | ---: | ---: | ---: | --- |
| `06-cmd_server` | 363 | 3 | 7 | Proven coordinator environment contamination |
| `17-internal_cli` | 71 | 4 | 1 | Proven coordinator environment contamination |
| `20-internal_daemon` | 890 | 7 | 2 | Proven coordinator environment contamination |
| `22-internal_daemon_execenv` | 656 | 4 | 0 | Proven coordinator environment contamination |
| `33-internal_handler` | 2858 | 5 | 7 | Unresolved claim responses; fresh-DB replay required |

The first four packages contain 18 failures caused by global pool/config/workspace overrides. `evidence/go/environment-findings.md` records the source observations and exact originals. Coordinator correction v2 removed those overrides from 60 not-yet-started permits, starting at package 23, preserved v1 files/hashes, retained DB assignments and selected per-package private HOME with existing Go caches. The child consumed those corrected permit files without altering them. Already-running package 22 remains an original environment-contaminated failure.

`internal/handler` ran under corrected v2 environment and recorded five unresolved ordinary claim/context/mismatch failures returning `task:null`. See `evidence/go/handler-unresolved-findings.md`. Shared-fixture agent selection with unordered `LIMIT 1` is a hypothesis, not a proved cause. A read-only sample recorded 53/100 PostgreSQL connections without capacity rejection. Preserve this database and allocate a new one for unchanged isolated replay before classifying a product defect. An isolated replay alone does not establish a full-suite pass.

Subsequent coordinator-owned replays are intentionally not merged into this first-run result. They need their own command, environment, fresh DB and result evidence.

## Skips and limitations

35 top-level and one nested skip remain explicit. The complete identities and original reasons are in `evidence/go/skips.json`. They include 15 Redis-dependent top-level cases (14 auth, one server), Linux containment/signal/proc requirements, Windows-only behavior, unavailable pg_bigm, opt-in performance/wire/live-GitHub cases, an explicitly documented CLI unknown-field limitation, a stub-injection 500 invariant, and a catalog with no HTTP recipes. Platform-excluded build-tag code is outside the default inventory as well. None is counted as a functional pass.

## Evidence integrity and source boundary

Independent recomputation verified all 82 package identities against permits, every terminal test outcome/count, every original stdout/stderr SHA-256, all JSON event lines, all DB identities and unchanged candidate HEAD. There are no unaccounted cases or guard violations. See `evidence/go/final-evidence-validation.json`, `go-tests.csv`, `failures.json`, `skips.json`, `progress.json` and the 84 command directories.

The `server/` tracked status is clean at completion. Concurrent coordinator/build changes were observed in `apps/web/next-env.d.ts` and `e2e/collaboration-memory-web.spec.ts`; this child did not edit them. They do not change these Go server-source claims, but whole-candidate/E2E harness provenance must retain their separate hashes/diffs.

Only task evidence and these result documents were written by this child. No product code, assertions, timeout, dependency or task state was changed. This first run does not approve complete functional acceptance or release.
