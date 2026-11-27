# Git requirement and plan materials

Use this workflow when the task names a project material or asks for a PRD,
technical design or task breakdown in the configured material repository.
Git owns the versioned definitions; Multica owns discussion, review decisions,
assignment and execution. A normal project resource is not automatically a
material output repository. A human configures the repository, reviewers and
one execution runtime in the project's Engineering materials tab. That runtime
performs publication, review-note synchronization, approved merge and planning
with its own local Git credentials. Material operations do not require a GitHub
App or server-side GitLab connection.

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

## Repository initialization and rule maintenance

The configured repository/root must have a current verified initialization
proof before a requirement can be published. Saving configuration starts a
read-only check on the selected runtime. A compatible existing foundation can
be adopted after independent readback; missing foundation files produce a
separate initialization review. Workspace owner/admin maintenance authority
approves those rule changes. Ordinary requirement reviewer rights do not grant
authority to modify AGENTS.md, material.yaml, schemas or templates.

You may save `ready=false` drafts while initialization is pending or blocked.
Do not work around `material_repository_not_ready` by writing foundation files
through a requirement revision. Follow the initialization state and remediation
in the project's Engineering materials panel. Existing files are preserved;
unknown rule content, partial reads and missing base branches require explicit
resolution. The workflow does not silently create an initial default branch.

An assigned initialization operation has the `repository_initialization`
target and a frozen file manifest. Use the same operation/permit/observe
commands below. Publish only the exact additions or reviewed maintenance
before/after changes in that descriptor; no arbitrary rule edits are allowed.
An initialization proof is neither a requirement baseline nor approval to code.
Rule upgrades require their own maintenance candidate and current human review;
a request for the already adopted supported bundle creates no redundant PR.

The daemon checks protected rules again before ordinary material execution.
Changed rules or a changed runtime/configuration invalidate old authority;
do not replace the task identity or choose another runtime to bypass refusal.

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

## Execute an assigned Git operation

An operation task supplies one immutable operation UUID and a stage-specific
descriptor. Read that descriptor with:

```bash
multica project material operation <operation-uuid>
```

Use only the assigned repository, source/target branches, exact document bytes
and authorized stage. Obtain the current operation permit before the remote
write using a retained request UUID:

```bash
multica project material permit <operation-uuid> --request-id <request-uuid>
```

Publication is not merge authorization. A merge requires the recorded
human decision, its exact HEAD and satisfied native checks; never bypass native
protection. Run `observe` before requesting a merge permit so the server can
check current remote facts. Use the selected runtime's Git/hosting tools for writes.

After the remote action, request independent readback:

```bash
multica project material observe <operation-uuid>
```

The local daemon reads the actual review, complete changes and immutable file
bytes. This command does not accept a receipt JSON or perform a remote write.
Model output, a review URL and exit status alone cannot establish a baseline.
The daemon's private observation credentials are not task credentials. Keep the
original operation identity on uncertain results and reconcile before another
write. If the runtime, permission or configuration changed, report the blocker;
do not choose another runtime or substitute a human token.

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

Before a planning model starts, the daemon independently reads the fixed
requirement baseline in the selected runtime environment. Missing or changed
bytes block that start. A human may retry a terminal planning attempt with no
submitted plan; the original issue and previous runs remain in history.

Multica controls its own stage authorization and acceptance of verified facts.
This is not operating-system isolation of arbitrary shell commands on a host
that already has broader Git credentials.

These commands use the ordinary task API context. They are unavailable inside
a collaboration run. Keep the task-scoped token and project authorization
supplied by the runtime; do not substitute a human or unrelated workspace token.
