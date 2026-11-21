# Inbox snooze acceptance

Status: implementation and the scoped quality gate passed on 2026-11-21. The
user approved the final implementation plan before execution. Web and Desktop
share the delivered feature; the native mobile entry remains outside scope.

## Observable acceptance

| Criterion | Evidence |
| --- | --- |
| AC1: personal issue-group/item isolation | Inbox handler group/item tests and the real group lifecycle journey |
| AC2: five presets, local calendar, future/custom/DST time | Core snooze-time suite, exact-instant picker regressions and Chinese/native custom-time journeys |
| AC3–AC4: all entries, Snoozed list, failure recovery, selection/focus/touch | Views 245 assertions and final production Web/native controls |
| AC5: ordinary suppression, important wake, replay and first-write races | Listener/producer suites, full inboxstate normal/race, source-workspace/two-device journey |
| AC6: server expiry/restart, original identity, no duplication | Real expiry journey and actual owned API restart with the same binary/database before the saved deadline |
| AC7: exact undo and stale-operation refusal | API/Views regressions, state transactions and real failed-save/409/native undo journeys |
| AC8: bulk effective-read, archive/delete/revoke and cleanup | Handler/producer normal/race, archive journey and zero-residual audits on fresh databases |
| AC9: grouped counts and source-workspace recovery | Core queries/realtime, grouped server SQL and two-device journey while another workspace is active |
| AC10: old fields/malformed receipts, actual native identities | Strict API suite, old-response tests and three independent real Electron views restored after reload |

## Final verification

- Core: 375 assertions; Views: 245 assertions. Native store follow-up: 83
  assertions; Core tab subject/presentation follow-up: 64 assertions.
- Full `pnpm typecheck` and `pnpm lint` passed. Lint reports existing warnings
  outside this feature; the task files have no lint errors.
- Go: 201 named results per mode across cmd/server, handlers, services,
  inboxstate and migration guards. Normal/race name sets match exactly; no
  failures or skips. Cmd/handler/service cleanup replay used fresh r2 databases,
  retained the original residue audit, and returned all Inbox/state/source rows
  and connections to zero. State/migration final databases were already clean.
- `go build ./...`, scoped `go vet`, sqlc generation, migration direction/index
  checks, UI exports and task context validation passed.
- Production Web and Desktop builds passed. Final E2E r10 ran all six journeys
  together: 6 passed, 0 failed/skipped/retried/flaky, 31.95 seconds. Original
  failed runs remain retained, including the native identity regression that
  led to the actual Desktop resource-key fix.
- Actual API process restart preserved the original schedule/notification and
  returned it unread once after expiry. The original and restarted processes
  used the same binary hash and database.
- Visual confirmation: 94/pass against the approved compact Inbox brief.
  Real ThemeProvider controls, readable dark native input, 44px settled touch
  targets and wide/narrow Web/native captures were checked. The mechanical
  Impeccable detector returned no findings.
- Inbox user documentation, built-in notification guidance and embedded Chinese
  help are synchronized. Private docs generation matched 44 pages/112 assets;
  22 named built-in guidance checks passed after keeping the added note within
  the existing skill body budget.

## Evidence locations and limits

The task's `final-go-r2-results.json`, `final-go-name-summary.json`,
`final-go-clean-audit.json`, `final-e2e-verification.json`,
`restart-process-proof.json`, `restart-final-proof.json`, visual verdicts and
private reviewer receipts retain the exact commands, named outcomes and hashes.
Raw logs, synthetic profiles/accounts and screenshots remain under the private
allocation referenced by `verification-allocation.json`; credentials are not
stored in this task record.

Validation covers this feature and affected paths. This is not a claim that the
entire unrelated repository regression matrix or `make check` was executed.
Production deployment, installers/updaters, native mobile UI, batch snooze,
recurring reminders and working-day calendars remain outside scope.
