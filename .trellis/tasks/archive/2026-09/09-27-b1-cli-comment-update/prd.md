# CLI comment editing

## Goal
Exercise missing command registration first; import the approved CLI implementation/tests. Keep positive expected-revision mandatory, preserve UTF-8/stdin/file input and existing server permission/concurrency semantics. Update current working-on-issues skill and source-map; no resurrected multica-platform.

## Source
Upstream commit: `91ad87186437230303d7a6862a16089656b4fdbe`. The user approved this bounded backport.

## Allowed implementation/test files
- `server/cmd/multica/cmd_issue.go`
- `server/cmd/multica/cmd_issue_test.go`

## Acceptance
- Observable red/green regression evidence.
- Relevant focused tests pass, with logs and command recorded.
- No schema, dependency, locale expansion or unrelated production edits.
- Leader reviews and commits separately; this child must not commit or merge.
