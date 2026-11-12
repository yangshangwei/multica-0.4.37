# R56 comment/selection subtask boundary — 2026-10-14

## Scope and implementation plan

The approved R56/AC87 contract rejects capture-based subtask preview and creation
for collaboration issues in both manual and agent modes. Existing ordinary issue
capture, including an ordinary child of a collaboration issue, must remain valid.

The gap is in `BuildSourceIssueSnapshot`: it resolves the source issue inside the
workspace and then projects its body/attachments without checking the authoritative
`issue_collaboration` relation. `BuildSourceContext` uses this helper before thread
assembly; both HTTP modes use it before cloning attachments or preparing an agent.
The manual issue and agent enqueue transactions also rebuild through this helper
before their required writes.

Add one shared resolved-issue guard there, with a bounded service error mapped to
the existing unsupported collaboration response. Lookup errors fail closed. Keep
the check on the source issue ID, not its parent. No router, SQL, core/views,
collaboration-field or recipient changes belong to this slice. UI entry hiding
remains P6 work.

Verification will use the task-owned `fields.env` and fake storage/provider data,
with a failing preview regression before implementation. Direct snapshot/build
calls and manual/agent HTTP requests must return no source payload, capture,
attachment copy or execution artifact. Ordinary controls and lookup-error tests
will run before completion; unrun checks are not passed.
