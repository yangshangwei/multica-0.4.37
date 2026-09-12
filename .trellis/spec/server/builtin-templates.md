# Built-in Template Registries

> Contracts for the server-embedded template registries (`builtin_agent_templates*`, `builtin_squad_templates*`, `builtin_autopilot_templates*`). A template is pre-fill content copied into an ordinary instance at creation — never a new entity kind.

## Role instructions, skills, and autonomy must agree

Default role skills describe work the role can actually deliver. Architect stays
Observer and returns a proposed ADR draft in an issue comment for an Implementer
or human to save; Technical Writer is Contributor and edits documentation only
on an isolated branch. Only Feature Delivery and Discovery leads default to
requirement clarification; other leads use their existing routing instructions.

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

The seven `builtin_role_skills/*/SKILL.md` bodies and discovery descriptions use
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

`GET /api/skills/templates` returns the seven role-skill templates under the
existing authenticated workspace route group. It reads the embedded registry,
does not materialize workspace rows, and returns no database identity. The UI
edits a snapshot and creates a new ordinary skill via `POST /api/skills`.
`config.template_source` is editable information, not authorization or official
provenance. Existing workspace instances, assignments and autonomy levels remain
independent from this read-only catalog.

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

The nine `builtin_autopilot_templates/*/PROMPT.md` bodies use canonical
Simplified Chinese, following the role-instruction and role-skill convention.
Locale selects catalog labels and the title stored on a newly created autopilot;
it does not select another execution body. The preview, `autopilot.description`,
and the dispatched brief must preserve the same content. Template releases and
locale changes never rewrite existing workspace copies.

Keep the five `run_only` patrols distinct from the four `create_issue` summaries.
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
eight templates retain version 1 for language-only wording changes.

`TestAutopilotTemplateCreate_ChineseTemplatesDispatchVerbatim` covers all nine
templates through real database creation, schedule dispatch and daemon claim;
`e2e/autopilot-template-zh.spec.ts` covers the Chinese browser flow with real APIs
and an isolated runtime fixture. Neither check launches a real model. Content
review must still compare the prompts' evidence, thresholds and authority rules;
string assertions alone do not establish semantic equivalence.

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
