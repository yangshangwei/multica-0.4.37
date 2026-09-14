---
name: multica-autopilots
description: "Use when creating, updating, inspecting, triggering, or debugging a Multica autopilot (scheduled, webhook, or manual)."
user-invocable: false
allowed-tools: Bash(multica *)
---

# Multica Autopilots

## Quick start

Autopilots are durable automations. Read before mutating:

```bash
multica autopilot list --output json
multica autopilot get <autopilot-id> --output json
multica autopilot runs <autopilot-id> --output json
```

Do not run `trigger`, `delete`, `trigger-delete`, or `trigger-rotate-url` to test. Those are real side effects.

## Core model

An autopilot is not an agent. It is a rule that dispatches work to an agent, or to a squad's leader agent.

The chain is: trigger fires (`schedule`, `webhook`, or `manual`) -> `autopilot_run` row -> `execution_mode` decides output -> assignee readiness check -> issue/task execution -> run status sync. Webhooks have a durable admission step in front: HTTP ingress stores a queued `webhook_delivery`, synchronously creates or reuses its idempotent run, and returns `200` with `status=accepted|skipped` plus `run_id`; a database-leased worker then resumes accepted runs and owns recoverable issue/task dispatch.

Execution modes:

- `create_issue` creates a Multica issue, making the run visible as issue state.
- `run_only` creates an agent task directly. No issue is created; any durable
  report location has to come from other task context or instructions.

`issue-title-template` only supports `{{date}}`. Do not invent `{{trigger_id}}`, `{{branch}}`, or other variables.

## Built-in templates

The built-in automation roster ships with the server binary. Two HTTP endpoints expose it (there is no CLI subcommand for either yet):

- `GET /api/autopilots/templates?language=` lists the roster with localized card copy (en/zh/ja/ko). Read-only and workspace-independent — every workspace on the server gets the same answer.
- `POST /api/autopilots/from-template` creates an autopilot AND its schedule trigger in one transaction. The request body carries only `template_key`, `assignee_id`, and optional `assignee_type` / `project_id` / `timezone` / `language` / `subscribers`; the title, prompt, issue title template, cron, and execution mode come from the server-side template, so a client cannot claim a template's provenance while supplying its own prompt. The result is an ordinary, fully editable autopilot stamped with `template_key` / `template_version` provenance.

The built-in prompts and dated summary issue titles use canonical Simplified Chinese. `language` selects catalog labels and the new autopilot's title, not its execution language. Creation copies the full prompt into `description`; later template releases or language changes never overwrite an existing workspace copy. The bug-triage template maps severity to the supported priorities `urgent` / `high` / `medium` / `low`; `none` stays unprioritized when evidence is insufficient, and read-only agents recommend rather than apply changes.

`daily-progress-report` (v1) and `weekly-progress-report` (v2) are adjacent
summary templates. Daily is `0 18 * * *` with a 24-hour window; weekly is
`0 17 * * 1` with a seven-day window. One `progress-reporter` agent can own
both. Select the timezone explicitly and state the source-query scope; linking
a project supplies execution context and report ownership, not automatic query
filtering. Reports exclude reporting issues from business counts.

These reporting templates post on the issue already created for this run.
Only after the report comment succeeds may the current report issue move to
`in_review`, completing the run; business issues stay read-only. This requires
Contributor authority. An Observer posts the report and names the remaining
human review transition instead of bypassing its policy. A model task finishing
does not itself close an issue or complete a `create_issue` run. Existing saved
weekly v1 prompts are not rewritten when v2 ships.

## CLI

```bash
multica autopilot list --output json
multica autopilot get <autopilot-id> --output json
multica autopilot create --title "<title>" --description "<task prompt>" --agent <agent-name-or-id> --mode create_issue|run_only --output json
multica autopilot update <autopilot-id> --status active|paused --output json
multica autopilot runs <autopilot-id> --output json
multica autopilot trigger-add <autopilot-id> --kind schedule --cron "0 9 * * *" --timezone Asia/Shanghai --output json
multica autopilot trigger-add <autopilot-id> --kind webhook --label "ci" --output json
multica autopilot trigger <autopilot-id> --output json
multica autopilot trigger-rotate-url <autopilot-id> <trigger-id> --yes --output json
```

Use `trigger` only when the user explicitly asks for a manual run. Use `trigger-rotate-url` only when rotating a webhook URL; the old URL stops being valid.

`autopilot get` redacts `webhook_token`, `webhook_path`, and `webhook_url` by default while reporting whether a token exists and its non-sensitive hint. Keep ordinary inspection redacted.

An agent may add `--show-secrets` only when all three conditions hold: the user
explicitly requested the live webhook credential, the agent's autonomy level is
`operator`, and a recorded human approval for `secret_access` is `approved` and
covers this exact read. Other roles hand the request to a human or an Operator;
a user request alone does not replace the approval. The flag warns on stderr,
but does not grant permission or enforce this approval policy. Do not paste
webhook tokens or signing material into comments, logs, docs, or PRs.

## Debugging

For "why didn't it run":

1. `multica autopilot get <id> --output json` — status, mode, assignee, triggers.
2. `multica autopilot runs <id> --output json` — run status and failure reason.
3. If assigned to a squad, inspect the squad: `multica squad get <squad-id> --output json`; execution goes to the leader.
4. Inspect the target agent/runtime: `multica agent get <agent-id> --output json` and `multica runtime list --output json`.
5. For webhooks, inspect delivery status: `queued` means the worker has not completed dispatch; `failed` carries the worker error. A provider retry with the same `X-GitHub-Delivery` / `Idempotency-Key` reuses the original delivery.
6. For `create_issue`, inspect the created issue if the run records one.

## Side effects

These mutate durable state or start work: `create`, `update`, `delete`, trigger add/update/delete/rotate, `trigger`, and webhook calls to `/api/webhooks/autopilots/{token}`.

More source-backed details: `references/autopilots-source-map.md`.
