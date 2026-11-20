# Step 7b: the callers of the budget HTTP API — implementation report

Lane: `f3-7b` (implementer). Brief: `/tmp/f3-main-s7b/brief-7b.md`. Design: `design.md` sections 12.2, 12.3, 12.6, 12.7, 12.9, 12.10.
Code: scratch worktree `/Volumes/artisan/code/2026/mc-f3-budget-7b-20261119` (detached at `86fea651c`, 56 dirty paths of earlier steps that are not this lane's). Nothing is committed.

This report is written stage by stage. Round 1 covers stages A to C (fixtures, money conversion, the commands). Round 2 covers three additions to the commands' tests, stage D (Core's schema and one API method) and stage E (the built-in skill's text). All five stages are built. One thing needs a ruling before anything else: **R5** in section 3, a test run of this lane that wrote to a database.

## 1. What was built

### Stage A: the fixture files

Directory `server/internal/handler/testdata/issue_budget_http_v1/`, 26 files. Each is one compact JSON document and one line feed, in the field order of design 12.9.

They are written by a script, `/tmp/f3-7b/gen_fixtures.py` (not in the tree): `python3 -I /tmp/f3-7b/gen_fixtures.py <directory>`. Before it writes anything the script checks every value against the spelling rules of 12.9 (decimal strings, JSON numbers, canonical UUIDs, UTC times) and the agreement rules inside one file (`total_known`, `remaining_ticks`, `unspent_ticks`, `overrun_ticks`, the nulls of an unavailable ledger), and it refuses to leave a file in the directory that it did not write.

| File | Content |
| --- | --- |
| `enable-request.json` | the example of 12.9, byte for byte |
| `operation-set-limit-request.json` | `limit_ticks` null, a reserve of 0.50 USD, expected revision 3 |
| `operation-set-mode-request.json` | mode `off`; the reason is Chinese text, so one fixture is not pure ASCII |
| `operation-close-request.json`, `operation-reopen-request.json` | the two requests without an amount or a mode |
| `write-201-enable.json` | the result example of 12.9 with full ids; all three enable-only fields |
| `write-201-enable-no-gaps.json` | a second root, no limit and no reserve, `member_count` 1, no `in_flight_gap_count`, an activation time without a fraction |
| `write-200-replayed-set-limit.json` | `replayed` true; the result of the set-limit request above (revision 4) |
| `operation-committed.json` | the enable result, `stale` false |
| `operation-committed-stale.json` | the set-limit result, `stale` true |
| `operation-unknown.json` | `retry_path` on the root route |
| `issue-binding.json` | a member bound by `create`, not the root issue |
| `show-open.json` | the example of 12.9 with full ids |
| `show-known.json` | limit 25.00 USD, nothing unknown, `total_known` true, `remaining_ticks` 166500000000 = 250000000000 − (73500000000 + 10000000000 + 0); a non-zero overrun |
| `show-no-limit.json` | the second root: no limit, no reserve, nothing unknown, `remaining_ticks` null |
| `show-unavailable.json` | `amounts` null, `remaining_ticks` null, `total_known` false, `included_attempts` null; `by_reason` holds two gap reasons of the observation table |
| `show-tombstoned-off.json` | `tombstoned`, `off`, revision 4; amounts and unknown set equal to `show-open.json` |
| `observations-page.json` | `would_refuse`, `attribution_difference` (empty `detail`), `coverage_gap` (covered); a cursor |
| `observations-last-page.json` | two gap rows, one with every nullable field null; `next_cursor` null |
| `observations-empty.json` | `"observations":[]` |
| `attempts-page.json` | six rows: open and sized, unsized, held, quick-create (settled with an overrun), old daemon, no reservation; a cursor |
| `attempts-unavailable.json` | three rows, every `reservation` null, `next_cursor` null |
| `attempts-empty.json` | `"attempts":[]` |
| `error-budget-forbidden.json`, `error-not-found.json`, `error-policy-revision-conflict.json` | the error body |

Three files are more than the brief's table asks for: `operation-reopen-request.json` (so the bytes of all five write requests are pinned by a file), and the two empty pages. The empty pages matter to the server lane: a nil Go slice encodes as `null`, 12.9 gives no null for these arrays, and the commands here read a `null` list as "not understood". The same holds for `"by_reason":[]` in `show-known.json` and `show-no-limit.json`.

Choices made so that the server lane's decode-and-encode gives the same bytes: no string holds `<`, `>`, `&`, U+2028, U+2029 or a control character (Go's encoder would escape them and 12.9 does not say which way); no fractional second ends in 0 (`time.RFC3339Nano` trims it); every `detail` object is `{}` or has one key at its top level (`{"task_status":"running"}`, `{"policy_epoch":"1"}`, `{"by_reason":{…}}`). The one nested object, `{"lost_contact":1,"observation_off":2}`, has two keys in alphabetical order, which is the order Go gives a map and the order `jsonb` gives these two names; a struct with its fields declared the other way round would not give these bytes.

A second script reads the twelve JSON examples out of the text of `design.md` section 12.9 and compares them with the files: same keys in the same order at every depth, the same kind of value, and the same values wherever a pairing is exact (the enable request, the three other request examples, the result of `write-201-enable.json`, `show-open.json`). It does not share the generator's reading of the design.

```
$ python3 -I /tmp/f3-7b/check_fixtures_against_design.py <task>/design.md server/internal/handler/testdata/issue_budget_http_v1
26 files agree with the 12 examples of design 12.9
```

`/tmp/f3-7b/probe_checker.sh` tampers a copy of the files in seven ways (two keys swapped, a value changed, a number for a decimal string, a key renamed, a key missing, an extra key, an error body reordered); the checker exits 1 on each.

The Go check of the files is `server/cmd/multica/cmd_issue_budget_fixtures_test.go`. It reads the files into the structs of the commands, so it was written with stage C:

- `TestIssueBudgetFixtureFiles`: the directory holds exactly the 26 named files; each is one compact JSON document on one line, ends with one line feed, is valid UTF-8, has no key twice at any depth, and holds no key the structs of the commands do not know (strict reading at every depth). Each of the five requests is also encoded again from its struct and must give the bytes of the file, which pins the field order of the requests.
- `TestIssueBudgetFixtureCheckFindsARepeatedKey`: the walk that looks for a repeated key finds one at every depth, and reports none where the same name occurs in two different objects.
- `TestIssueBudgetStrictReadingReachesEveryDepth`: a key the structs do not know, put at each depth of an answer, is refused by the strict reading and ignored by the reading the commands use.

### Stage B: money on the command line

`server/cmd/multica/cmd_issue_budget_money.go`:

- `parseBudgetUSD(input string) (*budget.Ticks, error)`. The word `none` gives a nil amount, which is sent as JSON `null`. Anything else must be `0` or digits without a leading zero, optionally a point and 1 to 10 digits. The conversion pads the fraction to ten digits, joins it to the whole part and reads the digit string with `budget.ParseDecimal`; no arithmetic on a floating point value and no rounding happens anywhere. Three distinct refusals: the form, more than ten decimals, above the largest int64 number of ticks.
- `formatBudgetTicks(budget.Ticks) string`: `25.00 USD (250000000000 ticks)`, `0.0000000001 USD (1 ticks)`, `0.025 USD (250000000 ticks)`.
- `formatBudgetAmount(*budget.Ticks, absent string) string` with the two wordings of a null, `budgetNotSet` (`not set`) and `budgetUnknown` (`unknown`).

Tests in `cmd_issue_budget_money_test.go`: `TestIssueBudgetParseUSD`, `TestIssueBudgetFormatTicks`, `TestIssueBudgetUSDTextRoundTrips`, `TestIssueBudgetUSDDecimalsMatchTheTickScale`.

### Stage C: the commands

All in `server/cmd/multica/`:

| File | Lines | Content |
| --- | --- | --- |
| `cmd_issue_budget.go` | 961 | the command tree, the local checks, the one HTTP call, the three outcomes of a write, the reads |
| `cmd_issue_budget_wire.go` | 425 | the shapes of 12.9 and their reading with explicit presence |
| `cmd_issue_budget_print.go` | 382 | the tables |
| `cmd_issue.go` | +1 | `issueCmd.AddCommand(issueBudgetCmd)`, the only line changed in an existing file |
| `cmd_issue_budget_test.go` | 904 | the fake server and the runner; requests, people only, required flags, flag forms |
| `cmd_issue_budget_write_test.go` | 695 | the three outcomes of a write |
| `cmd_issue_budget_read_test.go` | 1295 | `show`, `operation`, the two lists, refused and unanswered reads |

`multica issue budget` has nine subcommands with the flags of the brief's table: `enable`, `set-limit`, `set-mode`, `close`, `reopen`, `show`, `operation`, `observations`, `attempts`. Every one takes `--output table|json`, default `table`.

**What every subcommand does first** (`budgetStart`), before a flag is looked at or anything is sent: `requireHumanLocalCommand("issue budget <sub>")`; then `newAPIClient`; then `budgetHumanCredential`, which refuses an empty token, a token that starts with `mat_`, `mdt_` or `mcn_`, and a client that carries an agent id or a task id; then the workspace must be a canonical lower-case UUID; then `--output` must be `table` or `json`.

**Then what was typed is checked**, in the form the server takes: `--root` and `--request-key` with `reportSnapshotUUID` (the snapshot command's check: canonical, lower case, not all zeros); `--expect-revision` with `budget.ParseDecimal`, greater than 0; `--limit` and `--attempt-reserve` with `parseBudgetUSD`; `--mode` must be `off` or `observe`; `--reason` must be valid UTF-8; `--yes` must be given; `enable` takes exactly one non-empty issue, `show` one issue or `--root`, `operation` `--root` or `--issue`; the lists take `--limit` 1 to 100 and a `--cursor` that is not empty. A flag of the table that is left out is an error. Nothing is defaulted and no key is made up.

**Then one request** through `budgetRequest` (section 5 says why it is a call of its own):

| Command | Request |
| --- | --- |
| `enable <issue>` | `POST /api/issues/{issue}/budget` |
| `set-limit`, `set-mode`, `close`, `reopen` | `POST /api/issue-budgets/{root}/operations` |
| `show --root` | `GET /api/issue-budgets/{root}` |
| `show <issue>` | `GET /api/issues/{issue}/budget`, then `GET /api/issue-budgets/{root}` for the root the first answer names |
| `operation --root` | `GET /api/issue-budgets/{root}/operations/{key}` |
| `operation --issue` | `GET /api/issues/{issue}/budget/operations/{key}` |
| `observations`, `attempts` | `GET /api/issue-budgets/{root}/observations` and `…/attempts`, with `?cursor=…&limit=…` when they were typed |

It carries `Authorization: Bearer <token>`, `X-Workspace-ID`, the three `X-Client-*` headers when they are set, and `Content-Type: application/json` on a write. It never carries `X-Agent-ID`, `X-Task-ID` or an `Idempotency-Key`. The body of a write is `json.Marshal` of a struct whose fields are in the order of 12.9; `expected_policy_revision` is a `budget.Count` and the two amounts are `*budget.Ticks`, so they are sent as decimal strings or `null`.

**Then the answer is read** by `understandBudgetAnswer` (`cmd_issue_budget_wire.go`). Each field is declared as `budgetRequired[T]`, `budgetNullable[T]` or `budgetOptional[T]` (only the three enable-only fields are optional), and the reading records for each field whether it was there, whether it was null and whether it was there twice. A field that 12.9 requires and that is missing or null, a nullable field that is missing, a key that is there twice, and a value in another form (a JSON number where a decimal string belongs, a string where a number belongs, a `detail` that is not an object) make the answer "not understood", with the path of the field: `field "amounts.reserved_ticks" is missing`. Amounts and revisions are `budget.Ticks` and `budget.Count`; counts and `protocol_version` are read from their raw bytes with `budget.ParseDecimal`. There is no `float64`, no `interface{}` decode and no `json:",string"` tag in the three files. A key that 12.9 does not name is ignored at every depth; a value of an enum that the command does not know is printed as it came.

**How a caller tells the outcomes apart.** stdout holds something only on success. The error is printed by `main` through `cli.FormatError`, and the exit code is `cli.ExitCodeFor`'s:

| Outcome | stderr starts with | Exit code |
| --- | --- | --- |
| write committed (201, or 200 with `replayed` true) | nothing; stdout starts with `committed: ` | 0 |
| write or read refused by the server | `refused (<code>): ` or `refused (401): ` | 3 for a 401 or 403, 4 for a 404, 5 for a 400 or 422, 1 for a 409 |
| write with an unknown outcome | `outcome unknown: ` | 1, whatever the status was |
| read answered by something that is not the budget API | `the server answered HTTP <n>, which is not an answer of the budget API; nothing was read` | by status as for a refusal, otherwise 1 |
| read whose answer is not understood | `the answer of the server was not understood: ` | 1 |
| read answered with more than 1 MiB | `the server answered HTTP <n> with more than this command reads` | 1 |
| read without an answer | `no answer came back from the server, or not a whole one; nothing was read.` | 2 |
| `show <issue>`, a 404 on the binding | `issue <as typed> belongs to no budget root, or is not visible in this workspace` | 4 |
| refused before anything was sent (not a person, a flag) | the reason, for example `--yes is required: it confirms that this request may be sent` | 1 |

So a refusal and an unknown outcome are told apart by the first words of stderr, not by the exit code alone: a 409 refusal and an unknown outcome both exit 1. A dedicated exit code needs a change outside this lane's files (section 3).

**Examples.** Each block is one run against the fake server of the tests with a fixture as the answer: the command as typed, the exit code, stdout and stderr as `main` prints them.

*Writes.* `enable`, committed now (answer: `write-201-enable.json`):

```text
$ multica issue budget enable MUL-12 --request-key 3f0e6f0a-6f1c-4c55-9d1e-0f2b7c1a9e10 --reason 'Q4 launch' --limit 25 --attempt-reserve none --yes
[exit 0]
[stdout]
committed: the operation was written now
OPERATION                c4e1a2f7-08b3-4b9d-a6c5-3e9f1d2b7a80
KIND                     enable
BUDGET ROOT              7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f
ROOT ISSUE               0b7f4c1e-93a2-4d58-b6e1-52c8a7f0d314
STATE                    open
MODE                     observe
POLICY REVISION          1
POLICY EPOCH             1
LIMIT                    25.00 USD (250000000000 ticks)
ATTEMPT RESERVE          not set
REASON                   Q4 launch
ACTIVATED AT             2026-11-19T01:02:03.456789Z
ISSUES BOUND             4, the root issue included
IN FLIGHT AT ACTIVATION  1; already running when the root was enabled, so unknown from before activation and not a zero cost
```

When the answer leaves out `in_flight_gap_count` (`write-201-enable-no-gaps.json`, `member_count` 1), the last two lines are these. A count that 12.9 leaves out is 0, and a 0 gets no remark:

```text
ISSUES BOUND             1, the root issue included
IN FLIGHT AT ACTIVATION  0
```

`set-limit`, answered 200 with `replayed` true (`write-200-replayed-set-limit.json`):

```text
$ multica issue budget set-limit --root 7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f --request-key 9b2f7c4e-1d3a-4e6b-8f50-2c7d9e1a4b63 --reason 'reserve half a dollar per run, no cap yet' --expect-revision 3 --limit none --attempt-reserve 0.5 --yes
[exit 0]
[stdout]
committed: this request key had already committed this operation; nothing was written now
OPERATION        1a9d3c5e-7f2b-4086-b4d1-6e8c0a2f4b79
KIND             set_limit
BUDGET ROOT      7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f
ROOT ISSUE       0b7f4c1e-93a2-4d58-b6e1-52c8a7f0d314
STATE            open
MODE             observe
POLICY REVISION  4
POLICY EPOCH     1
LIMIT            not set
ATTEMPT RESERVE  0.50 USD (5000000000 ticks) for each execution; a declared reservation, not an estimate and not a cap
REASON           reserve half a dollar per run, no cap yet
```

`set-mode`, `close` and `reopen` print the same eleven lines. There is no fixture of their answers, so the three answers below are made up for this report from the set-limit fixture (the kind replaced, and the mode, the state, the revision and the reason): they show the lines, and are not a statement of what the server answers.

```text
$ multica issue budget set-mode --root 7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f --request-key 5c8e1f2a-7b4d-4a90-b3e6-0d1f2a3b4c5d --reason '迁移期间暂停观察' --expect-revision 3 --mode off --yes
[exit 0]
[stdout]
committed: the operation was written now
OPERATION        1a9d3c5e-7f2b-4086-b4d1-6e8c0a2f4b79
KIND             set_mode
BUDGET ROOT      7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f
ROOT ISSUE       0b7f4c1e-93a2-4d58-b6e1-52c8a7f0d314
STATE            open
MODE             off; nothing is recorded while observation is off
POLICY REVISION  4
POLICY EPOCH     1
LIMIT            not set
ATTEMPT RESERVE  0.50 USD (5000000000 ticks) for each execution; a declared reservation, not an estimate and not a cap
REASON           迁移期间暂停观察
```

```text
$ multica issue budget close --root 7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f --request-key e4a7d2c9-3b5f-4c18-9a6e-7f0b1c2d3e4f --reason 'work is done' --expect-revision 3 --yes
[exit 0]
[stdout]
committed: the operation was written now
OPERATION        1a9d3c5e-7f2b-4086-b4d1-6e8c0a2f4b79
KIND             close
BUDGET ROOT      7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f
ROOT ISSUE       0b7f4c1e-93a2-4d58-b6e1-52c8a7f0d314
STATE            closed
MODE             observe
POLICY REVISION  4
POLICY EPOCH     1
LIMIT            not set
ATTEMPT RESERVE  0.50 USD (5000000000 ticks) for each execution; a declared reservation, not an estimate and not a cap
REASON           work is done
```

```text
$ multica issue budget reopen --root 7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f --request-key 2d6b8f0c-4e1a-4d37-a59c-8b7e6f5a4d3c --reason 'one more round of fixes' --expect-revision 4 --yes
[exit 0]
[stdout]
committed: the operation was written now
OPERATION        1a9d3c5e-7f2b-4086-b4d1-6e8c0a2f4b79
KIND             reopen
BUDGET ROOT      7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f
ROOT ISSUE       0b7f4c1e-93a2-4d58-b6e1-52c8a7f0d314
STATE            open
MODE             observe
POLICY REVISION  5
POLICY EPOCH     1
LIMIT            not set
ATTEMPT RESERVE  0.50 USD (5000000000 ticks) for each execution; a declared reservation, not an estimate and not a cap
REASON           one more round of fixes
```

Refused: `policy_revision_conflict` (`error-policy-revision-conflict.json`, status 409), `budget_forbidden` (`error-budget-forbidden.json`, status 403), and a 401:

```text
$ multica issue budget close --root 7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f --request-key e4a7d2c9-3b5f-4c18-9a6e-7f0b1c2d3e4f --reason 'work is done' --expect-revision 3 --yes
[exit 1]
[stderr]
refused (policy_revision_conflict): the policy of this budget root has changed since the revision given with --expect-revision. Nothing was written.
Run "multica issue budget show --root 7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f" and decide again: this command does not look up the revision for you and does not send the request again.
```

```text
$ multica issue budget close --root 7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f --request-key e4a7d2c9-3b5f-4c18-9a6e-7f0b1c2d3e4f --reason 'work is done' --expect-revision 3 --yes
[exit 3]
[stderr]
refused (budget_forbidden): budget roots are for a person who is an owner or admin of the workspace and can see the root issue. Nothing was written.
```

```text
$ multica issue budget close --root 7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f --request-key e4a7d2c9-3b5f-4c18-9a6e-7f0b1c2d3e4f --reason 'work is done' --expect-revision 3 --yes
[exit 3]
[stderr]
refused (401): the server did not accept the credential; sign in with "multica login". Nothing was written.
```

Unknown outcome: a 502 with an HTML page, and a 201 whose body is cut after 40 bytes:

```text
$ multica issue budget set-limit --root 7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f --request-key 9b2f7c4e-1d3a-4e6b-8f50-2c7d9e1a4b63 --reason 'reserve half a dollar per run, no cap yet' --expect-revision 3 --limit none --attempt-reserve 0.5 --yes
[exit 1]
[stderr]
outcome unknown: the server answered HTTP 502, which does not say whether the operation was committed.
This command sent the request at most once and does not send it again; the operation may be committed or not.
Ask by the original key:
  multica issue budget operation --root 7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f --request-key 9b2f7c4e-1d3a-4e6b-8f50-2c7d9e1a4b63
Keep the original --request-key and the exact flags of this command: if the query answers "unknown", run this very same command again.
```

```text
$ multica issue budget close --root 7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f --request-key e4a7d2c9-3b5f-4c18-9a6e-7f0b1c2d3e4f --reason 'work is done' --expect-revision 3 --yes
[exit 1]
[stderr]
outcome unknown: the server answered HTTP 201, but its answer was not understood: it is not one JSON object in the form this command reads.
This command sent the request at most once and does not send it again; the operation may be committed or not.
Ask by the original key:
  multica issue budget operation --root 7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f --request-key e4a7d2c9-3b5f-4c18-9a6e-7f0b1c2d3e4f
Keep the original --request-key and the exact flags of this command: if the query answers "unknown", run this very same command again.
```

Refused before anything is sent:

```text
$ multica issue budget close --root 7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f --request-key e4a7d2c9-3b5f-4c18-9a6e-7f0b1c2d3e4f --reason 'work is done' --expect-revision 3
[exit 1]
[stderr]
--yes is required: it confirms that this request may be sent
```

```text
$ multica issue budget set-limit --root 7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f --request-key 9b2f7c4e-1d3a-4e6b-8f50-2c7d9e1a4b63 --reason x --expect-revision 3 --limit 1.00000000001 --attempt-reserve none --yes
[exit 1]
[stderr]
--limit has more than 10 digits after the point; one tick is 0.0000000001 USD and an amount is never rounded
```

*`show`.* By root, from `show-open.json`:

```text
$ multica issue budget show --root 7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f
[exit 0]
[stdout]
BUDGET ROOT                7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f
ROOT ISSUE                 0b7f4c1e-93a2-4d58-b6e1-52c8a7f0d314
STATE                      open
MODE                       observe
POLICY REVISION            3
POLICY EPOCH               1
LEDGER REVISION            42
ACTIVATED AT               2026-11-19T01:02:03.456789Z
SETTLED AS OF REVISION     not settled
LIMIT                      25.00 USD (250000000000 ticks)
ATTEMPT RESERVE            0.50 USD (5000000000 ticks) for each execution; a declared reservation, not an estimate and not a cap
LEDGER                     available
ACTUAL KNOWN (A)           7.35 USD (73500000000 ticks)
ESTIMATED (E)              0.12 USD (1200000000 ticks); outside A and outside the limit
RESERVED (R)               1.00 USD (10000000000 ticks)
UNKNOWN HOLD (H)           0.50 USD (5000000000 ticks)
OVERRUN                    0.00 USD (0 ticks)
UNKNOWN                    lost_contact: 1
UNKNOWN                    observation_off: 2
UNKNOWN BEFORE ACTIVATION  1
TOTAL KNOWN                no
REMAINING                  unknown; not everything is known
EXECUTIONS INCLUDED        12
NOT COUNTED                task_without_issue; not counted, which is not a zero cost
NOT COUNTED                collaboration_host_run; not counted, which is not a zero cost
NOT COUNTED                plugin_external_call; not counted, which is not a zero cost
```

From `show-unavailable.json`. No amount line holds a digit, and the remaining amount and the executions included are `unknown` as well:

```text
$ multica issue budget show --root 7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f
[exit 0]
[stdout]
BUDGET ROOT                7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f
ROOT ISSUE                 0b7f4c1e-93a2-4d58-b6e1-52c8a7f0d314
STATE                      open
MODE                       observe
POLICY REVISION            3
POLICY EPOCH               1
LEDGER REVISION            42
ACTIVATED AT               2026-11-19T01:02:03.456789Z
SETTLED AS OF REVISION     not settled
LIMIT                      25.00 USD (250000000000 ticks)
ATTEMPT RESERVE            0.50 USD (5000000000 ticks) for each execution; a declared reservation, not an estimate and not a cap
LEDGER                     unavailable; the ledger cannot be read, so every amount is unknown
ACTUAL KNOWN (A)           unknown
ESTIMATED (E)              unknown; outside A and outside the limit
RESERVED (R)               unknown
UNKNOWN HOLD (H)           unknown
OVERRUN                    unknown
UNKNOWN                    ledger_unreadable: 3
UNKNOWN                    observation_off: 2
UNKNOWN BEFORE ACTIVATION  1
TOTAL KNOWN                no
REMAINING                  unknown
EXECUTIONS INCLUDED        unknown
NOT COUNTED                task_without_issue; not counted, which is not a zero cost
NOT COUNTED                collaboration_host_run; not counted, which is not a zero cost
NOT COUNTED                plugin_external_call; not counted, which is not a zero cost
```

From `show-known.json`, `show-no-limit.json` (another root) and `show-tombstoned-off.json`: the command, and only the lines that differ from the output of `show-open.json` above.

```text
$ multica issue budget show --root 7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f
[exit 0]
[stdout]
LEDGER REVISION            57
UNKNOWN HOLD (H)           0.00 USD (0 ticks)
OVERRUN                    0.03 USD (300000000 ticks)
UNKNOWN                    none
UNKNOWN BEFORE ACTIVATION  0
TOTAL KNOWN                yes
REMAINING                  16.65 USD (166500000000 ticks)
```

```text
$ multica issue budget show --root b3a81f6d-2c47-4e95-8d0a-9f1e7c5b3a26
[exit 0]
[stdout]
BUDGET ROOT                b3a81f6d-2c47-4e95-8d0a-9f1e7c5b3a26
ROOT ISSUE                 8c1d5e9f-0a2b-4c3d-9e4f-5a6b7c8d9e0f
POLICY REVISION            1
LEDGER REVISION            9
ACTIVATED AT               2026-11-19T02:15:40Z
LIMIT                      not set
ATTEMPT RESERVE            not set
ACTUAL KNOWN (A)           0.42 USD (4200000000 ticks)
ESTIMATED (E)              0.00 USD (0 ticks); outside A and outside the limit
RESERVED (R)               0.00 USD (0 ticks)
UNKNOWN HOLD (H)           0.00 USD (0 ticks)
UNKNOWN                    none
UNKNOWN BEFORE ACTIVATION  0
TOTAL KNOWN                yes
REMAINING                  unknown; no limit is set
EXECUTIONS INCLUDED        3
```

```text
$ multica issue budget show --root 7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f
[exit 0]
[stdout]
STATE                      tombstoned; the root issue was deleted
MODE                       off; nothing is recorded while observation is off
POLICY REVISION            4
```

By issue (`issue-binding.json`, then `show-known.json`): four lines about the issue, then the lines of the root. The first nine lines of the run:

```text
$ multica issue budget show MUL-12
[exit 0]
[stdout]
ISSUE                      5e2d9a17-4c6b-4f03-9b8e-7a1c0d3f6e25
IS ROOT ISSUE              no
BIND SOURCE                create
BOUND AT                   2026-11-19T03:20:40.871536Z
BUDGET ROOT                7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f
ROOT ISSUE                 0b7f4c1e-93a2-4d58-b6e1-52c8a7f0d314
```

A 404 on the binding, a refusal, an answer that is not understood (`total_known` null), and a 500:

```text
$ multica issue budget show MUL-12
[exit 4]
[stderr]
issue MUL-12 belongs to no budget root, or is not visible in this workspace
```

```text
$ multica issue budget show --root 7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f
[exit 3]
[stderr]
refused (budget_forbidden): budget roots are for a person who is an owner or admin of the workspace and can see the root issue.
```

```text
$ multica issue budget show --root 7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f
[exit 1]
[stderr]
the answer of the server was not understood: field "total_known" is null; nothing of it is shown
```

```text
$ multica issue budget show --root 7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f
[exit 1]
[stderr]
the server answered HTTP 500, which is not an answer of the budget API; nothing was read
```

*`operation`.* Committed (`operation-committed.json`), committed and stale (`operation-committed-stale.json`), unknown (`operation-unknown.json`). All three exit 0: `unknown` is an answer.

```text
$ multica issue budget operation --issue MUL-12 --request-key 3f0e6f0a-6f1c-4c55-9d1e-0f2b7c1a9e10
[exit 0]
[stdout]
committed: an operation is committed under this key
OPERATION                c4e1a2f7-08b3-4b9d-a6c5-3e9f1d2b7a80
KIND                     enable
BUDGET ROOT              7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f
ROOT ISSUE               0b7f4c1e-93a2-4d58-b6e1-52c8a7f0d314
STATE                    open
MODE                     observe
POLICY REVISION          1
POLICY EPOCH             1
LIMIT                    25.00 USD (250000000000 ticks)
ATTEMPT RESERVE          not set
REASON                   Q4 launch
ACTIVATED AT             2026-11-19T01:02:03.456789Z
ISSUES BOUND             4, the root issue included
IN FLIGHT AT ACTIVATION  1; already running when the root was enabled, so unknown from before activation and not a zero cost
```

```text
$ multica issue budget operation --root 7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f --request-key 9b2f7c4e-1d3a-4e6b-8f50-2c7d9e1a4b63
[exit 0]
[stdout]
committed: an operation is committed under this key
stale: the policy of this budget root has changed since this operation; what follows is the result of the operation, not the policy of today
OPERATION        1a9d3c5e-7f2b-4086-b4d1-6e8c0a2f4b79
KIND             set_limit
BUDGET ROOT      7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f
ROOT ISSUE       0b7f4c1e-93a2-4d58-b6e1-52c8a7f0d314
STATE            open
MODE             observe
POLICY REVISION  4
POLICY EPOCH     1
LIMIT            not set
ATTEMPT RESERVE  0.50 USD (5000000000 ticks) for each execution; a declared reservation, not an estimate and not a cap
REASON           reserve half a dollar per run, no cap yet
```

```text
$ multica issue budget operation --issue MUL-12 --request-key 3f0e6f0a-6f1c-4c55-9d1e-0f2b7c1a9e10
[exit 0]
[stdout]
unknown: no committed operation is visible under this key yet
That is not a refusal and does not say that the write is lost.
To go on, run the original command again exactly as it was: the same --request-key and the same flags.
```

*`observations`.* A page with a next page (`observations-page.json`), the last page (`observations-last-page.json`), a page without a row (`observations-empty.json`). `-` stands for a null; the detail is the object as it came, in compact form.

```text
$ multica issue budget observations --root 7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f --limit 3
[exit 0]
[stdout]
ID                                    CREATED AT                   KIND                    REASON                          COVERED  ISSUE                                 TASK                                  CLAIM  ATTEMPT                               DETAIL
ab12cd34-ef56-4a78-9b90-c1d2e3f4a5b6  2026-11-19T05:41:09.250113Z  would_refuse            unbounded_unknown               no       5e2d9a17-4c6b-4f03-9b8e-7a1c0d3f6e25  1f2e3d4c-5b6a-4798-8a9b-0c1d2e3f4a5b  1      6f1e2d3c-4b5a-4978-8a6b-5c4d3e2f1a0b  {"by_reason":{"lost_contact":1,"observation_off":2}}
0f9e8d7c-6b5a-4493-8271-605f4e3d2c1b  2026-11-19T04:13:57.004321Z  attribution_difference  created_outside_context_parent  no       f2a4c6e8-1b3d-4f5a-97b9-d1e3f5a7c9b0  c2d3e4f5-a6b7-48c9-90d1-e2f3a4b5c6d7  1      b0c1d2e3-f4a5-46b7-88c9-d0e1f2a3b4c5  {}
4d5e6f70-8192-4a3b-bc4d-5e6f70819203  2026-11-19T03:20:40.871536Z  coverage_gap            in_flight_at_bind               yes      d91c4a6e-2f3b-4857-a0c9-6e5d4c3b2a19  71829304-a5b6-4c7d-8e9f-0a1b2c3d4e5f  2      -                                     {"task_status":"running"}
next page: multica issue budget observations --root 7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f --limit 3 --cursor MjAyNi0xMS0xOVQwMzoyMDo0MC44NzE1MzZaLDRkNWU2ZjcwLTgxOTItNGEzYi1iYzRkLTVlNmY3MDgxOTIwMw
```

```text
$ multica issue budget observations --root 7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f --limit 3 --cursor MjAyNi0xMS0xOVQwMzoyMDo0MC44NzE1MzZaLDRkNWU2ZjcwLTgxOTItNGEzYi1iYzRkLTVlNmY3MDgxOTIwMw
[exit 0]
[stdout]
ID                                    CREATED AT                   KIND          REASON            COVERED  ISSUE                                 TASK                                  CLAIM  ATTEMPT  DETAIL
c9b8a7f6-e5d4-43c2-a1b0-9f8e7d6c5b4a  2026-11-19T02:40:18.660902Z  coverage_gap  observation_off   no       -                                     -                                     -      -        {"policy_epoch":"1"}
58697a8b-9cad-4bce-8fd0-e1f203142536  2026-11-19T01:02:03.456789Z  coverage_gap  in_flight_legacy  no       5e2d9a17-4c6b-4f03-9b8e-7a1c0d3f6e25  d0e1f2a3-b4c5-46d7-a8e9-f0a1b2c3d4e5  2      -        {"task_status":"running"}
no further page
```

```text
$ multica issue budget observations --root 7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f
[exit 0]
[stdout]
no observations on this page
no further page
```

*`attempts`.* A page with a next page (`attempts-page.json`: sized, unsized, held, settled with an overrun, old daemon, no reservation), a page of a ledger that cannot be read (`attempts-unavailable.json`), a page without a row (`attempts-empty.json`).

```text
$ multica issue budget attempts --root 7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f
[exit 0]
[stdout]
ledger: available
ATTEMPT                               CREATED AT                   STATE                RESERVED                     RESERVATION     ACTUAL KNOWN                 UNSPENT                      OVERRUN                      PROTOCOL    CLAIM  TASK                                  ISSUE                                 CONTEXT PARENT                        AGENT                                 RUNTIME
6f1e2d3c-4b5a-4978-8a6b-5c4d3e2f1a0b  2026-11-19T05:41:09.118204Z  open                 0.50 USD (5000000000 ticks)  open            0.12 USD (1200000000 ticks)  0.38 USD (3800000000 ticks)  0.00 USD (0 ticks)           1           1      1f2e3d4c-5b6a-4798-8a9b-0c1d2e3f4a5b  5e2d9a17-4c6b-4f03-9b8e-7a1c0d3f6e25  -                                     4b8d2f6a-0c1e-4a3b-9d5f-7e9a1c3b5d7f  3e5a7c9b-1d2f-4b4d-8f6a-0c2e4a6b8d0f
0a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d  2026-11-19T05:12:44.730015Z  open                 unsized                      open            0.073 USD (730000000 ticks)  unsized                      unsized                      1           3      8b7a6c5d-4e3f-4201-b9a8-c7d6e5f4a3b2  d91c4a6e-2f3b-4857-a0c9-6e5d4c3b2a19  -                                     a7c9e1b3-d5f7-4092-b4d6-f8a0c2e4b6d8  3e5a7c9b-1d2f-4b4d-8f6a-0c2e4a6b8d0f
7e6d5c4b-3a29-4180-9f7e-6d5c4b3a2918  2026-11-19T04:58:21.904417Z  lost_contact         0.50 USD (5000000000 ticks)  held            0.21 USD (2100000000 ticks)  0.29 USD (2900000000 ticks)  0.00 USD (0 ticks)           1           1      5d4c3b2a-1908-4f7e-ad6c-5b4a39281706  5e2d9a17-4c6b-4f03-9b8e-7a1c0d3f6e25  -                                     a7c9e1b3-d5f7-4092-b4d6-f8a0c2e4b6d8  3e5a7c9b-1d2f-4b4d-8f6a-0c2e4a6b8d0f
b0c1d2e3-f4a5-46b7-88c9-d0e1f2a3b4c5  2026-11-19T04:12:55.310628Z  settled              0.50 USD (5000000000 ticks)  settled         0.62 USD (6200000000 ticks)  0.00 USD (0 ticks)           0.12 USD (1200000000 ticks)  1           1      c2d3e4f5-a6b7-48c9-90d1-e2f3a4b5c6d7  -                                     0b7f4c1e-93a2-4d58-b6e1-52c8a7f0d314  4b8d2f6a-0c1e-4a3b-9d5f-7e9a1c3b5d7f  3e5a7c9b-1d2f-4b4d-8f6a-0c2e4a6b8d0f
2c4e6a8b-0d1f-4357-9b9d-bf1d3f5b7d9f  2026-11-19T03:47:02Z         process_exit         0.50 USD (5000000000 ticks)  held            0.00 USD (0 ticks)           0.50 USD (5000000000 ticks)  0.00 USD (0 ticks)           old daemon  2      94a5b6c7-d8e9-4f0a-b1c2-d3e4f5a6b7c8  d91c4a6e-2f3b-4857-a0c9-6e5d4c3b2a19  -                                     4b8d2f6a-0c1e-4a3b-9d5f-7e9a1c3b5d7f  9f1b3d5e-7a9c-4e0b-a2d4-6f8b0d2c4e6a
e8d7c6b5-a493-4281-b06f-5e4d3c2b1a09  2026-11-19T03:20:41.118204Z  superseded_unproven  0.50 USD (5000000000 ticks)  no reservation  -                            -                            -                            1           1      3a4b5c6d-7e8f-4901-a2b3-c4d5e6f7a8b9  5e2d9a17-4c6b-4f03-9b8e-7a1c0d3f6e25  -                                     a7c9e1b3-d5f7-4092-b4d6-f8a0c2e4b6d8  3e5a7c9b-1d2f-4b4d-8f6a-0c2e4a6b8d0f
next page: multica issue budget attempts --root 7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f --cursor MjAyNi0xMS0xOVQwMzoyMDo0MS4xMTgyMDRaLGU4ZDdjNmI1LWE0OTMtNDI4MS1iMDZmLTVlNGQzYzJiMWEwOQ
```

```text
$ multica issue budget attempts --root 7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f
[exit 0]
[stdout]
ledger: unavailable; the ledger cannot be read, so what the reservations hold is unknown
ATTEMPT                               CREATED AT                   STATE         RESERVED                     RESERVATION  ACTUAL KNOWN  UNSPENT  OVERRUN  PROTOCOL    CLAIM  TASK                                  ISSUE                                 CONTEXT PARENT  AGENT                                 RUNTIME
6f1e2d3c-4b5a-4978-8a6b-5c4d3e2f1a0b  2026-11-19T05:41:09.118204Z  open          0.50 USD (5000000000 ticks)  unknown      unknown       unknown  unknown  1           1      1f2e3d4c-5b6a-4798-8a9b-0c1d2e3f4a5b  5e2d9a17-4c6b-4f03-9b8e-7a1c0d3f6e25  -               4b8d2f6a-0c1e-4a3b-9d5f-7e9a1c3b5d7f  3e5a7c9b-1d2f-4b4d-8f6a-0c2e4a6b8d0f
0a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d  2026-11-19T05:12:44.730015Z  open          unsized                      unknown      unknown       unknown  unknown  1           3      8b7a6c5d-4e3f-4201-b9a8-c7d6e5f4a3b2  d91c4a6e-2f3b-4857-a0c9-6e5d4c3b2a19  -               a7c9e1b3-d5f7-4092-b4d6-f8a0c2e4b6d8  3e5a7c9b-1d2f-4b4d-8f6a-0c2e4a6b8d0f
2c4e6a8b-0d1f-4357-9b9d-bf1d3f5b7d9f  2026-11-19T03:47:02Z         process_exit  0.50 USD (5000000000 ticks)  unknown      unknown       unknown  unknown  old daemon  2      94a5b6c7-d8e9-4f0a-b1c2-d3e4f5a6b7c8  d91c4a6e-2f3b-4857-a0c9-6e5d4c3b2a19  -               4b8d2f6a-0c1e-4a3b-9d5f-7e9a1c3b5d7f  9f1b3d5e-7a9c-4e0b-a2d4-6f8b0d2c4e6a
no further page
```

```text
$ multica issue budget attempts --root 7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f
[exit 0]
[stdout]
ledger: available
no attempts on this page
no further page
```

### Round 2: what was added to stage C

Nothing of the commands' behaviour changed in round 2. Three things were added (rulings R1 and reading 13, reading 27):

- One paragraph in the long help of `multica issue budget`, after the paragraph about `--request-key`:
  "A write exits with 0 only when its operation is committed. With any other exit code the first line of the message says what happened: it starts with "refused (" when the server refused the operation and nothing was written, and with "outcome unknown: " when nothing says whether it was committed."
  The wording differs from the ruling's sentence ("any other exit code is a refusal or an unknown outcome") on purpose; see reading 37 in section 3.
- Two new tests, `TestIssueBudgetHelpSaysHowToTellTheOutcomesApart` and `TestIssueBudgetServerTextCannotDrawOnTheTerminal`.
- Three new rows in the unknown-outcome table of the writes and in the table of the refused or unanswered reads: the two answers of the workspace membership check with their exact bytes, and a 404 with a code this command does not know.

The exit code of every row of the three outcome tables was pinned already; nothing was added for it (section 2 says where).

### Stage D: Core's schema and one API method (round 2)

Three paths in `packages/core/api/`. Nothing else of the package changed: no `package.json`, no lockfile, no `exports` entry, no hook, no store, no query key, no page.

- `issue-budget-schemas.ts` (new, 50 lines): `IssueBudgetSchema` and the type `IssueBudget` (`z.infer`), the `show` object of design 12.9.
  - Decimal strings that stay strings (`z.string().regex(/^(0|[1-9][0-9]*)$/)`): `policy_revision`, `policy_epoch`, `ledger_revision`, `settled_as_of_revision`, `limit_ticks`, `attempt_reserve_ticks`, `remaining_ticks` and the five `amounts.*_ticks`. A JSON number in their place fails the parse.
  - JSON numbers (`z.number().int().nonnegative()`): `unknown.by_reason[].count`, `unknown.pre_activation`, `coverage.included_attempts`.
  - May be null, must be present: `settled_as_of_revision`, `limit_ticks`, `attempt_reserve_ticks`, `remaining_ticks`, `amounts`, `coverage.included_attempts`.
  - Strings and not closed lists: `admission_state`, `mode`, `ledger_state`, the reasons, the names of `coverage.not_applicable`. `total_known` is a boolean. `budget_root_id`, `root_issue_id` and `activated_at` are strings.
  - The objects are plain `z.object`: a key the schema does not name is allowed and is left out of what is returned.
- `client.ts`: one import and one method after `getIssue` (29 lines added, none removed):
  `async getIssueBudget(rootId: string, options?: { workspaceId?: string; signal?: AbortSignal }): Promise<IssueBudget | null>`.
  It asks `GET /api/issue-budgets/{rootId}` (the ID through `encodeURIComponent`; workspace and signal through `workspaceRequestInit`, as `getIssue` does) and gives the body to `parseWithFallback(raw, IssueBudgetSchema, null, { endpoint: "GET /api/issue-budgets/:id", redact: true })`.
  - `null` is "no answer": a 2xx whose body is not a `show` answer (a field missing or of the wrong type), is not JSON, or is not there (204). It is never an object of zeros and never a state the server did not give.
  - A refusal is not `null`. 403 `budget_forbidden` and 404 `not_found` stay the client's `ApiError` with status and body; a request the caller gave up stays an `AbortError`.
  - The JSON reader's `SyntaxError` is caught in the method and dropped, and the body counts as malformed. On this runtime (node v22.23.2) the message of an "Unexpected token" error quotes about ten characters of the body on each side of the place where it stopped, which can be the name of a field and an amount. Every other error is rethrown unchanged.
  - With `redact: true` the one warning is `API response failed schema validation: GET /api/issue-budgets/:id` with `{ endpoint }` and nothing else: no issue list, no received value, no root ID.
- `issue-budget-schemas.test.ts` (new, 299 lines, 92 tests, first line `// @vitest-environment node`, no `any`). The fixtures are read where the server's tests read them, by a path relative to the test file (`new URL("../../../server/internal/handler/testdata/issue_budget_http_v1/", import.meta.url)`); nothing was copied. Nothing forbids that read: the package's ESLint configuration has no path restriction, `tsconfig` `rootDir` and `include` concern imports and not a file read at run time, Core has no boundary test, and `packages/views/skills/lib/skill-presentation.test.ts` already reads below `server/` in the same way. Every case goes through `client.getIssueBudget` with a stubbed `fetch`, a spy as the schema logger and a spy as the request logger.

### Stage E: the built-in skill says that `issue budget` is for people (round 2)

Three files of `server/internal/service/builtin_skills/multica-working-on-issues/`, the three places where `issue report-snapshot` is described today: 31 lines added and one line changed. `references/collaboration-workflow.md` and the frontmatter are untouched. No role skill changed.

- `SKILL.md`, line 14 (the paragraph that sends the reader to the report-snapshot reference) gets one sentence at its end and no new line:
  "`multica issue budget` is for people only, a workspace owner or admin using their own credentials: the command and the server (403 `budget_forbidden`) refuse agent and task credentials, reads included, so never call it from a task ([why](references/report-snapshots.md#issue-budget-commands-are-for-people-only))."
  The body was already 500 lines long as `TestBuiltinSkillsConformToTemplate` counts them, and 500 is the cap. It is 500 after the change.
- `references/report-snapshots.md` (44 to 63 lines): a section "Issue budget commands are for people only" at the end. It names the nine subcommands; says who runs them (a workspace owner or admin with their own credentials) and that the group has no read a task may use, unlike `issue report-snapshot show`; says what the command refuses before it sends anything (a missing token, a task, daemon or cloud token, an agent or task identity, a daemon-managed task context, a collaboration run) and what the server answers (403 `budget_forbidden`, the same whether or not a budget root exists, so it says nothing about the issue); and says what an agent does instead: no retry, no other credential, no other way to read the budget, and a line in the result comment that a workspace owner or admin has to do it.
- `references/working-on-issues-source-map.md` (500 to 511 lines): a section "Human-only issue budget CLI" after "Deterministic report-snapshot CLI", in that section's `| Contract | Source |` format, with four rows: the command group and where it is registered (`issueBudgetCmd`, `newIssueBudgetCommand`, `init` of `cmd_issue.go`, `TestIssueBudgetIsRegisteredWithItsSubcommands`); the local refusal (`budgetStart`, `budgetHumanCredential`, `requireHumanLocalCommand`); the server's refusal (design 12.1 and 12.3, the fixture `error-budget-forbidden.json`, and the CLI's reading of it in `budgetRefusal` and `budgetRefusalMeanings`); the tests of the local refusal (`TestIssueBudgetIsForPeopleOnly`, `TestIssueBudgetRefusesAClientThatIsNotAPerson`).

The row for the server's refusal cannot name a handler: the handlers of step 7a are not in this worktree, and here the code `budget_forbidden` occurs only in this lane's command files and in that fixture. The row cites the design by its path, as the row for the F2 execution contract in the section before it does. Section 3 lists this for the integration.

What the repository requires when a built-in skill's text changes is nothing beyond the tests that read the text: no version, digest or hash of a general built-in skill is stored anywhere. Section 5 has the search ("What pins a built-in skill's text").

## 2. Rules and the tests that pin them

All Go tests are in `server/cmd/multica/` and are named `TestIssueBudget…`: 36 top-level tests with 839 subtests (round 1 had 34 and 790; what round 2 added is under "Round 2" at the end of this section, followed by the tests of stages D and E). The stage C tests run the command tree against an `httptest` server that records every request (`newBudgetFake`); the token, the workspace and the server URL come from environment variables and a temporary `HOME`, the way the snapshot tests give them. An unplanned route answers 599, so a request that should not have been sent is seen.

### Stage A

| Rule of the brief | Test |
| --- | --- |
| each file is one line, ends with one line feed and is valid UTF-8 | `TestIssueBudgetFixtureFiles`, one subtest per file (26); it also fails when the directory holds a file the test does not name |
| each file decodes with `DisallowUnknownFields` into the structs the commands decode into | `TestIssueBudgetFixtureFiles`. `TestIssueBudgetStrictReadingReachesEveryDepth` proves that the strict reading reaches every depth: an unknown key put at each depth of an answer is refused by it and ignored by the reading the commands use |
| no key twice | `TestIssueBudgetFixtureFiles`; `TestIssueBudgetFixtureCheckFindsARepeatedKey` proves that the walk finds a repeated key at every depth and reports none for the same name in two objects |
| the five requests have the field order of 12.9 | `TestIssueBudgetFixtureFiles` encodes each request struct again and compares the bytes with the file |
| the amounts inside one file agree | not a Go test: the generator refuses to write a file whose amounts disagree (section 1) |

### Stage B

| Rule of the brief | Test |
| --- | --- |
| accepted: `0`, `0.0000000001`, `25`, `25.00`, `922337203.6854775807`, `none` | `TestIssueBudgetParseUSD` (each with its number of ticks; `none` gives no amount) |
| refused: `922337203.6854775808`, `-1`, `+1`, `1e3`, `1,000`, `.5`, `5.`, `01`, `1.00000000001`, the empty string, ` 1`, `1 `, `$1`, `None`, `0x10`, `1_000` | `TestIssueBudgetParseUSD`, each with the one of the three refusals it belongs to (form, more than ten decimals, above the largest), and 20 more of the same kinds |
| ten digits after the point are the tick scale | `TestIssueBudgetUSDDecimalsMatchTheTickScale` (`budget.TicksPerUSD` is 10 to the power of the digits the parser takes) |
| printing: 0, 1, 250000000000, the largest value | `TestIssueBudgetFormatTicks` |
| a null in both of its wordings, never 0 | `TestIssueBudgetFormatTicks` (`not set`, `unknown`; neither holds a digit) |
| what is printed is what was typed | `TestIssueBudgetUSDTextRoundTrips`: the dollars that are printed parse back to the same ticks |

### Stage C, rule by rule

**Rule 1, people only, read and write alike.**

- `TestIssueBudgetIsForPeopleOnly`. One read (`show --root`) and one write (`close`) are run with each of twelve credentials or surroundings: a task token, a daemon token, a cloud token, an agent id beside a human token, a task id beside a human token, the surrounding of a daemon task (all three), a task profile root, a daemon port beside a human token, a daemon port with a task token, a collaboration run, no token at all, and a task token that comes from a profile. Each time: an error, nothing on stdout, no request.
- The same test then runs every subcommand in each form it can be addressed (the five writes, `show --root`, `show <issue>`, `operation --root`, `operation --issue`, `observations`, `attempts`: 11 forms) with each of the eleven credentials that need no profile, with everything typed in full, in table and in json form: 121 subtests, each an error, nothing on stdout, no request.
- Its last subtest is the positive control: a human token that comes from a profile is let through and is the token that is sent.
- `TestIssueBudgetRefusesAClientThatIsNotAPerson` gives `budgetHumanCredential` a client that carries an agent id, a task id, each of the three token kinds, or no token, without the environment.
- `TestIssueBudgetNeedsAWorkspace`: without a workspace, or with one that is not a canonical lower-case UUID, a read and a write are refused before anything is sent.

**Rule 2, nothing is filled in for the operator.**

- `TestIssueBudgetRequiresEveryFlag`: each write without each of its flags, one at a time (28 subtests), and each write with `--yes=false` (5); then what a command is addressed by: `enable` without an issue, with two, with an empty one; a write with a stray argument; `show` without a root or an issue, with both, with two issues, with an empty one; `operation` without a key, without a root or an issue, with both, with an empty issue, with an argument; the lists without a root or with an argument. Each: an error that names the flag, nothing on stdout, no id that was not typed, no request.
- `TestIssueBudgetRefusesAFlagInTheWrongForm` (44 subtests): the key (upper case, all zeros, no dashes, braces, a URN, empty, a word); the root (an issue identifier, upper case, zeros, empty); the revision (zero, negative, a sign, a leading zero, a fraction, an exponent, a space, empty, above the largest, a word); one amount of each kind of refusal on `--limit` and on `--attempt-reserve`; the mode (`on`, `Off`, `enforce`, empty); a reason that is not valid UTF-8; `--output`; the page size and the cursor of a list. Each: an error, no request.
- That `--expect-revision` is sent as a decimal string, and both amounts as a decimal string or `null`, is pinned by the body bytes (rule 3).
- Of the reason only the encoding is checked. `TestIssueBudgetWriteBodyBytes` has two subtests for the other side of that: an empty and a blank `--reason` are sent as they were typed, and the refusal that is printed is the server's (`invalid_request`).

**Rule 3, one request per write, never a retry.**

- `TestIssueBudgetRequests`: for each of the five writes the body the server received equals the request fixture (without the line feed that ends the file), and the server saw exactly one request.
- `TestIssueBudgetWriteBodyBytes`: a reason with `<`, `>`, `&`, a quote, a backslash, a tab, U+2028, an emoji and an accented letter gives the same bytes on both Go lines, and the server reads the reason back as it was typed.
- `TestIssueBudgetWriteGivesTheTransportNothingToReplay`: with a transport that answers 201, 302, 307, 308 or 500, every write gives it exactly one request, a request without `GetBody` and without an `Idempotency-Key`. Section 5 says what net/http does with that.
- Every case of `TestIssueBudgetWriteRefused` and `TestIssueBudgetWriteOutcomeUnknown` ends with "the server saw exactly one request": no refusal and no unknown outcome is followed by a second request, a query by key or a read of the revision.

**Rule 4, the three outcomes of a write.**

- *Committed.* `TestIssueBudgetWriteCommitted`: the whole table for a 201 of an enable, for a 201 of an enable that leaves out a count of zero, and for a 200 of a key that had already committed; then every write with a 201 and with a 200 that says `replayed` (first line `committed: the operation was written now`, or `committed: this request key had already committed this operation; nothing was written now`); and a result that is not an enable has no activation lines.
- *Refused.* `TestIssueBudgetWriteRefused`: every write against each of the eleven codes of 12.3 with the status it comes with (55 subtests) and against a 401 with three bodies (the auth middleware's, an empty one, a page). Each: `refused (<code>): <what it means>. Nothing was written.`, the exit code of the status, nothing on stdout, one request, and the eleven meanings differ from each other. For `policy_revision_conflict` the text sends the operator to the `show` command of that root or issue to decide again; for every other code nothing follows the first line. Three more subtests per write put a code under a status the handlers do not give it (`state_conflict` under a 400, `invalid_request` under a 409, `not_found` under a 410): it is still a refusal, with the exit code of the status. The status and the code are not compared with each other.
- *Unknown.* `TestIssueBudgetWriteOutcomeUnknown`: every write against 38 answers (190 subtests). The seven of the brief: the connection is closed without an answer, a 500, a 502 with an HTML body, a 201 with a body cut in the middle, a 201 whose result names another root, a 201 whose result has another kind, a 404 whose body is not the error shape (two bodies). And: the wait ends while the server holds the request; a 500 with the code of a refusal; a 503; a 201 whose body ends before its declared length, without a body, with two answers, longer than 1 MiB, with the answer of another route, with a kind of a later server, with a revision or an amount as a JSON number, without the mode, without the limit, with a state of null, without `replayed`, with `replayed` as a word, with the mode twice, and for `enable` without the activation time; a 200 that does not say `replayed`; a 202; a 204; a 302 and a 307 to a route that would say committed; a 403 without a code; a 400 from in front of the handler; a 409 with a code this command does not know, with `internal_error`, with two codes that differ, with a code and no `error`, with a code of null; a 429.
- Each of them is checked by `wantBudgetUnknownOutcome`: exit 1, nothing on stdout, stderr starts with `outcome unknown: `, one line is exactly the by-key command with the key that was typed, the text says to keep the original `--request-key` and the exact flags and to run the very same command again if the query answers `unknown`; it does not hold "fail", "refused", "new key", "another key", "fresh key" or "different key" in any case of letters, and it holds no UUID other than the ones that were typed. The server saw one request.
- `TestIssueBudgetUnknownOutcomeDoesNotRepeatTheTransport`: the words of the transport are not shown (Go words an untrusted certificate as "failed to verify certificate").
- `TestIssueBudgetByKeyCommandIsRunnable`: the printed by-key command, run as it is, reaches the by-key route; it repeats a typed `--profile`, `--server-url` and `--workspace-id`, and quotes an issue the shell would split.
- `TestIssueBudgetWriteOutputJSON`: in json form a refusal, a 401, a 500 and each kind of answer that is not this request's print nothing on stdout.

**Rule 5, `operation`.** `TestIssueBudgetOperation`: committed (the result table under `committed: an operation is committed under this key`), committed and stale (one more line that the policy of the root has changed since), unknown asked by the root and asked by the issue (exit 0; the text says that it is not a refusal and does not say that the write is lost, and holds no "fail"); a kind of a later server is printed as it came. Not understood, each an error with nothing printed: a state of a later server, a state of null, no state, committed without `stale` or with a `stale` of null, without a result, with a result without its mode, with a revision as a number, with a limit that is missing, a committed enable without its activation time, unknown without `retry_path`, an answer cut in the middle, and an answer about another root than the one asked by.

**Rule 6, `show`.**

- `TestIssueBudgetShowByIssue`: two requests in order, the binding and then the root it names, and the four binding lines above the root's; the root issue itself; a 404 on the binding with the body of the budget handlers and with the body of the issue loader is `issue MUL-12 belongs to no budget root, or is not visible in this workspace`, exit 4, with no second request; a binding that is not understood (a root that is not an id, in upper case, missing or null; no `is_root_issue`; another answer; cut in the middle; followed by another body) is an error with no second request; the root that is shown must be the one the binding named; a refusal on the binding.
- `TestIssueBudgetShowTable`: the whole output for each of the five `show-*.json`, line by line: the ids, state, mode, the two revisions and the epoch, the activation time, the limit, the reserve with its remark, A, E with its remark, R, H, the overrun, one `UNKNOWN` line per reason, `UNKNOWN BEFORE ACTIVATION` on its own line, `TOTAL KNOWN`, `REMAINING` (`unknown` with the reason in words whenever the fixture has it null), `EXECUTIONS INCLUDED`, one `NOT COUNTED` line per name.
- `TestIssueBudgetShowWithoutALedgerShowsNoAmount`: from `show-unavailable.json` no amount line holds a digit, and exactly one line says that the ledger cannot be read.
- `TestIssueBudgetShowKeepsTheAmountsOfARootThatIsOff`: the amount lines of `show-tombstoned-off.json` equal those of `show-open.json`.

**Rule 7, the two lists.**

- `TestIssueBudgetObservations`: the whole table of a page with a next page (one row per item, `-` for a null) and the `next page: multica issue budget observations --root … --cursor …` line; the last page says `no further page`; a page without a row; the next-page command keeps `--limit` and takes the new cursor; a cursor that cannot be shown, or an empty one, is not put into a command and the page is not taken for the last one; a `detail` spread over lines is one cell; a kind and a reason of a later server are printed as they came; eleven answers that are not understood.
- `TestIssueBudgetAttempts`: the whole table of `attempts-page.json` (the open sized attempt, the unsized one, the held one, the quick-create one, the old daemon, the one without a reservation); `an unsized reservation never prints as 0` (`unsized` in the size and in the two cells a size would fill) beside `a sized reservation of zero prints as an amount`; an attempt without `protocol_version` prints `old daemon`; a page without a ledger prints `unknown` in the reservation cells, also when the answer carries reservations; the next-page command; thirteen answers that are not understood, among them a row without its `protocol_version` key (absent is not the same as null).

**Rule 8, answers are read leniently, never guessed.**

- `TestIssueBudgetShowReadsLeniently`: an extra field at every depth changes nothing; `mode` `enforce` prints `enforce`; a state, a ledger state, a reason and a `not_applicable` name of a later server are printed as they came; a ledger state the command does not know is taken neither for `available` nor for `unavailable` (the amounts that came with it are printed as they came, and without amounts every amount line says `unknown`); an empty `not_applicable` prints `NOT COUNTED | none`; a remaining amount of null that the answer does not explain prints `unknown` and no number; a text that would move the cursor is shown quoted.
- `TestIssueBudgetShowAnswerNotUnderstood` (41 cases), each an error with nothing printed: an amount as a JSON number, with a leading zero, with a sign, in dollars, empty, above the largest; a missing `remaining_ticks`; a missing `limit_ticks` (the output must not hold `not set`); a missing `attempt_reserve_ticks`; a `total_known` of null, as a word, missing; a revision as a JSON number or null; a count as a string, with a fraction, below zero; a reason without its count; `by_reason` of null; a mode of null; a state that is a number; a missing `ledger_state` (must not read as `available`); an activation time of null; an amount of the ledger missing or null; a missing `included_attempts`; a field that is there twice; a missing `amounts` key; `amounts` of null beside a ledger that can be read; another shape, an empty answer, an answer cut in the middle or followed by another; the answer about another root.
- The same for the other answers: `TestIssueBudgetOperation`, `TestIssueBudgetObservations`, `TestIssueBudgetAttempts` (above), and for a write the `a 201 …` cases of `TestIssueBudgetWriteOutcomeUnknown`.
- `TestIssueBudgetReadRefusedOrUnanswered`, each of the four reads against fourteen answers: a 401 and the three codes a read can get are refusals; a 404 of another handler, a 403 without a code, a 500, a 500 and a 503 with the body of a refusal, a 502 with a page, a 204, a 200 longer than the command reads and a 302 are "not an answer of the budget API" and nothing is read from them; a connection closed without an answer is "no answer". Nothing is printed on stdout in any of them.

### Round 2: three additions to stage C

The numbers of the stage C part above are those of round 1. Round 2 added two tests and three rows to two tables; nothing was removed. `TestIssueBudgetWriteOutcomeUnknown` now runs every write against 41 answers (205 subtests) and `TestIssueBudgetReadRefusedOrUnanswered` every read against 17 answers (68 subtests).

**The exit code (ruling R1).**

- `TestIssueBudgetHelpSaysHowToTellTheOutcomesApart` (new, 2 subtests). The help of the group must hold the sentence about exit code 0 and the two prefixes; then a write is run against a 409 that is a refusal and against a 409 that is not an answer of the budget handlers. Both exit 1, and the first words of stderr are exactly the two prefixes the help quotes. So the help cannot promise words that the commands do not print.
- Nothing was added to the three outcome tables, because every row asserted its exit code already (`cmd_issue_budget_write_test.go`):
  - committed: exit 0 and an empty stderr, in each case of `TestIssueBudgetWriteCommitted` (lines 86, 115, 141, 177) and of `TestIssueBudgetWriteOutputJSON` (line 222);
  - refused: `budgetRefusalCodes` gives every code of 12.3 its status and the exit code of that status (400 is 5, 403 is 3, 404 is 4, 422 is 5, and 1 for each of the seven 409 codes), and `TestIssueBudgetWriteRefused` compares it for every write (line 296); a 401 exits 3 (line 338); a code under another status exits by the status (line 365);
  - unknown: every row goes through `wantBudgetUnknownOutcome`, whose first check is exit 1 (line 384).
  The exit code in these tests is `cli.ExitCodeFor` of the error the command returned, taken by the test runner the way `main` takes it (`budgetRun.exit`). A mutant of round 1 ("an unknown outcome after a 4xx gets the exit code of that status") is killed by `TestIssueBudgetWriteOutcomeUnknown`.

**The membership check's own answers (reading 13).**

- Two rows with the exact bytes of `server/internal/middleware/workspace.go`: `the 404 of the workspace membership check` (`{"error":"workspace not found","code":"workspace_access_denied"}`) and `the 403 of the workspace role check` (`{"error":"insufficient permissions"}`).
  - In `TestIssueBudgetWriteOutcomeUnknown`, for each of the five writes: exit 1, `outcome unknown: ` first, the by-key command printed, one request seen by the server.
  - In `TestIssueBudgetReadRefusedOrUnanswered`, for each of the four reads: not an answer of the budget API, the exit code the status gives (4 and 3), the text holds `HTTP 404` or `HTTP 403` and not the word `refused`, nothing on stdout.
- A third row, `a 404 with a code this command does not know` (both keys of the error shape, equal values, a code that 12.3 does not have), in both tables. It was added after the first mutant run of round 2 left one mutant alive (section 4).
- The command still does not know the bodies of the membership check. The rows pin that it does not take them for a refusal.

**A text of the server cannot draw on the terminal (reading 27).**

- Before round 2 two things pinned it: the `MODE` line of `show` with an escape sequence and a line feed ("a text that would move the cursor is shown quoted"), and the cursor in the next-page footer.
- `TestIssueBudgetServerTextCannotDrawOnTheTerminal` (new, 20 subtests): five places where a text of the server is printed (the reason of a result, the kind of an observation, the reason of an observation, a state the command does not know, a mode it does not know), each with four texts (an escape sequence, a line feed, a carriage return, an empty text). Each comes out as one quoted Go string in its own cell: the output holds no ESC and no carriage return, and every line equals the line of the same output for the fixture as it is, except the one line that shows the text, where the quoted text stands in the place of the fixture's. No line is added and none is lost.

### Stage D (round 2)

All in `packages/core/api/issue-budget-schemas.test.ts`: 92 tests, each through `client.getIssueBudget`. The brief's list is the left column.

| Rule of the brief | Test |
| --- | --- |
| every `show-*.json` parses to an object with the same values | `finds the five answers of the fixture directory` (the list read from the directory is exactly the five names, so a sixth file or a missing one fails); `returns %s with the values it came with` (5): the answer is `toStrictEqual` to `JSON.parse` of the file, and no warning is logged |
| an amount as a number gives `null` | `gives no answer for an amount as a number`, `… an amount as a number that no JSON reader keeps` (9223372036854775807 written without quotes), `… an amount of the ledger as a number`, `… the remaining amount as a number`, and the same for the revisions (`a revision as a number`, `an epoch as a number`, `a settled revision as a number`) |
| an amount with a leading zero or a sign | `… an amount with a leading zero`, `… an amount of two zeros`, `… an amount with a minus sign`, `… an amount with a plus sign`; beyond the brief: in dollars, with an exponent, empty, followed by a line feed, after a space, on the second line of a text |
| a missing `ledger_state` | `… a missing ledger_state`; also `a ledger state of null` |
| a missing `amounts` key | `… a missing amounts key`; also one amount of the ledger missing or null, and `amounts` as a list |
| a count as a string | `… a count as a string`, `… the count from before activation as a string`, `… included attempts as a string`; also a count with a fraction and below zero |
| a missing `total_known` | `… a missing total_known`; also null and the word `"false"` |
| an extra field still parses | `leaves out a field of a later server %s` (5 depths: beside the amounts, among them, in the unknown part, in a reason, in the coverage): the answer equals the one without the field, and no warning |
| an unknown enum string still parses | `returns %s as it came` (8): a state, a mode, a ledger state, a reason and a not-counted name of a later server; and three values the first slice does not send but 12.9 allows: a settled revision, an amount of 9223372036854775807, an amount of 0 |
| a failed parse logs no part of the answer | every one of the 64 `gives no answer for %s` rows: the log holds exactly one warning, `API response failed schema validation: GET /api/issue-budgets/:id` with `{ endpoint }`, and `expectNothingOfTheAnswerLogged` finds no 8 characters in a row of any name, amount, ID or time of the body in the schema log or in the request log |

Beyond the brief's list:

- The request (5 tests): the URL and the `X-Workspace-ID` header; an ID with `/`, `?` and `#` is escaped; a request the caller has given up is not sent and stays an `AbortError`; a 403 `budget_forbidden` and a 404 `not_found` (the two error fixtures) reject with their status and code, and no warning is logged. A refusal is never `null`.
- Every nullable key removed, one at a time (6 rows): `null` is a value, an absent key is a malformed answer.
- Answers that are not an object of 12.9: `null`, a list, a text, the answer of another route (`attempts-page.json`), the body of a refusal under a 200.
- Bodies that are not JSON (7 rows) and no body at all (the 204): cut in the middle, a stray character, `NaN`, a number with a plus sign, a state without its quotes, an HTML page, an empty body. Three of them (`NaN`, the plus sign, the state without quotes) are the ones for which the runtime's error message quotes the body.
- Three checks of the check: the log search fails when a log holds the answer, fails when a log holds an error that quotes 21 characters of it, and passes on the log of a request that was answered well (so it does not pass only because nothing was logged).

**Each test was seen failing before the code existed.** On the first version of the file (82 tests), with no schema and no method: 80 failed, 2 passed (`TypeError: client.getIssueBudget is not a function`; the two that passed do not call the method). Then with the method and a schema that accepts anything (`z.unknown()`): 66 failed, 16 passed. Then with a schema that accepts nothing (`z.never()`): 15 failed, 67 passed. The same two stand-ins against the final file (92 tests, schema restored by copy and its SHA-256 compared): `z.unknown()` gives 70 failed and 22 passed (the 64 rows and the 204 of "gives no answer for", and the 5 "leaves out a field"); `z.never()` gives 18 failed and 74 passed (the 5 fixtures, the 5 "leaves out a field", the 8 "returns … as it came"). Outputs: `/tmp/f3-7b/r2/stage-d/red0.out`, `red-unknown.txt`, `red-never.txt`.

### Stage E (round 2)

No test was added, and none could be inside this lane's files (R6 in section 3). The tests that read the skill and need no database are five, all in `server/internal/service`; they pass with the same counts before and after the change (5 top-level, 27 at every depth, no skip):

| What it holds the changed text to | Test |
| --- | --- |
| the body of `SKILL.md` is at most 500 lines, the description at most 300 characters, no supporting file looks like an eval | `TestBuiltinSkillsConformToTemplate` (the body is 500 lines before and after) |
| the frontmatter is strict YAML | `TestBuiltinSkillsFrontmatterIsStrictYAML` (the frontmatter is untouched) |
| the skill still holds its anchors, holds none of the texts that would repeat the runtime prompt, and has its source map | `TestWorkingOnIssuesSkillCoversIssueLoopContracts` |
| the skill is still loaded and bundled under its name | `TestLoadRequestedAgentSkillBundles_OnboardingScope`, `TestBuildAgentSkillBundlesAssignsBuiltinID` |

Every symbol, file and test that the four new rows of the source map name was looked up in the worktree and exists (section 5).

## 3. Needs a ruling, and readings

### Needs a ruling

Seven items, and all seven are ruled. R1 to R4 are from round 1 and were ruled on 2026-11-19 (round 2 message, section 1.3). R5 (a test run of this lane wrote to a database), R6 and R7 are from round 2 and were ruled on 2026-11-20. Each ruling is the last line of its item, and the code is as ruled. The ruling on R6 added one file to this lane's list, `server/internal/service/builtin_skills_test.go` (one anchor line); nothing else outside the files of brief section 1 was changed for any of them.

**R1. An exit code of its own for the unknown outcome.** Rule 4 asks that the three outcomes "exit differently from success in the way the snapshot command reports an error". The exit code is `cli.ExitCodeFor`'s, which goes by the kind of the error: a code that only the unknown outcome has needs a new kind of error in `server/internal/cli/errors.go`, and that file is not this lane's.
Interim: committed exits 0; a refusal exits by its status (3, 4, 5, and 1 for a 409); the unknown outcome always exits 1. So nothing but success exits 0, but a 409 refusal and an unknown outcome share exit 1, and a caller tells them apart by the first words of stderr: `refused (` or `outcome unknown: `. Both prefixes are pinned by tests.
To rule: is the stderr prefix enough for the callers that are planned (the skill text of stage E tells an agent nothing here, because these commands refuse agents), or does the unknown outcome get its own exit code, and which one.
Ruled: no exit code of its own and no new kind of error in `internal/cli`; the first words of stderr tell a 409 refusal from an unknown outcome. Done in round 2: a paragraph in the group's help (reading 37) and `TestIssueBudgetHelpSaysHowToTellTheOutcomesApart`; the exit code of every row of the three outcome tables was pinned already (section 2).

**R2. Three fixture files more than the brief's table.** `operation-reopen-request.json`, `observations-empty.json`, `attempts-empty.json`.
Interim: kept. The Go check names all 26 files and fails on one it does not name; `TestIssueBudgetRequests`, `TestIssueBudgetReadOutputJSON` and the two list tests use the three. The server lane has to decode and encode them like the others. The two empty pages are there for a reason the server lane will meet: `"observations":[]` and `"attempts":[]` are what 12.9 allows, a nil Go slice encodes as `null`, and these commands read a list of `null` as "not understood".
To rule: keep them as part of the shared contract, or take them out (then the four tests named above lose the cases that use them).
Ruled: kept. The contract of the directory is these 26 files; an empty list is `[]` on the wire and never `null`. No fixture changed in round 2 (the 26 SHA-256 values are those of round 1, section 5).

**R3. `--mode` is checked on the command line.** The brief's table gives `--mode <off|observe>`, and the command refuses anything else before a request is sent (`mode on`, `Off`, `enforce` and an empty mode are pinned in `TestIssueBudgetRefusesAFlagInTheWrongForm`).
The consequence is one-sided: a mode that a later server adds is printed by this version when it is read (rule 8), but cannot be set with it.
Interim: the local check, as the table says. The other choice is to send what was typed and print the server's `invalid_request`; it costs one line and two test cases.
Ruled: kept as it is. The one-sided consequence goes into the spec (the main session writes it).

**R4. The cursor is a query value, not a path segment.** Brief section 4 item 6 asks for `url.PathEscape` "for the issue, the root, the key and the cursor". Design 12.9 has `limit` and `cursor` as query parameters of the two list routes.
Interim: the issue, the root and the key go into the path through `url.PathEscape`; the cursor and the limit go into the query through `url.Values.Encode`. `PathEscape` leaves `&`, `=` and `+` as they are, which is right in a path and wrong in a query: a cursor that holds one would be cut or changed on the way. `TestIssueBudgetRequests` ("attempts with a cursor that needs escaping") sends `a+b/c=d&e f` and reads the same text back on the server side.
To rule: confirm that the design is the one to follow here.
Ruled: the design is right; `url.Values.Encode` stays.

**R5 (round 2, ruled). One test run of this lane wrote to a database.** On 2026-11-20, 03:13:08 to 03:14:21 CST, as a baseline before stage E, this lane ran the whole package `server/internal/service` (`bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test ./internal/service/ -count=1 -v`, default toolchain, `DATABASE_URL` unset). Brief item 6 asks only for the tests that name the skill; the package-wide run was this lane's own addition and broke the rule "no database".
Five helpers of that package fall back to `postgres://multica:multica@localhost:5432/multica?sslmode=disable` when `DATABASE_URL` is empty: `newResolveOriginatorPool` (`resolve_originator_test.go`), `newTaskClaimRacePool` (`task_claim_race_test.go`), `newCancelFinalizePool` (`task_cancel_finalize_test.go`), `newHeadShaDedupPool` (`task_dedup_head_sha_test.go`), `newPluginStoragePool` (`plugin_storage_db_test.go`). A server answered at that address: the output holds 50 errors `relation "collaboration_stage_child" does not exist (SQLSTATE 42P01)`.
What the saved output shows: 254 top-level tests can reach one of those helpers; 155 passed, 38 failed, 61 skipped. The package as a whole: 849 PASS, 47 FAIL, 795 SKIP, exit 1; the other 9 failures refuse to run without an explicit `DATABASE_URL` and touched nothing. The tests insert rows of their own and delete them in `t.Cleanup`. Among those that passed are reconcilers and sweeps (the channel media reconciler, the autopilot quota reconciler, the cleanup of abandoned source contexts, the delegated-failure sweep, the stale-dispatch reclaim).
Not established: whether one of those sweeps acted on rows that were not its own, and whether a failed test left rows behind. This lane did not query, inspect or clean that database afterwards: the brief forbids it and the database is not this lane's.
Interim: stopped; the script (`/tmp/f3-7b/r2/skill-tests.sh`) now runs the five named tests only and says why; the main session was told at once (message of 2026-11-20, about 03:25 CST). Evidence: `/tmp/f3-7b/r2/skill-tests/before-package.out` (the `-v` output with times and the IDs the tests logged), `db-tests.txt` (the 254 tests with their result), `classify.py` (how the list was made).
To rule: whether that database is to be inspected and cleaned, and by whom. Nothing else of this lane touched a database: the five named tests reach none of the helpers, `./cmd/multica/...` has no such fallback, build and vet run no tests, and Core's tests have no database.
For whoever runs Go tests in this repository without a database: with `DATABASE_URL` unset, `server/internal/service` does not skip; it tries `localhost:5432/multica` and uses it when a server answers there. `server/internal/handler` is different: its `TestMain` refuses to start without an explicit `DATABASE_URL`.
Ruled (main session, 2026-11-20): the lane was right to stop and not to look. The main session inspected the database read-only. It is the scratch database `multica` of the shared container: no registered development environment uses it, its migration head is far behind, and it holds test fixtures left by earlier runs of the same kind since 2026-09-11. Rows created or captured in this run's window: 666 in 9 tables. No surviving row from before the run was updated in the window. Not established: whether a sweep-type test deleted an earlier leftover row (there are no counts from before the run). Nobody cleans it: not the lane and not the main session; whether to clean or rebuild it is the user's decision and is reported to the user. The rule from now on, for every lane: every `go test` carries an explicit `DATABASE_URL`; a run that is meant to touch no database gets `postgres://nodb:nodb@127.0.0.1:9/nodb?sslmode=disable&connect_timeout=2`, where nothing listens and the helpers skip.

**R6 (round 2, for the integration). No test pins the new sentence of the skill.** `TestWorkingOnIssuesSkillCoversIssueLoopContracts` (`server/internal/service/builtin_skills_test.go`) holds the list of texts the skill must contain. A line for the budget sentence would belong there, and that file is not in this lane's list, so nothing was added. As it is, the three additions of stage E can be removed without a test failing: the five tests that name the skill pass with the same counts before and after the change (5 top-level, 27 at every depth).
To rule: add an anchor to that list at integration (for example `` `multica issue budget` is for people only `` for `SKILL.md`), or leave the text unpinned.
Done in the closing turn (2026-11-20), by the ruling. The line added to the list `mustContain` of `TestWorkingOnIssuesSkillCoversIssueLoopContracts`, after the anchor `"--status backlog",`: `` "`multica issue budget` is for people only", ``. It is the only change of `server/internal/service/builtin_skills_test.go` (1 line added, none removed; SHA-256 `35639312170a4a0855f39f4c802cd4893be825a43e0b88c96b58db66bc26a670` before, `e03ed1ceed917a2bf36736ca4576482d2fe6ee7f49afa80873320501c74a507e` after), and `gofmt -l` on the file prints nothing with the gofmt of go1.27.0 and with that of go1.26.9 (exit 0 both).
Red (07:54:16 to 07:55:30 CST), the anchor with one wrong character, `onlx` for `only`: `DATABASE_URL='postgres://nodb:nodb@127.0.0.1:9/nodb?sslmode=disable&connect_timeout=2' bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test ./internal/service/ -run '^TestWorkingOnIssuesSkillCoversIssueLoopContracts$' -count=1 -v`, go1.27.0: exit 1; top-level PASS 0, FAIL 1, SKIP 0, and the same at every depth. Its one message: `` builtin_skills_test.go:269: working-on-issues skill missing "`multica issue budget` is for people onlx" ``. Evidence: `/tmp/f3-7b/r2/skill-tests/r6-red.txt`, `r6-red-named.out`.
Green (08:03:19 to 08:04:58 CST), the anchor as the words stand in `SKILL.md`: `/bin/bash /tmp/f3-7b/r2/skill-tests.sh r6-green`. The script exports the same `DATABASE_URL` (it no longer unsets it) and runs `bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test ./internal/service/ -run '^(TestBuiltinSkillsConformToTemplate|TestBuiltinSkillsFrontmatterIsStrictYAML|TestWorkingOnIssuesSkillCoversIssueLoopContracts|TestLoadRequestedAgentSkillBundles_OnboardingScope|TestBuildAgentSkillBundlesAssignsBuiltinID)$' -count=1 -v`, go1.27.0: exit 0; top-level PASS 5, FAIL 0, SKIP 0; 27 at every depth, with no FAIL and no SKIP at any depth. Evidence: `/tmp/f3-7b/r2/skill-tests/r6-green.txt`, `r6-green-named.out`. These are the two runs the ruling asks for: the package as a whole was not run, and no test ran on go1.26.9 in the closing turn.
Ruled: pinned.

**R7 (round 2, for the integration). The source-map row for the server's refusal names no handler.** In this worktree `budget_forbidden` occurs only in this lane's command files and in the fixture `error-budget-forbidden.json`; the handlers of step 7a are elsewhere. The row cites design 12.1 and 12.3, the fixture, and the CLI's reading of the answer.
To rule: when step 7a is integrated, the row gets the handler's file and symbol (and its test), in place of or beside the design reference. One line of `references/working-on-issues-source-map.md`.
Ruled: as proposed; the main session changes the row when step 7a is merged.

### Readings (where the brief and the design are silent)

Each is what the code does now. Where a test pins it, the test is named in section 2; the ones without a test are listed again in section 6.

**What is sent**

1. The body of a write is `json.Marshal` of the request struct: the bytes of the request fixture without the line feed that ends the file.
2. No `Idempotency-Key` header. 12.9 names none; the key travels in the body as `request_key`. (net/http would also read that header as leave to send a POST again, section 5.)
3. `X-Client-Capabilities` is not sent: its one value asks for a shape of attachment URLs, and no budget answer has one. `X-Client-Platform`, `X-Client-Version` and `X-Client-OS` are sent from the package-level values of `internal/cli`, as the snapshot command sends them.
4. An empty token is refused before anything is sent, and so is a workspace that is missing or is not a canonical lower-case UUID. The brief names neither; no budget route can answer without them.
5. The token kinds that are refused are the three the snapshot command refuses: `mat_`, `mdt_`, `mcn_`. `internal/auth/password_token.go` also knows `mpc_` and `mpi_`; a token with one of those is sent, and the server decides.
6. The people-only check comes before every other check, so a caller that is not a person gets that refusal also for a command line that is wrong in another way. For the same reason the number of arguments is checked in the command's own function and not by cobra's `Args`.
7. Of `--reason` only the encoding is checked. An empty, a blank or a very long reason is sent as typed, and the refusal is the server's.
8. `limit` and `cursor` are sent only when they were typed; without `--limit` the server's default applies (50, design 12.9). `--limit` is checked as a whole number from 1 to 100, the range of 12.9; an empty `--cursor` is refused.

**The outcome of a write**

9. A 201 that says `replayed` true is committed and is reported as "this request key had already committed": the body is believed over the status. A 200 that says `replayed` false is the unknown outcome: 12.9 gives a 200 only for a replay.
10. A result belongs to this request when its kind is the command's and, for a command addressed by root, its root is `--root`. Nothing else in it is compared with what was typed: a result with another reason, limit or revision is printed as committed. (For `enable` the result names the root issue by its id, and the operator may have typed an identifier; they cannot be compared here.)
11. A refusal is a status from 400 to 499 with a body of exactly the error shape whose `error` equals its `code` and whose code is one of the eleven of 12.3. The status is not compared with the code, and there is no list of codes per command: every command reports each code the same way.
12. A 401 is a refusal whatever its body is: the credential is turned away before a handler sees the request, and that body is not the budget shape.
13. Every other 4xx is the unknown outcome, as rule 4 says. That includes a 403 or a 404 that comes from in front of the budget handlers (the workspace check, a proxy) and a 429. An operator who is not a member of the workspace is therefore told "outcome unknown" for a write, although nothing can have been written; the query by key then says that the server did not give an answer of the budget API.
14. A 5xx is never a refusal, also with the body of one. A 3xx is not followed. A 202 and a 204 are the unknown outcome.
15. An answer longer than 1 MiB is not read: the unknown outcome for a write, an error for a read. (Measured on the fixtures: a row of attempts is 451 to 586 bytes and a row of observations 329 to 391 bytes, so a page of 100 rows stays under 60 KB.)
16. A request that cannot be made at all (a `--server-url` that is not a URL) is reported as the unknown outcome too; the text says that the request was sent "at most once".
17. The text of an unknown outcome is this command's own. What the transport or the body said is kept for `--debug` only: Go words an untrusted certificate as "failed to verify", and a body may hold an id that is not the operator's.

**Reading an answer**

18. `--output json` prints the received bytes only after the answer was understood and belongs to what was asked. For a refusal, an unknown outcome or an answer that is not understood nothing is printed on stdout; the error goes to stderr as in table form. A script therefore never gets a body on stdout that the command itself would not have used.
19. `show <issue> --output json` prints the second answer only (the root). The binding only says where to ask; printing both would not be one JSON document.
20. The root that `show` prints must be the root that was asked for, and the binding's `budget_root_id` must be a canonical UUID before it goes into the path of the second request. Otherwise: an error, and no second request.
21. Any 404 on the binding request is "belongs to no budget root, or is not visible in this workspace" (exit 4). The issue loader in front of the handler and the handler both answer 404; the command does not try to tell them apart.
22. The by-key answer: a `state` other than `committed` and `unknown` is not understood (it is the one enum that is not printed as it came, because the command would not know which of the two shapes follows). `retry_path` is required in the `unknown` answer and is not printed: the way forward is the original command, not a path. A committed answer asked by `--root` must name that root. A committed enable requires `activated_at`, as in the answer to the write.
23. `member_count` and `in_flight_gap_count` are read as 0 when they are absent, for an enable result only: 12.9 says they are left out when zero. It is the one place where an absent field has a value.
24. A key that is there twice makes the answer not understood (12.9 has none; which of the two a decoder keeps is not a thing to depend on).
25. A key that differs from a name of 12.9 only in the case of its letters is read as that name: `encoding/json` matches keys that way on both Go lines (`/tmp/f3-7b/exp2/`), and `{"mode":…,"MODE":…}` counts as a key that is there twice. No test pins this; it is a property of the decoder, not a choice.
26. A read that gets no answer exits 2 with a fixed text; what the connection reported is shown with `--debug`.

**What is printed**

27. A text of the server is printed as it came when every character of it can be seen, and as a quoted Go string otherwise (control characters, an escape sequence, a line break, an empty text). So an answer cannot move the cursor or add a line that looks like one of the table's.
28. With `ledger_state` `unavailable`, `show` hides the five amounts, the remaining amount and the executions included also when the answer carries them, and `attempts` prints `unknown` in the four reservation cells also when a row carries a reservation.
29. A ledger state the command does not know is taken neither for `available` nor for `unavailable`: amounts that came are printed, amounts of null print `unknown`, and an attempt without a reservation prints `unknown` and not `no reservation`.
30. A remaining amount of null prints `unknown; no limit is set` when the limit is null, `unknown; not everything is known` when `total_known` is false, and plain `unknown` otherwise. The command never works the amount out itself.
31. An empty `by_reason` prints `UNKNOWN | none` and an empty `not_applicable` prints `NOT COUNTED | none`, so the line a reader looks for is never missing.
32. `tombstoned` and `off` get a remark (`; the root issue was deleted`, `; nothing is recorded while observation is off`); every other state and mode is printed bare.
33. The size of an execution's reservation is shown once, in `RESERVED` (the attempt's `reserved_ticks`). The `reserved_ticks` inside `reservation` is read and required but has no column: for a sized reservation the two are the same number, and for an unsized one it is the 0 that must not be shown.
34. `detail` is printed as it came, as compact JSON in one cell. Its keys are not interpreted.
35. A next cursor that cannot be shown bare (not printable) or that is empty is not put into a command: the footer says that there is a next page and to read its position with `--output json`. The page is not taken for the last one.
36. A printed command (`operation` after an unknown outcome, `show` after a revision conflict, the next page) repeats `--profile`, `--server-url` and `--workspace-id` when the operator typed them, so it reaches the same server as the command that printed it. A word is left bare when it holds only letters, digits and `-_./:`, and is put in single quotes otherwise.

**Round 2: the commands**

37. The paragraph added to the help does not use the ruling's sentence "any other exit code is a refusal or an unknown outcome". That sentence would say too much: a command that was never sent (a flag in the wrong form, no `--yes`, a credential that is not a person's) and a commit whose result could not be printed also exit with something other than 0, and their message starts with neither prefix. The help says that 0 means committed and what each of the two prefixes means. `TestIssueBudgetHelpSaysHowToTellTheOutcomesApart` pins that the words the help quotes are the words the commands print.

**Round 2: Core (stage D)**

38. `getIssueBudget` returns `null` for every 2xx that is not a `show` answer: a field missing or of the wrong type, a body that is not JSON, no body (204). A refusal and a request the caller gave up stay errors. The brief asks for "a fallback of `null`" for a malformed answer; that a body which is not JSON counts as one is this lane's reading.
39. The JSON reader's error is caught inside the method. The other way, adding the budget path to the paths that the client's `fetch` treats specially, changes code that every request goes through, for one caller. The error is dropped and not logged, because its message can quote the answer.
40. Every field is checked on its own. The method does not compare the answered `budget_root_id` with the ID it asked with (the command does, reading 20), and it has no rule across fields.
41. A decimal string is checked for its form only. There is no upper bound: a string of more digits than an int64 has parses. Nothing in Core turns the string into a number.
42. A key the schema does not name is dropped from what is returned, at every depth (zod's default for `z.object`). The brief says such fields are allowed; keeping them (`.loose()`, as `triage-schemas.ts` does) was the other choice. Dropped, what is returned is what the type names.
43. `settled_as_of_revision` takes a decimal string or null, as 12.9 types it, although the first slice always sends null.
44. Keys are matched exactly, with regard to the case of their letters. The Go reader of the commands is not (reading 25); the round 2 message accepts the difference.
45. No `exports` entry was added to `packages/core/package.json`. The schema files that only `client.ts` imports have none (`triage-schemas.ts`, `iteration-schemas.ts`, `personal-access-token-schemas.ts`); the one schema file with an entry is imported from outside the package.
46. The method's name and options follow `getIssue` (`workspaceId`, `signal`). The endpoint in the warning is the fixed text `GET /api/issue-budgets/:id` and never holds the ID.
47. The test reads the server's fixtures where they are. A change of a `show-*.json` fixture therefore changes what Core is tested against, which is the purpose: both readers and the handlers are held to the same bytes.

**Round 2: the skill (stage E)**

48. The sentence in `SKILL.md` is appended to an existing line, because the body is at its 500-line cap. The line is now 936 characters long.
49. The explanation is a section of `references/report-snapshots.md`, a file named for another feature, because the brief asks to say it where `issue report-snapshot` is described. A reference file of its own would be the other choice; it would need a link from `SKILL.md` all the same.
50. The skill says "agent and task credentials" where the brief says "agent credentials": the command refuses both, and the server's allow-list of 12.1 admits neither.
51. The reference says three things beyond the brief's four facts, each from the design: the reads are refused like the writes (12.1); an ordinary member gets the same 403 (12.1, condition 2), so the code says nothing about whether a root exists; and what an agent does instead (no retry, no other credential, a line in the result comment). It does not tell an agent how the commands are used: no flag, no request key, no example.

## 4. Mutants

Driver: `/tmp/f3-7b/mutate.py <spec.py> <results.md>`. For each mutant it copies the file aside, replaces one piece of text that occurs exactly once, runs `go -C server test ./cmd/multica/ -run 'TestIssueBudget' -count=1 -v` on the default toolchain, copies the file back and compares its SHA-256 with the one taken before. A mutant survives when the run exits 0; otherwise the driver names the top-level tests that failed, at most four, and counts the rest. `mutate2.py` is the same driver and also records how a mutant died: by a failed test, by a panic inside a test, or by a build failure.

Mutants were run while each stage was built, and every one again at the end against the final files, so that the tables say what the final tests catch in the final code. The tables are from the last run, with `mutate2.py`, 2026-11-19 17:11 to 17:22 CST: 151 runs (stage B 11, stage C 138, and 2 that were added then), 151 killed, none survived, none that could not be applied; 150 died by a failed test, 1 by a panic inside a test (stage B, #10), none by a build failure. After each run the file was copied back with an equal SHA-256 (151 of 151). After the last one `shasum -a 256 -c` over the ten Go files of this lane printed `OK` ten times. The round 1 verification of section 5 ran after that; between it and the start of round 2 only commands that read ran in the worktree. Everything from here to the heading "Round 2" is round 1; the mutants of round 2 (the commands again, and Core) are at the end of this section.

### Stage B (`cmd_issue_budget_money.go`): 12 mutants, 12 killed

While the stage was built: 11 mutants, 11 killed. Against the final tree they ran twice (17:04 with `mutate.py`, 17:19 with `mutate2.py`), with the same tests named both times; the tests of stage C now catch some of them too. #10 ran with `-run 'TestIssueBudgetParseUSD|TestIssueBudgetUSDTextRoundTrips'`. It dies by a panic (`strings.Repeat` with a negative count, for a fraction of ten digits) and not by a comparison, so #12 was added at the end: the same line, padded one digit too far, run with all the tests.

| # | Mutant | Caught by | How |
| --- | --- | --- | --- |
| 1 | eleven decimals are cut to ten instead of refused | `TestIssueBudgetParseUSD`, `TestIssueBudgetRefusesAFlagInTheWrongForm` | failed test |
| 2 | a leading zero is accepted | `TestIssueBudgetParseUSD` | failed test |
| 3 | an amount above int64 is clamped to the largest instead of refused | `TestIssueBudgetParseUSD` | failed test |
| 4 | the word none is read without regard to case | `TestIssueBudgetParseUSD`, `TestIssueBudgetRefusesAFlagInTheWrongForm` | failed test |
| 5 | a point with nothing after it is accepted | `TestIssueBudgetParseUSD` | failed test |
| 6 | surrounding space is trimmed before reading | `TestIssueBudgetParseUSD` | failed test |
| 7 | a null amount prints as zero | `TestIssueBudgetAttempts`, `TestIssueBudgetFormatTicks`, `TestIssueBudgetOperation`, `TestIssueBudgetShowAnswerNotUnderstood`, and 4 more | failed test |
| 8 | the fraction is trimmed below two digits | `TestIssueBudgetAttempts`, `TestIssueBudgetFormatTicks`, `TestIssueBudgetOperation`, `TestIssueBudgetShowByIssue`, and 4 more | failed test |
| 9 | the fraction is cut to two digits (a rounding printer) | `TestIssueBudgetAttempts`, `TestIssueBudgetFormatTicks`, `TestIssueBudgetUSDTextRoundTrips` | failed test |
| 10 | the fraction is padded to nine digits | `TestIssueBudgetParseUSD` | panic in a test |
| 11 | a negative amount is printed as dollars | `TestIssueBudgetFormatTicks` | failed test |
| 12 | the fraction is padded to eleven digits | `TestIssueBudgetParseUSD`, `TestIssueBudgetRequests`, `TestIssueBudgetUSDTextRoundTrips` | failed test |

### Stage C (`cmd_issue_budget.go`, `cmd_issue_budget_print.go`, `cmd_issue_budget_wire.go`, the line in `cmd_issue.go`): 137 mutants, 137 killed

**While the stage was built: 149 runs in eight batches.** 140 were killed at once and 9 survived. Each survivor got a test, and the same mutant was run again in the next batch and was killed.

| Batch | Runs | Killed | Survived |
| --- | --- | --- | --- |
| c1 | 45 | 44 | 1 |
| c2 | 51 | 48 | 3 |
| c3 | 24 | 20 | 4 |
| c4 | 14 | 13 | 1 |
| c5 | 1 | 1 | 0 |
| c6 | 3 | 3 | 0 |
| c7 | 4 | 4 | 0 |
| c8 | 7 | 7 | 0 |
| all | 149 | 140 | 9 |

The nine that survived:

| Mutant | Survived in | Test added for it | Killed in |
| --- | --- | --- | --- |
| the write keeps GetBody, so a 307 sends its body again | c1 #26 | a new test, `TestIssueBudgetWriteGivesTheTransportNothingToReplay` | c2 #1, by `TestIssueBudgetWriteGivesTheTransportNothingToReplay` |
| an enable result without activated_at is printed by operation | c2 #5 | `TestIssueBudgetOperation`, the answer "a committed enable without its activation time" | c3 #1, by `TestIssueBudgetOperation` |
| a remaining amount of null prints as zero | c2 #28 | `TestIssueBudgetShowReadsLeniently`, "a remaining amount of null that the answer does not explain" | c3 #2, by `TestIssueBudgetShowReadsLeniently` |
| the reservations are printed although the ledger cannot be read | c2 #40 | `TestIssueBudgetAttempts`, "a page without a ledger that carries reservations" | c3 #3, by `TestIssueBudgetAttempts` |
| observations --output json prints a page that was not understood | c3 #9 | `TestIssueBudgetReadOutputJSON`, the cases "an answer that is not understood is not printed: …" (eleven, over the four reads) | c4 #1, by `TestIssueBudgetReadOutputJSON` |
| attempts --output json prints a page that was not understood | c3 #10 | the same cases | c4 #2, by `TestIssueBudgetReadOutputJSON` |
| operation --output json prints an answer whose state is not known | c3 #11 | the same cases | c4 #3, by `TestIssueBudgetReadOutputJSON` |
| a read answer that is too long is reported as no answer | c3 #12 | `TestIssueBudgetReadRefusedOrUnanswered`, "a 200 longer than the command reads" | c4 #4, by `TestIssueBudgetReadRefusedOrUnanswered` |
| a read reports a 5xx with the body of a refusal as refused (read tests only) | c4 #5 | `TestIssueBudgetReadRefusedOrUnanswered`, "a 500 with the body of a refusal" and "a 503 with the body of a refusal" | c5 #1, by `TestIssueBudgetReadRefusedOrUnanswered` |

The last of the nine is a mutant of batch c1 (a 5xx with the body of a refusal is reported as refused). The tests of the writes kill it, and that hid that no test of the reads did; so it was run again with `-run TestIssueBudgetReadRefusedOrUnanswered` alone, and survived until the two cases were added. One more mutant (a read answer is cut at 1 MiB and shown) was run in that restricted way and was killed.

Batches c1 to c5 ran against an earlier `cmd_issue_budget.go` (SHA-256 `392881ed1d7f1fe27250375d01cc4771af5dd117775025e561795793dfac9378`); the final file (`ec955711b780849ee790e8df6be3ddc850a915c4aed94641d65f8ae21b1e53c8`) differs from it in the sentence about a next page whose cursor cannot be put into a command, and in its comment. The other two files did not change after the first batch. Tests were added between the batches and after them. So the batches do not show that the final tests kill every mutant in the final files; the runs below do.

**At the end: every mutant again, against the final tree.** The 149 runs above hold 136 distinct mutants (100 in `cmd_issue_budget.go`, 29 in `cmd_issue_budget_print.go`, 7 in `cmd_issue_budget_wire.go`). `/tmp/f3-7b/mutants_final.py` collects them and keeps the 2 restricted runs as runs of their own (#116 and #117 in the table): 138 runs. They ran twice, 16:26 to 16:36 with `mutate.py` and 17:11 to 17:19 with `mutate2.py`. Both times: 138 killed, 0 survived, 0 not applied, and the same tests named for every mutant. All 138 died by a failed test.

#139 was added at the end: it takes out the one line this lane changed in an existing file.

In the table, "main" is `cmd_issue_budget.go`, "print" is `cmd_issue_budget_print.go` and "wire" is `cmd_issue_budget_wire.go`.

| # | File | Mutant | Caught by |
| --- | --- | --- | --- |
| 1 | main | the credential check is skipped for the reads (subcommands without --yes) | `TestIssueBudgetIsForPeopleOnly` |
| 2 | main | requireHumanLocalCommand is not called | `TestIssueBudgetIsForPeopleOnly` |
| 3 | main | a task token is accepted | `TestIssueBudgetIsForPeopleOnly`, `TestIssueBudgetRefusesAClientThatIsNotAPerson` |
| 4 | main | a daemon token is accepted | `TestIssueBudgetIsForPeopleOnly`, `TestIssueBudgetRefusesAClientThatIsNotAPerson` |
| 5 | main | a cloud token is accepted | `TestIssueBudgetIsForPeopleOnly`, `TestIssueBudgetRefusesAClientThatIsNotAPerson` |
| 6 | main | a client with an agent id is accepted | `TestIssueBudgetRefusesAClientThatIsNotAPerson` |
| 7 | main | a client with a task id is accepted | `TestIssueBudgetRefusesAClientThatIsNotAPerson` |
| 8 | main | a client without a token is let through | `TestIssueBudgetIsForPeopleOnly`, `TestIssueBudgetRefusesAClientThatIsNotAPerson` |
| 9 | main | the workspace is not checked | `TestIssueBudgetNeedsAWorkspace` |
| 10 | main | enable does not check --yes | `TestIssueBudgetRequiresEveryFlag` |
| 11 | main | the operations on a root do not check --yes | `TestIssueBudgetRequiresEveryFlag` |
| 12 | main | --yes=false counts as a confirmation | `TestIssueBudgetRequiresEveryFlag` |
| 13 | main | an amount flag that is left out is read as none | `TestIssueBudgetRequiresEveryFlag` |
| 14 | main | a flag that is left out is read as its empty default | `TestIssueBudgetRequiresEveryFlag` |
| 15 | main | the form of --request-key is not checked | `TestIssueBudgetRefusesAFlagInTheWrongForm` |
| 16 | main | the form of --root is not checked | `TestIssueBudgetRefusesAFlagInTheWrongForm` |
| 17 | main | --expect-revision 0 is accepted | `TestIssueBudgetRefusesAFlagInTheWrongForm` |
| 18 | main | --mode is not checked | `TestIssueBudgetRefusesAFlagInTheWrongForm` |
| 19 | main | a --reason that is not UTF-8 is sent | `TestIssueBudgetRefusesAFlagInTheWrongForm` |
| 20 | main | an --output other than table or json is taken as table | `TestIssueBudgetRefusesAFlagInTheWrongForm` |
| 21 | main | a positional argument is ignored where none is taken | `TestIssueBudgetRequiresEveryFlag` |
| 22 | main | the write is sent twice | `TestIssueBudgetByKeyCommandIsRunnable`, `TestIssueBudgetEscapesTheIssueInThePath`, `TestIssueBudgetRequests`, `TestIssueBudgetWriteBodyBytes`, and 4 more |
| 23 | main | the write is sent again when no answer came back | `TestIssueBudgetWriteOutcomeUnknown` |
| 24 | main | the write is sent again after a 5xx | `TestIssueBudgetByKeyCommandIsRunnable`, `TestIssueBudgetWriteOutcomeUnknown`, `TestIssueBudgetWriteOutputJSON` |
| 25 | main | a redirect is followed | `TestIssueBudgetReadRefusedOrUnanswered`, `TestIssueBudgetWriteGivesTheTransportNothingToReplay`, `TestIssueBudgetWriteOutcomeUnknown` |
| 26 | main | the write keeps GetBody, so a 307 sends its body again | `TestIssueBudgetWriteGivesTheTransportNothingToReplay` |
| 27 | main | an Idempotency-Key header is sent | `TestIssueBudgetRequests`, `TestIssueBudgetWriteGivesTheTransportNothingToReplay` |
| 28 | main | the workspace header is not sent | `TestIssueBudgetByKeyCommandIsRunnable`, `TestIssueBudgetRequests` |
| 29 | main | the issue is put into the path without escaping | `TestIssueBudgetByKeyCommandIsRunnable`, `TestIssueBudgetEscapesTheIssueInThePath` |
| 30 | main | set-limit sends the kind of a set-mode | `TestIssueBudgetRequests`, `TestIssueBudgetWriteCommitted`, `TestIssueBudgetWriteOutcomeUnknown` |
| 31 | main | a 500 with the body of a refusal is reported as refused | `TestIssueBudgetReadRefusedOrUnanswered`, `TestIssueBudgetWriteOutcomeUnknown` |
| 32 | main | a 4xx with a code this command does not know is reported as refused | `TestIssueBudgetWriteOutcomeUnknown` |
| 33 | main | a 4xx whose error and code differ is reported as refused | `TestIssueBudgetWriteOutcomeUnknown` |
| 34 | main | a 401 to a write is reported as an unknown outcome | `TestIssueBudgetWriteOutputJSON`, `TestIssueBudgetWriteRefused` |
| 35 | main | a 200 that does not say replayed is taken as committed | `TestIssueBudgetWriteOutcomeUnknown`, `TestIssueBudgetWriteOutputJSON` |
| 36 | main | a result of another kind is taken as committed | `TestIssueBudgetWriteOutcomeUnknown`, `TestIssueBudgetWriteOutputJSON` |
| 37 | main | a result of another root is taken as committed | `TestIssueBudgetWriteOutcomeUnknown` |
| 38 | main | a 202 is taken as committed | `TestIssueBudgetWriteOutcomeUnknown` |
| 39 | main | an enable result without activated_at is taken as committed | `TestIssueBudgetOperation`, `TestIssueBudgetReadOutputJSON`, `TestIssueBudgetWriteOutcomeUnknown` |
| 40 | main | an unknown outcome after a 4xx gets the exit code of that status | `TestIssueBudgetWriteOutcomeUnknown` |
| 41 | main | the unknown outcome repeats what the transport said | `TestIssueBudgetUnknownOutcomeDoesNotRepeatTheTransport` |
| 42 | main | the policy revision conflict does not send the operator to show | `TestIssueBudgetWriteRefused` |
| 43 | main | the command by key leaves out the profile, server and workspace that were typed | `TestIssueBudgetByKeyCommandIsRunnable` |
| 44 | main | the command by key does not quote its words for the shell | `TestIssueBudgetByKeyCommandIsRunnable`, `TestIssueBudgetObservations` |
| 45 | main | the command by key asks with --root for an enable | `TestIssueBudgetByKeyCommandIsRunnable`, `TestIssueBudgetWriteOutcomeUnknown` |
| 46 | main | a result of another root is printed by operation --root | `TestIssueBudgetOperation`, `TestIssueBudgetReadOutputJSON` |
| 47 | main | a state of a later server is read as unknown | `TestIssueBudgetOperation` |
| 48 | main | the unknown answer is not checked for its retry path | `TestIssueBudgetOperation`, `TestIssueBudgetReadOutputJSON` |
| 49 | main | an enable result without activated_at is printed by operation | `TestIssueBudgetOperation`, `TestIssueBudgetReadOutputJSON` |
| 50 | main | the 404 of the binding is reported like any refusal | `TestIssueBudgetShowByIssue` |
| 51 | main | the root the binding names is used whatever its form | `TestIssueBudgetShowByIssue` |
| 52 | main | show prints the answer about another root | `TestIssueBudgetReadOutputJSON`, `TestIssueBudgetShowAnswerNotUnderstood`, `TestIssueBudgetShowByIssue` |
| 53 | main | show accepts an available ledger without amounts | `TestIssueBudgetReadOutputJSON`, `TestIssueBudgetShowAnswerNotUnderstood` |
| 54 | main | show takes both --root and an issue | `TestIssueBudgetRequiresEveryFlag` |
| 55 | main | operation takes both --root and --issue | `TestIssueBudgetRequiresEveryFlag` |
| 56 | main | --output json prints the answer with its end trimmed | `TestIssueBudgetReadOutputJSON`, `TestIssueBudgetWriteOutputJSON` |
| 57 | main | a read takes any 2xx for an answer | `TestIssueBudgetReadRefusedOrUnanswered` |
| 58 | main | a read that got no answer exits like any error and repeats the transport | `TestIssueBudgetReadRefusedOrUnanswered` |
| 59 | main | a read reports a 401 like any other status | `TestIssueBudgetReadRefusedOrUnanswered` |
| 60 | main | a list accepts --limit 101 | `TestIssueBudgetRefusesAFlagInTheWrongForm` |
| 61 | main | a list accepts --limit 0 | `TestIssueBudgetRefusesAFlagInTheWrongForm` |
| 62 | main | a list sends an empty --cursor | `TestIssueBudgetRefusesAFlagInTheWrongForm` |
| 63 | main | the next page forgets the --limit that was typed | `TestIssueBudgetAttempts`, `TestIssueBudgetObservations` |
| 64 | main | the cursor goes into the query without escaping | `TestIssueBudgetRequests` |
| 65 | main | a next cursor that cannot be shown is put into the command | `TestIssueBudgetObservations` |
| 66 | print | a text of the server reaches the terminal as it came | `TestIssueBudgetObservations`, `TestIssueBudgetShowReadsLeniently` |
| 67 | print | a limit of null prints as zero | `TestIssueBudgetShowTable` |
| 68 | print | the amounts are printed although the ledger cannot be read | `TestIssueBudgetShowAnswerNotUnderstood` |
| 69 | print | amounts of null print as zero | `TestIssueBudgetShowAnswerNotUnderstood`, `TestIssueBudgetShowReadsLeniently`, `TestIssueBudgetShowTable`, `TestIssueBudgetShowWithoutALedgerShowsNoAmount` |
| 70 | print | the remaining amount is printed although the ledger cannot be read | `TestIssueBudgetShowAnswerNotUnderstood`, `TestIssueBudgetShowTable` |
| 71 | print | a remaining amount of null prints as zero | `TestIssueBudgetShowReadsLeniently` |
| 72 | print | a remaining amount of null prints as the limit when all is known | `TestIssueBudgetShowTable` |
| 73 | print | executions included of null prints as zero | `TestIssueBudgetShowTable`, `TestIssueBudgetShowWithoutALedgerShowsNoAmount` |
| 74 | print | the estimate is not marked as outside A and the limit | `TestIssueBudgetShowByIssue`, `TestIssueBudgetShowTable` |
| 75 | print | the reserve is not marked as a declared reservation | `TestIssueBudgetOperation`, `TestIssueBudgetShowByIssue`, `TestIssueBudgetShowTable`, `TestIssueBudgetWriteCommitted` |
| 76 | print | the names that are not counted are not marked as not a zero cost | `TestIssueBudgetShowByIssue`, `TestIssueBudgetShowReadsLeniently`, `TestIssueBudgetShowTable` |
| 77 | print | the unknown before activation is left out | `TestIssueBudgetShowByIssue`, `TestIssueBudgetShowKeepsTheAmountsOfARootThatIsOff`, `TestIssueBudgetShowTable` |
| 78 | print | only the first reason of an unknown is printed | `TestIssueBudgetShowByIssue`, `TestIssueBudgetShowTable` |
| 79 | print | an unsized reservation prints the 0 of the ledger | `TestIssueBudgetAttempts` |
| 80 | print | a reserve of null on an execution prints as zero | `TestIssueBudgetAttempts` |
| 81 | print | an execution without a protocol version prints as version 0 | `TestIssueBudgetAttempts` |
| 82 | print | an execution without a reservation is said to have none whatever the ledger | `TestIssueBudgetAttempts` |
| 83 | print | the reservations are printed although the ledger cannot be read | `TestIssueBudgetAttempts` |
| 84 | print | a replayed write is reported as written now | `TestIssueBudgetWriteCommitted` |
| 85 | print | the stale line is left out | `TestIssueBudgetOperation` |
| 86 | print | the result of any operation prints the lines of an enable | `TestIssueBudgetOperation`, `TestIssueBudgetWriteCommitted` |
| 87 | print | the in-flight count is not marked as unknown from before activation | `TestIssueBudgetOperation`, `TestIssueBudgetWriteCommitted` |
| 88 | print | a detail spread over lines is printed as it came | `TestIssueBudgetObservations` |
| 89 | wire | a field that is missing is read as its zero value | `TestIssueBudgetAttempts`, `TestIssueBudgetObservations`, `TestIssueBudgetOperation`, `TestIssueBudgetReadOutputJSON`, and 4 more |
| 90 | wire | a null where 12.9 allows none is read as the zero value | `TestIssueBudgetAttempts`, `TestIssueBudgetObservations`, `TestIssueBudgetOperation`, `TestIssueBudgetReadOutputJSON`, and 2 more |
| 91 | wire | a key that is there twice is read by its first value | `TestIssueBudgetShowAnswerNotUnderstood`, `TestIssueBudgetWriteOutcomeUnknown` |
| 92 | wire | what follows the answer is ignored | `TestIssueBudgetShowAnswerNotUnderstood`, `TestIssueBudgetShowByIssue`, `TestIssueBudgetWriteOutcomeUnknown` |
| 93 | wire | a detail that is not an object is accepted | `TestIssueBudgetObservations` |
| 94 | wire | a count with a sign or a fraction of zero is accepted | `TestIssueBudgetShowAnswerNotUnderstood` |
| 95 | main | --output json prints the body of a write that was not understood | `TestIssueBudgetWriteOutputJSON` |
| 96 | main | --output json prints the body of a refusal | `TestIssueBudgetWriteOutputJSON` |
| 97 | main | --output json prints a result of another kind | `TestIssueBudgetWriteOutputJSON` |
| 98 | main | --output json prints the body of a 200 that does not say replayed | `TestIssueBudgetWriteOutputJSON` |
| 99 | main | show --output json prints an answer that was not understood | `TestIssueBudgetReadOutputJSON` |
| 100 | main | observations --output json prints a page that was not understood | `TestIssueBudgetReadOutputJSON` |
| 101 | main | attempts --output json prints a page that was not understood | `TestIssueBudgetReadOutputJSON` |
| 102 | main | operation --output json prints an answer whose state is not known | `TestIssueBudgetReadOutputJSON` |
| 103 | main | a read answer that is too long is reported as no answer | `TestIssueBudgetReadRefusedOrUnanswered` |
| 104 | main | an answer is cut at the limit instead of refused | `TestIssueBudgetReadRefusedOrUnanswered`, `TestIssueBudgetWriteOutcomeUnknown` |
| 105 | main | the refusal of a read says that nothing was written | `TestIssueBudgetReadRefusedOrUnanswered` |
| 106 | main | the refusal of a write does not say that nothing was written | `TestIssueBudgetWriteRefused` |
| 107 | main | the unknown outcome proposes a new key | `TestIssueBudgetByKeyCommandIsRunnable`, `TestIssueBudgetUnknownOutcomeDoesNotRepeatTheTransport`, `TestIssueBudgetWriteOutcomeUnknown` |
| 108 | main | an unknown outcome without an answer exits like a network error | `TestIssueBudgetUnknownOutcomeDoesNotRepeatTheTransport`, `TestIssueBudgetWriteOutcomeUnknown` |
| 109 | wire | a key the structs do not know makes an answer not understood | `TestIssueBudgetAttempts`, `TestIssueBudgetObservations`, `TestIssueBudgetOperation`, `TestIssueBudgetReadOutputJSON`, and 4 more |
| 110 | main | operation sends an empty --issue | `TestIssueBudgetRequiresEveryFlag` |
| 111 | main | enable sends an empty issue | `TestIssueBudgetRequiresEveryFlag` |
| 112 | main | show sends an empty issue | `TestIssueBudgetRequiresEveryFlag` |
| 113 | print | the state tombstoned is printed without what it means | `TestIssueBudgetShowTable` |
| 114 | print | an unknown ledger state hides the amounts the answer gives | `TestIssueBudgetShowReadsLeniently` |
| 115 | print | the binding lines are left out of show <issue> | `TestIssueBudgetShowByIssue` |
| 116 | main | a read reports a 5xx with the body of a refusal as refused (read tests only) | `TestIssueBudgetReadRefusedOrUnanswered` |
| 117 | main | a read answer is cut at the limit and shown (read tests only) | `TestIssueBudgetReadRefusedOrUnanswered` |
| 118 | main | show --output json prints the answer before the root and the ledger are checked | `TestIssueBudgetReadOutputJSON` |
| 119 | main | show --output json prints the answer about another root | `TestIssueBudgetReadOutputJSON` |
| 120 | main | show --output json prints amounts of null beside a ledger that can be read | `TestIssueBudgetReadOutputJSON` |
| 121 | main | operation --output json prints the answer once its state is read | `TestIssueBudgetReadOutputJSON` |
| 122 | main | operation --output json prints the operation of another root | `TestIssueBudgetReadOutputJSON` |
| 123 | main | operation --output json prints an unknown answer without its retry path | `TestIssueBudgetReadOutputJSON` |
| 124 | main | operation --output json prints an enable without its activation time | `TestIssueBudgetReadOutputJSON` |
| 125 | main | operation --output json prints a committed answer that was not understood | `TestIssueBudgetReadOutputJSON` |
| 126 | main | an empty next cursor is put into a command that --cursor refuses | `TestIssueBudgetObservations` |
| 127 | main | an empty next cursor is taken for the last page | `TestIssueBudgetObservations` |
| 128 | main | attempts is not checked for the surrounding of a task or a collaboration run | `TestIssueBudgetIsForPeopleOnly` |
| 129 | main | operation is not checked for the credential of a person | `TestIssueBudgetIsForPeopleOnly` |
| 130 | main | enable is not checked for the credential of a person | `TestIssueBudgetIsForPeopleOnly` |
| 131 | main | observations is not checked for the credential of a person | `TestIssueBudgetIsForPeopleOnly` |
| 132 | print | an empty list of what is not counted has no line | `TestIssueBudgetShowReadsLeniently` |
| 133 | print | an empty list of what is not counted prints 0 | `TestIssueBudgetShowReadsLeniently` |
| 134 | print | the amounts that came with a ledger state this command does not know are hidden | `TestIssueBudgetShowReadsLeniently` |
| 135 | main | an empty reason is refused before anything is sent | `TestIssueBudgetWriteBodyBytes` |
| 136 | main | a blank reason is refused before anything is sent | `TestIssueBudgetWriteBodyBytes` |
| 137 | main | a refusal is taken only under the statuses the handlers give today | `TestIssueBudgetWriteRefused` |
| 138 | main | a code is a refusal only under the status it comes with | `TestIssueBudgetWriteRefused` |
| 139 | `cmd_issue.go` | the budget group is not registered under the issue command | `TestIssueBudgetIsRegisteredWithItsSubcommands` |

The texts that were replaced are in `/tmp/f3-7b/mutants_b.py`, `mutants_c1.py` to `mutants_c8.py` and `mutants_x.py`; the results in `mutants-b.md`, `mutants-c1.md` to `mutants-c8.md`, and for the final tree `mutants-b-final.md`, `mutants-b-final2.md`, `mutants-final.md`, `mutants-final2.md` and `mutants-x.md`. These files are outside the repository and do not survive a restart of the machine (section 6).

### Round 2: the commands (19 new mutants, and every earlier one again)

Same driver, with the outputs of every run kept (`/tmp/f3-7b/r2/mutate3.py`, spec `/tmp/f3-7b/r2/mutants_s2.py`, outputs `/tmp/f3-7b/r2/mutant-out-s2/`). The 19 mutants are for what round 2 added: the help paragraph, the reading of a 4xx that is not a refusal, and the quoting of a text of the server.

**Run 1** (2026-11-19, 21:11 to 21:14 CST): 18 killed, 1 survived. The survivor was #8, "a 404 with a code this command does not know is taken for not_found": no row had a 404 whose `error` and `code` are equal and hold a code that 12.3 does not have. A row with that answer was added to the table of the writes and to the table of the reads (section 2). **Run 2** (21:18 to 21:19 CST), on the tree with that row: 19 of 19 killed, each by a failed test, each file copied back with an equal SHA-256. The table is run 2.

| # | Mutant | Killed by |
| --- | --- | --- |
| 1 | the help does not say how to read what became of a write | `TestIssueBudgetHelpSaysHowToTellTheOutcomesApart` |
| 2 | the help quotes other first words for a refusal than the commands print | `TestIssueBudgetHelpSaysHowToTellTheOutcomesApart` |
| 3 | the help quotes other first words for the unknown outcome than the commands print | `TestIssueBudgetHelpSaysHowToTellTheOutcomesApart` |
| 4 | a 4xx with a code this command does not know is reported as refused | `TestIssueBudgetReadRefusedOrUnanswered`, `TestIssueBudgetWriteOutcomeUnknown` |
| 5 | a 4xx whose error and code differ is reported as refused | `TestIssueBudgetWriteOutcomeUnknown` |
| 6 | any 4xx with the two keys of the error shape is reported as refused | `TestIssueBudgetReadRefusedOrUnanswered`, `TestIssueBudgetWriteOutcomeUnknown` |
| 7 | a 404 with the two keys of the error shape is taken for not_found, whatever they hold | `TestIssueBudgetReadRefusedOrUnanswered`, `TestIssueBudgetWriteOutcomeUnknown` |
| 8 | a 404 with a code this command does not know is taken for not_found | `TestIssueBudgetReadRefusedOrUnanswered`, `TestIssueBudgetWriteOutcomeUnknown` |
| 9 | a text of the server reaches the terminal as it came | `TestIssueBudgetObservations`, `TestIssueBudgetServerTextCannotDrawOnTheTerminal`, `TestIssueBudgetShowReadsLeniently` |
| 10 | no text of the server is quoted | `TestIssueBudgetObservations`, `TestIssueBudgetServerTextCannotDrawOnTheTerminal`, `TestIssueBudgetShowReadsLeniently` |
| 11 | an empty text of the server prints as nothing | `TestIssueBudgetObservations`, `TestIssueBudgetServerTextCannotDrawOnTheTerminal` |
| 12 | an escape character is let through | `TestIssueBudgetObservations`, `TestIssueBudgetServerTextCannotDrawOnTheTerminal` |
| 13 | a line feed is let through | `TestIssueBudgetServerTextCannotDrawOnTheTerminal` |
| 14 | a carriage return is let through | `TestIssueBudgetServerTextCannotDrawOnTheTerminal` |
| 15 | the reason of a result is printed as it came | `TestIssueBudgetServerTextCannotDrawOnTheTerminal` |
| 16 | the kind of an observation is printed as it came | `TestIssueBudgetServerTextCannotDrawOnTheTerminal` |
| 17 | the reason of an observation is printed as it came | `TestIssueBudgetServerTextCannotDrawOnTheTerminal` |
| 18 | a state this command does not know is printed as it came | `TestIssueBudgetServerTextCannotDrawOnTheTerminal` |
| 19 | a mode this command does not know is printed as it came | `TestIssueBudgetServerTextCannotDrawOnTheTerminal`, `TestIssueBudgetShowReadsLeniently` |

**Which rows stand between a mutant and a pass** (round 2 message, 2.2). The round 2 message asked for one mutant of `budgetRefusal` that leaves the membership check's 404 as the only thing between the mutant and a pass, "without the comparison of `error` with `code`, or without the list of known codes". Neither of those two is such a mutant, and the report says so instead of naming one:

- The 404 of the membership check (`{"error":"workspace not found","code":"workspace_access_denied"}`) fails both checks of `budgetRefusal` at once: its two values differ, and its code is not one of 12.3. With one check dropped the other still turns it away.
- #4 (the list of known codes dropped) is killed by the older rows `a 409 with a code this command does not know` and `a 409 with internal_error as its code` (10 subtests), and since run 2 also by the new row `a 404 with a code this command does not know` (5 writes, 4 reads). Not by the membership 404.
- #5 (the comparison dropped) is killed by the older row `a 409 whose two codes differ` (5 subtests). Not by the membership 404.
- #6 (both dropped) is killed by those 15 subtests and by the membership 404 rows (5 writes, 4 reads).
- #7 ("a 404 with the two keys of the error shape is taken for not_found, whatever they hold") is the mutant that only the membership 404 rows kill: 5 subtests of `TestIssueBudgetWriteOutcomeUnknown` and 4 of `TestIssueBudgetReadRefusedOrUnanswered`, and nothing else.
- The 403 of the role check (`{"error":"insufficient permissions"}`) kills none of them on its own: it has the shape of the older row `a 403 without a code`. It is in the tables for its exact bytes.

**The quoting** (round 2 message, 2.3). #13 (a line feed is let through), #14 (a carriage return), #15 (the reason of a result), #16 (the kind of an observation), #17 (the reason of an observation) and #18 (a state the command does not know) are killed only by the new test. #9 and #10 (no quoting at all), #11 (an empty text), #12 (an escape character) and #19 (a mode) were killed by older tests as well.

**Every earlier mutant again, on the round 2 tree** (2026-11-19, 21:27:22 to 21:38:41 CST, `/tmp/f3-7b/r2/run-mutants-r2-all.out`): stage C 138 of 138 killed, each by a failed test; stage B 11 of 11 (10 by a failed test, #10 by a panic inside a test, as before); the 2 extra ones 2 of 2. Each file was copied back with an equal SHA-256, and `shasum -a 256 -c` over the ten Go files printed `OK` ten times afterwards. With the 19 above that is 170 runs on the round 2 tree and 170 killed. The ten SHA-256 values after these runs are the ones of section 5 ("The tree as it is delivered"): no Go file changed after the mutants ran. Results: `/tmp/f3-7b/r2/mutants-final3.md`, `mutants-b-final3.md`, `mutants-x3.md`, `mutants-s2.md`.

### Round 2: Core (stage D): 107 mutants, 107 killed

Driver: `/tmp/f3-7b/r2/stage-d/mutate_ts.py <spec.py> <results.md>`. For each mutant it replaces one piece of text that occurs exactly once in `issue-budget-schemas.ts` or `client.ts`, runs `pnpm --filter @multica/core exec vitest run api/issue-budget-schemas.test.ts`, copies the file back and compares its SHA-256 with the one taken before; the output of each run is kept (`mutant-out/NNN.txt`). A mutant is killed when the run names failed tests, and survives when the run exits 0. A run that exits with an error and names no failed test (a file that does not load) is counted as killed under its own heading; there was none. Type errors do not count: vitest does not check types, so each mutant is judged by the tests alone. Spec: `mutants_d.py`.

**Run 1** (2026-11-20, 01:03:57 to 01:06:59 CST, 105 mutants): 103 killed, 2 survived, "the syntax error is logged" and "the text of the syntax error is logged". Cause: on this runtime only an "Unexpected token" error quotes the body (about ten characters on each side); the one malformed body the test had then gave "Expected ',' or '}' after property value in JSON at position 319", which quotes nothing, and the log search compared whole values only, so ten characters of an amount would not have been found either. The defect was in the test, not in the method. Fixed in the test file only: the search goes over every 8 characters in a row of every part of the body; three bodies were added for which the runtime does quote the answer (`NaN`, a number with a plus sign, a state without its quotes); two checks of the check were added. Two mutants were added to the spec ("an answer of null is taken without a word in the log", "every field of the answer may be absent"). Run 1 is kept as `mutants-d-run1.md` and `mutant-out-run1/`.

**Run 2** (01:14:54 to 01:18:04 CST, 107 mutants, the final test file): 107 of 107 killed, each by failed tests; the three files were restored (`shasum -a 256 -c`: `OK` three times) and no backup file was left. The table is run 2. "Tests" is how many of the 92 tests failed.

The five kinds the round 2 message names are among them: a decimal field that accepts a number (#1, #2), a nullable made optional (#18 to #25), an object made strict (#69 to #73), an enum made a closed list (#79 to #83), the fallback made an object of zeros (#86).

| # | Mutant | Tests |
| --- | --- | --- |
| 1 | a decimal field accepts a number | 7 failed |
| 2 | a decimal field turns a number into a string | 7 failed |
| 3 | a decimal field is any string | 10 failed |
| 4 | a decimal may have leading zeros | 2 failed |
| 5 | a decimal may have a sign | 2 failed |
| 6 | a decimal may have a minus sign only | 1 failed |
| 7 | a decimal may have a plus sign only | 1 failed |
| 8 | the pattern of a decimal is not anchored at its start | 8 failed |
| 9 | the pattern of a decimal is not anchored at its end | 5 failed |
| 10 | the pattern of a decimal matches a line of a text | 2 failed |
| 11 | a decimal may be empty | 1 failed |
| 12 | a decimal may have a fraction | 1 failed |
| 13 | a decimal may have an exponent | 1 failed |
| 14 | a decimal may have spaces around it | 2 failed |
| 15 | a decimal is at most fifteen digits | 1 failed |
| 16 | a decimal comes out as a number | 18 failed |
| 17 | a decimal of zero is not accepted | 17 failed |
| 18 | settled_as_of_revision may be absent | 1 failed |
| 19 | limit_ticks may be absent | 1 failed |
| 20 | attempt_reserve_ticks may be absent | 1 failed |
| 21 | remaining_ticks may be absent | 1 failed |
| 22 | amounts may be absent | 1 failed |
| 23 | included_attempts may be absent | 1 failed |
| 24 | an absent limit_ticks is read as null | 1 failed |
| 25 | absent amounts are read as null | 1 failed |
| 26 | settled_as_of_revision may not be null | 17 failed |
| 27 | limit_ticks may not be null | 1 failed |
| 28 | attempt_reserve_ticks may not be null | 1 failed |
| 29 | remaining_ticks may not be null | 17 failed |
| 30 | amounts may not be null | 1 failed |
| 31 | included_attempts may not be null | 1 failed |
| 32 | ledger_state may be absent | 1 failed |
| 33 | ledger_state may be null | 1 failed |
| 34 | an absent ledger_state is read as available | 1 failed |
| 35 | admission_state may be absent | 1 failed |
| 36 | an absent admission_state is read as open | 1 failed |
| 37 | admission_state may be a number | 2 failed |
| 38 | mode may be null | 1 failed |
| 39 | total_known may be absent | 1 failed |
| 40 | total_known may be null | 1 failed |
| 41 | an absent total_known is read as false | 1 failed |
| 42 | total_known is read from any value | 3 failed |
| 43 | activated_at may be null | 1 failed |
| 44 | budget_root_id may be absent | 1 failed |
| 45 | root_issue_id may be a number | 1 failed |
| 46 | policy_revision may be a number | 1 failed |
| 47 | policy_epoch may be a number | 1 failed |
| 48 | ledger_revision may be null | 1 failed |
| 49 | settled_as_of_revision may be a number | 1 failed |
| 50 | remaining_ticks may be a number | 1 failed |
| 51 | an amount of the ledger may be a number | 1 failed |
| 52 | an amount of the ledger may be absent | 1 failed |
| 53 | an absent amount of the ledger is read as zero | 1 failed |
| 54 | an amount of the ledger may be null | 1 failed |
| 55 | the unknown part may be absent | 1 failed |
| 56 | pre_activation may be absent | 1 failed |
| 57 | an absent pre_activation is read as zero | 1 failed |
| 58 | the reasons may be null | 1 failed |
| 59 | a reason may lack its count | 1 failed |
| 60 | a reason may be a number | 1 failed |
| 61 | the coverage may be absent | 1 failed |
| 62 | the list of what is not counted may be absent | 1 failed |
| 63 | an absent list of what is not counted is read as empty | 1 failed |
| 64 | a name that is not counted may be null | 1 failed |
| 65 | a count may be a string | 3 failed |
| 66 | a count may have a fraction | 1 failed |
| 67 | a count may be below zero | 2 failed |
| 68 | a count is a decimal string | 18 failed |
| 69 | the answer is strict | 1 failed |
| 70 | the amounts are strict | 1 failed |
| 71 | the unknown part is strict | 1 failed |
| 72 | a reason is strict | 1 failed |
| 73 | the coverage is strict | 1 failed |
| 74 | the answer keeps an unknown field | 1 failed |
| 75 | the amounts keep an unknown field | 1 failed |
| 76 | the unknown part keeps an unknown field | 1 failed |
| 77 | a reason keeps an unknown field | 1 failed |
| 78 | the coverage keeps an unknown field | 1 failed |
| 79 | admission_state is a closed list | 1 failed |
| 80 | mode is a closed list | 1 failed |
| 81 | ledger_state is a closed list | 1 failed |
| 82 | a reason is a closed list | 1 failed |
| 83 | what is not counted is a closed list | 1 failed |
| 84 | a state this version does not know is read as closed | 3 failed |
| 85 | a settled revision must be null | 1 failed |
| 86 | the fallback is an object of zeros | 65 failed |
| 87 | a failed parse logs what was received | 64 failed |
| 88 | a failed parse is not redacted (no option) | 64 failed |
| 89 | the logged endpoint holds the ID that was asked for | 64 failed |
| 90 | a malformed answer is an error and not null | 65 failed |
| 91 | the answer is cast and not parsed | 70 failed |
| 92 | an answer of null is taken without a word in the log | 1 failed |
| 93 | every field of the answer may be absent | 13 failed |
| 94 | a body that is not JSON is an error | 7 failed |
| 95 | a body that is not JSON gives null without a warning | 7 failed |
| 96 | the syntax error is logged | 3 failed |
| 97 | the text of the syntax error is logged | 3 failed |
| 98 | every failure of the request gives null | 3 failed |
| 99 | a refusal gives null | 2 failed |
| 100 | the ID is not escaped | 1 failed |
| 101 | the ID is escaped as a whole URI | 1 failed |
| 102 | the request goes to the issue's own path | 9 failed |
| 103 | the request names a sub-path | 60 failed |
| 104 | the workspace that was given is not sent | 1 failed |
| 105 | the caller's signal is not passed on | 1 failed |
| 106 | the request is a POST | 1 failed |
| 107 | the request has a body | 1 failed |

73 of the 107 mutants are killed by exactly one test: for those rules one row is all that holds them. For example #92, which lets the schema take `null`, is caught only by `gives no answer for an answer of null`, and #6 and #7 (a minus sign only, a plus sign only) only by the row of that sign. #96 and #97 (the syntax error, or its text, is logged) are killed by exactly the three bodies that run 1 lacked.


## 5. Verification

The first five parts are round 2 and describe the tree as it is delivered. The three parts after them are the record of round 1, kept as they were accepted; their counts and four of their ten SHA-256 values are superseded by the round 2 parts. The last two parts are about the commands and hold for both rounds.

### The round 2 pass (2026-11-20)

One pass, 2026-11-20 03:58:42 to 04:01:36 CST, by one script that runs one command after another: `/tmp/f3-7b/r2/final-verify.sh` (with `/bin/bash`, from the worktree root, `DATABASE_URL` unset; for the Go commands `TMPDIR` is the temporary directory's path without a symbolic link). It ran after the last change of a file of this lane (a reference file of the skill, 03:48:26). Before and after the pass the script took the SHA-256 of the ten Go files, the 26 fixtures, the three Core files and the four files of the skill, and the output of `git status --short`; each is the same before and after. After the pass only commands that read ran in the worktree. Its log is `/tmp/f3-7b/r2/final/steps.txt`.

The default toolchain is go1.27.0 (`/opt/homebrew/bin/go`). The second one is go1.26.9, taken with `GOTOOLCHAIN=go1.26.9 GOPROXY=off`. "The ten files" are `cmd_issue.go` and the nine `cmd_issue_budget*.go` in `server/cmd/multica/`. Node is v22.23.2, pnpm 10.28.2, vitest 4.1.0.

| Item | Command | Toolchain | Exit | Result |
| --- | --- | --- | --- | --- |
| 1 | `gofmt -l <the ten files>` | go1.27.0 (`/opt/homebrew/bin/gofmt`) | 0 | prints nothing |
| 1 | `"$(GOTOOLCHAIN=go1.26.9 GOPROXY=off go env GOROOT)/bin/gofmt" -l <the ten files>` | go1.26.9 | 0 | prints nothing |
| 2 | `go -C server build ./...` | go1.27.0 | 0 | prints nothing |
| 2 | `go -C server build ./...` | go1.26.9 | 0 | prints nothing |
| 2 | `go -C server vet ./cmd/multica/...` | go1.27.0 | 0 | prints nothing |
| 2 | `go -C server vet ./cmd/multica/...` | go1.26.9 | 0 | prints nothing |
| 3 | `go -C server test ./cmd/multica/ -run TestIssueBudget -count=1 -v` | go1.27.0 | 0 | PASS 36, FAIL 0, SKIP 0; 875 at every depth |
| 3 | the same with `-race` | go1.27.0 | 0 | PASS 36, FAIL 0, SKIP 0; 875 at every depth |
| 3 | `go -C server test ./cmd/multica/ -run TestIssueBudget -count=1 -v` | go1.26.9 | 0 | PASS 36, FAIL 0, SKIP 0; 875 at every depth |
| 4 | `bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test ./cmd/multica/... -count=1 -v` | go1.27.0 | 0 | PASS 526, FAIL 0, SKIP 0; 2161 at every depth |
| 4 | the same | go1.26.9 | 0 | PASS 526, FAIL 0, SKIP 0; 2161 at every depth |
| 5 | `pnpm --filter @multica/core exec vitest run api/issue-budget-schemas.test.ts` | node v22.23.2 | 0 | 1 file passed, 92 tests passed |
| 5 | `pnpm --filter @multica/core typecheck` (`tsc --noEmit`) | | 0 | no error |
| 5 | `pnpm --filter @multica/core lint` (`eslint .`) | | 0 | no error, no warning printed |
| 5, beyond the brief | `pnpm --filter @multica/core exec vitest run` (the whole package) | | 0 | 366 files passed, 5165 tests passed |
| 6 | `bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test ./internal/service/ -run '^(TestBuiltinSkillsConformToTemplate|TestBuiltinSkillsFrontmatterIsStrictYAML|TestWorkingOnIssuesSkillCoversIssueLoopContracts|TestLoadRequestedAgentSkillBundles_OnboardingScope|TestBuildAgentSkillBundlesAssignsBuiltinID)$' -count=1 -v` | go1.27.0 | 0 | PASS 5, FAIL 0, SKIP 0; 27 at every depth |
| 6 | the same | go1.26.9 | 0 | PASS 5, FAIL 0, SKIP 0; 27 at every depth |

The Go counts are the top-level `--- PASS`, `--- FAIL` and `--- SKIP` lines; "at every depth" counts the `--- PASS` lines of tests and subtests together. No run has a `--- FAIL` or a `--- SKIP` line at any depth.

**Item 3.** 36 tests and 839 subtests. Round 1 had 34 and 790: round 2 added 2 tests with 22 subtests and 27 subtests to two tables (section 2).

**Item 4 against the baseline.** The baseline is the same command before the first change of this lane (`/tmp/f3-7b/baseline.txt`, 2026-11-19 04:35).

| Run | Toolchain | Exit | PASS | FAIL | SKIP | `--- PASS` at every depth |
| --- | --- | --- | --- | --- | --- | --- |
| baseline | go1.27.0 | 0 | 490 | 0 | 0 | 1286 |
| end of round 1 | go1.27.0 | 0 | 524 | 0 | 0 | 2110 |
| now | go1.27.0 | 0 | 526 | 0 | 0 | 2161 |
| now | go1.26.9 | 0 | 526 | 0 | 0 | 2161 |

Only PASS moved, by 36. Of the names of the top-level tests that passed, 36 are in the run of now only, and each starts with `TestIssueBudget`; none is in the baseline only. That holds on both toolchains. At every depth the difference is 875: the 36 tests and their 839 subtests.

**The guard.** `scripts/go-test-with-agent-cli-guard.sh` puts a stand-in for every name of `scripts/agent-cli-command-names.txt` first in `PATH` and exits 1 when one of them was started. It exited 0 in the four runs of items 4 and 6, with the exit code of the go command: no test started an agent CLI.

**Item 5 against the baseline.** Before any change in `packages/core` (2026-11-19, 22:11:48 to 22:13:01 CST, `/tmp/f3-7b/r2/core-baseline/`): `typecheck` exit 0, `lint` exit 0, the whole package exit 0 with 365 files and 5073 tests. Now 366 files and 5165 tests: one file and 92 tests more, and nothing else moved.

**Item 6.** These five are the tests of `server/internal/service` that name the skill or read every built-in skill, and that need no database. Before stage E the same command gave the same counts (2026-11-20 03:13:08 to 03:13:21 CST, `/tmp/f3-7b/r2/skill-tests/before-named.out`: exit 0, 5 and 27, no skip). Skipped: none, so there is no name to list. Why these five cannot reach a database: the package has no `TestMain`; the three files that hold them (`builtin_skills_test.go`, `builtin_skill_scope_test.go`, `skill_bundle_test.go`) name no pool, no `DATABASE_URL` and no database address; and the `-run` pattern is anchored at both ends, so no other test of the package starts.

**Tests that name the skill and need a database: two, not run by this lane.** Both are in `server/internal/handler`, whose `TestMain` refuses to start without an explicit `DATABASE_URL`. They are for the main session to run on a clone of its own:

- `TestClaimTaskByRuntime_BuiltinSkillsFollowOnboardingScope` (`server/internal/handler/daemon_builtin_skills_scope_test.go`): claims a task and compares the names of the built-in skills in the claim with a list that holds `multica-working-on-issues`. It goes through the embedded skill but compares names only; the hashes in it (`sha256:stale`) are made-up references for `multica-onboarding` and `multica-mentioning`.
- `TestWriteMeasuredJSONByteIdenticalToWriteJSON` (`server/internal/handler/handler_writejson_test.go`): encodes a made-up skill that has this name. It does not read the embedded skill.

Neither pins the text that stage E changed, so neither is expected to move.

### The tree as it is delivered

HEAD is `86fea651cc5d09360327b6cf0f8357c1f360070d`, detached. Nothing is committed, staged, stashed or reset by this lane (`git diff --cached --name-only` prints nothing; the newest entry of the repository's shared stash list is of 2026-11-09). `git status --short` has 73 lines: the 56 lines of `/tmp/f3-main-s7b/base-status.txt`, each still there as it was, and these 17:

```
 M packages/core/api/client.ts                      (29 lines added, none removed)
 M server/cmd/multica/cmd_issue.go                  (1 line added, none removed)
 M server/internal/service/builtin_skills/multica-working-on-issues/SKILL.md                                   (1 line changed)
 M server/internal/service/builtin_skills/multica-working-on-issues/references/report-snapshots.md             (19 lines added)
 M server/internal/service/builtin_skills/multica-working-on-issues/references/working-on-issues-source-map.md (11 lines added)
?? packages/core/api/issue-budget-schemas.test.ts
?? packages/core/api/issue-budget-schemas.ts
?? server/cmd/multica/cmd_issue_budget.go
?? server/cmd/multica/cmd_issue_budget_fixtures_test.go
?? server/cmd/multica/cmd_issue_budget_money.go
?? server/cmd/multica/cmd_issue_budget_money_test.go
?? server/cmd/multica/cmd_issue_budget_print.go
?? server/cmd/multica/cmd_issue_budget_read_test.go
?? server/cmd/multica/cmd_issue_budget_test.go
?? server/cmd/multica/cmd_issue_budget_wire.go
?? server/cmd/multica/cmd_issue_budget_write_test.go
?? server/internal/handler/testdata/issue_budget_http_v1/   (26 files)
```

The offline install of round 2 left three ignored directories that git does not show: `node_modules/`, `packages/core/node_modules/`, `packages/eslint-config/node_modules/`. Without them and without `.git`, `find . -newer /tmp/f3-7b/baseline.out -type f` lists 42 files, and every one is this lane's: the ten Go files, the 26 fixtures, the three Core files and the three skill files. So no file of the earlier steps was written since the baseline was taken.

SHA-256 of every file of this lane, taken after the pass. `server/cmd/multica/` (the same values as after the mutant runs of round 2, `/tmp/f3-7b/r2/sha-go-r2.txt`):

```
046b79e87f30d385b9d159c154e301f96aa6a5a26bdb2a702cb0b0d718764a7a  cmd_issue_budget.go
e1d147fa3bd1918235bf97bc887f4d9a2d9634dbf83989fa8642cb1f0a88121b  cmd_issue_budget_fixtures_test.go
fa8956b056d5226eb1e7e37816c7945b873af60faaffaf618cdab191d6ac7e7e  cmd_issue_budget_money.go
5b556717c58be40eab9265c2aeda93112c3706a8de010b64dc89bc701d7e6000  cmd_issue_budget_money_test.go
901503bfbb571fff72fcb8c9ec8e45fb5ebd6e28c0e554d6155b22be969a99e0  cmd_issue_budget_print.go
14426e959955c56ba1c57301fb4d32d55ecc9209dc3143b5f1fb7a9d8375c38a  cmd_issue_budget_read_test.go
619212c1b2559cf763da303dec84da642ee647d1631339ad26550927b42659b8  cmd_issue_budget_test.go
264a0c29ac31ba5ec71bc74c320acfa1500e43fbc82b02f9962ce77df981f86d  cmd_issue_budget_wire.go
3edda5e7e2c0dfa1487725f84c61b881cd58b9601d2e365b3c139dbaf46437c0  cmd_issue_budget_write_test.go
dba53918048f6538ddae83396d68dc6feedeb67fdb1311f21a9473a31b067eb8  cmd_issue.go
```

`server/internal/handler/testdata/issue_budget_http_v1/` (the 26 values of round 1, `/tmp/f3-7b/sha-fixtures.txt`; no fixture was written in round 2):

```
45f78bb7e5fc62f90f8308dd0822ea96355e513ed7c0b297b8d6f40d9193b037  attempts-empty.json
0ee9abfb0d5924087211b586ee85eb5a42743032468b1bfab479dcaca0cb21b6  attempts-page.json
d81311696e24517f2a04744717d797e88da944d83f686caa4ca061875ee63e3f  attempts-unavailable.json
a95a16009a1dbb2530ab881c442841aedf4d036fd534917266fac275175c8260  enable-request.json
43819e9d86c3e584709bced68079a17c28fa05d0cd3ef1a29d93f76ebf8f6e76  error-budget-forbidden.json
98893d5be12cd330e45ac75a41ae8a6165d8e8c823ab56cd5a6ae4f5b7220895  error-not-found.json
eb71a6c0ec85bc9645478fc0dd418297127b48fc20d81e35254024c010fa9db1  error-policy-revision-conflict.json
f2870ca0f1e73863f41e6c60afb9879e940c97c7a2e8f9311c6d5dc14d660757  issue-binding.json
7b7b52f31f42f9b4eeb10936172dae1d2cc1204c353a444e935da606b485224c  observations-empty.json
703a1c4705b205924b0fee80d1c7eccf071d5aaa10ee280f54c1057c0b8f86a4  observations-last-page.json
f40836b46bceb5a387e28c13bc7b2bf0618c78f2321bc817a935887878b30a0d  observations-page.json
56c427c290fb824dbd19ed6910cf60a06726d7da57208b971533740fc61f0cef  operation-close-request.json
50a7b3ec66c23cf2807b67c3d08d4b0f9906d2ac7809b41a6e4c87d4987c65e4  operation-committed-stale.json
be3b2599aa983652431008a71108308a42fa77cc131f890fc365be8cf2cbe443  operation-committed.json
2a0219ea73cc7137dc5c7647f7b7e07fdd5bbccfab12110e8875d28ca31649a8  operation-reopen-request.json
648f0f28a6eb6d99b8d10b72ba8483909bc77a49775f21cc061c2d768deff75e  operation-set-limit-request.json
02604c001df66729b09a939ff78aafc2fefc9943fb57b826882dc3c84ffca3e0  operation-set-mode-request.json
549c6959b77dad94f3f13d59bdfc5352cae3f343edf80d38068b4aeeff8dbc03  operation-unknown.json
5add911f285e14c441fb1c65b5aad76bc18ed1302545daad641e99977215678e  show-known.json
939133c9d18ca0080056fbede45e96ffddd666cc0b7c0b9e2443e6c8dcdd6a9d  show-no-limit.json
006534c26c0df9c5ca7785a1545b5b7d79a764cac45159f4d07fd2531c151954  show-open.json
db7f9e48902a557ee5f9c497a9ecef4c73acd95c60abf1f29832ccd374013ebf  show-tombstoned-off.json
3e8f62579af1337527d896fe401e21c7502cc7bf7fdd5b5c9b0ee7aa47205cf6  show-unavailable.json
184ebce53050763552c2811ad8abc7b1b66209bf2255f0bd023705b996457de2  write-200-replayed-set-limit.json
bfee615b585e3e39f1e922b03c06047c3689463ed246f0dcf9cb3e41a5048730  write-201-enable-no-gaps.json
94e2d4d52703ae56fb9ca8c0a419d51a37b177953364a8a798e4766137a45030  write-201-enable.json
```

`packages/core/api/` (the same values as before and after the 107 mutants; `client.ts` was `89fd63d8439b60882c1a33297172d3595718d7357a505950502c1698516a83a4` before this lane's change):

```
31c6fe9306cb69ff167420f2fde5a6ace08951e05764dc2010243e1010a060af  packages/core/api/issue-budget-schemas.ts
2d440eb42ab8957cbe17ea5c5bbc45f4aaee921bb3c2cc07bd8c074855697744  packages/core/api/issue-budget-schemas.test.ts
5a1623cb2d4824f85a96ea3a40783e944b278eb5d80fa097f5a26b76f5ed87c6  packages/core/api/client.ts
```

`server/internal/service/builtin_skills/multica-working-on-issues/`:

```
8b0b00acb5c51fe4612d01d4930e253be10a4b185b36392b08ab313b8d06b569  SKILL.md
f06fb450b15766b2600d91da3ff98c08d5e2852783f0ef1c6f1a05e1d2aa1249  references/collaboration-workflow.md
8e8b74cd7a14648129a28d29c81e6ffbaeb71f5df6278c700dd9b3012f6bffa3  references/report-snapshots.md
ea8911d9a38bff2a27450b16547dd6a346a90c943c8b782f3036860aef192cb5  references/working-on-issues-source-map.md
```

Before stage E the three changed files of the skill were `a24cc7256b9f38d4f942c2e1276be95fe9f8bc37cbe82551884e0c2253ed80f6` (`SKILL.md`), `04c58c51db44345b21aa9f963c1ec29f94a820a597f90f4411e54cf0274eb3a3` (`references/report-snapshots.md`) and `43ed97a0a49464d3ad30fa1f2a3086aaa4fc7a05f005f1cd747b43897c4f7faa` (`references/working-on-issues-source-map.md`). `references/collaboration-workflow.md` is unchanged.

`server/internal/service/builtin_skills_test.go` (one anchor line, R6), changed in the closing turn of 2026-11-20, after the round 2 pass:

```
e03ed1ceed917a2bf36736ca4576482d2fe6ee7f49afa80873320501c74a507e  server/internal/service/builtin_skills_test.go
```

Before the change it was `35639312170a4a0855f39f4c802cd4893be825a43e0b88c96b58db66bc26a670`; the file is committed at HEAD and was not one of the 56 dirty paths. It is the eighteenth status line of this lane, ` M server/internal/service/builtin_skills_test.go` (1 line added, none removed), and with it `git status --short` has 74 lines. The counts further up in this part (73 lines, 17 of this lane, 42 files newer than the baseline) are those of the round 2 pass. After the change (08:05 CST) `shasum -a 256 -c` over the four lists above printed `OK` for all 43 files: 10 Go files, 26 fixtures, 3 Core files and 4 files of the skill.

### What pins a built-in skill's text

The question of brief stage E and of the round 2 message (section 3, item 2). Searched: `server/internal/service`, `server/internal/handler` and `server/cmd/server`, tests and other files, with `/tmp/f3-7b/r2/pin-search.sh` (it only reads; its output is `pin-search.out`).

| Searched for | Found | Had to move |
| --- | --- | --- |
| the skill's name, `working-on-issues`, in Go files | 11 lines in 5 test files: `builtin_skills_test.go` (6), `skill_bundle_test.go` (2), `builtin_skill_scope_test.go` (1) in `service`; `daemon_builtin_skills_scope_test.go` (1), `handler_writejson_test.go` (1) in `handler`. Nothing in `server/cmd/server` | no. The three of `service` pass unchanged (item 6); the two of `handler` need a database and compare names only (above) |
| the directory name `builtin_skills` | one file, `server/internal/service/builtin_skills.go`: it embeds the directory (`//go:embed`) and loads every skill at run time, `SKILL.md` and every other file as a supporting file | no. No list of files and no manifest: the three files that stage E changed are loaded as they are |
| `source-map` | 22 lines, all in `builtin_skills_test.go`: for each general skill the test asks that its source map exists as a supporting file | no. The file exists; its content is not compared |
| a digest: a literal of 64 hex digits | 14 lines in 7 test files (`issue_budget_test.go`, `delivery_completion_types_test.go`, `knowledge_preparation_isolation_test.go`, `issue_budget_teardown_test.go`, `rk1_runtime_continuation_test.go`, `iteration_lifecycle_routes_test.go`, `main_test.go`); none of the lines holds the word skill | no |
| `builtin` on a line with `version`, `digest`, `hash` or `sha256`, in any case | 39 lines. 7 hold `role` and are about role skills, which this lane did not touch. The other 32 were read one by one: the version of the knowledge rules and the knowledge invocations (21 lines), the template key and version of built-in autopilots, reports and MCP servers (5), the built-in agent avatar (1), the made-up `sha256:stale` references of `daemon_builtin_skills_scope_test.go` (4, for `multica-onboarding` and `multica-mentioning`), and one comment of `builtin_agent_templates_test.go` (line 407) that names the version rule of `.trellis/spec/server/builtin-templates.md`. That rule, and the test under the comment (`TestGovernanceSkills_FailureExampleContracts`), are about role skills (`RoleSkillTemplateByName`) | no. None is a version or a hash of a general built-in skill |
| the reference file by its name, `report-snapshots.md`, in Go files under `server/` | nothing | no |

So a general built-in skill has no version, digest or hash that is stored anywhere: the hash of a skill bundle is computed from the loaded content when a task is claimed (`BuildAgentSkillBundles`). What the repository requires of a changed text is what `TestBuiltinSkillsConformToTemplate`, `TestBuiltinSkillsFrontmatterIsStrictYAML` and `TestWorkingOnIssuesSkillCoversIssueLoopContracts` check, and they pass. The one of their limits that mattered: the body of `SKILL.md` was already at the 500 lines the first of them allows, so the sentence was appended to an existing line.

Outside the three places the skill is named in: `server/internal/daemon/execenv` (the runtime brief points to the skill, and two tests pin that pointer, not the skill's text); `docs/upstream-backports.md` (one row of the history of a backport); `artifacts/qa/collaboration-multi-account-20261109/environment/source-hashes.json` (the evidence of a QA run of 2026-11-09: its three values for this skill already differed from the tree before this change, and no code and no test reads the file); a status listing of an audit under `.impeccable/`; and evidence files under `.trellis/`. None was touched and none had to move.

Every name that the four new rows of the source map cite was looked up and exists: `issueBudgetCmd` and `newIssueBudgetCommand`, `budgetStart`, `budgetHumanCredential`, `budgetRefusal`, `budgetRefusalMeanings` (`cmd_issue_budget.go`); `init` with the `AddCommand` line (`cmd_issue.go`); `requireHumanLocalCommand` (`cmd_agent.go`); `TestIssueBudgetIsRegisteredWithItsSubcommands`, `TestIssueBudgetIsForPeopleOnly`, `TestIssueBudgetRefusesAClientThatIsNotAPerson` (`cmd_issue_budget_test.go`); the fixture `error-budget-forbidden.json`. The link target `#issue-budget-commands-are-for-people-only` is the heading that stage E added to `references/report-snapshots.md`.

### The offline install

`pnpm install --offline --frozen-lockfile --filter '@multica/core...'` from the worktree root, 2026-11-19 22:09:36 to 22:09:43 CST: exit 0, "Scope: 3 of 11 workspace projects", "Lockfile is up to date", 490 packages resolved, 489 reused, 0 downloaded, pnpm v10.28.2. The store had every package; nothing was fetched. pnpm warned that it ignored the build scripts of four packages (core-js, msw, protobufjs, unrs-resolver); none ran. `git status --short` was the same before and after (67 lines then), and `pnpm-lock.yaml`, `pnpm-workspace.yaml` and the eleven `package.json` files of the root, `packages/*` and `apps/*` had the same SHA-256 before and after (`/tmp/f3-7b/r2/install.out`, `status-before-install.txt`, `status-after-install.txt`, `sha-manifests-before.txt`). No `package.json` and no lockfile is in this lane's 17 status lines.

### A run that is not part of the verification: the one that wrote to a database (R5)

It is listed here because it ran in this worktree, not because it shows anything about the delivered files.

| Command | Toolchain | When | Exit | Result |
| --- | --- | --- | --- | --- |
| `bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test ./internal/service/ -count=1 -v`, `DATABASE_URL` unset | go1.27.0 | 2026-11-20 03:13:08 to 03:14:21 CST | 1 | top-level PASS 849, FAIL 47, SKIP 795; at every depth 1700, 75 and 1996 |

Of the 47 failures, 38 are tests that reached `localhost:5432/multica` through one of five fallback helpers and met a schema that lacks the table `collaboration_stage_child` (50 errors with SQLSTATE 42P01 in the output); 9 are tests that refuse to run without an explicit `DATABASE_URL` and touched nothing. Of the tests that can reach that database 155 passed; tests of that kind insert rows of their own and delete them in `t.Cleanup`. R5 in section 3 has the account and what is not established. The run was this lane's own addition to brief item 6 and should not have been made; it was not repeated, and every later run of that package is limited to the five named tests.


### The round 1 pass (2026-11-19)

Everything in this part is one pass, 2026-11-19 17:24:11 to 17:24:49 CST, on the tree as round 1 left it; "now" in this part means the end of round 1. It ran after the last mutant run of round 1, and from then until round 2 began only commands that read ran in the worktree. Two scripts ran it: `/tmp/f3-7b/verify.sh` (items 1 to 3 of brief section 6) and `/tmp/f3-7b/run-package.sh` (item 4), both with `/bin/bash`, from the worktree root, with `DATABASE_URL` unset and `TMPDIR` set to the path the brief gives. The outputs are in `/tmp/f3-7b/verify/` and `/tmp/f3-7b/after-round1.out`.

The default toolchain is go1.27.0 (`/opt/homebrew/bin/go`). The second one is go1.26.9, taken with `GOTOOLCHAIN=go1.26.9 GOPROXY=off`. "The ten files" are `cmd_issue.go` and the nine `cmd_issue_budget*.go` in `server/cmd/multica/`.

| Item | Command | Toolchain | Exit | Result |
| --- | --- | --- | --- | --- |
| 1 | `gofmt -l <the ten files>` | go1.27.0 (`/opt/homebrew/bin/gofmt`) | 0 | prints nothing |
| 1 | `"$(GOTOOLCHAIN=go1.26.9 GOPROXY=off go env GOROOT)/bin/gofmt" -l <the ten files>` | go1.26.9 | 0 | prints nothing |
| 2 | `go -C server build ./...` | go1.27.0 | 0 | prints nothing |
| 2 | `go -C server build ./...` | go1.26.9 | 0 | prints nothing |
| 2 | `go -C server vet ./cmd/multica/...` | go1.27.0 | 0 | prints nothing |
| 2 | `go -C server vet ./cmd/multica/...` | go1.26.9 | 0 | prints nothing |
| 3 | `go -C server test ./cmd/multica/ -run TestIssueBudget -count=1 -v` | go1.27.0 | 0 | PASS 34, FAIL 0, SKIP 0 |
| 3 | the same with `-race` | go1.27.0 | 0 | PASS 34, FAIL 0, SKIP 0 |
| 3 | `go -C server test ./cmd/multica/ -run TestIssueBudget -count=1 -v` | go1.26.9 | 0 | PASS 34, FAIL 0, SKIP 0 |
| 4 | `bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test ./cmd/multica/... -count=1 -v` | go1.27.0 | 0 | PASS 524, FAIL 0, SKIP 0 |

The counts are the top-level `--- PASS`, `--- FAIL` and `--- SKIP` lines. Counted at every depth, each of the three runs of item 3 has 824 `--- PASS` lines (34 tests and 790 subtests) and no `--- FAIL` or `--- SKIP` line.

**Item 4 against the baseline.** The baseline is the same command before the first change (`/tmp/f3-7b/baseline.txt`, 04:35).

| Run | Exit | PASS | FAIL | SKIP | `--- PASS` at every depth |
| --- | --- | --- | --- | --- | --- |
| baseline | 0 | 490 | 0 | 0 | 1286 |
| now | 0 | 524 | 0 | 0 | 2110 |

Only PASS moved, by 34. Of the names of the top-level tests that passed, 34 are in the run of now only, and each starts with `TestIssueBudget`; none is in the baseline only. At every depth the difference is 824: the 34 tests and their 790 subtests.

**Skipped tests: none.** Neither run has a `--- SKIP` line at any depth. So there is no name to list, and no test of this lane is left for the main session to run against a database. The five test files of this lane hold no `t.Skip`, no `testing.Short` and no `DATABASE_URL`, so none of them passes by returning early without a database. They also hold no call into `time` (a wait that ends is a context the test cancels) and start no process. Their servers are `httptest` servers on the loopback interface, as in the snapshot tests.

**The guard.** The script puts a stand-in for every name of `scripts/agent-cli-command-names.txt` first in `PATH` and exits 1 when one of them was started. It exited 0, with the exit code of the go command: no test of the package started an agent CLI.

### The tree that the round 1 pass verified

HEAD is `86fea651cc5d09360327b6cf0f8357c1f360070d`, detached. Nothing is committed, staged, stashed or reset by this lane. `git status --porcelain` has 67 lines: the 56 lines of `/tmp/f3-main-s7b/base-status.txt`, each still there as it was, and these 11:

```
 M server/cmd/multica/cmd_issue.go            (1 line added, none removed)
?? server/cmd/multica/cmd_issue_budget.go
?? server/cmd/multica/cmd_issue_budget_fixtures_test.go
?? server/cmd/multica/cmd_issue_budget_money.go
?? server/cmd/multica/cmd_issue_budget_money_test.go
?? server/cmd/multica/cmd_issue_budget_print.go
?? server/cmd/multica/cmd_issue_budget_read_test.go
?? server/cmd/multica/cmd_issue_budget_test.go
?? server/cmd/multica/cmd_issue_budget_wire.go
?? server/cmd/multica/cmd_issue_budget_write_test.go
?? server/internal/handler/testdata/issue_budget_http_v1/   (26 files)
```

`git status --porcelain --ignored` shows no ignored file. `find . -newer /tmp/f3-7b/baseline.out -type f` (without `.git`) lists 36 files: the ten above and the 26 fixtures. So no file of the earlier steps was written since the baseline was taken.

SHA-256 of the ten Go files. `shasum -a 256 -c` over them printed `OK` ten times after this pass, and before and after the mutant runs against the final tree (section 4):

```
ec955711b780849ee790e8df6be3ddc850a915c4aed94641d65f8ae21b1e53c8  cmd_issue_budget.go
e1d147fa3bd1918235bf97bc887f4d9a2d9634dbf83989fa8642cb1f0a88121b  cmd_issue_budget_fixtures_test.go
fa8956b056d5226eb1e7e37816c7945b873af60faaffaf618cdab191d6ac7e7e  cmd_issue_budget_money.go
5b556717c58be40eab9265c2aeda93112c3706a8de010b64dc89bc701d7e6000  cmd_issue_budget_money_test.go
901503bfbb571fff72fcb8c9ec8e45fb5ebd6e28c0e554d6155b22be969a99e0  cmd_issue_budget_print.go
81c6753a69a5971be2a3a371dd139542412bb18edc3d3febed7a54901843e75d  cmd_issue_budget_read_test.go
f4d688cf91694637c9e3bdb4ad35e0f2554ac8d37a1ad77d958cd9f5db253a9a  cmd_issue_budget_test.go
264a0c29ac31ba5ec71bc74c320acfa1500e43fbc82b02f9962ce77df981f86d  cmd_issue_budget_wire.go
dba2ccde566b61a0a00f8783205e68b521736379b2fd5fe9d15114f128005e76  cmd_issue_budget_write_test.go
dba53918048f6538ddae83396d68dc6feedeb67fdb1311f21a9473a31b067eb8  cmd_issue.go
```

The 26 fixture files were last written at 04:56, in stage A. Their SHA-256 values are in `/tmp/f3-7b/sha-fixtures.txt`.

Round 2 changed four of these ten files (`cmd_issue_budget.go` for the two sentences of the help text, and the three test files `cmd_issue_budget_test.go`, `cmd_issue_budget_read_test.go`, `cmd_issue_budget_write_test.go`); the other six and the 26 fixtures have the values of this list still. The offline install of round 2 added the ignored `node_modules` directories that this part does not have yet.

### Earlier passes of round 1

The round 1 pass is the fifth pass of 2026-11-19. Passes 3 (14:31) and 4 (16:50) ran on the same ten files and gave the same exit code and the same counts in every line; their outputs are kept in `/tmp/f3-7b/verify-pass3/` and `/tmp/f3-7b/verify-pass4/`. Passes 1 and 2 ran before the last tests were added and counted fewer subtests (650 and 771); they also passed, and nothing here is taken from them.

### What the CLI's HTTP client does, and which call these commands use

This answers brief section 4 ("find out whether the CLI's HTTP client retries a request on its own"). It was read in the sources; what a test of this lane pins is said at the end.

**`internal/cli` has no retry of its own.** `grep -n 'retry\|Retry' server/internal/cli/*.go` gives 11 lines. Four are in `errors_test.go`. The other seven are a message and comments, none is a loop or a second call: `collaboration.go:222` (a message that tells the user to run the unchanged command again), `collaboration_memory.go:27` and `:41`, `errors.go:373` and `:414`, `update.go:423` and `:424`. `NewAPIClient` (`client.go:187`) builds `&http.Client{Timeout: httpTimeout()}` with the default transport: 30 seconds unless `MULTICA_HTTP_TIMEOUT` says otherwise. Each helper (`GetJSON`, `PostJSON`, `PutJSON` and the others) makes one `HTTPClient.Do` call.

**The helpers were not used, for two reasons.**

- They keep the answer to themselves. A status of 400 or more becomes an `*HTTPError`, and a 2xx is decoded into `out`. A write of these commands has to see the status and the whole body to tell its three outcomes apart: a 200 from a 201, a refusal body from any other 4xx body.
- `PostJSON` follows redirects and leaves `GetBody` on the request, so a 307 or a 308 sends the body a second time, to another URL. `PostJSONWithIdempotencyKey` (`client.go:370`) does stop both, but it sets an `Idempotency-Key` header, which 12.9 does not have, and it also hides the status.

**The call that is used** is `budgetRequest` in `cmd_issue_budget.go`, for the writes and the reads. It takes the `*http.Client` of the `APIClient` that `newAPIClient` built (so the timeout and the transport are the CLI's), and the context is `cli.APIContext`'s, as in the other commands. It differs from the helpers in four things:

1. `req.GetBody = nil` on a request with a body. `http.NewRequestWithContext` sets `GetBody` for a `*bytes.Reader` (`request.go:942` in go1.27.0, `:932` in go1.26.9), so it has to be taken away.
2. No `Idempotency-Key` header.
3. A copy of the client whose `CheckRedirect` returns `http.ErrUseLastResponse`: a 3xx comes back as the answer it is.
4. It returns the status and the body whatever the status is, and reads at most 1 MiB and one byte.

**What net/http can then do with a write.** Three places in net/http can send a request again. They were read in both toolchains.

| Place | go1.27.0 | go1.26.9 | For a POST with a body, without `GetBody`, without the header |
| --- | --- | --- | --- |
| a redirect, `redirectBehavior` | `client.go:514` | `client.go:512` | 307 and 308: not followed, because the body cannot be sent again. 301, 302 and 303 would become a GET of the new place; `CheckRedirect` stops that (`client.go:716`, `:704`). |
| the HTTP/1 transport, `shouldRetryRequest` | `transport.go:844` | `transport.go:818` | no: when nothing was written it retries only if the request has no body or has `GetBody`; otherwise only if `isReplayable` (`request.go:1561`, `:1534`), which needs no body or `GetBody`, and then a GET-like method or an `Idempotency-Key` header |
| the HTTP/2 transport, `shouldRetryRequest` | `internal/http2/transport.go:507` | `h2_bundle.go:8062` | only after `errClientConnUnusable`, which is returned before a stream is opened (`awaitOpenSlotForStreamLocked`, `:1539`, `:9162`), so before anything of the request is written |

One more path tries again on another connection: no usable cached HTTP/2 connection (`http2isNoCachedConnError`, at the top of the same HTTP/1 function). After it `rewindBody` (`transport.go:822`, `:796`) hands the request back only if nothing was read from its body, and fails with `errCannotRewind` otherwise, because there is no `GetBody`.

So the body of a write is put on the wire at most once. Without `GetBody = nil` the first and the third row would allow a second sending; with an `Idempotency-Key` header and `GetBody` the second row would. The text of the three functions named in the second row and of `rewindBody` and `redirectBehavior` is the same in both toolchains (compared with `diff`); the HTTP/2 code moved from `h2_bundle.go` to `internal/http2/` and its rule for a request with a body is the same.

A read has no body, so the transport may send a GET again on a connection that broke. That is harmless for a read, and nothing was done against it.

**What the tests pin of this.** `TestIssueBudgetWriteGivesTheTransportNothingToReplay` puts a `RoundTripper` in the place of the transport and checks, for each of the five writes and for the statuses 201, 302, 307, 308 and 500, that it is handed exactly one request, with a nil `GetBody` and without an `Idempotency-Key`. `TestIssueBudgetWriteOutcomeUnknown` runs every write against a real `httptest` server that answers 302 and 307 towards a route that would say "committed", and the server sees one request. Every case of `TestIssueBudgetWriteRefused` and `TestIssueBudgetWriteOutcomeUnknown` ends by counting the requests the server saw. Mutants #22 to #27 of section 4 (sent twice, sent again when no answer came, sent again after a 5xx, a redirect followed, `GetBody` kept, the header sent) are each killed.

What no test of this lane can show is the table above itself: that net/http, given such a request, does not send it again on a connection that breaks. That is read from the sources of the two toolchains, not run (section 6).

### How a caller tells the outcomes of a write apart

The table is in section 1 ("How a caller tells the outcomes apart"). In short: only success exits 0 and prints on stdout. A refusal starts stderr with `refused (` and exits by its status: 3 for a 401 or 403, 4 for a 404, 5 for a 400 or 422, 1 for a 409 and any other 4xx. An unknown outcome starts stderr with `outcome unknown: ` and always exits 1. A 409 refusal and an unknown outcome share exit 1, so the exit code alone does not tell those two apart. That is item R1 of section 3.

## 6. Not done and not verified

This is the state at the end of round 1 (2026-11-19, 20:15 CST). Round 2 brings it up to date.

- Stages D (Core schema and API method) and E (built-in skill text): both were done in round 2 (sections 1, 2, 4 and 5).
- The design changed while the lane worked: 906 lines at the first reading, 943 at the end of round 1. The file is untracked, so there is no diff. Compared again at the end: 12.1, 12.2, 12.3, 12.6 to 12.8, the prose of 12.9, 12.10, and the twelve JSON examples through the fixture check. No rule that the commands follow differs. Not read again: 12.4 and 12.5.
- A corner the design accepts (12.1): an enable is committed, its answer is lost, and the root issue is deleted before the query by key. `operation --issue` then gets 404 `not_found` and prints it as the refusal of a read; it does not say what that means for the enable.
- A server without the budget routes (an older server behind a newer CLI) answers the router's own 404. A write prints the unknown outcome, and a read by root prints `the server answered HTTP 404, which is not an answer of the budget API; nothing was read`; both are tested with that body. `show <issue>` prints `belongs to no budget root, or is not visible in this workspace`, as it does for any 404 on its first request (reading 21), so there it does not show that the server knows no budgets.
- The step 7a handler is not in the worktree. Nothing ran against the real routes. Which code comes under which status in the tests (`budgetRefusalCodes`) is this lane's reading of 12.3.
- Readings without a test: 3 (nothing asserts the `X-Client-*` headers), 5 (a `mpc_` or `mpi_` token is sent as it is), 7 (a very long reason), 16 (a request that cannot be made, for example a bad `--server-url`), 17 and 26 (what `--debug` shows), 25 (keys that differ in the case of their letters), 27 (an empty text of the server is printed as `""`).
- No test uses the exact bodies of the workspace membership check (`404 {"error":"workspace not found","code":"workspace_access_denied"}` and `403 {"error":"insufficient permissions"}`). By reading 13 they are the unknown outcome for a write.
- The client's own timeout (30 s, or `MULTICA_HTTP_TIMEOUT`) is not exercised: the test of a wait that ends cancels a context. The test harness does not clear `MULTICA_HTTP_TIMEOUT`, so a very small value in the runner's environment would turn answers into "no answer".
- The refusal of a daemon task is not simulated. Its marker is looked up from the working directory upwards (`daemonTaskContextMarkerPath`, `cmd_agent.go`), so a marker in a directory above the checkout would fail every budget test at the people-only check.
- Nothing ran through `main`. The tests build their own root command and call `cli.FormatError` and `cli.ExitCodeFor` on the returned error. The binary was not started, because it would read the operator's real profile. The registration under `issue` is pinned by `TestIssueBudgetIsRegisteredWithItsSubcommands` and mutant #139.
- What `net/http` does on a connection that breaks is read from the sources of both toolchains (section 5), not run.
- Only darwin/arm64: nothing was built or run for another system, and the printed commands are quoted for a POSIX shell.
- `-race` ran for `TestIssueBudget` only, as the brief asks.
- The example answers of set-mode, close and reopen in section 1 are made up from the set-limit fixture.
- `server/cmd/multica/cmd_runtime_test.go` (committed at HEAD, not one of the 56 dirty paths, not touched) is listed by `gofmt -l` with both versions.
- Seen in a file that is not this lane's, read and not run: the snapshot command wraps its error with the advice to keep the key (`cmd_issue_report_snapshot.go`, line 202). For an answer with a status, `cli.FormatError` prints the copy for the kind of error, so that advice shows only with `--debug`; for a transport error the whole text is printed. The budget commands use `cli.WithUserMessage` and do not have this.
- A stray binary `server/multica` (37 MB, from this lane's first build of stage C at 08:28, ignored by `.gitignore`) was removed shortly before 13:00. Verification pass 1 ran with it present; it is absent now.
- The evidence (mutant specifications and results, verification outputs) is under `/tmp/f3-7b/`, outside the repository, and does not survive a restart of the machine.
- No documentation was written. The skill text is stage E.

## 7. Hooks and reminders that asked for something outside the brief

None was acted on, and none asked for an edit outside brief section 1.

- The SessionStart hook's "first reply notice" asks for an acknowledgment that the Trellis context loaded. It came again after every compaction of this session.
- The Trellis workflow and task-status blocks, printed with it, suggest dispatching `trellis-implement` or `trellis-check`, updating the spec and committing.
- A reminder gives the attribution lines for commits and pull requests.
- The session's global instructions ask to delegate work to sub-agents and to keep a separate verifier pass. The brief says "You do the work yourself", and that was followed.

Closing turn, 2026-11-20, 07:23 to 08:14 CST, the main session's eight items and nothing else: R5 is marked ruled and carries the ruling's text, and the first line of section 3 says that all seven items are ruled (item 1); `/tmp/f3-7b/r2/skill-tests.sh` exports the explicit `DATABASE_URL` where it had `unset DATABASE_URL` (item 2); `server/internal/service/builtin_skills_test.go` got one anchor line, seen failing with one wrong character and passing as it stands, and R6 has the line, both runs and the ruling (items 3 and 4); R7 has its ruling (item 5); the line of section 6 about stages D and E (item 6); the test file and its SHA-256 in section 5 (item 7). No other line of the report was edited in this turn. Three statements therefore stand as they were written and are superseded: the sentence of the opening paragraph that R5 needs a ruling, and the two lines of section 6 that say reading 27 and the bodies of the membership check have no test (they have one since round 2, section 2).

STATUS: complete
