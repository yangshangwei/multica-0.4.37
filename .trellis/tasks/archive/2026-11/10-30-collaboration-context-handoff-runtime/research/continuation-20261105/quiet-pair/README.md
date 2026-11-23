# Second current-source runtime verification pair

Both actual test commands passed, but the exclusive-window precondition was not
maintained. This pair must not be described as fully quiet or used by itself to
open the paid entry. Root was notified immediately when overlap appeared and
directed the already-running race command to finish without retrying.

| Run | Actual exit | Named pass / fail / skip | Top-level | FakeProof artifacts |
| --- | --- | --- | --- | --- |
| Ordinary02 | 0 | 62 / 0 / 0 | 9 passed | 19 fixtures, 22 public trials, 32 fake/simulator launches |
| Race02 | 0 | 62 / 0 / 0 | 9 passed | 19 fixtures, 22 public trials, 32 fake/simulator launches |

Both used unchanged clean `1a32ca36c`, source ID `6b342ba2…29bdea`, 7,990 files,
the original seventeen-file framework `5eaea44f…ed94c9`, Go1.26.6, unchanged
test selector/assertions/timeouts and separate fresh databases migrated through764.
Every before/after source identity matched; no real Provider was invoked and no
runtime25 quota was used. Exact commands, database names, private paths, raw
hashes and process IDs are in [ordinary](normal-result.json),
[race](race-result.json) and the combined [summary](summary.json).

Root obtained explicit release from the password7/original14 and core E2E owners
before this pair. The16:40:57UTC preflight and16:43:41UTC between-run process
snapshots contained no other test, Playwright or Claude process. During race,
the16:46:09UTC snapshot showed the external knowledge
`codex-normal-03b.test` process41693. The16:48:15UTC snapshot then showed that
owner's `codex-race-01.test` process48804. These are retained process-name and
ancestry observations; no arguments, environments or credentials were collected.
Root contacted that owner and instructed this already-running race to complete.

The runner's `accepted=true` covers command exit, exact named inventory,
source/tool/database bindings and fixture artifacts. Its predicate does not
check exclusivity, so the separate summary explicitly records
`quiet_precondition_held=false`. Process snapshots also have gaps between samples;
they cannot prove the absence of all short-lived concurrent work.

The earlier race's60pass/2fail and all its raw evidence remain unchanged. This
new pass does not establish that external load caused the previous failure or
that the recording deadline mechanism was repaired. No product/framework patch,
selective rerun, third pair or real model run was started. The owned test
processes have exited and this lane's heavy execution window is released.
