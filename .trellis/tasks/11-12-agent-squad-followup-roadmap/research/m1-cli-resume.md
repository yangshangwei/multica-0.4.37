# M1-S1 CLI Implementation Evidence

Date: 2026-11-13. Scope: approved ordinary single-issue completion CLI (P4).
This lane does not change authorization backend behavior, shared Core/Web/Desktop,
the independent collaboration/R38 flow, or real Provider/deployment gates.

## Delivered

- Registered `issue completion` with `prepare`, `authorize`, `submit`, `show`,
  and `revoke`. Existing `issue status`, status-only `issue update`, their
  non-authorizing hint, and existing `--no-start` behavior remain unchanged.
- `prepare` reads paged current-authority evidence candidates, or previews an
  explicitly selected approval and writes a private operation file. It never
  selects the newest candidate automatically.
- `authorize` requires the human CLI identity, dedicated `--accept`, a reason,
  and UTC microsecond expiry within the server-provided horizon. It binds the
  selected issue/version/project/policy/evidence and human consumer.
- `submit` uses only the exact status and original completion binding. It saves
  the dispatch marker before the single non-redirected PUT.
- `show` and `revoke` retain original request/authorization lookup identity.
  Unknown completion/revocation outcomes do not resend. `issued/not_checked`
  after a possible PUT remains unknown. A consumed receipt and a reopened
  current issue are shown independently.
- The versioned operation file is created exclusively with mode 0600, rejects
  symlinks or widened permissions, checks an integrity digest, uses a
  per-file exclusive lock and atomic replacement, and contains no bearer or
  evaluation key.
- Updated both built-in issue/agent skill guidance files and their source maps.

## TDD And Verification

- Initial red: `go -C server test ./cmd/multica -run '^TestIssueCompletion' -count=1`
  compiled and failed with `issue completion command is missing` and
  `unknown flag: --status` across the public-command fake-server cases.
- Additional regression red: removing the `show` guard after a recorded PUT
  caused `TestIssueCompletionUnknownSubmitIsLookupOnlyAndShowSeparatesCurrent`
  to report `authorized` with exit success for `issued/not_checked`. Restoring
  the guard changed this to `unknown` and non-success without another PUT.
- Focused green: `go -C server test ./cmd/multica -run '^TestIssueCompletion' -count=1`.
- Full CLI package: `go -C server test ./cmd/multica -count=1` passed.
- Static/build: `go -C server vet ./cmd/multica` passed;
  `go -C server build ./cmd/multica` passed.
- Embedded skill contracts:
  `go -C server test ./internal/service -run 'TestBuiltinSkillsConformToTemplate|TestWorkingOnIssuesSkillCoversIssueLoopContracts|TestCreatingAgentsSkillCoversAgentCreationContracts' -count=1`
  passed, including the 500-line skill body limit.
- `git diff --check` passed for the owned CLI, test, and skill files.
- No database, real Provider, Hosted CI, release, or deployment action was run.

## SHA-256

```text
c230463fca86aaa2a48bd7bc95dda7e3fc9a5516830027b2b5bd3ead4a860769  server/cmd/multica/cmd_issue.go
70b50080224dda9c4e95920d96b4ec7e1be449d8e8dfa283fd99f11b7c5e81a7  server/cmd/multica/cmd_issue_completion.go
b9020770d27629ecd66f0f75666948e1fb4f52d66621b80883ae8e1005cde495  server/cmd/multica/cmd_issue_completion_test.go
eea9064dc21d1ffc29a32e7397ff964f77853ca82ecf19d1ff212608d1cf01a8  server/internal/service/builtin_skills/multica-working-on-issues/SKILL.md
7758ff07305f84cb6abb80e9c696b2ae45598f45a5ed43aa3f721167dbae6b71  server/internal/service/builtin_skills/multica-working-on-issues/references/working-on-issues-source-map.md
1f1895a1a0c0c41898f38b8c2fe85fcfe20e8aab6c3db0029485cc860124c4e7  server/internal/service/builtin_skills/multica-creating-agents/SKILL.md
aa39104c1e57ae35c0a3d7e62bb5d2d27b08c875d4297c5a147daac21e8b8342  server/internal/service/builtin_skills/multica-creating-agents/references/creating-agents-source-map.md
```

The hashes above were captured after implementation/docs sync and before this
evidence file was added. No product test result from an earlier stage is used as
evidence for this CLI lane.
