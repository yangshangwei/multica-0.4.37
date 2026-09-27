# CLI skill labels implementation evidence

Worktree: `/Volumes/artisan/code/2026/multica-upstream-b1`.
Source: `904693bed94f9bd0cd93908014114a9ebbead8e0`, limited to the six approved CLI files.

## Red / green

Commands ran from `server/`, always through the installed-agent CLI guard:

- Baseline: `bash ../scripts/go-test-with-agent-cli-guard.sh -- go test -p 2 -parallel 2 ./cmd/multica -run 'Skill|Label' -count=1` — exit 0, package 1.322s.
- Red: same command with `-run '^TestSkillLabelCommandsRegistered$'` — exit 1; real `rootCmd.Find` resolved each of `skill label list/add/remove` only to `multica skill`, leaving the missing subcommands unconsumed.
- Initial green after applying the six-file diff — exit 0, package 0.701s.
- Final focused tests, including local identity/prefix/description assertions — exit 0, package 0.726s.
- `bash ../scripts/go-test-with-agent-cli-guard.sh -- go vet ./cmd/multica` — exit 0.
- `gofmt -l` on the six CLI files produced no output; scoped `git diff --check` passed.

Logs and the exact upstream patch are outside the repository at:
`/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-upstream-b1-execution-amgeklaq/b1-labels-{baseline,red,green,final-tests,vet}.log`
and `b1-labels-upstream.patch` in that directory.

## Adaptations and boundaries

Production behavior is the approved upstream six-file patch. Existing executable bits were preserved despite the upstream 100644 context; the new file is 100644.

Kept the real Cobra registration test because upstream handler tests do not cover command registration. Added checks for workspace/agent/task headers with a fake task token, issue-only short-label prefix lookup, and both setting and clearing label descriptions. Existing issue reference behavior is unchanged.

Updated only the English/Chinese CLI command rows and the current `multica-skill-importing` skill/source map. The Chinese intranet content remains untouched. No backend handlers, schemas, stores, UI, locales, dependencies, migrations, task metadata, generated docs, ledger, or commits were changed by this executor.

The leader still needs to regenerate embedded Chinese help and perform the wider integration checks before committing. No real agent CLI or live task account was used. Labels are ready for leader review; comment-update work has not started.
