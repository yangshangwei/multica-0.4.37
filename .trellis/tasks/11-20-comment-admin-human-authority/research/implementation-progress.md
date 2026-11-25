# Comment lane implementation progress

Parent planning acceptance received and task activated. Product change is exactly the two admin predicates in UpdateComment/DeleteComment, guarded with !isMachineCredentialActor. Auth helper untouched; original author path preserved.

## Evidence
- Original legacy02/password01 probes remain untouched; hashes and original case IDs in original-evidence.json.
- Existing baseline:16 named PASS, exit0; moving checkout/private HOME originally inside Git, diagnostic only.
- Frozen red02 on allocated fix_comment_red02:220 PASS/50 FAIL including parents, exit1. Leaf failures:32 expected mat_ elevation assertions,2 expected local Cloud elevation assertions,6 fixture status expectations (removed membership returns404 rather than403). No infrastructure error. Source/config stable.
- Fixed only six status expectations to404; separated Cloud PUT/DELETE subtests so one failing case cannot suppress the other.
- green01:274 PASS exit0. Allocation-race record retained: cmd/server started after the requested label was provisioned but before the parent designated that label for handlers; completed before cancellation could intervene. No later reuse and not final acceptance.
- Explicit comment_router01:274 PASS, exit0; frozen source/config/Go binary unchanged; before/after DB identity and counts captured. Real HTTP listener + NewRouter auth in both modes.
- After router01 freeze, added queued-consumer denial sentinel and actual mdt_ negative case; these must be covered by the final frozen run. Source-ready-final manifest records their bytes.

## Coverage and follow-up
Main matrix covers JWT/PAT/mat_ × owner/admin/member × self/other human/other agent × body/attachment-only/delete/spoof; successful author/revision preservation, stale CAS conflict, denied full business-row state and no bus events. Negative lifecycle: actual task cancel route revokes token, deleted/expired task token, revoked PAT, restricted Reviewer, cross-workspace and removed membership. Direct-handler task/Cloud backstop tests await handler DB run. Cloud uses a local verifier only. Task credentials are actual generated mat_ strings with test-inserted hash rows, not claim lifecycle evidence.

Parent must provide fresh handler allocation and final coordinated full handler normal/race plus cmd/server normal/race after security diagnostics/dependency edits settle. No commits/pushes.

Built-in skill update required by CLAUDE.md: multica-working-on-issues/SKILL.md says owner/admin may edit without the human qualifier. Parent coordination requested for that file plus references/working-on-issues-source-map.md; no unauthorized edits made there. Generic upload contract unchanged and separately documented.

## Actual issuance and full-package follow-up
- Claimed legacy01 and password01 each134 PASS,0FAIL,0SKIP,exit0 on separate parent-owned databases, external frozen source/HOME/TMP, supported Go1.26.11. Full HTTP matrix now uses actual daemon claim/start and ordinary cancel routes. Token-row hash insertion remains only in the separate original auth matrix. Both claimed runs include queued-consumer non-cancellation and real mdt_ rejection. No provider ran.
- handler-normal01:6150PASS/53SKIP/4FAIL,exit1. Four existing plugin_example tests could not find examples/plugins/deploy-sentinel/multica.plugin.json because the exporter omitted examples. This is a harness failure, not product; original frozen copy/log preserved.
- Exporter corrected to include examples/packages/apps/scripts/package metadata; handler-race01 was frozen AFTER that correction and its plugin fixture hash matches. Race still in progress. Parent allocating handler_normal02 for fresh replay.
- Existing53 skips captured by exact names/reasons: Redis not configured, performance/live opt-ins, wire-fixture export and one stub-query TODO. No skips introduced or treated as pass.
- Required built-in skill and source-map wording updated after parent coordination. Generic upload unchanged.

## Verification harness corrections retained
handler-race01 ended with package timeout at default10m (4575PASS/9SKIP, exit1), while the active subtest had run less than1s. Original readonly run_backend.py uses -timeout=40m -p1 -parallel2; current runner now restores exactly those flags. No product/test deadline changed. Its database retains5users/1workspace/5comments after process timeout and is retained untouched.

Parent dedicated Redis indexes10/11/12/13 supplied for handler normal/race and router normal/race. Earlier default runs lacked REDIS_TEST_URL; subsequent runs include it and POSTGRES_DB, matching original backend environment. handler_normal02 runs with all corrections; handler_race02 fresh full replay required due prior package timeout, then router normal/race sequentially.

## Completed owning handler verification
handler-normal02 and handler-race02 both exit0 with6200PASS/7existingSKIP/0FAIL, Redis indexes10/11, original40m/p1/parallel2 budgets, Go1.26.11. All frozen source/config/tool/resource audits matched. Remaining skips: stub-query TODO, two iteration performance opt-ins, triage performance opt-in, triage wire exporter, live GitHub integration, no HTTP MCP catalog recipe. No new test skip.

Built-in documentation condensed to fit original500-line body limit after parent service conformance failure; focused TestBuiltinSkillsConformToTemplate passes. Hashes in skill-conformance01 receipt and lane-source-final manifest. Handler-normal02 source predates this prose-only shortening; owning handler code/tests/module bytes still match current and are independently bound. Race02 includes corrected prose.

Full router normal01/race01 DBs remain unused pending parent/runtime resolution of stale active-task references in default TestRK1RuntimeContinuationControls; see router-freeze-preflight.md. Actual issued-token probes both passed134 each with current handler/router test bytes.

## Final full router closure
router-normal01 and router-race01 each1751PASS/8existingSKIP,0FAIL,exit0. Current actual-claim and side-effect tests all pass normal/race. Separate final source/tool/config audit matched every9211file snapshot and original evidence; successful inventories have no unfinished tests. Archived-fixture concern was a mistaken default-suite classification because the source has agentintegration build tags; no change was made there. Full report rewritten around final state; original failed/misconfigured runs remain retained.
