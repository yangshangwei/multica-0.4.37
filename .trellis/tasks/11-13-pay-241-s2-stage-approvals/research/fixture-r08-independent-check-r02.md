# PAY-241 E2E fixture r02 independent review

**FIXTURE_R02_ACCEPTED_READY_FOR_BOUND_4CASE_RERUN — zero blocking findings.** ROOT may integrate only the reviewed existing `workflowAuthority` hunk and bind a new complete four-case runtime run. This is static and retained-evidence acceptance; the repaired runtime has not been exercised by this reviewer.

## Original failure and cause

The original r01 has four planned cases: **0 PASS, 1 FAIL, 3 NOT_RUN**. Its first setup invitation failed at `collaboration-stage-approvals.spec.ts:100`. The recorded participants POST body has the four workflow observations and target user, plus `execution_view_version:6`. Its actual response is400 `{code:"invalid_request",error:"Invalid request"}`. Independent hashing reproduces request SHA256 `1f6b9ed91b887d9ec97eaba693ed38381deec6809ab37b8fefbda89326a42c73` and the encoder-newline response SHA256 `608a8ce060daeff3ffbbf5bab17d9743fd04302d07a63bb374b9ad0f9d38ef3e`.

`collaborationWorkflowBody` accepts only collaboration revision, lifecycle, participant and account observations (`handler/collaboration_workflow.go:18–22`). The shared strict decoder rejects unknown fields at lines139–141. `hostAuthority` adds execution view version6 for the separate host/resource protocol, whose request DTO explicitly permits that field (`handler/collaboration_authority.go:14`). The old helper spread that host-only field into a workflow request. The malformed request failed before role/stage/action checks, so it is not evidence of a product gate defect.

The preceding issue create returned201 and exposed invite capability for the same actor. Original histories and failed requests are retained; no source or request was edited to make the past run green. ROOT's separate READ ONLY42P01 audit of `collaboration_participant` is an auditor table-name error, not another case failure; the source table is `issue_participant`.

## Narrow repair and consumers

The one helper now calls unchanged `hostAuthority(issue)` for its original current-participant guard, then explicitly returns its three epoch values and `issue.collaboration_revision`. The output has exactly four workflow fields; it does not forward execution view version. Host resource offer/adopt/consent continues using the original host builder and version6. No production decoder or authorization rule changed.

The six actual export consumers are participant setup invitations in both stage specs, project-field PATCH in the execution spec, stage observation composition, formal acceptance submission and self/submission completion-body composition. Each uses the workflow/stage/lifecycle protocol. The similarly named performance-local helper and order/export gap alias are independent implementations and were not changed. Completion M1 target identity, observation values, authorization and original request keys remain intact.

## Source and static evidence

Private input binding SHA256 is `7ea690fd7e8b4739c3499ec393b9b51a179ac0085fb79008a11456d9fff32044`. The reviewer independently rehashed all **3,358 files** and verified all **1,961 symlinks**. Exactly one existing helper differs; **3,357 protected files** and the host builder retain their bytes/modes. The helper's before SHA256 is `91b8b4c5cbaab035b6c70c1e2a8034d6ea1cbefacdfe61f13c1747d8bc9cc166`; after is `d40059ca4bc003dd343d19ccfb86c23bf9f57f9d65348fc727d6ea583eee73f5`, private mode0600. Patch SHA256 is `363f615abe91a205a0d36432130116a67c5c5f844070bbef539ab82651cedfde`.

Owner static result `624393acd7092f15c6225953a1d6be67b1b227a5d03fc7e0721446185a66389a` and handoff `39b0a3650301b39361eef910798dbd2243ba51801d8b22e9449c60c063f8cb8e` match their actual files. The existing E2E4 noEmit TypeScript configuration includes both stage specs and this helper/import closure; it exits0 with no output. Actual ESLint names this helper, is not ignored, and returns zero errors/warnings. The reviewer verified command/output hashes and rechecked the source binding rather than repeating behavior tests.

Production Web/Desktop inputs and outputs are unaffected by an E2E-only helper change and can be reused through the new exact run binding. The original source and RED evidence remain unchanged. No Go/service suite was repeated, and this reviewer performed no test, database, API or browser operation.

## Next gate and limits

ROOT should apply only the reviewed helper hunk while preserving original ROOT mode, then register the new fixture source and complete four-case selection under a new run ID. Original r01 remains0/1/3. The corrected request has not yet produced an observed successful invitation; only an actual newly bound run can establish that result and the later stages.

The reviewer wrote only its external evidence/report directory. ROOT/frozen inputs, previous reports and repository Markdown remained read-only. Full S2, real Web/Desktop scenarios, business signatures, external systems and parent24-case acceptance remain outside this static verdict.
