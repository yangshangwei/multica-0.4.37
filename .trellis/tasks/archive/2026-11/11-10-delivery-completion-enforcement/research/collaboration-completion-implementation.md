# P3 collaboration completion and shared union implementation

Date: 2026-11-11. Owner: native trellis-implement / implement_collaboration_completion.
Worktree: `/Volumes/artisan/code/2026/mc-delivery-completion-enforcement-20261110`.

## Implemented scope

- Shared authorization Create/GET/Prepare/Finish now supports exact collaboration submission, prospective self-result and distinct R38 administrative intent, plus the VCS owner's typed GitHub/VCS proof helpers. The existing single service observer still verifies the actual ordinary child artifact.
- Source evidence must be the exact current formal ordinary child, with matching parent, source workspace, actual project and child revision. The source evaluator must retain their own current workspace/artifact and target-read authority. Outside initiators get only their own narrow completion fact; they do not acquire source membership or private observation/child disclosure.
- Target collaboration/lifecycle/participant/account epochs, exact status catalog, current pending submission or self-result/admin request digest are frozen and rechecked. Attachment identity/version/content digest are included in the intent; the actual accepted result is rechecked at Finish. R38 binds its real administrator, organization and original operation and never fabricates an initiator decision.
- The real lifecycle and R38 transaction performs Prepare, its original status/Stop/decision/operation mutations, and Finish. The terminal record refers to the real original operation and generated self submission; late Finish errors roll back all preceding writes. Existing original-key workflow/admin recovery runs before fresh mutation and does not re-consume.
- Self preparation is now read-only until admission succeeds. The original attachment permission algorithm is extracted into validateCollaborationAttachments; the original bind path delegates to it and then performs the same writes. Grant issuance and missing-authorization refusal do not deliver a draft attachment or create a self handoff.
- Collaboration project editing consumes the existing D4b helper inside its original transaction. Current owner/admin authority intersects existing creator/handler and source-member structure permissions; reason is strict and adjustment evidence is retained.
- Finish rechecks actual consumer and retained issuer after observation, derives a consumer expiry from the real JWT/task/PAT and passes it into the reviewer's conditional terminal SQL. Integrations use their required live signature/trust proof instead of a fabricated human credential.
- Following the separately captured Contributor consumer regression, only the two ordinary consumer autonomy checks now require Contributor; the artifact producer still requires Operator. Restricted capability profiles remain denied.

## Verification observed

The initial red binary exercised the actual collaboration lifecycle and actual password/admin middleware. Self, submitted delivery and R38 incorrectly returned 200 without authorization; all expected refusals became green after wiring. The missing-authorization set checks no partial handoff/event/status writes.

A second actual chain creates the ordinary child through CreateIssue, uses testutil for the ordinary queue/runtime/agent setup, executes the real typed artifact approval/packager flow, and verifies tar contents and SHA. It then signs source authorization, completes self/external/admin workflow, verifies state plus one terminal record, repeats the original workflow key without new events and checks minimal consumer read privacy. This is a real handler/service/database/tar proof; ordinary collaboration tests inject an authenticated principal context, while the R38 branch uses the existing real password/admin HTTP middleware. It is not the final NewRouter middleware proof for every collaboration path.

The expanded 18-name run adds read-only uploaded attachment behavior, seven current-binding refusals (detached child, child project, issuer/participant revocation, modified result/policy/attachment version), and source-project edit permission intersection. The final 22-name run also passes the new child-lock refusal, real PostgreSQL terminal insertion failure with full self-workflow rollback, and disabled administration configuration checks. The final authorization GET test distinguishes source-evaluator historical read from the disabled administrative consumer surface.

| Run | Exit | Named pass/fail/skip |
| --- | ---: | --- |
| collab-red-compile01 | 0 | 0/0/0 |
| collab-red01 | 1 | 0/4/0 |
| collab-core-build01 | 0 | 0/0/0 |
| collab-writer-build01 | 0 | 0/0/0 |
| collab-union-build02 | 0 | 0/0/0 |
| collab-green01 | 0 | 4/0/0 |
| collab-positive-compile01 | 1 | 0/0/0 |
| collab-positive-compile02 | 0 | 0/0/0 |
| collab-actual01 | 0 | 4/0/0 |
| collab-full-compile01 | 0 | 0/0/0 |
| collab-iter02 | 0 | 18/0/0 |
| collab-consumer-intersection-build01 | 0 | 0/0/0 |
| collab-final-compile01 | 0 | 0/0/0 |
| collab-final-vet01 | 0 | 0/0/0 |
| collab-final01 | 0 | 22/0/0 |

Normal and race results are not added together. Every run references its original log hash in research/runs. Compiled-binary runs bind the earlier compile record; later product changes are not represented as tested by those binaries.

## Retained failures

- collab-red01: 4 named failures (self, submission, parent test, R38). Actual expected428/actual200 behavior, not fixture failure.
- collab-positive-compile01: test-authoring typo used ArtifactDigest instead of ExpectedArtifactDigest. Zero tests ran; exact log preserved. The typo was corrected without altering assertions, compile02 passed.
- Several initial read commands guessed nonexistent service/admin or test paths. rg --files located the real files; these were read-only lookup failures, not product red or state changes.
- No historical P0/P1 evidence was rewritten, no commits, database allocation/migration, vendor calls or external messages were performed by this lane. Only root-authorized fresh test databases were used.

## Frozen source inventory

| File | SHA-256 |
| --- | --- |
| server/internal/service/delivery_completion.go | 51cad03d2611eebe96504ece66f041a9916959e69b4aa45e8e5e74c27d7df8d7 |
| server/internal/service/delivery_completion_authorization.go | 214b9ebd813795b97d31aac6cdde6975ab8cfe083cf81c6335b38199ed537420 |
| server/internal/service/delivery_completion_collaboration.go | 678986fa927381ee487850dce33f9f3db313e4fd99434f0fd8f10c4041c20f97 |
| server/internal/service/collaboration_lifecycle.go | 18c2630f044887762c4ee59ef5e4b1bebb0afc49821b88f3e6d2a7e41ba9e67c |
| server/internal/service/collaboration_delivery.go | 59982f3e2d792a5261b7ff02192cbba508ea51021b21e5ea37c833a2f263d7df |
| server/internal/service/collaboration_attachments.go | 8d1f4eadb2a3aba14476b6d93e05c4855dd664bda449995bbd0004ee5d520b22 |
| server/internal/service/collaboration_edit.go | e4080186fb568572382f0a6b723ae50bd1904ae7e34f63f1af49645bee1f1783 |
| server/internal/service/admin_collaboration.go | d1905d1b04ea6baa19eca5226930c4984cef945556aae49ea2dbbb365eaa5adf |
| server/internal/handler/collaboration_lifecycle.go | 83bcc24297c482d650940d7e53aae15199fa697e8b049024920aca0848818ecc |
| server/internal/handler/admin_collaboration.go | cb2e44e7d4d7452d1e2d9335f939091bb892dca6324273f15016d957a340d7c8 |
| server/internal/handler/collaboration_edit.go | ed57619518845efad7b7fb001e58dd0872e71ad471e3fa283736d1b786d9dbec |
| server/internal/handler/delivery_completion_collaboration_test.go | 787173a5173a0afb6e19e6980e43540efb1b16ea04f469cca780abe48650e750 |

## Remaining verification and ownership handoff

- The new cases have passed the final 22-name normal run. Complete existing collaboration/admin/attachment/source-edit compatibility suites and final integrated normal/race on fresh databases remain root's final validation.
- Final current NewRouter authentication coverage for collaboration needs root's route validation; this lane does not label injected handler principals as authentication middleware acceptance.
- The Reviewer owns exact schema readiness/conditional terminal SQL and generated code. Root owns trusted GitHub/admin configuration fields in ArtifactAccess and handler completionAccess. VCS owner owns its proof helpers and real webhook writers. This report does not independently accept those other lanes.
- P4 public enforce enablement remains closed pending all writers and final independent verification. This lane does not claim complete M1, M2/M3 or six-business acceptance.

## Final compiled proof

- Binary: `/private/tmp/multica-completion-enforcement-20261111-y9c0v8ro/binaries/collab-final-tests`
- Binary SHA-256: `727164d7da0e022496d11fa412ec7e039e05ff6fba52e2623c3608f723b40a36`
- Compile source record: `research/runs/collab-final-compile01.json`
- Actual test record: `research/runs/collab-final01.json`, fresh database `enf_p3_collab_final01`, 22 named pass / 0 fail / 0 skip.
- Full backend vet: `research/runs/collab-final-vet01.json`, exit 0.
- All 12 lane-owned product/test SHA-256 entries above still match current files.
