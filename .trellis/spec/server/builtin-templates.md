# Built-in Template Registries

> Contracts for the server-embedded template registries (`builtin_agent_templates*`, `builtin_squad_templates*`, `builtin_autopilot_templates*`). A template is pre-fill content copied into an ordinary instance at creation — never a new entity kind.

## Convention: create_issue autopilot templates must carry a `{{date}}` issue title template

**What**: Every built-in autopilot template with `ExecutionMode: "create_issue"` must set `IssueTitleTemplate` to `"<English title> — {{date}}"` (em dash). Every `run_only` template must leave it empty so the column lands NULL.

**Why**: `dispatchCreateIssue` creates the issue BEFORE the agent runs, and `interpolateTemplate` (`server/internal/service/autopilot.go`) falls back to `ap.Title` when `issue_title_template` is empty. A `create_issue` template without `{{date}}` produces an identically-titled issue every period — 30 indistinguishable "每日变更回顾" issues in a month. `run_only` opens no issue, so a title template there would be dead weight.

**Example** (`server/internal/service/builtin_autopilot_templates_roster.go`):

```go
// Good — create_issue: dated title, one distinguishable issue per run
{
    Key:              "daily-change-review",
    ExecutionMode:    "create_issue",
    IssueTitleTemplate: "Daily Change Review — {{date}}",
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
    IssueTitleTemplate: "Daily Change Review",  // rejected by the roster test
    ...
}
```

**Enforcement**: `TestAutopilotTemplates_IssueTitleTemplates` (`server/internal/service/builtin_autopilot_templates_test.go`) asserts every create_issue template is non-empty, contains `{{date}}`, and passes `ValidateIssueTitleTemplate`; every run_only template is the empty string. `{{date}}` is the only placeholder `ValidateIssueTitleTemplate` accepts — do not add others without updating that validator and its docs.

## Gotcha: template-decided fields come from the server, never the request

> **Warning**: `POST /api/autopilots/from-template` must not accept `title` / `description` / `cron_expression` / `execution_mode` / `issue_title_template` from the client.
>
> Provenance honesty: a client that could supply its own prompt while claiming `template_key` would stamp a template's identity on an autopilot that runs something else entirely. The handler takes all content fields from the registry entry; the request carries only `template_key`, `assignee_id`, and optional `assignee_type` / `project_id` / `timezone` / `language` / `subscribers`. The e2e spec (`e2e/autopilot-template.spec.ts`) asserts the create body carries none of the template-decided fields.
