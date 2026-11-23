# Squad handoff contract verification

Implemented in `/Volumes/artisan/code/2026/mc-agent-rnd-p1-20261105`, based on `f1d35b66af1013b6147c0e167500cacae203b7a8`. No commit, deployment, database, real provider or paid model was used by this lane.

## Changes and acceptance

- C1: Generated assignment/comment/leader workflow now distinguishes delivery (`in_review`) from stage closure (`done`/`cancelled`). Backend stage/notification code is unchanged; human/integration acceptance remains required.
- C2: Working-on-issues and squads skills, their source maps, and EN/ZH docs distinguish same-issue member results from child-stage completion. Cancelled child wording now preserves the parent-stage wake while stating that no fresh work starts on the closed issue. No parent-comment bypass was introduced.
- C3: Review Gate leader now routes all six existing specialists with the same applicability and evidence requirements as the squad. The regression checks each instruction body separately against role titles and routing conditions.
- C4: `review-gate-lead` version changed from 2 to 3; squad remains 3. Autonomy, concurrency, attached skills and instance reuse are unchanged.
- C5: Backend and EN/ZH frontend summaries, public docs and the embedded Chinese squads page agree. Locale keys are unchanged. Both shipped skills pass the canonical 500-line body budget and strict frontmatter checks.

## Red / green evidence

- `research/red-execenv.jsonl`: exit 1; assignment/comment/leader all reproduced the false stage-advancement promise. The existing status contract also failed after its wrong assertion was corrected.
- `research/red-service.jsonl`: exit 1; leader v2 and missing agent/experience/migration routing plus unknown/hold failed. Both skills lacked the clarified delivery/acceptance rule.
- `research/green-service.jsonl` and `green-service-summary.json`: retained unsuccessful first implementation run. It had 59 named passes and failed the canonical skill body budget (501 lines). This is not a passing verification.
- After compressing the corrected cancelled-status paragraph, `research/green-service-final.jsonl` and its summary passed: 61 named tests/subtests, zero failures or skips.
- `research/green-execenv.jsonl` and its summary passed: 22 named tests/subtests, zero failures or skips.

Selected tests are pure generated-content/template tests. Both package directories have no TestMain. All selected Go tests ran through the agent CLI guard with `-p 2 -parallel 2 -count=1 -timeout=3m`, GOMAXPROCS=2, removed inherited DATABASE_URL/PG* settings and Git-external task-owned TMPDIR/TMP/TEMP. The state directories were first checked with `git rev-parse --show-toplevel`, which returned nonzero with empty output.

Exact successful test argument vectors:

```text
["bash", "scripts/go-test-with-agent-cli-guard.sh", "--", "go", "-C", "server", "test", "-p", "2", "-parallel", "2", "-count=1", "-timeout=3m", "./internal/daemon/execenv", "-run", "^(TestSubIssueCreationSection.*|TestIssueWorkflow.*|TestBriefHasNoParentNotificationGuidance|TestStatusRuleIsFactJudgmentAtBothMoments|TestSquadLeaderIssueWorkflowKeepsParentInProgress|TestProtocolHeadingInInstructionsGetsNoLeaderBrief|TestInstructionPrecedenceOnlyAppliesToIssueWorkflow)$", "-json"]
["bash", "scripts/go-test-with-agent-cli-guard.sh", "--", "go", "-C", "server", "test", "-p", "2", "-parallel", "2", "-count=1", "-timeout=3m", "./internal/service", "-run", "^(TestReviewGateInstructionsCoverEveryRosterRole|TestBuiltinSkillsConformToTemplate|TestBuiltinSkillsFrontmatterIsStrictYAML|TestWorkingOnIssuesSkillCoversIssueLoopContracts|TestSquadsSkillCoversLeaderRoutingContract|TestSquadTemplates_.*|TestAgentRoleTemplates_.*|TestAgentRoleTemplate_.*|TestRoleSkillTemplates_RosterIsFifteen|TestReleaseGateGovernanceDrill_Contract)$", "-json"]
```

## Other verification

- `go -C server vet -p 2 ./internal/daemon/execenv ./internal/service`: exit 0.
- `go -C server test -p 2 -count=1 ./internal/docs`: exit 0, package PASS (0.925s).
- `gofmt -l` for all six changed Go files: no output.
- Scoped `git diff --check`: exit 0.
- Python JSON parse/key checks: both locale key sets equal their HEAD baselines; catalog summary keys match EN/ZH.
- English comment-routing link resolves to the actual `leader-re-trigger-rules` heading using the existing docs TOC parser.
- Existing docs generator ran against a Git-external temporary output, produced 39 pages and 58 assets. Byte comparison to the worktree bundle found exactly `pages/squads.json` changed, which was copied. Manifest and unrelated pages/assets are unchanged. No dependency install: the existing external `github-slugger@2.0.0` was linked under ignored `apps/docs/node_modules` for module resolution.
- Final owned-file SHA-256 values are in `research/source-manifest.json`.

## Changed files

- `server/internal/daemon/execenv/runtime_config_sections.go`
- `server/internal/daemon/execenv/runtime_config_test.go`
- `server/internal/service/builtin_agent_autonomy_test.go`
- `server/internal/service/builtin_agent_templates/review-gate-lead/INSTRUCTIONS.md`
- `server/internal/service/builtin_agent_templates_roster.go`
- `server/internal/service/builtin_squad_templates.go`
- `server/internal/service/builtin_skills_test.go`
- `server/internal/service/builtin_skills/multica-working-on-issues/SKILL.md`
- `server/internal/service/builtin_skills/multica-working-on-issues/references/working-on-issues-source-map.md`
- `server/internal/service/builtin_skills/multica-squads/SKILL.md`
- `server/internal/service/builtin_skills/multica-squads/references/squad-source-map.md`
- `server/internal/service/builtin_skills/multica-creating-agents/references/creating-agents-source-map.md`
- `apps/docs/content/docs/squads.mdx`
- `apps/docs/content/docs/squads.zh.mdx`
- `packages/views/locales/en/squads.json`
- `packages/views/locales/zh-Hans/squads.json`
- `server/internal/docs/content/pages/squads.json`

## Limits and remaining work

- No backend state-machine behavior was changed or revalidated with a database. The fix removes incorrect runtime guidance; automatic delivery-before-acceptance is a later workflow decision.
- No real-model instruction-following claim; these tests verify the shipped contract and generated content.
- Existing saved leader/skill/squad copies are not upgraded automatically. The v3 leader applies when a new template instance is created; existing customized content is preserved.
- No TS source was changed. Full frontend typecheck/lint/browser suites were not run for JSON string and documentation edits.
- Main session owns spec integration, independent review, task status and commits.
