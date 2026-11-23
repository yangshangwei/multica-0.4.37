# Step 6 independent check, daemon half, part 1 of 2: the record and the batch

Checker: `f3-check-6kd1`. Date: 2026-11-20.

Tree checked: `/Volumes/artisan/code/2026/mc-f3-budget-6b-20261118`, detached at
`86fea651cc5d09360327b6cf0f8357c1f360070d`, uncommitted work of the daemon half.

Manifest check at start (`shasum -a 256 -c /tmp/f3-main-s6k/base-r2/manifest.txt`
from the worktree root): 21 of 21 `OK`, exit 0. `git status --short` shows the
6 modified and 15 new files the brief names and nothing else.

This file is written point by point while the check runs. A point without a
verdict below has not been reached yet. The last line is the status line.

## 1. Verdict per point

(entries are added in the order of section 4 of the brief as each is finished)

Paths below are relative to `server/internal/daemon/` of the worktree unless
they start with another directory. Mutant ids (`A3-1`, …) refer to the table
in section 5. Finding ids (`KD1-nn`) refer to section 2.

### A2. A daemon without a journal is today's daemon: holds

What the code does (`git diff -- server/internal/daemon/daemon.go`, 29 added
lines; every added statement listed):

| Added statement | Where | With `d.budgetUsage == nil` |
| --- | --- | --- |
| field `budgetUsage *budgetUsageJournal` | `daemon.go` struct `Daemon` | zero value, nil |
| `d.initializeBudgetUsage()` | `Run`, after `initializeCancellationReceipts`, before `preflightAuth` | the one statement that is not behind a nil check: it decides. `budget_usage_delivery.go:245-258`: on an error of `cli.ProfileDir` or `openBudgetUsageJournal` it logs one warning and returns; `d.budgetUsage` stays nil and `setBudgetUsageReady` is not reached |
| `go d.budgetUsageLoop(ctx, …)` | `Run` | `budget_usage_recovery.go:112` returns at once |
| `budgetAttempt := d.beginBudgetUsageAttempt(task, provider, taskLog)` | `handleTask` | `budget_usage_capture.go:127` returns nil before anything else |
| `withBudgetUsageAttempt(runCtx, budgetAttempt)` | `handleTask` | `budget_usage.go:460-463` returns `runCtx` itself |
| `d.finishBudgetUsageAttempt(ctx, budgetAttempt)` | `handleTask` | `budget_usage_delivery.go:266`: `attempt.seal()` on nil returns nil (`budget_usage.go:430`), `len == 0`, return; no goroutine |
| `budgetUsageAttemptFrom(ctx).describeLaunch(…)` | `runTask` | one `ctx.Value` lookup, nil receiver returns (`budget_usage.go:348`) |
| `d.executeAndDrainRecorded(…)` (twice, replacing `d.executeAndDrain`) | `runTask` | `budget_usage_capture.go:107-109`: the same call of `executeAndDrain` with the same context and arguments |
| `markBudgetUsageStartFailed(ctx)` | `executeAndDrain` | no holder in the context, nothing |
| `markBudgetUsageBackendResult(ctx, ok)` (twice) | `executeAndDrain` | no holder in the context, nothing |

`types.go` adds one field, `BudgetAttemptID string` with
`json:"budget_attempt_id,omitempty"`; nothing reads it but
`beginBudgetUsageAttempt` (`grep -rn BudgetAttemptID` over non-test code: three
lines, all in `budget_usage_capture.go:127-143`).

Declaration: `client.go` `capabilities()` appends
`DaemonCapabilityBudgetUsageV1` only when `budgetUsageReady` is set, and the
only non-test caller of `setBudgetUsageReady` is the success path of
`initializeBudgetUsage` (`budget_usage_delivery.go:257`). How the flag reaches
managed clients and the WebSocket handshake is part 2's.

Tests that can fail:

- `TestBudgetUsageCapabilityIsNotDeclaredWithoutAUsableJournal`
  (`budget_usage_send_test.go:392`), three ways of spoiling the journal through
  the real `initializeBudgetUsage`: asserts `d.budgetUsage == nil`, exactly one
  warning, the claim header and the handshake header equal
  `daemonClientCapabilities()`, a task that carries an attempt id is reported
  complete once, zero requests to the usage route.
- `TestBudgetUsageHandleTaskWithoutAttemptOrJournal`
  (`budget_usage_capture_test.go:1022`), case "attempt id, no journal": the run
  carries no attempt, the requests (method, path, capability header, body) are
  `reflect.DeepEqual` to those of a daemon with neither, no request to
  `/api/daemon/budget/`, no log line that contains "budget usage".

Mutant `A2-1` (the failure branch of `initializeBudgetUsage` still calls
`setBudgetUsageReady`): killed by
`TestBudgetUsageCapabilityIsNotDeclaredWithoutAUsableJournal`. The lane's own
`sc-no-journal-check`, `sc-nil-guard-*` and `r12-wrapper-always-marks` cover the
other guards and were not repeated.

### A3. The attempt id is checked before it names a file: holds

Trace: the claim response is decoded into `Task` (`client.go:243`, `:316`,
`wsrpc.go:346`; plain `string` field) → `handleTask(ctx, task, slot)` →
`beginBudgetUsageAttempt` (`budget_usage_capture.go:131-137`, the value is
copied into `budgetUsageSeed.AttemptID` unchanged) → `begin`
(`budget_usage.go:275`): `managedUUID.MatchString(seed.AttemptID)` is the first
statement, before the lock, before `j.recordPath` (`:285`) and so before
`filepath.Join` (`:224`). `managedUUID` is
`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`
(`managed_identity.go:25`); Go's `$` without the `m` flag matches only at the
end of the text, so a trailing line end is refused. The other callers of
`recordPath` in my files take `a.record.AttemptID` (`budget_usage.go:230`,
`:393`), which is the value `begin` checked, or check the name themselves
(`read`, `:236`). The failure is logged without the value unless it is
canonical (`budget_usage_capture.go:141-144`).

Tests: `TestBudgetUsageRecordRefusedForNonCanonicalAttemptID`
(`budget_usage_test.go:611`) gives `begin` nine values (empty, `not-a-uuid`,
upper case, braces, no hyphens, a trailing line feed and a leading space (both
37 characters), `../<uuid>`, `urn:uuid:<uuid>`) and then requires that the
journal directory is empty and that its parent holds exactly one name.
`TestBudgetUsageHandleTaskRunsWithoutARecordThatCannotBeCreated`, case "the
attempt id is not a canonical UUID", runs an upper-case id through `handleTask`:
the task runs, is reported complete once, carries no attempt, no request goes to
the usage route, the journal directory is empty.

Mutants: `A3-1` (`begin` accepts what `uuid.Parse` accepts) killed by both
tests; `A3-2` (`begin` trims white space before the check and names the file
with the untrimmed value) killed by
`TestBudgetUsageRecordRefusedForNonCanonicalAttemptID`.

### A4. Both ways of not having the protocol: holds (one note, KD1-01)

`budget_usage_capture.go:127`: `if d.budgetUsage == nil || task.BudgetAttemptID == ""`
returns nil; the rest follows as in A2. A claim without the field decodes to
the empty string (`TestBudgetUsageTaskDecodesAttemptID`), and so does
`"budget_attempt_id": null`.

- No id, journal present: `TestBudgetUsageHandleTaskWithoutAttemptOrJournal`,
  case "journal, no attempt id": no attempt in the context, the journal
  directory is empty, zero live records, legacy requests equal to the baseline,
  no "budget usage" log line.
- Id present, no journal: the two tests named under A2.

"This process did not declare" and "`d.budgetUsage == nil`" are the same
condition in the code: the flag is set in one place, right after the field
(A2). The daemon does not rely on the server withholding the id. Note KD1-01:
design 11.1 says the server sends the id to a daemon that did not declare, so
the second way is the ordinary case for a daemon without a journal, not an
impossible one.

A claim made without the declaration by a process that has a journal (a
single request that lacks the header) is not distinguishable in
`beginBudgetUsageAttempt`: the record is made and the batch is sent; what the
server answers (`protocol_mismatch`, 409) and what the daemon does with it is
part 2's.

### B1. Where and how a record is written: holds

Where: `initializeBudgetUsage` (`budget_usage_delivery.go:245-258`) opens the
journal in `filepath.Join(cli.ProfileDir(d.cfg.Profile), "budget-usage-reports")`
(the name is the constant at `:25`). One attempt has one file:
`recordPath` (`budget_usage.go:223-225`) is
`filepath.Join(j.directory, attemptID+".json")`, and the attempt id reached it
only as a canonical UUID (A3).

How: every write of a record goes through `budgetUsageJournal.write`
(`budget_usage.go:229-231`), which is one call of `writeManagementJSON`. Own
grep of the five non-test files for `writeManagementJSON`, `.write(`,
`os.WriteFile`, `os.Create`, `os.OpenFile`, `os.Rename`: the callers of
`j.write` are `begin` (`:304`), `persistLocked` (`:333`),
`persistLaunchIntentLocked` (`:389`) and the rebuild of recovery
(`budget_usage_recovery.go:200`, part 2's); nothing else creates or changes a
file of that name. The only other file the code creates is the probe of
`openBudgetUsageJournal` (`.probe-<uuid>.tmp`, `:196-204`, removed at once).

The helper, read in full: `writeManagementJSON` →
`writeManagementJSONWithRename` (`managed_identity.go:205-243`; the lane did not
touch the file, `git diff` of it is empty). It marshals the whole value, creates
`.installation-*.tmp` with `os.CreateTemp` in the directory of the target,
writes the bytes and a line feed, `file.Sync()`, `Close`, `os.Rename` onto the
target, and, unless `runtime.GOOS == "windows"`, opens the directory and
`Sync()`s it; a deferred `os.Remove` takes the temporary file away on every
path. A record name therefore leads to the previous whole record or to the new
whole record, never to a part of one. It is the helper `cancel_receipts.go:161`
uses, which is what design 11.10 asks for. On darwin `File.Sync` is
`F_FULLFSYNC` (`internal/poll/fd_fsync_darwin.go` of the toolchain); G2 has
what that costs.

Modes: the directory is made with `os.MkdirAll(directory, 0o700)`
(`budget_usage.go:193`; an existing directory keeps the mode it has, accepted
item 7). A record file gets `0600` from `os.CreateTemp`, and the rename keeps
it. No other user of the machine can read a record. Tests:
`TestBudgetUsageJournalOpen` requires `0700`,
`TestBudgetUsageRecordWriteAndReadBack` requires `0600` and that the directory
holds the one name `<attempt>.json`;
`TestBudgetUsageCapabilityIsDeclaredWithAUsableJournal` runs the real
`initializeBudgetUsage` under a profile of its own and requires
`journal.directory` to be `<ProfileDir>/budget-usage-reports`.

Mutants (all killed): `B1-1` a record is written in place with `os.WriteFile`
(five tests, all of them through a directory whose mode lets the file be
replaced and not be synced: what the tests can see of the helper is that the
directory is opened after the rename; the fsync itself is not observable by a
test); `B1-2` the suffix is `.rec`; `B1-3` the directory name is that of the
cancellation receipts; `B1-4` the journal is opened under the default
profile; `B1-5` a record is `0644` after every write
(`TestBudgetUsageRecordWriteAndReadBack` alone); `B1-6` the directory is
`0750` (`TestBudgetUsageJournalOpen` alone).

### B2. What is fixed when the record is created: holds (one remark on the runtime id)

Before the run: `handleTask` calls `beginBudgetUsageAttempt` at `daemon.go:5265`
and `d.runner.run(...)` at `:5266`. `begin` (`budget_usage.go:274-314`) builds
the whole record in one literal (`:291-302`) and writes it before it returns:

| Field | Value at creation | Assigned again after creation |
| --- | --- | --- |
| `batch_key` | `uuid.NewString()` (`:294`): a version 4 UUID from `crypto/rand` (`google/uuid` v1.6.0, `NewString` is `Must(NewRandom()).String()`; nothing in the repository calls `uuid.SetRand` or `uuid.EnableRandPool`). 36 characters of `[0-9a-f-]`, inside `^[A-Za-z0-9_-]{16,64}$`; 122 random bits, not derived from the attempt, the time or the process | never |
| `producer_epoch` | `j.epoch`, minted once per `openBudgetUsageJournal` (`:210`), which `Run` reaches once per process | never |
| `server_url` | `j.serverURL`, the `d.cfg.ServerBaseURL` the journal was opened with | never |
| `workspace_id`, `task_id`, `attempt_id` | the claimed task's (`budget_usage_capture.go:131-137`) | never |
| `created_at`, `version`, `normalizer_version` | clock in UTC, 1, `agent.UsageNormalizerVersion(provider)` | never |

Own grep for every assignment to a field of a record in the five non-test
files: after `begin`, the code assigns `Adapter` and `AdapterVersion`
(`budget_usage.go:356-357`, `describeLaunch`), `Invocations` (`:377`), the
three outcome fields of one invocation (`:419-421`) and `Batch` (`:443`, and
`budget_usage_recovery.go:199` on the copy recovery read from the file).
Nothing else. `adapter` is the one envelope field that changes after
creation: the seed value comes from `customProfileLaunchForRuntime` in
`handleTask`, `describeLaunch` sets it again from the same function's answer
inside `runTask` (`daemon.go:6935`, `:7793`), so the record names what was
launched; a sealed record is not touched (`:353-355`).

The key is never minted again. `uuid.NewString()` occurs four times in the
non-test budget code: the probe name (`budget_usage.go:196`), the epoch
(`:210`), the batch key in `begin` (`:294`), the invocation id (`:373`).
`seal` and recovery hand the record to `buildBudgetUsageBatch`, which copies
`record.BatchKey` (`budget_usage_batch.go:92`).

Remark: the record holds no runtime id. The brief lists "the attempt, task and
runtime ids it needs later"; design 11.10 does not ask for one, and nothing
later needs one: the request path is
`/api/daemon/budget/attempts/<attempt id>/usage` (`budget_usage_delivery.go:141`)
and the credential is chosen by `record.WorkspaceID` (`:330`). `task_id` is
stored and read by no non-test code. Not a finding.

Tests: `TestBudgetUsageRecordWriteAndReadBack` (every field after create, the
file text, the same key after each later step),
`TestBudgetUsageRecordIsOnDiskBeforeTheProviderStarts` (the key on disk at the
first launch is the key at the second launch, of the sealed record and of the
batch), `TestBudgetUsageRecordRefusedForSecondAttempt` (two attempts, two
keys).

Mutants (all killed): `B2-1` `describeLaunch` mints a new key; `B2-2` every
launch intent does; `B2-3` every outcome does; `B2-4` the batch a later
process rebuilds gets another key (six tests of part 2's files); `B2-5` every
record of one process gets the same key (`TestBudgetUsageRecordRefusedForSecondAttempt`
alone); `B2-6` the creator identity is a fresh value per record; `B2-7`
`describeLaunch` replaces the creator identity; `B2-8` every launch intent
moves `created_at`; `B2-9` sealing clears the server address.

### B3. The launch intent is on disk before the provider starts: holds

Own inventory, from `grep` over the non-test code of `internal/daemon/**`
(table in section 3). `backend.Execute` is called in two places:
`daemon.go:8521` in `executeAndDrain`, and
`collaborationrun/host_recording.go:108`. `executeAndDrain` has two callers,
both in `executeAndDrainRecorded` (`budget_usage_capture.go:108` without an
attempt, `:116` with one), and that has two callers, both in `runTask`
(`daemon.go:7969` the first run, `:8025` the fresh-session retry). `runTask`
is reached through `d.runner.run` at `daemon.go:5266` only (and directly from
`rk1k_driver_agentintegration.go:75`, which the default build does not
contain).

The one invocation site, step by step, for a task with an attempt:

1. `budget_usage_capture.go:114` `invocationID := attempt.beginInvocation(resumed)`.
2. `budget_usage.go:372-377`: the invocation (new id, `resumed`, launch time)
   is appended to the record in memory.
3. `:378` `a.persistLaunchIntentLocked()` → `:389` `a.journal.write(a.record)`
   → `writeManagementJSON`: temporary file, fsync, rename, directory fsync, all
   in the calling goroutine. No goroutine, no channel, no deferred write.
4. `beginInvocation` returns (`:379`).
5. `budget_usage_capture.go:116` `d.executeAndDrain(...)`.
6. `daemon.go:8512-8520`: a context is derived and, for a task without a
   knowledge context, `beginCancellationProcess(ctx)` registers the run; no
   process is started.
7. `daemon.go:8521` `backend.Execute(agentCtx, prompt, opts)`.

When the write of step 3 fails, the launch still happens (ruling 1.2); B4 has
what the file then says.

The second `backend.Execute` belongs to a collaboration host run. It sends a
user message, but it is not an invocation of an attempt: such a run is claimed
through another route, has no `Task`, never passes `handleTask` or `runTask`
and gets no `budget_attempt_id` (design 10.11: collaboration runs, host and
contained, are excluded by mechanism and listed as `not_applicable`). The
same holds for the contained run of `collaborationrun/preparation_linux.go:368`.
The other launches of a provider executable (model discovery, version
detection, the openclaw configuration command, the dsh probe) send no user
message and are named as non-invocations by design 11.10.

Compared with the lane's list (its report, section 4): the same sites; its
line numbers are 7 lower throughout. It does not list
`preparation_linux.go:368` and `agents_probe.go:312`; both are outside the
claimed-task path, so nothing follows.

Test: `TestBudgetUsageRecordIsOnDiskBeforeTheProviderStarts`
(`budget_usage_capture_test.go:1463`) reads the record file with `journal.read`
from inside `backend.Execute`, for two launches of one task: at the first the
file holds one invocation with an id and a launch time and no outcome; at the
second it holds two, the first with its outcome. Mutants: `B3-1` (the record
is written before the intent is added to it, so the file is one launch
behind) killed by that test and nine others; `B3-2` (the wrapper writes the
intent after `executeAndDrain` returned; the same idea as the lane's
`sc-intent-after-the-launch`) killed by that test alone.

### B4. The three kinds of failed write: holds (two surviving mutants, one note KD1-03)

The code, read without the lane's description:

- Launch intent, `persistLaunchIntentLocked` (`budget_usage.go:388-401`): when
  `journal.write` fails it calls `os.Remove` on the record file. If the file is
  gone afterwards (removed, or it was not there), one warning that says the
  file was removed; otherwise one warning that carries `remove_error`. It
  touches nothing else: not `a.record`, not `j.attempts`, not the return value
  of `beginInvocation` (the id is returned at `:379` whatever the write did, so
  the launch goes on).
- Outcome and batch bytes, `persistLocked` (`:332-337`), called by
  `endInvocation` (`:422`), `seal` (`:444`) and `describeLaunch` (`:358`): one
  warning, the file stays as it is.
- The live process reports from memory: `seal` builds the batch from
  `a.record` (`:438`), never from the file.

"The next whole-record write brings the file back complete" needs two things,
and both are true. Every write is `journal.write(a.record)`: the whole record
in memory, marshalled anew (`:229-231`; there is no partial or appending
write). And nothing ever takes an invocation out of `a.record`: the only
assignment to `Invocations` is the append at `:377` (own grep), and the failed
write leaves memory alone. So the first later write that succeeds (the
outcome, the retry's intent, the batch) holds every launch, the one whose
write failed included.

What the file says between a failed write and the next good one:

| Failed write | File afterwards | A later process that finds it |
| --- | --- | --- |
| intent, file removed | none | reports nothing; the attempt stays unknown through its hold |
| intent, removal failed too, first launch | the created record, no invocation | recovery removes it with a warning and reports nothing (`budget_usage_recovery.go:172-178`) |
| intent, removal failed too, Nth launch | N−1 launches | reports N−1 invocations (the accepted limit, brief section 6 item 1) |
| outcome | every intent, this one without an outcome | reports it as `unknown` |
| batch bytes | every intent and outcome, no batch | rebuilds the batch under the same key |

Tests: `TestBudgetUsageLaunchIntentWriteFailureRemovesTheRecordFile` (the
write fails after the rename; the directory is empty, one warning without
`remove_error`, the attempt is still the live one, memory holds the launch,
and after the outcome the file holds intent and outcome, the adapter version
and the same key), `…WhenTheRecordFileCannotBeRemoved` (one warning with
`remove_error`, the live process's batch holds the invocation, the file is
byte for byte what it was), `TestBudgetUsageOutcomeAndSealWriteFailuresLeaveTheRecordFile`
(both ways of failing; the file still holds the intent after each; one and
then two warnings; no removal),
`TestBudgetUsageHandleTaskSealsFromMemoryWhenTheJournalFails` (through
`handleTask`).

Mutants: `B4-1` (a failed intent write leaves the file and says it was
removed), `B4-2` (it drops the launch from memory), `B4-3` (it drops the
attempt from the process's table), `B4-4` (a failed outcome or batch write
removes the file), `B4-5` (a failed removal logs two warnings), `B4-7` (a
failed intent write whose file was removed is silent): all killed.

Two survive:

- `B4-6`: an absent file counts as a removal that failed
  (`removeErr == nil` instead of `removeErr == nil || errors.Is(removeErr, os.ErrNotExist)`).
  The `os.ErrNotExist` arm is reached when an intent write fails while the
  file is already gone: the first intent write failed after the rename and
  the file was removed, the outcome write then failed before the rename, and
  the retry's intent write fails again. No test goes there. The arm only
  chooses which of the two warnings is logged; there is still exactly one.
  Note KD1-03.
- `B4-8`: a new launch intent replaces an earlier one that has no outcome.
  Through the code this cannot happen: `executeAndDrainRecorded` calls
  `endInvocation` after every `executeAndDrain` before `runTask` can launch
  again, and the two launches of `runTask` are sequential. No test calls
  `beginInvocation` twice without an outcome between. Not a finding; it is in
  the mutant table so that the gap is known.

### B5. A failed create: holds (one note, KD1-04)

`begin` (`budget_usage.go:304-311`): when the first write fails, the file is
removed best effort (`_ = os.Remove(path)`), the error is returned, and the
attempt is not put into `j.attempts` (the registration is the statement after
the write). `beginBudgetUsageAttempt` (`budget_usage_capture.go:138-147`) logs
one warning ("…running the task without the budget usage protocol"; the
attempt id is named only when it is canonical) and returns nil.

A nil attempt is the absent protocol, statement by statement (the same path
as A2): `withBudgetUsageAttempt(runCtx, nil)` returns `runCtx`
(`budget_usage.go:460-463`), so `budgetUsageAttemptFrom` finds nothing in
`runTask`; `describeLaunch` on nil returns (`:348`);
`executeAndDrainRecorded` calls `executeAndDrain` with the caller's context
and arguments (`budget_usage_capture.go:107-109`): no intent, no origin
holder, no outcome; `finishBudgetUsageAttempt(ctx, nil)` gets nil from
`seal()` and returns before it starts a goroutine
(`budget_usage_delivery.go:266`). No path dereferences the nil attempt
(`TestBudgetUsageNilAttemptIsInert` calls all four methods on nil).

Tests: `TestBudgetUsageRecordCreationFailures` (the path is no directory any
more; the directory cannot be searched, cannot be written, cannot be synced:
no attempt, an error that names the step and is not "exists", zero live
records, an empty directory, and `begin` works again once the directory is
usable); `TestBudgetUsageHandleTaskRunsWithoutARecordThatCannotBeCreated`
(five causes through `handleTask`: the task ran, its context carried no
attempt, it was reported complete once, zero requests to the usage route, the
warning names the cause).

Can a daemon be handed the same attempt id twice? Not by a server that
follows the design: the attempt is written once per claim generation (design
10.2 step 8), and a claim whose answer was lost is claimed again "with the
generation plus one", which supersedes the earlier attempt and makes a new one
(10.2 step 7; 10.11, supplement of 2026-11-20). Inside the daemon a claimed
`Task` reaches `handleTask` once (`daemon.go:4953`); nothing stores a claim
and runs it again (`recordManagedClaims` keeps routing only). It takes a
server or an intermediary that replays a claim response.

What `begin` does then (`budget_usage.go:280-290`): a record that this process
holds, or a file of that name whoever wrote it, gives
`errBudgetUsageRecordExists`; an `Lstat` error other than "does not exist"
refuses as well. The existing record and its key are not touched, the second
task runs without the protocol, and nothing is sent for it. Killed mutants:
`B5-1` (the second task shares the live record) and `B5-2` (`begin` removes
the existing file and starts a record with a new key). Note KD1-04 says what
the batch of that attempt then lacks.

`B5-3` survives: `begin` treats a write error that is not a permission error
as success. Every failing write of the tests is a permission error (the tests
spoil the directory with `chmod`); a full disk cannot be made in a test here.
The code has one branch for every error, so nothing is wrong; the mutant only
shows that the kind of error is not varied.

### B6. Reading a record back: holds (one finding KD1-02 minor, one note KD1-05)

`read` (`budget_usage.go:235-259`), refusal by refusal:

| Refused | Statement | Lane test row (`TestBudgetUsageRecordReadRejectsForeignFiles`) |
| --- | --- | --- |
| a name that is not a canonical UUID | `managedUUID.MatchString(attemptID)`, `:236`, before the path is built | upper case, `not-an-attempt-id`, `nested/<id>`, `../<id>`, each leading to a well-formed record of that name |
| not a regular file | `os.Lstat` (`:240`, does not follow a link) and `!info.Mode().IsRegular()` (`:244`) | a symbolic link to a well-formed record of this attempt; a directory |
| more than 4 MiB | `info.Size() > maxBudgetUsageRecordBytes` (`:244`), before the file is read | a record padded past the bound; one padded to exactly the bound is accepted |
| more than one JSON value | `json.Unmarshal` (`:252`) refuses anything but white space after the value | the record followed by `{}`; a final line break is accepted |
| another on-disk version | `record.Version != budgetUsageRecordVersion` (`:255`) | `"version":2` only (KD1-02) |
| an attempt id that differs from the name | `record.AttemptID != attemptID` (`:255`) | another attempt's record under this name |
| a batch key outside the pattern | `budgetUsageBatchKeyPattern` (`:255`) | `"batch_key":"short"` |

What a pass does with each (part 2's code, read for this point only):
a name that is not `<canonical uuid>.json` never reaches `read`
(`budgetUsageRecordIDs`, `budget_usage_recovery.go:76-87`: skipped, not read,
not reported). For a name that does, any error of `read` goes to
`skipUnreadableBudgetUsage` (`:142-151`): something that is not a regular file
is skipped at every pass with a debug line and is never removed; a regular
file is skipped until its modification time is older than seven days and is
then removed with one warning; the read error is not logged. A pass cannot
fail, loop or stop on such a file: `handleBudgetUsage` returns nothing, each
id is handled once per pass, and `takeRound` hands out at most 32 ids and goes
on after the last one at the next pass, so records that stay unreadable do not
keep the others from their turn. A FIFO or a device of a record's name is
refused by its mode before anything opens it. Test:
`TestBudgetUsageUnreadableRecordIsSkippedUntilTheCapAndThenDropped` (a file
cut short, a later version, a misnamed record, a directory, a link to a
sealed record outside the journal: two passes change nothing and warn about
nothing; past the cap the three regular files go with one warning each, the
directory and the link stay, nothing is sent, no file text is logged).

`read` does not refuse unknown fields, a repeated key or a missing field; the
design does not ask it to. A record that lacks fields is handled by what it
then holds (no `created_at` is older than the cap and dropped; no invocation
and no batch is removed).

Mutants: `B6-1` (only the first JSON value is decoded), `B6-2` (a record of
exactly the bound is refused), `B6-3` (only directories are refused, so a link
is followed), `B6-5` (the bound is 8 MiB and checked after the read): killed.
`B6-4` (a version below the current one is accepted, which includes a file
without the `version` key) survives: KD1-02.

## 2. Findings

(most severe first; added as they are found)

## 3. Launch sites (B3), own inventory

Made with `grep` over the non-test Go files of `server/internal/daemon/**`
before the lane's list was read. Paths are relative to
`server/internal/daemon/`.

Calls of `backend.Execute`:

| Site | What it is | Invocation of an attempt | Launch intent |
| --- | --- | --- | --- |
| `daemon.go:8521`, in `executeAndDrain` | the provider run of a claimed task (issue, chat, autopilot, quick-create, with or without a knowledge context) | yes | `budget_usage_capture.go:114` → `budget_usage.go:377-378`, written and synced before `:116` calls `executeAndDrain` (B3, steps 1 to 7) |
| `collaborationrun/host_recording.go:108`, in `HostExecutor.execute` | a collaboration host run | no: no `Task`, no `handleTask`, no attempt (design 10.11, excluded by mechanism) | none, by design |

Callers of `executeAndDrain` and of its wrapper:

| Site | Caller | Intent |
| --- | --- | --- |
| `budget_usage_capture.go:108` | `executeAndDrainRecorded`, no attempt in the context | none: the task runs without the protocol (A2, A4) |
| `budget_usage_capture.go:116` | `executeAndDrainRecorded`, attempt in the context | `:114`, before the call |
| `daemon.go:7969` | `runTask`, first run, through the wrapper | as above |
| `daemon.go:8025` | `runTask`, fresh-session retry, through the wrapper | as above, a second invocation with its own id |

How `runTask` is reached: `d.runner = taskRunnerFunc(d.runTask)`
(`daemon.go:691`); `d.runner.run` is called at `daemon.go:5266` only;
`handleTask` is called at `daemon.go:4953` only, one goroutine per claimed
task. `rk1k_driver_agentintegration.go:75` calls `runTask` directly; the file
has the build tag `agentintegration && !windows` and its context carries no
attempt.

Other places that start a provider executable; none sends a user message,
none is an invocation (design 11.10, "不算调用的进程"):

| Site | What it starts |
| --- | --- |
| `daemon.go:4177` (`listModels`, outside a task) and `daemon.go:6759` (`resolveTaskModelSelection`, inside `runTask`, called at `:7852`) | model discovery |
| `daemon.go:1122`, `:2384`, `:2895` (`detectAgentVersion`) | version detection |
| `execenv/openclaw_config.go:1335` | the openclaw configuration command |
| `agents_probe.go:312` | the dsh probe |
| `collaborationrun/preparation_linux.go:368` (`executeContainedClaude`; also `:97`, `:192`, `:275` and `controller_linux.go:121`) | the contained collaboration run and its preparation, Linux only: a collaboration run, excluded like the host run |

Not provider executables: `auto_update.go:26` (the daemon's own binary),
`execenv/isolation.go:137`, the `git` calls (`gc.go`, `local_directory.go`,
`repocache`, `execenv/git.go`, `local_worktree.go`), `managed_process.go:40`,
`config.go:1059`, `codex_home_link_windows.go:18`. `pkg/agent` starts
processes only in `launch.go:114` and `:309`, both below `backend.Execute`.

## 4. G3: item, test, mutant

## 5. Mutants

## 6. Runs

## 7. Test files added and the D1 validator

## 8. Disagreements with section 6 of the brief

## 9. Not checked; hooks and reminders

STATUS: in progress (not a final status line)
