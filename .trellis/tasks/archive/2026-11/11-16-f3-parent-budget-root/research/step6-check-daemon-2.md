# Step 6 independent check, daemon half, part 2 of 2 (`f3-check-6kd2`)

Scope: declaring the protocol, sending, resending, and recovery after a
restart (`budget_usage_delivery.go`, `budget_usage_recovery.go`, the changes
to `client.go` and `managed_transport.go`, the start of the loop in
`daemon.go`). Part 1 (`f3-check-6kd1`) has the record and the batch; nothing
here judges those.

This file is written incrementally: one section per point as its verdict is
reached. A section that says `pending` has not been reached yet.

## 0. Start state

- Brief: `/tmp/f3-main-s6k/split/brief-6kd2.md` (the only source of
  instructions).
- Tree checked: `/Volumes/artisan/code/2026/mc-f3-check-6kd2-20261119`,
  detached at `86fea651cc5d09360327b6cf0f8357c1f360070d`, 6 modified and 15
  new files.
- Digest check at start (2026-11-19):
  `cd /Volumes/artisan/code/2026/mc-f3-check-6kd2-20261119 && shasum -a 256 -c /tmp/f3-main-s6k/base-r2/manifest.txt`
  gave 21 lines `OK`, exit 0. The tree is the one that was reviewed.
- No sub-agent was started. No existing file of the worktree was changed.

## 1. Verdict per point

Paths below are relative to `server/internal/daemon/` of the worktree unless
they start with another directory. Mutant ids (`M…`) refer to section 5.

### A1. Declared only with a usable journal: `holds`

My inventory of every place that builds the daemon's `X-Client-Capabilities`
value, made before reading the lane's list (`grep -rn 'X-Client-Capabilities\|capabilities()' server --include='*.go'`
without the test files):

| Place | What it sends | Reads the flag |
| --- | --- | --- |
| `client.go:192` (`setIdentityHeaders`, every HTTP request of a `Client`, the usage report included) | `c.capabilities()` | yes |
| `wakeup.go:137` (the WebSocket dial; callers `wakeup.go:107` with `d.client` and `managed_wakeup.go:91` with `workspace.client`) | `client.capabilities()` | yes |
| `wsrpc.go:323-383` (the `tasks.claim` RPC) | no header of its own: the server reads what the handshake said | through the dial |
| `collaborationrun/client.go`, `managed_lifecycle.go:251`, `cmd/multica/cmd_daemon.go:2113` | no capability header, or a fresh `NewClient` that never claims | not a claim path |

`capabilities()` (`client.go:195-207`) appends `budget-usage-v1` only
`if c.budgetUsageReady.Load()`. The flag is stored in one place,
`setBudgetUsageReady` (`budget_usage_delivery.go:103-116`), which has one
caller, the last statement of `initializeBudgetUsage`
(`budget_usage_delivery.go:245-258`), reached only after
`openBudgetUsageJournal` returned no error (directory made, probe file
written with fsync and removed, directory listed). Both failure returns leave
the flag false and `d.budgetUsage` nil, so `beginBudgetUsageAttempt` returns
nil and no record, no batch and no report exist.

Managed installation: a workspace client made later copies the flag under
`registry.mu` (`managed_transport.go:75-83`); `setBudgetUsageReady` stores
the root flag first and then walks the workspaces under the same lock, so a
client made between the two steps reads the stored root flag and a client
made before is reached by the walk. No interleaving leaves a workspace client
without the capability while the root has it.

The value a WebSocket RPC claim relies on is fixed at dial time. The first
dial is in `taskWakeupLoop` (`daemon.go:2076`), after `initializeBudgetUsage`
(`daemon.go:2053`), so a claim over the socket never precedes the decision.
Requests made before line 2053 (`resolveAuth`, `initializeManagedTransport`)
carry no capability and none of them is a claim.

One thing that is not a defect: the declaration is made once and stays when
the directory becomes unusable later. Design 11.10 covers that case by the
record rule (a task whose record cannot be written runs without the
protocol), not by withdrawing the capability.

Tests: `TestBudgetUsageCapabilityIsDeclaredWithAUsableJournal`,
`…IsNotDeclaredWithoutAUsableJournal` (three ways to spoil the directory),
`…ReachesManagedWorkspaceClients`. Mutants: `MA1-1` to `MA1-6b` in
section 5.1, all killed.

### A5. Order at start: `holds`

`daemon.go`, function `Run`: `listenHealth` at line 1972 with its error return
at 1974; `initializeBudgetUsage` at 2053; `preflightAuth` at 2060;
`go d.budgetUsageLoop(ctx, budgetUsageResendInterval)` at 2083;
`d.pollLoop` at 2092. A second daemon of the same profile fails at 1974
(`health.go:88-95`, "another daemon is already running") before the journal
directory is opened, probed or listed, and before any pass. The first pass is
the first iteration of the loop, so it is after the health port, after the
journal and after the authentication check, and it runs beside the poll loop
rather than before it (a record of an earlier process and a task claimed now
have different attempt ids, so the two do not meet).

Test: `TestBudgetUsageRunOpensTheJournalBeforeThePreflightAndStartsTheResendLoop` in
`budget_usage_wiring_test.go` (a syntax-tree test over the top-level
statements of `Run`, accepted as item 23). Mutants: `MA5-1w` to `MA5-7w` in
section 5.1, all killed, including the one the brief asks about (the call
moved into a branch that is not taken).

### E1. The request: `holds`

`reportBudgetUsage` (`budget_usage_delivery.go:140-169`) is the only place
that builds the request. What it carries and where each part comes from:

| Part | Value | Source |
| --- | --- | --- |
| method, URL | `POST <c.baseURL>/api/daemon/budget/attempts/<attempt id>/usage` | `c.baseURL` is the client's own (`NewClient(cfg.ServerBaseURL)`, a managed child `NewClient(c.baseURL)`); the id is `record.AttemptID`, a canonical UUID checked when the record was created (`begin`) or read (`read`) |
| body | `bytes.NewReader(batch)` | `record.Batch`, a `[]byte` (`budget_usage.go:58-80`, base64 in the file). Between the seal (or the file of an earlier process) and the wire nothing decodes it: `snapshot` clones the slice, `sendBudgetUsage` passes it on. No `json.Marshal` of a batch exists outside `buildBudgetUsageBatch` |
| `Content-Type` | `application/json` | constant |
| `Authorization` | `Bearer <c.Token()>` | the token of the client that sends: the daemon's, or in a managed installation the credential of the record's workspace client (E8) |
| identity and capability headers | `setIdentityHeaders(req)` | the same call every other request of the client makes (`client.go:192`), so a report says `budget-usage-v1` like a claim does |
| bound | the per-call copy of the client (`Timeout` 30 s) and `ctx` | section 1, E2 |

The batch key, the attempt id inside the body and the invocations are not
set here; they are in the bytes. Mutants `ME1-1` to `ME1-6` (method, path,
body re-encoded, no `Content-Type`, no `Authorization`, no identity headers)
are killed, the re-encoding one by
`TestBudgetUsageHandleTaskSendsTheSealedBatch` and the restart test
`…RestartResendsThePersistedBatchByteForByte`, which compare the bytes on the
wire with the bytes of the file.

Two lines of this function are among those the lane believes unpinned
(report 11.4 (f)); they are sorted in section 3.5: the path escape
(`ME1-7b`) and the token guard (`ME1-8`).

### E2. Never in the task's way: `holds` (one note)

Which goroutine. After the run, `handleTask` calls
`d.finishBudgetUsageAttempt(ctx, budgetAttempt)` (`daemon.go:5267`). On the
task's goroutine that function does two local things: `attempt.seal()` (the
batch is built and written with `writeManagementJSON`, under `a.mu`) and
`attempt.attemptID()`. The send itself is
`go func() { … d.deliverBudgetUsage(ctx, attemptID) }()`
(`budget_usage_delivery.go:271-275`), a goroutine of its own, counted by
`journal.background`, which production code never waits on (only the tests'
cleanup does). `handleTask` goes on to the legacy usage report and the
terminal report at once.

What bounds it. (1) The per-call copy of the client keeps the 30 s `Timeout`
of `NewClient` (`client.go:133`), which covers the dial, the headers and the
reading of the answer. (2) `ctx` is the context `handleTask` was given, the
daemon's root context with two values added (`daemon.go:5135-5136`), not
`runCtx`, so it does not end with the run and does end with the daemon. The
two values (the knowledge fence and the cancellation execution) are read
only by code that is handed the context explicitly
(`cancel_receipts.go:251-401`, `daemon.go:5201`, `5755`); no transport reads
them, so a usage report neither writes nor is stopped by a cancellation
receipt.

When the server does not answer. `Do` returns an error after 30 s, the
verdict is `kept`, one Debug line is written, the deferred `release` gives
the mark back and the next pass of the loop sends the same bytes. While the
send holds the mark, a pass skips the record (`claim` fails) and neither
blocks nor sends. At process exit a send in flight is abandoned; the sealed
record is on disk and the next process resends it byte for byte.

No lock is held across the network: `load` and `snapshot` have returned
before `sendBudgetUsage` runs, and the mark (`inFlight`) is an entry in a
map, not a held mutex (G2).

Tests: `TestBudgetUsageHeldOpenReportDoesNotDelayOrChangeTheTerminalReport`
(the usage answer is held open and the terminal report of the task must
arrive meanwhile, unchanged), `…SendEndsWithTheDaemonAndKeepsTheRecord`,
`…UnreachableServerKeepsTheRecord`. Mutants `ME2-1` (the send made on the
task's goroutine), `ME2-2` (the send made under `runCtx`) and `ME2-3` (no
send after the seal) are killed. That the 30 s bound itself has no test is
finding `KD2-01`.

Note (nothing to change): the one budget step on the task's goroutine after
the run is the seal, a local write with two fsyncs. On a file system that
hangs it would delay the terminal report, as every other local write of the
task path would. Design 11.10 orders "persist, then send", so the write
cannot move to the background.

### E3. The status classes: `holds` for the code, `finding` for the test (`KD2-03`)

My table, from `verdict()` (`budget_usage_delivery.go:64-77`) and the
returns of `reportBudgetUsage` that carry an error (table 3.1 has one line
per status):

| What came back | Class | What the daemon does |
| --- | --- | --- |
| an error from `Do` (no connection, the 30 s bound, the context ended) | kept | file untouched, Debug line, resent with the same bytes |
| any status whose answer cannot be read to its end | kept, whatever the status line said | the same |
| 200 to 299 | delivered | record removed, Debug line; the body is not looked at |
| 400, 404, 409, 413, 422 | refused for good | record removed, one warning with the status and the code |
| every other status that Go's client hands over (000 to 199, 300 to 399, the other 4xx, 500 to 999) | kept | as the first line |

This is the rule of design 11.10 as revised on 2026-11-18. Per class, the
test and a mutant that moves one status:

| Class | Test (all are cases of `TestBudgetUsageAnswerDecidesWhatBecomesOfTheRecord`, which looks at the record file, the warning and what a second send carries) | Mutants killed |
| --- | --- | --- |
| delivered | 200 with four bodies, 201, 202, 204, 299 | `ME3-6` (200 kept: the lower bound moved to 201), `ME3-3` (299 kept: the upper bound moved to 298), `ME3-4` (300 delivers) |
| refused | 400, 404, 409 with three codes, 413, 422 | `ME3-13` (400 kept), `ME3-2` (404 kept), `ME9-1` (409 kept), `ME3-8` (413 kept), `ME3-7` (422 kept) |
| kept | 401, 402, 403, 405, 408, 410, 418, 425, 429, 500, 502, 503, 504, eight 3xx cases, two answers that end early, a closed connection | `ME3-1` (401 final), `ME3-12` (403 final), `ME3-11` (429 final), `ME3-9` (503 final), `ME3-10` (an unreadable answer judged by its status line) |

What the test cannot see. It lists statuses one by one, so it pins the
class of each status it lists and nothing between them:

- `ME3-5` (the lower bound of "delivered" moved from 200 to 100) survives.
  The lane lists this line as unpinned (report 11.4 (f)) and says nothing
  observable depends on it. Something does: Go's client consumes the other
  1xx answers but hands a `101 Switching Protocols` to the caller as a
  final answer with an empty body, and a status line below 100 is accepted
  by its parser too. Under the mutant a 101 from a proxy removes the record
  as delivered.
- A status the test does not list can move class unseen: section 5.2 has
  the result for `ME3-14` (412 final) and `ME3-15` (306 delivered).

Pin, added as my test `TestBudgetUsageVerdictOfEveryStatus` (section 7): the
rule written once as a function of the status and `verdict()` checked
against it for every status from 0 to 999, with and without an error. It
fails on each of these mutants.

### E4. Redirects are not followed: `holds` for the rule, `finding` for the other half (`KD2-05`)

Which client. `reportBudgetUsage` does not use the shared client itself: it
makes `httpClient := *c.client` (`budget_usage_delivery.go:152`), a copy of
the `http.Client` value that shares the transport (so connections are
reused) and the 30 s `Timeout`, and sets `CheckRedirect` on the copy to
return `http.ErrUseLastResponse` (line 153). A 3xx then comes back as the
answer it is, its status is "everything else", the record is kept and no
request goes to the `Location`. The POST is never turned into a GET.

Test: the cases 301, 302, 303, 307, 308 (and 300, 304) of
`TestBudgetUsageAnswerDecidesWhatBecomesOfTheRecord`; the fake's `/landing`
answers 200, and the test requires that nothing arrived there
(`budget_usage_send_test.go:629`). `ME4-1` (the `CheckRedirect` line removed)
is killed by it.

That the daemon's other requests did not change. By the code they did not:
the only new write of a `CheckRedirect` field is line 153, on the local
copy; `c.client` is read, not written (`grep -n CheckRedirect *.go`: the
other writes are the managed ones that were there before,
`managed_transport.go:87`, `managed_lifecycle.go:82` and `:252`,
`knowledge_context.go:201`). In a managed installation the shared clients
already refuse redirects, so there the question is empty; in an ordinary
installation `c.client.CheckRedirect` stays nil and `getJSON` and `postJSON`
follow redirects as before.

By a test they are not guarded. `ME4-2` (`httpClient := c.client`, so the
line after it switches following off on the shared client for every later
request of the daemon) survives all 78 tests in a plain run. Under `-race`
the same mutant fails 18 tests (`ME4-2r`), but through the race detector
and not through anything about redirects: the send goroutine writes a field
of the shared client while a request of the task reads it. A variant that
does not race, the same switch made once where the protocol is declared
(`ME4-3`), survives with and without `-race` (`ME4-3r`). So a change of that
kind would be silent: every poll, heartbeat and task report of an ordinary
installation behind a redirecting front would stop following after the
protocol is declared. Pin, added as my test
`TestBudgetUsageReportLeavesTheDaemonsOtherRequestsFollowingRedirects`
(section 7): after a usage report, a task status request of the same client
that is answered with a 302 still arrives at the target.

### E5. Nothing new on any failure (B33): `holds`

The kept branch of `sendBudgetUsage` (`budget_usage_delivery.go:346-349`)
writes one Debug line and returns. It does not call `journal.write`, does
not touch the attempt and does not build anything: the record in memory and
the file are what they were, so the next send has the same bytes, the same
key inside them and the same path. A batch is built in two places only,
`seal` (once per attempt: a sealed attempt returns its bytes again,
`budget_usage.go:435-437`) and `recoverBudgetUsage` for a record without
batch bytes that an earlier process left; a record with batch bytes never
reaches the second (`budget_usage_recovery.go`, the "What was persisted is
what is sent" branch). Nothing in the three files creates an attempt id, an
invocation id or a batch key after `begin` and `beginInvocation`.

Tests: every kept case of `TestBudgetUsageAnswerDecidesWhatBecomesOfTheRecord`
requires the file byte for byte unchanged and a second send with the same
body, key and path; `…RestartResendsThePersistedBatchByteForByte` does the
same across a restart. Mutants: `ME5-1` (recovery builds again for a record
that has batch bytes) killed by ten tests, `ME5-2` (a kept answer unseals
the stored record) killed by fourteen.

### E6. A 404 of this path removes this record and does nothing else: `holds`

The 404 reactions of the daemon work on a `*requestError`, which four
places of `client.go` build (lines 764, 1214, 1259, 1293: the JSON helpers
and two raw requests) and no budget file does
(`grep -n '&requestError{' *.go`): `isRuntimeNotFoundError`
(`daemon.go:3969`, `4114`: drop the runtime and register again) and
`isTaskNotFoundError` (`daemon.go:5071`: interrupt the running agent; the
same line also interrupts on a 401 through `isUnauthorizedError`). Each of
them starts with `errors.As(err, &reqErr)` and reads the answer's text.
`reportBudgetUsage` goes through neither helper. It calls `Do` on its own
copy of the client and returns a `budgetUsageReply` value, a struct with a
status, a code and an error, which no caller hands to those checks: its one
consumer is `sendBudgetUsage`, whose 404 path is the refused branch (one
warning, the record removed) and whose 401 path is the kept branch. The
reply never becomes an `error` that leaves the budget code, and the text of
the answer is not kept at all.

Test: `TestBudgetUsageNotFoundAnswerOnlyDropsTheRecord`, once for each of
the three texts the other routes react to ("runtime not found", "task not
found", "workspace not found"). While the provider of a task is in the
middle of its run under `handleTask`, the report of another attempt is
answered 404 with that text. The test requires the refused record gone with
one warning (status 404, no code, the text not in the log), the record of
the running task still held, the runtime still registered, the agent not
interrupted and the task not ended; then the task finishes, is reported
complete once and never failed, and no request outside the task's own
routes was made. `ME6-1` (a 404 of the usage route clears the daemon's
runtime index) is killed by it.

### E7. The code in the warning, the logs, the bounded read: `holds`, with one `finding` (`KD2-04`)

The code. `budgetUsageErrorCode` (`budget_usage_delivery.go:83-99`) decodes
the kept part of the answer, takes `code`, without one `error`, and returns
it only when it matches `^[a-z][a-z0-9_]{0,63}$` (required of both fields,
the accepted item 19). The warning of a refusal carries `attempt_id`,
`status` and, when there is one, `code` (lines 341-345). Mutants `ME7-1`
(form not checked), `ME7-2` (`error` never consulted), `ME7-3` (`error` wins
over `code`), `ME7-4` (the empty return on a decode error removed, one of
the lane's "unpinned" lines: a test kills it after all), `ME7-7` (the
warning carries the answer itself), `ME7-8` (no status in the warning) are
all killed, by `…RefusalWarningNamesTheCodeAndNeverTheBody` and the case
table.

Every log call of the two new files, and what each can print:

| Place | Level | Attributes | Can it print a body |
| --- | --- | --- | --- |
| `budget_usage_delivery.go:248`, `:253` journal not usable | Warn | `error` (a path error of the profile or journal directory) | no |
| `:332` workspace without a valid binding | Debug | `attempt_id`, `error` (one of two fixed sentences of `workspaceLocked`) | no |
| `:338` delivered | Debug | `attempt_id`, `status` | no |
| `:345` refused for good | Warn | `attempt_id`, `status`, `code` (pattern-checked) | no: the code is at most 64 characters of `[a-z0-9_]` |
| `:347` kept | Debug | `attempt_id`, `status`, `error` (an error of `http.Client.Do`, or "read the answer: …" around a transport error) | no; see below for what the error names |
| `:357` file of a finished record cannot be removed | Warn | `attempt_id`, `error` (a path error) | no |
| `budget_usage_recovery.go:74` directory cannot be listed | Debug | `error` (a path error) | no |
| `:130` older than seven days | Warn | `attempt_id`, `created_at` | no |
| `:145` unreadable, skipped | Debug | `attempt_id` | no (the read error, which could quote file content, is deliberately not logged) |
| `:148` unreadable and old, dropped | Warn | `attempt_id` | no |
| `:164` another server | Debug | `attempt_id` (neither address is printed) | no |
| `:175` names no launch | Warn | `attempt_id` | no |
| `:183` leftover of this process | Warn | `attempt_id` | no |
| `:195` batch cannot be built | Debug | `attempt_id`, `error` (an encoder error) | no |
| `:201` batch cannot be persisted | Debug | `attempt_id`, `error` (a path error) | no |

Fifteen calls; none takes the batch, the answer or a header. The `error` of
the kept line is a `*url.Error`, which names the method and the URL: the
server address and the attempt id, not the credential (it travels in a
header) and not a body. Mutants `ME7-9`, `ME7-11`, `ME7-12` (the kept line,
the warning and the delivered line print the batch) are killed by
`budgetUsageTestRequireNoBodyLogged`, which the send tests call on the
whole log at Debug level.

The bounded read. `io.ReadAll(io.LimitReader(resp.Body, 64 KiB))` keeps the
head; `io.Copy(io.Discard, resp.Body)` then reads the rest in a fixed
buffer without keeping it (lines 159-163). An answer of a gigabyte costs
time, bounded by the client's 30 s, and no memory. `ME7-5` (the whole answer
read into memory) and `ME7-10` (64 MiB kept) are killed by one case of
`…RefusalWarningNamesTheCodeAndNeverTheBody`, "a code behind more text than
the daemon reads for it": a code placed after 256 KiB of text must not be
found. That pins the bound through what the daemon can see of the answer,
which is the observable side of it; no test measures memory.

The finding: `ME7-6` (the copy of the rest removed) survives. With that line
gone an answer is judged by its status line once its first 64 KiB are read,
so a long answer that breaks off after that point would remove the record
as delivered or refused, against the accepted item 18 ("an answer that
cannot be read to its end keeps the record whatever its status line said").
The two "ends early" cases of the table use an answer of 20 bytes, which
breaks off inside the kept part, where `io.ReadAll` already reports it. Pin,
added as my test `TestBudgetUsageLongAnswerThatEndsEarlyKeepsTheRecord`
(section 7).

### E8. Only to its own server: `holds` for the code, `finding` for the test (`KD2-02`), one hole in the rules (`KD2-10`)

Rule: design 11.10, "a record goes back only to the server that created
it: the record stores the server address at its creation, a record whose
address differs from the current one is not sent and stays until the 7-day
cap … in a managed installation the credential is chosen by the workspace of
the record, and without a valid binding for that workspace the record stays
unsent"; accepted item 22 (an exact string comparison).

How two addresses are compared. `recoverBudgetUsage`
(`budget_usage_recovery.go:163`) compares `record.ServerURL` with
`journal.serverURL` as two strings, after the retention check and before
anything is built, written or sent. Both strings were once
`cfg.ServerBaseURL`, which is always the output of `NormalizeServerBaseURL`
(`config.go:689-710`): trimmed, parsed, `ws`/`wss` turned into
`http`/`https`, a path `/ws` dropped, query and fragment cleared, trailing
slashes cut. So:

| Difference in spelling | Survives normalization? | Effect on the records of the old spelling |
| --- | --- | --- |
| a trailing slash | no | none, they are sent |
| upper case in the scheme | no (`url.Parse` lower-cases it) | none, they are sent |
| upper case in the host | yes | kept unsent until the cap |
| the default port spelled out (`:443`) | yes | kept unsent until the cap |
| a trailing dot after the host | yes | kept unsent until the cap |
| a path (`/api`) | yes | kept unsent until the cap |
| `http` instead of `https` | yes | kept unsent until the cap |

A difference in spelling alone can therefore strand every record of the old
spelling (the accepted item 22) and cannot let one through to another server:
the comparison only ever errs towards keeping. A record this process holds is
sent without the comparison (`handleBudgetUsage`, the `held.attempt != nil`
branch), which is sound: `begin` stamps `ServerURL: j.serverURL`
(`budget_usage.go:297`) of the same process, and the client that sends it was
built from the same `cfg.ServerBaseURL`. In a managed installation
`sendBudgetUsage` takes the client of `record.WorkspaceID` through
`budgetUsageTransport`, and a workspace without a valid binding returns an
error and keeps the record with a Debug line.

Mutants killed: `ME8-1` (the comparison removed), `ME8-2` (the comparison
after build and write), `ME8-5b` (retention and server tests swapped: a
record of another server is then never dropped), `ME8-6` (a managed
installation sends with the installation's client), `ME8-7` (a workspace
without a valid binding falls back to the installation's client), `ME8-8`
(… borrows the client of another workspace), `ME8-9` (an expired binding
still sends). The killers are in section 5.

The finding (`KD2-02`): `ME8-3` (`strings.EqualFold`) and `ME8-4` (a
trailing slash ignored on both sides) survive every test of the lane. Both
are lines of brief 5.1 item 4.
`TestBudgetUsageRestartKeepsARecordOfAnotherServer` uses one other address,
`https://another.example.test`, which differs in the host name, so a
comparison that forgives any spelling passes it. The rule that decides is
"exact", and the test named after the rule cannot tell exact from
case-insensitive. Pin, added as my test
`TestBudgetUsageRestartKeepsARecordWhoseServerAddressIsSpelledDifferently`
(seven spellings, a sealed and a running record under each; two passes: no
request at all, every file byte for byte the same, no warning). It kills both
(`ME8-3-k`, `ME8-4-k`).

The hole in the rules (`KD2-10`). The record names the server and the
workspace. It does not name the account. In an installation that is not
managed, `budgetUsageTransport` returns the daemon's own client, whose token
is whatever the profile holds now. After another account signs in on the same
profile against the same server, the records the former account left are sent
with the new account's token. By design 11.3 the server reads the attempt,
then checks that the caller can enter the attempt's workspace and reach its
runtime; a caller who cannot gets the same 404 as for an attempt that does not
exist. The daemon takes a 404 for a final refusal, warns once and removes the
record. So: no usage is recorded under a wrong account (the server decides by
the attempt row, not by the caller), nothing reaches another server, and the
batch of the former account is lost although that account could have
delivered it. When the new account has access to the workspace the batch is
accepted. This is not the blocker clause of the brief ("goes with another
workspace's credential" is the managed case, and the code handles it). The
cancellation receipts of the same daemon are scoped by server, deployment,
installation, user, workspace and binding (`cancel_receipts.go:27-32`, the
credential check at 244); design 11.10 decides server and workspace for usage
records and says nothing about the account. Whether a batch lost at an
account switch is acceptable is the main session's to rule.

### E9. The collision of ruling 1.3: `holds`

Rule: ruling 1.3 of round 2 (a batch that a later process rebuilds from the
record file can differ from the one the dead process sent from memory under
the same key; the server answers 409 `operation_conflict`; that is a final
refusal like every other and nothing new is minted).

The code has no branch of its own for it, which is the rule: a 409 is in the
list of `verdict` (`budget_usage_delivery.go:64-77`), `sendBudgetUsage` warns
with the status and the code and removes the record.

The test names the cause:
`TestBudgetUsageRebuiltBatchAnsweredWithOperationConflictIsDropped`
(`budget_usage_recovery_test.go:814`). An earlier process ends an invocation
and leaves a record with the outcome and without batch bytes ("the write of
the seal failed. The earlier process sent its batch from memory and died").
A later process rebuilds, the server answers 409 with
`{"error":"operation_conflict","code":"operation_conflict"}`. Asserted: one
report, on the attempt's path, under the key of the record, with the one
invocation as it ended; the record dropped with one warning that carries
`status=409` and `code=operation_conflict`; the body not in the log; after
two more passes one request in all, an empty directory, nothing held, one
warning.

Mutants: `ME9-1` (a 409 kept) is killed by this test and by
`TestBudgetUsageAnswerDecidesWhatBecomesOfTheRecord`; `MG3-1` (recovery mints
a new key for the batch it builds) fails this test too, at "batch_key … want
the key of the record".

One remark, not a finding: the test does not show the collision itself (the
fake server answers 409 to anything), only what the daemon does with the
answer. Whether the real server answers `operation_conflict` to different
bytes under one key is the server half's to prove.

### F1. The decision table of recovery: `holds` (two notes: `KD2-13`, `KD2-14`)

My own table, written from `handleBudgetUsage`
(`budget_usage_delivery.go:288-324`), `budgetUsageRecordIDs`,
`skipUnreadableBudgetUsage` and `recoverBudgetUsage`
(`budget_usage_recovery.go:68-91`, `142-206`), is table 3.2: fourteen lines
in the order in which the code decides, the first line that applies ends it,
each with its test and the mutants it kills.

Compared with the table of the brief, line by line:

| The brief's line | Line of 3.2 | Same outcome? |
| --- | --- | --- |
| batch bytes, any creator: sent as they are, an empty batch included | 11 (not held), 6 (held) | yes; `…RestartResendsThePersistedBatchByteForByte` has the case "an empty batch" |
| no batch bytes, earlier process, at least one invocation: built, sealed, sent | 14 | yes |
| no batch bytes, no invocation, any creator: removed with one warning | 12 | yes |
| no batch bytes, this process, not among the running attempts: removed with one warning | 13 (and 12 when it names no invocation) | yes |
| no batch bytes, this process, a running attempt: not touched | 4 | yes, at any age |
| a name that is not `<canonical uuid>.json`: skipped | 1 | yes: never read, no log line |
| a record `read` refuses | 8 | the code: one Debug line per pass, nothing sent, the file kept; when it is a regular file last changed more than seven days ago, one warning and removed; a directory or a link of that name is never removed |
| a record of another server address: kept, not sent | 10 | yes: nothing built, nothing written |
| older than 7 days: dropped with one warning | 9 (not held), 5 (held and sealed, by a pass) | yes, with one exception by design: the record of a running attempt is never capped (line 4) |

What my table has and the brief's has not: a record that is being handled by
the other sender (line 3), a sealed record this process still holds (lines 5
and 6), an answered record whose file could not be removed (line 7), and the
old temporary files (line 2).

Two lines that apply to one file:

- **Old and with batch bytes.** The cap wins: `recoverBudgetUsage` tests the
  age first (`budget_usage_recovery.go:156`), and for a record this process
  holds the pass does (`budget_usage_delivery.go:306`). The batch is dropped
  with the warning and not sent. The design decides it: 11.10, "a record is
  kept at most 7 days; once expired it is dropped with a warning", and
  "everything else is kept and resent, with the 7-day cap as the backstop".
  Mutant `MF1-10` (such a record is sent instead) is killed by
  `…RecordOlderThanSevenDaysIsDroppedWithAWarning`.
- **Another server and without an invocation.** The server wins: the
  comparison (`:163`) comes before the test for an invocation (`:172`), so
  the file is kept, untouched and without a warning, until the cap. The
  design does not decide it. 11.10 has both sentences ("a record whose
  address differs from the current one is not sent and stays until the
  7-day cap"; "recovery does not report a record without batch bytes and
  without any launch intent, whichever process created it: remove it and
  warn") and no order between them. Nothing is reported in either order;
  the difference is one file that stays up to seven days, and that the
  process of the other server, if it comes back, removes it with the
  warning. The lane's report states the choice (11.3, choice 11) and
  `…RestartKeepsARecordOfAnotherServer` pins it (`MF1-4`, such a record is
  removed, is killed). A hole in the rules, nothing to change in the code:
  note `KD2-13`.
- A third pair, for completeness: **another server and old.** The cap wins
  (`ME8-5b`, the two tests swapped, is killed), which is the design's
  "stays until the 7-day cap".

Note `KD2-14`: `read` does not look at `created_at`. A record file without
one (the zero time) is taken for older than seven days and dropped at the
first pass with the warning of the cap, where every other file that is not a
record of this version is kept until its last change is seven days old. The
daemon always writes the field, so only a file from elsewhere can lack it.
What `read` refuses is part 1's point; what the pass then does is said here.

### F2. A rebuilt batch is sealed before it is sent: `holds`

`recoverBudgetUsage`, the last branch (`budget_usage_recovery.go:193-205`):
(1) `buildBudgetUsageBatch(record, journal.epoch, journal.now())`; on an
error one Debug line and return, the record as it was. (2)
`record.Batch = batch; journal.write(record)`, the atomic write (temporary
file, fsync, rename, directory fsync). On an error one Debug line and return:
**nothing is sent**. (3) `d.sendBudgetUsage(ctx, journal, record)`.

When the write fails, the bytes just built were never sent, so no byte
string has gone out under the key. The next pass reads the file again. If
the failure came before the rename, the file is the old one and the pass
builds again (the same start identity, a later `reported_at`): still the
first bytes ever sent. If it came after the rename (the directory fsync),
the file already holds the bytes of the first build and the next pass sends
exactly those (line 11 of table 3.2). In both cases one byte string per key.

Mutants: `MF2-1` (the rebuilt batch is sent without being written into the
record) is killed by `…RecoveryDoesNotSendABatchItCouldNotPersist` and
`…RestartReportsAnInvocationWithoutAnOutcomeAsUnknown` (the latter reads the
file at the moment the request arrives); `MF2-2` (sent although the write
failed) by the first of the two; `MF2-3` (the rebuilt batch carries the
start identity of the dead process) by
`…RestartKeepsARecordOfAnotherServer` and the `unknown` test.

The live process is different by ruling, not by accident: `seal`
(`budget_usage.go:429-446`) builds, writes and returns the bytes also when
the write failed (`persistLocked` only warns), and the send goes out from
memory. That is ruling 1.2 item 2 and the collision of ruling 1.3 (E9).

### F3. "This process" and "an earlier process": `holds`

The start identity is `journal.epoch`: `uuid.NewString()`, a random
version 4 UUID, made once in `openBudgetUsageJournal`
(`budget_usage.go:210`), which `initializeBudgetUsage` calls once per start
of the daemon. `begin` stores it in the record (`ProducerEpoch: j.epoch`,
`:295`); recovery compares `record.ProducerEpoch == journal.epoch`
(`budget_usage_recovery.go:180`); a batch carries the identity of the
process that builds it (`seal` and recovery both pass `journal.epoch`).

It is not derived from the clock, the process id, the host or the
directory. So a restart within the same second, a clock set back and a
copied state directory all give another identity; two processes share one
only by a collision of 122 random bits. A copied directory is then read as
"left by an earlier process" by whoever opens the copy, and both copies
would report the same attempts under the same keys with different bytes;
that is two daemons on one set of records, outside the accepted item 4.

Mutant `MF3-1` (every process gets the same identity) is killed by
`TestBudgetUsageJournalOpen` and nine other tests.

### F4. The running attempts: `holds` for the code, `finding` for the test (`KD2-12`)

The table `journal.attempts` is written at two places: `begin` adds the
entry after the file is written (`budget_usage.go:312`), and `remove`
deletes it together with the file (`budget_usage_delivery.go:223-224`),
also when the file cannot be removed. `remove` is the only removal of a
record that the sending and recovery code has, so whatever removes a record
drops the entry (`MF4-1`, a removal that leaves the entry, is killed by nine
tests). The exception of B4 is as ruled: `persistLaunchIntentLocked` removes
the file with `os.Remove` and leaves the entry (`budget_usage.go:393`).

The reverse does not exist. Every caller of `remove` was listed:
`sendBudgetUsage` after an answer (the record is sealed),
`dropOutlivedBudgetUsage` (sealed and held, or not held),
`skipUnreadableBudgetUsage` and the two removals of `recoverBudgetUsage`
(reached only when `load` found no entry), and the settled branch (no
entry). For an entry that exists and has no batch bytes, `handleBudgetUsage`
does nothing at all (`:303`), whoever calls it and whatever the age. An
unsealed attempt therefore never leaves the table, and its file is never
removed by a pass.

A record that this process is creating cannot be taken for a leftover
either: `begin` holds `journal.mu` from its check for an existing file to
the entry in the table, and `load` looks at the table and reads the file
under one hold of the same lock. By reading. No test of the lane pins this
lock (its report says so, 11.3 choice 17); see the end of this point.

A task that ends at the moment a pass runs. T is the task's goroutine, S
the send goroutine that `finishBudgetUsageAttempt` starts, P the pass.

| Who is first | Steps | Outcome |
| --- | --- | --- |
| P claims before the seal starts | P: `claim`, `load` (held), `snapshot` (no batch): nothing, `release`. T: `seal`, starts S. S: `claim`. | If S claims after P released: S sends. If S claims while P still holds the mark: S returns; the sealed record is in the table and the next pass sends it (at most one interval later). |
| P claims while the seal writes | P: `claim`, `load` (held), `snapshot` waits for `a.mu`, which `seal` holds across its write. T: `seal` ends, starts S. P: sees batch bytes: sends (or drops, F7a). S: `claim` fails, or finds no entry afterwards: nothing. | Sent once, by P. |
| P claims after the seal and before S | P: sees batch bytes: sends (or drops, F7a). S: as above. | Sent once, by P. |
| S claims first | S: sends, removes on an answer. P: `claim` fails; a later pass finds no entry and no file: one Debug line. | Sent once, by S. |

In no order is an unsealed record removed, and in no order do two requests
go out at once. The one order that loses a batch is that of F7a, and it
loses a sealed batch of a record older than the cap, not the record of a
running attempt.

`begin` refuses an attempt id that already has a file
(`errBudgetUsageRecordExists`) and the task then runs without the protocol,
so a leftover of an earlier process is never adopted by a new run. The
server issues an attempt id once, so this is a guard, not a path.

**The lock of `begin` has no test: `KD2-12`.** Mutant `MF4-3`: `begin` lets
go of `journal.mu` while it writes the record file and takes it again for
the entry. It passes all 78 tests of the lane. What it does:

- A pass that lists the directory during that write finds a file and no
  entry, reads a record of this process that names no launch, and removes
  it with the warning "names no provider launch" (line 12 of table 3.2).
- In the usual order the entry is made afterwards, and the next write of
  the record brings the file back: a false warning and a file that is
  missing for a moment.
- When the entry is made between the pass's `load` and its `remove`,
  `remove` deletes the entry as well. The run goes on with a record that no
  pass knows: every later pass removes its file as a leftover of this
  process (line 13), and the send that follows the seal finds nothing held.
  Nothing is ever reported for a run that launched. That is the blocker
  this point names.

My test `TestBudgetUsageRecordBeingCreatedIsNeverTakenForALeftover`
(`budget_usage_begin_lock_check6kd2_test.go`) creates 24 records while two
goroutines make passes without a pause, and wants every attempt still held,
every file in place, no warning and no report. On the tree as checked it
passes 50 of 50 (`begin-lock-alone-50`). On `MF4-3` it fails 50 of 50
(`begin-lock-mf43-50`; also `MF4-3-k`, and `MF4-3-kr` with `-race`): in
every run all 24 files were removed with the warning, and in one of the
1,200 creations the entry was taken too. It is a stress test and says so in
its comment: there is no seam between the write and the entry (the clock is
read before the write), so it relies on a pass falling into the window of
the two fsyncs.

`MF4-2` (`load` lets go of the lock before it reads the file) passes every
test, mine included, and nothing observable changes: a pass takes
`journal.mu` in `claim` after it listed the file, so it waits for `begin` in
any case, and an id is begun once. The comment on `load` credits its single
hold with what `begin`'s hold and that wait already give. Sorted in
section 3.5.

### F5. The same record, two senders: `holds`

Both senders come through one function. The send that follows the seal is
`deliverBudgetUsage` → `handleBudgetUsage(ctx, id, false)`, a pass is
`resendBudgetUsage` → `handleBudgetUsage(ctx, id, true)`
(`budget_usage_delivery.go:282`, `budget_usage_recovery.go:104`); nothing
else calls `sendBudgetUsage`, `recoverBudgetUsage` or `reportBudgetUsage`
(grep over the package, test files left out).

What keeps them apart is the mark: `handleBudgetUsage` begins with
`journal.claim(id)` and returns when it is taken; `release` is deferred, so
the mark covers the lookup, the request and the removal
(`budget_usage_delivery.go:293-296`, `174-188`; a map entry under
`journal.mu`, never waited for). The daemon does not rely on "sending twice
is harmless": within one process two requests for one record never overlap.

- `M2-1` (`claim` always says yes): **two requests do go out.** It is killed
  by `…PassDoesNotSendARecordThatIsBeingSent` and
  `…RecordIsNeverSentTwiceAtOnce` ("2 usage reports while the first send is
  under way, want 1").
- `M2-3` (the release made at once): killed by the same two tests.
- `M2-2` (the release dropped): killed by seventeen tests; a kept record
  would never be sent again by this process.

The removals and the table. Whoever gets an answer first calls
`journal.remove`: entry and file go under one hold of `journal.mu`. The
other sender then finds nothing to do: the send that follows the seal finds
no entry and returns (`:315`); a pass finds no entry and no file and writes
one Debug line (line 8 of table 3.2). After a kept answer nothing was
removed, the record stays held and sealed, and the next pass sends the same
bytes from memory. The order in which a pass takes the mark while the
record is still unsealed is in F4's table: the send that follows the seal
then returns and the next pass sends, at most one interval later.

Across two processes nothing keeps two senders apart (two daemons on one
profile, accepted item 4); there the server's answer to a repeated key is
what makes it harmless, which is the server half's to prove.

Runs with `-race`: the twelve tests of this area ten times each
(`f5-race-10`, section 6.2): exit 0, 120 top-level tests and 190 at every
level pass, no data race reported. The two baseline runs of all budget
tests with `-race` (section 6.1) passed as well.

### F6. The background pass: `holds`, with two findings about tests (`KD2-08`, `KD2-11`) and a note (`KD2-15`)

**Start and stop.** One goroutine, started once:
`go d.budgetUsageLoop(ctx, budgetUsageResendInterval)` is a top-level
statement of `Run` (`daemon.go:2083`), after the journal is opened and
before the poll loop (A5), under `Run`'s context. A daemon without a journal
has no loop (`budget_usage_recovery.go:112`; `MF6-4` killed). The loop ends
at `ctx.Done()`; inside a pass the context is looked at before every record
(`:100`), and a request in flight ends with it because the request is made
under the same context. `Run` does not wait for the goroutine and nothing in
production waits for the sends of `journal.background` (only tests call its
`Wait`): at exit a request in flight is given up, the sealed record stays,
and the next process sends the same bytes.

In the lane's tests no goroutine stays behind: the loop test starts the loop
itself, and its `stop` cancels and waits for the goroutine, also as a
cleanup (`budget_usage_recovery_test.go:1258-1269`). Mutants: `MF6-9` (the
loop does not end with its context) is killed by that wait running into the
test timeout; `MF6-3` (a pass goes on after the daemon was told to stop) by
`…PassAfterTheDaemonStoppedHandlesNothing`; `M9-b` (a send under
`context.Background()`) by `…SendEndsWithTheDaemonAndKeepsTheRecord`.

**How often.** A pass at start, then one per tick of a `time.Ticker` of 30
seconds (`:111-125`; `MF6-1`, no pass at start, and `MF6-6`, none after the
first, are killed by `…LoopPassesAtStartAndThenAtEveryTick`). Passes never
overlap, being one goroutine. A pass that takes longer than the interval is
followed at once by the next one, and by no more than one: the ticker holds
a single tick and drops the rest. A pass handles at most 32 records and the
next goes on after the last of them (`M6-1`, `M6-1b`, `M6-2`, `M6-3`, `M6-4`
are killed by `…PassHandlesAtMostThirtyTwoRecordsAndTakesUpWhereItStopped`).
No loop is tight: every path of a pass ends, and between two passes stands
the tick.

**A server that answers 503 forever** gets, per kept record, the one send
after the seal and then one request per pass that takes the record:

| Kept records | Per record | In all |
| --- | --- | --- |
| up to 32 | 1 every 30 s: 120 per hour, 2,880 per day, at most 20,160 until the cap | 120 per hour and record |
| n above 32 | 1 every n/32 passes on average, never rarer than every ⌈n/32⌉ | 32 per pass: 64 per minute, 3,840 per hour, whatever n is |

That one pass sends a record once, and not once for the table and once for
the file, was not pinned (`MF-sort`, `KD2-07`); my
`…PassHandlesARecordThisProcessHoldsOnce` pins it. Against a server that
holds every request open the loop makes one request at a time, each up to
30 seconds (accepted item 17). Bounded, then, by the interval, the 32 and
the seven days; it ends for a record when the server answers 2xx or one of
the five refusals, or at the cap.

Note `KD2-15`: nothing spaces the attempts out. There is no backoff and no
jitter, and a `Retry-After` of a 429 or 503 is not read (no such word in the
budget files); every daemon of a deployment sends at the same rate to a
server that is down or says it is overloaded. The design asks for nothing
else (11.10: "kept and resent, with the 7-day cap as the backstop") and the
lane states the choice (its report 11.3, choice 8). For the main session to
know; nothing to change in this slice.

**The interval has no test: `KD2-08`.** `MF6-7` (a pass every 30 ms instead
of every 30 s, which is up to 120,000 requests per hour and record against a
server that is down) passes all 78 tests of the lane: they hand the loop an
interval of their own, the syntax-tree test pins that `Run` passes the
constant, and nothing pins the constant, as the retention
(`budget_usage_recovery_test.go:660`) and the 32 (`:1143`) are pinned. My
`…ResendIntervalIsThirtySeconds` kills it (`MF6-7-k`).

**A test of the lane that depends on timing: `KD2-11`.** The subtest
"stops with the daemon" of `…LoopPassesAtStartAndThenAtEveryTick`
(`budget_usage_recovery_test.go:1318-1331`) waits for three reports at the
fake server, stops the loop, reads the fake's count, sleeps 50 ms and wants
the count unchanged. It failed once in a run of all budget tests (the plain
run of `XF7a-fix`) with "1 usage reports went out after the loop ended"
(`:1326`). Re-run alone as the brief asks, on the unchanged tree, 200 times
(`loop-alone-200`): 13 of 200 fail, all at that line, with the machine under
load (load average 17 to 24). The cause is the test, not the loop:
cancelling the context makes the client's `Do` return at once while its
request can already be on the wire, and the fake's handler registers a call
only after it has read the body, which can be after `stop()` returned and
the count was read. Shown with an overlay of the test file only
(`XLOOP-fix`, `loop-overlay-200`): the same subtest with the requests
counted where the daemon starts them passes 200 of 200, and in 11 of those
runs the fake registered one more report after the loop had ended while the
daemon had started none. For the product it means what F8 line 7 says: a
request given up at shutdown may still arrive, and the next process sends
the same bytes under the same key. No kill of a mutant in this report rests
on that line (`MF6-1` fails at `:1282`, `MF6-4` at `:1341`, `MF6-6` at
`:1302` and `:1321`, `MF6-9` by the timeout).

### F7. The 7-day pass: `holds` (one note: `KD2-16`)

**Measured from what.**

| What | Its age is counted from | Where |
| --- | --- | --- |
| a record that can be read, held or not | `created_at` in the record, set once by `begin` (`budget_usage.go:296`) and carried through every rewrite | `budget_usage_recovery.go:156`, `budget_usage_delivery.go:306` |
| a file with a record's name that cannot be read as one | the file's modification time (`os.Lstat`) | `budget_usage_recovery.go:143-144` |
| a temporary file (`.`…`.tmp`) | the file's modification time | `budget_usage_recovery.go:85` |

So a rewrite does not move the cap of a record: `MF7-1` (a readable record
measured from its file's modification time) is killed by
`…RecordOlderThanSevenDaysIsDroppedWithAWarning`; `MF7-2` and `MF7-3` (the
seven days start again at the seal, at every outcome) by
`…BatchOfARunThatOutlastedTheCapIsSentOnce`, `…HandleTaskRecordsAndSeals`
and `…RecordWriteAndReadBack`. The modification time is used only for files
the daemon never rewrites. "Older" is strictly more than seven days
(`outlived`, `:30-32`): `M5-2` (exactly seven days is dropped) and `M5-2c`
(seven days and one second is kept) are killed by three tests each; `MF6-8`
(eight days) by the test of the number.

**The clock.** It is the wall clock: `journal.now` is `time.Now`
(`budget_usage.go:212`) and `created_at` is stored through `.UTC()`, which
drops the monotonic reading, so also inside one process the age is a
difference of wall times. A clock set forward by a month:

- **The record of a running attempt is not touched.** A pass does nothing
  with a held record that has no batch bytes, whatever its age
  (`budget_usage_delivery.go:303`). `…RecoveryLeavesARunningAttemptAlone`
  advances the clock by seven days and an hour over a running attempt and
  wants the file byte for byte and no warning; `M5-4a` (such a record is
  dropped) and `M5-4b` (it is sent) are killed by it.
- When that attempt ends, its record is older than the cap: the send that
  follows the seal tries it once (the age is not looked at there) and the
  next pass drops what was not delivered. The order of F7a applies to it,
  although the run was short.
- Note `KD2-16`: every other record is dropped at the next pass, unsent and
  with the warning of the cap: the sealed records this process still holds
  because the server did not answer, and everything an earlier process
  left. Their executions stay `unknown` on the server through its hold,
  which is what the design says of an expired record; a daemon cannot tell a
  wrong clock from a late one. For the main session to know.

A clock set back keeps records longer than seven days of real time and
loses nothing.

**Temporary files and foreign names.** The atomic write names its temporary
file `.installation-*.tmp` (`managed_identity.go:214`) and the probe of the
directory is `.probe-<uuid>.tmp` (`budget_usage.go:196`); both match. In the
listing of every pass a name that starts with `.` and ends in `.tmp` is
removed when it is a regular file last changed more than seven days ago
(`budget_usage_recovery.go:82-87`), without a log line; `entry.Info()` does
not follow a link. Nothing else that is not a record's name is touched.
`…RetentionRemovesOldTemporaryFilesAndNoOtherForeignName` kills `M5-7` (the
regular-file condition dropped), `M5-8` (whatever the age), `M5-9` (any
name ending in `.tmp`) and `M5-10` (any name starting with a dot);
`notes.txt` is one of the names that test wants to stay.

### F7a. The main session's question: the order is possible, the sentence is not true: `finding` (`KD2-06`, `minor`, argued below)

**Is the order possible as the code stands? Yes**, and the window is wider
than the question assumes. T is the task's goroutine, P a pass of the resend
loop, S the send goroutine. The attempt's record was created more than seven
days ago and its run has just ended.

1. T: `finishBudgetUsageAttempt` calls `attempt.seal()`
   (`budget_usage_delivery.go:266`). `seal` takes `a.mu`, builds the batch
   and writes the record, two fsyncs, with the lock held
   (`budget_usage.go:433-445`).
2. P: `handleBudgetUsage(ctx, id, true)`: `claim` succeeds, since nobody
   holds the mark (`:293`). `load` finds the attempt held (`:298`).
   `held.attempt.snapshot()` (`:301`) waits for `a.mu` when it arrives
   during the write of step 1, and returns at once when it arrives after it.
3. P: the record has batch bytes, the call is a pass, and
   `journal.outlived(record.CreatedAt)` is true (`:306`):
   `dropOutlivedBudgetUsage` writes the warning of the cap and removes entry
   and file. No request was made.
4. T: `seal` has returned the bytes; `finishBudgetUsageAttempt` starts S
   (`:272-275`).
5. S: `handleBudgetUsage(ctx, id, false)`: either `claim` fails because P
   still holds the mark, or it succeeds after P released and `load` finds
   nothing held, which for this caller means "nothing to do" (`:315`).

The batch never leaves the daemon. P can take the mark at any moment from
the start of the seal's write to S's `claim`, so the window is the two
fsyncs of the seal plus the start of a goroutine, not the gap between two
statements. (A pass that reads the record before the seal has the lock sees
no batch bytes and does nothing, F4's first row.)

Evidence: `TestBudgetUsageCheckOutlastedBatchIsDroppedUnsentWhenAPassComesFirst`
in `budget_usage_outlasted_check6kd2_test.go`, two subtests, both
deterministic. "a pass between the seal and the send" calls `seal`, one
pass, then the send. "a pass while the seal is under way" goes through
`finishBudgetUsageAttempt` itself and starts the pass from the journal's
clock, which `seal` reads with the attempt locked, and waits until the pass
has taken the mark before the seal goes on. Both end with no report at the
fake server (which would have accepted one), the one warning "older than
seven days and was not delivered", no file and no entry. Both pass on the
code as checked. That they pass is the finding: the file is a
reproduction, and three of its assertions are marked `FLIP` for the day the
order is fixed.

**Does a test pin "tried once"? In one order only.** The sentence is the
lane's own, in three places: the comment on `deliverBudgetUsage`
(`budget_usage_delivery.go:278-280`), choice 10 of its report 11.3 ("its
batch is then sent once, and the next pass drops it if that send did not
deliver it"), and the name of
`TestBudgetUsageBatchOfARunThatOutlastedTheCapIsSentOnce`. That test lets
the send run first; it kills `M5-3` (the send that follows the seal drops a
sealed record older than the cap) and `MF1-11` (a pass never caps a held
record). No test of the lane lets a pass come first.

**Severity.** Argued `minor`:

- The design is not broken in either order. Its rule is the cap (11.10: "a
  record is kept at most 7 days; once expired it is dropped with a warning;
  that execution stays unknown on the server through its hold"). Sending an
  expired batch once is something the lane added on top; where the pass
  comes first the daemon does what the design says, warning included.
  Nothing wrong is sent, nothing is sent twice, the server keeps its hold.
- It needs a record older than seven days at its seal (a run without a time
  limit that lasted that long, or a clock set forward, F7) and a pass that
  reaches the record within some milliseconds of a 30 second period.
- What is wrong is a sentence: a comment, a line of the lane's report and
  the name of a test say something the code does in one of two orders.

The other reading: section 3 of the brief calls "a test named after a rule
that passes on its mutant" `major`. The test named "…IsSentOnce" passes
while the batch can be sent zero times, but there is no mutant here: it is
the code as written. I rate it `minor`; the main session rules.

**Smallest change**, either of two:

- In the code: a field `tried bool` on `budgetUsageAttempt`, set in the
  `default:` branch of `handleBudgetUsage` before `sendBudgetUsage`, and the
  cap of a held record made `fromPass && tried && journal.outlived(...)`. A
  batch is then sent once by whoever handles it first, and capped only
  after that. Tried as an overlay (`XF7a-fix-r`, with `-race`): the three
  `FLIP` assertions of my reproduction turn over in both subtests and every
  other budget test passes. (In the plain run of the same overlay,
  `XF7a-fix`, one more test failed once: the timing-dependent subtest of
  `KD2-11`.)
- Without a code change: say in the comment, in the lane's report and in
  the test's name that the batch of such a run is sent once unless a pass
  reaches the sealed record first, and that it is then dropped like any
  expired record.

### F8. Where the process can die: `holds`

The list is table 3.3: the eight moments of the brief and five of my own
(inside the first write, the two failures of a launch-intent write, a death
of the recovering process before and after its write, a death during a
removal), each with what is on disk, what the next process does and what the
server ends with.

- **Fewer invocations than were started:** one line only, 2b, the accepted
  limit: the write of a launch intent failed, the removal of the file
  failed too, and the process died before a later write succeeded. In every
  other line the file either holds every launch intent or does not exist.
  A record without any invocation is never reported (line 1), because it
  cannot be told from 2b at the first launch.
- **Two byte strings under one key:** one line only, 7a, the collision of
  ruling 1.3 (E9): the seal's write failed, the request went out from
  memory, the process died. Where a batch is built by a later process, its
  bytes are written before they are sent (F2, lines 9 and 10), and stored
  bytes are never built again (line 6; `ME5-1`).

The tests behind the lines:

| Lines | Test | Mutants it kills |
| --- | --- | --- |
| 1 | `…RecoveryNeverReportsARecordWithoutBatchAndWithoutInvocation` | `MF1-1` |
| 2, 3, 4, 5, 10 | `…RestartReportsAnInvocationWithoutAnOutcomeAsUnknown`: the earlier process left one invocation that returned and one without an outcome; the wire is compared with the expected batch, the file holds the bytes when the request arrives, a second pass sends the same body | `MG3-1`, `MG3-2`, `MG3-3`, `MF2-1`, `MF2-3` |
| 6, 7, 8 | `…RestartResendsThePersistedBatchByteForByte`: 503 first, then delivered; the same bytes and key both times | `ME5-1` |
| 7a | `…RebuiltBatchAnsweredWithOperationConflictIsDropped` | `ME9-1`, `MG3-1` |
| 8a | `…AnsweredRecordIsNotSentAgainWhenItsFileCannotBeRemoved` | `M3-1`, `M3-2`, `M3-4` |
| 9 | `…RecoveryDoesNotSendABatchItCouldNotPersist` | `MF2-1`, `MF2-2` |
| 0, 2a, 2b, 11 | not mine: the atomic write and the launch-intent rule (B4) are part 1's; line 0's temporary file is F7 | |

A death is a process that ends. For a machine that loses power the lines
hold as far as the fsyncs do: every write of a record is fsynced, file and
directory, before the call returns (not the directory on Windows, a choice
of `writeManagementJSON` that is older than this slice); a removal is not
followed by a directory fsync, so a removed file can come back, which gives
line 8 for a sealed record and section 6 item 2 for the one of B4.

### G2. Locks: `holds`

The new code has two mutexes and one mark.

| Lock | Held by | Across |
| --- | --- | --- |
| `journal.mu` (`budget_usage.go:169`) | `begin` | the check for an existing file, the first write of the record (two fsyncs), the entry in the table; a removal after a failed write |
| | `load` | the lookup in the tables and one read of a record file (at most 4 MiB) |
| | `remove` | the two tables and one unlink |
| | `claim`, `release`, `liveIDs`, `takeRound` | memory only |
| `attempt.mu` (`budget_usage.go:323`), one per attempt | `describeLaunch`, `beginInvocation`, `endInvocation`, `seal` | one write of the record (two fsyncs) each; `beginInvocation` also the removal of B4; a warning when a write fails |
| | `snapshot`, `attemptID` | a copy in memory |
| the mark (`journal.inFlight`), not a mutex | `handleBudgetUsage` | the lookup, the request (up to 30 s) and the removal. Taken with `claim`, which never waits: a second taker returns |

**No two are held together.** `load` gives `journal.mu` back before the
caller takes `attempt.mu` in `snapshot` (`budget_usage_delivery.go:298`,
`:301`). `seal` and the other writers of an attempt call `journal.write`,
`journal.now` and `recordPath`, none of which locks
(`budget_usage.go:223-231`), and the removal of B4 is a plain `os.Remove`
under `attempt.mu`. `begin` makes the attempt without locking it. With no
pair there is no order to get wrong. What can wait on what: a pass that
holds the mark waits in `snapshot` for a `seal` in progress, and `seal`
needs no mark, so nothing waits in a circle (the second subtest of my F7a
reproduction runs exactly this).

**Existing locks the new code meets**, each alone and never under a budget
mutex:

- `managedClientRegistry.mu`: written under by `setBudgetUsageReady` at
  start (it stores one atomic flag per workspace client), read under by
  `budgetUsageTransport` for a map lookup, before the request and outside
  the journal's locks (`budget_usage_delivery.go:103-133`).
  `installManagedCredential` reads the root client's flag while it holds the
  same lock, which is what makes a workspace client made during
  `setBudgetUsageReady` get the flag either way (`MA1-3`, `MA1-4` killed).
- `Client.tokenMu`: `Token()` in `reportBudgetUsage`, a leaf.
- the log handler's own lock: warnings are written under `attempt.mu`
  (`persistLocked`, `seal`), never under `journal.mu`.

**No network under any mutex.** The request is made with only the mark
held.

**The fsyncs.** They are under `journal.mu` in `begin` and under
`attempt.mu` in the four writers. Neither lock is needed by the cancel path
or the heartbeat: the journal is referred to in `daemon.go` at the start
(`:2053`, `:2083`), around the run of a task (`:5265-5267`), at the launch
description (`:7793`), at the two launches (`:7969`, `:8025`) and at three
marks of where a result came from (`:8523`, `:8896`, `:8921`, a field
without a lock), and nowhere in `heartbeatLoop` or `watchTaskCancellation`.
All of those are on the goroutine of the task itself. What a slow disk
delays is therefore the task that writes: its start by the first write, each
launch by the write of its intent (the rule: the intent is on disk before
the provider starts), its terminal report by the seal. That is E2's note.
No other task waits, except in `begin`, which can stand behind another
task's first write, or behind a pass that reads one record file or removes
one. All of it is local disk; none of it is the network.

### G3. The acceptance rows, this half: `holds`

The table is section 4: for each of the six items the test, what it asserts
and a mutant of mine that it kills. No item is without a test that can fail.
One limit is stated under the table (every "unusable directory" case is a
failure of opening the journal at start).

### G5. Nothing weakened: `holds` (one note: `KD2-17`)

What "before" is. The lane says (its report 11.2, "Round 1 tests of mine
that changed") that it made four edits with
`/tmp/f3-6b/r2/edit_round1_tests_stage_d.py` and kept a copy of each file as
it was. I did not take the copies on trust: the round 1 final files were
rebuilt from the main session's own snapshot
(`/tmp/f3-main-s6b/snap0224/server/internal/daemon/`) and its two patches
(`test-diff-capture.patch`, `test-diff-main.patch`), and each has the sha256
of the lane's copy (`dbddc889…` capture, `34e58a97…` runtask, `1c8ae400…`
main). Round 1 had three test files. Their diffs against the tree: capture
6 lines removed and 35 added, runtask 5 and 13, main 45 and 106. Every
changed line belongs to a row of the table below or to the golden rewrite
under it.

No test went away. The 38 top-level names and 137 names of every level of
the main session's round 1 log (`/tmp/f3-main-s6b/v1/test127.txt`) all exist
in my baseline log. Along the lane's own stage logs the set only grows: 42 /
158 (`green-p1p2.txt`), 55 / 236 (stage D), 78 / 280 (stage E), 78 / 280
(stage F, the same names), 78 / 280 (my baseline, the same names). `git
status` of the worktree shows six modified tracked files, none of them a
test, so no test older than this task was edited.

| Test or helper | Before (round 1 final) | Now | Lost |
| --- | --- | --- | --- |
| helper `budgetUsageTestNewServerReplying` (`budget_usage_capture_test.go:62`) | records the request, calls the hook, hands every request to the test's `reply` | the same, but a request under `/api/daemon/budget/` is answered here with 503 `ledger_unavailable` and never reaches `reply` | no assertion (the helper makes none). A consequence to know: every `handleTask` and `runTask` test built on it sees only a kept answer, so the record stays where those tests read it. A delivered answer through `handleTask` is `…HandleTaskSendsTheSealedBatch`, on the other fake |
| `…HandleTaskRecordsAndSeals` | "Round 1 ends at the seal: nothing is sent": any request to the budget route is an error | `budgetUsageTestRequireReports`: after the sends ended, exactly one request went to the budget route, the journal holds the live record, and the one `POST` to the attempt's path carries the sealed batch | the old assertion is reversed by design. The new one is two-sided: no send fails it (`ME2-3`), two sends fail it (`MG5-1`) |
| `…RunTaskFreshSessionRetryIsTwoInvocations` | no request to the budget route, with or without the protocol | with the protocol one report per attempt (two attempts) with its sealed batch; without it still no request | nothing (`ME2-3`, `MG5-1`) |
| `…RunTaskLaunchThatCannotStart` | no request whose path ends in `/usage`; no request to the budget route | no request to `/tasks/<this task>/usage`; one report with the sealed batch | the width of the legacy check: a legacy usage report under another task's path no longer fails this test. `KD2-17`, below |
| `…RunTaskWithoutTheProtocol` | no request to the budget route in any case | a case that leaves a record wants its one report; the others still want no request | nothing (`MG3-4` is killed by the cases without a record) |
| helper `budgetUsageTestOpenJournal` | opens the journal | also `t.Cleanup(journal.background.Wait)` | no assertion. A test can no longer end while a send it started still runs; a send that never ends now holds the test until the package timeout |

`KD2-17` (`note`). The new route ends in `/usage` as the legacy one does, so
the lane narrowed the legacy check of `…RunTaskLaunchThatCannotStart` to the
task's own path. What its message says is still pinned: a run that used
nothing and is reported on its task's legacy route fails this test and no
other (`MG5-2b`). What went is the rest of the old width: the same report
under another task's path passes this test (`MG5-3d`) and is caught in the
whole suite by one test about something else
(`…NotFoundAnswerOnlyDropsTheRecord`, whose fake refuses every request that
is not about its task; `MG5-3b`). The form that loses nothing is
`budgetUsageTestRequestsTo(budgetUsageTestLegacyRequests(requests), "/usage")`:
with it the test passes on the tree (`XG5-fix`) and fails on that mutant
(`MG5-3c`). Nothing has to change for this slice.

A fifth rewrite, which the list of 11.2 does not name and the brief's does
not either: `TestBudgetUsageBatchGolden` and the first subtest of
`TestBudgetUsageBatchEnvelope` (`budget_usage_test.go`, by
`/tmp/f3-6b/r2/edit_golden.py`; the lane reports it in 11.1). Before, the
builder's bytes for one record were compared with a literal in the test.
Now two records are compared byte for byte with the shared files
`server/pkg/protocol/testdata/budget_usage_v1_batch.json` and
`…_batch_empty.json`, and the golden record itself changed (the adapter
version, which invocation is resumed and which interrupted, two sent usage
items in the first invocation where there were three). What that lost is
that the expected bytes are no longer in the test file: they are in a file
that no daemon test writes. The wire format and its fixtures are part 1's
subject, so I record the rewrite here and do not judge it. The main session
should make sure part 1's report covers it.

### H. Runs: `holds`

Every command with its toolchain, exit code and counts is in section 6
(6.1 before, 6.2 at the end).

1. `go build ./...` and `go vet ./internal/daemon/` exit 0 on go1.27.0 and
   on go1.26.9, before I added anything and again at the end. The vet at the
   end covers my four test files.
2. `go test ./internal/daemon/ -run '^TestBudgetUsage' -count=1 -v` and the
   same with `-race`. Before: 78 top-level tests, 280 of every level, on
   both toolchains, exit 0, the counts of the main session. At the end, with
   my files in the tree: 90 / 298 on both toolchains, with and without
   `-race`, exit 0, no FAIL, no SKIP, no data race reported.

The count differs by 12 top-level tests and 18 names of every level. Those
are the 12 tests and 6 subtests of my four files (section 7) and nothing
else: the names of the log before and of the log at the end were compared,
none of the 280 is missing, and the 18 new ones are all mine.

Four more things were run at the end, because my files are new:

- The agent CLI guard, once, on go1.27.0, from the root of the worktree
  with my files in the tree: exit 0, 90 / 298, no "unexpected agent CLI"
  line. (The guard run of the tree as dispatched is part 1's.)
- `gofmt -l` over my four files prints nothing.
- `go vet ./internal/daemon/` for `GOOS=windows GOARCH=amd64` and for
  `GOOS=linux GOARCH=amd64`, go1.27.0: exit 0 both, nothing printed. My
  files use no helper of the lane's `!windows` test file, and the one
  test of mine that needs permission bits skips where the lane's do
  (`budgetUsageTestRequirePOSIX`). The cross-compiles of the tree as
  dispatched are part 1's.
- `shasum -a 256 -c /tmp/f3-main-s6k/base-r2/manifest.txt` from the root of
  the worktree, at 17:30 on 2026-11-20 and again at 20:34 after the last
  run: 21 of 21 `OK` both times. No file of the tree
  as dispatched changed while I worked; `git status` shows the same six
  modified and fifteen untracked files plus my four.

No run at the end failed, so there is nothing timing-dependent to report
from them. The one timing-dependent failure met on the way is a test of the
lane, re-run alone as the brief asks; both runs are under F6 (`KD2-11`).

## 2. Findings, most severe first

No `blocker`. Three `major`, eight `minor`, one hole in the rules, six
`note`. The ids were given in the order found and were not renumbered.
"Survives" means: all 78 tests of the lane pass on the mutant (its line is
in section 5). Test names are prefixed `TestBudgetUsage`. I made none of
the changes named here; what I added is four test files (section 7).

In no finding does the code do something a rule forbids on a path that is
reached today. Three kinds: a rule that stands between the code and a
`blocker` and has no test that can fail (`KD2-12`, `KD2-01`); a test that
passes on a mutant of the rule it is named after (`KD2-02`) or pins less
than it seems to (`KD2-03`, `KD2-04`, `KD2-05`, `KD2-07`, `KD2-08`,
`KD2-09`, `KD2-11`); and one sentence of the lane that the code does not
keep in one order of events (`KD2-06`).

### `major`

**`KD2-12`. The lock that keeps a record being created from being taken for a leftover has no test.** `major`, comes with its test.

- Rule: brief F4, "An attempt of a running task whose record is removed by
  recovery is a blocker (the batch says nothing where it should say what
  was launched)"; by the scale of brief section 3, "a required case … has
  no test that can fail".
- Place: `budget_usage.go:278-312` (`begin` keeps `journal.mu` from its
  check for an existing file to the entry in the table);
  `budget_usage_delivery.go:203-213` (`load` looks at the table and reads
  the file under one hold of the same lock).
- Evidence: the code is right (F4). `MF4-3`, `begin` letting go of the lock
  while it writes the record file, survives. Under it a pass that lists the
  directory during the write removes the file as a record that "names no
  provider launch"; when the entry is made between the pass's `load` and
  its `remove` the entry goes too, and nothing is ever reported for a run
  that launched. The lane's report says that no test pins the lock (11.3,
  choice 17). My `…RecordBeingCreatedIsNeverTakenForALeftover` passes 50 of
  50 on the tree and fails 50 of 50 on the mutant (`begin-lock-alone-50`,
  `begin-lock-mf43-50`; `MF4-3-k`, `MF4-3-kr`).
- Smallest change: move `budget_usage_begin_lock_check6kd2_test.go` into
  the lane's files. It is a stress test, because `begin` has no seam
  between the write and the entry (`write` calls `writeManagementJSON`
  directly, `budget_usage.go:229-231`). A deterministic test needs such a
  seam, which is production code and the lane's to weigh.

**`KD2-01`. The 30 second bound of one report has no test.** `major`, comes with its test (the brief asks for both).

- Rule: brief section 6, item 17 and the paragraph "The 30 second bound has
  no test, and the main session rules that it needs one". What the bound
  stands for is the first clause of `blocker`: budget code may not delay
  without bound. The resend loop is one goroutine, so one report that never
  ends stops it for good.
- Place: `budget_usage_delivery.go:152` (`httpClient := *c.client`, the
  copy that keeps the `Timeout`); `client.go:136`
  (`Timeout: 30 * time.Second`).
- Evidence: `M9-a` (the per-call client built as
  `http.Client{Transport: c.client.Transport}`, the mutant the brief names)
  survives; so does `M8-4` (the 15 s bound of stage F put back: no test
  tells 15 s from 30 s or from none). `M9-b` (the send under
  `context.Background()`, the brief's second mutant) is killed by the
  lane's `…SendEndsWithTheDaemonAndKeepsTheRecord`. My
  `…ReportGivesUpByTheClientsOwnBound` fails on `M9-a` (`M9-a-k`) and on a
  second form, the copy giving up its timeout (`M9-d-k`); my
  `…ReportBoundIsThirtySeconds` fails on `M9-c` (300 s on the client), which
  no test of the lane notices. The bound could be shortened from the test
  without touching production code (`d.client.client.Timeout`).
- Smallest change: move `budget_usage_bound_check6kd2_test.go` into the
  lane's files, as the brief plans.

**`KD2-02`. The test named after the server rule passes on two mutants of it.** `major` by the letter of the scale; the exposure is small.

- Rule: design 11.10, "a record whose address differs from the current one
  is not sent and stays until the 7-day cap"; accepted item 22, the check
  "is an exact string comparison"; brief 5.1 item 4 names both mutants;
  brief section 3, "a test named after a rule that passes on its mutant".
- Place: `budget_usage_recovery.go:163`
  (`record.ServerURL != journal.serverURL`); the lane's
  `…RestartKeepsARecordOfAnotherServer`.
- Evidence: `ME8-3` (`strings.EqualFold`) and `ME8-4` (a trailing slash
  ignored on both sides) survive. The lane's test has one other address,
  which differs in the host name, so it cannot tell "exact" from "the same
  after some folding". What limits the exposure: both strings come out of
  `NormalizeServerBaseURL`, which already cuts trailing slashes and
  lower-cases the scheme. The difference `ME8-4` forgives therefore cannot
  come from the configuration, and what `ME8-3` forgives is the case of the
  host, which is the same server, or the case of a path, which need not be.
  My
  `…RestartKeepsARecordWhoseServerAddressIsSpelledDifferently` (seven
  spellings, a sealed and a running record under each) kills both
  (`ME8-3-k`, `ME8-4-k`).
- Smallest change: add the near spellings to the lane's test, or take mine.

### `minor`

**`KD2-06`. A batch of a run that outlasted the cap is dropped unsent when a pass comes first (the answer to F7a).** `minor`, argued; the other reading is `major`, and the main session rules.

- Rule: the lane's own sentence, in three places: the comment on
  `deliverBudgetUsage` ("A batch is tried once whatever the age of its
  record"), choice 10 of its report 11.3 ("its batch is then sent once, and
  the next pass drops it if that send did not deliver it") and the name of
  `…BatchOfARunThatOutlastedTheCapIsSentOnce`. The design does not ask for
  it (11.10: a record is kept at most 7 days and then dropped with a
  warning).
- Place: `budget_usage_delivery.go:306` (a pass caps a sealed record this
  process holds), `:272-275` (the send is started after the seal has
  returned), `:278-280` (the comment).
- Evidence: the order is possible, written out step by step under F7a: a
  pass can take the mark at any moment from the start of the seal's write
  to the `claim` of the send goroutine, finds the record sealed and older
  than the cap, and drops it; the send then finds nothing. My
  `…CheckOutlastedBatchIsDroppedUnsentWhenAPassComesFirst` reproduces it in
  two deterministic subtests on the code as checked: no report, one warning
  of the cap, no file. The lane's test tries only the order in which the
  send comes first.
- Why `minor`: nothing wrong is sent, nothing is sent twice, the server
  keeps its hold, and the daemon does what the design says of an expired
  record. It needs a record older than seven days at its seal and a pass
  inside a window of some milliseconds. What is wrong is a sentence. The
  other reading: a test whose name says "sent once" passes while the batch
  can be sent zero times.
- Smallest change, either of two: a field `tried bool` on the attempt, set
  before `sendBudgetUsage` in the `default:` branch, and the cap of a held
  record made `fromPass && tried && journal.outlived(...)` (six lines; with
  it my reproduction turns over and all 78 tests of the lane pass under
  `-race`, `XF7a-fix-r`); or the comment, the lane's report and the test's
  name say "sent once unless a pass reaches the sealed record first".

**`KD2-03`. The class of an answer is pinned for the statuses the test lists and for nothing between them.** `minor`; by the letter of brief 5.1 item 1 it could read `major`.

- Rule: design 11.10 as completed on 2026-11-18: any 2xx is delivered;
  400, 404, 409, 413, 422 are final; everything else is kept. Brief 5.1
  item 1: one mutant per boundary.
- Place: `budget_usage_delivery.go:64-77` (`verdict`); the lane's
  `…AnswerDecidesWhatBecomesOfTheRecord` (38 cases).
- Evidence: every boundary the brief names is killed, and two more
  (thirteen mutants, table 3.4). Three I added survive: `ME3-5` (delivered from 100 on),
  `ME3-14` (a 412 is final), `ME3-15` (a 306 is delivered). The first is
  reachable: Go's client hands a `101 Switching Protocols` to the caller as
  a final answer, so under `ME3-5` a 101 from a proxy removes the record as
  delivered. My `…VerdictOfEveryStatus` checks `verdict()` against the rule
  for every status from 0 to 999, with and without an error, and kills all
  three; `…SwitchingProtocolsAnswerKeepsTheRecord` sends a real 101.
- Why `minor`: the code is right and the named boundaries are pinned; what
  is open is the statuses nobody listed.
- Smallest change: take the exhaustive test (it is one function of the
  status).

**`KD2-11`. A test of the lane fails without a defect, 13 times in 200.** `minor`.

- Rule: a test that fails on correct code; brief section 7 asks for both
  runs of a timing-dependent failure.
- Place: `budget_usage_recovery_test.go:1318-1331`, the subtest "stops with
  the daemon" of `…LoopPassesAtStartAndThenAtEveryTick`; the assertion is
  line 1326.
- Evidence: it failed once in a run of all budget tests. Alone, 200 times
  on the unchanged tree with the machine under load: 13 fail, all at that
  line (`loop-alone-200`). The test counts reports where the fake server
  registers them, which is after it has read the body; a request that the
  daemon gave up at the stop can be registered after the count was read.
  With the requests counted where the daemon starts them, 200 of 200 pass
  (`XLOOP-fix`, `loop-overlay-200`), and in 11 of those runs the fake
  registered one more report after the loop had ended while the daemon had
  started none.
- Smallest change: count at the daemon, with a counting `RoundTripper` on
  the test daemon's client; the changed subtest is
  `/tmp/f3-s6kd2/m/XLOOP-fix/diff.txt`.

**`KD2-05`. Nothing guards the daemon's other requests against a change of their redirect behaviour.** `minor`.

- Rule: brief E4, "and that the daemon's other requests did not change
  their behavior".
- Place: `budget_usage_delivery.go:152-153` (the copy of the client and the
  `CheckRedirect` set on the copy).
- Evidence: by the code they did not change (E4). `ME4-2` (the line sets
  `CheckRedirect` on the shared client) survives a plain run; with `-race`
  it fails 18 tests through the race detector and not through anything
  about redirects (`ME4-2r`). `ME4-3` (the same switch made once, where the
  protocol is declared) survives with and without `-race`. Under either,
  every poll, heartbeat and task report of an ordinary installation behind
  a redirecting front stops following. My
  `…ReportLeavesTheDaemonsOtherRequestsFollowingRedirects` kills both
  (`ME4-2-k`, `ME4-3-k`).
- Smallest change: take the test.

**`KD2-04`. The rest of a long answer.** `minor`.

- Rule: accepted item 18, "An answer that cannot be read to its end keeps
  the record whatever its status line said."
- Place: `budget_usage_delivery.go:160-162` (`io.Copy(io.Discard, resp.Body)`).
- Evidence: `ME7-6` (the line removed) survives: the lane's two "ends
  early" cases use an answer of 20 bytes, which breaks off inside the 64
  KiB that are kept, where `io.ReadAll` already reports it. Under the
  mutant an answer longer than 64 KiB that breaks off later is judged by
  its status line: a 200 removes the record as delivered, a 400 as refused.
  My `…LongAnswerThatEndsEarlyKeepsTheRecord` (subtests `200` and `400`)
  kills it (`ME7-6-k`).
- Smallest change: take the test, or add a long answer to the lane's cases.

**`KD2-07`. One pass may handle a record twice once the sort is gone.** `minor`.

- Rule: brief F6 (what a server that answers 503 forever gets per record);
  the lane's choice 8, one send per pass that reaches a kept record.
- Place: `budget_usage_recovery.go:89-90` (`slices.Sort(ids)` before
  `slices.Compact(ids)`).
- Evidence: `MF-sort` (the sort removed) survives. The list is the ids this
  process holds followed by the names of the directory, and `Compact`
  removes only neighbours, so a record that is held and has a file is in
  the list twice: two requests per pass for a kept batch. The lane's reason
  for leaving it unpinned ("a directory listing is sorted already") misses
  the ids in front of the listing. My
  `…PassHandlesARecordThisProcessHoldsOnce` kills it (`MF-sort-k`: "2
  reports … want 1: one per pass").
- Smallest change: take the test.

**`KD2-08`. The interval of the resend loop has no pin.** `minor`.

- Rule: brief F6, "An unbounded or tight loop is a finding"; the 30
  seconds are the lane's choice 8, on which accepted item 17 rests ("up to
  32 times 30 seconds").
- Place: `budget_usage_recovery.go:20-22` (`budgetUsageResendInterval`).
- Evidence: `MF6-7` (30 ms) survives: the tests hand the loop an interval
  of their own and the syntax-tree test pins that `Run` passes the
  constant, not what the constant is. The retention and the 32 have such a
  pin (`budget_usage_recovery_test.go:660`, `:1143`). Under the mutant a
  server that is down gets up to 120,000 requests per hour and record. My
  `…ResendIntervalIsThirtySeconds` kills it (`MF6-7-k`).
- Smallest change: take the test (three lines).

**`KD2-09`. A settled attempt is never forgotten once the line goes.** `minor`.

- Rule: brief 5.1 item 3, "a removal that succeeds later leaves the entry
  behind". Accepted item 21 says what the settled set is for and does not
  say when an entry leaves it.
- Place: `budget_usage_delivery.go:226` (`delete(j.settled, attemptID)`).
- Evidence: `M3-3` (the line removed) survives. The effect is memory only:
  one entry per such attempt for the life of the process; a held entry is
  looked at before a settled one and an attempt id is issued once, so no
  batch is lost or sent again. My
  `…SettledAttemptIsForgottenOnceItsFileIsRemoved` kills it (`M3-3-k`).
- Smallest change: take the test.

### A hole in the rules

**`KD2-10`. A record names its server and its workspace, not its account.** For the main session to rule; I lean `minor`.

- Rule: design 11.10 decides the server ("a record goes back only to the
  server that created it") and, for a managed installation, the workspace
  of the credential. It says nothing about the account in an installation
  that is not managed.
- Place: the fields of the record (`ServerURL`, `WorkspaceID`;
  `budget_usage.go:291-303`); `budgetUsageTransport`, which in an ordinary
  installation returns the daemon's own client with whatever token the
  profile holds now.
- What follows: after another account signs in on the same profile against
  the same server, the records the former account left are sent with the
  new account's token. By design 11.3 a caller without access to the
  attempt's workspace gets the 404 of an attempt that does not exist; the
  daemon takes a 404 for a final refusal, warns once and removes the
  record. No usage is recorded under a wrong account and nothing reaches
  another server; the batch of the former account is lost although that
  account could have delivered it. Not the `blocker` clause ("goes with
  another workspace's credential" is the managed case, which the code
  handles and `ME8-6` to `ME8-9` pin). The cancellation receipts of the
  same daemon are scoped by user as well (`cancel_receipts.go:27-32`).
- Smallest change, if the ruling is that such a batch must not be lost: the
  record stores the user the profile was signed in as, and a pass keeps a
  record of another user as it keeps one of another server. If the ruling
  is that the loss is acceptable: one sentence in design 11.10.

### `note`

Nothing to change in this slice; each is in the text of its point.

- **`KD2-13`** (F1). The design does not order "a record of another server
  is kept" against "a record without batch bytes and without an invocation
  is removed". The code keeps (the server test comes first,
  `budget_usage_recovery.go:163` before `:172`), the lane states the
  choice (11.3, choice 11) and its test pins it (`MF1-4` is killed).
  Nothing is reported in either order.
- **`KD2-14`** (F1). A record file without `created_at` is dropped at the
  first pass with the warning of the cap, because the zero time is older
  than seven days; every other file that is not a record of this version
  is kept until its last change is seven days old. The daemon always
  writes the field.
- **`KD2-15`** (F6). No backoff, no jitter, and a `Retry-After` is not
  read: every daemon of a deployment sends at the same rate to a server
  that is down, 120 requests per hour and kept record, at most 3,840 per
  hour and daemon. The design asks for nothing else and the lane states it
  (choice 8).
- **`KD2-16`** (F7). The age is a difference of wall-clock times. A clock
  set forward by more than seven days drops, at the next pass, every sealed
  record that is still waiting and everything an earlier process left,
  unsent and with the warning of the cap. The record of a running attempt
  is not touched.
- **`KD2-17`** (G5). The one thing a round 1 test lost in round 2:
  `…RunTaskLaunchThatCannotStart` no longer fails on a legacy usage report
  under another task's path (`budget_usage_runtask_unix_test.go:641`;
  `MG5-3d` passes it). The form that loses nothing is
  `budgetUsageTestRequestsTo(budgetUsageTestLegacyRequests(requests), "/usage")`
  (`XG5-fix` passes, `MG5-3c` fails).
- **`KD2-18`** (3.4, line 8). The third removal of stage F is not
  behaviour-preserving to the letter: the Debug line of a kept report now
  always carries `status` and `error` (`status=0` when no answer arrived,
  `error=<nil>` when one did; `budget_usage_delivery.go:347-348`). No test
  reads the attributes of that line, before or after (`M8-3` survives).

Remarks without an id, each said where it belongs: the body of an answer is
closed by a line no test can see, and it should stay (`MF-close`, 3.5);
`journal.background` is a counter that only tests wait on (3.5); the
comment on `load` credits its hold of the lock with what `begin`'s hold and
the wait in `claim` already give (F4); the fake server of the collision
test answers 409 to anything, so the collision itself is the server half's
to show (E9); the golden test of the batch was rewritten against the shared
fixture files in round 2, which is part 1's subject (G5); an overlay of
`daemon.go` does not reach the wiring test (the head of section 5).

## 3. Tables

### 3.1 The class of an answer, one line per status (E3)

From `verdict()` (`budget_usage_delivery.go:64-77`) and the returns of
`reportBudgetUsage` that carry an error (`:143-145` a request that cannot
be built, `:155-157` no answer, `:163-167` an answer that cannot be read).
"Lane"
is a case of `TestBudgetUsageAnswerDecidesWhatBecomesOfTheRecord`, which
goes over the wire and looks at the record file, the warning and what a
second send carries. "Mine" is `TestBudgetUsageVerdictOfEveryStatus`, which
asks `verdict()` for every status from 0 to 999, with and without an error.

| What came back | Design 11.10 | The code | Pinned by a lane case | Mutant, and what it moved |
| --- | --- | --- | --- | --- |
| no answer: an error from `Do` (no connection, the 30 s bound, the context ended) | kept | kept | "the connection is closed without an answer"; `…UnreachableServerKeepsTheRecord`; `…SendEndsWithTheDaemonAndKeepsTheRecord` | `ME3-10` killed (a failed read judged by its status line) |
| any status, the answer not readable to its end | kept | kept, whatever the status line said | "200 with a body that ends early", "400 with a body that ends early" | `ME3-10` killed. A long answer that ends early: only mine (`ME7-6`, `KD2-04`) |
| 000 to 099 (Go's parser takes such a status line) | not named: "everything else" | kept | none | none of mine moves these alone; mine pins them |
| 100, 102 to 199 | never reach the caller (Go's client consumes them) | kept, were one to arrive | none | none |
| 101 | "everything else" | kept | none | `ME3-5` survives the lane's tests (delivered from 100 on); killed by mine and by `…SwitchingProtocolsAnswerKeepsTheRecord`, which sends a 101 over the wire. `KD2-03` |
| 200 | delivered | delivered | "200 recorded", "200 not recorded", "200 with an empty body", "200 with a body that is not JSON" | `ME3-6` killed (delivered from 201 on) |
| 201, 202, 204 | delivered | delivered | "201", "202", "204" | no mutant of mine moves one of these alone |
| 203, 205 to 298 | delivered | delivered | none | none; mine pins them |
| 299 | delivered | delivered | "299" | `ME3-3` killed (delivered up to 298) |
| 300 | kept | kept | "300 with a body" | `ME3-4` killed (delivered up to 300) |
| 301, 302, 303, 307, 308 | kept; a redirect is not followed | kept; the request goes to the first address only | "301 redirect" … "308 redirect" | `ME4-1` killed (redirects followed) |
| 304 | kept | kept | "304" | none |
| 305, 306, 309 to 399 | kept | kept | none | `ME3-15` survives the lane's tests (306 delivered); killed by mine. `KD2-03` |
| 400 | refused for good | refused: record removed, one warning | "400 invalid_request" | `ME3-13` killed (400 kept) |
| 401 | kept | kept | "401" | `ME3-1` killed (401 final) |
| 402 | kept | kept | "402" | none |
| 403 | kept | kept | "403" | `ME3-12` killed (403 final) |
| 404 | refused for good | refused; nothing else of the daemon is touched (E6) | "404 not_found"; `…NotFoundAnswerOnlyDropsTheRecord` | `ME3-2` killed (404 kept); `ME6-1` killed (a 404 drops the runtimes) |
| 405, 408, 410, 418, 425 | kept | kept | "405", "408", "410", "418", "425" | none |
| 409 | refused for good, whatever its code (E9) | refused | "409 operation_conflict", "409 batch_conflict", "409 protocol_mismatch"; `…RebuiltBatchAnsweredWithOperationConflictIsDropped` | `ME9-1` killed (409 kept) |
| 412 | kept | kept | none | `ME3-14` survives the lane's tests (412 final); killed by mine. `KD2-03` |
| 413 | refused for good | refused | "413 request_too_large" | `ME3-8` killed (413 kept) |
| 422 | refused for good | refused | "422 invalid_batch" | `ME3-7` killed (422 kept) |
| 429 | kept | kept; `Retry-After` is not read (`KD2-15`) | "429" | `ME3-11` killed (429 final) |
| every other 4xx (406, 407, 411, 414 to 417, 419 to 421, 423, 424, 426 to 428, 430 to 499) | kept | kept | none | none; mine pins them |
| 500, 502, 503, 504 | kept | kept | "500", "502", "503 ledger_unavailable", "504" | `ME3-9` killed (503 final) |
| 501, 505 to 599 | kept | kept | none | none; mine pins them |
| 600 to 999 | not named | kept | none | none; mine pins them |

Read from the table: the code has the classes of the design for every
status. The lane's test pins 38 answers one by one; a status it does not
list can change class without a red test (three survivors: `ME3-5`,
`ME3-14`, `ME3-15`). With my test in the tree no status from 0 to 999 can.

### 3.2 What a pass does with one record (F1)

A pass lists the ids this process holds and the names
`<canonical uuid>.json` of the directory, sorted and without duplicates
(`budgetUsageRecordIDs`), takes at most 32 of them after the cursor
(`takeRound`), and for each calls `handleBudgetUsage(ctx, id, true)`. The
lines are in the order in which the code decides; the first that applies
ends it. "Held" means an entry in `journal.attempts`.

| # | What is found | What the code does | Where | Test; mutants it kills |
| --- | --- | --- | --- | --- |
| 1 | a name that is not `<canonical lower-case uuid>.json` | not listed: not read, not reported, no log line | `budget_usage_recovery.go:76-84` | `…RecoveryActsOnlyOnCanonicalRecordNames`; `MF1-7`, `M5-10` |
| 2 | of those, a regular file `.`…`.tmp` last changed more than 7 days ago | removed in the listing of every pass (before the cap of 32 applies), no log line | `budget_usage_recovery.go:82-87` | `…RetentionRemovesOldTemporaryFilesAndNoOtherForeignName`; `M5-7`, `M5-8`, `M5-9`, `M5-10` |
| 3 | a record the other sender is handling (marked) | nothing in this pass | `budget_usage_delivery.go:293` | `…PassDoesNotSendARecordThatIsBeingSent`, `…RecordIsNeverSentTwiceAtOnce`; `M2-1`, `M2-3` |
| 4 | held, no batch bytes (a running attempt) | not touched, at any age | `budget_usage_delivery.go:303` | `…RecoveryLeavesARunningAttemptAlone`; `M5-4a`, `M5-4b` |
| 5 | held, batch bytes, `created_at` older than 7 days | one warning, entry and file removed, not sent | `budget_usage_delivery.go:306-307` | `…RecordOlderThanSevenDaysIsDroppedWithAWarning` (case "sealed by this process and not delivered"), the last step of `…BatchOfARunThatOutlastedTheCapIsSentOnce`; `MF1-11`. See F7a |
| 6 | held, batch bytes, otherwise | sent from memory: no file is read and no server address compared, so also when the file is gone or the directory cannot be listed | `budget_usage_delivery.go:308-309` | `…KeptBatchOfThisProcessIsResentFromMemory`; it fails on `ME3-6`, `ME3-9`, `ME1-2`, `ME1-3`, `M2-2`, `M5-1`, `MF4-1` |
| 7 | settled (answered, and its file could not be removed) | the removal tried again, quietly; nothing sent | `budget_usage_delivery.go:311-314` | `…PassLeavesAnAnsweredRecordAloneWhenItsFileCannotBeRemoved`, `…AnsweredRecordIsNotSentAgainWhenItsFileCannotBeRemoved`; `M3-1`, `M3-2`, `M3-2b`, `M3-4` (`M3-3` survives: `KD2-09`) |
| 8 | not held; the file is gone or cannot be read as a record | one Debug line, kept. A regular file last changed more than 7 days ago: one warning, removed. A directory or a link of that name is never removed | `budget_usage_delivery.go:318-320`, `budget_usage_recovery.go:142-151` | `…UnreadableRecordIsSkippedUntilTheCapAndThenDropped`; `MF1-8`, `MF1-9`, `M5-5`, `M5-6`. `…PassOverARecordThatIsGoneDoesNothing`; `MF1-12` |
| 9 | not held; readable; `created_at` older than 7 days | one warning, removed, never sent, whatever else it holds | `budget_usage_recovery.go:156-159` | `…RecordOlderThanSevenDaysIsDroppedWithAWarning`; `M5-2`, `M5-2c`, `MF1-10`, `MF7-1`, `MF6-8`, `ME8-5b` |
| 10 | not held; of another server address | kept exactly as it is, one Debug line; nothing built, nothing written | `budget_usage_recovery.go:163-166` | `…RestartKeepsARecordOfAnotherServer`; `ME8-1`, `ME8-2`, `MF1-4` (`ME8-3`, `ME8-4` survive: `KD2-02`) |
| 11 | not held; batch bytes (any creator) | sent as they are, an empty batch included | `budget_usage_recovery.go:167-171` | `…RestartResendsThePersistedBatchByteForByte` (both cases); `ME5-1` |
| 12 | not held; no batch bytes; no invocation (any creator) | one warning, removed, never reported | `budget_usage_recovery.go:172-179` | `…RecoveryNeverReportsARecordWithoutBatchAndWithoutInvocation`; `MF1-1` |
| 13 | not held; no batch bytes; an invocation; created by this process | one warning, removed, never reported | `budget_usage_recovery.go:180-187` | `…RecoveryDropsARecordOfThisProcessThatNoRunningAttemptHolds`; `MF1-2` |
| 14 | not held; no batch bytes; an invocation; created by an earlier process | the batch is built under this process's start identity (an invocation without an outcome becomes `unknown`), the record is written with the bytes, then sent. A failed build or write: one Debug line, the record as it was, nothing sent | `budget_usage_recovery.go:188-205` | `…RestartReportsAnInvocationWithoutAnOutcomeAsUnknown`, `…RecoveryDoesNotSendABatchItCouldNotPersist`; `MF1-3`, `MG3-1`, `MG3-2`, `MG3-3`, `MF2-1`, `MF2-2`, `MF2-3` |

The send that follows the seal (`handleBudgetUsage(ctx, id, false)`) knows
lines 3, 4, 6 and 7 only. Line 5 does not apply to it (the age is not looked
at: `M5-3`, a sealed held record dropped by that send, is killed by
`…BatchOfARunThatOutlastedTheCapIsSentOnce`), and for a record it does not
hold it does nothing (`budget_usage_delivery.go:315`;
`…SendAfterTheSealHandlesOnlyWhatThisProcessHolds` kills `MF1-5`).

### 3.3 Where the process can die (F8)

"The next process" is the next daemon of the same profile and server, at its
pass at start. "Hold" is the server's own state for an attempt it has no
batch for; what it does with it is the server half's. Lines 1 to 8 are the
brief's, 0 and 9 to 11 are mine.

| # | The process dies | On disk | The next process | The server ends with |
| --- | --- | --- | --- | --- |
| 0 | inside the first write of `begin` | nothing, or a temporary file `.installation-*.tmp`; after the rename, as line 1 | nothing; the temporary file is removed by a pass once it is seven days old (line 2 of table 3.2) | no batch; its hold. Nothing was launched |
| 1 | after the record is created | the record: no invocation, no batch bytes | one warning, the file removed, nothing reported (line 12 of 3.2, ruling 1.2 item 3) | no batch; its hold. Nothing was launched, and a record without an invocation is never reported as "launched nothing" because it cannot be told from the double failure of line 2 |
| 2 | after a launch intent is written, before the provider starts | every invocation so far, the last without an outcome | builds the batch under its own start identity, the last invocation as `unknown` (observed at its launch time, no usage), writes it into the record, sends it (line 14) | one batch naming every launch, the last `unknown` |
| 2a | the same, the write of that intent had failed | nothing: the file was removed (B4) | nothing | no batch; its hold |
| 2b | the same, the write and the removal had both failed | the record with the launches before this one | reports those | a batch that lacks the last launch: the accepted limit (section 6 item 1) and no other line |
| 3 | while the provider runs | as line 2 | as line 2 | as line 2 |
| 4 | after the provider returned, before the outcome is written (or that write failed) | as line 2 | as line 2: the invocation is `unknown`, its usage is not reported | every launch named; this one `unknown` |
| 5 | after the outcome | every invocation with its outcome, no batch bytes | builds the batch with every invocation as it ended, under its own start identity, writes it, sends it | one complete batch |
| 6 | after the seal | the batch bytes and the key | sends the stored bytes under the stored key, nothing built (line 11; B33) | one complete batch, the dead process's |
| 6a | the same, the seal's write had failed and nothing was sent yet | as line 5 | as line 5 | one complete batch, the next process's; the dead one's bytes never left it |
| 7 | after the request was sent, before the answer | as line 6 | sends the same bytes under the same key | the batch once: the server records it now, or answers with what it stored when the first copy arrived |
| 7a | the same, the seal's write had failed (the request went out from memory) | as line 5 | builds other bytes under the same key and sends them | if the first copy arrived: it keeps the first, answers 409 `operation_conflict`, the daemon drops the record with one warning. The one accepted case of two byte strings under one key (E9, ruling 1.3). If it did not arrive: the second, once |
| 8 | after the answer, before the removal | as line 6 | sends once more: a delivered batch is answered from the store and removed; a refused one is refused again, with one more warning, and removed | unchanged |
| 8a | the removal failed (the attempt is settled) and the process ends later | as line 6 | as line 8 (accepted item 21) | unchanged |
| 9 | the next process, between building a batch and writing it | as line 2 or 5 | a third process builds again; the bytes of the second never left it (F2) | one batch |
| 10 | the next process, after that write | the rebuilt bytes | a third process sends exactly those (line 11) | one batch |
| 11 | any process, while it drops or removes a record | the file, or nothing | sealed: as line 8. Unsealed leftover: removed again with its warning | unchanged |

No line ends with a batch that names fewer invocations than were started,
except 2b, and none with two byte strings under one key, except 7a. Every
launch intent is written and fsynced before its provider process starts
(`executeAndDrainRecorded`, `budget_usage_capture.go:105-120`), which is
what lines 2 to 4 rest on; whether every launch goes through that function
is part 1's point.

### 3.4 The round 2 list of brief 5.1, line by line

Every mutant is one change through the overlay; the text replaced is in
`/tmp/f3-s6kd2/m/<id>/diff.txt` and the full list of tests each fails is in
section 5. Test names are prefixed `TestBudgetUsage`. "Lane" in the last
column means: killed by a test of the lane.

| Line of 5.1 | Mutant | What it changes | Result | Verdict |
| --- | --- | --- | --- | --- |
| 1. class: a kept answer treated as final | `ME3-1`, `ME3-12`, `ME3-11`, `ME3-9` | 401, 403, 429, 503 added to the final list | each killed by `…AnswerDecidesWhatBecomesOfTheRecord` (`ME3-9` by 19 more) | lane |
| 1. the same, a status the test does not list | `ME3-14` | 412 added to the final list | survives; killed by my `…VerdictOfEveryStatus` | `KD2-03` |
| 1. class: a final answer kept | `ME3-13`, `ME3-2`, `ME9-1`, `ME3-8`, `ME3-7` | 400, 404, 409, 413, 422 taken off the final list | each killed by `…AnswerDecidesWhatBecomesOfTheRecord` (`ME3-2` also by the test of E6, `ME9-1` by the test of E9, `ME3-13` by three more) | lane |
| 1. class: a success treated as kept | `ME3-6`, `ME3-3` | delivered from 201 on; delivered up to 298 | killed: `ME3-6` by 21 tests, `ME3-3` by `…AnswerDecidesWhatBecomesOfTheRecord` | lane |
| 1. class: a kept answer treated as delivered | `ME3-4` | delivered up to 300 | killed by `…AnswerDecidesWhatBecomesOfTheRecord` | lane |
| 1. the same, below 200 and inside 3xx | `ME3-5`, `ME3-15` | delivered from 100 on; 306 delivered | both survive; killed by my `…VerdictOfEveryStatus` (`ME3-5` also by `…SwitchingProtocolsAnswerKeepsTheRecord`) | `KD2-03` |
| 1. an answer that cannot be read | `ME3-10` | judged by its status line | killed by `…AnswerDecidesWhatBecomesOfTheRecord` | lane |
| 2. `claim` always answers yes | `M2-1` | the busy test of `claim` removed | killed by `…PassDoesNotSendARecordThatIsBeingSent` and `…RecordIsNeverSentTwiceAtOnce`. **Two requests do go out**: both fail with "2 usage reports while the first send is under way, want 1" | lane |
| 2. the release dropped | `M2-2` | `defer journal.release(attemptID)` removed | killed by 17 tests; the first message is "1 usage reports after the second pass, want 2": this process never handles the record again | lane |
| 2. the release made at once | `M2-3` | `defer` removed from the release | killed by the two tests of `M2-1`, with the same message | lane |
| 3. a removal that fails does not remember | `M3-1` | `j.settled[attemptID] = struct{}{}` removed | killed by `…AnsweredRecordIsNotSentAgainWhenItsFileCannotBeRemoved` and `…PassLeavesAnAnsweredRecordAloneWhenItsFileCannotBeRemoved` | lane |
| 3. the `settled` case dropped | `M3-2`, `M3-2b`, `M3-4` | the case removed; the case does nothing; the set never consulted | each killed by the same two tests | lane |
| 3. a later successful removal leaves the entry behind | `M3-3` | `delete(j.settled, attemptID)` removed | survives; killed by my `…SettledAttemptIsForgottenOnceItsFileIsRemoved` | `KD2-09` |
| 4. recovery sends a record of another server | `ME8-1` | the comparison removed | killed by `…RestartKeepsARecordOfAnotherServer` and `…RecordOlderThanSevenDaysIsDroppedWithAWarning` | lane |
| 4. the comparison after the batch was built and written | `ME8-2` | the comparison moved behind the build and the write | killed by `…RestartKeepsARecordOfAnotherServer` (the record of the other server changed on disk) | lane |
| 4. the comparison ignores case, or a trailing slash | `ME8-3`, `ME8-4` | `strings.EqualFold`; `strings.TrimRight(…, "/")` on both sides | both survive; killed by my `…RestartKeepsARecordWhoseServerAddressIsSpelledDifferently` | `KD2-02` |
| 4. retention test and server test swapped | `ME8-5b` (`ME8-5` was a spec error) | the server test first | killed by `…RecordOlderThanSevenDaysIsDroppedWithAWarning` | lane |
| 5. the age test inverted | `M5-1` | `>` became `<` | killed by 18 tests | lane |
| 5. its boundary moved | `M5-2`, `M5-2c` | `>=` (exactly seven days is dropped); seven days and one second is kept | each killed by `…RecordOlderThanSevenDaysIsDroppedWithAWarning`, `…RetentionRemovesOldTemporaryFilesAndNoOtherForeignName`, `…UnreadableRecordIsSkippedUntilTheCapAndThenDropped` | lane |
| 5. a sealed held record dropped by the send after its seal | `M5-3` | `fromPass &&` removed from the cap of a held record | killed by `…BatchOfARunThatOutlastedTheCapIsSentOnce` and `…RecoveryLeavesARunningAttemptAlone` | lane (and see F7a for the order this test does not try) |
| 5. a held unsealed record dropped by a pass | `M5-4a` | the "no batch yet" case no longer covers a record past the cap | killed by `…RecoveryLeavesARunningAttemptAlone` | lane |
| 5. a held unsealed record sent by a pass | `M5-4b` | the "no batch yet" case never taken | killed by `…RecoveryLeavesARunningAttemptAlone` | lane |
| 5. an unreadable file removed before seven days | `M5-5` | the age condition of that removal removed | killed by `…UnreadableRecordIsSkippedUntilTheCapAndThenDropped` | lane |
| 5. the regular-file condition dropped | `M5-6` | `!info.Mode().IsRegular()` removed | killed by the same test | lane |
| 5. (added) old temporary files | `M5-7`, `M5-8`, `M5-9`, `M5-10` | regular-file condition removed; removed whatever their age; any name ending in `.tmp`; any name starting with a dot | each killed by `…RetentionRemovesOldTemporaryFilesAndNoOtherForeignName` | lane |
| 6. the cap raised by one | `M6-1`, `M6-1b` | the constant 33; `limit+1` where the round is cut | killed by `…PassHandlesAtMostThirtyTwoRecordsAndTakesUpWhereItStopped` | lane |
| 6. the cursor never advanced | `M6-2` | `j.cursor = …` removed | killed by the same test | lane |
| 6. the cursor advanced past records not tried | `M6-3` | the cursor set one record further | killed by the same test | lane |
| 6. (added) a pass starts again at the last record of the one before | `M6-4` | `if found` never true | killed by the same test | lane |
| 7. the loop before the journal | `MA5-1w` | the `go` statement of the loop moved above the call of `initializeBudgetUsage` | killed by `…RunOpensTheJournalBeforeThePreflightAndStartsTheResendLoop` | lane |
| 7. the loop after the poll loop | `MA5-2w` | the `go` statement of the loop moved below the call of `pollLoop` | killed by the same test | lane |
| 7. the call moved into a branch not taken | `MA5-3w` (loop), `MA5-7w` (journal) | `if false { … }` around the call | killed by the same test: **yes, it also fails then** | lane |
| 8. removal 1: the empty-version operand | `M8-1` (the old code) | `version == "" \|\|` back in `budgetUsageAdapterVersion` | survives, and should: an empty version returns `""` either way. What the operand was there for is pinned: `M8-1p` (an empty version gives `"0"`) is killed by `…AdapterVersionNeverCostsTheBatch` and `…RunTaskNamesTheAdapter` | holds |
| 8. removal 2: the nil check before the seal | `M8-2` (the old code) | `if attempt == nil { return }` back in `finishBudgetUsageAttempt` | survives, and should: `seal` answers nil for a nil attempt and the function returns on an empty batch. Pinned: `M8-2p` (`seal` without its own nil check) is killed by `…HandleTaskWithoutAttemptOrJournal` (a nil dereference) | holds |
| 8. removal 3: the conditional attributes of the kept Debug line | `M8-3` (the old code) | `status` only when not zero, `error` only when not nil | survives. Not behaviour-preserving to the letter: the line now always carries both, so `status=0` when no answer arrived and `error=<nil>` when one did. No test reads the attributes of that line, before or after; no test went with the removal | `KD2-18` (`note`) |
| 8. removal 4: the 15 s bound of one send | `M8-4` (the old code) | `context.WithTimeout(ctx, 15*time.Second)` back around the report | survives: no test of the lane holds an answer open, so none tells a bound of 15 s from one of 30 s or from none. What the bound was there for is the subject of line 9 (a) | `KD2-01` |
| 9 (a). the per-call client built without the timeout | `M9-a` | `http.Client{Transport: c.client.Transport}` with the same `CheckRedirect` | survives all 78 tests of the lane; killed by my `…ReportGivesUpByTheClientsOwnBound` (`M9-a-k`). Also `M9-d-k` (the copy gives up its timeout) by the same test, and `M9-c` (300 s on the client) by my `…ReportBoundIsThirtySeconds` | `KD2-01`, `major`, with its test |
| 9 (b). the send under `context.Background()` | `M9-b` | the report made with a context that never ends | killed by `…SendEndsWithTheDaemonAndKeepsTheRecord` | lane |

Count: 9 lines, 56 mutants in the Mutant column and four more named in the
Result column (`M8-1p`, `M8-2p`, `M9-d-k`, `M9-c`). No line is `not checked`. Seven mutants survive
the lane's tests where behaviour changes (`ME3-5`, `ME3-14`, `ME3-15`,
`M3-3`, `ME8-3`, `ME8-4`, `M9-a`); each has a finding and a test of mine
that kills it. Four survive because they are the old code of stage F
(`M8-1` to `M8-4`): two change nothing, one changes a Debug line, one is
the bound of line 9.

### 3.5 The lines the lane believes no test pins (its report 11.4 (f))

One mutant for each line of the lane's list, which has thirteen (the brief
names nine of them), and one line of my own at the end.

**A test kills it after all**

| Line | Mutant | Killed by |
| --- | --- | --- |
| `budgetUsageErrorCode`: the `return ""` on a decode error | `ME7-4` (the error of `json.Unmarshal` is ignored) | `…RefusalWarningNamesTheCodeAndNeverTheBody` |
| `journal.background` ("waited on by tests only") | `MF-bg` (the sends are not counted) | `…BatchOfARunThatOutlastedTheCapIsSentOnce`, `…HeldOpenReportDoesNotDelayOrChangeTheTerminalReport`, `…PassDoesNotSendARecordThatIsBeingSent`, `…RecoveryLeavesARunningAttemptAlone`. The lane's sentence is true as it stands: `Wait` has no caller outside the tests, so the counter is test machinery in production code |

**Nothing kills it, and nothing observable changes**

| Line | Mutant | Why nothing changes |
| --- | --- | --- |
| `reportBudgetUsage`: `url.PathEscape` | `ME1-7b` (the id goes into the path as it is) | the id is a canonical UUID wherever it comes from: `begin` refuses any other, and a pass takes only names of the form `<canonical uuid>.json`. Escaping such a string returns it |
| `reportBudgetUsage`: the `token != ""` guard | `ME1-8` (the header is set whatever the token) | the difference is `Authorization: Bearer ` with nothing after it on a request of a client that has no token, which is not authenticated either way. The guard is the form the client's other requests use |
| `load`: the early return for a held or settled attempt | `MF-load` | one more file read under the lock. The two branches of the handler that follow use neither the record nor the error |
| `persistLaunchIntentLocked`: the operand `errors.Is(removeErr, os.ErrNotExist)` | `MF-intent` | reached only when the write of a launch intent fails and the file is already gone. Then one warning says "could not remove its file" where it said "removed its file". The record in memory, the run and the later writes are the same |
| `budgetUsageLoop`: `defer ticker.Stop()` | `MF6-10` | the ticker of a loop that has ended. Nothing reads it, and the runtime frees a ticker nobody refers to |
| `reportBudgetUsage`: `defer resp.Body.Close()` | `MF-close` | on every path the body was read to its end or to an error before the function returns, and either gives the connection back. Go's contract still asks for the call; the line should stay. I measured no leak and looked for none |
| mine: `load` holds the journal's lock over the file read | `MF4-2` (the lock let go before the read) | a pass takes that lock in `claim` after it listed the file, so it waits for `begin` in any case, and an id is begun once (F4) |

**Nothing kills it, and behaviour changes: findings**

| Line | Mutant | What changes | Finding |
| --- | --- | --- | --- |
| `reportBudgetUsage`: the 30 s bound | `M9-a` | a report against a server that holds the request open never ends, and the resend loop stops with it | `KD2-01` |
| `verdict`: the lower bound `r.status >= 200` | `ME3-5` | a 101 removes the record as delivered | `KD2-03` |
| `reportBudgetUsage`: `io.Copy(io.Discard, resp.Body)` | `ME7-6` | an answer longer than 64 KiB that ends early is judged by its status line: a 200 delivers, a 400 refuses for good | `KD2-04` |
| `budgetUsageRecordIDs`: `slices.Sort` | `MF-sort` | a record this process holds and whose file exists is handled twice in one pass: two requests per pass for a kept batch | `KD2-07` |
| `remove`: `delete(j.settled, attemptID)` | `M3-3` | the entry of an attempt whose file was removed later stays for the life of the process. Nothing else: a held entry is looked at before a settled one, so no batch is lost or sent again | `KD2-09` |

So of the thirteen lines: two are pinned after all, six can go unpinned
without loss, five are findings. The lane's "as far as I can tell by
reading" was right for eleven of the thirteen and wrong in its reason for two
of those eleven: the lower bound of 2xx ("no test has a status below 200 without
an error", true, but a 101 reaches the code) and the sort ("a directory
listing is sorted already", true, but the list also has the table's ids in
front of it).

## 4. G3 items: test and mutant

Every test name is prefixed `TestBudgetUsage`; every mutant listed fails the
test in its row (section 5 has the other tests it fails).

| Item | Test | What it asserts | A mutant of mine it kills |
| --- | --- | --- | --- |
| B33: a daemon that crashed after sealing resends the stored bytes under the stored key | `…RestartResendsThePersistedBatchByteForByte` (`budget_usage_recovery_test.go:218`), cases "a batch with an invocation" and "an empty batch" | The next process's first pass (answered 503) makes one `POST` to the attempt's usage path with `Content-Type` and the credential; the body equals the bytes on disk; the record file is unchanged. The second pass (delivered) sends the same bytes again; the batch carries the record's key and the start identity of the process that built it, not of the one that sends; then the file is gone, the sender holds no record of its own and nothing was warned | `ME5-1` (recovery builds a batch again for a record that has batch bytes) |
| B53: no declaration without a journal | `…CapabilityIsNotDeclaredWithoutAUsableJournal` (`budget_usage_send_test.go:392`), three ways to spoil it: the journal path is a file, the directory cannot be written, the profile directory cannot be resolved | The daemon holds no journal and warned once; the capabilities header of the HTTP claim and of the WebSocket handshake is exactly what the daemon declared before the protocol; a task that carries an attempt id runs and is reported complete once; no usage report; still one warning | `MA1-2` (the flag is set before the journal's open error is looked at); also `MA1-1` (the capability is always declared) |
| B53: against an older server nothing is recorded or sent | `…HandleTaskWithoutAttemptOrJournal` (`budget_usage_capture_test.go:1022`) and `…RunTaskWithoutTheProtocol` (`budget_usage_runtask_unix_test.go:676`), case "journal, no attempt id": a task as an older server hands it out | The run carries no attempt; the legacy requests are the four of a daemon without the protocol, same order and bodies; no request goes to the usage route; the journal directory is empty and holds no live record; nothing about the protocol is logged | `MG3-4` (a task without `budget_attempt_id` gets a record and a report) |
| 11.11: restart resends under the original key | stored bytes: the B33 test. A batch built after the restart: `…RestartReportsAnInvocationWithoutAnOutcomeAsUnknown` (`budget_usage_recovery_test.go:303`) | B33 test: `batch_key` of the sent batch is the record's. The other: the expected batch has `BatchKey: left.BatchKey`, the key the dead process minted, and is compared as a whole | `ME5-1`; `MG3-1` (a rebuilt batch goes under another key than the record's) |
| 11.11: an intent without an outcome is reported as `unknown` by the process that finds it | `…RestartReportsAnInvocationWithoutAnOutcomeAsUnknown` | The dead process left one invocation that returned and one with only its intent. The decoded batch of the next process equals the expected one field for field: its own start identity, `reported_at` at the time it builds, the first invocation `returned` with its usage, the second `unknown` with `observed_at` at the time of the intent and `"usage":[]` (also checked in the raw bytes). The record file held those bytes when the request arrived; the record on disk is the one left plus the bytes; a later pass sends the identical body | `MG3-2` (reported as `returned`), `MG3-3` (left out of the batch), `MF2-3` (the batch carries the dead process's identity) |
| 11.11: no declaration when the directory is unusable | the B53 test, cases "the journal path is a file" and "the journal directory cannot be written" | as above | `MA1-2` |

One limit of the last row and of the second: the three ways the test spoils
the journal are all failures of opening it. A directory that stops being
usable while the daemon runs keeps the declaration (the flag is set once at
start); the task then runs without a record, which is part 1's point
(`…HandleTaskRunsWithoutARecordThatCannotBeCreated`) and the accepted item 3.

## 5. Mutants

Method. One mutant is one change, fed to the compiler with
`go test -vet=off -overlay <file> -run '<tests>' -count=1 ./internal/daemon/`
from `server/` of the worktree; the tree is never written. The runner is
`/tmp/f3-s6kd2/mut.py` (`python3 -I /tmp/f3-s6kd2/mut.py /tmp/f3-s6kd2/specNN.py`);
each mutant has a directory `/tmp/f3-s6kd2/m/<id>/` with `diff.txt` (the
text replaced and the replacement), `overlay.json` and `out.log`; one line
per mutant is in `/tmp/f3-s6kd2/m/results.tsv`. Unless a line says
otherwise the tests run are `^TestBudgetUsage` (the lane's 78, and mine
from the moment each file existed: the table before 5.2), on go1.27.0.
Controls: `C00` (an unchanged copy of `budget_usage_delivery.go`), `C01`
(the wiring test reading an unchanged copy of `daemon.go`), `C02` (an
unchanged copy of `budget_usage_recovery.go`) and `C03` (an unchanged copy
of `budget_usage.go`) pass; each was run before the first series that
changes its file. The other files a mutant changes (`client.go`,
`managed_transport.go`, `budget_usage_batch.go`, `budget_usage_capture.go`)
have no control of their own; the mechanism is the same one.

One limit of the method that changes a result. The wiring test reads
`daemon.go` from disk with `go/parser`, so an overlay of `daemon.go` alone
never reaches it: seven mutants of `Run` "survived" a first series for that
reason only (`MA5-1` to `MA5-7` in `results.tsv`). They were run again
(`MA5-1w` to `MA5-7w`) with the overlay also replacing
`budget_usage_wiring_test.go` by a copy whose one change is the path it
parses (`"daemon.go"` became the mutated copy). Those results are the ones
that count. Whoever repeats a mutant run over `Run` with an overlay must do
the same, or will read seven false survivors.

### 5.1 Declaring the protocol and the start order (A1, A5; brief 5.1 item 7)

| Id | File | Change | Result |
| --- | --- | --- | --- |
| `MA1-1` | `client.go` | `capabilities()` appends the capability whatever the flag says | killed by `…CapabilityIsDeclaredWithAUsableJournal`, `…IsNotDeclaredWithoutAUsableJournal`, `…ReachesManagedWorkspaceClients` |
| `MA1-2` | `budget_usage_delivery.go` | `setBudgetUsageReady()` called before the error of `openBudgetUsageJournal` is looked at | killed by `…CapabilityIsNotDeclaredWithoutAUsableJournal` |
| `MA1-3` | `managed_transport.go` | a workspace client made later does not copy the flag | killed by `…CapabilityReachesManagedWorkspaceClients` |
| `MA1-4` | `budget_usage_delivery.go` | existing workspace clients are not told | killed by `…CapabilityReachesManagedWorkspaceClients` |
| `MA1-5` | `budget_usage_delivery.go` | a usable journal and no declaration | killed by `…CapabilityIsDeclaredWithAUsableJournal`, `…ReachesManagedWorkspaceClients` |
| `MA1-6` | `budget_usage_delivery.go` | declared and the journal never installed, first form | does not compile (`journal` unused) |
| `MA1-6b` | `budget_usage_delivery.go` | the same with `_ = journal` | killed by `…CapabilityIsDeclaredWithAUsableJournal` (its task sends no report), `…ReachesManagedWorkspaceClients` |
| `MA5-1w` | `daemon.go` | the loop started before `initializeBudgetUsage` | killed by the wiring test |
| `MA5-2w` | `daemon.go` | the loop started after `pollLoop` | killed by the wiring test |
| `MA5-3w` | `daemon.go` | the loop call moved into `if false { … }` | killed by the wiring test |
| `MA5-4w` | `daemon.go` | the journal opened before `listenHealth` | killed by the wiring test |
| `MA5-5w` | `daemon.go` | the journal opened after `preflightAuth` | killed by the wiring test |
| `MA5-6w` | `daemon.go` | the loop never started | killed by the wiring test |
| `MA5-7w` | `daemon.go` | the journal call moved into `if false { … }` | killed by the wiring test |

The question of brief 5.1 item 7: yes, the syntax-tree test also fails when
the call is moved into a branch that is not taken (`MA5-3w`, `MA5-7w`). It
reads only the statements of `Run`'s own body and requires each of the five
exactly once, so a call inside an `if` is "no statement of its own".

In the tables that follow the result is copied from `results.tsv` by
`/tmp/f3-s6kd2/gen5.py`; nothing in the Result column is written from
memory. A list longer than four tests is cut to three "and N more" (the
whole list is the fifth column of the mutant's line in `results.tsv`).
"mine:" marks a test of my files (section 7). The Change column is the
mutant's note; the exact text replaced and its replacement are in
`/tmp/f3-s6kd2/m/<id>/diff.txt`.

Which tests were compiled into a run depends on when it was made, because my
test files are part of the package from the moment they exist:

| Lines of `results.tsv` | Series | When (2026-11-20) | Tests in the package |
| --- | --- | --- | --- |
| 1 to 147 | `spec01` to `spec05b` | 01:09 to 04:23 | the lane's 78 |
| 148 to 152 | `spec06` | 04:53 to 04:55 | the 78 and my bound file (2 tests) |
| 153 to 173 | `spec07` to `spec10` | 05:52 to 11:03 | the 78 and my bound, pins and outlasted files (11 tests) |
| 174 to 185 | `spec11` to `spec13` | 12:37 to 17:23 | the 78 and my four files (12 tests) |

So "survived" on a line up to 147 means: all 78 tests of the lane pass on
the mutant. Every mutant of sections 5.2 to 5.4 is on such a line except
`M9-c` (150), `MF1-10`, `MF-close`, `MF-bg` (167 to 169), `MF1-11`,
`MF1-12`, `MF4-2`, `MF4-3` (170 to 173) and `MG5-1`, `MG5-2b`, `MG5-3b`
(178, 181, 182); for those the tests of mine that existed ran too, and a
kill by one of them is marked "mine:".

### 5.2 The request, the answer, the classes (E1 to E9)

`budget_usage_delivery.go` and `budget_usage_recovery.go`; all budget tests.

| Id | File | Change | Result |
| --- | --- | --- | --- |
| `ME1-1` | `budget_usage_delivery.go` | PUT instead of POST | killed by `…HandleTaskRecordsAndSeals`, `…HandleTaskSendsTheSealedBatch`, `…RestartResendsThePersistedBatchByteForByte` and 3 more |
| `ME1-2` | `budget_usage_delivery.go` | another path | killed by `…CapabilityIsDeclaredWithAUsableJournal`, `…HandleTaskRecordsAndSeals`, `…HandleTaskSendsTheSealedBatch` and 16 more |
| `ME1-3` | `budget_usage_delivery.go` | the body is decoded and encoded again on the way | killed by `…AnswerDecidesWhatBecomesOfTheRecord`, `…AnsweredRecordIsNotSentAgainWhenItsFileCannotBeRemoved`, `…BatchOfARunThatOutlastedTheCapIsSentOnce` and 20 more |
| `ME1-4` | `budget_usage_delivery.go` | no Content-Type | killed by `…HandleTaskSendsTheSealedBatch`, `…RestartResendsThePersistedBatchByteForByte` |
| `ME1-5` | `budget_usage_delivery.go` | no Authorization header | killed by `…HandleTaskSendsTheSealedBatch`, `…ManagedInstallationSendsThroughTheWorkspaceOfTheRecord`, `…RestartInAManagedInstallationSendsWithTheCredentialOfTheRecordsWorkspace`, `…RestartResendsThePersistedBatchByteForByte` |
| `ME1-6` | `budget_usage_delivery.go` | no identity or capability headers on a report | killed by `…HandleTaskSendsTheSealedBatch`, `…ManagedInstallationSendsThroughTheWorkspaceOfTheRecord`, `…RestartInAManagedInstallationSendsWithTheCredentialOfTheRecordsWorkspace` |
| `ME1-7b` | `budget_usage_delivery.go` | the path escape removed (the attempt id goes into the path as it is) | survived |
| `ME1-8` | `budget_usage_delivery.go` | the token guard removed (the header is set whatever the token) | survived |
| `ME2-1` | `budget_usage_delivery.go` | the send made on the task's goroutine | killed by `…HeldOpenReportDoesNotDelayOrChangeTheTerminalReport`, `…PassDoesNotSendARecordThatIsBeingSent` |
| `ME2-2` | `daemon.go` | the send made under the context of the run, which ends with handleTask | killed by `…HeldOpenReportDoesNotDelayOrChangeTheTerminalReport` |
| `ME2-3` | `budget_usage_delivery.go` | no send after the seal at all (only the loop sends) | killed by `…BatchOfARunThatOutlastedTheCapIsSentOnce`, `…CapabilityIsDeclaredWithAUsableJournal`, `…HandleTaskRecordsAndSeals` and 8 more |
| `ME3-1` | `budget_usage_delivery.go` | a kept answer (401) treated as final | killed by `…AnswerDecidesWhatBecomesOfTheRecord` |
| `ME3-2` | `budget_usage_delivery.go` | a final answer (404) kept | killed by `…AnswerDecidesWhatBecomesOfTheRecord`, `…NotFoundAnswerOnlyDropsTheRecord` |
| `ME3-3` | `budget_usage_delivery.go` | a success (299) treated as kept | killed by `…AnswerDecidesWhatBecomesOfTheRecord` |
| `ME3-4` | `budget_usage_delivery.go` | 300 treated as delivered | killed by `…AnswerDecidesWhatBecomesOfTheRecord` |
| `ME3-5` | `budget_usage_delivery.go` | the lower bound of the 2xx range moved to 100 | survived |
| `ME3-6` | `budget_usage_delivery.go` | 200 treated as kept | killed by `…AnswerDecidesWhatBecomesOfTheRecord`, `…AnsweredRecordIsNotSentAgainWhenItsFileCannotBeRemoved`, `…CapabilityIsDeclaredWithAUsableJournal` and 18 more |
| `ME3-7` | `budget_usage_delivery.go` | 422 kept | killed by `…AnswerDecidesWhatBecomesOfTheRecord` |
| `ME3-8` | `budget_usage_delivery.go` | 413 kept | killed by `…AnswerDecidesWhatBecomesOfTheRecord` |
| `ME3-9` | `budget_usage_delivery.go` | 503 treated as final | killed by `…AnswerDecidesWhatBecomesOfTheRecord`, `…BatchOfARunThatOutlastedTheCapIsSentOnce`, `…HandleTaskRecordsAndSeals` and 17 more |
| `ME3-10` | `budget_usage_delivery.go` | an answer that cannot be read to its end is judged by its status line | killed by `…AnswerDecidesWhatBecomesOfTheRecord` |
| `ME3-11` | `budget_usage_delivery.go` | 429 treated as final | killed by `…AnswerDecidesWhatBecomesOfTheRecord` |
| `ME3-12` | `budget_usage_delivery.go` | 403 treated as final | killed by `…AnswerDecidesWhatBecomesOfTheRecord` |
| `ME3-13` | `budget_usage_delivery.go` | a 400 is kept and sent again | killed by `…AnswerDecidesWhatBecomesOfTheRecord`, `…AnsweredRecordIsNotSentAgainWhenItsFileCannotBeRemoved`, `…PassLeavesAnAnsweredRecordAloneWhenItsFileCannotBeRemoved`, `…RefusalWarningNamesTheCodeAndNeverTheBody` |
| `ME3-14` | `budget_usage_delivery.go` | a 412 is final (a status the test does not list moves to the refused class) | survived |
| `ME3-15` | `budget_usage_delivery.go` | a 306 is delivered (a 3xx the test does not list moves to the delivered class) | survived |
| `ME4-1` | `budget_usage_delivery.go` | redirects are followed | killed by `…AnswerDecidesWhatBecomesOfTheRecord` |
| `ME4-2` | `budget_usage_delivery.go` | following is switched off on the shared client, for every later request of the daemon | survived |
| `ME4-2r` | `budget_usage_delivery.go` | ME4-2 again with -race: the shared client is written from the send goroutine | killed by `…CapabilityIsDeclaredWithAUsableJournal`, `…EndedFollowsTheResultSource`, `…HandleTaskRecordsAndSeals` and 15 more |
| `ME4-3` | `budget_usage_delivery.go` | following is switched off on the shared client once, where the protocol is declared (nothing races) | survived |
| `ME4-3r` | `budget_usage_delivery.go` | ME4-3 again with -race | survived |
| `ME5-1` | `budget_usage_recovery.go` | recovery builds a batch again for a record that has batch bytes | killed by `…LoopPassesAtStartAndThenAtEveryTick`, `…PassHandlesAtMostThirtyTwoRecordsAndTakesUpWhereItStopped`, `…PassLeavesAnAnsweredRecordAloneWhenItsFileCannotBeRemoved` and 7 more |
| `ME5-2` | `budget_usage_delivery.go` | a kept answer unseals the stored record (the next process builds other bytes under the key) | killed by `…AnswerDecidesWhatBecomesOfTheRecord`, `…HandleTaskRecordsAndSeals`, `…HandleTaskWithoutAttemptOrJournal` and 11 more |
| `ME6-1` | `budget_usage_delivery.go` | a 404 of the usage route drops the daemon's runtimes | killed by `…NotFoundAnswerOnlyDropsTheRecord` |
| `ME7-1` | `budget_usage_delivery.go` | the form of a code is not checked | killed by `…NotFoundAnswerOnlyDropsTheRecord`, `…RefusalWarningNamesTheCodeAndNeverTheBody` |
| `ME7-2` | `budget_usage_delivery.go` | the error field is never consulted | killed by `…RefusalWarningNamesTheCodeAndNeverTheBody` |
| `ME7-3` | `budget_usage_delivery.go` | the error field wins over the code field | killed by `…AnswerDecidesWhatBecomesOfTheRecord`, `…RefusalWarningNamesTheCodeAndNeverTheBody` |
| `ME7-4` | `budget_usage_delivery.go` | the empty return on a decode error removed | killed by `…RefusalWarningNamesTheCodeAndNeverTheBody` |
| `ME7-5` | `budget_usage_delivery.go` | the whole answer is read into memory | killed by `…RefusalWarningNamesTheCodeAndNeverTheBody` |
| `ME7-6` | `budget_usage_delivery.go` | the rest of an answer is not read | survived |
| `ME7-7` | `budget_usage_delivery.go` | the warning carries the answer itself | killed by `…AnswerDecidesWhatBecomesOfTheRecord`, `…AnsweredRecordIsNotSentAgainWhenItsFileCannotBeRemoved`, `…NotFoundAnswerOnlyDropsTheRecord` and 3 more |
| `ME7-8` | `budget_usage_delivery.go` | the warning does not name the status | killed by `…AnswerDecidesWhatBecomesOfTheRecord`, `…NotFoundAnswerOnlyDropsTheRecord`, `…RebuiltBatchAnsweredWithOperationConflictIsDropped`, `…RefusalWarningNamesTheCodeAndNeverTheBody` |
| `ME7-9` | `budget_usage_delivery.go` | the Debug line of a kept report prints the batch | killed by `…AnswerDecidesWhatBecomesOfTheRecord`, `…HandleTaskRecordsAndSeals`, `…HandleTaskSealsFromMemoryWhenTheJournalFails` |
| `ME7-10` | `budget_usage_delivery.go` | 64 MiB of an answer are kept instead of 64 KiB | killed by `…RefusalWarningNamesTheCodeAndNeverTheBody` |
| `ME7-11` | `budget_usage_delivery.go` | the warning of a refusal prints the batch | killed by `…AnswerDecidesWhatBecomesOfTheRecord`, `…AnsweredRecordIsNotSentAgainWhenItsFileCannotBeRemoved`, `…PassLeavesAnAnsweredRecordAloneWhenItsFileCannotBeRemoved` and 2 more |
| `ME7-12` | `budget_usage_delivery.go` | the Debug line of a delivered report prints the batch | killed by `…AnswerDecidesWhatBecomesOfTheRecord`, `…AnsweredRecordIsNotSentAgainWhenItsFileCannotBeRemoved`, `…HandleTaskSendsTheSealedBatch`, `…PassLeavesAnAnsweredRecordAloneWhenItsFileCannotBeRemoved` |
| `ME8-1` | `budget_usage_recovery.go` | recovery sends a record whose server_url differs | killed by `…RecordOlderThanSevenDaysIsDroppedWithAWarning`, `…RestartKeepsARecordOfAnotherServer` |
| `ME8-2` | `budget_usage_recovery.go` | the comparison made after the batch of an earlier process was built and written | killed by `…RestartKeepsARecordOfAnotherServer` |
| `ME8-3` | `budget_usage_recovery.go` | the comparison ignores case | survived |
| `ME8-4` | `budget_usage_recovery.go` | the comparison ignores a trailing slash | survived |
| `ME8-5b` | `budget_usage_recovery.go` | retention test and server test swapped | killed by `…RecordOlderThanSevenDaysIsDroppedWithAWarning` |
| `ME8-6` | `budget_usage_delivery.go` | a managed installation sends with the root client | killed by `…ManagedInstallationSendsThroughTheWorkspaceOfTheRecord`, `…RestartInAManagedInstallationSendsWithTheCredentialOfTheRecordsWorkspace` |
| `ME8-7` | `budget_usage_delivery.go` | a workspace without a valid binding falls back to the root client | killed by `…ManagedInstallationSendsThroughTheWorkspaceOfTheRecord`, `…RestartInAManagedInstallationSendsWithTheCredentialOfTheRecordsWorkspace` |
| `ME8-8` | `budget_usage_delivery.go` | a workspace without a valid binding borrows the client of another workspace | killed by `…ManagedInstallationSendsThroughTheWorkspaceOfTheRecord`, `…RestartInAManagedInstallationSendsWithTheCredentialOfTheRecordsWorkspace` |
| `ME8-9` | `budget_usage_delivery.go` | an expired binding still sends | killed by `…ManagedInstallationSendsThroughTheWorkspaceOfTheRecord` |
| `ME9-1` | `budget_usage_delivery.go` | a 409 is kept and sent again | killed by `…AnswerDecidesWhatBecomesOfTheRecord`, `…RebuiltBatchAnsweredWithOperationConflictIsDropped` |

Read with the table:

- `ME3-14` (a 412 is final) and `ME3-15` (a 306 is delivered) survive, as
  `ME3-5` (delivered from 100 on) does: the lane's test lists its statuses
  one by one (38 cases), and no 1xx, no 306 and no 412 is among them. That
  is `KD2-03`; section 5.5 has the three killed by my exhaustive test.
- `ME4-2r` is a kill by the race detector, not by an assertion: 17 data race
  reports (the write is `budget_usage_delivery.go:153` of the mutant, the
  read is `net/http.(*Client).do`), and the 18 tests named fail with "race
  detected during execution of test". Without `-race` (`ME4-2`) and for the
  variant that writes once before any request (`ME4-3`, `ME4-3r`) nothing
  fails. That is `KD2-05`.
- `ME1-7b` and `ME1-8` change nothing that can be observed (section 3.5).
- `ME7-6`, `ME8-3`, `ME8-4` are `KD2-04` and `KD2-02`.

### 5.3 The mark, the settled set, retention, the cap of a pass, stage F, the bound (brief 5.1 items 2, 3, 5, 6, 8, 9)

All budget tests.

| Id | File | Change | Result |
| --- | --- | --- | --- |
| `M2-1` | `budget_usage_delivery.go` | claim always answers yes | killed by `…PassDoesNotSendARecordThatIsBeingSent`, `…RecordIsNeverSentTwiceAtOnce` |
| `M2-2` | `budget_usage_delivery.go` | the release dropped | killed by `…AnswerDecidesWhatBecomesOfTheRecord`, `…AnsweredRecordIsNotSentAgainWhenItsFileCannotBeRemoved`, `…BatchOfARunThatOutlastedTheCapIsSentOnce` and 14 more |
| `M2-3` | `budget_usage_delivery.go` | the release made at once instead of at the end of the handling | killed by `…PassDoesNotSendARecordThatIsBeingSent`, `…RecordIsNeverSentTwiceAtOnce` |
| `M3-1` | `budget_usage_delivery.go` | a removal that fails does not remember the attempt | killed by `…AnsweredRecordIsNotSentAgainWhenItsFileCannotBeRemoved`, `…PassLeavesAnAnsweredRecordAloneWhenItsFileCannotBeRemoved` |
| `M3-2` | `budget_usage_delivery.go` | the settled case of the handler dropped | killed by `…AnsweredRecordIsNotSentAgainWhenItsFileCannotBeRemoved`, `…PassLeavesAnAnsweredRecordAloneWhenItsFileCannotBeRemoved` |
| `M3-2b` | `budget_usage_delivery.go` | the settled case does nothing: the file is never removed by this process | killed by `…AnsweredRecordIsNotSentAgainWhenItsFileCannotBeRemoved`, `…PassLeavesAnAnsweredRecordAloneWhenItsFileCannotBeRemoved` |
| `M3-3` | `budget_usage_delivery.go` | a removal that succeeds later leaves the settled entry behind | survived |
| `M3-4` | `budget_usage_delivery.go` | the settled set is never consulted | killed by `…AnsweredRecordIsNotSentAgainWhenItsFileCannotBeRemoved`, `…PassLeavesAnAnsweredRecordAloneWhenItsFileCannotBeRemoved` |
| `M5-1` | `budget_usage_recovery.go` | the age test inverted | killed by `…BatchOfARunThatOutlastedTheCapIsSentOnce`, `…KeptBatchOfThisProcessIsResentFromMemory`, `…LoopPassesAtStartAndThenAtEveryTick` and 15 more |
| `M5-2` | `budget_usage_recovery.go` | the boundary moved: exactly seven days is dropped | killed by `…RecordOlderThanSevenDaysIsDroppedWithAWarning`, `…RetentionRemovesOldTemporaryFilesAndNoOtherForeignName`, `…UnreadableRecordIsSkippedUntilTheCapAndThenDropped` |
| `M5-2c` | `budget_usage_recovery.go` | the boundary moved: seven days and one second is kept | killed by `…RecordOlderThanSevenDaysIsDroppedWithAWarning`, `…RetentionRemovesOldTemporaryFilesAndNoOtherForeignName`, `…UnreadableRecordIsSkippedUntilTheCapAndThenDropped` |
| `M5-3` | `budget_usage_delivery.go` | a sealed held record is dropped by the send that follows its seal | killed by `…BatchOfARunThatOutlastedTheCapIsSentOnce`, `…RecoveryLeavesARunningAttemptAlone` |
| `M5-4a` | `budget_usage_delivery.go` | a held unsealed record past the cap is dropped by a pass | killed by `…RecoveryLeavesARunningAttemptAlone` |
| `M5-4b` | `budget_usage_delivery.go` | a held unsealed record is sent | killed by `…RecoveryLeavesARunningAttemptAlone` |
| `M5-5` | `budget_usage_recovery.go` | an unreadable file with a record's name is removed before it is seven days old | killed by `…UnreadableRecordIsSkippedUntilTheCapAndThenDropped` |
| `M5-6` | `budget_usage_recovery.go` | the regular-file condition of that removal dropped | killed by `…UnreadableRecordIsSkippedUntilTheCapAndThenDropped` |
| `M5-7` | `budget_usage_recovery.go` | temporary files: the regular-file condition dropped | killed by `…RetentionRemovesOldTemporaryFilesAndNoOtherForeignName` |
| `M5-8` | `budget_usage_recovery.go` | temporary files: removed whatever their age | killed by `…RetentionRemovesOldTemporaryFilesAndNoOtherForeignName` |
| `M5-9` | `budget_usage_recovery.go` | temporary files: any name ending in .tmp | killed by `…RetentionRemovesOldTemporaryFilesAndNoOtherForeignName` |
| `M5-10` | `budget_usage_recovery.go` | temporary files: any name starting with a dot | killed by `…RecoveryActsOnlyOnCanonicalRecordNames`, `…RetentionRemovesOldTemporaryFilesAndNoOtherForeignName` |
| `M6-1` | `budget_usage_recovery.go` | the cap raised by one (the constant) | killed by `…PassHandlesAtMostThirtyTwoRecordsAndTakesUpWhereItStopped` |
| `M6-1b` | `budget_usage_recovery.go` | the cap raised by one (where it is applied) | killed by `…PassHandlesAtMostThirtyTwoRecordsAndTakesUpWhereItStopped` |
| `M6-2` | `budget_usage_recovery.go` | the cursor never advanced | killed by `…PassHandlesAtMostThirtyTwoRecordsAndTakesUpWhereItStopped` |
| `M6-3` | `budget_usage_recovery.go` | the cursor advanced past a record that was not tried | killed by `…PassHandlesAtMostThirtyTwoRecordsAndTakesUpWhereItStopped` |
| `M6-4` | `budget_usage_recovery.go` | a pass starts again at the last record of the pass before | killed by `…PassHandlesAtMostThirtyTwoRecordsAndTakesUpWhereItStopped` |
| `M8-1` | `budget_usage.go` | removal 1 put back: the empty-version operand | survived |
| `M8-1p` | `budget_usage.go` | what removal 1 was there for: an empty version gives something else than nothing | killed by `…AdapterVersionNeverCostsTheBatch`, `…RunTaskNamesTheAdapter` |
| `M8-2` | `budget_usage_delivery.go` | removal 2 put back: the nil check before the seal | survived |
| `M8-2p` | `budget_usage.go` | what removal 2 was there for: seal is not safe on a nil attempt | killed by `…HandleTaskWithoutAttemptOrJournal` |
| `M8-3` | `budget_usage_delivery.go` | removal 3 put back: conditional log attributes of a kept report | survived |
| `M8-4` | `budget_usage_delivery.go` | removal 4 put back: a 15 s bound of its own around one report | survived |
| `M9-a` | `budget_usage_delivery.go` | the per-call client built without the timeout | survived |
| `M9-b` | `budget_usage_delivery.go` | the send made under context.Background() | killed by `…SendEndsWithTheDaemonAndKeepsTheRecord` |
| `M9-c` | `client.go` | the client's bound is 300 s instead of 30 s (all budget tests, the checker's included) | killed by mine: `…ReportBoundIsThirtySeconds` |

Read with the table:

- `M8-1` to `M8-4` are not changes of mine: each puts back the text one of
  the four removals of stage F took out, as item 8 of brief 5.1 asks. A
  removal that changed nothing must survive. `M8-1p` and `M8-2p` are the
  mutants of what the removed guard relied on, and both are killed.
  `M8-3` is `KD2-18`; `M8-4` and `M9-a` are `KD2-01`.
- `M9-c` (line 150) ran with my bound file in the package and is killed by
  it alone: all 78 tests of the lane pass with a bound of 300 s on the
  client.
- `M3-3` is `KD2-09`.

### 5.4 Recovery, the loop, the 7-day cap, the acceptance rows, G5 (points F1 to F7, G3, G5)

All budget tests. The ids `MF1-6`, `MF6-2` and `MF6-5` were never used
(the numbering has gaps, no run is missing).

| Id | File | Change | Result |
| --- | --- | --- | --- |
| `MF1-1` | `budget_usage_recovery.go` | a record without batch bytes and without an invocation is built and reported | killed by `…RecoveryNeverReportsARecordWithoutBatchAndWithoutInvocation`, `…RestartKeepsARecordOfAnotherServer`, `…SendAfterTheSealHandlesOnlyWhatThisProcessHolds` |
| `MF1-2` | `budget_usage_recovery.go` | a leftover of this process is built and reported | killed by `…RecoveryDropsARecordOfThisProcessThatNoRunningAttemptHolds` |
| `MF1-3` | `budget_usage_recovery.go` | what an earlier process left is removed instead of reported | killed by `…PassAfterTheDaemonStoppedHandlesNothing`, `…RebuiltBatchAnsweredWithOperationConflictIsDropped`, `…RecordOlderThanSevenDaysIsDroppedWithAWarning` and 5 more |
| `MF1-4` | `budget_usage_recovery.go` | a record of another server that names no launch is removed | killed by `…RestartKeepsARecordOfAnotherServer` |
| `MF1-5` | `budget_usage_delivery.go` | the send that follows the seal also recovers what an earlier process left | killed by `…SendAfterTheSealHandlesOnlyWhatThisProcessHolds` |
| `MF1-7` | `budget_usage_recovery.go` | any name ending in .json is taken for a record | killed by `…RecoveryActsOnlyOnCanonicalRecordNames` |
| `MF1-8` | `budget_usage_delivery.go` | a record that read refuses is removed at once | killed by `…UnreadableRecordIsSkippedUntilTheCapAndThenDropped` |
| `MF1-9` | `budget_usage_delivery.go` | a record that read refuses is never removed, not even after seven days | killed by `…UnreadableRecordIsSkippedUntilTheCapAndThenDropped` |
| `MF1-10` | `budget_usage_recovery.go` | a record of an earlier process with batch bytes and older than the cap is sent instead of dropped | killed by `…RecordOlderThanSevenDaysIsDroppedWithAWarning` |
| `MF1-11` | `budget_usage_delivery.go` | a sealed record this process holds is never capped by a pass | killed by `…BatchOfARunThatOutlastedTheCapIsSentOnce`, `…RecordOlderThanSevenDaysIsDroppedWithAWarning`; mine: `…CheckOutlastedBatchIsDroppedUnsentWhenAPassComesFirst` |
| `MF1-12` | `budget_usage_recovery.go` | a pass over a record whose file is gone warns and removes | killed by `…PassOverARecordThatIsGoneDoesNothing` |
| `MF2-1` | `budget_usage_recovery.go` | a rebuilt batch is sent without being written into the record | killed by `…RecoveryDoesNotSendABatchItCouldNotPersist`, `…RestartReportsAnInvocationWithoutAnOutcomeAsUnknown` |
| `MF2-2` | `budget_usage_recovery.go` | a rebuilt batch is sent although its write failed | killed by `…RecoveryDoesNotSendABatchItCouldNotPersist` |
| `MF2-3` | `budget_usage_recovery.go` | a rebuilt batch carries the start identity of the dead process | killed by `…RestartKeepsARecordOfAnotherServer`, `…RestartReportsAnInvocationWithoutAnOutcomeAsUnknown` |
| `MF3-1` | `budget_usage.go` | two processes get the same start identity | killed by `…JournalOpen`, `…PassAfterTheDaemonStoppedHandlesNothing`, `…RebuiltBatchAnsweredWithOperationConflictIsDropped` and 7 more |
| `MF4-1` | `budget_usage_delivery.go` | a removal leaves the attempt in the table of running attempts | killed by `…AnswerDecidesWhatBecomesOfTheRecord`, `…AnsweredRecordIsNotSentAgainWhenItsFileCannotBeRemoved`, `…BatchOfARunThatOutlastedTheCapIsSentOnce` and 6 more |
| `MF4-2` | `budget_usage_delivery.go` | load lets go of the journal lock before it reads the record file | survived |
| `MF4-3` | `budget_usage.go` | begin lets go of the journal lock while it writes the record file (file without an entry in the table) | survived |
| `MF6-1` | `budget_usage_recovery.go` | no pass at start | killed by `…LoopPassesAtStartAndThenAtEveryTick` |
| `MF6-3` | `budget_usage_recovery.go` | a pass goes on after the daemon was told to stop | killed by `…PassAfterTheDaemonStoppedHandlesNothing` |
| `MF6-4` | `budget_usage_recovery.go` | a daemon without a journal keeps a loop | killed by `…LoopPassesAtStartAndThenAtEveryTick` |
| `MF6-6` | `budget_usage_recovery.go` | one pass at start and none after | killed by `…LoopPassesAtStartAndThenAtEveryTick` |
| `MF6-7` | `budget_usage_recovery.go` | a pass every 30 ms instead of every 30 s | survived |
| `MF6-8` | `budget_usage_recovery.go` | the cap is eight days | killed by `…RecordOlderThanSevenDaysIsDroppedWithAWarning` |
| `MF6-9` | `budget_usage_recovery.go` | the loop does not end with its context (a goroutine is left behind) | killed (by the test timeout) by `…LoopPassesAtStartAndThenAtEveryTick` |
| `MF6-10` | `budget_usage_recovery.go` | the ticker is not stopped | survived |
| `MF7-1` | `budget_usage_recovery.go` | a readable record is measured from its file's modification time | killed by `…RecordOlderThanSevenDaysIsDroppedWithAWarning` |
| `MF7-2` | `budget_usage.go` | the seven days start again at the seal | killed by `…BatchOfARunThatOutlastedTheCapIsSentOnce`, `…HandleTaskRecordsAndSeals`, `…RecordWriteAndReadBack` |
| `MF7-3` | `budget_usage.go` | the seven days start again at every outcome (a rewrite moves the time the cap is measured from) | killed by `…BatchOfARunThatOutlastedTheCapIsSentOnce`, `…HandleTaskRecordsAndSeals`, `…RecordWriteAndReadBack` |
| `MF-load` | `budget_usage_delivery.go` | the early return of load removed | survived |
| `MF-sort` | `budget_usage_recovery.go` | the sort of the record ids removed | survived |
| `MF-intent` | `budget_usage.go` | one operand in the launch-intent removal | survived |
| `MF-close` | `budget_usage_delivery.go` | the body of an answer is not closed | survived |
| `MF-bg` | `budget_usage_delivery.go` | journal.background does not count the sends (waited on by tests only) | killed by `…BatchOfARunThatOutlastedTheCapIsSentOnce`, `…HeldOpenReportDoesNotDelayOrChangeTheTerminalReport`, `…PassDoesNotSendARecordThatIsBeingSent`, `…RecoveryLeavesARunningAttemptAlone` |
| `MG3-1` | `budget_usage_recovery.go` | a rebuilt batch goes under another key than the record's | killed by `…PassAfterTheDaemonStoppedHandlesNothing`, `…RebuiltBatchAnsweredWithOperationConflictIsDropped`, `…RecoveryDoesNotSendABatchItCouldNotPersist` and 3 more |
| `MG3-2` | `budget_usage_batch.go` | an invocation without an outcome is reported as returned | killed by `…BatchEnvelope`, `…BatchGolden`, `…RestartReportsAnInvocationWithoutAnOutcomeAsUnknown`, `…SealNamesTheBuildingProcess` |
| `MG3-3` | `budget_usage_batch.go` | an invocation without an outcome is left out of the batch | killed by `…BatchEnvelope`, `…BatchGolden`, `…RestartReportsAnInvocationWithoutAnOutcomeAsUnknown`, `…SealNamesTheBuildingProcess` |
| `MG3-4` | `budget_usage_capture.go` | a task of an older server (no budget_attempt_id) gets a record and a report | killed by `…HandleTaskWithoutAttemptOrJournal`, `…RunTaskWithoutTheProtocol` |
| `MG5-1` | `budget_usage_delivery.go` | the send that follows the seal is made twice in a row (two requests under one key, same bytes) | killed by `…BatchOfARunThatOutlastedTheCapIsSentOnce`, `…HandleTaskRecordsAndSeals`, `…RunTaskFreshSessionRetryIsTwoInvocations` and 2 more |
| `MG5-2b` | `daemon.go`, `client.go` | a run that used nothing is reported on the legacy usage route of its task (both guards removed) | killed by `…RunTaskLaunchThatCannotStart` |
| `MG5-3b` | `daemon.go`, `client.go` | a run that used nothing is reported on the legacy usage route of ANOTHER task (what the narrowed check of LaunchThatCannotStart no longer sees) | killed by `…NotFoundAnswerOnlyDropsTheRecord` |

Read with the table:

- `MF6-9` is a kill by the test timeout, which the mutant's run had set to
  120 s: the loop subtest waits for a loop that never ends
  (`panic: test timed out after 2m0s`).
- `MF1-11` (line 170) ran with my reproduction of F7a in the package; the
  two tests of the lane kill it without mine.
- `MG5-2b` and `MG5-3b` replace text in two files each (`daemon.go` and
  `client.go`), because the empty usage report is refused twice, in
  `handleTask` and in `ReportTaskUsage`, and one guard alone hides the
  other. The first forms, which touched one file, are in section 5.7.
- `MF4-3` is `KD2-12`, `MF6-7` is `KD2-08`, `MF-sort` is `KD2-07`. `MF4-2`,
  `MF-load`, `MF-intent`, `MF6-10` and `MF-close` change nothing that can be
  observed (section 3.5).

### 5.5 Survivors run again against my tests

Each line repeats the change of a mutant of 5.2 to 5.4 and runs only tests
of mine on it, to show which of them fails. The tests run: the two of the
bound file for `M9-a-k` to `M8-4-k`; all eleven tests of my first three
files for `ME3-5-k` to `M3-3-k` (so each pin is also shown to pass on the
mutants of the others); the begin-lock test for the four `MF4-…` lines;
the lane's `…RunTaskLaunchThatCannotStart` for the last two. `M9-d-k` is
the one change here that has no earlier run: the per-call copy of the
client gives up its timeout, a second form of `M9-a`. A line that says
"expected to pass" is the opposite control: a change my test must not fail
on.

| Id | File | Change | Result |
| --- | --- | --- | --- |
| `M9-a-k` | `budget_usage_delivery.go` | M9-a against the checker's bound test: the per-call client built without the timeout | killed by mine: `…ReportGivesUpByTheClientsOwnBound` |
| `M9-d-k` | `budget_usage_delivery.go` | the per-call copy gives up its timeout, against the checker's bound test | killed by mine: `…ReportGivesUpByTheClientsOwnBound` |
| `M9-b-k` | `budget_usage_delivery.go` | M9-b against the checker's bound test only (expected to pass: the client's bound still holds) | survived |
| `M8-4-k` | `budget_usage_delivery.go` | M8-4 against the checker's bound test only (expected to pass: the client's bound still holds) | survived |
| `ME3-5-k` | `budget_usage_delivery.go` | ME3-5 against the checker's tests: the lower bound of delivered moved to 100 | killed by mine: `…SwitchingProtocolsAnswerKeepsTheRecord`, `…VerdictOfEveryStatus` |
| `ME3-14-k` | `budget_usage_delivery.go` | ME3-14 against the checker's tests: a 412 is final | killed by mine: `…VerdictOfEveryStatus` |
| `ME3-15-k` | `budget_usage_delivery.go` | ME3-15 against the checker's tests: a 306 is delivered | killed by mine: `…VerdictOfEveryStatus` |
| `ME4-2-k` | `budget_usage_delivery.go` | ME4-2 against the checker's tests: following switched off on the shared client by a report | killed by mine: `…ReportLeavesTheDaemonsOtherRequestsFollowingRedirects` |
| `ME4-3-k` | `budget_usage_delivery.go` | ME4-3 against the checker's tests: following switched off on the shared client where the protocol is declared | killed by mine: `…ReportLeavesTheDaemonsOtherRequestsFollowingRedirects` |
| `ME7-6-k` | `budget_usage_delivery.go` | ME7-6 against the checker's tests: the rest of an answer is not read | killed by mine: `…LongAnswerThatEndsEarlyKeepsTheRecord` |
| `ME8-3-k` | `budget_usage_recovery.go` | ME8-3 against the checker's tests: the comparison ignores case | killed by mine: `…RestartKeepsARecordWhoseServerAddressIsSpelledDifferently` |
| `ME8-4-k` | `budget_usage_recovery.go` | ME8-4 against the checker's tests: the comparison ignores a trailing slash | killed by mine: `…RestartKeepsARecordWhoseServerAddressIsSpelledDifferently` |
| `MF-sort-k` | `budget_usage_recovery.go` | MF-sort against the checker's tests: the sort of the record ids removed | killed by mine: `…PassHandlesARecordThisProcessHoldsOnce` |
| `MF6-7-k` | `budget_usage_recovery.go` | MF6-7 against the checker's tests: a pass every 30 ms | killed by mine: `…ResendIntervalIsThirtySeconds` |
| `M3-3-k` | `budget_usage_delivery.go` | M3-3 against the checker's tests: a removal that succeeds later leaves the settled entry behind | killed by mine: `…SettledAttemptIsForgottenOnceItsFileIsRemoved` |
| `MF4-3-k` | `budget_usage.go` | MF4-3 against the checker's test: begin lets go of the journal lock while it writes the record file | killed by mine: `…RecordBeingCreatedIsNeverTakenForALeftover` |
| `MF4-3-kr` | `budget_usage.go` | MF4-3 against the checker's test, with -race | killed by mine: `…RecordBeingCreatedIsNeverTakenForALeftover` |
| `MF4-2-k` | `budget_usage_delivery.go` | MF4-2 against the checker's test: load lets go of the journal lock before it reads the record file (expected to pass: claim waits for begin) | survived |
| `MF4-2-kr` | `budget_usage_delivery.go` | MF4-2 against the checker's test, with -race | survived |
| `MG5-3c` | `daemon.go`, `client.go`, `budget_usage_runtask_unix_test.go` | MG5-3b against LaunchThatCannotStart with the legacy check widened as in XG5-fix (expected to fail) | killed by `…RunTaskLaunchThatCannotStart` |
| `MG5-3d` | `daemon.go`, `client.go` | MG5-3b against LaunchThatCannotStart as it is (expected to pass: the narrowed check does not see another task's path) | survived |

Read with the table:

- `M9-b-k` and `M8-4-k` pass, as they must: with a context that never ends
  or with the old 15 s bound around the call, the client's own bound still
  ends the report. My bound test proves the client's bound and says nothing
  about the context; the lane's `…SendEndsWithTheDaemonAndKeepsTheRecord`
  has the context (`M9-b`, section 5.3).
- `MF4-2-k` and `MF4-2-kr` pass: the change is not observable (section 3.5).
- `MG5-3c` and `MG5-3d` are the pair behind `KD2-17`. The change is the one
  of `MG5-3b`. The test as the lane left it passes on it (`MG5-3d`); the
  same test with the legacy check over every legacy request that ends in
  `/usage` fails on it (`MG5-3c`, whose overlay replaces the test file too,
  by the copy of `XG5-fix`).

### 5.6 Overlays that are not mutants

Four runs feed the compiler a change I propose, to show that the smallest
change of a finding does what the finding says. None of them is in the
tree. In `results.tsv` "killed" means only that the run had a failing test
and "survived" that it had none.

| Id | File | Change | Result |
| --- | --- | --- | --- |
| `XF7a-fix` | `budget_usage.go`, `budget_usage_delivery.go` | NOT A MUTANT: the smallest change of the F7a finding (an untried batch is sent by whoever handles it first); all budget tests | killed by `…LoopPassesAtStartAndThenAtEveryTick`; mine: `…CheckOutlastedBatchIsDroppedUnsentWhenAPassComesFirst` |
| `XF7a-fix-r` | `budget_usage.go`, `budget_usage_delivery.go` | NOT A MUTANT: the same change with -race | killed by mine: `…CheckOutlastedBatchIsDroppedUnsentWhenAPassComesFirst` |
| `XLOOP-fix` | `budget_usage_recovery_test.go` | NOT A MUTANT: the lane's subtest 'stops with the daemon' with the requests counted where the daemon starts them | survived |
| `XG5-fix` | `budget_usage_runtask_unix_test.go` | G5 overlay, not a mutant: the legacy check of LaunchThatCannotStart over every legacy request ending in /usage (expected to pass) | survived |

Read with the table:

- `XF7a-fix` and `XF7a-fix-r` are the smallest change of `KD2-06` (six
  lines in two files: a `tried` flag on the attempt, set by the first send
  and asked for by the cap of a pass). The failing test is my reproduction,
  and its failing is the result wanted: both subtests now say "1 usage
  reports reached the server" and "warnings = []" at the assertions marked
  `FLIP`, so the batch that the code as checked drops unsent is sent once.
  All 78 tests of the lane pass under the change in the `-race` run. In the
  plain run one of them failed, the loop subtest "stops with the daemon" at
  `budget_usage_recovery_test.go:1326`: that is the timing-dependent
  failure of `KD2-11`, which the unchanged tree shows in 13 of 200 runs
  alone (section 6.2), not an effect of the change.
- `XLOOP-fix` is the smallest change of `KD2-11`, in the lane's test file:
  the subtest counts the requests where the daemon starts them (a counting
  `RoundTripper`) and not where the fake server registers them. One run
  here, 200 of 200 in `loop-overlay-200` (section 6.2).
- `XG5-fix` is the widened legacy check of `KD2-17` (one line of the lane's
  test). It passes on the code as checked; `MG5-3c` (section 5.5) shows
  what it catches.

### 5.7 Runs that prove nothing (kept so that `results.tsv` is accounted for)

Twelve lines of `results.tsv` are not results. Each was run again in a form
that is one, named in the last column here.

| Id | What went wrong | Replaced by |
| --- | --- | --- |
| `MA5-1` to `MA5-7` | false survivors: the wiring test parses `daemon.go` from disk, so an overlay of `daemon.go` alone never reaches it (the paragraph at the head of this section) | `MA5-1w` to `MA5-7w` (5.1), all killed |
| `MA1-6` | does not compile (`declared and not used: journal`) | `MA1-6b` (5.1), killed |
| `ME1-7` | does not compile (`"net/url" imported and not used`) | `ME1-7b` (5.2), survived |
| `ME8-5` | error of my spec: the text to replace was not found, nothing was run | `ME8-5b` (5.2), killed |
| `MG5-2` | removed one of the two guards against an empty usage report; the other (`ReportTaskUsage`, `client.go:558-565`) hid the change, so it "survived" | `MG5-2b` (5.4), killed |
| `MG5-3` | changed the path of every legacy usage report, not only of an empty one, so the four tests that fail say nothing about the narrowed check | `MG5-3b` (5.4), killed; `MG5-3c`, `MG5-3d` (5.5) |

The lines as `results.tsv` has them:

| Id | File | Change | Result |
| --- | --- | --- | --- |
| `MA5-1` | `daemon.go` | the loop started before the journal is initialized | survived |
| `MA5-2` | `daemon.go` | the loop started after the poll loop | survived |
| `MA5-3` | `daemon.go` | the loop call moved into a branch that is not taken | survived |
| `MA5-4` | `daemon.go` | the journal opened before the health port is bound | survived |
| `MA5-5` | `daemon.go` | the journal opened after the authentication preflight | survived |
| `MA5-6` | `daemon.go` | the loop is never started | survived |
| `MA5-7` | `daemon.go` | the journal call moved into a branch that is not taken | survived |
| `MA1-6` | `budget_usage_delivery.go` | declared, and the journal never installed (attempts believed unreported) | does not compile |
| `ME1-7` | `budget_usage_delivery.go` | the path escape removed | does not compile |
| `ME8-5` | ? | ME8-5: text not found: '\tif journal.outlived(record.CreatedAt) {\n\t\td.dropOutlivedBudgetUsage(journal, record)\n\t\treturn\n\t}\n\tif record.ServerURL != journal.serverURL {\n\t\td.logger.Debug("budget usage: keeping a usage report that belongs to another server", "attempt_id", record.AttemptID)\n\t\treturn\n\t}\n' | SPEC ERROR |
| `MG5-2` | `daemon.go` | the legacy usage report is made even when the run used nothing | survived |
| `MG5-3` | `daemon.go` | the legacy usage report is made even when the run used nothing, and under another task's path (what the narrowed check of LaunchThatCannotStart no longer sees) | killed by `…HandleTaskRecordsAndSeals`, `…HandleTaskWithoutAttemptOrJournal`, `…RunTaskFreshSessionRetryIsTwoInvocations`, `…RunTaskWithoutTheProtocol` |

### 5.8 Totals

`results.tsv` has 185 lines:

| Kind | Lines | Where |
| --- | --- | --- |
| controls (`C00` to `C03`), all pass | 4 | the head of this section |
| overlays that are not mutants | 4 | 5.6 |
| runs that prove nothing | 12 | 5.7 |
| an earlier change run again, against other tests or with `-race` (`ME4-2r`, `ME4-3r` and the lines of 5.5 other than `M9-d-k`) | 22 | 5.2, 5.5 |
| distinct mutants | 143 | 5.1 to 5.5 |

By what the line says, over all 185: 137 `killed`, 1 `killed (timeout)`
(`MF6-9`), 44 `survived` (the four controls and two of the overlays among
them), 2 `does not compile`, 1 spec error.

Of the 143 distinct mutants, 117 are killed by at least one test of the
lane. The 26 that are not:

| How many | Which | What it means |
| --- | --- | --- |
| 14 | `ME3-5`, `ME3-14`, `ME3-15` (`KD2-03`); `ME4-2`, `ME4-3` (`KD2-05`); `ME7-6` (`KD2-04`); `ME8-3`, `ME8-4` (`KD2-02`); `M3-3` (`KD2-09`); `MF-sort` (`KD2-07`); `MF6-7` (`KD2-08`); `M9-a`, `M9-c` (`KD2-01`); `MF4-3` (`KD2-12`) | behaviour changes and no test of the lane fails. Each is killed by a test of mine (5.5; `M9-c` in 5.3). `ME4-2` is the one the lane's tests do catch under `-race`, as a data race report (`ME4-2r`) |
| 7 | `ME1-7b`, `ME1-8`, `MF-load`, `MF-intent`, `MF6-10`, `MF-close`, `MF4-2` | nothing observable changes (section 3.5) |
| 4 | `M8-1`, `M8-2`, `M8-3`, `M8-4` | the old code of stage F, put back as brief 5.1 item 8 asks (section 3.4 line 8) |
| 1 | `M9-d-k` | run against my bound test only; what the lane's tests do on it was not run (by reading, what they do on `M9-a`: none of them holds an answer open) |

Scripts, all in `/tmp/f3-s6kd2/`: the runner `mut.py`; the series
`spec01.py` to `spec13.py` (the fifth is two files, `spec05a.py` and
`spec05b.py`), each a list of
ids with the file, the text replaced, its replacement, the tests and a
note; `check_spec.py` (a dry run of a series: every text to replace is
found exactly once); `gen5.py` and `tab.py` (the tables of this section
from `results.tsv`).

## 6. Runs

All runs from `server/` of the worktree, with
`TMPDIR='/private/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/'`, output
redirected to a file, exit code recorded, counts taken with
`grep -c -E '^\s*--- PASS'` (every level) and `'^--- PASS'` (top level).
Runner: `/tmp/f3-s6kd2/run_one.sh`; one log per run in `/tmp/f3-s6kd2/runs/`,
one line per run in `/tmp/f3-s6kd2/runs/summary.txt`. "default" is go1.27.0;
"go1.26.9" is `GOTOOLCHAIN=go1.26.9 GOPROXY=off`.

### 6.1 Before anything of mine was in the tree (H1, H2 "before")

| Command | Toolchain | Exit | PASS top / all | FAIL | SKIP |
| --- | --- | --- | --- | --- | --- |
| `go build ./...` | go1.27.0 | 0 | n/a | n/a | n/a |
| `go vet ./internal/daemon/` | go1.27.0 | 0 | n/a | n/a | n/a |
| `go test ./internal/daemon/ -run '^TestBudgetUsage' -count=1 -v` | go1.27.0 | 0 | 78 / 280 | 0 | 0 |
| the same with `-race` | go1.27.0 | 0 | 78 / 280 | 0 | 0 |
| `go build ./...` | go1.26.9 | 0 | n/a | n/a | n/a |
| `go vet ./internal/daemon/` | go1.26.9 | 0 | n/a | n/a | n/a |
| `go test ./internal/daemon/ -run '^TestBudgetUsage' -count=1 -v` | go1.26.9 | 0 | 78 / 280 | 0 | 0 |
| the same with `-race` | go1.26.9 | 0 | 78 / 280 | 0 | 0 |

The counts equal the ones the main session measured (78 / 280 on both
toolchains, with and without `-race`). No count differs.

### 6.2 Mutant series and the runs at the end

**The other runs**, in the order they were made. All are of
`./internal/daemon/` and of no other package. Counts are top-level / every
level. No run has a SKIP.

| Label (log in `/tmp/f3-s6kd2/runs/`) | Toolchain | Command | Exit | PASS | FAIL | Time |
| --- | --- | --- | --- | --- | --- | --- |
| `bound-alone-1` | go1.27.0 | `go test ./internal/daemon/ -run` of my two bound tests, `-count=1 -v` | 0 | 2 / 4 | 0 | 4 s |
| `vet-mine-1` | go1.27.0 | `go vet ./internal/daemon/` with my first three files in the tree | 0 | n/a | n/a | (empty log) |
| `mine-alone-1` | go1.27.0 | `go test ./internal/daemon/ -run` of the eleven tests of my first three files, `-count=1 -v` | 0 | 11 / 17 | 0 | 5 s |
| `loop-alone-200` | go1.27.0 | `go test ./internal/daemon/ -run '^TestBudgetUsageLoopPassesAtStartAndThenAtEveryTick$' -count=200 -v` | 1 | 187 / 974 | 13 / 26, each at `budget_usage_recovery_test.go:1326` | 129 s |
| `loop-overlay-200` | go1.27.0 | the same with `-vet=off -overlay /tmp/f3-s6kd2/m/XLOOP-fix/overlay.json` | 0 | 200 / 1000 | 0 | 243 s |
| `f5-race-10` | go1.27.0 | `go test -race ./internal/daemon/ -run '^TestBudgetUsage(…)$' -count=10 -v`, the twelve tests listed below | 0 | 120 / 190 | 0; no data race | 29 s |
| `begin-lock-alone-50` | go1.27.0 | `go test ./internal/daemon/ -run '^TestBudgetUsageRecordBeingCreatedIsNeverTakenForALeftover$' -count=50 -v` | 0 | 50 / 50 | 0 | 17 s |
| `begin-lock-mf43-50` | go1.27.0 | the same with `-vet=off -overlay /tmp/f3-s6kd2/m/MF4-3/overlay.json` | 1 | 0 | 50 / 50 | 16 s |
| `build-default-after` | go1.27.0 | `go build ./...` | 0 | n/a | n/a | 11 s |
| `vet-daemon-default-after` | go1.27.0 | `go vet ./internal/daemon/` | 0 | n/a | n/a | 1 s |
| `test-budget-default-after` | go1.27.0 | `go test ./internal/daemon/ -run '^TestBudgetUsage' -count=1 -v` | 0 | 90 / 298 | 0 | 27 s |
| `test-budget-race-default-after` | go1.27.0 | the same with `-race` | 0 | 90 / 298 | 0; no data race | 35 s |
| `build-go1.26.9-after` | go1.26.9 | `go build ./...` | 0 | n/a | n/a | 13 s |
| `vet-daemon-go1.26.9-after` | go1.26.9 | `go vet ./internal/daemon/` | 0 | n/a | n/a | 1 s |
| `test-budget-go1.26.9-after` | go1.26.9 | `go test ./internal/daemon/ -run '^TestBudgetUsage' -count=1 -v` | 0 | 90 / 298 | 0 | 32 s |
| `test-budget-race-go1.26.9-after` | go1.26.9 | the same with `-race` | 0 | 90 / 298 | 0; no data race | 38 s |
| `guard-budget-default-after` | go1.27.0 | from the root of the worktree: `bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test ./internal/daemon/ -run '^TestBudgetUsage' -count=1 -v` (`/tmp/f3-s6kd2/run_guard.sh`) | 0 | 90 / 298 | 0; no "unexpected agent CLI" line | 41 s |
| `vet-daemon-windows-after` | go1.27.0 | `env GOPROXY=off GOOS=windows GOARCH=amd64 go vet ./internal/daemon/`, my four files in the tree | 0 | n/a | n/a (empty log) | 16 s |
| `vet-daemon-linux-after` | go1.27.0 | `env GOPROXY=off GOOS=linux GOARCH=amd64 go vet ./internal/daemon/`, the same tree | 0 | n/a | n/a (empty log) | 14 s |

`go build ./...` compiles every package of `server/` and runs nothing. It
is the one command here that is not limited to the daemon package, and it
is the command the brief names.

The twelve tests of `f5-race-10` (prefix `TestBudgetUsage`):
`RecordIsNeverSentTwiceAtOnce`, `PassDoesNotSendARecordThatIsBeingSent`,
`HeldOpenReportDoesNotDelayOrChangeTheTerminalReport`,
`KeptBatchOfThisProcessIsResentFromMemory`,
`SendAfterTheSealHandlesOnlyWhatThisProcessHolds`,
`SendEndsWithTheDaemonAndKeepsTheRecord`,
`RecoveryLeavesARunningAttemptAlone`,
`BatchOfARunThatOutlastedTheCapIsSentOnce`,
`CapabilityReachesManagedWorkspaceClients`,
`ManagedInstallationSendsThroughTheWorkspaceOfTheRecord`, and two of mine,
`CheckOutlastedBatchIsDroppedUnsentWhenAPassComesFirst` and
`PassHandlesARecordThisProcessHoldsOnce`.

The timing-dependent failure and its second run, as section 7 of the brief
asks. `loop-alone-200` is the lane's loop test alone, 200 times, on the
unchanged tree, while other lanes ran suites on this machine (load average
17 to 24): 13 of the 200 fail, every one in the subtest "stops with the
daemon" at `budget_usage_recovery_test.go:1326` ("usage reports went out
after the loop ended"), and nowhere else. `loop-overlay-200` is the same
200 runs with the subtest counting requests where the daemon starts them:
none fails. So the failure is the test's and not the loop's (`KD2-11`).

**The mutant runs.** One `go test -vet=off -overlay <overlay.json> -run
'<tests>' -count=1 -timeout <t> ./internal/daemon/` per line of
`/tmp/f3-s6kd2/m/results.tsv`, all on go1.27.0, with `-race` for `ME4-2r`,
`ME4-3r`, `XF7a-fix-r`, `MF4-3-kr` and `MF4-2-kr`. The timeout is 240 s,
600 s for three of the `-race` runs and 120 s for `MF6-9`. The exit code of
each run is the third column of its line, the seconds the fourth, the
failing tests of every level the fifth, the `-run` pattern the sixth; the
whole output is `/tmp/f3-s6kd2/m/<id>/out.log`. These runs are not verbose,
so they have no PASS count: a line says which tests failed, and every other
test the pattern selects passed. No mutant run has a SKIP line.

| Series | Lines of `results.tsv` | Ids | When (2026-11-20) | Runs | Seconds of `go test` |
| --- | --- | --- | --- | --- | --- |
| `spec01` | 1 to 26 | `C00`, `MA1-1` to `MA1-6`, `MA5-1` to `MA5-7`, `ME3-1` to `ME3-12` | 01:10 to 01:22 | 26 | 731 |
| `spec02` | 27 to 35 | `C01`, `MA5-1w` to `MA5-7w`, `MA1-6b` | 01:26 to 01:27 | 9 | 58 |
| `spec03` | 36 to 74 | `C02`, `ME1-1` to `ME9-1` without `ME3-…` | 01:33 to 01:51 | 39 | 1070 |
| `spec04` | 75 to 134 | `C03`, `ME1-7b`, `ME8-5b`, `M2-1` to `M9-b`, `MF1-1` to `MF6-8`, `MG3-1` to `MG3-4` | 03:01 to 03:28 | 60 | 1627 |
| `spec05a` | 135 to 145 | `ME3-13` to `ME3-15`, `ME4-2r`, `MF7-1` to `MF7-3`, `MF6-9`, `MF6-10`, `MF1-8`, `MF1-9` | 03:59 to 04:05 | 11 | 370 |
| `spec05b` | 146, 147 | `ME4-3`, `ME4-3r` | 04:22 to 04:23 | 2 | 55 |
| `spec06` | 148 to 152 | `M9-a-k`, `M9-d-k`, `M9-c`, `M9-b-k`, `M8-4-k` | 04:53 to 04:54 | 5 | 106 |
| `spec07` | 153 to 165 | `ME3-5-k` to `M3-3-k`, `XF7a-fix`, `XF7a-fix-r` | 05:52 to 05:55 | 13 | 140 |
| `spec08` | 166 | `XLOOP-fix` | 07:03 to 07:04 | 1 | 18 |
| `spec09` | 167 to 169 | `MF1-10`, `MF-close`, `MF-bg` | 08:28 to 08:30 | 3 | 122 |
| `spec10` | 170 to 173 | `MF1-11`, `MF1-12`, `MF4-2`, `MF4-3` | 11:01 to 11:03 | 4 | 114 |
| `spec11` | 174 to 177 | `MF4-3-k`, `MF4-3-kr`, `MF4-2-k`, `MF4-2-kr` | 12:37 | 4 | 26 |
| `spec12` | 178 to 180 | `MG5-1`, `MG5-2`, `MG5-3` | 17:18 to 17:20 | 3 | 96 |
| `spec13` | 181 to 185 | `MG5-2b`, `MG5-3b`, `XG5-fix`, `MG5-3c`, `MG5-3d` | 17:22 to 17:23 | 5 | 69 |

185 runs, 4,602 seconds of `go test`. By exit code: 0 for the 44 lines that
say `survived`, 1 for the 138 that say `killed` (one of them by the
timeout) and for the 2 that do not compile; `ME8-5` ran nothing.

**`DATABASE_URL`.** From 10:38 on 2026-11-20 both runners (`run_one.sh` and
`mut.py`) export
`DATABASE_URL='postgres://nodb:nodb@127.0.0.1:9/nodb?sslmode=disable&connect_timeout=2'`,
as the main session's rule of that day asks: the runs from `begin-lock-alone-50`
on in the first table and the lines from 170 on in the second. Every run
before that (the eight of 6.1, `bound-alone-1`, `vet-mine-1`,
`mine-alone-1`, `loop-alone-200`, `loop-overlay-200`, `f5-race-10` and the
lines 1 to 169 of `results.tsv`) had the variable unset. All of them are
runs of `./internal/daemon/`, which has no database code, and
`go build ./...`, which runs nothing; no test of another package was run at
any time, and nothing of mine opened a database connection.

**Not `go` commands.** `gofmt -l` over my four files prints nothing.
`shasum -a 256 -c /tmp/f3-main-s6k/base-r2/manifest.txt` from the root of
the worktree at 17:30 and again at 20:34, after the last run: 21 of 21
`OK` both times.

## 7. Test files added

Four files, all in `server/internal/daemon/` of the worktree, all new, all
named `…_check6kd2_test.go`; 837 lines, 12 top-level tests and 6 subtests.
`gofmt -l` prints nothing for them, `go vet ./internal/daemon/` exits 0 with
them in the tree, and they pass with and without `-race` on both toolchains
(the end runs of 6.2: 90 / 298 is the lane's 78 / 280 and these). They use
the lane's test helpers and no production seam that was not there; they
start no process, open no connection outside loopback and need no database.
No existing file was changed for them.

**The test of the 30 second bound** (`KD2-01`; the brief asks for it by
name): `budget_usage_bound_check6kd2_test.go`, 152 lines.

| Test | What it proves | Fails on |
| --- | --- | --- |
| `TestBudgetUsageReportGivesUpByTheClientsOwnBound`, subtests `no answer at all` and `an answer that stops in the middle of its body` | The client of the test daemon gets a `Timeout` of 1.5 s in place of the 30 s (`d.client.client.Timeout`, no production change). The fake server takes the report and holds it open: without any answer, or after a status line 200 and the first bytes of a body it never finishes. The report is made under `context.Background()`, so only the client's bound can end it. Asserted: the report returns, and not before 1.5 s have passed; exactly one request arrived; the record file is byte for byte what it was; the process still holds the record; no warning; and a second handling of the same record sends the same bytes again and is delivered, so the mark was released. A report that has not returned after 15 s fails the test with "nothing bounds one report" | `M9-a` (the mutant the brief names: the per-call client built as `http.Client{Transport: c.client.Transport}` with the same `CheckRedirect`): run as `M9-a-k`, both subtests fail at the 15 s guard. Also `M9-d-k` (the per-call copy sets its `Timeout` to 0) |
| `TestBudgetUsageReportBoundIsThirtySeconds` | the number: `NewClient(…).client.Timeout` is 30 s. Accepted item 17 rests on it (a pass takes up to 32 times 30 s) | `M9-c` (300 s in `NewClient`), which all 78 tests of the lane pass |

The opposite controls: with the send under `context.Background()` (`M9-b-k`)
and with the old 15 s bound around the call (`M8-4-k`) the test passes, as
it must, because the client's bound still ends the report. The whole file
takes about 4 s.

**Pins for lines no test of the lane could tell from a changed one:**
`budget_usage_pins_check6kd2_test.go`, 460 lines, eight tests.

| Test | What it proves | Fails on | Finding |
| --- | --- | --- | --- |
| `TestBudgetUsageVerdictOfEveryStatus` | the rule of design 11.10 written once as a function of the status, and `verdict()` checked against it for every status from 0 to 999, with and without an error, and for an error without a status | `ME3-5-k` (delivered from 100 on), `ME3-14-k` (412 final), `ME3-15-k` (306 delivered) | `KD2-03` |
| `TestBudgetUsageSwitchingProtocolsAnswerKeepsTheRecord` | a real `101 Switching Protocols` over the wire reaches the verdict as a 101 (the kept line of the log says `status=101`) and keeps the record: file unchanged, no warning, the second send carries the same bytes | `ME3-5-k` | `KD2-03` |
| `TestBudgetUsageLongAnswerThatEndsEarlyKeepsTheRecord`, subtests `200` and `400` | an answer of more than 64 KiB that ends before its declared length, after the part the daemon keeps, keeps the record whatever its status line said | `ME7-6-k` (the copy of the rest removed) | `KD2-04` |
| `TestBudgetUsageRestartKeepsARecordWhoseServerAddressIsSpelledDifferently` | seven spellings of the daemon's own address (upper case in the host, upper case in the scheme, a trailing slash, the default port, a trailing dot, a path, `http`), a sealed and a running record left under each: two passes make no request at all, every file is byte for byte the same, no warning | `ME8-3-k` (the comparison ignores case), `ME8-4-k` (ignores a trailing slash) | `KD2-02` |
| `TestBudgetUsageReportLeavesTheDaemonsOtherRequestsFollowingRedirects` | a task status request of the same client that the server answers with a 302 arrives at the target: before the protocol is declared (the control), after it is declared, and after a usage report | `ME4-2-k` (following switched off on the shared client by a report), `ME4-3-k` (switched off where the protocol is declared) | `KD2-05` |
| `TestBudgetUsagePassHandlesARecordThisProcessHoldsOnce` | a record an earlier process left and, sorting after it, a record this process holds, against a server that answers 503: after each of three passes each attempt has exactly one more report, with its own bytes | `MF-sort-k` (the sort of the ids removed: two reports per pass for the held record) | `KD2-07` |
| `TestBudgetUsageResendIntervalIsThirtySeconds` | the number: `budgetUsageResendInterval` is 30 s | `MF6-7-k` (30 ms) | `KD2-08` |
| `TestBudgetUsageSettledAttemptIsForgottenOnceItsFileIsRemoved` | an answered record whose file could not be removed (the directory made read-only) is remembered as settled; once the directory allows it, the next pass removes the file and the entry, and no second report goes out. POSIX only, as the lane's tests of that case are | `M3-3-k` (the entry is left behind) | `KD2-09` |

Each of the eleven tests of the first three files was run against every
mutant of that series (section 5.5), so each pin is also shown to pass on
the mutants of the others.

**A reproduction, not a pin:** `budget_usage_outlasted_check6kd2_test.go`,
121 lines, for `KD2-06` (F7a).

| Test | What it shows |
| --- | --- |
| `TestBudgetUsageCheckOutlastedBatchIsDroppedUnsentWhenAPassComesFirst`, subtest `a pass between the seal and the send` | a run that lasted longer than seven days ends; `seal`, then one pass, then the send that follows the seal: no report reaches a server that would have accepted it, the one warning of the cap is written, the file and the entry are gone |
| the same, subtest `a pass while the seal is under way` | the same outcome through `finishBudgetUsageAttempt` itself: the pass is started from the journal's clock, which `seal` reads with the attempt locked, and has taken the mark before the seal goes on. It shows that the window is the write of the seal and not only the gap between two statements |

Both subtests pass on the code as checked, and that they pass is the
finding. The file says so at its top and is not meant to be merged as it
is: three assertions are marked `FLIP`, and under the smallest change of
the finding they turn over (`XF7a-fix`, `XF7a-fix-r`, section 5.6). It is
also killed by `MF1-11` (a pass never caps a held record), which is the
other way to make the batch go out.

**A stress test:** `budget_usage_begin_lock_check6kd2_test.go`, 104 lines,
for `KD2-12` (F4).

| Test | What it proves | Fails on |
| --- | --- | --- |
| `TestBudgetUsageRecordBeingCreatedIsNeverTakenForALeftover` | 24 records are created one after the other while two goroutines make passes without a pause; each new id sorts before the earlier ones, so a pass reaches the record being created first. Afterwards every attempt is still the one the journal holds, every file is there, no warning was written and no report went out | `MF4-3` (`begin` lets go of the journal's lock while it writes the record file): 50 of 50 runs fail (`begin-lock-mf43-50`), against 50 of 50 that pass on the tree (`begin-lock-alone-50`); also `MF4-3-k` and, with `-race`, `MF4-3-kr` |

It has no seam to make the order certain: it relies on a pass falling into
the two fsyncs of the write, which it did in every one of the 50 runs here,
on a machine that was under load. On a much faster disk it could pass on the
mutant. By reading, nothing it asserts can fail on correct code; its two
waits (for four more passes, and for the passes to end) have the 20 s
deadline of the lane's helpers. `MF4-2` (the lock of `load` let go before
the file is read) passes it, and should (section 3.5).

**For the merge.** The bound file is the one the brief plans to move into
the lane's files. The pins file and the begin-lock file can go the same way
as they are; each test names in its comment the change it fails on. The
outlasted file is evidence and goes only if `KD2-06` is settled in the
code, with its three `FLIP` assertions turned over.

## 8. Disagreements with decided items

(pending)

## 9. Not checked; hooks and reminders

(pending)
