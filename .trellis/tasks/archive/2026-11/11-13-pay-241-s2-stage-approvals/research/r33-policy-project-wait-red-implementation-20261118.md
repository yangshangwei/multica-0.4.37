# R33 policy-project wait RED test implementation

**R33_RED_ONLY_TEST_SOURCE_FROZEN_STATIC_HANDOFF.** One new private handler regression file is ready for ROOT's actual RED execution. No product implementation or executed RED/GREEN result is claimed.

Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`; owner `/root/r32_policy_implement`; captured UTC `2026-11-18T00:14:53.524180+00:00`. Only the new test and this new report pair were written. Reports are sealed0444.

## Authoring boundary

ROOT boundary SHA256 `096f3069487cebb570a4d91503f040e015a7650f367255cc894f3be5a3ea7000` authorizes only `server/internal/handler/delivery_policy_project_wait_test.go` beneath `/Volumes/artisan/.multica-acceptance/pay241-r33-policy-wait-20261118-hc2gedj3/source-red-r01`. Preparation SHA256 `f3004874103dc379e5e690d8cf7d5d5f011f1d2f36bc23b2703e87bd0338a764` binds the accepted R32 baseline5055 files. Sealed plan SHA256 `64486bd4ff02d9a88eb46f507c52abc18104fc09d2ac41311cbb3e22212d7e8a` defines the four leaves and the separate M1-only product decision after accepted RED.

The new file is 458 lines/18812 bytes, mode0644, SHA256 `aed914a8acd64465219a8fcb07541af3caed74e899c54796b873a6deaa8340a0`. It reuses `newDeliveryPolicyFixture`, its actual current human JWT/request builder, `testutil.Call`, `deliveryHintSnapshot`, `progressTxStarter` and `progressObservedRow`. The new observer forwards original Exec/QueryRow SQL, arguments, generated Scan destinations and errors. It never substitutes a SQL result, PostgreSQL error, actor or backend PID.

## Exact source inventory and behavior

| Exact leaf | Required real boundary |
| --- | --- |
| `TestDeliveryPolicyProjectWaitShortReader/short_reader_release` | Actual GET holds after its GetDeliveryPolicy Scan and acquired project SHARE fence. Actual PUT must reach its project query on a distinct PID; an actual55P03 or pg_blocking_pids witness precedes the50ms release. Target response200 and one exact append. |
| `TestDeliveryPolicyProjectWaitBoundaries/long_reader` | Same actual GET remains held until at least350ms after PUT project entry. PUT must return native55P03/409 artifact_authority_busy, no append or unrelated effect; GET then completes with its original observation. |
| `TestDeliveryPolicyProjectWaitBoundaries/competing_writer` | First actual PUT holds after CreateDeliveryPolicy Scan with an uncommitted append. Second same-revision PUT enters before50ms release; first200, second exact409 delivery_policy_revision_conflict, one retained winner append and zero loser append. |
| `TestDeliveryPolicyProjectWaitBoundaries/member_denied` | A genuine newly created current member can GET but receives403 delivery_policy_management_denied before a project-writer query or append. The original owner row remains unchanged. |

The static literal inventory is **2top/4leaf/6named** and exactly matches the boundary. Recognizing only the old and future explicit Wait query names lets the same test source run against both product versions. The competing-writer stale-CAS assertion may fail the old immediate busy branch; ROOT must retain that actual outcome rather than force every control green. The short-reader leaf is the primary RED target.

Backend IDs are cached at Begin, before pool-backed completion releases Conn. Actual native-error records retain query name/SHA, same authenticated request actor, project/workspace UUID arguments, error type/SQLSTATE and real source frames. Getter ErrNoRows remains its original virtual0/off outcome. Both recordable native failure and real database wait prove entry before release; a notification or sleep alone cannot authorize a successful test. A buffered query entry is retained even if old NOWAIT completes before the coordinator selects its response.

Each real request preserves its original fixture principal/route context and uses the original five-second orchestration bound. Successful original250ms SET LOCAL preparation is recorded, never replaced; this is a per-lock budget. Long-response observation has a one-second test guard, while the real holder stays beyond250ms. Deferred cleanup releases both barriers, cancels bounded requests and joins every started goroutine on failures too. No test calls Want from its goroutine.

Snapshots begin after legitimate fixture/member setup and protect project/issue, existing policy/evaluation/artifact/approval/task/comment/status facts plus workspace/member/user/password, stage task/policy, completion authorization and terminal rows. Only the expected winner policy append is permitted. Snapshot payloads containing account facts stay in memory; logs contain hashes/equality and safe query/error identities. `evidence_issue_id` and terminal/stage fields were checked against existing generated/query source. No whole-database-zero-change claim is made.

## Static checks and handoff

- `gofmt -w` on the new file only: exit0, empty output.
- Final one-pass manifest:5056 regular files, exactly one new test,5055 protected original files byte/mode-identical, no removed/nonregular paths, existing product/query/generated/test/config deltas0.
- Full path-sorted compact sorted-key final manifest SHA256 `2fe7a44b273bff9627cc00b8cbe21b4b2db281c1c567013d3a8cfa2ce0d7b5b8`; protected5055 SHA256 `26a4ad38ecf668f5fd665c05aa860dee948cfbfdd0af8b97035afadfc1c0f0e3`. The JSON embeds the final manifest, new source and exact inventory.
- Original handler/SQL/generated query, stage-policy caller,250ms preparation and reused fixture/helper hashes remain identical to baseline.
- Go AST/compile/vet/test/list/TestMain, SQL generation, database/API/Redis/browser/process-control/Git: **NOT_RUN by this implementer**, as assigned.

ROOT owns fresh handler allocation SHA256 `8243a9ba1a2361e7762877fc6100fc42f75552c135d4c9f86af2b7989a93e5a4`, actual compilation and RED, and independent RED acceptance before the future three-product M1-only slice. Exact selector: `^(TestDeliveryPolicyProjectWaitShortReader|TestDeliveryPolicyProjectWaitBoundaries)$`. The full final manifest is `source_after_manifest` in the JSON.

R32's actual policy-project55P03→HTTP409 correlation identifies that request's busy branch. Its physical holder and R31's separate historical attribution remain unknown. This newly authored deterministic test proves no runtime result until ROOT executes it. Original successful completions/history, fullS2 and parent24/S3–S5 remain separate gates. Source is frozen at the recorded hash, with no further author edit pending.
