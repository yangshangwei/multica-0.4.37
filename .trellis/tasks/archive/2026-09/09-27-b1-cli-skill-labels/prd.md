# CLI skill labels

## Goal
Exercise the actual Cobra command route before importing implementation; then retain upstream coverage of list/add/remove, labels scoped by resource type, workspace identity and existing issue-label behavior. Relevant docs may be updated, but no shared client/schema/store/UI or backend implementation.

## Source
Upstream commit: `904693bed94f9bd0cd93908014114a9ebbead8e0`. The user approved this bounded backport.

## Allowed implementation/test files
- `server/cmd/multica/cmd_id_resolver.go`
- `server/cmd/multica/cmd_issue_label.go`
- `server/cmd/multica/cmd_label.go`
- `server/cmd/multica/cmd_label_test.go`
- `server/cmd/multica/cmd_skill_label.go`
- `server/cmd/multica/cmd_skill_test.go`

## Acceptance
- Observable red/green regression evidence.
- Relevant focused tests pass, with logs and command recorded.
- No schema, dependency, locale expansion or unrelated production edits.
- Leader reviews and commits separately; this child must not commit or merge.
