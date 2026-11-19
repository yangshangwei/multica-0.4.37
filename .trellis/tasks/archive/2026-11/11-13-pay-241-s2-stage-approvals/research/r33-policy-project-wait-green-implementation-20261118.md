# R33 M1 policy-project wait product authoring

**R33_M1_ONLY_WAIT_TWO_PRODUCT_FILES_FROZEN_GENERATION_PENDING_ROOT.** The two authorized product source edits are frozen for ROOT's canonical generation and actual GREEN verification. This receipt makes no compilation or executed GREEN claim.

Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`; owner `/root/r32_policy_implement`; captured UTC `2026-11-18T00:46:55.602156+00:00`. Only the two permitted private product files and this new report pair were written; reports are sealed0444.

## Accepted RED and authorization

Independent review SHA256 `b841936614abb003de8ecdbbdcd73f020b562914d7506a35a7c43f9c98f3b25a` is `ACCEPTED_R33_GENUINE_POLICY_PROJECT_WAIT_RED_CONTROLS_PASS`. Its actual RED result SHA256 `3b94b4a452213059922556d102f58735f06a7ba8c1be6434a1f16b5819bfc92f` retains2PASS/4FAIL named and2PASS/2FAIL leaves: short reader receives busy409 instead of200, competing writer receives busy409 instead of exact stale CAS, while long reader/member controls pass with original effects. The initial coordinator-failed1PASS/5FAIL source/result remains retained separately.

ROOT permit SHA256 `37beb8cd81865778d6416d138db221b5a683242cb37b011de1e92469d41d01d6` authorizes exactly the two files below in `/Volumes/artisan/.multica-acceptance/pay241-r33-policy-wait-20261118-hc2gedj3/source-green-r01`. Copy receipt SHA256 `77451a93dbae5ab2ec0d2e2152baee170a20b017debfd71bf30f370443e094ab` binds the unchanged5056-file RED source before authoring. Its earlier pending-review note is satisfied by the later permit and sealed accepted RED above.

## Resulting product change

- `server/pkg/db/queries/delivery_policy.sql` adds only `LockDeliveryPolicyProjectWait :one` with `SELECT * FROM project WHERE id=$1 AND workspace_id=$2 FOR UPDATE;`. The original LockDeliveryPolicyProject query remains byte-for-byte NOWAIT. Both query variants select the same full project row, scoped ID/workspace parameters and exclusive FOR UPDATE strength.
- `server/internal/handler/delivery_policy.go` switches the single deliveryPolicyInTx PUT call and params type to Wait. Exact text substitution reconstructs every original handler byte. GET retains its original shared NOWAIT association fence; current member/account/credential, schema, policy/CAS, append, final credential, transaction ownership and error handling remain unchanged.

M1 PUT can wait for a brief reader or writer within the existing250ms per-lock timeout. A committed competing writer then makes the original current-policy CAS refuse the stale revision. This intentionally changes immediate refusal timing; it retains serialization and the exclusive fence. The budget does not bound total transaction time. No retry, no-op shortcut, flag, DTO, dependency, migration or permission change is introduced.

## Frozen files and static checks

| Path | Final SHA256 | Mode | Lines |
| --- | --- | --- | ---: |
| `server/internal/handler/delivery_policy.go` | `fff6c379877e869a56036b0ae25331136ee959fcd008415cedd6c7bf1a6e050a` | `0o644` | 169 |
| `server/pkg/db/queries/delivery_policy.sql` | `42d1efba5afb5f9b428dc2d4454f9a0a1f4c252375c00feb00ec69115ab5eec2` | `0o644` | 49 |

- `gofmt -w` on the handler only: exit0, empty output.
- Final one-pass source inventory:5056 regular files, unchanged path set/modes, exactly two product diffs,5054 protected files byte/mode-identical, no added/removed/nonregular paths.
- Removing only the new three-line SQL query block reconstructs the complete original SQL. Replacing only the one PUT call/params reconstructs the complete original handler. The old query and its stage-policy caller remain unchanged; stage caller SHA256 `a892c72b67010b78a7d1383406ba11412efc2a9ecc3acd1e3df36295265cf9d6`.
- Fixed six-name test remains SHA256 `443fc8a99a88130df7005854d9759360341c982190c75b0e84100de7547f22b3`; all timing/authority/effect assertions are untouched.
- Full path-sorted compact sorted-key manifest SHA256 `ca59ed37756a658f1b66b8f7d417f6e114199823c50bcd94a4c8077c50c18da2`; protected5054 SHA256 `ad3a452cd99733e19b5b66c687e185633e2785c135279267006922faaa694125`. The JSON embeds the full manifest, both diffs and exact text proofs.

## Generation and ROOT verification handoff

`server/pkg/db/generated/delivery_policy.sql.go` remains unchanged at SHA256 `7cf3a60e58209f406dc55f909de0d527791aa7d72c70e2d5d1006df86ff79fdf`. The new method/params are intentionally absent until ROOT canonically generates that single owned output. No sqlc/Go AST/compile/vet/test/list/TestMain, database/API/browser/process-control/Git execution occurred in this implementer turn.

ROOT owns canonical generation, scoped static/build/vet and fresh normal/race verification of the planned10top/33leaf/40named union (original eight policy/CAS/writer/schema/activation tops plus unchanged new six names). Generated parameters retain ID/WorkspaceID UUIDs and the same Project scanner/error shape. A successful authoring receipt does not declare a GREEN test result, matching build, runtime or fullS2 acceptance.

No further product edit is pending within this two-file slice. Original stage-policy NOWAIT, all RED/raw history and successful E04 completions remain preserved. ROOT integration, matching programs, original runtime cases and fullS2/parent24/S3–S5 remain separate gates.
