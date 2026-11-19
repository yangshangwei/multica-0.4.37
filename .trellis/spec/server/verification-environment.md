# Joint verification environment

## 1. Scope / trigger

Apply when one private launcher runs Go package tests and deploys a task API from a shared configuration, or passes a generated environment file through Make and Bash. Database ownership follows [database test isolation](./database-test-isolation.md); this document covers configuration propagation and verified command admission.

## 2. Signatures

- Repository entry: `make check ENV_FILE=<private configuration>`.
- Private Go adapter: `MULTICA_TASK_PACKAGE_DB_MODE=1` allocates a fresh migrated clone per package; `0` admits a recursive test only against its recorded clone and package.
- Allocation record: `database`, `template`, canonical `package`, `state=running|complete`, command and actual named test inventory.
- Independent clearance: `candidate_sha`, `adapter_sha256` (`go`, `pnpm`), `launcher_sha256`, `source_env_sha256`, accepted `verdict`, `blocking_findings_open=0`.

## 3. Contracts

Keep API deployment choices separate from test-fixture choices:

```python
test_env = inherited_env.copy()
test_env.pop("DATABASE_MAX_CONNS", None)
test_env.pop("DATABASE_MIN_CONNS", None)
test_env.pop("MULTICA_RESOURCE_PUBLISH_DIR", None)
```

The publisher key is removed from the ordinary top-level package child, not globally. API startup, non-test/compile commands and a recursive test's explicitly selected publisher configuration retain their intended values. API pool overrides are removed from actual ordinary and recursive test branches. Individual tests may set their own environment.

`scripts/check.sh:prepare_check_environment()` generates a new 32-byte signing
key for each isolated check and appends its hexadecimal value as `JWT_SECRET`
only to the private task copy. Never inherit a deployment key into test actors
or leave the public development default in place: private history cursors
correctly reject that default. `scripts/check.test.sh` exercises missing, blank,
both public defaults and custom source values, verifies distinct generated keys
and mode0600 task files, and compares source bytes unchanged.

`MULTICA_RESOURCE_PUBLISH_DIR` enables organization-scoped catalog reads. The router's ordinary `TestMain` inserts user/workspace fixtures directly; it does not perform the public `CreateWorkspace` call that ensures an active internal organization. An API publishing path inherited by this fixture can therefore make an embedded-only skill catalog return503. Do not add an organization seed or change a product assertion to hide this configuration leak.

Use a private0700 HOME for the launcher child and a separate HOME per top-level Go package. Pin actual `GOCACHE`, `GOMODCACHE` and `GOPATH` explicitly. Recursive tests retain a test-selected HOME. Do not export global task config/workspace roots that override `t.Setenv("HOME", ...)`.

Parse the source environment as data. The verified private launcher rejects dollar signs and control characters after decoding values, before writing its canonical0600 file. Make `include/export` can evaluate `$(shell ...)` and `${shell ...}` even inside shell quotes; escaping the raw dollar is insufficient when decoding restores it. Verify the supported input grammar with the actual Make consumer as well as the downstream shell loader.

Check actual source/configuration hashes against the independent clearance before creating writable execution state or starting Make. Each complete run has new output/configuration/home names; preserve previous failures and their source bytes.

## 4. Validation / error matrix

| Condition | Required behavior |
| --- | --- |
| API pool values present | API retains its budget; Go tests start without those overrides |
| API publisher path present | Ordinary package test child starts without it; fixture-selected recursive/API configuration remains intact |
| Missing ordinary Go/SQL/evidence configuration | Refuse before Go test or SQL |
| Recursive package differs, multiple/wildcard selection, missing allocation | Refuse before test execution |
| Recursive `-modfile` / `-overlay` source rewrite | Reject the unsupported selection before list/test execution, including equals/double-dash forms |
| `-c` appears as an option value or after a binary-argument boundary | Do not grant compile-only admission |
| Legal global `-- test` / `-C directory -- test` | Enter the same test admission path; genuine `-- test -c` stays compile-only |
| Unknown pre-boundary Go test option | Refuse an unclassified command before forwarding |
| Decoded Make-active dollar/control value | Refuse before canonical file publication or Make |
| Script/config/candidate differs from review | Refuse; register and review the new bytes |
| No successful named test events | Do not classify a silent `TestMain` exit as tested behavior; distinguish compile-only packages |

## 5. Good / base / bad cases

Good: API max10/min1 and a task-owned publisher directory remain on the deployment; ordinary tests use a fresh database/HOME and attach their own publisher when needed. Base: a clearly supported compile command runs without `TestMain`. Bad: an API-only publishing option changes an embedded-only router fixture, then the test is weakened to accept503.

## 6. Required tests and assertion points

- Fake process propagation must distinguish ordinary package, recursive test-selected, non-test and compile environments; verify the publisher and pool fields at the actual process boundary.
- Keep the original skill-template router assertions: the same two top-level tests reproduced3 named failures with leaked publishing configuration, then7/7 passed in a separate fresh clone with the ordinary-test field removed.
- Verify package resolution under `-C`, default/relative selectors, argument values, Boolean/forwarded aliases, source-rewrite refusals and the legal global separator. Use cached Go list or compile-only probes for syntax; never execute `TestMain` just to prove parsing.
- Use credential-free, task-owned Make probes for both dollar expression forms and decoded escaped forms. Assert the harmless marker remains absent; a Bash-only fake Make does not verify Make's parser.
- Verify actual before/after hashes and distinct run paths. A passing preflight/targeted replay does not replace a new complete run; keep exact pass/fail/skip inventories.

## 7. Wrong vs correct

Wrong: export all deployment variables into every package and interpret shell-quoted configuration as safe for Make.

Correct: define each process's environment at its real boundary, verify every configuration consumer, bind the exact reviewed bytes, and keep original assertions and failed runs.

Evidence: `10-30-collaboration-fix-closure/research/continue-20261102/{final-v9-review.json,catalog-actual-redgreen.json}` and the versioned private diffs/red/green records in that task. These contracts do not change production catalog or organization behavior.

## Final verification identity

### 1. Scope / trigger

Apply when a long complete check validates source/artifact bindings only before
starting Make. A private counterexample changed tracked auth source or redirected
its workspace link after admission, and the unchanged launcher still recorded
`complete/exit0`. Initial admission therefore needs a separately required final
audit before acceptance.

### 2. Signatures

The task-private `final_state_audit.py --output <new JSON>` reuses the registered
`verify_owned_bindings` function. It exits2 on identity mismatch and creates its
report exclusively, mode0600. Run, review and clearance share `candidate_sha`,
`launcher_sha256`, `adapter_sha256` and `source_env_sha256`. The run's
`owned_bindings_sha256` equals review/clearance `bindings_sha256`; the run's
`resource_ownership_sha256` equals the clearance's resource-proof hash. Run
`dev_environment_source_id` must equal binding `source.checkout_source_id`;
the independent review labels that purpose under
`source_identities.development_HEAD_overlay`.

### 3. Contracts

Verify final HEAD/dirty inventory, tracked bytes/modes, pinned tools, declared
workspace links and actual exports, current CLI, and installed/staged desktop
artifacts against the initial reviewed inputs. Workspace dependencies must
resolve into their owning checkout; equal auth bytes at different physical
source paths do not establish one store identity.

Keep canonical product source ID, development HEAD/overlay ID and all-tracked
snapshot fingerprint separate. The current audit permits only the exact
`apps/web/next-env.d.ts` import rewrite from `.next/dev/types/routes.d.ts` to
`.next/types/routes.d.ts`, proved against the registered Git blob. Record its raw
and normalized hashes without rewriting candidate files. Do not expand this
exception to other generated files or arbitrary declaration edits.

Bind the audit script/review hashes to root clearance, then require all three:

```python
accepted = make_exit == 0 and audit["status"] == "pass" and inventory_complete
```

Root separately verifies live resource ownership and actual named test inventory;
the filesystem audit neither connects to the API/database nor establishes those
facts. A matched final snapshot can accompany a failed Make result.

### 4. Validation / error matrix

| Observation | Required result |
| --- | --- |
| Tracked source, tool, workspace edge or desktop artifact changed | Audit refuses; preserve failed run and register new bytes |
| Exact declared Next import rewrite only | Record raw/normalized identities; retain every other guard |
| Next mode/type/additional bytes changed | Refuse normalization |
| Run development identity or log differs | Refuse record binding |
| Make exit2 and filesystem identities match | Audit may pass; wholecheck remains false |
| Initial/fake/SSR pass without complete final audit | Keep complete acceptance open |

### 5. Good / base / bad cases

Good: one frozen complete run, its unchanged log/config, a reviewed final audit,
and exact named inventory all agree. Base: retain a passing identity audit for a
failed run to narrow diagnosis. Bad: treat preparation or a raw Make exit0 as
complete acceptance while source/workspace identity is unverified.

### 6. Required tests

Use credential-free temporary fixtures for clean/allowed generated import,
unrelated tracked mutation, foreign workspace link, artifact/tool/config/run-log
changes, Next mode/type, altered development identity, failed-Make reporting and
report overwrite refusal. The missing development-ID equality must fail before
its fix. Actual final execution remains a root gate after these private tests.

### 7. Wrong vs correct

Wrong: carry an initial review forward as final proof after Make exits.
Correct: run the hash-bound final auditor against actual current files and compare
its result with the complete command and inventory, preserving all failures.

Evidence: `10-30-collaboration-fix-closure/research/continue-20261103/` contains the
v11 counterexamples, final-state auditor and its independent16-case regression
review. These private tests do not claim a complete repository or Provider pass.

## Mixed build and test JSON events

### 1. Scope / trigger

Apply to recorders that consume `go test -json` and prove a complete named test
inventory. The stream can interleave build events and test events. Requiring
`Package` on every JSON object rejects legitimate toolchain output.

### 2. Signatures

The installed Go documentation (`go help buildjson`) defines `BuildEvent` with
`ImportPath`, `Action` and `Output`. Build actions are `build-output` and
`build-fail`; distinguish these from `TestEvent` by `Action`. `ImportPath` is a
build package ID and must not be substituted for `TestEvent.Package`.

```json
{"ImportPath":"","Action":"build-output","Output":"linker diagnostic\n"}
```

An empty string `ImportPath` occurred in the actual Go1.27 knowledge-service
run. This valid build event has no `Package` and no named test identity.

### 3. Contracts

Classify the known action before validating its event shape. Require string
`ImportPath` and `Output` for `build-output`; retain its raw bytes separately
from the test inventory. Treat `build-fail` as a build failure even when no
named tests have started. Test events still require the expected `Package` and
the existing run/terminal identity checks. Unknown actions, malformed JSON and
invalid event shapes make the inventory incomplete; do not skip them silently.

### 4. Validation / error matrix

| Event / result | Required classification |
| --- | --- |
| Valid `build-output`, including empty `ImportPath` | Build output; no test PASS or parse error |
| `build-fail` or nonzero command exit | Build/command failure; retain stderr and actual test terminals |
| Test event without the expected `Package` | Incomplete inventory |
| Unknown action or malformed event | Incomplete inventory, with the original line retained |
| Command exit0 and complete named terminal set | Eligible for test acceptance after source/configuration and resource checks |

### 5. Good / base / bad cases

Good: replay an immutable raw stream into a new parsing receipt that separately
counts build events and exact top-level/named/leaf test sets. Base: retain the
raw result while parser correctness is unresolved. Bad: count build output as
test failure, or accept exit0 while ignoring missing named terminals.

### 6. Required tests and assertion points

Use a minimal captured mixed stream to reproduce the old shape rejection.
Assert valid build output does not affect named denominators; build failures,
missing test-package fields, unknown actions and malformed JSON still prevent
acceptance. Reconcile the full raw stream against the frozen AST top-level set,
one terminal per named test, no unfinished names and the normal/race set match.
Bind the new parsing receipt to the original raw, command exit, source,
configuration, schema and final resource hashes. Preserve the original failed
recorder receipt; a parsing correction is not a new test execution.

### 7. Wrong vs correct

Wrong: the recorder reports shape errors, so rerun all database tests or weaken
all JSON validation. Correct: verify the installed toolchain's documented event
types, repair the event classifier in a new owned parser, and independently
reconcile the retained raw result before deciding whether execution is missing.

Evidence: `11-14-project-knowledge-automation/research/`
`p6-root-knowledge-service-verification-intake-20261115-292e45b21e81.json`
links the retained run whose normal raw SHA256 is
`2316db743f9b3d35dfd3518fb46de8bc8f6ba4d91b2de4227ef208f6c4f5214c`.
Its original recorder rejected 2,226 legitimate build-output events; the raw
test command exited0. Go1.27 `src/cmd/go/alldocs.go:2245` documents the mixed
stream. Independent normal/race acceptance is recorded in
`p6-root-knowledge-service-independent-v3-normal-race-20261115-87a4b72f8b7f.json`:
each mode has 194 top-level, 428 named and 365 leaf PASS, with identical sets and
zero failures, skips or incomplete identities. The private v3 parser passes
29 cases, including the missing/non-string named-output counterexamples and a
valid empty string. The original recorder failure and earlier parser versions
remain intact. This accepts the bounded service check, not final shared ROOT,
PAY P2 or product acceptance.

## Frozen source bytes and Git index blobs

### 1. Scope / trigger

Apply when publishing a private source snapshot or validating a newly staged
Git index. The repository's `.gitattributes` uses `* text=auto` and
`*.bat text eol=crlf`: a Windows BAT worktree can retain CRLF while its clean
Git blob contains LF.

### 2. Signatures

Use the pinned Git executable with the selected checkout and isolated Git
configuration. Path-aware clean hashing is:

```sh
git -C "$TASK_SOURCE" hash-object --path="$TASK_RELATIVE_PATH" --stdin \
  < "$TASK_SOURCE/$TASK_RELATIVE_PATH"
```

### 3. Contracts

Bind physical source bytes with the raw SHA256 manifest, full permissions,
discovered path set and workspace edges. Separately validate index paths,
executable modes and clean Git object IDs under the captured attributes.
Do not rewrite source files, attributes or raw manifests to make those two
representations equal. An index validation failure must preserve its receipt
and roll back the exact owned source delta before another reviewed attempt.

### 4. Validation / error matrix

| Observation | Required result |
| --- | --- |
| CRLF BAT source and its correct LF clean blob | Accept when raw source and index contracts both match |
| Raw byte, permission, path or workspace-edge drift | Refuse even if the clean Git blob is unchanged |
| Incorrect index path, executable mode or clean blob | Refuse and preserve the failed index |
| Failed publication with exact before-state restored | Accept rollback only; source readiness remains closed |

### 5. Good / base / bad cases

Good: raw snapshot and attribute-aware index checks both pass. Base: retain a
failed attempt and independently verify its complete rollback. Bad: assume
`SHA1(blob_header + raw_bytes)` must equal an attribute-normalized index entry.

### 6. Required tests and assertion points

Use a disposable repository with `*.bat text eol=crlf` and a CRLF BAT file.
Require the original raw-blob comparison to reject that valid staged index,
then require the corrected comparison to accept it. Keep separate raw-drift,
path/mode, partial-publication rollback and protected-configuration assertions.

### 7. Wrong vs correct

Wrong: normalize the BAT file or weaken the raw manifest guard after an index
mismatch. Correct: preserve its exact raw bytes and compare the index with
Git's path-aware clean object ID. The P6 `9b0291c83697` failure involved only
`scripts/uninstall-desktop-windows.bat`; its 389 CRLFs and executable mode were
unchanged, and the source/configuration rollback was independently accepted.
The new executor's CRLF regression changes from one named failure to passing
while all eight bounded fixture cases remain covered.
