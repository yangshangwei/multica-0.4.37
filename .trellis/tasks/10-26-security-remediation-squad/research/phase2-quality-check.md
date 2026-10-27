# Phase-two quality review

Reviewed the product diff against `18b176612` and the current `phase2-plan.md` on
2026-10-27. The latter is the current authorization and acceptance boundary;
historical real-model gates remain unpassed. No real provider was started by
this review. Prior simulation files and unrelated workspace/journal changes are
outside this product review.

## Findings (fixed)

The main session applied the first two corrections after this review reported
them. The UI agent corrected the browser assertion after its initial execution.
The reviewer re-read the final sources, tests and execution evidence; no
additional product edits were needed.

1. **Parent-status ownership.**
   Files: `server/internal/service/builtin_agent_templates/security-remediation-lead/INSTRUCTIONS.md`,
   `server/internal/service/builtin_squad_templates/security-remediation/INSTRUCTIONS.md`,
   `server/internal/service/builtin_security_remediation_templates_test.go`, and
   `server/internal/handler/squad_template_security_test.go`.
   The new bodies initially prescribed status transitions without the ownership
   prerequisite, conflicting with the injected guest-leader protocol. Both now
   require actual assignment to this squad plus status ownership in the injected
   protocol. Guest mentions and quick-create without ownership retain the
   assignee and status, and may not acquire authority by reassigning the issue.
   Per-body tests require the ownership condition before either status value.
   A new composition test uses the staffed leader and squad bodies with both
   owning and guest briefings, renders the real runtime configuration without
   launching a provider, and verifies the guest has no owning-status grant or
   command. Owning closeout remains available.
2. **Built-in skill UI entry description.**
   Files: `server/internal/service/builtin_skills/multica-squads/SKILL.md` and
   `references/squad-source-map.md` in that skill.
   Inherited wording incorrectly identified the standalone staffing endpoint
   as the current catalog action. The text now distinguishes standalone
   `POST /api/squads/from-template` from “Apply to project” and its plural
   `PUT /api/projects/{id}/execution-squads`. It correctly describes replacing
   the complete ordered selection, preserving existing choices, appending the
   new selection, and the `needs_runtime`/`failed` recovery states. The source
   map also removes inherited wording denying the existing guest-status gate.
3. **Persisted configuration is not derived execution readiness.**
   File: `e2e/security-remediation-squad.spec.ts`.
   The initial English and Chinese browser cases expected the saved selection
   to contain `state: ready`; the API correctly returned `configured`.
   `ProjectExecutionSquad.state` permits `none`, `needs_runtime`, `configured`
   and `failed`; `configureProjectSquads` writes `configured` after successful
   materialization, while `getProjectSquadReadiness` separately derives `ready`
   from current targets and runtime availability. The test now imports the
   canonical `ProjectExecutionSquad` type and asserts `configured`, retaining
   the squad identity, provenance, localized UI, request-shape and renamed-copy
   checks. This changes only the test and an explanatory comment; it does not
   relax product validation or change runtime behavior. Both cases then passed.

## Findings (not fixed)

No remaining blocking code, instruction, test or spec findings. Existing
unrelated lint warnings remain outside this task. Real-model behavior is an
explicitly unverified scope boundary, not a passed gate.

## Verified contracts

- `security-remediation` and its dedicated unlisted leader are version 1. The
  leader is Coordinator with concurrency two and no default role skill. The
  seven working seats remain existing listed roles; the fresh roster contains
  no Operator. Placement precedes Release and Incident. Existing template
  versions, working-role instructions and role-skill definitions are unchanged.
- Both new instruction files describe dependency/code/other routing, complete
  evidence fields, conditional migration review, one minimal repair, independent
  QA and code/security review. Both retain the Diagnostician's safety stop and
  prohibit routing the same work to another member to bypass any required stop.
  Production exploitation, sensitive-data exposure and high-risk operations
  stop local execution for human/incident/release handling. Missing targets or
  evidence retain an owner and `hold/unknown`.
- The unchanged staffing path checks Coordinator authority and the template
  ceiling before writes, validates the runtime/workspace, and creates agents,
  squad and roster within one transaction. Reuse reads Access through the same
  transaction and returns workspace-owned agents unchanged. New tests verify
  private defaults, localized names and notes, exact roster/provenance, preserved
  runtime/instructions/autonomy/skills/bindings, denied reuse rollback and the
  unlisted-leader endpoint refusal.
- Templates are added to the existing embedded directories. No schema,
  endpoint, dependency, runtime-provider or database change is introduced.
  Existing string-key, loose API schemas accept the new entry; the catalog keeps
  its unknown-key description fallback. Shared English/Chinese summaries and
  the existing `shield-check` icon avoid platform-specific changes.
- New fixture-only browser tests cover both languages, nine catalog entries,
  eight displayed seats, the shield icon, project adoption request/data and
  opening a renamed existing squad. The existing squad audit retains prior
  assertions and includes the new template. The project-workspace eight-squad
  scenario intentionally selects eight ordinary squads; it is not a stale
  catalog-count assertion.
- English/Chinese product documentation distinguishes prompt constraints from
  filesystem/network isolation and scripted checks from model compliance. The
  three generated Chinese pages contain the new content, and the generated
  squad TOC contains the linked `安全漏洞修复` anchor. The corrected English
  mention-routing text matches `comment.go`: explicit agent/squad mentions take
  precedence over the agent-result fallback to the assigned leader.

## Verification

The main session owns verification execution; this reviewer inspected its
output without starting competing test runs or editing tested product sources.

- **Lint: pass.** Main `scripts/check.sh` step 1 completed all 15 static tasks
  with zero cache hits. Existing warnings in unrelated files remain; there were
  no lint errors. UI wildcard-export check passed.
- **TypeCheck: pass.** The same completed static step checked the participating
  packages, including views, docs, core, Web and Desktop.
- **Focused backend tests: pass.** Independently inspected
  `backend-template-tests-summary.json`: 26 service plus 30 handler top-level
  tests, exact expected run/pass sets, no failures and no skips, using the agent
  CLI guard. These are product-contract checks, not model evaluation.
- **Review-fix regressions: pass.** Independently inspected
  `backend-review-fixes-tests-summary.json` and the actual test bodies: four
  service plus four handler top-level tests, 26 passes including subtests,
  exact expected run/pass sets, no failures and no skips. The guarded run
  covers both new ownership checks, actual copied content, skill contracts
  and the existing owning/guest composition tests. Reviewer-run `gofmt -l`
  returned no files for either changed Go test file.
- **Full TypeScript tests: pass.** Main pipeline output records 12,807 passes:
  core 4,173; docs 62; views 7,049; Desktop 1,238; Web 285.
- **Go race tests and vet: pass.** The main script completed its isolated Go
  race/vet step and proceeded to start the isolated API. The API became healthy.
- **Original full pipeline: incomplete, exit 1.** At production-Web startup,
  the existing build-ownership fence refused to rebuild while
  `work-management-local` still used this checkout's Web build. This was not a
  failing product test. The script stopped its own API and removed its isolated
  Go database. The existing environment was left intact. Do not describe this
  original invocation as an all-green `make check`.
- **Production-Web/E2E continuation: pass after correcting the test.** Main
  completed the production build and ran five selected fixture-only browser
  files from the isolated `.omx/reports/security-remediation-phase2/checkout`
  worktree. The initial 32-case execution passed 30 and failed only the two
  incorrect `ready` assertions. The English/Chinese rerun passed both cases
  with zero skips, unexpected results or flaky retries. The reviewer compared
  `(file, title)` identities: the rerun covers exactly the initial failures,
  yielding 32 distinct passing logical cases. The initial failures and their
  command exit remain in the evidence; this is not a claim that the first run
  passed. Commands use one worker and zero retries. The main session also
  reports 16 passing canonical core checks for the configuration/readiness
  distinction.
- **Snapshot and rerun scope:** reviewer verified all 30 copied product/test
  files against both checkouts and the manifest before the final test change,
  then verified the final main-checkout bytes against the updated SHA-256
  manifest after the correction. Main verified both checkouts before removing
  the temporary worktree. The base is `18b176612`; the install was frozen,
  offline and reported zero downloads. The final correction changed only the
  E2E file, so the passing rerun appropriately reused the unchanged product
  build. The full TS and Go suites were not repeated for the prompt/documentation
  corrections; affected backend contracts were rerun as documented above.
- **Generated bundle:** new page content and anchor inspected; main separately
  reported 98-file byte parity, 62 docs tests and the docs build. Final delivery
  should retain that evidence alongside the full pipeline.

Task evidence is under `../evidence/phase2/`, including
`verification-summary.json`, both browser execution summaries, both focused
backend summaries, snapshot hashes and `cleanup.json`. Raw-log paths and hashes
are retained in `log-provenance.json`; the original private log directory is
`/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-security-phase2-hwx7f_6x`.

## Completion notes

This review is complete with no blocking findings. The built-in-template spec
now records the ninth squad, dedicated leader, four Diagnostician placements,
explicit status ownership and persisted `configured` versus derived readiness.
The reviewer checked those statements against their source paths.

`cleanup.json` records successful cleanup of the two task-owned environments,
removal of the dedicated test database and removal of the temporary worktree.
The reviewer confirmed the worktree path no longer exists. Main owns the
remaining acceptance notes, commit and task archival; this reviewer did not
commit, modify live APIs or change historical simulation/provider evidence.

The template does not establish autonomous model judgment, instruction
compliance or guaranteed closure. Do not relabel `real_model_gate_passed` or
claim a new runtime sandbox. Existing unrelated lint warnings and workspace
changes were left intact.
