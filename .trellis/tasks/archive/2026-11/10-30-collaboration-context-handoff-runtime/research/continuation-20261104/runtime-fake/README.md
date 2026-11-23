# Current-source runtime fake verification

The ordinary run passed. The race run failed one fixture, so current-source RK1
acceptance remains open. Neither run invoked an installed Provider or consumed
runtime25 quota. Product and framework source were unchanged.

Both runs used clean HEAD `1a32ca36c2c4f4455a01bdd39d0e0ab4db20cccc`, canonical
source ID `6b342ba23c2f5ae6826e8841794d086d1258274d3a39eac84983bfb9f429bdea`
(7,990 files), the unchanged seventeen-file framework
`5eaea44f480f8e94a9a90d35025ca2d68d76c895c5bf70fa902da37c5bed94c9`, and pinned
Go 1.26.6. They ran serially against separate newly migrated databases supplied
by root: `multica_rk1_20261104_normal_a729` and
`multica_rk1_20261104_race_a729`, both migration764. Actual command, configuration
fingerprint, tool hashes, source before/after, process IDs and raw-log hashes
are in the result records.

Root subsequently identified another session's password-mode browser run from
15:42:54 through 15:45:03 UTC, overlapping this race run and its failed trial.
The race environment was therefore not exclusive, despite the preflight finding
no active full check/browser process. This overlap is a fact reported by the
coordinator, not proof that it caused the checkpoint timeouts. The original
race failure remains unchanged; a quiet-window pass would not establish that
the underlying one-second deadline mechanism was repaired.

| Run | Actual Go exit | Named pass / fail / skip | FakeProof artifacts | Accepted |
| --- | --- | --- | --- | --- |
| Ordinary | 0 | 62 / 0 / 0 | 19 fixtures, 22 public trials, 32 fake/simulator launches | Yes, after correcting collector paths |
| Race | 1 | 60 / 2 / 0 | 19 fixtures, 22 public trials | No |

The two race failure events are `TestRK1FakeProof/wrong_continues` and its parent
`TestRK1FakeProof`, representing one failed fixture. Its second trial became
`incomplete/measured:handoff_or_recording_event_missing` instead of the expected
complete wrong answer. The provider started once and exited normally; outcome
and proof succeeded, while checkpoint delivery timed out and the persisted seal
was not sent. Source/framework identities remained stable. The read-only
[diagnosis](race-diagnosis.json) preserves exact facts and three falsifiable
hypotheses; it does not claim a proven root cause or a repair.

The first collector mistakenly searched flat `case/RK1-*.json` paths, while the
framework writes `case/label/public.json`. The original [runner](run_fake.py),
ordinary [result](normal-result.json), admission and raw output are retained.
The separate [reconciliation](reconcile_normal.py) checked original hashes and
counted the twenty-two existing files without rerunning tests; its
[result](normal-reconciled-result.json) is the ordinary acceptance record.
The unused race runner was corrected before starting race and retained as
[v2](run_fake_v2.py). Its [race result](race-result.json) records the real failure.

All private credentials, ledgers, stdout and trace bodies remain under the
isolated worktree's `.omx/private/runtime-fake-20261104/`; public records contain
paths and hashes. No failed database or ledger was modified during diagnosis,
and no retry, additional Runner invocation, timeout increase or scoring change
was used to replace the failed sample. Real model and password-mode browser
acceptance remain separate gates.
