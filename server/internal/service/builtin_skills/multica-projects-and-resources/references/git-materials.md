# Git requirement and plan materials

Use this workflow when the task names a project material or asks for a PRD,
technical design or task breakdown in the configured material repository.
Git owns the versioned definitions; Multica owns discussion, review decisions,
assignment and execution. A normal project resource is not automatically a
material output repository. A human configures the binding, reviewers and
planning agent in the project's Engineering materials tab.

## Read before writing

Use the exact project and material UUIDs from the task context. These commands
do not resolve project names, short prefixes or issue display identifiers.
Their output is always JSON; no `--output` flag is needed.

```bash
multica project material list <project-uuid>
multica project material get <project-uuid> <material-uuid>
```

Read `material.version`, the current `revisions`, `baseline`, `plan`,
`capabilities` and any `blocker`. A missing baseline means there is no verified
requirement input yet. A Git review URL or human approval is not proof of merge,
and a planning task ID is not proof that an agent ran.

## Create a requirement draft

Choose and retain one request UUID for this create intent. The command never
generates one or automatically retries.

```bash
multica project material create <project-uuid> --title "<requirement title>" --request-id <request-uuid>
```

Keep the returned material ID and version. Creating the draft does not publish
it to Git. The server allocates its stable material path; title, progress,
completion and archive changes do not move the Git files.

## Save one exact revision

Write the request to a UTF-8 JSON file, then send that file or stdin:

```bash
multica project material revise <project-uuid> <material-uuid> --file revision.json
multica project material revise <project-uuid> <material-uuid> --file - < revision.json
```

The input limit is 600 KiB. Use exactly these fields; all three arrays and the
explicit `ready` boolean are required. IDs and commit hashes below are
placeholders to replace with actual task-owned values.

```json
{
  "request_id": "<request-uuid>",
  "expected_version": 2,
  "kind": "requirement",
  "content": "<complete Markdown requirement, including scope and constraints>",
  "sources": [
    {
      "resource_id": "<authorized-project-resource-uuid>",
      "commit": "<full-40-or-64-character-commit-sha>",
      "path": "src/example.go"
    }
  ],
  "acceptance_criteria": ["AC1: <observable expected outcome>"],
  "blocking_questions": [],
  "ready": false
}
```

`expected_version` is the material version you actually read. Saving an
incomplete draft keeps `ready=false`. Once clarification is complete, submit
the complete content, authorized fixed-version sources and uniquely numbered
acceptance criteria with `ready=true` and no blocking questions. The server
checks readiness and automatically queues publication to the configured Git
branch/PR/MR; there is no separate submit-for-review command. Content fields
are each limited to 256 KiB. Use actual evidence, not inferred execution or
fabricated source commits.

For a technical design and task breakdown, set `kind` to `plan`, put the
complete design in `content`, include the complete task definitions in
`plan_content`, and set `source_baseline_id` to the verified requirement's
`baseline.id`. Preserve its acceptance conditions and fixed source references.
Both plan documents are required when `ready=true`. Omit unused optional
fields instead of sending null. Server-side access and readiness checks remain
authoritative even after local CLI validation.

## Verify and recover

After a successful save, inspect the returned revision and state, then use
`get` to follow publication, review, verified merge and planning. A successful
save alone does not mean a PR/MR was created or merged. For a timeout or an
unconfirmed response, retain the original request UUID and exact JSON/flags;
read current state and retry that same intent if needed. Do not substitute a
new request ID merely to make a failed request appear successful. On version
conflict, compare the new current content, reconcile it, and use a new request
only for the new deliberate revision.

Human review happens in Multica and is pinned to the candidate and Git HEAD.
Agents cannot approve, configure Git permissions or impersonate a reviewer
through these commands. Native repository approvals/checks remain required.
Verified requirement merge starts planning only under the project's configured
policy and current execution permission; when disabled, the same planning task
waits for a human start. Turning the switch back on does not release old waiting
tasks. A current approved plan is required before coding; this material loop
does not automatically launch coding or merge code.

These commands use the ordinary task API context. They are unavailable inside
a collaboration run. Keep the task-scoped token and project authorization
supplied by the runtime; do not substitute a human or unrelated workspace token.
