# Built-in Template Registries

> Contracts for the server-embedded template registries (`builtin_agent_templates*`, `builtin_squad_templates*`, `builtin_autopilot_templates*`). A template is pre-fill content copied into an ordinary instance at creation — never a new entity kind.

## Role instructions, skills, and autonomy must agree

Default role skills describe work the role can actually deliver. Architect stays
Observer and returns a proposed ADR draft in an issue comment for an Implementer
or human to save; Technical Writer is Contributor and edits documentation only
on an isolated branch. Only Feature Delivery and Discovery leads default to
requirement clarification; other leads use their existing routing instructions.

Progress Reporter uses Contributor only so a
successfully delivered report can move its own assigned report issue to
`in_review`; its instructions keep business issues and repository files
read-only. Its sole default role skill is `multica-progress-report`, and its
concurrency cap is one. Contributor is an existing permission level, not a
report-only API sandbox; never claim the role's narrower instructions add a
new server-enforced boundary.

Diagnostician brings the listed roster to ten roles and the role-skill catalog
to nine entries. It uses Contributor, concurrency one, and only
`multica-debugging` (`quality`, `microscope`). It reproduces an observed failure,
tests hypotheses, and hands evidence and a suggested fix direction to the
Implementer; it does not submit the fix or change issue status. Confirmed causes
need a reproducible evidence chain and a concrete location/mechanism. Suspected
or unresolved investigations may close with explicit evidence gaps rather than
inventing a minimal reproduction or fix direction. Data loss, security defects,
or exposed credentials require reporting and stopping for human handling.
These narrower role/skill instructions do not add a server-enforced sandbox.

The default squad placement is deliberately narrow. `bug-fix` seats the
Diagnostician between reproduction and implementation when the cause is unknown;
`maintenance` seats it for unexplained flaky-test or upgrade failures; and
`incident` seats it for a separate post-recovery root-cause follow-up without
delaying mitigation. Feature delivery, review gate, discovery, docs, and release
do not provision the role by default. The corresponding squad instructions and
leader templates must describe the same handoff; changing only one layer creates
conflicting routing advice at claim time.

Increment template and role-skill versions for material behavior changes. These
defaults apply when creating agents or materializing a missing role skill;
existing workspace copies are reused without overwriting customized content.
`builtin_agent_templates_test.go` pins the full role-to-skill and autonomy maps.

## Role skill body language and explicit workspace updates

Role, squad-leader, squad, and Mika instruction bodies are authored in
Simplified Chinese. Picker language still selects labels and descriptions only;
the instructions editor shows the canonical persisted body. Preserve CLI syntax,
protocol values, the Mika name placeholder, and each role's behavior when
translating. Existing copies are never retranslated when a viewer changes locale.

Use the explicit maintenance workflow in `scripts/localize-agent-instructions.md`
for existing instructions. Template key/version alone cannot identify an
unmodified copy: compare the full historical body, and translate that version's
rules. Agent/squad PUT supports paired `expected_instructions` and
`expected_updated_at` fields with an atomic SQL comparison. Echo GET's complete
RFC3339 timestamp, discover `X-Multica-Instructions-Precondition: 1`, and finish
upgrading every API replica before relying on this optional-field contract.
Stale writes return 409 without updates or events; successful writes keep the
normal notification path. Conditional rollback also compares the recorded
post-write timestamp and validates the original backup hash.

The nine `builtin_role_skills/*/SKILL.md` bodies and discovery descriptions use
Simplified Chinese. Descriptions state when the skill applies and its concrete
result, preserving important delivery and authority boundaries without repeating
the workflow. Keep canonical names, other frontmatter, CLI syntax and
protocol/status values unchanged. Translate the working method, not the role's
permissions or business rules. These translations do not
set a global response language or change the role instructions' output contract.

Language-only edits keep the existing behavior version; bundle hashes already
track content changes. Materialization must continue to reuse existing workspace
copies unchanged. Updating an existing copy is an explicit content update scoped
to the intended workspace, with unrelated frontmatter and customizations
preserved. Historical bodies need their own translation when their rules differ
from today's template; do not silently upgrade behavior while localizing.

Grounding changes to the ADR, requirement-clarification and documentation skills
are material behavior changes, not translation-only edits. Their versions are
3, 3 and 2 respectively. ADR facts need supporting task/code/document evidence;
requirement clarification separates known requirements from unverified technical
advice; documentation summaries must follow searches of the final files and
disclose coverage limits. Requirement-clarification version 3 retains the generic
evidence rules and removes CSV-specific routing from the default role skill.
CSV regression review material lives in
`scripts/skill-eval/references/csv-export-safety.md`, outside distributed skill
bundles and model fixtures. Preserve the original CSV case inputs so the reviewer
reference does not become a supplied answer.
Supporting references must ship in `RoleSkillTemplate.Files` and in any native
evaluation snapshot or explicit workspace update. The reference-delivery test
`TestRoleSkillTemplates_FilesMatchSource` in `builtin_agent_templates_test.go`
checks complete bundles against source files without requiring a domain-specific
attachment on a general-purpose role skill.

`skill update --content-file` replaces the entire stored content but does not
synchronize frontmatter into database metadata. When updating a description,
send both `description` and the content with matching frontmatter through the
normal update API. Omit `files` and unrelated fields
to preserve supporting files, bindings and labels. The endpoint has no atomic
compare-and-swap contract: compare before writing and verify afterwards, without
claiming those checks eliminate concurrent-write races. New tasks receive the
updated body through the existing content-hash cache.

## Skill template catalog

`GET /api/skills/templates` returns the nine role-skill templates under the
existing authenticated workspace route group. It reads the embedded registry,
does not materialize workspace rows, and returns no database identity. The UI
edits a snapshot and creates a new ordinary skill via `POST /api/skills`.
`config.template_source` is editable information, not authorization or official
provenance. Existing workspace instances, assignments and autonomy levels remain
independent from this read-only catalog.

### Scenario: operator-mounted skill-template directory

**1. Scope / Trigger** — Infra/env wiring: air-gapped deployments cannot pull
skills from clawhub/skills.sh/github, so the catalog gains a second source read
from a read-only mounted directory. Handler source switched from
`service.RoleSkillTemplates()` to `TaskService.SkillTemplates()`.

**2. Signatures**
- `func (s *TaskService) SkillTemplates() []RoleSkillTemplate` — nil-safe;
  `RoleSkillTemplates()` (embed, stable order) first, then mounted entries.
- `scanSkillTemplateDir(dir string, embedNames map[string]struct{}) []RoleSkillTemplate`.

**3. Contracts**
- Env: `MULTICA_SKILL_TEMPLATE_DIR` (optional). Read once at handler assembly,
  `TrimSpace`d into `TaskService.SkillTemplateDir`. Empty/unset = embed-only.
- Compose: `${SKILL_TEMPLATE_DIRECTORY:-./skill-templates}:/app/data/skill-templates:ro`;
  default env points at that mount.
- On-disk: `<name>/SKILL.md` (+ optional supporting files), byte-identical shape
  to `builtin_role_skills/<name>/`. Directory name is the authoritative template
  `Name`; frontmatter still parsed for `description`.
- Response: unchanged `SkillTemplateResponse` (`name/version/description/content/files`);
  mounted entries carry `version: 0`. No frontend or `SkillTemplateListResponseSchema`
  change — `version` already `.default(0)`.

**4. Validation & Error Matrix** (each is skip-one-entry + `slog.Warn`, never a list-level failure)
- name fails `^[A-Za-z0-9_-]+$` (also blocks `..`, `/`, `\`, dot-prefix) -> skip
- top-level entry is a symlink (`entry.Type()&os.ModeSymlink`) -> skip, SKILL.md never read
- not a directory / missing `SKILL.md` / frontmatter has no `name` -> skip
- name clashes with an embed role skill -> skip (embed wins, D4)
- supporting file is a symlink, or per-file (1 MiB) / total (8 MiB) / count (256) cap exceeded -> skip file
- dir unset / missing / unreadable -> embed-only, no error (missing dir is not `slog.Warn`ed)

**5. Good/Base/Bad Cases**
- Good: `team-style/SKILL.md` + `references/lint.md` -> lists after embed with files.
- Base: env unset -> byte-identical to the prior embed-only catalog.
- Bad: `sneaky` symlinked to an external dir -> not listed, body not leaked.

**6. Tests Required** — `skill_template_dir_test.go`: `EmbedOnlyWhenDirUnset`
(nil/`{}`/empty), `MissingDirIsEmbedOnly`, `MountedEntryWithFiles`,
`MalformedEntriesSkipped`, `OversizedSkillMdSkipped`, `PathEscapeSkipped`
(supporting-file symlink), `TopLevelSymlinkSkipped`, `EmbedWinsOnNameClash`.
`skill_template_test.go`: `IncludesMountedDirectory` + the retained verbatim
embed-only test (nil `TaskService`). Frontend: `skill-template-schemas.test.ts`
(version-0/non-builtin parse + malformed-mount fallback) and
`skill-presentation.test.ts` (mounted name -> null -> raw-description panel fallback).

**7. Wrong vs Correct**
- Wrong: `os.Stat(<name>)` to test IsDir — follows a top-level symlink and reads
  its `SKILL.md`, leaking an out-of-tree body even though supporting files are refused.
- Correct: check `entry.Type()&os.ModeSymlink != 0` first (link's own mode, no
  follow) and skip; then `entry.IsDir()`. Symlink refusal is uniform across the
  top-level entry and supporting files.

## Onboarding skills require task provenance

`BuiltinSkills()` returns general platform skills. Use `TaskBuiltinSkills` for
dispatch: `multica-onboarding` requires the task's own built-in Mika and a
persisted `onboarding_kickoff` in its chat session. The session marker preserves
availability for follow-up and retry turns. Do not use display names, prompt
text, or only the current input batch to decide eligibility.

Both inline and slim claims, and subsequent bundle resolution, must use the same
scope. A failed scope read must fail the request rather than silently omit the
skill. General builtin downloads do not query onboarding scope. Regression
coverage lives in `daemon_builtin_skills_scope_test.go` and
`builtin_skill_scope_test.go`.

## Autopilot prompt language and business boundaries

The ten `builtin_autopilot_templates/*/PROMPT.md` bodies use canonical
Simplified Chinese, following the role-instruction and role-skill convention.
Locale selects catalog labels and the title stored on a newly created autopilot;
it does not select another execution body. The preview, `autopilot.description`,
and the dispatched brief must preserve the same content. Template releases and
locale changes never rewrite existing workspace copies.

Keep the five `run_only` patrols distinct from the five `create_issue` summaries.
Patrols search the workspace for their existing open issues before creating work,
append evidence to an existing issue when it covers the finding, and create no
issue or comment when there is no substantive finding. Summaries comment on the
issue dispatch already created, including for an empty reporting period. A
missing check result is not a verified failure or a zero count.

The bug-triage template is version 2 because it explicitly maps severity to the
supported `priority` values: `urgent`, `high`, `medium`, and `low`. `none` remains
unprioritized when evidence is insufficient. Contributor authority is required
by its instructions to change priority; observers recommend in comments. State
whether each write was applied or only recommended, according to the actual
response. Do not assume that every priority write has a dedicated server-side
observer rejection: the task's autonomy policy must also be respected. The other
templates keep their existing versions unless their behavior changes.

Daily Progress Report v1 is immediately before Weekly Progress Report v2 in
the roster. Daily uses `0 18 * * *` and the preceding 24 hours; weekly keeps
`0 17 * * 1` and the preceding seven days. Both are `create_issue` summaries
with dated titles. Template adoption selects timezone and execution context;
source queries must separately apply the requested workspace/project filter.

Both reporting prompts require paginated evidence and status history for
period completions, disclose truncation/missing data, and exclude reporting
issues from business counts. A successful report comment permits only the
current report issue to enter `in_review`. `TaskService.CompleteTask` does not
change issue status; `SyncRunFromIssue` completes a create-issue run on
`in_review` or `done`. An Observer can post a report but cannot make that
transition. Weekly v2 repairs the previous blanket status prohibition;
existing v1 workspace copies are deliberately preserved.

`TestAutopilotTemplateCreate_ChineseTemplatesDispatchVerbatim` covers all ten
templates through real database creation, schedule dispatch and daemon claim;
`e2e/autopilot-template-zh.spec.ts` covers the Chinese browser flow with real APIs
and an isolated runtime fixture. Neither check launches a real model. Content
review must still compare the prompts' evidence, thresholds and authority rules;
string assertions alone do not establish semantic equivalence.

`autopilot_template_progress_test.go` covers daily/weekly comment delivery and
review closeout, Observer refusal and unchanged source tasks with real DB
fixtures. Its handler harness invokes the sync callback explicitly;
`e2e/progress-reporting.spec.ts` verifies the real server listener wiring and
the role/catalog/adoption UI. E2E report comments are explicit fixtures, not
evidence that an LLM independently gathered or summarized the data.

## Convention: create_issue autopilot templates must carry a `{{date}}` issue title template

**What**: Every built-in autopilot template with `ExecutionMode: "create_issue"` must set `IssueTitleTemplate` to `"<Chinese title> — {{date}}"` (em dash), using its canonical Chinese catalog title. Every `run_only` template must leave it empty so the column lands NULL.

**Why**: `dispatchCreateIssue` creates the issue BEFORE the agent runs, and `interpolateTemplate` (`server/internal/service/autopilot.go`) falls back to `ap.Title` when `issue_title_template` is empty. A `create_issue` template without `{{date}}` produces an identically-titled issue every period — 30 indistinguishable "每日变更回顾" issues in a month. `run_only` opens no issue, so a title template there would be dead weight.

**Example** (`server/internal/service/builtin_autopilot_templates_roster.go`):

```go
// Good — create_issue: dated title, one distinguishable issue per run
{
    Key:              "daily-change-review",
    ExecutionMode:    "create_issue",
    IssueTitleTemplate: "每日变更回顾 — {{date}}",
    ...
}

// Good — run_only: zero value, column lands NULL
{
    Key:           "hourly-queue-check",
    ExecutionMode: "run_only",
    ...
}

// Bad — create_issue without {{date}}: every run opens an identically-titled issue
{
    ExecutionMode:      "create_issue",
    IssueTitleTemplate: "每日变更回顾",  // rejected by the roster test
    ...
}
```

**Enforcement**: `TestAutopilotTemplates_IssueTitleTemplates` (`server/internal/service/builtin_autopilot_templates_test.go`) asserts every create_issue template uses its Chinese title, contains `{{date}}`, and passes `ValidateIssueTitleTemplate`; every run_only template is the empty string. `{{date}}` is the only placeholder `ValidateIssueTitleTemplate` accepts — do not add others without updating that validator and its docs. A schedule dispatch uses its trigger's timezone; a manual run without a trigger ID keeps the existing UTC fallback.

## Gotcha: template-decided fields come from the server, never the request

> **Warning**: `POST /api/autopilots/from-template` must not accept `title` / `description` / `cron_expression` / `execution_mode` / `issue_title_template` from the client.
>
> Provenance honesty: a client that could supply its own prompt while claiming `template_key` would stamp a template's identity on an autopilot that runs something else entirely. The handler takes all content fields from the registry entry; the request carries only `template_key`, `assignee_id`, and optional `assignee_type` / `project_id` / `timezone` / `language` / `subscribers`. The e2e spec (`e2e/autopilot-template.spec.ts`) asserts the create body carries none of the template-decided fields.
