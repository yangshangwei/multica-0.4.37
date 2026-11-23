# PG implementation and verification

Date: 2026-10-05. Worktree: `/Volumes/artisan/code/2026/multica-projects-p1`, branch `codex/projects-p1`. Foundation gate was verified by the parent at `c4250df70`; the shared health collector is available from health's HG work. This report covers the progress backend. It does not certify the Web/Desktop journey, mobile behavior, production rollout, or I1 integration.

## Delivered behavior

- `project_update.go`: preview, create, correction, timeline and revision history; actual human actor plus machine-credential gate; current membership and source authorization in the shared RR transaction; exact request replay; description-bound immutable acceptance; per-source observed/current versions and historical redaction; identity-only project events.
- `project_update_notifications.go`: durable explicit-member mention notifications, immutable source revision, lock-free candidate scan, individual RC transactions, workspace → recipient fence/member → project → outbox ordering, atomic inbox/outbox delivery, five-second polling, bounded retry/backoff and twelve-attempt dead letter, context cancellation.
- Shared statistics use `projecthealth.Collect` and `StatisticsSnapshot` directly in the publishing transaction. Incomplete/error facts return `project_health_unavailable`; no client counts are accepted. Corrections without recollection preserve the original statistics bytes semantically, including the original timestamp/version.
- `FF_PROJECTS_P1=false` prevents new publishing/correction after the authorized exact-request replay check. Existing history and completed request results remain readable.
- SQL proposal was integrated by foundation into `queries/project_update.sql` and its generated code. Foundation also owns protocol constants, routes, capability flags and the worker registration beside the triage worker on `sweepCtx`. No generated or shared foundation file was edited by this owner.

## Verification commands and results

All runtime tests use the parent's separately migrated progress database, never the shared application database or real agent accounts:

```sh
set -a
source .env.worktree
source .omx/projects-p1-test-env/progress.env
set +a
bash scripts/go-test-with-agent-cli-guard.sh go -C server test -race ./internal/handler -run '^TestProjectUpdate' -count=1 -json
go -C server vet ./internal/handler
git diff --check
```

Final race run: **exit 0, 3.233s, 25 top-level tests / 32 including subtests, zero failures, zero skips**. The prefix includes foundation's `TestProjectUpdateConstraintErrorPreserved`; progress owns 24 top-level tests / 31 including subtests. Full structured output: [evidence/go-race.jsonl](evidence/go-race.jsonl). Handler vet and whitespace checks passed. No real agent executable was invoked by the guarded tests.

Observed RED → GREEN evidence during development:

| Missing behavior | Observed failure | Final proof |
| --- | --- | --- |
| First preview endpoint | expected 200, actual 501 | Create/replay/correction contract succeeds |
| Notification worker | explicit failure: worker not implemented | Immediate scan, one inbox item, cancellation succeeds |
| Statistics publication | expected 200, actual 503 `project_health_unavailable` | Shared collector snapshot and stale-preview matrix succeeds |
| Read-only rollback | expected 403, actual 201 | Flag-off new writes rejected, exact replay/history succeeds |
| Unavailable statistics error | actual `project_write_retry_exhausted` instead of `project_health_unavailable` | Classified 503 and no published rows |

Temporary fixture failures (missing execution runtime IDs) were corrected as test setup failures, not reported as product regressions. During parallel development the package briefly included other owners' incomplete RED tests; those intermediate compilation/assertion failures are not included in the final pass count.

## Requirement evidence

| Contract / requirement | Canonical proof |
| --- | --- |
| PRJ-003, AC-05/06/20 | `AcceptanceKeepsOriginalDescription`, `ValidationAndHumanGate`: explicit passed/partial basis, original text/version retained after description change and correction |
| PRJ-008/009, AC-18/19/20 | `CreateReplayAndImmutableCorrection`: replay always returns the request's original revision, append-only history, corrected author/time preserved |
| PRJ-008/014, AC-26/27, FR-16/18 | `LegacyAgentRejectedAndEventsContainOnlyIdentity`, `HumanMembersKeepActualAuthorship`, `MentionsNeverExecuteAndNotifyOnce`: member/admin/owner authorship, task/cloud/legacy agent exclusion, only identity events, unchanged execution count |
| FR-01/17 | `CharacterEvidenceAndMalformedInputBoundaries`, `CanonicalFixtureAndNormalization`: Unicode limits, correction/evidence bounds, forged recipients/statistics rejected, CRLF normalization, safe unverified HTTP(S), deduplication and independent canonical SHA vector |
| FR-05/09 | `StatisticsSnapshotIsServerGeneratedAndImmutable`, `UnavailableStatisticsNeverPublish`: actual shared transaction snapshot, relevant preview change rejected, no-statistics intent survives unrelated task changes, correction preserves historical facts |
| FR-06 | `EvidenceChangeRequiresFreshPreview`, `ExecutionEvidenceRechecksResultAndPrivacy`: issue revision/result-digest changes, private-agent and chat-creator restrictions, source label/href/current-version redacted on later reads |
| FR-06 current authorization | `SourceLockRetriesInsteadOfReadingStaleVersion`, `EvidenceGrantRevocationRestartsRRAuthorization`, `MembershipLockRejectsRevokedRRSnapshot`: source, allowlist and actor membership changed after the RR snapshot all restart and reject stale authority |
| FR-07 | `ConcurrentRequestsAndCorrections`, `ConcurrentRequestIdentityAcrossProjects`: same intent produces one ID; two correction writers have one winner; same request with different project/payload commits only one intent |
| AC-18, FR-09 | `HistoryIdentityAndCursorScope`: stable time/id keyset, cross-project cursor rejection, departed author retained, deleted editor placeholder and no email exposure |
| FR-10/11/12 | `NotificationDeletionBothLockOrders`, `NotificationRevocationBothLockOrders`: barriers hold the first transaction and use `pg_stat_activity` to prove the second is actually waiting for a lock; completion leaves no recreated inbox or project outbox |
| FR-12 | `NotificationConcurrentWorkersAndCorrection`, `NotificationFailureRecoveryAndDeadLetter`: two workers deliver once, correction only adds new recipients, rollback leaves no inbox, restart/existing inbox deduplication, exact backoff caps and twelve-attempt stop |
| FR-14 | `FlagOffRetainsHistoryAndExactReplay`: server-side read-only behavior retains history/results and prevents a second revision |

Health owns latest/current acceptance selection and formal-scope matrices; those are covered by the shared health suite rather than duplicated here. The parent owns complete route/E2E, migration/catalog, global regression and performance evidence.

## Operational and remaining integration boundaries

- Delivery timeout is five seconds per record; a restart resumes pending rows. Error logs contain IDs/codes/counts, not body/evidence/token contents. Dead letters remain available for existing operational DB inspection/reset; there is no new external notification channel.
- Referenced URLs are never fetched. The UI owner must render the returned references with existing safe link components. The backend stores no copied execution result text in evidence.
- This owner made no new dependency, migration, execution-path or frontend changes. Broader lint/typecheck/E2E and a final cross-owner review remain parent verification responsibilities. No production deployment or push occurred.

## Independent review corrections

The independent frontend/backend review found an actual inbox wire regression and unusable internal evidence URLs, plus a limit in the original synthetic revocation proof. These were verified against the current code and addressed using the receiving-code-review and TDD workflows.

- **FR-01:** the real notification worker emitted numeric `details.revision`. `TestProjectUpdateInboxDetailsRemainStringValues` delivered a notification and called the actual member-protected `ListInbox`; RED failed with `cannot unmarshal number ... details.revision ... string`. The worker now builds `map[string]string` and formats revision with `strconv.FormatInt`. The mixed old/new response retains both items. The captured response is [evidence/inbox-wire.json](evidence/inbox-wire.json), with its passing [test log](evidence/inbox-wire-test.log). Mobile reused this exact response as its compatibility fixture.
- **FR-07:** RED observed `/issues/{id}` and an absent execution reader. Internal evidence now has `href=null`; `input.kind/id` and the record's workspace identity define the navigation target. External URLs retain their HTTP(S) href. The new `GetProjectUpdateExecutionEvidence` reader returns a real task and real messages for an execution actually referenced by the requested immutable revision. It shares the existing RR/source authorization path, including private-agent/allowlist and chat-creator restrictions; it does not use the less restrictive global task-message endpoint or introduce a task page. The parent registers the route and UI uses its existing transcript dialog. The approved API-contract supplement describes this read-only endpoint.
- **Real revocation proof:** `TestProjectUpdateRealLeaveWorkspaceRevokesAccessAndStopsDelivery` calls the actual `LeaveWorkspace` handler for pending and already-delivered notifications. Subsequent project/history/replay access is denied, member-protected inbox access returns the established 404, and no additional inbox delivery appears. Already-delivered rows may remain stored; this test deliberately distinguishes authorization from row retention. The earlier `progressRevoke` helper remains only a controlled lock-protocol fixture, not proof of real endpoint cleanup behavior.

Final correction verification: the same guarded private-database command with `-race -run '^TestProjectUpdate' -count=1` passed **29 top-level tests / 38 including subtests, exit 0, 3.810s, no failures/skips**. This includes foundation's one CHECK compatibility test; progress owns 28 top-level tests. Full output: [evidence/review-fixes-race.jsonl](evidence/review-fixes-race.jsonl). `go -C server vet ./internal/handler` and `git diff --check` also passed. The new evidence-reader regression checks real task/message identity, unreferenced execution404, cross-project/workspace404, private-agent403, chat non-creator403, creator success and lost-membership403.
