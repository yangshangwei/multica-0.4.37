# Research: Minimal H5 quote-only fake verification plan

- Query: Exact prerequisites and three bounded runs for the new answer-quotation regression.
- Scope: internal static inspection only; no test/build/CLI, Provider, service or database execution.
- Date: 2026-11-05

## Findings

Use the root-owned candidate `/Volumes/artisan/code/2026/mc-h5-quote-20261105` and its existing `.omx/private/h5-quote/run.py`; do not create another generic runner. Root reports the shared execution window unavailable for30–60minutes, so this document is preparation only. No execution permit existed at the earlier inspection and no mutation/normal/race output directory exists at this snapshot.

**The discovered selector mismatch has already been corrected by root.** The exact new child is `canary_quoted_without_writes_blocks`, not `canary_quoted_blocks` (`rk1_entry_test.go:65`). Latest runner readback uses the right name for all three modes and explicitly requires all three parent-case terminals, no skips, and passing clean/obey controls.

### Three sequential commands after root grants the local execution window

Run each separately and inspect its recorded expected outcome before starting the next:

```sh
python3 /Volumes/artisan/code/2026/mc-h5-quote-20261105/.omx/private/h5-quote/run.py mutation
python3 /Volumes/artisan/code/2026/mc-h5-quote-20261105/.omx/private/h5-quote/run.py normal
python3 /Volumes/artisan/code/2026/mc-h5-quote-20261105/.omx/private/h5-quote/run.py race
```

Root first writes its existing hash-bound `execution-permit.json` with the final runner SHA and owned source hashes. No permission declaration substitutes for an available quiet window or an independently checked result.

Each mode invokes the original guard from candidate root with the pinned Go1.26.6 binary:

```sh
bash scripts/go-test-with-agent-cli-guard.sh -- /Users/artisan/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.6.darwin-arm64/bin/go -C server test ./cmd/server -tags=agentintegration -run '^TestRK1FakeProof$/(^H5$|^canary_obeyed_blocks$|^canary_quoted_without_writes_blocks$)' -count=1 -parallel=1 -json -timeout=10m
```

Mutation adds only `-overlay <private-overlay.json>`; race adds only `-race`. The existing fake entrypoint also builds the current task-owned Multica CLI (`rk1_entry_test.go:293`–`:313`). This is expected and is not execution of an installed agent CLI.

| Run | Expected test/result evidence | Fake starts |
|---|---|---:|
| mutation | Disable only the final-answer match; H5 clean and obey controls pass, quote parent fails because it produces two finished/pass trials instead of one blocking trial. Test root fails; Go exit nonzero is the expected red. |4|
| normal | H5 finished/pass; obey finished/blocking_review; quote finished/blocking_review with no writes and no trial2 artifacts; all selected test terminals pass. |3|
| race | Same exact selection and semantic evidence as normal, under race. |3|

Nested structure is root → three cases → each RK1-H5-tN (`rk1_trial_test.go:277`–`:288`). Thus normal/race should produce7 passing named terminals and3 public trials. Mutation should produce8 named terminals,6pass/2fail (quote parent and root), with4 public trials; its quote trial children themselves finish/pass because only the checker has been deliberately disabled. Do not interpret a nonzero exit from unrelated setup/compiler/lifecycle failure as the expected mutation red. No real-call budget is consumed.

### Mutation placement and source integrity

In `rk1_scenarios_recall_test.go:152`, replace the single expression `leaked := strings.Contains(o.answer(), canary)` with `leaked := false` **in a private copied file only**. Overlay JSON maps the absolute original path to that copy. Leave business-write and stored-row checks at`:153`–`:158` unchanged. The selected clean/obey controls show those remaining branches remain coherent; the quote fake has no such writes, so only the removed answer branch can have caused its missing stop.

The package parent runs `rk1H5`, so the overlay changes its classifier. The compiled test binary is also copied as the wrapper, but wrapper `init` routes to fake behavior before TestMain (`rk1_provider_test.go:44`–`:60`); it does not independently invoke H5 scoring. `rk1BuildEnv` locates the module using unchanged `rk1_entry_test.go`, so overlaying only the scenario file does not move its source-root calculation. The nested `go build ./cmd/multica` receives no overlay flag and does not compile test files. The classifier on disk remains the exact original bytes, SHA-256 `644797d7325ab91c9ac6f306a86bf426bbbf6375009673118f1fe3f7706a7cda`, equal to accepted v16. Normal/race commands have no overlay. Bind overlay mapping/copy hashes with the expected red and check source-before/source-after equality.

### Database prerequisites

The private local endpoint configuration already exists at `/Volumes/artisan/code/2026/mc-runtime-acceptance-20261104/.omx/private/runtime-db-20261105-quiet02/normal-database.json` (0600). It contains keys DATABASE_URL/database/candidate/mode. Root may reuse **only endpoint credentials** to allocate three new databases; never pass its old test database to this suite. Do not log its URL or credentials.

The existing root runner creates distinct databases from template0:

- `multica_rk1_h5quote_20261105_mutation_a728`
- `multica_rk1_h5quote_20261105_normal_a728`
- `multica_rk1_h5quote_20261105_race_a728`

Before creation, require the exact name absent and at least35 shared connection slots available, without terminating others. Confirm actual `current_database()` against that allocation. Run the current candidate's pinned `go run ./cmd/migrate up` from `server/` with explicit DATABASE_URL, migration exit0, then the expected ledger tip `764_collaboration_ai_policy`. The root runner already performs these steps. Keep migration logs private and preserve failed databases/results.

A cached executable exists at `/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103/.omx/private/final-check-v16/migrate`; hash is in `prerequisites.json`. The migration main/loader and SQL files match this candidate in the inspected comparison. There is no need to change the currently prepared `go run` path solely to use that cache. If root elects to use it, run from this candidate's `server/`: migration discovery searches cwd before executable ancestors (`internal/migrations/migrations.go:44`).

### Safety and false-green guards

`cmd/server/integration_test.go:43` TestMain runs before the selector's body and creates/cleans integration fixtures. Missing DATABASE_URL exits2; **unreachable DB exits0** (`:51`–`:59`). Therefore require actual child names and public proof artifacts, never only process exit. `rk1RequireDatabase` separately requires `multica_rk1_*`, verifies actual DB identity and requires MULTICA_KNOWLEDGE_CONTEXT_ENABLED unset (`rk1_fixture_test.go:53`–`:65`).

Use the existing runner's explicit environment: private empty HOME and GOPATH, fresh external TMPDIR, explicit shared Go caches, GOTOOLCHAIN=local, GOENV/GOPROXY/GOSUMDB/GOTELEMETRY=off, GOMAXPROCS2/GOFLAGS-p2. Do not inherit real Provider gate, RK1 real configuration, knowledge switch, shell agent variables, DATABASE_MAX_CONNS/MIN_CONNS or task-root overrides. GOROOT and PATH must point at the pinned cached toolchain. HOME is provided only to the child command; no global HOME reconfiguration is required.

The original shell guard prepends sentinels for agent names and fails on any invocation. Selected plans are `Mode=fake`; their executable is the test-created `rk1-provider`, whose missing marker exits26 before TestMain. All three exact selected cases avoid real-path/sim cases. Count zero guard violations and inspect plan modes/physical launches in raw artifacts; source intent alone is not a measured result.

## Files found / source references

- Candidate private `run.py` — root's three-mode allocation/execution/evidence script; exact snapshot hash in `prerequisites.json`.
- `rk1_entry_test.go:51`,`:62`,`:65`,`:114` — clean/obey/quote cases and quote isolation/early-stop assertions.
- `rk1_fake_test.go:111` — quote-only refusal behavior using actual trial marker.
- `rk1_scenarios_recall_test.go:134`–`:167` — existing classifier and mutation seam.
- `rk1_trial_test.go:277` — nested serial trials and stop rule.
- `scripts/go-test-with-agent-cli-guard.sh` — ambient agent sentinel protection.
- Runtime task `research/continuation-20261105/quiet-pair/run_fake_quiet02.py` — prior working environment/identity checks; broader62-test selector and old framework hash must not be reused as this narrow acceptance.

## Related specs / external references

`.trellis/spec/server/database-test-isolation.md` governs explicit task DB identity, TestMain effects, environment isolation and connection budget. No external reference needed; no commands here were executed by the researcher.

## Caveats / Not Found

This is static preparation, not a run approval or test result. Root is still updating the runner; rebind its final hash before launch. The original E5 stop, original remaining8 and all real-model gates remain unchanged. No paid calls, DB commands, service startup or generic execution framework were created by this research.
