# Step 6b: daemon half of the budget usage protocol v1 (implementation report)

Lane: `f3-daemon-6b`. Scratch worktree: `/Volumes/artisan/code/2026/mc-f3-budget-6b-20261118`
(detached at `86fea651c`). Go module root: `server/`. Times are local times of 2026-11-19
unless a date is given. Line numbers are those of the final files of section 2.

## 1. State

Round 2 (stages D, E and F) is described in section 11, and section 11 wins wherever an
earlier section says otherwise. Sections 1 to 10 are the round 1 text and were not brought
up to date: their file list, checksums, line numbers and counts are those of round 1.

Round 1 is built and verified: stages A, B and C with their tests. Stages D, E and F are not
started.

**Built**

- Stage A, names: the capability name `budget-usage-v1` as a constant, `Task.BudgetAttemptID`
  decoded from the claim, `agent.UsageNormalizerVersion`, one comment in `claude.go`.
- Stage B, record and batch builder: a journal directory that has to prove it can hold
  records, one record file per attempt written atomically, and the builder of the batch
  bytes in the frozen form of design 11.5.
- Stage C, capturing invocations: `handleTask` creates the record right before the run and
  seals it right after; the two launch sites of `runTask` go through a wrapper that records
  the launch intent before `executeAndDrain` and the outcome after it; three marks in
  `executeAndDrain` tell the backend's own Result from one it made up.
- 38 new tests in three files, all named `TestBudgetUsage...`. No existing test was changed.

**What a daemon built from this tree does**

Exactly what it did before. No production code opens the journal, so no record is created,
no capability is advertised and nothing is sent (section 10.1).

**Verified** (section 6, on the final files of section 2)

- All eight items of the brief's section 8 on both toolchains. Item 4: go1.27.0 PASS 3082,
  FAIL 0, SKIP 5 on the second run; the first run had 3 failures, all three the same 5 s
  timeout of `TestReasonixBackend...` in `pkg/agent` under load (section 6.3). go1.26.9
  PASS 3082, FAIL 0, SKIP 5. Baseline 3044, 0, 5. The only new names are the 38 tests.
- `-race`: the 38 tests and the whole daemon package, no data race.
- Mutation checks (section 7): 178 mutants, 177 killed, 1 survived and explained, 8 of 8
  controls pass.

**Open**

- One rule of the design is not implemented. The design gained a supplement on
  `adapter_version` at 04:55:36, after the verification of the final tree; the tree records
  the version as it is given (section 8). It changes nothing for a daemon built from round
  1 and is the first thing to do in round 2.
- One item needs a ruling (section 9): a record file that is behind its process when the
  process dies.

## 2. Files changed or added

Scratch worktree `/Volumes/artisan/code/2026/mc-f3-budget-6b-20261118`, `HEAD`
`86fea651cc5d09360327b6cf0f8357c1f360070d` (detached, unchanged). Nothing is staged and
nothing is committed. Paths are relative to `server/`. The checksums are those of the files
the verification of section 6 ran on (`shasum -a 256`, taken at 04:14 after the last run).

Changed (4 files, `git diff --numstat`: 30 lines added, 3 removed):

| File | + / - | sha256 | Change |
| --- | --- | --- | --- |
| `internal/daemon/daemon.go` | 19 / 3 | `fd551fe5a62607749c57961afe2a5cec3f1d2c6c45e5ea695f8c634a5145c6f7` | field `budgetUsage` on `Daemon`; three lines around `d.runner.run` in `handleTask`; one line after `agent.ResolveBackend` in `runTask`; the two launch sites call the wrapper; three marks in `executeAndDrain` |
| `internal/daemon/types.go` | 5 / 0 | `196b321bb7b38d5f147d7aa029c3076b79616f95346395e6f23d64d39c58c1ca` | `Task.BudgetAttemptID` |
| `pkg/agent/claude.go` | 1 / 0 | `3edaf7de65f9fb6aed71a10b69946d6da165d52b781be95a371772fec490892b` | one comment line above `claudeResultUsage` |
| `pkg/protocol/messages.go` | 5 / 0 | `49e6e75e0ab6c0cfcedfd6b0a1b14c2b7ae38a1c1260d2f4c005812ec818b312` | `DaemonCapabilityBudgetUsageV1` |

Added (7 files):

| File | Lines | sha256 | Content |
| --- | --- | --- | --- |
| `internal/daemon/budget_usage.go` | 406 | `3f5d6569d380913fa2348c566a68e2f7b760ea37f44b2669e9f9cf4683732598` | journal, record, the live record of an attempt, the context carrier |
| `internal/daemon/budget_usage_batch.go` | 127 | `32a42161623487752c8d51b52c4cea1274659fe9960c920e35adad1d1f3e1f40` | wire types and the batch builder |
| `internal/daemon/budget_usage_capture.go` | 156 | `a5d139c92a83892d8d2c46afffa3502bccfeccaa5c4440b7cae3dacdd01821be` | origin holder and marks, the outcome mapping, the wrapper, the two `handleTask` helpers |
| `pkg/agent/usage_normalizer.go` | 19 | `5fc57743ceb1e2b0512d14d4abcc754ebc387370bc764111af67d65d59c30343` | `UsageNormalizerVersion` |
| `internal/daemon/budget_usage_test.go` | 1197 | `1c8ae4002f4d5aac189dd0c2970d16345179d0681cb081a12f739f3c71b260a5` | 19 tests: names, journal, record, batch |
| `internal/daemon/budget_usage_capture_test.go` | 1513 | `dbddc8895e30d191ab427fc7536e3af89a00beb42d0a6bcad5284b45b5b2e5d4` | 12 tests: `executeAndDrain` and `handleTask` with function backends |
| `internal/daemon/budget_usage_runtask_unix_test.go` | 722 | `34e58a973b77775c4acab36caa0bdfd3194ebdfd74eef246241d4413abb6d5ae` | 7 tests through the real `runTask` with a test-created fake executable (`//go:build !windows`) |

No existing test file was edited. All 38 new top-level tests are named `TestBudgetUsage...`
and every helper they add is named `budgetUsageTest...`.

The complete change to existing code in `daemon.go` (context lines left out):

```
@@ type Daemon struct
+	// budgetUsage is the journal of usage report records (budget usage
+	// protocol v1). It is nil until this process has proven that it can keep
+	// records; until then a task runs without the protocol.
+	budgetUsage *budgetUsageJournal
+
@@ func (d *Daemon) handleTask
-	result, err := d.runner.run(runCtx, task, provider, slot, taskLog)
+	// Budget usage protocol v1: the record of the attempt exists before the
+	// run and is sealed as soon as the run is over. budgetAttempt is nil for a
+	// task without the protocol, and everything below is the legacy path.
+	budgetAttempt := d.beginBudgetUsageAttempt(task, provider, taskLog)
+	result, err := d.runner.run(withBudgetUsageAttempt(runCtx, budgetAttempt), task, provider, slot, taskLog)
+	d.finishBudgetUsageAttempt(budgetAttempt)
@@ func (d *Daemon) runTask, after the error check of agent.ResolveBackend
+	// The usage report of the attempt names the adapter and the CLI version
+	// this run launches. A task without a record carries a nil attempt.
+	budgetUsageAttemptFrom(ctx).describeLaunch(provider, usesCustomProfileCommand, backendConfig.CLIVersion)
@@ func (d *Daemon) runTask, first run
-	result, tools, err := d.executeAndDrain(ctx, backend, prompt, execOpts, taskLog, task.ID, env.CodexHome, &msgSeq)
+	result, tools, err := d.executeAndDrainRecorded(ctx, backend, prompt, execOpts, taskLog, task.ID, env.CodexHome, &msgSeq)
@@ func (d *Daemon) runTask, fresh-session retry
-		retryResult, retryTools, retryErr := d.executeAndDrain(ctx, backend, freshPrompt, execOpts, taskLog, task.ID, env.CodexHome, &msgSeq)
+		retryResult, retryTools, retryErr := d.executeAndDrainRecorded(ctx, backend, freshPrompt, execOpts, taskLog, task.ID, env.CodexHome, &msgSeq)
@@ func (d *Daemon) executeAndDrain, three places (section 5)
+		markBudgetUsageStartFailed(ctx)
+		markBudgetUsageBackendResult(ctx, ok)
+				markBudgetUsageBackendResult(ctx, ok)
```

## 3. Rules and stage items: where implemented, which test pins it

Paths are relative to `server/`. `budget_usage.go`, `budget_usage_batch.go` and
`budget_usage_capture.go` are in `internal/daemon/`. Test names are given without their
common prefix `TestBudgetUsage`; `RunTask...` tests are in
`budget_usage_runtask_unix_test.go` (`//go:build !windows`), the `HandleTask...`,
`Ended...`, `Marks...`, `InvocationOutcome`, `Resumed...`, `Wrapper...` and
`RecordIsOnDisk...` tests in `budget_usage_capture_test.go`, the rest in
`budget_usage_test.go`. "Pins" means: a mutant that breaks the item fails that test
(section 7).

### 3.1 Stage items (brief section 3)

**Stage A: names**

| Item | Where | Test |
| --- | --- | --- |
| Baseline of the untouched tree | `/tmp/f3-6b/baseline/`, numbers in section 6 | - |
| `DaemonCapabilityBudgetUsageV1 = "budget-usage-v1"`, next to `DaemonCapabilityAdminCancelAckV1` | `pkg/protocol/messages.go:44` (the other constant is at `:39`) | `CapabilityNameIsFrozen` |
| `Task.BudgetAttemptID`, tag `json:"budget_attempt_id,omitempty"` | `internal/daemon/types.go:214` | `TaskDecodesAttemptID` (present, absent, and left out of the encoding when empty) |
| `UsageNormalizerVersion(provider string) string`, `"1"` for every provider, with the doc comment asked for | `pkg/agent/usage_normalizer.go:15` | `NormalizerVersionIsOne` |
| One comment line at `claudeResultUsage` | `pkg/agent/claude.go:702` | - (comment) |

In round 1 nothing in production reads the capability constant: no code advertises it yet.

**Stage B: record and batch builder**

| Item | Where | Test |
| --- | --- | --- |
| Journal = directory, epoch (one UUID per open), server base URL | `budgetUsageJournal`, `budget_usage.go:137`; `openBudgetUsageJournal`, `:155` | `JournalOpen` |
| Open: create with mode 0700 | `os.MkdirAll(directory, 0o700)`, `:157` | `JournalOpen` (mode checked on POSIX) |
| Open: confirm it is a directory | the same `MkdirAll`: it fails when the path, or a parent of it, is a regular file (section 8, deviation 2) | `JournalOpenFailsOnUnusableDirectory` (two subtests, error names the step "create budget usage journal directory") |
| Open: write and remove a probe file | `writeManagementJSON(probe, ...)` then `os.Remove`, `:160-168` | `JournalOpen` (nothing left behind), `JournalOpenFailsOnUnusableDirectory` (modes 0500 and 0300: the error names the step "write budget usage journal probe"; mode 0300: nothing left behind) |
| Open: list it | `os.ReadDir`, `:169` | none can fail it alone here; see section 7, the one surviving mutant |
| Open fails: an error and no journal | every step returns `nil, err` | `JournalOpenFailsOnUnusableDirectory` |
| One file `<attempt-id>.json` per attempt, own version number | `recordPath`, `:181`; `budgetUsageRecordVersion`, `:35` | `RecordWriteAndReadBack` (exact file text), `HandleTaskRecordsAndSeals` (file names) |
| Written with the existing atomic helper | `write`, `:187`, calls `writeManagementJSON` (`managed_identity.go:205`: temporary file, fsync, rename, directory fsync) | `RecordWriteAndReadBack`, `RecordCreationFailures` |
| Record fields | `budgetUsageRecord`, `:52`; `budgetUsageInvocation`, `:77`; `budgetUsageCounts`, `:89` | `RecordWriteAndReadBack` pins the exact bytes of the file after creation and after sealing |
| Attempt id must be a canonical lower-case UUID, else no record, the task runs without the protocol, with a warning | `begin`, `:233` (`managedUUID`, `managed_identity.go:25`); warning in `beginBudgetUsageAttempt`, `budget_usage_capture.go:145` | `RecordRefusedForNonCanonicalAttemptID`, `HandleTaskRunsWithoutARecordThatCannotBeCreated`, `RunTaskWithoutTheProtocol` |
| Batch key generated at creation, matches `^[A-Za-z0-9_-]{16,64}$` | `BatchKey: uuid.NewString()`, `:252` | `RecordWriteAndReadBack`, `SealBuildsOnce`, `RecordIsOnDiskBeforeTheProviderStarts` |
| One attempt never gets two keys: a record in memory or on disk means no new record | `begin`, `:240-248` | `RecordRefusedForSecondAttempt`, `HandleTaskRunsWithoutARecordThatCannotBeCreated` (two cases), `RecordCreationFailures` |
| Every change rewrites the whole record | `persistLocked`, `:288`, called by `describeLaunch`, `beginInvocation`, `endInvocation`, `seal` | `RecordWriteAndReadBack` (file compared with memory after every change), `RecordIsOnDiskBeforeTheProviderStarts` |
| A failed write is logged and memory stays the truth | `persistLocked`, `:289-292` | `RecordWriteFailureKeepsMemory`, `HandleTaskSealsFromMemoryWhenTheJournalFails` |
| Batch builder: the exact bytes of design 11.5 | `buildBudgetUsageBatch`, `budget_usage_batch.go:89` | `BatchGolden`, `BatchUsageItems`, `BatchEnvelope` |

The record also keeps `server_url` (the accepted change of the plan). `read` (`:193`) loads a
record back: the name must be a canonical UUID, the file a regular file of at most 4 MiB,
the content one JSON value of on-disk version 1 that carries the same attempt id and a batch
key inside the pattern (`RecordReadRejectsForeignFiles`). Round 1 calls `read` from tests
only; round 2 needs it for the records a previous process left.

**Stage C: capturing invocations**

| Item | Where | Test |
| --- | --- | --- |
| `executeAndDrain` marks whether the Result is the backend's own; signature, return values and cancellation unchanged; no effect without an attempt | `daemon.go:8516`, `:8889`, `:8914` (section 5) | `MarksNameTheResultSource`, `EndedFollowsTheResultSource`, `WrapperWithoutAttemptIsExecuteAndDrain`; the existing tests that call `executeAndDrain` directly pass unchanged (section 6) |
| A wrapper with the same parameter list replaces the call at the two sites in `runTask` | `executeAndDrainRecorded`, `budget_usage_capture.go:105`; `daemon.go:7962` and `:8018` | `RunTaskFreshSessionRetryIsTwoInvocations` and the other `RunTask...` tests |
| Launch intent before the call, outcome with that call's own usage after it | `beginInvocation` at `budget_usage_capture.go:114`, `endInvocation` at `:118` | `RecordIsOnDiskBeforeTheProviderStarts` (the intent is on disk while `backend.Execute` runs), `EndedFollowsTheResultSource` |
| One line after `agent.ResolveBackend` gives the record the adapter name and the CLI version of this launch | `daemon.go:7786` | `RunTaskNamesTheAdapter` |
| `handleTask`: create the record immediately before `d.runner.run`, carry it in the context, seal right after | `daemon.go:5258-5260` | `HandleTaskRecordsAndSeals`, `HandleTaskSealsAnAttemptThatLaunchedNothing`, `HandleTaskCreatesNoRecordBeforeTheRun` |
| Sealing is where round 1 ends: nothing is sent | `finishBudgetUsageAttempt`, `budget_usage_capture.go:154`, calls `seal` and drops the bytes | `HandleTaskRecordsAndSeals` and `HandleTaskWithoutAttemptOrJournal` (the requests of a run are the legacy ones, none to another route) |
| No attempt id, or no journal: no record, no file, the same requests | `beginBudgetUsageAttempt`, `budget_usage_capture.go:127` | `HandleTaskWithoutAttemptOrJournal`, `RunTaskWithoutTheProtocol` |
| Everything downstream is safe on a nil handle | nil checks in `describeLaunch`, `beginInvocation`, `endInvocation`, `seal`; `withBudgetUsageAttempt` leaves the context alone for nil | `NilAttemptIsInert`, `HandleTaskWithoutAttemptOrJournal` |
| Only tests open the journal; `Run` is not touched | `openBudgetUsageJournal` has no caller outside `budget_usage_test.go`; `Daemon.budgetUsage` (`daemon.go:530`) is assigned only by test fixtures | `git diff` (section 2): no line of `Run` changed |

### 3.2 Rules (brief section 4)

| # | Rule (short) | Where | Tests that pin it |
| --- | --- | --- | --- |
| 1 | `resumed` is false only when the `ExecOptions` of this call has an empty `ResumeSessionID`, empty `ExtraArgs` and empty `CustomArgs`; slices judged as passed | `budget_usage_capture.go:113`, on the `opts` the wrapper received | `ResumedIsJudgedFromTheOptionsAsPassed` (through the real `executeAndDrain`; an empty non-nil slice is empty); `RunTaskResumedFollowsTheLaunchInput` (four inputs through the real `runTask` with a fake executable: nothing, daemon-level arguments, a custom `-p` that the claude adapter drops, a resume id); `RunTaskFreshSessionRetryIsTwoInvocations` (the refused resume is `true`, the retry `false`) |
| 2 | `adapter` is the provider key, or `custom-<provider key>` when `usesCustomProfileCommand` | `budgetUsageAdapterName`, `budget_usage.go:128`; seed in `beginBudgetUsageAttempt`, `budget_usage_capture.go:130-135`; the run's own value at `daemon.go:7786` | `AdapterName`; `RunTaskNamesTheAdapter` (own executable with and without a known version, custom profile, a profile that took the runtime over after the claim); `HandleTaskSealsAnAttemptThatLaunchedNothing` (the seed alone) |
| 3 | `ended` and usage follow where the Result came from and the status handed back | marks in `executeAndDrain` (section 5); `budgetUsageInvocationOutcome`, `budget_usage_capture.go:86`; `endInvocation`, `budget_usage.go:340`; `unknown` for an intent without outcome, `budget_usage_batch.go:110-114` | `EndedFollowsTheResultSource` (18 cases through the real `executeAndDrain`, one of them waits the real 10 s); `InvocationOutcome` (source by status matrix, every Result carrying usage); `MarksNameTheResultSource`; `RunTaskLaunchThatCannotStart`; `BatchEnvelope` (`unknown`) |
| 4 | Usage items: one per model sorted by model, four token strings always, `cost_ticks` only when > 0, an item left out only when it says nothing, nothing else checked | `budgetUsageWireItems`, `budget_usage_batch.go:60`; the copy into the record, `budgetUsageCountsFrom`, `budget_usage.go:107` | `BatchUsageItems` (cost only is sent, all zero is not, no `cost_ticks` at 0 and below 0, negative counts are sent as they are, order); `BatchGolden`; `HandleTaskRecordsAndSeals` |
| 5 | An invocation's usage is that call's own Result usage, before `reconcileFreshRetryResult` and `mergeUsage`; it is copied | the wrapper records the outcome at `budget_usage_capture.go:117-118`, before `runTask` reaches `reconcileFreshRetryResult` (`daemon.go:8033`); `budgetUsageCountsFrom` builds a new map of another type | `RunTaskFreshSessionRetryIsTwoInvocations` (3/1/0/0 and 20/9/4/2 in the batch, 23/10/4/2 on the legacy route); `RecordHandsOutCopies` |
| 6 | No launch sends `"invocations": []`; arrays are never null or left out; the two early returns of `handleTask` create no record | empty slices that are not nil at `budget_usage_batch.go:61`, `:100` and `:113`; `Invocations: []budgetUsageInvocation{}` at creation, `budget_usage.go:260`; record created only at `daemon.go:5258` | `BatchEnvelope`; `RunTaskThatLaunchesNothing`; `HandleTaskSealsAnAttemptThatLaunchedNothing`; `HandleTaskCreatesNoRecordBeforeTheRun` (untracked runtime, refused local directory) |
| 7 | Model discovery, version probes and `openclaw config` are not invocations; collaboration runs are out of scope | they do not pass through `executeAndDrain` (section 4); the only recording site is the wrapper | `RunTaskProbesAndDiscoveryAreNotInvocations` (the fake executable is started for `--version`, `--print`, `--help` and once for the task; the batch has one invocation) |
| 8 | `producer_epoch` is the epoch of the building process, `reported_at` the build time, `observed_at` of an `unknown` invocation its launch time; times are UTC, whole seconds | `seal`, `budget_usage.go:373` (`a.journal.epoch`, `a.journal.now()`); `budgetUsageWireTime`, `budget_usage_batch.go:51`; `:112` | `BatchGolden` (a zoned clock with a fraction of a second); `BatchEnvelope`; `SealBuildsOnce`; `SealNamesTheBuildingProcess` (the builder is another journal than the creator) |
| 9 | Bytes: field order of 11.5, `protocol_version` the number 1, no HTML escaping, `adapter_version` left out when empty, no other field | wire types declared in the order of 11.5, `budget_usage_batch.go:17-47`; `SetEscapeHTML(false)`, `:121`; the encoder's newline removed, `:126` | `BatchGolden` (the whole byte string, with a model called `<synthetic>`); `BatchEnvelope`; `HandleTaskRecordsAndSeals` |
| 10 | What was persisted is what is sent; a batch with bytes is never rebuilt; never a new key, attempt id or invocation id for something that has one | `sealedLocked`, `budget_usage.go:297`, checked by `describeLaunch`, `beginInvocation`, `endInvocation`, `seal`; `seal` returns a copy of the stored bytes; `begin` refuses a second record | `SealBuildsOnce` (a second seal an hour later returns the same bytes; a launch intent, an outcome and a launch description that arrive after the seal change nothing in memory or on disk); `RecordRefusedForSecondAttempt`; `RecordIsOnDiskBeforeTheProviderStarts` (key and invocation ids on disk before each launch are the ones in the batch); `RecordHandsOutCopies` |
| 11 | A journal that cannot be written never blocks a task | `beginBudgetUsageAttempt` returns nil on any error, `budget_usage_capture.go:138-147`; `persistLocked` only logs, `budget_usage.go:288` | `HandleTaskRunsWithoutARecordThatCannotBeCreated` (five reasons; the task runs without a record and is reported complete once, the warning names the reason, a record that already exists is left as it was); `HandleTaskSealsFromMemoryWhenTheJournalFails` (the journal directory is replaced by a file after the record was created; the launch that follows stays in memory and is sealed from it); `RecordCreationFailures`; `RecordWriteFailureKeepsMemory` |
| 12 | The legacy path stays byte for byte | no line of the legacy usage block (`daemon.go:5262-5271`), of the usage assembly in `runTask` or of `reconcileFreshRetryResult` changed; the wrapper returns what `executeAndDrain` returned | `HandleTaskWithoutAttemptOrJournal` (method, path, capability header and body of every request equal the run without the protocol, in three combinations); `RunTaskWithoutTheProtocol` and `RunTaskFreshSessionRetryIsTwoInvocations` (the exact body of the legacy usage report, with and without the protocol); `WrapperWithoutAttemptIsExecuteAndDrain` |
| 13 | No log line carries a request body or a response body | the three log calls of the feature (`budget_usage.go:290`, `:375`, `budget_usage_capture.go:145`) carry an attempt id, the name of the change and an error | `RecordWriteFailureKeepsMemory` and `HandleTaskSealsFromMemoryWhenTheJournalFails` (the captured log has one warning per failed write and contains neither the model name nor `"tokens"` nor `"invocations"`); `HandleTaskRecordsAndSeals` (the captured log contains neither `protocol_version` nor the batch key nor `"invocations"`) |

Notes on single rules:

- Rule 3, "status handed back": `EndedFollowsTheResultSource` has two cases for it. A backend
  Result `completed` that `executeAndDrain` re-tags `idle_watchdog` is `interrupted` with its
  usage; a backend Result `aborted` that `settleKnowledge` turns into `failed` is `returned`
  with its usage. A mutant that judges by the backend's status in either place fails.
- Rule 5: `runTask` never sees a merged value before the record does, because the wrapper
  returns only after `endInvocation`.
- Rule 6, "known limit": the two early returns create no record, so the server gets no batch
  for those attempts in round 2 either. This is the ruled limit of the brief, not decided here.
- Rule 8: in round 1 the building process is always the creating one. The test that tells
  them apart builds its attempt by hand around a record that a second journal read from disk.
  That stands in for round 2's adoption of a record a previous process left.
- Rule 9, `adapter_version`: the table states the rule of the brief, and the tree follows it.
  Design 11.10 has a further rule for this field since 2026-11-19 04:55, which the tree does
  not implement (section 8).
- Rule 10, "what was persisted is what is sent": round 1 sends nothing. What it fixes is that
  the stored bytes never change once they exist and that `seal` hands out exactly them.
- Rule 12: the rule names `daemon.go:8041-8056` for the usage assembly; in the changed file
  the same lines are 13 lines further down because of the lines added above them.

## 4. Launch sites verified

Line numbers are those of the scratch worktree after the change. Each list is the complete
result of a `grep` over `server/internal`, `server/pkg` and `server/cmd` (non-test files).

**`backend.Execute`** (two call sites in the whole server tree):

- `internal/daemon/daemon.go:8514`, inside `executeAndDrain`. This is the only place that
  launches the provider process of a claimed task.
- `internal/daemon/collaborationrun/host_recording.go:108`. Collaboration runs come from the
  collaboration poller and never pass through `handleTask` or `runTask`. Out of scope
  (rule 7); they carry no attempt and are not recorded.

**`executeAndDrain`** (declared at `daemon.go:8499`):

- Production callers: only the wrapper, `internal/daemon/budget_usage_capture.go:108` (no
  record in the context) and `:116` (with a record).
- The wrapper `executeAndDrainRecorded` is called at `daemon.go:7962` (first run) and
  `daemon.go:8018` (fresh-session retry). Before the change these two lines called
  `executeAndDrain` (the brief names them `:7949` and `:8005`).
- Existing tests call `executeAndDrain` directly 34 times: `daemon_test.go` 29,
  `tool_output_preview_test.go` 2, `cancel_receipts_test.go` 1, `knowledge_context_test.go` 1,
  `knowledge_server_integration_test.go` 1. (The brief says "about forty".) None of them was
  edited, and all of them pass a context without an origin holder.

**`runTask`** (declared at `daemon.go:6866`):

- `d.runner = taskRunnerFunc(d.runTask)` at `daemon.go:691`; the only `d.runner.run(...)` is
  `daemon.go:5259` in `handleTask`; the only caller of `handleTask` is the claim loop at
  `daemon.go:4946`.
- `internal/daemon/rk1k_driver_agentintegration.go:75` calls `runTask` directly, under the
  build tag `agentintegration`. It does not go through `handleTask`, so it has no record and
  the wrapper behaves as `executeAndDrain` there.

**Not invocations** (rule 7), none of which passes through `executeAndDrain`:

- Model discovery: `listModels` (package variable at `daemon.go:263`), called at
  `daemon.go:4170` (outside a task) and at `daemon.go:6752` (`resolveTaskModelSelection`,
  inside `runTask`).
- Version probes: `detectAgentVersion` (package variable at `daemon.go:257`), called at
  `daemon.go:1122`, `:2377` and `:2888`.
- `openclaw config`: `execenv/openclaw_config.go:748` (`openclawExec(ctx, bin, "config", "file")`).

**Hooks in `handleTask` and `runTask`:**

- `daemon.go:5258` creates the record, `:5259` carries it in the context of `runner.run`,
  `:5260` seals it. The two early returns before that point (`!tracked` at `:5140-5152`, the
  local-directory abort at `:5189-5203`) create no record.
- `daemon.go:7786`, the statement after the error check of `agent.ResolveBackend` (`:7780`),
  gives the record the adapter name and `backendConfig.CLIVersion`.
- The legacy usage block (`daemon.go:5262-5271`, `ReportTaskUsage` at `:5268`) is unchanged.

**Where the capability header reaches the two claim transports** (read for round 2; round 1
does not change any of it):

- One function produces the value: `(*Client).capabilities()` at `client.go:190`.
- HTTP: `setIdentityHeaders` (`client.go:177-188`) sets `X-Client-Capabilities` at `:187`. It
  is called from `ListWorkspaces` (`:729`), `postJSONViaObserved` (`:1197`),
  `getJSONWithToken` (`:1241`), `postJSONWithToken` (`:1275`) and `postKnowledge`
  (`knowledge_context.go:199`). The claim requests `ClaimTask` (`:233`), `ClaimTasks` (`:301`)
  and `claimTasksLegacy` (`:335`) go through the JSON post path.
- WebSocket: the handshake headers are built in `runTaskWakeupConnectionUsingTransport`,
  `wakeup.go:137` (`headers.Set("X-Client-Capabilities", client.capabilities())`), for the root
  client and for each managed child. The WS-first claims are `ClaimTasksWSFirst`
  (`wsrpc.go:316`), `claimTasksUsingTransport` (`wsrpc.go:323`) and
  `claimManagedTasksWSFirst` (`managed_wakeup.go:115`).
- So a flag read by `capabilities()` reaches both transports. Round 1 adds no such flag and
  does not change `capabilities()`. Its tests record the `X-Client-Capabilities` header of
  every request `handleTask` makes and require the requests to be the same with and without
  a journal (`TestBudgetUsageHandleTaskWithoutAttemptOrJournal`).

## 5. The change to `executeAndDrain`

Three statements were added, each a call on its own line. Nothing else in the function
changed: not the signature, not a return value, not a condition, not an existing line.

| Line | Statement | Place |
| --- | --- | --- |
| `daemon.go:8516` | `markBudgetUsageStartFailed(ctx)` | first statement of `if err != nil {` after `backend.Execute` |
| `daemon.go:8889` | `markBudgetUsageBackendResult(ctx, ok)` | first statement of `case result, ok := <-session.Result:` (outer select) |
| `daemon.go:8914` | `markBudgetUsageBackendResult(ctx, ok)` | first statement of the same receive inside the knowledge sub-branch of `case <-drainCtx.Done():` |

Both functions are in `budget_usage_capture.go`. Each looks up one value in the context
(`budgetUsageResultOriginKey{}`) and, when a holder is there, assigns one `int` field of it.

Why this preserves behaviour:

- **Without a holder nothing happens.** `ctx.Value` returns nil, the type assertion yields a
  nil pointer and the function returns. The 34 existing direct callers, the collaboration path
  and every task without a record pass a context without a holder. The wrapper adds the holder
  only when the context already carries a record, so "without an attempt in the context the
  marks do nothing" holds: `TestBudgetUsageWrapperWithoutAttemptIsExecuteAndDrain` checks that
  the launch context then carries no holder and that the wrapper returns what
  `executeAndDrain` returns for a launch that returns a Result and for one that cannot start.
- **The marks read the context `executeAndDrain` received (`ctx`), not `agentCtx` or
  `drainCtx`,** and they neither cancel nor derive anything. The holder is put there with
  `context.WithValue`, which adds no deadline and no cancellation, so the cancellation tree of
  the call is the one it had (`agentCtx` is still a child of the caller's context, `drainCtx`
  still a child of `agentCtx`).
- **They do not touch the Result.** The marks take `ctx` and the `ok` of the receive. `result`
  is not passed. What is handed back, including the `idle_watchdog` re-tag and
  `settleKnowledge`, is computed by the same statements as before.
- **No new blocking, goroutine, lock or channel operation.** A mark is a map-free lookup and
  an assignment. The select statements, their cases and their order are as they were.
- **No data race.** The holder is written only by the goroutine that runs `executeAndDrain`
  (all three marks are in the function body, none in a goroutine it starts) and read by the
  caller after the call returned. It is a plain field for that reason; see section 8,
  deviation 1. The `-race` run of section 6 covers it.
- **The made-up Results set no mark.** The holder's zero value means "made up", so the three
  made-up returns at the end of the function, the 10 s knowledge timeout and a Result channel
  that was closed without a value (`ok == false`) need no statement of their own. A future
  return path that forgets to mark is recorded as `interrupted` with empty usage, which
  under-reports and never invents usage.

What the marks mean for rule 3 (mapping in `budgetUsageInvocationOutcome`):

| Return path of `executeAndDrain` | Mark | `ended` | Usage recorded |
| --- | --- | --- | --- |
| `backend.Execute` returned an error (plain error, or the knowledge failure Result built from it) | failed start | `returned` | none |
| Outer receive, a value arrived, status handed back is `completed` or `failed` | backend | `returned` | that Result's |
| Outer receive, a value arrived, any other status handed back (including a Result re-tagged `idle_watchdog`) | backend | `interrupted` | that Result's |
| Knowledge sub-branch, the backend's Result arrived within the 10 s wait | backend | by status, as above | that Result's |
| Receive on a channel closed without a value (`ok == false`) | none | `interrupted` | none |
| Knowledge sub-branch, 10 s passed | none | `interrupted` | none |
| Made-up `idle_watchdog`, `cancelled`, `timeout` | none | `interrupted` | none |

The status is the one handed back: the mapping runs in the wrapper on the Result
`executeAndDrain` returned, after the re-tag and after `settleKnowledge`. Two consequences,
both pinned by `TestBudgetUsageEndedFollowsTheResultSource`:

- A backend Result with status `completed` that is re-tagged `idle_watchdog` is `interrupted`
  with its usage.
- A backend Result with status `aborted` that carries a knowledge failure is rewritten to
  `failed` by `settleKnowledge` and is therefore `returned` with its usage.

One existing behaviour is worth knowing and is not changed: on the made-up `cancelled` path
`awaitCancellationProcessResult` (`cancel_receipts.go:400`) may still receive the backend's
late Result and drops it. Its usage is lost for the record exactly as it is lost for the
legacy report today.

## 6. Verification

All commands ran from `server/` in the scratch worktree with
`TMPDIR='/private/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/'`, on darwin/arm64. Default
toolchain: `go version go1.27.0 darwin/arm64`. Second toolchain:
`GOTOOLCHAIN=go1.26.9 GOPROXY=off go version` prints `go version go1.26.9 darwin/arm64`; it
did not try to download. Counts are the top-level `--- PASS`, `--- FAIL` and `--- SKIP` lines
of the `-v` output. Outputs are under `/tmp/f3-6b/verify/` (scripts `quick.sh`,
`test-default.sh`, `test-rest.sh`, `test-default-run2.sh`, `compare-names.sh`).

### 6.1 Baseline (untouched tree at `86fea651c`, default toolchain)

| Command | Exit | Result |
| --- | --- | --- |
| `go build ./...` | 0 | no output |
| `go vet ./internal/daemon/... ./pkg/protocol/... ./pkg/agent/...` | 0 | no output |
| `go test -count=1 -v ./internal/daemon/... ./pkg/protocol/... ./pkg/agent/...` | 0 | PASS 3044, FAIL 0, SKIP 5; 7 packages `ok` |

The baseline was taken with the default toolchain only. Its 3049 result lines
(`/tmp/f3-6b/baseline/test.names.txt`) are what every later run is compared with.

### 6.2 Section 8 on the final tree

The final tree is the one whose checksums section 2 lists. Runs between 03:53 and 04:14.

| Item | Command | Toolchain | Exit | Result |
| --- | --- | --- | --- | --- |
| 1 | `gofmt -l` on the 11 files of section 2 | 1.27.0 | 0 | no output |
| 2 | `go build ./...` | 1.27.0 | 0 | no output |
| 3 | `go vet ./internal/daemon/... ./pkg/protocol/... ./pkg/agent/...` | 1.27.0 | 0 | no output |
| 4 | `go test -count=1 -v ./internal/daemon/... ./pkg/protocol/... ./pkg/agent/...`, first run | 1.27.0 | 1 | PASS 3079, FAIL 3, SKIP 5; `pkg/agent` failed, the six other packages `ok`. Three timeouts that are not caused by this change, see 6.3 |
| 4 | the same command, second run | 1.27.0 | 0 | PASS 3082, FAIL 0, SKIP 5; 7 packages `ok` |
| 5 | `go test -race -count=1 -v -run '^TestBudgetUsage' ./internal/daemon/` | 1.27.0 | 0 | PASS 38, FAIL 0, SKIP 0; no `WARNING: DATA RACE` |
| 5 | `go test -race -count=1 -v -timeout 40m ./internal/daemon/` (the whole package; it finished in 94.8 s) | 1.27.0 | 0 | PASS 943, FAIL 0, SKIP 2; no `WARNING: DATA RACE` |
| 6 | `GOTOOLCHAIN=go1.26.9 GOPROXY=off go build ./...` | 1.26.9 | 0 | no output |
| 6 | `GOTOOLCHAIN=go1.26.9 GOPROXY=off go vet ./internal/daemon/... ./pkg/protocol/... ./pkg/agent/...` | 1.26.9 | 0 | no output |
| 6 | `GOTOOLCHAIN=go1.26.9 GOPROXY=off go test -count=1 -v ./internal/daemon/... ./pkg/protocol/... ./pkg/agent/...` | 1.26.9 | 0 | PASS 3082, FAIL 0, SKIP 5; 7 packages `ok` |
| 7 | `GOOS=windows GOARCH=amd64 go build ./...` | 1.27.0 | 0 | no output |
| 7 | `GOOS=windows GOARCH=amd64 go vet ./internal/daemon/ ./pkg/agent/ ./pkg/protocol/` | 1.27.0 | 0 | no output |
| 7 | `GOOS=linux GOARCH=amd64 go build ./...` | 1.27.0 | 0 | no output |
| 8 | `git status --short`, `git diff --cached --name-only`, `git rev-parse HEAD` | - | 0 | 4 files ` M`, 7 files `??`, exactly those of section 2; nothing staged; `HEAD` is `86fea651cc5d09360327b6cf0f8357c1f360070d` |

The `-timeout 40m` on the whole-package race run is the only flag added to a command of the
brief. It was not needed.

**Comparison with the baseline (item 4).** `compare-names.sh` sorts the result lines
(status and test name, without the duration) of a run and compares them with the baseline's:

| Run | Lines | Baseline lines missing | New lines | New lines that are not `--- PASS: TestBudgetUsage...` |
| --- | --- | --- | --- | --- |
| default toolchain, second run | 3087 | 0 | 38 | 0 |
| go1.26.9 | 3087 | 0 | 38 | 0 |
| default toolchain, first run | 3087 | 3 | 41 | 3 (the three tests of 6.3, `FAIL` instead of `PASS`) |

So in the two green runs every test that passed in the baseline passes, the five skipped
tests are the baseline's five, and the only new names are the 38 tests of this change. The
result lines of the second default run and of the go1.26.9 run are identical (`cmp`).
None of the new tests or their subtests was skipped on this machine.

Package times in seconds:

| Package | Baseline | Final, 1.27.0 (second run) | Final, 1.26.9 |
| --- | --- | --- | --- |
| `internal/daemon` | 83.2 | 98.8 | 95.5 |
| `internal/daemon/collaborationrun` | 48.0 | 47.8 | 46.9 |
| `internal/daemon/execenv` | 64.3 | 63.6 | 62.4 |
| `internal/daemon/processtree` | 2.8 | 3.2 | 2.8 |
| `internal/daemon/repocache` | 7.5 | 6.9 | 5.6 |
| `pkg/protocol` | 0.8 | 1.3 | 3.3 |
| `pkg/agent` | 193.6 | 257.5 | 243.7 |

The new tests add about 15 s to `internal/daemon` (alone they take 13.9 s; ten of those are
the one case that waits the real 10 s of the knowledge branch, run in parallel with the
others). `pkg/agent` got no test; its time moves with the load of the machine.

### 6.3 The three failures of the first default run

`TestReasonixBackendCancelledStopReasonAborts`, `TestReasonixBackendQuestionFailsWithActionableError`
and `TestReasonixBackendStopReasonErrorFails` (`pkg/agent/reasonix_execute_unix_test.go`)
failed after 5.00 s each with `Status:timeout Error:reasonix timed out after 5s`. They start a
test-created fake executable and give it five seconds.

| Run | These three tests | Their four siblings `TestReasonixBackend...` |
| --- | --- | --- |
| Baseline (untouched tree) | PASS, 0.56 to 1.51 s | PASS, 0.00 to 1.63 s |
| Final tree, default, first run (03:54 to 03:58) | FAIL at 5.00 s | PASS, three of them at 3.34 to 4.89 s |
| Final tree, alone: `go test -count=3 -v -run '^TestReasonixBackend(CancelledStopReasonAborts\|QuestionFailsWithActionableError\|StopReasonErrorFails)$' ./pkg/agent/` | 9 of 9 PASS, 0.30 to 1.15 s, exit 0 | - |
| Final tree, go1.26.9 (03:58 to 04:02) | PASS, 2.06 to 2.98 s | PASS |
| Final tree, default, second run (04:09 to 04:13) | PASS, 3.10 to 3.31 s | PASS |

The change to `pkg/agent` is one comment line in `claude.go` and the new 19-line file
`usage_normalizer.go`; neither is reached by these tests. The times show a machine that was
three to five times slower than at the baseline while other agents were building on it. I
read this as the load noise the brief describes, in a test other than the two it names. It
was not investigated further and the test was not touched.

### 6.4 Runs on the tree before the last comment correction

After the first complete verification two doc comments in `budget_usage.go` were corrected
(no statement changed; the diff is `/tmp/f3-6b/comment-edit/comment-edit.diff`, 9 comment
lines). Everything above was then run again and is what 6.2 reports. The runs before the
correction (`/tmp/f3-6b/verify-before-comment-edit/`) were all green: default toolchain
exit 0 with PASS 3082, FAIL 0, SKIP 5; go1.26.9 exit 0 with PASS 3082, FAIL 0, SKIP 5; race
38/0/0 and 943/0/2 without a data race; items 1, 2, 3, 6 (build, vet), 7 and 8 exit 0.

### 6.5 Red states

Each stage's tests were written before its code and run once before the code existed:

| Stage | Command | Exit | Red state |
| --- | --- | --- | --- |
| A | `go test -count=1 -run 'TestBudgetUsage' ./internal/daemon/` | 1 | build failed: `undefined: protocol.DaemonCapabilityBudgetUsageV1`, `task.BudgetAttemptID undefined`, `undefined: agent.UsageNormalizerVersion` |
| B | the same | 1 | build failed: `undefined: budgetUsageJournal`, `openBudgetUsageJournal`, `budgetUsageSeed`, `budgetUsageRecord`, ... |
| C | the same | 1 | build failed: `unknown field budgetUsage in struct literal of type Daemon`, `executeAndDrainRecorded undefined`, `undefined: budgetUsageResultSource`, ... |

These red states are build failures, because the tests name identifiers the stage introduces.
They do not show that a single assertion can fail. The mutation checks of section 7 do: every
assertion-bearing behaviour was broken once and a test failed. The tests added after the
first mutation run were written after the code; their red state is the mutant they kill.

### 6.6 Two runs that section 8 does not ask for

Both ran on the final tree, with the default toolchain, after the runs of 6.2. Neither changed
a file of the worktree.

| Run | Command (from `server/`) | Exit | Result |
| --- | --- | --- | --- |
| Agent CLI guard, 05:41:19 to 05:41:33 | `../scripts/go-test-with-agent-cli-guard.sh go test -count=1 -v -run '^TestBudgetUsage' ./internal/daemon/` | 0 | PASS 38, FAIL 0, SKIP 0; no line "unexpected agent CLI invocation". The guard puts a failing stand-in for every name of `scripts/agent-cli-command-names.txt` first on `PATH`; none was started. Output: `/tmp/f3-6b/verify/9-guard-mine.txt` |
| Empty home directory, 05:08:50 to 05:09:04 | `go test -c -o /tmp/f3-6b/home-probe/daemon.test ./internal/daemon/`, then from `internal/daemon`: `HOME=/tmp/f3-6b/home-probe/home /tmp/f3-6b/home-probe/daemon.test -test.run '^TestBudgetUsage' -test.count=1 -test.v` (with the `TMPDIR` of section 6) | 0 | PASS 38, FAIL 0, SKIP 0; the directory had no entry afterwards (section 8, deviation 3). Output: `/tmp/f3-6b/home-probe/out.txt` |

The guard matters for `budget_usage_runtask_unix_test.go`: its provider is a shell script the
test writes into `t.TempDir()` under the name `claude`, and the version probe and the model
discovery of `runTask` run through that path too.

## 7. Mutation checks

### 7.1 Method

A mutant is a copy of a production file with one deliberate defect. The copies are under
`/tmp/f3-6b/mutants/<name>/` and are laid over the worktree with `go test -overlay`; the
worktree is never changed. The generator (`/tmp/f3-6b/mutants/gen.py`) replaces a piece of
text that must occur an exact number of times in the file (once, unless the definition says
otherwise) and stops when it does not, or when the replacement equals the original.

Each mutant was run once, from `server/`, with the default toolchain, four runners in
parallel (`/tmp/f3-6b/mutants/run.sh`):

```
go test -vet=off -overlay /tmp/f3-6b/mutants/<name>/overlay.json -count=1 -run 'TestBudgetUsage' ./internal/daemon/
```

`-vet=off` leaves the verdict to the tests: without it the vet checks that `go test` runs
first could reject a mutant before any test ran.

| Verdict | Meaning |
| --- | --- |
| KILLED | the exit code is not 0 and the output has a `--- FAIL` line of a test or a panic in a test |
| SURVIVED | the exit code is 0 |
| INVALID | the mutant did not compile (`[build failed]`). That proves nothing; the definition is rewritten |
| PASS (controls only) | the exit code is 0 |

A control is a copy of `budget_usage_capture.go` with one comment line added. Eight controls
ran as the first and the last entry of each of the four runners. They show that a run through
the overlay is green under the load of the four runners, so a red mutant run is red because
of the mutant.

Files mutated: `budget_usage.go` 77 mutants, `budget_usage_batch.go` 38,
`budget_usage_capture.go` 41, `daemon.go` 21, `types.go` 1, `pkg/protocol/messages.go` 1,
`pkg/agent/usage_normalizer.go` 1. That is 180 for 178 mutants because two of them
(`r03-status-judged-before-the-retag`, `r03-status-judged-before-the-knowledge-settle`) change
`budget_usage_capture.go` and `daemon.go` together. One mutant changes legacy code on purpose
(`r12-retry-usage-not-merged`), to show that the tests notice a changed legacy request.

### 7.2 Result

| | |
| --- | --- |
| Mutants | 178 |
| Killed | 177 |
| Survived | 1: `sb-open-without-listing`, see 7.3 |
| Invalid | 0 |
| Controls | 8 of 8 pass |

Each of the 38 new tests fails for at least one mutant, so none of them is a test that cannot
fail.

Seven mutants are killed by a panic and not by a failed assertion: `sc-no-journal-check`,
`sc-nil-guard-describe-launch`, `sc-nil-guard-begin-invocation`, `sc-nil-guard-end-invocation`,
`sc-nil-guard-seal`, `sc-origin-holder-nil-check` and `sc-result-mark-nil-check`. Each removes
a nil check, and the test that runs the case without a record or without a journal then
dereferences nil. A panic ends the test binary, so only that one test is listed for them.

### 7.3 The surviving mutant

`sb-open-without-listing` removes the `os.ReadDir` step of `openBudgetUsageJournal`
(`budget_usage.go:169-171`). No test fails, and I found no test that can fail on this machine:

- On POSIX the step cannot fail on its own. The probe write before it ends with the directory
  fsync of `writeManagementJSON` (`managed_identity.go:235-241`), which opens the directory
  for reading. Listing needs the same permission. A directory that cannot be listed (mode
  0300) is therefore refused one step earlier, at "write budget usage journal probe";
  `TestBudgetUsageJournalOpenFailsOnUnusableDirectory`, subtest "the directory cannot be
  listed", pins that refusal and that no probe file stays behind.
- On Windows `writeManagementJSON` skips the directory fsync, so there the listing is the only
  check that the directory can be read. Windows code cannot be run on this machine; it is
  cross-compiled only (section 6.2, item 7).

The line stays because stage B asks for it ("write and remove a probe file, list it") and
because of Windows. It is the one statement of round 1 for which a mutant exists and no test
fails. The mutant was run again against the final test files (04:36): exit 0, 38 PASS.

### 7.4 The three runs

| Run | When | Mutants | Killed | Survived | Invalid | Controls |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | about 01:56 to 02:12 | 174 | 150 | 18 | 6 | 8 of 8 pass |
| 2 | 02:57 to 03:15 | 178 | 177 | 1 | 0 | 8 of 8 pass |
| 3 | 03:21 to 03:23 | 14 of the 178, again | 14 | 0 | 0 | 2 of 2 pass |

After run 1:

- The 18 survivors were missing assertions, except the one of 7.3. Four tests were added
  (`TestBudgetUsageInvocationOutcome`, `TestBudgetUsageSealNamesTheBuildingProcess`,
  `TestBudgetUsageRecordHandsOutCopies`, `TestBudgetUsageRecordCreationFailures`) and six were
  strengthened (`RecordWriteAndReadBack`, `RecordReadRejectsForeignFiles`,
  `JournalOpenFailsOnUnusableDirectory`, `SealBuildsOnce`,
  `RecordIsOnDiskBeforeTheProviderStarts`, `HandleTaskRunsWithoutARecordThatCannotBeCreated`).
  The survivors were: `r03-failed-start-carries-result-usage`,
  `r03-made-up-carries-result-usage`, `r08-seal-epoch-from-record`,
  `r10-begin-ignores-unreadable-state`, `r10-seal-mints-a-new-batch-key`,
  `r11-record-kept-in-memory-when-it-cannot-be-created`, `r11-failed-create-leaves-the-file`,
  `sb-open-without-listing`, `sb-open-without-mkdir-error`, `sb-record-created-at-local`,
  `sb-record-invocations-null`, `sb-read-accepts-any-name`, `sb-read-accepts-non-regular`,
  `sb-read-accepts-oversized`, `sb-launch-time-local`, `sb-snapshot-shares-batch`,
  `sb-seal-returns-live-bytes`, `sb-empty-usage-stored-as-empty-map`.
- The six invalid definitions were rewritten so that they compile
  (`r03-wrapper-records-no-outcome`, `r04-items-in-map-order`, `r04-item-never-dropped`,
  `r12-wrapper-swallows-the-error`, `sb-snapshot-shares-invocations`,
  `sb-snapshot-shares-usage`).
- Four mutants were added: `sb-first-seal-returns-live-bytes`, `sb-observed-time-local`,
  `sb-read-follows-links`, `sb-failed-create-stays-registered`.
- No production file was changed because of a mutant: from the start of run 1 on, only test
  files changed, apart from the two doc comments named below.

After run 2 one precondition line was added to `TestBudgetUsageRecordHandsOutCopies`, so that
it fails with a message where it ended in a nil-map panic for `r03-outcome-drops-usage`. Run 3
repeated the 14 mutants that this test fails for, and two controls. The other 164 results are
those of run 2. The added line can only change how that one test fails, and for those 164
mutants that test did not fail.

What the mutants ran against, compared with the final tree:

- Test files: run 3 and the repeat of 7.3 used the final test files. Run 2 used them without
  the one precondition line.
- Production files: after run 2 started, no production file changed except two doc comments
  in `budget_usage.go` (section 6.4). The mutants were not run again after that. The 77 stored
  copies of that file therefore differ from the final file by their one defect and by those
  comment lines; a `diff` of the file before and after the correction, comment lines left
  out, is empty.

### 7.5 Statements without a mutant and without a test

- `seal`'s error branch (`budget_usage.go:374-377`): `buildBudgetUsageBatch` returns the error
  of `encoding/json`, which cannot fail for the wire types (strings, booleans, one integer,
  slices and structs of those). The branch cannot be reached by a test.
- The "remove budget usage journal probe" error return of `openBudgetUsageJournal`
  (`budget_usage.go:166-168`): removing a file that was created a moment ago in a directory
  that could just be written fails only in a race with another process.
- The `os.ReadFile` error return of `read` (`budget_usage.go:205-208`), after a successful
  `Lstat`. Ignoring that error would still end in the decode error, so no mutant can tell the
  two apart by more than the error text. `read` has no production caller in round 1.

### 7.6 Every mutant

The failing tests are written without their `TestBudgetUsage` prefix. A row shows the two
tests that fail for the fewest mutants overall, the most specific ones, and counts the rest.
The complete lists are in `/tmp/f3-6b/mutants/summary-final.tsv`, which is not durable; the
generator, the runner and every mutant's output are in the same directory.

**Rule 1: `resumed`** (7 mutants, 7 killed)

| Mutant | What is broken | Result | Tests that fail |
| --- | --- | --- | --- |
| `r01-resumed-ignores-resume-id` | resumed does not look at ResumeSessionID | KILLED | `ResumedIsJudgedFromTheOptionsAsPassed`, `RunTaskResumedFollowsTheLaunchInput` (+1 more) |
| `r01-resumed-ignores-extra-args` | resumed does not look at ExtraArgs | KILLED | `ResumedIsJudgedFromTheOptionsAsPassed`, `RunTaskResumedFollowsTheLaunchInput` |
| `r01-resumed-ignores-custom-args` | resumed does not look at CustomArgs | KILLED | `ResumedIsJudgedFromTheOptionsAsPassed`, `RunTaskResumedFollowsTheLaunchInput` |
| `r01-resumed-always-true` | resumed is always true | KILLED | `ResumedIsJudgedFromTheOptionsAsPassed`, `RunTaskLaunchThatCannotStart` (+7 more) |
| `r01-resumed-always-false` | resumed is always false | KILLED | `ResumedIsJudgedFromTheOptionsAsPassed`, `RunTaskResumedFollowsTheLaunchInput` (+1 more) |
| `r01-resumed-nil-instead-of-empty` | an empty non-nil argument list counts as resumed | KILLED | `ResumedIsJudgedFromTheOptionsAsPassed` |
| `r01-resumed-judged-after-adapter-filter` | custom arguments the claude adapter drops (-p) do not count | KILLED | `ResumedIsJudgedFromTheOptionsAsPassed`, `RunTaskResumedFollowsTheLaunchInput` |

**Rule 2: `adapter`** (9 mutants, 9 killed)

| Mutant | What is broken | Result | Tests that fail |
| --- | --- | --- | --- |
| `r02-adapter-name-never-custom` | budgetUsageAdapterName always returns the provider key | KILLED | `AdapterName`, `HandleTaskSealsAnAttemptThatLaunchedNothing` (+2 more) |
| `r02-adapter-name-always-custom` | budgetUsageAdapterName always returns custom-<provider> | KILLED | `AdapterName`, `RunTaskThatLaunchesNothing` (+3 more) |
| `r02-seed-not-custom` | the record is created with the provider key even for a custom profile | KILLED | `HandleTaskSealsAnAttemptThatLaunchedNothing` |
| `r02-no-describe-launch` | runTask does not describe the launch | KILLED | `RunTaskNamesTheAdapter` |
| `r02-describe-not-custom` | runTask describes every launch as the provider's own executable | KILLED | `RunTaskNamesTheAdapter` |
| `r02-describe-builtin-version` | runTask reports the built-in agent's version instead of the launch's | KILLED | `RunTaskNamesTheAdapter` |
| `r02-describe-no-version` | runTask reports no CLI version | KILLED | `RunTaskNamesTheAdapter` |
| `r02-describe-keeps-seed-adapter` | describeLaunch does not set the adapter | KILLED | `RecordWriteAndReadBack`, `RunTaskNamesTheAdapter` |
| `r02-describe-drops-version` | describeLaunch does not set the adapter version | KILLED | `RecordWriteFailureKeepsMemory`, `RecordWriteAndReadBack` (+1 more) |

**Rule 3: `ended` and the usage of an invocation** (23 mutants, 23 killed)

| Mutant | What is broken | Result | Tests that fail |
| --- | --- | --- | --- |
| `r03-failed-is-not-returned` | a backend Result with status failed is interrupted | KILLED | `InvocationOutcome`, `RecordIsOnDiskBeforeTheProviderStarts` (+3 more) |
| `r03-completed-is-not-returned` | a backend Result with status completed is interrupted | KILLED | `InvocationOutcome`, `RunTaskProbesAndDiscoveryAreNotInvocations` (+7 more) |
| `r03-backend-result-always-returned` | every backend Result is returned | KILLED | `InvocationOutcome`, `EndedFollowsTheResultSource` |
| `r03-backend-result-always-interrupted` | every backend Result is interrupted | KILLED | `InvocationOutcome`, `RecordIsOnDiskBeforeTheProviderStarts` (+8 more) |
| `r03-failed-start-interrupted` | a launch that could not start is interrupted | KILLED | `InvocationOutcome`, `RunTaskLaunchThatCannotStart` (+1 more) |
| `r03-failed-start-carries-result-usage` | a launch that could not start reports the Result's usage | KILLED | `InvocationOutcome` |
| `r03-made-up-returned` | a made-up Result is returned | KILLED | `InvocationOutcome`, `EndedFollowsTheResultSource` |
| `r03-made-up-carries-result-usage` | a made-up Result reports the Result's usage | KILLED | `InvocationOutcome` |
| `r03-status-decides-before-source` | status completed/failed is returned whatever the source | KILLED | `InvocationOutcome`, `EndedFollowsTheResultSource` |
| `r03-source-defaults-to-backend` | a return path without a mark counts as the backend's own Result | KILLED | `EndedFollowsTheResultSource` |
| `r03-no-start-failed-mark` | executeAndDrain does not mark a failed start | KILLED | `MarksNameTheResultSource`, `RunTaskLaunchThatCannotStart` (+1 more) |
| `r03-no-result-mark` | executeAndDrain does not mark the Result it received in its main select | KILLED | `MarksNameTheResultSource`, `RecordIsOnDiskBeforeTheProviderStarts` (+8 more) |
| `r03-no-knowledge-wait-mark` | executeAndDrain does not mark the Result that arrives within the 10 s knowledge wait | KILLED | `EndedFollowsTheResultSource` |
| `r03-closed-channel-counts-as-backend` | a Result channel closed without a value counts as the backend's Result | KILLED | `MarksNameTheResultSource` |
| `r03-start-failed-marked-as-backend` | a failed start is marked as a backend Result | KILLED | `MarksNameTheResultSource`, `RunTaskLaunchThatCannotStart` (+1 more) |
| `r03-wrapper-records-no-usage` | the wrapper records the outcome without usage | KILLED | `RecordIsOnDiskBeforeTheProviderStarts`, `RunTaskProbesAndDiscoveryAreNotInvocations` (+7 more) |
| `r03-wrapper-records-no-outcome` | the wrapper records only the launch intent | KILLED | `RunTaskLaunchThatCannotStart`, `RecordIsOnDiskBeforeTheProviderStarts` (+8 more) |
| `r03-wrapper-passes-no-origin-holder` | the wrapper does not ask executeAndDrain to mark the source | KILLED | `RunTaskLaunchThatCannotStart`, `RecordIsOnDiskBeforeTheProviderStarts` (+8 more) |
| `r03-outcome-always-returned` | endInvocation stores returned whatever it is told | KILLED | `EndedFollowsTheResultSource`, `RecordWriteAndReadBack` |
| `r03-outcome-drops-usage` | endInvocation does not store the usage | KILLED | `RecordHandsOutCopies`, `RecordWriteFailureKeepsMemory` (+10 more) |
| `r03-intent-only-reported-interrupted` | an invocation with only a launch intent is reported interrupted, not unknown | KILLED | `SealNamesTheBuildingProcess`, `BatchEnvelope` (+1 more) |
| `r03-status-judged-before-the-retag` | a Result re-tagged idle_watchdog is judged by the status the backend gave it | KILLED | `EndedFollowsTheResultSource` |
| `r03-status-judged-before-the-knowledge-settle` | a Result settled as a knowledge failure is judged by the status the backend gave it | KILLED | `EndedFollowsTheResultSource` |

**Rule 4: usage items** (21 mutants, 21 killed)

| Mutant | What is broken | Result | Tests that fail |
| --- | --- | --- | --- |
| `r04-items-sorted-backwards` | usage items are sorted by model in descending order | KILLED | `BatchUsageItems`, `BatchGolden` (+1 more) |
| `r04-items-in-map-order` | usage items are emitted in map iteration order (random) | KILLED | `BatchUsageItems`, `BatchGolden` |
| `r04-cost-emitted-when-zero` | cost_ticks is emitted when the cost is zero | KILLED | `BatchUsageItems`, `RunTaskProbesAndDiscoveryAreNotInvocations` (+7 more) |
| `r04-cost-emitted-when-negative` | cost_ticks is emitted when the cost is negative | KILLED | `BatchUsageItems` |
| `r04-cost-never-emitted` | cost_ticks is never emitted | KILLED | `BatchUsageItems`, `BatchGolden` (+1 more) |
| `r04-cost-key-always-present` | the cost_ticks key is present even without a cost | KILLED | `BatchUsageItems`, `BatchGolden` (+2 more) |
| `r04-item-dropped-despite-cost` | an item with zero tokens and a positive cost is left out | KILLED | `BatchUsageItems`, `BatchGolden` |
| `r04-item-kept-with-negative-cost` | an item with zero tokens and a negative cost is kept | KILLED | `BatchUsageItems` |
| `r04-item-never-dropped` | an item that says nothing is sent | KILLED | `BatchUsageItems`, `BatchGolden` |
| `r04-no-tokens-ignores-input` | the drop test does not look at input tokens | KILLED | `RecordWriteFailureKeepsMemory`, `BatchUsageItems` (+1 more) |
| `r04-no-tokens-ignores-output` | the drop test does not look at output tokens | KILLED | `BatchUsageItems`, `HandleTaskRecordsAndSeals` |
| `r04-no-tokens-ignores-cache-read` | the drop test does not look at cache read tokens | KILLED | `BatchUsageItems` |
| `r04-no-tokens-ignores-cache-write` | the drop test does not look at cache write tokens | KILLED | `BatchUsageItems` |
| `r04-negative-count-dropped` | an item whose counts are zero or negative is left out (a check on the server's behalf) | KILLED | `BatchUsageItems` |
| `r04-input-output-swapped` | input and output tokens are swapped on the wire | KILLED | `RecordWriteFailureKeepsMemory`, `BatchUsageItems` (+8 more) |
| `r04-cache-read-write-swapped` | cache read and cache write tokens are swapped on the wire | KILLED | `BatchUsageItems`, `RunTaskProbesAndDiscoveryAreNotInvocations` (+6 more) |
| `r04-input-not-decimal` | input tokens are written in base 16 | KILLED | `BatchUsageItems`, `RunTaskProbesAndDiscoveryAreNotInvocations` (+6 more) |
| `r04-cost-not-decimal` | cost_ticks is written in base 16 | KILLED | `BatchUsageItems`, `BatchGolden` |
| `r04-record-swaps-cache-counts` | the record stores cache read and cache write swapped | KILLED | `RecordIsOnDiskBeforeTheProviderStarts`, `RunTaskProbesAndDiscoveryAreNotInvocations` (+7 more) |
| `r04-record-swaps-input-output` | the record stores input and output swapped | KILLED | `RecordWriteFailureKeepsMemory`, `RecordIsOnDiskBeforeTheProviderStarts` (+9 more) |
| `r04-record-drops-cost` | the record does not store the cost | KILLED | `RecordIsOnDiskBeforeTheProviderStarts`, `EndedFollowsTheResultSource` (+2 more) |

**Rule 5: the usage of the call's own Result** (3 mutants, 3 killed)

| Mutant | What is broken | Result | Tests that fail |
| --- | --- | --- | --- |
| `r05-first-launch-not-recorded` | the first launch of runTask goes through executeAndDrain directly | KILLED | `RunTaskLaunchThatCannotStart`, `RunTaskProbesAndDiscoveryAreNotInvocations` (+4 more) |
| `r05-retry-launch-not-recorded` | the fresh-session retry goes through executeAndDrain directly | KILLED | `RunTaskFreshSessionRetryIsTwoInvocations` |
| `r05-usage-merged-with-earlier-invocation` | an invocation's usage includes the usage of the invocation before it | KILLED | `RecordWriteAndReadBack`, `RunTaskFreshSessionRetryIsTwoInvocations` (+1 more) |

**Rule 6: an attempt that launched nothing; arrays** (8 mutants, 8 killed)

| Mutant | What is broken | Result | Tests that fail |
| --- | --- | --- | --- |
| `r06-invocations-null-when-empty` | an attempt without invocations sends null | KILLED | `RunTaskThatLaunchesNothing`, `BatchEnvelope` (+1 more) |
| `r06-invocations-omitted-when-empty` | an attempt without invocations omits the array | KILLED | `RunTaskThatLaunchesNothing`, `BatchEnvelope` (+1 more) |
| `r06-usage-null-when-empty` | an invocation without usage sends null | KILLED | `RunTaskLaunchThatCannotStart`, `BatchUsageItems` |
| `r06-usage-omitted-when-empty` | an invocation without usage omits the array | KILLED | `BatchEnvelope`, `RunTaskLaunchThatCannotStart` (+2 more) |
| `r06-unknown-usage-null` | an unknown invocation sends usage null | KILLED | `BatchEnvelope`, `BatchGolden` |
| `r06-unknown-keeps-stored-usage` | an unknown invocation sends whatever usage the record holds | KILLED | `BatchEnvelope` |
| `r06-record-created-before-the-local-directory-check` | the record is created before handleTask's second early return | KILLED | `HandleTaskCreatesNoRecordBeforeTheRun` |
| `r06-record-created-for-an-untracked-runtime` | a record is created before handleTask's first early return | KILLED | `HandleTaskCreatesNoRecordBeforeTheRun` |

**Rule 7: probes and discovery are not invocations** (1 mutant, 1 killed)

| Mutant | What is broken | Result | Tests that fail |
| --- | --- | --- | --- |
| `r07-model-discovery-recorded` | the model discovery of runTask is recorded as an invocation | KILLED | `RunTaskProbesAndDiscoveryAreNotInvocations` |

**Rule 8: epoch and times** (8 mutants, 8 killed)

| Mutant | What is broken | Result | Tests that fail |
| --- | --- | --- | --- |
| `r08-batch-epoch-from-record` | the batch carries the epoch of the process that created the record | KILLED | `SealNamesTheBuildingProcess`, `BatchEnvelope` (+1 more) |
| `r08-seal-epoch-from-record` | seal passes the record's epoch instead of the journal's | KILLED | `SealNamesTheBuildingProcess` |
| `r08-reported-at-is-creation-time` | reported_at is the creation time of the record (builder) | KILLED | `SealBuildsOnce`, `BatchEnvelope` (+2 more) |
| `r08-seal-time-is-creation-time` | seal passes the creation time of the record as the build time | KILLED | `SealBuildsOnce`, `HandleTaskRecordsAndSeals` |
| `r08-unknown-observed-at-not-launch-time` | an unknown invocation does not get its launch time as observed_at | KILLED | `BatchEnvelope`, `BatchGolden` |
| `r08-observed-at-is-launch-time` | an ended invocation gets its launch time as observed_at | KILLED | `SealBuildsOnce`, `RecordWriteAndReadBack` (+1 more) |
| `r08-time-not-utc` | times keep their zone on the wire | KILLED | `BatchGolden` |
| `r08-time-with-fraction` | times carry fractional seconds on the wire | KILLED | `BatchGolden` |

**Rule 9: the bytes** (9 mutants, 9 killed)

| Mutant | What is broken | Result | Tests that fail |
| --- | --- | --- | --- |
| `r09-top-level-fields-swapped` | batch_key and producer_epoch change places | KILLED | `BatchEnvelope`, `HandleTaskSealsAnAttemptThatLaunchedNothing` (+2 more) |
| `r09-invocation-fields-swapped` | resumed and ended change places | KILLED | `BatchEnvelope`, `BatchGolden` (+2 more) |
| `r09-token-fields-swapped` | cache_read and cache_write change places | KILLED | `BatchUsageItems`, `BatchGolden` (+2 more) |
| `r09-protocol-version-2` | protocol_version is 2 | KILLED | `BatchEnvelope`, `HandleTaskSealsAnAttemptThatLaunchedNothing` (+2 more) |
| `r09-protocol-version-as-string` | protocol_version is a JSON string | KILLED | `BatchEnvelope`, `HandleTaskSealsAnAttemptThatLaunchedNothing` (+2 more) |
| `r09-html-escaping` | the batch is HTML-escaped | KILLED | `BatchUsageItems`, `BatchGolden` |
| `r09-adapter-version-always-present` | adapter_version is sent even when empty | KILLED | `RunTaskThatLaunchesNothing`, `BatchEnvelope` (+3 more) |
| `r09-extra-field` | the batch carries a field 11.5 does not list | KILLED | `BatchEnvelope`, `HandleTaskSealsAnAttemptThatLaunchedNothing` (+2 more) |
| `r09-trailing-newline` | the bytes end with the encoder's newline | KILLED | `RunTaskThatLaunchesNothing`, `BatchEnvelope` (+5 more) |

**Rule 10: what was persisted is what is sent** (11 mutants, 11 killed)

| Mutant | What is broken | Result | Tests that fail |
| --- | --- | --- | --- |
| `r10-seal-rebuilds` | sealing again rebuilds the batch | KILLED | `SealBuildsOnce` |
| `r10-begin-skips-live-check` | begin does not look at the records this process holds | KILLED | `HandleTaskRunsWithoutARecordThatCannotBeCreated` |
| `r10-begin-skips-disk-check` | begin does not look for a record file | KILLED | `RecordRefusedForSecondAttempt`, `RecordCreationFailures` (+1 more) |
| `r10-begin-ignores-unreadable-state` | begin goes on when it cannot tell whether a record file exists | KILLED | `RecordCreationFailures`, `HandleTaskRunsWithoutARecordThatCannotBeCreated` |
| `r10-invocation-added-after-seal` | a launch intent is appended to a sealed record | KILLED | `SealBuildsOnce` |
| `r10-outcome-changed-after-seal` | an outcome is written into a sealed record | KILLED | `SealBuildsOnce` |
| `r10-launch-described-after-seal` | the adapter of a sealed record is changed | KILLED | `SealBuildsOnce` |
| `r10-seal-mints-a-new-batch-key` | sealing replaces the batch key | KILLED | `SealNamesTheBuildingProcess`, `SealBuildsOnce` (+2 more) |
| `r10-outcome-mints-a-new-invocation-id` | recording the outcome replaces the invocation id | KILLED | `SealBuildsOnce`, `RecordWriteFailureKeepsMemory` (+2 more) |
| `r10-seal-returns-other-bytes` | seal returns bytes that differ from the persisted ones | KILLED | `SealNamesTheBuildingProcess`, `SealBuildsOnce` (+3 more) |
| `r10-sealed-bytes-not-persisted` | seal keeps the bytes in memory only | KILLED | `RunTaskThatLaunchesNothing`, `SealNamesTheBuildingProcess` (+14 more) |

**Rule 11: a journal that cannot be written never blocks a task** (5 mutants, 5 killed)

| Mutant | What is broken | Result | Tests that fail |
| --- | --- | --- | --- |
| `r11-task-not-run-without-a-record` | a task whose record cannot be created is not run | KILLED | `HandleTaskWithoutAttemptOrJournal`, `HandleTaskRunsWithoutARecordThatCannotBeCreated` (+1 more) |
| `r11-record-kept-in-memory-when-it-cannot-be-created` | begin ignores the write error and hands out the record | KILLED | `RecordCreationFailures`, `HandleTaskRunsWithoutARecordThatCannotBeCreated` |
| `r11-failed-create-leaves-the-file` | begin does not remove a file left by a write that failed after the rename | KILLED | `RecordCreationFailures` |
| `r11-write-failure-drops-the-invocations` | a failed write empties the record in memory | KILLED | `RecordWriteFailureKeepsMemory`, `HandleTaskSealsFromMemoryWhenTheJournalFails` |
| `r11-write-failure-is-silent` | a failed write is not logged | KILLED | `RecordWriteFailureKeepsMemory`, `HandleTaskSealsFromMemoryWhenTheJournalFails` |

**Rule 12: the legacy path** (5 mutants, 5 killed)

| Mutant | What is broken | Result | Tests that fail |
| --- | --- | --- | --- |
| `r12-legacy-report-skipped-with-a-record` | the legacy usage report is skipped for a task with a record | KILLED | `HandleTaskWithoutAttemptOrJournal`, `RunTaskWithoutTheProtocol` (+2 more) |
| `r12-wrapper-clears-result-usage` | the wrapper hands back a Result without usage | KILLED | `RunTaskWithoutTheProtocol`, `EndedFollowsTheResultSource` (+1 more) |
| `r12-wrapper-swallows-the-error` | the wrapper does not hand back executeAndDrain's error | KILLED | `RunTaskLaunchThatCannotStart`, `EndedFollowsTheResultSource` |
| `r12-wrapper-always-marks` | the wrapper takes the recording path without a record | KILLED | `WrapperWithoutAttemptIsExecuteAndDrain` |
| `r12-retry-usage-not-merged` | (legacy code) the fresh-session retry does not merge the first run's usage | KILLED | `RunTaskFreshSessionRetryIsTwoInvocations` |

**Rule 13: no body in a log line** (2 mutants, 2 killed)

| Mutant | What is broken | Result | Tests that fail |
| --- | --- | --- | --- |
| `r13-seal-logs-the-batch` | seal logs the batch bytes at debug level | KILLED | `RecordWriteFailureKeepsMemory`, `HandleTaskSealsFromMemoryWhenTheJournalFails` (+1 more) |
| `r13-write-failure-logs-the-record` | the write-failure warning carries the record | KILLED | `RecordWriteFailureKeepsMemory`, `HandleTaskSealsFromMemoryWhenTheJournalFails` |

**Stage A: names** (3 mutants, 3 killed)

| Mutant | What is broken | Result | Tests that fail |
| --- | --- | --- | --- |
| `sa-task-json-tag` | Task decodes the attempt id from another JSON key | KILLED | `TaskDecodesAttemptID` |
| `sa-normalizer-version-2` | UsageNormalizerVersion returns 2 | KILLED | `NormalizerVersionIsOne`, `HandleTaskSealsAnAttemptThatLaunchedNothing` (+2 more) |
| `sa-capability-value` | the capability constant has another value | KILLED | `CapabilityNameIsFrozen` |

**Stage B: record and batch builder** (41 mutants, 40 killed, 1 survived)

| Mutant | What is broken | Result | Tests that fail |
| --- | --- | --- | --- |
| `sb-open-without-probe` | opening the journal does not prove that a file can be written | KILLED | `JournalOpenFailsOnUnusableDirectory` |
| `sb-open-leaves-the-probe` | opening the journal leaves its probe file behind | KILLED | `RecordRefusedForNonCanonicalAttemptID`, `HandleTaskCreatesNoRecordBeforeTheRun` (+8 more) |
| `sb-open-failed-probe-left-behind` | a failed probe write is not cleaned up | KILLED | `JournalOpenFailsOnUnusableDirectory` |
| `sb-open-without-listing` | opening the journal does not list the directory | SURVIVED | none; see 7.3 |
| `sb-open-without-mkdir-error` | opening the journal ignores a directory that cannot be created | KILLED | `JournalOpenFailsOnUnusableDirectory` |
| `sb-directory-mode-755` | the journal directory is created with mode 0755 | KILLED | `JournalOpen` |
| `sb-epoch-is-constant` | every journal open has the same epoch | KILLED | `JournalOpen`, `SealNamesTheBuildingProcess` |
| `sb-journal-forgets-server-url` | the journal does not keep the server URL | KILLED | `JournalOpen`, `RecordWriteAndReadBack` (+1 more) |
| `sb-begin-accepts-any-id` | begin creates a record for a non-canonical attempt id | KILLED | `RecordRefusedForNonCanonicalAttemptID`, `HandleTaskRunsWithoutARecordThatCannotBeCreated` |
| `sb-batch-key-outside-pattern` | the batch key is too short for the pattern | KILLED | `RecordRefusedForSecondAttempt`, `RecordReadRejectsForeignFiles` (+15 more) |
| `sb-batch-key-is-attempt-id` | the batch key is the attempt id | KILLED | `HandleTaskSealsFromMemoryWhenTheJournalFails` |
| `sb-record-epoch-empty` | the record does not keep the creator's epoch | KILLED | `SealNamesTheBuildingProcess`, `RecordWriteAndReadBack` (+1 more) |
| `sb-record-created-at-local` | the creation time keeps the local zone | KILLED | `RecordRefusedForSecondAttempt`, `RecordHandsOutCopies` (+2 more) |
| `sb-record-without-server-url` | the record does not keep the server URL | KILLED | `RecordWriteAndReadBack`, `HandleTaskRecordsAndSeals` |
| `sb-record-without-workspace` | the record does not keep the workspace id | KILLED | `RecordWriteAndReadBack`, `HandleTaskRecordsAndSeals` |
| `sb-record-without-task` | the record does not keep the task id | KILLED | `RecordWriteAndReadBack`, `HandleTaskRecordsAndSeals` |
| `sb-record-without-normalizer` | the record does not keep the normalizer version | KILLED | `HandleTaskSealsAnAttemptThatLaunchedNothing`, `RecordWriteAndReadBack` (+2 more) |
| `sb-record-invocations-null` | a new record stores its invocations as null | KILLED | `RecordWriteAndReadBack` |
| `sb-record-version-0` | a new record carries on-disk version 0 | KILLED | `RecordRefusedForSecondAttempt`, `RecordReadRejectsForeignFiles` (+15 more) |
| `sb-begin-does-not-register` | begin does not keep the record in the process | KILLED | `HandleTaskWithoutAttemptOrJournal`, `HandleTaskRunsWithoutARecordThatCannotBeCreated` (+4 more) |
| `sb-read-accepts-any-name` | read accepts a name that is not a canonical UUID | KILLED | `RecordReadRejectsForeignFiles` |
| `sb-read-accepts-non-regular` | read accepts something that is not a regular file | KILLED | `RecordReadRejectsForeignFiles` |
| `sb-read-accepts-oversized` | read accepts a file larger than the bound | KILLED | `RecordReadRejectsForeignFiles` |
| `sb-read-accepts-other-version` | read accepts another on-disk version | KILLED | `RecordReadRejectsForeignFiles` |
| `sb-read-accepts-other-attempt` | read accepts the record of another attempt | KILLED | `RecordReadRejectsForeignFiles` |
| `sb-read-accepts-bad-batch-key` | read accepts a batch key outside the pattern | KILLED | `RecordReadRejectsForeignFiles` |
| `sb-intent-not-persisted` | a launch intent is not written to disk | KILLED | `SealNamesTheBuildingProcess`, `RecordWriteFailureKeepsMemory` (+3 more) |
| `sb-outcome-not-persisted` | an outcome is not written to disk | KILLED | `RecordHandsOutCopies`, `RecordWriteFailureKeepsMemory` (+3 more) |
| `sb-description-not-persisted` | the launch description is not written to disk | KILLED | `RecordWriteFailureKeepsMemory`, `RecordWriteAndReadBack` |
| `sb-launch-time-local` | the launch time keeps the local zone | KILLED | `RecordHandsOutCopies`, `EndedFollowsTheResultSource` (+1 more) |
| `sb-invocation-id-not-unique` | every invocation of an attempt gets the same id | KILLED | `RecordIsOnDiskBeforeTheProviderStarts`, `RecordWriteAndReadBack` (+2 more) |
| `sb-resumed-not-stored` | the record does not store resumed | KILLED | `ResumedIsJudgedFromTheOptionsAsPassed`, `RunTaskResumedFollowsTheLaunchInput` (+2 more) |
| `sb-snapshot-shares-invocations` | a snapshot shares the invocations with the live record | KILLED | `RecordHandsOutCopies` |
| `sb-snapshot-shares-usage` | a snapshot shares the usage maps with the live record | KILLED | `RecordHandsOutCopies` |
| `sb-snapshot-shares-batch` | a snapshot shares the batch bytes with the live record | KILLED | `RecordHandsOutCopies` |
| `sb-seal-returns-live-bytes` | sealing again returns the stored slice itself | KILLED | `RecordHandsOutCopies` |
| `sb-empty-usage-stored-as-empty-map` | an invocation without usage stores an empty map instead of nothing | KILLED | `EndedFollowsTheResultSource`, `RecordWriteAndReadBack` |
| `sb-first-seal-returns-live-bytes` | the first seal returns the stored slice itself | KILLED | `RecordHandsOutCopies` |
| `sb-observed-time-local` | the outcome time keeps the local zone | KILLED | `RecordHandsOutCopies`, `EndedFollowsTheResultSource` (+1 more) |
| `sb-read-follows-links` | read follows a symbolic link at the record path | KILLED | `RecordReadRejectsForeignFiles` |
| `sb-failed-create-stays-registered` | a record whose first write failed stays registered in the process | KILLED | `RecordCreationFailures`, `HandleTaskRunsWithoutARecordThatCannotBeCreated` |

**Stage C: capturing invocations** (22 mutants, 22 killed)

| Mutant | What is broken | Result | Tests that fail |
| --- | --- | --- | --- |
| `sc-not-sealed` | handleTask does not seal the record | KILLED | `RunTaskThatLaunchesNothing`, `RunTaskLaunchThatCannotStart` (+9 more) |
| `sc-seal-does-nothing` | finishBudgetUsageAttempt does not seal | KILLED | `RunTaskThatLaunchesNothing`, `RunTaskLaunchThatCannotStart` (+9 more) |
| `sc-sealed-before-the-run` | the record is sealed before the run | KILLED | `RunTaskLaunchThatCannotStart`, `RecordIsOnDiskBeforeTheProviderStarts` (+7 more) |
| `sc-record-not-carried` | the run does not carry the record | KILLED | `HandleTaskWithoutAttemptOrJournal`, `RunTaskLaunchThatCannotStart` (+8 more) |
| `sc-sealed-after-the-legacy-report` | the record is sealed after the legacy usage report | KILLED | `HandleTaskRecordsAndSeals` |
| `sc-no-attempt-id-check` | a task without an attempt id is not let through silently | KILLED | `HandleTaskWithoutAttemptOrJournal` |
| `sc-no-journal-check` | a daemon without a journal is not let through | KILLED | `HandleTaskWithoutAttemptOrJournal` (panic) |
| `sc-record-for-every-task` | a record is made for a task without an attempt id (id minted by the daemon) | KILLED | `HandleTaskWithoutAttemptOrJournal`, `RunTaskWithoutTheProtocol` |
| `sc-warning-always-names-the-id` | the warning carries whatever the claim put into the attempt id | KILLED | `HandleTaskRunsWithoutARecordThatCannotBeCreated` |
| `sc-warning-never-names-the-id` | the warning never names the attempt | KILLED | `HandleTaskRunsWithoutARecordThatCannotBeCreated` |
| `sc-no-warning` | a record that cannot be created is not logged | KILLED | `HandleTaskRunsWithoutARecordThatCannotBeCreated` |
| `sc-seed-swaps-task-and-workspace` | the record is created with the task id and the workspace id swapped | KILLED | `HandleTaskRecordsAndSeals` |
| `sc-seed-without-normalizer` | the record is created without a normalizer version | KILLED | `HandleTaskSealsAnAttemptThatLaunchedNothing`, `RunTaskNamesTheAdapter` (+1 more) |
| `sc-seed-other-attempt` | the record is created under the task id instead of the attempt id | KILLED | `HandleTaskWithoutAttemptOrJournal`, `RunTaskThatLaunchesNothing` (+11 more) |
| `sc-nil-guard-describe-launch` | describeLaunch is not safe on a nil attempt | KILLED | `NilAttemptIsInert` (panic) |
| `sc-nil-guard-begin-invocation` | beginInvocation is not safe on a nil attempt | KILLED | `NilAttemptIsInert` (panic) |
| `sc-nil-guard-end-invocation` | endInvocation is not safe on a nil attempt | KILLED | `NilAttemptIsInert` (panic) |
| `sc-nil-guard-seal` | seal is not safe on a nil attempt | KILLED | `HandleTaskWithoutAttemptOrJournal` (panic) |
| `sc-context-carries-a-nil-attempt` | a nil attempt is stored in the context | KILLED | `NilAttemptIsInert` |
| `sc-origin-holder-nil-check` | the start-failed mark writes through a missing holder | KILLED | `WrapperWithoutAttemptIsExecuteAndDrain` (panic) |
| `sc-result-mark-nil-check` | the Result mark writes through a missing holder | KILLED | `HandleTaskWithoutAttemptOrJournal` (panic) |
| `sc-intent-after-the-launch` | the launch intent is written after the provider ended | KILLED | `RecordIsOnDiskBeforeTheProviderStarts` |

## 8. Deviations from the design or the accepted plan

The wire format (design 11.5), the route and the capability name are as frozen. I found no
place where the design contradicts the existing code.

**One rule of the design is not implemented: `adapter_version` is recorded as it is given.**
`design.md` was changed at 04:55:36 on 2026-11-19, after the last verification run of the
final tree (04:14) and while this report was being written; I saw the change at 05:14. The
new last bullet of the "2026-11-18 补充" list of 11.10 (line 696, "2026-11-19 补充（主会话审
6b 第一轮时定）") says: before the daemon records the CLI version it trims white space; if
what is left is not 1 to 120 bytes of valid UTF-8, or holds a rune for which
`unicode.IsControl` is true, there is no version and the field is left out. The round 1 tree
does not do this. `describeLaunch` stores the value it is given (`budget_usage.go:313`) and
the builder copies it into the envelope (`budget_usage_batch.go:95`), leaving the field out
only when the value is empty. The value comes from `extractVersionLine`
(`pkg/agent/claude.go:1304`): the first line of the `--version` output that carries a version
token, or, when no line does, the whole trimmed output. For a custom profile command that
output can be longer than 120 bytes or have line breaks inside.

The design wins, so the code has to change. I did not change it in round 1:

- A daemon built from the round 1 tree is not affected. No production code opens the journal,
  so no record is written and nothing is sent. The rule matters from stage D on.
- A change to `describeLaunch` now would make all 178 mutants and the whole run of section
  6.2 describe a tree that no longer exists. Round 2 runs both again on its final tree.
- The supplement says it was decided while the main session reviewed round 1, so it reads as
  a result of that review.

It is the first item of section 10.2. The tests of round 1 pin this field for two plain
versions (`2.1.34` and `9.9.9`) and for an empty one: the record and the batch carry the
version the launch resolved, and an empty version leaves the field out. No test gives it
white space, a long value, a line break or invalid UTF-8.

The points below differ from the accepted plan (`/tmp/f3-main-s6b/prior-plan.md`) or from the
wording of the brief.

1. **The origin holder is a plain field, not an atomic value.** The plan says "a per-call
   origin holder (atomic value)". The holder (`budgetUsageResultOrigin`,
   `budget_usage_capture.go:40`) is written by the three marks, which run on the goroutine
   that runs `executeAndDrain`, and read by the wrapper after that call returned, on the same
   goroutine. Its context key is an unexported type of package `daemon`, so no backend can
   reach it. An atomic would guard an access that does not exist. Both `-race` runs of
   section 6.2 are clean.
2. **No separate "is a directory" check after `os.MkdirAll`.** The plan lists "`os.MkdirAll` +
   is a directory + probe write + `os.ReadDir`". `os.MkdirAll` already returns an error when
   the path, or a parent of it, exists and is not a directory, so a `Stat` and `IsDir` after
   its success could never fail and no test could pin it. A first version had the check; it
   was removed for that reason. What is pinned is that the open fails at the step "create
   budget usage journal directory" for a regular file at the path and for one above it.
3. **The tests through the real `runTask` do not set `HOME`.** The plan has
   `t.Setenv("HOME", t.TempDir())`. These tests run in parallel, and `t.Setenv` cannot be
   used in a parallel test; the existing `leader_workdir_reuse_test.go` runs the real
   `runTask` with a fake script the same way. The workspaces root, the journal and the fake
   executable are in `t.TempDir()`. That nothing is written under the home directory was
   checked: the test binary was built into `/tmp/f3-6b/home-probe/` and the 38 tests were run
   from it with `HOME` set to an empty directory there. Exit 0, PASS 38, FAIL 0, SKIP 0, and
   the directory was still empty afterwards. Reads were not measured. The two lookups of the
   home directory I found on the claude path of `runTask` need an agent MCP configuration
   (`mergeRuntimeAndAgentMcpConfig`) or a knowledge context (`prepareKnowledgePrompt`); the
   tasks of these tests carry neither.
4. **A journal that fails mid-run is a directory replaced by a regular file, and permission
   bits.** The plan says "simulated by removing the directory during the run". The tests
   remove the directory and write a regular file at its path, so that every later write fails
   whatever the code does; a write that silently re-created the directory would otherwise
   pass. That state never reaches the write of `begin`, because the check for an existing
   record fails first. The steps "the write fails" and "the write fails after the rename" are
   therefore reached with directory modes 0500 and 0300 (`TestBudgetUsageRecordCreationFailures`).
   Those cases are skipped on Windows and for root; on this machine they ran.
5. **The test of `UsageNormalizerVersion` is in `internal/daemon/budget_usage_test.go`.** The
   brief allows one new file in `pkg/agent`, which is `usage_normalizer.go`. A test file
   there would be a second one.
6. **34 direct calls, not "about forty".** The brief and the plan say that about forty
   existing tests call `executeAndDrain` directly. There are 34 calls in five test files
   (section 4). Nothing follows from the difference.
7. **`begin` removes the file of a record whose first write failed** (`budget_usage.go:262-268`).
   Neither the plan nor the brief says this. The atomic helper can fail at the directory
   fsync, after the rename. The file then exists although `begin` returns an error and the
   task runs without the protocol. Left there, it would be a record without invocations, which
   round 2's restart path would report as an attempt that launched nothing while the run did
   launch a provider. `openBudgetUsageJournal` removes its probe file in the same case. The
   removal is best effort; section 9 names what remains.
8. **`read` refuses more than the plan asks.** It requires a canonical UUID as the name, a
   regular file (`Lstat`, so a symbolic link is refused) of at most 4 MiB
   (`maxBudgetUsageRecordBytes`, `budget_usage.go:37`), one JSON value, on-disk version 1, the
   attempt id of the name and a batch key inside the pattern. `read` is for round 2, which
   reads whatever a previous process left in the directory; the bound keeps a foreign or
   damaged file from being read into memory whole. A record of this daemon is a few kilobytes.
   Round 1 calls `read` from tests only.

Not deviations, stated for completeness: the record keeps `server_url` (the accepted change
of the plan); the whole-package race run carries `-timeout 40m` (section 6.2); the tests added
after the first mutation run were written after the code (section 6.5).

## 9. Needs a ruling

One item. Nothing in round 1 depends on it: it is about what round 2's restart path reports,
and whether stage B's write path has to change for that.

1. **A record file that is behind its process when the process dies.**
   Rule 11 and design 11.10 (line 698) settle the process that is alive: a write that fails
   is logged, the invocation stays in memory, and it is sealed and reported from there. They
   do not say what the next process may conclude from the file. Every change rewrites the
   whole record, so a file is behind only from a failed write until the next write that
   succeeds. If the process dies inside that window, the next process has only the file, and
   the design tells it to report a record of an earlier process that has no batch bytes
   (invocations without an outcome as `unknown`). What the file lacks depends on the write
   that failed:

   - The outcome of an invocation (`endInvocation`) or the batch bytes (`seal`). The launch
     intent is in the file. The restart reports the invocation as `unknown`, or builds the
     batch again. Nothing false is said.
   - The launch intent (`beginInvocation`, `budget_usage.go:334`). The invocation is not in
     the file at all, although the provider process was started. The restart's batch lists
     fewer provider processes than ran. For the first launch of an attempt it is a batch with
     `"invocations": []`, which 11.5 (line 586) defines as "claimed, but no provider call was
     started". If the dead process had delivered nothing, the server has nothing to compare
     that batch with.
   - The first write of a record that failed after the rename, where the removal of the file
     failed as well (section 8, deviation 7). The same statement, for an attempt that then
     ran without the protocol.

   The design wants the intent on disk before the launch so that a crash leaves an `unknown`.
   It does not say what holds when that write is the one that fails. The choices I see:

   a. Accept it as a known limit. A failed write and the death of the process before the
      next successful write have to coincide.
   b. When the write of a launch intent fails, remove the record file, best effort. Later
      changes still try to write the whole record, which brings the file back complete. The
      live process reports from memory as now. After a crash in the window nothing is
      reported and the attempt stays unknown through its hold. If the removal fails too, (a)
      is what remains.
   c. Do not launch without a persisted intent. This contradicts rule 11; I do not propose it.

   I would take (b): it is a few lines in `beginInvocation` and a test. I did not build it,
   because the design does not ask for it.

## 10. What was not done. Notes for the main session

### 10.1 Not done

- **Stages D, E and F.** Nothing is sent and there is no client method for the route. The
  capability constant exists (`pkg/protocol/messages.go:44`) and nothing uses it:
  `capabilities()` and `Run` are untouched. No production code calls `openBudgetUsageJournal`
  and nothing assigns `Daemon.budgetUsage`. In a daemon built from this tree
  `beginBudgetUsageAttempt` therefore returns nil for every task and every run takes the
  legacy path. There is no restart scan, no retention and no removal of a record.
  `journal.read`, `(*budgetUsageAttempt).snapshot` and `budgetUsageRecord.clone` are reached
  from tests only.
- **The `adapter_version` supplement of design 11.10** (section 8).
- **The choice of section 9** is not built.
- **Windows.** The module builds and the three packages vet for `windows/amd64` (section
  6.2, item 7). Nothing ran there. Not run anywhere: the listing check of
  `openBudgetUsageJournal` as the only check that fails (section 7.3), and on Windows the
  journal failure cases that use permission bits, which the tests skip there.
  `budget_usage_runtask_unix_test.go` is not built for Windows.
- **A baseline on the second toolchain.** The baseline is go1.27.0 only (section 6.1). The
  go1.26.9 run is compared with it by test name.
- No real agent CLI, no provider call, no database, Docker or dev environment command, no
  commit, stash, reset, checkout, merge or push, no sub-agent. I wrote to the worktree, to
  this report and to `/tmp/f3-6b/`; the Go tool wrote to its caches and to `TMPDIR`.

### 10.2 For round 2

1. **`adapter_version`.** Apply the rule where the version is recorded, in `describeLaunch`
   (`budget_usage.go:313`), so that the record and the batch agree. A test for a value that
   is not valid UTF-8 has to assert that the field is absent, not compare bytes: the two
   toolchains write such a byte differently (10.3).
2. **`snapshot` has no nil guard** (`budget_usage.go:384`). The comment of
   `budgetUsageAttempt` (`:274-276`) names the four methods that are safe on a nil attempt:
   `describeLaunch`, `beginInvocation`, `endInvocation` and `seal`. Until 03:50 it said
   "every method", which `snapshot` did not keep. A caller in round 2 either holds a live
   attempt or needs the guard.
3. **Removal.** Round 1 removes no record file and no entry of `journal.attempts`. Whatever
   deletes a record in stage D or E has to drop the entry too. An entry that stays keeps the
   record in memory and makes `begin` refuse that attempt id for the life of the process.
4. **Names in the journal directory that are not records.** `.installation-*.tmp` is the
   temporary file of `writeManagementJSON`, left behind when the process dies between create
   and rename. `.probe-<uuid>.tmp` is the probe of `openBudgetUsageJournal`, left behind when
   its removal fails. `read` refuses every name that is not `<canonical uuid>.json`. The
   restart scan has to skip these names, and nothing removes them yet.
5. **A rebuilt batch has other bytes.** A batch that a later process builds from a record
   without batch bytes carries that process's `producer_epoch` and its own `reported_at`. If
   the first process had built and delivered a batch whose bytes never reached the file
   (section 9, first case), the two differ under one batch key.
6. **A panic in `handleTask` is not recovered.** Package `daemon` has no `recover()` outside
   its tests. The process dies with an unsealed record, which is the ordinary input of the
   restart path.
7. **Before `describeLaunch`.** When `runTask` returns before it has built the backend, the
   batch names the adapter the record was created with, which `beginBudgetUsageAttempt`
   takes from the provider and the runtime's profile right before the run, and it carries no
   `adapter_version` (`TestBudgetUsageRunTaskThatLaunchesNothing`,
   `TestBudgetUsageHandleTaskSealsAnAttemptThatLaunchedNothing`).
8. **The journal directory.** `os.MkdirAll(directory, 0o700)` does not tighten a directory
   that exists, and it follows a symbolic link to a directory. Only record files are checked
   with `Lstat`. Round 2 chooses the directory (design: a subdirectory of `cli.ProfileDir`).

### 10.3 Notes worth keeping

- **The design changed while the lane worked.** A lane gets no message before its turn ends
  and has no reason to read a section of the design twice. I saw the supplement of 04:55:36
  only because I searched `design.md` for `adapter_version` while collecting facts for this
  section. A rule added during a lane's turn also has to be in the message that continues
  the lane. The file changed once more at 06:05:56 (164,167 bytes). The lines dated
  2026-11-19 that are new since my last look are 741, 790 and 834; they are outside 11.5 and
  11.10, whose headings and whose line 696 are where they were. I did not read those two
  sections again as a whole after that change.
- **Files of the main session that I read.** At about 05:20 I read
  `/tmp/f3-main-s6b/review1-notes.md` and `/tmp/f3-main-s6b/findings-draft.md` once, to see
  whether the supplement was meant for round 1. They are not addressed to me and I changed
  nothing because of them. Two details in them do not fit the final tree: the comment they
  quote as "every method is safe on a nil attempt" was narrowed at 03:50 (10.2, item 2), and
  the assignment in `describeLaunch` is at `budget_usage.go:313`, not `:318`.
- **Invalid UTF-8 and the two toolchains.** Measured with a small program
  (`/tmp/f3-6b/utf8probe/`): `encoding/json` with `SetEscapeHTML(false)` writes a byte that
  is not valid UTF-8 as the three bytes `ef bf bd` under go1.27.0 and as the six characters
  `\ufffd` under go1.26.9. Both read back as U+FFFD. No test of round 1 gives the builder or
  the record such a string, so no expected byte string depends on the toolchain. A sealed
  batch is kept in the record as its exact bytes (`Batch []byte`), so a later process sends
  them as they are.
- **What the daemon does not check.** The builder sends `adapter`, model names and counts as
  it has them (rule 4). It does not count invocations or usage items and does not measure
  the body against the limits the server decodes with.
- **Three tests of `pkg/agent` can time out under load.** `TestReasonixBackend...` with a
  5 s limit, section 6.3. They are not touched by this change and pass alone.
- **File modes.** The seven new files have mode 0644.
- **Evidence.** Everything this report quotes is under `/tmp/f3-6b/` (`baseline/`,
  `verify/`, `verify-before-comment-edit/`, `mutants/`, `home-probe/`, `utf8probe/`,
  `comment-edit/`, `notes.md`). That directory is not durable.

## 11. Round 2 log (appended stage by stage; sections 1 to 10 keep the round 1 text, and this section wins where they differ)

Round 2 started on 2026-11-19 at about 06:40 from `/tmp/f3-main-s6b/round2-6b.md`.
The worktree had the round 1 checksums of section 2 at 06:56.

### 11.1 Before stage D: the two record rules and the shared wire fixtures (07:24)

- **`adapter_version` never costs the batch** (continue message 1.1). `budget_usage.go`:
  `budgetUsageAdapterVersion` trims white space and returns no version for a value that
  is empty, longer than 120 bytes, not valid UTF-8, or holds a rune for which
  `unicode.IsControl` is true; `describeLaunch` records its result. Test:
  `TestBudgetUsageAdapterVersionNeverCostsTheBatch`, 15 cases, in the new file
  `budget_usage_record_rules_test.go`.
- **A record file is never behind its process in launches** (continue message 1.2, rules
  1 and 2). `beginInvocation` writes through `persistLaunchIntentLocked`: when the write
  fails it removes the record file, and logs exactly one warning either way (the one for a
  removal that failed too carries `remove_error`). `endInvocation` and `seal` are
  unchanged: a failed write there leaves the file. Tests:
  `TestBudgetUsageLaunchIntentWriteFailureRemovesTheRecordFile`,
  `TestBudgetUsageLaunchIntentWriteFailureWhenTheRecordFileCannotBeRemoved`,
  `TestBudgetUsageOutcomeAndSealWriteFailuresLeaveTheRecordFile` (same file; POSIX
  permission bits, skipped on Windows and for root). Rules 3 to 5 are recovery and come
  with stage E.
- Red state of both: `go test -count=1 -run
  'TestBudgetUsage(AdapterVersion|LaunchIntent|OutcomeAndSeal)' ./internal/daemon/`,
  exit 1 at 07:04:10: 11 of the 15 version cases and both launch-intent tests failed by
  assertion (`/tmp/f3-6b/r2/red-p1p2.txt`). The third test passed on the round 1 code:
  it pins that these writes do not remove the file.
- No round 1 test was edited for these two rules. Both new warnings begin with the words
  the round 1 warning has, so `TestBudgetUsageHandleTaskSealsFromMemoryWhenTheJournalFails`
  still counts three.
- **The shared wire fixtures** (continue message section 2). Two new files, written from
  design 11.5 by a script that spells the bytes out (`/tmp/f3-6b/r2/write_fixtures.py`),
  not by the builder:

  | File | Bytes | SHA-256 |
  | --- | ---: | --- |
  | `server/pkg/protocol/testdata/budget_usage_v1_batch.json` | 1023 | `0d76f057284362e657cb20e1e864b66883f8e4f553e15ac540ae0d2439db7dc0` |
  | `server/pkg/protocol/testdata/budget_usage_v1_batch_empty.json` | 234 | `bfe8d8baf7b275a45c9bb28251a5f86b7f2726dba691c55e4de6de944b63cdf8` |

  The tests that read them are `TestBudgetUsageBatchGolden` (both files) and the first
  subtest of `TestBudgetUsageBatchEnvelope` (the empty one), in `budget_usage_test.go`.
  Both are round 1 tests of mine that pinned these shapes with literals; the literals are
  gone and the files are the one source. Red state: the same two tests, exit 1 at
  07:20:46, because the files did not exist yet; green on go1.27.0 and on go1.26.9 at the
  first build of the files.
- All `TestBudgetUsage*` after these three items: exit 0, 42 top-level PASS, 0 FAIL,
  0 SKIP on go1.27.0 (`/tmp/f3-6b/r2/green-fx.txt`, 07:23:48).

### 11.2 Stage D: declaring the protocol and sending the batch (09:10)

**What exists now.** One new production file, `internal/daemon/budget_usage_delivery.go`,
and four hooks:

| Stage D item (brief) | Where | Test (`budget_usage_send_test.go`) |
| --- | --- | --- |
| The capability is a flag on `Client`, read by `capabilities()` | `client.go`: field `budgetUsageReady atomic.Bool`, three lines in `capabilities()` (root and workspace clients alike) | `TestBudgetUsageCapabilityIsDeclaredWithAUsableJournal` |
| Set on the root client and on every existing managed child when the journal opens; copied to children created later | `(*Client).setBudgetUsageReady` (delivery file); one line in `installManagedCredential` (`managed_transport.go`) | `TestBudgetUsageCapabilityReachesManagedWorkspaceClients` |
| `Run` opens the journal after `listenHealth` and before `preflightAuth` | `daemon.go`, `Run`: `d.initializeBudgetUsage()` after the managed-installation block, directly before `d.preflightAuth(ctx)` | read, not run: no test starts `Run` |
| Opening fails: one warning, no journal, no capability, the daemon runs as before | `(*Daemon).initializeBudgetUsage` | `TestBudgetUsageCapabilityIsNotDeclaredWithoutAUsableJournal` (the path is a file; the directory cannot be written; the profile directory cannot be resolved) |
| The header reaches both claim transports | nothing new: `setIdentityHeaders` and the WebSocket handshake both call `capabilities()` | both capability tests read `X-Client-Capabilities` of a real `ClaimTask` request and of a real handshake request |
| `POST /api/daemon/budget/attempts/{attemptId}/usage`, the persisted bytes, the daemon's normal authentication | `(*Client).reportBudgetUsage` | `TestBudgetUsageHandleTaskSendsTheSealedBatch` |
| No redirect is followed | same: a per-call copy of the HTTP client with `CheckRedirect` returning `http.ErrUseLastResponse` | `TestBudgetUsageAnswerDecidesWhatBecomesOfTheRecord`, the five redirect cases (no request reaches `/landing`) |
| Its own reply type, never a `*requestError` | `budgetUsageReply{status, code, err}` with `verdict()` | `TestBudgetUsageNotFoundAnswerOnlyDropsTheRecord` |
| Classification | `budgetUsageReply.verdict` | `TestBudgetUsageAnswerDecidesWhatBecomesOfTheRecord`: 8 delivered, 7 refused, 23 kept |
| The warning of a final refusal: status and code, no body | `sendBudgetUsage`, `budgetUsageErrorCode` | `TestBudgetUsageRefusalWarningNamesTheCodeAndNeverTheBody` (15 bodies) |
| A record goes only to the server it was created under | `sendBudgetUsage` compares `record.ServerURL` with the journal's | comes with the restart tests of stage E |
| Managed: the child of the record's workspace; no valid binding, the record stays | `(*Client).budgetUsageTransport` | `TestBudgetUsageManagedInstallationSendsThroughTheWorkspaceOfTheRecord` |
| After sealing, `handleTask` sends in the background; never blocks, never changes the terminal report | `(*Daemon).finishBudgetUsageAttempt(ctx, attempt)`, called from `handleTask` with its own `ctx`, not the run's | `TestBudgetUsageHeldOpenReportDoesNotDelayOrChangeTheTerminalReport` (a task that completes, a task that fails) |
| One record is never sent twice at once (stage E shares the mark) | `claim` / `release` on the journal, taken in `deliverBudgetUsage`, the one handler | `TestBudgetUsageRecordIsNeverSentTwiceAtOnce` |
| Whatever deletes a record drops its `journal.attempts` entry (continue message 1.4 item 3) | `(*budgetUsageJournal).remove` | the answer table checks the file and the entry for every delivered and refused case |

Three more tests of the same file: `TestBudgetUsageSendEndsWithTheDaemonAndKeepsTheRecord`
(the context is cancelled while the report is open), `TestBudgetUsageUnreachableServerKeepsTheRecord`
(the server is closed: no connection), and
`TestBudgetUsageAnsweredRecordIsNotSentAgainWhenItsFileCannotBeRemoved` (see "my choices").

**The answers the table sends**, each to one sealed record, each followed by a second
send that the server accepts:

- delivered, record and entry gone, no warning: 200 with `recorded`, with `not_recorded`,
  with an empty body, with a body that is not JSON; 201, 202, 204, 299;
- refused, record and entry gone, exactly one warning with `status`, `code` and
  `attempt_id`: 400 `invalid_request`, 404, 409 `operation_conflict`, 409 `batch_conflict`,
  409 `protocol_mismatch`, 413 `request_too_large`, 422;
- kept, file byte-identical, entry still the same object, no warning, and the second send
  carries the same bytes, the same key and the same path: 401, 402, 403, 405, 408, 410,
  418, 425, 429, 500, 502, 503 `ledger_unavailable`, 504, 300 with a body, 301, 302, 303,
  304, 307, 308, a 200 and a 400 whose body ends before its declared length, and a
  connection closed without an answer.

After every case a third call sends nothing, and the log holds neither the marker text of
the answers nor the batch key, `protocol_version`, `"invocations"` or the model name.

**The 404 test.** While the provider of a task is in the middle of its run, the record of
another attempt is refused with 404 and the body `{"error":"runtime not found"}` (then
`task not found`, then `workspace not found`). Checked: that record is gone, the runtime is
still in `d.runtimeIndex`, the context of the running provider is not cancelled, the task
has not ended; after the provider ends, the task is reported complete once and failed
never, its own report (same 404) is gone as well, every other request of the daemon is one
about the task, and the text of the answer is nowhere in the log.

**Red state.** The tests were written against a stub (`initializeBudgetUsage` and
`deliverBudgetUsage` doing nothing): `go test -count=1 -timeout 300s -run
'^TestBudgetUsage(Capability|Answer|Refusal|HandleTaskSendsTheSealedBatch|HeldOpen|NotFound|RecordIsNeverSentTwice|SendEnds|Unreachable|ManagedInstallation|AnsweredRecord)'
./internal/daemon/`, exit 1 at 08:43:26, 12 of the 13 new top-level tests failed by
assertion (`/tmp/f3-6b/r2/red-stage-d.txt`). The thirteenth,
`TestBudgetUsageUnreachableServerKeepsTheRecord`, passes on the stub: a daemon that sends
nothing keeps the record too. It pins that a network error is not read as an answer.

**Green.** `go test -count=1 -timeout 600s -v -run '^TestBudgetUsage' ./internal/daemon/`:
exit 0 at 09:00:20, 55 top-level PASS, 0 FAIL, 0 SKIP (`/tmp/f3-6b/r2/green-stage-d.txt`).
The same with `-race`: exit 0 at 09:01:51, 55 PASS, no data race
(`/tmp/f3-6b/r2/race-stage-d.txt`). `go vet ./internal/daemon/` exit 0, `gofmt -l` empty.
The three packages in full on go1.27.0 after stage D (`go test -count=1 -v ./internal/daemon/... ./pkg/protocol/... ./pkg/agent/...`, 09:04:50 to 09:08:55): exit 0, 3099 top-level PASS, 0 FAIL, 5 SKIP; every baseline name is there, and the 55 names the baseline does not have are all `TestBudgetUsage*` (`/tmp/f3-6b/r2/full-after-stage-d.txt`).

**Round 1 tests of mine that changed, and why.** In round 1 a record ended at the seal
and three tests said so. Now the seal is followed by a send, so:

- `budgetUsageTestNewServerReplying` (`budget_usage_capture_test.go`), the daemon API of
  the round 1 `handleTask` and `runTask` tests, now answers the budget usage route itself
  with 503 `ledger_unavailable`. The record of an attempt therefore stays in the journal,
  where those tests read it after `handleTask`. Without this the route would have been
  answered 200 and the record deleted while the test reads it.
- `TestBudgetUsageHandleTaskRecordsAndSeals`: "nothing is sent" became "one report with
  the sealed bytes" (new helper `budgetUsageTestRequireReports`).
- `budget_usage_runtask_unix_test.go`: the three places that required no request to the
  budget usage route now require one report per attempt when the run has a record, and
  still none when it has no record
  (`TestBudgetUsageRunTaskFreshSessionRetryIsTwoInvocations`,
  `TestBudgetUsageRunTaskLaunchThatCannotStart`, `TestBudgetUsageRunTaskWithoutTheProtocol`).
  In the second of them the check for the legacy usage route named the suffix `/usage`,
  which the new route also ends in; it now names the task's own path.
- `budgetUsageTestOpenJournal` (`budget_usage_test.go`) registers a cleanup that waits for
  the background sends of its journal, so that no test ends with a send under way.

No test that existed before this task was edited. The script that made the edits is
`/tmp/f3-6b/r2/edit_round1_tests_stage_d.py`; the files as they were are next to it
(`*.before-stage-d.go`).

**My choices where the design gives no value** (all in `budget_usage_delivery.go`):

1. One usage report is bounded by the `Timeout` of the client's `http.Client`, 30 s
   (`client.go:136`), and by the context it is sent under. `reportBudgetUsage` copies the
   client for each call and the copy keeps that `Timeout`; the client of a managed
   workspace is made by `NewClient` too and has the same value. The report has no bound of
   its own any more: stage D gave it 15 s (`budgetUsageSendTimeout`), and stage F removed
   that constant (section 11.4). A pass against a server that holds every request open
   takes up to 32 times 30 s.
2. Of an answer the first 64 KiB are kept to look for the error code; the rest is read
   and dropped. An answer that cannot be read to its end keeps the record whatever its
   status line said, as design 11.10 says.
3. The form `^[a-z][a-z0-9_]{0,63}$` is required of `code` as well, not only of `error`.
   The design states the form for `error` only. Without it a server or a device in between
   could put any text into the log through `code`. A `code` that is present and not of
   that form gives no code; `error` is then not consulted.
4. A report that is kept is logged at Debug only (status, error; a network error names
   the URL, never a body). The warning comes when something is lost: at a final refusal,
   and in stage E at the 7-day cap. A server that is down for a day does not fill the log
   with one warning per attempt and round.
5. When a record is done with (delivered or refused) and its file cannot be removed, the
   daemon logs one more warning, drops the entry and remembers the attempt as settled in
   memory: this process does not send that batch again and does not warn again, and a
   later round removes the file when the directory allows it. A refusal plus a failed
   removal are two warnings. Without this a refused record in a directory that lost its
   write permission would be sent and warned about again in every round until the cap.
6. The server check is an exact string comparison of the record's `server_url` with the
   URL the journal was opened under, which is `cfg.ServerBaseURL`.
7. `snapshot` got no nil guard: the only new caller takes the attempt from
   `journal.attempts`.

### 11.3 Stage E: the resend loop, recovery after a restart, the retention cap (12:35)

**What exists now.** One new production file, `internal/daemon/budget_usage_recovery.go`
(206 lines), one handler in `budget_usage_delivery.go`, one field and one constant in
`budget_usage.go`, and one line in `Run`.

| Stage E item (brief; continue message) | Where | Test (`budget_usage_recovery_test.go` unless named) |
| --- | --- | --- |
| A loop with a pass at start and then a ticker | `(*Daemon).budgetUsageLoop(ctx, every)`; started in `Run` by `go d.budgetUsageLoop(ctx, budgetUsageResendInterval)`, directly after `go d.cancellationReceiptLoop(ctx)` | `TestBudgetUsageLoopPassesAtStartAndThenAtEveryTick`: a pass at start with a one-hour ticker; three passes with a 5 ms ticker; the loop ends with its context; a daemon without a journal has no loop. The line in `Run`: `TestBudgetUsageRunOpensTheJournalBeforeThePreflightAndStartsTheResendLoop` (`budget_usage_wiring_test.go`) |
| Bounded per round | `(*Daemon).resendBudgetUsage`, `(*budgetUsageJournal).takeRound`: at most 32 records per pass, in the order of their attempt ids; the next pass goes on after the last one and around the end | `TestBudgetUsagePassHandlesAtMostThirtyTwoRecordsAndTakesUpWhereItStopped` (40 records: 0 to 31, then 32 to 23, then 24 to 15; then delivered 16 to 7 and 8 to 15) |
| One in-flight set with the immediate send | `handleBudgetUsage` is the one handler and takes the mark (`claim` / `release`); `deliverBudgetUsage` (after the seal) and the pass both go through it | `TestBudgetUsagePassDoesNotSendARecordThatIsBeingSent`; from stage D `TestBudgetUsageRecordIsNeverSentTwiceAtOnce` |
| A record with batch bytes is resent as it is | `recoverBudgetUsage` | `TestBudgetUsageRestartResendsThePersistedBatchByteForByte` (a batch with an invocation, an empty batch): the same bytes twice, with the key and the `producer_epoch` of the earlier process |
| No bytes, earlier process, at least one invocation: `unknown` with empty usage, built, persisted, sent (continue message 1.2 rule 5; rule 8) | `recoverBudgetUsage`: `buildBudgetUsageBatch(record, journal.epoch, journal.now())`, then `journal.write`, then `sendBudgetUsage` | `TestBudgetUsageRestartReportsAnInvocationWithoutAnOutcomeAsUnknown`: `producer_epoch` of the building process, `reported_at` its clock, `observed_at` the launch-intent time; an invocation that had ended keeps its outcome and usage; the file held the bytes when the report arrived; a second pass three hours later sends the same bytes |
| The bytes must be persisted before they are sent | same | `TestBudgetUsageRecoveryDoesNotSendABatchItCouldNotPersist` |
| 1.2 rule 3: no bytes and no invocation is never reported, whichever process created it | `recoverBudgetUsage` | `TestBudgetUsageRecoveryNeverReportsARecordWithoutBatchAndWithoutInvocation` (left by an earlier process; left by this process) |
| 1.2 rule 4: a leftover of this process is removed; a record in `journal.attempts` is not touched | `recoverBudgetUsage` (the epoch); `handleBudgetUsage` (a live record without bytes) | `TestBudgetUsageRecoveryDropsARecordOfThisProcessThatNoRunningAttemptHolds`; `TestBudgetUsageRecoveryLeavesARunningAttemptAlone` (before the first launch, during a run, past the cap) |
| 1.3: a rebuilt batch answered 409 `operation_conflict` | nothing new: `verdict` | `TestBudgetUsageRebuiltBatchAnsweredWithOperationConflictIsDropped` |
| A record goes only to the server it was created under; it stays until the cap | `recoverBudgetUsage` | `TestBudgetUsageRestartKeepsARecordOfAnotherServer` |
| A record older than 7 days is dropped with a warning | `(*budgetUsageJournal).outlived`, `dropOutlivedBudgetUsage` | `TestBudgetUsageRecordOlderThanSevenDaysIsDroppedWithAWarning` (four kinds of record, each at exactly seven days and one second later); `TestBudgetUsageBatchOfARunThatOutlastedTheCapIsSentOnce` |
| 1.4 item 4: only `<canonical uuid>.json` is a record | `budgetUsageRecordIDs` | `TestBudgetUsageRecoveryActsOnlyOnCanonicalRecordNames` (seven foreign names, each holding a complete sealed record) |
| 1.4 item 4: old temporary files go, no other foreign name does | `budgetUsageRecordIDs` | `TestBudgetUsageRetentionRemovesOldTemporaryFilesAndNoOtherForeignName` |
| 1.4 item 3: whatever deletes a record drops its entry | every removal of stage E goes through `removeBudgetUsageRecord`, which calls `journal.remove` | `budgetUsageTestRequireDropped`, used by every test that expects a drop, checks the file and the entry |
| Managed installation after a restart | `sendBudgetUsage` (unchanged) | `TestBudgetUsageRestartInAManagedInstallationSendsWithTheCredentialOfTheRecordsWorkspace` |

Seven more tests: `TestBudgetUsageKeptBatchOfThisProcessIsResentFromMemory` (with its
file, without its file, when the directory cannot be listed),
`TestBudgetUsageUnreadableRecordIsSkippedUntilTheCapAndThenDropped`,
`TestBudgetUsagePassAfterTheDaemonStoppedHandlesNothing`,
`TestBudgetUsagePassLeavesAnAnsweredRecordAloneWhenItsFileCannotBeRemoved`,
`TestBudgetUsageSendAfterTheSealHandlesOnlyWhatThisProcessHolds`,
`TestBudgetUsagePassOverARecordThatIsGoneDoesNothing`, and the wiring test named above.
In all: 22 top-level tests in `budget_usage_recovery_test.go` and one in
`budget_usage_wiring_test.go`.

**How one record is handled** (`handleBudgetUsage`, then `recoverBudgetUsage`), in this
order; the first line that applies ends it:

1. Another goroutine holds the mark of the record: nothing.
2. This process holds the record (`journal.attempts`). No batch bytes, so the attempt is
   running: nothing, whatever the age of the record. Batch bytes, called by a pass, older
   than seven days: one warning, file and entry removed. Batch bytes otherwise: sent from
   memory.
3. The record is done with and only its file is left (`settled`, section 11.2 choice 5):
   the removal is tried again, without a log line.
4. Called by the send that follows the seal: nothing. That send acts on what its own
   process holds.
5. The file cannot be read as a record of that attempt: skipped; see choice 12.
6. Older than seven days: one warning, removed.
7. Created under another server URL: left exactly as it is.
8. Has batch bytes: sent as they are.
9. Names no invocation: one warning, removed, nothing sent (rule 3).
10. Created by this process: one warning, removed, nothing sent (rule 4).
11. Otherwise: the batch is built under this process's epoch, the record is written with
    the bytes, then it is sent. If the write fails nothing is sent and the file is as it
    was.

Steps 2 and 3 and the read of step 5 happen under one hold of the journal lock
(`(*budgetUsageJournal).load`, which replaces the stage D `lookup`). `begin` holds the same
lock from its check for an existing file to the entry in `journal.attempts`, so a record
that this process is creating cannot be seen as a file without an entry and removed by
step 10.

**Stage D code that changed.** `deliverBudgetUsage(ctx, attemptID)` is now one line that
calls `handleBudgetUsage(ctx, attemptID, false)`. The comparison of the server URL moved
from `sendBudgetUsage` to `recoverBudgetUsage`: a record this process created is of this
server by construction, so only a record read from disk needs the check, and it has to
come before the batch is built (choice 11). `lookup` is gone. No stage D test changed.

**Red state.** The tests were written against a stub (`resendBudgetUsage` and
`budgetUsageLoop` doing nothing, the three constants present): `/bin/bash
/tmp/f3-6b/r2/stage-e-tests.sh /tmp/f3-6b/r2/red-stage-e.txt`, which runs `go test
-count=1 -timeout 600s -v -run '^(<the 19 names>)$' ./internal/daemon/`: exit 1 at
10:55:01, 17 of the 19 top-level tests failed by assertion. The two that pass on the stub
assert that a pass does nothing: `TestBudgetUsageRecoveryLeavesARunningAttemptAlone` and
`TestBudgetUsagePassDoesNotSendARecordThatIsBeingSent`. Section 7 names the mutants that
make each of them fail. Four tests were added after the implementation existed (the last
three of the list above and the wiring test); they have no red state of their own, and
section 7 names the mutants that they kill.

**Green.** `go test -count=1 -timeout 900s -v -run '^TestBudgetUsage' ./internal/daemon/`:
exit 0 at 12:02:36, 78 top-level PASS, 0 FAIL, 0 SKIP (`/tmp/f3-6b/r2/green-stage-e-2.txt`).
The same with `-race`: exit 0 at 12:05:09, 78 PASS, no data race
(`/tmp/f3-6b/r2/race-stage-e.txt`). `go vet ./internal/daemon/` exit 0 for the host, for
`GOOS=windows GOARCH=amd64` and for `GOOS=linux GOARCH=amd64`; `gofmt -l` empty. The three
packages in full on go1.27.0 (`go test -count=1 -v ./internal/daemon/... ./pkg/protocol/...
./pkg/agent/...`, 12:20:50 to 12:25:35): exit 0, 3122 top-level PASS, 0 FAIL, 5 SKIP;
every baseline name is there, and the 78 names the baseline does not have are all
`TestBudgetUsage*` (`/tmp/f3-6b/r2/full-after-stage-e.txt`).

**My choices where the design gives no value** (numbered on from section 11.2):

8. The loop passes every 30 s (`budgetUsageResendInterval`) and a pass handles at most 32
   records (`maxBudgetUsageRecordsPerPass`), the bound of the cancellation receipt replay
   in the same package. One report is bounded by the `Timeout` of the client's
   `http.Client`, 30 s (`client.go:136`, kept by the per-call copy in `reportBudgetUsage`),
   and by the context. So a pass against a server that holds every request open takes up
   to 32 times 30 s, which is 16 minutes; the next pass starts when it has ended. (Until
   stage F the bound of one report was 15 s and this figure was 8 minutes; section 11.4.)
   There is no backoff: a record that is kept is sent again at every pass that reaches it.
9. The age of a record is counted from its `created_at`, on the journal's clock. The cap
   is strict: a record that is exactly seven days old is still handled.
10. The cap is applied by the passes of the loop and not by the send that follows the
    seal. A run has no time limit by default (`DefaultAgentTimeout = 0`), so a run can
    outlast seven days; its batch is then sent once, and the next pass drops it if that
    send did not deliver it. A record whose attempt is still running is never capped.
11. A record that was created under another server URL is not sent and not changed in any
    way: no batch is built for it and nothing is written. It stays until the cap and is
    then dropped with the same warning as any other record. This is the brief's "is not
    sent and stays until the cap"; the part that is mine is that the batch of such a
    record is not built either, so that the process that does talk to its server builds
    it under its own epoch.
12. A file that has the name of a record and cannot be read as one (cut short, another
    version, another attempt id inside, larger than 4 MiB, not readable) is skipped with
    a Debug line. When it is a regular file and was last changed more than seven days
    ago, it is removed with one warning: a record is at least as old as the last change
    of its file. A directory or a symbolic link with the name of a record is never
    removed. The text of the read error is not logged, because a decode error can quote
    the file.
13. When the batch that recovery built cannot be written, the record stays as it was and
    the next pass tries again. This is a Debug line, not a warning: it would repeat every
    30 s for as long as the directory cannot be written.
14. Temporary files are removed without a log line, and only regular files. Their age is
    their modification time; exactly seven days stays.
15. When the directory cannot be listed, the pass still handles the records this process
    holds, from memory, plus the names the listing returned before it failed. Debug line.
16. `handleBudgetUsage` got the flag `fromPass` instead of a second handler. It decides
    two things: whether the cap applies (choice 10), and whether a record that this
    process does not hold is looked at (step 4 above).
17. The lock that step 5 holds while it reads a file (at most 4 MiB) is the lock `begin`
    needs. A task whose record is being created waits for that read. No test pins the
    lock: the window it closes is between two system calls of `begin`.
18. `Run` cannot be started by a default test, so the two lines of `Run` are pinned by a
    test that parses `daemon.go` (`go/parser`) and checks that each of five statements is
    a statement of `Run` itself, exactly once, in the order `listenHealth`,
    `initializeBudgetUsage`, `preflightAuth`, `go budgetUsageLoop`, `pollLoop`. The
    package already has a test of that kind (`agent_command_names_test.go`). This
    replaces "read, not run" in the third row of the table in section 11.2.

**Limits of stage E.**

- The accepted limit of continue message 1.2 rule 6 stands as stated there: when the
  directory cannot be written from the Nth launch on (N > 1), the removal fails too and
  the process dies before it delivers, the next process reports the first N-1 invocations
  and not the Nth. Nothing was built for it.
- A sealed record whose write failed at the seal lives in memory only. The loop sends it
  from memory for as long as the process lives and does not try to write it again
  (design 11.10 says this needs no handling). If the process dies first, the next one
  finds the file as it was before the seal and builds a new batch under the same key;
  when the earlier bytes had reached the server, the answer is the 409
  `operation_conflict` of continue message 1.3.
- An error of `buildBudgetUsageBatch` in recovery is handled (Debug, record kept) and
  cannot be produced: the builder fails only when the JSON encoder does, and it encodes
  strings, integers and slices. That branch has no test.

### 11.4 Stage F (16:25)

The main session narrowed stage F at about 16:10 (`/tmp/f3-main-s6b/stage-f-narrowed.md`):
this lane runs no round 2 mutants and not the matrix of brief section 8. What this lane
did in stage F is listed here; it did nothing else.

**(a) Four simplifications of production code.** Each removed something that no test
could tell from its absence. Where section 11.2 or 11.3 describes one of these four places
otherwise, this list wins. Line numbers of `budget_usage_delivery.go` that those sections
quote moved up with the removals; the file has 360 lines now.

1. `budgetUsageAdapterVersion` (`budget_usage.go`): the operand `version == ""` of the
   first condition is gone. Behaviour is unchanged: an empty value passes both conditions
   and is returned as it is, which is the empty string the operand returned. Covered by
   `TestBudgetUsageAdapterVersionNeverCostsTheBatch`, cases `empty` and `white space only`.
2. `finishBudgetUsageAttempt` (`budget_usage_delivery.go`): the guard
   `if attempt == nil { return }` is gone. Behaviour is unchanged: `seal` is safe on a nil
   attempt and returns nil, and the check `len(attempt.seal()) == 0` that follows returns
   before anything else touches the attempt. Covered by
   `TestBudgetUsageHandleTaskWithoutAttemptOrJournal` and
   `TestBudgetUsageCapabilityIsNotDeclaredWithoutAUsableJournal`, whose tasks go through
   `handleTask` with a nil attempt.
3. `sendBudgetUsage`, the report that is kept: one Debug call with the fixed attributes
   `attempt_id`, `status` and `error` replaces the code that added `status` only when an
   answer had arrived and `error` only when there was one. What happens to the record is
   unchanged (kept, same level, same message). The line itself changed: it now also says
   `status=0` or `error=<nil>`. Covered by the 23 kept cases of
   `TestBudgetUsageAnswerDecidesWhatBecomesOfTheRecord` (file unchanged, no warning, no
   text of a body in the log, the next send byte for byte the same).
4. `budgetUsageSendTimeout` (15 s), the `context.WithTimeout` around the send in
   `sendBudgetUsage` and the `time` import are gone. This one changes a number, so it is
   not neutral: a report is now bounded by the 30 s of the client's `http.Client` and by
   the context, as choice 1 of section 11.2 and choice 8 of section 11.3 now say. No test
   waited for either bound. What remains is covered by
   `TestBudgetUsageSendEndsWithTheDaemonAndKeepsTheRecord` (the context ends a send, the
   record stays) and by the kept case "the connection is closed without an answer" of the
   test named under 3. The 30 s bound itself has no test; see (e).

**(b) Round 2 mutants: not run by this lane.** The main session moved them to the
independent check. `/tmp/f3-6b/mutants2/` is not evidence of anything: it holds a copy of
the round 1 generator and its scripts, and nothing was generated or run there. Six round 1
mutants no longer apply to the round 2 tree as they are written, because the text they
replace changed or moved: `r02-describe-drops-version`, `sb-intent-not-persisted`,
`sc-not-sealed`, `sc-seal-does-nothing`, `sc-sealed-before-the-run` and
`sc-sealed-after-the-legacy-report`. The results of section 7 are those of the round 1
tree.

**(c) The one run of stage F on the final tree.** From `server/`, with `TMPDIR` set as
brief section 7 says, go1.27.0 darwin/arm64, 16:14:14 to 16:16:00:

`go test ./internal/daemon/ -count=1 -v > /tmp/f3-6b/r2/final-daemon.txt 2>&1`

Exit 0. 983 top-level `--- PASS`, 0 `--- FAIL`, 2 `--- SKIP`
(`TestCollaborationBatchPollerSlotsAndRunnerLinux` and `TestIsDriveRoot`, skipped on this
platform, not mine). 78 of the 983 are `TestBudgetUsage*`. The package line is `ok` in
101.135 s. Nothing failed, so nothing was fixed.

Earlier in stage F, after the fourth simplification and on the default toolchain:
`go build ./internal/daemon/` and `go vet ./internal/daemon/` exit 0, `gofmt -l` empty for
the two production files that stage F changed, and the `TestBudgetUsage` tests alone
exit 0 with 78 PASS, 0 FAIL, 0 SKIP (`/tmp/f3-6b/r2/green-f-2.txt`).

`git status --short` at 16:16 lists 20 paths and nothing is staged. Modified (6), under
`server/`: `internal/daemon/client.go`, `daemon.go`, `managed_transport.go`, `types.go`,
`pkg/agent/claude.go`, `pkg/protocol/messages.go`. Untracked (14): in `internal/daemon/`
the production files `budget_usage.go`, `budget_usage_batch.go`, `budget_usage_capture.go`,
`budget_usage_delivery.go`, `budget_usage_recovery.go` and the test files
`budget_usage_test.go`, `budget_usage_capture_test.go`, `budget_usage_runtask_unix_test.go`,
`budget_usage_record_rules_test.go`, `budget_usage_send_test.go`,
`budget_usage_recovery_test.go`, `budget_usage_wiring_test.go`; then
`pkg/agent/usage_normalizer.go` and the directory `pkg/protocol/testdata/` with the two
fixture files. HEAD is `86fea651c`.

**(d) Brief section 8 beyond that run: not run by this lane; the main session runs it.**
Not run on the final tree: `gofmt -l` over the files that stage F did not change,
`go build ./...`, the packages under `internal/daemon/`, `./pkg/protocol/...` and
`./pkg/agent/...`, the comparison of test names with the baseline, `-race`, everything
under `GOTOOLCHAIN=go1.26.9`, and the two cross-compiles. The runs of section 11.3 are
those of the tree before the four simplifications.

**(e) A report that gives up.** No: no test holds an answer open until a report gives up
by its own time bound, and no test sets the client's `Timeout`. The two nearest tests end
the wait another way. `TestBudgetUsageSendEndsWithTheDaemonAndKeepsTheRecord` holds the
answer open and cancels the context; it checks that the record is kept (file unchanged,
entry still held, no warning) and does not look at the in-flight mark.
`TestBudgetUsageRecordIsNeverSentTwiceAtOnce` holds the answer open and then lets the
server answer 503; it checks that the record is kept and that the mark is released, by
sending again. The mark is released by a `defer` in `handleBudgetUsage`, so by reading it
goes however the send returns. No test shows that for a send that timed out or was
cancelled.

**(f) For the independent check: lines that, as far as I can tell by reading, no test of
mine tells from their absence.** These are expectations. No mutant was run on any of them.

- `reportBudgetUsage`: the 30 s bound, see (e). `io.Copy(io.Discard, resp.Body)`: an
  answer that ends early inside its first 64 KiB already fails in `io.ReadAll`, and no
  test has a longer answer that ends early. `url.PathEscape`: it changes nothing for a
  canonical UUID, which is all the daemon passes. The `token != ""` guard: every test
  that reads the `Authorization` header of a report uses a client that has a token.
- `verdict`: the lower bound `r.status >= 200`. No test has a status below 200 without
  an error.
- `remove`: `delete(j.settled, attemptID)`. It frees memory, and no test reads
  `journal.settled`.
- `load`: the early return for an attempt that is held or settled. It saves a file read.
- `persistLaunchIntentLocked`: the operand `errors.Is(removeErr, os.ErrNotExist)`. A
  search of the test file found no launch intent that fails while the file is already
  gone.
- `budgetUsageRecordIDs`: `slices.Sort`. A directory listing is sorted already; a test
  tells only if one pass mixes a held record with other records, which I did not check.
- `budgetUsageErrorCode`: the `return ""` on a decode error. It matters only for valid
  JSON whose `code` has another type; I did not check for such a case.
- `defer ticker.Stop()` and `defer resp.Body.Close()` do nothing a test can see, and
  `journal.background` is waited on by tests only.

The identity headers of a report are pinned: `budget_usage_send_test.go:882` and `:1335`
and `budget_usage_recovery_test.go:1524` read `X-Client-Capabilities` of a report.

**(g) Notes.** The harness refused the Write tool for a scratch file named like a report
(`/tmp/f3-6b/r2/report-11.3.md`), so this report was written with the Edit tool only and
no such scratch file exists. After the last compaction my context showed the text of
another lane's brief (`/tmp/f3-main-s5e/brief-5e.md`) as if I had read it. I did not act
on it and wrote nothing for it. Nothing in round 2 needs a ruling. One choice of mine
changed after it was reported and is open to a different decision: the bound of one
report, 15 s before stage F and 30 s now ((a) item 4).

STATUS: complete
