# CLI comment update implementation evidence

Worktree: `/Volumes/artisan/code/2026/multica-upstream-b1`.
Source: `91ad87186437230303d7a6862a16089656b4fdbe`, limited to cmd_issue.go and cmd_issue_test.go.

## Verification

All Go commands ran from server/ through the installed-agent CLI guard.

- Baseline: `bash ../scripts/go-test-with-agent-cli-guard.sh -- go test -p 2 -parallel 2 ./cmd/multica -run 'Comment' -count=1` — exit 0, 0.906s.
- Red: same command with `-run '^TestIssueCommentUpdateCommandRegistration$'` — exit 1, 1.132s. Actual rootCmd.Find returned `multica issue comment` with remaining `[update]`, instead of the update command.
- Initial green after port: Comment suite exit 0, 1.351s.
- Final: `bash ../scripts/go-test-with-agent-cli-guard.sh -- go test -p 2 -parallel 2 ./cmd/multica -run 'Comment|ResolveTextFlag|GuardLocalPathLinks' -count=1` — exit 0, 0.746s.
- `bash ../scripts/go-test-with-agent-cli-guard.sh -- go vet ./cmd/multica` — exit 0.
- gofmt -l on both Go files was empty; scoped git diff --check passed.

Logs and the original two-file patch are in:
`/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-upstream-b1-execution-amgeklaq/`
with names `b1-comments-baseline.log`, `b1-comments-red.log`, `b1-comments-green.log`,
`b1-comments-final-tests.log`, `b1-comments-vet.log`, and `b1-comments-upstream.patch`.

## Adaptations

The temporary red test was replaced by the upstream canonical registration test using the full root Cobra route; there is only one registration test. Existing executable bits were preserved.

Help includes the conflict recovery guidance from c3920bc05 (#8548), with trigger wording matching the existing local server: re-evaluate using the editor's current invocation permissions, not a guarantee that every mentioned agent starts. Its unrelated skill deletion was not imported. Added stdin/new-revision response coverage and expanded the existing API-error test to 403/409, checking exactly one request and no success output. Shared text/encoding/workdir guard matrices remain in TestResolveTextFlag rather than being duplicated.

Only the two Go files, English/Chinese CLI pages, and the existing multica-working-on-issues skill/source map changed. Label documentation is preserved. The leader owns embedded Chinese help generation and broader integration validation; no generated documentation, handlers, schemas, dependencies, DB, UI, commits or original checkout files were changed by this executor.

## Isolated CLI HTTP smoke contract

The update command makes no preliminary GET. It sends PUT /api/comments/<comment UUID>, Content-Type application/json, with only `content` (string) and `expected_revision` (positive integer). Existing newAPIClient adds X-Workspace-ID and, in a task context, X-Agent-ID / X-Task-ID; task context requires a fake mat_-prefixed token in tests.

`issue comment list --output json` receives a bare array of comment objects; using a full issue UUID avoids the issue-key resolver GET.

A successful real server response is HTTP 200 with a bare CommentResponse object, including id, issue_id, content, revision, existing attachments, optional issue_revision and trigger_outcomes. Default JSON mode prints the response object to stdout and `Comment <id> updated.` to stderr. Table mode prints only that stderr confirmation.

A stale revision returns HTTP 409 with error `resource changed since it was loaded`, code `revision_conflict`, resource_type `comment`, resource_id, expected_revision and actual_revision. A forbidden edit returns 403. Both cause a nonzero CLI result without an automatic retry; validation failures for missing/zero/negative revision or missing content happen before HTTP. The request omits attachment_ids and suppress_agent_ids, preserving the existing attachment set and server-side trigger/authorization behavior.
