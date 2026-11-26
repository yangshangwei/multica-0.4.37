# Official CLI direct O1 follow-up

New `scripts/direct_cli_followup_harness_test.go` is independent of the frozen
P1 and former app-server O1 sources. It applies the exact same fixture rules and
complete workspace oracle while selecting the task-only official CLI backend.
Each manifest copies the actual claim's `PriorSessionID` as `resume_session_id`;
the backend only requests resume when production runTask retains that ID, and
the CLI wrapper refuses a different ID or an absent owned rollout. No thread ID
is synthesized in a real run.

The four roles remain: squad leader intake; actual restricted security reviewer;
ordinary leader reply without squad context; leader following a valid human hold.
The human hold is a separate operator gate in real mode: the private approval file
binds the actual reviewer comment/content and predetermined decision/owner. It
never appears in the model container. Hold wording uses `type=other, route=human`,
and the actual human timestamp must follow the reviewer by a distinct serialized
second. The synthetic issue/material and decisions are unchanged from the prior
frozen O1 planning.

`TestSecurityRemediationDirectO1Fake` passed through the actual router and
Claim/Run/Report with a test-created CLI-shaped fixture, using the actual compiled
Multica CLI and claimed identities. Four role runs completed, restricted result
dispatched nobody, invalid reply did not delegate or change status, and all eight
platform assertions passed. This is scripted evidence, not model behavior.

Evidence: private `logs/direct-fake-o1-01.log`, `runs/direct-fake-o1-01/`, config
`harness-private/direct-fake-o1-01.json`; hashes and machine verdict are in
`direct-cli-o1-fake-20261120.json`. Real direct O1 entry requires a separate reviewed
O1 registration; P1 admission does not cover it. Existing P1 binary/source bytes
were not modified by this follow-up implementation.
